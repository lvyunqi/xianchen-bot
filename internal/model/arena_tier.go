package model

import "time"

type ArenaTier struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Code          string    `gorm:"size:64;uniqueIndex" json:"code"`
	Name          string    `gorm:"size:64;uniqueIndex" json:"name"`
	Sequence      int       `gorm:"uniqueIndex" json:"sequence"`
	MinimumRating int64     `gorm:"index" json:"minimum_rating"`
	DailyCoin     int64     `json:"daily_coin"`
	DailySilver   int64     `json:"daily_silver"`
	Description   string    `gorm:"size:500" json:"description"`
	Enabled       bool      `gorm:"index" json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
