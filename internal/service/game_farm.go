package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"xianlv/internal/model"
)

func (g *Game) farmOverview(player *model.Player, argument string) (GameResult, bool, error) {
	mansion, created, err := g.getOrCreateMansion(player)
	if err != nil {
		return GameResult{}, true, err
	}
	weather, weatherEvents, err := g.applyDailyFarmWeather(player, mansion)
	if err != nil {
		return GameResult{}, true, err
	}
	var crops []model.MansionCrop
	if err := g.store.DB.Where("mansion_id = ? AND harvested = ?", mansion.ID, false).Order("plot").Find(&crops).Error; err != nil {
		return GameResult{}, true, err
	}
	farmLevel := maxInt(mansion.FarmLevel, 1)
	maxPlots := maxInt(farmLevel*2, 2)
	page := int(parsePositiveInt(strings.TrimSpace(argument), 1))
	const pageSize = 8
	totalPages := maxInt((maxPlots+pageSize-1)/pageSize, 1)
	if page > totalPages {
		page = totalPages
	}
	occupiedCount := len(crops)
	lines := []string{
		fmt.Sprintf("洞天：%s", mansion.Name),
		fmt.Sprintf("田阶：%s · 灵田%d阶", farmGrade(farmLevel), farmLevel),
		fmt.Sprintf("田契：共%d块 · 空闲%d块 · 生长中%d块", maxPlots, maxInt(maxPlots-occupiedCount, 0), occupiedCount),
		fmt.Sprintf("灵壤道效：生长缩短%d分钟 · 基础增产+%d株", farmGrowthReduction(farmLevel), farmLevel/2),
		fmt.Sprintf("今日天象：%s · %s", weather.Name, map[bool]string{true: fmt.Sprintf("已影响%d块田垄", len(weatherEvents)), false: "当前无新增影响"}[len(weatherEvents) > 0]),
		"━━━━━━━━━━━",
	}
	byPlot := make(map[int]model.MansionCrop, len(crops))
	for _, crop := range crops {
		byPlot[crop.Plot] = crop
	}
	startPlot := (page-1)*pageSize + 1
	endPlot := minInt(startPlot+pageSize-1, maxPlots)
	for plot := startPlot; plot <= endPlot; plot++ {
		crop, ok := byPlot[plot]
		if !ok {
			lines = append(lines, fmt.Sprintf("地块%s：空闲，可播种", chineseDigit(plot)))
			continue
		}
		var item model.Item
		_ = g.store.DB.First(&item, crop.ItemID).Error
		state := "生长中 · 还需" + formatDuration(time.Until(crop.ReadyAt))
		if !crop.ReadyAt.After(time.Now()) {
			state = "已经成熟"
		}
		care := []string{}
		if !crop.Watered {
			care = append(care, "待浇水")
		}
		if !crop.Weeded {
			care = append(care, "有杂灵草")
		}
		if !crop.PestFree {
			care = append(care, "有噬灵虫")
		}
		if crop.Fertilized {
			care = append(care, "已施"+displayOr(crop.FertilizerName, "灵肥"))
		} else {
			care = append(care, "待施灵肥")
		}
		guard := "无守护"
		if crop.Protected {
			guard = "护田灵兽守护"
		}
		lines = append(lines, fmt.Sprintf("地块%s：%s · 预计%d株\n  %s · %s · %s", chineseDigit(plot), displayOr(item.Name, "未知灵植"), crop.Quantity, state, strings.Join(care, "、"), guard))
	}
	if created {
		lines = append(lines, "━━━━━━━━━━━", "你引来第一缕洞天灵泉，凡土由此化作灵壤。先到种子商店购入灵种，再选择空闲田垄播种。")
	}
	materialCost, stoneCost, requiredRealm := g.farmUpgradeRequirements(farmLevel + 1)
	lines = append(lines,
		"━━━━━━━━━━━",
		fmt.Sprintf("洞天繁荣：%d · 累计收获：%d株 · 潜入采灵：%d次", mansion.Prosperity, g.playerValueInt(player.ID, "farm.harvested", 0), g.playerValueInt(player.ID, "farm.gathered", 0)),
		fmt.Sprintf("下一田阶：新增2块灵地 · 需仙府%d级、%s、仙府材料×%d、灵石×%d", farmLevel+1, requiredRealm, materialCost, stoneCost),
	)
	if totalPages > 1 {
		lines = append(lines, fmt.Sprintf("第%d/%d页 · 每页展示%d块灵地", page, totalPages, pageSize))
	}
	actions := []string{"种子商店", "种植", "一键种植", "一键除草", "一键除虫", "施肥 1", "一键施肥 灵壤肥", "灵肥图鉴", "收菜", "升级灵田", "土地详情", "灵田仓库", "偷菜", "护田记录", "灵田说明"}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("灵田 %d", page-1))
	}
	if page < totalPages {
		actions = append(actions, fmt.Sprintf("灵田 %d", page+1))
	}
	return GameResult{Title: "🌿 仙府灵田", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func farmGrade(level int) string {
	grades := []string{"凡壤初辟", "聚灵药畦", "玄脉灵园", "地泉灵圃", "五行药境", "洞天灵泽", "云上仙圃", "星髓药天", "太虚灵域", "造化仙田"}
	if level < 1 {
		level = 1
	}
	if level <= len(grades) {
		return grades[level-1]
	}
	return fmt.Sprintf("造化仙田·%d重", level-len(grades)+1)
}

func farmGrowthReduction(level int) int { return maxInt(level*2, 2) }

func (g *Game) farmUpgradeRequirements(nextLevel int) (int64, int64, string) {
	if nextLevel < 2 {
		nextLevel = 2
	}
	materialCost := int64(nextLevel*2 + 1)
	stoneCost := int64(nextLevel * nextLevel * 80)
	requiredSequence := 1 + (nextLevel-1)/4
	realmName := fmt.Sprintf("第%d大境", requiredSequence)
	var realm model.Realm
	if g.store.DB.Where("sequence = ?", requiredSequence).First(&realm).Error == nil {
		realmName = realm.Name
	}
	return materialCost, stoneCost, realmName
}

func (g *Game) upgradeFarm(player *model.Player) (GameResult, bool, error) {
	mansion, _, err := g.getOrCreateMansion(player)
	if err != nil {
		return GameResult{}, true, err
	}
	currentLevel := maxInt(mansion.FarmLevel, 1)
	nextLevel := currentLevel + 1
	materialCost, stoneCost, requiredRealmName := g.farmUpgradeRequirements(nextLevel)
	requiredSequence := 1 + (nextLevel-1)/4
	var currentRealm model.Realm
	if err := g.store.DB.First(&currentRealm, player.RealmID).Error; err != nil {
		return GameResult{}, true, err
	}
	var material model.Item
	if err := g.store.DB.Where("name = ?", "仙府材料").First(&material).Error; err != nil {
		return GameResult{Title: "灵田升阶配置缺失", Content: "物品库中缺少仙府材料，请管理员修复数据后再试。"}, true, nil
	}
	ownedMaterials := g.itemQuantity(player.ID, material.ID)
	var unmet []string
	if mansion.Level < nextLevel {
		unmet = append(unmet, fmt.Sprintf("仙府需达到%d级，当前%d级", nextLevel, mansion.Level))
	}
	if currentRealm.Sequence < requiredSequence {
		unmet = append(unmet, fmt.Sprintf("境界需达到%s，当前%s", requiredRealmName, player.RealmName))
	}
	if ownedMaterials < materialCost {
		unmet = append(unmet, fmt.Sprintf("仙府材料需%d份，当前%d份", materialCost, ownedMaterials))
	}
	if player.SpiritStones < stoneCost {
		unmet = append(unmet, fmt.Sprintf("灵石需%d枚，当前%d枚", stoneCost, player.SpiritStones))
	}
	if len(unmet) > 0 {
		return GameResult{Title: "🌿 灵田升阶条件不足", Content: fmt.Sprintf("目标：%s · 灵田%d阶\n升阶道法：拓宽洞天田契，引地脉灵泉重炼灵壤。\n━━━━━━━━━━━\n未满足：\n- %s\n━━━━━━━━━━━\n升阶完成后永久新增2块灵地，生长再缩短2分钟，并提高高阶灵植基础产量。", farmGrade(nextLevel), nextLevel, strings.Join(unmet, "\n- ")), Actions: []string{"升级府", "物品 仙府材料", "货铺", "副本", "灵田"}}, true, nil
	}
	prosperityGain := int64(nextLevel * 15)
	if err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := consumeNamedItemTx(tx, player.ID, "仙府材料", materialCost); err != nil {
			return err
		}
		stoneUpdate := tx.Model(&model.Player{}).Where("id = ? AND spirit_stones >= ?", player.ID, stoneCost).Update("spirit_stones", gorm.Expr("spirit_stones - ?", stoneCost))
		if stoneUpdate.Error != nil {
			return stoneUpdate.Error
		}
		if stoneUpdate.RowsAffected != 1 {
			return fmt.Errorf("灵石余额在升阶时发生变化")
		}
		farmUpdate := tx.Model(&model.Mansion{}).Where("id = ? AND farm_level = ? AND level >= ?", mansion.ID, mansion.FarmLevel, nextLevel).Updates(map[string]any{"farm_level": nextLevel, "prosperity": gorm.Expr("prosperity + ?", prosperityGain)})
		if farmUpdate.Error != nil {
			return farmUpdate.Error
		}
		if farmUpdate.RowsAffected != 1 {
			return fmt.Errorf("灵田状态已变化，请重新查看")
		}
		return nil
	}); err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "🌿 灵田升阶成功", Content: fmt.Sprintf("地脉灵泉贯入洞天，旧田垄向两侧铺开，新炼灵壤泛起青金灵辉。\n━━━━━━━━━━━\n田阶：%s → %s\n灵地：%d块 → %d块（永久新增2块）\n生长缩减：%d分钟 → %d分钟\n基础增产：+%d株 → +%d株\n洞天繁荣：+%d\n━━━━━━━━━━━\n本次消耗：仙府材料×%d、灵石×%d\n下一步可在新地块播种，或继续提升仙府为下一次灵田升阶做准备。", farmGrade(currentLevel), farmGrade(nextLevel), currentLevel*2, nextLevel*2, farmGrowthReduction(currentLevel), farmGrowthReduction(nextLevel), currentLevel/2, nextLevel/2, prosperityGain, materialCost, stoneCost), Actions: []string{"我的灵田", "种子商店", "一键种植", "升级府", "升级灵田", "灵田榜"}}, true, nil
}

