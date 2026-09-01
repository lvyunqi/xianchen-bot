package storage

import (
	"xianlv/internal/model"

	"gorm.io/gorm"
)

const migrationCombatPowerSyncKey = "migration.combat_power_sync"

func (s *Store) normalizePetEvolutionBalance() error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var pets []model.Pet
		if err := tx.Where("evolution > ?", 0).Find(&pets).Error; err != nil {
			return err
		}
		for _, pet := range pets {
			var template model.PetTemplate
			if err := tx.Where("name = ? OR code = ?", pet.Species, pet.Species).Order("id").First(&template).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					continue
				}
				return err
			}
			requirement := model.PetEvolutionRequirementFor(template)
			validEvolution := pet.Level >= requirement.Level
			if pet.Evolution == 1 && validEvolution {
				continue
			}
			evolution := 0
			rarity := "凡品"
			if validEvolution {
				evolution = 1
				rarity = "灵品"
			}
			attack, defense, health := model.PetStatsAtLevel(template, pet.Level, evolution == 1)
			if err := tx.Model(&model.Pet{}).Where("id = ?", pet.ID).Updates(map[string]any{
				"evolution": evolution, "rarity": rarity,
				"attack": attack, "defense": defense, "health": health,
			}).Error; err != nil {
				return err
			}
			if err := markMigrationCombatPowerSync(tx, pet.PlayerID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) normalizeCreatedSkillBalance() error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var skills []model.Skill
		if err := tx.Where("rarity = ?", "自创").Find(&skills).Error; err != nil {
			return err
		}
		for _, skill := range skills {
			clamped, changed := model.ClampCreatedSkillEffectJSON(skill.Type, skill.EffectJSON)
			if !changed {
				continue
			}
			if err := tx.Model(&model.Skill{}).Where("id = ?", skill.ID).Update("effect_json", clamped).Error; err != nil {
				return err
			}
			var playerIDs []uint
			if err := tx.Model(&model.PlayerSkill{}).Where("skill_id = ?", skill.ID).Distinct("player_id").Pluck("player_id", &playerIDs).Error; err != nil {
				return err
			}
			for _, playerID := range playerIDs {
				if err := markMigrationCombatPowerSync(tx, playerID); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func markMigrationCombatPowerSync(tx *gorm.DB, playerID uint) error {
	if playerID == 0 {
		return nil
	}
	marker := model.PlayerValue{PlayerID: playerID, Key: migrationCombatPowerSyncKey, Value: "true"}
	return tx.Where("player_id = ? AND key = ?", playerID, marker.Key).
		Assign(map[string]any{"value": marker.Value, "expires_at": nil}).FirstOrCreate(&marker).Error
}
