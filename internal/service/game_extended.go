package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"xianlv/internal/handler"
	"xianlv/internal/model"
	"xianlv/internal/storage"
)

type extendedSystem struct {
	Table   string
	Actions []string
}

var extendedSystems = map[string]extendedSystem{
	"阵法":   {"formation_configs", []string{"learn", "activate", "list", "upgrade", "combine", "challenge"}},
	"符箓":   {"talisman_configs", []string{"learn", "craft", "list", "use", "upgrade", "market"}},
	"傀儡":   {"puppet_configs", []string{"craft", "list", "battle", "upgrade", "repair", "combine"}},
	"秘境争夺": {"secret_realm_conflict_configs", []string{"detect", "enter", "battle", "occupy", "defend", "ranking"}},
	"传承":   {"inheritance_configs", []string{"seek", "accept", "list", "combine", "transfer", "awaken"}},
	"悟道":   {"dao_insight_configs", []string{"practice", "status", "discover", "study", "use", "create"}},
	"仙魔战场": {"immortal_demon_battlefield_configs", []string{"enter", "choose", "battle", "task", "ranking", "shop"}},
	"灵根进化": {"spiritual_root_evolution_configs", []string{"inspect", "refine", "evolve", "awaken", "combine", "transfer"}},
	"渡劫心魔": {"inner_demon_configs", []string{"inspect", "suppress", "refine", "challenge", "battle", "seal"}},
	"合体技":  {"couple_combination_skill_configs", []string{"learn", "list", "use", "upgrade", "combine", "transfer"}},
	"仙药培育": {"immortal_herb_configs", []string{"plant", "cultivate", "harvest", "graft", "accelerate", "atlas"}},
	"法宝炼化": {"artifact_refinement_configs", []string{"refine", "awaken", "cultivate", "combine", "bind", "transfer"}},
	"天机推演": {"destiny_deduction_configs", []string{"deduce", "forecast", "warning", "seek", "change", "backlash"}},
	"天地灵脉": {"leyline_configs", []string{"detect", "occupy", "challenge", "practice", "transfer", "combine"}},
	"宗门战争": {"sect_war_configs", []string{"declare", "prepare", "battle", "negotiate", "ally", "territory"}},
	"仙缘奇遇": {"immortal_encounter_configs", []string{"trigger", "choose", "record", "deepen", "transfer", "awaken"}},
	"宇宙星河": {"star_realm_configs", []string{"explore", "absorb", "awaken", "teleport"}},
}

func (g *Game) executeExtended(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	system, ok := extendedSystems[command.Spec.Category]
	if !ok {
		return GameResult{}, false, fmt.Errorf("扩展玩法分类不存在: %s", command.Spec.Category)
	}
	if command.Spec.Category == "灵根进化" {
		return g.executeSpiritualRootEvolution(player, command, system)
	}
	baseID := extendedCategoryBaseID(command.Spec.Category)
	slot := command.Spec.ID - baseID
	if slot < 0 || slot >= len(system.Actions) {
		return GameResult{}, false, fmt.Errorf("扩展玩法动作不存在: %d", command.Spec.ID)
	}
	return g.executeExtendedAction(player, command, system, system.Actions[slot])
}

func (g *Game) readExtendedSystem(player *model.Player, command handler.ParsedCommand, system extendedSystem, action string) (GameResult, bool, error) {
	const pageSize = 6
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(command.RawArguments), 1)), 1)
	var total int64
	_ = g.store.DB.Table(system.Table).Where("status = ?", "启用").Count(&total).Error
	pages := maxInt(int((total+pageSize-1)/pageSize), 1)
	if page > pages {
		page = pages
	}
	var rows []model.GameplayConfigBase
	if err := g.store.DB.Table(system.Table).Where("status = ?", "启用").Order("sort_order, id").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("当前启用配置：%d项 · 第%d/%d页", total, page, pages))
	for _, row := range rows {
		requirement, unmet, _ := g.prerequisiteStatus(player, row.Prerequisite)
		state := "已解锁"
		if len(unmet) > 0 {
			state = "未解锁"
		}
		lines = append(lines, fmt.Sprintf("- %s【%s】\n  %s · 配置%d级\n  前置：%s\n  %s", row.Name, state, row.Type, row.Level, requirement, row.Description))
	}
	if len(rows) == 0 {
		lines = append(lines, "天机阁暂未开放此脉传承，本次查询不会消耗任何资源。")
	}
	actions := append([]string{extendedMenuAction(command.Spec.Category)}, extendedSystemCommands(command.Spec.Category)...)
	if page > 1 {
		actions = append(actions, fmt.Sprintf("%s %d", command.Spec.Command, page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("%s %d", command.Spec.Command, page+1))
	}
	return GameResult{Title: command.Spec.Name, Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) extendedTransferTarget(player *model.Player, command handler.ParsedCommand, config model.GameplayConfigBase) (model.Player, GameResult, bool) {
	if len(command.Arguments) == 0 {
		return model.Player{}, GameResult{Title: command.Spec.Name, Content: "请输入：`" + command.Spec.Input + "`"}, false
	}
	target, err := g.findPlayer(command.Arguments[0])
	if err != nil {
		return model.Player{}, GameResult{Title: command.Spec.Name + "失败", Content: "没有找到目标修士。", Actions: []string{extendedMenuAction(command.Spec.Category)}}, false
	}
	if target.ID == player.ID {
		return model.Player{}, GameResult{Title: command.Spec.Name + "失败", Content: "不能将传承转给自己。"}, false
	}
	requirement, unmet, requirementErr := g.prerequisiteStatus(&target, config.Prerequisite)
	if requirementErr != nil {
		return model.Player{}, GameResult{Title: command.Spec.Name + "配置错误", Content: "前置条件JSON无法解析。"}, false
	}
	if len(unmet) > 0 {
		return model.Player{}, GameResult{Title: "对方无法承接", Content: fmt.Sprintf("目标：%s\n配置：%s\n承接前置：%s\n━━━━━━━\n对方未满足：\n- %s", target.DaoName, config.Name, requirement, strings.Join(unmet, "\n- "))}, false
	}
	return target, GameResult{}, true
}

func (g *Game) transferExtendedPower(player *model.Player, target *model.Player, command handler.ParsedCommand, system extendedSystem, config model.GameplayConfigBase) (GameResult, bool, error) {
	sourceKey := fmt.Sprintf("extended.%s.%s.transfer", system.Table, config.Code)
	value, err := g.addPlayerValueInt(player.ID, sourceKey, 1)
	if err != nil {
		return GameResult{}, true, err
	}
	grantAction := extendedTransferGrantAction(command.Spec.Category)
	grantKey := fmt.Sprintf("extended.%s.%s.%s", system.Table, config.Code, grantAction)
	if _, err := g.addPlayerValueInt(target.ID, grantKey, 1); err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: command.Spec.Name, Content: fmt.Sprintf("传承者：%s\n承接者：%s\n传承配置：%s\n已为对方解锁：%s\n累计传承：%d次\n━━━━━━━\n前置、材料和前序步骤均已结算，对方可进入%s菜单查看。", player.DaoName, target.DaoName, config.Name, extendedActionCommand(command.Spec.Category, system, grantAction), value, command.Spec.Category), Actions: []string{extendedMenuAction(command.Spec.Category), "状态"}}, true, nil
}

