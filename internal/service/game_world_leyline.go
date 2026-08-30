package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/model"
	"xianlv/internal/storage"
)

const leylineMeditationState = "灵脉打坐"

type leylineMeditationJob struct {
	LeylineID uint      `json:"leyline_id"`
	StartedAt time.Time `json:"started_at"`
}

func (g *Game) worldLeylineMap(player *model.Player, raw string) (GameResult, bool, error) {
	filter, page := parseCatalogFilterAndPage(raw)
	filter = normalizeElementFilter(filter)
	const pageSize = 6
	query := g.store.DB.Model(&model.WorldLeyline{}).Where("enabled = ?", true)
	if filter != "" {
		like := "%" + filter + "%"
		query = query.Where("element = ? OR required_root_element = ? OR element LIKE ? OR name LIKE ?", filter, filter, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt((int(total)+pageSize-1)/pageSize, 1)
	if page > pages {
		page = pages
	}
	var rows []model.WorldLeyline
	if err := query.Order("minimum_realm_sequence, sort_order").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	currentElement := g.playerSpiritualRootElement(player)
	lines := []string{fmt.Sprintf("当前位置：%s · 第%d/%d页 · 共%d条灵脉", player.Location, page, pages, total), fmt.Sprintf("你的灵根本源：%s · 全界索引按最低境界排列", displayOr(currentElement, "尚未辨明")), "━━━━━━━━━━━"}
	if filter != "" {
		lines = append(lines[:2], append([]string{"本源筛选：" + filter + " · 以下为全界结果，不限当前位置", "━━━━━━━━━━━"}, lines[3:]...)...)
	}
	actions := []string{"寻脉", "灵脉出定", "灵脉修行榜"}
	if currentElement != "" && currentElement != filter {
		actions = append(actions, "灵脉地图 "+currentElement)
	}
	planner, routeErr := g.loadWorldRoutePlanner()
	if routeErr != nil {
		return GameResult{}, true, routeErr
	}
	for _, row := range rows {
		route := planner.shortest(player.Location, row.LocationName)
		locationMark := worldRouteLocationMark(route)
		unmet, _ := g.worldLeylineUnmet(player, row)
		state := "可尝试入脉"
		if len(unmet) > 0 {
			state = "前置未满"
		}
		lines = append(lines, fmt.Sprintf("- %s【%s】\n  %s · %s本源 · %.3f倍修炼\n  最低第%d境·%d层 · 战力%d · %s/%s", row.Name, row.Grade, row.Region+"·"+row.LocationName, row.Element, row.CultivationMultiplier, row.MinimumRealmSequence, row.MinimumRealmLevel, row.MinimumCombatPower, locationMark, state))
		actions = append(actions, "灵脉详情 "+row.Name)
		actions = appendWorldRouteAction(actions, route)
	}
	if len(rows) == 0 {
		lines = append(lines, "天机阁尚未载入契合“"+displayOr(filter, "该条件")+"”的启用灵脉。", "该结果不会扣除法力；可查看同源灵根或提交BUG，由仙盟补齐世界数据。")
		actions = append(actions, "灵根图鉴 "+filter, "灵根合成", "提交BUG 灵脉地图 "+filter+"；现象：全界没有契合灵脉；期望：显示可抵达的同源灵脉")
	}
	if page > 1 {
		actions = append(actions, catalogPageCommand("灵脉地图", filter, page-1))
	}
	if page < pages {
		actions = append(actions, catalogPageCommand("灵脉地图", filter, page+1))
	}
	return GameResult{Title: "修仙界灵脉图", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) discoverLocalLeylines(player *model.Player) (GameResult, bool, error) {
	var rows []model.WorldLeyline
	if err := g.store.DB.Where("enabled = ? AND location_name = ?", true, player.Location).Order("minimum_realm_sequence,sort_order").Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	if len(rows) == 0 {
		return g.guideToCompatibleLeylines(player, "此地地脉沉寂，没有可探明的灵脉；本次没有扣除法力。")
	}
	minimumMana := rows[0].DiscoveryManaCost
	for _, row := range rows[1:] {
		if row.DiscoveryManaCost < minimumMana {
			minimumMana = row.DiscoveryManaCost
		}
	}
	if player.Mana < minimumMana {
		return GameResult{Title: "神识不足", Content: fmt.Sprintf("探查%s的地脉至少需要法力%d，当前法力%d。", player.Location, minimumMana, player.Mana), Actions: []string{"状态", "灵脉地图"}}, true, nil
	}
	if err := g.store.DB.Model(&model.Player{}).Where("id = ? AND mana >= ?", player.ID, minimumMana).Update("mana", gorm.Expr("mana - ?", minimumMana)).Error; err != nil {
		return GameResult{}, true, err
	}
	lines := []string{fmt.Sprintf("神识沉入%s地底，消耗法力%d，共发现%d条灵脉。", player.Location, minimumMana, len(rows)), "━━━━━━━━━━━"}
	currentElement := g.playerSpiritualRootElement(player)
	compatibleLocal := 0
	actions := []string{"位置", "灵脉地图"}
	for _, row := range rows {
		_ = g.setPlayerValue(player.ID, fmt.Sprintf("leyline.discovered.%d", row.ID), "true", nil)
		lines = append(lines, fmt.Sprintf("- %s · %s · %s · %.3f倍", row.Name, row.Grade, row.Element, row.CultivationMultiplier))
		actions = append(actions, "灵脉详情 "+row.Name)
		if rootElementMatches(row.RequiredRootElement, currentElement) || rootElementMatches(currentElement, row.RequiredRootElement) {
			compatibleLocal++
		}
	}
	if currentElement != "" && compatibleLocal == 0 {
		guidance, guidanceActions, guidanceErr := g.compatibleLeylineGuidance(player, currentElement, 3)
		if guidanceErr != nil {
			return GameResult{}, true, guidanceErr
		}
		lines = append(lines, "━━━━━━━━━━━", "本地没有"+currentElement+"本源灵脉。")
		lines = append(lines, guidance...)
		actions = append(actions, guidanceActions...)
	}
	return GameResult{Title: "寻脉有得", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) guideToCompatibleLeylines(player *model.Player, reason string) (GameResult, bool, error) {
	element := g.playerSpiritualRootElement(player)
	if element == "" {
		return GameResult{Title: "地脉无声", Content: reason + "\n你的灵根本源尚未辨明，请先检测灵根。", Actions: []string{"灵检", "灵根图鉴", "地图"}}, true, nil
	}
	guidance, actions, err := g.compatibleLeylineGuidance(player, element, 3)
	if err != nil {
		return GameResult{}, true, err
	}
	lines := []string{reason, "━━━━━━━━━━━", fmt.Sprintf("你的本源：%s · 最近契合灵脉", element)}
	lines = append(lines, guidance...)
	actions = append([]string{"灵脉地图 " + element, "地图"}, actions...)
	return GameResult{Title: "地脉指路", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) compatibleLeylineGuidance(player *model.Player, element string, limit int) ([]string, []string, error) {
	like := "%" + strings.TrimSpace(element) + "%"
	var rows []model.WorldLeyline
	if err := g.store.DB.Where("enabled = ? AND (element = ? OR required_root_element = ? OR element LIKE ?)", true, element, element, like).Order("minimum_realm_sequence,minimum_realm_level,sort_order").Limit(40).Find(&rows).Error; err != nil {
		return nil, nil, err
	}
	if len(rows) == 0 {
		return []string{"全界尚无" + element + "本源灵脉，请通过反馈菜单提交缺失数据。"}, []string{"灵根图鉴 " + element, "灵根合成", "反馈菜单"}, nil
	}
	planner, err := g.loadWorldRoutePlanner()
	if err != nil {
		return nil, nil, err
	}
	type candidate struct {
		row   model.WorldLeyline
		route []string
	}
	candidates := make([]candidate, 0, len(rows))
	for _, row := range rows {
		candidates = append(candidates, candidate{row: row, route: planner.shortest(player.Location, row.LocationName)})
	}
	sort.SliceStable(candidates, func(left, right int) bool {
		leftDistance, rightDistance := len(candidates[left].route), len(candidates[right].route)
		if leftDistance == 0 {
			leftDistance = 1 << 30
		}
		if rightDistance == 0 {
			rightDistance = 1 << 30
		}
		if leftDistance != rightDistance {
			return leftDistance < rightDistance
		}
		if candidates[left].row.MinimumRealmSequence != candidates[right].row.MinimumRealmSequence {
			return candidates[left].row.MinimumRealmSequence < candidates[right].row.MinimumRealmSequence
		}
		return candidates[left].row.SortOrder < candidates[right].row.SortOrder
	})
	limit = minInt(maxInt(limit, 1), len(candidates))
	lines := make([]string, 0, limit)
	actions := make([]string, 0, limit*2)
	for _, candidate := range candidates[:limit] {
		lines = append(lines, fmt.Sprintf("- %s【第%d境·%d层】\n  所在：%s·%s\n  路线：%s", candidate.row.Name, candidate.row.MinimumRealmSequence, candidate.row.MinimumRealmLevel, candidate.row.Region, candidate.row.LocationName, worldRouteSummary(candidate.route)))
		actions = append(actions, "灵脉详情 "+candidate.row.Name)
		actions = appendWorldRouteAction(actions, candidate.route)
	}
	return lines, actions, nil
}

func (g *Game) worldLeylineDetails(player *model.Player, raw string) (GameResult, bool, error) {
	row, err := g.worldLeylineByName(raw)
	if err != nil {
		return GameResult{Title: "灵脉不存在", Content: "没有找到已启用的灵脉“" + strings.TrimSpace(raw) + "”。", Actions: []string{"灵脉地图"}}, true, nil
	}
	unmet, err := g.worldLeylineUnmet(player, row)
	if err != nil {
		return GameResult{}, true, err
	}
	realmName := fmt.Sprintf("第%d境", row.MinimumRealmSequence)
	var realm model.Realm
	if g.store.DB.Where("sequence = ?", row.MinimumRealmSequence).First(&realm).Error == nil {
		realmName = realm.Name
	}
	state := "全部满足"
	if len(unmet) > 0 {
		state = strings.Join(unmet, "；")
	}
	currentSequence, _ := g.playerRealmSequence(player)
	planner, routeErr := g.loadWorldRoutePlanner()
	if routeErr != nil {
		return GameResult{}, true, routeErr
	}
	route := planner.shortest(player.Location, row.LocationName)
	content := fmt.Sprintf("灵脉：%s\n阶级：%s · 本源：%s\n位置：%s · %s\n抵达路线：%s\n━━━━━━━━━━━\n修炼倍率：%.3f倍\n每分钟灵气：%d\n打坐位：%d\n独立加成：%s\n━━━━━━━━━━━\n打坐境界限制：最低%s · %d层\n当前境界：第%d境 · %s%d层\n最低战力：%d · 最低神识：%d\n契合灵根：%s本源\n入脉消耗：法力%d · %s×%d\n当前判定：%s\n━━━━━━━━━━━\n%s", row.Name, row.Grade, row.Element, row.Region, row.LocationName, worldRouteSummary(route), row.CultivationMultiplier, row.AuraPerMinute, row.MeditationSlots, displayConfigText(row.BonusJSON), realmName, row.MinimumRealmLevel, currentSequence, player.RealmName, player.RealmLevel, row.MinimumCombatPower, row.MinimumSpirit, row.RequiredRootElement, row.DiscoveryManaCost, row.RequiredItem, row.RequiredItemCount, state, row.Description)
	return GameResult{Title: "灵脉详情", Content: content, ImageURL: row.ImageURL, Actions: g.leylineGuidanceActions(player, row, true)}, true, nil
}

func (g *Game) startLeylineMeditation(player *model.Player, raw string) (GameResult, bool, error) {
	row, err := g.worldLeylineByName(raw)
	if err != nil {
		return GameResult{Title: "入脉失败", Content: "灵脉不存在，发送 `灵脉地图` 查看。", Actions: []string{"灵脉地图"}}, true, nil
	}
	if player.Location != row.LocationName {
		planner, routeErr := g.loadWorldRoutePlanner()
		if routeErr != nil {
			return GameResult{}, true, routeErr
		}
		route := planner.shortest(player.Location, row.LocationName)
		actions := []string{"灵脉详情 " + row.Name, "地图"}
		actions = appendWorldRouteAction(actions, route)
		return GameResult{Title: "尚未抵达", Content: fmt.Sprintf("%s位于%s，当前在%s。\n抵达路线：%s\n请逐站前往；系统不会用一个无效指令跨过未解锁地图。", row.Name, row.LocationName, player.Location, worldRouteSummary(route)), Actions: actions}, true, nil
	}
	if _, err := g.playerValue(player.ID, fmt.Sprintf("leyline.discovered.%d", row.ID)); err != nil {
		return GameResult{Title: "灵脉尚未探明", Content: "先发送 `寻脉` 以神识确定入口与灵气潮汐。", Actions: []string{"寻脉"}}, true, nil
	}
	if player.State != "" && player.State != model.PlayerStateIdle {
		return GameResult{Title: "当前无法入脉", Content: "当前状态：" + player.State + "。请先结束当前行动。", Actions: []string{"状态"}}, true, nil
	}
	unmet, err := g.worldLeylineUnmet(player, row)
	if err != nil {
		return GameResult{}, true, err
	}
	if len(unmet) > 0 {
		return GameResult{Title: "灵脉前置未满", Content: strings.Join(unmet, "\n") + "\n━━━━━━━━━━━\n境界、层数、战力、神识、本源和护脉材料均为真实入脉条件；未满足时不会扣除法力或物品。", Actions: g.leylineGuidanceActions(player, row, false)}, true, nil
	}
	occupied, err := g.occupiedLeylineSlots(row.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	if occupied >= row.MeditationSlots {
		return GameResult{Title: "灵脉蒲团已满", Content: fmt.Sprintf("%s共有%d个打坐位，目前均被占用。稍后再来或寻找其他灵脉。", row.Name, row.MeditationSlots), Actions: []string{"灵脉地图", "寻脉"}}, true, nil
	}
	job := leylineMeditationJob{LeylineID: row.ID, StartedAt: time.Now()}
	encoded, _ := json.Marshal(job)
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Player{}).Where("id = ? AND mana >= ?", player.ID, row.DiscoveryManaCost).Updates(map[string]any{"mana": gorm.Expr("mana - ?", row.DiscoveryManaCost), "state": leylineMeditationState})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("法力不足")
		}
		if row.RequiredItemCount > 0 {
			var item model.Item
			if itemErr := tx.Where("name = ? OR code = ?", row.RequiredItem, row.RequiredItem).First(&item).Error; itemErr != nil {
				return fmt.Errorf("护脉材料%s未配置: %w", row.RequiredItem, itemErr)
			}
			if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, item.ID, -row.RequiredItemCount); err != nil {
				return errors.New("护脉材料不足")
			}
		}
		value := model.PlayerValue{PlayerID: player.ID, Key: "leyline.meditation", Value: string(encoded)}
		return tx.Where("player_id = ? AND key = ?", player.ID, value.Key).Assign(map[string]any{"value": value.Value, "expires_at": nil}).FirstOrCreate(&value).Error
	})
	if err != nil {
		return GameResult{Title: "入脉失败", Content: err.Error(), Actions: []string{"灵脉详情 " + row.Name, "背包", "状态"}}, true, nil
	}
	return GameResult{Title: "灵脉入定", Content: fmt.Sprintf("灵脉：%s\n阶级：%s · 本源：%s\n境界校验：%s%d层，已满足最低第%d境·%d层\n修炼倍率：%.3f倍\n每分钟灵气：%d\n契合判定：%s\n消耗：法力%d · %s×%d\n━━━━━━━━━━━\n你已占据第%d/%d个蒲团位。经过实际时间后发送 `灵脉出定` 结算，高阶灵脉可大幅提高修行。", row.Name, row.Grade, row.Element, player.RealmName, player.RealmLevel, row.MinimumRealmSequence, row.MinimumRealmLevel, row.CultivationMultiplier, row.AuraPerMinute, map[bool]string{true: "灵根契合", false: "灵根不契合"}[g.playerRootElementMatches(player, row.RequiredRootElement)], row.DiscoveryManaCost, row.RequiredItem, row.RequiredItemCount, occupied+1, row.MeditationSlots), Actions: []string{"灵脉出定", "状态", "灵脉详情 " + row.Name}}, true, nil
}

func (g *Game) finishLeylineMeditation(player *model.Player) (GameResult, bool, error) {
	value, err := g.playerValue(player.ID, "leyline.meditation")
	if err != nil {
		return GameResult{Title: "尚未入定", Content: "当前没有灵脉打坐记录。", Actions: []string{"灵脉地图", "寻脉"}}, true, nil
	}
	var job leylineMeditationJob
	if json.Unmarshal([]byte(value), &job) != nil || job.LeylineID == 0 || job.StartedAt.IsZero() {
		return GameResult{Title: "入定记录损坏", Content: "请联系主人检查玩家状态。"}, true, nil
	}
	var row model.WorldLeyline
	if err := g.store.DB.First(&row, job.LeylineID).Error; err != nil {
		return GameResult{}, true, err
	}
	elapsed := time.Since(job.StartedAt)
	minimumMinutes := g.settingInt("leyline.minimum_minutes", 5)
	minutes := int64(elapsed / time.Minute)
	if minutes < minimumMinutes {
		return GameResult{Title: "灵脉周天未成", Content: fmt.Sprintf("已打坐%s，至少需要%d分钟才能完成一个聚灵周天。", formatDuration(elapsed), minimumMinutes), Actions: []string{"灵脉出定", "灵脉详情 " + row.Name}}, true, nil
	}
	maximumMinutes := g.settingInt("leyline.maximum_minutes", 20160)
	if minutes > maximumMinutes {
		minutes = maximumMinutes
	}
	rootFactor := 1.0
	matchText := "灵根不契合"
	if g.playerRootElementMatches(player, row.RequiredRootElement) {
		rootFactor = 1.25 + float64(player.RootQuality)/400
		matchText = fmt.Sprintf("灵根契合×%.2f", rootFactor)
	}
	basePerMinute := g.settingInt("cultivation.base_reward", 5)
	base := (basePerMinute + row.AuraPerMinute) * minutes
	reward := int64(float64(base) * row.CultivationMultiplier * rootFactor)
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{"cultivation": gorm.Expr("cultivation + ?", reward), "state": model.PlayerStateIdle}).Error; err != nil {
			return err
		}
		return tx.Where("player_id = ? AND key = ?", player.ID, "leyline.meditation").Delete(&model.PlayerValue{}).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	_, _ = g.addPlayerValueInt(player.ID, "stats.leyline_minutes", minutes)
	_, _ = g.addPlayerValueInt(player.ID, "stats.leyline_gain", reward)
	return GameResult{Title: "灵脉出定", Content: fmt.Sprintf("灵脉：%s【%s】\n打坐：%d分钟\n基础灵气：%d×%d分钟\n灵脉倍率：×%.3f\n%s\n━━━━━━━━━━━\n最终修为：+%d\n当前修为：%d/%d", row.Name, row.Grade, minutes, basePerMinute+row.AuraPerMinute, minutes, row.CultivationMultiplier, matchText, reward, player.Cultivation+reward, player.CultivationRequired), Actions: []string{"突破", "状态", "灵脉修行榜", "灵脉打坐 " + row.Name}}, true, nil
}

