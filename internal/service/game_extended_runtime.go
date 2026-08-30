package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"xianlv/internal/handler"
	"xianlv/internal/model"
)

type extendedEffectProfile struct {
	Power    int64   `json:"power"`
	Duration int     `json:"duration"`
	Growth   float64 `json:"growth"`
}

type extendedBattleBuff struct {
	Name           string `json:"name"`
	Category       string `json:"category"`
	AttackPercent  int64  `json:"attack_percent"`
	DefensePercent int64  `json:"defense_percent"`
	SpeedPercent   int64  `json:"speed_percent"`
	Power          int64  `json:"power"`
}

func (g *Game) executeExtendedAction(player *model.Player, command handler.ParsedCommand, system extendedSystem, action string) (GameResult, bool, error) {
	switch command.Spec.Category {
	case "天地灵脉":
		return g.executeWorldLeylineExtended(player, command, system, action)
	case "仙药培育":
		return g.executeImmortalHerbExtended(player, command, system, action)
	case "法宝炼化":
		return g.executeArtifactRefinementExtended(player, command, system, action)
	case "宗门战争":
		return g.executeSectWarExtended(player, command, system, action)
	case "仙缘奇遇":
		return g.executeImmortalEncounterExtended(player, command, system, action)
	case "仙魔战场":
		return g.executeBattlefieldExtended(player, command, system, action)
	}
	if isExtendedReadAction(action) {
		return g.readExtendedRuntime(player, command, system, action)
	}
	if action == "combine" || action == "graft" {
		return g.combineExtendedRuntime(player, command, system, action)
	}
	config, result, ok, err := g.resolveExtendedRuntimeConfig(player, command, system, action)
	if err != nil || !ok {
		return result, true, err
	}
	return g.executeGenericExtendedRuntime(player, command, system, action, config)
}

func decodeExtendedEffect(config model.GameplayConfigBase) extendedEffectProfile {
	effect := extendedEffectProfile{Power: int64(maxInt(config.Level*10, 10)), Duration: 30, Growth: 1}
	_ = json.Unmarshal([]byte(config.EffectParams), &effect)
	effect.Power = max64(effect.Power, 1)
	effect.Duration = minInt(maxInt(effect.Duration, 10), 3600)
	if effect.Growth < 1 {
		effect.Growth = 1
	}
	if effect.Growth > 5 {
		effect.Growth = 5
	}
	return effect
}

func (g *Game) resolveExtendedRuntimeConfig(player *model.Player, command handler.ParsedCommand, system extendedSystem, action string) (model.GameplayConfigBase, GameResult, bool, error) {
	name := extendedConfigArgument(command)
	if action == "transfer" && len(command.Arguments) > 1 {
		name = command.Arguments[len(command.Arguments)-1]
	}
	if strings.HasPrefix(name, "@") || isExtendedFreeArgument(command.Spec.Category, action, name) {
		name = ""
	}
	if name != "" {
		config, err := g.extendedConfig(system.Table, name)
		if err == nil {
			return config, GameResult{}, true, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return model.GameplayConfigBase{}, GameResult{}, false, err
		}
		return model.GameplayConfigBase{}, GameResult{Title: command.Spec.Name + "未找到", Content: fmt.Sprintf("没有找到“%s”。请从蓝字图鉴选择完整名称；本次没有扣除资源。", name), Actions: []string{extendedPrimaryListCommand(command.Spec.Category), extendedMenuAction(command.Spec.Category)}}, false, nil
	}
	if code, valueErr := g.playerValue(player.ID, "extended.active."+system.Table); valueErr == nil && strings.TrimSpace(code) != "" {
		if config, configErr := g.extendedConfig(system.Table, code); configErr == nil {
			return config, GameResult{}, true, nil
		}
	}
	var progress model.PlayerExtendedProgress
	if err := g.store.DB.Where("player_id = ? AND system = ?", player.ID, command.Spec.Category).Order("updated_at DESC,id DESC").First(&progress).Error; err == nil {
		if config, configErr := g.extendedConfig(system.Table, progress.ConfigCode); configErr == nil {
			return config, GameResult{}, true, nil
		}
	}
	var configs []model.GameplayConfigBase
	if err := g.store.DB.Table(system.Table).Where("status = ?", "启用").Order("sort_order,id").Limit(30).Scan(&configs).Error; err != nil {
		return model.GameplayConfigBase{}, GameResult{}, false, err
	}
	for _, config := range configs {
		_, unmet, requirementErr := g.prerequisiteStatus(player, config.Prerequisite)
		if requirementErr == nil && len(unmet) == 0 {
			return config, GameResult{}, true, nil
		}
	}
	return model.GameplayConfigBase{}, GameResult{Title: command.Spec.Name + "暂无可用道藏", Content: "当前没有满足前置的配置。请从图鉴查看最低境界、战力、属性与关系要求；本次没有扣除资源。", Actions: []string{extendedPrimaryListCommand(command.Spec.Category), "状态", "帮助 " + command.Spec.Category}}, false, nil
}

