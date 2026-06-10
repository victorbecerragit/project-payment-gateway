# Payment Gateway Load Generator

This directory contains scripts and tooling to generate traffic against the Payment Gateway for demo and validation purposes. It supports both `mock` scenarios and `stripe` scenarios.

## Usage

You can run the bash load generator directly if you have `curl` and `uuidgen` installed.

### Mock Mode (Default)
Generates traffic against the mock backend provider.
```bash
./loadgen.sh mock
```

### Stripe Mode
Generates traffic aimed at the Stripe provider implementation. Generates realistic idempotency keys required for external PSP calls.
```bash
./loadgen.sh stripe
```

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `GATEWAY_URL` | `http://localhost:8080` | URL of the Payment Gateway |
| `CONCURRENCY` | `1` | Number of concurrent workers |
| `REQUESTS` | `10` | Number of payment creation requests per worker |
| `TRIGGER_WEBHOOKS` | `false` | If `true` and in `stripe` mode, attempts to trigger `payment_intent.succeeded` asynchronously via the Stripe CLI |

## Kubernetes Deployment
To run this continuously in a cluster for dashboards:
You can wrap `loadgen.sh` in a simple alpine container and deploy it as a Kubernetes `Job` or `Deployment` passing the appropriate `GATEWAY_URL`.

## Webhook Helper Scripts

When running in `stripe` mode without `TRIGGER_WEBHOOKS=true`, the Gateway creates payments that stay in `PENDING`/`PROCESSING` states. You can use the Stripe CLI manually to push webhooks through your system to observe completed and failed interactions:

```bash
stripe trigger payment_intent.succeeded
stripe trigger payment_intent.payment_failed
stripe trigger payment_intent.canceled
```