func (g *Game) plantAllAvailable(player *model.Player, argument string) (GameResult, bool, error) {
	seed := strings.TrimSpace(argument)
	if seed == "" {
		return GameResult{Title: "一键播种", Content: "请输入：`一键种植 种子名`。系统会按空闲地块和持有种子数量依次播种。", Actions: []string{"种子商店", "灵田仓库"}}, true, nil
	}
	mansion, _, err := g.getOrCreateMansion(player)
	if err != nil {
		return GameResult{}, true, err
	}
	var occupied int64
	_ = g.store.DB.Model(&model.MansionCrop{}).Where("mansion_id = ? AND harvested = ?", mansion.ID, false).Count(&occupied).Error
	available := maxInt(mansion.FarmLevel*2-int(occupied), 0)
	if available == 0 {
		return GameResult{Title: "灵田已满", Content: "没有空闲地块，请先收取成熟灵植。", Actions: []string{"收菜", "灵田"}}, true, nil
	}
	planted := 0
	for planted < available {
		result, _, plantErr := g.plantCrop(player, seed)
		if plantErr != nil {
			return GameResult{}, true, plantErr
		}
		if result.Title != "灵田播种" {
			break
		}
		planted++
	}
	if planted == 0 {
		return GameResult{Title: "一键播种失败", Content: "种子不足或种子配置无效。", Actions: []string{"种子商店", "灵田仓库"}}, true, nil
	}
	return GameResult{Title: "一键播种完成", Content: fmt.Sprintf("灵种：%s\n成功播种：%d块地\n剩余空地：%d块\n可发送 `灵田` 查看每块地的成熟时间。", seed, planted, available-planted), Actions: []string{"灵田", "浇水", "种子商店"}}, true, nil
}

type fertilizerEffect struct {
	AdvanceMinutes     int   `json:"advance_minutes"`
	AdvancePercent     int   `json:"advance_percent"`
	YieldBonus         int64 `json:"yield_bonus"`
	DisasterResistance int   `json:"disaster_resistance"`
	MinimumFarmLevel   int   `json:"minimum_farm_level"`
}

var (
	errFarmCropChanged       = errors.New("灵田作物状态已经变化")
	errFarmFertilizerMissing = errors.New("灵肥数量不足")
)

func decodeFertilizer(item model.Item) (fertilizerEffect, error) {
	var effect fertilizerEffect
	if item.EffectFunc != "fertilize_crop" {
		return effect, fmt.Errorf("%s不是可施用灵肥", item.Name)
	}
	if err := json.Unmarshal([]byte(item.EffectParams), &effect); err != nil {
		return effect, fmt.Errorf("%s的灵肥道纹无法解析: %w", item.Name, err)
	}
	if effect.AdvanceMinutes < 0 || effect.AdvancePercent < 0 || effect.AdvancePercent > 95 || effect.YieldBonus < 0 || effect.DisasterResistance < 0 {
		return effect, fmt.Errorf("%s的灵肥效果数值无效", item.Name)
	}
	if effect.MinimumFarmLevel < 1 {
		effect.MinimumFarmLevel = 1
	}
	if effect.DisasterResistance > 100 {
		effect.DisasterResistance = 100
	}
	return effect, nil
}

