package service

import (
	"fmt"
	"strings"

	"gorm.io/gorm"

	"xianlv/internal/handler"
	"xianlv/internal/model"
	"xianlv/internal/storage"
)

func (g *Game) executeArtifactRefinementExtended(player *model.Player, command handler.ParsedCommand, system extendedSystem, action string) (GameResult, bool, error) {
	if action == "combine" {
		parts := command.Arguments
		if len(parts) < 2 {
			return GameResult{Title: "法宝融合", Content: "请输入两件不同且未穿戴的法宝：`宝融 主法宝 副法宝`。副法宝会永久消耗。", Actions: []string{"装备背包", "装备帮助"}}, true, nil
		}
		return g.fuseArtifacts(player, parts[0]+"="+parts[1])
	}
	if action == "transfer" {
		if len(command.Arguments) < 2 {
			return GameResult{Title: "法宝传承", Content: "请输入：`宝传 @对方 法宝名`。", Actions: []string{"装备背包", "好友"}}, true, nil
		}
		return g.transferArtifact(player, strings.Join(command.Arguments, " "))
	}
	name := strings.TrimSpace(command.RawArguments)
	if name == "" {
		return g.artifactRefinementChoices(player, command), true, nil
	}
	row, err := g.ownedArtifact(player.ID, name)
	if err != nil {
		return GameResult{Title: command.Spec.Name + "失败", Content: "装备背包中没有“" + name + "”，请点击本人实际拥有的法宝。", Actions: []string{"装备背包", "当前装备"}}, true, nil
	}
	switch action {
	case "refine":
		return g.refineOwnedArtifact(player, row, system)
	case "awaken":
		return g.awakenOwnedArtifact(player, row, system)
	case "cultivate":
		return g.cultivateOwnedArtifact(player, row, system)
	case "bind":
		return g.bindOwnedArtifact(player, row, system)
	default:
		return GameResult{}, false, fmt.Errorf("未知法宝炼化动作: %s", action)
	}
}

func (g *Game) artifactRefinementChoices(player *model.Player, command handler.ParsedCommand) GameResult {
	var rows []model.PlayerArtifact
	_ = g.store.DB.Where("player_id = ?", player.ID).Order("equipped DESC,quality DESC,level DESC,id").Limit(12).Find(&rows).Error
	lines := []string{"选择本人实际拥有的法宝", "炼化、开光与蕴养会直接改变该件法宝；穿戴中的属性与总战力实时同步。", "━━━━━━━━━━━"}
	actions := []string{"装备背包", "当前装备", "装备帮助"}
	for _, row := range rows {
		stats := g.equipmentStats(row)
		slot, archetype := g.artifactDisplayIdentity(&row)
		state := "背包"
		if row.Equipped {
			state = "已穿戴"
		}
		lines = append(lines, fmt.Sprintf("- %s【%s】\n  槽位：%s · 器型：%s\n  %s +%d · 锻造%d · 星阶%d\n  攻击%d · 防御%d · 气血%d · 法力%d", row.Name, state, slot, archetype, row.Quality, row.Level, row.ForgeLevel, row.StarLevel, stats.Attack+stats.Power, stats.Defense, stats.Health, stats.Mana))
		actions = append(actions, command.Spec.Command+" "+row.Name, "装备详情 "+row.Name)
	}
	if len(rows) == 0 {
		lines = append(lines, "尚无法宝。先从器谱、仙盟器阁、副本或首领获取。")
		actions = append(actions, "仙盟器阁", "装备图鉴", "炼器菜单")
	}
	return GameResult{Title: command.Spec.Name, Content: strings.Join(lines, "\n"), Actions: uniqueExtendedActions(actions)}
}

func (g *Game) artifactRefinementProgress(playerID uint, row model.PlayerArtifact) model.PlayerExtendedProgress {
	code := fmt.Sprintf("owned_artifact:%d", row.ID)
	var progress model.PlayerExtendedProgress
	if g.store.DB.Where("player_id = ? AND system = ? AND config_code = ?", playerID, "法宝炼化", code).First(&progress).Error != nil {
		progress = model.PlayerExtendedProgress{PlayerID: playerID, System: "法宝炼化", ConfigCode: code, ConfigName: row.Name, State: "器灵初醒", Level: maxInt(row.Level, 1), MetadataJSON: fmt.Sprintf(`{"artifact_id":%d}`, row.ID)}
	}
	progress.ConfigName = row.Name
	return progress
}

