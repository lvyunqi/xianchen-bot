package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"xianlv/internal/handler"
	"xianlv/internal/model"
)

func (g *Game) executeSkill(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 69:
		return g.learnSkill(player, command.RawArguments)
	case 70:
		return g.viewSkills(player)
	case 71:
		return g.switchSkill(player, command.RawArguments)
	case 72:
		return g.breakSkill(player)
	case 73:
		return g.createSkill(player, command.RawArguments)
	case 74:
		return g.inheritSkill(player, command.Arguments)
	default:
		return GameResult{}, false, nil
	}
}

func (g *Game) skillByName(name string) (model.Skill, error) {
	var skill model.Skill
	err := g.store.DB.Where("name = ?", strings.TrimSpace(name)).First(&skill).Error
	return skill, err
}

func (g *Game) learnSkill(player *model.Player, argument string) (GameResult, bool, error) {
	name := strings.TrimSpace(argument)
	if name == "" {
		return GameResult{Title: "学习功法", Content: "请输入：`学功 功法名`。发送 `功法` 查看功法库。"}, true, nil
	}
	skill, err := g.skillByName(name)
	if err != nil {
		return GameResult{Title: "功法不存在", Content: "万法阁中没有收录“" + name + "”。", Actions: []string{"功法"}}, true, nil
	}
	var exists int64
	g.store.DB.Model(&model.PlayerSkill{}).Where("player_id = ? AND skill_id = ?", player.ID, skill.ID).Count(&exists)
	if exists > 0 {
		return GameResult{Title: "已学功法", Content: "你已经掌握" + skill.Name + "。"}, true, nil
	}
	creator := ""
	if skill.Rarity == "自创" {
		var publication model.SkillPublication
		if err := g.store.DB.Where("skill_id = ? AND published = ?", skill.ID, true).First(&publication).Error; err != nil {
			return GameResult{Title: "自创功法尚未公开", Content: fmt.Sprintf("%s当前由作者私藏，没有进入全服分享阁。只有原作者主动上传后，其他玩家才能学习。", skill.Name), Actions: []string{"功法分享", "功法", "我的创功"}}, true, nil
		}
		creator = publication.CreatorName
	}
	row := model.PlayerSkill{PlayerID: player.ID, SkillID: skill.ID, Level: 1, Mastery: 0, Equipped: player.CurrentSkillID == 0}
	if err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := consumeNamedItemTx(tx, player.ID, "功法残卷", 1); err != nil {
			return err
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if player.CurrentSkillID == 0 {
			return tx.Model(&model.Player{}).Where("id = ?", player.ID).Update("current_skill_id", skill.ID).Error
		}
		return nil
	}); err != nil {
		if strings.Contains(err.Error(), "insufficient item quantity") || errors.Is(err, gorm.ErrRecordNotFound) {
			return GameResult{Title: "学习失败", Content: "需要功法残卷×1。", Actions: []string{"探幽", "秘境", "物品 功法残卷"}}, true, nil
		}
		return GameResult{}, true, err
	}
	creatorText := ""
	if creator != "" {
		creatorText = "\n创功者：" + creator
	}
	return GameResult{Title: "功法入门", Content: fmt.Sprintf("消耗功法残卷×1\n学会：%s\n流派：%s%s\n真实道效：%s\n%s", skill.Name, skill.Type, creatorText, skillBonusText(decodeSkillStatBonus(skill, 1)), skill.Description), Actions: []string{"换功 " + skill.Name, "功法", "精进", "功法分享"}}, true, nil
}

func (g *Game) viewSkills(player *model.Player) (GameResult, bool, error) {
	type skillRow struct {
		ID         uint
		Name       string
		Type       string
		Rarity     string
		EffectJSON string
		Level      int
		Mastery    int64
		Equipped   bool
	}
	var rows []skillRow
	err := g.store.DB.Table("player_skills").Select("skills.id, skills.name, skills.type, skills.rarity, skills.effect_json, player_skills.level, player_skills.mastery, player_skills.equipped").Joins("JOIN skills ON skills.id = player_skills.skill_id").Where("player_skills.player_id = ?", player.ID).Order("player_skills.equipped DESC, skills.name").Scan(&rows).Error
	if err != nil {
		return GameResult{}, true, err
	}
	if len(rows) == 0 {
		var library []model.Skill
		_ = g.store.DB.Where("rarity <> ? OR id IN (SELECT skill_id FROM skill_publications WHERE published = ?)", "自创", true).Order("id").Limit(10).Find(&library).Error
		lines := []string{"你尚未学习功法。", "", "可学功法："}
		actions := []string{}
		for _, skill := range library {
			lines = append(lines, fmt.Sprintf("- %s · %s · %s", skill.Name, skill.Type, skillBonusText(decodeSkillStatBonus(skill, 1))))
			actions = append(actions, "学功 "+skill.Name)
		}
		actions = append(actions, "功法分享", "创功")
		return GameResult{Title: "功法阁", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		mark := ""
		if row.Equipped || row.ID == player.CurrentSkillID {
			mark = "【主修】"
		}
		skill := model.Skill{Name: row.Name, Type: row.Type, Rarity: row.Rarity, EffectJSON: row.EffectJSON}
		lines = append(lines, fmt.Sprintf("- %s%s【%s】· %d级 · 熟练%d\n  当前道效：%s", mark, row.Name, row.Type, row.Level, row.Mastery, skillBonusText(decodeSkillStatBonus(skill, row.Level))))
	}
	return GameResult{Title: "已学功法", Content: strings.Join(lines, "\n"), Actions: []string{"精进", "功突", "我的创功", "功法分享"}}, true, nil
}

func (g *Game) switchSkill(player *model.Player, argument string) (GameResult, bool, error) {
	skill, err := g.skillByName(argument)
	if err != nil {
		return GameResult{Title: "切换失败", Content: "没有找到该功法。"}, true, nil
	}
	var owned model.PlayerSkill
	if err := g.store.DB.Where("player_id = ? AND skill_id = ?", player.ID, skill.ID).First(&owned).Error; err != nil {
		return GameResult{Title: "切换失败", Content: "你尚未学会" + skill.Name + "。"}, true, nil
	}
	oldBonus := g.activeSkillStatBonus(player)
	oldPower := skillCombatPowerContribution(player, oldBonus)
	newBonus := decodeSkillStatBonus(skill, owned.Level)
	newPower := skillCombatPowerContribution(player, newBonus)
	newMaximumHealth := max64(player.MaxHealth+newBonus.Health, 1)
	newMaximumMana := max64(player.MaxMana+newBonus.Mana, 0)
	newHealth := min64(max64(player.Health, 0), newMaximumHealth)
	newMana := min64(max64(player.Mana, 0), newMaximumMana)
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.PlayerSkill{}).Where("player_id = ?", player.ID).Update("equipped", false).Error; err != nil {
			return err
		}
		if err := tx.Model(&owned).Update("equipped", true).Error; err != nil {
			return err
		}
		if err := tx.Model(player).Updates(map[string]any{"current_skill_id": skill.ID, "health": newHealth, "mana": newMana}).Error; err != nil {
			return err
		}
		return syncPVEBattleVitalsTx(tx, player.ID, &newHealth, &newMana)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	oldName := displayOr(oldBonus.Name, "无")
	return GameResult{Title: "主修切换", Content: fmt.Sprintf("原主修：%s\n当前主修：**%s**【%s】\n━━━━━━━━━━━\n原道效：%s\n新道效：%s\n战力贡献：%d → %d（%+d）\n气血：%d/%d · 法力：%d/%d\n━━━━━━━━━━━\n%s", oldName, skill.Name, skill.Type, skillBonusText(oldBonus), skillBonusText(newBonus), oldPower, newPower, newPower-oldPower, newHealth, newMaximumHealth, newMana, newMaximumMana, skill.Description), Actions: []string{"状态", "功法", "精进", "功突"}}, true, nil
}