func fertilizerAdvance(effect fertilizerEffect, readyAt, now time.Time) time.Duration {
	remaining := readyAt.Sub(now)
	if remaining <= 0 {
		return 0
	}
	advance := time.Duration(effect.AdvanceMinutes) * time.Minute
	if effect.AdvancePercent > 0 {
		advance += remaining * time.Duration(effect.AdvancePercent) / 100
	}
	if advance > remaining {
		return remaining
	}
	return advance
}

func fertilizerEffectText(effect fertilizerEffect) string {
	advance := []string{}
	if effect.AdvanceMinutes > 0 {
		advance = append(advance, fmt.Sprintf("固定提前%d分钟", effect.AdvanceMinutes))
	}
	if effect.AdvancePercent > 0 {
		advance = append(advance, fmt.Sprintf("剩余生长时间缩短%d%%", effect.AdvancePercent))
	}
	if len(advance) == 0 {
		advance = append(advance, "不改变成熟时间")
	}
	return fmt.Sprintf("%s · 预计增产+%d株 · 抗灾+%d · 需灵田%d阶", strings.Join(advance, "、"), effect.YieldBonus, effect.DisasterResistance, effect.MinimumFarmLevel)
}

func parseFertilizePlot(raw string) (int, string, error) {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return 0, "", errors.New("缺少地块号")
	}
	if plot, err := strconv.Atoi(fields[0]); err == nil && plot > 0 {
		name := strings.TrimSpace(strings.Join(fields[1:], " "))
		if name == "" {
			name = "灵壤肥"
		}
		return plot, name, nil
	}
	if len(fields) > 1 {
		if plot, err := strconv.Atoi(fields[len(fields)-1]); err == nil && plot > 0 {
			return plot, strings.Join(fields[:len(fields)-1], " "), nil
		}
	}
	return 0, "", errors.New("地块号必须是正整数")
}