func (g *Game) gatherLeylineAura(player *model.Player, raw string) (GameResult, bool, error) {
	row, err := g.worldLeylineByName(raw)
	if err != nil || row.LocationName != player.Location {
		return GameResult{Title: "采气失败", Content: "必须抵达灵脉所在地图并填写正确灵脉名。", Actions: []string{"寻脉", "灵脉地图"}}, true, nil
	}
	if _, err := g.playerValue(player.ID, fmt.Sprintf("leyline.discovered.%d", row.ID)); err != nil {
		return GameResult{Title: "灵脉未探明", Content: "先发送 `寻脉`。", Actions: []string{"寻脉"}}, true, nil
	}
	if unmet, _ := g.worldLeylineUnmet(player, row); len(unmet) > 0 {
		return GameResult{Title: "采气前置不足", Content: strings.Join(unmet, "\n"), Actions: g.leylineGuidanceActions(player, row, false)}, true, nil
	}
	remaining, allowed, err := g.cooldown(player.ID, fmt.Sprintf("leyline.gather.%d", row.ID), 6*time.Hour)
	if err != nil {
		return GameResult{}, true, err
	}
	if !allowed {
		return GameResult{Title: "灵气尚未回潮", Content: "再次采气还需" + formatDuration(remaining) + "。", Actions: []string{"灵脉打坐 " + row.Name}}, true, nil
	}
	aura := row.AuraPerMinute * int64(10+row.MeditationSlots)
	cultivation := int64(float64(aura) * row.CultivationMultiplier / 4)
	if err := g.store.DB.Model(&model.Player{}).Where("id = ?", player.ID).Update("cultivation", gorm.Expr("cultivation + ?", cultivation)).Error; err != nil {
		return GameResult{}, true, err
	}
	_, _ = g.addPlayerValueInt(player.ID, "currency.leyline_aura", aura)
	return GameResult{Title: "采撷灵气", Content: fmt.Sprintf("灵脉：%s\n收摄灵气：+%d\n转化修为：+%d\n独立道韵：%s\n该灵脉六小时后再次回潮。", row.Name, aura, cultivation, displayConfigText(row.BonusJSON)), Actions: []string{"灵脉打坐 " + row.Name, "状态", "灵脉修行榜"}}, true, nil
}