func (g *Game) breakSkill(player *model.Player) (GameResult, bool, error) {
	if player.CurrentSkillID == 0 {
		return GameResult{Title: "功法突破", Content: "当前没有主修功法。"}, true, nil
	}
	var row model.PlayerSkill
	if err := g.store.DB.Where("player_id = ? AND skill_id = ?", player.ID, player.CurrentSkillID).First(&row).Error; err != nil {
		return GameResult{}, true, err
	}
	var skill model.Skill
	_ = g.store.DB.First(&skill, row.SkillID).Error
	required := int64(row.Level * 100)
	if row.Mastery < required {
		return GameResult{Title: "功法火候不足", Content: fmt.Sprintf("%s突破需要熟练度%d，当前%d。", skill.Name, required, row.Mastery), Actions: []string{"精进"}}, true, nil
	}
	if err := g.store.DB.Model(&row).Updates(map[string]any{"level": gorm.Expr("level + 1"), "mastery": gorm.Expr("mastery - ?", required)}).Error; err != nil {
		return GameResult{}, true, err
	}
	_ = g.store.DB.Model(player).Updates(map[string]any{"physical_attack": gorm.Expr("physical_attack + 2"), "magic_attack": gorm.Expr("magic_attack + 2"), "combat_power": gorm.Expr("combat_power + 8")}).Error
	return GameResult{Title: "功法突破", Content: fmt.Sprintf("%s：%d级 → %d级\n物攻：+2\n法强：+2", skill.Name, row.Level, row.Level+1), Actions: []string{"功法", "状态"}}, true, nil
}

func (g *Game) createSkill(player *model.Player, argument string) (GameResult, bool, error) {
	return g.createPlayerSkill(player, argument)
}

func (g *Game) nextCreatedSkillName(player *model.Player) (string, error) {
	prefixes := []string{"", "太初", "玄元", "青冥", "紫府", "星河", "归元", "照夜", "问心", "御虚", "藏锋", "化劫", "逐星", "镇岳", "流光", "长生"}
	suffixes := []string{"心经", "玄功", "道典", "真解", "仙诀", "灵章", "天书", "秘录", "法经", "道藏", "神篇", "宝卷", "妙法", "玉章", "圣典", "元经"}
	for _, prefix := range prefixes {
		for _, suffix := range suffixes {
			candidate := player.DaoName + prefix + suffix
			var count int64
			if err := g.store.DB.Model(&model.Skill{}).Where("name = ?", candidate).Count(&count).Error; err != nil {
				return "", err
			}
			if count == 0 {
				return candidate, nil
			}
		}
	}
	return "", errors.New("created skill name catalog exhausted")
}

func (g *Game) inheritSkill(player *model.Player, args []string) (GameResult, bool, error) {
	if len(args) < 2 {
		return GameResult{Title: "传承功法", Content: "请输入：`传功 @对方 功法名`"}, true, nil
	}
	_, partner, err := g.activeCouple(player)
	if err != nil {
		return GameResult{Title: "传承失败", Content: "功法只能传给仙侣。"}, true, nil
	}
	target, findErr := g.findPlayer(args[0])
	if findErr != nil || target.ID != partner.ID {
		return GameResult{Title: "传承失败", Content: "目标不是你的仙侣。"}, true, nil
	}
	skillName := strings.Join(args[1:], " ")
	skill, err := g.skillByName(skillName)
	if err != nil {
		return GameResult{Title: "传承失败", Content: "功法不存在。"}, true, nil
	}
	var owned model.PlayerSkill
	if err := g.store.DB.Where("player_id = ? AND skill_id = ?", player.ID, skill.ID).First(&owned).Error; err != nil {
		return GameResult{Title: "传承失败", Content: "你尚未掌握该功法。"}, true, nil
	}
	copyRow := model.PlayerSkill{PlayerID: target.ID, SkillID: skill.ID, Level: maxInt(1, owned.Level-1), Mastery: 0}
	if err := g.store.DB.Where("player_id = ? AND skill_id = ?", target.ID, skill.ID).FirstOrCreate(&copyRow).Error; err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "功法传承", Content: fmt.Sprintf("你将%s的运功法门传给%s。\n对方获得：%s %d级", skill.Name, target.DaoName, skill.Name, copyRow.Level)}, true, nil
}

