package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xianlv/internal/config"
	"xianlv/internal/handler"
	"xianlv/internal/model"
	"xianlv/internal/storage"
)

func TestRankingCenterRewardsTopTenOncePerDay(t *testing.T) {
	game, store := testGame(t)
	champion := registerPlayer(t, game, "ranking-champion", "青霄剑主")
	challenger := registerPlayer(t, game, "ranking-challenger", "照夜真人")
	if err := store.DB.Model(&model.Player{}).Where("id = ?", champion.ID).Update("combat_power", 9000).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", challenger.ID).Update("combat_power", 3000).Error; err != nil {
		t.Fatal(err)
	}
	result, handled, err := game.Execute("group", champion.AccountID, mustParse(t, "排行 战力"))
	if err != nil || !handled || !strings.Contains(result.Content, "青霄剑主") || !strings.Contains(result.Content, "第1名") {
		t.Fatalf("combat ranking: handled=%v err=%v content=%s", handled, err, result.Content)
	}
	before, err := game.players.Get(champion.ID)
	if err != nil {
		t.Fatal(err)
	}
	reward, handled, err := game.Execute("group", champion.AccountID, mustParse(t, "领取排行奖励 战力"))
	if err != nil || !handled || !strings.Contains(reward.Content, "第1名") || reward.BroadcastContent == "" {
		t.Fatalf("ranking reward: handled=%v err=%v result=%+v", handled, err, reward)
	}
	after, err := game.players.Get(champion.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.SpiritStones-before.SpiritStones != 300 || after.Merit-before.Merit != 10 {
		t.Fatalf("unexpected ranking reward delta: stones=%d merit=%d", after.SpiritStones-before.SpiritStones, after.Merit-before.Merit)
	}
	again, _, err := game.Execute("group", champion.AccountID, mustParse(t, "领取排行奖励 战力"))
	if err != nil || !strings.Contains(again.Title, "已领") {
		t.Fatalf("duplicate reward was not rejected: err=%v result=%+v", err, again)
	}
}

func TestActivityCenterClaimsCodesInvitesAndTieredTasks(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "activity-player", "扶光散人")

	menu, handled, err := game.Execute("group", player.AccountID, mustParse(t, "活动菜单"))
	if err != nil || !handled || !strings.Contains(menu.Content, "七日目标") || !strings.Contains(menu.Content, "密令兑换") || !strings.Contains(menu.Content, "助力修炼") || !strings.Contains(menu.Content, "庆典特卖") || !containsAction(menu.Actions, "活动总览") {
		t.Fatalf("activity menu incomplete: handled=%v err=%v result=%+v", handled, err, menu)
	}
	overview, handled, err := game.Execute("group", player.AccountID, mustParse(t, "活动总览"))
	if err != nil || !handled || !strings.Contains(overview.Content, "【进行中】") || !strings.Contains(overview.Content, "倒计时：") || !strings.Contains(overview.Content, "个人：") || !containsAction(overview.Actions, "活动总览 2") {
		t.Fatalf("activity overview: handled=%v err=%v result=%+v", handled, err, overview)
	}

	benefit, handled, err := game.Execute("group", player.AccountID, mustParse(t, "领取七日福利"))
	if err != nil || !handled || !strings.Contains(benefit.Title, "已领取") || !strings.Contains(benefit.Content, "青云引路礼") {
		t.Fatalf("seven-day benefit: handled=%v err=%v result=%+v", handled, err, benefit)
	}
	again, _, err := game.Execute("group", player.AccountID, mustParse(t, "领取七日福利 青云引路礼"))
	if err != nil || !strings.Contains(again.Title, "已经领取") {
		t.Fatalf("duplicate seven-day benefit: err=%v result=%+v", err, again)
	}

	codeReward, handled, err := game.Execute("group", player.AccountID, mustParse(t, "密令兑换 XIANLV666"))
	if err != nil || !handled || !strings.Contains(codeReward.Title, "成功") || !strings.Contains(codeReward.Content, "灵果") {
		t.Fatalf("code redemption: handled=%v err=%v result=%+v", handled, err, codeReward)
	}
	codeAgain, _, err := game.Execute("group", player.AccountID, mustParse(t, "密令兑换 XIANLV666"))
	if err != nil || !strings.Contains(codeAgain.Title, "已经兑换") {
		t.Fatalf("duplicate code redemption: err=%v result=%+v", err, codeAgain)
	}

	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("realm_level", 2).Error; err != nil {
		t.Fatal(err)
	}
	realmReward, handled, err := game.Execute("group", player.AccountID, mustParse(t, "领取境界冲刺 炼气·引气成周"))
	if err != nil || !handled || !strings.Contains(realmReward.Title, "达成") || !strings.Contains(realmReward.Content, "获得") {
		t.Fatalf("realm sprint claim: handled=%v err=%v result=%+v", handled, err, realmReward)
	}

	invitee := registerPlayer(t, game, "activity-invitee", "听松道人")
	if _, handled, err := game.Execute("group", player.AccountID, mustParse(t, "邀请道友")); err != nil || !handled {
		t.Fatalf("create invitation code: handled=%v err=%v", handled, err)
	}
	code := alphaInvitationCode(player.ID)
	accepted, handled, err := game.Execute("group", invitee.AccountID, mustParse(t, "接受邀请 "+code))
	if err != nil || !handled || !strings.Contains(accepted.Title, "成功") || game.successfulInvitationCount(player.ID) != 1 {
		t.Fatalf("activity invitation: handled=%v err=%v result=%+v", handled, err, accepted)
	}
	duplicateInvite, _, err := game.Execute("group", invitee.AccountID, mustParse(t, "接受邀请 "+code))
	if err != nil || !strings.Contains(duplicateInvite.Title, "已绑定") {
		t.Fatalf("duplicate invite binding: err=%v result=%+v", err, duplicateInvite)
	}
	companion, handled, err := game.Execute("group", player.AccountID, mustParse(t, "结伴奖励 初识同道"))
	if err != nil || !handled || !strings.Contains(companion.Title, "已领取") {
		t.Fatalf("companion reward: handled=%v err=%v result=%+v", handled, err, companion)
	}

	prayer, handled, err := game.Execute("group", player.AccountID, mustParse(t, "限时祈福 问道"))
	if err != nil || !handled || !strings.Contains(prayer.Title, "问道签") {
		t.Fatalf("limited prayer: handled=%v err=%v result=%+v", handled, err, prayer)
	}
	prayerAgain, _, err := game.Execute("group", player.AccountID, mustParse(t, "限时祈福 纳福"))
	if err != nil || !strings.Contains(prayerAgain.Title, "已经祈福") {
		t.Fatalf("duplicate prayer: err=%v result=%+v", err, prayerAgain)
	}

	daily, handled, err := game.Execute("group", invitee.AccountID, mustParse(t, "日常"))
	if err != nil || !handled || !strings.Contains(daily.Content, "按前置境界从最低到最高排列") || !strings.Contains(daily.Content, "当前：炼气·1层") {
		t.Fatalf("tiered daily tasks: handled=%v err=%v result=%+v", handled, err, daily)
	}
	bounties, handled, err := game.Execute("group", invitee.AccountID, mustParse(t, "悬赏"))
	if err != nil || !handled || !strings.Contains(bounties.Content, "从最低到最高排列") || strings.Contains(bounties.Content, "第100境") {
		t.Fatalf("tiered bounty tasks: handled=%v err=%v result=%+v", handled, err, bounties)
	}
}

func TestFestivalSaleSellsFormationStonesInBulk(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "festival-stone-player", "叩阵散人")
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("silver_coins", 500).Error; err != nil {
		t.Fatal(err)
	}
	var entry model.ShopEntry
	if err := store.DB.Where("code = ?", "event_sale_formation_stone").First(&entry).Error; err != nil {
		t.Fatal(err)
	}
	if entry.ItemName != "阵基石" || entry.Currency != "银币" || entry.Price != 48 || !entry.Enabled {
		t.Fatalf("unexpected formation stone sale entry: %+v", entry)
	}
	page, handled, err := game.Execute("group", player.AccountID, mustParse(t, "庆典特卖 2"))
	if err != nil || !handled || !strings.Contains(page.Content, "阵基石 · 活动价48银币") || !containsAction(page.Actions, "庆典购买 阵基石") {
		t.Fatalf("formation stone missing from sale page: handled=%v err=%v result=%+v", handled, err, page)
	}
	beforeQuantity := game.itemQuantity(player.ID, entry.ItemID)
	bought, handled, err := game.Execute("group", player.AccountID, mustParse(t, "庆典购买 阵基石 3"))
	if err != nil || !handled || !strings.Contains(bought.Title, "购买成功") || !strings.Contains(bought.Content, "阵基石×3") || !strings.Contains(bought.Content, "银币144") {
		t.Fatalf("bulk formation stone purchase: handled=%v err=%v result=%+v", handled, err, bought)
	}
	after, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.SilverCoins != 356 || game.itemQuantity(player.ID, entry.ItemID) != beforeQuantity+3 {
		t.Fatalf("bulk purchase settlement: silver=%d quantity=%d before=%d", after.SilverCoins, game.itemQuantity(player.ID, entry.ItemID), beforeQuantity)
	}
}

