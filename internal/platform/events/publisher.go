package events

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/segmentio/kafka-go"
	"github.com/victorbecerragit/project-payment-gateway/internal/platform/metrics"
)

// Publisher is the only contract the application layer depends on.
// Swap implementations without changing service.go.
type Publisher interface {
	Publish(ctx context.Context, event PaymentEvent) error
	Close() error
}

// KafkaPublisher publishes events to a Kafka topic using kafka-go.
type KafkaPublisher struct {
	writer *kafka.Writer
	topic  string
}

func NewKafkaPublisher(broker, topic string) *KafkaPublisher {
	return &KafkaPublisher{
		topic: topic,
		writer: &kafka.Writer{
			Addr:                   kafka.TCP(broker),
			Topic:                  topic,
			Balancer:               &kafka.LeastBytes{},
			AllowAutoTopicCreation: false,
		},
	}
}

func (p *KafkaPublisher) Publish(ctx context.Context, event PaymentEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	err = p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.PaymentID),
		Value: data,
	})
	if err != nil {
		metrics.EventsPublished.WithLabelValues(event.EventType, "error").Inc()
		slog.ErrorContext(ctx, "failed to publish kafka event",
			"event_type", event.EventType,
			"payment_id", event.PaymentID,
			"error", err,
		)
		return err
	}
	metrics.EventsPublished.WithLabelValues(event.EventType, "success").Inc()
	slog.InfoContext(ctx, "event published",
		"topic", p.topic,
		"event_type", event.EventType,
		"payment_id", event.PaymentID,
	)
	return nil
}

func (p *KafkaPublisher) Close() error {
	return p.writer.Close()
}
