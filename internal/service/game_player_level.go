package service

import (
	"fmt"
	"math"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"xianlv/internal/model"
)

func playerExperienceProgress(player model.Player) string {
	level := maxInt(player.Level, 1)
	required := model.PlayerExperienceRequired(level)
	experience := max64(player.Experience, 0)
	if experience > required {
		experience = required
	}
	filled := 0
	percentage := 0
	if required > 0 {
		percentage = int(float64(experience) * 100 / float64(required))
		filled = percentage / 10
	}
	if filled > 10 {
		filled = 10
	}
	return fmt.Sprintf("[%s%s] %d%% · %d/%d", strings.Repeat("█", filled), strings.Repeat("░", 10-filled), percentage, experience, required)
}

func (g *Game) applyCultivationExperience(playerID uint, gain int64) (model.PlayerLevelProgress, error) {
	var progress model.PlayerLevelProgress
	if gain <= 0 {
		return progress, nil
	}
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		var player model.Player
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&player, playerID).Error; err != nil {
			return err
		}
		progress = model.ApplyPlayerExperience(&player, gain)
		return tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(playerLevelUpdates(player)).Error
	})
	return progress, err
}

func (g *Game) grantCultivationExperience(playerID uint, gain int64) (model.PlayerLevelProgress, error) {
	var progress model.PlayerLevelProgress
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		progress, err = grantCultivationExperienceTx(tx, playerID, gain)
		return err
	})
	return progress, err
}

func grantCultivationExperienceTx(tx *gorm.DB, playerID uint, gain int64) (model.PlayerLevelProgress, error) {
	var progress model.PlayerLevelProgress
	if gain <= 0 {
		return progress, nil
	}
	var player model.Player
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&player, playerID).Error; err != nil {
		return progress, err
	}
	if player.Cultivation > math.MaxInt64-gain {
		return progress, fmt.Errorf("player %d cultivation exceeds safe integer range", playerID)
	}
	player.Cultivation += gain
	progress = model.ApplyPlayerExperience(&player, gain)
	updates := playerLevelUpdates(player)
	updates["cultivation"] = player.Cultivation
	return progress, tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(updates).Error
}

func playerLevelUpdates(player model.Player) map[string]any {
	return map[string]any{
		"level": player.Level, "experience": player.Experience,
		"health": player.Health, "max_health": player.MaxHealth,
		"mana": player.Mana, "max_mana": player.MaxMana,
		"physical_attack": player.PhysicalAttack, "magic_attack": player.MagicAttack,
		"physical_defense": player.PhysicalDefense, "magic_defense": player.MagicDefense,
		"agility": player.Agility, "strength": player.Strength, "constitution": player.Constitution,
		"spirit": player.Spirit, "perception": player.Perception, "willpower": player.Willpower,
	}
}

func (g *Game) playerLevelPowerSyncPending(playerID uint) bool {
	for _, key := range []string{"migration.player_level_power_sync", "migration.combat_power_sync"} {
		if _, err := g.playerValue(playerID, key); err == nil {
			return true
		}
	}
	return false
}

func (g *Game) clearPlayerLevelPowerSync(playerID uint) {
	_ = g.store.DB.Where("player_id = ? AND key IN ?", playerID, []string{"migration.player_level_power_sync", "migration.combat_power_sync"}).Delete(&model.PlayerValue{}).Error
}

func (g *Game) syncPendingMigrationCombatPower() {
	var playerIDs []uint
	if err := g.store.DB.Model(&model.PlayerValue{}).
		Where("key IN ?", []string{"migration.player_level_power_sync", "migration.combat_power_sync"}).
		Distinct("player_id").Pluck("player_id", &playerIDs).Error; err != nil {
		return
	}
	for _, playerID := range playerIDs {
		player, err := g.players.Get(playerID)
		if err != nil {
			continue
		}
		if g.syncPlayerCombatPower(&player) == nil {
			g.clearPlayerLevelPowerSync(playerID)
		}
	}
}