func isExtendedFreeArgument(category, action, value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if category == "仙魔战场" && action == "choose" {
		return value == "仙" || value == "魔" || value == "仙盟" || value == "魔域"
	}
	if category == "仙缘奇遇" && action == "choose" {
		return true
	}
	return false
}

func (g *Game) executeGenericExtendedRuntime(player *model.Player, command handler.ParsedCommand, system extendedSystem, action string, config model.GameplayConfigBase) (GameResult, bool, error) {
	requirementText, unmet, requirementErr := g.prerequisiteStatus(player, config.Prerequisite)
	if requirementErr != nil {
		return GameResult{Title: command.Spec.Name + "道纹紊乱", Content: "前置条件无法解析，本次没有扣除资源。"}, true, nil
	}
	if len(unmet) > 0 {
		return GameResult{Title: command.Spec.Name + "尚未解锁", Content: fmt.Sprintf("道藏：%s\n前置：%s\n━━━━━━━━━━━\n未满足：\n- %s", config.Name, requirementText, strings.Join(unmet, "\n- ")), Actions: append(g.prerequisiteActions(unmet), extendedPrimaryListCommand(command.Spec.Category))}, true, nil
	}
	dependency := extendedActionDependency(command.Spec.Category, action)
	if dependency != "" && !g.hasExtendedAction(player.ID, system, config.Code, dependency) {
		dependencyCommand := extendedActionCommand(command.Spec.Category, system, dependency) + " " + config.Name
		return GameResult{Title: command.Spec.Name + "前序未完成", Content: fmt.Sprintf("道藏：%s\n需要先完成：%s\n本次没有扣除资源。", config.Name, strings.TrimSpace(dependencyCommand)), Actions: []string{strings.TrimSpace(dependencyCommand), extendedPrimaryListCommand(command.Spec.Category)}}, true, nil
	}
	var progress model.PlayerExtendedProgress
	progressErr := g.store.DB.Where("player_id = ? AND system = ? AND config_code = ?", player.ID, command.Spec.Category, config.Code).First(&progress).Error
	if progressErr != nil && !errors.Is(progressErr, gorm.ErrRecordNotFound) {
		return GameResult{}, true, progressErr
	}
	if progress.ID != 0 && extendedActionOnlyOnce(command.Spec.Category, action) && g.hasExtendedAction(player.ID, system, config.Code, action) {
		return GameResult{Title: command.Spec.Name + "已经完成", Content: fmt.Sprintf("%s已经记录在你的%s道藏中。重复执行不会再次扣除材料。\n等级：%d · 熟练：%d · 当前威力：%d", config.Name, command.Spec.Category, progress.Level, progress.Mastery, progress.Power), Actions: g.extendedProgressActions(command.Spec.Category, config.Name)}, true, nil
	}
	if action == "transfer" {
		target, targetResult, targetOK := g.extendedTransferTarget(player, command, config)
		if !targetOK {
			return targetResult, true, nil
		}
		return g.transferExtendedRuntime(player, &target, command, system, config, progress)
	}
	costText, missing, costErr := g.extendedCostStatus(player, config.CostMaterials)
	if costErr != nil {
		return GameResult{Title: command.Spec.Name + "配置错误", Content: "消耗配置无法解析，本次没有扣除资源。"}, true, nil
	}
	if len(missing) > 0 {
		return GameResult{Title: command.Spec.Name + "材料不足", Content: fmt.Sprintf("道藏：%s\n本次需要：%s\n━━━━━━━━━━━\n缺少：\n- %s", config.Name, costText, strings.Join(missing, "\n- ")), Actions: []string{"背包", "物品 " + firstExtendedMaterial(config.CostMaterials), "地图", "副本", "货铺"}}, true, nil
	}
	effect := decodeExtendedEffect(config)
	if isExtendedBattleAction(action) {
		return g.beginExtendedRuntimeBattle(player, command, system, action, config, effect, costText)
	}
	before := progress
	if progress.ID == 0 {
		progress = model.PlayerExtendedProgress{PlayerID: player.ID, System: command.Spec.Category, ConfigCode: config.Code, ConfigName: config.Name, State: extendedActionState(action), Level: 1, Power: effect.Power, MetadataJSON: `{}`}
	}
	progress.ConfigName = config.Name
	progress.State = extendedActionState(action)
	progress.Uses++
	progress.Experience += int64(maxInt(config.Level, 1) * 10)
	progress.Mastery += int64(maxInt(config.Level, 1))
	if action == "upgrade" || action == "awaken" || action == "refine" || action == "cultivate" || action == "deepen" || action == "absorb" || action == "seal" || action == "create" {
		progress.Level++
		progress.Power += max64(int64(math.Round(float64(effect.Power)*effect.Growth/4)), 1)
	}
	if command.Spec.Category == "符箓" && action == "craft" {
		progress.Quantity++
	}
	if command.Spec.Category == "符箓" && action == "use" {
		if progress.Quantity < 1 {
			return GameResult{Title: "符箓不足", Content: "你学会了此符，但尚未炼成可用符箓。请先制符；本次没有扣除施展材料。", Actions: []string{"制符 " + config.Name, "符箓"}}, true, nil
		}
		progress.Quantity--
	}
	playerUpdates, effectLines := extendedPermanentEffects(command.Spec.Category, action, effect, progress.Level)
	key := fmt.Sprintf("extended.%s.%s.%s", system.Table, config.Code, action)
	count := g.playerValueInt(player.ID, key, 0) + 1
	var buff *extendedBattleBuff
	if action == "activate" || action == "use" {
		value := extendedBuffFromEffect(command.Spec.Category, config.Name, effect, progress.Level)
		buff = &value
	}
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		costs := make(map[string]int64)
		if err := json.Unmarshal([]byte(config.CostMaterials), &costs); err != nil {
			return err
		}
		if err := consumeExtendedCostTx(tx, player.ID, costs); err != nil {
			return err
		}
		if len(playerUpdates) > 0 {
			if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(playerUpdates).Error; err != nil {
				return err
			}
		}
		if err := upsertExtendedProgressTx(tx, progress); err != nil {
			return err
		}
		if err := upsertPlayerValueTx(tx, player.ID, key, strconv.FormatInt(count, 10), nil); err != nil {
			return err
		}
		if err := upsertPlayerValueTx(tx, player.ID, "extended.active."+system.Table, config.Code, nil); err != nil {
			return err
		}
		if buff != nil {
			until := time.Now().Add(time.Duration(effect.Duration) * time.Second)
			encoded, _ := json.Marshal(buff)
			if err := upsertPlayerValueTx(tx, player.ID, "extended.battle_buff", string(encoded), &until); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return GameResult{}, true, err
	}
	if command.Spec.Category == "合体技" && action == "use" {
		partnerLine, partnerErr := g.shareCoupleExtendedBuff(player, *buff, time.Duration(effect.Duration)*time.Second)
		if partnerErr != nil {
			return GameResult{}, true, partnerErr
		}
		effectLines = append(effectLines, partnerLine)
	}
	if len(effectLines) == 0 {
		effectLines = append(effectLines, "道藏已写入个人修行记录，后续动作将读取这一真实阶段。")
	}
	content := fmt.Sprintf("道藏：%s\n类型：%s · 配置阶位%d\n━━━━━━━━━━━\n状态：%s\n个人等级：%d → %d\n熟练度：%d → %d\n实际威力：%d\n%s\n━━━━━━━━━━━\n实际消耗：%s\n解锁条件：%s", config.Name, config.Type, config.Level, progress.State, maxInt(before.Level, 0), progress.Level, before.Mastery, progress.Mastery, progress.Power, strings.Join(effectLines, "\n"), displayOr(costText, "无"), requirementText)
	return GameResult{Title: command.Spec.Name + "完成", Content: content, ImageURL: config.ImageURL, Actions: g.extendedProgressActions(command.Spec.Category, config.Name)}, true, nil
}

