package model

import "time"

type WorldLeyline struct {
	ID                    uint      `gorm:"primaryKey" json:"id"`
	Code                  string    `gorm:"size:64;uniqueIndex" json:"code"`
	Name                  string    `gorm:"size:64;uniqueIndex" json:"name"`
	Region                string    `gorm:"size:32;index" json:"region"`
	LocationName          string    `gorm:"size:64;index" json:"location_name"`
	Element               string    `gorm:"size:32;index" json:"element"`
	Grade                 string    `gorm:"size:32;index" json:"grade"`
	AuraPerMinute         int64     `json:"aura_per_minute"`
	CultivationMultiplier float64   `json:"cultivation_multiplier"`
	MeditationSlots       int       `json:"meditation_slots"`
	DiscoveryManaCost     int64     `json:"discovery_mana_cost"`
	MinimumRealmSequence  int       `json:"minimum_realm_sequence"`
	MinimumRealmLevel     int       `json:"minimum_realm_level"`
	MinimumCombatPower    int64     `json:"minimum_combat_power"`
	MinimumSpirit         int64     `json:"minimum_spirit"`
	RequiredRootElement   string    `gorm:"size:32" json:"required_root_element"`
	RequiredItem          string    `gorm:"size:64" json:"required_item"`
	RequiredItemCount     int64     `json:"required_item_count"`
	BonusJSON             string    `gorm:"type:text" json:"bonus_json"`
	Description           string    `gorm:"size:1000" json:"description"`
	ImageURL              string    `gorm:"size:500" json:"image_url"`
	Enabled               bool      `gorm:"index" json:"enabled"`
	SortOrder             int       `gorm:"index" json:"sort_order"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}
