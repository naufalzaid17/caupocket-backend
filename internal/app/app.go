// Package app merangkai seluruh layer (repository -> service -> handler)
// dan membangun HTTP router Gin.
package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"caupocket-api/internal/config"
	"caupocket-api/internal/events"
	"caupocket-api/internal/handler"
	"caupocket-api/internal/middleware"
	"caupocket-api/internal/model"
	"caupocket-api/internal/repository"
	"caupocket-api/internal/service"
	"caupocket-api/internal/worker"
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
	habitSettingRepo := repository.NewHabitSettingRepository(db)
	habitLogRepo := repository.NewHabitLogRepository(db)

	// Event bus (EPIC V2.2): in-memory default, Redis Pub/Sub opsional.
	var bus events.EventBus
	switch cfg.EventBus {
	case "redis":
		bus = events.NewRedisBus(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	default:
		bus = events.NewInMemoryBus()
	}

	// Service layer.
	authService := service.NewAuthService(db, userRepo, walletRepo, categoryRepo, cfg.JWTSecret, cfg.JWTExpiresIn)
	walletService := service.NewWalletService(walletRepo)
	categoryService := service.NewCategoryService(db, categoryRepo)
	goalService := service.NewGoalService(db, goalRepo)
	transactionService := service.NewTransactionService(db, walletRepo, goalRepo, categoryRepo, transactionRepo, bus)
	ocrService := service.NewOcrService(cfg.TesseractBin, cfg.TesseractLang, cfg.OcrTimeout)
	habitService := service.NewHabitService(
		db, userRepo, walletRepo, goalRepo, transactionRepo,
		habitSettingRepo, habitLogRepo, transactionService,
		cfg.HabitStreakDays, cfg.HabitReward,
	)

	// Background worker: listen "transaction.created" → habit engine.
	habitWorker := worker.NewHabitWorker(bus, habitService)
	workerCtx, stopWorker := context.WithCancel(context.Background())
	if err := habitWorker.Start(workerCtx); err != nil {
		stopWorker()
		cleanup()
		if busErr := bus.Close(); busErr != nil {
			log.Printf("tutup bus saat gagal start worker: %v", busErr)
		}
		return nil, nil, fmt.Errorf("gagal menjalankan habit worker (periksa EVENT_BUS/REDIS_ADDR): %w", err)
	}

	// Handler layer.
	authHandler := handler.NewAuthHandler(authService)
	walletHandler := handler.NewWalletHandler(walletService)
	categoryHandler := handler.NewCategoryHandler(categoryService)
	goalHandler := handler.NewGoalHandler(goalService)
	transactionHandler := handler.NewTransactionHandler(transactionService)
	ocrHandler := handler.NewOcrHandler(ocrService)
	habitHandler := handler.NewHabitHandler(habitService)

	router := gin.New()
	// Jangan percaya proxy apa pun: ClientIP = RemoteAddr sehingga tidak bisa
	// dipalsukan lewat X-Forwarded-For (bypass rate limiter).
	_ = router.SetTrustedProxies(nil)
	router.Use(gin.Logger(), gin.Recovery(), middleware.CORS(), middleware.SecurityHeaders())

	router.GET("/healthz", func(c *gin.Context) {
		response.Success(c, http.StatusOK, "Caupocket API berjalan", gin.H{"status": "ok"})
	})
	if cfg.SwaggerEnabled {
		router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	v1 := router.Group("/api/v1")
	{
		// Rate limiting pada endpoint sensitif (brute-force login & penyalahgunaan
		// OCR). Batas dikonfigurasi via env; 0 = nonaktif (mis. untuk load test).
		v1.POST("/auth/register", optRateLimit(cfg.RateLimitRegisterPerMin), authHandler.Register)
		v1.POST("/auth/login", optRateLimit(cfg.RateLimitLoginPerMin), authHandler.Login)

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
			protected.POST("/ocr/receipt", optRateLimit(cfg.RateLimitOcrPerMin), ocrHandler.ScanReceipt)
			protected.GET("/habits/settings", habitHandler.GetSettings)
			protected.PUT("/habits/settings", habitHandler.UpdateSettings)
			protected.GET("/habits/logs", habitHandler.ListLogs)
		}
	}

	fullCleanup := func() {
		stopWorker()
		if err := bus.Close(); err != nil {
			log.Printf("tutup event bus: %v", err)
		}
		cleanup()
	}
	return router, fullCleanup, nil
}

// optRateLimit mengembalikan middleware pembatas `max` request/menit/IP;
// max <= 0 berarti tanpa pembatasan (mis. untuk load test terkontrol).
func optRateLimit(max int) gin.HandlerFunc {
	if max <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	return middleware.RateLimit(max, time.Minute)
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
		&model.HabitSetting{},
		&model.HabitLog{},
	)
}
