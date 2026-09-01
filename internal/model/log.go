package model

import "time"

type GameLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Level        string    `gorm:"size:16;index" json:"level"`
	Type         string    `gorm:"size:32;index" json:"type"`
	PlayerID     uint      `gorm:"index" json:"player_id"`
	Message      string    `gorm:"size:2000" json:"message"`
	MetadataJSON string    `gorm:"type:text" json:"metadata_json"`
	CreatedAt    time.Time `gorm:"index" json:"created_at"`
}
type OperationLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	GMName     string    `gorm:"size:64;index" json:"gm_name"`
	Action     string    `gorm:"size:64;index" json:"action"`
	TargetType string    `gorm:"size:32;index" json:"target_type"`
	TargetID   string    `gorm:"size:64;index" json:"target_id"`
	BeforeJSON string    `gorm:"type:text" json:"before_json"`
	AfterJSON  string    `gorm:"type:text" json:"after_json"`
	IP         string    `gorm:"size:64" json:"ip"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}
type GMAccount struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Name        string     `gorm:"size:64;uniqueIndex" json:"name"`
	TokenHash   string     `gorm:"size:255" json:"-"`
	Permissions string     `gorm:"size:1000" json:"permissions"`
	Enabled     bool       `gorm:"index" json:"enabled"`
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