func (g *Game) fertilizeFarmPlot(player *model.Player, raw string) (GameResult, bool, error) {
	plot, fertilizerName, parseErr := parseFertilizePlot(raw)
	if parseErr != nil {
		return GameResult{Title: "🌿 灵田施肥", Content: "请输入：`施肥 地块 [灵肥名]`，例如 `施肥 1 灵壤肥`。省略灵肥名时默认使用灵壤肥。", Actions: []string{"灵肥图鉴", "灵田", "货铺"}}, true, nil
	}
	mansion, _, err := g.getOrCreateMansion(player)
	if err != nil {
		return GameResult{}, true, err
	}
	var crop model.MansionCrop
	if err := g.store.DB.Where("mansion_id = ? AND plot = ? AND harvested = ?", mansion.ID, plot, false).First(&crop).Error; err != nil {
		return GameResult{Title: "🌿 地块不可施肥", Content: fmt.Sprintf("第%d号地块没有生长中的灵植。请先播种，再在成熟前施入灵肥。", plot), Actions: []string{"种植", "一键种植", "灵田"}}, true, nil
	}
	if crop.Fertilized {
		return GameResult{Title: "🌿 本轮已经施肥", Content: fmt.Sprintf("第%d号地块已经施过%s。每块田垄每轮生长只能承受一次灵肥，收获并重新播种后才可再次施肥。\n本次没有扣除任何物品。", plot, displayOr(crop.FertilizerName, "灵肥")), Actions: []string{"土地详情 " + strconv.Itoa(plot), "灵田"}}, true, nil
	}
	now := time.Now()
	if !crop.ReadyAt.After(now) {
		return GameResult{Title: "🌿 灵植已经成熟", Content: fmt.Sprintf("第%d号地块已经成熟，现时施肥无法再吸收药力。请先收菜；本次没有扣除任何物品。", plot), Actions: []string{"收菜 " + strconv.Itoa(plot), "灵田"}}, true, nil
	}
	fertilizer, err := g.itemByName(fertilizerName)
	if err != nil {
		return GameResult{Title: "🌿 灵肥未收录", Content: "没有找到“" + fertilizerName + "”，请从灵肥图鉴蓝字中选择。", Actions: []string{"灵肥图鉴", "货铺"}}, true, nil
	}
	effect, err := decodeFertilizer(fertilizer)
	if err != nil {
		return GameResult{Title: "🌿 不可施用", Content: err.Error() + "。本次没有扣除物品。", Actions: []string{"灵肥图鉴", "物品 " + fertilizer.Name}}, true, nil
	}
	if mansion.FarmLevel < effect.MinimumFarmLevel {
		return GameResult{Title: "🌿 田阶无法承载", Content: fmt.Sprintf("%s需要灵田%d阶，当前仅%d阶。强行施用会冲坏地脉，本次没有扣除灵肥。", fertilizer.Name, effect.MinimumFarmLevel, mansion.FarmLevel), Actions: []string{"升级灵田", "灵肥图鉴", "灵田"}}, true, nil
	}
	if g.itemQuantity(player.ID, fertilizer.ID) < 1 {
		return GameResult{Title: "🌿 灵肥不足", Content: fmt.Sprintf("乾坤袋中没有%s。可在仙门货铺购入，或按配方自行合成。", fertilizer.Name), Actions: []string{"购入 " + fertilizer.Name, "配方 " + fertilizer.Name, "合成 " + fertilizer.Name, "灵肥图鉴"}}, true, nil
	}
	advance := fertilizerAdvance(effect, crop.ReadyAt, now)
	readyAt := crop.ReadyAt.Add(-advance)
	if readyAt.Before(now) {
		readyAt = now
	}
	resistance := minInt(crop.DisasterResistance+effect.DisasterResistance, 100)
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		update := tx.Model(&model.MansionCrop{}).Where("id = ? AND harvested = ? AND (fertilized = ? OR fertilized IS NULL) AND ready_at > ?", crop.ID, false, false, now).Updates(map[string]any{
			"fertilized": true, "fertilizer_name": fertilizer.Name, "disaster_resistance": resistance,
			"ready_at": readyAt, "quantity": gorm.Expr("quantity + ?", effect.YieldBonus),
		})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return errFarmCropChanged
		}
		consume := tx.Model(&model.PlayerItem{}).Where("player_id = ? AND item_id = ? AND quantity >= ?", player.ID, fertilizer.ID, 1).Update("quantity", gorm.Expr("quantity - 1"))
		if consume.Error != nil {
			return consume.Error
		}
		if consume.RowsAffected != 1 {
			return errFarmFertilizerMissing
		}
		return nil
	})
	if errors.Is(err, errFarmCropChanged) {
		return GameResult{Title: "🌿 灵田状态已变化", Content: "该田垄可能刚被收获或施肥，请重新查看。事务已回滚，没有扣除灵肥。", Actions: []string{"灵田", "土地详情 " + strconv.Itoa(plot)}}, true, nil
	}
	if errors.Is(err, errFarmFertilizerMissing) {
		return GameResult{Title: "🌿 灵肥不足", Content: "施肥结算时库存发生变化，事务已回滚，没有改变作物。", Actions: []string{"背包", "灵肥图鉴"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	_, _ = g.addPlayerValueInt(player.ID, "farm.fertilized", 1)
	remainingText := "灵植已经催熟，可立即收取。"
	if readyAt.After(now) {
		remainingText = "距离成熟还需" + formatDuration(time.Until(readyAt)) + "。"
	}
	return GameResult{Title: "🌿 灵肥归壤", Content: fmt.Sprintf("田垄：第%d垄\n施用：%s × 1\n成熟提前：%s\n预计增产：+%d株（%d → %d）\n抗灾道韵：%d\n━━━━━━━━━━━\n%s\n每块田垄每轮只可施肥一次。", plot, fertilizer.Name, formatDuration(advance), effect.YieldBonus, crop.Quantity, crop.Quantity+effect.YieldBonus, resistance, remainingText), Actions: []string{"土地详情 " + strconv.Itoa(plot), "收菜 " + strconv.Itoa(plot), "灵田", "灵肥图鉴"}}, true, nil
}

func (g *Game) fertilizeAllFarmPlots(player *model.Player, raw string) (GameResult, bool, error) {
	fertilizerName := strings.TrimSpace(raw)
	if fertilizerName == "" {
		fertilizerName = "灵壤肥"
	}
	mansion, _, err := g.getOrCreateMansion(player)
	if err != nil {
		return GameResult{}, true, err
	}
	fertilizer, err := g.itemByName(fertilizerName)
	if err != nil {
		return GameResult{Title: "🌿 灵肥未收录", Content: "没有找到“" + fertilizerName + "”，请从灵肥图鉴选择。", Actions: []string{"灵肥图鉴", "货铺"}}, true, nil
	}
	effect, err := decodeFertilizer(fertilizer)
	if err != nil {
		return GameResult{Title: "🌿 不可施用", Content: err.Error(), Actions: []string{"灵肥图鉴"}}, true, nil
	}
	if mansion.FarmLevel < effect.MinimumFarmLevel {
		return GameResult{Title: "🌿 田阶无法承载", Content: fmt.Sprintf("%s需要灵田%d阶，当前%d阶。本次没有扣除灵肥。", fertilizer.Name, effect.MinimumFarmLevel, mansion.FarmLevel), Actions: []string{"升级灵田", "灵肥图鉴"}}, true, nil
	}
	held := g.itemQuantity(player.ID, fertilizer.ID)
	if held < 1 {
		return GameResult{Title: "🌿 灵肥不足", Content: fmt.Sprintf("乾坤袋中没有%s。", fertilizer.Name), Actions: []string{"购入 " + fertilizer.Name, "合成 " + fertilizer.Name, "灵肥图鉴"}}, true, nil
	}
	now := time.Now()
	var crops []model.MansionCrop
	if err := g.store.DB.Where("mansion_id = ? AND harvested = ? AND (fertilized = ? OR fertilized IS NULL) AND ready_at > ?", mansion.ID, false, false, now).Order("plot").Find(&crops).Error; err != nil {
		return GameResult{}, true, err
	}
	if len(crops) == 0 {
		return GameResult{Title: "🌿 无需一键施肥", Content: "当前没有尚未施肥且仍在生长的田垄。本次没有扣除任何物品。", Actions: []string{"灵田", "种植", "土地详情"}}, true, nil
	}
	limit := len(crops)
	if held < int64(limit) {
		limit = int(held)
	}
	applied := int64(0)
	totalYield := int64(0)
	totalAdvance := time.Duration(0)
	plots := make([]string, 0, limit)
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		for index := 0; index < limit; index++ {
			crop := crops[index]
			advance := fertilizerAdvance(effect, crop.ReadyAt, now)
			readyAt := crop.ReadyAt.Add(-advance)
			if readyAt.Before(now) {
				readyAt = now
			}
			resistance := minInt(crop.DisasterResistance+effect.DisasterResistance, 100)
			update := tx.Model(&model.MansionCrop{}).Where("id = ? AND harvested = ? AND (fertilized = ? OR fertilized IS NULL) AND ready_at > ?", crop.ID, false, false, now).Updates(map[string]any{
				"fertilized": true, "fertilizer_name": fertilizer.Name, "disaster_resistance": resistance,
				"ready_at": readyAt, "quantity": gorm.Expr("quantity + ?", effect.YieldBonus),
			})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected == 0 {
				continue
			}
			applied++
			totalYield += effect.YieldBonus
			totalAdvance += advance
			plots = append(plots, strconv.Itoa(crop.Plot))
		}
		if applied == 0 {
			return errFarmCropChanged
		}
		consume := tx.Model(&model.PlayerItem{}).Where("player_id = ? AND item_id = ? AND quantity >= ?", player.ID, fertilizer.ID, applied).Update("quantity", gorm.Expr("quantity - ?", applied))
		if consume.Error != nil {
			return consume.Error
		}
		if consume.RowsAffected != 1 {
			return errFarmFertilizerMissing
		}
		return nil
	})
	if errors.Is(err, errFarmCropChanged) || errors.Is(err, errFarmFertilizerMissing) {
		return GameResult{Title: "🌿 一键施肥未完成", Content: "田垄或灵肥库存在结算时发生变化，全部事务已回滚，请重新查看后再试。", Actions: []string{"灵田", "背包", "灵肥图鉴"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	_, _ = g.addPlayerValueInt(player.ID, "farm.fertilized", applied)
	shortage := ""
	if applied < int64(len(crops)) {
		shortage = fmt.Sprintf("\n尚有%d块可施肥田垄，当前灵肥已经用尽。", int64(len(crops))-applied)
	}
	return GameResult{Title: "🌿 一键施肥完成", Content: fmt.Sprintf("施用：%s × %d\n田垄：第%s垄\n累计催生：%s\n累计增产：+%d株\n每垄抗灾：+%d\n剩余灵肥：%d%s", fertilizer.Name, applied, strings.Join(plots, "、第"), formatDuration(totalAdvance), totalYield, effect.DisasterResistance, held-applied, shortage), Actions: []string{"灵田", "土地详情", "灵肥图鉴", "购入 " + fertilizer.Name}}, true, nil
}

func (g *Game) fertilizerCatalog(player *model.Player, raw string) (GameResult, bool, error) {
	const pageSize = 2
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(raw), 1)), 1)
	query := g.store.DB.Model(&model.Item{}).Where("effect_func = ?", "fertilize_crop")
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt((int(total)+pageSize-1)/pageSize, 1)
	if page > pages {
		page = pages
	}
	var items []model.Item
	if err := query.Order("base_value,id").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return GameResult{}, true, err
	}
	lines := []string{"灵肥用于洞天灵田，不是凡俗化肥。每块田垄每轮只可施用一种，成熟或已经施肥的作物不会消耗灵肥。", "━━━━━━━━━━━"}
	actions := []string{}
	for _, item := range items {
		effect, decodeErr := decodeFertilizer(item)
		if decodeErr != nil {
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s【%s】· 持有%d\n  %s\n  %s\n  来源：仙门货铺常设不限购，亦可按配方合成。", item.Name, item.RarityName, g.itemQuantity(player.ID, item.ID), fertilizerEffectText(effect), item.Description))
		actions = append(actions, "购入 "+item.Name, "配方 "+item.Name, "施肥 1 "+item.Name)
	}
	if len(items) == 0 {
		lines = append(lines, "当前没有已经启用的灵肥数据。")
	}
	lines = append(lines, "━━━━━━━━━━━", fmt.Sprintf("第%d/%d页 · 共%d种灵肥", page, pages, total), "用法：`施肥 地块 [灵肥名]`；批量使用：`一键施肥 [灵肥名]`。")
	if page > 1 {
		actions = append(actions, fmt.Sprintf("灵肥图鉴 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("灵肥图鉴 %d", page+1))
	}
	actions = append(actions, "一键施肥 灵壤肥", "灵田", "货铺", "合成列表")
	return GameResult{Title: "🌿 灵肥图鉴", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) tendFarmPlot(player *model.Player, action, argument string) (GameResult, bool, error) {
	plot := int(parsePositiveInt(strings.TrimSpace(argument), 0))
	if plot <= 0 {
		return GameResult{Title: action + "灵田", Content: fmt.Sprintf("请输入：`%s 地块号`，例如 `%s 1`。", action, action), Actions: []string{"灵田", "土地详情"}}, true, nil
	}
	mansion, _, err := g.getOrCreateMansion(player)
	if err != nil {
		return GameResult{}, true, err
	}
	var crop model.MansionCrop
	if err := g.store.DB.Where("mansion_id = ? AND plot = ? AND harvested = ?", mansion.ID, plot, false).First(&crop).Error; err != nil {
		return GameResult{Title: "地块空闲", Content: fmt.Sprintf("第%d号地块没有生长中的灵植。", plot), Actions: []string{"种植", "灵田"}}, true, nil
	}
	field := map[string]string{"浇水": "watered", "除草": "weeded", "除虫": "pest_free"}[action]
	if field == "" {
		return GameResult{}, true, fmt.Errorf("未知灵田照料操作：%s", action)
	}
	already := map[string]bool{"浇水": crop.Watered, "除草": crop.Weeded, "除虫": crop.PestFree}[action]
	if already {
		return GameResult{Title: "无需重复" + action, Content: fmt.Sprintf("第%d号地块本轮已经完成%s。", plot, action), Actions: []string{"灵田"}}, true, nil
	}
	remaining, staminaErr := g.useStamina(player.ID, 1)
	if staminaErr != nil {
		return GameResult{Title: "体力不足", Content: staminaErr.Error(), Actions: []string{"状态", "灵田"}}, true, nil
	}
	advance := 5 * time.Minute
	if action == "浇水" {
		advance = 10 * time.Minute
	}
	ready := crop.ReadyAt.Add(-advance)
	if ready.Before(time.Now()) {
		ready = time.Now()
	}
	updates := map[string]any{field: true, "ready_at": ready}
	if action == "除草" || action == "除虫" {
		updates["quantity"] = gorm.Expr("quantity + 1")
	}
	if err := g.store.DB.Model(&crop).Updates(updates).Error; err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "灵田" + action, Content: fmt.Sprintf("地块：%d\n操作：%s完成\n成熟提前：%s\n%s\n剩余体力：%d", plot, action, formatDuration(advance), map[bool]string{true: "灵植已经成熟。", false: "距离成熟还需" + formatDuration(time.Until(ready))}[!ready.After(time.Now())], remaining), Actions: []string{"灵田", "收菜 " + strconv.Itoa(plot)}}, true, nil
}

var errFarmBatchStaminaChanged = errors.New("灵田批量照料时体力发生变化")

func (g *Game) tendAllFarmPlots(player *model.Player, action string) (GameResult, bool, error) {
	field := map[string]string{"一键除草": "weeded", "一键除虫": "pest_free"}[action]
	if field == "" {
		return GameResult{}, true, fmt.Errorf("未知灵田批量照料操作：%s", action)
	}
	mansion, _, err := g.getOrCreateMansion(player)
	if err != nil {
		return GameResult{}, true, err
	}
	var targetCount int64
	if err := g.store.DB.Model(&model.MansionCrop{}).Where("mansion_id = ? AND harvested = ? AND ("+field+" = ? OR "+field+" IS NULL)", mansion.ID, false, false).Count(&targetCount).Error; err != nil {
		return GameResult{}, true, err
	}
	careName := strings.TrimPrefix(action, "一键")
	if targetCount == 0 {
		return GameResult{Title: "🌿 无需" + action, Content: "当前没有需要" + careName + "的生长中田垄，本次没有扣除体力，也没有改变成熟时间或产量。", Actions: []string{"我的灵田", "一键除草", "一键除虫"}}, true, nil
	}
	currentStamina, err := g.currentStamina(player.ID)
	if err != nil {
		return GameResult{}, true, err
	}
	if currentStamina < targetCount {
		maximum, _ := g.staminaMaximum(player.ID)
		return GameResult{Title: "🌿 批量" + careName + "体力不足", Content: fmt.Sprintf("需要处理%d块田垄，每块消耗1点体力。\n需要体力：%d\n当前体力：%d/%d\n━━━━━━━━━━━\n本次没有处理任何田垄，也没有扣除体力。", targetCount, targetCount, currentStamina, maximum), Actions: []string{"体力", "我的灵田", careName + " 1"}}, true, nil
	}
	processed := int64(0)
	remainingStamina := currentStamina
	now := time.Now()
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		var crops []model.MansionCrop
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("mansion_id = ? AND harvested = ? AND ("+field+" = ? OR "+field+" IS NULL)", mansion.ID, false, false).Order("plot").Find(&crops).Error; err != nil {
			return err
		}
		if len(crops) == 0 {
			return nil
		}
		var staminaRow model.PlayerValue
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("player_id = ? AND key = ?", player.ID, "stamina.value").First(&staminaRow).Error; err != nil {
			return err
		}
		lockedStamina, parseErr := strconv.ParseInt(strings.TrimSpace(staminaRow.Value), 10, 64)
		if parseErr != nil || lockedStamina < int64(len(crops)) {
			return errFarmBatchStaminaChanged
		}
		for _, crop := range crops {
			readyAt := crop.ReadyAt.Add(-5 * time.Minute)
			if readyAt.Before(now) {
				readyAt = now
			}
			updated := tx.Model(&model.MansionCrop{}).Where("id = ? AND harvested = ? AND ("+field+" = ? OR "+field+" IS NULL)", crop.ID, false, false).Updates(map[string]any{
				field: true, "ready_at": readyAt, "quantity": gorm.Expr("quantity + 1"),
			})
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return errFarmCropChanged
			}
		}
		processed = int64(len(crops))
		remainingStamina = lockedStamina - processed
		return upsertPlayerValueTx(tx, player.ID, "stamina.value", strconv.FormatInt(remainingStamina, 10), nil)
	})
	if errors.Is(err, errFarmBatchStaminaChanged) {
		return GameResult{Title: "🌿 批量" + careName + "未执行", Content: "结算时体力发生变化，整次操作已经回滚；田垄、体力、成熟时间和产量均未改变，请重新发送。", Actions: []string{"体力", "我的灵田", action}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	if processed == 0 {
		return GameResult{Title: "🌿 无需" + action, Content: "田垄状态刚刚发生变化，当前已经没有需要处理的目标。本次没有扣除体力。", Actions: []string{"我的灵田"}}, true, nil
	}
	return GameResult{Title: "🌿 " + action + "完成", Content: fmt.Sprintf("完成%s：%d块田垄\n成熟时间：每块提前5分钟\n预计增产：每块+1株，共+%d株\n体力消耗：%d\n剩余体力：%d\n━━━━━━━━━━━\n所有田垄状态与体力已经在同一事务结算。", careName, processed, processed, processed, remainingStamina), Actions: []string{"我的灵田", "一键除草", "一键除虫", "一键施肥 灵壤肥", "收菜"}}, true, nil
}

func (g *Game) harvestFarmPlot(player *model.Player, argument string) (GameResult, bool, error) {
	argument = strings.TrimSpace(argument)
	if argument == "" {
		return g.harvestCrops(player)
	}
	plot := int(parsePositiveInt(argument, 0))
	mansion, _, err := g.getOrCreateMansion(player)
	if err != nil {
		return GameResult{}, true, err
	}
	var crop model.MansionCrop
	if err := g.store.DB.Where("mansion_id = ? AND plot = ? AND harvested = ?", mansion.ID, plot, false).First(&crop).Error; err != nil {
		return GameResult{Title: "无物可收", Content: fmt.Sprintf("第%d号地块没有可收取灵植。", plot), Actions: []string{"灵田"}}, true, nil
	}
	if crop.ReadyAt.After(time.Now()) {
		return GameResult{Title: "灵植未熟", Content: fmt.Sprintf("第%d号地块还需%s成熟。浇水、除草和除虫可以缩短时间并增加产量。", plot, formatDuration(time.Until(crop.ReadyAt))), Actions: []string{"浇水 " + argument, "除草 " + argument, "除虫 " + argument, "灵田"}}, true, nil
	}
	var item model.Item
	if err := g.store.DB.First(&item, crop.ItemID).Error; err != nil {
		return GameResult{}, true, err
	}
	if err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.PlayerItem{}).Where("player_id = ? AND item_id = ?", player.ID, item.ID).FirstOrCreate(&model.PlayerItem{PlayerID: player.ID, ItemID: item.ID}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.PlayerItem{}).Where("player_id = ? AND item_id = ?", player.ID, item.ID).Update("quantity", gorm.Expr("quantity + ?", crop.Quantity)).Error; err != nil {
			return err
		}
		if err := tx.Model(&crop).Update("harvested", true).Error; err != nil {
			return err
		}
		return tx.Model(&mansion).Update("prosperity", gorm.Expr("prosperity + ?", crop.Quantity)).Error
	}); err != nil {
		return GameResult{}, true, err
	}
	_, _ = g.addPlayerValueInt(player.ID, "farm.harvested", crop.Quantity)
	return GameResult{Title: "灵植丰收", Content: fmt.Sprintf("地块：%d\n收获：%s × %d\n被采撷：%d株\n繁荣度：+%d\n灵植已经收入灵田仓库。", plot, item.Name, crop.Quantity, crop.Stolen, crop.Quantity), Actions: []string{"灵田仓库", "出售灵植 " + item.Name, "种子商店", "灵田"}}, true, nil
}