func (g *Game) executePet(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 75:
		return g.capturePet(player)
	case 76:
		return g.viewPets(player)
	case 77:
		return g.activatePet(player, command.RawArguments)
	case 78:
		return g.feedPet(player, command.RawArguments)
	case 79:
		return g.evolvePet(player)
	case 80:
		return g.releasePet(player, command.RawArguments)
	default:
		return GameResult{}, false, nil
	}
}

const pendingPetEncounterKey = "pet.capture.encounter"

type pendingPetEncounter struct {
	TemplateID uint      `json:"template_id"`
	PetName    string    `json:"pet_name"`
	Location   string    `json:"location"`
	StartedAt  time.Time `json:"started_at"`
}

func petCareKey(petID uint) string {
	return "pet.care." + strconv.FormatUint(uint64(petID), 10)
}

func (g *Game) discoverPetEncounter(player *model.Player, location model.WorldLocation, remainingStamina int64) (GameResult, bool, error) {
	if pending, err := g.loadPendingPetEncounter(player.ID); err == nil {
		var existing model.PetTemplate
		if g.store.DB.First(&existing, pending.TemplateID).Error == nil && existing.Enabled {
			baseRate, rate, luckBonus := g.petCaptureRate(player, existing)
			return GameResult{Title: "🐉 灵兽踪迹尚在", Content: fmt.Sprintf("你再次循迹确认，%s仍潜伏在%s附近。\n━━━━━━━━━━━\n灵兽战力：%d\n捕获率：基础%.1f%% · 运气+%.1f%% · 实际%.1f%%\n所需：灵兽口粮×1、体力5\n剩余体力：%d\n━━━━━━━━━━━\n这次遭遇只能尝试捕获一次，十分钟后踪迹消散。", existing.Name, pending.Location, existing.InitialPower, baseRate*100, luckBonus*100, rate*100, remainingStamina), Actions: []string{"捕获", "物品 灵兽口粮", "灵兽", "位置"}}, true, nil
		}
	}
	var templates []model.PetTemplate
	if err := g.store.DB.Where("enabled = ?", true).Find(&templates).Error; err != nil || len(templates) == 0 {
		return GameResult{Title: "🐉 万兽谱尚未载入", Content: "当前没有可遭遇的灵兽，本次探索不会生成空白目标。请主人检查灵兽数据。", Actions: []string{"探索", "反馈菜单"}}, true, nil
	}
	template := templates[rand.Intn(len(templates))]
	pending := pendingPetEncounter{TemplateID: template.ID, PetName: template.Name, Location: location.Name, StartedAt: time.Now()}
	encoded, _ := json.Marshal(pending)
	expires := time.Now().Add(time.Duration(max64(g.settingInt("pet.encounter_minutes", 10), 1)) * time.Minute)
	if err := g.setPlayerValue(player.ID, pendingPetEncounterKey, string(encoded), &expires); err != nil {
		return GameResult{}, true, err
	}
	baseRate, rate, luckBonus := g.petCaptureRate(player, template)
	return GameResult{Title: "🐉 " + location.Name + "·灵兽现踪", Content: fmt.Sprintf("林间灵息忽然收束，一只%s从%s的山雾中现身，正警惕地观察你的御兽印。\n━━━━━━━━━━━\n灵兽战力：%d\n捕获率：基础%.1f%% · 运气+%.1f%% · 实际%.1f%%\n所需：灵兽口粮×1、体力5\n当前体力：%d\n━━━━━━━━━━━\n遭遇保留十分钟且只能尝试一次；成功、失败或离开当前地图后，必须重新探索灵兽踪迹。", template.Name, location.Name, template.InitialPower, baseRate*100, luckBonus*100, rate*100, remainingStamina), ImageURL: location.ImageURL, Actions: []string{"捕获", "物品 灵兽口粮", "背包搜索 灵兽", "灵兽", "位置"}}, true, nil
}

func (g *Game) loadPendingPetEncounter(playerID uint) (pendingPetEncounter, error) {
	value, err := g.playerValue(playerID, pendingPetEncounterKey)
	if err != nil {
		return pendingPetEncounter{}, err
	}
	var pending pendingPetEncounter
	if json.Unmarshal([]byte(value), &pending) != nil || pending.TemplateID == 0 {
		return pendingPetEncounter{}, gorm.ErrRecordNotFound
	}
	return pending, nil
}

func (g *Game) petCaptureRate(player *model.Player, template model.PetTemplate) (float64, float64, float64) {
	baseRate := .25 + float64(player.ImmortalAffinity)/2000
	if player.CombatPower < template.InitialPower/2 {
		baseRate -= .10
	}
	if player.Spirit > 10 {
		baseRate += float64(min64(player.Spirit-10, 40)) / 400
	}
	if baseRate < .08 {
		baseRate = .08
	}
	if baseRate > .65 {
		baseRate = .65
	}
	rate, luckBonus := probabilityWithLuck(baseRate, player.Luck, luckPetCaptureBonusCap)
	return baseRate, rate, luckBonus
}

func (g *Game) petCapacity(player *model.Player) int64 {
	sequence, err := g.playerRealmSequence(player)
	if err != nil || sequence < 1 {
		sequence = 1
	}
	base := max64(g.settingInt("pet.base_capacity", 5), 1)
	perRealm := max64(g.settingInt("pet.capacity_per_realm", 1), 0)
	return base + int64(sequence-1)*perRealm
}

