package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/handler"
	"xianlv/internal/model"
	"xianlv/internal/storage"
)

func (g *Game) executeMap(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	switch command.Spec.ID {
	case 241:
		return g.worldMap(player)
	case 242:
		return g.travelTo(player, command.RawArguments)
	case 244:
		return g.bossStatus(player)
	case 245:
		return g.huntBoss(player)
	case 252:
		return g.worldMap(player)
	default:
		return GameResult{}, false, nil
	}
}

func (g *Game) worldMap(player *model.Player) (GameResult, bool, error) {
	location, err := g.currentWorldLocation(player)
	if err != nil {
		return GameResult{}, true, err
	}
	if location.ID == 0 {
		return GameResult{Title: "世界地图", Content: "诸界舆图尚未载入，请主人检查地图地点数据。"}, true, nil
	}
	activatedNow, err := g.activateTeleportLocation(player.ID, location)
	if err != nil {
		return GameResult{}, true, err
	}

	neighbors, err := g.neighborLocations(location)
	if err != nil {
		return GameResult{Title: "地图配置错误", Content: err.Error()}, true, nil
	}
	var total, accessible, totalRegions, accessibleRegions int64
	baseLocations := g.store.DB.Model(&model.WorldLocation{}).Where("enabled = ?", true)
	if err := baseLocations.Count(&total).Error; err != nil {
		return GameResult{}, true, err
	}
	if err := g.store.DB.Model(&model.WorldLocation{}).Where("enabled = ?", true).Distinct("region").Count(&totalRegions).Error; err != nil {
		return GameResult{}, true, err
	}
	sequence, err := g.playerRealmSequence(player)
	if err != nil {
		return GameResult{}, true, err
	}
	accessibleQuery := g.store.DB.Model(&model.WorldLocation{}).Where("enabled = ?", true).
		Where("(minimum_realm_sequence <= 0 OR minimum_realm_sequence < ? OR (minimum_realm_sequence = ? AND minimum_realm_level <= ?))", sequence, sequence, player.RealmLevel).
		Where("minimum_level <= ?", player.Level)
	if err := accessibleQuery.Count(&accessible).Error; err != nil {
		return GameResult{}, true, err
	}
	if err := accessibleQuery.Distinct("region").Count(&accessibleRegions).Error; err != nil {
		return GameResult{}, true, err
	}
	npcs := decodeTextList(location.NPCJSON)
	tasks := decodeTextList(location.TasksJSON)
	actions := []string{"位置", "状态", "传送阵", "传送列表", "诸界列表", "帮助"}
	lines := []string{
		fmt.Sprintf("🚶 %s 踏入此地，", displayOr(player.DaoName, "无名道者")),
		"⛩️ **" + location.Name + "**",
		"---",
		displayOr(location.Description, "此地云气氤氲，尚无详细记载。"),
		"",
		"🏔️ 当前场景",
		fmt.Sprintf("界域：%s · 当前可踏足：%d/%d处", displayOr(location.Region, "未知区域"), accessible, total),
		fmt.Sprintf("诸界进度：已解锁%d/%d界 · 每座系统界域1000处地图", accessibleRegions, totalRegions),
		fmt.Sprintf("地图前置：第%d境·第%d层 · 角色等级%d", maxInt(location.MinimumRealmSequence, 1), maxInt(location.MinimumRealmLevel, 1), maxInt(location.MinimumLevel, 1)),
	}
	markdownLines := append([]string(nil), lines...)
	if len(npcs) > 0 {
		lines = append(lines, "👥 NPC："+strings.Join(npcs, "、"))
		links := make([]string, 0, len(npcs))
		for _, npc := range npcs {
			links = append(links, markdownInlineCommand(npc, "对话 "+npc))
		}
		markdownLines = append(markdownLines, "👥 NPC："+strings.Join(links, "、"))
	} else {
		lines = append(lines, "👥 NPC：暂无记载")
		markdownLines = append(markdownLines, "👥 NPC：暂无记载")
	}
	if len(tasks) > 0 {
		lines = append(lines, "📜 当前地图任务：")
		markdownLines = append(markdownLines, "📜 当前地图任务：")
		for index, task := range tasks {
			lines = append(lines, fmt.Sprintf("  %d. %s", index+1, task))
			markdownLines = append(markdownLines, fmt.Sprintf("  %d. %s", index+1, markdownInlineCommand(task, "接任务 "+task)))
		}
	} else {
		lines = append(lines, "📜 当前地图任务：暂无任务")
		markdownLines = append(markdownLines, "📜 当前地图任务：暂无任务")
	}
	teleport := "无阵"
	if isTeleportLocation(location) {
		teleport = "已激活"
	}
	if activatedNow {
		teleport += "（新刻录）"
	}
	crossRegion := "未开启"
	if location.CrossRegionEnabled {
		crossRegion = "已开启"
	}
	lines = append(lines, fmt.Sprintf("✨ 传送阵：%s | 🌌 跨界门：%s", teleport, crossRegion))
	markdownLines = append(markdownLines, fmt.Sprintf("✨ %s | 🌌 %s", markdownInlineCommand("传送阵："+teleport, "传送阵"), markdownInlineCommand("跨界门："+crossRegion, "诸界列表")))
	if location.MonsterName != "" {
		actions = append(actions, "挑战 "+location.MonsterName)
		monsterText := fmt.Sprintf("%s（战力%d · 遭遇率%.0f%%）", location.MonsterName, location.MonsterPower, location.MonsterEncounterRate*100)
		lines = append(lines, "", "👾 对战", "  1. "+monsterText)
		markdownLines = append(markdownLines, "", "👾 对战", "  1. "+markdownInlineCommand(monsterText, "挑战 "+location.MonsterName))
	}
	if location.BossName != "" {
		actions = append(actions, "首领", "讨伐")
		bossText := fmt.Sprintf("%s（战力%d）", location.BossName, location.BossPower)
		lines = append(lines, "👑 区域首领", "  "+bossText)
		markdownLines = append(markdownLines, "👑 区域首领", "  "+markdownInlineCommand(bossText, "首领"))
	}
	if strings.TrimSpace(location.ResourceName) != "" {
		actions = append(actions, "采集 "+location.ResourceName)
		quantity, refreshAt, stateErr := mapResourceStateFromDB(g.store.DB, player.ID, location)
		if stateErr != nil {
			return GameResult{}, true, stateErr
		}
		cooldown := maxInt(location.ResourceCooldownMin, 10)
		resourceText := fmt.Sprintf("%s x%d（采尽后%d分钟刷新）", location.ResourceName, quantity, cooldown)
		if quantity == 0 && refreshAt.After(time.Now()) {
			resourceText = fmt.Sprintf("%s x0（%s后刷新）", location.ResourceName, formatDuration(time.Until(refreshAt)))
		}
		lines = append(lines, "🌿 区域采集", "  "+resourceText)
		markdownLines = append(markdownLines, "🌿 区域采集", "  "+markdownInlineCommand(resourceText, "采集 "+location.ResourceName))
	}
	var leylines []model.WorldLeyline
	if err := g.store.DB.Where("enabled = ? AND location_name = ?", true, location.Name).Order("minimum_realm_sequence,sort_order").Limit(3).Find(&leylines).Error; err == nil && len(leylines) > 0 {
		lines = append(lines, "🔆 修仙界灵脉")
		markdownLines = append(markdownLines, "🔆 修仙界灵脉")
		for _, leyline := range leylines {
			text := fmt.Sprintf("%s【%s · %.3f倍】", leyline.Name, leyline.Grade, leyline.CultivationMultiplier)
			lines = append(lines, "  "+text)
			markdownLines = append(markdownLines, "  "+markdownInlineCommand(text, "灵脉详情 "+leyline.Name))
			actions = append(actions, "灵脉详情 "+leyline.Name)
		}
		lines = append(lines, "  先发送寻脉探明入口，满足前置后可入脉打坐。")
		markdownLines = append(markdownLines, "  "+markdownInlineCommand("寻脉探查入口", "寻脉"))
		actions = append(actions, "寻脉")
	}

	lines = append(lines, "", "🗺️ 四方通路：")
	markdownLines = append(markdownLines, "", "🗺️ 四方通路：")
	if len(neighbors) == 0 {
		lines = append(lines, "  当前地点没有配置可通行路线。")
		markdownLines = append(markdownLines, "  当前地点没有配置可通行路线。")
	} else {
		directions := []string{"⬅️ 左", "➡️ 右", "⬆️ 前", "⬇️ 后"}
		for index, neighbor := range neighbors {
			actions = append(actions, "前往 "+neighbor.Name)
			direction := "•"
			if index < len(directions) {
				direction = directions[index]
			}
			plain := fmt.Sprintf("  %s：%s · 体力%d", direction, neighbor.Name, maxInt64(neighbor.StaminaCost, 0))
			linked := fmt.Sprintf("  %s：%s", direction, markdownInlineCommand(neighbor.Name, "前往 "+neighbor.Name))
			lines = append(lines, plain)
			markdownLines = append(markdownLines, linked)
		}
	}
	return GameResult{Title: "世界地图", Content: strings.Join(lines, "\n"), MarkdownContent: strings.Join(markdownLines, "\n"), ImageURL: location.ImageURL, Actions: actions}, true, nil
}

