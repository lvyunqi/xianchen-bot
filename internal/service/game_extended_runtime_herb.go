package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/handler"
	"xianlv/internal/model"
	"xianlv/internal/storage"
)

func (g *Game) executeImmortalHerbExtended(player *model.Player, command handler.ParsedCommand, system extendedSystem, action string) (GameResult, bool, error) {
	if action == "atlas" {
		return g.extendedAtlasRuntime(player, command, system)
	}
	if action == "graft" {
		return g.graftImmortalHerbs(player, command, system)
	}
	name := strings.TrimSpace(command.RawArguments)
	if name == "" {
		return g.immortalHerbGarden(player, command, system)
	}
	config, err := g.extendedConfig(system.Table, name)
	if err != nil {
		return GameResult{Title: command.Spec.Name + "未找到", Content: "仙药图鉴中没有“" + name + "”，请点击药鉴中的完整名称。", Actions: []string{"药鉴", "仙药培育菜单"}}, true, nil
	}
	switch action {
	case "plant":
		return g.plantImmortalHerb(player, command, config)
	case "cultivate":
		return g.cultivateImmortalHerb(player, config)
	case "harvest":
		return g.harvestImmortalHerb(player, config)
	case "accelerate":
		return g.accelerateImmortalHerb(player, config)
	default:
		return GameResult{}, false, fmt.Errorf("未知仙药动作: %s", action)
	}
}

func (g *Game) immortalHerbGarden(player *model.Player, command handler.ParsedCommand, system extendedSystem) (GameResult, bool, error) {
	var rows []model.PlayerExtendedProgress
	if err := g.store.DB.Where("player_id = ? AND system = ?", player.ID, "仙药培育").Order("updated_at DESC,id DESC").Limit(12).Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	lines := []string{"个人仙药圃", "每株仙药都有独立成熟时间、培育次数与预计产量。", "━━━━━━━━━━━"}
	actions := []string{"药鉴", "仙府", "背包"}
	for _, row := range rows {
		state := row.State
		if row.ReadyAt != nil {
			if row.ReadyAt.After(time.Now()) {
				state = formatDuration(time.Until(*row.ReadyAt)) + "后成熟"
			} else if row.State != "已采摘" {
				state = "已经成熟"
			}
		}
		lines = append(lines, fmt.Sprintf("- %s【%s】\n  培育%d次 · 预计产量%d · 药力%d", row.ConfigName, displayOr(state, "培育中"), row.Uses, max64(row.Quantity, 1), row.Power))
		actions = append(actions, "育药 "+row.ConfigName, "催药 "+row.ConfigName, "采药 "+row.ConfigName)
	}
	if len(rows) == 0 {
		lines = append(lines, "药圃尚未种下仙药。先查看药鉴，选择满足前置的一株。")
		preview, previewActions, err := g.extendedAtlasPreview(player, "仙药培育", system, 4)
		if err != nil {
			return GameResult{}, true, err
		}
		lines = append(lines, preview...)
		for _, action := range previewActions {
			actions = append(actions, strings.Replace(action, "药鉴 ", "种药 ", 1))
		}
	}
	return GameResult{Title: command.Spec.Name, Content: strings.Join(lines, "\n"), Actions: uniqueExtendedActions(actions)}, true, nil
}

