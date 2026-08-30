package model

import "time"

type TradeListing struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	SellerID   uint      `gorm:"index" json:"seller_id"`
	SellerName string    `gorm:"size:32" json:"seller_name"`
	ItemID     uint      `gorm:"index" json:"item_id"`
	ItemName   string    `gorm:"size:64" json:"item_name"`
	Quantity   int64     `json:"quantity"`
	UnitPrice  int64     `json:"unit_price"`
	Status     string    `gorm:"size:16;index" json:"status"`
	BuyerID    uint      `gorm:"index" json:"buyer_id"`
	ExpiresAt  time.Time `gorm:"index" json:"expires_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
type TradeRecord struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ListingID  uint      `gorm:"index" json:"listing_id"`
	SellerID   uint      `gorm:"index" json:"seller_id"`
	BuyerID    uint      `gorm:"index" json:"buyer_id"`
	ItemID     uint      `json:"item_id"`
	Quantity   int64     `json:"quantity"`
	TotalPrice int64     `json:"total_price"`
	CreatedAt  time.Time `json:"created_at"`
}

type BarterRequest struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	InitiatorID       uint       `gorm:"index" json:"initiator_id"`
	InitiatorName     string     `gorm:"size:32" json:"initiator_name"`
	RecipientID       uint       `gorm:"index" json:"recipient_id"`
	RecipientName     string     `gorm:"size:32" json:"recipient_name"`
	OfferedItemID     uint       `gorm:"index" json:"offered_item_id"`
	OfferedItemName   string     `gorm:"size:64" json:"offered_item_name"`
	OfferedQuantity   int64      `json:"offered_quantity"`
	RequestedItemID   uint       `gorm:"index" json:"requested_item_id"`
	RequestedItemName string     `gorm:"size:64" json:"requested_item_name"`
	RequestedQuantity int64      `json:"requested_quantity"`
	Status            string     `gorm:"size:16;index" json:"status"`
	ExpiresAt         time.Time  `gorm:"index" json:"expires_at"`
	RespondedAt       *time.Time `json:"responded_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