func extendedTransferGrantAction(category string) string {
	return map[string]string{
		"传承": "accept", "灵根进化": "evolve", "合体技": "learn",
		"法宝炼化": "bind", "天地灵脉": "occupy", "仙缘奇遇": "deepen",
	}[category]
}

func (g *Game) extendedConfig(table, name string) (model.GameplayConfigBase, error) {
	var row model.GameplayConfigBase
	query := g.store.DB.Table(table).Where("status = ?", "启用")
	if name != "" {
		query = query.Where("name = ? OR code = ?", name, name)
	}
	err := query.Order("sort_order, id").First(&row).Error
	return row, err
}

func extendedConfigArgument(command handler.ParsedCommand) string {
	if len(command.Arguments) == 0 {
		return ""
	}
	if strings.HasPrefix(command.Arguments[0], "@") && len(command.Arguments) > 1 {
		return command.Arguments[len(command.Arguments)-1]
	}
	if strings.HasPrefix(command.Arguments[0], "@") {
		return ""
	}
	return command.Arguments[0]
}

func isExtendedReadAction(action string) bool {
	switch action {
	case "list", "market", "detect", "ranking", "status", "inspect", "atlas", "forecast", "warning", "record", "territory", "shop":
		return true
	}
	return false
}

func needsTwoArguments(command handler.ParsedCommand, action string) bool {
	if action != "combine" && action != "graft" {
		return false
	}
	return len(strings.Fields(command.Spec.Input)) > 2
}

func needsNamedConfig(command handler.ParsedCommand, action string) bool {
	if isExtendedReadAction(action) {
		return false
	}
	return len(strings.Fields(command.Spec.Input)) > 1
}

func extendedActionDependency(category, action string) string {
	dependencies := map[string]map[string]string{
		"阵法":   {"activate": "learn", "upgrade": "learn", "combine": "upgrade", "challenge": "activate"},
		"符箓":   {"craft": "learn", "use": "craft", "upgrade": "craft"},
		"傀儡":   {"battle": "craft", "upgrade": "craft", "repair": "craft", "combine": "upgrade"},
		"秘境争夺": {"battle": "enter", "occupy": "battle", "defend": "occupy"},
		"传承":   {"accept": "seek", "combine": "accept", "awaken": "accept"},
		"悟道":   {"study": "discover", "use": "study", "create": "study"},
		"仙魔战场": {"battle": "choose", "task": "enter"},
		"灵根进化": {"evolve": "refine", "awaken": "evolve", "combine": "evolve"},
		"渡劫心魔": {"refine": "suppress", "challenge": "suppress", "battle": "challenge", "seal": "suppress"},
		"合体技":  {"use": "learn", "upgrade": "learn", "combine": "upgrade"},
		"仙药培育": {"cultivate": "plant", "harvest": "cultivate", "graft": "cultivate", "accelerate": "plant"},
		"法宝炼化": {"awaken": "refine", "cultivate": "awaken", "combine": "cultivate", "bind": "refine"},
		"天机推演": {"seek": "deduce", "change": "deduce", "backlash": "change"},
		"天地灵脉": {"occupy": "detect", "challenge": "occupy", "practice": "occupy", "combine": "occupy"},
		"宗门战争": {"prepare": "declare", "battle": "prepare", "negotiate": "declare", "ally": "negotiate"},
		"仙缘奇遇": {"choose": "trigger", "deepen": "trigger", "awaken": "deepen"},
		"宇宙星河": {"absorb": "explore", "awaken": "absorb", "teleport": "explore"},
	}
	transferDependencies := map[string]string{
		"传承": "accept", "灵根进化": "awaken", "合体技": "learn",
		"法宝炼化": "bind", "天地灵脉": "occupy", "仙缘奇遇": "deepen",
	}
	if action == "transfer" {
		return transferDependencies[category]
	}
	return dependencies[category][action]
}

