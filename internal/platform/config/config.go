package config

import (
	"os"
)

// Config holds the application configuration
type Config struct {
	Port     string
	LogLevel string
}

// Load loads the configuration from environment variables
func Load() *Config {
	return &Config{
		Port:     getEnv("PORT", "8080"),
		LogLevel: getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
