package service

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/handler"
	"xianlv/internal/model"
	"xianlv/internal/storage"
)

func worldLeylineProgressCode(id uint) string { return fmt.Sprintf("world_leyline:%d", id) }

func (g *Game) executeWorldLeylineExtended(player *model.Player, command handler.ParsedCommand, system extendedSystem, action string) (GameResult, bool, error) {
	switch action {
	case "detect":
		return g.discoverLocalLeylines(player)
	case "occupy":
		return g.occupyWorldLeyline(player, command.RawArguments)
	case "challenge":
		return g.challengeWorldLeyline(player, command.RawArguments)
	case "practice":
		name := strings.TrimSpace(command.RawArguments)
		if name == "" {
			var progress model.PlayerExtendedProgress
			if err := g.store.DB.Where("player_id = ? AND system = ? AND state = ? AND config_code LIKE ?", player.ID, "天地灵脉", "已占据", "world_leyline:%").Order("updated_at DESC").First(&progress).Error; err != nil {
				return GameResult{Title: "灵脉修炼", Content: "你尚未占据可修炼灵脉。可先寻脉并抵达对应地图，也可直接使用世界灵脉的“灵脉打坐 完整名称”。", Actions: []string{"寻脉", "灵脉地图", "脉探"}}, true, nil
			}
			name = progress.ConfigName
		}
		return g.startLeylineMeditation(player, name)
	case "transfer":
		return g.transferWorldLeyline(player, command)
	case "combine":
		return g.combineWorldLeylines(player, command)
	default:
		return GameResult{}, false, fmt.Errorf("未知灵脉动作: %s (%s)", action, system.Table)
	}
}

func (g *Game) occupyWorldLeyline(player *model.Player, raw string) (GameResult, bool, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return GameResult{Title: "灵脉占据", Content: "请先寻脉，再发送 `脉占 灵脉完整名称`。", Actions: []string{"寻脉", "灵脉地图"}}, true, nil
	}
	row, err := g.worldLeylineByName(name)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return GameResult{Title: "灵脉未收录", Content: "世界灵脉中没有“" + name + "”。请从灵脉地图或寻脉结果点击完整名称。", Actions: []string{"灵脉地图", "寻脉"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	if player.Location != row.LocationName {
		return GameResult{Title: "尚未抵达灵脉", Content: fmt.Sprintf("%s位于%s·%s，你当前在%s。必须沿地图路线抵达后才能占据。", row.Name, row.Region, row.LocationName, player.Location), Actions: g.leylineGuidanceActions(player, row, false)}, true, nil
	}
	if _, err := g.playerValue(player.ID, fmt.Sprintf("leyline.discovered.%d", row.ID)); err != nil {
		return GameResult{Title: "灵脉尚未探明", Content: "先发送 `寻脉` 以神识确定入口，本次没有扣除材料或法力。", Actions: []string{"寻脉", "灵脉详情 " + row.Name}}, true, nil
	}
	code := worldLeylineProgressCode(row.ID)
	var mine model.PlayerExtendedProgress
	if g.store.DB.Where("player_id = ? AND system = ? AND config_code = ? AND state = ?", player.ID, "天地灵脉", code, "已占据").First(&mine).Error == nil {
		return GameResult{Title: "灵脉已归你掌控", Content: fmt.Sprintf("灵脉：%s\n灵气：%d/分钟 · 修炼倍率×%.3f\n无需重复消耗护脉材料。", row.Name, row.AuraPerMinute, row.CultivationMultiplier), Actions: []string{"脉修 " + row.Name, "采灵气 " + row.Name, "灵脉详情 " + row.Name}}, true, nil
	}
	var owner model.PlayerExtendedProgress
	ownerErr := g.store.DB.Where("system = ? AND config_code = ? AND state = ? AND player_id <> ?", "天地灵脉", code, "已占据", player.ID).Order("updated_at DESC").First(&owner).Error
	if ownerErr == nil {
		var ownerPlayer model.Player
		_ = g.store.DB.First(&ownerPlayer, owner.PlayerID).Error
		return GameResult{Title: "灵脉已有归属", Content: fmt.Sprintf("灵脉：%s\n当前占据者：%s\n必须发送“脉争 %s”并在逐回合斗法中取胜，才会转移占领权。", row.Name, displayOr(ownerPlayer.DaoName, "未知修士"), row.Name), Actions: []string{"脉争 " + row.Name, "灵脉详情 " + row.Name, "状态"}}, true, nil
	}
	if ownerErr != nil && !errors.Is(ownerErr, gorm.ErrRecordNotFound) {
		return GameResult{}, true, ownerErr
	}
	unmet, err := g.worldLeylineUnmet(player, row)
	if err != nil {
		return GameResult{}, true, err
	}
	if len(unmet) > 0 {
		return GameResult{Title: "灵脉前置未满", Content: strings.Join(unmet, "\n"), Actions: g.leylineGuidanceActions(player, row, false)}, true, nil
	}
	progress := model.PlayerExtendedProgress{PlayerID: player.ID, System: "天地灵脉", ConfigCode: code, ConfigName: row.Name, State: "已占据", Level: maxInt(row.MinimumRealmSequence, 1), Power: max64(row.AuraPerMinute*10, row.MinimumCombatPower), MetadataJSON: fmt.Sprintf(`{"leyline_id":%d}`, row.ID)}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if row.RequiredItemCount > 0 {
			var item model.Item
			if err := tx.Where("name = ?", row.RequiredItem).First(&item).Error; err != nil {
				return err
			}
			if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, item.ID, -row.RequiredItemCount); err != nil {
				return err
			}
		}
		result := tx.Model(&model.Player{}).Where("id = ? AND mana >= ?", player.ID, row.DiscoveryManaCost).Update("mana", gorm.Expr("mana - ?", row.DiscoveryManaCost))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("法力不足")
		}
		return upsertExtendedProgressTx(tx, progress)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "灵脉占据成功", Content: fmt.Sprintf("灵脉：%s【%s】\n位置：%s·%s\n灵气：%d/分钟 · 修炼倍率×%.3f\n消耗：法力%d · %s×%d\n━━━━━━━━━━━\n占领权已经写入个人道籍；其他修士只能通过真实灵脉争夺战取走。", row.Name, row.Grade, row.Region, row.LocationName, row.AuraPerMinute, row.CultivationMultiplier, row.DiscoveryManaCost, displayOr(row.RequiredItem, "护脉材料"), row.RequiredItemCount), Actions: []string{"脉修 " + row.Name, "采灵气 " + row.Name, "灵脉详情 " + row.Name, "领地"}}, true, nil
}

