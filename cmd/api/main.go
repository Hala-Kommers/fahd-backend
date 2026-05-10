package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fahd-backend/internal/config"
	"fahd-backend/internal/db"
	httpserver "fahd-backend/internal/http"
	"fahd-backend/internal/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config error", "error", err)
		os.Exit(1)
	}
	logger.Configure(cfg)

	database, err := db.Connect(cfg)
	if err != nil {
		slog.Error("database connection error", "error", err)
		os.Exit(1)
	}

	sqlDB, err := database.DB()
	if err != nil {
		slog.Error("database handle error", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      httpserver.NewRouter(cfg, database),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("api listening", "port", cfg.Port)
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			slog.Error("server error", "error", serveErr)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	if err := sqlDB.Close(); err != nil {
		slog.Error("database close error", "error", err)
	}
}
