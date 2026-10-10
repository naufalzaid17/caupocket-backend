package model

import "github.com/google/uuid"

// Wallet menyimpan cache saldo (BIGINT, satuan rupiah utuh) per pengguna.
type Wallet struct {
	ID      uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	UserID  uuid.UUID `json:"user_id" gorm:"type:uuid;not null;index"`
	Name    string    `json:"name" gorm:"type:varchar(100);not null"`
	Balance int64     `json:"balance" gorm:"not null;default:0"`
	User    *User     `json:"-" gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	Timestamps
}

func (Wallet) TableName() string { return "wallets" }
