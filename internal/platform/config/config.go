package config

import (
	"os"
	"strings"
)

// Config holds the application configuration
type Config struct {
	Port     string
	LogLevel string
	SupportedCurrencies []string
}

// Load loads the configuration from environment variables
func Load() *Config {
	currenciesStr := getEnv("SUPPORTED_CURRENCIES", "USD,EUR,GBP")
	supportedCurrencies := strings.Split(currenciesStr, ",")
	for i, curr := range supportedCurrencies {
		supportedCurrencies[i] = strings.TrimSpace(strings.ToUpper(curr))
	}
	return &Config{
		Port:     getEnv("PORT", "8080"),
		LogLevel: getEnv("LOG_LEVEL", "info"),
		SupportedCurrencies: supportedCurrencies,
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