func TestHealingAndPetPowerStaySynchronized(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "vitals-pet-player", "照水真人")
	lowHealth := int64(20)
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("health", lowHealth).Error; err != nil {
		t.Fatal(err)
	}
	player, _ = game.players.Get(player.ID)
	battle := mapMonsterBattleState{LocationID: 1, BattleKind: "地图", Round: 1, EnemyName: "试剑木傀", EnemyPower: 1, PlayerHP: lowHealth, PlayerMana: player.Mana, EnemyHP: 10000, EnemyMaxHP: 10000, StartedAt: time.Now().UnixMilli()}
	if err := game.beginPVEBattle(player.ID, battle); err != nil {
		t.Fatal(err)
	}
	dew, err := game.itemByName("仙露")
	if err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, dew.ID, 1); err != nil {
		t.Fatal(err)
	}
	healed, handled, err := game.Execute("group", player.AccountID, mustParse(t, "使用 仙露"))
	if err != nil || !handled || !strings.Contains(healed.Title, "仙露") {
		t.Fatalf("healing item: handled=%v err=%v result=%+v", handled, err, healed)
	}
	updated, _ := game.players.Get(player.ID)
	rawBattle, err := game.playerValue(player.ID, "pve.battle")
	if err != nil {
		t.Fatal(err)
	}
	var synchronized mapMonsterBattleState
	if err := json.Unmarshal([]byte(rawBattle), &synchronized); err != nil {
		t.Fatal(err)
	}
	if updated.Health <= lowHealth || synchronized.PlayerHP != updated.Health {
		t.Fatalf("battle healing desynchronized: player=%d battle=%d", updated.Health, synchronized.PlayerHP)
	}
	turn, handled, err := game.Execute("group", player.AccountID, mustParse(t, "攻击"))
	if err != nil || !handled || !strings.Contains(turn.Content, "当前状况") {
		t.Fatalf("post-heal battle turn: handled=%v err=%v result=%+v", handled, err, turn)
	}
	rawBattle, _ = game.playerValue(player.ID, "pve.battle")
	_ = json.Unmarshal([]byte(rawBattle), &synchronized)
	if synchronized.PlayerHP <= lowHealth {
		t.Fatalf("next battle turn reverted healing: %+v", synchronized)
	}
	_ = game.clearMapMonsterBattle(player.ID)

	pet := model.Pet{PlayerID: player.ID, Name: "流风仙鹤", Species: "流风仙鹤", Rarity: "凡品", Level: 1, Attack: 2402, Defense: 1201, Health: 24020, Loyalty: 80}
	if err := store.DB.Create(&pet).Error; err != nil {
		t.Fatal(err)
	}
	activated, handled, err := game.Execute("group", player.AccountID, mustParse(t, "出战 流风仙鹤"))
	if err != nil || !handled || !strings.Contains(activated.Content, "灵兽战力：2402") {
		t.Fatalf("pet activation power: handled=%v err=%v result=%+v", handled, err, activated)
	}
	petList, handled, err := game.Execute("group", player.AccountID, mustParse(t, "灵兽"))
	if err != nil || !handled || !strings.Contains(petList.Content, "灵兽战力2402") {
		t.Fatalf("pet list power: handled=%v err=%v result=%+v", handled, err, petList)
	}
	withPet, _ := game.players.Get(player.ID)
	if withPet.CombatPower != calculateCombatPower(withPet)+2402 {
		t.Fatalf("active pet not included in character power: got=%d base=%d", withPet.CombatPower, calculateCombatPower(withPet))
	}
	rows, err := game.loadLeaderboard("灵兽")
	if err != nil || len(rows) == 0 || rows[0].Score != 2402 {
		t.Fatalf("pet leaderboard power mismatch: rows=%+v err=%v", rows, err)
	}
}

func TestBulkItemUseAndAFKRunQueue(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "bulk-player", "听雨散人")
	var fruit model.Item
	if err := store.DB.Where("name = ?", "灵果").First(&fruit).Error; err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, fruit.ID, 5); err != nil {
		t.Fatal(err)
	}
	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "使用 灵果*3"))
	if err != nil || !handled || !strings.Contains(result.Content, "数量：3") {
		t.Fatalf("bulk use: handled=%v err=%v content=%s", handled, err, result.Content)
	}
	updated, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Cultivation-player.Cultivation != 30 || game.itemQuantity(player.ID, fruit.ID) != 2 {
		t.Fatalf("bulk use mismatch: cultivation=%d quantity=%d", updated.Cultivation-player.Cultivation, game.itemQuantity(player.ID, fruit.ID))
	}
	afkResult, handled, err := game.Execute("group", player.AccountID, mustParse(t, "挂机 猎妖*99"))
	if err != nil || !handled || !strings.Contains(afkResult.Content, "计划99轮") {
		t.Fatalf("afk queue: handled=%v err=%v content=%s", handled, err, afkResult.Content)
	}
	value, err := game.playerValue(player.ID, "afk.job")
	if err != nil {
		t.Fatal(err)
	}
	var job afkJob
	if err := json.Unmarshal([]byte(value), &job); err != nil || job.RequestedRuns != 99 {
		t.Fatalf("stored afk job=%+v err=%v", job, err)
	}
}

func TestAFKClaimAliasesSettleRewardsAndStop(t *testing.T) {
	game, _ := testGame(t)
	player := registerPlayer(t, game, "afk-claim-player", "松间客")
	for _, command := range []string{"领取挂机", "收取挂机", "收获挂机", "挂机领取", "挂机收取", "挂机收获", "结束挂机", "停止挂机", "挂机结束", "挂机停止"} {
		parsed, ok := handler.ParseCommand(command)
		if !ok || parsed.Spec.ID != 247 {
			t.Fatalf("AFK alias %q parsed as %+v, ok=%v", command, parsed, ok)
		}
	}

	if result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "挂机 猎妖*3")); err != nil || !handled || !strings.Contains(result.Content, "计划3轮") {
		t.Fatalf("start AFK: handled=%v err=%v result=%+v", handled, err, result)
	}
	value, err := game.playerValue(player.ID, "afk.job")
	if err != nil {
		t.Fatal(err)
	}
	var job afkJob
	if err := json.Unmarshal([]byte(value), &job); err != nil {
		t.Fatal(err)
	}
	job.StartedAt = time.Now().Add(-25 * time.Minute)
	data, _ := json.Marshal(job)
	if err := game.setPlayerValue(player.ID, "afk.job", string(data), nil); err != nil {
		t.Fatal(err)
	}
	before, _ := game.players.Get(player.ID)
	beforeStamina, err := game.currentStamina(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	claimed, handled, err := game.Execute("group", player.AccountID, mustParse(t, "领取挂机"))
	if err != nil || !handled || !strings.Contains(claimed.Content, "本次领取：2轮") {
		t.Fatalf("claim AFK: handled=%v err=%v result=%+v", handled, err, claimed)
	}
	after, _ := game.players.Get(player.ID)
	afterStamina, _ := game.currentStamina(player.ID)
	if after.Cultivation-before.Cultivation != 16 || after.Merit-before.Merit != 4 || after.SpiritStones != before.SpiritStones || beforeStamina-afterStamina != 8 {
		t.Fatalf("AFK reward mismatch: cultivation=%d merit=%d stones=%d stamina=%d", after.Cultivation-before.Cultivation, after.Merit-before.Merit, after.SpiritStones-before.SpiritStones, beforeStamina-afterStamina)
	}
	value, err = game.playerValue(player.ID, "afk.job")
	if err != nil || json.Unmarshal([]byte(value), &job) != nil || job.CompletedRuns != 2 {
		t.Fatalf("AFK progress after claim: job=%+v err=%v", job, err)
	}

	beforeRepeat := after.Cultivation
	repeated, handled, err := game.Execute("group", player.AccountID, mustParse(t, "挂机收获"))
	if err != nil || !handled || !strings.Contains(repeated.Title, "尚未可领") {
		t.Fatalf("repeat AFK claim: handled=%v err=%v result=%+v", handled, err, repeated)
	}
	afterRepeat, _ := game.players.Get(player.ID)
	if afterRepeat.Cultivation != beforeRepeat || game.playerValueInt(player.ID, "stats.afk_runs", 0) != 2 {
		t.Fatalf("repeat claim granted rewards: cultivation=%d/%d runs=%d", afterRepeat.Cultivation, beforeRepeat, game.playerValueInt(player.ID, "stats.afk_runs", 0))
	}

	job.StartedAt = time.Now().Add(-15 * time.Minute)
	data, _ = json.Marshal(job)
	if err := game.setPlayerValue(player.ID, "afk.job", string(data), nil); err != nil {
		t.Fatal(err)
	}
	ended, handled, err := game.Execute("group", player.AccountID, mustParse(t, "结束挂机"))
	if err != nil || !handled || !strings.Contains(ended.Title, "挂机结束") || !strings.Contains(ended.Content, "本次领取：1轮") {
		t.Fatalf("end AFK: handled=%v err=%v result=%+v", handled, err, ended)
	}
	if _, err := game.playerValue(player.ID, "afk.job"); err == nil {
		t.Fatal("ended AFK job still exists")
	}
	if game.playerValueInt(player.ID, "stats.afk_runs", 0) != 3 {
		t.Fatalf("AFK run stats=%d, want 3", game.playerValueInt(player.ID, "stats.afk_runs", 0))
	}
}

func TestAFKStopPreservesUnclaimedRunsWhenStaminaIsInsufficient(t *testing.T) {
	game, _ := testGame(t)
	player := registerPlayer(t, game, "afk-low-stamina", "听潮子")
	if _, handled, err := game.Execute("group", player.AccountID, mustParse(t, "挂机 猎妖")); err != nil || !handled {
		t.Fatalf("start AFK: handled=%v err=%v", handled, err)
	}
	job := afkJob{Type: "monster", Target: "猎妖", StartedAt: time.Now().Add(-25 * time.Minute), Interval: 10}
	data, _ := json.Marshal(job)
	if err := game.setPlayerValue(player.ID, "afk.job", string(data), nil); err != nil {
		t.Fatal(err)
	}
	if err := game.setPlayerValue(player.ID, "stamina.date", time.Now().Format("2006-01-02"), nil); err != nil {
		t.Fatal(err)
	}
	if err := game.setPlayerValueInt(player.ID, "stamina.value", 4); err != nil {
		t.Fatal(err)
	}
	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "结束挂机"))
	if err != nil || !handled || !strings.Contains(result.Content, "任务已保留") {
		t.Fatalf("partial stop: handled=%v err=%v result=%+v", handled, err, result)
	}
	value, err := game.playerValue(player.ID, "afk.job")
	if err != nil || json.Unmarshal([]byte(value), &job) != nil || job.CompletedRuns != 1 {
		t.Fatalf("partial stop lost AFK job: job=%+v err=%v", job, err)
	}
	if game.playerValueInt(player.ID, "stats.afk_runs", 0) != 1 {
		t.Fatalf("partial stop run stats=%d", game.playerValueInt(player.ID, "stats.afk_runs", 0))
	}
}

