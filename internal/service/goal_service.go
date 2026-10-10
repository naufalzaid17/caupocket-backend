package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"caupocket-api/internal/model"
	"caupocket-api/internal/repository"
)

// CreateGoalInput payload pembuatan target tabungan baru.
type CreateGoalInput struct {
	Title        string
	TargetAmount int64
	TargetDate   *time.Time
}

// GoalService business logic pengelolaan saving goals (non-ledger).
type GoalService interface {
	Create(ctx context.Context, userID uuid.UUID, input CreateGoalInput) (*model.SavingGoal, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.SavingGoal, error)
}

type goalService struct {
	db    *gorm.DB
	goals repository.GoalRepository
}

func NewGoalService(db *gorm.DB, goals repository.GoalRepository) GoalService {
	return &goalService{db: db, goals: goals}
}

func (s *goalService) Create(ctx context.Context, userID uuid.UUID, input CreateGoalInput) (*model.SavingGoal, error) {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return nil, ErrInvalidInput
	}
	if input.TargetAmount <= 0 {
		return nil, ErrInvalidAmount
	}

	goal := &model.SavingGoal{
		ID:            uuid.New(),
		UserID:        userID,
		Title:         input.Title,
		TargetAmount:  input.TargetAmount,
		CurrentAmount: 0,
		TargetDate:    input.TargetDate,
		Status:        model.GoalActive,
	}
	if err := s.goals.Create(ctx, s.db, goal); err != nil {
		return nil, err
	}
	return goal, nil
}

func (s *goalService) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.SavingGoal, error) {
	return s.goals.ListByUser(ctx, userID)
}
