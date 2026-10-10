package model

import (
	"time"

	"github.com/google/uuid"
)

type TransactionType string

const (
	TrxIncome      TransactionType = "INCOME"
	TrxExpense     TransactionType = "EXPENSE"
	TrxGoalFunding TransactionType = "GOAL_FUNDING"
)

// Transaction adalah ledger utama. Amount selalu absolut (positif);
// arah dana ditentukan oleh Type.
type Transaction struct {
	ID          uuid.UUID       `json:"id" gorm:"type:uuid;primaryKey"`
	UserID      uuid.UUID       `json:"user_id" gorm:"type:uuid;not null;index"`
	WalletID    uuid.UUID       `json:"wallet_id" gorm:"type:uuid;not null;index"`
	CategoryID  *uuid.UUID      `json:"category_id" gorm:"type:uuid"`
	GoalID      *uuid.UUID      `json:"goal_id" gorm:"type:uuid"`
	Type        TransactionType `json:"type" gorm:"type:varchar(20);not null;index"`
	Amount      int64           `json:"amount" gorm:"not null"`
	Description string          `json:"description" gorm:"type:text;not null;default:''"`
	TrxDate     time.Time       `json:"trx_date" gorm:"not null"`
	Wallet      *Wallet         `json:"wallet,omitempty" gorm:"foreignKey:WalletID;constraint:OnDelete:CASCADE"`
	Category    *Category       `json:"category,omitempty" gorm:"foreignKey:CategoryID;constraint:OnDelete:SET NULL"`
	Goal        *SavingGoal     `json:"goal,omitempty" gorm:"foreignKey:GoalID;constraint:OnDelete:SET NULL"`
	Timestamps
}

func (Transaction) TableName() string { return "transactions" }
