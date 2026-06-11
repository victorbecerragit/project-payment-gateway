# Stripe Sandbox Demo

This directory provides the necessary configuration and commands to test the Payment Gateway with Stripe in a sandbox environment.

## Prerequisites

- [Stripe CLI](https://stripe.com/docs/stripe-cli) installed and authenticated.
- A Stripe account (test mode).

## Local Development

1. Copy `.env.example` to the root project directory as `.env`:
   ```bash
   cp extras/stripe-sandbox/.env.example .env
   ```

2. Generate your Stripe API key from the Stripe Dashboard (Test Mode -> Developers -> API Keys) and add it to the root `.env` as `STRIPE_API_KEY`.

3. Start listening for Stripe webhooks locally:
   ```bash
   stripe listen --forward-to localhost:8080/api/v1/webhooks/payment
   ```
   *Note: Ensure your local payment gateway is running on port 8080.*

4. The Stripe CLI will output a webhook signing secret (`whsec_...`). Add this to your `.env` file as `STRIPE_WEBHOOK_SECRET`.

5. Restart your payment gateway with the loaded environment variables.

## Triggering Events

All trigger commands require `--override` with the `payment_id` returned by `POST /api/v1/payments`, otherwise the webhook will return `404` (payment not found in gateway).

First, create a payment and capture its ID:
```bash
PAYMENT=$(curl -s -X POST http://localhost:8080/api/v1/payments \
  -H "Content-Type: application/json" \
  -H "X-Idempotency-Key: demo-$(date +%s)" \
  -d '{"amount":99.99,"currency":"USD","description":"Stripe demo","customer_id":"cust_123"}')
PAY_ID=$(echo $PAYMENT | python3 -c "import sys,json; print(json.load(sys.stdin)['payment_id'])")
echo "Payment ID: $PAY_ID"
```

### Payment Intent Succeeded
```bash
stripe trigger payment_intent.succeeded \
  --override "payment_intent:metadata[payment_id]=$PAY_ID"
```

### Payment Intent Payment Failed
```bash
stripe trigger payment_intent.payment_failed \
  --override "payment_intent:metadata[payment_id]=$PAY_ID"
```

### Payment Intent Canceled
```bash
stripe trigger payment_intent.canceled \
  --override "payment_intent:metadata[payment_id]=$PAY_ID"
```

## Kubernetes Usage

To use the Stripe integration in Kubernetes:

1. Create a Kubernetes Secret containing your Stripe API key and Webhook Secret:
   ```bash
   kubectl create secret generic stripe-credentials \
     --from-literal=stripe-api-key=sk_test_... \
     --from-literal=stripe-webhook-secret=whsec_...
   ```

2. Mount these secrets as environment variables in your gateway deployment.

3. *Note*: To receive real webhooks from Stripe to your Kubernetes cluster, you'll need to expose your service via an Ingress and configure the actual Webhook endpoint in the Stripe Dashboard.
