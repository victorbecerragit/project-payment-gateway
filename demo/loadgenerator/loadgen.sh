#!/bin/bash
# Payment Gateway Load Generator
# Supports Mock and Stripe demo modes

set -euo pipefail

MODE="${1:-mock}" # 'mock' or 'stripe'
GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"
CONCURRENCY="${CONCURRENCY:-1}"
REQUESTS="${REQUESTS:-10}"

echo "Starting load generation in '$MODE' mode"
echo "Target: $GATEWAY_URL"
echo "Concurrency: $CONCURRENCY"
echo "Requests per worker: $REQUESTS"

generate_traffic() {
  local worker_id=$1
  for i in $(seq 1 "$REQUESTS"); do
    local idempotency_key
    idempotency_key=$(uuidgen || cat /proc/sys/kernel/random/uuid)
    
    local provider="mock"
    if [ "$MODE" = "stripe" ]; then
      provider="stripe"
    fi

    echo "[Worker $worker_id] POST /api/v1/payments (Key: $idempotency_key)"
    
    curl -s -X POST "$GATEWAY_URL/api/v1/payments" \
      -H "Content-Type: application/json" \
      -H "Idempotency-Key: $idempotency_key" \
      -d "{
        \"amount\": $((RANDOM % 5000 + 100)),
        \"currency\": \"USD\",
        \"provider\": \"$provider\",
        \"reference_id\": \"test_order_${worker_id}_${i}\"
      }" > /dev/null

    # Optional: trigger Stripe webhooks locally (requires Stripe CLI if in stripe mode)
    if [ "$MODE" = "stripe" ] && [ "${TRIGGER_WEBHOOKS:-false}" = "true" ]; then
      stripe trigger payment_intent.succeeded > /dev/null 2>&1 &
    fi

    sleep 0.5
  done
}

export -f generate_traffic

for w in $(seq 1 "$CONCURRENCY"); do
  generate_traffic "$w" &
done

wait
echo "Load generation completed."