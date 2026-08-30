package service

import (
	"strings"
	"sync"
	"testing"
	"time"

	"xianlv/internal/model"
)

func setCompensationRegistrationTime(t *testing.T, game *Game, player model.Player, createdAt time.Time) model.Player {
	t.Helper()
	if err := game.store.DB.Model(&model.Player{}).Where("id = ?", player.ID).UpdateColumn("created_at", createdAt).Error; err != nil {
		t.Fatal(err)
	}
	updated, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	return updated
}

func TestV221ServerCompensationOldPlayerClaimsExactlyOnce(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "compensation-old-account", "归墟散人")
	// The published cutoff is inclusive down to the exact second.
	player = setCompensationRegistrationTime(t, game, player, v221CompensationCutoff)

	menu, handled, err := game.Execute("group", player.AccountID, mustParse(t, "活动菜单"))
	if err != nil || !handled || !strings.Contains(menu.Content, "全服补偿") || !containsAction(menu.Actions, "补偿公告") || !containsAction(menu.Actions, "领取全服补偿") {
		t.Fatalf("compensation missing from activity menu: handled=%v err=%v result=%+v", handled, err, menu)
	}
	var notice model.Notice
	if err := store.DB.Where("code = ?", "world_notice_v222_compensation_20260724").First(&notice).Error; err != nil || !notice.Published || notice.Type != "公告" || !strings.Contains(notice.Content, "玄铁×188") || !strings.Contains(notice.Content, "本批次不发仙金") {
		t.Fatalf("compensation notice is incomplete: err=%v notice=%+v", err, notice)
	}
	var memorial model.Item
	if err := store.DB.Where("code = ?", "v222_runtime_repair_memorial_token").First(&memorial).Error; err != nil || memorial.Tradable || memorial.EffectValue != 0 || memorial.BaseValue != 0 {
		t.Fatalf("memorial token must not affect power or economy: err=%v item=%+v", err, memorial)
	}
	announcement, handled, err := game.Execute("group", player.AccountID, mustParse(t, "补偿公告"))
	if err != nil || !handled || !strings.Contains(announcement.Title, "v2.2.2 全服补偿公告") || !strings.Contains(announcement.Content, "角色等级基础属性") || !strings.Contains(announcement.Content, "造化仙壤×20") || !containsAction(announcement.Actions, "领取全服补偿") {
		t.Fatalf("compensation announcement command is incomplete: handled=%v err=%v result=%+v", handled, err, announcement)
	}
	overview, handled, err := game.Execute("group", player.AccountID, mustParse(t, "全服补偿"))
	if err != nil || !handled || !strings.Contains(overview.Content, "当前状态：可领取") || !strings.Contains(overview.Content, "不发仙金") {
		t.Fatalf("compensation overview: handled=%v err=%v result=%+v", handled, err, overview)
	}

	before, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	itemBefore := make(map[string]int64, len(v221CompensationReward.Items))
	for name := range v221CompensationReward.Items {
		item, itemErr := game.itemByName(name)
		if itemErr != nil {
			t.Fatalf("reward item %s is not seeded: %v", name, itemErr)
		}
		itemBefore[name] = game.itemQuantity(player.ID, item.ID)
	}

	claimed, handled, err := game.Execute("group", player.AccountID, mustParse(t, "领取全服补偿"))
	if err != nil || !handled || !strings.Contains(claimed.Title, "领取成功") || !strings.Contains(claimed.Content, "万象归元纪念令×1") {
		t.Fatalf("claim compensation: handled=%v err=%v result=%+v", handled, err, claimed)
	}
	after, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.SpiritStones-before.SpiritStones != v221CompensationReward.SpiritStones || after.SilverCoins-before.SilverCoins != v221CompensationReward.SilverCoins || after.Merit-before.Merit != v221CompensationReward.Merit || after.Reputation-before.Reputation != v221CompensationReward.Reputation || after.ImmortalJade != before.ImmortalJade {
		t.Fatalf("unexpected compensation currency delta: before=%+v after=%+v", before, after)
	}
	for name, quantity := range v221CompensationReward.Items {
		item, _ := game.itemByName(name)
		if got := game.itemQuantity(player.ID, item.ID); got != itemBefore[name]+quantity {
			t.Fatalf("reward item %s quantity=%d, want=%d", name, got, itemBefore[name]+quantity)
		}
	}
	var receipts int64
	if err := store.DB.Model(&model.AccountRewardClaim{}).Where("claim_key = ? AND account_id = ?", v221CompensationClaimKey, player.AccountID).Count(&receipts).Error; err != nil || receipts != 1 {
		t.Fatalf("claim receipt count=%d err=%v", receipts, err)
	}
	// Defense in depth: a changed OpenID cannot create a second receipt for
	// the same permanent character even if a caller bypasses the service check.
	duplicatePlayerReceipt := model.AccountRewardClaim{
		AccountID: "compensation-other-open-id", ClaimKey: v221CompensationClaimKey,
		PlayerID: player.ID, RewardJSON: `{}`, ClaimedAt: time.Now(),
	}
	if err := store.DB.Create(&duplicatePlayerReceipt).Error; err == nil {
		t.Fatal("player-scoped unique receipt constraint accepted a duplicate")
	}

	again, handled, err := game.Execute("group", player.AccountID, mustParse(t, "领取全服补偿"))
	if err != nil || !handled || !strings.Contains(again.Title, "已经领取") {
		t.Fatalf("duplicate compensation was not rejected: handled=%v err=%v result=%+v", handled, err, again)
	}
	repeated, _ := game.players.Get(player.ID)
	if repeated.SpiritStones != after.SpiritStones || repeated.SilverCoins != after.SilverCoins || repeated.Merit != after.Merit {
		t.Fatalf("duplicate claim changed balances: after=%+v repeated=%+v", after, repeated)
	}

	// Exercise the real migration flow. It keeps the same player ID, while the
	// released old OpenID may later establish another character. Neither path
	// may claim the same account-scoped repair reward again.
	const migratedAccount = "compensation-migrated-account"
	generated, handled, err := game.createAccountMigrationCode(&player)
	if err != nil || !handled {
		t.Fatalf("create migration code: handled=%v err=%v result=%+v", handled, err, generated)
	}
	token := ""
	for _, line := range strings.Split(generated.Content, "\n") {
		if strings.HasPrefix(line, "迁移凭证：") {
			token = strings.TrimSpace(strings.TrimPrefix(line, "迁移凭证："))
			break
		}
	}
	if token == "" {
		t.Fatalf("migration token missing: %s", generated.Content)
	}
	imported, handled, err := game.importAccountMigration("migrated-group", migratedAccount, token)
	if err != nil || !handled || !strings.Contains(imported.Title, "迁移成功") {
		t.Fatalf("import migration: handled=%v err=%v result=%+v", handled, err, imported)
	}
	migratedAgain, handled, err := game.Execute("group", migratedAccount, mustParse(t, "领取全服补偿"))
	if err != nil || !handled || !strings.Contains(migratedAgain.Title, "已经领取") {
		t.Fatalf("migrated player bypassed receipt: handled=%v err=%v result=%+v", handled, err, migratedAgain)
	}
	reusedAccountPlayer := registerPlayer(t, game, player.AccountID, "临川散人")
	reusedAccountPlayer = setCompensationRegistrationTime(t, game, reusedAccountPlayer, v221CompensationCutoff.Add(-time.Minute))
	reusedAgain, handled, err := game.Execute("group", reusedAccountPlayer.AccountID, mustParse(t, "领取全服补偿"))
	if err != nil || !handled || !strings.Contains(reusedAgain.Title, "已经领取") {
		t.Fatalf("released account bypassed receipt: handled=%v err=%v result=%+v", handled, err, reusedAgain)
	}
}