func (g *Game) refineOwnedArtifact(player *model.Player, row model.PlayerArtifact, system extendedSystem) (GameResult, bool, error) {
	if row.ForgeLevel >= 30 {
		return GameResult{Title: "炼化圆满", Content: row.Name + "已完成三十重真火炼化，不会继续消耗玄铁。", Actions: []string{"装备详情 " + row.Name, "开光 " + row.Name}}, true, nil
	}
	cost := int64(maxInt(row.ForgeLevel+1, 1) * 2)
	iron, err := g.itemByName("玄铁")
	if err != nil || g.itemQuantity(player.ID, iron.ID) < cost {
		return GameResult{Title: "炼化材料不足", Content: fmt.Sprintf("%s下一重炼化需要玄铁×%d。", row.Name, cost), Actions: []string{"物品 玄铁", "地图", "副本", "装备背包"}}, true, nil
	}
	before := g.equipmentStats(row)
	updated := row
	updated.ForgeLevel++
	after := g.equipmentStats(updated)
	progress := g.artifactRefinementProgress(player.ID, row)
	progress.State, progress.Level, progress.Uses, progress.Mastery, progress.Power = "真火炼化", updated.ForgeLevel, progress.Uses+1, progress.Mastery+int64(updated.ForgeLevel), artifactStatsPower(after)
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, iron.ID, -cost); err != nil {
			return err
		}
		if err := tx.Model(&row).Update("forge_level", updated.ForgeLevel).Error; err != nil {
			return err
		}
		return upsertExtendedProgressTx(tx, progress)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	if row.Equipped {
		if err := g.applyEquipmentStatDifference(player.ID, before, after); err != nil {
			return GameResult{}, true, err
		}
	}
	latest, _ := g.players.Get(player.ID)
	_ = g.syncPlayerCombatPower(&latest)
	slot, archetype := g.artifactDisplayIdentity(&row)
	return GameResult{Title: "玄火炼化", Content: fmt.Sprintf("法宝：%s\n槽位：%s · 器型：%s\n炼化：%d → %d/30\n消耗：玄铁×%d\n器物道力：%d → %d\n穿戴同步：%s", row.Name, slot, archetype, row.ForgeLevel, updated.ForgeLevel, cost, artifactStatsPower(before), artifactStatsPower(after), artifactWearState(row.Equipped)), Actions: []string{"炼化 " + row.Name, "开光 " + row.Name, "蕴养 " + row.Name, "装备详情 " + row.Name, "状态"}}, true, nil
}

func (g *Game) awakenOwnedArtifact(player *model.Player, row model.PlayerArtifact, system extendedSystem) (GameResult, bool, error) {
	if row.StarLevel >= 20 {
		return GameResult{Title: "法宝开光圆满", Content: row.Name + "已达二十星，不会继续消耗星辰砂。", Actions: []string{"装备详情 " + row.Name}}, true, nil
	}
	cost := int64(maxInt(row.StarLevel+1, 1) * 2)
	sand, err := g.itemByName("星辰砂")
	if err != nil || g.itemQuantity(player.ID, sand.ID) < cost {
		return GameResult{Title: "开光材料不足", Content: fmt.Sprintf("唤醒下一重星辉需要星辰砂×%d。", cost), Actions: []string{"物品 星辰砂", "地图", "副本"}}, true, nil
	}
	before := g.equipmentStats(row)
	updated := row
	updated.StarLevel++
	updated.Activated = true
	after := g.equipmentStats(updated)
	progress := g.artifactRefinementProgress(player.ID, row)
	progress.State, progress.Level, progress.Uses, progress.Mastery, progress.Power = "器灵开光", updated.StarLevel, progress.Uses+1, progress.Mastery+int64(updated.StarLevel*2), artifactStatsPower(after)
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, sand.ID, -cost); err != nil {
			return err
		}
		if err := tx.Model(&row).Updates(map[string]any{"star_level": updated.StarLevel, "activated": true}).Error; err != nil {
			return err
		}
		return upsertExtendedProgressTx(tx, progress)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	if row.Equipped {
		if err := g.applyEquipmentStatDifference(player.ID, before, after); err != nil {
			return GameResult{}, true, err
		}
	}
	latest, _ := g.players.Get(player.ID)
	_ = g.syncPlayerCombatPower(&latest)
	slot, archetype := g.artifactDisplayIdentity(&row)
	return GameResult{Title: "法宝开光", Content: fmt.Sprintf("法宝：%s\n槽位：%s · 器型：%s\n星阶：%d → %d/20\n消耗：星辰砂×%d\n器灵已激活：是\n器物道力：%d → %d", row.Name, slot, archetype, row.StarLevel, updated.StarLevel, cost, artifactStatsPower(before), artifactStatsPower(after)), Actions: []string{"开光 " + row.Name, "蕴养 " + row.Name, "装备详情 " + row.Name, "状态"}}, true, nil
}

