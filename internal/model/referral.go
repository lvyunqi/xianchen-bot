package model

import "time"

// Referral records are account-scoped so deleting and recreating a character
// cannot reset invitation eligibility or milestone rewards.
type ReferralCode struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	AccountID       string    `gorm:"size:64;uniqueIndex" json:"account_id"`
	CurrentPlayerID uint      `gorm:"index" json:"current_player_id"`
	Code            string    `gorm:"size:24;uniqueIndex" json:"code"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type ReferralBinding struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	InviteeAccountID string    `gorm:"size:64;uniqueIndex" json:"invitee_account_id"`
	InviteePlayerID  uint      `gorm:"index" json:"invitee_player_id"`
	InviterAccountID string    `gorm:"size:64;index" json:"inviter_account_id"`
	InviterPlayerID  uint      `gorm:"index" json:"inviter_player_id"`
	InvitationCode   string    `gorm:"size:24;index" json:"invitation_code"`
	Rewarded         bool      `gorm:"index" json:"rewarded"`
	CreatedAt        time.Time `gorm:"index" json:"created_at"`
}

type ReferralClaim struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AccountID string    `gorm:"size:64;uniqueIndex:idx_referral_claim" json:"account_id"`
	ClaimKey  string    `gorm:"size:64;uniqueIndex:idx_referral_claim" json:"claim_key"`
	CreatedAt time.Time `json:"created_at"`
}
