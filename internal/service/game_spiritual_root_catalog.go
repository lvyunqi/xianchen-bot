package service

import (
	"fmt"
	"strconv"
	"strings"

	"xianlv/internal/model"
)

func (g *Game) spiritualRootCatalog(player *model.Player, raw string) (GameResult, bool, error) {
	filter, page := parseCatalogFilterAndPage(raw)
	filter = normalizeElementFilter(filter)
	const pageSize = 6
	query := g.store.DB.Model(&model.SpiritualRootTemplate{}).Where("enabled = ?", true)
	if filter != "" {
		like := "%" + filter + "%"
		query = query.Where("element = ? OR element LIKE ? OR name LIKE ? OR grade LIKE ?", filter, like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt((int(total)+pageSize-1)/pageSize, 1)
	if page > pages {
		page = pages
	}
	var rows []model.SpiritualRootTemplate
	if err := query.Order("rarity_weight ASC, cultivation_bonus DESC, id ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	lines := []string{
		fmt.Sprintf("当前收录：%d种 · 第%d/%d页", total, page, pages),
		fmt.Sprintf("你的灵根：%s · 纯度%d", player.SpiritualRoot, player.RootQuality),
		"━━━━━━━",
	}
	if filter != "" {
		lines = append(lines[:2], append([]string{"筛选本源：" + filter, "━━━━━━━"}, lines[3:]...)...)
	}
	actions := []string{"灵根详情 " + player.SpiritualRoot, "灵检", "灵根合成", "灵根道种", "灵根进化菜单"}
	for _, row := range rows {
		mark := ""
		if row.Name == player.SpiritualRoot {
			mark = "【当前】"
		}
		lines = append(lines, fmt.Sprintf("- %s%s【%s · %s】\n  本源：%s · 修炼倍率×%.4f · 稀有权重%d\n  主加成：%s\n  副加成：%s", mark, row.Name, row.Grade, row.Element, row.Element, row.CultivationBonus, row.RarityWeight, row.PrimaryBonus, row.SecondaryBonus))
		actions = append(actions, "灵根详情 "+row.Name)
	}
	if page > 1 {
		actions = append(actions, catalogPageCommand("灵根图鉴", filter, page-1))
	}
	if page < pages {
		actions = append(actions, catalogPageCommand("灵根图鉴", filter, page+1))
	}
	lines = append(lines, "━━━━━━━", "灵根为入道时按稀有权重抽取；淬炼和觉醒提升纯度与阶段。两条不同图鉴道纹还可消耗灵根精粹进行随机合成。")
	title := "千种灵根图鉴"
	if filter != "" {
		title = filter + "本源灵根图鉴"
	}
	return GameResult{Title: title, Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) spiritualRootDetails(player *model.Player, raw string) (GameResult, bool, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return GameResult{Title: "灵根详情", Content: "请输入：`灵根详情 灵根名`，或从图鉴点击。", Actions: []string{"灵根图鉴"}}, true, nil
	}
	var row model.SpiritualRootTemplate
	if err := g.store.DB.Where("enabled = ? AND (name = ? OR code = ?)", true, name, name).First(&row).Error; err != nil {
		filter := normalizeElementFilter(name)
		var matches int64
		like := "%" + filter + "%"
		_ = g.store.DB.Model(&model.SpiritualRootTemplate{}).Where("enabled = ? AND (element = ? OR element LIKE ? OR name LIKE ?)", true, filter, like, like).Count(&matches).Error
		if filter != "" && matches > 0 {
			return g.spiritualRootCatalog(player, filter)
		}
		return GameResult{Title: "灵根未收录", Content: "图鉴中没有找到“" + name + "”。可输入完整灵根名，也可按“庚金本源”或“庚金”查询同源灵根。", Actions: []string{"灵根图鉴", "灵根图鉴 庚金", "灵脉地图"}}, true, nil
	}
	owned := "未持有"
	qualityLine := "抽取后会根据实际纯度折算加成"
	if player.SpiritualRoot == row.Name {
		owned = "当前灵根"
		qualityLine = fmt.Sprintf("当前纯度%d · 实际修炼加成%s", player.RootQuality, g.spiritualRootBonuses(row.Name, player.RootQuality).CultivationDisplay)
	}
	content := fmt.Sprintf("灵根：%s\n状态：%s\n品阶：%s · 本源：%s\n基础纯度：%d · 稀有权重：%d\n完整修炼倍率：×%.5f\n%s\n━━━━━━━\n主加成：%s\n副加成：%s\n五维与道力：%s\n战斗定位：%s\n━━━━━━━\n%s", row.Name, owned, row.Grade, row.Element, row.BaseQuality, row.RarityWeight, row.CultivationBonus, qualityLine, row.PrimaryBonus, row.SecondaryBonus, displayConfigText(row.AttributeJSON), row.CombatDescription, row.Description)
	actions := []string{"灵根图鉴", "灵根图鉴 " + row.Element, "灵脉地图 " + row.Element, "灵检", "灵淬", "灵进", "灵根合成", "灵根进化菜单"}
	if row.Name != player.SpiritualRoot {
		actions = append(actions, "灵根合成 "+player.SpiritualRoot+" "+row.Name)
	}
	return GameResult{Title: "灵根详情", Content: content, ImageURL: row.ImageURL, Actions: actions}, true, nil
}

func parseCatalogFilterAndPage(raw string) (string, int) {
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

func normalizeElementFilter(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, "灵根")
	value = strings.TrimSuffix(value, "本源")
	return strings.TrimSpace(value)
}

func catalogPageCommand(command, filter string, page int) string {
	if strings.TrimSpace(filter) == "" {
		return fmt.Sprintf("%s %d", command, page)
	}
	return fmt.Sprintf("%s %s %d", command, strings.TrimSpace(filter), page)
}
