package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/victorbecerragit/project-payment-gateway/internal/platform/events"
)

func main() {
	broker := requireEnv("KAFKA_BROKER")
	topic := getEnv("KAFKA_TOPIC", "payment-events")
	groupID := getEnv("KAFKA_GROUP_ID", "payment-audit-consumer")
	dlqTopic := getEnv("KAFKA_DLQ_TOPIC", "payment-events-dlq")

	slog.Info("payment-event-consumer starting",
		"broker", broker,
		"topic", topic,
		"group_id", groupID,
		"dlq_topic", dlqTopic,
	)

	dlq := events.NewDLQPublisher(broker, dlqTopic, topic, groupID)
	defer func() { _ = dlq.Close() }()

	consumer := events.NewConsumer(broker, topic, groupID, dlq)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := consumer.Run(ctx, auditHandler); err != nil {
		slog.Error("consumer exited with error", "error", err)
		os.Exit(1)
	}

	slog.Info("payment-event-consumer stopped")
}

// auditHandler logs a structured audit line for every payment event.
func auditHandler(e events.PaymentEvent) error {
	slog.Info("payment event received",
		"event_id", e.EventID,
		"event_type", e.EventType,
		"payment_id", e.PaymentID,
		"provider", e.Provider,
		"amount", e.Amount,
		"currency", e.Currency,
		"status", e.Status,
		"created_at", e.CreatedAt,
	)
	return nil
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required env var not set", "key", key)
		os.Exit(1)
	}
	return v
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
