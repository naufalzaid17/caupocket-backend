package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"caupocket-api/internal/model"
)

// HabitSettingRepository pengaturan auto-nabung per user.
type HabitSettingRepository interface {
	// FindOrCreate mengembalikan setting user; membuat default (disabled) bila belum ada.
	FindOrCreate(ctx context.Context, db *gorm.DB, userID uuid.UUID) (*model.HabitSetting, error)
	Update(ctx context.Context, db *gorm.DB, setting *model.HabitSetting) error
}

type habitSettingRepository struct{ db *gorm.DB }

func NewHabitSettingRepository(db *gorm.DB) HabitSettingRepository {
	return &habitSettingRepository{db: db}
}

func (r *habitSettingRepository) FindOrCreate(ctx context.Context, db *gorm.DB, userID uuid.UUID) (*model.HabitSetting, error) {
	var setting model.HabitSetting
	err := db.WithContext(ctx).First(&setting, "user_id = ?", userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		setting = model.HabitSetting{UserID: userID, AutoFundEnabled: false}
		if err := db.WithContext(ctx).Create(&setting).Error; err != nil {
			return nil, err
		}
		return &setting, nil
	}
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

func (r *habitSettingRepository) Update(ctx context.Context, db *gorm.DB, setting *model.HabitSetting) error {
	return db.WithContext(ctx).Save(setting).Error
}

// HabitLogRepository riwayat reward habit engine.
type HabitLogRepository interface {
	Create(ctx context.Context, db *gorm.DB, logEntry *model.HabitLog) error
	ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]model.HabitLog, error)
	// ExistsSince mendeteksi reward yang sudah diberikan pada rentang tanggal
	// (guard agar satu streak hanya direward sekali).
	ExistsSince(ctx context.Context, userID uuid.UUID, since time.Time) (bool, error)
}

type habitLogRepository struct{ db *gorm.DB }

func NewHabitLogRepository(db *gorm.DB) HabitLogRepository {
	return &habitLogRepository{db: db}
}

func (r *habitLogRepository) Create(ctx context.Context, db *gorm.DB, logEntry *model.HabitLog) error {
	if logEntry.ID == uuid.Nil {
		logEntry.ID = uuid.New()
	}
	return db.WithContext(ctx).Create(logEntry).Error
}

func (r *habitLogRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]model.HabitLog, error) {
	var logs []model.HabitLog
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("reward_date DESC, created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

func (r *habitLogRepository) ExistsSince(ctx context.Context, userID uuid.UUID, since time.Time) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.HabitLog{}).
		Where("user_id = ? AND reward_date >= ?", userID, since).
		Count(&count).Error
	return count > 0, err
}