func TestAFKDungeonClaimPaysStones(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "afk-dungeon-player", "云渡道人")
	var dungeon model.Dungeon
	if err := store.DB.Where("enabled = ?", true).Order("id").First(&dungeon).Error; err != nil {
		t.Fatal(err)
	}
	manual := model.DungeonRun{PlayerID: player.ID, DungeonID: dungeon.ID, RunDate: time.Now().AddDate(0, 0, -1).Format("2006-01-02"), DurationMS: 1000, Score: 1, Success: true}
	if err := store.DB.Create(&manual).Error; err != nil {
		t.Fatal(err)
	}
	var sweepTicket model.Item
	if err := store.DB.Where("name = ?", "扫荡券").First(&sweepTicket).Error; err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, sweepTicket.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, handled, err := game.Execute("group", player.AccountID, mustParse(t, "挂机 "+dungeon.Name+"*1")); err != nil || !handled {
		t.Fatalf("start dungeon AFK: handled=%v err=%v", handled, err)
	}
	value, _ := game.playerValue(player.ID, "afk.job")
	var job afkJob
	_ = json.Unmarshal([]byte(value), &job)
	job.StartedAt = time.Now().Add(-11 * time.Minute)
	data, _ := json.Marshal(job)
	if err := game.setPlayerValue(player.ID, "afk.job", string(data), nil); err != nil {
		t.Fatal(err)
	}
	before, _ := game.players.Get(player.ID)
	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "领取挂机"))
	if err != nil || !handled || !strings.Contains(result.Content, "本次领取：1轮") || !strings.Contains(result.Content, "消耗扫荡券：1") {
		t.Fatalf("dungeon AFK claim: handled=%v err=%v result=%+v", handled, err, result)
	}
	after, _ := game.players.Get(player.ID)
	if after.Cultivation <= before.Cultivation || after.SpiritStones <= before.SpiritStones || after.Merit <= before.Merit {
		t.Fatalf("dungeon AFK rewards not credited: before=%+v after=%+v", before, after)
	}
	if _, err := game.playerValue(player.ID, "afk.job"); err == nil {
		t.Fatal("completed dungeon AFK job still exists")
	}
}

func TestSpiritualRootCatalogModerationAndShortcut(t *testing.T) {
	game, store := testGame(t)
	var rootCount int64
	if err := store.DB.Model(&model.SpiritualRootTemplate{}).Count(&rootCount).Error; err != nil || rootCount != 3 {
		t.Fatalf("test spiritual root seed count=%d err=%v", rootCount, err)
	}
	blocked, handled, err := game.Execute("group", "blocked-root-user", mustParse(t, "入道 加微信找我"))
	if err != nil || !handled || !strings.Contains(blocked.Title, "审核未通过") {
		t.Fatalf("sensitive dao name not blocked: handled=%v err=%v result=%+v", handled, err, blocked)
	}
	player := registerPlayer(t, game, "root-shortcut-user", "观澜真人")
	var root model.SpiritualRootTemplate
	if err := store.DB.Where("name = ?", player.SpiritualRoot).First(&root).Error; err != nil {
		t.Fatalf("registered root absent from catalog: %v", err)
	}
	if root.CultivationBonus <= 1 || root.AttributeJSON == "" || root.PrimaryBonus == "" {
		t.Fatalf("root bonuses incomplete: %+v", root)
	}
	setResult, handled, err := game.Execute("group", player.AccountID, mustParse(t, "设置快捷 回家=位置"))
	if err != nil || !handled || !strings.Contains(setResult.Title, "已设置") {
		t.Fatalf("set shortcut: handled=%v err=%v result=%+v", handled, err, setResult)
	}
	parsed, ok := game.ResolveShortcut(player.AccountID, "回家")
	if !ok || parsed.Spec.ID != 252 {
		t.Fatalf("shortcut did not resolve: ok=%v parsed=%+v", ok, parsed)
	}
}

func TestDaoNameModerationRejectsProfanityAndUnreadableGlyphStacks(t *testing.T) {
	game, store := testGame(t)
	for index, testCase := range []struct {
		name        string
		titlePart   string
		contentPart string
	}{
		{name: "我草泥马", titlePart: "审核未通过"},
		{name: "夨坕汚幵", titlePart: "格式审核未通过", contentPart: "生僻字"},
		{name: "9999", titlePart: "格式审核未通过", contentPart: "纯数字"},
	} {
		result, handled, err := game.Execute("group", fmt.Sprintf("blocked-dao-name-%d", index), mustParse(t, "入道 "+testCase.name))
		if err != nil || !handled || !strings.Contains(result.Title, testCase.titlePart) || (testCase.contentPart != "" && !strings.Contains(result.Content, testCase.contentPart)) {
			t.Fatalf("dao name %q was not blocked: handled=%v err=%v result=%+v", testCase.name, handled, err, result)
		}
	}
	accepted := registerPlayer(t, game, "single-character-dao-name", "凛")
	if accepted.DaoName != "凛" {
		t.Fatalf("readable single-character dao name rejected: %+v", accepted)
	}
	legacy := registerPlayer(t, game, "legacy-unreadable-dao-name", "清风")
	if err := store.DB.Model(&model.Player{}).Where("id = ?", legacy.ID).Update("dao_name", "夨坕汚幵").Error; err != nil {
		t.Fatal(err)
	}
	review, handled, err := game.Execute("group", legacy.AccountID, mustParse(t, "状态"))
	if err != nil || !handled || !strings.Contains(review.Title, "复核未通过") || !containsAction(review.Actions, "改名 新道号") {
		t.Fatalf("legacy unreadable name was not restricted: handled=%v err=%v result=%+v", handled, err, review)
	}
	renamed, handled, err := game.Execute("group", legacy.AccountID, mustParse(t, "改名 清川"))
	if err != nil || !handled || !strings.Contains(renamed.Content, "本次免费") {
		t.Fatalf("legacy forced rename was not free: handled=%v err=%v result=%+v", handled, err, renamed)
	}
}

func TestAlchemyProducesNamedPillsAndAppliesRealEffects(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "alchemy-effect-player", "丹霞真人")
	var spiritRecipe model.AlchemyRecipe
	if err := store.DB.Where("code = ?", "recipe_spirit").First(&spiritRecipe).Error; err != nil {
		t.Fatal(err)
	}
	if spiritRecipe.OutputName != "聚灵丹" || !strings.Contains(spiritRecipe.MaterialsJSON, "赤焰草") {
		t.Fatalf("spirit recipe was not migrated: %+v", spiritRecipe)
	}
	var recoveryRecipe model.AlchemyRecipe
	if err := store.DB.Where("code = ?", "recipe_recovery").First(&recoveryRecipe).Error; err != nil {
		t.Fatal(err)
	}
	if recoveryRecipe.OutputName != "回元散" || !strings.Contains(recoveryRecipe.MaterialsJSON, "凝露草") {
		t.Fatalf("recovery recipe was not migrated: %+v", recoveryRecipe)
	}
	var manaRecipe model.AlchemyRecipe
	if err := store.DB.Where("code = ?", "recipe_mana").First(&manaRecipe).Error; err != nil {
		t.Fatal(err)
	}
	if manaRecipe.OutputName != "回灵丹" || !strings.Contains(manaRecipe.MaterialsJSON, "灵茶") {
		t.Fatalf("mana recipe was not seeded: %+v", manaRecipe)
	}
	if err := store.DB.Model(&spiritRecipe).Update("success_rate", 1).Error; err != nil {
		t.Fatal(err)
	}
	for name, quantity := range map[string]int64{"灵果": 2, "灵茶": 1, "赤焰草": 1} {
		item, err := game.itemByName(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := game.players.AdjustItem(player.ID, item.ID, quantity); err != nil {
			t.Fatal(err)
		}
	}
	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "炼药 聚灵丹"))
	if err != nil || !handled || !strings.Contains(result.Content, "获得：聚灵丹×1") || !strings.Contains(result.Content, "修为+120") || strings.Contains(result.Content, "获得：灵果") {
		t.Fatalf("alchemy result is still wrong: handled=%v err=%v result=%+v", handled, err, result)
	}
	pill, err := game.itemByName("聚灵丹")
	if err != nil || game.itemQuantity(player.ID, pill.ID) != 1 {
		t.Fatalf("crafted pill missing: item=%+v err=%v quantity=%d", pill, err, game.itemQuantity(player.ID, pill.ID))
	}
	before, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	used, handled, err := game.Execute("group", player.AccountID, mustParse(t, "使用 聚灵丹"))
	if err != nil || !handled || !strings.Contains(used.Content, "修为+120") {
		t.Fatalf("pill use did not report effect: handled=%v err=%v result=%+v", handled, err, used)
	}
	after, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Cultivation-before.Cultivation != 120 || game.itemQuantity(player.ID, pill.ID) != 0 {
		t.Fatalf("pill effect mismatch: cultivation delta=%d quantity=%d", after.Cultivation-before.Cultivation, game.itemQuantity(player.ID, pill.ID))
	}
	if err := store.DB.Model(&manaRecipe).Update("success_rate", 1).Error; err != nil {
		t.Fatal(err)
	}
	for name, quantity := range map[string]int64{"凝露草": 1, "灵果": 1, "灵茶": 2} {
		item, itemErr := game.itemByName(name)
		if itemErr != nil {
			t.Fatal(itemErr)
		}
		if err := game.players.AdjustItem(player.ID, item.ID, quantity); err != nil {
			t.Fatal(err)
		}
	}
	craftedMana, handled, err := game.Execute("group", player.AccountID, mustParse(t, "炼药 回灵丹"))
	if err != nil || !handled || !strings.Contains(craftedMana.Content, "获得：回灵丹×1") || !strings.Contains(craftedMana.Content, "最大法力40%") {
		t.Fatalf("mana pill craft failed: handled=%v err=%v result=%+v", handled, err, craftedMana)
	}
	manaPill, err := game.itemByName("回灵丹")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("mana", 5).Error; err != nil {
		t.Fatal(err)
	}
	beforeMana, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	battleState := mapMonsterBattleState{BattleKind: "副本", EnemyName: "试炼守境灵", EnemyHP: 100, EnemyMaxHP: 100, PlayerHP: beforeMana.Health, PlayerMana: beforeMana.Mana, Round: 1}
	encodedBattle, err := json.Marshal(battleState)
	if err != nil {
		t.Fatal(err)
	}
	if err := game.setPlayerValue(player.ID, "pve.battle", string(encodedBattle), nil); err != nil {
		t.Fatal(err)
	}
	restored, handled, err := game.Execute("group", player.AccountID, mustParse(t, "使用 回灵丹"))
	if err != nil || !handled || !strings.Contains(restored.Content, "本次法力：") || !strings.Contains(restored.Content, "实际+") {
		t.Fatalf("mana pill did not report recovery: handled=%v err=%v result=%+v", handled, err, restored)
	}
	afterMana, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	expectedMana := min64(beforeMana.MaxMana, beforeMana.Mana+itemManaRecoveryAmount(manaPill, beforeMana.MaxMana))
	if afterMana.Mana != expectedMana || game.itemQuantity(player.ID, manaPill.ID) != 0 {
		t.Fatalf("mana recovery mismatch: got=%d want=%d quantity=%d", afterMana.Mana, expectedMana, game.itemQuantity(player.ID, manaPill.ID))
	}
	battleValue, err := game.playerValue(player.ID, "pve.battle")
	if err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal([]byte(battleValue), &battleState) != nil || battleState.PlayerMana != expectedMana {
		t.Fatalf("battle mana was not synchronized: state=%+v want=%d", battleState, expectedMana)
	}
	for name, quantity := range map[string]int64{"凝露草": 99, "灵果": 99, "灵茶": 198} {
		item, itemErr := game.itemByName(name)
		if itemErr != nil {
			t.Fatal(itemErr)
		}
		if err := game.players.AdjustItem(player.ID, item.ID, quantity); err != nil {
			t.Fatal(err)
		}
	}
	batch, handled, err := game.Execute("group", player.AccountID, mustParse(t, "炼药 回灵丹*99"))
	if err != nil || !handled || !strings.Contains(batch.Content, "开炉：99次") || !strings.Contains(batch.Content, "成丹：99") || strings.Contains(batch.Title, "不存在") {
		t.Fatalf("batch mana pill craft failed: handled=%v err=%v result=%+v", handled, err, batch)
	}
	if quantity := game.itemQuantity(player.ID, manaPill.ID); quantity != 99 {
		t.Fatalf("batch mana pill quantity=%d want=99", quantity)
	}
	for _, name := range []string{"赤焰草", "阵基石", "雷灵晶"} {
		detail, handled, err := game.Execute("group", player.AccountID, mustParse(t, "物品 "+name))
		if err != nil || !handled || !strings.Contains(detail.Content, "获取来源：") || !strings.Contains(detail.Content, "具体用途：") || strings.Contains(detail.Content, "当前没有配置为其他配方材料") {
			t.Fatalf("material %s lacks guidance: handled=%v err=%v result=%+v", name, handled, err, detail)
		}
	}
}