func extendedActionCommand(category string, system extendedSystem, action string) string {
	for index, candidate := range system.Actions {
		if candidate != action {
			continue
		}
		id := extendedCategoryBaseID(category) + index
		for _, spec := range handler.CommandTable {
			if spec.ID == id {
				return spec.Command
			}
		}
	}
	return extendedMenuAction(category)
}

func extendedResultText(category, action string, config model.GameplayConfigBase, value int64) string {
	verbs := map[string]string{
		"learn": "参悟成功", "activate": "布置成功", "craft": "炼制成功", "use": "施展成功", "upgrade": "提升成功",
		"combine": "融合成功", "challenge": "挑战告捷", "battle": "战斗告捷", "repair": "修复完成", "enter": "已进入",
		"occupy": "占领成功", "defend": "防守成功", "seek": "寻得线索", "accept": "通过考验", "awaken": "觉醒成功",
		"practice": "悟道有成", "discover": "感悟有得", "study": "参悟成功", "create": "自创成功", "choose": "选择已记录",
		"task": "任务已推进", "refine": "淬炼完成", "evolve": "进化完成", "suppress": "镇压成功", "seal": "封印完成",
		"plant": "种植成功", "cultivate": "培育完成", "harvest": "采摘完成", "graft": "嫁接成功", "accelerate": "催化完成",
		"bind": "认主成功", "deduce": "推演完成", "change": "改命完成", "backlash": "反噬已化解", "declare": "宣战完成",
		"prepare": "备战完成", "negotiate": "议和完成", "ally": "结盟完成", "trigger": "奇遇触发", "deepen": "仙缘加深",
		"explore": "星图展开", "absorb": "星力入体", "teleport": "传送完成",
	}
	verb := verbs[action]
	if verb == "" {
		verb = "操作完成"
	}
	narratives := map[string]string{
		"阵法": "阵纹沿地脉次第亮起，四方灵气在阵眼汇聚。", "符箓": "朱砂落纸，灵纹一笔贯通，符胆随之生光。",
		"傀儡": "机括咬合，灵核苏醒，傀儡眼中亮起一点神光。", "秘境争夺": "秘境界壁震动，守卫、资源与占领权同时进入争夺。",
		"传承": "古老道音穿过岁月，在识海中展开完整传承考验。", "悟道": "心神沉入道台，万物声息化作可以参悟的道痕。",
		"仙魔战场": "战鼓越过天河，阵营令牌与战场贡献同时被记录。", "渡劫心魔": "识海旧念显形，唯有直面执念才能将心魔炼为己用。",
		"合体技": "两道灵息循同心契交汇，术式威能由道缘共同承担。", "仙药培育": "药圃仙雾升腾，根须、药性与成熟周期开始变化。",
		"法宝炼化": "玄火包裹器胚，法宝灵性与主人神识逐步相合。", "天机推演": "星轨与因果线在识海交叠，推演结果同时伴随反噬风险。",
		"天地灵脉": "地底龙脉回应神识，灵气产出与争夺印记随之显现。", "宗门战争": "护山大阵转入战时，物资、士气与领地归属开始结算。",
		"仙缘奇遇": "一缕陌生仙光落在前路，选择将改变后续因果和奖励。", "宇宙星河": "星图在虚空展开，星力坐标与传送航路逐一显现。",
		"灵根进化": "灵根深处的本源被唤醒，纯度、阶段与觉醒方向开始变化。",
	}
	lines := []string{
		fmt.Sprintf("%s：**%s**", verb, config.Name),
		narratives[category],
		"━━━━━━━━━━━",
		fmt.Sprintf("类型：%s · 配置等级：%d · 本次为第%d次", config.Type, config.Level, value),
		"实际效果：" + displayConfigText(config.EffectParams),
		"实际消耗：" + displayConfigText(config.CostMaterials),
		"解锁条件：" + displayConfigText(config.Prerequisite),
		config.Description,
	}
	return strings.Join(lines, "\n")
}