func (g *Game) capturePet(player *model.Player) (GameResult, bool, error) {
	if player.Health <= 1 {
		return GameResult{Title: "🐉 元神离体，无法捕获", Content: "阵亡状态不能追踪或收服灵兽。请先回城复生并恢复气血。", Actions: []string{"回城复活", "状态"}}, true, nil
	}
	pending, err := g.loadPendingPetEncounter(player.ID)
	if err != nil {
		return GameResult{Title: "🐉 当前没有灵兽遭遇", Content: "捕获不能凭空随机生成灵兽。请先在当前地图发送“探索”，触发“灵兽现踪”后再于十分钟内尝试捕获。\n━━━━━━━━━━━\n没有遭遇时发送捕获，不会扣除体力、口粮或冷却。", Actions: []string{"探索", "位置", "物品 灵兽口粮", "灵兽菜单"}}, true, nil
	}
	if strings.TrimSpace(player.Location) != strings.TrimSpace(pending.Location) {
		_ = g.store.DB.Where("player_id = ? AND key = ?", player.ID, pendingPetEncounterKey).Delete(&model.PlayerValue{}).Error
		return GameResult{Title: "🐉 灵兽踪迹已断", Content: fmt.Sprintf("这只灵兽出没于%s，你已离开该地，遭遇随之失效。请在当前地图重新探索。", pending.Location), Actions: []string{"探索", "位置"}}, true, nil
	}
	var template model.PetTemplate
	if err := g.store.DB.Where("id = ? AND enabled = ?", pending.TemplateID, true).First(&template).Error; err != nil {
		return GameResult{Title: "🐉 灵兽踪迹已失效", Content: "该灵兽已离开万兽谱，本次不会扣除资源。请重新探索。", Actions: []string{"探索", "灵兽菜单"}}, true, nil
	}
	capacity := g.petCapacity(player)
	var ownedPets int64
	if err := g.store.DB.Model(&model.Pet{}).Where("player_id = ?", player.ID).Count(&ownedPets).Error; err != nil {
		return GameResult{}, true, err
	}
	if ownedPets >= capacity {
		return GameResult{Title: "🐉 灵兽空间已满", Content: fmt.Sprintf("当前灵兽：%d/%d\n请先放生闲置灵兽，或提升大境界扩展御兽空间。本次没有扣除体力、口粮或遭遇次数。", ownedPets, capacity), Actions: []string{"灵兽", "放生 灵兽名", "状态", "突破"}}, true, nil
	}
	food, err := g.itemByName("灵兽口粮")
	if err != nil || g.itemQuantity(player.ID, food.ID) < 1 {
		return GameResult{Title: "🐉 缺少安抚口粮", Content: "尝试捕获需要灵兽口粮×1，当前不足。遭遇仍会保留至十分钟期限，不会提前扣除体力。\n━━━━━━━━━━━\n可从青云入道礼匣、普通妖兽、货铺或“百草灵兽膳”配方获得。", Actions: []string{"物品 灵兽口粮", "开启礼包 青云入道礼匣", "合成 百草灵兽膳", "货铺", "背包"}}, true, nil
	}
	currentStamina, err := g.currentStamina(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	if currentStamina < 5 {
		maximum, maximumErr := g.staminaMaximum(player.ID)
		if maximumErr != nil {
			return GameResult{}, true, maximumErr
		}
		return GameResult{Title: "🐉 体力不足", Content: fmt.Sprintf("捕获需要体力5，当前%d/%d。遭遇不会因体力不足而提前结算。", currentStamina, maximum), Actions: []string{"体力", "状态", "灵兽", "位置"}}, true, nil
	}
	if remaining := g.playerCooldownRemaining(player.ID, "cooldown.pet.capture"); remaining > 0 {
		return GameResult{Title: "🐉 御兽印尚未平复", Content: "还需" + formatDuration(remaining) + "才能再次尝试捕获；本次没有扣除资源，当前遭遇仍然保留。", Actions: []string{"灵兽", "状态"}}, true, nil
	}
	baseRate, rate, luckBonus := g.petCaptureRate(player, template)
	succeeded := rand.Float64() <= rate
	remainingStamina := currentStamina - 5
	pet := model.Pet{PlayerID: player.ID, Name: template.Name, Species: template.Name, Rarity: "凡品", Level: 1, Attack: template.InitialPower, Defense: template.InitialPower / 2, Health: template.InitialPower * 10, Loyalty: 50, Active: false, SkillJSON: `[]`}
	cooldownUntil := time.Now().Add(time.Duration(max64(g.settingInt("pet.capture_cooldown_seconds", 60), 1)) * time.Second)
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		var pendingCount int64
		if err := tx.Model(&model.PlayerValue{}).Where("player_id = ? AND key = ?", player.ID, pendingPetEncounterKey).Count(&pendingCount).Error; err != nil {
			return err
		}
		if pendingCount != 1 {
			return fmt.Errorf("灵兽遭遇已经失效")
		}
		var count int64
		if err := tx.Model(&model.Pet{}).Where("player_id = ?", player.ID).Count(&count).Error; err != nil {
			return err
		}
		if count >= capacity {
			return fmt.Errorf("灵兽空间已满")
		}
		if err := consumeNamedItemTx(tx, player.ID, "灵兽口粮", 1); err != nil {
			return err
		}
		if err := upsertPlayerValueTx(tx, player.ID, "stamina.value", strconv.FormatInt(remainingStamina, 10), nil); err != nil {
			return err
		}
		if err := tx.Where("player_id = ? AND key = ?", player.ID, pendingPetEncounterKey).Delete(&model.PlayerValue{}).Error; err != nil {
			return err
		}
		if err := upsertPlayerValueTx(tx, player.ID, "cooldown.pet.capture", cooldownUntil.Format(time.RFC3339Nano), &cooldownUntil); err != nil {
			return err
		}
		if !succeeded {
			return nil
		}
		if err := tx.Create(&pet).Error; err != nil {
			return err
		}
		return upsertPlayerValueTx(tx, player.ID, petCareKey(pet.ID), time.Now().Format(time.RFC3339Nano), nil)
	})
	if err != nil {
		return GameResult{Title: "🐉 捕获结算失败", Content: err.Error() + "。事务已经回滚，不会出现只扣材料却没有结果的情况。", Actions: []string{"灵兽", "背包", "状态"}}, true, nil
	}
	if !succeeded {
		return GameResult{Title: "🐉 灵兽挣脱御兽印", Content: fmt.Sprintf("%s吞下口粮后忽然识破御兽印，踏开山雾遁走。该次遭遇已经结束，必须重新探索。\n━━━━━━━━━━━\n捕获率：基础%.1f%% · 运气+%.1f%% · 实际%.1f%%\n消耗：灵兽口粮×1、体力5\n剩余体力：%d", template.Name, baseRate*100, luckBonus*100, rate*100, remainingStamina), Actions: []string{"探索", "物品 灵兽口粮", "灵兽", "位置"}}, true, nil
	}
	return GameResult{Title: "🐉 捕获灵兽", Content: fmt.Sprintf("你以口粮安抚兽性，再以神识闭合御兽印，成功收服 **%s**。\n━━━━━━━━━━━\n捕获率：基础%.1f%% · 运气+%.1f%% · 实际%.1f%%\n灵兽战力：%d\n攻击：%d · 防御：%d · 体魄：%d\n等级：1 · %s\n忠诚：50 · 灵兽空间：%d/%d\n消耗：灵兽口粮×1、体力5\n剩余体力：%d\n━━━━━━━━━━━\n忠诚会随未照料天数下降；焦躁、拒战、反噬与叛变均会真实结算。喂养可增加灵悟经验并重置照料周期。", pet.Name, baseRate*100, luckBonus*100, rate*100, petCombatPower(pet), pet.Attack, pet.Defense, pet.Health, petExperienceProgress(pet), ownedPets+1, capacity, remainingStamina), Actions: []string{"灵兽", "出战 " + pet.Name, "喂养 灵兽口粮", "状态"}}, true, nil
}