func TestResurrectionPenaltyAndDungeonAccessArePlayerFriendly(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "revive-dungeon-player", "归元客")
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{
		"cultivation": 3720, "cultivation_required": 80, "health": 1, "max_health": 157, "mana": 0, "max_mana": 76,
	}).Error; err != nil {
		t.Fatal(err)
	}
	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "回城复活"))
	if err != nil || !handled || !strings.Contains(result.Content, "修为惩罚：-20") || !strings.Contains(result.Content, "当前修为1%") || !containsAction(result.Actions, "使用 回灵丹") {
		t.Fatalf("resurrection penalty is not capped: handled=%v err=%v result=%+v", handled, err, result)
	}
	updated, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Cultivation != 3700 {
		t.Fatalf("resurrection cultivation=%d want 3700", updated.Cultivation)
	}
	profiles := map[string]struct{ stamina, limit int }{
		"普通": {3, 20}, "困难": {6, 12}, "噩梦": {9, 8}, "地狱": {12, 5},
	}
	for difficulty, profile := range profiles {
		var mismatches int64
		if err := store.DB.Model(&model.Dungeon{}).Where("difficulty = ? AND (stamina_cost <> ? OR daily_limit <> ?)", difficulty, profile.stamina, profile.limit).Count(&mismatches).Error; err != nil || mismatches != 0 {
			t.Fatalf("dungeon profile %s mismatches=%d err=%v", difficulty, mismatches, err)
		}
	}
	list, handled, err := game.Execute("group", player.AccountID, mustParse(t, "副本"))
	if err != nil || !handled || !strings.Contains(list.Content, "普通20次") || !strings.Contains(list.Content, "普通副本每次只消耗3点") {
		t.Fatalf("dungeon list lacks access rules: handled=%v err=%v result=%+v", handled, err, list)
	}
}

func TestTemporaryMedicineBonusesFeedGameplayCalculations(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "medicine-bonus-player", "药王散人")
	items := []model.Item{
		{Code: "test_breakthrough_medicine", Name: "试炼破障丸", CategoryName: "丹药", EffectType: "突破", EffectFunc: "breakthrough_bonus", EffectParams: `{"rate":0.12,"minutes":30}`, EffectValue: 120, Stackable: true},
		{Code: "test_tribulation_medicine", Name: "试炼渡厄丹", CategoryName: "丹药", EffectType: "渡劫", EffectFunc: "tribulation_bonus", EffectParams: `{"rate":0.15,"minutes":30}`, EffectValue: 150, Stackable: true},
		{Code: "test_agility_medicine", Name: "试炼神行散", CategoryName: "丹药", EffectType: "身法", EffectFunc: "temporary_buff", EffectParams: `{"duration_minutes":30}`, EffectValue: 200, Stackable: true},
		{Code: "test_defense_medicine", Name: "试炼护脉丹", CategoryName: "丹药", EffectType: "防御", EffectFunc: "temporary_buff", EffectParams: `{"duration_minutes":30}`, EffectValue: 250, Stackable: true},
		{Code: "test_root_medicine", Name: "试炼洗髓液", CategoryName: "丹药", EffectType: "灵根", EffectFunc: "root_refine", EffectValue: 500, Stackable: true},
	}
	baseStats := game.playerCombatStats(&player)
	for index := range items {
		if err := store.DB.Create(&items[index]).Error; err != nil {
			t.Fatal(err)
		}
		if err := game.players.AdjustItem(player.ID, items[index].ID, 1); err != nil {
			t.Fatal(err)
		}
		if result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "使用 "+items[index].Name)); err != nil || !handled || strings.Contains(result.Title, "失败") {
			t.Fatalf("using %s: handled=%v err=%v result=%+v", items[index].Name, handled, err, result)
		}
	}
	bonuses := game.activeItemBonuses(player.ID)
	if bonuses.BreakthroughRate < .12 || bonuses.TribulationRate < .15 || bonuses.AgilityRate <= 0 || bonuses.DefenseRate <= 0 {
		t.Fatalf("temporary bonuses not loaded: %+v", bonuses)
	}
	updated, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.RootQuality <= player.RootQuality {
		t.Fatalf("root refine did not change purity: %d -> %d", player.RootQuality, updated.RootQuality)
	}
	buffedStats := game.playerCombatStats(&updated)
	if buffedStats.Agility <= baseStats.Agility || buffedStats.PhysicalDefense <= baseStats.PhysicalDefense || buffedStats.MagicDefense <= baseStats.MagicDefense {
		t.Fatalf("combat medicine did not change stats: base=%+v buffed=%+v", baseStats, buffedStats)
	}
	doubleCard, err := game.itemByName("双倍修为卡")
	if err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, doubleCard.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, _, err := game.Execute("group", player.AccountID, mustParse(t, "使用 双倍修为卡")); err != nil {
		t.Fatal(err)
	}
	if multiplier := game.activeItemBonuses(player.ID).CultivationMultiplier; multiplier < 2 {
		t.Fatalf("cultivation medicine multiplier=%v", multiplier)
	}
	overview, handled, err := game.Execute("group", player.AccountID, mustParse(t, "当前药效"))
	if err != nil || !handled || !strings.Contains(overview.Content, "试炼破障丸") || !strings.Contains(overview.Content, "试炼渡厄丹") || !strings.Contains(overview.Content, "实战双防") || !strings.Contains(overview.Content, "突破+12.0%") || !strings.Contains(overview.Content, "渡劫+15.0%") {
		t.Fatalf("active medicine overview: handled=%v err=%v result=%+v", handled, err, overview)
	}
	if err := store.DB.Model(&model.SystemSetting{}).Where("key = ?", "display.status_image_mode").Update("value", "false").Error; err != nil {
		t.Fatal(err)
	}
	status, handled, err := game.Execute("group", player.AccountID, mustParse(t, "状态"))
	if err != nil || !handled || !strings.Contains(status.Content, "当前药力：") || !strings.Contains(status.Content, "双防+25.0%") || !containsAction(status.Actions, "当前药效") {
		t.Fatalf("medicine not visible in status: handled=%v err=%v result=%+v", handled, err, status)
	}
}

