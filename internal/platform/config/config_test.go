package config

import (
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid development config",
			config: Config{
				Port:                "8080",
				SupportedCurrencies: []string{"USD", "EUR"},
				APIRateLimit:        10.0,
				APIBurst:            20,
				WebhookRateLimit:    50.0,
				WebhookBurst:        100,
			},
			wantErr: false,
		},
		{
			name: "valid production config with postgres and stripe",
			config: Config{
				Port:                "443",
				SupportedCurrencies: []string{"USD"},
				DatabaseURL:         "postgres://user:pass@localhost:5432/db",
				StripeAPIKey:        "sk_test_123",
				StripeWebhookSecret: "whsec_456",
				APIRateLimit:        10.0,
				APIBurst:            20,
				WebhookRateLimit:    50.0,
				WebhookBurst:        100,
			},
			wantErr: false,
		},
		{
			name: "valid production config with postgresql prefix",
			config: Config{
				Port:                "8080",
				SupportedCurrencies: []string{"USD"},
				DatabaseURL:         "postgresql://user:pass@localhost:5432/db",
				APIRateLimit:        10.0,
				APIBurst:            20,
				WebhookRateLimit:    50.0,
				WebhookBurst:        100,
			},
			wantErr: false,
		},
		{
			name: "invalid port - non numeric",
			config: Config{
				Port:                "abc",
				SupportedCurrencies: []string{"USD"},
			},
			wantErr: true,
		},
		{
			name: "invalid port - too low",
			config: Config{
				Port:                "0",
				SupportedCurrencies: []string{"USD"},
			},
			wantErr: true,
		},
		{
			name: "invalid port - too high",
			config: Config{
				Port:                "70000",
				SupportedCurrencies: []string{"USD"},
			},
			wantErr: true,
		},
		{
			name: "empty currencies",
			config: Config{
				Port:                "8080",
				SupportedCurrencies: []string{},
			},
			wantErr: true,
		},
		{
			name: "currencies with empty string",
			config: Config{
				Port:                "8080",
				SupportedCurrencies: []string{""},
			},
			wantErr: true,
		},
		{
			name: "invalid database url protocol",
			config: Config{
				Port:                "8080",
				SupportedCurrencies: []string{"USD"},
				DatabaseURL:         "mysql://user:pass@localhost:3306/db",
			},
			wantErr: true,
		},
		{
			name: "stripe key without secret",
			config: Config{
				Port:                "8080",
				SupportedCurrencies: []string{"USD"},
				StripeAPIKey:        "sk_test_123",
			},
			wantErr: true,
		},
		{
			name: "stripe secret without key",
			config: Config{
				Port:                "8080",
				SupportedCurrencies: []string{"USD"},
				StripeWebhookSecret: "whsec_456",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.config.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}