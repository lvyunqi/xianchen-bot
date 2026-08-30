package model

import "time"

type Friendship struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PlayerID  uint      `gorm:"uniqueIndex:idx_friend_pair" json:"player_id"`
	FriendID  uint      `gorm:"uniqueIndex:idx_friend_pair" json:"friend_id"`
	Status    string    `gorm:"size:16;index" json:"status"`
	Intimacy  int64     `json:"intimacy"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type SocialMessage struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	SenderID   uint      `gorm:"index" json:"sender_id"`
	ReceiverID uint      `gorm:"index" json:"receiver_id"`
	Type       string    `gorm:"size:16;index" json:"type"`
	Content    string    `gorm:"size:1000" json:"content"`
	Read       bool      `gorm:"index" json:"read"`
	CreatedAt  time.Time `json:"created_at"`
}
type Mentorship struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	MasterID   uint      `gorm:"index" json:"master_id"`
	DiscipleID uint      `gorm:"uniqueIndex" json:"disciple_id"`
	Status     string    `gorm:"size:16" json:"status"`
	Bond       int64     `json:"bond"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
