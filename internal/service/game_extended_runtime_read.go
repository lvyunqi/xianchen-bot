package service

import (
	"fmt"
	"strings"
	"time"

	"xianlv/internal/handler"
	"xianlv/internal/model"
)

func extendedPrimaryListCommand(category string) string {
	return map[string]string{
		"阵法": "阵法", "符箓": "符箓", "傀儡": "傀儡", "秘境争夺": "探秘",
		"传承": "传承", "悟道": "道韵", "仙魔战场": "战场", "渡劫心魔": "心魔",
		"合体技": "合技", "仙药培育": "药鉴", "法宝炼化": "法宝", "天机推演": "天命",
		"天地灵脉": "脉探", "宗门战争": "领地", "仙缘奇遇": "仙录", "宇宙星河": "星图",
	}[category]
}

func (g *Game) extendedProgressActions(category, name string) []string {
	name = strings.TrimSpace(name)
	commands := map[string][]string{
		"阵法":   {"布阵", "升阵", "破阵"},
		"符箓":   {"制符", "用符", "强符"},
		"傀儡":   {"傀战", "傀升", "傀修"},
		"秘境争夺": {"入秘", "秘战", "占秘", "守秘"},
		"传承":   {"受传", "觉传"},
		"悟道":   {"道痕", "道法"},
		"渡劫心魔": {"镇魔", "炼魔", "魔试", "劫魔", "封魔"},
		"合体技":  {"合施", "合强"},
		"天机推演": {"天机", "机缘", "改命"},
		"宇宙星河": {"星力", "星魂", "星传"},
	}
	actions := []string{extendedPrimaryListCommand(category)}
	for _, command := range commands[category] {
		if name == "" {
			actions = append(actions, command)
		} else {
			actions = append(actions, command+" "+name)
		}
	}
	actions = append(actions, extendedMenuAction(category), "状态")
	return uniqueExtendedActions(actions)
}

func (g *Game) readExtendedRuntime(player *model.Player, command handler.ParsedCommand, system extendedSystem, action string) (GameResult, bool, error) {
	switch action {
	case "detect", "atlas":
		return g.extendedAtlasRuntime(player, command, system)
	case "ranking":
		return g.extendedRankingRuntime(player, command)
	case "forecast", "warning":
		return g.extendedDestinyReading(player, command, action), true, nil
	default:
		return g.extendedOwnedRuntime(player, command, system)
	}
}

func (g *Game) extendedOwnedRuntime(player *model.Player, command handler.ParsedCommand, system extendedSystem) (GameResult, bool, error) {
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(command.RawArguments), 1)), 1)
	const pageSize = 6
	query := g.store.DB.Model(&model.PlayerExtendedProgress{}).Where("player_id = ? AND system = ?", player.ID, command.Spec.Category)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt((int(total)+pageSize-1)/pageSize, 1)
	page = minInt(page, pages)
	var rows []model.PlayerExtendedProgress
	if err := query.Order("level DESC,mastery DESC,id").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	lines := []string{fmt.Sprintf("个人%s道藏 · 第%d/%d页 · 已掌握%d项", command.Spec.Category, page, pages, total), "这里只记录你实际发现、炼成、学会或占据的内容。", "━━━━━━━━━━━"}
	actions := []string{extendedMenuAction(command.Spec.Category)}
	for _, row := range rows {
		state := displayOr(row.State, "已记录")
		extra := ""
		if row.Quantity > 0 {
			extra += fmt.Sprintf(" · 持有%d", row.Quantity)
		}
		if row.ReadyAt != nil {
			if row.ReadyAt.After(time.Now()) {
				extra += " · " + formatDuration(time.Until(*row.ReadyAt)) + "后成熟"
			} else {
				extra += " · 已成熟"
			}
		}
		lines = append(lines, fmt.Sprintf("- %s【%s】\n  等级%d · 熟练%d · 使用%d次 · 威力%d%s", row.ConfigName, state, maxInt(row.Level, 1), row.Mastery, row.Uses, row.Power, extra))
		actions = append(actions, g.extendedProgressActions(command.Spec.Category, row.ConfigName)...)
	}
	if total == 0 {
		lines = append(lines, "尚未拥有本系统内容。下方列出当前最接近你境界的可入门道藏，点击完整名称即可开始。")
		preview, previewActions, err := g.extendedAtlasPreview(player, command.Spec.Category, system, 4)
		if err != nil {
			return GameResult{}, true, err
		}
		lines = append(lines, preview...)
		actions = append(actions, previewActions...)
	}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("%s %d", command.Spec.Command, page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("%s %d", command.Spec.Command, page+1))
	}
	actions = append(actions, "道藏图鉴 "+command.Spec.Category)
	return GameResult{Title: command.Spec.Name, Content: strings.Join(lines, "\n"), Actions: uniqueExtendedActions(actions)}, true, nil
}

