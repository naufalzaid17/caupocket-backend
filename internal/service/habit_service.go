package service

// HabitService (EPIC V2.2): aturan habit & auto-funding.
//
// Alur event-driven:
//  1. CreateTransaction sukses → publish "transaction.created".
//  2. Worker consumer memanggil HandleTransactionCreated.
//  3. Bila user mengaktifkan auto-nabung dan streak "tanpa jajan" tercapai,
//     sistem memanggil FundGoal (TX ACID yang sama dengan setoran manual)
//     dan mencatat reward ke habit_logs.

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"caupocket-api/internal/model"
	"caupocket-api/internal/repository"
)

// HabitService business logic habit engine.
type HabitService interface {
	HandleTransactionCreated(ctx context.Context, userID uuid.UUID) error
	GetSettings(ctx context.Context, userID uuid.UUID) (*model.HabitSetting, error)
	UpdateSettings(ctx context.Context, userID uuid.UUID, enabled bool) (*model.HabitSetting, error)
	ListLogs(ctx context.Context, userID uuid.UUID, limit int) ([]model.HabitLog, error)
	// StreakDays & RewardAmount untuk ditampilkan frontend.
	RuleConfig() (streakDays int, rewardAmount int64)
}

type habitService struct {
	db            *gorm.DB
	users         repository.UserRepository
	wallets       repository.WalletRepository
	goals         repository.GoalRepository
	transactions  repository.TransactionRepository
	habitSettings repository.HabitSettingRepository
	habitLogs     repository.HabitLogRepository
	ledger        TransactionService

	streakDays   int
	rewardAmount int64
}

func NewHabitService(
	db *gorm.DB,
	users repository.UserRepository,
	wallets repository.WalletRepository,
	goals repository.GoalRepository,
	transactions repository.TransactionRepository,
	habitSettings repository.HabitSettingRepository,
	habitLogs repository.HabitLogRepository,
	ledger TransactionService,
	streakDays int,
	rewardAmount int64,
) HabitService {
	return &habitService{
		db:            db,
		users:         users,
		wallets:       wallets,
		goals:         goals,
		transactions:  transactions,
		habitSettings: habitSettings,
		habitLogs:     habitLogs,
		ledger:        ledger,
		streakDays:    streakDays,
		rewardAmount:  rewardAmount,
	}
}

func (s *habitService) RuleConfig() (int, int64) { return s.streakDays, s.rewardAmount }

// HandleTransactionCreated mengevaluasi streak "tanpa jajan" milik user.
// Aman dipanggil asinkron dari worker; error tidak pernah membatalkan
// transaksi pemicu (sudah commit).
func (s *habitService) HandleTransactionCreated(ctx context.Context, userID uuid.UUID) error {
	setting, err := s.habitSettings.FindOrCreate(ctx, s.db, userID)
	if err != nil {
		return fmt.Errorf("memuat setting habit: %w", err)
	}
	if !setting.AutoFundEnabled {
		return nil
	}

	streak, anchor, err := s.currentStreak(ctx, userID)
	if err != nil {
		return err
	}
	if streak < s.streakDays {
		return nil
	}

	// Guard: satu streak (sejak hari bersih pertama setelah pengeluaran
	// terakhir) hanya direward satu kali.
	streakStart := startOfDay(anchor).AddDate(0, 0, 1)
	already, err := s.habitLogs.ExistsSince(ctx, userID, streakStart)
	if err != nil {
		return err
	}
	if already {
		return nil
	}

	goal, err := s.goals.FirstActiveByUser(ctx, userID)
	if err != nil {
		return err
	}
	if goal == nil {
		log.Printf("[habit] user %s streak %d hari tapi tidak punya goal aktif — reward dilewati", userID, streak)
		return nil
	}

	wallets, err := s.wallets.ListByUser(ctx, userID)
	if err != nil {
		return err
	}
	if len(wallets) == 0 || wallets[0].Balance < s.rewardAmount {
		log.Printf("[habit] saldo user %s tidak cukup untuk reward — dilewati", userID)
		return nil
	}

	result, err := s.ledger.FundGoal(ctx, userID, FundGoalInput{
		GoalID:      goal.ID,
		WalletID:    wallets[0].ID,
		Amount:      s.rewardAmount,
		Description: fmt.Sprintf("Reward habit: %d hari tanpa jajan", streak),
	})
	if err != nil {
		return fmt.Errorf("auto-funding gagal: %w", err)
	}

	today := startOfDay(time.Now())
	entry := &model.HabitLog{
		ID:           uuid.New(),
		UserID:       userID,
		GoalID:       goal.ID,
		GoalTitle:    goal.Title,
		StreakDays:   streak,
		RewardAmount: s.rewardAmount,
		RewardDate:   today,
		Description:  fmt.Sprintf("Auto-nabung Rp%d ke \"%s\" (streak %d hari)", s.rewardAmount, goal.Title, streak),
	}
	if err := s.habitLogs.Create(ctx, s.db, entry); err != nil {
		return fmt.Errorf("mencatat habit_log: %w", err)
	}

	log.Printf("[habit] reward user %s: streak %d hari → Rp%d ke \"%s\" (goal kini %d/%d)",
		userID, streak, s.rewardAmount, goal.Title, result.Goal.CurrentAmount, result.Goal.TargetAmount)
	return nil
}

// currentStreak menghitung jumlah hari penuh tanpa pengeluaran.
// Anchor = tanggal pengeluaran EXPENSE terakhir (atau tanggal registrasi
// bila belum pernah berpengeluaran).
func (s *habitService) currentStreak(ctx context.Context, userID uuid.UUID) (int, time.Time, error) {
	lastExpense, err := s.transactions.LastExpenseDate(ctx, userID)
	if err != nil {
		return 0, time.Time{}, err
	}

	anchor := time.Time{}
	if lastExpense != nil {
		anchor = *lastExpense
	} else {
		user, err := s.users.FindByID(ctx, userID)
		if err != nil {
			return 0, time.Time{}, err
		}
		if user == nil {
			return 0, time.Time{}, ErrInvalidCredentials
		}
		anchor = user.CreatedAt
	}

	today := startOfDay(time.Now())
	streak := int(today.Sub(startOfDay(anchor)).Hours() / 24)
	return streak, anchor, nil
}

func (s *habitService) GetSettings(ctx context.Context, userID uuid.UUID) (*model.HabitSetting, error) {
	return s.habitSettings.FindOrCreate(ctx, s.db, userID)
}

func (s *habitService) UpdateSettings(ctx context.Context, userID uuid.UUID, enabled bool) (*model.HabitSetting, error) {
	setting, err := s.habitSettings.FindOrCreate(ctx, s.db, userID)
	if err != nil {
		return nil, err
	}
	setting.AutoFundEnabled = enabled
	if err := s.habitSettings.Update(ctx, s.db, setting); err != nil {
		return nil, err
	}
	return setting, nil
}

func (s *habitService) ListLogs(ctx context.Context, userID uuid.UUID, limit int) ([]model.HabitLog, error) {
	return s.habitLogs.ListByUser(ctx, userID, limit)
}

func startOfDay(t time.Time) time.Time {
	t = t.In(time.Local)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}
