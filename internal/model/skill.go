package model

import "time"

type Skill struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Name          string    `gorm:"size:64;uniqueIndex" json:"name"`
	Type          string    `gorm:"size:32;index" json:"type"`
	Rarity        string    `gorm:"size:32;index" json:"rarity"`
	RealmRequired string    `gorm:"size:32" json:"realm_required"`
	Description   string    `gorm:"size:1000" json:"description"`
	EffectJSON    string    `gorm:"type:text" json:"effect_json"`
	UpgradeJSON   string    `gorm:"type:text" json:"upgrade_json"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
type PlayerSkill struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PlayerID  uint      `gorm:"uniqueIndex:idx_player_skill" json:"player_id"`
	SkillID   uint      `gorm:"uniqueIndex:idx_player_skill" json:"skill_id"`
	Level     int       `json:"level"`
	Mastery   int64     `json:"mastery"`
	Equipped  bool      `json:"equipped"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SkillPublication keeps player-created skills private until their creator
// explicitly publishes them to the shared skill library.
type SkillPublication struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	SkillID         uint       `gorm:"uniqueIndex;not null" json:"skill_id"`
	CreatorPlayerID uint       `gorm:"index;not null" json:"creator_player_id"`
	CreatorName     string     `gorm:"size:32;index" json:"creator_name"`
	Published       bool       `gorm:"index" json:"published"`
	PublishedAt     *time.Time `gorm:"index" json:"published_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
