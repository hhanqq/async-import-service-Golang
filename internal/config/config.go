package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppPort              int
	PostgresDSN          string
	RabbitMQURL          string
	RabbitMQQueue        string
	RabbitMQRetryQueue   string
	RabbitMQDLQQueue     string
	RabbitMQMaxRetries   int
	RabbitMQRetryDelayMS int
	LogLevel             slog.Level
}

func Load() (Config, error) {
	cfg := Config{
		AppPort:              8080,
		RabbitMQQueue:        "client_import_queue",
		RabbitMQMaxRetries:   3,
		RabbitMQRetryDelayMS: 5000,
		LogLevel:             slog.LevelInfo,
	}

	if port := strings.TrimSpace(os.Getenv("APP_PORT")); port != "" {
		parsed, err := strconv.Atoi(port)
		if err != nil {
			return Config{}, fmt.Errorf("parse APP_PORT: %w", err)
		}
		cfg.AppPort = parsed
	}

	cfg.PostgresDSN = strings.TrimSpace(os.Getenv("POSTGRES_DSN"))
	cfg.RabbitMQURL = strings.TrimSpace(os.Getenv("RABBITMQ_URL"))

	if queue := strings.TrimSpace(os.Getenv("RABBITMQ_QUEUE")); queue != "" {
		cfg.RabbitMQQueue = queue
	}
	if retryQueue := strings.TrimSpace(os.Getenv("RABBITMQ_RETRY_QUEUE")); retryQueue != "" {
		cfg.RabbitMQRetryQueue = retryQueue
	}
	if dlqQueue := strings.TrimSpace(os.Getenv("RABBITMQ_DLQ_QUEUE")); dlqQueue != "" {
		cfg.RabbitMQDLQQueue = dlqQueue
	}
	if maxRetries := strings.TrimSpace(os.Getenv("RABBITMQ_MAX_RETRIES")); maxRetries != "" {
		parsed, err := strconv.Atoi(maxRetries)
		if err != nil {
			return Config{}, fmt.Errorf("parse RABBITMQ_MAX_RETRIES: %w", err)
		}
		if parsed < 0 {
			return Config{}, fmt.Errorf("RABBITMQ_MAX_RETRIES must be >= 0")
		}
		cfg.RabbitMQMaxRetries = parsed
	}
	if retryDelay := strings.TrimSpace(os.Getenv("RABBITMQ_RETRY_DELAY_MS")); retryDelay != "" {
		parsed, err := strconv.Atoi(retryDelay)
		if err != nil {
			return Config{}, fmt.Errorf("parse RABBITMQ_RETRY_DELAY_MS: %w", err)
		}
		if parsed < 0 {
			return Config{}, fmt.Errorf("RABBITMQ_RETRY_DELAY_MS must be >= 0")
		}
		cfg.RabbitMQRetryDelayMS = parsed
	}

	if cfg.RabbitMQRetryQueue == "" {
		cfg.RabbitMQRetryQueue = cfg.RabbitMQQueue + ".retry"
	}
	if cfg.RabbitMQDLQQueue == "" {
		cfg.RabbitMQDLQQueue = cfg.RabbitMQQueue + ".dlq"
	}

	if lvl := strings.TrimSpace(os.Getenv("LOG_LEVEL")); lvl != "" {
		switch strings.ToLower(lvl) {
		case "debug":
			cfg.LogLevel = slog.LevelDebug
		case "info":
			cfg.LogLevel = slog.LevelInfo
		case "warn", "warning":
			cfg.LogLevel = slog.LevelWarn
		case "error":
			cfg.LogLevel = slog.LevelError
		default:
			return Config{}, fmt.Errorf("unsupported LOG_LEVEL: %s", lvl)
		}
	}

	if cfg.PostgresDSN == "" {
		return Config{}, fmt.Errorf("POSTGRES_DSN is required")
	}

	if cfg.RabbitMQURL == "" {
		return Config{}, fmt.Errorf("RABBITMQ_URL is required")
	}

	return cfg, nil
}
