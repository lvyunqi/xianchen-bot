package service

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/handler"
	"xianlv/internal/model"
)

func (g *Game) executeCultivation(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 13:
		return g.startCultivation(player)
	case 14:
		return g.quickCultivation(player)
	case 15:
		return g.breakthrough(player)
	case 16:
		return g.meditate(player)
	case 17:
		return g.comprehend(player)
	case 18:
		return g.recite(player)
	case 19:
		return g.mansionPractice(player)
	case 20:
		return g.improveSkill(player)
	case 21:
		return g.cultivationRecord(player), true, nil
	case 22:
		return g.finishCultivation(player)
	default:
		return GameResult{}, false, nil
	}
}

func (g *Game) startCultivation(player *model.Player) (GameResult, bool, error) {
	if player.State == model.PlayerStateCultivating && player.CultivationStartedAt != nil {
		elapsed := time.Since(*player.CultivationStartedAt)
		minimum := time.Duration(g.settingInt("cultivation.minimum_minutes", 5)) * time.Minute
		content := "你正在闭关，已持续" + formatDuration(elapsed) + "。"
		if elapsed < minimum {
			content += "\n最少还需" + formatDuration(minimum-elapsed) + "才能出关结算。"
		} else {
			content += "\n现在可以发送 `出关` 结算修为。"
		}
		return GameResult{Title: "闭关中", Content: content, Actions: []string{"出关", "修记"}}, true, nil
	}
	now := time.Now()
	err := g.store.DB.Model(player).Updates(map[string]any{"state": model.PlayerStateCultivating, "cultivation_started_at": &now}).Error
	if err != nil {
		return GameResult{}, true, err
	}
	minimum := g.settingInt("cultivation.minimum_minutes", 5)
	maximum := g.settingInt("cultivation.maximum_minutes", 480)
	return GameResult{Title: "闭关入定", Content: fmt.Sprintf("你已封闭六识，开始运转周天。\n最短闭关：%d分钟\n最长收益：%d分钟\n达到时间后发送 `出关` 结算。", minimum, maximum), Actions: []string{"修记", "出关"}}, true, nil
}

func (g *Game) finishCultivation(player *model.Player) (GameResult, bool, error) {
	if player.State != model.PlayerStateCultivating || player.CultivationStartedAt == nil {
		return GameResult{Title: "尚未闭关", Content: "先发送 `修炼` 开始闭关。", Actions: []string{"修炼"}}, true, nil
	}
	elapsed := time.Since(*player.CultivationStartedAt)
	minimum := time.Duration(g.settingInt("cultivation.minimum_minutes", 5)) * time.Minute
	if elapsed < minimum {
		return GameResult{Title: "道基未稳", Content: "现在强行出关不会获得收益。\n还需闭关：" + formatDuration(minimum-elapsed), Actions: []string{"修记"}}, true, nil
	}
	maximum := time.Duration(g.settingInt("cultivation.maximum_minutes", 480)) * time.Minute
	if elapsed > maximum {
		elapsed = maximum
	}
	minutes := int64(elapsed / time.Minute)
	base := g.settingInt("cultivation.base_reward", 20)
	aptitudeFactor := g.settingFloat("cultivation.aptitude_factor", .05)
	rootProfile := g.spiritualRootBonuses(player.SpiritualRoot, player.RootQuality)
	rootBonus := g.settingFloat("cultivation.root_bonus", 1.3) * rootProfile.Cultivation
	aptitude := 1 + (float64(player.RootQuality)/100)*aptitudeFactor
	coupleBonus := 1.0
	if player.CoupleID != 0 {
		coupleBonus = g.settingFloat("cultivation.couple_bonus", 1.3)
	}
	medicineBonus := g.activeItemBonuses(player.ID).CultivationMultiplier
	titleMultiplier := 1 + g.activeTitleGameplayPercent(player, "cultivation_percent")/100
	reward := int64(float64(base*minutes) * aptitude * rootBonus * coupleBonus * medicineBonus * titleMultiplier)
	if reward < 1 {
		reward = 1
	}
	err := g.store.DB.Model(player).Updates(map[string]any{
		"cultivation":            gorm.Expr("cultivation + ?", reward),
		"state":                  model.PlayerStateIdle,
		"cultivation_started_at": nil,
	}).Error
	if err != nil {
		return GameResult{}, true, err
	}
	_, _ = g.addPlayerValueInt(player.ID, "stats.cultivation_minutes", minutes)
	_, _ = g.addPlayerValueInt(player.ID, "stats.cultivation_gain", reward)
	return GameResult{Title: "出关", Content: fmt.Sprintf("周天运转完毕，道基更加凝实。\n闭关时长：%s\n基础修为：%d\n灵根：%s · 纯度%d（额外+%s）\n仙侣加成：×%.2f\n丹药加成：×%.2f\n称号加成：×%.2f\n最终获得修为：+%d\n当前修为：%d/%d", formatDuration(elapsed), base*minutes, player.SpiritualRoot, player.RootQuality, rootProfile.CultivationDisplay, coupleBonus, medicineBonus, titleMultiplier, reward, player.Cultivation+reward, player.CultivationRequired), Actions: []string{"突破", "状态", "灵检", "修炼"}}, true, nil
}

