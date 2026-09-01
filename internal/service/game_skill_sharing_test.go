package service

import (
	"fmt"
	"strings"
	"testing"

	"xianlv/internal/model"
)

func TestCreatedSkillDifficultyLuckAndStyles(t *testing.T) {
	first := createdSkillRequirements(0)
	third := createdSkillRequirements(2)
	if first.Perception != 30 || first.Cultivation != 200 || first.Scrolls != 2 || first.Tea != 3 {
		t.Fatalf("unexpected first creation requirement: %+v", first)
	}
	if third.Perception <= first.Perception || third.Cultivation <= first.Cultivation || third.Scrolls <= first.Scrolls || third.Tea <= first.Tea {
		t.Fatalf("creation difficulty did not increase: first=%+v third=%+v", first, third)
	}
	ordinary := &model.Player{Perception: 100, DaoHeart: 100, Luck: initialPlayerLuck}
	lucky := &model.Player{Perception: 100, DaoHeart: 100, Luck: maximumPlayerLuck}
	if createdSkillSuccessRate(lucky, first) <= createdSkillSuccessRate(ordinary, first) {
		t.Fatalf("luck did not improve creation rate: ordinary=%d lucky=%d", createdSkillSuccessRate(ordinary, first), createdSkillSuccessRate(lucky, first))
	}

	creator := &model.Player{Perception: 500, Spirit: 300, Luck: 40}
	seen := map[string]bool{}
	for _, style := range createdSkillStyleOrder {
		bonus, description := createdSkillEffect(style, creator, 0)
		text := skillBonusText(bonus)
		if text == "无战斗属性" || description == "" {
			t.Fatalf("style %s has no real effect: bonus=%+v description=%q", style, bonus, description)
		}
		if seen[text] {
			t.Fatalf("style %s duplicated another style effect: %s", style, text)
		}
		seen[text] = true
	}
}

func TestCreatedSkillEffectCapsExtremePlayerAttributes(t *testing.T) {
	extreme := &model.Player{Perception: 9_000_000, Spirit: 8_000_000, Luck: maximumPlayerLuck}
	bonus, _ := createdSkillEffect("剑道", extreme, 50_000)
	limits := model.CreatedSkillEffectLimits("剑道")
	if float64(bonus.PhysicalAttack) > limits["physical_attack"] || float64(bonus.Speed) > limits["speed"] || bonus.CritRate > limits["crit_rate"] {
		t.Fatalf("extreme attributes escaped created-skill limits: bonus=%+v limits=%v", bonus, limits)
	}
}

