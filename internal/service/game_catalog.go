package service

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"xianlv/internal/handler"
	"xianlv/internal/model"
)

type catalogSection struct {
	Name        string
	Icon        string
	Description string
}

var catalogSections = []catalogSection{
	{"物品", "🎒", "全部仙物、用途与来源"}, {"材料", "⛏️", "炼丹、炼器、阵法与渡劫材料"},
	{"丹药", "🧪", "真实药效、使用限制与来源"}, {"丹方", "📜", "炼丹材料、成丹与成功率"},
	{"合成", "🔧", "合成材料、前置与产物"}, {"装备", "🛡️", "十槽装备、器型、套装与器谱"},
	{"功法", "📖", "功法定位、境界前置与招式参数"}, {"灵根", "🌱", "千种灵根与独立加成"},
	{"灵脉", "🔆", "千条灵脉、契合本源与打坐前置"}, {"境界", "🧘", "千个大境与每境十层规则"},
	{"地图", "🗺️", "州域地点、路线、资源与前置"}, {"NPC", "🧑", "各地人物与所在地点"},
	{"妖兽", "👾", "普通妖灵、战力与掉落"}, {"首领", "👑", "区域首领、狂暴与奖励"},
	{"灵兽", "🐉", "灵兽成长、忠诚、进化与战力"}, {"副本", "🏯", "难度、体力、次数与奖励"},
	{"任务", "📜", "任务链、前置、目标与奖励"}, {"称号", "🏅", "成就尊号、属性与佩戴条件"},
	{"礼包", "🎁", "礼包内容与获取途径"}, {"种子", "🌾", "灵田种子、成熟时间与产物"},
	{"商城", "🏪", "各币种商品、价格与用途"}, {"活动", "🎯", "当前及历史活动状态"},
	{"竞技段位", "🏆", "千阶段位、积分与俸禄"}, {"事件", "🌌", "奇遇、劫难、选择与触发条件"},
	{"掉落", "💠", "地图、妖兽、首领与副本掉落池"}, {"阵法", "☯️", "阵图、效果、消耗与前置"},
	{"符箓", "🧿", "符箓效果、制符材料与前置"}, {"傀儡", "🪆", "傀儡炼制、成长与出战条件"},
	{"秘境争夺", "⚔️", "秘境占领、收益与争夺规则"}, {"传承", "📚", "道统传承、融合与觉醒"},
	{"悟道", "🪷", "道痕、悟道台与参悟条件"}, {"仙魔战场", "⚔️", "阵营战场、军功与奖惩"},
	{"灵根进化", "🌱", "淬炼、进化、觉醒与传承道藏"}, {"心魔", "👁️", "心魔试炼、镇压与反噬"},
	{"合体技", "💞", "仙侣合击、强化与融合"}, {"仙药", "🌿", "仙药培育、嫁接与药性"},
	{"法宝炼化", "🔥", "炼化、开光、蕴养与认主"}, {"天机", "🔮", "推演、预警、改命与反噬"},
	{"灵脉道藏", "🔆", "灵脉占据、争夺与融合配置"}, {"宗门战争", "🏯", "宣战、备战、结盟与领地"},
	{"仙缘奇遇", "✨", "仙缘阶段、选择、奖励与惩罚"}, {"星河", "🌌", "星图、星魂与星域传送"},
}

var extendedCatalogTables = map[string]string{
	"阵法": "formation_configs", "符箓": "talisman_configs", "傀儡": "puppet_configs",
	"秘境争夺": "secret_realm_conflict_configs", "传承": "inheritance_configs", "悟道": "dao_insight_configs",
	"仙魔战场": "immortal_demon_battlefield_configs", "灵根进化": "spiritual_root_evolution_configs",
	"心魔": "inner_demon_configs", "合体技": "couple_combination_skill_configs", "仙药": "immortal_herb_configs",
	"法宝炼化": "artifact_refinement_configs", "天机": "destiny_deduction_configs", "灵脉道藏": "leyline_configs",
	"宗门战争": "sect_war_configs", "仙缘奇遇": "immortal_encounter_configs", "星河": "star_realm_configs",
}

func normalizeCatalogCategory(value string) string {
	value = strings.TrimSpace(strings.TrimSuffix(value, "图鉴"))
	aliases := map[string]string{
		"法宝": "装备", "器谱": "装备", "装备系统": "装备", "丹方数据": "丹方", "药方": "丹方",
		"灵兽空间": "灵兽", "秘境": "秘境争夺", "仙魔": "仙魔战场", "星河界": "星河",
		"NPC": "NPC", "npc": "NPC", "人物": "NPC", "Boss": "首领", "BOSS": "首领", "boss": "首领",
		"段位": "竞技段位", "竞技": "竞技段位", "世界灵脉": "灵脉", "天地灵脉": "灵脉道藏",
	}
	if canonical := aliases[value]; canonical != "" {
		return canonical
	}
	for _, row := range catalogSections {
		if row.Name == value {
			return row.Name
		}
	}
	return ""
}

func parseCatalogFilterPage(raw string) (string, int) {
	parts := strings.Fields(strings.TrimSpace(raw))
	page := 1
	if len(parts) > 0 {
		if parsed, err := strconv.Atoi(parts[len(parts)-1]); err == nil && parsed > 0 {
			page = parsed
			parts = parts[:len(parts)-1]
		}
	}
	return strings.Join(parts, " "), page
}

