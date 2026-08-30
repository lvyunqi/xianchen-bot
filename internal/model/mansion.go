package model

import "time"

type Mansion struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	PlayerID         uint      `gorm:"uniqueIndex" json:"player_id"`
	Name             string    `gorm:"size:64" json:"name"`
	Level            int       `json:"level"`
	FarmLevel        int       `json:"farm_level"`
	AlchemyRoomLevel int       `json:"alchemy_room_level"`
	FormationLevel   int       `json:"formation_level"`
	BeastRoomLevel   int       `json:"beast_room_level"`
	WarehouseLevel   int       `json:"warehouse_level"`
	Prosperity       int64     `json:"prosperity"`
	LayoutJSON       string    `gorm:"type:text" json:"layout_json"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
type MansionCrop struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	MansionID          uint      `gorm:"index" json:"mansion_id"`
	ItemID             uint      `json:"item_id"`
	SeedName           string    `gorm:"size:64" json:"seed_name"`
	Plot               int       `json:"plot"`
	Quantity           int64     `json:"quantity"`
	Watered            bool      `json:"watered"`
	Weeded             bool      `json:"weeded"`
	PestFree           bool      `json:"pest_free"`
	Protected          bool      `json:"protected"`
	Fertilized         bool      `gorm:"index" json:"fertilized"`
	FertilizerName     string    `gorm:"size:64" json:"fertilizer_name"`
	DisasterResistance int       `json:"disaster_resistance"`
	Stolen             int64     `json:"stolen"`
	PlantedAt          time.Time `json:"planted_at"`
	ReadyAt            time.Time `json:"ready_at"`
	Harvested          bool      `json:"harvested"`
}