func displayConfigText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "{}" || value == "null" {
		return "未配置"
	}
	var decoded map[string]any
	if json.Unmarshal([]byte(value), &decoded) != nil {
		return value
	}
	labels := map[string]string{
		"power": "威力", "duration": "持续", "growth": "成长倍率", "minimum_level": "最低等级",
		"minimum_realm": "最低境界", "minimum_realm_sequence": "最低大境序位", "minimum_realm_level": "最低境界层数",
		"minimum_combat_power": "最低战力", "minimum_reputation": "最低声望", "minimum_merit": "最低功德",
		"minimum_spirit": "最低神识", "minimum_perception": "最低悟性", "minimum_willpower": "最低意志",
		"minimum_luck": "最低运气", "minimum_dao_heart": "最低道心", "minimum_immortal_affinity": "最低仙缘",
		"minimum_root_quality": "最低灵根纯度", "minimum_mana": "最低法力", "required_root_element": "契合灵根本源",
		"sect_required": "需要宗门", "couple_required": "需要仙侣", "mansion_required": "需要仙府",
		"previous_task": "前序任务", "location": "指定地点",
		"cultivation": "修为", "spirit_stones": "灵石", "silver_coins": "银币", "immortal_jade": "仙金",
		"merit": "功德", "reputation": "声望", "immortal_affinity": "仙缘", "dao_heart": "道心",
		"perception": "悟性", "root_quality": "灵根纯度", "items": "物品", "title": "称号",
		"contribution": "宗门贡献", "sect_funds": "宗门资金", "affinity": "道缘",
		"attack_basis_points": "攻击加成(万分比)", "defense_basis_points": "防御加成(万分比)",
		"health_basis_points": "气血加成(万分比)", "mana_basis_points": "法力加成(万分比)",
		"speed_basis_points": "身法加成(万分比)", "unique_power_index": "独立道力指数",
		"aura_control": "聚灵掌控", "breakthrough_insight": "破境悟性", "special_effect": "特殊道韵",
		"unique_bonus_power": "独立道力", "physical_attack": "物理攻击", "magic_attack": "法术攻击",
		"physical_defense": "物理防御", "magic_defense": "法术防御", "speed": "身法", "mana_cost": "法力消耗",
		"attack": "攻击", "defense": "防御", "health": "气血", "mana": "法力", "all_percent": "全属性百分比",
		"cultivation_percent": "修炼收益百分比", "drop_percent": "掉落收益百分比", "fortune_percent": "机缘概率百分比",
		"pet_percent": "灵兽成长百分比", "forge_percent": "锻造成功率百分比", "joint_attack_percent": "合击威力百分比",
		"dungeon_percent": "副本伤害百分比", "harvest_percent": "灵田收获百分比", "boss_percent": "首领伤害百分比",
		"alchemy_percent": "炼丹成功率百分比", "tribulation_percent": "渡劫成功率百分比",
	}
	keys := make([]string, 0, len(decoded))
	for key := range decoded {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		label := labels[key]
		if label == "" {
			label = key
		}
		unit := ""
		if key == "duration" {
			unit = "秒"
		}
		separator := "×"
		if labels[key] != "" {
			separator = "："
		}
		parts = append(parts, fmt.Sprintf("%s%s%s%s", label, separator, displayConfigValue(decoded[key]), unit))
	}
	return strings.Join(parts, "、")
}

func displayConfigValue(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, fmt.Sprintf("%s×%s", key, displayConfigValue(typed[key])))
		}
		return strings.Join(parts, "、")
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, displayConfigValue(item))
		}
		return strings.Join(parts, "、")
	case bool:
		if typed {
			return "是"
		}
		return "否"
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%d", int64(typed))
		}
		return fmt.Sprintf("%.4f", typed)
	case nil:
		return "无"
	default:
		return fmt.Sprint(typed)
	}
}

func (g *Game) extendedCostStatus(player *model.Player, raw string) (string, []string, error) {
	costs := make(map[string]int64)
	if err := json.Unmarshal([]byte(raw), &costs); err != nil {
		return "", nil, err
	}
	parts := make([]string, 0, len(costs))
	missing := []string{}
	for name, amount := range costs {
		if amount <= 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s×%d", name, amount))
		if name == "灵石" {
			if player.SpiritStones < amount {
				missing = append(missing, fmt.Sprintf("灵石需要%d，当前%d", amount, player.SpiritStones))
			}
			continue
		}
		item, err := g.itemByName(name)
		owned := int64(0)
		if err == nil {
			owned = g.itemQuantity(player.ID, item.ID)
		}
		if err != nil || owned < amount {
			missing = append(missing, fmt.Sprintf("%s需要%d，当前%d", name, amount, owned))
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, "、"), missing, nil
}

func (g *Game) consumeExtendedCost(player *model.Player, raw string) error {
	costs := make(map[string]int64)
	if err := json.Unmarshal([]byte(raw), &costs); err != nil {
		return err
	}
	return g.store.DB.Transaction(func(tx *gorm.DB) error {
		return consumeExtendedCostTx(tx, player.ID, costs)
	})
}

func consumeExtendedCostTx(tx *gorm.DB, playerID uint, costs map[string]int64) error {
	repo := storage.NewPlayerRepository(tx)
	for name, amount := range costs {
		if amount <= 0 {
			continue
		}
		if name == "灵石" {
			result := tx.Model(&model.Player{}).Where("id = ? AND spirit_stones >= ?", playerID, amount).Update("spirit_stones", gorm.Expr("spirit_stones - ?", amount))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errors.New("insufficient spirit stones")
			}
			continue
		}
		var item model.Item
		if err := tx.Where("name = ?", name).First(&item).Error; err != nil {
			return err
		}
		if err := repo.AdjustItem(playerID, item.ID, -amount); err != nil {
			return err
		}
	}
	return nil
}

func extendedMenuAction(category string) string { return strings.TrimSpace(category) + "菜单" }

func extendedSystemCommands(category string) []string {
	commands := make([]string, 0, 6)
	for _, spec := range handler.CommandTable {
		if spec.Category == category && !spec.EventOnly {
			commands = append(commands, spec.Command)
		}
	}
	return commands
}

