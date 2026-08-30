package service

import (
	"strings"
	"testing"
	"time"

	"xianlv/internal/model"
)

func TestCultivationRewardsAutomaticallyGrantPlayerExperience(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "player-level-cultivation", "问道行者")
	var fruit model.Item
	if err := store.DB.Where("name = ?", "灵果").First(&fruit).Error; err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, fruit.ID, 2); err != nil {
		t.Fatal(err)
	}
	before, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	used, handled, err := game.Execute("group", player.AccountID, mustParse(t, "使用 灵果"))
	if err != nil || !handled || !strings.Contains(used.Content, "角色经验：+10") || !strings.Contains(used.Content, "等级进度") {
		t.Fatalf("first cultivation experience: handled=%v err=%v result=%+v", handled, err, used)
	}
	after, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Cultivation-before.Cultivation != 10 || after.Experience-before.Experience != 10 || after.Level != 1 {
		t.Fatalf("first experience before=%+v after=%+v", before, after)
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("experience", 95).Error; err != nil {
		t.Fatal(err)
	}
	beforeLevel, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	leveled, handled, err := game.Execute("group", player.AccountID, mustParse(t, "使用 灵果"))
	if err != nil || !handled || !strings.Contains(leveled.Content, "角色等级：LV1 → LV2") || !strings.Contains(leveled.Content, "升级成长") {
		t.Fatalf("level-up settlement: handled=%v err=%v result=%+v", handled, err, leveled)
	}
	afterLevel, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if afterLevel.Level != 2 || afterLevel.Experience != 5 || afterLevel.MaxHealth != beforeLevel.MaxHealth+model.PlayerHealthPerLevel || afterLevel.MaxMana != beforeLevel.MaxMana+4 || afterLevel.PhysicalAttack != beforeLevel.PhysicalAttack+model.PlayerAttackPerLevel || afterLevel.PhysicalDefense != beforeLevel.PhysicalDefense+model.PlayerDefensePerLevel || afterLevel.CombatPower <= beforeLevel.CombatPower {
		t.Fatalf("leveled player before=%+v after=%+v", beforeLevel, afterLevel)
	}
	levelPage, handled, err := game.Execute("group", player.AccountID, mustParse(t, "等级"))
	if err != nil || !handled || !strings.Contains(levelPage.Content, "角色等级：LV2") || !strings.Contains(levelPage.Content, "5/400") || !strings.Contains(levelPage.Content, "境界修为") {
		t.Fatalf("level page: handled=%v err=%v result=%+v", handled, err, levelPage)
	}
	if err := store.DB.Model(&model.SystemSetting{}).Where("key = ?", "display.status_image_mode").Update("value", "false").Error; err != nil {
		t.Fatal(err)
	}
	status, handled, err := game.Execute("group", player.AccountID, mustParse(t, "状态"))
	if err != nil || !handled || !strings.Contains(status.Content, "角色等级：LV2") || !strings.Contains(status.Content, "等级进度") {
		t.Fatalf("text status level: handled=%v err=%v result=%+v", handled, err, status)
	}
}

func TestEarnSilverIsRealWorkAndFailuresDoNotConsumeResources(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "silver-job-player", "执事清风")
	before, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	beforeStamina, err := game.currentStamina(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "赚银币"))
	if err != nil || !handled || !strings.Contains(result.Title, "仙盟差事完成") || !strings.Contains(result.Content, "银币：+") || !strings.Contains(result.Content, "角色经验：+") {
		t.Fatalf("earn silver: handled=%v err=%v result=%+v", handled, err, result)
	}
	after, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	afterStamina, err := game.currentStamina(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.SilverCoins <= before.SilverCoins || after.Cultivation <= before.Cultivation || after.Experience != after.Cultivation-before.Cultivation || afterStamina != beforeStamina-4 {
		t.Fatalf("job settlement before=%+v after=%+v stamina=%d->%d", before, after, beforeStamina, afterStamina)
	}
	second, handled, err := game.Execute("group", player.AccountID, mustParse(t, "赚银币"))
	if err != nil || !handled || !strings.Contains(second.Title, "尚未刷新") || !strings.Contains(second.Content, "没有扣除体力") {
		t.Fatalf("job cooldown: handled=%v err=%v result=%+v", handled, err, second)
	}
	afterSecond, _ := game.players.Get(player.ID)
	afterSecondStamina, _ := game.currentStamina(player.ID)
	if afterSecond.SilverCoins != after.SilverCoins || afterSecond.Cultivation != after.Cultivation || afterSecondStamina != afterStamina {
		t.Fatalf("cooldown changed resources player=%+v stamina=%d", afterSecond, afterSecondStamina)
	}
	guide, handled, err := game.Execute("group", player.AccountID, mustParse(t, "银币来源"))
	if err != nil || !handled || !strings.Contains(guide.Title, "银币来源") || !strings.Contains(guide.Content, "仙盟差事") || guide.Content == result.Content {
		t.Fatalf("silver guide: handled=%v err=%v result=%+v", handled, err, guide)
	}

	noStamina := registerPlayer(t, game, "silver-job-no-stamina", "倦云散修")
	if err := game.setPlayerValue(noStamina.ID, "stamina.date", time.Now().Format("2006-01-02"), nil); err != nil {
		t.Fatal(err)
	}
	if err := game.setPlayerValueInt(noStamina.ID, "stamina.value", 0); err != nil {
		t.Fatal(err)
	}
	blocked, handled, err := game.Execute("group", noStamina.AccountID, mustParse(t, "赚银币"))
	if err != nil || !handled || !strings.Contains(blocked.Title, "体力不足") || !strings.Contains(blocked.Content, "没有写入冷却") {
		t.Fatalf("stamina block: handled=%v err=%v result=%+v", handled, err, blocked)
	}
	var cooldowns int64
	if err := store.DB.Model(&model.PlayerValue{}).Where("player_id = ? AND key = ?", noStamina.ID, "cooldown.silver_job").Count(&cooldowns).Error; err != nil || cooldowns != 0 {
		t.Fatalf("failed job cooldown count=%d err=%v", cooldowns, err)
	}
}
