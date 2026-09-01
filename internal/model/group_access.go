package model

import "time"

type GroupAccessRequest struct {
	ID                 uint       `gorm:"primaryKey" json:"id"`
	GroupID            string     `gorm:"size:64;uniqueIndex;not null" json:"group_id"`
	GroupName          string     `gorm:"size:128;index" json:"group_name"`
	ApplicantAccountID string     `gorm:"size:64;index" json:"applicant_account_id"`
	ApplicantPlayerID  uint       `gorm:"index" json:"applicant_player_id"`
	ApplicantName      string     `gorm:"size:64;index" json:"applicant_name"`
	Purpose            string     `gorm:"size:500" json:"purpose"`
	Status             string     `gorm:"size:16;index;not null" json:"status"`
	ReviewReason       string     `gorm:"size:255" json:"review_reason"`
	ReviewedBy         string     `gorm:"size:64;index" json:"reviewed_by"`
	ReviewedAt         *time.Time `gorm:"index" json:"reviewed_at"`
	CreatedAt          time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