func (g *Game) executeSpiritualRootEvolution(player *model.Player, command handler.ParsedCommand, system extendedSystem) (GameResult, bool, error) {
	action := system.Actions[command.Spec.ID-extendedCategoryBaseID(command.Spec.Category)]
	if action == "combine" {
		return g.fuseSpiritualRoots(player, command.Arguments)
	}
	if action == "transfer" {
		return g.transferSpiritualRoot(player, command.Arguments)
	}
	config, err := g.extendedConfig(system.Table, extendedConfigArgument(command))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return GameResult{Title: command.Spec.Name + "道藏未载", Content: "灵根传承道统尚未完成载入，本次没有扣除材料、灵石或传承次数。请先查看灵根进化菜单中的开放道统。", Actions: []string{"灵根进化菜单", "灵检", "帮助 灵根进化"}}, true, nil
		}
		return GameResult{}, true, err
	}
	requirementText, unmet, requirementErr := g.prerequisiteStatus(player, config.Prerequisite)
	if requirementErr != nil {
		return GameResult{Title: command.Spec.Name + "配置错误", Content: "前置条件JSON无法解析：\n" + config.Prerequisite}, true, nil
	}
	if action != "inspect" && len(unmet) > 0 {
		return GameResult{Title: command.Spec.Name + "尚未解锁", Content: fmt.Sprintf("灵根进化：%s\n前置：%s\n━━━━━━━━━━━\n未满足：\n- %s", config.Name, requirementText, strings.Join(unmet, "\n- ")), Actions: append(g.prerequisiteActions(unmet), "灵检")}, true, nil
	}
	if dependency := extendedActionDependency(command.Spec.Category, action); action != "inspect" && dependency != "" {
		if g.spiritualRootEvolutionValue(player.ID, dependency) < 1 {
			commandName := extendedActionCommand(command.Spec.Category, system, dependency)
			return GameResult{Title: command.Spec.Name + "尚未解锁", Content: "需要先完成前序步骤：" + commandName, Actions: []string{commandName, "灵检"}}, true, nil
		}
	}
	if action == "inspect" {
		return g.spiritualRootInspect(player, config), true, nil
	}
	evolveValue := g.spiritualRootEvolutionValue(player.ID, "evolve")
	stage := spiritualRootStage(evolveValue)
	awakenCount := g.spiritualRootEvolutionValue(player.ID, "awaken")
	if action == "evolve" {
		if stage >= spiritualRootMaxStage {
			return GameResult{Title: "🌱 灵根本源圆满", Content: fmt.Sprintf("灵根：%s\n本源道阶：%s · 已达普通进化上限\n━━━━━━━━━━━\n十重本源已经合一，继续发送“灵进”不会消耗任何材料。后续成长需通过灵根觉醒、灵根融合、悟道和天地灵脉获得更高层次的本源道力。", player.SpiritualRoot, spiritualRootStageName(stage)), Actions: []string{"灵觉", "灵融", "悟道菜单", "灵脉地图", "灵检"}}, true, nil
		}
		if requiredAwakenings := stage / 2; awakenCount < int64(requiredAwakenings) {
			return GameResult{Title: "🌱 灵根天关未破", Content: fmt.Sprintf("当前：%s · %s\n进化被本源天关阻断。\n━━━━━━━━━━━\n需要完成第%d次灵根觉醒，才能继续向下一重道阶进化。觉醒会重新校验境界、气血、战力和材料，不可无条件连续突破。", player.SpiritualRoot, spiritualRootStageName(stage), requiredAwakenings), Actions: []string{"灵觉", "灵检", "状态", "物品 玄铁"}}, true, nil
		}
		if dynamicUnmet := g.spiritualRootEvolutionGate(player, stage); len(dynamicUnmet) > 0 {
			return GameResult{Title: "🌱 灵根进化条件不足", Content: fmt.Sprintf("目标：%s → %s\n━━━━━━━━━━━\n本重天关未满足：\n- %s\n━━━━━━━━━━━\n条件满足前不会扣除灵石、材料或记录冷却。", spiritualRootStageName(stage), spiritualRootStageName(stage+1), strings.Join(dynamicUnmet, "\n- ")), Actions: []string{"状态", "修炼", "疗伤", "地图", "灵检"}}, true, nil
		}
	}
	if action == "awaken" {
		effective := g.playerWithActiveSkillStats(player)
		requiredStage := 2 + int(awakenCount)*2
		if requiredStage > spiritualRootMaxStage {
			return GameResult{Title: "🌱 灵根觉醒圆满", Content: "五次本源觉醒已经全部完成。后续请通过灵融、悟道与高阶灵脉继续成长。", Actions: []string{"灵融", "悟道菜单", "灵脉地图", "灵检"}}, true, nil
		}
		if stage < requiredStage {
			return GameResult{Title: "🌱 灵根觉醒未解锁", Content: fmt.Sprintf("下一次觉醒需要达到%s，当前仅为%s。\n阶段进度：%d/%d", spiritualRootStageName(requiredStage), spiritualRootStageName(stage), spiritualRootProgress(evolveValue), spiritualRootStageRequirement(stage)), Actions: []string{"灵进", "灵检", "状态"}}, true, nil
		}
		if effective.Health*100 < effective.MaxHealth*80 || effective.Mana*100 < effective.MaxMana*60 {
			return GameResult{Title: "🌱 觉醒状态不足", Content: fmt.Sprintf("觉醒需气血至少80%%、法力至少60%%。\n当前气血：%d/%d · 法力：%d/%d", effective.Health, effective.MaxHealth, effective.Mana, effective.MaxMana), Actions: []string{"疗伤", "冥想", "状态", "灵检"}}, true, nil
		}
	}

	cooldownKey, cooldownDuration := g.spiritualRootCooldown(action)
	if remaining := g.playerCooldownRemaining(player.ID, cooldownKey); remaining > 0 {
		return GameResult{Title: "🌱 本源尚在沉淀", Content: fmt.Sprintf("%s刚完成一次变化，经脉中的本源灵息尚未稳定。\n还需：%s\n━━━━━━━━━━━\n冷却结束前不会扣除任何材料。", player.SpiritualRoot, formatDuration(remaining)), Actions: []string{"灵检", "状态", "地图", "灵根进化菜单"}}, true, nil
	}
	costs, costErr := spiritualRootActionCosts(config.CostMaterials, action, stage)
	if costErr != nil {
		return GameResult{Title: command.Spec.Name + "配置错误", Content: "材料消耗JSON无法解析：\n" + config.CostMaterials}, true, nil
	}
	costText, costMissing := g.spiritualRootCostStatus(player, costs)
	if len(costMissing) > 0 {
		actions := []string{"背包", "货铺", "地图", "合成图鉴", "灵检"}
		for name := range costs {
			if name != "灵石" {
				actions = append(actions, "物品 "+name)
			}
		}
		return GameResult{Title: "🌱 灵根修行材料不足", Content: fmt.Sprintf("道阶：%s\n本次实际消耗：%s\n━━━━━━━━━━━\n缺少：\n- %s\n━━━━━━━━━━━\n可点击对应物品查询地图妖兽、副本、采集和商店来源。", spiritualRootStageName(stage), costText, strings.Join(costMissing, "\n- ")), Actions: actions}, true, nil
	}

	key := spiritualRootEvolutionKey(action)
	currentValue := g.spiritualRootEvolutionValue(player.ID, action)
	value := currentValue + 1
	updates := map[string]any{}
	originGain := int64(0)
	resultLines := []string{fmt.Sprintf("灵根：%s", player.SpiritualRoot), fmt.Sprintf("本源道阶：%s", spiritualRootStageName(stage)), "━━━━━━━━━━━"}
	switch action {
	case "refine":
		baseGain := int64(2 + stage/2)
		if player.RootQuality < 100 {
			remainingCap := int64(100 - player.RootQuality)
			gain := min64(baseGain, remainingCap)
			updates["root_quality"] = gorm.Expr("root_quality + ?", gain)
			originGain = (baseGain - gain) * int64(stage+1)
			resultLines = append(resultLines, "淬炼结果：灵根杂质被真火炼去", fmt.Sprintf("灵根纯度：%d → %d", player.RootQuality, player.RootQuality+int(gain)))
		} else {
			originGain = baseGain * int64(stage+1)
			updates["dao_heart"] = gorm.Expr("MIN(dao_heart + 1, 100)")
			resultLines = append(resultLines, "灵根纯度已满，淬炼之力转化为本源道力与道心感悟。", "道心：+1")
		}
	case "evolve":
		before := spiritualRootStage(value - 1)
		after := spiritualRootStage(value)
		attackGain := int64(stage + 1)
		defenseGain := int64(maxInt(stage/2, 1))
		healthGain := int64(stage * 8)
		manaGain := int64(stage * 4)
		if after > before {
			attackGain *= 2
			defenseGain *= 2
			healthGain *= 2
			manaGain *= 2
		}
		updates["physical_attack"] = gorm.Expr("physical_attack + ?", attackGain)
		updates["magic_attack"] = gorm.Expr("magic_attack + ?", attackGain)
		updates["physical_defense"] = gorm.Expr("physical_defense + ?", defenseGain)
		updates["magic_defense"] = gorm.Expr("magic_defense + ?", defenseGain)
		updates["max_health"] = gorm.Expr("max_health + ?", healthGain)
		updates["health"] = gorm.Expr("health + ?", healthGain)
		updates["max_mana"] = gorm.Expr("max_mana + ?", manaGain)
		updates["mana"] = gorm.Expr("mana + ?", manaGain)
		updates["combat_power"] = gorm.Expr("combat_power + ?", attackGain*4+defenseGain*3+healthGain/5+manaGain/5)
		if player.RootQuality < 100 {
			updates["root_quality"] = gorm.Expr("MIN(root_quality + 1, 100)")
		} else {
			originGain = int64(stage * 10)
		}
		resultLines = append(resultLines,
			fmt.Sprintf("进化：%s → %s", spiritualRootStageName(before), spiritualRootStageName(after)),
			fmt.Sprintf("永久成长：攻法+%d · 双防+%d · 气血+%d · 法力+%d", attackGain, defenseGain, healthGain, manaGain),
			fmt.Sprintf("阶段进度：%d/%d", spiritualRootProgress(value), spiritualRootStageRequirement(after)),
		)
	case "awaken":
		bonus := int64(4 + value*2)
		originGain = bonus * int64(stage)
		luckGain := min64(max64(value, 1), max64(maximumPlayerLuck-normalizedPlayerLuck(player.Luck), 0))
		updates["dao_heart"] = gorm.Expr("MIN(dao_heart + ?, 100)", bonus)
		updates["willpower"] = gorm.Expr("willpower + ?", bonus)
		if luckGain > 0 {
			updates["luck"] = gorm.Expr("CASE WHEN luck + ? > ? THEN ? ELSE luck + ? END", luckGain, maximumPlayerLuck, maximumPlayerLuck, luckGain)
		}
		updates["combat_power"] = gorm.Expr("combat_power + ?", bonus*8)
		resultLines = append(resultLines, fmt.Sprintf("第%d次本源觉醒完成", value), fmt.Sprintf("永久成长：道心+%d · 意志+%d · 运气+%d（%d/%d）", bonus, bonus, luckGain, normalizedPlayerLuck(player.Luck)+luckGain, maximumPlayerLuck))
	default:
		resultLines = append(resultLines, extendedResultText(command.Spec.Category, action, config, value))
	}
	if originGain > 0 {
		resultLines = append(resultLines, fmt.Sprintf("本源道力：+%d", originGain))
	}
	resultLines = append(resultLines, "━━━━━━━━━━━", "实际消耗："+costText)
	if err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := consumeExtendedCostTx(tx, player.ID, costs); err != nil {
			return err
		}
		if len(updates) > 0 {
			if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(updates).Error; err != nil {
				return err
			}
		}
		if err := upsertPlayerValueTx(tx, player.ID, key, strconv.FormatInt(value, 10), nil); err != nil {
			return err
		}
		if originGain > 0 {
			originKey := "spiritual_root.origin_power"
			originValue := playerValueIntTx(tx, player.ID, originKey, 0) + originGain
			if err := upsertPlayerValueTx(tx, player.ID, originKey, strconv.FormatInt(originValue, 10), nil); err != nil {
				return err
			}
		}
		if cooldownKey != "" && cooldownDuration > 0 {
			until := time.Now().Add(cooldownDuration)
			return upsertPlayerValueTx(tx, player.ID, cooldownKey, until.Format(time.RFC3339Nano), &until)
		}
		return nil
	}); err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "🌱 " + command.Spec.Name, Content: strings.Join(resultLines, "\n"), ImageURL: config.ImageURL, Actions: []string{"灵检", "灵进", "灵觉", "状态", "物品 玄铁", "灵根进化菜单"}}, true, nil
}

