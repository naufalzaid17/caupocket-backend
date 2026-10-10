package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"caupocket-api/internal/model"
)

// WalletRepository abstraksi akses data tabel wallets.
//
// FindForUpdate dan ApplyBalanceDelta WAJIB dipanggil di dalam database
// transaction: kunci baris (SELECT ... FOR UPDATE) mencegah race condition
// ketika dua request memotong saldo secara bersamaan.
type WalletRepository interface {
	Create(ctx context.Context, db *gorm.DB, wallet *model.Wallet) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.Wallet, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Wallet, error)
	FindForUpdate(ctx context.Context, tx *gorm.DB, userID, walletID uuid.UUID) (*model.Wallet, error)
	ApplyBalanceDelta(ctx context.Context, tx *gorm.DB, walletID uuid.UUID, delta int64) error
}

type walletRepository struct {
	db *gorm.DB
}

func NewWalletRepository(db *gorm.DB) WalletRepository {
	return &walletRepository{db: db}
}

func (r *walletRepository) Create(ctx context.Context, db *gorm.DB, wallet *model.Wallet) error {
	if wallet.ID == uuid.Nil {
		wallet.ID = uuid.New()
	}
	return db.WithContext(ctx).Create(wallet).Error
}

func (r *walletRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Wallet, error) {
	var wallet model.Wallet
	err := r.db.WithContext(ctx).First(&wallet, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &wallet, nil
}

func (r *walletRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Wallet, error) {
	var wallets []model.Wallet
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at ASC").
		Find(&wallets).Error
	return wallets, err
}

func (r *walletRepository) FindForUpdate(ctx context.Context, tx *gorm.DB, userID, walletID uuid.UUID) (*model.Wallet, error) {
	var wallet model.Wallet
	err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND user_id = ?", walletID, userID).
		First(&wallet).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &wallet, nil
}

func (r *walletRepository) ApplyBalanceDelta(ctx context.Context, tx *gorm.DB, walletID uuid.UUID, delta int64) error {
	return tx.WithContext(ctx).
		Model(&model.Wallet{}).
		Where("id = ?", walletID).
		Update("balance", gorm.Expr("balance + ?", delta)).Error
}