func appendPlayerLevelSettlement(result *GameResult, player model.Player, progress model.PlayerLevelProgress) {
	if result == nil || progress.ExperienceGain <= 0 {
		return
	}
	lines := []string{"━━━━━━━━━━━", fmt.Sprintf("角色经验：+%d（本次修为收益同步）", progress.ExperienceGain)}
	if progress.AfterLevel > progress.BeforeLevel {
		lines = append(lines, fmt.Sprintf("角色等级：LV%d → LV%d（连续提升%d级）", progress.BeforeLevel, progress.AfterLevel, progress.AfterLevel-progress.BeforeLevel))
		growth := []string{
			fmt.Sprintf("气血+%d", progress.HealthGain), fmt.Sprintf("法力+%d", progress.ManaGain),
			fmt.Sprintf("物攻/法强各+%d", progress.AttackGain), fmt.Sprintf("双防各+%d", progress.DefenseGain),
		}
		if progress.AgilityGain > 0 {
			growth = append(growth, fmt.Sprintf("身法+%d", progress.AgilityGain))
		}
		if progress.StrengthGain+progress.ConstitutionGain+progress.SpiritGain > 0 {
			growth = append(growth, fmt.Sprintf("力量+%d", progress.StrengthGain), fmt.Sprintf("体魄+%d", progress.ConstitutionGain), fmt.Sprintf("神识+%d", progress.SpiritGain))
		}
		if progress.PerceptionGain+progress.WillpowerGain > 0 {
			growth = append(growth, fmt.Sprintf("悟性+%d", progress.PerceptionGain), fmt.Sprintf("意志+%d", progress.WillpowerGain))
		}
		lines = append(lines, "升级成长："+strings.Join(growth, " · "))
	} else {
		lines = append(lines, fmt.Sprintf("角色等级：LV%d", progress.AfterLevel))
	}
	lines = append(lines, "等级进度 "+playerExperienceProgress(player))
	settlement := strings.Join(lines, "\n")
	if strings.TrimSpace(result.Content) == "" {
		result.Content = settlement
	} else {
		result.Content += "\n" + settlement
	}
	if strings.TrimSpace(result.MarkdownContent) != "" {
		result.MarkdownContent += "\n" + settlement
	}
}

func (g *Game) playerLevelOverview(player *model.Player) (GameResult, bool, error) {
	if player.Level < 1 || player.Experience < 0 {
		if err := g.players.UpdateColumns(player.ID, map[string]any{"level": maxInt(player.Level, 1), "experience": max64(player.Experience, 0)}); err != nil {
			return GameResult{}, true, err
		}
		player.Level = maxInt(player.Level, 1)
		player.Experience = max64(player.Experience, 0)
	}
	nextLevel := player.Level + 1
	nextHealth := model.PlayerHealthPerLevel
	nextMana := int64(4 + nextLevel/20)
	nextAttack := model.PlayerAttackPerLevel
	nextDefense := model.PlayerDefensePerLevel
	extra := []string{}
	if nextLevel%2 == 0 {
		extra = append(extra, "身法+1")
	}
	if nextLevel%5 == 0 {
		extra = append(extra, "力量/体魄/神识各+1")
	}
	if nextLevel%10 == 0 {
		extra = append(extra, "悟性/意志各+1")
	}
	if len(extra) == 0 {
		extra = append(extra, "本级无额外五维成长")
	}
	content := fmt.Sprintf("角色等级：LV%d\n等级进度 %s\n━━━━━━━━━━━\n升至LV%d：气血+%d · 法力+%d · 物攻/法强各+%d · 双防各+%d\n阶段成长：%s\n━━━━━━━━━━━\n经验规则：任务、战斗、挂机、修炼、丹药、差事、双修、论道、助力及神令等系统修行奖励，会按修为净增数额同步获得等量角色经验；一次收益可以连续跨级，超出经验会保留。玩家之间传功只转移已有修为，不重复生成角色经验。\n升级需求：100 × 当前等级²。角色等级负责基础属性与等级前置，境界修为仍用于小境突破和大境晋升，两者互不替代。", player.Level, playerExperienceProgress(*player), nextLevel, nextHealth, nextMana, nextAttack, nextDefense, strings.Join(extra, " · "))
	return GameResult{Title: "角色等级", Content: content, Actions: []string{"修炼", "任务菜单", "挂机菜单", "赚银币", "状态"}}, true, nil
}
