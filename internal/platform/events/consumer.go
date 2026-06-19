package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
)

// Consumer reads PaymentEvents from a Kafka topic for a given consumer group.
type Consumer struct {
	reader *kafka.Reader
	dlq    *DLQPublisher
	topic  string
}

func NewConsumer(broker, topic, groupID string, dlq *DLQPublisher) *Consumer {
	return &Consumer{
		topic:  topic,
		dlq:    dlq,
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:        []string{broker},
			Topic:          topic,
			GroupID:        groupID,
			MinBytes:       1,
			MaxBytes:       10e6,
			CommitInterval: 0, // synchronous commit
		}),
	}
}

// Run blocks and calls handler for each message. Returns on context cancellation.
func (c *Consumer) Run(ctx context.Context, handler func(PaymentEvent) error) error {
	defer func() { _ = c.reader.Close() }()
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // clean shutdown
			}
			slog.ErrorContext(ctx, "fetch error", "error", err)
			continue
		}

		var event PaymentEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			slog.ErrorContext(ctx, "unmarshal error", "offset", msg.Offset, "error", err)
			_ = c.routeToDLQ(ctx, event, 0, fmt.Sprintf("unmarshal error: %v", err))
			_ = c.reader.CommitMessages(ctx, msg)
			continue
		}

		if err := c.processWithRetry(ctx, event, handler); err != nil {
			slog.ErrorContext(ctx, "event failed after retries",
				"event_id", event.EventID,
				"event_type", event.EventType,
				"error", err,
			)
		}

		_ = c.reader.CommitMessages(ctx, msg)
	}
}

// processWithRetry attempts handler with exponential backoff for transient errors.
// Permanent errors go to DLQ immediately.
func (c *Consumer) processWithRetry(ctx context.Context, event PaymentEvent, handler func(PaymentEvent) error) error {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		lastErr = handler(event)
		if lastErr == nil {
			return nil
		}

		// Permanent errors — DLQ immediately, no retries
		if IsPermanent(lastErr) {
			return c.routeToDLQ(ctx, event, 0, lastErr.Error())
		}

		// Transient errors — retry with backoff
		if attempt < maxRetries {
			backoff := time.Duration(500*attempt*attempt) * time.Millisecond
			slog.WarnContext(ctx, "retrying after transient error",
				"event_id", event.EventID,
				"attempt", attempt+1,
				"backoff_ms", backoff.Milliseconds(),
				"error", lastErr,
			)
			select {
			case <-ctx.Done():
				return lastErr
			case <-time.After(backoff):
			}
		}
	}

	// Exhausted retries — send to DLQ
	return c.routeToDLQ(ctx, event, maxRetries, lastErr.Error())
}

func (c *Consumer) routeToDLQ(ctx context.Context, event PaymentEvent, retryCount int, reason string) error {
	if c.dlq == nil {
		slog.ErrorContext(ctx, "no DLQ publisher configured, event lost",
			"payment_id", event.PaymentID,
			"reason", reason,
		)
		return fmt.Errorf("DLQ unavailable: %s", reason)
	}
	return c.dlq.Send(ctx, event, retryCount, reason)
}
