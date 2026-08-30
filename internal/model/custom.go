package model

import "time"

// PlayerValue stores feature-specific state without forcing every configurable
// gameplay system into the core player row.
type PlayerValue struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	PlayerID  uint       `gorm:"uniqueIndex:idx_player_value;index" json:"player_id"`
	Key       string     `gorm:"size:64;uniqueIndex:idx_player_value" json:"key"`
	Value     string     `gorm:"type:text" json:"value"`
	ExpiresAt *time.Time `gorm:"index" json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type PetTemplate struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	Code               string    `gorm:"size:64;uniqueIndex" json:"code"`
	Name               string    `gorm:"size:64;uniqueIndex" json:"name"`
	InitialPower       int64     `json:"initial_power"`
	GrowthPerLevel     int64     `json:"growth_per_level"`
	LoyaltyDecay       int       `json:"loyalty_decay"`
	EvolutionCondition string    `gorm:"type:text" json:"evolution_condition"`
	EvolutionTarget    string    `gorm:"size:64" json:"evolution_target"`
	Enabled            bool      `gorm:"index" json:"enabled"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type Dungeon struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	Code             string    `gorm:"size:64;uniqueIndex" json:"code"`
	Name             string    `gorm:"size:64;uniqueIndex" json:"name"`
	Difficulty       string    `gorm:"size:16;index" json:"difficulty"`
	RecommendedPower int64     `json:"recommended_power"`
	StaminaCost      int       `json:"stamina_cost"`
	RewardPoolJSON   string    `gorm:"type:text" json:"reward_pool_json"`
	DailyLimit       int       `json:"daily_limit"`
	Enabled          bool      `gorm:"index" json:"enabled"`
	ImageURL         string    `gorm:"size:500" json:"image_url"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Title struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Code           string    `gorm:"size:64;uniqueIndex" json:"code"`
	Name           string    `gorm:"size:64;uniqueIndex" json:"name"`
	Condition      string    `gorm:"size:255" json:"condition"`
	AttributeBonus string    `gorm:"type:text" json:"attribute_bonus"`
	Type           string    `gorm:"size:32;index" json:"type"`
	Enabled        bool      `gorm:"index" json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Activity struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Code       string    `gorm:"size:64;uniqueIndex" json:"code"`
	Name       string    `gorm:"size:64;index" json:"name"`
	Type       string    `gorm:"size:32;index" json:"type"`
	StartsAt   time.Time `gorm:"index" json:"starts_at"`
	EndsAt     time.Time `gorm:"index" json:"ends_at"`
	Effect     string    `gorm:"size:255" json:"effect"`
	EffectJSON string    `gorm:"type:text" json:"effect_json"`
	Status     string    `gorm:"size:16;index" json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Mail struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	Code       string     `gorm:"size:64;uniqueIndex" json:"code"`
	Title      string     `gorm:"size:100" json:"title"`
	Content    string     `gorm:"type:text" json:"content"`
	Sender     string     `gorm:"size:64" json:"sender"`
	RewardJSON string     `gorm:"type:text" json:"reward_json"`
	TargetType string     `gorm:"size:16;index" json:"target_type"`
	TargetID   string     `gorm:"size:64;index" json:"target_id"`
	Sent       bool       `gorm:"index" json:"sent"`
	SentAt     *time.Time `json:"sent_at"`
	ExpiresAt  *time.Time `gorm:"index" json:"expires_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type CheckinReward struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Day           int       `gorm:"uniqueIndex" json:"day"`
	ItemName      string    `gorm:"size:64" json:"item_name"`
	Quantity      int64     `json:"quantity"`
	SpecialReward string    `gorm:"size:255" json:"special_reward"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ShopEntry struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Code          string    `gorm:"size:64;uniqueIndex" json:"code"`
	ItemID        uint      `gorm:"index" json:"item_id"`
	ItemName      string    `gorm:"size:64;index" json:"item_name"`
	Currency      string    `gorm:"size:16;index" json:"currency"`
	Price         int64     `json:"price"`
	PurchaseLimit int       `json:"purchase_limit"`
	RefreshCycle  string    `gorm:"size:16" json:"refresh_cycle"`
	Sort          int       `json:"sort"`
	Enabled       bool      `gorm:"index" json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type RedemptionCode struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	Code       string     `gorm:"size:64;uniqueIndex" json:"code"`
	RewardJSON string     `gorm:"type:text" json:"reward_json"`
	MaxUses    int        `json:"max_uses"`
	UsedCount  int        `json:"used_count"`
	ExpiresAt  *time.Time `gorm:"index" json:"expires_at"`
	Status     string     `gorm:"size:16;index" json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type Notice struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Code        string     `gorm:"size:64;uniqueIndex" json:"code"`
	Title       string     `gorm:"size:100" json:"title"`
	Content     string     `gorm:"type:text" json:"content"`
	Type        string     `gorm:"size:16;index" json:"type"`
	Pinned      bool       `gorm:"index" json:"pinned"`
	Published   bool       `gorm:"index" json:"published"`
	PublishedAt *time.Time `gorm:"index" json:"published_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Sect struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:64;uniqueIndex" json:"name"`
	OwnerID     uint      `gorm:"uniqueIndex" json:"owner_id"`
	Level       int       `json:"level"`
	Funds       int64     `json:"funds"`
	Reputation  int64     `json:"reputation"`
	MemberLimit int       `json:"member_limit"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SectMember struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SectID       uint      `gorm:"index" json:"sect_id"`
	PlayerID     uint      `gorm:"uniqueIndex" json:"player_id"`
	Position     string    `gorm:"size:16" json:"position"`
	Contribution int64     `gorm:"index" json:"contribution"`
	JoinedAt     time.Time `json:"joined_at"`
}