func (g *Game) viewPets(player *model.Player) (GameResult, bool, error) {
	events, err := g.reconcilePetCare(player)
	if err != nil {
		return GameResult{}, true, err
	}
	var pets []model.Pet
	if err := g.store.DB.Where("player_id = ?", player.ID).Order("active DESC, level DESC").Find(&pets).Error; err != nil {
		return GameResult{}, true, err
	}
	powerChanged := false
	for index := range pets {
		before := pets[index]
		g.applyPetExperience(&pets[index], 0)
		if pets[index].Level == before.Level && pets[index].Experience == before.Experience && pets[index].Attack == before.Attack && pets[index].Defense == before.Defense && pets[index].Health == before.Health {
			continue
		}
		if err := g.store.DB.Model(&model.Pet{}).Where("id = ?", pets[index].ID).Updates(map[string]any{
			"level": pets[index].Level, "experience": pets[index].Experience, "attack": pets[index].Attack,
			"defense": pets[index].Defense, "health": pets[index].Health,
		}).Error; err != nil {
			return GameResult{}, true, err
		}
		powerChanged = powerChanged || pets[index].Active
	}
	if powerChanged {
		if latest, loadErr := g.players.Get(player.ID); loadErr == nil {
			_ = g.syncPlayerCombatPower(&latest)
		}
	}
	if len(pets) == 0 {
		content := "你尚未收服灵兽。先在地图探索中发现灵兽踪迹，再发送捕获。"
		if len(events) > 0 {
			content = "【照料事件】\n" + strings.Join(events, "\n") + "\n━━━━━━━━━━━\n" + content
		}
		return GameResult{Title: "🐉 灵兽空间", Content: content, Actions: []string{"探索", "捕获", "物品 灵兽口粮"}}, true, nil
	}
	lines := make([]string, 0, len(pets)+4)
	if len(events) > 0 {
		lines = append(lines, "【照料事件】", strings.Join(events, "\n"), "━━━━━━━━━━━")
	}
	lines = append(lines, fmt.Sprintf("灵兽空间：%d/%d · 每日未照料会降低忠诚", len(pets), g.petCapacity(player)), "━━━━━━━━━━━")
	actions := []string{"喂养 灵兽口粮"}
	for _, pet := range pets {
		mark := ""
		if pet.Active {
			mark = "【出战】"
		}
		lines = append(lines, fmt.Sprintf("- %s%s · %d级 · 忠诚%d · 灵兽战力%d\n  %s\n  攻击%d · 防御%d · 体魄%d", mark, pet.Name, pet.Level, pet.Loyalty, petCombatPower(pet), petExperienceProgress(pet), pet.Attack, pet.Defense, pet.Health))
		if !pet.Active {
			actions = append(actions, "出战 "+pet.Name)
		}
	}
	return GameResult{Title: "🐉 灵兽空间", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) activeOrNamedPet(playerID uint, name string) (model.Pet, error) {
	var pet model.Pet
	db := g.store.DB.Where("player_id = ?", playerID)
	if strings.TrimSpace(name) == "" {
		db = db.Where("active = ?", true)
	} else {
		db = db.Where("name = ?", strings.TrimSpace(name))
	}
	err := db.Order("active DESC, id").First(&pet).Error
	return pet, err
}

func (g *Game) activatePet(player *model.Player, argument string) (GameResult, bool, error) {
	events, err := g.reconcilePetCare(player)
	if err != nil {
		return GameResult{}, true, err
	}
	pet, err := g.activeOrNamedPet(player.ID, argument)
	if err != nil {
		return GameResult{Title: "出战失败", Content: "请输入：`出战 灵兽名`"}, true, nil
	}
	if pet.Loyalty < 30 {
		return GameResult{Title: "🐉 忠诚不足，拒绝出战", Content: fmt.Sprintf("%s当前忠诚仅%d，已拒绝响应御兽印。请先使用灵兽口粮照料，忠诚恢复到30以上后才能出战。\n%s", pet.Name, pet.Loyalty, strings.Join(events, "\n")), Actions: []string{"喂养 灵兽口粮", "灵兽", "物品 灵兽口粮"}}, true, nil
	}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Pet{}).Where("player_id = ?", player.ID).Update("active", false).Error; err != nil {
			return err
		}
		if err := tx.Model(&pet).Update("active", true).Error; err != nil {
			return err
		}
		return tx.Model(player).Update("active_pet_id", pet.ID).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	eventText := ""
	if len(events) > 0 {
		eventText = "\n━━━━━━━━━━━\n照料事件：\n" + strings.Join(events, "\n")
	}
	return GameResult{Title: "🐉 灵兽出战", Content: fmt.Sprintf("%s已进入出战位。\n灵兽战力：%d\n角色总战力加成：+%d\n%s\n━━━━━━━━━━━\n状态、灵兽空间和灵兽排行榜均使用这一战力口径。%s", pet.Name, petCombatPower(pet), petCombatPower(pet), petExperienceProgress(pet), eventText), Actions: []string{"状态", "灵兽", "排行 灵兽", "喂养 灵兽口粮"}}, true, nil
}