func catalogPageAction(command, filter string, page int) string {
	if strings.TrimSpace(filter) == "" {
		return fmt.Sprintf("%s %d", command, page)
	}
	return fmt.Sprintf("%s %s %d", command, filter, page)
}

func (g *Game) executeCatalog(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 1120:
		return g.catalogHub(player, command.RawArguments)
	case 1121:
		category := strings.TrimSuffix(command.Spec.Command, "图鉴")
		raw := command.RawArguments
		if command.Spec.Command == "道藏图鉴" {
			parts := strings.Fields(raw)
			if len(parts) == 0 {
				return g.catalogHub(player, "")
			}
			category = parts[0]
			raw = strings.Join(parts[1:], " ")
		}
		return g.catalogList(player, normalizeCatalogCategory(category), raw)
	case 1122:
		category := "装备"
		name := command.RawArguments
		if command.Spec.Command == "图鉴详情" {
			parts := strings.Fields(command.RawArguments)
			if len(parts) < 2 {
				return GameResult{Title: "🪪 图鉴详情", Content: "请输入：`图鉴详情 类别 完整名称`。", Actions: []string{"图鉴菜单"}}, true, nil
			}
			category = normalizeCatalogCategory(parts[0])
			name = strings.Join(parts[1:], " ")
		}
		return g.catalogDetails(player, category, name)
	default:
		return GameResult{}, false, nil
	}
}

func (g *Game) catalogHub(player *model.Player, raw string) (GameResult, bool, error) {
	raw = strings.TrimSpace(raw)
	parts := strings.Fields(raw)
	if len(parts) > 0 {
		if category := normalizeCatalogCategory(parts[0]); category != "" {
			return g.catalogList(player, category, strings.Join(parts[1:], " "))
		}
	}
	page := 1
	if raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return GameResult{Title: "🪪 图鉴类别未收录", Content: "没有找到“" + raw + "”图鉴。发送“图鉴菜单”查看全部类别，也可发送“查询 完整名称”跨图鉴查找。", Actions: []string{"图鉴菜单", "查询"}}, true, nil
		}
		page = parsed
	}
	const pageSize = 10
	pages := maxInt((len(catalogSections)+pageSize-1)/pageSize, 1)
	if page > pages {
		page = pages
	}
	start := (page - 1) * pageSize
	end := minInt(start+pageSize, len(catalogSections))
	lines := []string{fmt.Sprintf("第%d/%d页 · 共%d类道藏", page, pages, len(catalogSections)), "每类均有独立分页、条目详情、前置、用途与来源反查。", "━━━━━━━━━━━"}
	actions := []string{"查询"}
	for _, row := range catalogSections[start:end] {
		count := g.catalogCount(row.Name)
		lines = append(lines, fmt.Sprintf("%s %s图鉴 · 收录%d条\n%s", row.Icon, row.Name, count, row.Description), "━━━━━━━")
		actions = append(actions, "图鉴 "+row.Name)
	}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("图鉴菜单 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("图鉴菜单 %d", page+1))
	}
	return GameResult{Title: "🪪 仙尘万象图鉴", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) catalogCount(category string) int64 {
	var count int64
	switch category {
	case "物品":
		_ = g.store.DB.Model(&model.Item{}).Count(&count).Error
	case "材料":
		_ = g.store.DB.Model(&model.Item{}).Where("category_name IN ?", []string{"材料", "灵草", "矿石", "任务物品", "仙府资源", "宗门资源", "战场物资", "星河奇珍"}).Count(&count).Error
	case "丹药":
		_ = g.store.DB.Model(&model.Item{}).Where("category_name = ?", "丹药").Count(&count).Error
	case "丹方":
		_ = g.store.DB.Model(&model.AlchemyRecipe{}).Where("enabled = ?", true).Count(&count).Error
	case "合成":
		_ = g.store.DB.Model(&model.SynthesisRecipe{}).Where("enabled = ?", true).Count(&count).Error
	case "装备":
		_ = g.store.DB.Model(&model.ArtifactTemplate{}).Where("enabled = ?", true).Count(&count).Error
	case "功法":
		_ = g.store.DB.Model(&model.Skill{}).Where("rarity <> ? OR id IN (SELECT skill_id FROM skill_publications WHERE published = ?)", "自创", true).Count(&count).Error
	case "灵根":
		_ = g.store.DB.Model(&model.SpiritualRootTemplate{}).Where("enabled = ?", true).Count(&count).Error
	case "灵脉":
		_ = g.store.DB.Model(&model.WorldLeyline{}).Where("enabled = ?", true).Count(&count).Error
	case "境界":
		_ = g.store.DB.Model(&model.Realm{}).Count(&count).Error
	case "地图":
		_ = g.store.DB.Model(&model.WorldLocation{}).Where("enabled = ?", true).Count(&count).Error
	case "NPC":
		var rows []model.WorldLocation
		_ = g.store.DB.Where("enabled = ?", true).Select("npc_json").Find(&rows).Error
		for _, row := range rows {
			count += int64(len(decodeTextList(row.NPCJSON)))
		}
	case "妖兽":
		_ = g.store.DB.Model(&model.WorldLocation{}).Where("enabled = ? AND monster_name <> ''", true).Count(&count).Error
	case "首领":
		_ = g.store.DB.Model(&model.WorldLocation{}).Where("enabled = ? AND boss_name <> ''", true).Count(&count).Error
	case "灵兽":
		_ = g.store.DB.Model(&model.PetTemplate{}).Where("enabled = ?", true).Count(&count).Error
	case "副本":
		_ = g.store.DB.Model(&model.Dungeon{}).Where("enabled = ?", true).Count(&count).Error
	case "任务":
		_ = g.store.DB.Model(&model.TaskTemplate{}).Where("enabled = ?", true).Count(&count).Error
	case "称号":
		_ = g.store.DB.Model(&model.Title{}).Where("enabled = ?", true).Count(&count).Error
	case "礼包":
		_ = g.store.DB.Model(&model.Item{}).Where("effect_func = ?", "open_gift_pack").Count(&count).Error
	case "种子":
		_ = g.store.DB.Model(&model.Item{}).Where("category_name = ?", "种子").Count(&count).Error
	case "商城":
		_ = g.store.DB.Model(&model.ShopEntry{}).Where("enabled = ?", true).Count(&count).Error
	case "活动":
		_ = g.store.DB.Model(&model.Activity{}).Count(&count).Error
	case "竞技段位":
		_ = g.store.DB.Model(&model.ArenaTier{}).Where("enabled = ?", true).Count(&count).Error
	case "事件":
		_ = g.store.DB.Model(&model.Event{}).Where("enabled = ?", true).Count(&count).Error
	case "掉落":
		_ = g.store.DB.Model(&model.DropPool{}).Where("enabled = ?", true).Count(&count).Error
	default:
		if table := extendedCatalogTables[category]; table != "" {
			_ = g.store.DB.Table(table).Where("status <> ? OR status IS NULL", "disabled").Count(&count).Error
		}
	}
	return count
}