func TestUnifiedRechargeDaoNameTransferAndDeletionRelease(t *testing.T) {
	game, store := testGame(t)
	donor := registerPlayer(t, game, "dao-donor", "照夜剑主")
	receiver := registerPlayer(t, game, "dao-receiver", "听雪散人")
	owner := model.SystemSetting{Key: "owner.user_id", Value: "owner-account", ValueType: "string"}
	if err := store.DB.Where("key = ?", owner.Key).Assign(map[string]any{"value": owner.Value, "value_type": owner.ValueType}).FirstOrCreate(&owner).Error; err != nil {
		t.Fatal(err)
	}
	for _, commandText := range []string{"充值 照夜剑主 银币 800", "充值 照夜剑主 仙金 60"} {
		command, ok := ParseGMCommand(commandText)
		if !ok {
			t.Fatalf("GM command did not parse: %s", commandText)
		}
		if result, handled, err := game.ExecuteGM("owner-account", command); err != nil || !handled || !strings.Contains(result.Title, "成功") {
			t.Fatalf("GM recharge %s: handled=%v err=%v result=%+v", commandText, handled, err, result)
		}
	}
	donor, _ = game.players.Get(donor.ID)
	if donor.SilverCoins != 800 || donor.ImmortalJade != 60 {
		t.Fatalf("recharge mismatch: silver=%d jade=%d", donor.SilverCoins, donor.ImmortalJade)
	}
	request, handled, err := game.Execute("group", donor.AccountID, mustParse(t, "转让道号 听雪散人 云隐道人"))
	if err != nil || !handled || !strings.Contains(request.Title, "已送达") {
		t.Fatalf("dao transfer request: handled=%v err=%v result=%+v", handled, err, request)
	}
	accepted, handled, err := game.Execute("group", receiver.AccountID, mustParse(t, "接受道号"))
	if err != nil || !handled || !strings.Contains(accepted.Title, "完成") {
		t.Fatalf("dao transfer accept: handled=%v err=%v result=%+v", handled, err, accepted)
	}
	donor, _ = game.players.Get(donor.ID)
	receiver, _ = game.players.Get(receiver.ID)
	if donor.DaoName != "云隐道人" || receiver.DaoName != "照夜剑主" || donor.SilverCoins != 300 {
		t.Fatalf("dao transfer result donor=%+v receiver=%+v", donor, receiver)
	}
	if err := game.players.Delete(receiver.ID); err != nil {
		t.Fatal(err)
	}
	reused := registerPlayer(t, game, "released-name-user", "照夜剑主")
	if reused.DaoName != "照夜剑主" {
		t.Fatalf("deleted dao name not released: %+v", reused)
	}
}

func TestWorldLeylineDiscoveryPrerequisitesAndMeditation(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "leyline-user", "地脉行者")
	var leyline model.WorldLeyline
	if err := store.DB.Where("location_name = ?", player.Location).Order("id").First(&leyline).Error; err != nil {
		t.Fatal(err)
	}
	var root model.SpiritualRootTemplate
	if err := store.DB.Where("name = ?", player.SpiritualRoot).First(&root).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&leyline).Updates(map[string]any{
		"required_root_element": root.Element, "minimum_realm_sequence": 1, "minimum_realm_level": 1,
		"minimum_combat_power": 0, "minimum_spirit": 0, "discovery_mana_cost": 1,
		"required_item": "灵果", "required_item_count": 1, "cultivation_multiplier": 4.25, "aura_per_minute": 300,
	}).Error; err != nil {
		t.Fatal(err)
	}
	var fruit model.Item
	if err := store.DB.Where("name = ?", "灵果").First(&fruit).Error; err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, fruit.ID, 1); err != nil {
		t.Fatal(err)
	}
	if result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "寻脉")); err != nil || !handled || !strings.Contains(result.Title, "有得") {
		t.Fatalf("discover leyline: handled=%v err=%v result=%+v", handled, err, result)
	}
	if result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "灵脉打坐 "+leyline.Name)); err != nil || !handled || !strings.Contains(result.Title, "入定") {
		t.Fatalf("start leyline meditation: handled=%v err=%v result=%+v", handled, err, result)
	}
	value, err := game.playerValue(player.ID, "leyline.meditation")
	if err != nil {
		t.Fatal(err)
	}
	var job leylineMeditationJob
	if err := json.Unmarshal([]byte(value), &job); err != nil {
		t.Fatal(err)
	}
	job.StartedAt = time.Now().Add(-10 * time.Minute)
	encoded, _ := json.Marshal(job)
	if err := game.setPlayerValue(player.ID, "leyline.meditation", string(encoded), nil); err != nil {
		t.Fatal(err)
	}
	before, _ := game.players.Get(player.ID)
	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "灵脉出定"))
	if err != nil || !handled || !strings.Contains(result.Content, "最终修为") {
		t.Fatalf("finish leyline meditation: handled=%v err=%v result=%+v", handled, err, result)
	}
	after, _ := game.players.Get(player.ID)
	if after.Cultivation <= before.Cultivation || after.State != model.PlayerStateIdle {
		t.Fatalf("leyline reward/state mismatch before=%+v after=%+v", before, after)
	}
}

func TestPrerequisitesEnforceSystemRulesAndDoNotRegressHigherRealms(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "prerequisite-user", "守门道人")

	var coupleSkill model.CoupleCombinationSkillConfig
	if err := store.DB.Order("sort_order").First(&coupleSkill).Error; err != nil {
		t.Fatal(err)
	}
	blocked, handled, err := game.Execute("group", player.AccountID, mustParse(t, "合学 "+coupleSkill.Name))
	if err != nil || !handled || !strings.Contains(blocked.Content, "尚未结为仙侣") {
		t.Fatalf("couple prerequisite was not enforced: handled=%v err=%v result=%+v", handled, err, blocked)
	}

	var secondRealm model.Realm
	if err := store.DB.Where("sequence = ?", 2).First(&secondRealm).Error; err != nil {
		t.Fatal(err)
	}
	player.RealmID, player.RealmName, player.RealmLevel = secondRealm.ID, secondRealm.Name, 1
	if _, unmet, err := game.prerequisiteStatus(&player, `{"minimum_realm_sequence":1,"minimum_realm_level":10}`); err != nil || len(unmet) != 0 {
		t.Fatalf("higher realm was incorrectly blocked by a lower realm layer: unmet=%v err=%v", unmet, err)
	}
}

func testGame(t *testing.T) (*Game, *storage.Store) {
	t.Helper()
	cfg := config.Runtime(t.TempDir())
	cfg.Database.DSN = filepath.Join(t.TempDir(), "game.db")
	store, err := storage.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	game, err := NewGame(store)
	if err != nil {
		t.Fatal(err)
	}
	return game, store
}

func mustParse(t *testing.T, message string) handler.ParsedCommand {
	t.Helper()
	parsed, ok := handler.ParseCommand(message)
	if !ok {
		t.Fatalf("command %q did not parse", message)
	}
	return parsed
}

func registerPlayer(t *testing.T, game *Game, accountID, name string) model.Player {
	t.Helper()
	_, handled, err := game.Execute("group", accountID, mustParse(t, "入道 "+name))
	if err != nil || !handled {
		t.Fatalf("register: handled=%v err=%v", handled, err)
	}
	player, err := game.players.GetByAccount(accountID)
	if err != nil {
		t.Fatal(err)
	}
	return player
}

func TestUnregisteredPlayerStaysSilent(t *testing.T) {
	game, _ := testGame(t)
	_, handled, err := game.Execute("group", "new-user", mustParse(t, "状态"))
	if err != nil {
		t.Fatal(err)
	}
	if handled {
		t.Fatal("unregistered 状态 should stay silent")
	}
}

func TestCultivationRequiresTimeBeforeSettlement(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "cultivator", "青玄")

	_, handled, err := game.Execute("group", player.AccountID, mustParse(t, "修炼"))
	if err != nil || !handled {
		t.Fatalf("start cultivation: handled=%v err=%v", handled, err)
	}
	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "出关"))
	if err != nil || !handled {
		t.Fatalf("early finish: handled=%v err=%v", handled, err)
	}
	if !strings.Contains(result.Title, "道基未稳") {
		t.Fatalf("early finish title = %q", result.Title)
	}
	current, _ := game.players.GetByAccount(player.AccountID)
	if current.Cultivation != 0 || current.State != model.PlayerStateCultivating {
		t.Fatalf("early finish changed player: cultivation=%d state=%s", current.Cultivation, current.State)
	}

	started := time.Now().Add(-6 * time.Minute)
	if err := store.DB.Model(&current).Updates(map[string]any{"state": model.PlayerStateCultivating, "cultivation_started_at": &started}).Error; err != nil {
		t.Fatal(err)
	}
	result, handled, err = game.Execute("group", player.AccountID, mustParse(t, "出关"))
	if err != nil || !handled {
		t.Fatalf("finish cultivation: handled=%v err=%v", handled, err)
	}
	current, _ = game.players.GetByAccount(player.AccountID)
	if current.Cultivation <= 0 || current.State != model.PlayerStateIdle || current.CultivationStartedAt != nil {
		t.Fatalf("settlement not persisted: cultivation=%d state=%s started=%v result=%+v", current.Cultivation, current.State, current.CultivationStartedAt, result)
	}
}

func TestEveryPlayerCommandHasBusinessRoute(t *testing.T) {
	game, _ := testGame(t)
	player := registerPlayer(t, game, "route-user", "路由真人")
	for _, spec := range handler.CommandTable {
		if spec.EventOnly || spec.ID == 1 || spec.ID == 74 || spec.ID == 1082 || spec.ID == 1083 || spec.ID == 1084 {
			continue
		}
		parsed, ok := handler.ParseCommand(spec.Command)
		if !ok {
			t.Errorf("ID %d command %q did not parse", spec.ID, spec.Command)
			continue
		}
		_, handled, err := game.Execute("group", player.AccountID, parsed)
		if err != nil {
			t.Errorf("ID %d (%s) returned error: %v", spec.ID, spec.Name, err)
		}
		if !handled {
			t.Errorf("ID %d (%s) was not handled", spec.ID, spec.Name)
		}
	}
	parsed := mustParse(t, "传功 missing-user 太虚引")
	if parsed.Spec.ID != 74 {
		t.Fatalf("skill inheritance parsed as ID %d", parsed.Spec.ID)
	}
	if _, handled, err := game.Execute("group", player.AccountID, parsed); err != nil || !handled {
		t.Fatalf("ID 74 route: handled=%v err=%v", handled, err)
	}
}

func TestSubmenuFunctionsAreNativeMarkdownLinks(t *testing.T) {
	game, _ := testGame(t)
	player := registerPlayer(t, game, "menu-user", "菜单真人")
	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "角色菜单"))
	if err != nil || !handled {
		t.Fatalf("角色菜单: handled=%v err=%v", handled, err)
	}
	markdown := result.Markdown()
	for _, command := range []string{"入道", "状态", "背包", "档案"} {
		want := "[" + command + "](mqqapi://aio/inlinecmd?command="
		if !strings.Contains(markdown, want) {
			t.Errorf("submenu markdown missing native link for %s: %s", command, markdown)
		}
	}
	if hasGlobalPagination(result) {
		t.Fatalf("submenu must be a single panel: %+v", result)
	}
	if strings.Contains(result.Text(), "mqqapi://") {
		t.Fatalf("plain-text fallback contains markdown URI: %s", result.Text())
	}
}