func TestV221ServerCompensationConcurrentClaimsGrantOnce(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "compensation-concurrent-account", "并明散人")
	player = setCompensationRegistrationTime(t, game, player, v221CompensationCutoff.Add(-time.Minute))
	before, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}

	const attempts = 8
	type claimResult struct {
		result  GameResult
		handled bool
		err     error
	}
	start := make(chan struct{})
	results := make(chan claimResult, attempts)
	var wg sync.WaitGroup
	for index := 0; index < attempts; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			result, handled, claimErr := game.claimV221ServerCompensation(&player)
			results <- claimResult{result: result, handled: handled, err: claimErr}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	successes, duplicates := 0, 0
	for claim := range results {
		if claim.err != nil || !claim.handled {
			t.Fatalf("concurrent claim failed: handled=%v err=%v result=%+v", claim.handled, claim.err, claim.result)
		}
		switch {
		case strings.Contains(claim.result.Title, "领取成功"):
			successes++
		case strings.Contains(claim.result.Title, "已经领取"):
			duplicates++
		default:
			t.Fatalf("unexpected concurrent result: %+v", claim.result)
		}
	}
	if successes != 1 || duplicates != attempts-1 {
		t.Fatalf("concurrent outcomes: success=%d duplicate=%d", successes, duplicates)
	}

	after, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.SpiritStones-before.SpiritStones != v221CompensationReward.SpiritStones || after.SilverCoins-before.SilverCoins != v221CompensationReward.SilverCoins || after.Merit-before.Merit != v221CompensationReward.Merit {
		t.Fatalf("concurrent claim granted currency more than once: before=%+v after=%+v", before, after)
	}
	var receipts int64
	if err := store.DB.Model(&model.AccountRewardClaim{}).Where("claim_key = ?", v221CompensationClaimKey).Count(&receipts).Error; err != nil || receipts != 1 {
		t.Fatalf("concurrent receipt count=%d err=%v", receipts, err)
	}
}