func TestSkillVitalBonusesCanBeRecoveredAndClampOnSwitch(t *testing.T) {
	game, store := testGame(t)
	player := registerPlayer(t, game, "skill-vital-player", "养元真人")
	healthSkill := model.Skill{Name: "养元炼体经", Type: "炼体", Rarity: "自创", EffectJSON: `{"health":200,"mana":100}`, UpgradeJSON: `{}`}
	plainSkill := model.Skill{Name: "藏锋剑录", Type: "剑道", Rarity: "自创", EffectJSON: `{"physical_attack":20}`, UpgradeJSON: `{}`}
	if err := store.DB.Create(&healthSkill).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Create(&plainSkill).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range []model.PlayerSkill{
		{PlayerID: player.ID, SkillID: healthSkill.ID, Level: 1, Equipped: true},
		{PlayerID: player.ID, SkillID: plainSkill.ID, Level: 1},
	} {
		if err := store.DB.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{
		"current_skill_id": healthSkill.ID, "health": int64(100), "max_health": int64(100), "mana": int64(50), "max_mana": int64(50),
	}).Error; err != nil {
		t.Fatal(err)
	}
	dew, err := game.itemByName("仙露")
	if err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(player.ID, dew.ID, 5); err != nil {
		t.Fatal(err)
	}

	for attempt := 0; attempt < 5; attempt++ {
		current, err := game.players.Get(player.ID)
		if err != nil {
			t.Fatal(err)
		}
		effective := game.playerWithActiveSkillStats(&current)
		if effective.Health >= effective.MaxHealth {
			break
		}
		result, handled, err := game.Execute("group", player.AccountID, mustParse(t, "疗伤"))
		if err != nil || !handled || strings.Contains(result.Title, "气血充盈") {
			t.Fatalf("heal skill health attempt %d: handled=%v err=%v result=%+v", attempt+1, handled, err, result)
		}
		if !strings.Contains(result.Content, "/300") {
			t.Fatalf("heal result did not use effective maximum: %+v", result)
		}
	}
	filled, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	effective := game.playerWithActiveSkillStats(&filled)
	if filled.Health != 300 || effective.Health != 300 || effective.MaxHealth != 300 || effective.MaxMana != 150 {
		t.Fatalf("effective skill vitals not recoverable: stored=%d/%d effective=%d/%d manaMax=%d", filled.Health, filled.MaxHealth, effective.Health, effective.MaxHealth, effective.MaxMana)
	}
	stats := game.playerCombatStats(&filled)
	if stats.Health != 300 || stats.MaxHealth != 300 {
		t.Fatalf("combat vitals re-added or lost skill health: %+v", stats)
	}
	if err := store.DB.Model(&model.SystemSetting{}).Where("key = ?", "display.status_image_mode").Update("value", "false").Error; err != nil {
		t.Fatal(err)
	}
	status, handled, err := game.Execute("group", player.AccountID, mustParse(t, "状态"))
	if err != nil || !handled || !strings.Contains(status.Content, "气血：300/300") {
		t.Fatalf("status did not show recoverable skill health: handled=%v err=%v result=%+v", handled, err, status)
	}
	if err := store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("mana", 150).Error; err != nil {
		t.Fatal(err)
	}
	switched, handled, err := game.Execute("group", player.AccountID, mustParse(t, "换功 "+plainSkill.Name))
	if err != nil || !handled || !strings.Contains(switched.Content, "气血：100/100 · 法力：50/50") {
		t.Fatalf("switch to lower vital limits: handled=%v err=%v result=%+v", handled, err, switched)
	}
	clamped, err := game.players.Get(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if clamped.Health != 100 || clamped.Mana != 50 {
		t.Fatalf("switch did not clamp stored vitals: health=%d mana=%d", clamped.Health, clamped.Mana)
	}
}

func TestCreatedSkillIsPrivateUntilCreatorPublishes(t *testing.T) {
	game, store := testGame(t)
	author := registerPlayer(t, game, "skill-sharing-author", "问剑真人")
	learner := registerPlayer(t, game, "skill-sharing-learner", "听潮散人")
	lateLearner := registerPlayer(t, game, "skill-sharing-late", "观澜道人")
	if err := store.DB.Model(&model.Player{}).Where("id = ?", author.ID).Updates(map[string]any{
		"perception": 500, "spirit": 300, "dao_heart": 100, "luck": 50, "cultivation": 1_000_000,
	}).Error; err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"功法残卷", "灵茶"} {
		var item model.Item
		if err := store.DB.Where("name = ?", name).First(&item).Error; err != nil {
			t.Fatal(err)
		}
		if err := game.players.AdjustItem(author.ID, item.ID, 100); err != nil {
			t.Fatal(err)
		}
	}
	var scroll model.Item
	if err := store.DB.Where("name = ?", "功法残卷").First(&scroll).Error; err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(learner.ID, scroll.ID, 2); err != nil {
		t.Fatal(err)
	}
	if err := game.players.AdjustItem(lateLearner.ID, scroll.ID, 2); err != nil {
		t.Fatal(err)
	}

	const skillName = "云岫问剑录"
	var created GameResult
	for attempt := 0; attempt < 20; attempt++ {
		result, handled, err := game.Execute("group", author.AccountID, mustParse(t, "创功 剑道 "+skillName))
		if err != nil || !handled {
			t.Fatalf("create attempt %d: handled=%v err=%v result=%+v", attempt+1, handled, err, result)
		}
		if strings.Contains(result.Title, "自创功法") {
			created = result
			break
		}
	}
	if !strings.Contains(created.Content, "公开状态：私藏") || !containsAction(created.Actions, "上传功法 "+skillName) {
		t.Fatalf("created skill did not default to private: %+v", created)
	}
	var skill model.Skill
	if err := store.DB.Where("name = ?", skillName).First(&skill).Error; err != nil {
		t.Fatal(err)
	}
	if skill.Type != "剑道" {
		t.Fatalf("created skill type=%q", skill.Type)
	}
	bonus := decodeSkillStatBonus(skill, 1)
	if bonus.PhysicalAttack <= 0 || bonus.Speed <= 0 || bonus.CritRate <= 0 || bonus.MagicAttack != 0 {
		t.Fatalf("sword skill effect=%+v", bonus)
	}
	var publication model.SkillPublication
	if err := store.DB.Where("skill_id = ?", skill.ID).First(&publication).Error; err != nil {
		t.Fatal(err)
	}
	if publication.CreatorPlayerID != author.ID || publication.Published {
		t.Fatalf("private publication=%+v", publication)
	}

	beforeScrolls := game.itemQuantity(learner.ID, scroll.ID)
	blocked, handled, err := game.Execute("group", learner.AccountID, mustParse(t, "学功 "+skillName))
	if err != nil || !handled || !strings.Contains(blocked.Title, "尚未公开") || game.itemQuantity(learner.ID, scroll.ID) != beforeScrolls {
		t.Fatalf("private skill learning was not blocked safely: handled=%v err=%v result=%+v", handled, err, blocked)
	}
	privateLibrary, _, err := game.Execute("group", learner.AccountID, mustParse(t, "功法分享"))
	if err != nil || strings.Contains(privateLibrary.Content, skillName) {
		t.Fatalf("private skill leaked into library: err=%v result=%+v", err, privateLibrary)
	}

	uploaded, handled, err := game.Execute("group", author.AccountID, mustParse(t, "上传功法 "+skillName))
	if err != nil || !handled || !strings.Contains(uploaded.Title, "上传成功") {
		t.Fatalf("publish skill: handled=%v err=%v result=%+v", handled, err, uploaded)
	}
	shared, handled, err := game.Execute("group", learner.AccountID, mustParse(t, "功法分享 剑道"))
	if err != nil || !handled || !strings.Contains(shared.Content, skillName) || !strings.Contains(shared.Content, author.DaoName) || !strings.Contains(shared.Content, "物攻") {
		t.Fatalf("shared library: handled=%v err=%v result=%+v", handled, err, shared)
	}
	learned, handled, err := game.Execute("group", learner.AccountID, mustParse(t, "学功 "+skillName))
	if err != nil || !handled || !strings.Contains(learned.Title, "功法入门") || !strings.Contains(learned.Content, author.DaoName) {
		t.Fatalf("learn shared skill: handled=%v err=%v result=%+v", handled, err, learned)
	}
	viewed, _, err := game.Execute("group", learner.AccountID, mustParse(t, "功法"))
	if err != nil || !strings.Contains(viewed.Content, skillName) || !strings.Contains(viewed.Content, "当前道效") {
		t.Fatalf("learned skill effects missing: err=%v result=%+v", err, viewed)
	}
	switched, _, err := game.Execute("group", learner.AccountID, mustParse(t, "换功 "+skillName))
	if err != nil || !strings.Contains(switched.Content, "战力贡献") || !strings.Contains(switched.Content, "新道效") {
		t.Fatalf("switch did not show real effect delta: err=%v result=%+v", err, switched)
	}

	withdrawn, handled, err := game.Execute("group", author.AccountID, mustParse(t, "撤下功法 "+skillName))
	if err != nil || !handled || !strings.Contains(withdrawn.Title, "已撤下") {
		t.Fatalf("withdraw skill: handled=%v err=%v result=%+v", handled, err, withdrawn)
	}
	late, handled, err := game.Execute("group", lateLearner.AccountID, mustParse(t, "学功 "+skillName))
	if err != nil || !handled || !strings.Contains(late.Title, "尚未公开") {
		t.Fatalf("withdrawn skill allowed new learner: handled=%v err=%v result=%+v", handled, err, late)
	}
	var retained int64
	if err := store.DB.Model(&model.PlayerSkill{}).Where("player_id = ? AND skill_id = ?", learner.ID, skill.ID).Count(&retained).Error; err != nil || retained != 1 {
		t.Fatalf("existing learner lost withdrawn skill: count=%d err=%v", retained, err)
	}
	if t.Failed() {
		t.Log(fmt.Sprintf("created=%+v uploaded=%+v withdrawn=%+v", created, uploaded, withdrawn))
	}
}