func (g *Game) quickCultivation(player *model.Player) (GameResult, bool, error) {
	item, err := g.itemByName("灵果")
	if err != nil {
		return GameResult{}, true, err
	}
	if g.itemQuantity(player.ID, item.ID) < 1 {
		return GameResult{Title: "速修失败", Content: "乾坤袋中没有灵果。", Actions: []string{"探索", "集市"}}, true, nil
	}
	if err := g.players.AdjustItem(player.ID, item.ID, -1); err != nil {
		return GameResult{}, true, err
	}
	reward := int64(item.EffectValue)
	if reward <= 0 {
		reward = 10
	}
	if err := g.store.DB.Model(player).Update("cultivation", gorm.Expr("cultivation + ?", reward)).Error; err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "灵果速修", Content: fmt.Sprintf("灵果化作精纯灵气。\n消耗：灵果×1\n修为：+%d\n当前：%d/%d", reward, player.Cultivation+reward, player.CultivationRequired), Actions: []string{"速修", "突破"}}, true, nil
}

func (g *Game) breakthrough(player *model.Player) (GameResult, bool, error) {
	effective := g.playerWithActiveSkillStats(player)
	skillBonus := g.activeSkillStatBonus(player)
	var current model.Realm
	if err := g.store.DB.First(&current, player.RealmID).Error; err != nil {
		return GameResult{}, true, err
	}
	var next model.Realm
	err := g.store.DB.Where("sequence > ?", current.Sequence).Order("sequence").First(&next).Error
	if errors.Is(err, gorm.ErrRecordNotFound) && player.RealmLevel >= realmStageCount {
		return GameResult{Title: "大道之巅", Content: "你已达到当前世界的最高境界。", Actions: []string{"飞升", "转世"}}, true, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return GameResult{}, true, err
	}
	cost := realmStageCost(current, next)
	if player.Cultivation < cost {
		target := fmt.Sprintf("%s·%d层", current.Name, minInt(player.RealmLevel+1, realmStageCount))
		if player.RealmLevel >= realmStageCount && next.ID != 0 {
			target = next.Name + "·1层"
		}
		return GameResult{Title: "突破条件不足", Content: fmt.Sprintf("目标：%s\n本次需要修为：%d\n当前可用修为：%d\n还差：%d\n每次突破只提升一层，十层圆满后方可跨越大境。", target, cost, player.Cultivation, cost-player.Cultivation), Actions: []string{"修炼", "速修", "修记"}}, true, nil
	}
	if player.RealmLevel < realmStageCount {
		medicineBonus := g.activeItemBonuses(player.ID)
		effectiveDaoHeart := player.DaoHeart + medicineBonus.DaoHeart
		materialName := breakthroughMaterialForLevel(player.RealmLevel)
		material, materialErr := g.itemByName(materialName)
		if materialErr != nil {
			return GameResult{}, true, materialErr
		}
		materialOwned := g.itemQuantity(player.ID, material.ID)
		if materialOwned < 1 {
			return GameResult{Title: "缺少破境前置", Content: fmt.Sprintf("冲击：%s·%d层 → %s·%d层\n需要：%s×1\n当前持有：%d\n━━━━━━━━━━━\n每次突破尝试都会消耗一枚对应丹药，无论成功或失败。低层使用淬脉丹，中层使用凝元丹，高层使用破境丹。", current.Name, player.RealmLevel, current.Name, player.RealmLevel+1, materialName, materialOwned), Actions: []string{"合成 " + materialName, "合成图鉴", "物品 " + materialName, "背包"}}, true, nil
		}
		minimumDaoHeart := int64(20 + player.RealmLevel*3)
		if effectiveDaoHeart < minimumDaoHeart || effective.Health*100 < effective.MaxHealth*70 || effective.Mana*100 < effective.MaxMana*50 {
			return GameResult{Title: "突破准备不足", Content: fmt.Sprintf("冲击：%s·%d层 → %s·%d层\n━━━━━━━━━━━\n道心：%d/%d（本体%d · 药力+%d）\n气血：%d/%d（至少70%%）\n法力：%d/%d（至少50%%）\n修为：%d/%d\n前置丹药：%s×1（持有%d）\n━━━━━━━━━━━\n突破会经历真实失败判定；请先疗伤、恢复法力并稳固道心。", current.Name, player.RealmLevel, current.Name, player.RealmLevel+1, effectiveDaoHeart, minimumDaoHeart, player.DaoHeart, medicineBonus.DaoHeart, effective.Health, effective.MaxHealth, effective.Mana, effective.MaxMana, player.Cultivation, cost, materialName, materialOwned), Actions: []string{"疗伤", "冥想", "道心", "状态"}}, true, nil
		}
		rate := .72 + float64(player.Perception)/1000 + float64(effectiveDaoHeart)/1000 + float64(player.RootQuality)/2000 + medicineBonus.BreakthroughRate
		if rate > .95 {
			rate = .95
		}
		if rand.Float64() > rate {
			loss := max64(cost*15/100, 1)
			health := max64(effective.Health-effective.MaxHealth/10, 1)
			mana := max64(effective.Mana-effective.MaxMana/5, 0)
			if err := g.store.DB.Transaction(func(tx *gorm.DB) error {
				if err := consumeNamedItemTx(tx, player.ID, materialName, 1); err != nil {
					return err
				}
				return tx.Model(player).Updates(map[string]any{"cultivation": gorm.Expr("MAX(cultivation - ?, 0)", loss), "health": health, "mana": mana}).Error
			}); err != nil {
				return GameResult{}, true, err
			}
			return GameResult{Title: "小境突破失败", Content: fmt.Sprintf("灵力冲击壁垒时周天失衡，经脉受到反噬。\n目标：%s·%d层\n本次成功率：%.1f%%\n消耗前置：%s×1\n损失修为：%d\n气血：%d/%d\n法力：%d/%d\n境界仍为：%s·%d层\n━━━━━━━━━━━\n失败不会倒退境界，但再次突破前需要恢复状态。", current.Name, player.RealmLevel+1, rate*100, materialName, loss, health, effective.MaxHealth, mana, effective.MaxMana, current.Name, player.RealmLevel), Actions: []string{"疗伤", "修炼", "合成 " + materialName, "状态", "突破"}}, true, nil
		}
		newLevel := player.RealmLevel + 1
		healthGain := max64((next.BaseHealth-current.BaseHealth)/realmStageCount, 2)
		manaGain := max64((next.BaseMana-current.BaseMana)/realmStageCount, 1)
		attackGain := max64((next.BaseAttack-current.BaseAttack)/realmStageCount, 1)
		defenseGain := max64((next.BaseDefense-current.BaseDefense)/realmStageCount, 1)
		if next.ID == 0 {
			healthGain, manaGain, attackGain, defenseGain = 10, 5, 2, 2
		}
		updates := map[string]any{
			"realm_level": newLevel, "cultivation": gorm.Expr("cultivation - ?", cost), "cultivation_required": cost,
			"max_health": gorm.Expr("max_health + ?", healthGain), "health": gorm.Expr("MIN(health + ?, max_health + ? + ?)", healthGain, healthGain, skillBonus.Health),
			"max_mana": gorm.Expr("max_mana + ?", manaGain), "mana": gorm.Expr("MIN(mana + ?, max_mana + ? + ?)", manaGain, manaGain, skillBonus.Mana),
			"physical_attack": gorm.Expr("physical_attack + ?", attackGain), "magic_attack": gorm.Expr("magic_attack + ?", attackGain),
			"physical_defense": gorm.Expr("physical_defense + ?", defenseGain), "magic_defense": gorm.Expr("magic_defense + ?", defenseGain),
		}
		preview := *player
		preview.PhysicalAttack += attackGain
		preview.MagicAttack += attackGain
		preview.PhysicalDefense += defenseGain
		preview.MagicDefense += defenseGain
		preview.MaxHealth += healthGain
		updates["combat_power"] = calculateCombatPower(preview)
		if err := g.store.DB.Transaction(func(tx *gorm.DB) error {
			if err := consumeNamedItemTx(tx, player.ID, materialName, 1); err != nil {
				return err
			}
			return tx.Model(player).Updates(updates).Error
		}); err != nil {
			return GameResult{}, true, err
		}
		_, _ = g.addPlayerValueInt(player.ID, "stats.breakthroughs", 1)
		status := "继续积累修为，可冲击下一层。"
		if newLevel == realmStageCount {
			status = "小境圆满，下一次突破将冲击“" + next.Name + "”。"
		}
		return GameResult{Title: "小境突破", Content: fmt.Sprintf("道友运转周天，冲开本境第%d重壁垒。\n境界：%s·%d层 → **%s·%d层**\n成功率：%.1f%%\n消耗前置：%s×1\n消耗修为：%d\n气血上限：+%d\n法力上限：+%d\n攻法：+%d\n防法：+%d\n%s", newLevel, current.Name, player.RealmLevel, current.Name, newLevel, rate*100, materialName, cost, healthGain, manaGain, attackGain, defenseGain, status), Actions: []string{"状态", "修炼", "突破"}}, true, nil
	}

	checklist, _, checklistErr := g.tribulationChecklist(player)
	if checklistErr != nil {
		return GameResult{}, true, checklistErr
	}
	checklist.Title = "十层圆满·待渡天劫"
	checklist.Content = fmt.Sprintf("%s已达十层圆满，普通突破不能直接跨入%s。\n必须完成备劫并连续通过三道劫关。\n━━━━━━━━━━━\n%s", current.Name, next.Name, checklist.Content)
	return checklist, true, nil
}