func (g *Game) challengeWorldLeyline(player *model.Player, raw string) (GameResult, bool, error) {
	name := strings.TrimSpace(raw)
	row, err := g.worldLeylineByName(name)
	if err != nil {
		return GameResult{Title: "灵脉争夺", Content: "请从灵脉地图填写真实灵脉完整名称。", Actions: []string{"灵脉地图", "寻脉"}}, true, nil
	}
	if player.Location != row.LocationName {
		return GameResult{Title: "尚未抵达灵脉", Content: fmt.Sprintf("必须先抵达%s，当前在%s。", row.LocationName, player.Location), Actions: g.leylineGuidanceActions(player, row, false)}, true, nil
	}
	code := worldLeylineProgressCode(row.ID)
	var owner model.PlayerExtendedProgress
	if err := g.store.DB.Where("system = ? AND config_code = ? AND state = ? AND player_id <> ?", "天地灵脉", code, "已占据", player.ID).Order("updated_at DESC").First(&owner).Error; err != nil {
		return GameResult{Title: "灵脉无人镇守", Content: "这条灵脉当前没有其他占据者，无需争夺；满足前置后可直接占据。", Actions: []string{"脉占 " + row.Name, "灵脉详情 " + row.Name}}, true, nil
	}
	if player.State != "" && player.State != model.PlayerStateIdle {
		return GameResult{Title: "当前无法争夺", Content: "请先结束当前修行或战斗状态。", Actions: []string{"状态", "投降"}}, true, nil
	}
	var ownerPlayer model.Player
	_ = g.store.DB.First(&ownerPlayer, owner.PlayerID).Error
	enemyPower := max64(row.MinimumCombatPower, ownerPlayer.CombatPower)
	enemyHP := max64(enemyPower*2, 100)
	effective := g.playerWithActiveSkillStats(player)
	state := mapMonsterBattleState{BattleKind: "道藏试炼", Round: 1, EnemyName: displayOr(ownerPlayer.DaoName, row.Name+"守脉法相"), EnemyPower: enemyPower, PlayerHP: effective.Health, PlayerMana: effective.Mana, EnemyHP: enemyHP, EnemyMaxHP: enemyHP, ExtendedCategory: "天地灵脉", ExtendedConfigCode: code, ExtendedConfigName: row.Name, ExtendedAction: "challenge", StartedAt: time.Now().UnixMilli()}
	if err := g.beginPVEBattle(player.ID, state); err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "灵脉争夺战开启", Content: fmt.Sprintf("灵脉：%s\n镇守者：%s · 战力%d\n敌方气血：%d/%d\n你的气血：%d/%d · 法力：%d/%d\n━━━━━━━━━━━\n只有逐回合取胜才会转移占领权。", row.Name, state.EnemyName, enemyPower, enemyHP, enemyHP, effective.Health, effective.MaxHealth, effective.Mana, effective.MaxMana), Actions: []string{"攻击", "技能", "防御", "投降", "功法"}}, true, nil
}