func extendedActionOnlyOnce(category, action string) bool {
	if action == "learn" || action == "accept" || action == "seek" || action == "discover" || action == "enter" || action == "suppress" || action == "deduce" || action == "explore" {
		return true
	}
	if action == "craft" && category == "傀儡" {
		return true
	}
	return false
}

func extendedActionState(action string) string {
	states := map[string]string{
		"seek": "已发现", "discover": "已发现", "detect": "已发现", "learn": "已学会", "accept": "已传承",
		"craft": "已炼成", "activate": "已启用", "enter": "探索中", "occupy": "已占据", "suppress": "已镇压",
		"plant": "培育中", "harvest": "已采摘", "refine": "炼化中", "bind": "已认主", "deduce": "已推演",
		"declare": "交战中", "trigger": "待抉择", "choose": "已抉择", "explore": "已探明", "awaken": "已觉醒",
		"upgrade": "已强化", "cultivate": "已培育", "deepen": "已加深", "seal": "已封印", "use": "已施展",
	}
	if state := states[action]; state != "" {
		return state
	}
	return "已记录"
}

func extendedPermanentEffects(category, action string, effect extendedEffectProfile, level int) (map[string]any, []string) {
	updates := map[string]any{}
	lines := []string{}
	gain := max64(int64(math.Round(math.Sqrt(float64(effect.Power))*effect.Growth*float64(maxInt(level, 1))/2)), 1)
	addAttack := func(value int64) {
		updates["physical_attack"] = gorm.Expr("physical_attack + ?", value)
		updates["magic_attack"] = gorm.Expr("magic_attack + ?", value)
		lines = append(lines, fmt.Sprintf("永久攻法+%d", value))
	}
	addDefense := func(value int64) {
		updates["physical_defense"] = gorm.Expr("physical_defense + ?", value)
		updates["magic_defense"] = gorm.Expr("magic_defense + ?", value)
		lines = append(lines, fmt.Sprintf("永久双防+%d", value))
	}
	switch category {
	case "阵法":
		if action == "upgrade" || action == "combine" {
			addDefense(gain)
		}
	case "傀儡":
		if action == "craft" || action == "upgrade" || action == "combine" {
			addAttack(gain)
			addDefense(max64(gain/2, 1))
		}
	case "秘境争夺":
		if action == "occupy" || action == "defend" {
			updates["reputation"] = gorm.Expr("reputation + ?", gain)
			updates["merit"] = gorm.Expr("merit + ?", max64(gain/2, 1))
			lines = append(lines, fmt.Sprintf("声望+%d · 功德+%d", gain, max64(gain/2, 1)))
		}
	case "传承":
		if action == "accept" || action == "awaken" || action == "combine" {
			multiplier := int64(1)
			if action == "awaken" {
				multiplier = 2
			}
			addAttack(gain * multiplier)
			updates["perception"] = gorm.Expr("perception + ?", max64(gain/2, 1)*multiplier)
			lines = append(lines, fmt.Sprintf("永久悟性+%d", max64(gain/2, 1)*multiplier))
		}
	case "悟道":
		if action == "practice" {
			updates["cultivation"] = gorm.Expr("cultivation + ?", effect.Power)
			lines = append(lines, fmt.Sprintf("修为+%d", effect.Power))
		}
		if action == "study" || action == "create" {
			updates["perception"] = gorm.Expr("perception + ?", gain)
			updates["dao_heart"] = gorm.Expr("MIN(dao_heart + ?, 100)", max64(gain/2, 1))
			lines = append(lines, fmt.Sprintf("永久悟性+%d · 道心+%d", gain, max64(gain/2, 1)))
		}
	case "渡劫心魔":
		if action == "suppress" || action == "refine" || action == "seal" {
			updates["willpower"] = gorm.Expr("willpower + ?", gain)
			updates["dao_heart"] = gorm.Expr("MIN(dao_heart + ?, 100)", max64(gain/2, 1))
			lines = append(lines, fmt.Sprintf("永久意志+%d · 道心+%d", gain, max64(gain/2, 1)))
		}
	case "合体技":
		if action == "upgrade" || action == "combine" {
			addAttack(gain)
			addDefense(gain)
		}
	case "天机推演":
		if action == "deduce" || action == "seek" {
			updates["perception"] = gorm.Expr("perception + ?", max64(gain/2, 1))
			lines = append(lines, fmt.Sprintf("永久悟性+%d", max64(gain/2, 1)))
		}
		if action == "change" {
			updates["luck"] = gorm.Expr("CASE WHEN luck + 1 > ? THEN ? ELSE luck + 1 END", maximumPlayerLuck, maximumPlayerLuck)
			lines = append(lines, "永久运气+1（不超过50）")
		}
	case "宇宙星河":
		if action == "absorb" || action == "awaken" {
			mana := gain * 3
			updates["max_mana"] = gorm.Expr("max_mana + ?", mana)
			updates["mana"] = gorm.Expr("mana + ?", mana)
			updates["spirit"] = gorm.Expr("spirit + ?", max64(gain/2, 1))
			lines = append(lines, fmt.Sprintf("永久法力+%d · 神识+%d", mana, max64(gain/2, 1)))
		}
	}
	return updates, lines
}

