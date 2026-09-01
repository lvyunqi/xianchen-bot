package service

import (
	"strings"
	"testing"

	"xianlv/internal/model"
)

func TestLuckStartsAtTenGrowsToFiftyAndChangesProbabilities(t *testing.T) {
	game, store := testGame(t)
	result, handled, err := game.Execute("group", "luck-player", mustParse(t, "入道 观星客 男"))
	if err != nil || !handled {
		t.Fatalf("register with gender: handled=%v err=%v result=%+v", handled, err, result)
	}
	player, err := game.players.GetByAccount("luck-player")
	if err != nil {
		t.Fatal(err)
	}
	if player.Luck != initialPlayerLuck || player.Gender != "男修" {
		t.Fatalf("initial profile luck=%d gender=%q", player.Luck, player.Gender)
	}
	baseActual, baseBonus := probabilityWithLuck(.30, initialPlayerLuck, luckTreasureBonusCap)
	maxActual, maxBonus := probabilityWithLuck(.30, maximumPlayerLuck, luckTreasureBonusCap)
	if baseActual != .30 || baseBonus != 0 || maxActual != .50 || maxBonus != .20 {
		t.Fatalf("unexpected luck probability curve: base=%.3f/%.3f max=%.3f/%.3f", baseActual, baseBonus, maxActual, maxBonus)
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("luck", 49).Error; err != nil {
		t.Fatal(err)
	}
	player.Luck = 49
	line, err := game.tryGrowLuckFromEncounterRoll(&player, 0)
	if err != nil || !strings.Contains(line, "49 → 50/50") {
		t.Fatalf("guaranteed encounter luck growth: line=%q err=%v", line, err)
	}
	updated, _ := game.players.Get(player.ID)
	if updated.Luck != maximumPlayerLuck {
		t.Fatalf("luck after growth=%d", updated.Luck)
	}
	line, err = game.tryGrowLuckFromEncounterRoll(&updated, 0)
	if err != nil || !strings.Contains(line, "已达上限") {
		t.Fatalf("luck cap response: line=%q err=%v", line, err)
	}
	fortune, handled, err := game.Execute("group", player.AccountID, mustParse(t, "运气"))
	if err != nil || !handled || !strings.Contains(fortune.Content, "50/50") || !strings.Contains(fortune.Content, "炼丹") {
		t.Fatalf("luck command: handled=%v err=%v result=%+v", handled, err, fortune)
	}
}

func TestGenderGatesCoupleProfileAndSmallCultivationTransferDoesNotVanish(t *testing.T) {
	game, store := testGame(t)
	unregisteredGender := registerPlayer(t, game, "legacy-gender-player", "听风客")
	search, handled, err := game.Execute("group", unregisteredGender.AccountID, mustParse(t, "寻缘"))
	if err != nil || !handled || !strings.Contains(search.Title, "道籍不全") {
		t.Fatalf("unset gender gate: handled=%v err=%v result=%+v", handled, err, search)
	}
	if _, handled, err = game.Execute("group", unregisteredGender.AccountID, mustParse(t, "性别 男")); err != nil || !handled {
		t.Fatalf("fill legacy gender: handled=%v err=%v", handled, err)
	}

	_, _, err = game.Execute("group", "female-player", mustParse(t, "入道 照月客 女"))
	if err != nil {
		t.Fatal(err)
	}
	female, _ := game.players.GetByAccount("female-player")
	if err := store.DB.Model(&model.Player{}).Where("id = ?", unregisteredGender.ID).Update("cultivation", 1).Error; err != nil {
		t.Fatal(err)
	}
	transfer, handled, err := game.Execute("group", unregisteredGender.AccountID, mustParse(t, "传功 "+female.DaoName+" 1"))
	if err != nil || !handled || !strings.Contains(transfer.Content, "对方实得：1") || !strings.Contains(transfer.Content, "传输损耗：0") {
		t.Fatalf("small transfer: handled=%v err=%v result=%+v", handled, err, transfer)
	}
	femaleAfter, _ := game.players.Get(female.ID)
	if femaleAfter.Cultivation != female.Cultivation+1 {
		t.Fatalf("recipient cultivation=%d want=%d", femaleAfter.Cultivation, female.Cultivation+1)
	}

	if _, _, err := game.Execute("group", unregisteredGender.AccountID, mustParse(t, "结缘 "+female.DaoName)); err != nil {
		t.Fatal(err)
	}
	bond, handled, err := game.Execute("group", female.AccountID, mustParse(t, "应缘"))
	if err != nil || !handled || !strings.Contains(bond.Content, "男修") || !strings.Contains(bond.Content, "女修") {
		t.Fatalf("gendered bond: handled=%v err=%v result=%+v", handled, err, bond)
	}
	beforeFemaleDual, _ := game.players.Get(female.ID)
	beforePartnerDual, _ := game.players.Get(unregisteredGender.ID)
	dual, handled, err := game.Execute("group", female.AccountID, mustParse(t, "双修"))
	if err != nil || !handled || !strings.Contains(dual.Content, "男修") || !strings.Contains(dual.Content, "女修") {
		t.Fatalf("gendered dual cultivation: handled=%v err=%v result=%+v", handled, err, dual)
	}
	afterFemaleDual, _ := game.players.Get(female.ID)
	afterPartnerDual, _ := game.players.Get(unregisteredGender.ID)
	if afterFemaleDual.Experience <= beforeFemaleDual.Experience || afterPartnerDual.Experience <= beforePartnerDual.Experience || afterFemaleDual.Cultivation-beforeFemaleDual.Cultivation != afterFemaleDual.Experience-beforeFemaleDual.Experience || afterPartnerDual.Cultivation-beforePartnerDual.Cultivation != afterPartnerDual.Experience-beforePartnerDual.Experience {
		t.Fatalf("dual experience was not granted to both players: female=%+v -> %+v partner=%+v -> %+v", beforeFemaleDual, afterFemaleDual, beforePartnerDual, afterPartnerDual)
	}
}

func TestSpiritualRootFusionUsesDistinctParentsAndRequiresConfirmation(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "root-fusion-player", "合道真人")
	guide, handled, err := game.Execute("group", player.AccountID, mustParse(t, "灵融"))
	if err != nil || !handled || !strings.Contains(guide.Title, "替换型") || !strings.Contains(guide.Content, "不叠加") || !strings.Contains(guide.Content, "替换当前灵根") || !strings.Contains(guide.Content, "材料不返还") {
		t.Fatalf("fusion guide is ambiguous: handled=%v err=%v result=%+v", handled, err, guide)
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{
		"realm_level": 3, "root_quality": 60, "spirit_stones": 3000, "luck": 50,
	}).Error; err != nil {
		t.Fatal(err)
	}
	var roots []model.SpiritualRootTemplate
	if err := store.DB.Where("enabled = ? AND name <> ?", true, player.SpiritualRoot).Order("id").Limit(2).Find(&roots).Error; err != nil || len(roots) != 2 {
		t.Fatalf("load fusion parents: roots=%d err=%v", len(roots), err)
	}
	// Test seeds intentionally keep catalogues tiny; add one valid non-parent
	// result so the production rule (random third root) can be exercised.
	third := model.SpiritualRootTemplate{Code: "test_fusion_third", Name: "太初星河界心灵根", Element: "星辰", Grade: "仙灵", BaseQuality: 72, CultivationBonus: 1.42, RarityWeight: 8, Enabled: true}
	if err := store.DB.Where("code = ?", third.Code).FirstOrCreate(&third).Error; err != nil {
		t.Fatal(err)
	}
	essence, err := game.itemByName("灵根精粹")
	if err != nil {
		t.Fatal(err)
	}
	stone, err := game.itemByName("阵基石")
	if err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, essence.ID, 4); err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, stone.ID, 2); err != nil {
		t.Fatal(err)
	}

	same, handled, err := game.Execute("group", player.AccountID, mustParse(t, "灵根合成 "+roots[0].Name+" "+roots[0].Name))
	if err != nil || !handled || !strings.Contains(same.Title, "父系道纹相同") {
		t.Fatalf("same-parent fusion: handled=%v err=%v result=%+v", handled, err, same)
	}
	if game.itemQuantity(player.ID, essence.ID) != 4 || game.itemQuantity(player.ID, stone.ID) != 2 {
		t.Fatal("same-parent fusion consumed materials")
	}

	fused, handled, err := game.Execute("group", player.AccountID, mustParse(t, "灵根合成 "+roots[0].Name+" "+roots[1].Name))
	if err != nil || !handled || !strings.Contains(fused.Content, "已参与稀有灵根权重") {
		t.Fatalf("distinct fusion: handled=%v err=%v result=%+v", handled, err, fused)
	}
	pending, err := game.loadPendingSpiritualRoot(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Result == roots[0].Name || pending.Result == roots[1].Name || pending.Result == player.SpiritualRoot {
		t.Fatalf("fusion did not generate a third root: %+v", pending)
	}
	afterFusion, _ := game.players.Get(player.ID)
	if afterFusion.SpiritualRoot != player.SpiritualRoot || game.itemQuantity(player.ID, essence.ID) != 2 || game.itemQuantity(player.ID, stone.ID) != 1 || afterFusion.SpiritStones != 2500 {
		t.Fatalf("fusion settlement player=%+v essence=%d stone=%d", afterFusion, game.itemQuantity(player.ID, essence.ID), game.itemQuantity(player.ID, stone.ID))
	}
	duplicate, _, err := game.Execute("group", player.AccountID, mustParse(t, "灵根合成 "+roots[0].Name+" "+roots[1].Name))
	if err != nil || !strings.Contains(duplicate.Title, "已有待定灵根") || game.itemQuantity(player.ID, essence.ID) != 2 {
		t.Fatalf("pending root overwrite guard: err=%v result=%+v", err, duplicate)
	}
	discarded, handled, err := game.Execute("group", player.AccountID, mustParse(t, "放弃灵根"))
	if err != nil || !handled || !strings.Contains(discarded.Title, "已散去") {
		t.Fatalf("discard fused root: handled=%v err=%v result=%+v", handled, err, discarded)
	}
	afterDiscard, _ := game.players.Get(player.ID)
	if afterDiscard.SpiritualRoot != player.SpiritualRoot {
		t.Fatalf("discard changed current root: before=%s after=%s", player.SpiritualRoot, afterDiscard.SpiritualRoot)
	}
	if _, err := game.loadPendingSpiritualRoot(player.ID); err == nil {
		t.Fatal("pending root remained after discard")
	}

	refused, handled, err := game.Execute("group", player.AccountID, mustParse(t, "灵根合成 "+roots[0].Name+" "+roots[1].Name))
	if err != nil || !handled || !strings.Contains(refused.Title, "替换型灵根道种已凝成") || !strings.Contains(refused.Content, "不会与当前灵根叠加属性") {
		t.Fatalf("second fusion: handled=%v err=%v result=%+v", handled, err, refused)
	}
	pending, err = game.loadPendingSpiritualRoot(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	pendingResult, handled, err := game.Execute("group", player.AccountID, mustParse(t, "灵根道种"))
	if err != nil || !handled || !strings.Contains(pendingResult.Title, "替换型") || !strings.Contains(pendingResult.Content, "不会与当前灵根叠加属性") || !strings.Contains(pendingResult.Content, "不会返还") {
		t.Fatalf("pending fusion guide is ambiguous: handled=%v err=%v result=%+v", handled, err, pendingResult)
	}

	absorbed, handled, err := game.Execute("group", player.AccountID, mustParse(t, "吸收灵根"))
	if err != nil || !handled || !strings.Contains(absorbed.Title, "吸收完成") {
		t.Fatalf("absorb fused root: handled=%v err=%v result=%+v", handled, err, absorbed)
	}
	afterAbsorb, _ := game.players.Get(player.ID)
	if afterAbsorb.SpiritualRoot != pending.Result || afterAbsorb.CombatPower <= 0 {
		t.Fatalf("absorbed root profile=%+v pending=%+v", afterAbsorb, pending)
	}
	if _, err := game.loadPendingSpiritualRoot(player.ID); err == nil {
		t.Fatal("pending root remained after absorption")
	}
}
