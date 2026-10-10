package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"caupocket-api/internal/model"
)

// TransactionRepository abstraksi akses data tabel transactions (ledger).
//
// Create WAJIB dipanggil di dalam database transaction agar insert ledger dan
// mutasi saldo tidak pernah terpisah.
type TransactionRepository interface {
	Create(ctx context.Context, tx *gorm.DB, trx *model.Transaction) error
	ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]model.Transaction, error)
}

type transactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepository{db: db}
}

func (r *transactionRepository) Create(ctx context.Context, tx *gorm.DB, trx *model.Transaction) error {
	if trx.ID == uuid.Nil {
		trx.ID = uuid.New()
	}
	return tx.WithContext(ctx).Create(trx).Error
}

func (r *transactionRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]model.Transaction, error) {
	var transactions []model.Transaction
	err := r.db.WithContext(ctx).
		Preload("Wallet").
		Preload("Category").
		Preload("Goal").
		Where("user_id = ?", userID).
		Order("trx_date DESC, created_at DESC").
		Limit(limit).
		Find(&transactions).Error
	return transactions, err
}