func (g *Game) feedPet(player *model.Player, argument string) (GameResult, bool, error) {
	events, err := g.reconcilePetCare(player)
	if err != nil {
		return GameResult{}, true, err
	}
	pet, err := g.activeOrNamedPet(player.ID, "")
	if err != nil {
		err = g.store.DB.Where("player_id = ?", player.ID).Order("loyalty ASC, id").First(&pet).Error
		if err != nil {
			return GameResult{Title: "🐉 喂养失败", Content: "灵兽空间中没有可照料的灵兽。请先探索并捕获。", Actions: []string{"探索", "捕获"}}, true, nil
		}
	}
	itemName := strings.TrimSpace(argument)
	if itemName == "" {
		itemName = "灵兽口粮"
	}
	item, err := g.itemByName(itemName)
	if err != nil || item.EffectFunc != "pet_loyalty" {
		return GameResult{Title: "🐉 这不是灵兽食粮", Content: "灵兽只接受登记为御兽口粮的物品。当前可用：灵兽口粮；可通过货铺、妖兽掉落或百草灵兽膳配方获得。", Actions: []string{"物品 灵兽口粮", "合成 百草灵兽膳", "货铺"}}, true, nil
	}
	if g.itemQuantity(player.ID, item.ID) < 1 {
		return GameResult{Title: "🐉 喂养失败", Content: "背包中没有" + itemName + "。", Actions: []string{"物品 " + itemName, "货铺", "合成 百草灵兽膳"}}, true, nil
	}
	gain := max64(int64(item.EffectValue), 10)
	beforeLoyalty := pet.Loyalty
	beforeLevel := pet.Level
	beforePower := petCombatPower(pet)
	pet.Loyalty = minInt(pet.Loyalty+int(gain), 100)
	levelsGained := g.applyPetExperience(&pet, gain)
	careTime := time.Now()
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := consumeNamedItemTx(tx, player.ID, item.Name, 1); err != nil {
			return err
		}
		if err := tx.Model(&model.Pet{}).Where("id = ?", pet.ID).Updates(map[string]any{
			"loyalty": pet.Loyalty, "level": pet.Level, "experience": pet.Experience,
			"attack": pet.Attack, "defense": pet.Defense, "health": pet.Health,
		}).Error; err != nil {
			return err
		}
		return upsertPlayerValueTx(tx, player.ID, petCareKey(pet.ID), careTime.Format(time.RFC3339Nano), nil)
	})
	if err != nil {
		return GameResult{Title: "🐉 喂养结算失败", Content: "喂养事务未能完成，口粮、忠诚与经验均未改变，请稍后重试。", Actions: []string{"灵兽", "背包"}}, true, nil
	}
	if pet.Active && levelsGained > 0 {
		if latest, loadErr := g.players.Get(player.ID); loadErr == nil {
			_ = g.syncPlayerCombatPower(&latest)
		}
	}
	eventText := ""
	if len(events) > 0 {
		eventText = "\n此前结算：\n" + strings.Join(events, "\n")
	}
	levelText := fmt.Sprintf("等级：%d", pet.Level)
	attributeText := ""
	if levelsGained > 0 {
		levelText = fmt.Sprintf("等级：%d → %d（提升%d级）", beforeLevel, pet.Level, levelsGained)
		attributeText = fmt.Sprintf("\n升级成长：攻击%d · 防御%d · 体魄%d\n灵兽战力：%d → %d（%+d）", pet.Attack, pet.Defense, pet.Health, beforePower, petCombatPower(pet), petCombatPower(pet)-beforePower)
	}
	return GameResult{Title: "🐉 喂养灵兽", Content: fmt.Sprintf("%s吃下%s，躁动的灵息逐渐平复。\n忠诚：%d → %d\n灵悟经验：+%d\n%s\n%s%s\n照料周期：已从现在重新计算%s", pet.Name, itemName, beforeLoyalty, pet.Loyalty, gain, levelText, petExperienceProgress(pet), attributeText, eventText), Actions: []string{"灵兽", "进化", "状态", "物品 灵兽口粮"}}, true, nil
}