func (g *Game) leylineCultivationRanking(player *model.Player, raw string) (GameResult, bool, error) {
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(raw), 1)), 1)
	const pageSize = 10
	type row struct {
		PlayerID uint
		DaoName  string
		Gain     int64
		Minutes  int64
	}
	var rows []row
	err := g.store.DB.Raw(`SELECT p.id AS player_id,p.dao_name,
		CAST(g.value AS BIGINT) AS gain,COALESCE(CAST(m.value AS BIGINT),0) AS minutes
		FROM player_values g JOIN players p ON p.id=g.player_id
		LEFT JOIN player_values m ON m.player_id=p.id AND m.key='stats.leyline_minutes'
		WHERE g.key='stats.leyline_gain' AND CAST(g.value AS BIGINT)>0
		ORDER BY gain DESC,p.id ASC`).Scan(&rows).Error
	if err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt((len(rows)+pageSize-1)/pageSize, 1)
	if page > pages {
		page = pages
	}
	start := minInt((page-1)*pageSize, len(rows))
	end := minInt(start+pageSize, len(rows))
	lines := []string{fmt.Sprintf("第%d/%d页 · 按灵脉累计修为排序", page, pages), "━━━━━━━━━━━"}
	for index, row := range rows[start:end] {
		lines = append(lines, fmt.Sprintf("%d. %s · 修为%d · 打坐%d分钟", start+index+1, row.DaoName, row.Gain, row.Minutes))
	}
	if len(rows) == 0 {
		lines = append(lines, "尚无人完成灵脉周天。")
	}
	actions := []string{"灵脉地图", "寻脉"}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("灵脉修行榜 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("灵脉修行榜 %d", page+1))
	}
	return GameResult{Title: "灵脉修行榜", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) worldLeylineByName(raw string) (model.WorldLeyline, error) {
	var row model.WorldLeyline
	err := g.store.DB.Where("enabled = ? AND (name = ? OR code = ?)", true, strings.TrimSpace(raw), strings.TrimSpace(raw)).First(&row).Error
	return row, err
}

