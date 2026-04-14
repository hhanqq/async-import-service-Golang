package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"async-import-service/internal/model"
)

const (
	retryCountHeader = "x-retry-count"
	lastErrorHeader  = "x-last-error"
)

type Settings struct {
	MainQueue  string
	RetryQueue string
	DLQQueue   string
	MaxRetries int
	RetryDelay time.Duration
}

type RabbitMQ struct {
	conn       *amqp.Connection
	ch         *amqp.Channel
	mainQueue  string
	retryQueue string
	dlqQueue   string
	maxRetries int
	logger     *slog.Logger
}

func NewRabbitMQ(url string, settings Settings, logger *slog.Logger) (*RabbitMQ, error) {
	if settings.MainQueue == "" {
		return nil, fmt.Errorf("main queue name is required")
	}
	if settings.RetryQueue == "" {
		return nil, fmt.Errorf("retry queue name is required")
	}
	if settings.DLQQueue == "" {
		return nil, fmt.Errorf("dlq queue name is required")
	}
	if settings.MaxRetries < 0 {
		return nil, fmt.Errorf("max retries must be >= 0")
	}
	if settings.RetryDelay < 0 {
		return nil, fmt.Errorf("retry delay must be >= 0")
	}

	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("dial rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open rabbitmq channel: %w", err)
	}

	if _, err := ch.QueueDeclare(
		settings.MainQueue,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("declare main queue: %w", err)
	}

	retryArgs := amqp.Table{
		"x-message-ttl":             int32(settings.RetryDelay.Milliseconds()),
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": settings.MainQueue,
	}
	if _, err := ch.QueueDeclare(
		settings.RetryQueue,
		true,
		false,
		false,
		false,
		retryArgs,
	); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("declare retry queue: %w", err)
	}

	if _, err := ch.QueueDeclare(
		settings.DLQQueue,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("declare dlq queue: %w", err)
	}

	return &RabbitMQ{
		conn:       conn,
		ch:         ch,
		mainQueue:  settings.MainQueue,
		retryQueue: settings.RetryQueue,
		dlqQueue:   settings.DLQQueue,
		maxRetries: settings.MaxRetries,
		logger:     logger,
	}, nil
}

func (r *RabbitMQ) PublishTask(ctx context.Context, msg model.TaskMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal task message: %w", err)
	}

	if err := r.ch.PublishWithContext(ctx,
		"",
		r.mainQueue,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	); err != nil {
		return fmt.Errorf("publish message: %w", err)
	}

	r.logger.Info("task published", "task_id", msg.TaskID, "queue", r.mainQueue)
	return nil
}

func (r *RabbitMQ) Consume(ctx context.Context, handler func(context.Context, []byte) error) error {
	if err := r.ch.Qos(1, 0, false); err != nil {
		return fmt.Errorf("set qos: %w", err)
	}

	deliveries, err := r.ch.Consume(
		r.mainQueue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("start consume: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("deliveries channel closed")
			}

			if err := handler(ctx, d.Body); err != nil {
				r.logger.Error("message processing failed", "error", err)
				if retryErr := r.handleRetryOrDLQ(ctx, d, err); retryErr != nil {
					r.logger.Error("failed to process retry/dlq flow", "error", retryErr)
					if nackErr := d.Nack(false, true); nackErr != nil {
						r.logger.Error("failed to nack message", "error", nackErr)
					}
				}
				continue
			}

			if err := d.Ack(false); err != nil {
				r.logger.Error("failed to ack message", "error", err)
			}
		}
	}
}

func (r *RabbitMQ) handleRetryOrDLQ(ctx context.Context, d amqp.Delivery, processErr error) error {
	currentRetryCount := retryCountFromHeaders(d.Headers)

	if currentRetryCount < r.maxRetries {
		headers := cloneHeaders(d.Headers)
		headers[retryCountHeader] = int32(currentRetryCount + 1)
		headers[lastErrorHeader] = processErr.Error()

		if err := r.publishWithHeaders(ctx, r.retryQueue, d.Body, headers); err != nil {
			return fmt.Errorf("publish to retry queue: %w", err)
		}
		if err := d.Ack(false); err != nil {
			return fmt.Errorf("ack after retry publish: %w", err)
		}

		r.logger.Warn(
			"message moved to retry queue",
			"current_retry", currentRetryCount+1,
			"max_retries", r.maxRetries,
			"retry_queue", r.retryQueue,
		)
		return nil
	}

	headers := cloneHeaders(d.Headers)
	headers[lastErrorHeader] = processErr.Error()

	if err := r.publishWithHeaders(ctx, r.dlqQueue, d.Body, headers); err != nil {
		return fmt.Errorf("publish to dlq: %w", err)
	}
	if err := d.Ack(false); err != nil {
		return fmt.Errorf("ack after dlq publish: %w", err)
	}

	r.logger.Warn(
		"message moved to dlq after max retries",
		"max_retries", r.maxRetries,
		"dlq_queue", r.dlqQueue,
	)

	return nil
}

func (r *RabbitMQ) publishWithHeaders(ctx context.Context, routingKey string, body []byte, headers amqp.Table) error {
	return r.ch.PublishWithContext(
		ctx,
		"",
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Headers:      headers,
			Body:         body,
			Timestamp:    time.Now(),
		},
	)
}

func retryCountFromHeaders(headers amqp.Table) int {
	if headers == nil {
		return 0
	}
	value, ok := headers[retryCountHeader]
	if !ok {
		return 0
	}

	switch v := value.(type) {
	case int:
		return v
	case int8:
		return int(v)
	case int16:
		return int(v)
	case int32:
		return int(v)
	case int64:
		return int(v)
	case uint:
		return int(v)
	case uint8:
		return int(v)
	case uint16:
		return int(v)
	case uint32:
		return int(v)
	case uint64:
		return int(v)
	default:
		return 0
	}
}

func cloneHeaders(headers amqp.Table) amqp.Table {
	if headers == nil {
		return amqp.Table{}
	}

	cloned := make(amqp.Table, len(headers))
	for k, v := range headers {
		cloned[k] = v
	}
	return cloned
}

func (r *RabbitMQ) Close() error {
	var firstErr error
	if r.ch != nil {
		if err := r.ch.Close(); err != nil {
			firstErr = err
		}
	}
	if r.conn != nil {
		if err := r.conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
