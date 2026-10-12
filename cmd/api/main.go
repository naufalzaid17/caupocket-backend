package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"caupocket-api/internal/app"
	"caupocket-api/internal/config"
)

// @title						Caupocket API
// @version					1.0
// @description				RESTful API untuk Caupocket — personal finance & goal-based saving planner.
// @description				Seluruh mutasi dana (cash-out dan goal funding) dijamin integritas ACID melalui database transaction.
//
// @host						localhost:8080
// @BasePath					/api/v1
//
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
// @description				Masukkan nilai: "Bearer <token JWT hasil login>"
func main() {
	cfg := config.MustLoad()

	router, cleanup, err := app.Build(cfg)
	if err != nil {
		log.Fatalf("gagal membangun aplikasi: %v", err)
	}
	defer cleanup()

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second, // mitigasi Slowloris (gosec G112)
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	go func() {
		log.Printf("Caupocket API berjalan di http://localhost:%s (swagger: /swagger/index.html)", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server gagal berjalan: %v", err)
		}
	}()

	// Graceful shutdown saat menerima SIGINT/SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("mematikan server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("graceful shutdown gagal: %v", err)
	}
	log.Println("server berhenti")
}
