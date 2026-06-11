# Observability Walkthrough — Stripe Demo

What to watch, what it means, and what stakeholders should see during a live demo.

---

## 1. Structured Logs

All logs are JSON, written to stdout. Every request carries a `request_id` that links all log lines within that request.

### Payment creation trace

```bash
docker compose logs payment-gateway | grep -v span_ | grep "pay_20260611xxxxxx\|efa04e0f"
```

**What you see:**

```
{"level":"INFO","msg":"received CreatePayment request",
  "request_id":"efa04e0f","customer_id":"cust_123","amount":99.99,"currency":"USD"}

{"level":"INFO","msg":"payment created successfully",
  "request_id":"efa04e0f","payment_id":"pay_20260611050232","status":"pending","duration_ms":685}
```

Two lines, same `request_id`. The second carries `payment_id` and `duration_ms` — the round-trip time including the Stripe API call.

### Webhook trace

```bash
docker compose logs payment-gateway | grep -v span_ | grep "pay_20260611xxxxxx"
```

**What you see:**

```
{"level":"INFO","msg":"processing webhook event",
  "request_id":"b2374bd1","event_type":"payment.completed","payment_id":"pay_20260611050232"}

{"level":"INFO","msg":"webhook processed successfully",
  "request_id":"b2374bd1","event_type":"payment.completed","payment_id":"pay_20260611050232","duration_ms":12}
```

The `payment_id` links the webhook log to the original creation log. Different `request_id` (different HTTP request) but same `payment_id` — that's the correlation path.

### Log-level filter cheatsheet

```bash
# Errors only
docker compose logs payment-gateway | grep -v span_ | grep '"level":"ERROR"'

# Full payment lifecycle by payment_id
docker compose logs payment-gateway | grep -v span_ | grep "pay_20260611xxxxxx"

# Webhook events only
docker compose logs payment-gateway | grep -v span_ | grep "webhook"

# Slow requests (duration > 500ms — adjust as needed)
docker compose logs payment-gateway | grep -v span_ | python3 -c "
import sys, json
for line in sys.stdin:
    if '| {' not in line: continue
    try:
        d = json.loads(line.split('| ',1)[1])
        if d.get('duration_ms', 0) > 500:
            print(line.strip())
    except: pass
"
```

---

## 2. Metrics Endpoint

```bash
curl -s http://localhost:8080/metrics
```

### Key metrics for the demo

| Metric | What it shows |
|--------|--------------|
| `http_requests_total{method="POST",path="/api/v1/payments",status_code="201"}` | Successful payments created |
| `http_requests_total{method="POST",path="/api/v1/webhooks/payment",status_code="200"}` | Webhooks successfully processed |
| `http_requests_total{method="POST",path="/api/v1/webhooks/payment",status_code="400"}` | Rejected webhooks (bad signature) |
| `http_requests_total{method="POST",path="/api/v1/webhooks/payment",status_code="404"}` | Webhook for unknown payment |
| `http_request_duration_seconds{method="POST",path="/api/v1/payments"}` | Stripe API latency (full histogram) |
| `go_goroutines` | Concurrency health |
| `process_resident_memory_bytes` | Memory footprint |

### Quick metric checks during demo

```bash
# Payment creation success rate
curl -s http://localhost:8080/metrics | grep 'http_requests_total.*payments'

# Webhook success vs error breakdown
curl -s http://localhost:8080/metrics | grep 'http_requests_total.*webhooks'

# Payment creation latency (p50 approximation from histogram)
curl -s http://localhost:8080/metrics | grep 'http_request_duration.*payments.*sum\|_count'
```

**Example output during a healthy demo run:**
```
http_requests_total{method="POST",path="/api/v1/payments",status_code="201"} 3
http_requests_total{method="POST",path="/api/v1/webhooks/payment",status_code="200"} 3
http_request_duration_seconds_sum{method="POST",path="/api/v1/payments",status_code="201"} 1.98
http_request_duration_seconds_count{method="POST",path="/api/v1/payments",status_code="201"} 3
# → average ~660ms per payment (Stripe API round-trip)
```

---

## 3. Correlation: Request → Webhook → DB

The full observability chain for one payment:

```
POST /api/v1/payments
  → log: "payment created" + payment_id + duration_ms     (LOG)
  → metric: http_requests_total{status="201"} ++           (METRIC)
  → DB: payments row, status=pending                       (DB)

stripe trigger → POST /api/v1/webhooks/payment
  → log: "processing webhook event" + payment_id           (LOG)
  → log: "webhook processed successfully" + duration_ms    (LOG)
  → metric: http_requests_total{status="200"} ++           (METRIC)
  → DB: payments row, status=completed, updated_at=now     (DB)

GET /api/v1/payments/{payment_id}
  → response: status=completed                             (API)
  → DB: SELECT confirms updated_at > created_at            (DB)
```

---

## 4. What Stakeholders See During the Demo

Run this sequence live — takes ~60 seconds:

```bash
# Terminal 1: tail logs live
docker compose logs -f payment-gateway | grep -v span_

# Terminal 2: demo commands
IDEM="demo-$(date +%s)"
PAYMENT=$(curl -s -X POST http://localhost:8080/api/v1/payments \
  -H "Content-Type: application/json" \
  -H "X-Idempotency-Key: $IDEM" \
  -d '{"amount":99.99,"currency":"USD","description":"Stripe demo","customer_id":"cust_123"}')
echo $PAYMENT
PAY_ID=$(echo $PAYMENT | python3 -c "import sys,json; print(json.load(sys.stdin)['payment_id'])")

# Show Stripe PI created, status=pending
curl -s http://localhost:8080/api/v1/payments/$PAY_ID

# Trigger — stripe listen must be running
stripe trigger payment_intent.succeeded \
  --override "payment_intent:metadata[payment_id]=$PAY_ID"

# Show status=completed
curl -s http://localhost:8080/api/v1/payments/$PAY_ID

# Show metrics updated
curl -s http://localhost:8080/metrics | grep 'http_requests_total.*webhook\|http_requests_total.*payments'

# Show DB record
docker compose exec db psql -U user -d payment_gateway \
  -c "SELECT id, status, created_at, updated_at FROM payments WHERE id = '$PAY_ID';"
```

**What stakeholders should see:**

| Signal | Before trigger | After trigger |
|--------|---------------|---------------|
| `GET /api/v1/payments/$PAY_ID` | `"status":"pending"` | `"status":"completed"` |
| Log stream | `payment created successfully` | `webhook processed successfully` |
| Metric `webhooks/payment status=200` | 0 | 1 |
| DB `updated_at > created_at` | false | true |

---

## 5. Observability Gaps (current state)

| Gap | Impact | Recommended fix |
|-----|--------|----------------|
| No `payment_id` on the first webhook log line | Hard to filter webhook logs before lookup | Add `payment_id` to `processing webhook event` log after it's resolved |
| No payment-level metrics (counts by status) | Can't chart `completed` vs `failed` over time | Add a `payment_status_transitions_total{from,to}` counter in `ProcessEvent` |
| No distributed tracing export | Spans logged locally only, not queryable | Wire `tracing.go` to OTLP exporter (Jaeger/Tempo) |
| `updated_at` not returned by `GET /api/v1/payments/{payment_id}` | Can't confirm state transition time from API alone | Add `updated_at` to `PaymentResponse` DTO |
