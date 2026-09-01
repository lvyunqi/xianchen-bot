package model

import "time"

type RankEntry struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Type        string    `gorm:"uniqueIndex:idx_rank_type_player;size:32" json:"type"`
	PlayerID    uint      `gorm:"uniqueIndex:idx_rank_type_player" json:"player_id"`
	PlayerName  string    `gorm:"size:32" json:"player_name"`
	Rank        int       `gorm:"index" json:"rank"`
	Score       int64     `gorm:"index" json:"score"`
	ExtraJSON   string    `gorm:"type:text" json:"extra_json"`
	RefreshedAt time.Time `gorm:"index" json:"refreshed_at"`
}
