package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"async-import-service/internal/config"
	"async-import-service/internal/handler"
	"async-import-service/internal/queue"
	"async-import-service/internal/repository"
	"async-import-service/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Errorf("load config: %w", err))
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Error("failed to ping postgres", "error", err)
		os.Exit(1)
	}

	rabbit, err := queue.NewRabbitMQ(cfg.RabbitMQURL, queue.Settings{
		MainQueue:  cfg.RabbitMQQueue,
		RetryQueue: cfg.RabbitMQRetryQueue,
		DLQQueue:   cfg.RabbitMQDLQQueue,
		MaxRetries: cfg.RabbitMQMaxRetries,
		RetryDelay: time.Duration(cfg.RabbitMQRetryDelayMS) * time.Millisecond,
	}, logger)
	if err != nil {
		logger.Error("failed to connect rabbitmq", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := rabbit.Close(); err != nil {
			logger.Error("failed to close rabbitmq", "error", err)
		}
	}()

	repo := repository.NewPostgresRepository(pool)
	importService := service.NewImportService(repo, rabbit)
	importHandler := handler.NewImportHandler(importService)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	importHandler.RegisterRoutes(r)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.AppPort),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("api server started", "port", cfg.AppPort)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-serverErr:
		logger.Error("http server failed", "error", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("failed to shutdown http server", "error", err)
	}

	logger.Info("api stopped")
}
