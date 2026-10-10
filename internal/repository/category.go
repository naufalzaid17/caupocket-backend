package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"caupocket-api/internal/model"
)

// CategoryRepository abstraksi akses data tabel categories.
type CategoryRepository interface {
	Create(ctx context.Context, db *gorm.DB, category *model.Category) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.Category, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Category, error)
}

type categoryRepository struct {
	db *gorm.DB
}

func NewCategoryRepository(db *gorm.DB) CategoryRepository {
	return &categoryRepository{db: db}
}

func (r *categoryRepository) Create(ctx context.Context, db *gorm.DB, category *model.Category) error {
	if category.ID == uuid.Nil {
		category.ID = uuid.New()
	}
	return db.WithContext(ctx).Create(category).Error
}

func (r *categoryRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Category, error) {
	var category model.Category
	err := r.db.WithContext(ctx).First(&category, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &category, nil
}

func (r *categoryRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Category, error) {
	var categories []model.Category
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("type ASC, name ASC").
		Find(&categories).Error
	return categories, err
}
