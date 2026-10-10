package model

import "github.com/google/uuid"

type CategoryType string

const (
	CategoryIncome  CategoryType = "INCOME"
	CategoryExpense CategoryType = "EXPENSE"
)

type Category struct {
	ID       uuid.UUID    `json:"id" gorm:"type:uuid;primaryKey"`
	UserID   uuid.UUID    `json:"user_id" gorm:"type:uuid;not null;index"`
	Name     string       `json:"name" gorm:"type:varchar(100);not null"`
	Type     CategoryType `json:"type" gorm:"type:varchar(20);not null"`
	IconName string       `json:"icon_name" gorm:"type:varchar(100);not null;default:'tag'"`
	User     *User        `json:"-" gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	Timestamps
}

func (Category) TableName() string { return "categories" }
