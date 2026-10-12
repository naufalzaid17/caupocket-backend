package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"caupocket-api/internal/events"
	"caupocket-api/internal/model"
	"caupocket-api/internal/repository"
)

// CreateTransactionInput payload pencatatan transaksi income/expense.
type CreateTransactionInput struct {
	WalletID    uuid.UUID
	CategoryID  *uuid.UUID
	Type        model.TransactionType
	Amount      int64
	Description string
	TrxDate     time.Time
}

// FundGoalInput payload setor tabungan ke sebuah goal.
type FundGoalInput struct {
	GoalID   uuid.UUID
	WalletID uuid.UUID
	Amount   int64
	// Description opsional; kosong = "Setor tabungan: <judul goal>".
	Description string
}

// FundGoalResult hasil operasi goal funding: ledger entry + state goal terbaru.
type FundGoalResult struct {
	Transaction *model.Transaction
	Goal        *model.SavingGoal
}

// TransactionService inti Ledger Engine. Seluruh mutasi dana dijalankan dalam
// satu database transaction demi ACID: gagal di salah satu langkah maka semua
// di-rollback.
type TransactionService interface {
	CreateTransaction(ctx context.Context, userID uuid.UUID, input CreateTransactionInput) (*model.Transaction, error)
	FundGoal(ctx context.Context, userID uuid.UUID, input FundGoalInput) (*FundGoalResult, error)
	ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]model.Transaction, error)
}

type transactionService struct {
	db           *gorm.DB
	wallets      repository.WalletRepository
	goals        repository.GoalRepository
	categories   repository.CategoryRepository
	transactions repository.TransactionRepository
	// bus dipakai untuk publish event "transaction.created" setelah TX sukses
	// (EPIC V2.2). Nilai nil menonaktifkan event (mis. pada unit test).
	bus events.EventBus
}

func NewTransactionService(
	db *gorm.DB,
	wallets repository.WalletRepository,
	goals repository.GoalRepository,
	categories repository.CategoryRepository,
	transactions repository.TransactionRepository,
	bus events.EventBus,
) TransactionService {
	return &transactionService{
		db:           db,
		wallets:      wallets,
		goals:        goals,
		categories:   categories,
		transactions: transactions,
		bus:          bus,
	}
}

// CreateTransaction menangani INCOME dan EXPENSE.
//
// Cash-Out (EXPENSE):
//  1. Kunci baris wallet & validasi kepemilikan + saldo mencukupi.
//  2. Potong balance wallets.
//  3. Catat ke transactions.
//
// Seluruh langkah dibungkus satu blok TX.
func (s *transactionService) CreateTransaction(ctx context.Context, userID uuid.UUID, input CreateTransactionInput) (*model.Transaction, error) {
	if input.Amount <= 0 {
		return nil, ErrInvalidAmount
	}
	if input.Type != model.TrxIncome && input.Type != model.TrxExpense {
		return nil, fmt.Errorf("%w: tipe hanya INCOME atau EXPENSE (GOAL_FUNDING via endpoint /goals/:id/fund)", ErrInvalidInput)
	}
	if input.TrxDate.IsZero() {
		input.TrxDate = time.Now()
	}

	var created *model.Transaction
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		wallet, err := s.wallets.FindForUpdate(ctx, tx, userID, input.WalletID)
		if err != nil {
			return err
		}
		if wallet == nil {
			return ErrWalletNotFound
		}

		if input.Type == model.TrxExpense && wallet.Balance < input.Amount {
			return ErrInsufficientBalance
		}

		if input.CategoryID != nil {
			category, err := s.categories.FindByID(ctx, *input.CategoryID)
			if err != nil {
				return err
			}
			if category == nil || category.UserID != userID {
				return ErrCategoryNotFound
			}
			if category.Type != model.CategoryType(input.Type) {
				return ErrCategoryMismatch
			}
		}

		delta := input.Amount
		if input.Type == model.TrxExpense {
			delta = -input.Amount
		}
		if err := s.wallets.ApplyBalanceDelta(ctx, tx, wallet.ID, delta); err != nil {
			return err
		}

		trx := &model.Transaction{
			ID:          uuid.New(),
			UserID:      userID,
			WalletID:    wallet.ID,
			CategoryID:  input.CategoryID,
			Type:        input.Type,
			Amount:      input.Amount,
			Description: input.Description,
			TrxDate:     input.TrxDate,
		}
		if err := s.transactions.Create(ctx, tx, trx); err != nil {
			return err
		}
		created = trx
		return nil
	})
	if err != nil {
		return nil, err
	}

	// EPIC V2.2: publikasikan event ASYNCHRONOUS setelah TX commit agar
	// consumer (habit worker) selalu melihat data yang sudah tersimpan.
	s.publishTransactionCreated(created)

	return created, nil
}

