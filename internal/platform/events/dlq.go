package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
)

const maxRetries = 3

// DLQEnvelope wraps a failed event with error metadata for forensic investigation.
type DLQEnvelope struct {
	OriginalEvent PaymentEvent `json:"original_event"`
	OriginalTopic string       `json:"original_topic"`
	ConsumerGroup string       `json:"consumer_group"`
	ErrorReason   string       `json:"error_reason"`
	RetryCount    int          `json:"retry_count"`
	FailedAt      time.Time    `json:"failed_at"`
}

// DLQPublisher writes DLQEnvelope messages to the dead letter topic.
type DLQPublisher struct {
	writer        *kafka.Writer
	dlqTopic      string
	consumerGroup string
	originalTopic string
}

func NewDLQPublisher(broker, dlqTopic, originalTopic, consumerGroup string) *DLQPublisher {
	return &DLQPublisher{
		dlqTopic:      dlqTopic,
		originalTopic: originalTopic,
		consumerGroup: consumerGroup,
		writer: &kafka.Writer{
			Addr:                   kafka.TCP(broker),
			Topic:                  dlqTopic,
			Balancer:               &kafka.LeastBytes{},
			AllowAutoTopicCreation: false,
		},
	}
}

func (d *DLQPublisher) Send(ctx context.Context, event PaymentEvent, retryCount int, reason string) error {
	envelope := DLQEnvelope{
		OriginalEvent: event,
		OriginalTopic: d.originalTopic,
		ConsumerGroup: d.consumerGroup,
		ErrorReason:   reason,
		RetryCount:    retryCount,
		FailedAt:      time.Now(),
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	err = d.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.PaymentID),
		Value: data,
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to write to DLQ",
			"payment_id", event.PaymentID,
			"dlq_topic", d.dlqTopic,
			"error", err,
		)
		return err
	}
	slog.WarnContext(ctx, "event routed to DLQ",
		"payment_id", event.PaymentID,
		"event_type", event.EventType,
		"retry_count", retryCount,
		"reason", reason,
	)
	return nil
}

func (d *DLQPublisher) Close() error { return d.writer.Close() }
