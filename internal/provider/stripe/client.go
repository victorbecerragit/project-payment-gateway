package stripe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/victorbecerragit/project-payment-gateway/internal/platform/slogext"
	"github.com/victorbecerragit/project-payment-gateway/internal/platform/tracing"
	"github.com/victorbecerragit/project-payment-gateway/internal/provider"
)

// Config holds Stripe adapter configuration options.
type Config struct {
	APIKey        string
	BaseURL       string
	WebhookSecret string
}

// StripeProvider implements payment provider interaction for Stripe.
type StripeProvider struct {
	config Config
	client *http.Client // HTTP client for making requests to Stripe API
	tracer tracing.Tracer
}

// NewStripeProvider creates a new StripeProvider adapter instance.
func NewStripeProvider(config Config, tracer tracing.Tracer) *StripeProvider {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.stripe.com"
	}
	if tracer == nil {
		tracer = tracing.NewNoOpTracer()
	}
	return &StripeProvider{
		config: config,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		tracer: tracer,
	}
}


// Name returns the provider identifier.
func (p *StripeProvider) Name() string {
	return "stripe"
}

// CreatePayment turns a generic domain/gateway payment intent into Stripe PaymentIntent creation request,
// serializes it as application/x-www-form-urlencoded, and calls Stripe API.
func (p *StripeProvider) CreatePayment(ctx context.Context, req *provider.CreatePaymentRequest) (*provider.CreatePaymentResponse, error) {
	if req == nil {
		return nil, &provider.ErrProviderError{
			Provider: p.Name(),
			Message:  "request is nil",
			Code:     "invalid_request",
		}
	}

	ctx, span := p.tracer.StartSpan(ctx, "stripe.CreatePayment")
	defer span.End()

	endpoint := fmt.Sprintf("%s/v1/payment_intents", p.config.BaseURL)

	// Stripe API uses custom x-www-form-urlencoded payloads
	form := url.Values{}
	form.Set("amount", strconv.FormatInt(req.Amount, 10))
	form.Set("currency", strings.ToLower(req.Currency))
	form.Set("description", req.Description)
	form.Set("metadata[payment_id]", req.PaymentID)
	form.Set("metadata[customer_id]", req.CustomerID)
	form.Set("metadata[idempotency_key]", req.IdempotencyKey)

	// If card token is present, we can configure it onto the intent request
	if req.PaymentMethod != nil {
		if tokenStr, ok := req.PaymentMethod.(string); ok && tokenStr != "" {
			form.Set("payment_method", tokenStr)
		}
	}

	// Standard confirm flow for simple Card API integrations
	form.Set("confirm", "true")
	form.Set("payment_method_types[]", "card")

	span.SetAttribute("payment.id", req.PaymentID)
	span.SetAttribute("amount", req.Amount)
	span.SetAttribute("currency", req.Currency)
	span.SetAttribute("idempotency_key", req.IdempotencyKey)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, &provider.ErrProviderError{
			Provider: p.Name(),
			Message:  fmt.Sprintf("failed to create http request: %v", err),
			Code:     "request_creation_failed",
		}
	}

	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.config.APIKey))
	if req.IdempotencyKey != "" {
		httpReq.Header.Set("Idempotency-Key", req.IdempotencyKey)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, &provider.ErrProviderError{
			Provider: p.Name(),
			Message:  fmt.Sprintf("http request execution failed: %v", err),
			Code:     "network_error",
		}
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &provider.ErrProviderError{
			Provider: p.Name(),
			Message:  fmt.Sprintf("failed to read response body: %v", err),
			Code:     "read_error",
		}
	}
	slogext.Ctx(ctx).Debug("stripe API response", "status_code", resp.StatusCode, "body", string(bodyBytes))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var stripeErr struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
			} `json:"error"`
		}
		_ = json.Unmarshal(bodyBytes, &stripeErr)
		msg := stripeErr.Error.Message
		if msg == "" {
			msg = fmt.Sprintf("stripe returned status code %d: %s", resp.StatusCode, string(bodyBytes))
		}
		return nil, &provider.ErrProviderError{
			Provider: p.Name(),
			Code:     stripeErr.Error.Code,
			Message:  msg,
			Code:     stripeErr.Error.Code,
		}
	}

	var paymentIntent struct {
		ID           string                 `json:"id"`
		Status       string                 `json:"status"`
		Amount       int64                  `json:"amount"`
		Currency     string                 `json:"currency"`
		ClientSecret string                 `json:"client_secret"`
		Metadata     map[string]interface{} `json:"metadata"`
	}

	if err := json.Unmarshal(bodyBytes, &paymentIntent); err != nil {
		return nil, &provider.ErrProviderError{
			Provider: p.Name(),
			Message:  fmt.Sprintf("failed to unmarshal stripe response: %v", err),
			Code:     "unmarshal_error",
		}
	}

	span.SetAttribute("provider.payment_intent_id", paymentIntent.ID)
	span.SetAttribute("provider.status", paymentIntent.Status)
	providerData := map[string]interface{}{
		"provider":      "stripe",
		"payment_intent": paymentIntent.ID,
		"client_secret":  paymentIntent.ClientSecret,
	}

	return &provider.CreatePaymentResponse{
		TransactionID:  paymentIntent.ID,
		ProviderStatus: paymentIntent.Status,
		ProviderData:   providerData,
	}, nil
}
