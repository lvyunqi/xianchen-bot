package service

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"xianlv/internal/model"
	"xianlv/internal/storage"
)

const teleportActivationPrefix = "map.teleport."

var worldRegionOrder = []string{"东洲", "南疆", "西漠", "北原", "中天域", "沧海", "幽冥界", "九霄天", "太虚境", "星河界"}

func teleportActivationKey(locationID uint) string {
	return teleportActivationPrefix + strconv.FormatUint(uint64(locationID), 10)
}

func isTeleportLocation(location model.WorldLocation) bool {
	return location.TeleportEnabled || location.CrossRegionEnabled
}

func (g *Game) activateTeleportLocation(playerID uint, location model.WorldLocation) (bool, error) {
	if location.ID == 0 || !isTeleportLocation(location) {
		return false, nil
	}
	key := teleportActivationKey(location.ID)
	if _, err := g.playerValue(playerID, key); err == nil {
		return false, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	return true, g.setPlayerValue(playerID, key, location.Name, nil)
}

func (g *Game) activatedTeleportLocations(playerID uint) (map[uint]struct{}, error) {
	var values []model.PlayerValue
	if err := g.store.DB.Select("key").Where("player_id = ? AND key LIKE ?", playerID, teleportActivationPrefix+"%").Find(&values).Error; err != nil {
		return nil, err
	}
	result := make(map[uint]struct{}, len(values))
	for _, value := range values {
		raw := strings.TrimPrefix(value.Key, teleportActivationPrefix)
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err == nil && parsed > 0 {
			result[uint(parsed)] = struct{}{}
		}
	}
	return result, nil
}

func (g *Game) worldLocationAccessIssues(player *model.Player, location model.WorldLocation) ([]string, error) {
	sequence, err := g.playerRealmSequence(player)
	if err != nil {
		return nil, err
	}
	issues := []string{}
	if location.MinimumRealmSequence > sequence || location.MinimumRealmSequence == sequence && location.MinimumRealmLevel > player.RealmLevel {
		issues = append(issues, "境界未达："+g.locationRealmRequirement(location))
	}
	if location.MinimumLevel > player.Level {
		issues = append(issues, fmt.Sprintf("角色等级需%d，当前%d", location.MinimumLevel, player.Level))
	}
	return issues, nil
}

func normalizeWorldRegion(raw string) string {
	raw = strings.TrimSpace(raw)
	aliases := map[string]string{
		"中天": "中天域", "幽冥": "幽冥界", "九霄": "九霄天", "太虚": "太虚境", "星河": "星河界",
	}
	if canonical := aliases[raw]; canonical != "" {
		return canonical
	}
	for _, region := range worldRegionOrder {
		if raw == region {
			return region
		}
	}
	return ""
}

func (g *Game) worldEntryGate(region string) (model.WorldLocation, error) {
	var gate model.WorldLocation
	err := g.store.DB.Where("enabled = ? AND region = ? AND cross_region_enabled = ?", true, region, true).
		Order("minimum_realm_sequence,minimum_realm_level,sort_order,id").First(&gate).Error
	return gate, err
}

func (g *Game) teleportHub(player *model.Player) (GameResult, bool, error) {
	current, err := g.currentWorldLocation(player)
	if err != nil {
		return GameResult{}, true, err
	}
	activatedNow, err := g.activateTeleportLocation(player.ID, current)
	if err != nil {
		return GameResult{}, true, err
	}
	activated, err := g.activatedTeleportLocations(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	charm, charmErr := g.itemByName("传送符")
	charmCount := int64(0)
	if charmErr == nil {
		charmCount = g.itemQuantity(player.ID, charm.ID)
	}
	arrayState := "此地没有传送阵"
	if isTeleportLocation(current) {
		arrayState = "阵纹已激活，可进行界内传送"
		if current.CrossRegionEnabled {
			arrayState = "界门已激活，可进行界内与跨界传送"
		}
	}
	if activatedNow {
		arrayState += "（本次新刻录）"
	}
	content := fmt.Sprintf("当前位置：%s·%s\n阵势：%s\n已刻录阵点：%d处\n传送符：%d张\n━━━━━━━━━━━\n界内传送消耗传送符×1；跨界传送消耗传送符×3。步行抵达带阵地点或查看当地地图后会永久刻录阵纹。跨界只能从界门出发，目标世界达到开启境界后，其入口界门可直接接引。", displayOr(current.Region, "未知界域"), displayOr(current.Name, player.Location), arrayState, len(activated), charmCount)
	actions := []string{"传送列表", "诸界列表", "位置", "地图菜单", "物品 传送符", "配方 缩地传送符"}
	return GameResult{Title: "✨ 山河传送阵", Content: content, Actions: actions}, true, nil
}

func (g *Game) teleportList(player *model.Player, raw string) (GameResult, bool, error) {
	parts := strings.Fields(strings.TrimSpace(raw))
	page := 1
	if len(parts) > 0 {
		if parsed, err := strconv.Atoi(parts[len(parts)-1]); err == nil && parsed > 0 {
			page = parsed
			parts = parts[:len(parts)-1]
		}
	}
	region := normalizeWorldRegion(strings.Join(parts, ""))
	if region == "" {
		current, err := g.currentWorldLocation(player)
		if err != nil {
			return GameResult{}, true, err
		}
		region = current.Region
	}
	return g.teleportRegionList(player, region, page)
}

func (g *Game) worldRegionList(player *model.Player) (GameResult, bool, error) {
	var locations []model.WorldLocation
	if err := g.store.DB.Select("id,name,region,teleport_enabled,cross_region_enabled,minimum_realm_sequence,minimum_realm_level,minimum_level,sort_order").Where("enabled = ?", true).Find(&locations).Error; err != nil {
		return GameResult{}, true, err
	}
	activated, err := g.activatedTeleportLocations(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	byRegion := make(map[string][]model.WorldLocation)
	for _, location := range locations {
		byRegion[location.Region] = append(byRegion[location.Region], location)
	}
	regions := append([]string(nil), worldRegionOrder...)
	for region := range byRegion {
		if normalizeWorldRegion(region) == "" {
			regions = append(regions, region)
		}
	}
	if len(regions) > len(worldRegionOrder) {
		sort.Strings(regions[len(worldRegionOrder):])
	}
	playerRegion := currentRegion(player, locations)
	lines := []string{"天地已勘定十座正式界域；每座系统界域收录一千处地图。个人能否踏入，由入口界门的境界前置决定。", "━━━━━━━━━━━"}
	markdownLines := append([]string(nil), lines...)
	actions := []string{"传送阵", "位置", "地图菜单"}
	for _, region := range regions {
		rows := byRegion[region]
		if len(rows) == 0 {
			continue
		}
		var gate model.WorldLocation
		activatedCount := 0
		for _, row := range rows {
			if _, ok := activated[row.ID]; ok {
				activatedCount++
			}
			if row.CrossRegionEnabled && (gate.ID == 0 || row.MinimumRealmSequence < gate.MinimumRealmSequence || row.MinimumRealmSequence == gate.MinimumRealmSequence && row.MinimumRealmLevel < gate.MinimumRealmLevel) {
				gate = row
			}
		}
		status := "🔒 未解锁"
		detail := "入口界门尚未载入"
		open := false
		if gate.ID != 0 {
			issues, issueErr := g.worldLocationAccessIssues(player, gate)
			if issueErr != nil {
				return GameResult{}, true, issueErr
			}
			if len(issues) == 0 {
				open = true
				status = "🟢 已开放"
				detail = "入口：" + gate.Name
			} else {
				detail = strings.Join(issues, "；")
			}
		}
		if region == playerRegion {
			status += " · 当前所在"
		}
		plain := fmt.Sprintf("%s【%s】地图%d处 · 已刻录%d处\n  %s", status, region, len(rows), activatedCount, detail)
		lines = append(lines, plain)
		if open {
			markdownLines = append(markdownLines, fmt.Sprintf("%s【%s】地图%d处 · 已刻录%d处\n  %s", status, region, len(rows), activatedCount, markdownInlineCommand(detail, "跨界传送 "+region)))
			actions = append(actions, "跨界传送 "+region)
		} else {
			markdownLines = append(markdownLines, plain)
		}
		actions = append(actions, "传送列表 "+region)
		lines = append(lines, "━━━━━━━")
		markdownLines = append(markdownLines, "━━━━━━━")
	}
	return GameResult{Title: "🌌 诸界传送列表", Content: strings.Join(lines, "\n"), MarkdownContent: strings.Join(markdownLines, "\n"), Actions: actions}, true, nil
}

func currentRegion(player *model.Player, locations []model.WorldLocation) string {
	for _, location := range locations {
		if location.Name == player.Location {
			return location.Region
		}
	}
	return ""
}

func (g *Game) teleportRegionList(player *model.Player, region string, page int) (GameResult, bool, error) {
	var rows []model.WorldLocation
	if err := g.store.DB.Where("enabled = ? AND region = ? AND (teleport_enabled = ? OR cross_region_enabled = ?)", true, region, true, true).
		Order("minimum_realm_sequence,minimum_realm_level,sort_order,id").Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	if len(rows) == 0 {
		return GameResult{Title: "✨ 界域阵图未载入", Content: region + "尚未配置可用传送阵。", Actions: []string{"诸界列表", "地图菜单"}}, true, nil
	}
	activated, err := g.activatedTeleportLocations(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	const pageSize = 8
	pages := maxInt((len(rows)+pageSize-1)/pageSize, 1)
	page = minInt(maxInt(page, 1), pages)
	start := (page - 1) * pageSize
	end := minInt(start+pageSize, len(rows))
	lines := []string{fmt.Sprintf("第%d/%d页 · 共%d座阵点", page, pages, len(rows)), "已刻录阵点可直接传送；未刻录阵点需先沿地图路线亲自抵达。", "━━━━━━━━━━━"}
	markdownLines := append([]string(nil), lines...)
	actions := []string{"传送阵", "诸界列表", "位置"}
	for _, row := range rows[start:end] {
		issues, issueErr := g.worldLocationAccessIssues(player, row)
		if issueErr != nil {
			return GameResult{}, true, issueErr
		}
		_, recorded := activated[row.ID]
		state := "⚪ 未刻录"
		command := "图鉴详情 地图 " + row.Name
		if len(issues) > 0 {
			state = "🔒 " + strings.Join(issues, "；")
		} else if recorded {
			state = "🟢 已刻录"
			command = "传送 " + row.Name
		} else if row.CrossRegionEnabled {
			state = "🟡 入口界门可接引"
			command = "跨界传送 " + region
		}
		kind := "界内阵"
		if row.CrossRegionEnabled {
			kind = "跨界门"
		}
		plain := fmt.Sprintf("%s【%s】%s\n  %s · %s", mapStatusIcon(kind), row.Name, kind, state, g.locationRealmRequirement(row))
		lines = append(lines, plain, "━━━━━━━")
		markdownLines = append(markdownLines, fmt.Sprintf("%s【%s】%s\n  %s · %s", mapStatusIcon(kind), markdownInlineCommand(row.Name, command), kind, state, g.locationRealmRequirement(row)), "━━━━━━━")
		if len(issues) == 0 && (recorded || row.CrossRegionEnabled) {
			actions = append(actions, command)
		}
	}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("传送列表 %s %d", region, page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("传送列表 %s %d", region, page+1))
	}
	return GameResult{Title: "✨ " + region + "传送阵图", Content: strings.Join(lines, "\n"), MarkdownContent: strings.Join(markdownLines, "\n"), Actions: actions}, true, nil
}

func mapStatusIcon(kind string) string {
	if kind == "跨界门" {
		return "🌌 "
	}
	return "✨ "
}

func (g *Game) teleportTo(player *model.Player, raw string, crossOnly bool) (GameResult, bool, error) {
	targetText := strings.TrimSpace(raw)
	if targetText == "" {
		command := "传送 地点"
		if crossOnly {
			command = "跨界传送 界域"
		}
		return GameResult{Title: "✨ 传送目标缺失", Content: "请输入：`" + command + "`", Actions: []string{"传送列表", "诸界列表", "传送阵"}}, true, nil
	}
	if player.Health <= 0 {
		return GameResult{Title: "👻 元神离体，无法传送", Content: "肉身已经失去行动能力，阵法无法牵引散落元神。请先返回地脉复生阵。", Actions: []string{"回城复活", "状态"}}, true, nil
	}
	if player.State != "" && player.State != model.PlayerStateIdle {
		return GameResult{Title: "✨ 当前无法传送", Content: "当前状态：" + player.State + "。请先结束当前行动。", Actions: []string{"状态", "传送阵"}}, true, nil
	}
	current, err := g.currentWorldLocation(player)
	if err != nil {
		return GameResult{}, true, err
	}
	if !isTeleportLocation(current) {
		return GameResult{Title: "✨ 此地没有传送阵", Content: fmt.Sprintf("%s没有可用阵纹，需先沿相邻路线前往带阵地点。", current.Name), Actions: []string{"位置", "地图", "传送列表 " + current.Region}}, true, nil
	}
	if _, err := g.activateTeleportLocation(player.ID, current); err != nil {
		return GameResult{}, true, err
	}

	regionTarget := normalizeWorldRegion(targetText)
	var destination model.WorldLocation
	if regionTarget != "" {
		destination, err = g.worldEntryGate(regionTarget)
	} else {
		err = g.store.DB.Where("enabled = ? AND (name = ? OR code = ?)", true, targetText, targetText).First(&destination).Error
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return GameResult{Title: "✨ 传送目标不存在", Content: "没有找到界域或阵点：" + targetText + "。", Actions: []string{"传送列表", "诸界列表"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	if destination.Name == current.Name {
		return GameResult{Title: "✨ 已在此处", Content: "你当前就在" + destination.Name + "，无需消耗传送符。", Actions: []string{"位置", "传送列表"}}, true, nil
	}
	if !isTeleportLocation(destination) {
		return GameResult{Title: "✨ 目标没有阵纹", Content: destination.Name + "未设传送阵，只能沿地图路线步行抵达。", Actions: []string{"图鉴详情 地图 " + destination.Name, "位置", "传送列表 " + destination.Region}}, true, nil
	}
	issues, err := g.worldLocationAccessIssues(player, destination)
	if err != nil {
		return GameResult{}, true, err
	}
	if len(issues) > 0 {
		return GameResult{Title: "🔒 目标世界尚未解锁", Content: fmt.Sprintf("目标：%s·%s\n%s\n━━━━━━━━━━━\n前置未满时不会扣除传送符。", destination.Region, destination.Name, strings.Join(issues, "；")), Actions: []string{"诸界列表", "状态", "修炼"}}, true, nil
	}
	crossRegion := destination.Region != current.Region
	if crossOnly && !crossRegion {
		return GameResult{Title: "✨ 无需跨界", Content: destination.Name + "与当前位置同属" + current.Region + "，请使用界内传送。", Actions: []string{"传送 " + destination.Name, "传送列表 " + current.Region}}, true, nil
	}
	if crossRegion && (!current.CrossRegionEnabled || !destination.CrossRegionEnabled) {
		return GameResult{Title: "🌌 跨界门未连通", Content: fmt.Sprintf("跨界必须从两端界门通行。\n当前：%s（跨界%s）\n目标：%s（跨界%s）", current.Name, enabledText(current.CrossRegionEnabled), destination.Name, enabledText(destination.CrossRegionEnabled)), Actions: []string{"传送列表 " + current.Region, "诸界列表", "位置"}}, true, nil
	}
	activated, err := g.activatedTeleportLocations(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	_, destinationRecorded := activated[destination.ID]
	if !destinationRecorded && !(crossRegion && destination.CrossRegionEnabled) {
		return GameResult{Title: "✨ 阵点尚未刻录", Content: destination.Name + "尚未亲自到访，不能隔空传送。请先沿地图路线抵达并查看当地地图。", Actions: []string{"图鉴详情 地图 " + destination.Name, "传送列表 " + destination.Region, "位置"}}, true, nil
	}
	cost := int64(1)
	if crossRegion {
		cost = 3
	}
	charm, err := g.itemByName("传送符")
	if err != nil {
		return GameResult{Title: "✨ 传送道具未载入", Content: "传送符数据尚未载入，本次没有移动或扣除物品。", Actions: []string{"物品 传送符", "合成列表", "地图菜单"}}, true, nil
	}
	owned := g.itemQuantity(player.ID, charm.ID)
	if owned < cost {
		return GameResult{Title: "✨ 传送符不足", Content: fmt.Sprintf("本次需要传送符×%d，当前持有%d。\n可通过阵基石与星辰砂合成缩地传送符。", cost, owned), Actions: []string{"配方 缩地传送符", "合成 缩地传送符", "物品 传送符", "背包"}}, true, nil
	}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, charm.ID, -cost); err != nil {
			return err
		}
		if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Update("location", destination.Name).Error; err != nil {
			return err
		}
		return upsertPlayerValueTx(tx, player.ID, teleportActivationKey(destination.ID), destination.Name, nil)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	player.Location = destination.Name
	mapResult, _, err := g.worldMap(player)
	if err != nil {
		return GameResult{}, true, err
	}
	mode := "界内挪移"
	if crossRegion {
		mode = "跨界接引"
	}
	arrival := fmt.Sprintf("🌌 %s完成\n%s·%s → %s·%s\n传送符：-%d · 剩余%d\n目标阵纹：已刻录\n━━━━━━━━━━━\n", mode, current.Region, current.Name, destination.Region, destination.Name, cost, owned-cost)
	mapResult.Title = "🌌 " + mode + "成功"
	mapResult.Content = arrival + mapResult.Content
	mapResult.MarkdownContent = arrival + mapResult.MarkdownContent
	return mapResult, true, nil
}

func enabledText(enabled bool) string {
	if enabled {
		return "已开启"
	}
	return "未开启"
}
