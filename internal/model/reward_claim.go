package model

import "time"

// AccountRewardClaim is an account-scoped receipt for one-time operational
// rewards. PlayerID keeps the receipt attached across OpenID migrations, while
// AccountID prevents character deletion and recreation from resetting it.
type AccountRewardClaim struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	AccountID  string    `gorm:"size:64;uniqueIndex:idx_account_reward_claim" json:"account_id"`
	ClaimKey   string    `gorm:"size:64;uniqueIndex:idx_account_reward_claim;uniqueIndex:idx_player_reward_claim" json:"claim_key"`
	PlayerID   uint      `gorm:"uniqueIndex:idx_player_reward_claim" json:"player_id"`
	RewardJSON string    `gorm:"type:text" json:"reward_json"`
	ClaimedAt  time.Time `gorm:"index" json:"claimed_at"`
	CreatedAt  time.Time `json:"created_at"`
}