func (g *Game) extendedAtlasRuntime(player *model.Player, command handler.ParsedCommand, system extendedSystem) (GameResult, bool, error) {
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(command.RawArguments), 1)), 1)
	const pageSize = 6
	query := g.store.DB.Table(system.Table).Where("status = ?", "启用")
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt((int(total)+pageSize-1)/pageSize, 1)
	page = minInt(page, pages)
	var rows []model.GameplayConfigBase
	if err := query.Order("sort_order,id").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	lines := []string{fmt.Sprintf("%s图鉴 · 第%d/%d页 · 共%d项", command.Spec.Category, page, pages, total), "图鉴展示世界中真实存在的道藏；是否拥有以个人记录为准。", "━━━━━━━━━━━"}
	actions := []string{extendedPrimaryListCommand(command.Spec.Category), extendedMenuAction(command.Spec.Category)}
	for _, row := range rows {
		requirement, unmet, _ := g.prerequisiteStatus(player, row.Prerequisite)
		state := "可开始"
		if len(unmet) > 0 {
			state = "前置未满"
		}
		lines = append(lines, fmt.Sprintf("- %s【%s】\n  %s · %d阶 · 威力%d\n  前置：%s\n  消耗：%s", row.Name, state, row.Type, row.Level, decodeExtendedEffect(row).Power, requirement, displayConfigText(row.CostMaterials)))
		actions = append(actions, extendedAcquireCommand(command.Spec.Category, row.Name), "图鉴详情 "+command.Spec.Category+" "+row.Name)
	}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("%s %d", command.Spec.Command, page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("%s %d", command.Spec.Command, page+1))
	}
	return GameResult{Title: command.Spec.Name, Content: strings.Join(lines, "\n"), Actions: uniqueExtendedActions(actions)}, true, nil
}

func (g *Game) extendedAtlasPreview(player *model.Player, category string, system extendedSystem, limit int) ([]string, []string, error) {
	var rows []model.GameplayConfigBase
	if err := g.store.DB.Table(system.Table).Where("status = ?", "启用").Order("sort_order,id").Limit(maxInt(limit*5, limit)).Scan(&rows).Error; err != nil {
		return nil, nil, err
	}
	eligible := make([]model.GameplayConfigBase, 0, limit)
	locked := make([]model.GameplayConfigBase, 0, limit)
	for _, row := range rows {
		_, unmet, _ := g.prerequisiteStatus(player, row.Prerequisite)
		if len(unmet) == 0 {
			eligible = append(eligible, row)
		} else {
			locked = append(locked, row)
		}
		if len(eligible) >= limit {
			break
		}
	}
	if len(eligible) == 0 {
		eligible = locked[:minInt(len(locked), limit)]
	}
	lines, actions := []string{}, []string{}
	for _, row := range eligible {
		requirement, unmet, _ := g.prerequisiteStatus(player, row.Prerequisite)
		state := "可开始"
		if len(unmet) > 0 {
			state = "前置未满"
		}
		lines = append(lines, fmt.Sprintf("- %s【%s】 · %s\n  前置：%s", row.Name, state, row.Type, requirement))
		actions = append(actions, extendedAcquireCommand(category, row.Name))
	}
	return lines, actions, nil
}

func extendedAcquireCommand(category, name string) string {
	command := map[string]string{
		"阵法": "学阵", "符箓": "学符", "傀儡": "炼傀", "秘境争夺": "入秘",
		"传承": "寻传", "悟道": "自然", "渡劫心魔": "镇魔", "合体技": "合学",
		"仙药培育": "种药", "天机推演": "天机", "宇宙星河": "星图",
	}[category]
	if command == "" {
		command = extendedPrimaryListCommand(category)
	}
	return strings.TrimSpace(command + " " + name)
}

