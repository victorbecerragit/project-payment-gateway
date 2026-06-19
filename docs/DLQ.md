# Dead Letter Queue (DLQ) — Payment Events

## Overview

The event consumer classifies handler failures into **transient** and **permanent** errors.
Transient errors (DB timeout, network blip) are retried up to 3 times with exponential backoff.
Permanent errors (invalid JSON, missing `payment_id`) and exhausted retries are routed to the
`payment-events-dlq` topic with a structured envelope for forensic investigation.

```
payment-events → Consumer → handler()
                               │
               ┌───────────────┼───────────────┐
               │               │               │
            success      transient err    permanent err
               │               │               │
           commit offset   retry 3×        DLQ immediately
                               │               │
                          exhausted?        DLQ envelope
                               │               │
                            DLQ envelope    commit offset
                               │
                            commit offset
```

## Error Classification

| Error Class | Examples | Strategy |
| :-- | :-- | :-- |
| **Transient** | DB timeout, broker restart, network blip | Retry up to 3× with exponential backoff (500ms × attempt²) |
| **Permanent** | Invalid JSON, missing `payment_id`, unknown `event_type` | DLQ immediately, no retries |
| **Business logic** | Payment not found, invalid state transition | DLQ immediately + structured log |

## DLQ Envelope

Every message in `payment-events-dlq` is wrapped in a `DLQEnvelope`:

```json
{
  "original_event": {
    "event_id": "pay_1781860399342230203_ea4f9f0a",
    "event_type": "payment.created",
    "payment_id": "pay_1781860398579005172_b4d63d57",
    "provider": "stripe",
    "amount": 49.99,
    "currency": "USD",
    "status": "pending",
    "created_at": "2026-06-19T09:13:19.342Z"
  },
  "original_topic": "payment-events",
  "consumer_group": "payment-audit-consumer",
  "error_reason": "unmarshal error: invalid character ',' looking for beginning of value",
  "retry_count": 0,
  "failed_at": "2026-06-19T09:22:03.123Z"
}
```

| Field | Description |
|---|---|
| `original_event` | The full `PaymentEvent` that failed |
| `original_topic` | Source topic (`payment-events`) |
| `consumer_group` | Consumer group that failed (`payment-audit-consumer`) |
| `error_reason` | Human-readable error string |
| `retry_count` | 0 for permanent errors, 3 for exhausted retries |
| `failed_at` | ISO 8601 timestamp of when the event was routed to DLQ |

## Topics Reference

| Topic | Partitions | Retention | Purpose |
|-------|-----------|-----------|---------|
| `payment-events-dlq` | 3 | 30 days | Failed events awaiting investigation and replay |

## Monitoring

### Watch DLQ in real time

```bash
make kafka-dlq-watch
```

### Check DLQ topic details

```bash
make kafka-dlq-count
```

### Replay DLQ events

After root cause is fixed, replay original events back into `payment-events`:

```bash
make kafka-dlq-replay
```

This extracts `original_event` from each DLQ envelope and publishes it back to the main topic.

## Replay Workflow

1. Identify the root cause from `error_reason` in DLQ envelopes
2. Fix the bug (e.g., handle new event type, fix JSON schema)
3. Redeploy the consumer
4. Run `make kafka-dlq-replay` to reprocess failed events
5. Verify with `make kafka-dlq-count` that DLQ is drained

## Interview Talking Points

- **Never lose events** — every failure is preserved in the DLQ with full context for audit compliance
- **Error classification** — distinguishing transient from permanent errors prevents retry storms on bad data
- **Exponential backoff** — 500ms × attempt² avoids overwhelming a struggling dependency
- **Replay capability** — DLQ is not a black hole; events can be reprocessed after root cause is fixed
- **Structured envelope** — `DLQEnvelope` carries enough metadata for automated alerting and forensic analysis without raw log diving