const realmStageCount = 10

func breakthroughMaterialForLevel(currentLevel int) string {
	switch {
	case currentLevel <= 3:
		return "淬脉丹"
	case currentLevel <= 6:
		return "凝元丹"
	default:
		return "破境丹"
	}
}

func realmStageCost(current, next model.Realm) int64 {
	if next.ID != 0 {
		return max64((next.RequiredCultivation-current.RequiredCultivation+realmStageCount-1)/realmStageCount, 1)
	}
	return max64(current.RequiredCultivation/realmStageCount, 100)
}

func (g *Game) meditate(player *model.Player) (GameResult, bool, error) {
	remaining, ok, err := g.cooldown(player.ID, "meditate", 5*time.Minute)
	if err != nil {
		return GameResult{}, true, err
	}
	if !ok {
		return GameResult{Title: "冥想冷却", Content: "神识尚未平复，还需" + formatDuration(remaining) + "。"}, true, nil
	}
	if err := g.store.DB.Model(player).Update("spirit", gorm.Expr("spirit + 1")).Error; err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "冥想静坐", Content: fmt.Sprintf("心神沉入识海，神识更加凝练。\n神识：%d → %d", player.Spirit, player.Spirit+1)}, true, nil
}

func (g *Game) comprehend(player *model.Player) (GameResult, bool, error) {
	if err := g.adjustNamedItem(player.ID, "灵茶", -1); err != nil {
		return GameResult{Title: "悟道失败", Content: "需要灵茶×1。", Actions: []string{"探索", "集市"}}, true, nil
	}
	if randomPercent() <= 35 {
		if err := g.store.DB.Model(player).Updates(map[string]any{"root_quality": gorm.Expr("MIN(root_quality + 1, 100)"), "perception": gorm.Expr("perception + 1")}).Error; err != nil {
			return GameResult{}, true, err
		}
		return GameResult{Title: "悟道有成", Content: "茶香入道，灵台一片清明。\n资质：+1\n悟性：+1"}, true, nil
	}
	return GameResult{Title: "悟道", Content: "你观云卷云舒，虽未顿悟，心境却平和了许多。"}, true, nil
}

