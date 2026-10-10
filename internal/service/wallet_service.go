package service

import (
	"context"

	"github.com/google/uuid"

	"caupocket-api/internal/model"
	"caupocket-api/internal/repository"
)

// WalletService business logic pembacaan dompet.
type WalletService interface {
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Wallet, error)
}

type walletService struct {
	wallets repository.WalletRepository
}

func NewWalletService(wallets repository.WalletRepository) WalletService {
	return &walletService{wallets: wallets}
}

func (s *walletService) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Wallet, error) {
	return s.wallets.ListByUser(ctx, userID)
}
