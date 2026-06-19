# Kafka Event Bus — Payment Lifecycle Events

## 1. Overview

The payment gateway publishes structured lifecycle events to Apache Kafka after each state transition. Kafka decouples the synchronous payment API from downstream consumers — audit logging, analytics, notifications — without adding latency to the request path. Kafka runs on Kubernetes managed by the [Strimzi operator](https://strimzi.io) in KRaft mode (no ZooKeeper).

## 2. Architecture

```
┌──────────────────────────────────────────────────────────────────────────┐
│                          Kubernetes Cluster                              │
│                                                                          │
│  ┌──────────────┐    HTTP     ┌──────────────────┐                      │
│  │   Client     │───────────▶│  Payment Gateway  │                      │
│  └──────────────┘            │  (Go API)         │                      │
│                              └────────┬──────────┘                      │
│                                       │ publish                          │
│                                       ▼                                  │
│                         ┌─────────────────────────┐                     │
│                         │   Apache Kafka (Strimzi) │                     │
│                         │   payment-events topic    │                     │
│                         └────────────┬────────────┘                     │
│                                      │ consume                          │
│                                      ▼                                   │
│                         ┌──────────────────────────┐                    │
│                         │  Event Consumer (Audit)   │                    │
│                         │  structured slog output   │                    │
│                         └──────────────────────────┘                    │
└──────────────────────────────────────────────────────────────────────────┘
```

**Key properties:**

- **Producer** (`internal/platform/events`) — publishes after `CreatePayment` and `ProcessEvent`
- **Topic** `payment-events` — 3 partitions, 7-day retention, ephemeral storage
- **Consumer** (`cmd/payment-event-consumer`) — audit log worker, consumer group `payment-audit-consumer`

## 3. Event Contract

All events are JSON objects written to the `payment-events` topic.

### payment.created

```json
{
  "event_id": "pay_1781860399342230203_ea4f9f0a",
  "event_type": "payment.created",
  "payment_id": "pay_1781860398579005172_b4d63d57",
  "provider": "stripe",
  "amount": 49.99,
  "currency": "USD",
  "status": "pending",
  "created_at": "2026-06-19T09:13:19.342Z"
}
```

### payment.completed

```json
{
  "event_id": "pay_1781860407780972876_bfa0a344",
  "event_type": "payment.completed",
  "payment_id": "pay_1781860404187152859_3244c849",
  "provider": "stripe",
  "amount": 49.99,
  "currency": "USD",
  "status": "completed",
  "created_at": "2026-06-19T09:13:27.780Z"
}
```

### payment.failed

```json
{
  "event_id": "pay_1781860409178207845_1d1b3971",
  "event_type": "payment.failed",
  "payment_id": "pay_1781860408566924098_6e91273b",
  "provider": "stripe",
  "amount": 49.99,
  "currency": "USD",
  "status": "failed",
  "created_at": "2026-06-19T09:13:29.178Z"
}
```

### payment.refunded

```json
{
  "event_id": "pay_1781860414118305975_009b7083",
  "event_type": "payment.refunded",
  "payment_id": "pay_1781860413507874715_3c8b1b6c",
  "provider": "stripe",
  "amount": 49.99,
  "currency": "USD",
  "status": "refunded",
  "created_at": "2026-06-19T09:13:34.118Z"
}
```

## 4. Topics Reference

| Topic | Partitions | Retention | Purpose |
|-------|-----------|-----------|---------|
| `payment-events` | 3 | 7 days | Payment lifecycle events (created, completed, failed, refunded) |
| `payment-events-dlq` | 3 | 30 days | Failed events — retry exhausted or permanently invalid |

## 5. Running the Kafka Stack

### Deploy the Kafka cluster

```bash
make kafka-up
```

This applies the Strimzi operator, Kafka cluster (KRaft), and the `payment-events` topic. Wait for the operator to be ready before proceeding.

### Deploy the event consumer

```bash
# Build consumer image
docker build -t payment-event-consumer:latest -f cmd/payment-event-consumer/Dockerfile .
kind load docker-image payment-event-consumer:latest --name payment-demo

# Deploy
kubectl apply -k k8s/event-consumer/
kubectl wait --for=condition=ready pod -l app=payment-event-consumer -n payment-system --timeout=60s
```

### Trigger a payment

Use the Stripe trigger job or create a payment via the API:

```bash
# Via API
curl -X POST http://payment-gateway/api/v1/payments \
  -H "Content-Type: application/json" \
  -H "X-Idempotency-Key: demo-$(date +%s)" \
  -d '{"amount":49.99,"currency":"USD","description":"Kafka demo","customer_id":"cust_kafka"}'
```

### Watch events

```bash
# Raw Kafka output
make kafka-consume-raw

# Consumer structured logs
make consumer-logs
```

## 6. Consumer Group Monitoring

Check consumer group lag:

```bash
make consumer-lag
```

**Expected output (all partitions LAG = 0):**

```
GROUP                  TOPIC           PARTITION  CURRENT-OFFSET  LOG-END-OFFSET  LAG
payment-audit-consumer payment-events  0          5               5               0
payment-audit-consumer payment-events  1          3               3               0
payment-audit-consumer payment-events  2          2               2               0
```

## 7. Dead Letter Queue

Failed events are routed to `payment-events-dlq` with a structured envelope containing the
original event, error reason, retry count, and failure timestamp. See [docs/DLQ.md](DLQ.md)
for the full error classification, replay workflow, and monitoring commands.

## 8. Interview Talking Points

- **Event-driven decoupling** — the API publishes events without knowing who consumes them. Adding a new consumer (analytics, notifications) requires zero changes to the payment service.
- **Consumer groups** — multiple consumer instances share partitions automatically. Adding horizontal scale to the audit worker is a replica bump in the Deployment.
- **Kubernetes-native Kafka** — Strimzi operator manages the full Kafka lifecycle (upgrades, topic management, node pools) as Kubernetes CRDs. No Helm charts, no manual brokers.
- **Structured event contracts** — every event carries `event_id`, `event_type`, `payment_id`, and full payload. Consumers can validate against a schema; producers can evolve independently.
- **Non-blocking publish** — Kafka publish errors are logged but never block the payment flow. The API always responds within its SLA regardless of broker availability.