type simpleCatalogEntry struct {
	Name    string
	Summary string
	Detail  string
}

func (g *Game) catalogList(player *model.Player, category, raw string) (GameResult, bool, error) {
	if category == "" {
		return g.catalogHub(player, "")
	}
	switch category {
	case "装备":
		return g.equipmentCatalog(player, raw)
	case "礼包":
		return g.giftPackList(player, raw)
	case "合成":
		return g.synthesisCatalog(player, raw)
	case "灵根":
		return g.spiritualRootCatalog(player, raw)
	case "灵脉":
		return g.worldLeylineMap(player, raw)
	case "竞技段位":
		return g.arenaTierInfo(player, raw)
	case "NPC":
		return g.npcCatalog(raw)
	}
	if table := extendedCatalogTables[category]; table != "" {
		return g.extendedCatalogList(category, table, raw)
	}
	filter, page := parseCatalogFilterPage(raw)
	const pageSize = 6
	entries := []simpleCatalogEntry{}
	queryLike := "%" + filter + "%"
	switch category {
	case "物品", "材料", "丹药", "种子":
		query := g.store.DB.Model(&model.Item{})
		switch category {
		case "材料":
			query = query.Where("category_name IN ?", []string{"材料", "灵草", "矿石", "任务物品", "仙府资源", "宗门资源", "战场物资", "星河奇珍"})
		case "丹药":
			query = query.Where("category_name = ?", "丹药")
		case "种子":
			query = query.Where("category_name = ?", "种子")
		}
		if filter != "" {
			query = query.Where("name LIKE ? OR category_name LIKE ? OR rarity_name LIKE ? OR effect_type LIKE ?", queryLike, queryLike, queryLike, queryLike)
		}
		var rows []model.Item
		_ = query.Order("category_name,rarity_id,id").Find(&rows).Error
		for _, row := range rows {
			entries = append(entries, simpleCatalogEntry{row.Name, fmt.Sprintf("%s · %s · %s", displayOr(row.CategoryName, "未分类"), displayOr(row.RarityName, "凡品"), itemEffectSummary(row, 1)), displayOr(row.Description, "暂无说明")})
		}
	case "丹方":
		query := g.store.DB.Where("enabled = ?", true)
		if filter != "" {
			query = query.Where("name LIKE ? OR output_name LIKE ? OR description LIKE ?", queryLike, queryLike, queryLike)
		}
		var rows []model.AlchemyRecipe
		_ = query.Order("id").Find(&rows).Error
		for _, row := range rows {
			entries = append(entries, simpleCatalogEntry{row.Name, fmt.Sprintf("产物：%s · 成功率%.1f%% · 材料：%s", row.OutputName, row.SuccessRate*100, displayConfigText(row.MaterialsJSON)), row.Description})
		}
	case "功法":
		query := g.store.DB.Model(&model.Skill{}).Where("rarity <> ? OR id IN (SELECT skill_id FROM skill_publications WHERE published = ?)", "自创", true)
		if filter != "" {
			query = query.Where("name LIKE ? OR type = ? OR rarity = ? OR realm_required LIKE ?", queryLike, filter, filter, queryLike)
		}
		var rows []model.Skill
		_ = query.Order("rarity,type,id").Find(&rows).Error
		for _, row := range rows {
			entries = append(entries, simpleCatalogEntry{row.Name, fmt.Sprintf("%s · %s · 前置%s · %s", row.Type, row.Rarity, row.RealmRequired, displayConfigText(row.EffectJSON)), row.Description})
		}
	case "境界":
		query := g.store.DB.Model(&model.Realm{})
		if filter != "" {
			query = query.Where("name LIKE ? OR description LIKE ?", queryLike, queryLike)
		}
		var rows []model.Realm
		_ = query.Order("sequence").Find(&rows).Error
		for _, row := range rows {
			entries = append(entries, simpleCatalogEntry{row.Name, fmt.Sprintf("第%d境 · 每境十层 · 圆满需求%d修为 · 属性倍率×%.3f", row.Sequence, row.RequiredCultivation, row.AttributeMultiplier), row.Description})
		}
	case "地图", "妖兽", "首领":
		query := g.store.DB.Where("enabled = ?", true)
		if category == "妖兽" {
			query = query.Where("monster_name <> ''")
		} else if category == "首领" {
			query = query.Where("boss_name <> ''")
		}
		if filter != "" {
			query = query.Where("region = ? OR name LIKE ? OR monster_name LIKE ? OR boss_name LIKE ?", filter, queryLike, queryLike, queryLike)
		}
		var rows []model.WorldLocation
		_ = query.Order("minimum_realm_sequence,sort_order,id").Find(&rows).Error
		for _, row := range rows {
			switch category {
			case "地图":
				entries = append(entries, simpleCatalogEntry{row.Name, fmt.Sprintf("%s · %s · 地图前置%s · 体力%d", row.Region, row.Description, g.locationRealmRequirement(row), row.StaminaCost), fmt.Sprintf("妖兽%s · 首领%s · 采集%s×%d", displayOr(row.MonsterName, "无"), displayOr(row.BossName, "无"), displayOr(row.ResourceName, "无"), row.ResourceQuantity)})
			case "妖兽":
				entries = append(entries, simpleCatalogEntry{row.MonsterName, fmt.Sprintf("%s·%s · 战力%d · 遭遇率%.1f%%", row.Region, row.Name, row.MonsterPower, row.MonsterEncounterRate*100), "掉落：" + displayConfigText(row.MonsterRewardJSON)})
			case "首领":
				entries = append(entries, simpleCatalogEntry{row.BossName, fmt.Sprintf("%s·%s · 战力%d · 刷新%d分钟", row.Region, row.Name, row.BossPower, row.BossCooldownMinutes), "奖励：" + displayConfigText(row.BossRewardJSON)})
			}
		}
	case "灵兽":
		query := g.store.DB.Where("enabled = ?", true)
		if filter != "" {
			query = query.Where("name LIKE ? OR evolution_target LIKE ?", queryLike, queryLike)
		}
		var rows []model.PetTemplate
		_ = query.Order("initial_power,id").Find(&rows).Error
		for _, row := range rows {
			entries = append(entries, simpleCatalogEntry{row.Name, fmt.Sprintf("初始战力%d · 每级成长%d · 忠诚日耗%d", row.InitialPower, row.GrowthPerLevel, row.LoyaltyDecay), fmt.Sprintf("进化：%s → %s", displayConfigText(row.EvolutionCondition), row.EvolutionTarget)})
		}
	case "副本":
		query := g.store.DB.Where("enabled = ?", true)
		if filter != "" {
			query = query.Where("difficulty = ? OR name LIKE ?", filter, queryLike)
		}
		var rows []model.Dungeon
		_ = query.Order("recommended_power,id").Find(&rows).Error
		for _, row := range rows {
			entries = append(entries, simpleCatalogEntry{row.Name, fmt.Sprintf("%s · 推荐战力%d · 体力%d · 每日%d次", row.Difficulty, row.RecommendedPower, row.StaminaCost, row.DailyLimit), "奖励：" + displayConfigText(row.RewardPoolJSON)})
		}
	case "任务":
		query := g.store.DB.Where("enabled = ?", true)
		if filter != "" {
			query = query.Where("type = ? OR name LIKE ? OR description LIKE ?", filter, queryLike, queryLike)
		}
		var rows []model.TaskTemplate
		_ = query.Order("type,id").Find(&rows).Error
		for _, row := range rows {
			requirement, _, _ := g.prerequisiteStatus(player, row.PrerequisiteJSON)
			entries = append(entries, simpleCatalogEntry{row.Name, fmt.Sprintf("%s · 前置：%s · 目标：%s", row.Type, requirement, taskObjectiveText(row.ObjectiveJSON)), "奖励：" + taskRewardText(row)})
		}
	case "称号":
		query := g.store.DB.Where("enabled = ?", true)
		if filter != "" {
			query = query.Where("type = ? OR name LIKE ? OR condition LIKE ?", filter, queryLike, queryLike)
		}
		var rows []model.Title
		_ = query.Order("type,id").Find(&rows).Error
		for _, row := range rows {
			entries = append(entries, simpleCatalogEntry{row.Name, fmt.Sprintf("%s · 属性：%s", row.Type, displayConfigText(row.AttributeBonus)), "解锁：" + row.Condition})
		}
	case "商城":
		query := g.store.DB.Where("enabled = ?", true)
		if filter != "" {
			query = query.Where("currency = ? OR item_name LIKE ?", filter, queryLike)
		}
		var rows []model.ShopEntry
		_ = query.Order("currency,sort,id").Find(&rows).Error
		for _, row := range rows {
			entries = append(entries, simpleCatalogEntry{row.ItemName, fmt.Sprintf("%d%s · 常设%s", row.Price, row.Currency, map[bool]string{true: "不限购", false: fmt.Sprintf("限购%d", row.PurchaseLimit)}[row.PurchaseLimit == 0]), "购买后收入乾坤袋，不会自动使用。"})
		}
	case "活动":
		query := g.store.DB.Model(&model.Activity{})
		if filter != "" {
			query = query.Where("type = ? OR status = ? OR name LIKE ?", filter, filter, queryLike)
		}
		var rows []model.Activity
		_ = query.Order("starts_at DESC,id DESC").Find(&rows).Error
		for _, row := range rows {
			entries = append(entries, simpleCatalogEntry{row.Name, fmt.Sprintf("%s · %s · %s至%s", row.Type, row.Status, row.StartsAt.Format("01-02"), row.EndsAt.Format("01-02")), row.Effect})
		}
	case "事件":
		query := g.store.DB.Where("enabled = ?", true)
		if filter != "" {
			query = query.Where("type = ? OR name LIKE ? OR description LIKE ?", filter, queryLike, queryLike)
		}
		var rows []model.Event
		_ = query.Order("type,id").Find(&rows).Error
		for _, row := range rows {
			entries = append(entries, simpleCatalogEntry{row.Name, fmt.Sprintf("%s · 基础触发率%.2f%% · 条件：%s", row.Type, row.Probability*100, displayConfigText(row.ConditionJSON)), row.Description + " · 奖励/惩罚：" + displayConfigText(row.RewardJSON)})
		}
	case "掉落":
		query := g.store.DB.Where("enabled = ?", true)
		if filter != "" {
			query = query.Where("source_type = ? OR source_name LIKE ? OR name LIKE ?", filter, queryLike, queryLike)
		}
		var rows []model.DropPool
		_ = query.Order("source_type,id").Find(&rows).Error
		for _, row := range rows {
			var itemCount int64
			_ = g.store.DB.Model(&model.DropEntry{}).Where("drop_pool_id = ?", row.ID).Count(&itemCount).Error
			entries = append(entries, simpleCatalogEntry{row.Name, fmt.Sprintf("%s · %s · 收录%d种掉落", row.SourceType, row.SourceName, itemCount), "发送详情可查看每种物品的权重和数量范围。"})
		}
	default:
		return GameResult{Title: "🪪 图鉴尚未接入", Content: category + "图鉴尚未完成数据接入。", Actions: []string{"图鉴菜单", "提交BUG 图鉴：类别无法打开；现象：" + category + "无数据；期望：显示分页图鉴"}}, true, nil
	}
	return buildSimpleCatalogResult(category, filter, page, pageSize, entries), true, nil
}

