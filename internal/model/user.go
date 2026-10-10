package model

import "github.com/google/uuid"

type User struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	Email        string    `json:"email" gorm:"type:varchar(255);uniqueIndex;not null"`
	PasswordHash string    `json:"-" gorm:"type:varchar(255);not null"`
	Name         string    `json:"name" gorm:"type:varchar(255);not null"`
	Timestamps
}

func (User) TableName() string { return "users" }
