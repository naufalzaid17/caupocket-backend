// Package app merangkai seluruh layer (repository -> service -> handler)
// dan membangun HTTP router Gin.
package app

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	ginSwagger "github.com/swaggo/gin-swagger"
	swaggerFiles "github.com/swaggo/files"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"caupocket-api/internal/config"
	"caupocket-api/internal/handler"
	"caupocket-api/internal/middleware"
	"caupocket-api/internal/model"
	"caupocket-api/internal/repository"
	"caupocket-api/internal/service"
	"caupocket-api/pkg/response"

	_ "caupocket-api/docs" // registrasi dokumen Swagger hasil swag init
)

// Build melakukan koneksi database, migrasi, wiring dependency, dan
// mengembalikan router siap jalankan beserta fungsi cleanup.
func Build(cfg *config.Config) (*gin.Engine, func(), error) {
	gin.SetMode(cfg.GinMode)

	db, err := connectDB(cfg.DatabaseURL)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}

	if err := migrate(db); err != nil {
		cleanup()
		return nil, nil, err
	}

	// Repository layer.
	userRepo := repository.NewUserRepository(db)
	walletRepo := repository.NewWalletRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)
	goalRepo := repository.NewGoalRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)

	// Service layer.
	authService := service.NewAuthService(db, userRepo, walletRepo, categoryRepo, cfg.JWTSecret, cfg.JWTExpiresIn)
	walletService := service.NewWalletService(walletRepo)
	categoryService := service.NewCategoryService(db, categoryRepo)
	goalService := service.NewGoalService(db, goalRepo)
	transactionService := service.NewTransactionService(db, walletRepo, goalRepo, categoryRepo, transactionRepo)

	// Handler layer.
	authHandler := handler.NewAuthHandler(authService)
	walletHandler := handler.NewWalletHandler(walletService)
	categoryHandler := handler.NewCategoryHandler(categoryService)
	goalHandler := handler.NewGoalHandler(goalService)
	transactionHandler := handler.NewTransactionHandler(transactionService)

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), middleware.CORS())

	router.GET("/healthz", func(c *gin.Context) {
		response.Success(c, http.StatusOK, "Caupocket API berjalan", gin.H{"status": "ok"})
	})
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := router.Group("/api/v1")
	{
		v1.POST("/auth/register", authHandler.Register)
		v1.POST("/auth/login", authHandler.Login)

		protected := v1.Group("")
		protected.Use(middleware.AuthRequired(cfg.JWTSecret))
		{
			protected.GET("/me", authHandler.Me)
			protected.GET("/wallets", walletHandler.ListWallets)
			protected.GET("/categories", categoryHandler.ListCategories)
			protected.POST("/categories", categoryHandler.CreateCategory)
			protected.GET("/transactions", transactionHandler.ListTransactions)
			protected.POST("/transactions", transactionHandler.CreateTransaction)
			protected.GET("/goals", goalHandler.ListGoals)
			protected.POST("/goals", goalHandler.CreateGoal)
			protected.POST("/goals/:id/fund", transactionHandler.FundGoal)
		}
	}

	return router, cleanup, nil
}

// connectDB membuka koneksi PostgreSQL dengan retry singkat agar server
// tetap bisa boot bersamaan dengan database.
func connectDB(url string) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	const retries = 5
	for attempt := 1; attempt <= retries; attempt++ {
		db, err = gorm.Open(postgres.Open(url), &gorm.Config{
			Logger: gormlogger.Default.LogMode(gormlogger.Warn),
		})
		if err == nil {
			sqlDB, pingErr := db.DB()
			if pingErr == nil {
				pingErr = sqlDB.Ping()
			}
			if pingErr == nil {
				return db, nil
			}
			err = pingErr
		}
		log.Printf("menunggu PostgreSQL (%d/%d): %v", attempt, retries, err)
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("tidak bisa terhubung ke PostgreSQL: %w", err)
}

// migrate membuat/mutakhirkan skema tabel beserta foreign key-nya.
func migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{},
		&model.Wallet{},
		&model.Category{},
		&model.SavingGoal{},
		&model.Transaction{},
	)
}
