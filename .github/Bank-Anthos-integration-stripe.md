# Stripe Integration Guide for Project Payment Gateway

This guide provides a comprehensive overview of how to integrate Stripe into the existing `project-payment-gateway` scaffold. It covers everything from configuration to handling payments and webhooks, following best practices for a robust and secure integration.

## 1. Introduction

The `project-payment-gateway` is designed to be a flexible and extensible platform for processing payments. This guide focuses on integrating Stripe as a payment provider, leveraging the existing structure of the application to create a seamless and scalable solution.

## 2. Prerequisites

Before you begin, ensure you have the following:

- A Stripe account.
- Your Stripe API keys (publishable and secret).
- Go 1.18 or higher installed on your local machine.
- Docker and Docker Compose for running the application and its dependencies.

## 3. Configuration

Properly configuring your Stripe integration is crucial for security and maintainability. Follow these steps to set up your configuration:

### 3.1. Environment Variables

Store your Stripe API keys and other configuration settings as environment variables. This prevents sensitive information from being hardcoded in your application. Add the following variables to your `.env` file:

```bash
STRIPE_API_KEY="your_stripe_secret_key"
STRIPE_WEBHOOK_SECRET="your_stripe_webhook_secret"
```

### 3.2. Loading Configuration

The application uses a dedicated `config` package to load environment variables. Ensure that the `config.go` file includes the necessary fields for your Stripe configuration:

```go
package config

import (
	"github.com/kelseyhightower/envconfig"
)

// Config holds the application's configuration.
type Config struct {
	// ... other config fields
	StripeAPIKey       string `envconfig:"STRIPE_API_KEY" required:"true"`
	StripeWebhookSecret string `envconfig:"STRIPE_WEBHOOK_SECRET" required:"true"`
}

// FromEnv loads the configuration from environment variables.
func FromEnv() (*Config, error) {
	var cfg Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
```

## 4. Stripe Client

The Stripe client is responsible for all communication with the Stripe API. Here's how to set it up:

### 4.1. Client Initialization

Create a new file `internal/provider/stripe/client.go` to initialize the Stripe client. This file should include a function that returns a new Stripe client instance:

```go
package stripe

import (
	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/client"
)

// NewClient creates a new Stripe client.
func NewClient(apiKey string) *client.API {
	return client.New(apiKey, nil)
}
```

### 4.2. Integrating the Client

In your `main.go` file, initialize the Stripe client and pass it to your payment service:

```go
package main

import (
	// ... other imports
	"github.com/user/project-payment-gateway/internal/platform/config"
	"github.com/user/project-payment-gateway/internal/provider/stripe"
)

func main() {
	// ... other setup
	cfg, err := config.FromEnv()
	if err != nil {
		// ... handle error
	}

	stripeClient := stripe.NewClient(cfg.StripeAPIKey)
	// ... pass stripeClient to your payment service
}
```

## 5. Payment Intent Handling

Stripe's Payment Intents API is used to manage the lifecycle of a payment. Here's how to integrate it into your payment service:

### 5.1. Creating a Payment Intent

In your `internal/application/payment/service.go` file, add a method to create a new payment intent:

```go
package payment

import (
	// ... other imports
	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/paymentintent"
)

// ... PaymentService struct

// CreatePaymentIntent creates a new Stripe Payment Intent.
func (s *PaymentService) CreatePaymentIntent(amount int64, currency string) (*stripe.PaymentIntent, error) {
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(amount),
		Currency: stripe.String(currency),
	}

	pi, err := paymentintent.New(params)
	if err != nil {
		return nil, err
	}

	return pi, nil
}
```

### 5.2. Handling the Payment in Your Handler

In your `internal/transport/http/handlers/payment.go` file, call the `CreatePaymentIntent` method and return the client secret to the client:

```go
package handlers

import (
	// ... other imports
	"net/http"
)

// ... PaymentHandler struct

// CreatePayment handles the creation of a new payment.
func (h *PaymentHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	// ... parse request body
	pi, err := h.paymentService.CreatePaymentIntent(req.Amount, req.Currency)
	if err != nil {
		// ... handle error
		return
	}

	// ... return client secret to the client
}
```

## 6. Webhook Integration

Webhooks are essential for receiving real-time updates from Stripe about the status of a payment. Here's how to set up a webhook handler:

### 6.1. Webhook Handler

Create a new file `internal/provider/stripe/webhook.go` to handle incoming webhooks:

```go
package stripe

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/webhook"
)

// HandleWebhook handles incoming Stripe webhooks.
func HandleWebhook(w http.ResponseWriter, r *http.Request, webhookSecret string) {
	const MaxBodyBytes = int64(65536)
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	payload, err := ioutil.ReadAll(r.Body)
	if err != nil {
		// ... handle error
		return
	}

	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), webhookSecret)
	if err != nil {
		// ... handle error
		return
	}

	switch event.Type {
	case "payment_intent.succeeded":
		var paymentIntent stripe.PaymentIntent
		err := json.Unmarshal(event.Data.Raw, &paymentIntent)
		if err != nil {
			// ... handle error
			return
		}
		// ... handle successful payment
	case "payment_intent.payment_failed":
		var paymentIntent stripe.PaymentIntent
		err := json.Unmarshal(event.Data.Raw, &paymentIntent)
		if err != nil {
			// ... handle error
			return
		}
		// ... handle failed payment
	default:
		// ... handle other event types
	}

	w.WriteHeader(http.StatusOK)
}
```

### 6.2. Adding the Webhook Route

In your `internal/transport/http/router.go` file, add a new route for the webhook handler:

```go
package http

import (
	// ... other imports
	"github.com/user/project-payment-gateway/internal/provider/stripe"
)

// ... other routes

// Webhook route
router.HandleFunc("/webhooks/stripe", func(w http.ResponseWriter, r *http.Request) {
	stripe.HandleWebhook(w, r, cfg.StripeWebhookSecret)
}).Methods("POST")
```

## 7. Testing

Thoroughly testing your Stripe integration is crucial to ensure it's working correctly. Here are some testing strategies:

### 7.1. Unit Tests

Write unit tests for your payment service and webhook handler. Use a mock Stripe client to simulate API calls and test different scenarios.

### 7.2. Integration Tests

Write integration tests to verify the end-to-end payment flow. Use Stripe's test API keys and test payment methods to simulate real payments.

### 7.3. End-to-End Tests

Use a tool like Cypress or Playwright to write end-to-end tests that simulate a user making a payment through your application.

## 8. Conclusion

By following this guide, you can successfully integrate Stripe into your `project-payment-gateway` and create a robust and secure payment processing solution. Remember to always follow best practices for security and maintainability, and to thoroughly test your integration before deploying to production.

