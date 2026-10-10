package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"caupocket-api/internal/model"
)

// GoalRepository abstraksi akses data tabel saving_goals.
//
// FindForUpdate dan AddProgress WAJIB dipanggil di dalam database transaction.
type GoalRepository interface {
	Create(ctx context.Context, db *gorm.DB, goal *model.SavingGoal) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.SavingGoal, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.SavingGoal, error)
	FindForUpdate(ctx context.Context, tx *gorm.DB, userID, goalID uuid.UUID) (*model.SavingGoal, error)
	AddProgress(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, amount int64) error
}

type goalRepository struct {
	db *gorm.DB
}

func NewGoalRepository(db *gorm.DB) GoalRepository {
	return &goalRepository{db: db}
}

func (r *goalRepository) Create(ctx context.Context, db *gorm.DB, goal *model.SavingGoal) error {
	if goal.ID == uuid.Nil {
		goal.ID = uuid.New()
	}
	return db.WithContext(ctx).Create(goal).Error
}

func (r *goalRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.SavingGoal, error) {
	var goal model.SavingGoal
	err := r.db.WithContext(ctx).First(&goal, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &goal, nil
}

func (r *goalRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.SavingGoal, error) {
	var goals []model.SavingGoal
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("status ASC, created_at DESC").
		Find(&goals).Error
	return goals, err
}

func (r *goalRepository) FindForUpdate(ctx context.Context, tx *gorm.DB, userID, goalID uuid.UUID) (*model.SavingGoal, error) {
	var goal model.SavingGoal
	err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND user_id = ?", goalID, userID).
		First(&goal).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &goal, nil
}

// AddProgress menambah current_amount secara atomik sekaligus mengubah status
// menjadi ACHIEVED bila target tercapai, dalam satu statement UPDATE.
func (r *goalRepository) AddProgress(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, amount int64) error {
	return tx.WithContext(ctx).
		Model(&model.SavingGoal{}).
		Where("id = ?", goalID).
		Updates(map[string]any{
			"current_amount": gorm.Expr("current_amount + ?", amount),
			"status": gorm.Expr(
				"CASE WHEN current_amount + ? >= target_amount THEN ? ELSE status END",
				amount, model.GoalAchieved,
			),
			"updated_at": time.Now(),
		}).Error
}