func (g *Game) plantImmortalHerb(player *model.Player, command handler.ParsedCommand, config model.GameplayConfigBase) (GameResult, bool, error) {
	var mansion model.Mansion
	if err := g.store.DB.Where("player_id = ?", player.ID).First(&mansion).Error; err != nil {
		return GameResult{Title: "尚无仙府药圃", Content: "仙药必须种在自己的仙府药圃中，请先建立仙府。", Actions: []string{"仙府", "药鉴"}}, true, nil
	}
	requirement, unmet, err := g.prerequisiteStatus(player, config.Prerequisite)
	if err != nil {
		return GameResult{Title: "仙药前置配置错误", Content: "本次没有扣除资源。"}, true, nil
	}
	if len(unmet) > 0 {
		return GameResult{Title: "仙药前置未满", Content: strings.Join(unmet, "\n"), Actions: append(g.prerequisiteActions(unmet), "药鉴")}, true, nil
	}
	var existing model.PlayerExtendedProgress
	if g.store.DB.Where("player_id = ? AND system = ? AND config_code = ?", player.ID, "仙药培育", config.Code).First(&existing).Error == nil && existing.State != "已采摘" {
		state := "已成熟"
		if existing.ReadyAt != nil && existing.ReadyAt.After(time.Now()) {
			state = formatDuration(time.Until(*existing.ReadyAt)) + "后成熟"
		}
		return GameResult{Title: "仙药已经在圃", Content: fmt.Sprintf("仙药：%s\n状态：%s\n预计产量：%d\n不会重复扣除种植材料。", config.Name, state, max64(existing.Quantity, 1)), Actions: []string{"育药 " + config.Name, "催药 " + config.Name, "采药 " + config.Name}}, true, nil
	}
	costText, missing, err := g.extendedCostStatus(player, config.CostMaterials)
	if err != nil || len(missing) > 0 {
		return GameResult{Title: "仙药种植材料不足", Content: fmt.Sprintf("需要：%s\n缺少：%s", costText, strings.Join(missing, "、")), Actions: []string{"背包", "货铺", "药鉴"}}, true, nil
	}
	effect := decodeExtendedEffect(config)
	growMinutes := minInt(maxInt(effect.Duration, 10), 180)
	readyAt := time.Now().Add(time.Duration(growMinutes) * time.Minute)
	yield := int64(1 + maxInt(config.Level, 1)/3)
	progress := model.PlayerExtendedProgress{PlayerID: player.ID, System: "仙药培育", ConfigCode: config.Code, ConfigName: config.Name, State: "培育中", Level: 1, Quantity: yield, Power: effect.Power, ReadyAt: &readyAt, MetadataJSON: fmt.Sprintf(`{"mansion_id":%d,"care":0}`, mansion.ID)}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		costs := make(map[string]int64)
		if err := json.Unmarshal([]byte(config.CostMaterials), &costs); err != nil {
			return err
		}
		if err := consumeExtendedCostTx(tx, player.ID, costs); err != nil {
			return err
		}
		return upsertExtendedProgressTx(tx, progress)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "仙药入圃", Content: fmt.Sprintf("仙药：%s\n药性：%s · 药力%d\n成熟时间：%s（约%d分钟）\n预计产量：%d\n实际消耗：%s\n前置：%s", config.Name, config.Type, effect.Power, readyAt.Format("01-02 15:04"), growMinutes, yield, costText, requirement), Actions: []string{"育药 " + config.Name, "催药 " + config.Name, "采药 " + config.Name, "药鉴"}}, true, nil
}

func (g *Game) immortalHerbProgress(playerID uint, code string) (model.PlayerExtendedProgress, error) {
	var progress model.PlayerExtendedProgress
	err := g.store.DB.Where("player_id = ? AND system = ? AND config_code = ?", playerID, "仙药培育", code).First(&progress).Error
	return progress, err
}

func (g *Game) cultivateImmortalHerb(player *model.Player, config model.GameplayConfigBase) (GameResult, bool, error) {
	progress, err := g.immortalHerbProgress(player.ID, config.Code)
	if err != nil || progress.State == "已采摘" || progress.ReadyAt == nil {
		return GameResult{Title: "仙药尚未种下", Content: "请先发送 `种药 " + config.Name + "`。", Actions: []string{"种药 " + config.Name, "药鉴"}}, true, nil
	}
	if !progress.ReadyAt.After(time.Now()) {
		return GameResult{Title: "仙药已经成熟", Content: "继续育药不会增加产量，也不会消耗灵茶。", Actions: []string{"采药 " + config.Name}}, true, nil
	}
	if progress.Uses >= 3 {
		return GameResult{Title: "本轮培育圆满", Content: "每株每轮最多精心培育三次，继续操作不会消耗资源。", Actions: []string{"催药 " + config.Name, "采药 " + config.Name}}, true, nil
	}
	item, itemErr := g.itemByName("灵茶")
	if itemErr != nil || g.itemQuantity(player.ID, item.ID) < 1 {
		return GameResult{Title: "育药资源不足", Content: "精心育药需要灵茶×1。", Actions: []string{"物品 灵茶", "货铺", "背包"}}, true, nil
	}
	remaining := time.Until(*progress.ReadyAt)
	newReady := time.Now().Add(remaining * 85 / 100)
	progress.ReadyAt = &newReady
	progress.Uses++
	progress.Quantity++
	progress.Mastery += int64(maxInt(config.Level, 1))
	progress.State = "精心培育"
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, item.ID, -1); err != nil {
			return err
		}
		return upsertExtendedProgressTx(tx, progress)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "仙药培育完成", Content: fmt.Sprintf("仙药：%s\n灵茶药雾：消耗1\n成熟缩短：%s\n预计产量：%d\n本轮培育：%d/3", config.Name, formatDuration(remaining-time.Until(newReady)), progress.Quantity, progress.Uses), Actions: []string{"育药 " + config.Name, "催药 " + config.Name, "采药 " + config.Name}}, true, nil
}