func (g *Game) farmWarehouse(player *model.Player, argument string) (GameResult, bool, error) {
	const pageSize = 8
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(argument), 1)), 1)
	query := g.store.DB.Table("player_items").Select("items.*, player_items.quantity AS owned_quantity").Joins("JOIN items ON items.id = player_items.item_id").Where("player_items.player_id = ? AND player_items.quantity > 0 AND (items.category_name IN ? OR items.effect_func = ?)", player.ID, []string{"灵草", "种子", "灵肥"}, "plant_seed")
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt(int((total+pageSize-1)/pageSize), 1)
	if page > pages {
		page = pages
	}
	type row struct {
		model.Item
		OwnedQuantity int64
	}
	var rows []row
	if err := query.Order("items.category_name,items.id").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	if len(rows) == 0 {
		return GameResult{Title: "灵田仓库", Content: "仓库中没有种子或灵植。", Actions: []string{"种子商店", "灵田"}}, true, nil
	}
	lines := []string{"种子用于播种，成熟灵植可炼丹、交易或出售；灵肥用于催生、增产并抵御灵田灾异。", "━━━━━━━━━━━"}
	actions := []string{}
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("- %s【%s】×%d · 估值%d灵石\n  %s", row.Name, row.CategoryName, row.OwnedQuantity, row.BaseValue, row.Description))
		if row.EffectFunc == "plant_seed" {
			actions = append(actions, "种植 "+row.Name)
		} else if row.EffectFunc == "fertilize_crop" {
			actions = append(actions, "施肥 1 "+row.Name)
		} else {
			actions = append(actions, "出售灵植 "+row.Name)
		}
	}
	lines = append(lines, "━━━━━━━━━━━", fmt.Sprintf("第%d/%d页 · 共%d类", page, pages, total))
	if page > 1 {
		actions = append(actions, fmt.Sprintf("灵田仓库 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("灵田仓库 %d", page+1))
	}
	return GameResult{Title: "灵田仓库", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) sellFarmProduce(player *model.Player, args []string) (GameResult, bool, error) {
	if len(args) == 0 {
		return GameResult{Title: "出售灵植", Content: "请输入：`出售灵植 灵植名 [数量]`。", Actions: []string{"灵田仓库"}}, true, nil
	}
	quantity := int64(1)
	nameParts := args
	if len(args) > 1 {
		if parsed, err := strconv.ParseInt(args[len(args)-1], 10, 64); err == nil && parsed > 0 {
			quantity = parsed
			nameParts = args[:len(args)-1]
		}
	}
	name := strings.Join(nameParts, " ")
	item, err := g.itemByName(name)
	if err != nil || item.CategoryName != "灵草" || g.itemQuantity(player.ID, item.ID) < quantity {
		return GameResult{Title: "出售失败", Content: "灵植不存在、数量不足，或该物品不是成熟灵植。", Actions: []string{"灵田仓库"}}, true, nil
	}
	price := max64(item.BaseValue/2, 1) * quantity
	if err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.PlayerItem{}).Where("player_id = ? AND item_id = ? AND quantity >= ?", player.ID, item.ID, quantity).Update("quantity", gorm.Expr("quantity - ?", quantity)).Error; err != nil {
			return err
		}
		return tx.Model(player).Update("spirit_stones", gorm.Expr("spirit_stones + ?", price)).Error
	}); err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "灵植售出", Content: fmt.Sprintf("售出：%s × %d\n单价：%d灵石\n获得：%d灵石\n当前灵石：%d", item.Name, quantity, max64(item.BaseValue/2, 1), price, player.SpiritStones+price), Actions: []string{"灵田仓库", "灵田", "货铺"}}, true, nil
}

