package model

import "time"

type AccountMigrationCode struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	PlayerID     uint       `gorm:"index;not null" json:"player_id"`
	OldAccountID string     `gorm:"size:64;index;not null" json:"old_account_id"`
	NewAccountID string     `gorm:"size:64;index" json:"new_account_id"`
	TokenHash    string     `gorm:"size:64;uniqueIndex;not null" json:"-"`
	Status       string     `gorm:"size:16;index;not null" json:"status"`
	ExpiresAt    time.Time  `gorm:"index;not null" json:"expires_at"`
	UsedAt       *time.Time `gorm:"index" json:"used_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