func (g *Game) accelerateImmortalHerb(player *model.Player, config model.GameplayConfigBase) (GameResult, bool, error) {
	progress, err := g.immortalHerbProgress(player.ID, config.Code)
	if err != nil || progress.State == "已采摘" || progress.ReadyAt == nil {
		return GameResult{Title: "没有可催化仙药", Content: "请先种下对应仙药。", Actions: []string{"种药 " + config.Name, "药鉴"}}, true, nil
	}
	if !progress.ReadyAt.After(time.Now()) {
		return GameResult{Title: "仙药已经成熟", Content: "催化不会消耗仙露。", Actions: []string{"采药 " + config.Name}}, true, nil
	}
	dew, itemErr := g.itemByName("仙露")
	if itemErr != nil || g.itemQuantity(player.ID, dew.ID) < 1 {
		return GameResult{Title: "催药资源不足", Content: "仙药催化需要仙露×1。", Actions: []string{"物品 仙露", "货铺", "背包"}}, true, nil
	}
	remaining := time.Until(*progress.ReadyAt)
	newReady := time.Now().Add(remaining * 65 / 100)
	progress.ReadyAt = &newReady
	progress.State = "仙露催化"
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, dew.ID, -1); err != nil {
			return err
		}
		return upsertExtendedProgressTx(tx, progress)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "仙药催化完成", Content: fmt.Sprintf("仙药：%s\n消耗：仙露×1\n剩余成熟时间缩短35%%：%s → %s", config.Name, formatDuration(remaining), formatDuration(time.Until(newReady))), Actions: []string{"育药 " + config.Name, "采药 " + config.Name}}, true, nil
}

func (g *Game) harvestImmortalHerb(player *model.Player, config model.GameplayConfigBase) (GameResult, bool, error) {
	progress, err := g.immortalHerbProgress(player.ID, config.Code)
	if err != nil || progress.State == "已采摘" || progress.ReadyAt == nil {
		return GameResult{Title: "没有可采仙药", Content: "请先种下对应仙药。", Actions: []string{"种药 " + config.Name, "药鉴"}}, true, nil
	}
	if progress.ReadyAt.After(time.Now()) {
		return GameResult{Title: "仙药尚未成熟", Content: fmt.Sprintf("%s还需%s成熟，提前采摘不会获得产物。", config.Name, formatDuration(time.Until(*progress.ReadyAt))), Actions: []string{"育药 " + config.Name, "催药 " + config.Name}}, true, nil
	}
	quantity := max64(progress.Quantity, 1)
	var item model.Item
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		var findErr error
		item, findErr = ensureImmortalHerbItemTx(tx, config)
		if findErr != nil {
			return findErr
		}
		if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, item.ID, quantity); err != nil {
			return err
		}
		progress.State, progress.ReadyAt, progress.Quantity = "已采摘", nil, 0
		progress.Experience += int64(maxInt(config.Level, 1) * 20)
		return upsertExtendedProgressTx(tx, progress)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "仙药采摘完成", Content: fmt.Sprintf("仙药：%s\n获得：%s×%d\n实际药效：每株修为+%d\n产物已经进入乾坤袋，本轮药圃记录已结算。", config.Name, item.Name, quantity, int64(item.EffectValue)), Actions: []string{"物品 " + item.Name, "使用 " + item.Name, "背包", "种药 " + config.Name}}, true, nil
}