func (g *Game) sellAllFarmProduce(player *model.Player) (GameResult, bool, error) {
	type row struct {
		ItemID    uint
		Name      string
		Quantity  int64
		BaseValue int64
	}
	var rows []row
	if err := g.store.DB.Table("player_items").Select("items.id AS item_id,items.name,player_items.quantity,items.base_value").Joins("JOIN items ON items.id = player_items.item_id").Where("player_items.player_id = ? AND player_items.quantity > 0 AND items.category_name = ?", player.ID, "灵草").Scan(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	if len(rows) == 0 {
		return GameResult{Title: "无灵植可售", Content: "灵田仓库中没有成熟灵植。", Actions: []string{"灵田", "种子商店"}}, true, nil
	}
	total := int64(0)
	count := int64(0)
	for _, row := range rows {
		total += max64(row.BaseValue/2, 1) * row.Quantity
		count += row.Quantity
	}
	if err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		for _, row := range rows {
			if err := tx.Model(&model.PlayerItem{}).Where("player_id = ? AND item_id = ?", player.ID, row.ItemID).Update("quantity", 0).Error; err != nil {
				return err
			}
		}
		return tx.Model(player).Update("spirit_stones", gorm.Expr("spirit_stones + ?", total)).Error
	}); err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "灵植清仓", Content: fmt.Sprintf("售出：%d类灵植 · 共%d株\n获得：%d灵石\n当前灵石：%d", len(rows), count, total, player.SpiritStones+total), Actions: []string{"灵田仓库", "种子商店", "货铺"}}, true, nil
}

