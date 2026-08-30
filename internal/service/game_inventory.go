package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"xianlv/internal/handler"
	"xianlv/internal/model"
)

func (g *Game) executeInventoryCommand(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 248:
		return g.itemDetails(command.RawArguments)
	case 251:
		return g.queryEverything(player, command.RawArguments)
	case 249:
		return g.searchInventory(player, command.RawArguments)
	case 250:
		if strings.TrimSpace(command.RawArguments) == "" {
			return GameResult{Title: "使用物品", Content: "请输入：`使用 物品名` 或 `使用 物品名*数量`。\n批量数量不设游戏上限，但不能超过背包实际持有数量。丹药会直接生效，材料和残卷会提示对应用途。", Actions: []string{"背包", "物品"}}, true, nil
		}
		return g.consumeItem(player, command.RawArguments)
	default:
		return GameResult{}, false, nil
	}
}

func (g *Game) queryEverything(player *model.Player, argument string) (GameResult, bool, error) {
	argument = strings.TrimSpace(argument)
	if argument == "" {
		return GameResult{Title: "万象查询", Content: "可查询：物品、功法、境界、副本、地图地点、阵法、符箓、傀儡、灵根、仙药、法宝、天机、灵脉、任务和称号。\n示例：`查询 灵果`、`查询 青云山脚`、`查询 火云诀`。\n查询结果会给出属性、条件、消耗、来源和下一步指令。", Actions: []string{"物品", "功法", "地图", "副本", "菜单"}}, true, nil
	}
	if id, err := strconv.ParseUint(argument, 10, 64); err == nil && id > 0 {
		return g.itemDetails(argument)
	}
	type hit struct {
		Kind string
		Name string
		Text string
		Do   string
	}
	hits := make([]hit, 0, 12)
	var item model.Item
	if g.store.DB.Where("name = ? OR code = ?", argument, argument).First(&item).Error == nil {
		hits = append(hits, hit{"物品", item.Name, fmt.Sprintf("分类：%s · 品级：%s\n实际效果：%s\n用途：%s\n价值：%d", item.CategoryName, item.RarityName, itemEffectSummary(item, 1), displayOr(item.Description, "暂无说明"), item.BaseValue), "物品 " + item.Name})
	}
	var skill model.Skill
	if g.store.DB.Where("name = ?", argument).First(&skill).Error == nil && g.skillVisibleToPlayer(player, skill) {
		hits = append(hits, hit{"功法", skill.Name, fmt.Sprintf("流派：%s · 稀有度：%s\n境界条件：%s\n一级真实道效：%s\n说明：%s", skill.Type, skill.Rarity, skill.RealmRequired, skillBonusText(decodeSkillStatBonus(skill, 1)), skill.Description), "学功 " + skill.Name})
	}
	var realm model.Realm
	if g.store.DB.Where("name = ?", argument).First(&realm).Error == nil {
		hits = append(hits, hit{"境界", realm.Name, fmt.Sprintf("第%d境\n需求修为：%d\n属性倍率：%.2f\n寿元：%d\n渡劫基数：%.0f%%\n%s", realm.Sequence, realm.RequiredCultivation, realm.AttributeMultiplier, realm.BaseLifespan, realm.TribulationBaseRate*100, realm.Description), "突破"})
	}
	var dungeon model.Dungeon
	if g.store.DB.Where("name = ?", argument).First(&dungeon).Error == nil {
		hits = append(hits, hit{"副本", dungeon.Name, fmt.Sprintf("难度：%s · 推荐战力：%d\n体力：%d · 每日次数：%d\n奖励配置：%s", dungeon.Difficulty, dungeon.RecommendedPower, dungeon.StaminaCost, dungeon.DailyLimit, displayConfigText(dungeon.RewardPoolJSON)), "进入 " + dungeon.Name})
	}
	var artifact model.ArtifactTemplate
	if g.store.DB.Where("enabled = ? AND (name = ? OR code = ?)", true, argument, argument).First(&artifact).Error == nil {
		hits = append(hits, hit{"装备", artifact.Name, fmt.Sprintf("槽位：%s · 器型：%s\n定位：%s · 套装：%s\n基础属性：%s\n炼制材料：%s\n穿戴前置：%s", artifactTemplateSlot(artifact), artifactTemplateArchetype(artifact), displayOr(artifact.Positioning, "均衡养器"), displayOr(artifact.SetName, "散修器物"), artifactTemplateStatsText(artifact.AttributeJSON), displayConfigText(artifact.MaterialsJSON), g.artifactRequirementText(artifact)), "装备详情 " + artifact.Name})
	}
	var location model.WorldLocation
	if g.store.DB.Where("name = ? OR code = ?", argument, argument).First(&location).Error == nil {
		hits = append(hits, hit{"地图", location.Name, fmt.Sprintf("区域：%s\n%s\n普通妖灵：%s（战力%d）\n区域Boss：%s（战力%d）\n体力：%d\n相邻路线：%s", location.Region, location.Description, location.MonsterName, location.MonsterPower, location.BossName, location.BossPower, location.StaminaCost, location.NeighborsJSON), "前往 " + location.Name})
	}
	for _, spec := range []struct{ kind, table string }{
		{"阵法", "formation_configs"}, {"符箓", "talisman_configs"}, {"傀儡", "puppet_configs"}, {"秘境", "secret_realm_conflict_configs"}, {"传承", "inheritance_configs"}, {"悟道", "dao_insight_configs"}, {"仙魔", "immortal_demon_battlefield_configs"}, {"灵根", "spiritual_root_evolution_configs"}, {"心魔", "inner_demon_configs"}, {"合体技", "couple_combination_skill_configs"}, {"仙药", "immortal_herb_configs"}, {"法宝", "artifact_refinement_configs"}, {"天机", "destiny_deduction_configs"}, {"灵脉", "leyline_configs"}, {"宗战", "sect_war_configs"}, {"奇遇", "immortal_encounter_configs"}, {"星河", "star_realm_configs"},
	} {
		var config model.GameplayConfigBase
		if g.store.DB.Table(spec.table).Where("name = ? OR code = ?", argument, argument).First(&config).Error == nil {
			hits = append(hits, hit{spec.kind, config.Name, fmt.Sprintf("类型：%s · 等级：%d\n效果：%s\n消耗：%s\n条件：%s\n%s", config.Type, config.Level, displayConfigText(config.EffectParams), displayConfigText(config.CostMaterials), displayConfigText(config.Prerequisite), config.Description), queryCategoryMenu(spec.kind)})
		}
	}
	if len(hits) == 0 {
		return GameResult{Title: "万象查询", Content: "天机阁没有找到“" + argument + "”。可先发送 `菜单` 查看系统，并核对完整名称。", Actions: []string{"物品", "地图", "菜单"}}, true, nil
	}
	lines := []string{"查询关键词：" + argument, "━━━━━━━━━━━"}
	actions := []string{"系统", "菜单"}
	for _, row := range hits {
		lines = append(lines, "【"+row.Kind+"】"+row.Name, row.Text, "下一步："+row.Do, "━━━━━━━━━━━")
		actions = append(actions, row.Do)
	}
	return GameResult{Title: "万象查询", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func queryCategoryMenu(kind string) string {
	return map[string]string{"阵法": "阵法菜单", "符箓": "符箓菜单", "傀儡": "傀儡菜单", "秘境": "秘境争夺菜单", "传承": "传承菜单", "悟道": "悟道菜单", "仙魔": "仙魔战场菜单", "灵根": "灵根进化菜单", "心魔": "渡劫心魔菜单", "合体技": "合体技菜单", "仙药": "仙药培育菜单", "法宝": "法宝炼化菜单", "天机": "天机推演菜单", "灵脉": "天地灵脉菜单", "宗战": "宗门战争菜单", "奇遇": "仙缘奇遇菜单", "星河": "宇宙星河菜单"}[kind]
}

func (g *Game) itemDetails(argument string) (GameResult, bool, error) {
	argument = strings.TrimSpace(argument)
	if argument == "" {
		return GameResult{Title: "物品图鉴", Content: "发送 `物品 物品名` 查看详情，或发送 `查询 物品ID`。\n发送 `背包搜索 关键词` 筛选自己的物品。", Actions: []string{"背包", "背包搜索 功法", "查询 1"}}, true, nil
	}
	var item model.Item
	if id, err := strconv.ParseUint(argument, 10, 64); err == nil && id > 0 {
		if err := g.store.DB.First(&item, id).Error; err != nil {
			return GameResult{Title: "物品不存在", Content: "没有找到物品ID：" + argument, Actions: []string{"背包", "物品"}}, true, nil
		}
	} else if err := g.store.DB.Where("name = ? OR code = ?", argument, argument).First(&item).Error; err != nil {
		var artifact model.ArtifactTemplate
		if artifactErr := g.store.DB.Where("enabled = ? AND (name = ? OR code = ?)", true, argument, argument).First(&artifact).Error; artifactErr == nil {
			return GameResult{Title: "器物图鉴", Content: fmt.Sprintf("装备：%s\n槽位：%s · 器型：%s\n定位：%s · 套装：%s\n基础属性：%s\n━━━━━━━━━━━\n此物属于独立装备背包，不占用普通乾坤袋格数；强化、锻造、星阶、灵纹和嵌灵均保存在装备实例中。", artifact.Name, artifactTemplateSlot(artifact), artifactTemplateArchetype(artifact), displayOr(artifact.Positioning, "均衡养器"), displayOr(artifact.SetName, "散修器物"), artifactTemplateStatsText(artifact.AttributeJSON)), Actions: []string{"装备详情 " + artifact.Name, "装备背包", "装备图鉴 " + artifactTemplateSlot(artifact), "炼器 " + artifact.Name}}, true, nil
		}
		return GameResult{Title: "物品不存在", Content: "没有找到物品：" + argument, Actions: []string{"背包", "物品"}}, true, nil
	}
	sources, uses := g.itemCatalogueRelations(item)
	sourceText := "暂无配置掉落来源"
	if len(sources) > 0 {
		sourceText = strings.Join(sources, "、")
	}
	useText := "当前没有配置为其他配方材料"
	if len(uses) > 0 {
		useText = strings.Join(uses, "、")
	}
	return GameResult{Title: "物品图鉴", Content: fmt.Sprintf("物品：%s\n编号：%d · 编码：%s\n分类：%s · 品级：%s\n基础价值：%d\n━━━━━━━━━━━\n实际效果：%s\n说明：%s\n━━━━━━━━━━━\n获取来源：%s\n具体用途：%s\n可交易：%s · 可堆叠：%s", item.Name, item.ID, item.Code, displayOr(item.CategoryName, "未分类"), displayOr(item.RarityName, "凡品"), item.BaseValue, itemEffectSummary(item, 1), displayOr(item.Description, "暂无说明"), sourceText, useText, yesNo(item.Tradable), yesNo(item.Stackable)), ImageURL: item.ImageURL, Actions: []string{"使用 " + item.Name, "药效 " + item.Name, "背包", "丹方", "合成列表", "地图"}}, true, nil
}

func (g *Game) itemCatalogueRelations(item model.Item) ([]string, []string) {
	sources := make([]string, 0, 8)
	uses := make([]string, 0, 12)
	seenSource := map[string]bool{}
	seenUse := map[string]bool{}
	appendUnique := func(target *[]string, seen map[string]bool, value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		*target = append(*target, value)
	}
	type dropSource struct {
		SourceType string
		SourceName string
	}
	var drops []dropSource
	_ = g.store.DB.Table("drop_entries").Select("drop_pools.source_type, drop_pools.source_name").Joins("JOIN drop_pools ON drop_pools.id = drop_entries.drop_pool_id").Where("drop_entries.item_id = ? AND drop_pools.enabled = ?", item.ID, true).Scan(&drops).Error
	for _, row := range drops {
		appendUnique(&sources, seenSource, displayOr(row.SourceType, "掉落")+"："+displayOr(row.SourceName, "未知地点"))
	}
	var seeds []model.Item
	_ = g.store.DB.Where("effect_func = ?", "plant_seed").Find(&seeds).Error
	for _, seed := range seeds {
		var parameters struct {
			Crop string `json:"crop"`
		}
		if json.Unmarshal([]byte(seed.EffectParams), &parameters) == nil && parameters.Crop == item.Name {
			appendUnique(&sources, seenSource, "灵田种植："+seed.Name)
		}
	}
	var synthesisSources []model.SynthesisRecipe
	_ = g.store.DB.Where("output_name = ? AND enabled = ?", item.Name, true).Order("sort_order,id").Find(&synthesisSources).Error
	for _, recipe := range synthesisSources {
		appendUnique(&sources, seenSource, "合成："+recipe.Name)
	}
	var alchemySources []model.AlchemyRecipe
	_ = g.store.DB.Where("output_name = ? AND enabled = ?", item.Name, true).Order("id").Find(&alchemySources).Error
	for _, recipe := range alchemySources {
		appendUnique(&sources, seenSource, "炼丹："+recipe.Name)
	}
	var shops []model.ShopEntry
	_ = g.store.DB.Where("item_id = ? AND enabled = ?", item.ID, true).Order("sort,id").Find(&shops).Error
	for _, shop := range shops {
		appendUnique(&sources, seenSource, fmt.Sprintf("商城：%d%s", shop.Price, shop.Currency))
	}
	var alchemyUses []model.AlchemyRecipe
	_ = g.store.DB.Where("enabled = ?", true).Find(&alchemyUses).Error
	for _, recipe := range alchemyUses {
		var materials map[string]int64
		if json.Unmarshal([]byte(recipe.MaterialsJSON), &materials) == nil && materials[item.Name] > 0 {
			appendUnique(&uses, seenUse, fmt.Sprintf("炼丹%s×%d", recipe.Name, materials[item.Name]))
		}
	}
	var synthesisUses []model.SynthesisRecipe
	_ = g.store.DB.Where("enabled = ?", true).Order("sort_order,id").Find(&synthesisUses).Error
	for _, recipe := range synthesisUses {
		var materials map[string]int64
		if json.Unmarshal([]byte(recipe.MaterialsJSON), &materials) == nil && materials[item.Name] > 0 {
			appendUnique(&uses, seenUse, fmt.Sprintf("合成%s×%d", recipe.Name, materials[item.Name]))
		}
	}
	var artifacts []model.ArtifactTemplate
	_ = g.store.DB.Where("enabled = ?", true).Find(&artifacts).Error
	for _, recipe := range artifacts {
		var materials map[string]int64
		if json.Unmarshal([]byte(recipe.MaterialsJSON), &materials) == nil && materials[item.Name] > 0 {
			appendUnique(&uses, seenUse, fmt.Sprintf("炼器%s×%d", recipe.Name, materials[item.Name]))
		}
	}
	if item.Name == "阵基石" {
		appendUnique(&uses, seenUse, "装备篆刻×1")
	}
	if len(sources) > 10 {
		sources = sources[:10]
	}
	if len(uses) > 12 {
		uses = uses[:12]
	}
	return sources, uses
}

func (g *Game) searchInventory(player *model.Player, keyword string) (GameResult, bool, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return GameResult{Title: "背包搜索", Content: "请输入关键词，例如：`背包搜索 功法`。", Actions: []string{"背包", "物品"}}, true, nil
	}
	type row struct {
		ID       uint
		Name     string
		Category string
		Rarity   string
		Quantity int64
	}
	var rows []row
	like := "%" + keyword + "%"
	err := g.store.DB.Table("player_items").Select("items.id, items.name, items.category_name AS category, items.rarity_name AS rarity, player_items.quantity").Joins("JOIN items ON items.id = player_items.item_id").Where("player_items.player_id = ? AND player_items.quantity > 0 AND (items.name LIKE ? OR items.category_name LIKE ? OR items.description LIKE ?)", player.ID, like, like, like).Order("items.category_name, items.name").Limit(30).Scan(&rows).Error
	if err != nil {
		return GameResult{}, true, err
	}
	if len(rows) == 0 {
		return GameResult{Title: "背包搜索", Content: "没有找到包含“" + keyword + "”的物品。", Actions: []string{"背包", "物品"}}, true, nil
	}
	lines := []string{fmt.Sprintf("关键词：%s · 找到%d种物品", keyword, len(rows))}
	markdownLines := append([]string(nil), lines...)
	actions := []string{"背包", "物品"}
	for _, row := range rows {
		line := fmt.Sprintf("%s × %d · %s · %s", row.Name, row.Quantity, displayOr(row.Category, "未分类"), displayOr(row.Rarity, "凡品"))
		lines = append(lines, line)
		markdownLines = append(markdownLines, fmt.Sprintf("%s × %d · %s · %s", markdownInlineCommand(row.Name, "物品 "+row.Name), row.Quantity, displayOr(row.Category, "未分类"), displayOr(row.Rarity, "凡品")))
	}
	return GameResult{Title: "背包搜索", Content: strings.Join(lines, "\n"), MarkdownContent: strings.Join(markdownLines, "\n"), Actions: actions}, true, nil
}

func yesNo(value bool) string {
	if value {
		return "是"
	}
	return "否"
}