func (g *Game) recite(player *model.Player) (GameResult, bool, error) {
	if err := g.adjustNamedItem(player.ID, "灵茶", -1); err != nil {
		return GameResult{Title: "诵经失败", Content: "需要灵茶×1。"}, true, nil
	}
	err := g.store.DB.Model(player).Updates(map[string]any{"perception": gorm.Expr("perception + 2"), "cultivation": gorm.Expr("cultivation + 10")}).Error
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "诵读经书", Content: "经义在识海流转。\n悟性：+2\n修为：+10"}, true, nil
}

func (g *Game) mansionPractice(player *model.Player) (GameResult, bool, error) {
	var mansion model.Mansion
	if err := g.store.DB.Where("player_id = ?", player.ID).First(&mansion).Error; err != nil {
		return GameResult{Title: "无处练功", Content: "先发送 `仙府` 建立自己的洞府。", Actions: []string{"仙府"}}, true, nil
	}
	remaining, ok, err := g.cooldown(player.ID, "mansion_practice", 10*time.Minute)
	if err != nil {
		return GameResult{}, true, err
	}
	if !ok {
		return GameResult{Title: "灵气未复", Content: "仙府灵气还需" + formatDuration(remaining) + "恢复。"}, true, nil
	}
	reward := int64(float64(g.settingInt("cultivation.base_reward", 20)) * 1.2 * float64(maxInt(mansion.Level, 1)))
	if err := g.store.DB.Model(player).Update("cultivation", gorm.Expr("cultivation + ?", reward)).Error; err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "仙府练功", Content: fmt.Sprintf("借仙府灵脉运功，收益提升20%%。\n获得修为：+%d", reward), Actions: []string{"仙府", "修炼"}}, true, nil
}