func (g *Game) gatherFromFriendFarm(player *model.Player, argument string) (GameResult, bool, error) {
	target, err := g.findPlayer(argument)
	if err != nil || target.ID == player.ID {
		return GameResult{Title: "🌙 潜入采灵", Content: "请输入其他玩家的唯一道号，例如：`偷菜 青玄`。不需要输入账号或玩家ID。\n潜入只会取走一株成熟灵植，并严格保留最后一株。", Actions: []string{"灵田榜", "好友", "灵田说明"}}, true, nil
	}
	cooldownKey := fmt.Sprintf("cooldown.farm.gather.%d", target.ID)
	if value, valueErr := g.playerValue(player.ID, cooldownKey); valueErr == nil {
		if until, parseErr := time.Parse(time.RFC3339Nano, value); parseErr == nil && until.After(time.Now()) {
			return GameResult{Title: "🌙 灵田戒备未散", Content: fmt.Sprintf("%s的护府禁制仍记得你的气息。\n再次潜入还需：%s\n━━━━━━━━━━━\n可先经营自己的灵田、查看其他道友，或等待禁制消散。", target.DaoName, formatDuration(time.Until(until))), Actions: []string{"我的灵田", "灵田榜", "护田记录"}}, true, nil
		}
	}
	var mansion model.Mansion
	if err := g.store.DB.Where("player_id = ?", target.ID).First(&mansion).Error; err != nil {
		return GameResult{Title: "🌙 无田可访", Content: target.DaoName + "尚未开辟仙府灵田，本次没有触发潜入冷却。", Actions: []string{"灵田榜"}}, true, nil
	}
	var crop model.MansionCrop
	if err := g.store.DB.Where("mansion_id = ? AND harvested = ? AND ready_at <= ? AND quantity > 1", mansion.ID, false, time.Now()).Order("ready_at,plot").First(&crop).Error; err != nil {
		return GameResult{Title: "🌙 潜入无获", Content: target.DaoName + "的灵田没有可采灵植：作物可能尚未成熟，或只剩用于留种的最后一株。\n本次没有触发六小时冷却。", Actions: []string{"我的灵田", "灵田榜"}}, true, nil
	}
	until := time.Now().Add(6 * time.Hour)
	if err := g.setPlayerValue(player.ID, cooldownKey, until.Format(time.RFC3339Nano), &until); err != nil {
		return GameResult{}, true, err
	}
	guardChance := minInt(35+mansion.BeastRoomLevel*5+mansion.FormationLevel*4, 85)
	if crop.Protected && randomPercent() <= guardChance {
		_ = g.appendFarmLog(target.ID, fmt.Sprintf("%s试图采撷第%d号地块，被护田灵兽发现。", player.DaoName, crop.Plot))
		return GameResult{Title: "🐾 护田灵兽现身", Content: fmt.Sprintf("你刚踏过%s的洞天田界，护府阵纹便与护田灵兽同时示警。\n━━━━━━━━━━━\n目标田垄：第%d垄\n守护判定：%d%%\n结果：潜入失败，未取得灵植\n禁制记忆：六小时内不能再次进入该灵田", target.DaoName, crop.Plot, guardChance), Actions: []string{"我的灵田", "灵兽", "护田记录", "灵田榜"}}, true, nil
	}
	var item model.Item
	if err := g.store.DB.First(&item, crop.ItemID).Error; err != nil {
		return GameResult{}, true, err
	}
	if err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&crop).Updates(map[string]any{"quantity": gorm.Expr("quantity - 1"), "stolen": gorm.Expr("stolen + 1")}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.PlayerItem{}).Where("player_id = ? AND item_id = ?", player.ID, item.ID).FirstOrCreate(&model.PlayerItem{PlayerID: player.ID, ItemID: item.ID}).Error; err != nil {
			return err
		}
		return tx.Model(&model.PlayerItem{}).Where("player_id = ? AND item_id = ?", player.ID, item.ID).Update("quantity", gorm.Expr("quantity + 1")).Error
	}); err != nil {
		return GameResult{}, true, err
	}
	_, _ = g.addPlayerValueInt(player.ID, "farm.gathered", 1)
	_ = g.appendFarmLog(target.ID, fmt.Sprintf("%s从第%d号地块采走了%s一株。", player.DaoName, crop.Plot, item.Name))
	return GameResult{Title: "🌙 潜入采灵成功", Content: fmt.Sprintf("你以敛息诀避开田界阵纹，从%s的第%d号灵垄摘得一株%s。\n━━━━━━━━━━━\n所得：%s × 1\n守护判定：%d%% · 本次未被察觉\n留种规则：已为田主保留最后一株\n再次潜入：六小时后\n灵植已收入你的灵田仓库。", target.DaoName, crop.Plot, item.Name, item.Name, guardChance), Actions: []string{"灵田仓库", "物品 " + item.Name, "我的灵田", "灵田榜", "护田记录"}}, true, nil
}

