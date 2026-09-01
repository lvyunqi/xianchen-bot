package model

import "time"

type ItemCategory struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:32;uniqueIndex" json:"name"`
	Description string    `gorm:"size:255" json:"description"`
	Sort        int       `json:"sort"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type Rarity struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Name            string    `gorm:"size:32;uniqueIndex" json:"name"`
	Level           int       `gorm:"index" json:"level"`
	ValueMultiplier float64   `json:"value_multiplier"`
	DropWeight      int       `json:"drop_weight"`
	Color           string    `gorm:"size:16" json:"color"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
type Item struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Code         string    `gorm:"size:64;uniqueIndex" json:"code"`
	Name         string    `gorm:"size:64;uniqueIndex" json:"name"`
	CategoryID   uint      `gorm:"index" json:"category_id"`
	CategoryName string    `gorm:"size:32;index" json:"category_name"`
	RarityID     uint      `gorm:"index" json:"rarity_id"`
	RarityName   string    `gorm:"size:32;index" json:"rarity_name"`
	Description  string    `gorm:"size:500" json:"description"`
	EffectType   string    `gorm:"size:32" json:"effect_type"`
	EffectFunc   string    `gorm:"size:64" json:"effect_func"`
	EffectParams string    `gorm:"type:text" json:"effect_params"`
	EffectValue  float64   `json:"effect_value"`
	BaseValue    int64     `gorm:"index" json:"base_value"`
	StackLimit   int64     `json:"stack_limit"`
	Stackable    bool      `json:"stackable"`
	Tradable     bool      `json:"tradable"`
	StoreEnabled bool      `gorm:"index" json:"store_enabled"`
	StorePrice   int64     `json:"store_price"`
	ImageURL     string    `gorm:"size:500" json:"image_url"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type DropPool struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"size:64;uniqueIndex" json:"name"`
	SourceType string    `gorm:"size:32;index" json:"source_type"`
	SourceName string    `gorm:"size:64" json:"source_name"`
	Enabled    bool      `gorm:"index" json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
type DropEntry struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	DropPoolID uint      `gorm:"index" json:"drop_pool_id"`
	ItemID     uint      `gorm:"index" json:"item_id"`
	ItemName   string    `gorm:"size:64" json:"item_name"`
	Weight     int       `json:"weight"`
	Minimum    int64     `json:"minimum"`
	Maximum    int64     `json:"maximum"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