const spiritualRootMaxStage = 10

func spiritualRootStageRequirement(stage int) int { return maxInt(stage+2, 3) }

func spiritualRootProgression(value int64) (int, int) {
	remaining := maxInt(int(value), 0)
	stage := 1
	for stage < spiritualRootMaxStage {
		required := spiritualRootStageRequirement(stage)
		if remaining < required {
			return stage, remaining
		}
		remaining -= required
		stage++
	}
	return spiritualRootMaxStage, 0
}

func spiritualRootStage(value int64) int {
	stage, _ := spiritualRootProgression(value)
	return stage
}

func spiritualRootProgress(value int64) int {
	_, progress := spiritualRootProgression(value)
	return progress
}

func spiritualRootStageName(stage int) string {
	names := []string{"初醒灵胚", "洗髓灵脉", "凝纹灵骨", "化相灵胎", "五行道种", "归元道脉", "天衍道胎", "法相本源", "混元灵域", "太初道根"}
	stage = maxInt(minInt(stage, len(names)), 1)
	return names[stage-1]
}

func (g *Game) spiritualRootEvolutionGate(player *model.Player, stage int) []string {
	effective := g.playerWithActiveSkillStats(player)
	requiredSequence := 1 + (stage-1)/2
	requiredLayer := 1
	if stage%2 == 0 {
		requiredLayer = 5
	}
	requiredPower := int64(60 + stage*35)
	requiredQuality := minInt(10+stage*8, 100)
	var realm model.Realm
	_ = g.store.DB.First(&realm, player.RealmID).Error
	var requiredRealm model.Realm
	requiredRealmName := fmt.Sprintf("第%d大境", requiredSequence)
	if g.store.DB.Where("sequence = ?", requiredSequence).First(&requiredRealm).Error == nil {
		requiredRealmName = requiredRealm.Name
	}
	var unmet []string
	if realm.Sequence < requiredSequence || (realm.Sequence == requiredSequence && player.RealmLevel < requiredLayer) {
		unmet = append(unmet, fmt.Sprintf("境界需达到%s·%d层，当前%s·%d层", requiredRealmName, requiredLayer, player.RealmName, player.RealmLevel))
	}
	if player.CombatPower < requiredPower {
		unmet = append(unmet, fmt.Sprintf("战力需%d，当前%d", requiredPower, player.CombatPower))
	}
	if player.RootQuality < requiredQuality {
		unmet = append(unmet, fmt.Sprintf("灵根纯度需%d，当前%d", requiredQuality, player.RootQuality))
	}
	if effective.Health*100 < effective.MaxHealth*70 {
		unmet = append(unmet, fmt.Sprintf("气血需至少70%%，当前%d/%d", effective.Health, effective.MaxHealth))
	}
	if effective.Mana*100 < effective.MaxMana*50 {
		unmet = append(unmet, fmt.Sprintf("法力需至少50%%，当前%d/%d", effective.Mana, effective.MaxMana))
	}
	return unmet
}