func (g *Game) worldLeylineUnmet(player *model.Player, row model.WorldLeyline) ([]string, error) {
	sequence, err := g.playerRealmSequence(player)
	if err != nil {
		return nil, err
	}
	var unmet []string
	requiredRealmName := fmt.Sprintf("第%d境", row.MinimumRealmSequence)
	var requiredRealm model.Realm
	if g.store.DB.Where("sequence = ?", row.MinimumRealmSequence).First(&requiredRealm).Error == nil {
		requiredRealmName = requiredRealm.Name
	}
	if sequence < row.MinimumRealmSequence {
		unmet = append(unmet, fmt.Sprintf("境界不足：需要%s·%d层（第%d境），当前%s·%d层（第%d境）", requiredRealmName, row.MinimumRealmLevel, row.MinimumRealmSequence, player.RealmName, player.RealmLevel, sequence))
	} else if sequence == row.MinimumRealmSequence && player.RealmLevel < row.MinimumRealmLevel {
		unmet = append(unmet, fmt.Sprintf("境界层数不足：需要%s·%d层，当前%s·%d层", requiredRealmName, row.MinimumRealmLevel, player.RealmName, player.RealmLevel))
	}
	if player.CombatPower < row.MinimumCombatPower {
		unmet = append(unmet, fmt.Sprintf("战力不足：需要%d，当前%d", row.MinimumCombatPower, player.CombatPower))
	}
	if player.Spirit < row.MinimumSpirit {
		unmet = append(unmet, fmt.Sprintf("神识不足：需要%d，当前%d", row.MinimumSpirit, player.Spirit))
	}
	if row.RequiredRootElement != "" && !g.playerRootElementMatches(player, row.RequiredRootElement) {
		unmet = append(unmet, fmt.Sprintf("灵根不契合：需要%s本源，当前%s（%s本源）", row.RequiredRootElement, player.SpiritualRoot, displayOr(g.playerSpiritualRootElement(player), "未知")))
	}
	if row.RequiredItemCount > 0 {
		item, itemErr := g.itemByName(row.RequiredItem)
		owned := int64(0)
		if itemErr == nil {
			owned = g.itemQuantity(player.ID, item.ID)
		}
		if itemErr != nil || owned < row.RequiredItemCount {
			unmet = append(unmet, fmt.Sprintf("护脉材料不足：需要%s×%d，持有%d", row.RequiredItem, row.RequiredItemCount, owned))
		}
	}
	if player.Mana < row.DiscoveryManaCost {
		unmet = append(unmet, fmt.Sprintf("法力不足：入脉需要%d，当前%d", row.DiscoveryManaCost, player.Mana))
	}
	return unmet, nil
}