func ensureImmortalHerbItemTx(tx *gorm.DB, config model.GameplayConfigBase) (model.Item, error) {
	var item model.Item
	if err := tx.Where("name = ? OR code = ?", config.Name, "extended_herb_"+config.Code).First(&item).Error; err == nil {
		return item, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return item, err
	}
	effect := decodeExtendedEffect(config)
	rarity := []string{"凡品", "灵品", "玄品", "地品", "天品", "仙品", "神品"}[minInt(maxInt(config.Level-1, 0)/2, 6)]
	item = model.Item{Code: "extended_herb_" + config.Code, Name: config.Name, CategoryName: "仙药", RarityName: rarity, Description: fmt.Sprintf("由仙府药圃培育的%s仙药，服下一株可增加%d修为。来源、成熟周期与培育记录均可追溯。", config.Type, max64(effect.Power/2, 1)), EffectType: "修为", EffectFunc: "add_cultivation", EffectValue: float64(max64(effect.Power/2, 1)), BaseValue: max64(effect.Power*3, 10), StackLimit: 9999, Stackable: true, Tradable: true}
	return item, tx.Create(&item).Error
}

func (g *Game) graftImmortalHerbs(player *model.Player, command handler.ParsedCommand, system extendedSystem) (GameResult, bool, error) {
	if len(command.Arguments) < 2 || command.Arguments[0] == command.Arguments[1] {
		return GameResult{Title: "仙药嫁接", Content: "请输入两株不同且已经培育过的仙药：`嫁药 仙药一 仙药二`。", Actions: []string{"育药", "药鉴"}}, true, nil
	}
	parents := make([]model.GameplayConfigBase, 0, 2)
	mastery := int64(0)
	for _, name := range command.Arguments[:2] {
		config, err := g.extendedConfig(system.Table, name)
		if err != nil {
			return GameResult{Title: "嫁接仙药未找到", Content: "图鉴中没有“" + name + "”。", Actions: []string{"药鉴"}}, true, nil
		}
		progress, err := g.immortalHerbProgress(player.ID, config.Code)
		if err != nil {
			return GameResult{Title: "嫁接前置不足", Content: "你尚未实际培育过“" + config.Name + "”。", Actions: []string{"种药 " + config.Name, "药鉴"}}, true, nil
		}
		parents, mastery = append(parents, config), mastery+progress.Mastery+progress.Uses
	}
	var total int64
	_ = g.store.DB.Table(system.Table).Where("status = ?", "启用").Count(&total).Error
	offset := int((int64(parents[0].ID) + int64(parents[1].ID) + mastery) % max64(total, 1))
	var child model.GameplayConfigBase
	if err := g.store.DB.Table(system.Table).Where("status = ? AND code NOT IN ?", "启用", []string{parents[0].Code, parents[1].Code}).Order("sort_order,id").Offset(offset).First(&child).Error; err != nil {
		return GameResult{}, true, err
	}
	costText, missing, err := g.extendedCostStatus(player, child.CostMaterials)
	if err != nil || len(missing) > 0 {
		return GameResult{Title: "嫁接材料不足", Content: fmt.Sprintf("嫁接道种趋向%s，需要%s。\n缺少：%s", child.Name, costText, strings.Join(missing, "、")), Actions: []string{"背包", "货铺", "药鉴"}}, true, nil
	}
	effect := decodeExtendedEffect(child)
	readyAt := time.Now().Add(time.Duration(minInt(maxInt(effect.Duration, 10), 180)) * time.Minute)
	progress := model.PlayerExtendedProgress{PlayerID: player.ID, System: "仙药培育", ConfigCode: child.Code, ConfigName: child.Name, State: "嫁接培育中", Level: 1, Quantity: 2, Power: effect.Power + decodeExtendedEffect(parents[0]).Power/4 + decodeExtendedEffect(parents[1]).Power/4, ReadyAt: &readyAt, MetadataJSON: fmt.Sprintf(`{"parents":[%q,%q]}`, parents[0].Code, parents[1].Code)}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		costs := make(map[string]int64)
		if err := json.Unmarshal([]byte(child.CostMaterials), &costs); err != nil {
			return err
		}
		if err := consumeExtendedCostTx(tx, player.ID, costs); err != nil {
			return err
		}
		return upsertExtendedProgressTx(tx, progress)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "仙药嫁接成功", Content: fmt.Sprintf("母株：%s\n母株：%s\n━━━━━━━━━━━\n新生药株：%s\n药力：%d · 预计产量2\n成熟：%s\n实际消耗：%s", parents[0].Name, parents[1].Name, child.Name, progress.Power, readyAt.Format("01-02 15:04"), costText), Actions: []string{"育药 " + child.Name, "催药 " + child.Name, "采药 " + child.Name}}, true, nil
}
