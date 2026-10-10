package service

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"caupocket-api/internal/model"
	"caupocket-api/internal/repository"
	"caupocket-api/pkg/password"
)

// RegisterInput payload pendaftaran user baru.
type RegisterInput struct {
	Name     string
	Email    string
	Password string
}

// AuthService business logic autentikasi.
type AuthService interface {
	Register(ctx context.Context, input RegisterInput) (*model.User, error)
	Login(ctx context.Context, email, password string) (string, *model.User, error)
	GenerateToken(userID uuid.UUID) (string, error)
	GetUser(ctx context.Context, id uuid.UUID) (*model.User, error)
}

type authService struct {
	db         *gorm.DB
	users      repository.UserRepository
	wallets    repository.WalletRepository
	categories repository.CategoryRepository
	jwtSecret  string
	jwtTTL     time.Duration
}

func NewAuthService(
	db *gorm.DB,
	users repository.UserRepository,
	wallets repository.WalletRepository,
	categories repository.CategoryRepository,
	jwtSecret string,
	jwtTTL time.Duration,
) AuthService {
	return &authService{
		db:         db,
		users:      users,
		wallets:    wallets,
		categories: categories,
		jwtSecret:  jwtSecret,
		jwtTTL:     jwtTTL,
	}
}

// Register membuat user baru beserta dompet utama dan kategori default
// di dalam satu database transaction.
func (s *authService) Register(ctx context.Context, input RegisterInput) (*model.User, error) {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Name = strings.TrimSpace(input.Name)

	if input.Name == "" || input.Email == "" || len(input.Password) < 8 {
		return nil, ErrInvalidInput
	}

	existing, err := s.users.FindByEmail(ctx, input.Email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrEmailTaken
	}

	hash, err := password.HashPassword(input.Password)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		ID:           uuid.New(),
		Email:        input.Email,
		PasswordHash: hash,
		Name:         input.Name,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.users.Create(ctx, tx, user); err != nil {
			return err
		}
		wallet := &model.Wallet{
			ID:      uuid.New(),
			UserID:  user.ID,
			Name:    "Dompet Utama",
			Balance: 0,
		}
		if err := s.wallets.Create(ctx, tx, wallet); err != nil {
			return err
		}
		for i := range defaultCategories {
			cat := defaultCategories[i]
			cat.ID = uuid.New()
			cat.UserID = user.ID
			if err := s.categories.Create(ctx, tx, &cat); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("gagal mendaftarkan user %s: %v", input.Email, err)
		return nil, err
	}
	return user, nil
}

// Login memvalidasi kredensial lalu menghasilkan JWT.
func (s *authService) Login(ctx context.Context, email, plainPassword string) (string, *model.User, error) {
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return "", nil, err
	}
	if user == nil || !password.CheckPasswordHash(user.PasswordHash, plainPassword) {
		return "", nil, ErrInvalidCredentials
	}

	token, err := s.GenerateToken(user.ID)
	if err != nil {
		return "", nil, err
	}
	return token, user, nil
}

// GenerateToken menghasilkan JWT (HS256) yang memuat user_id pada claim sub.
func (s *authService) GenerateToken(userID uuid.UUID) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		Issuer:    "caupocket-api",
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.jwtTTL)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

// GetUser mengambil user berdasarkan ID (dipakai endpoint /me).
func (s *authService) GetUser(ctx context.Context, id uuid.UUID) (*model.User, error) {
	user, err := s.users.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}
	return user, nil
}

var defaultCategories = []model.Category{
	{Name: "Gaji", Type: model.CategoryIncome, IconName: "briefcase"},
	{Name: "Bonus", Type: model.CategoryIncome, IconName: "gift"},
	{Name: "Makanan & Minuman", Type: model.CategoryExpense, IconName: "utensils"},
	{Name: "Transportasi", Type: model.CategoryExpense, IconName: "car-front"},
	{Name: "Belanja", Type: model.CategoryExpense, IconName: "shopping-bag"},
	{Name: "Tagihan", Type: model.CategoryExpense, IconName: "receipt"},
	{Name: "Hiburan", Type: model.CategoryExpense, IconName: "gamepad-2"},
	{Name: "Lainnya", Type: model.CategoryExpense, IconName: "tag"},
}