func (g *Game) finishWorldLeylineExtendedBattle(player *model.Player, state mapMonsterBattleState, won bool, logLine string) (GameResult, bool, error) {
	remainingHP, remainingMana := max64(state.PlayerHP, 1), max64(state.PlayerMana, 0)
	var row model.WorldLeyline
	leylineID, _ := strconv.ParseUint(strings.TrimPrefix(state.ExtendedConfigCode, "world_leyline:"), 10, 64)
	if err := g.store.DB.First(&row, uint(leylineID)).Error; err != nil {
		// ConfigCode stores the numeric ID after the prefix; fall back to name for migrated battles.
		if err := g.store.DB.Where("name = ?", state.ExtendedConfigName).First(&row).Error; err != nil {
			return GameResult{}, true, err
		}
	}
	if !won {
		if err := g.store.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("player_id = ? AND key = ?", player.ID, "pve.battle").Delete(&model.PlayerValue{}).Error; err != nil {
				return err
			}
			return tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{"health": remainingHP, "mana": remainingMana, "state": model.PlayerStateIdle}).Error
		}); err != nil {
			return GameResult{}, true, err
		}
		return GameResult{Title: "灵脉争夺失败", Content: fmt.Sprintf("灵脉：%s\n%s\n━━━━━━━━━━━\n占领权未发生变化，本次无奖励。", state.ExtendedConfigName, logLine), Actions: []string{"疗伤", "灵脉详情 " + state.ExtendedConfigName, "状态"}}, true, nil
	}
	code := worldLeylineProgressCode(row.ID)
	progress := model.PlayerExtendedProgress{PlayerID: player.ID, System: "天地灵脉", ConfigCode: code, ConfigName: row.Name, State: "已占据", Level: maxInt(row.MinimumRealmSequence, 1), Mastery: 1, Uses: 1, Power: max64(row.AuraPerMinute*10, row.MinimumCombatPower), MetadataJSON: fmt.Sprintf(`{"leyline_id":%d}`, row.ID)}
	oldOwnerID := uint(0)
	var oldOwner model.PlayerExtendedProgress
	if g.store.DB.Where("system = ? AND config_code = ? AND state = ? AND player_id <> ?", "天地灵脉", code, "已占据", player.ID).First(&oldOwner).Error == nil {
		oldOwnerID = oldOwner.PlayerID
	}
	reward := max64(row.AuraPerMinute*5, 20)
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("player_id = ? AND key = ?", player.ID, "pve.battle").Delete(&model.PlayerValue{}).Error; err != nil {
			return err
		}
		if oldOwner.ID != 0 {
			if err := tx.Model(&oldOwner).Update("state", "失守").Error; err != nil {
				return err
			}
		}
		if err := upsertExtendedProgressTx(tx, progress); err != nil {
			return err
		}
		return tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{"health": remainingHP, "mana": remainingMana, "state": model.PlayerStateIdle, "cultivation": gorm.Expr("cultivation + ?", reward), "merit": gorm.Expr("merit + 2")}).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	if oldOwnerID != 0 {
		_ = g.createPlayerNotification(oldOwnerID, "灵脉失守", fmt.Sprintf("%s在逐回合斗法中攻破你的守脉法相，%s的占领权已经转移。", player.DaoName, row.Name))
	}
	return GameResult{Title: "灵脉争夺胜利", Content: fmt.Sprintf("灵脉：%s\n%s\n━━━━━━━━━━━\n第%d回合取胜，占领权已真实转移。\n修为+%d · 功德+2", row.Name, logLine, state.Round, reward), Actions: []string{"脉修 " + row.Name, "采灵气 " + row.Name, "灵脉详情 " + row.Name, "状态"}}, true, nil
}