func (g *Game) playerSpiritualRootElement(player *model.Player) string {
	if player == nil || strings.TrimSpace(player.SpiritualRoot) == "" {
		return ""
	}
	var root model.SpiritualRootTemplate
	if g.store.DB.Select("element").Where("enabled = ? AND name = ?", true, player.SpiritualRoot).First(&root).Error == nil && strings.TrimSpace(root.Element) != "" {
		return strings.TrimSpace(root.Element)
	}
	for _, element := range worldLeylineRootElements() {
		if rootElementMatches(player.SpiritualRoot, element) {
			return element
		}
	}
	return ""
}

func (g *Game) playerRootElementMatches(player *model.Player, required string) bool {
	return strings.TrimSpace(required) == "" || rootElementMatches(player.SpiritualRoot, required) || rootElementMatches(g.playerSpiritualRootElement(player), required)
}

func (g *Game) leylineGuidanceActions(player *model.Player, row model.WorldLeyline, includePrimary bool) []string {
	actions := make([]string, 0, 10)
	if includePrimary {
		actions = append(actions, "灵脉打坐 "+row.Name, "采灵气 "+row.Name)
	}
	actions = append(actions, "灵脉详情 "+row.Name)
	if row.RequiredRootElement != "" {
		actions = append(actions, "灵根图鉴 "+row.RequiredRootElement)
	}
	if currentElement := g.playerSpiritualRootElement(player); currentElement != "" {
		actions = append(actions, "灵脉地图 "+currentElement)
	}
	actions = append(actions, "灵根合成")
	if row.RequiredItem != "" {
		actions = append(actions, "物品 "+row.RequiredItem)
	}
	actions = append(actions, "修炼", "状态")
	if planner, err := g.loadWorldRoutePlanner(); err == nil {
		actions = appendWorldRouteAction(actions, planner.shortest(player.Location, row.LocationName))
	}
	actions = append(actions, "寻脉", "灵脉地图")
	return actions
}