func spiritualRootActionCosts(raw, action string, stage int) (map[string]int64, error) {
	costs := make(map[string]int64)
	if strings.TrimSpace(raw) != "" && strings.TrimSpace(raw) != "{}" {
		if err := json.Unmarshal([]byte(raw), &costs); err != nil {
			return nil, err
		}
	}
	if len(costs) == 0 {
		costs["灵石"] = 5
	}
	multiplier := int64(1)
	if action == "evolve" || action == "awaken" {
		multiplier += int64((maxInt(stage, 1) - 1) / 3)
	}
	for name, amount := range costs {
		if amount > 0 {
			costs[name] = amount * multiplier
		}
	}
	return costs, nil
}

func (g *Game) spiritualRootCostStatus(player *model.Player, costs map[string]int64) (string, []string) {
	names := make([]string, 0, len(costs))
	for name := range costs {
		names = append(names, name)
	}
	sort.Strings(names)
	var parts, missing []string
	for _, name := range names {
		amount := costs[name]
		if amount <= 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s×%d", name, amount))
		if name == "灵石" {
			if player.SpiritStones < amount {
				missing = append(missing, fmt.Sprintf("灵石需要%d，当前%d", amount, player.SpiritStones))
			}
			continue
		}
		item, err := g.itemByName(name)
		owned := int64(0)
		if err == nil {
			owned = g.itemQuantity(player.ID, item.ID)
		}
		if err != nil || owned < amount {
			missing = append(missing, fmt.Sprintf("%s需要%d，当前%d", name, amount, owned))
		}
	}
	return strings.Join(parts, "、"), missing
}