func buildSimpleCatalogResult(category, filter string, page, pageSize int, entries []simpleCatalogEntry) GameResult {
	pages := maxInt((len(entries)+pageSize-1)/pageSize, 1)
	if page > pages {
		page = pages
	}
	start := minInt((page-1)*pageSize, len(entries))
	end := minInt(start+pageSize, len(entries))
	lines := []string{fmt.Sprintf("第%d/%d页 · 共%d条", page, pages, len(entries)), "━━━━━━━━━━━"}
	actions := []string{"图鉴菜单", "查询"}
	for _, row := range entries[start:end] {
		lines = append(lines, "【"+row.Name+"】", row.Summary, row.Detail, "━━━━━━━")
		actions = append(actions, "图鉴详情 "+category+" "+row.Name)
	}
	if len(entries) == 0 {
		lines = append(lines, "当前筛选下没有收录条目。")
	}
	command := category + "图鉴"
	if category == "材料" || category == "事件" || category == "掉落" || category == "境界" {
		command = "图鉴 " + category
	}
	if page > 1 {
		actions = append(actions, catalogPageAction(command, filter, page-1))
	}
	if page < pages {
		actions = append(actions, catalogPageAction(command, filter, page+1))
	}
	return GameResult{Title: catalogIcon(category) + " " + category + "图鉴", Content: strings.Join(lines, "\n"), Actions: actions}
}