func (g *Game) travelTo(player *model.Player, destinationText string) (GameResult, bool, error) {
	destinationText = strings.TrimSpace(destinationText)
	if destinationText == "" {
		return GameResult{Title: "前往地点", Content: "请输入：`前往 地点`", Actions: []string{"地图"}}, true, nil
	}
	if player.State != "" && player.State != model.PlayerStateIdle {
		return GameResult{Title: "暂时无法移动", Content: "当前状态：" + player.State + "。请先结束当前行动。", Actions: []string{"状态"}}, true, nil
	}
	if player.Health <= 0 {
		return GameResult{Title: "👻 元神离体，无法赶路", Content: "阵亡状态不能沿地图跋涉或传送。请先回城复生。", Actions: []string{"回城复活", "状态"}}, true, nil
	}

	var destination model.WorldLocation
	err := g.store.DB.Where("enabled = ? AND (name = ? OR code = ?)", true, destinationText, destinationText).First(&destination).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return GameResult{Title: "地点不存在", Content: "没有找到已启用的地点：" + destinationText, Actions: []string{"地图"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	if destination.Name == player.Location {
		return GameResult{Title: "已在此处", Content: "你当前就在" + destination.Name + "。", Actions: []string{"地图"}}, true, nil
	}

	current, err := g.currentWorldLocation(player)
	if err != nil {
		return GameResult{}, true, err
	}
	if current.ID != 0 {
		neighborNames, parseErr := decodeNeighborNames(current.NeighborsJSON)
		if parseErr != nil {
			return GameResult{Title: "地图配置错误", Content: fmt.Sprintf("%s的相邻路线不是有效JSON：%v", current.Name, parseErr)}, true, nil
		}
		if !containsText(neighborNames, destination.Name) && !containsText(neighborNames, destination.Code) {
			return GameResult{Title: "路线不通", Content: fmt.Sprintf("无法从%s直接前往%s，请按地图相邻路线移动。", current.Name, destination.Name), Actions: []string{"地图"}}, true, nil
		}
	}

	currentRealmSequence, err := g.playerRealmSequence(player)
	if err != nil {
		return GameResult{}, true, err
	}
	if destination.MinimumRealmSequence > currentRealmSequence {
		required := fmt.Sprintf("第%d境", destination.MinimumRealmSequence)
		var realm model.Realm
		if g.store.DB.Where("sequence >= ?", destination.MinimumRealmSequence).Order("sequence").First(&realm).Error == nil {
			required = realm.Name
		}
		return GameResult{Title: "境界不足", Content: fmt.Sprintf("%s至少需要%s方可进入。", destination.Name, required), Actions: []string{"地图", "修炼"}}, true, nil
	}
	if destination.MinimumRealmLevel > 0 && currentRealmSequence == destination.MinimumRealmSequence && player.RealmLevel < destination.MinimumRealmLevel {
		return GameResult{Title: "境界层数不足", Content: fmt.Sprintf("%s需要当前大境至少第%d层，当前第%d层。\n先完成本层修行与破境，方可继续前行。", destination.Name, destination.MinimumRealmLevel, player.RealmLevel), Actions: []string{"修炼", "突破", "状态", "地图"}}, true, nil
	}
	if destination.MinimumLevel > 0 && player.Level < destination.MinimumLevel {
		return GameResult{Title: "等级不足", Content: fmt.Sprintf("%s需要等级%d，当前等级%d。", destination.Name, destination.MinimumLevel, player.Level), Actions: []string{"地图", "状态"}}, true, nil
	}

	cost := maxInt64(destination.StaminaCost, 0)
	remaining, err := g.useStamina(player.ID, cost)
	if err != nil {
		return GameResult{Title: "体力不足", Content: err.Error(), Actions: []string{"地图"}}, true, nil
	}
	if err := g.store.DB.Model(player).Update("location", destination.Name).Error; err != nil {
		if restoreErr := g.setPlayerValueInt(player.ID, "stamina.value", remaining+cost); restoreErr != nil {
			return GameResult{}, true, fmt.Errorf("update location: %w; restore stamina: %v", err, restoreErr)
		}
		return GameResult{}, true, err
	}
	player.Location = destination.Name
	activatedNow, activationErr := g.activateTeleportLocation(player.ID, destination)
	if activationErr != nil {
		return GameResult{}, true, activationErr
	}

	mapResult, _, err := g.worldMap(player)
	if err != nil {
		return GameResult{}, true, err
	}
	activationText := ""
	if activatedNow {
		activationText = "\n✨ 首次抵达，已永久刻录此地传送阵纹。"
	}
	arrival := fmt.Sprintf("🚶 %s 沿山河通路跋涉至此，\n已抵达：【%s - %s】\n消耗体力：%d · 剩余体力：%d%s\n\n", displayOr(player.DaoName, "无名道者"), displayOr(destination.Region, "未知区域"), destination.Name, cost, remaining, activationText)
	mapResult.Title = "🗺️ 抵达新地图"
	if current.ID != 0 && current.Region != destination.Region {
		mapResult.Title = "🌌 跨界跋涉"
	}
	mapResult.Content = arrival + mapResult.Content
	mapResult.MarkdownContent = arrival + mapResult.MarkdownContent
	return mapResult, true, nil
}

func (g *Game) gatherLocalMapResource(player *model.Player, raw string) (GameResult, bool, error) {
	location, err := g.currentWorldLocation(player)
	if err != nil {
		return GameResult{}, true, err
	}
	name := strings.TrimSpace(raw)
	if name != location.ResourceName || name == "" {
		return GameResult{Title: "采集目标不存在", Content: fmt.Sprintf("当前地点：%s\n可采集资源：%s", location.Name, displayOr(location.ResourceName, "无")), Actions: []string{"采集 " + location.ResourceName, "位置"}}, true, nil
	}
	item, itemErr := g.itemByName(name)
	if itemErr != nil {
		return GameResult{Title: "采集道纹缺失", Content: "地图资源未关联有效物品：“" + name + "”，本次未消耗采集次数，请主人检查资源配置。"}, true, nil
	}
	duration := time.Duration(maxInt(location.ResourceCooldownMin, 10)) * time.Minute
	var remainingStock int64
	var refreshAt time.Time
	var available bool
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		stock, currentRefresh, stateErr := mapResourceStateFromDB(tx, player.ID, location)
		if stateErr != nil {
			return stateErr
		}
		if stock <= 0 {
			refreshAt = currentRefresh
			return nil
		}
		available = true
		remainingStock = stock - 1
		if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, item.ID, 1); err != nil {
			return err
		}
		stockKey := fmt.Sprintf("map.resource.stock.%d", location.ID)
		if err := upsertPlayerValueTx(tx, player.ID, stockKey, strconv.FormatInt(remainingStock, 10), nil); err != nil {
			return err
		}
		refreshKey := fmt.Sprintf("map.resource.refresh.%d", location.ID)
		if remainingStock == 0 {
			refreshAt = time.Now().Add(duration)
			if err := upsertPlayerValueTx(tx, player.ID, refreshKey, refreshAt.Format(time.RFC3339Nano), &refreshAt); err != nil {
				return err
			}
		} else if err := tx.Where("player_id = ? AND key = ?", player.ID, refreshKey).Delete(&model.PlayerValue{}).Error; err != nil {
			return err
		}
		collects := playerValueIntTx(tx, player.ID, "stats.collects", 0) + 1
		return upsertPlayerValueTx(tx, player.ID, "stats.collects", strconv.FormatInt(collects, 10), nil)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	if !available {
		wait := time.Duration(0)
		if refreshAt.After(time.Now()) {
			wait = time.Until(refreshAt)
		}
		return GameResult{Title: "灵植尚未再生", Content: fmt.Sprintf("%s当前剩余0株，整片灵植还需%s恢复。", name, formatDuration(wait)), Actions: []string{"位置", "背包"}}, true, nil
	}
	refreshText := fmt.Sprintf("当前还有%d株，可继续采集。", remainingStock)
	actions := []string{"采集 " + item.Name, "物品 " + item.Name, "背包", "位置"}
	if remainingStock == 0 {
		refreshText = fmt.Sprintf("本轮已全部采尽，%s后整片刷新。", formatDuration(duration))
		actions = []string{"物品 " + item.Name, "背包", "位置"}
	}
	return GameResult{Title: "区域采集", Content: fmt.Sprintf("地点：%s\n本次获得：%s×1\n剩余可采：%d株\n%s\n用途：%s", location.Name, item.Name, remainingStock, refreshText, item.Description), ImageURL: item.ImageURL, Actions: actions}, true, nil
}

func mapResourceStateFromDB(db *gorm.DB, playerID uint, location model.WorldLocation) (int64, time.Time, error) {
	capacity := int64(maxInt(location.ResourceQuantity, 1))
	now := time.Now()
	refreshKey := fmt.Sprintf("map.resource.refresh.%d", location.ID)
	var refreshRow model.PlayerValue
	refreshErr := db.Where("player_id = ? AND key = ?", playerID, refreshKey).First(&refreshRow).Error
	if refreshErr == nil {
		if refreshAt, parseErr := time.Parse(time.RFC3339Nano, refreshRow.Value); parseErr == nil && refreshAt.After(now) {
			return 0, refreshAt, nil
		}
	} else if !errors.Is(refreshErr, gorm.ErrRecordNotFound) {
		return 0, time.Time{}, refreshErr
	}

	stockKey := fmt.Sprintf("map.resource.stock.%d", location.ID)
	var stockRow model.PlayerValue
	stockErr := db.Where("player_id = ? AND key = ?", playerID, stockKey).First(&stockRow).Error
	if stockErr == nil && refreshErr == nil {
		// An expired refresh always starts a new full growth cycle.
		return capacity, time.Time{}, nil
	}
	if stockErr == nil {
		stock, parseErr := strconv.ParseInt(stockRow.Value, 10, 64)
		if parseErr != nil {
			return capacity, time.Time{}, nil
		}
		return min64(max64(stock, 0), capacity), time.Time{}, nil
	}
	if !errors.Is(stockErr, gorm.ErrRecordNotFound) {
		return 0, time.Time{}, stockErr
	}

	// Honor the old all-at-once cooldown once, then switch to per-plant stock.
	legacyKey := fmt.Sprintf("cooldown.map.resource.%d", location.ID)
	var legacyRow model.PlayerValue
	legacyErr := db.Where("player_id = ? AND key = ?", playerID, legacyKey).First(&legacyRow).Error
	if legacyErr == nil {
		if legacyUntil, parseErr := time.Parse(time.RFC3339Nano, legacyRow.Value); parseErr == nil && legacyUntil.After(now) {
			return 0, legacyUntil, nil
		}
	} else if !errors.Is(legacyErr, gorm.ErrRecordNotFound) {
		return 0, time.Time{}, legacyErr
	}
	return capacity, time.Time{}, nil
}

func (g *Game) bossStatus(player *model.Player) (GameResult, bool, error) {
	location, err := g.currentWorldLocation(player)
	if err != nil {
		return GameResult{}, true, err
	}
	if location.BossName == "" {
		return GameResult{Title: "区域首领", Content: "当前地点尚未配置区域Boss。", Actions: []string{"地图"}}, true, nil
	}
	key := "boss." + location.Code + ".cooldown"
	value, _ := g.playerValue(player.ID, key)
	status := "已刷新，可立即讨伐"
	if until, parseErr := time.Parse(time.RFC3339Nano, value); parseErr == nil && until.After(time.Now()) {
		status = "距离下一次刷新还需" + formatDuration(time.Until(until))
	}
	return GameResult{Title: "区域首领", Content: fmt.Sprintf("地点：%s\n首领：%s\n战力：%d\n状态：%s\n奖励：击败后按地图配置发放。", location.Name, location.BossName, location.BossPower, status), Actions: []string{"讨伐", "地图"}}, true, nil
}

func (g *Game) huntBoss(player *model.Player) (GameResult, bool, error) {
	location, err := g.currentWorldLocation(player)
	if err != nil {
		return GameResult{}, true, err
	}
	if location.BossName == "" {
		return GameResult{Title: "没有首领", Content: "当前地点没有配置区域Boss。", Actions: []string{"地图"}}, true, nil
	}
	if player.State != "" && player.State != model.PlayerStateIdle {
		if player.State == model.PlayerStateBattling {
			return GameResult{Title: "已在战斗中", Content: "请完成当前回合或发送 `投降`，不能重复发起讨伐。", Actions: []string{"攻击", "技能", "防御", "投降"}}, true, nil
		}
		return GameResult{Title: "当前无法讨伐", Content: "当前状态：" + player.State + "。请先结束当前行动。", Actions: []string{"状态"}}, true, nil
	}
	if player.Health <= 1 {
		return GameResult{Title: "重伤难战", Content: "当前气血过低，请先疗伤再挑战首领。", Actions: []string{"疗伤", "状态"}}, true, nil
	}
	key := "boss." + location.Code + ".cooldown"
	if value, valueErr := g.playerValue(player.ID, key); valueErr == nil {
		if until, parseErr := time.Parse(time.RFC3339Nano, value); parseErr == nil && until.After(time.Now()) {
			return GameResult{Title: "首领未刷新", Content: "还需" + formatDuration(time.Until(until)) + "才能再次讨伐。", Actions: []string{"首领", "地图"}}, true, nil
		}
	}
	const staminaCost int64 = 20
	remaining, err := g.useStamina(player.ID, staminaCost)
	if err != nil {
		return GameResult{Title: "无法讨伐", Content: err.Error(), Actions: []string{"地图"}}, true, nil
	}
	enemyPower := max64(location.BossPower, 80)
	enemyHP := max64(enemyPower, max64(player.PhysicalAttack, player.MagicAttack)*6)
	effective := g.playerWithActiveSkillStats(player)
	state := mapMonsterBattleState{
		LocationID: location.ID, BattleKind: "首领", Round: 1, EnemyName: location.BossName,
		EnemyPower: enemyPower, PlayerHP: effective.Health, PlayerMana: effective.Mana,
		EnemyHP: enemyHP, EnemyMaxHP: enemyHP, StartedAt: time.Now().UnixMilli(),
	}
	if err := g.beginPVEBattle(player.ID, state); err != nil {
		return GameResult{}, true, err
	}
	_, _ = g.addPlayerValueInt(player.ID, "stats.boss_attempts", 1)
	content := fmt.Sprintf("道友「%s」向镇域首领发起讨伐！\n━━━━━━━━━━━\n【敌方阵位】\nD1：%s【战力%d】\n气血：%d/%d\n特性：气血低于35%%后进入狂暴\n\n【我方阵位】\nA1：%s【战力%d】\n气血：%d/%d · 法力：%d/%d\n\n消耗体力：%d · 剩余体力：%d\n━━━━━━━━━━━\n战斗不会自动结算。现在轮到你行动，请选择普通攻击、已学功法或防御。", player.DaoName, location.BossName, enemyPower, enemyHP, enemyHP, player.DaoName, player.CombatPower, effective.Health, effective.MaxHealth, effective.Mana, effective.MaxMana, staminaCost, remaining)
	return GameResult{Title: "镇域首领讨伐开始", Content: content, ImageURL: location.ImageURL, Actions: []string{"攻击", "技能", "功法", "防御", "投降"}}, true, nil
}

func decodeRewardMap(raw string) map[string]int64 {
	values := make(map[string]int64)
	if strings.TrimSpace(raw) == "" {
		return values
	}
	_ = json.Unmarshal([]byte(raw), &values)
	return values
}

func rewardValue(values map[string]int64, key string, fallback int64) int64 {
	if value := values[key]; value > 0 {
		return value
	}
	return fallback
}

func (g *Game) currentWorldLocation(player *model.Player) (model.WorldLocation, error) {
	var location model.WorldLocation
	err := g.store.DB.Where("enabled = ? AND name = ?", true, player.Location).First(&location).Error
	if err == nil {
		return location, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.WorldLocation{}, err
	}
	err = g.store.DB.Where("enabled = ?", true).Order("sort_order, id").First(&location).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.WorldLocation{}, nil
	}
	return location, err
}