func (g *Game) evolvePet(player *model.Player) (GameResult, bool, error) {
	if events, err := g.reconcilePetCare(player); err != nil {
		return GameResult{}, true, err
	} else if len(events) > 0 {
		if latest, loadErr := g.players.Get(player.ID); loadErr == nil {
			*player = latest
		}
	}
	pet, err := g.activeOrNamedPet(player.ID, "")
	if err != nil {
		return GameResult{Title: "进化失败", Content: "没有出战灵兽。"}, true, nil
	}
	var template model.PetTemplate
	if err := g.store.DB.Where("enabled = ? AND (name = ? OR code = ?)", true, pet.Species, pet.Species).Order("id").First(&template).Error; err != nil {
		return GameResult{Title: "进化谱系缺失", Content: fmt.Sprintf("%s对应的万兽谱模板不存在，本次没有改变任何属性。", pet.Name), Actions: []string{"灵兽详情 " + pet.Species, "灵兽"}}, true, nil
	}
	requirement := model.PetEvolutionRequirementFor(template)
	if pet.Evolution >= 1 {
		return GameResult{Title: "灵兽已经进化", Content: fmt.Sprintf("%s已经完成唯一一次血脉进化，不能重复叠加属性。\n当前进化次数：1\n%s", pet.Name, petExperienceProgress(pet)), Actions: []string{"灵兽", "状态", "排行 灵兽"}}, true, nil
	}
	if pet.Loyalty < requirement.Loyalty || pet.Level < requirement.Level {
		return GameResult{Title: "进化条件不足", Content: fmt.Sprintf("谱系：%s → %s\n需要忠诚≥%d且等级≥%d。\n当前：忠诚%d，等级%d\n%s", template.Name, displayOr(template.EvolutionTarget, "灵品血脉"), requirement.Loyalty, requirement.Level, pet.Loyalty, pet.Level, petExperienceProgress(pet)), Actions: []string{"喂养", "灵兽", "灵兽详情 " + template.Name}}, true, nil
	}
	beforePower := petCombatPower(pet)
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		var current model.Pet
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND player_id = ? AND active = ?", pet.ID, player.ID, true).First(&current).Error; err != nil {
			return err
		}
		if current.Evolution >= 1 {
			return errPetAlreadyEvolved
		}
		if current.Loyalty < requirement.Loyalty || current.Level < requirement.Level {
			return errPetEvolutionRequirementsChanged
		}
		attack, defense, health := model.PetStatsAtLevel(template, current.Level, true)
		if value, bonusErr := playerValueTx(tx, player.ID, fmt.Sprintf("custom.pet.%d.bonus", current.ID)); bonusErr == nil {
			var bonus equipmentStats
			if json.Unmarshal([]byte(value), &bonus) == nil {
				attack += bonus.Attack
				defense += bonus.Defense
				health += bonus.Health
			}
		}
		updated := tx.Model(&model.Pet{}).Where("id = ? AND evolution = ?", current.ID, 0).Updates(map[string]any{
			"evolution": 1, "attack": attack, "defense": defense, "health": health, "rarity": "灵品",
		})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return errPetAlreadyEvolved
		}
		return nil
	})
	if errors.Is(err, errPetAlreadyEvolved) {
		return GameResult{Title: "灵兽已经进化", Content: "该灵兽刚刚完成进化，不能再次叠加属性。", Actions: []string{"灵兽", "状态"}}, true, nil
	}
	if errors.Is(err, errPetEvolutionRequirementsChanged) {
		return GameResult{Title: "进化条件已变化", Content: "忠诚或等级在结算前发生变化，本次没有改变属性，请重新查看灵兽状态。", Actions: []string{"灵兽", "喂养"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	_ = g.store.DB.First(&pet, pet.ID).Error
	if latest, loadErr := g.players.Get(player.ID); loadErr == nil {
		_ = g.syncPlayerCombatPower(&latest)
	}
	return GameResult{Title: "灵兽进化", Content: fmt.Sprintf("%s血脉觉醒，显化%s道相。\n进化次数：1/1\n品质提升为灵品\n属性按万兽谱、当前等级与一次进化重新结算。\n%s\n灵兽战力：%d → %d（%+d）", pet.Name, displayOr(template.EvolutionTarget, "灵品血脉"), petExperienceProgress(pet), beforePower, petCombatPower(pet), petCombatPower(pet)-beforePower), Actions: []string{"灵兽", "状态", "排行 灵兽"}}, true, nil
}

var (
	errPetAlreadyEvolved               = errors.New("pet already evolved")
	errPetEvolutionRequirementsChanged = errors.New("pet evolution requirements changed")
)

func petExperienceRequired(level int) int64 {
	if level < 1 {
		level = 1
	}
	return 100 + int64(level-1)*50
}

func petExperienceProgress(pet model.Pet) string {
	required := petExperienceRequired(pet.Level)
	experience := min64(max64(pet.Experience, 0), required)
	filled := int(experience * 10 / required)
	percent := int(experience * 100 / required)
	return fmt.Sprintf("灵悟 [%s%s] %d%% · %d/%d", strings.Repeat("█", filled), strings.Repeat("░", 10-filled), percent, experience, required)
}

func (g *Game) petGrowthPerLevel(pet model.Pet) int64 {
	var template model.PetTemplate
	if err := g.store.DB.Where("name = ? OR code = ?", pet.Species, pet.Species).Order("id").First(&template).Error; err == nil && template.GrowthPerLevel > 0 {
		return template.GrowthPerLevel
	}
	return max64(pet.Attack/20, 1)
}

func (g *Game) applyPetExperience(pet *model.Pet, gain int64) int {
	if pet.Level < 1 {
		pet.Level = 1
	}
	pet.Experience = max64(pet.Experience, 0) + max64(gain, 0)
	growth := g.petGrowthPerLevel(*pet)
	levels := 0
	for pet.Experience >= petExperienceRequired(pet.Level) {
		pet.Experience -= petExperienceRequired(pet.Level)
		pet.Level++
		pet.Attack += growth
		pet.Defense += max64(growth/2, 1)
		pet.Health += growth * 10
		levels++
	}
	return levels
}

// petCombatPower is the single power formula used by capture messages, pet
// lists, character power and leaderboards. Pet rows store attack=P,
// defense=P/2 and health=P*10 at capture time; the normalized formula returns
// P and scales naturally when evolution changes all three attributes.
func petCombatPower(pet model.Pet) int64 {
	return max64((pet.Attack*2+pet.Defense*2+pet.Health/10)/4, 1)
}

func (g *Game) releasePet(player *model.Player, argument string) (GameResult, bool, error) {
	pet, err := g.activeOrNamedPet(player.ID, argument)
	if err != nil {
		return GameResult{Title: "放生失败", Content: "请输入：`放生 灵兽名`；留空则放生当前出战灵兽。"}, true, nil
	}
	merit := int64(5 + pet.Level*2)
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("player_id = ? AND key = ?", player.ID, petCareKey(pet.ID)).Delete(&model.PlayerValue{}).Error; err != nil {
			return err
		}
		if err := tx.Where("player_id = ? AND key = ?", player.ID, fmt.Sprintf("custom.pet.%d.bonus", pet.ID)).Delete(&model.PlayerValue{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&pet).Error; err != nil {
			return err
		}
		updates := map[string]any{"merit": gorm.Expr("merit + ?", merit)}
		if player.ActivePetID == pet.ID {
			updates["active_pet_id"] = 0
		}
		return tx.Model(player).Updates(updates).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "放归山林", Content: fmt.Sprintf("%s回望一眼，消失在林海。\n功德：+%d", pet.Name, merit)}, true, nil
}

func (g *Game) reconcilePetCare(player *model.Player) ([]string, error) {
	var pets []model.Pet
	if err := g.store.DB.Where("player_id = ?", player.ID).Find(&pets).Error; err != nil {
		return nil, err
	}
	now := time.Now()
	events := make([]string, 0)
	activeChanged := false
	for _, pet := range pets {
		last := pet.UpdatedAt
		if value, err := g.playerValue(player.ID, petCareKey(pet.ID)); err == nil {
			if parsed, parseErr := time.Parse(time.RFC3339Nano, value); parseErr == nil {
				last = parsed
			}
		}
		if last.IsZero() {
			last = now
		}
		days := int(now.Sub(last) / (24 * time.Hour))
		if days < 1 {
			if _, err := g.playerValue(player.ID, petCareKey(pet.ID)); err != nil {
				_ = g.setPlayerValue(player.ID, petCareKey(pet.ID), now.Format(time.RFC3339Nano), nil)
			}
			continue
		}
		decayPerDay := 2
		var template model.PetTemplate
		if g.store.DB.Where("name = ?", pet.Species).First(&template).Error == nil {
			decayPerDay = maxInt(template.LoyaltyDecay, 1)
		}
		before := pet.Loyalty
		after := maxInt(before-days*decayPerDay, 0)
		careKey := petCareKey(pet.ID)
		if after == 0 {
			err := g.store.DB.Transaction(func(tx *gorm.DB) error {
				if err := tx.Where("player_id = ? AND key = ?", player.ID, careKey).Delete(&model.PlayerValue{}).Error; err != nil {
					return err
				}
				if err := tx.Delete(&pet).Error; err != nil {
					return err
				}
				if pet.Active || player.ActivePetID == pet.ID {
					return tx.Model(&model.Player{}).Where("id = ?", player.ID).Update("active_pet_id", 0).Error
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
			activeChanged = activeChanged || pet.Active || player.ActivePetID == pet.ID
			events = append(events, fmt.Sprintf("【叛变】%s因连续%d日无人照料，忠诚%d→0，挣脱御兽契离去。", pet.Name, days, before))
			broadcast := fmt.Sprintf("【灵兽叛变】%s疏于照料，%s忠诚耗尽后挣脱御兽契，重归山野。", player.DaoName, pet.Name)
			_ = g.publishWorldBroadcast("灵兽", pet.Name+"叛变离去", broadcast)
			continue
		}
		updates := map[string]any{"loyalty": after}
		if after < 30 && pet.Active {
			updates["active"] = false
			activeChanged = true
		}
		err := g.store.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&model.Pet{}).Where("id = ?", pet.ID).Updates(updates).Error; err != nil {
				return err
			}
			if after < 30 && (pet.Active || player.ActivePetID == pet.ID) {
				if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Update("active_pet_id", 0).Error; err != nil {
					return err
				}
			}
			return upsertPlayerValueTx(tx, player.ID, careKey, now.Format(time.RFC3339Nano), nil)
		})
		if err != nil {
			return nil, err
		}
		switch {
		case after < 15:
			damage := max64(g.playerWithActiveSkillStats(player).MaxHealth/10, 1)
			if err := g.players.UpdateColumn(player.ID, "health", gorm.Expr("MAX(health - ?, 1)", damage)); err != nil {
				return nil, err
			}
			events = append(events, fmt.Sprintf("【反噬】%s连续%d日未被照料，忠诚%d→%d，挣脱时反噬主人气血%d并拒绝出战。", pet.Name, days, before, after, damage))
		case after < 30:
			events = append(events, fmt.Sprintf("【拒战】%s连续%d日未被照料，忠诚%d→%d，已自行退出战位。", pet.Name, days, before, after))
		case after < 50:
			events = append(events, fmt.Sprintf("【焦躁】%s连续%d日未被照料，忠诚%d→%d，开始抗拒御兽指令。", pet.Name, days, before, after))
		default:
			events = append(events, fmt.Sprintf("【思食】%s已有%d日未被照料，忠诚%d→%d，请尽快喂养。", pet.Name, days, before, after))
		}
	}
	if activeChanged {
		if latest, err := g.players.Get(player.ID); err == nil {
			_ = g.syncPlayerCombatPower(&latest)
			*player = latest
		}
	}
	return events, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var _ = errors.Is
