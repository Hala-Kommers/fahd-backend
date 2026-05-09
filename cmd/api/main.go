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

	"fahd-backend/internal/config"
	"fahd-backend/internal/db"
	httpserver "fahd-backend/internal/http"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	database, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("database connection error: %v", err)
	}

	sqlDB, err := database.DB()
	if err != nil {
		log.Fatalf("database handle error: %v", err)
	}

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      httpserver.NewRouter(cfg, database),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("api listening on :%s", cfg.Port)
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Fatalf("server error: %v", serveErr)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}

	if err := sqlDB.Close(); err != nil {
		log.Printf("database close error: %v", err)
	}
}