func (g *Game) extendedRankingRuntime(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(command.RawArguments), 1)), 1)
	const pageSize = 10
	type rankingRow struct {
		PlayerID uint
		DaoName  string
		Power    int64
		Mastery  int64
		Count    int64
	}
	var rows []rankingRow
	err := g.store.DB.Table("player_extended_progresses AS progress").
		Select("players.id AS player_id,players.dao_name,COALESCE(SUM(progress.power),0) AS power,COALESCE(SUM(progress.mastery),0) AS mastery,COUNT(progress.id) AS count").
		Joins("JOIN players ON players.id = progress.player_id").
		Where("progress.system = ? AND players.deleted_at IS NULL", command.Spec.Category).
		Group("players.id,players.dao_name").Order("power DESC,mastery DESC,players.id").Scan(&rows).Error
	if err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt((len(rows)+pageSize-1)/pageSize, 1)
	page = minInt(page, pages)
	start, end := minInt((page-1)*pageSize, len(rows)), minInt(page*pageSize, len(rows))
	lines := []string{fmt.Sprintf("第%d/%d页 · 按实际拥有道藏总威力排序", page, pages), "━━━━━━━━━━━"}
	for index, row := range rows[start:end] {
		lines = append(lines, fmt.Sprintf("%d. %s · %d项 · 威力%d · 熟练%d", start+index+1, row.DaoName, row.Count, row.Power, row.Mastery))
	}
	if len(rows) == 0 {
		lines = append(lines, "尚无人真正掌握本系统道藏。")
	}
	actions := []string{extendedPrimaryListCommand(command.Spec.Category), extendedMenuAction(command.Spec.Category)}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("%s %d", command.Spec.Command, page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("%s %d", command.Spec.Command, page+1))
	}
	return GameResult{Title: command.Spec.Name, Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) extendedDestinyReading(player *model.Player, command handler.ParsedCommand, action string) GameResult {
	if action == "warning" {
		effective := g.playerWithActiveSkillStats(player)
		healthRate := effective.Health * 100 / max64(effective.MaxHealth, 1)
		manaRate := effective.Mana * 100 / max64(effective.MaxMana, 1)
		risk := max64(0, 70-normalizedPlayerLuck(player.Luck)-player.DaoHeart/5-player.Willpower/5)
		lines := []string{fmt.Sprintf("当前气血：%d%% · 法力：%d%%", healthRate, manaRate), fmt.Sprintf("道心：%d · 意志：%d · 运气：%d/%d", player.DaoHeart, player.Willpower, normalizedPlayerLuck(player.Luck), maximumPlayerLuck), fmt.Sprintf("推演劫险：%d%%", min64(risk, 95)), "━━━━━━━━━━━"}
		if healthRate < 80 || manaRate < 60 {
			lines = append(lines, "预警：气血或法力未满，不宜立即引劫。")
		} else {
			lines = append(lines, "灵台稳定，可继续备劫；最终结果仍由境界、护法、道具与实际战斗共同决定。")
		}
		return GameResult{Title: command.Spec.Name, Content: strings.Join(lines, "\n"), Actions: []string{"备劫", "心魔", "疗伤", "状态"}}
	}
	seed := int64(player.ID)*97 + int64(time.Now().YearDay())*31 + normalizedPlayerLuck(player.Luck)
	fortunes := []string{"宜静修养神，先稳固道基再行远游。", "宜寻脉采气，地脉回响较往日清晰。", "宜炼器研法，火候与悟性彼此相合。", "宜结伴论道，同心协力可减去险阻。", "宜探索奇遇，但需保留气血与法力退路。"}
	index := int(seed % int64(len(fortunes)))
	return GameResult{Title: command.Spec.Name, Content: fmt.Sprintf("道号：%s\n今日天机：%s\n运气：%d/%d · 悟性：%d · 道心：%d\n━━━━━━━━━━━\n此结果由当日道籍、运气与道心实时推演，不会把图鉴配置冒充个人记录。", player.DaoName, fortunes[index], normalizedPlayerLuck(player.Luck), maximumPlayerLuck, player.Perception, player.DaoHeart), Actions: []string{"机缘", "劫预", "改命", "状态"}}
}

func uniqueExtendedActions(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}