func extendedBuffFromEffect(category, name string, effect extendedEffectProfile, level int) extendedBattleBuff {
	percent := min64(max64(effect.Power/10+int64(level), 5), 60)
	buff := extendedBattleBuff{Name: name, Category: category, Power: effect.Power}
	switch category {
	case "阵法":
		buff.DefensePercent = percent
	case "符箓":
		buff.AttackPercent, buff.SpeedPercent = percent, max64(percent/2, 3)
	case "合体技":
		buff.AttackPercent, buff.DefensePercent = percent, max64(percent/2, 3)
	default:
		buff.AttackPercent = percent
	}
	return buff
}

func (g *Game) shareCoupleExtendedBuff(player *model.Player, buff extendedBattleBuff, duration time.Duration) (string, error) {
	var couple model.Couple
	if err := g.store.DB.Where("id = ? AND status = ?", player.CoupleID, "active").First(&couple).Error; err != nil {
		return "道侣不在同心契中，仅你获得本次合体技效果。", nil
	}
	partnerID := couple.PlayerAID
	if partnerID == player.ID {
		partnerID = couple.PlayerBID
	}
	until := time.Now().Add(duration)
	encoded, _ := json.Marshal(buff)
	if err := g.setPlayerValue(partnerID, "extended.battle_buff", string(encoded), &until); err != nil {
		return "", err
	}
	var partner model.Player
	_ = g.store.DB.First(&partner, partnerID).Error
	return fmt.Sprintf("同心共鸣：%s同步获得%d秒战斗加成", displayOr(partner.DaoName, "道侣"), int(duration/time.Second)), nil
}

