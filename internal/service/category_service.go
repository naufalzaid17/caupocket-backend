package service

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"caupocket-api/internal/model"
	"caupocket-api/internal/repository"
)

// CreateCategoryInput payload kategori baru.
type CreateCategoryInput struct {
	Name     string
	Type     model.CategoryType
	IconName string
}

// CategoryService business logic kategori.
type CategoryService interface {
	Create(ctx context.Context, userID uuid.UUID, input CreateCategoryInput) (*model.Category, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Category, error)
}

type categoryService struct {
	db         *gorm.DB
	categories repository.CategoryRepository
}

func NewCategoryService(db *gorm.DB, categories repository.CategoryRepository) CategoryService {
	return &categoryService{db: db, categories: categories}
}

func (s *categoryService) Create(ctx context.Context, userID uuid.UUID, input CreateCategoryInput) (*model.Category, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return nil, ErrInvalidInput
	}
	if input.Type != model.CategoryIncome && input.Type != model.CategoryExpense {
		return nil, ErrInvalidInput
	}

	category := &model.Category{
		ID:       uuid.New(),
		UserID:   userID,
		Name:     input.Name,
		Type:     input.Type,
		IconName: input.IconName,
	}
	if err := s.categories.Create(ctx, s.db, category); err != nil {
		return nil, err
	}
	return category, nil
}

func (s *categoryService) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Category, error) {
	return s.categories.ListByUser(ctx, userID)
}
