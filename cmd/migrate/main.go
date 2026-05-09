package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"fahd-backend/internal/config"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: go run ./cmd/migrate <up|down|status|create>")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	migrationsDir := filepath.Join("migrations")
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("goose dialect error: %v", err)
	}

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database open error: %v", err)
	}
	defer db.Close()

	command := os.Args[1]

	if command == "create" {
		if len(os.Args) < 3 {
			log.Fatalf("usage: go run ./cmd/migrate create <name>")
		}
		name := os.Args[2]
		if err := goose.Create(db, migrationsDir, name, "sql"); err != nil {
			log.Fatalf("create migration error: %v", err)
		}
		fmt.Println("migration created")
		return
	}

	if err := goose.Run(command, db, migrationsDir); err != nil {
		log.Fatalf("migration command error: %v", err)
	}
}