func TestWorldMapTravelUpdatesLocationAndConsumesStamina(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "map-user", "云游客")
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("realm_level", 10).Error; err != nil {
		t.Fatal(err)
	}
	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "地图"))
	if err != nil || !handled {
		t.Fatalf("地图: handled=%v err=%v", handled, err)
	}
	var travelAction string
	for _, action := range result.Actions {
		if strings.HasPrefix(action, "前往 ") {
			travelAction = action
			break
		}
	}
	if travelAction == "" {
		t.Fatalf("地图没有相邻地点蓝字动作: %+v", result)
	}

	destination := strings.TrimPrefix(travelAction, "前往 ")
	result, handled, err = game.Execute("group", player.AccountID, mustParse(t, travelAction))
	if err != nil || !handled {
		t.Fatalf("前往: handled=%v err=%v", handled, err)
	}
	current, err := game.players.GetByAccount(player.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Location != destination {
		t.Fatalf("location=%q, want %q", current.Location, destination)
	}
	if stamina := game.playerValueInt(player.ID, "stamina.value", 100); stamina >= 100 {
		t.Fatalf("travel did not consume stamina: %d", stamina)
	}

	var locations []model.WorldLocation
	if err := store.DB.Order("sort_order").Find(&locations).Error; err != nil {
		t.Fatal(err)
	}
	if len(locations) != contentSeedCountForTest() {
		t.Fatalf("location seed count=%d", len(locations))
	}
}

func TestCheckInClaimsOnceAndAppearsInTaskMenu(t *testing.T) {
	game, _ := testGame(t)
	player := registerPlayer(t, game, "checkin-user", "守时真人")
	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "签到"))
	if err != nil || !handled || !strings.Contains(result.Content, "签到成功") {
		t.Fatalf("first check-in: handled=%v err=%v result=%+v", handled, err, result)
	}
	result, handled, err = game.Execute("group", player.AccountID, mustParse(t, "签到"))
	if err != nil || !handled || !strings.Contains(result.Content, "不能重复签到") {
		t.Fatalf("duplicate check-in: handled=%v err=%v result=%+v", handled, err, result)
	}
	result, handled, err = game.Execute("group", player.AccountID, mustParse(t, "任务菜单"))
	if err != nil || !handled || !strings.Contains(result.Markdown(), "签到") {
		t.Fatalf("task menu missing check-in: handled=%v err=%v result=%+v", handled, err, result)
	}
}

func TestLocalNPCDialogueAndMonsterBattleOriginalCommands(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "world-interaction-user", "照山客")
	var location model.WorldLocation
	if err := store.DB.Where("name = ?", "青云山脚").First(&location).Error; err != nil {
		t.Fatal(err)
	}
	dialogue, handled, err := game.Execute("group", player.AccountID, mustParse(t, "对话 东洲巡游使·一"))
	if err != nil || !handled || !strings.Contains(dialogue.Content, "巡游使") || !strings.Contains(dialogue.Content, "可承接委托") {
		t.Fatalf("npc dialogue: handled=%v err=%v result=%+v", handled, err, dialogue)
	}
	battle, handled, err := game.Execute("group", player.AccountID, mustParse(t, "挑战 东洲·青云山脚妖灵"))
	if err != nil || !handled || !strings.Contains(battle.Content, "战斗不会自动结算") {
		t.Fatalf("monster challenge: handled=%v err=%v result=%+v", handled, err, battle)
	}
	turn, handled, err := game.Execute("group", player.AccountID, mustParse(t, "攻击"))
	if err != nil || !handled || (!strings.Contains(turn.Content, "造成") && !strings.Contains(turn.Title, "胜利")) {
		t.Fatalf("monster battle turn: handled=%v err=%v result=%+v", handled, err, turn)
	}
}

func TestDungeonAndGlobalResultPaginationNeverReturnNull(t *testing.T) {
	game, _ := testGame(t)
	player := registerPlayer(t, game, "pagination-user", "观卷真人")
	dungeons, handled, err := game.Execute("group", player.AccountID, mustParse(t, "副本"))
	if err != nil || !handled || strings.TrimSpace(dungeons.Content) == "" || strings.Contains(strings.ToLower(dungeons.Content), "null") {
		t.Fatalf("dungeon list: handled=%v err=%v result=%+v", handled, err, dungeons)
	}

	plainLines := make([]string, 0, 25)
	markdownLines := make([]string, 0, 25)
	for index := 1; index <= 25; index++ {
		plainLines = append(plainLines, fmt.Sprintf("纯文本内容第%d行", index))
		markdownLines = append(markdownLines, fmt.Sprintf("不应优先缓存的Markdown第%d行", index))
	}
	page, err := game.paginateGameResult(&player, GameResult{Title: "分页测试", Content: strings.Join(plainLines, "\n"), MarkdownContent: strings.Join(markdownLines, "\n")})
	if err != nil || !strings.Contains(page.Content, "纯文本内容第1行") || strings.Contains(page.Content, "不应优先缓存") || !containsAction(page.Actions, "翻页 2") {
		t.Fatalf("first page mismatch: err=%v result=%+v", err, page)
	}
	next, handled, err := game.Execute("group", player.AccountID, mustParse(t, "翻页 2"))
	if err != nil || !handled || !strings.Contains(next.Content, "纯文本内容第11行") || strings.Contains(next.Title, "⚙️ ⚙️") {
		t.Fatalf("second page mismatch: handled=%v err=%v result=%+v", handled, err, next)
	}
}

func TestPaidShopRatioAndSpiritualRootCustomization(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "paid-custom-user", "太初客")
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("immortal_jade", 60000).Error; err != nil {
		t.Fatal(err)
	}
	priceTable, handled, err := game.Execute("group", player.AccountID, mustParse(t, "充值菜单"))
	if err != nil || !handled || !strings.Contains(priceTable.Content, "1元 = 2000") || !strings.Contains(priceTable.Content, "200万") || !containsAction(priceTable.Actions, "氪金菜单 2") || containsAction(priceTable.Actions, "定制菜单") {
		t.Fatalf("price table: handled=%v err=%v result=%+v", handled, err, priceTable)
	}
	shop, handled, err := game.Execute("group", player.AccountID, mustParse(t, "仙金商城"))
	if err != nil || !handled || !containsAction(shop.Actions, "仙金购买 "+rootCustomizationVoucher) {
		t.Fatalf("jade shop: handled=%v err=%v result=%+v", handled, err, shop)
	}
	purchase, handled, err := game.Execute("group", player.AccountID, mustParse(t, "仙金购买 "+rootCustomizationVoucher))
	if err != nil || !handled || !strings.Contains(purchase.Content, rootCustomizationVoucher) {
		t.Fatalf("voucher purchase: handled=%v err=%v result=%+v", handled, err, purchase)
	}
	var target model.SpiritualRootTemplate
	if err := store.DB.Where("enabled = ? AND name <> ?", true, player.SpiritualRoot).Order("id DESC").First(&target).Error; err != nil {
		t.Fatal(err)
	}
	customized, handled, err := game.Execute("group", player.AccountID, mustParse(t, "定制灵根 "+target.Name))
	if err != nil || !handled || !strings.Contains(customized.Content, "灵根定制完成") && !strings.Contains(customized.Title, "定制完成") {
		t.Fatalf("root customization: handled=%v err=%v result=%+v", handled, err, customized)
	}
	current, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	var voucher model.Item
	if err := store.DB.Where("name = ?", rootCustomizationVoucher).First(&voucher).Error; err != nil {
		t.Fatal(err)
	}
	if current.SpiritualRoot != target.Name || current.ImmortalJade != 10000 || game.itemQuantity(player.ID, voucher.ID) != 0 {
		t.Fatalf("customization persistence: root=%s jade=%d voucher=%d", current.SpiritualRoot, current.ImmortalJade, game.itemQuantity(player.ID, voucher.ID))
	}
}

func TestPrimaryPanelsNotGloballyPaginated(t *testing.T) {
	game, _ := testGame(t)
	player := registerPlayer(t, game, "single-panel-user", "一览真人")
	for _, command := range []string{"状态", "地图", "位置", "灵检", "菜单", "角色菜单", "公告", "更新公告", "排行榜", "排行 战力"} {
		result, handled, err := game.Execute("group", player.AccountID, mustParse(t, command))
		if err != nil || !handled {
			t.Fatalf("%s: handled=%v err=%v", command, handled, err)
		}
		if hasGlobalPagination(result) {
			t.Fatalf("%s unexpectedly used global pagination: %+v", command, result)
		}
	}
}