func (g *Game) spiritualRootCooldown(action string) (string, time.Duration) {
	switch action {
	case "evolve":
		minutes := maxInt(int(g.settingFloat("spiritual_root.evolve_cooldown_minutes", 10)), 1)
		return "cooldown.spiritual_root.evolve", time.Duration(minutes) * time.Minute
	case "awaken":
		hours := maxInt(int(g.settingFloat("spiritual_root.awaken_cooldown_hours", 24)), 1)
		return "cooldown.spiritual_root.awaken", time.Duration(hours) * time.Hour
	default:
		return "", 0
	}
}

func (g *Game) playerCooldownRemaining(playerID uint, key string) time.Duration {
	if key == "" {
		return 0
	}
	value, err := g.playerValue(playerID, key)
	if err != nil {
		return 0
	}
	until, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || !until.After(time.Now()) {
		return 0
	}
	return time.Until(until)
}

func upsertPlayerValueTx(tx *gorm.DB, playerID uint, key, value string, expiresAt *time.Time) error {
	row := model.PlayerValue{PlayerID: playerID, Key: key, Value: value, ExpiresAt: expiresAt}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "player_id"}, {Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "expires_at", "updated_at"}),
	}).Create(&row).Error
}

func playerValueTx(tx *gorm.DB, playerID uint, key string) (string, error) {
	var row model.PlayerValue
	if err := tx.Where("player_id = ? AND key = ?", playerID, key).First(&row).Error; err != nil {
		return "", err
	}
	if row.ExpiresAt != nil && !row.ExpiresAt.After(time.Now()) {
		return "", gorm.ErrRecordNotFound
	}
	return row.Value, nil
}

func playerValueIntTx(tx *gorm.DB, playerID uint, key string, fallback int64) int64 {
	var row model.PlayerValue
	if err := tx.Where("player_id = ? AND key = ?", playerID, key).First(&row).Error; err != nil {
		return fallback
	}
	value, err := strconv.ParseInt(strings.TrimSpace(row.Value), 10, 64)
	if err != nil {
		return fallback
	}
	return value
}

func (g *Game) spiritualRootInspect(player *model.Player, config model.GameplayConfigBase) GameResult {
	value := g.spiritualRootEvolutionValue(player.ID, "evolve")
	awakened := g.spiritualRootEvolutionValue(player.ID, "awaken") > 0
	awakenText := "未觉醒"
	if awakened {
		awakenText = "已觉醒"
	}
	stage := spiritualRootStage(value)
	return GameResult{Title: "🌱 灵根检测", Content: g.spiritualRootGuide(player) + fmt.Sprintf("\n━━━━━━━━━━━\n本源道阶：%s（第%d/10重）\n阶段进度：%d/%d\n本源道力：%d\n觉醒状态：%s\n当前进化配置：%s\n%s", spiritualRootStageName(stage), stage, spiritualRootProgress(value), spiritualRootStageRequirement(stage), g.playerValueInt(player.ID, "spiritual_root.origin_power", 0), awakenText, config.Name, config.Description), Actions: []string{"灵淬", "灵进", "灵觉", "灵融", "灵根合成", "灵根道种", "灵根进化菜单"}}
}

func spiritualRootEvolutionKey(action string) string {
	return "extended.spiritual_root_evolution_configs.progress." + strings.TrimSpace(action)
}

// Older releases stored progress under whichever generated config happened to
// be selected. Merge the highest legacy value into one player-wide progression
// key so inspect, evolve and awaken always see the same stage.
func (g *Game) spiritualRootEvolutionValue(playerID uint, action string) int64 {
	action = strings.TrimSpace(action)
	canonicalKey := spiritualRootEvolutionKey(action)
	best := g.playerValueInt(playerID, canonicalKey, 0)
	var rows []model.PlayerValue
	pattern := "extended.spiritual_root_evolution_configs.%." + action
	if g.store.DB.Where("player_id = ? AND key LIKE ?", playerID, pattern).Find(&rows).Error == nil {
		for _, row := range rows {
			value, err := strconv.ParseInt(strings.TrimSpace(row.Value), 10, 64)
			if err == nil && value > best {
				best = value
			}
		}
	}
	if best > g.playerValueInt(playerID, canonicalKey, 0) {
		_ = g.setPlayerValueInt(playerID, canonicalKey, best)
	}
	return best
}

func extendedCategoryBaseID(category string) int {
	order := []string{"阵法", "符箓", "傀儡", "秘境争夺", "传承", "悟道", "仙魔战场", "灵根进化", "渡劫心魔", "合体技", "仙药培育", "法宝炼化", "天机推演", "天地灵脉", "宗门战争", "仙缘奇遇", "宇宙星河"}
	base := 141
	for _, name := range order {
		if name == category {
			return base
		}
		if name == "宇宙星河" {
			base += 4
		} else {
			base += 6
		}
	}
	return -1
}
