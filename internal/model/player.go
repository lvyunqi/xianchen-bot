package model

import "time"

type Player struct {
	ID                   uint       `gorm:"primaryKey" json:"id"`
	AccountID            string     `gorm:"size:64;uniqueIndex;not null" json:"account_id"`
	DaoName              string     `gorm:"size:32;uniqueIndex;not null" json:"dao_name"`
	Gender               string     `gorm:"size:8" json:"gender"`
	ServerName           string     `gorm:"size:32;index" json:"server_name"`
	RealmID              uint       `gorm:"index" json:"realm_id"`
	RealmName            string     `gorm:"size:32;index" json:"realm_name"`
	RealmLevel           int        `json:"realm_level"`
	Cultivation          int64      `gorm:"index" json:"cultivation"`
	CultivationRequired  int64      `json:"cultivation_required"`
	SpiritualRoot        string     `gorm:"size:32;index" json:"spiritual_root"`
	RootQuality          int        `json:"root_quality"`
	Level                int        `gorm:"index" json:"level"`
	Experience           int64      `json:"experience"`
	Health               int64      `json:"health"`
	MaxHealth            int64      `json:"max_health"`
	Mana                 int64      `json:"mana"`
	MaxMana              int64      `json:"max_mana"`
	PhysicalAttack       int64      `json:"physical_attack"`
	MagicAttack          int64      `json:"magic_attack"`
	PhysicalDefense      int64      `json:"physical_defense"`
	MagicDefense         int64      `json:"magic_defense"`
	Agility              int64      `json:"agility"`
	Strength             int64      `json:"strength"`
	Constitution         int64      `json:"constitution"`
	Spirit               int64      `json:"spirit"`
	Perception           int64      `json:"perception"`
	Willpower            int64      `json:"willpower"`
	Luck                 int64      `json:"luck"`
	CritRate             float64    `json:"crit_rate"`
	CritDamage           float64    `json:"crit_damage"`
	DodgeRate            float64    `json:"dodge_rate"`
	DamageReduction      float64    `json:"damage_reduction"`
	CombatPower          int64      `gorm:"index" json:"combat_power"`
	Lifespan             int64      `json:"lifespan"`
	MaxLifespan          int64      `json:"max_lifespan"`
	Age                  int64      `json:"age"`
	SpiritStones         int64      `json:"spirit_stones"`
	SilverCoins          int64      `gorm:"index" json:"silver_coins"`
	ImmortalJade         int64      `gorm:"index" json:"immortal_jade"`
	ArenaCoins           int64      `json:"arena_coins"`
	Merit                int64      `gorm:"index" json:"merit"`
	Reputation           int64      `json:"reputation"`
	DaoHeart             int64      `json:"dao_heart"`
	ImmortalAffinity     int64      `json:"immortal_affinity"`
	SectName             string     `gorm:"size:64;index" json:"sect_name"`
	TeamName             string     `gorm:"size:64" json:"team_name"`
	Location             string     `gorm:"size:64" json:"location"`
	AvatarURL            string     `gorm:"size:500" json:"avatar_url"`
	Title                string     `gorm:"size:64" json:"title"`
	CurrentSkillID       uint       `json:"current_skill_id"`
	ActivePetID          uint       `json:"active_pet_id"`
	MansionID            uint       `json:"mansion_id"`
	CoupleID             uint       `gorm:"index" json:"couple_id"`
	State                string     `gorm:"size:16;index" json:"state"`
	Online               bool       `gorm:"index" json:"online"`
	Banned               bool       `gorm:"index" json:"banned"`
	BanReason            string     `gorm:"size:255" json:"ban_reason"`
	DailyTaskDate        string     `gorm:"size:10;index" json:"daily_task_date"`
	LastLoginAt          *time.Time `json:"last_login_at"`
	CultivationStartedAt *time.Time `json:"cultivation_started_at"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	DeletedAt            *time.Time `gorm:"index" json:"-"`
}

type PlayerItem struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PlayerID  uint      `gorm:"uniqueIndex:idx_player_item" json:"player_id"`
	ItemID    uint      `gorm:"uniqueIndex:idx_player_item" json:"item_id"`
	Quantity  int64     `json:"quantity"`
	Bound     bool      `json:"bound"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
