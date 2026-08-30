package service

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"xianlv/internal/model"
)

const playerLevelFloorRepairKey = "migration.player_level_floor.v2.2.2"

func playerNeedsLevelFloor(player model.Player) bool {
	floor := model.PlayerLevelStats(player.Level)
	return player.MaxHealth < floor.MaxHealth || player.MaxMana < floor.MaxMana ||
		player.PhysicalAttack < floor.PhysicalAttack || player.MagicAttack < floor.MagicAttack ||
		player.PhysicalDefense < floor.PhysicalDefense || player.MagicDefense < floor.MagicDefense ||
		player.Agility < floor.Agility || player.Strength < floor.Strength ||
		player.Constitution < floor.Constitution || player.Spirit < floor.Spirit ||
		player.Perception < floor.Perception || player.Willpower < floor.Willpower
}

func playerWithLevelFloor(player model.Player) model.Player {
	floor := model.PlayerLevelStats(player.Level)
	if player.MaxHealth < floor.MaxHealth {
		delta := floor.MaxHealth - player.MaxHealth
		player.MaxHealth = floor.MaxHealth
		player.Health = min64(player.Health+delta, player.MaxHealth)
	}
	if player.MaxMana < floor.MaxMana {
		delta := floor.MaxMana - player.MaxMana
		player.MaxMana = floor.MaxMana
		player.Mana = min64(player.Mana+delta, player.MaxMana)
	}
	player.PhysicalAttack = max64(player.PhysicalAttack, floor.PhysicalAttack)
	player.MagicAttack = max64(player.MagicAttack, floor.MagicAttack)
	player.PhysicalDefense = max64(player.PhysicalDefense, floor.PhysicalDefense)
	player.MagicDefense = max64(player.MagicDefense, floor.MagicDefense)
	player.Agility = max64(player.Agility, floor.Agility)
	player.Strength = max64(player.Strength, floor.Strength)
	player.Constitution = max64(player.Constitution, floor.Constitution)
	player.Spirit = max64(player.Spirit, floor.Spirit)
	player.Perception = max64(player.Perception, floor.Perception)
	player.Willpower = max64(player.Willpower, floor.Willpower)
	return player
}

func playerLevelFloorUpdates(player model.Player) map[string]any {
	return map[string]any{
		"health": player.Health, "max_health": player.MaxHealth,
		"mana": player.Mana, "max_mana": player.MaxMana,
		"physical_attack": player.PhysicalAttack, "magic_attack": player.MagicAttack,
		"physical_defense": player.PhysicalDefense, "magic_defense": player.MagicDefense,
		"agility": player.Agility, "strength": player.Strength, "constitution": player.Constitution,
		"spirit": player.Spirit, "perception": player.Perception, "willpower": player.Willpower,
	}
}

func (g *Game) ensurePlayerLevelFloor(player *model.Player) error {
	if player == nil || !playerNeedsLevelFloor(*player) {
		return nil
	}
	var updated model.Player
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&updated, player.ID).Error; err != nil {
			return err
		}
		if !playerNeedsLevelFloor(updated) {
			return nil
		}
		updated = playerWithLevelFloor(updated)
		return tx.Model(&model.Player{}).Where("id = ?", updated.ID).Updates(playerLevelFloorUpdates(updated)).Error
	})
	if err != nil {
		return err
	}
	*player = updated
	return g.syncPlayerCombatPower(player)
}

// repairPlayerLevelFloors restores only the published level minimum. It never
// reduces an existing value and therefore preserves legitimate permanent,
// equipment, title, root and inheritance growth. The marker makes the live
// database pass cheap on later starts; ensurePlayerLevelFloor remains a guard
// on every loaded character in case a future write path regresses.
func (g *Game) repairPlayerLevelFloors() error {
	var marker model.SystemSetting
	if err := g.store.DB.Where("key = ? AND value = ?", playerLevelFloorRepairKey, "complete").First(&marker).Error; err == nil {
		return nil
	} else if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	var repaired []uint
	err := g.store.DB.Transaction(func(tx *gorm.DB) error {
		var players []model.Player
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("deleted_at IS NULL").Find(&players).Error; err != nil {
			return err
		}
		for _, player := range players {
			if !playerNeedsLevelFloor(player) {
				continue
			}
			updated := playerWithLevelFloor(player)
			if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Updates(playerLevelFloorUpdates(updated)).Error; err != nil {
				return err
			}
			repaired = append(repaired, player.ID)
		}
		marker = model.SystemSetting{Key: playerLevelFloorRepairKey, Value: "complete", ValueType: "string", Description: "v2.2.2角色等级基础属性回正，幂等执行"}
		return tx.Where("key = ?", marker.Key).Assign(map[string]any{"value": marker.Value, "value_type": marker.ValueType, "description": marker.Description}).FirstOrCreate(&marker).Error
	})
	if err != nil {
		return err
	}
	for _, playerID := range repaired {
		player, loadErr := g.players.Get(playerID)
		if loadErr != nil {
			return loadErr
		}
		if syncErr := g.syncPlayerCombatPower(&player); syncErr != nil {
			return fmt.Errorf("sync repaired player %d combat power: %w", playerID, syncErr)
		}
	}
	return nil
}
