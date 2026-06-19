package events

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/segmentio/kafka-go"
)

// Consumer reads PaymentEvents from a Kafka topic for a given consumer group.
type Consumer struct {
	reader *kafka.Reader
}

func NewConsumer(broker, topic, groupID string) *Consumer {
	return &Consumer{
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
	defer c.reader.Close()
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
			_ = c.reader.CommitMessages(ctx, msg)
			continue
		}

		if err := handler(event); err != nil {
			slog.ErrorContext(ctx, "handler error",
				"event_id", event.EventID,
				"event_type", event.EventType,
				"error", err,
			)
		}

		_ = c.reader.CommitMessages(ctx, msg)
	}
}