func TestStatusImageModeRendersLiveAttributesAndCanSwitchToText(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "status-image-user", "照海真君")
	if !statusImageRenderingSupported() {
		result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "状态"))
		if err != nil || !handled || result.ImageOnly || strings.TrimSpace(result.Content) == "" {
			t.Fatalf("non-Windows status fallback: handled=%v err=%v result=%+v", handled, err, result)
		}
		return
	}
	avatarPath := strings.TrimSpace(os.Getenv("XIANCHEN_STATUS_AVATAR"))
	usingFixtureAvatar := avatarPath == ""
	if usingFixtureAvatar {
		avatarPath = filepath.Join(t.TempDir(), "avatar.png")
		avatar := image.NewRGBA(image.Rect(0, 0, 512, 512))
		for y := 0; y < 512; y++ {
			for x := 0; x < 512; x++ {
				avatar.SetRGBA(x, y, color.RGBA{R: 194, G: 54, B: 62, A: 255})
			}
		}
		avatarFile, err := os.Create(avatarPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(avatarFile, avatar); err != nil {
			_ = avatarFile.Close()
			t.Fatal(err)
		}
		if err := avatarFile.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{
		"avatar_url": avatarPath, "level": 9, "health": 37, "max_health": 200,
		"mana": 81, "max_mana": 120, "cultivation": 4321, "cultivation_required": 9000,
		"combat_power": 6789, "location": "青云山脚", "reputation": 88,
	}).Error; err != nil {
		t.Fatal(err)
	}
	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "状态"))
	if err != nil || !handled || !result.ImageOnly || strings.TrimSpace(result.ImageURL) == "" {
		t.Fatalf("image status: handled=%v err=%v result=%+v", handled, err, result)
	}
	defer os.Remove(result.ImageURL)
	if result.Text() != "" || result.Markdown() != "" || result.Content != "" || len(result.Actions) != 0 {
		t.Fatalf("image status leaked visible text: %+v", result)
	}
	firstBytes, err := os.ReadFile(result.ImageURL)
	if err != nil {
		t.Fatal(err)
	}
	decoded, _, err := image.Decode(bytes.NewReader(firstBytes))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() != 2532 || decoded.Bounds().Dy() != 1484 {
		t.Fatalf("status image size=%v", decoded.Bounds())
	}
	center := color.RGBAModel.Convert(decoded.At(statusPortraitCenterX, statusPortraitCenterY)).(color.RGBA)
	if usingFixtureAvatar && (center.R < 150 || center.G > 100 || center.B > 110) {
		t.Fatalf("player avatar was not drawn into portrait frame: %+v", center)
	}
	frame := color.RGBAModel.Convert(decoded.At(640, statusPortraitCenterY)).(color.RGBA)
	if usingFixtureAvatar && frame.R > 150 && frame.G < 100 && frame.B < 110 {
		t.Fatalf("player avatar covered the template portrait frame: %+v", frame)
	}
	synced, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if synced.CombatPower == 6789 || synced.CombatPower != calculateCombatPower(synced) {
		t.Fatalf("status did not reconcile combat power with live attributes: stored=%d calculated=%d", synced.CombatPower, calculateCombatPower(synced))
	}
	if previewPath := strings.TrimSpace(os.Getenv("XIANCHEN_STATUS_PREVIEW")); previewPath != "" {
		if err := os.WriteFile(previewPath, firstBytes, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{"health": 1, "realm_level": 4, "level": 3, "physical_attack": 45, "combat_power": 7777}).Error; err != nil {
		t.Fatal(err)
	}
	changed, handled, err := game.Execute("group", player.AccountID, mustParse(t, "状态"))
	if err != nil || !handled || !changed.ImageOnly {
		t.Fatalf("changed image status: handled=%v err=%v result=%+v", handled, err, changed)
	}
	defer os.Remove(changed.ImageURL)
	changedBytes, err := os.ReadFile(changed.ImageURL)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(firstBytes, changedBytes) {
		t.Fatal("status image did not change after health, level, realm layer and attack changed")
	}
	changedPlayer, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if changedPlayer.CombatPower == 7777 || changedPlayer.CombatPower != calculateCombatPower(changedPlayer) {
		t.Fatalf("changed status retained stale combat power: stored=%d calculated=%d", changedPlayer.CombatPower, calculateCombatPower(changedPlayer))
	}

	if err := store.DB.Model(&model.SystemSetting{}).Where("key = ?", "display.status_image_mode").Update("value", "false").Error; err != nil {
		t.Fatal(err)
	}
	plain, handled, err := game.Execute("group", player.AccountID, mustParse(t, "状态"))
	if err != nil || !handled || plain.ImageOnly || !strings.Contains(plain.Content, "气血：1/292") || strings.TrimSpace(plain.Text()) == "" {
		t.Fatalf("text status mode: handled=%v err=%v result=%+v", handled, err, plain)
	}
}

func TestSpiritualRootEvolutionUsesOneMigratedProgress(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "root-progress-user", "七转真人")
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("combat_power", 1_000_000).Error; err != nil {
		t.Fatal(err)
	}
	legacyKey := "extended.spiritual_root_evolution_configs.legacy_root.evolve"
	if err := game.setPlayerValueInt(player.ID, legacyKey, 18); err != nil {
		t.Fatal(err)
	}
	inspection, handled, err := game.Execute("group", player.AccountID, mustParse(t, "灵检"))
	if err != nil || !handled || !strings.Contains(inspection.Content, "第5/10重") || !strings.Contains(inspection.Content, "阶段进度：0/7") || hasGlobalPagination(inspection) {
		t.Fatalf("inspect did not migrate the legacy progress into stage five: handled=%v err=%v result=%+v", handled, err, inspection)
	}
	var rootConfig model.SpiritualRootEvolutionConfig
	if err := store.DB.Where("status = ?", "启用").Order("sort_order,id").First(&rootConfig).Error; err != nil {
		t.Fatal(err)
	}
	costs, err := spiritualRootActionCosts(rootConfig.CostMaterials, "awaken", 5)
	if err != nil {
		t.Fatal(err)
	}
	for name, quantity := range costs {
		if name == "灵石" {
			if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("spirit_stones", quantity).Error; err != nil {
				t.Fatal(err)
			}
			continue
		}
		item, err := game.itemByName(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := game.players.AdjustItem(player.ID, item.ID, quantity); err != nil {
			t.Fatal(err)
		}
	}
	awakening, handled, err := game.Execute("group", player.AccountID, mustParse(t, "灵觉"))
	if err != nil || !handled || strings.Contains(awakening.Title, "未解锁") || !strings.Contains(awakening.Content, "本源觉醒") {
		t.Fatalf("awakening did not use migrated evolve progress: handled=%v err=%v result=%+v", handled, err, awakening)
	}
	if value := game.spiritualRootEvolutionValue(player.ID, "evolve"); value != 18 {
		t.Fatalf("canonical evolve progress=%d, want 18", value)
	}
}

func TestShortcutListIsDiscoverableSortedAndPlayerScoped(t *testing.T) {
	game, _ := testGame(t)
	owner := registerPlayer(t, game, "shortcut-list-owner", "听松真人")
	other := registerPlayer(t, game, "shortcut-list-other", "枕云散人")
	for _, command := range []string{"设置快捷 收田=收菜", "设置快捷 回家=位置"} {
		if result, handled, err := game.Execute("group", owner.AccountID, mustParse(t, command)); err != nil || !handled || !strings.Contains(result.Title, "已设置") {
			t.Fatalf("set %s: handled=%v err=%v result=%+v", command, handled, err, result)
		}
	}
	if result, handled, err := game.Execute("group", other.AccountID, mustParse(t, "设置快捷 偷看=状态")); err != nil || !handled || !strings.Contains(result.Title, "已设置") {
		t.Fatalf("set other shortcut: handled=%v err=%v result=%+v", handled, err, result)
	}
	list, handled, err := game.Execute("group", owner.AccountID, mustParse(t, "快捷列表"))
	if err != nil || !handled || !strings.Contains(list.Content, "当前已设置：2条") || !strings.Contains(list.Content, "1. 回家 → 位置") || !strings.Contains(list.Content, "2. 收田 → 收菜") || strings.Contains(list.Content, "偷看") {
		t.Fatalf("shortcut list: handled=%v err=%v result=%+v", handled, err, list)
	}
	if !containsAction(list.Actions, "回家") || !containsAction(list.Actions, "删除快捷 回家") {
		t.Fatalf("shortcut list actions=%v", list.Actions)
	}
	menu, handled, err := game.Execute("group", owner.AccountID, mustParse(t, "菜单"))
	if err != nil || !handled || !strings.Contains(menu.Markdown(), "系统菜单") {
		t.Fatalf("main menu cannot discover system menu: handled=%v err=%v result=%+v", handled, err, menu)
	}
	systemMenu, handled, err := game.Execute("group", owner.AccountID, mustParse(t, "系统菜单"))
	if err != nil || !handled || !strings.Contains(systemMenu.Markdown(), "快捷列表") {
		t.Fatalf("system menu cannot discover shortcut list: handled=%v err=%v result=%+v", handled, err, systemMenu)
	}
	if result, handled, err := game.Execute("group", owner.AccountID, mustParse(t, "删除快捷 回家")); err != nil || !handled || !containsAction(result.Actions, "快捷列表") {
		t.Fatalf("delete shortcut: handled=%v err=%v result=%+v", handled, err, result)
	}
}

func TestBreakthroughAndTribulationRequireConsumableMaterials(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "breakthrough-material-user", "渡玄真人")
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{
		"cultivation": 1_000_000, "dao_heart": 100, "health": player.MaxHealth, "mana": player.MaxMana,
	}).Error; err != nil {
		t.Fatal(err)
	}
	missing, handled, err := game.Execute("group", player.AccountID, mustParse(t, "突破"))
	if err != nil || !handled || !strings.Contains(missing.Title, "缺少破境前置") || !strings.Contains(missing.Content, "淬脉丹×1") {
		t.Fatalf("missing breakthrough material: handled=%v err=%v result=%+v", handled, err, missing)
	}
	material, err := game.itemByName("淬脉丹")
	if err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, material.ID, 1); err != nil {
		t.Fatal(err)
	}
	attempt, handled, err := game.Execute("group", player.AccountID, mustParse(t, "突破"))
	if err != nil || !handled || !strings.Contains(attempt.Content, "淬脉丹×1") || game.itemQuantity(player.ID, material.ID) != 0 {
		t.Fatalf("breakthrough did not consume material: handled=%v err=%v result=%+v quantity=%d", handled, err, attempt, game.itemQuantity(player.ID, material.ID))
	}

	player, err = game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{
		"realm_level": realmStageCount, "cultivation": 1_000_000, "dao_heart": 100,
		"health": player.MaxHealth, "mana": player.MaxMana,
	}).Error; err != nil {
		t.Fatal(err)
	}
	checklist, handled, err := game.Execute("group", player.AccountID, mustParse(t, "备劫"))
	if err != nil || !handled || !strings.Contains(checklist.Content, "引劫玉符：需要1枚 · 当前0枚") || !strings.Contains(checklist.Content, "准备判定：未满足") {
		t.Fatalf("tribulation checklist material: handled=%v err=%v result=%+v", handled, err, checklist)
	}
	blocked, handled, err := game.Execute("group", player.AccountID, mustParse(t, "引劫"))
	if err != nil || !handled || !strings.Contains(blocked.Title, "引劫法器不足") {
		t.Fatalf("tribulation without talisman: handled=%v err=%v result=%+v", handled, err, blocked)
	}
}