func (g *Game) occupiedLeylineSlots(leylineID uint) (int, error) {
	var values []model.PlayerValue
	if err := g.store.DB.Where("key = ?", "leyline.meditation").Find(&values).Error; err != nil {
		return 0, err
	}
	occupied := 0
	for _, value := range values {
		var job leylineMeditationJob
		if json.Unmarshal([]byte(value.Value), &job) != nil || job.StartedAt.IsZero() || time.Since(job.StartedAt) > 15*24*time.Hour {
			_ = g.store.DB.Delete(&value).Error
			continue
		}
		var player model.Player
		if g.store.DB.Select("state").First(&player, value.PlayerID).Error != nil || player.State != leylineMeditationState {
			_ = g.store.DB.Delete(&value).Error
			continue
		}
		if job.LeylineID == leylineID {
			occupied++
		}
	}
	return occupied, nil
}

func rootElementMatches(root, element string) bool {
	if strings.Contains(root, element) {
		return true
	}
	aliases := map[string][]string{
		"庚金": {"金", "剑"}, "乙木": {"木", "生灵"}, "玄水": {"水", "沧海"}, "离火": {"火", "太阳"}, "厚土": {"土", "山"},
		"风灵": {"风"}, "雷灵": {"雷"}, "冰魄": {"冰", "阴", "月"}, "时空": {"时", "空", "虚"}, "轮回": {"轮回", "六道"},
		"风雷": {"风", "雷"}, "太阴": {"冰", "阴", "月"}, "太阳": {"阳", "日", "火"}, "星辰": {"星"},
	}
	for _, alias := range aliases[element] {
		if strings.Contains(root, alias) {
			return true
		}
	}
	return false
}

func worldLeylineRootElements() []string {
	return []string{"庚金", "乙木", "玄水", "离火", "厚土", "风灵", "雷灵", "冰魄", "时空", "轮回"}
}
