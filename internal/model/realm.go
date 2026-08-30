package model

import "time"

type Realm struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	Name                string    `gorm:"size:32;uniqueIndex;not null" json:"name"`
	Sequence            int       `gorm:"uniqueIndex" json:"sequence"`
	RequiredCultivation int64     `json:"required_cultivation"`
	AttributeMultiplier float64   `json:"attribute_multiplier"`
	BaseHealth          int64     `json:"base_health"`
	BaseMana            int64     `json:"base_mana"`
	BaseAttack          int64     `json:"base_attack"`
	BaseDefense         int64     `json:"base_defense"`
	BaseSpeed           int64     `json:"base_speed"`
	BaseDodge           float64   `json:"base_dodge"`
	BaseLifespan        int64     `json:"base_lifespan"`
	TribulationBaseRate float64   `json:"tribulation_base_rate"`
	Description         string    `gorm:"size:500" json:"description"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type SystemSetting struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Key         string    `gorm:"size:64;uniqueIndex" json:"key"`
	Value       string    `gorm:"size:2000" json:"value"`
	ValueType   string    `gorm:"size:16" json:"value_type"`
	Description string    `gorm:"size:255" json:"description"`
	UpdatedBy   string    `gorm:"size:64" json:"updated_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
