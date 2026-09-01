package model

import "time"

type Couple struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	PlayerAID         uint       `gorm:"uniqueIndex;not null" json:"player_a_id"`
	PlayerBID         uint       `gorm:"uniqueIndex;not null" json:"player_b_id"`
	PlayerAName       string     `gorm:"size:32;index" json:"player_a_name"`
	PlayerBName       string     `gorm:"size:32;index" json:"player_b_name"`
	Affinity          int64      `gorm:"index" json:"affinity"`
	BondLevel         int        `gorm:"index" json:"bond_level"`
	CultivationBonus  float64    `json:"cultivation_bonus"`
	JointAttackBonus  float64    `json:"joint_attack_bonus"`
	InteractionCount  int64      `json:"interaction_count"`
	GiftCount         int64      `json:"gift_count"`
	JointBattleCount  int64      `json:"joint_battle_count"`
	Status            string     `gorm:"size:16;index" json:"status"`
	BondedAt          time.Time  `json:"bonded_at"`
	LastInteractionAt *time.Time `json:"last_interaction_at"`
	DissolvedAt       *time.Time `json:"dissolved_at"`
	Notes             string     `gorm:"size:500" json:"notes"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