type AlchemyRecipe struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Code          string    `gorm:"size:64;uniqueIndex" json:"code"`
	Name          string    `gorm:"size:64;uniqueIndex" json:"name"`
	MaterialsJSON string    `gorm:"type:text" json:"materials_json"`
	OutputItemID  uint      `json:"output_item_id"`
	OutputName    string    `gorm:"size:64" json:"output_name"`
	SuccessRate   float64   `json:"success_rate"`
	Description   string    `gorm:"size:500" json:"description"`
	Enabled       bool      `gorm:"index" json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ArtifactTemplate struct {
	ID                   uint      `gorm:"primaryKey" json:"id"`
	Code                 string    `gorm:"size:64;uniqueIndex" json:"code"`
	Name                 string    `gorm:"size:64;uniqueIndex" json:"name"`
	Type                 string    `gorm:"size:32;index" json:"type"`
	Slot                 string    `gorm:"size:32;index" json:"slot"`
	Archetype            string    `gorm:"size:32;index" json:"archetype"`
	Positioning          string    `gorm:"size:64" json:"positioning"`
	SetName              string    `gorm:"size:64;index" json:"set_name"`
	SetBonusJSON         string    `gorm:"type:text" json:"set_bonus_json"`
	MaterialsJSON        string    `gorm:"type:text" json:"materials_json"`
	AttributeJSON        string    `gorm:"type:text" json:"attribute_json"`
	MinimumRealmSequence int       `gorm:"index" json:"minimum_realm_sequence"`
	MinimumRealmLevel    int       `json:"minimum_realm_level"`
	MinimumCombatPower   int64     `json:"minimum_combat_power"`
	Description          string    `gorm:"size:1000" json:"description"`
	SourceJSON           string    `gorm:"type:text" json:"source_json"`
	MaxLevel             int       `json:"max_level"`
	Enabled              bool      `gorm:"index" json:"enabled"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type PlayerArtifact struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	PlayerID    uint      `gorm:"index" json:"player_id"`
	TemplateID  uint      `gorm:"index" json:"template_id"`
	Name        string    `gorm:"size:64" json:"name"`
	Level       int       `json:"level"`
	Quality     string    `gorm:"size:16" json:"quality"`
	Slot        string    `gorm:"size:32;index" json:"slot"`
	ForgeLevel  int       `json:"forge_level"`
	Inscription string    `gorm:"size:32" json:"inscription"`
	StarLevel   int       `json:"star_level"`
	SocketCount int       `json:"socket_count"`
	SocketJSON  string    `gorm:"type:text" json:"socket_json"`
	Activated   bool      `gorm:"index" json:"activated"`
	InSmelter   bool      `gorm:"index" json:"in_smelter"`
	Equipped    bool      `gorm:"index" json:"equipped"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type DungeonRun struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	PlayerID   uint      `gorm:"index" json:"player_id"`
	DungeonID  uint      `gorm:"index" json:"dungeon_id"`
	RunDate    string    `gorm:"size:10;index" json:"run_date"`
	DurationMS int64     `json:"duration_ms"`
	Score      int64     `gorm:"index" json:"score"`
	Success    bool      `gorm:"index" json:"success"`
	CreatedAt  time.Time `json:"created_at"`
}

type ArenaRecord struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PlayerID  uint      `gorm:"uniqueIndex" json:"player_id"`
	Rating    int64     `gorm:"index" json:"rating"`
	Wins      int64     `json:"wins"`
	Losses    int64     `json:"losses"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ContentReview struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	Type           string     `gorm:"size:16;index" json:"type"`
	PlayerID       uint       `gorm:"index" json:"player_id"`
	PlayerName     string     `gorm:"size:64;index" json:"player_name"`
	Content        string     `gorm:"type:text" json:"content"`
	Status         string     `gorm:"size:16;index;default:待审核" json:"status"`
	Reason         string     `gorm:"size:255" json:"reason"`
	Diagnosis      string     `gorm:"type:text" json:"diagnosis"`
	ResolutionType string     `gorm:"size:32;index" json:"resolution_type"`
	Resolution     string     `gorm:"type:text" json:"resolution"`
	ReviewedAt     *time.Time `gorm:"index" json:"reviewed_at"`
	ResolvedAt     *time.Time `gorm:"index" json:"resolved_at"`
	CreatedAt      time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type SensitiveWord struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Word        string    `gorm:"size:128;uniqueIndex" json:"word"`
	Replacement string    `gorm:"size:128" json:"replacement"`
	Enabled     bool      `gorm:"index" json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SlowQueryLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	SQL        string    `gorm:"type:text" json:"sql"`
	DurationMS int64     `gorm:"index" json:"duration_ms"`
	Source     string    `gorm:"size:64;index" json:"source"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

type ManagerAccount struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      string    `gorm:"size:64;uniqueIndex" json:"user_id"`
	Name        string    `gorm:"size:64" json:"name"`
	Role        string    `gorm:"size:16;index" json:"role"`
	Permissions string    `gorm:"type:text" json:"permissions"`
	Enabled     bool      `gorm:"index" json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
