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
	"fahd-backend/internal/queue"
	"fahd-backend/internal/session"
	"fahd-backend/internal/worker"
	"fahd-backend/internal/ws"
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

	sessionTTL, err := time.ParseDuration(cfg.SessionTTL)
	if err != nil {
		sessionTTL = 1 * time.Hour
	}

	hub := ws.NewHub()
	sessionMgr := session.NewManager(cfg.JWTSecret, sessionTTL)
	msgQueue := queue.NewMemoryQueue(cfg.WSQueueSize)

	msgHandler := func(sessionID, content string) {
		if err := msgQueue.Enqueue(queue.Message{SessionID: sessionID, Content: content}); err != nil {
			slog.Error("enqueue failed", "session_id", sessionID, "error", err)
			hub.SendToSession(sessionID, ws.ServerMessage{
				Type:  "ai_error",
				Error: "system busy, please try again",
			})
		}
	}

	chatHandler := ws.NewChatHandler(hub, sessionMgr, database, msgHandler)

	workerPool := worker.NewPool(cfg.WorkerCount, msgQueue, hub, database, cfg, sessionMgr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	workerPool.Start(ctx)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      httpserver.NewRouter(cfg, database, http.HandlerFunc(chatHandler.ServeWS)),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
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

	slog.Info("shutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	if err := sqlDB.Close(); err != nil {
		slog.Error("database close error", "error", err)
	}
}
