package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds the application configuration
type Config struct {
	Port                string
	LogLevel            string
	SupportedCurrencies []string
	DatabaseURL         string
	StripeAPIKey        string
	StripeWebhookSecret string
	APIRateLimit        float64
	APIBurst            int
	WebhookRateLimit    float64
	WebhookBurst        int
}

// Load loads the configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists. This will not override existing environment variables.
	if data, err := os.ReadFile(".env"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if k, v, ok := strings.Cut(line, "="); ok && os.Getenv(strings.TrimSpace(k)) == "" {
				_ = os.Setenv(strings.TrimSpace(k), strings.TrimSpace(v))
			}
		}
	}

	currenciesStr := getEnv("SUPPORTED_CURRENCIES", "USD,EUR,GBP")
	supportedCurrencies := strings.Split(currenciesStr, ",")
	for i, curr := range supportedCurrencies {
		supportedCurrencies[i] = strings.TrimSpace(strings.ToUpper(curr))
	}

	cfg := &Config{
		Port:                getEnv("PORT", "8080"),
		LogLevel:            getEnv("LOG_LEVEL", "info"),
		SupportedCurrencies: supportedCurrencies,
		DatabaseURL:         getEnv("DATABASE_URL", ""),
		StripeAPIKey:        getEnv("STRIPE_API_KEY", ""),
		StripeWebhookSecret: getEnv("STRIPE_WEBHOOK_SECRET", ""),
		APIRateLimit:        getEnvFloat("API_RATE_LIMIT", 10.0),
		APIBurst:            getEnvInt("API_BURST", 20),
		WebhookRateLimit:    getEnvFloat("WEBHOOK_RATE_LIMIT", 50.0),
		WebhookBurst:        getEnvInt("WEBHOOK_BURST", 100),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate ensures the configuration is semantically correct and fails fast on invalid production settings.
func (c *Config) Validate() error {
	// Validate Port
	port, err := strconv.Atoi(c.Port)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid PORT %q: must be a number between 1 and 65535", c.Port)
	}

	// Validate Currencies
	if len(c.SupportedCurrencies) == 0 || (len(c.SupportedCurrencies) == 1 && c.SupportedCurrencies[0] == "") {
		return fmt.Errorf("SUPPORTED_CURRENCIES cannot be empty")
	}

	// Validate Postgres connection string if provided
	if c.DatabaseURL != "" {
		if !strings.HasPrefix(c.DatabaseURL, "postgres://") && !strings.HasPrefix(c.DatabaseURL, "postgresql://") {
			return fmt.Errorf("DATABASE_URL must be a valid postgres connection string")
		}
	}

	// Ensure Stripe is fully configured if a key is provided
	if (c.StripeAPIKey != "" && c.StripeWebhookSecret == "") || (c.StripeAPIKey == "" && c.StripeWebhookSecret != "") {
		return fmt.Errorf("Stripe integration requires both STRIPE_API_KEY and STRIPE_WEBHOOK_SECRET")
	}

	// Validate Rate Limiting settings
	if c.APIRateLimit <= 0 {
		return fmt.Errorf("API_RATE_LIMIT must be a positive number")
	}
	if c.APIBurst <= 0 {
		return fmt.Errorf("API_BURST must be a positive integer")
	}
	if c.WebhookRateLimit <= 0 {
		return fmt.Errorf("WEBHOOK_RATE_LIMIT must be a positive number")
	}
	if c.WebhookBurst <= 0 {
		return fmt.Errorf("WEBHOOK_BURST must be a positive integer")
	}

	return nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// getEnvInt parses an environment variable as an integer, returning a fallback if not found or invalid.
func getEnvInt(key string, fallback int) int {
	if valueStr, exists := os.LookupEnv(key); exists {
		if value, err := strconv.Atoi(valueStr); err == nil {
			return value
		}
	}
	return fallback
}

// getEnvFloat parses an environment variable as a float64, returning a fallback if not found or invalid.
func getEnvFloat(key string, fallback float64) float64 {
	if valueStr, exists := os.LookupEnv(key); exists {
		if value, err := strconv.ParseFloat(valueStr, 64); err == nil {
			return value
		}
	}
	return fallback
}