func (g *Game) neighborLocations(location model.WorldLocation) ([]model.WorldLocation, error) {
	names, err := decodeNeighborNames(location.NeighborsJSON)
	if err != nil {
		return nil, fmt.Errorf("%s的相邻路线配置错误: %w", location.Name, err)
	}
	if len(names) == 0 {
		return nil, nil
	}
	var rows []model.WorldLocation
	if err := g.store.DB.Where("enabled = ? AND (name IN ? OR code IN ?)", true, names, names).Find(&rows).Error; err != nil {
		return nil, err
	}
	byKey := make(map[string]model.WorldLocation, len(rows)*2)
	for _, row := range rows {
		byKey[row.Name] = row
		byKey[row.Code] = row
	}
	ordered := make([]model.WorldLocation, 0, len(rows))
	seen := make(map[uint]struct{}, len(rows))
	for _, name := range names {
		row, exists := byKey[name]
		if !exists {
			continue
		}
		if _, exists := seen[row.ID]; exists {
			continue
		}
		seen[row.ID] = struct{}{}
		ordered = append(ordered, row)
	}
	return ordered, nil
}

func (g *Game) playerRealmSequence(player *model.Player) (int, error) {
	var realm model.Realm
	err := g.store.DB.Where("id = ? OR name = ?", player.RealmID, player.RealmName).Order("sequence").First(&realm).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	return realm.Sequence, err
}

func decodeNeighborNames(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var names []string
	if err := json.Unmarshal([]byte(value), &names); err != nil {
		return nil, err
	}
	return names, nil
}

func decodeTextList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(value), &values); err != nil {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func containsText(values []string, wanted string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(wanted)) {
			return true
		}
	}
	return false
}

func maxInt64(value, minimum int64) int64 {
	if value < minimum {
		return minimum
	}
	return value
}