func (g *Game) hasExtendedAction(playerID uint, system extendedSystem, code, action string) bool {
	key := fmt.Sprintf("extended.%s.%s.%s", system.Table, code, action)
	if g.playerValueInt(playerID, key, 0) > 0 {
		return true
	}
	var progress model.PlayerExtendedProgress
	if g.store.DB.Where("player_id = ? AND config_code = ?", playerID, code).First(&progress).Error != nil {
		return false
	}
	if action == "learn" || action == "accept" || action == "seek" || action == "discover" || action == "craft" || action == "enter" || action == "suppress" || action == "plant" || action == "refine" || action == "deduce" || action == "occupy" || action == "declare" || action == "trigger" || action == "explore" {
		return true
	}
	return false
}

func upsertExtendedProgressTx(tx *gorm.DB, progress model.PlayerExtendedProgress) error {
	progress.ID = 0
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "player_id"}, {Name: "system"}, {Name: "config_code"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"config_name", "state", "level", "experience", "mastery", "uses", "quantity", "power", "ready_at", "active_until", "metadata_json", "updated_at",
		}),
	}).Create(&progress).Error
}

func firstExtendedMaterial(raw string) string {
	var costs map[string]int64
	if json.Unmarshal([]byte(raw), &costs) != nil {
		return "材料"
	}
	keys := make([]string, 0, len(costs))
	for key := range costs {
		if key != "灵石" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return "灵石"
	}
	return keys[0]
}

