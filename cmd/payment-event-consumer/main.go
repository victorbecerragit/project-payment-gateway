package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	metricsMux := http.NewServeMux()
	metricsMux.Handle("GET /metrics", promhttp.Handler())
	metricsSrv := &http.Server{Addr: ":9090", Handler: metricsMux}
	go func() {
		slog.Info("metrics server starting", "addr", ":9090")
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("metrics server failed", "error", err)
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = metricsSrv.Shutdown(shutdownCtx)
	}()

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
