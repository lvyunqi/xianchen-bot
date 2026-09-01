package model

import "time"

type SynthesisRecipe struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	Code             string    `gorm:"size:64;uniqueIndex" json:"code"`
	Name             string    `gorm:"size:64;uniqueIndex" json:"name"`
	Category         string    `gorm:"size:32;index" json:"category"`
	MaterialsJSON    string    `gorm:"type:text" json:"materials_json"`
	OutputItemID     uint      `gorm:"index" json:"output_item_id"`
	OutputName       string    `gorm:"size:64;index" json:"output_name"`
	OutputQuantity   int64     `json:"output_quantity"`
	SuccessRate      float64   `json:"success_rate"`
	PrerequisiteJSON string    `gorm:"type:text" json:"prerequisite_json"`
	Description      string    `gorm:"type:text" json:"description"`
	Enabled          bool      `gorm:"index" json:"enabled"`
	SortOrder        int       `gorm:"index" json:"sort_order"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
