package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"async-import-service/internal/config"
	"async-import-service/internal/queue"
	"async-import-service/internal/repository"
	"async-import-service/internal/worker"
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
		logger.Error("failed to connect postgres", "error", err)
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
	processor := worker.NewProcessor(repo, repo, logger)

	logger.Info("worker started", "queue", cfg.RabbitMQQueue)
	if err := rabbit.Consume(ctx, processor.ProcessMessage); err != nil {
		logger.Error("worker stopped with error", "error", err)
		os.Exit(1)
	}

	logger.Info("worker stopped")
}