func catalogIcon(category string) string {
	for _, row := range catalogSections {
		if row.Name == category {
			return row.Icon
		}
	}
	return "🪪"
}

func (g *Game) npcCatalog(raw string) (GameResult, bool, error) {
	filter, page := parseCatalogFilterPage(raw)
	var locations []model.WorldLocation
	query := g.store.DB.Where("enabled = ? AND npc_json <> ''", true)
	if filter != "" {
		like := "%" + filter + "%"
		query = query.Where("region = ? OR name LIKE ? OR npc_json LIKE ?", filter, like, like)
	}
	if err := query.Order("minimum_realm_sequence,sort_order,id").Find(&locations).Error; err != nil {
		return GameResult{}, true, err
	}
	entries := []simpleCatalogEntry{}
	for _, location := range locations {
		for _, npc := range decodeTextList(location.NPCJSON) {
			entries = append(entries, simpleCatalogEntry{npc, fmt.Sprintf("%s·%s · %s", location.Region, location.Name, g.locationRealmRequirement(location)), "可前往当地发送“对话 " + npc + "；人物会按当地任务链回应。"})
		}
	}
	return buildSimpleCatalogResult("NPC", filter, page, 6, entries), true, nil
}

func (g *Game) extendedCatalogList(category, table, raw string) (GameResult, bool, error) {
	filter, page := parseCatalogFilterPage(raw)
	query := g.store.DB.Table(table)
	if filter != "" {
		like := "%" + filter + "%"
		query = query.Where("type = ? OR name LIKE ? OR description LIKE ?", filter, like, like)
	}
	var rows []model.GameplayConfigBase
	if err := query.Order("sort_order,level,id").Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	entries := make([]simpleCatalogEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, simpleCatalogEntry{row.Name, fmt.Sprintf("%s · 道阶%d · 状态%s", row.Type, row.Level, displayOr(row.Status, "开放")), fmt.Sprintf("效果：%s\n消耗：%s\n前置：%s\n%s", displayConfigText(row.EffectParams), displayConfigText(row.CostMaterials), displayConfigText(row.Prerequisite), row.Description)})
	}
	return buildSimpleCatalogResult(category, filter, page, 5, entries), true, nil
}

