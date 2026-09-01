package model

import "time"

// BankAccount keeps spendable currency separate from safeguarded deposits and
// credit debt. Immortal jade is intentionally excluded from lending.
type BankAccount struct {
	ID                   uint       `gorm:"primaryKey" json:"id"`
	PlayerID             uint       `gorm:"uniqueIndex" json:"player_id"`
	SilverDeposit        int64      `json:"silver_deposit"`
	SpiritStoneDeposit   int64      `json:"spirit_stone_deposit"`
	SilverPrincipal      int64      `json:"silver_principal"`
	SilverInterest       int64      `json:"silver_interest"`
	CreditScore          int        `gorm:"index" json:"credit_score"`
	LoanDueAt            *time.Time `gorm:"index" json:"loan_due_at"`
	InterestCalculatedAt *time.Time `json:"interest_calculated_at"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type BankTransaction struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	PlayerID     uint      `gorm:"index" json:"player_id"`
	Type         string    `gorm:"size:16;index" json:"type"`
	Currency     string    `gorm:"size:16;index" json:"currency"`
	Amount       int64     `json:"amount"`
	BalanceAfter int64     `json:"balance_after"`
	DebtAfter    int64     `json:"debt_after"`
	Description  string    `gorm:"size:255" json:"description"`
	CreatedAt    time.Time `gorm:"index" json:"created_at"`
}
