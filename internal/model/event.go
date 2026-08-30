package model

import "time"

type Event struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Name          string    `gorm:"size:64;uniqueIndex" json:"name"`
	Type          string    `gorm:"size:32;index" json:"type"`
	Description   string    `gorm:"size:1000" json:"description"`
	Probability   float64   `json:"probability"`
	RewardJSON    string    `gorm:"type:text" json:"reward_json"`
	ConditionJSON string    `gorm:"type:text" json:"condition_json"`
	DropPoolID    uint      `json:"drop_pool_id"`
	Enabled       bool      `gorm:"index" json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
type Broadcast struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Content   string    `gorm:"size:2000" json:"content"`
	Level     string    `gorm:"size:16" json:"level"`
	CreatedBy string    `gorm:"size:64" json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}
