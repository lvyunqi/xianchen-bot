package model

import "time"

// PlayerExtendedProgress is the durable, player-facing state behind the
// configurable cultivation systems. Configuration rows describe what exists;
// this table records what a player has actually discovered, learned or raised.
type PlayerExtendedProgress struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	PlayerID     uint       `gorm:"uniqueIndex:idx_player_extended_progress;index" json:"player_id"`
	System       string     `gorm:"size:32;uniqueIndex:idx_player_extended_progress;index" json:"system"`
	ConfigCode   string     `gorm:"size:64;uniqueIndex:idx_player_extended_progress;index" json:"config_code"`
	ConfigName   string     `gorm:"size:64;index" json:"config_name"`
	State        string     `gorm:"size:24;index" json:"state"`
	Level        int        `gorm:"index" json:"level"`
	Experience   int64      `json:"experience"`
	Mastery      int64      `json:"mastery"`
	Uses         int64      `json:"uses"`
	Quantity     int64      `json:"quantity"`
	Power        int64      `json:"power"`
	ReadyAt      *time.Time `gorm:"index" json:"ready_at"`
	ActiveUntil  *time.Time `gorm:"index" json:"active_until"`
	MetadataJSON string     `gorm:"type:text" json:"metadata_json"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
