package model

import "time"

type SpiritualRootTemplate struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	Code              string    `gorm:"size:64;uniqueIndex" json:"code"`
	Name              string    `gorm:"size:64;uniqueIndex" json:"name"`
	Element           string    `gorm:"size:32;index" json:"element"`
	Grade             string    `gorm:"size:32;index" json:"grade"`
	BaseQuality       int       `json:"base_quality"`
	CultivationBonus  float64   `json:"cultivation_bonus"`
	PrimaryBonus      string    `gorm:"size:255" json:"primary_bonus"`
	SecondaryBonus    string    `gorm:"size:255" json:"secondary_bonus"`
	CombatDescription string    `gorm:"size:500" json:"combat_description"`
	Description       string    `gorm:"type:text" json:"description"`
	AttributeJSON     string    `gorm:"type:text" json:"attribute_json"`
	ImageURL          string    `gorm:"size:500" json:"image_url"`
	RarityWeight      int       `gorm:"index" json:"rarity_weight"`
	Enabled           bool      `gorm:"index" json:"enabled"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