func isExtendedBattleAction(action string) bool {
	return action == "battle" || action == "challenge" || action == "defend"
}

func (g *Game) beginExtendedRuntimeBattle(player *model.Player, command handler.ParsedCommand, system extendedSystem, action string, config model.GameplayConfigBase, effect extendedEffectProfile, costText string) (GameResult, bool, error) {
	if player.State != "" && player.State != model.PlayerStateIdle {
		return GameResult{Title: "当前无法开战", Content: "你正处于其他修行或战斗状态。已完成的前置结算会保留，请先结束当前状态。", Actions: []string{"状态", "投降", extendedMenuAction(command.Spec.Category)}}, true, nil
	}
	requirement, _ := decodeGameplayPrerequisite(config.Prerequisite)
	enemyPower := max64(requirement.MinimumCombatPower, max64(effect.Power*4, 80))
	enemyHP := max64(enemyPower*2, 80)
	effective := g.playerWithActiveSkillStats(player)
	state := mapMonsterBattleState{
		BattleKind: "道藏试炼", Round: 1, EnemyName: config.Name + "·守关道影", EnemyPower: enemyPower,
		PlayerHP: effective.Health, PlayerMana: effective.Mana, EnemyHP: enemyHP, EnemyMaxHP: enemyHP,
		ExtendedCategory: command.Spec.Category, ExtendedConfigCode: config.Code, ExtendedConfigName: config.Name, ExtendedAction: action,
		StartedAt: time.Now().UnixMilli(),
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		return GameResult{}, true, err
	}
	costs := make(map[string]int64)
	if err := json.Unmarshal([]byte(config.CostMaterials), &costs); err != nil {
		return GameResult{Title: command.Spec.Name + "配置错误", Content: "消耗配置无法解析，本次没有创建战局。"}, true, nil
	}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := consumeExtendedCostTx(tx, player.ID, costs); err != nil {
			return err
		}
		if err := upsertPlayerValueTx(tx, player.ID, "pve.battle", string(encoded), nil); err != nil {
			return err
		}
		result := tx.Model(&model.Player{}).Where("id = ? AND (state = ? OR state = '')", player.ID, model.PlayerStateIdle).Update("state", model.PlayerStateBattling)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("角色状态已经变化，无法创建道藏战局")
		}
		return upsertPlayerValueTx(tx, player.ID, "extended.pending.system", system.Table, nil)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: command.Spec.Name + "战局开启", Content: fmt.Sprintf("道藏：%s\n敌方：%s（战力%d）\n敌方气血：%d/%d\n你的气血：%d/%d · 法力：%d/%d\n本次消耗：%s\n━━━━━━━━━━━\n战斗不会自动完成。现在由你逐回合选择普通攻击、功法技能、防御或投降。", config.Name, state.EnemyName, enemyPower, enemyHP, enemyHP, effective.Health, effective.MaxHealth, effective.Mana, effective.MaxMana, displayOr(costText, "无")), Actions: []string{"攻击", "技能", "防御", "投降", "功法"}}, true, nil
}

