package model

import "time"

// WorldLocation is an editable node in the game's travel graph.
type WorldLocation struct {
	ID                   uint      `gorm:"primaryKey" json:"id"`
	Code                 string    `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Name                 string    `gorm:"size:64;uniqueIndex;not null" json:"name"`
	Region               string    `gorm:"size:32;index" json:"region"`
	Description          string    `gorm:"size:500" json:"description"`
	ImageURL             string    `gorm:"size:500" json:"image_url"`
	NPCJSON              string    `gorm:"type:text" json:"npc_json"`
	TasksJSON            string    `gorm:"type:text" json:"tasks_json"`
	ResourceName         string    `gorm:"size:64" json:"resource_name"`
	ResourceQuantity     int       `json:"resource_quantity"`
	ResourceCooldownMin  int       `json:"resource_cooldown_minutes"`
	TeleportEnabled      bool      `json:"teleport_enabled"`
	CrossRegionEnabled   bool      `json:"cross_region_enabled"`
	MinimumRealmSequence int       `gorm:"index" json:"minimum_realm_sequence"`
	MinimumRealmLevel    int       `json:"minimum_realm_level"`
	MinimumLevel         int       `json:"minimum_level"`
	StaminaCost          int64     `json:"stamina_cost"`
	MonsterName          string    `gorm:"size:64" json:"monster_name"`
	MonsterPower         int64     `json:"monster_power"`
	MonsterEncounterRate float64   `json:"monster_encounter_rate"`
	MonsterRewardJSON    string    `gorm:"type:text" json:"monster_reward_json"`
	BossName             string    `gorm:"size:64" json:"boss_name"`
	BossPower            int64     `json:"boss_power"`
	BossRewardJSON       string    `gorm:"type:text" json:"boss_reward_json"`
	BossCooldownMinutes  int       `json:"boss_cooldown_minutes"`
	NeighborsJSON        string    `gorm:"type:text" json:"neighbors_json"`
	Enabled              bool      `gorm:"index" json:"enabled"`
	SortOrder            int       `gorm:"index" json:"sort_order"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}