func (g *Game) catalogDetails(player *model.Player, category, raw string) (GameResult, bool, error) {
	name := strings.TrimSpace(raw)
	if category == "装备" {
		return g.equipmentDetails(player, name)
	}
	if name == "" || category == "" {
		return GameResult{Title: "🪪 图鉴详情", Content: "请输入：`图鉴详情 类别 完整名称`。", Actions: []string{"图鉴菜单"}}, true, nil
	}
	if category == "物品" || category == "材料" || category == "丹药" || category == "种子" || category == "礼包" {
		return g.itemDetails(name)
	}
	if category == "合成" {
		return g.synthesisRecipeDetails(player, name)
	}
	if category == "灵根" {
		return g.spiritualRootDetails(player, name)
	}
	if category == "灵脉" {
		return g.worldLeylineDetails(player, name)
	}
	if table := extendedCatalogTables[category]; table != "" {
		var row model.GameplayConfigBase
		if g.store.DB.Table(table).Where("name = ? OR code = ?", name, name).First(&row).Error != nil {
			return catalogNotFound(category, name), true, nil
		}
		content := fmt.Sprintf("名称：%s\n类型：%s · 道阶%d · 状态%s\n━━━━━━━━━━━\n效果：%s\n消耗：%s\n前置：%s\n━━━━━━━━━━━\n%s", row.Name, row.Type, row.Level, displayOr(row.Status, "开放"), displayConfigText(row.EffectParams), displayConfigText(row.CostMaterials), displayConfigText(row.Prerequisite), row.Description)
		return GameResult{Title: catalogIcon(category) + " " + category + "详情", Content: content, ImageURL: row.ImageURL, Actions: []string{"图鉴 " + category, queryCategoryMenu(strings.TrimSuffix(category, "道藏")), "查询 " + row.Name}}, true, nil
	}
	switch category {
	case "丹方":
		var row model.AlchemyRecipe
		if g.store.DB.Where("enabled = ? AND (name = ? OR output_name = ? OR code = ?)", true, name, name, name).First(&row).Error != nil {
			return catalogNotFound(category, name), true, nil
		}
		sources, actions := g.craftingMaterialGuide(row.MaterialsJSON)
		content := fmt.Sprintf("丹方：%s\n成丹：%s\n基础成功率：%.1f%%\n材料：%s\n实际药效：%s\n━━━━━━━━━━━\n%s\n━━━━━━━━━━━\n【材料来源】\n%s", row.Name, row.OutputName, row.SuccessRate*100, displayConfigText(row.MaterialsJSON), g.alchemyRecipeEffect(row), row.Description, sources)
		return GameResult{Title: "🧪 丹方详情", Content: content, Actions: append([]string{"炼药 " + row.Name, "物品 " + row.OutputName, "丹方图鉴"}, actions...)}, true, nil
	case "功法":
		var row model.Skill
		if g.store.DB.Where("name = ?", name).First(&row).Error != nil {
			return catalogNotFound(category, name), true, nil
		}
		if !g.skillVisibleToPlayer(player, row) {
			return catalogNotFound(category, name), true, nil
		}
		creatorText := ""
		if row.Rarity == "自创" {
			var publication model.SkillPublication
			if g.store.DB.Where("skill_id = ?", row.ID).First(&publication).Error == nil {
				creatorText = "\n创功者：" + publication.CreatorName
			}
		}
		return GameResult{Title: "📖 功法详情", Content: fmt.Sprintf("功法：%s\n流派：%s · 品阶：%s%s\n境界前置：%s\n━━━━━━━━━━━\n一级真实道效：%s\n修炼成长：%s\n━━━━━━━━━━━\n%s", row.Name, row.Type, row.Rarity, creatorText, row.RealmRequired, skillBonusText(decodeSkillStatBonus(row, 1)), displayConfigText(row.UpgradeJSON), row.Description), Actions: []string{"学功 " + row.Name, "功法图鉴", "功法分享", "功法"}}, true, nil
	case "境界":
		var row model.Realm
		if g.store.DB.Where("name = ?", name).First(&row).Error != nil {
			return catalogNotFound(category, name), true, nil
		}
		return GameResult{Title: "🧘 境界详情", Content: fmt.Sprintf("境界：%s · 第%d大境\n层次：一层至十层，十层圆满后方可进入下一境\n圆满基准修为：%d · 属性倍率×%.3f\n基础气血%d · 法力%d · 攻击%d · 防御%d · 身法%d\n寿元%d · 渡劫基准%.1f%%\n━━━━━━━━━━━\n%s", row.Name, row.Sequence, row.RequiredCultivation, row.AttributeMultiplier, row.BaseHealth, row.BaseMana, row.BaseAttack, row.BaseDefense, row.BaseSpeed, row.BaseLifespan, row.TribulationBaseRate*100, row.Description), Actions: []string{"图鉴 境界", "状态", "修炼", "突破", "备劫"}}, true, nil
	case "地图", "妖兽", "首领":
		var row model.WorldLocation
		query := g.store.DB.Where("enabled = ?", true)
		if category == "地图" {
			query = query.Where("name = ? OR code = ?", name, name)
		} else if category == "妖兽" {
			query = query.Where("monster_name = ?", name)
		} else {
			query = query.Where("boss_name = ?", name)
		}
		if query.First(&row).Error != nil {
			return catalogNotFound(category, name), true, nil
		}
		if category == "地图" {
			return GameResult{Title: "🗺️ 地图详情", Content: fmt.Sprintf("地点：%s · %s\n前置：%s · 移动体力%d\n━━━━━━━━━━━\n%s\nNPC：%s\n妖兽：%s（战力%d）\n首领：%s（战力%d）\n采集：%s×%d · 刷新%d分钟\n相邻路线：%s", row.Region, row.Name, g.locationRealmRequirement(row), row.StaminaCost, row.Description, strings.Join(decodeTextList(row.NPCJSON), "、"), displayOr(row.MonsterName, "无"), row.MonsterPower, displayOr(row.BossName, "无"), row.BossPower, displayOr(row.ResourceName, "无"), row.ResourceQuantity, row.ResourceCooldownMin, displayConfigText(row.NeighborsJSON)), ImageURL: row.ImageURL, Actions: []string{"前往 " + row.Name, "位置", "地图图鉴", "挑战 " + row.MonsterName, "首领"}}, true, nil
		}
		power, reward, action := row.MonsterPower, row.MonsterRewardJSON, "挑战 "+row.MonsterName
		cooldown := "遭遇率" + fmt.Sprintf("%.1f%%", row.MonsterEncounterRate*100)
		if category == "首领" {
			power, reward, action = row.BossPower, row.BossRewardJSON, "讨伐"
			cooldown = fmt.Sprintf("刷新%d分钟", row.BossCooldownMinutes)
		}
		return GameResult{Title: catalogIcon(category) + " " + category + "详情", Content: fmt.Sprintf("名称：%s\n栖息：%s·%s\n战力：%d · %s\n地图前置：%s\n奖励：%s\n━━━━━━━━━━━\n战斗采用逐回合斗法，玩家每回合选择普通攻击、功法、防御、服药或投降。", name, row.Region, row.Name, power, cooldown, g.locationRealmRequirement(row), displayConfigText(reward)), ImageURL: row.ImageURL, Actions: []string{"前往 " + row.Name, action, "图鉴 " + category, "物品"}}, true, nil
	case "NPC":
		var rows []model.WorldLocation
		if g.store.DB.Where("enabled = ? AND npc_json LIKE ?", true, "%"+name+"%").Order("minimum_realm_sequence,id").Find(&rows).Error != nil {
			return catalogNotFound(category, name), true, nil
		}
		for _, row := range rows {
			for _, npc := range decodeTextList(row.NPCJSON) {
				if npc == name {
					return GameResult{Title: "🧑 NPC详情", Content: fmt.Sprintf("人物：%s\n所在：%s·%s\n地图前置：%s\n━━━━━━━━━━━\n%s\n当地任务：%s", name, row.Region, row.Name, g.locationRealmRequirement(row), row.Description, strings.Join(decodeTextList(row.TasksJSON), "、")), Actions: []string{"前往 " + row.Name, "对话 " + name, "NPC图鉴", "任务图鉴"}}, true, nil
				}
			}
		}
		return catalogNotFound(category, name), true, nil
	case "灵兽":
		var row model.PetTemplate
		if g.store.DB.Where("enabled = ? AND (name = ? OR code = ?)", true, name, name).First(&row).Error != nil {
			return catalogNotFound(category, name), true, nil
		}
		return GameResult{Title: "🐉 灵兽详情", Content: fmt.Sprintf("灵兽：%s\n初始战力：%d · 每级成长：%d\n每日忠诚衰减：%d\n进化条件：%s\n进化目标：%s\n━━━━━━━━━━━\n长期饥饿、忠诚过低会触发拒战、离巢或叛变事件；喂养、出战与照料会影响成长。", row.Name, row.InitialPower, row.GrowthPerLevel, row.LoyaltyDecay, displayConfigText(row.EvolutionCondition), row.EvolutionTarget), Actions: []string{"捕获", "灵兽", "喂养 灵兽口粮", "灵兽图鉴"}}, true, nil
	case "副本":
		var row model.Dungeon
		if g.store.DB.Where("enabled = ? AND (name = ? OR code = ?)", true, name, name).First(&row).Error != nil {
			return catalogNotFound(category, name), true, nil
		}
		return GameResult{Title: "🏯 副本详情", Content: fmt.Sprintf("副本：%s【%s】\n推荐战力：%d · 体力%d · 每日%d次\n奖励：%s\n━━━━━━━━━━━\n进入后不会自动结算整场；每回合由玩家选择攻击、功法、防御、服药或退出。", row.Name, row.Difficulty, row.RecommendedPower, row.StaminaCost, row.DailyLimit, displayConfigText(row.RewardPoolJSON)), ImageURL: row.ImageURL, Actions: []string{"进入 " + row.Name, "副本图鉴", "体力", "装备系统"}}, true, nil
	case "任务":
		var row model.TaskTemplate
		if g.store.DB.Where("enabled = ? AND name = ?", true, name).First(&row).Error != nil {
			return catalogNotFound(category, name), true, nil
		}
		requirement, unmet, _ := g.prerequisiteStatus(player, row.PrerequisiteJSON)
		state := "可接取"
		if len(unmet) > 0 {
			state = "未满足：" + strings.Join(unmet, "；")
		}
		return GameResult{Title: "📜 任务详情", Content: fmt.Sprintf("任务：%s【%s】\n%s\n前置：%s\n当前判定：%s\n目标：%s\n奖励：%s\n━━━━━━━━━━━\n任务进度从接取后开始统计，不继承接取前的历史行为。", row.Name, row.Type, row.Description, requirement, state, taskObjectiveText(row.ObjectiveJSON), taskRewardText(row)), Actions: []string{"接任务 " + row.Name, "交任务 " + row.Name, "任务图鉴", "任务菜单", "银币来源"}}, true, nil
	case "称号":
		var row model.Title
		if g.store.DB.Where("enabled = ? AND (name = ? OR code = ?)", true, name, name).First(&row).Error != nil {
			return catalogNotFound(category, name), true, nil
		}
		grade := titleBroadcastGrade(row)
		return GameResult{Title: "🏅 称号详情", Content: fmt.Sprintf("称号：%s【%s】\n解锁：%s\n佩戴属性：%s\n通报级别：%s\n━━━━━━━━━━━\n称号是独立佩戴位；更换时会移除旧称号属性再应用新称号属性。普通称号不触发全区通报。", row.Name, row.Type, row.Condition, displayConfigText(row.AttributeBonus), grade), Actions: []string{"激活称号 " + row.Name, "佩戴称号 " + row.Name, "称号图鉴", "我的称号"}}, true, nil
	case "商城":
		var rows []model.ShopEntry
		if g.store.DB.Where("enabled = ? AND item_name = ?", true, name).Order("currency,price").Find(&rows).Error != nil || len(rows) == 0 {
			return catalogNotFound(category, name), true, nil
		}
		lines := []string{"商品：" + name, "━━━━━━━━━━━"}
		actions := []string{"物品 " + name, "商城图鉴"}
		for _, row := range rows {
			lines = append(lines, fmt.Sprintf("%d%s · 常设不限购 · %s", row.Price, row.Currency, displayOr(row.RefreshCycle, "永不下架")))
			command := "购入 " + name
			if row.Currency == "银币" || row.Currency == "仙金" {
				command = row.Currency + "购买 " + name
			}
			actions = append(actions, command)
		}
		return GameResult{Title: "🏪 商城详情", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
	case "活动":
		var row model.Activity
		if g.store.DB.Where("name = ? OR code = ?", name, name).First(&row).Error != nil {
			return catalogNotFound(category, name), true, nil
		}
		remaining := "已结束"
		if time.Now().Before(row.EndsAt) {
			remaining = row.EndsAt.Format("2006-01-02 15:04") + "结束"
		}
		return GameResult{Title: "🎯 活动详情", Content: fmt.Sprintf("活动：%s【%s · %s】\n时间：%s至%s\n当前：%s\n效果：%s\n参数：%s", row.Name, row.Type, row.Status, row.StartsAt.Format("2006-01-02 15:04"), row.EndsAt.Format("2006-01-02 15:04"), remaining, row.Effect, displayConfigText(row.EffectJSON)), Actions: []string{"活动总览", "活动图鉴", "活动菜单"}}, true, nil
	case "事件":
		var row model.Event
		if g.store.DB.Where("enabled = ? AND name = ?", true, name).First(&row).Error != nil {
			return catalogNotFound(category, name), true, nil
		}
		return GameResult{Title: "🌌 事件详情", Content: fmt.Sprintf("事件：%s【%s】\n基础触发率：%.2f%%\n条件：%s\n奖励与惩罚：%s\n━━━━━━━━━━━\n%s\n事件会结合当前地图、运气、境界与前置判定，不会把一千条事件同时塞进一次探索。", row.Name, row.Type, row.Probability*100, displayConfigText(row.ConditionJSON), displayConfigText(row.RewardJSON), row.Description), Actions: []string{"探索", "仙遇", "图鉴 事件", "运气"}}, true, nil
	case "掉落":
		var pool model.DropPool
		if g.store.DB.Where("enabled = ? AND name = ?", true, name).First(&pool).Error != nil {
			return catalogNotFound(category, name), true, nil
		}
		var entries []model.DropEntry
		_ = g.store.DB.Where("drop_pool_id = ?", pool.ID).Order("weight DESC,id").Find(&entries).Error
		lines := []string{fmt.Sprintf("掉落池：%s\n来源：%s·%s", pool.Name, pool.SourceType, pool.SourceName), "━━━━━━━━━━━"}
		actions := []string{"图鉴 掉落"}
		for _, row := range entries {
			lines = append(lines, fmt.Sprintf("%s · 权重%d · 数量%d至%d", row.ItemName, row.Weight, row.Minimum, row.Maximum))
			actions = append(actions, "物品 "+row.ItemName)
		}
		return GameResult{Title: "💠 掉落详情", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
	}
	return catalogNotFound(category, name), true, nil
}

func catalogNotFound(category, name string) GameResult {
	return GameResult{Title: "🪪 图鉴未收录", Content: category + "图鉴中没有找到“" + name + "”。请核对完整名称，或使用万象查询。", Actions: []string{"图鉴 " + category, "查询 " + name, "图鉴菜单"}}
}

func titleBroadcastGrade(row model.Title) string {
	if isHighTitle(row) {
		return "高阶称号：首次解锁或首次佩戴全区通报"
	}
	return "普通称号：不发送全区通报"
}

func (g *Game) alchemyRecipeEffect(row model.AlchemyRecipe) string {
	var item model.Item
	if g.store.DB.Where("id = ? OR name = ?", row.OutputItemID, row.OutputName).Order("id").First(&item).Error == nil {
		return itemEffectSummary(item, 1)
	}
	return "产物药效尚未关联物品图鉴"
}