func (g *Game) transferExtendedRuntime(player, target *model.Player, command handler.ParsedCommand, system extendedSystem, config model.GameplayConfigBase, progress model.PlayerExtendedProgress) (GameResult, bool, error) {
	if progress.ID == 0 {
		return GameResult{Title: command.Spec.Name + "失败", Content: "你尚未真正掌握这条道藏，不能传给他人。", Actions: []string{extendedPrimaryListCommand(command.Spec.Category)}}, true, nil
	}
	costText, missing, err := g.extendedCostStatus(player, config.CostMaterials)
	if err != nil || len(missing) > 0 {
		return GameResult{Title: command.Spec.Name + "材料不足", Content: fmt.Sprintf("传承%s需要：%s\n缺少：%s", config.Name, costText, strings.Join(missing, "、")), Actions: []string{"背包", "货铺"}}, true, nil
	}
	grant := progress
	grant.ID = 0
	grant.PlayerID = target.ID
	grant.Level = maxInt(progress.Level-1, 1)
	grant.Mastery = progress.Mastery / 2
	grant.Experience = progress.Experience / 2
	grant.Uses = 0
	grant.State = "承接传承"
	key := fmt.Sprintf("extended.%s.%s.%s", system.Table, config.Code, extendedTransferGrantAction(command.Spec.Category))
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		costs := make(map[string]int64)
		if json.Unmarshal([]byte(config.CostMaterials), &costs) != nil {
			return errors.New("传承消耗配置无法解析")
		}
		if err := consumeExtendedCostTx(tx, player.ID, costs); err != nil {
			return err
		}
		if err := upsertExtendedProgressTx(tx, grant); err != nil {
			return err
		}
		if err := upsertPlayerValueTx(tx, target.ID, key, "1", nil); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: command.Spec.Name + "完成", Content: fmt.Sprintf("传承者：%s\n承接者：%s\n道藏：%s\n承接等级：%d · 熟练度：%d · 威力：%d\n实际消耗：%s\n━━━━━━━━━━━\n原道藏仍由你保留，对方已经获得可继续成长的真实记录。", player.DaoName, target.DaoName, config.Name, grant.Level, grant.Mastery, grant.Power, costText), Actions: []string{extendedPrimaryListCommand(command.Spec.Category), "通知", "状态"}}, true, nil
}

