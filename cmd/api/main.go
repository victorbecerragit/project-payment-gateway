package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	apphealth "github.com/victorbecerragit/project-payment-gateway/internal/application/health"
	apppayment "github.com/victorbecerragit/project-payment-gateway/internal/application/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/domain/payment" // Import domain payment
	"github.com/victorbecerragit/project-payment-gateway/internal/platform/config"
	"github.com/victorbecerragit/project-payment-gateway/internal/platform/events"
	"github.com/victorbecerragit/project-payment-gateway/internal/platform/slogext"
	"github.com/victorbecerragit/project-payment-gateway/internal/platform/tracing"
	"github.com/victorbecerragit/project-payment-gateway/internal/provider"
	"github.com/victorbecerragit/project-payment-gateway/internal/provider/stripe"
	"github.com/victorbecerragit/project-payment-gateway/internal/provider/webhook"
	"github.com/victorbecerragit/project-payment-gateway/internal/storage/inmemory"
	"github.com/victorbecerragit/project-payment-gateway/internal/storage/postgres"
	transport "github.com/victorbecerragit/project-payment-gateway/internal/transport/http"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/handlers"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/middleware"
)

func main() {
	cfg, err := config.Load()

	// Fail fast on invalid configuration
	if err != nil {
		slog.Error("configuration error", "error", err)
		os.Exit(1)
	}

	// Initialize structured logger
	var level slog.Level
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	// Initialize domain-level configurations
	payment.SetSupportedCurrencies(cfg.SupportedCurrencies)

	// Initialize Tracer
	appTracer := tracing.NewLoggerTracer(logger)

	// Initialize Repositories
	var paymentRepo payment.Repository
	if strings.ToLower(cfg.StorageType) == "postgres" {
		paymentRepo = postgres.NewRepository(context.Background(), cfg.DatabaseURL, appTracer)
		slogext.Ctx(context.Background()).Info("using postgres repository", "type", "postgres") // Use slogext for consistency
	} else {
		paymentRepo = inmemory.NewRepository(appTracer)
		slogext.Ctx(context.Background()).Info("using in-memory repository", "type", "inmemory") // Use slogext for consistency
	}

	// Initialize Provider based on configuration flag
	var paymentProvider provider.Provider
	if cfg.StripeAPIKey != "" {
		paymentProvider = stripe.NewStripeProvider(stripe.Config{
			APIKey:        cfg.StripeAPIKey,
			WebhookSecret: cfg.StripeWebhookSecret,
		}, appTracer)
		slogext.Ctx(context.Background()).Info("using stripe payment provider", "provider", "stripe") // Use slogext for consistency
	} else {
		paymentProvider = provider.NewMockProvider(appTracer)
		slogext.Ctx(context.Background()).Info("using mock payment provider", "provider", "mock") // Use slogext for consistency
	}

	// Initialize Publisher based on KAFKA_BROKER env var
	kafkaBroker := os.Getenv("KAFKA_BROKER")
	var pub events.Publisher
	if kafkaBroker != "" {
		pub = events.NewKafkaPublisher(kafkaBroker, "payment-events")
		slogext.Ctx(context.Background()).Info("using kafka publisher", "broker", kafkaBroker)
	} else {
		pub = events.NewNoOpPublisher()
		slogext.Ctx(context.Background()).Info("using noop publisher (KAFKA_BROKER not set)")
	}
	defer pub.Close()

	// Initialize Services
	healthService := apphealth.NewService()
	paymentService := apppayment.NewService(paymentRepo, paymentProvider, appTracer, pub)
	webhookVerifier := webhook.NewMockVerifier()

	// Initialize Handlers
	healthHandler := handlers.NewHealthHandler(healthService)
	paymentHandler := handlers.NewPaymentHandler(paymentService, webhookVerifier)

	// Initialize Prometheus metrics
	requestMetrics := middleware.NewRequestMetrics()

	// Create a context for rate limiters that will be cancelled on shutdown
	rateLimiterCtx, rateLimiterCancel := context.WithCancel(context.Background())
	defer rateLimiterCancel() // Ensure this is called when main exits
	// Setup routes using the new router, which will be wrapped by the correlation ID middleware.
	routerMux := http.NewServeMux()
	transport.SetupRoutes(routerMux, paymentHandler, healthHandler, requestMetrics, cfg, rateLimiterCtx) // Pass cfg and rateLimiterCtx

	// Apply CorrelationIDMiddleware to the entire router
	finalHandler := middleware.CORSMiddleware(middleware.CorrelationIDMiddleware(appTracer, routerMux))

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: finalHandler,
	}

	// Run server in a goroutine
	go func() {
		slogext.Ctx(context.Background()).Info("starting payment gateway server", "port", cfg.Port) // Use slogext for consistency
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slogext.Ctx(context.Background()).Error("server failed to start", "error", err) // Use slogext for consistency
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slogext.Ctx(context.Background()).Info("received shutdown signal", "signal", sig.String()) // Use slogext for consistency

	slogext.Ctx(context.Background()).Info("shutting down server...") // Use slogext for consistency
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slogext.Ctx(context.Background()).Error("server forced to shutdown", "error", err) // Use slogext for consistency
		os.Exit(1)
	}

	slogext.Ctx(context.Background()).Info("closing repository...") // Use slogext for consistency
	paymentRepo.Close()

	slogext.Ctx(context.Background()).Info("server exited gracefully") // Use slogext for consistency
}