func TestSynthesisConsumesMaterialsAndAddsRealOutput(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "synthesis-player", "百炼道人")
	var recipe model.SynthesisRecipe
	if err := store.DB.Where("name = ?", "淬脉丹").First(&recipe).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&recipe).Update("success_rate", 1).Error; err != nil {
		t.Fatal(err)
	}
	for name, quantity := range map[string]int64{"凝露草": 3, "灵茶": 1} {
		item, err := game.itemByName(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := game.players.AdjustItem(player.ID, item.ID, quantity); err != nil {
			t.Fatal(err)
		}
	}
	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "合成 淬脉丹"))
	if err != nil || !handled || !strings.Contains(result.Title, "合成结算") || !strings.Contains(result.Content, "获得：淬脉丹×1") {
		t.Fatalf("synthesis result: handled=%v err=%v result=%+v", handled, err, result)
	}
	output, err := game.itemByName("淬脉丹")
	if err != nil {
		t.Fatal(err)
	}
	if quantity := game.itemQuantity(player.ID, output.ID); quantity != 1 {
		t.Fatalf("synthesis output quantity=%d, want 1", quantity)
	}
	for _, name := range []string{"凝露草", "灵茶"} {
		item, err := game.itemByName(name)
		if err != nil {
			t.Fatal(err)
		}
		if quantity := game.itemQuantity(player.ID, item.ID); quantity != 0 {
			t.Fatalf("material %s quantity=%d, want 0", name, quantity)
		}
	}
}

func TestAllShopsIgnoreLegacyLimitsAndAllowLargePurchases(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "unlimited-shop-player", "万商道人")
	var limitedRows int64
	if err := store.DB.Model(&model.ShopEntry{}).Where("purchase_limit <> ? OR refresh_cycle <> ?", 0, "永不").Count(&limitedRows).Error; err != nil || limitedRows != 0 {
		t.Fatalf("shop migration left %d limited rows: %v", limitedRows, err)
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{"spirit_stones": 5000, "silver_coins": 5000}).Error; err != nil {
		t.Fatal(err)
	}

	var ordinary model.ShopEntry
	if err := store.DB.Where("enabled = ? AND currency = ? AND code NOT LIKE ?", true, "灵石", "seed_shop_%").Order("id").First(&ordinary).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&ordinary).Updates(map[string]any{"price": 1, "purchase_limit": 1, "refresh_cycle": "每日"}).Error; err != nil {
		t.Fatal(err)
	}
	before := game.itemQuantity(player.ID, ordinary.ItemID)
	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "购入 "+ordinary.ItemName+"*1000"))
	if err != nil || !handled || !strings.Contains(result.Content, "常设不限购") || strings.Contains(result.Content, "限购：") {
		t.Fatalf("ordinary unlimited purchase: handled=%v err=%v result=%+v", handled, err, result)
	}
	if quantity := game.itemQuantity(player.ID, ordinary.ItemID); quantity-before != 1000 {
		t.Fatalf("ordinary purchase delta=%d, want 1000", quantity-before)
	}

	var silver model.ShopEntry
	if err := store.DB.Where("enabled = ? AND currency = ?", true, "银币").Order("id").First(&silver).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&silver).Updates(map[string]any{"price": 1, "purchase_limit": 1, "refresh_cycle": "每周"}).Error; err != nil {
		t.Fatal(err)
	}
	before = game.itemQuantity(player.ID, silver.ItemID)
	result, handled, err = game.Execute("group", player.AccountID, mustParse(t, "银币购买 "+silver.ItemName+" 1000"))
	if err != nil || !handled || !strings.Contains(result.Content, "常设不限购") || strings.Contains(result.Title, "限购") {
		t.Fatalf("silver unlimited purchase: handled=%v err=%v result=%+v", handled, err, result)
	}
	if quantity := game.itemQuantity(player.ID, silver.ItemID); quantity-before != 1000 {
		t.Fatalf("silver purchase delta=%d, want 1000", quantity-before)
	}
}

func TestSeedPurchaseFailureGuidesEverySpiritStoneSource(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "seed-guide-player", "耕云散人")
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("spirit_stones", 0).Error; err != nil {
		t.Fatal(err)
	}
	var seed model.ShopEntry
	if err := store.DB.Where("enabled = ? AND code LIKE ?", true, "seed_shop_%").Order("id").First(&seed).Error; err != nil {
		t.Fatal(err)
	}
	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "购买种子 "+seed.ItemName))
	if err != nil || !handled || !strings.Contains(result.Title, "灵石不足") || !strings.Contains(result.Content, "获取灵石") || !strings.Contains(result.Content, "青云入道礼匣") || !containsAction(result.Actions, "礼包") || !containsAction(result.Actions, "探索") || !containsAction(result.Actions, "日常") {
		t.Fatalf("seed currency guide: handled=%v err=%v result=%+v", handled, err, result)
	}
}

func TestHelpIncludesFullPlayFlowCategoriesAndPagedCommandCatalog(t *testing.T) {
	game, _ := testGame(t)
	player := registerPlayer(t, game, "complete-help-player", "问路真人")

	overview, handled, err := game.Execute("group", player.AccountID, mustParse(t, "帮助"))
	if err != nil || !handled || !strings.Contains(overview.Content, "【从零开始】") || !strings.Contains(overview.Content, "【日常成长循环】") || !strings.Contains(overview.Content, "【全部系统入口 · 共43类】") || !strings.Contains(overview.Content, "装备菜单") || !strings.Contains(overview.Content, "图鉴菜单") || !strings.Contains(overview.Content, "活动菜单") || !strings.Contains(overview.Content, "系统菜单") || !strings.Contains(overview.MarkdownContent, "mqqapi://aio/inlinecmd") || !containsAction(overview.Actions, "指令大全") || hasGlobalPagination(overview) {
		t.Fatalf("complete help overview: handled=%v err=%v result=%+v", handled, err, overview)
	}

	category, handled, err := game.Execute("group", player.AccountID, mustParse(t, "帮助 装备"))
	if err != nil || !handled || !strings.Contains(category.Title, "装备操作指南") || !strings.Contains(category.Content, "用途：") || !strings.Contains(category.MarkdownContent, "mqqapi://aio/inlinecmd") || !containsAction(category.Actions, "帮助 装备 2") || hasGlobalPagination(category) {
		t.Fatalf("category help: handled=%v err=%v result=%+v", handled, err, category)
	}
	categorySecond, handled, err := game.Execute("group", player.AccountID, mustParse(t, "帮助 装备 2"))
	if err != nil || !handled || !strings.Contains(categorySecond.Content, "第2/") || !containsAction(categorySecond.Actions, "帮助 装备 1") {
		t.Fatalf("category help page 2: handled=%v err=%v result=%+v", handled, err, categorySecond)
	}

	catalog, handled, err := game.Execute("group", player.AccountID, mustParse(t, "指令大全"))
	if err != nil || !handled || !strings.Contains(catalog.Title, "全部指令大全") || !strings.Contains(catalog.Content, "第1/") || !containsAction(catalog.Actions, "指令大全 2") || hasGlobalPagination(catalog) {
		t.Fatalf("command catalog: handled=%v err=%v result=%+v", handled, err, catalog)
	}
	second, handled, err := game.Execute("group", player.AccountID, mustParse(t, "指令大全 2"))
	if err != nil || !handled || !strings.Contains(second.Content, "第2/") || !containsAction(second.Actions, "指令大全 1") {
		t.Fatalf("command catalog page 2: handled=%v err=%v result=%+v", handled, err, second)
	}

	guide, handled, err := game.Execute("group", player.AccountID, mustParse(t, "怎么玩"))
	if err != nil || !handled || !strings.Contains(guide.Title, "完整玩法指南") {
		t.Fatalf("play guide alias: handled=%v err=%v result=%+v", handled, err, guide)
	}
}

func TestInventoryAndGiftCatalogExposeClickableCompleteEntries(t *testing.T) {
	game, _ := testGame(t)
	player := registerPlayer(t, game, "clickable-inventory-player", "纳珍真人")

	bag, handled, err := game.Execute("group", player.AccountID, mustParse(t, "背包"))
	if err != nil || !handled || !strings.Contains(bag.Content, "青云入道礼匣") || !strings.Contains(bag.MarkdownContent, "command=%E7%89%A9%E5%93%81%20%E9%9D%92%E4%BA%91%E5%85%A5%E9%81%93%E7%A4%BC%E5%8C%A3") || !strings.Contains(bag.MarkdownContent, "开启") {
		t.Fatalf("inventory clickable entries: handled=%v err=%v result=%+v", handled, err, bag)
	}

	gifts, handled, err := game.Execute("group", player.AccountID, mustParse(t, "礼包"))
	if err != nil || !handled || !strings.Contains(gifts.Content, "礼包图鉴") || !strings.Contains(gifts.Content, "当前持有") || !strings.Contains(gifts.Content, "内含：") || !strings.Contains(gifts.Content, "获取：") || !strings.Contains(gifts.MarkdownContent, "mqqapi://aio/inlinecmd") {
		t.Fatalf("complete gift catalog: handled=%v err=%v result=%+v", handled, err, gifts)
	}
}

func TestSpiritualRootTransferDoesNotDependOnEvolutionConfig(t *testing.T) {
	game, _ := testGame(t)
	donor := registerPlayer(t, game, "root-transfer-donor", "雪兔")
	registerPlayer(t, game, "root-transfer-target", "凛")

	result, handled, err := game.Execute("group", donor.AccountID, mustParse(t, "灵传 @凛"))
	if err != nil || !handled || strings.Contains(result.Content, "后台") || strings.Contains(result.Content, "没有启用灵根进化配置") {
		t.Fatalf("spiritual root transfer config fallback: handled=%v err=%v result=%+v", handled, err, result)
	}
	if !strings.Contains(result.Content, "灵根精粹×1") || !strings.Contains(result.Content, "灵石×300") || !containsAction(result.Actions, "合成 太初灵根精粹") {
		t.Fatalf("spiritual root transfer returned no material guidance: %+v", result)
	}
}

func hasGlobalPagination(result GameResult) bool {
	if strings.Contains(result.Content, "长内容保留十分钟") {
		return true
	}
	for _, action := range result.Actions {
		if strings.HasPrefix(action, "翻页 ") {
			return true
		}
	}
	return false
}

func containsAction(actions []string, expected string) bool {
	for _, action := range actions {
		if action == expected {
			return true
		}
	}
	return false
}

func contentSeedCountForTest() int { return 3 }
