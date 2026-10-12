// devpg menjalankan embedded PostgreSQL untuk development lokal tanpa Docker.
//
//	go run ./scripts/devpg            # port default 5433, data di os.TempDir()
//	go run ./scripts/devpg -port 5433 -data C:\pgdata
//
// Tekan Ctrl+C untuk menghentikan (data dipertahankan di direktori data).
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
)

func main() {
	port := flag.Int("port", 5433, "port TCP PostgreSQL")
	dataDir := flag.String("data", filepath.Join(os.TempDir(), "caupocket-dev-pg"), "direktori data")
	flag.Parse()

	if *port < 1 || *port > 65535 {
		log.Fatalf("port harus 1-65535, dapat %d", *port)
	}

	pg := embeddedpostgres.NewDatabase(
		embeddedpostgres.DefaultConfig().
			Port(uint32(*port)).
			Username("caupocket").Password("caupocket").Database("caupocket").
			BinariesPath(filepath.Join(*dataDir, "bin")).
			DataPath(filepath.Join(*dataDir, "data")).
			RuntimePath(filepath.Join(*dataDir, "runtime")),
	)

	if err := pg.Start(); err != nil {
		log.Fatalf("gagal menjalankan postgres: %v", err)
	}
	fmt.Printf("PostgreSQL berjalan: postgres://caupocket:caupocket@localhost:%d/caupocket?sslmode=disable\n", *port)
	fmt.Println("Tekan Ctrl+C untuk berhenti.")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("mematikan postgres...")
	if err := pg.Stop(); err != nil {
		log.Printf("stop: %v", err)
	}
}