func (g *Game) cultivateOwnedArtifact(player *model.Player, row model.PlayerArtifact, system extendedSystem) (GameResult, bool, error) {
	var template model.ArtifactTemplate
	_ = g.store.DB.First(&template, row.TemplateID).Error
	maximum := maxInt(template.MaxLevel, 10)
	if row.Level >= maximum {
		return GameResult{Title: "法宝蕴养圆满", Content: fmt.Sprintf("%s已达%d级，不会继续消耗灵茶。", row.Name, maximum), Actions: []string{"装备详情 " + row.Name}}, true, nil
	}
	tea, err := g.itemByName("灵茶")
	if err != nil || g.itemQuantity(player.ID, tea.ID) < 1 {
		return GameResult{Title: "蕴养资源不足", Content: "以神识蕴养法宝需要灵茶×1。", Actions: []string{"物品 灵茶", "货铺", "背包"}}, true, nil
	}
	before := g.equipmentStats(row)
	updated := row
	updated.Level++
	after := g.equipmentStats(updated)
	progress := g.artifactRefinementProgress(player.ID, row)
	progress.State, progress.Level, progress.Uses, progress.Experience, progress.Power = "神识蕴养", updated.Level, progress.Uses+1, progress.Experience+int64(updated.Level*10), artifactStatsPower(after)
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := storage.NewPlayerRepository(tx).AdjustItem(player.ID, tea.ID, -1); err != nil {
			return err
		}
		if err := tx.Model(&row).Update("level", updated.Level).Error; err != nil {
			return err
		}
		return upsertExtendedProgressTx(tx, progress)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	if row.Equipped {
		if err := g.applyEquipmentStatDifference(player.ID, before, after); err != nil {
			return GameResult{}, true, err
		}
	}
	latest, _ := g.players.Get(player.ID)
	_ = g.syncPlayerCombatPower(&latest)
	slot, archetype := g.artifactDisplayIdentity(&row)
	return GameResult{Title: "法宝蕴养", Content: fmt.Sprintf("法宝：%s\n槽位：%s · 器型：%s\n等级：%d → %d/%d\n消耗：灵茶×1\n器物道力：%d → %d\n穿戴属性与总战力：已同步", row.Name, slot, archetype, row.Level, updated.Level, maximum, artifactStatsPower(before), artifactStatsPower(after)), Actions: []string{"蕴养 " + row.Name, "炼化 " + row.Name, "装备详情 " + row.Name, "状态"}}, true, nil
}

func (g *Game) bindOwnedArtifact(player *model.Player, row model.PlayerArtifact, system extendedSystem) (GameResult, bool, error) {
	key := fmt.Sprintf("artifact.bound.%d", row.ID)
	if value, err := g.playerValue(player.ID, key); err == nil && value == fmt.Sprint(player.ID) {
		return GameResult{Title: "法宝已经认主", Content: fmt.Sprintf("法宝：%s\n主人：%s\n重复认主不会消耗资源或叠加属性。", row.Name, player.DaoName), Actions: []string{"装备详情 " + row.Name, "当前装备"}}, true, nil
	}
	progress := g.artifactRefinementProgress(player.ID, row)
	progress.State, progress.Uses, progress.Power = "神魂认主", progress.Uses+1, artifactStatsPower(g.equipmentStats(row))
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&row).Update("activated", true).Error; err != nil {
			return err
		}
		if err := upsertPlayerValueTx(tx, player.ID, key, fmt.Sprint(player.ID), nil); err != nil {
			return err
		}
		return upsertExtendedProgressTx(tx, progress)
	})
	if err != nil {
		return GameResult{}, true, err
	}
	slot, archetype := g.artifactDisplayIdentity(&row)
	return GameResult{Title: "法宝认主完成", Content: fmt.Sprintf("法宝：%s\n槽位：%s · 器型：%s\n主人：%s\n品质：%s · 等级+%d · 锻造%d · 星阶%d\n认主记录已写入道籍；装备属性仍只在实际穿戴时计入。", row.Name, slot, archetype, player.DaoName, row.Quality, row.Level, row.ForgeLevel, row.StarLevel), Actions: []string{"穿戴 " + row.Name, "装备详情 " + row.Name, "当前装备"}}, true, nil
}

func artifactStatsPower(stats equipmentStats) int64 {
	return max64((stats.Attack+stats.Power)*2+stats.Defense*2+stats.Health/5+stats.Mana/5+stats.Speed, 1)
}

func artifactWearState(value bool) string {
	if value {
		return "已同步"
	}
	return "未穿戴，穿戴后生效"
}
