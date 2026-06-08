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
}

// Load loads the configuration from environment variables
func Load() (*Config, error) {
	currenciesStr := getEnv("SUPPORTED_CURRENCIES", "USD,EUR,GBP")
	supportedCurrencies := strings.Split(currenciesStr, ",")
	for i, curr := range supportedCurrencies {
		supportedCurrencies[i] = strings.TrimSpace(strings.ToUpper(curr))
	}
	return &Config{
		Port:                getEnv("PORT", "8080"),
		LogLevel:            getEnv("LOG_LEVEL", "info"),
		SupportedCurrencies: supportedCurrencies,
		DatabaseURL:         getEnv("DATABASE_URL", ""),
		StripeAPIKey:        getEnv("STRIPE_API_KEY", ""),
		StripeWebhookSecret: getEnv("STRIPE_WEBHOOK_SECRET", ""),
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

	return nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