func TestV221ServerCompensationRejectsNewRegistration(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "compensation-new-account", "新雨散人")
	player = setCompensationRegistrationTime(t, game, player, v221CompensationCutoff.Add(time.Second))

	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "领取全服补偿"))
	if err != nil || !handled || !strings.Contains(result.Title, "不在本次补偿范围") {
		t.Fatalf("new player eligibility boundary failed: handled=%v err=%v result=%+v", handled, err, result)
	}
	var receipts int64
	if err := store.DB.Model(&model.AccountRewardClaim{}).Where("claim_key = ?", v221CompensationClaimKey).Count(&receipts).Error; err != nil || receipts != 0 {
		t.Fatalf("ineligible player created receipt: count=%d err=%v", receipts, err)
	}
}

func TestV221ServerCompensationRollsBackWhenRewardCannotComplete(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "compensation-rollback-account", "守一散人")
	player = setCompensationRegistrationTime(t, game, player, v221CompensationCutoff.Add(-time.Minute))
	before, _ := game.players.Get(player.ID)

	if err := store.DB.Unscoped().Where("name = ?", "万象归元纪念令").Delete(&model.Item{}).Error; err != nil {
		t.Fatal(err)
	}
	_, handled, err := game.Execute("group", player.AccountID, mustParse(t, "领取全服补偿"))
	if err == nil || !handled {
		t.Fatalf("incomplete reward should fail atomically: handled=%v err=%v", handled, err)
	}
	after, _ := game.players.Get(player.ID)
	if after.SpiritStones != before.SpiritStones || after.SilverCoins != before.SilverCoins || after.Merit != before.Merit {
		t.Fatalf("failed claim changed balances: before=%+v after=%+v", before, after)
	}
	var receipts int64
	if err := store.DB.Model(&model.AccountRewardClaim{}).Where("claim_key = ?", v221CompensationClaimKey).Count(&receipts).Error; err != nil || receipts != 0 {
		t.Fatalf("failed claim left receipt: count=%d err=%v", receipts, err)
	}
}