func (g *Game) transferWorldLeyline(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	if len(command.Arguments) == 0 {
		return GameResult{Title: "灵脉转移", Content: "请输入：`脉转 @对方 [灵脉名]`。未填写灵脉名时转移最近占据的一条。", Actions: []string{"脉探", "灵脉地图"}}, true, nil
	}
	target, err := g.findPlayer(command.Arguments[0])
	if err != nil || target.ID == player.ID {
		return GameResult{Title: "转移对象无效", Content: "请选择另一名现存道友。", Actions: []string{"好友"}}, true, nil
	}
	query := g.store.DB.Where("player_id = ? AND system = ? AND state = ? AND config_code LIKE ?", player.ID, "天地灵脉", "已占据", "world_leyline:%")
	if len(command.Arguments) > 1 {
		query = query.Where("config_name = ?", strings.Join(command.Arguments[1:], " "))
	}
	var source model.PlayerExtendedProgress
	if err := query.Order("updated_at DESC").First(&source).Error; err != nil {
		return GameResult{Title: "没有可转移灵脉", Content: "你没有占据对应灵脉。", Actions: []string{"脉探", "灵脉地图"}}, true, nil
	}
	grant := source
	grant.ID, grant.PlayerID, grant.State, grant.Uses = 0, target.ID, "已占据", 0
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&source).Update("state", "已转让").Error; err != nil {
			return err
		}
		return upsertExtendedProgressTx(tx, grant)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	_ = g.createPlayerNotification(target.ID, "灵脉转让", fmt.Sprintf("%s将%s的占领权转让给你。抵达对应地图后可修炼与采气。", player.DaoName, source.ConfigName))
	return GameResult{Title: "灵脉转移完成", Content: fmt.Sprintf("灵脉：%s\n原占据者：%s\n新占据者：%s\n占领记录与修炼入口已同步转移。", source.ConfigName, player.DaoName, target.DaoName), Actions: []string{"脉探", "通知", "灵脉地图"}}, true, nil
}

func (g *Game) combineWorldLeylines(player *model.Player, command handler.ParsedCommand) (GameResult, bool, error) {
	if len(command.Arguments) < 2 || command.Arguments[0] == command.Arguments[1] {
		return GameResult{Title: "灵脉融合", Content: "请输入两条不同且由你掌控的灵脉：`脉合 灵脉一 灵脉二`。", Actions: []string{"脉探", "灵脉地图"}}, true, nil
	}
	rows := make([]model.WorldLeyline, 0, 2)
	for _, name := range command.Arguments[:2] {
		row, err := g.worldLeylineByName(name)
		if err != nil {
			return GameResult{Title: "灵脉融合失败", Content: "没有找到“" + name + "”。", Actions: []string{"灵脉地图"}}, true, nil
		}
		var progress model.PlayerExtendedProgress
		if g.store.DB.Where("player_id = ? AND system = ? AND config_code = ? AND state = ?", player.ID, "天地灵脉", worldLeylineProgressCode(row.ID), "已占据").First(&progress).Error != nil {
			return GameResult{Title: "灵脉融合失败", Content: "你尚未掌控“" + row.Name + "”，本次没有消耗。", Actions: []string{"脉占 " + row.Name, "灵脉详情 " + row.Name}}, true, nil
		}
		rows = append(rows, row)
	}
	ids := []uint{rows[0].ID, rows[1].ID}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	code := fmt.Sprintf("leyline_resonance:%d:%d", ids[0], ids[1])
	var existing model.PlayerExtendedProgress
	if g.store.DB.Where("player_id = ? AND system = ? AND config_code = ?", player.ID, "天地灵脉", code).First(&existing).Error == nil {
		return GameResult{Title: "灵脉共鸣已成", Content: fmt.Sprintf("%s已经完成，不会重复叠加永久属性。", existing.ConfigName), Actions: []string{"状态", "脉探"}}, true, nil
	}
	gain := max64((rows[0].AuraPerMinute+rows[1].AuraPerMinute)/2, 5)
	name := rows[0].Element + rows[1].Element + "两仪共鸣脉"
	progress := model.PlayerExtendedProgress{PlayerID: player.ID, System: "天地灵脉", ConfigCode: code, ConfigName: name, State: "融合共鸣", Level: maxInt((rows[0].MinimumRealmSequence+rows[1].MinimumRealmSequence)/2, 1), Power: rows[0].AuraPerMinute*10 + rows[1].AuraPerMinute*10, MetadataJSON: fmt.Sprintf(`{"parents":[%d,%d]}`, ids[0], ids[1])}
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := upsertExtendedProgressTx(tx, progress); err != nil {
			return err
		}
		return tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(map[string]any{"max_mana": gorm.Expr("max_mana + ?", gain), "mana": gorm.Expr("mana + ?", gain), "spirit": gorm.Expr("spirit + ?", max64(gain/5, 1))}).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	latest, _ := g.players.Get(player.ID)
	_ = g.syncPlayerCombatPower(&latest)
	return GameResult{Title: "灵脉融合成功", Content: fmt.Sprintf("父脉：%s\n父脉：%s\n━━━━━━━━━━━\n共鸣道脉：%s\n永久法力+%d · 神识+%d\n两条世界灵脉仍保留原有位置与归属。", rows[0].Name, rows[1].Name, name, gain, max64(gain/5, 1)), Actions: []string{"状态", "脉探", "灵脉地图"}}, true, nil
}