func (g *Game) farmPlotDetails(player *model.Player, argument string) (GameResult, bool, error) {
	mansion, _, err := g.getOrCreateMansion(player)
	if err != nil {
		return GameResult{}, true, err
	}
	plot := int(parsePositiveInt(strings.TrimSpace(argument), 0))
	query := g.store.DB.Where("mansion_id = ? AND harvested = ?", mansion.ID, false)
	if plot > 0 {
		query = query.Where("plot = ?", plot)
	}
	var crops []model.MansionCrop
	if err := query.Order("plot").Find(&crops).Error; err != nil {
		return GameResult{}, true, err
	}
	if len(crops) == 0 {
		return GameResult{Title: "土地详情", Content: "当前查询范围内没有生长中的灵植。", Actions: []string{"种子商店", "种植", "灵田"}}, true, nil
	}
	lines := []string{}
	actions := []string{}
	for _, crop := range crops {
		var item model.Item
		_ = g.store.DB.First(&item, crop.ItemID).Error
		fertilizer := "未施肥"
		if crop.Fertilized {
			fertilizer = displayOr(crop.FertilizerName, "已施灵肥") + fmt.Sprintf(" · 抗灾%d", crop.DisasterResistance)
		}
		lines = append(lines, fmt.Sprintf("地块%d · %s\n种子：%s · 预计%d株\n播种：%s\n成熟：%s\n照料：浇水%t · 除草%t · 除虫%t · 守护%t\n灵肥：%s", crop.Plot, item.Name, crop.SeedName, crop.Quantity, crop.PlantedAt.Format("01-02 15:04"), crop.ReadyAt.Format("01-02 15:04"), crop.Watered, crop.Weeded, crop.PestFree, crop.Protected, fertilizer))
		actions = append(actions, "浇水 "+strconv.Itoa(crop.Plot), "除草 "+strconv.Itoa(crop.Plot), "除虫 "+strconv.Itoa(crop.Plot), "施肥 "+strconv.Itoa(crop.Plot), "收菜 "+strconv.Itoa(crop.Plot))
	}
	return GameResult{Title: "土地详情", Content: strings.Join(lines, "\n━━━━━━━━━━━\n"), Actions: actions}, true, nil
}

func (g *Game) farmGuide(player *model.Player) GameResult {
	return GameResult{Title: "🌿 仙府灵田说明", Content: "灵田并非独立农场，而是仙府洞天中的修行产业。\n━━━━━━━━━━━\n一、发送 `灵田` 开辟并查看田契；地块过多时发送 `灵田 页码` 翻页。\n二、从 `种子商店` 购入灵种，发送 `种植 种子名 地块号`，或 `一键种植 种子名`。\n三、浇水、除草、除虫会引灵泉、清浊煞、驱噬灵虫，缩短成熟时间并保住产量；发送 `一键除草`、`一键除虫` 可处理全部对应田垄，每块消耗1点体力。\n四、灵肥是凝炼灵草、地脉与造化生机所得，并非凡俗化肥。发送 `灵肥图鉴` 查看田阶、催生、增产、抗灾与来源；发送 `施肥 地块 灵肥名` 或 `一键施肥 灵肥名` 使用。每块田垄每轮只能施肥一次。\n五、成熟后发送 `收菜 地块号`；灵植进入灵田仓库，可用于炼丹、合成、交易或出售。\n六、发送 `升级灵田` 重炼灵壤。每升一阶永久增加两块土地，并缩短生长时间、提高基础产量。\n七、灵田升阶受仙府等级、修士境界、仙府材料和灵石共同限制，不能无条件连升。\n八、发送 `偷菜 道号` 可潜入道友灵田；每个目标六小时一次，只取一株且保留最后一株。没有成熟灵植时不触发冷却。\n九、护田灵兽、兽室和护府阵法共同提高拦截率，拦截与失窃都会写入护田记录。\n十、木系灵根与灵植最为契合，可额外提高收成。所有玩家操作均使用唯一道号，不需要输入账号ID。", Actions: []string{"我的灵田", "一键除草", "一键除虫", "灵肥图鉴", "一键施肥 灵壤肥", "升级灵田", "种子商店", "灵田仓库", "偷菜", "护田记录", "灵田榜"}}
}

func (g *Game) farmGuardLog(player *model.Player) (GameResult, bool, error) {
	log, err := g.playerValue(player.ID, "farm.guard.log")
	if err != nil || strings.TrimSpace(log) == "" {
		return GameResult{Title: "护田记录", Content: "暂无他人采撷或护田灵兽拦截记录。", Actions: []string{"灵田", "灵兽"}}, true, nil
	}
	return GameResult{Title: "护田记录", Content: log, Actions: []string{"灵田", "灵兽", "灵田榜"}}, true, nil
}

func (g *Game) appendFarmLog(playerID uint, line string) error {
	old, _ := g.playerValue(playerID, "farm.guard.log")
	lines := nonEmptyLines(strings.Split(old, "\n"))
	lines = append(lines, time.Now().Format("01-02 15:04")+" · "+line)
	if len(lines) > 10 {
		lines = lines[len(lines)-10:]
	}
	return g.setPlayerValue(playerID, "farm.guard.log", strings.Join(lines, "\n"), nil)
}

func (g *Game) farmRanking(player *model.Player) (GameResult, bool, error) {
	type rank struct {
		DaoName    string
		FarmLevel  int
		Prosperity int64
	}
	var rows []rank
	if err := g.store.DB.Table("mansions").Select("players.dao_name,mansions.farm_level,mansions.prosperity").Joins("JOIN players ON players.id = mansions.player_id").Order("mansions.prosperity DESC,mansions.farm_level DESC").Limit(10).Scan(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	if len(rows) == 0 {
		return GameResult{Title: "灵田排行", Content: "尚无道友开启灵田。", Actions: []string{"灵田"}}, true, nil
	}
	lines := []string{"按仙府繁荣度与灵田等级排序。", "━━━━━━━━━━━"}
	actions := []string{}
	for index, row := range rows {
		lines = append(lines, fmt.Sprintf("%d. %s · 灵田%d级 · 繁荣%d", index+1, row.DaoName, row.FarmLevel, row.Prosperity))
		if row.DaoName != player.DaoName {
			actions = append(actions, "采撷 "+row.DaoName)
		}
	}
	return GameResult{Title: "灵田排行", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func chineseDigit(value int) string {
	digits := []string{"零", "一", "二", "三", "四", "五", "六", "七", "八", "九", "十", "十一", "十二", "十三", "十四", "十五", "十六", "十七", "十八", "十九", "二十"}
	if value >= 0 && value < len(digits) {
		return digits[value]
	}
	return strconv.Itoa(value)
}