func (g *Game) improveSkill(player *model.Player) (GameResult, bool, error) {
	if player.CurrentSkillID == 0 {
		return GameResult{Title: "尚无主修", Content: "先使用 `学功 功法名` 学习并切换主修功法。", Actions: []string{"功法"}}, true, nil
	}
	if player.Cultivation < 50 {
		return GameResult{Title: "修为不足", Content: "精进功法需要消耗50修为。"}, true, nil
	}
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(player).Update("cultivation", gorm.Expr("cultivation - 50")).Error; err != nil {
			return err
		}
		return tx.Model(&model.PlayerSkill{}).Where("player_id = ? AND skill_id = ?", player.ID, player.CurrentSkillID).Update("mastery", gorm.Expr("mastery + 50")).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "功法精进", Content: "消耗修为：50\n功法熟练度：+50", Actions: []string{"功法", "功突"}}, true, nil
}

func (g *Game) cultivationRecord(player *model.Player) GameResult {
	minutes := g.playerValueInt(player.ID, "stats.cultivation_minutes", 0)
	gain := g.playerValueInt(player.ID, "stats.cultivation_gain", 0)
	breakthroughs := g.playerValueInt(player.ID, "stats.breakthroughs", 0)
	state := "未闭关"
	if player.State == model.PlayerStateCultivating && player.CultivationStartedAt != nil {
		state = "闭关中，已持续" + formatDuration(time.Since(*player.CultivationStartedAt))
	}
	return GameResult{Title: "修炼记录", Content: fmt.Sprintf("当前状态：%s\n累计闭关：%d分钟\n闭关所得：%d修为\n成功突破：%d次", state, minutes, gain, breakthroughs), Actions: []string{"修炼", "出关"}}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func parsePositiveInt(value string, fallback int64) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
