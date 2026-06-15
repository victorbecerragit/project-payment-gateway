# DEMO.md — Payment Gateway Interview Demo

Live walkthrough for a ~10 minute demo on kind (Kubernetes).  
Covers: full K8s deploy → payment creation → idempotency → webhook → DB verification → observability.

> **Fully in-cluster** — no local Stripe CLI session needed during the demo.  
> `k8s/tools/stripe-listener.yaml` forwards Stripe events to the gateway inside the cluster.  
> `k8s/tools/stripe-trigger-job.yaml` fires webhooks as a K8s Job.

---

## Prerequisites

| Tool | Check |
|---|---|
| `kind` | `kind version` |
| `kubectl` | `kubectl version --client` |
| `helm` | `helm version` |
| Docker | `docker info` |

---

## 0. One-time cluster bootstrap

Run once per machine. Skip if your kind cluster already exists.

```bash
# Create kind cluster
kind create cluster --name payment-demo --config kind-config.yaml

# Nginx ingress controller
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=90s

# External Secrets Operator
helm repo add external-secrets https://charts.external-secrets.io
helm repo update
helm install external-secrets external-secrets/external-secrets \
  --namespace external-secrets --create-namespace --set installCRDs=true
kubectl wait --namespace external-secrets \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/name=external-secrets \
  --timeout=60s

# Build and load app image into kind (no registry needed)
make docker-build
kind load docker-image payment-gateway:latest --name payment-demo
```

---

## 1. Configure secrets

Edit `k8s/eso/secretstore-literal-fake.yaml` with your Stripe test keys before every demo:

```yaml
spec:
  provider:
    fake:
      data:
        - key: "api_key"
          value: "sk_test_YOUR_KEY"       # Stripe Dashboard → Developers → API keys
        - key: "webhook_secret"
          value: "whsec_YOUR_SECRET"      # Stripe Dashboard → Developers → Webhooks
        - key: "url"
          value: "postgres://pguser:pgpassword@postgres:5432/payments?sslmode=disable"
```

> `k8s/eso/secretstore-literal-fake.yaml` is in `.gitignore` — never commit real values.  
> For production, swap the `SecretStore` backend to AWS Secrets Manager or GCP Secret Manager —  
> the `ExternalSecret` and `Deployment` manifests stay unchanged.  
> See `docs/secrets-management-eso.md` for the full reference.

---

## 2. Deploy the full stack

