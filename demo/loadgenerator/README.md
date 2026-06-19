# Payment Gateway Load Generator

Generates traffic against the Payment Gateway for demo and validation purposes.

## Scenarios

| Scenario | Description | External deps |
|----------|-------------|--------------|
| `mock` | Creates payments using in-memory provider. No external calls. | None |
| `stripe` | Creates real Stripe PaymentIntents. Optionally triggers webhooks. | Stripe API key |
| `stripe-fail` | Creates payments then triggers `payment_failed` webhooks. | Stripe CLI |
| `multi-state` | Creates payments across all states (pending, completed, failed, cancelled) using mock webhooks. Exercises Prometheus metrics and Grafana dashboard. | None |
| `stress` | Rapid-fire mock payments to test rate limiting and throughput. | None |

## Usage

```bash
# Default: 5 mock payments
./loadgen.sh mock

# Multi-state: creates payments across all 4 states (no external deps)
./loadgen.sh multi-state

# Multi-state with 12 payments (3 per state)
REQUESTS=12 ./loadgen.sh multi-state

# Stripe happy path — creates payments only
REQUESTS=3 ./loadgen.sh stripe

# Stripe happy path — creates payments AND triggers webhooks
TRIGGER_WEBHOOKS=true REQUESTS=3 ./loadgen.sh stripe

# Stripe failure path
REQUESTS=3 ./loadgen.sh stripe-fail

# Stress test (50 requests, no delay)
REQUESTS=50 ./loadgen.sh stress
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `GATEWAY_URL` | `http://payment-gateway` | Payment gateway base URL (Kubernetes service) |
| `REQUESTS` | `5` | Number of payments to create |
| `DELAY` | `0.5` | Seconds between requests |
| `TRIGGER_WEBHOOKS` | `false` | Auto-trigger webhooks after creation (stripe mode only) |
| `WEBHOOK_SECRET` | *(auto-detect)* | Stripe webhook secret for `multi-state` scenario. Falls back to `kubectl get secret` |

## Prerequisites

- `curl` and `python3` must be available
- For `stripe` and `stripe-fail` scenarios: `stripe` CLI authenticated and `stripe listen` running in a separate terminal
- For `stripe` + `TRIGGER_WEBHOOKS=true`: `stripe listen --forward-to localhost:8080/api/v1/webhooks/payment` must be active
