package model

import (
	"time"

	"github.com/google/uuid"
)

// HabitSetting konfigurasi fitur auto-nabung per user (EPIC V2.2).
type HabitSetting struct {
	UserID          uuid.UUID `json:"user_id" gorm:"type:uuid;primaryKey"`
	AutoFundEnabled bool      `json:"auto_fund_enabled" gorm:"not null;default:false"`
	Timestamps
}

func (HabitSetting) TableName() string { return "habit_settings" }

// HabitLog catatan reward yang sudah diberikan oleh Habit Engine.
// goal_title di-snapshot agar riwayat tetap akurat walau goal berubah/hapus.
type HabitLog struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	UserID       uuid.UUID `json:"user_id" gorm:"type:uuid;not null;index"`
	GoalID       uuid.UUID `json:"goal_id" gorm:"type:uuid;not null"`
	GoalTitle    string    `json:"goal_title" gorm:"type:varchar(255);not null"`
	StreakDays   int       `json:"streak_days" gorm:"not null"`
	RewardAmount int64     `json:"reward_amount" gorm:"not null"`
	RewardDate   time.Time `json:"reward_date" gorm:"not null"`
	Description  string    `json:"description" gorm:"type:varchar(255);not null;default:''"`
	Timestamps
}

func (HabitLog) TableName() string { return "habit_logs" }