The deploy uses [Kustomize](https://kustomize.io) overlays under `k8s/kustomize/`.  
ESO secrets are applied first and verified before the rest of the stack starts —  
this prevents the `payment-gateway` pods from crash-looping while the secret syncs.

```bash
# Step 1 — secrets (order matters: ESO must sync before pods start)
kubectl apply -k k8s/kustomize/eso/
kubectl wait --for=condition=Ready externalsecret/payment-gateway-secrets --timeout=30s

# Step 2 — full stack (postgres + payment-gateway + frontend + stripe-listener)
kubectl apply -k k8s/kustomize/base/
kubectl wait --for=condition=ready pod -l app=payment-gateway --timeout=90s
```

Verify all pods:

```bash
kubectl get pods
```

Expected — all `Running`:

```
NAME                               READY   STATUS
frontend-xxx                       1/1     Running
payment-gateway-xxx (x3)           1/1     Running
postgres-0                         1/1     Running
stripe-listener-xxx                1/1     Running
```

> Each component also has a standalone kustomization for selective deploys:  
> `kubectl apply -k k8s/kustomize/payment-gateway/`  
> `kubectl apply -k k8s/kustomize/postgres/`  
> etc.

---

## 3. Access the Application

Since we configured `kind` with port-mappings and updated `/etc/hosts`, the application is available at:
*   **UI:** http://payment-gateway
*   **API:** http://payment-gateway/api
*   *No port-forwarding required.*

---

## 4. Create a payment

### Via UI
Click **Create Payment** in the browser. A new row appears with status `pending`.

### Via CLI
```bash
IDEM="demo-$(date +%s)"

PAYMENT=$(curl -s -X POST http://payment-gateway/api/v1/payments \
  -H "Content-Type: application/json" \
  -H "X-Idempotency-Key: $IDEM" \
  -d '{"amount":99.99,"currency":"USD","description":"Interview demo","customer_id":"cust_demo"}')

echo $PAYMENT | python3 -m json.tool

PAY_ID=$(echo $PAYMENT | python3 -c "import sys,json; print(json.load(sys.stdin)['payment_id'])")
echo "Payment ID: $PAY_ID"
```

**Expected response:**
```json
{
  "payment_id": "pay_20260612xxxxxx",
  "status": "pending",
  "amount": 99.99,
  "currency": "USD",
  "transaction_id": "pi_3Txx..."
}
```

---

## 5. Demonstrate idempotency

Replay the **exact same request** with the same `X-Idempotency-Key`:

```bash
curl -s -X POST http://payment-gateway/api/v1/payments \
  -H "Content-Type: application/json" \
  -H "X-Idempotency-Key: $IDEM" \
  -d '{"amount":99.99,"currency":"USD","description":"Interview demo","customer_id":"cust_demo"}' \
  | python3 -m json.tool
```

**Expected:** same `payment_id` returned — no second Stripe PaymentIntent, no duplicate charge.

> **Interview talking point:** idempotency keys are a core payment systems primitive.  
> On a network timeout, clients retry — without idempotency, a dropped response causes a  
> double charge. The gateway deduplicates by `X-Idempotency-Key` before calling Stripe.

---

## 6. Trigger the Stripe webhook

The `stripe-trigger-job` runs inside the cluster, fetches all `pending` payments from the  
gateway API, and fires a `stripe trigger payment_intent.succeeded` for each one.  
No local Stripe CLI needed — the Job reads `STRIPE_API_KEY` from `payment-gateway-secrets`.

```bash
# Delete any previous run (Jobs are immutable)
kubectl delete job stripe-trigger-demo --ignore-not-found

# Apply — default MODE=drain triggers webhooks for all pending payments
kubectl apply -f k8s/tools/stripe-trigger-job.yaml

# Watch the job complete (~10s)
kubectl logs -l app=stripe-trigger-demo -f
```

**Expected job output:**
```
▶ Fetching pending payments from http://payment-gateway:8080/api/v1/payments?status=pending...
  Found 1 pending payment(s). Triggering webhooks...
  ✅ Triggered: pay_20260612xxxxxx
✔ Done.
```

Confirm the listener received and forwarded the event to `POST /api/v1/webhooks/payment`:

```bash
kubectl logs -l app=stripe-listener --tail=10
# --> payment_intent.succeeded [evt_xxx]
# <-- [200] POST http://payment-gateway:8080/api/v1/webhooks/payment
```

---

## 7. Verify status transition

```bash
curl -s http://payment-gateway/api/v1/payments/$PAY_ID | python3 -m json.tool
# "status": "completed"
```

The UI auto-refreshes every 2s — the status badge turns green.

Check the full lifecycle in logs:

```bash
kubectl logs -l app=payment-gateway --tail=50 | grep -v span_ | grep $PAY_ID
```

**Expected log sequence:**
```
received CreatePayment request    payment_id=pay_20260612xxx  status=pending
processing webhook event          event_type=payment.completed  payment_id=pay_20260612xxx
webhook processed successfully    duration_ms=12
```

> **Interview talking point:** every log line carries `payment_id` and `request_id`.  
> The creation request and the webhook are different HTTP requests (different `request_id`)  
> but the same business event (`payment_id`) — that is the structured logging correlation path.

---

## 8. Verify in Postgres

```bash
kubectl apply -f k8s/tools/postgres-query.yaml
kubectl logs busybox-postgres-query
kubectl delete pod busybox-postgres-query
```

**Expected output includes:**
```
=== 4. Status breakdown ===
 status    | count
-----------+-------
 completed |     1

=== 5. Most recent 10 payments ===
 id                  | status    | created_at
---------------------+-----------+----------------------------
 pay_20260612xxxxxx  | completed | 2026-06-12 14:xx:xx+00
```

> **Interview talking point:** `updated_at > created_at` confirms the state transition was  
> driven by the webhook event, not the original creation request.

---

## 9. Demonstrate failure path (bonus)

```bash
# Create a new payment to fail
IDEM_FAIL="demo-fail-$(date +%s)"
FAIL=$(curl -s -X POST http://payment-gateway/api/v1/payments \
  -H "Content-Type: application/json" \
  -H "X-Idempotency-Key: $IDEM_FAIL" \
  -d '{"amount":9.99,"currency":"USD","description":"Failure demo","customer_id":"cust_demo"}')
FAIL_ID=$(echo $FAIL | python3 -c "import sys,json; print(json.load(sys.stdin)['payment_id'])")

# Trigger payment_intent.payment_failed
kubectl delete job stripe-trigger-demo --ignore-not-found
kubectl apply -f - <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: stripe-trigger-demo
  namespace: default
spec:
  backoffLimit: 0
  ttlSecondsAfterFinished: 600
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: stripe-trigger
        image: stripe/stripe-cli:latest
        command:
        - stripe
        - trigger
        - payment_intent.payment_failed
        - --override
        - "payment_intent:amount=999"
        - --override
        - "payment_intent:metadata[payment_id]=$FAIL_ID"
        env:
        - name: STRIPE_API_KEY
          valueFrom:
            secretKeyRef:
              name: payment-gateway-secrets
              key: STRIPE_API_KEY
EOF

sleep 5
curl -s http://payment-gateway/api/v1/payments/$FAIL_ID | python3 -m json.tool
# "status": "failed"
```

> **Interview talking point:** the same webhook handler routes both `payment_intent.succeeded`  
> and `payment_intent.payment_failed` through the state machine.  
> `pending → failed` uses the same code path as `pending → completed` — no duplication.

---

## 10. Show observability

```bash
# Prometheus metrics
curl -s http://payment-gateway/metrics | grep -E "http_requests_total|http_request_duration"

# Health and readiness probes
curl -s http://payment-gateway/health
curl -s http://payment-gateway/ready
```

| Metric | What it shows |
|---|---|
| `http_requests_total{status_code="201"}` | Successful payment creations |
| `http_requests_total{path="...webhooks/payment",status_code="200"}` | Webhook events verified and processed |
| `http_requests_total{path="...webhooks/payment",status_code="400"}` | Rejected — bad HMAC signature |
| `http_request_duration_seconds_sum/_count` | Average Stripe API round-trip latency |

---

## 11. Show Kubernetes resilience (bonus)

```bash
# Delete one pod — maxUnavailable=0 keeps the service live during replacement
kubectl delete pod $(kubectl get pod -l app=payment-gateway -o jsonpath='{.items[0].metadata.name}')

# Service still responds
curl -s http://localhost:8080/health

# Watch replacement
kubectl get pods -l app=payment-gateway -w

# HPA status
kubectl get hpa
```

---

## Teardown

```bash
# Delete all cluster resources
kubectl delete job stripe-trigger-demo --ignore-not-found
kubectl delete -k k8s/kustomize/base/
kubectl delete -k k8s/kustomize/eso/

# Full cluster teardown
kind delete cluster --name payment-demo
```

---

## What each step proves

| Step | Demonstrates |
|---|---|
| 2 | K8s-native deploy via Kustomize: Deployment, Service, Ingress, HPA, ESO secrets, in-cluster Stripe listener |
| 4 | Stripe PaymentIntent creation, `pending` state, structured JSON response |
| 5 | Idempotency — safe retries, no duplicate charges |
| 6 | Async webhook flow via `stripe-trigger-job`, HMAC signature verified by gateway |
| 7 | `pending → completed` state machine, structured logging with `payment_id` correlation |
| 8 | Persistence — state transition confirmed in Postgres |
| 9 | Failure path — `pending → failed`, same handler, same code path |
| 10 | Prometheus metrics, liveness/readiness probes |
| 11 | Zero-downtime rolling update (`maxUnavailable=0`), HPA awareness |

---

## Common issues

| Symptom | Fix |
|---|---|
| `stripe-listener` pod not starting | `kubectl get secret payment-gateway-secrets` — must exist with `STRIPE_API_KEY` key |
| Job: `No pending payments found` | Create at least one payment in Step 4 before running the Job |
| `[400]` on webhook | `STRIPE_WEBHOOK_SECRET` mismatch — re-apply `secretstore-literal-fake.yaml` and force-sync ESO: `kubectl annotate externalsecret payment-gateway-secrets force-sync=$(date +%s) --overwrite` |
| Frontend "Network error" | Ingress not reachable. Ensure `127.0.0.1 payment-gateway` is in /etc/hosts and Kind was created with port 80. |
| `ImagePullBackOff` on payment-gateway | `kind load docker-image payment-gateway:latest --name payment-demo` |
| `job already exists` error | `kubectl delete job stripe-trigger-demo` before re-applying |
| ESO `SecretSyncedError` | Check `kubectl describe secretstore payment-gateway-secret-store` — re-apply with corrected values |
