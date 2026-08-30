package model

import "time"

type Pet struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	PlayerID   uint      `gorm:"index" json:"player_id"`
	Name       string    `gorm:"size:64;index" json:"name"`
	Species    string    `gorm:"size:64;index" json:"species"`
	Rarity     string    `gorm:"size:32" json:"rarity"`
	Level      int       `json:"level"`
	Experience int64     `json:"experience"`
	Attack     int64     `json:"attack"`
	Defense    int64     `json:"defense"`
	Health     int64     `json:"health"`
	Loyalty    int       `json:"loyalty"`
	Evolution  int       `json:"evolution"`
	Active     bool      `gorm:"index" json:"active"`
	SkillJSON  string    `gorm:"type:text" json:"skill_json"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
