package model

import (
	"time"

	"github.com/google/uuid"
)

type GoalStatus string

const (
	GoalActive   GoalStatus = "ACTIVE"
	GoalAchieved GoalStatus = "ACHIEVED"
)

// SavingGoal merepresentasikan target tabungan (mis. dana liburan).
type SavingGoal struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	UserID        uuid.UUID  `json:"user_id" gorm:"type:uuid;not null;index"`
	Title         string     `json:"title" gorm:"type:varchar(255);not null"`
	TargetAmount  int64      `json:"target_amount" gorm:"not null"`
	CurrentAmount int64      `json:"current_amount" gorm:"not null;default:0"`
	TargetDate    *time.Time `json:"target_date" gorm:"type:date"`
	Status        GoalStatus `json:"status" gorm:"type:varchar(20);not null;default:'ACTIVE'"`
	User          *User      `json:"-" gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	Timestamps
}

func (SavingGoal) TableName() string { return "saving_goals" }