// publishTransactionCreated mengirim event ke bus dengan best-effort;
// kegagalan publish tidak pernah menggagalkan transaksi yang sudah commit.
func (s *transactionService) publishTransactionCreated(trx *model.Transaction) {
	if s.bus == nil {
		return
	}
	if err := s.bus.Publish(context.Background(), events.TopicTransactionCreated, events.TransactionCreatedEvent{
		UserID:        trx.UserID,
		TransactionID: trx.ID,
		Type:          string(trx.Type),
		Amount:        trx.Amount,
		TrxDate:       trx.TrxDate,
		OccurredAt:    time.Now(),
	}); err != nil {
		log.Printf("gagal publish %s: %v", events.TopicTransactionCreated, err)
	}
}

// FundGoal memindahkan dana dari dompet ke target tabungan dalam satu TX:
//  1. Kunci baris wallet & goal, validasi kepemilikan.
//  2. Validasi saldo dompet mencukupi.
//  3. Kurangi balance wallets.
//  4. Tambah current_amount saving_goals (otomatis ACHIEVED bila tercapai).
//  5. Catat transactions tipe GOAL_FUNDING.
//
// Jika langkah manapun gagal, seluruh mutasi di-rollback.
func (s *transactionService) FundGoal(ctx context.Context, userID uuid.UUID, input FundGoalInput) (*FundGoalResult, error) {
	if input.Amount <= 0 {
		return nil, ErrInvalidAmount
	}

	var result *FundGoalResult
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		wallet, err := s.wallets.FindForUpdate(ctx, tx, userID, input.WalletID)
		if err != nil {
			return err
		}
		if wallet == nil {
			return ErrWalletNotFound
		}

		goal, err := s.goals.FindForUpdate(ctx, tx, userID, input.GoalID)
		if err != nil {
			return err
		}
		if goal == nil {
			return ErrGoalNotFound
		}
		if goal.Status == model.GoalAchieved {
			return ErrGoalAlreadyAchieved
		}

		if wallet.Balance < input.Amount {
			return ErrInsufficientBalance
		}

		if err := s.wallets.ApplyBalanceDelta(ctx, tx, wallet.ID, -input.Amount); err != nil {
			return err
		}
		if err := s.goals.AddProgress(ctx, tx, goal.ID, input.Amount); err != nil {
			return err
		}

		trx := &model.Transaction{
			ID:          uuid.New(),
			UserID:      userID,
			WalletID:    wallet.ID,
			GoalID:      &goal.ID,
			Type:        model.TrxGoalFunding,
			Amount:      input.Amount,
			Description: fmt.Sprintf("Setor tabungan: %s", goal.Title),
			TrxDate:     time.Now(),
		}
		if err := s.transactions.Create(ctx, tx, trx); err != nil {
			return err
		}

		// Proyeksi state goal setelah UPDATE (konsisten dengan AddProgress).
		goal.CurrentAmount += input.Amount
		if goal.CurrentAmount >= goal.TargetAmount {
			goal.Status = model.GoalAchieved
		}

		result = &FundGoalResult{Transaction: trx, Goal: goal}
		return nil
	})
	if err != nil {
		log.Printf("goal funding gagal (user=%s goal=%s): %v", userID, input.GoalID, err)
		return nil, err
	}
	return result, nil
}

func (s *transactionService) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]model.Transaction, error) {
	return s.transactions.ListByUser(ctx, userID, limit)
}