func (g *Game) combineExtendedRuntime(player *model.Player, command handler.ParsedCommand, system extendedSystem, action string) (GameResult, bool, error) {
	if len(command.Arguments) < 2 {
		return GameResult{Title: command.Spec.Name, Content: "请输入两个不同的完整名称：`" + command.Spec.Input + "`。", Actions: []string{extendedPrimaryListCommand(command.Spec.Category)}}, true, nil
	}
	firstName, secondName := command.Arguments[0], command.Arguments[1]
	if firstName == secondName {
		return GameResult{Title: command.Spec.Name + "失败", Content: "两条道藏必须不同。本次没有扣除资源。", Actions: []string{extendedPrimaryListCommand(command.Spec.Category)}}, true, nil
	}
	first, firstErr := g.extendedConfig(system.Table, firstName)
	second, secondErr := g.extendedConfig(system.Table, secondName)
	if firstErr != nil || secondErr != nil {
		return GameResult{Title: command.Spec.Name + "未找到", Content: "至少一个名称不在本系统图鉴中，请从已拥有列表点击选择。", Actions: []string{extendedPrimaryListCommand(command.Spec.Category)}}, true, nil
	}
	var firstProgress, secondProgress model.PlayerExtendedProgress
	if g.store.DB.Where("player_id = ? AND system = ? AND config_code = ?", player.ID, command.Spec.Category, first.Code).First(&firstProgress).Error != nil || g.store.DB.Where("player_id = ? AND system = ? AND config_code = ?", player.ID, command.Spec.Category, second.Code).First(&secondProgress).Error != nil {
		return GameResult{Title: command.Spec.Name + "前置不足", Content: "只有已经真正掌握的两条不同道藏才能融合；本次没有扣除资源。", Actions: []string{extendedPrimaryListCommand(command.Spec.Category)}}, true, nil
	}
	var total int64
	_ = g.store.DB.Table(system.Table).Where("status = ?", "启用").Count(&total).Error
	offset := 0
	if total > 0 {
		offset = int((int64(first.ID) + int64(second.ID) + firstProgress.Mastery + secondProgress.Mastery) % total)
	}
	var resultConfig model.GameplayConfigBase
	if err := g.store.DB.Table(system.Table).Where("status = ?", "启用").Order("sort_order,id").Offset(offset).First(&resultConfig).Error; err != nil {
		return GameResult{}, true, err
	}
	if resultConfig.Code == first.Code || resultConfig.Code == second.Code {
		_ = g.store.DB.Table(system.Table).Where("status = ? AND code NOT IN ?", "启用", []string{first.Code, second.Code}).Order("sort_order,id").First(&resultConfig).Error
	}
	effect := decodeExtendedEffect(resultConfig)
	merged := model.PlayerExtendedProgress{PlayerID: player.ID, System: command.Spec.Category, ConfigCode: resultConfig.Code, ConfigName: resultConfig.Name, State: "融合新生", Level: maxInt((firstProgress.Level+secondProgress.Level)/2, 1), Experience: firstProgress.Experience/2 + secondProgress.Experience/2, Mastery: firstProgress.Mastery/2 + secondProgress.Mastery/2, Power: max64(effect.Power+(firstProgress.Power+secondProgress.Power)/4, 1), MetadataJSON: fmt.Sprintf(`{"parents":[%q,%q]}`, first.Code, second.Code)}
	costText, missing, err := g.extendedCostStatus(player, resultConfig.CostMaterials)
	if err != nil || len(missing) > 0 {
		return GameResult{Title: command.Spec.Name + "材料不足", Content: fmt.Sprintf("融合结果将指向%s，但还缺少：%s\n本次没有扣除资源。", resultConfig.Name, strings.Join(missing, "、")), Actions: []string{"背包", "地图", "副本"}}, true, nil
	}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		costs := make(map[string]int64)
		if json.Unmarshal([]byte(resultConfig.CostMaterials), &costs) != nil {
			return errors.New("融合消耗无法解析")
		}
		if err := consumeExtendedCostTx(tx, player.ID, costs); err != nil {
			return err
		}
		if err := upsertExtendedProgressTx(tx, merged); err != nil {
			return err
		}
		return upsertPlayerValueTx(tx, player.ID, fmt.Sprintf("extended.%s.%s.%s", system.Table, resultConfig.Code, extendedTransferGrantAction(command.Spec.Category)), "1", nil)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: command.Spec.Name + "成功", Content: fmt.Sprintf("父系一：%s\n父系二：%s\n━━━━━━━━━━━\n新生道藏：%s\n等级：%d · 熟练：%d · 威力：%d\n实际消耗：%s", first.Name, second.Name, merged.ConfigName, merged.Level, merged.Mastery, merged.Power, costText), Actions: g.extendedProgressActions(command.Spec.Category, merged.ConfigName)}, true, nil
}
