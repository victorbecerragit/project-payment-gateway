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

	"github.com/victorbecerra/kube-refresh/project-payment-gateway/internal/platform/slogext"
	"github.com/victorbecerra/kube-refresh/project-payment-gateway/internal/platform/tracing"
	"github.com/victorbecerra/kube-refresh/project-payment-gateway/internal/transport/http/middleware"
	apphealth "github.com/victorbecerragit/project-payment-gateway/internal/application/health"
	apppayment "github.com/victorbecerragit/project-payment-gateway/internal/application/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/domain/payment" // Import domain payment
	"github.com/victorbecerragit/project-payment-gateway/internal/platform/config"
	"github.com/victorbecerragit/project-payment-gateway/internal/provider"
	"github.com/victorbecerragit/project-payment-gateway/internal/provider/stripe"
	"github.com/victorbecerragit/project-payment-gateway/internal/provider/webhook"
	"github.com/victorbecerragit/project-payment-gateway/internal/storage/inmemory"
	"github.com/victorbecerragit/project-payment-gateway/internal/storage/postgres"
	transport "github.com/victorbecerragit/project-payment-gateway/internal/transport/http"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/handlers"
)

func main() {
	cfg, err := config.Load()

	// Fail fast on invalid configuration
	if err != nil { // Use standard log here as slog might not be fully configured yet
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

	// Initialize Tracer
	appTracer := tracing.NewLoggerTracer(logger)

	// Initialize domain-level configurations
	payment.SetSupportedCurrencies(cfg.SupportedCurrencies)

	// Initialize Repositories
	var paymentRepo payment.Repository
	if cfg.DatabaseURL != "" {
		paymentRepo = postgres.NewRepository(context.Background(), cfg.DatabaseURL, appTracer)
		slogext.Ctx(context.Background()).Info("using postgres repository", "type", "postgres")
	} else {
		paymentRepo = inmemory.NewRepository(appTracer)
		slogext.Ctx(context.Background()).Info("using in-memory repository", "type", "inmemory")
	}

	// Initialize Provider based on configuration flag
	var paymentProvider provider.Provider
	if cfg.StripeAPIKey != "" {
		paymentProvider = stripe.NewStripeProvider(stripe.Config{
			APIKey:        cfg.StripeAPIKey,
			WebhookSecret: cfg.StripeWebhookSecret,
		}, appTracer)
		slogext.Ctx(context.Background()).Info("using stripe payment provider", "provider", "stripe")
	} else {
		paymentProvider = provider.NewMockProvider(appTracer)
		slogext.Ctx(context.Background()).Info("using mock payment provider", "provider", "mock")
	}

	// Initialize Services
	healthService := apphealth.NewService()
	paymentService := apppayment.NewService(paymentRepo, paymentProvider, appTracer)
	webhookVerifier := webhook.NewMockVerifier()

	// Initialize Handlers
	healthHandler := handlers.NewHealthHandler(healthService)
	paymentHandler := handlers.NewPaymentHandler(paymentService, webhookVerifier)

	// Initialize Prometheus metrics
	requestMetrics := middleware.NewRequestMetrics()

	// Setup routes using the new router
	// The router itself will be wrapped by the correlation ID middleware
	routerMux := http.NewServeMux()
	transport.SetupRoutes(routerMux, paymentHandler, healthHandler, requestMetrics)

	// Apply CorrelationIDMiddleware to the entire router
	finalHandler := middleware.CorrelationIDMiddleware(appTracer, routerMux)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: finalHandler,
	}

	// Run server in a goroutine
	go func() { // Use standard log here as slog might not be fully configured yet
		slogext.Ctx(context.Background()).Info("starting payment gateway server", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slogext.Ctx(context.Background()).Error("server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit // Use standard log here as slog might not be fully configured yet
	slogext.Ctx(context.Background()).Info("received shutdown signal", "signal", sig.String())

	slogext.Ctx(context.Background()).Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slogext.Ctx(context.Background()).Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slogext.Ctx(context.Background()).Info("closing repository...")
	paymentRepo.Close()

	slogext.Ctx(context.Background()).Info("server exited gracefully")
}
