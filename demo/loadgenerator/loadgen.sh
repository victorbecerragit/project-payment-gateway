#!/bin/bash
# Payment Gateway Load Generator
# Scenarios: mock | stripe | stripe-fail | multi-state | stress

set -euo pipefail

SCENARIO="${1:-mock}"
GATEWAY_URL="${GATEWAY_URL:-http://payment-gateway}"
REQUESTS="${REQUESTS:-5}"
DELAY="${DELAY:-0.5}"          # seconds between requests
TRIGGER_WEBHOOKS="${TRIGGER_WEBHOOKS:-false}"
WEBHOOK_SECRET="${WEBHOOK_SECRET:-}"  # Falls back to kubectl get secret

CUSTOMERS=("cust_alice" "cust_bob" "cust_carol" "cust_dave" "cust_eve")
AMOUNTS=("9.99" "24.99" "49.99" "99.99" "199.99" "299.00")
CURRENCIES=("USD" "EUR" "GBP")
DESCRIPTIONS=("Online purchase" "Subscription renewal" "Service fee" "Product order" "Demo payment")

rand_element() { local arr=("$@"); echo "${arr[$((RANDOM % ${#arr[@]}))]}"; }
uuid() { cat /proc/sys/kernel/random/uuid 2>/dev/null || uuidgen; }

create_payment() {
  local idem="$1" customer="$2" amount="$3" currency="$4" description="$5"
  curl -s -w "\n%{http_code}" -X POST "$GATEWAY_URL/api/v1/payments" \
    -H "Content-Type: application/json" \
    -H "X-Idempotency-Key: $idem" \
    -d "{\"amount\":$amount,\"currency\":\"$currency\",\"description\":\"$description\",\"customer_id\":\"$customer\"}"
}

send_webhook() {
  local event_type="$1" payment_id="$2"

  # Map event_type to Stripe format
  local stripe_type stripe_status
  case "$event_type" in
    payment.completed)  stripe_type="payment_intent.succeeded"; stripe_status="succeeded" ;;
    payment.failed)     stripe_type="payment_intent.payment_failed"; stripe_status="failed" ;;
    payment.cancelled)  stripe_type="payment_intent.canceled"; stripe_status="canceled" ;;
    *)                  echo "unknown" ;;
  esac

  # Build Stripe-format payload
  local timestamp
  timestamp=$(date +%s)
  local evt_id="evt_$(uuid | tr -d '-')"
  local pi_id="pi_$(uuid | tr -d '-')"
  local payload
  payload=$(printf '{"id":"%s","type":"%s","data":{"object":{"id":"%s","object":"payment_intent","status":"%s","metadata":{"payment_id":"%s"}}}}' \
    "$evt_id" "$stripe_type" "$pi_id" "$stripe_status" "$payment_id")

  # Compute HMAC-SHA256 signature: signed_content = timestamp + "." + payload
  local signed_content="${timestamp}.${payload}"
  local secret="$WEBHOOK_SECRET"

  # Fallback: try fetching webhook secret from cluster if not set via env
  if [ -z "$secret" ]; then
    secret=$(kubectl get secret -n default payment-gateway-secrets -o jsonpath='{.data.STRIPE_WEBHOOK_SECRET}' 2>/dev/null | base64 -d) || true
  fi

  local signature=""
  if [ -n "$secret" ]; then
    signature=$(printf '%s' "$signed_content" | openssl dgst -sha256 -hmac "$secret" | awk '{print $NF}') || true
  fi

  curl -s -o /dev/null -w "%{http_code}" -X POST "$GATEWAY_URL/api/v1/webhooks/payment" \
    -H "Content-Type: application/json" \
    -H "Stripe-Signature: t=${timestamp},v1=${signature}" \
    -d "$payload"
}

echo "============================================"
echo " Payment Gateway Load Generator"
echo " Scenario : $SCENARIO"
echo " Target   : $GATEWAY_URL"
echo " Requests : $REQUESTS"
echo "============================================"
echo ""

case "$SCENARIO" in

  # ── Scenario 1: Mock end-to-end ──────────────────────────────────────────
  mock)
    echo "▶ Mock: creating $REQUESTS payments (in-memory, no external calls)"
    echo ""
    for i in $(seq 1 "$REQUESTS"); do
      IDEM="load-mock-$(uuid)"
      CUSTOMER=$(rand_element "${CUSTOMERS[@]}")
      AMOUNT=$(rand_element "${AMOUNTS[@]}")
      CURRENCY=$(rand_element "${CURRENCIES[@]}")
      DESC=$(rand_element "${DESCRIPTIONS[@]}")

      RESP=$(create_payment "$IDEM" "$CUSTOMER" "$AMOUNT" "$CURRENCY" "$DESC")
      HTTP=$(echo "$RESP" | tail -1)
      BODY=$(echo "$RESP" | head -1)
      PAY_ID=$(echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('payment_id','ERROR'))" 2>/dev/null || echo "ERROR")

      STATUS_ICON="✅"; [ "$HTTP" != "201" ] && STATUS_ICON="❌"
      printf "%s [%d/%d] HTTP %s  %-25s  %s %s %s\n" \
        "$STATUS_ICON" "$i" "$REQUESTS" "$HTTP" "$PAY_ID" "$AMOUNT" "$CURRENCY" "$CUSTOMER"
      sleep "$DELAY"
    done
    ;;

  # ── Scenario 2: Stripe happy path ────────────────────────────────────────
  stripe)
    echo "▶ Stripe: creating $REQUESTS payments and optionally triggering webhooks"
    echo "  TRIGGER_WEBHOOKS=$TRIGGER_WEBHOOKS"
    echo ""
    PAY_IDS=()

    for i in $(seq 1 "$REQUESTS"); do
      IDEM="load-stripe-$(uuid)"
      CUSTOMER=$(rand_element "${CUSTOMERS[@]}")
      AMOUNT=$(rand_element "${AMOUNTS[@]}")
      DESC=$(rand_element "${DESCRIPTIONS[@]}")

      RESP=$(create_payment "$IDEM" "$CUSTOMER" "$AMOUNT" "USD" "$DESC")
      HTTP=$(echo "$RESP" | tail -1)
      BODY=$(echo "$RESP" | head -1)
      PAY_ID=$(echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('payment_id','ERROR'))" 2>/dev/null || echo "ERROR")

      STATUS_ICON="✅"; [ "$HTTP" != "201" ] && STATUS_ICON="❌"
      printf "%s [%d/%d] HTTP %s  %-25s  created\n" "$STATUS_ICON" "$i" "$REQUESTS" "$HTTP" "$PAY_ID"

      [ "$PAY_ID" != "ERROR" ] && PAY_IDS+=("$PAY_ID")
      sleep "$DELAY"
    done

    if [ "${TRIGGER_WEBHOOKS}" = "true" ] && [ ${#PAY_IDS[@]} -gt 0 ]; then
      echo ""
      echo "▶ Triggering payment_intent.succeeded for ${#PAY_IDS[@]} payments..."
      for PAY_ID in "${PAY_IDS[@]}"; do
        stripe trigger payment_intent.succeeded \
          --override "payment_intent:metadata[payment_id]=$PAY_ID" > /dev/null 2>&1 \
          && printf "  ✅ webhook triggered for %s\n" "$PAY_ID" \
          || printf "  ❌ webhook failed for %s\n" "$PAY_ID"
        sleep 0.3
      done
    fi
    ;;

  # ── Scenario 3: Stripe failure path ──────────────────────────────────────
  stripe-fail)
    echo "▶ Stripe-fail: creating $REQUESTS payments then triggering payment_failed webhooks"
    echo ""
    PAY_IDS=()

    for i in $(seq 1 "$REQUESTS"); do
      IDEM="load-fail-$(uuid)"
      RESP=$(create_payment "$IDEM" "cust_failtest" "50.00" "USD" "Failure scenario test")
      HTTP=$(echo "$RESP" | tail -1)
      BODY=$(echo "$RESP" | head -1)
      PAY_ID=$(echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('payment_id','ERROR'))" 2>/dev/null || echo "ERROR")

      STATUS_ICON="✅"; [ "$HTTP" != "201" ] && STATUS_ICON="❌"
      printf "%s [%d/%d] HTTP %s  %-25s  created (will fail)\n" "$STATUS_ICON" "$i" "$REQUESTS" "$HTTP" "$PAY_ID"
      [ "$PAY_ID" != "ERROR" ] && PAY_IDS+=("$PAY_ID")
      sleep "$DELAY"
    done

    echo ""
    echo "▶ Triggering payment_intent.payment_failed for ${#PAY_IDS[@]} payments..."
    for PAY_ID in "${PAY_IDS[@]}"; do
      stripe trigger payment_intent.payment_failed \
        --override "payment_intent:metadata[payment_id]=$PAY_ID" > /dev/null 2>&1 \
        && printf "  ✅ failure webhook triggered for %s\n" "$PAY_ID" \
        || printf "  ❌ webhook failed for %s\n" "$PAY_ID"
      sleep 0.3
    done
    ;;

  # ── Scenario 4: Multi-state (mock webhooks, exercises Grafana metrics) ──
  multi-state)
    echo "▶ Multi-state: creating $REQUESTS payments across pending/completed/failed/cancelled"
    echo ""
    STATES=("pending" "completed" "failed" "cancelled")
    STATE_LABELS=("pending" "completed" "failed" "cancelled")
    WEBHOOK_TYPES=("" "payment.completed" "payment.failed" "payment.cancelled")
    CREATED=()

    for i in $(seq 1 "$REQUESTS"); do
      IDEM="load-multi-$(uuid)"
      CUSTOMER=$(rand_element "${CUSTOMERS[@]}")
      AMOUNT=$(rand_element "${AMOUNTS[@]}")
      CURRENCY=$(rand_element "${CURRENCIES[@]}")
      DESC=$(rand_element "${DESCRIPTIONS[@]}")

      RESP=$(create_payment "$IDEM" "$CUSTOMER" "$AMOUNT" "$CURRENCY" "$DESC")
      HTTP=$(echo "$RESP" | tail -1)
      BODY=$(echo "$RESP" | head -1)
      PAY_ID=$(echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('payment_id','ERROR'))" 2>/dev/null || echo "ERROR")

      STATE_IDX=$(( (i-1) % 4 ))
      STATE="${STATE_LABELS[$STATE_IDX]}"

      STATUS_ICON="✅"; [ "$HTTP" != "201" ] && STATUS_ICON="❌"
      printf "%s [%d/%d] HTTP %s  %-25s  → %s\n" \
        "$STATUS_ICON" "$i" "$REQUESTS" "$HTTP" "$PAY_ID" "$STATE"

      CREATED+=("$PAY_ID|${STATE_IDX}|${AMOUNT}|${CURRENCY}|${CUSTOMER}")
      sleep "$DELAY"
    done

    echo ""
    echo "▶ Sending mock webhooks to advance payments..."
    echo ""
    for entry in "${CREATED[@]}"; do
      IFS='|' read -r PAY_ID STATE_IDX AMOUNT CURRENCY CUSTOMER <<< "$entry"
      WEBHOOK_TYPE="${WEBHOOK_TYPES[$STATE_IDX]}"
      STATE="${STATE_LABELS[$STATE_IDX]}"

      if [ -n "$WEBHOOK_TYPE" ]; then
        WH_HTTP=$(send_webhook "$WEBHOOK_TYPE" "$PAY_ID")
        if [ "$WH_HTTP" = "200" ]; then
          printf "  ✅ %-25s  → %s\n" "$PAY_ID" "$STATE"
        else
          printf "  ❌ %-25s  webhook failed (HTTP %s)\n" "$PAY_ID" "$WH_HTTP"
        fi
        sleep 0.3
      else
        printf "  ⏸️  %-25s  → pending (no webhook)\n" "$PAY_ID"
      fi
    done

    echo ""
    echo "Summary:"
    for state in pending completed failed cancelled; do
      COUNT=$(printf '%s\n' "${CREATED[@]}" | awk -F'|' -v s="$state" '
        BEGIN {t["pending"]=0; t["completed"]=1; t["failed"]=2; t["cancelled"]=3}
        $2 == t[s] {c++}
        END {print c+0}
      ')
      printf "  %-12s %d\n" "$state:" "$COUNT"
    done
    ;;

  # ── Scenario 5: Stress (mock, no external deps) ──────────────────────────
  stress)
    REQUESTS="${REQUESTS:-50}"
    DELAY="0"
    echo "▶ Stress: $REQUESTS rapid-fire mock payments (no delay)"
    echo ""
    SUCCESS=0; FAIL=0
    for i in $(seq 1 "$REQUESTS"); do
      IDEM="load-stress-$(uuid)"
      HTTP=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$GATEWAY_URL/api/v1/payments" \
        -H "Content-Type: application/json" \
        -H "X-Idempotency-Key: $IDEM" \
        -d "{\"amount\":9.99,\"currency\":\"USD\",\"description\":\"stress test\",\"customer_id\":\"cust_stress\"}")
      if [ "$HTTP" = "201" ]; then SUCCESS=$((SUCCESS+1)); else FAIL=$((FAIL+1)); fi
    done
    echo "Done. ✅ $SUCCESS succeeded  ❌ $FAIL failed  (total: $REQUESTS)"
    ;;

  # ── Scenario 6: Stress rate-limit (parallel burst, expects 429s) ─────────
  stress-rate-limit)
    REQUESTS="${REQUESTS:-60}"
    CONCURRENCY="${CONCURRENCY:-60}"  # fire all at once to blow past burst=20
    TMPDIR_RL=$(mktemp -d)
    trap 'rm -rf "$TMPDIR_RL"' EXIT

    echo "▶ Stress-rate-limit: $REQUESTS parallel requests against POST /api/v1/payments"
    echo ""
    echo "  Rate limit config (env overrides or server defaults):"
    echo "    API_RATE_LIMIT = ${API_RATE_LIMIT:-10}  req/s   (token refill rate)"
    echo "    API_BURST      = ${API_BURST:-20}        tokens  (initial bucket size)"
    echo ""
    echo "  ⚠  Expected 201s > burst (${API_BURST:-20}) because curl jobs take real"
    echo "     wall-clock time: each extra second refills ${API_RATE_LIMIT:-10} tokens."
    echo "     201s ≈ burst + (elapsed_sec × rate). That is correct limiter behaviour."
    echo ""
    echo "  Strategy: fire $REQUESTS requests simultaneously from one IP."
    echo ""

    T_START=$(date +%s%N)

    # Launch all curl requests in parallel; write each HTTP status to a temp file
    PIDS=()
    for i in $(seq 1 "$REQUESTS"); do
      IDEM="load-rl-$(uuid)"
      (
        HTTP=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$GATEWAY_URL/api/v1/payments" \
          -H "Content-Type: application/json" \
          -H "X-Idempotency-Key: $IDEM" \
          -d "{\"amount\":9.99,\"currency\":\"USD\",\"description\":\"rate limit test\",\"customer_id\":\"cust_rl\"}")
        echo "$HTTP" > "$TMPDIR_RL/$i"
      ) &
      PIDS+=($!)

      # Honour concurrency cap: wait for a batch before launching more
      if [ ${#PIDS[@]} -ge "$CONCURRENCY" ]; then
        for pid in "${PIDS[@]}"; do wait "$pid" 2>/dev/null || true; done
        PIDS=()
      fi
    done
    # Wait for any remaining background jobs
    for pid in "${PIDS[@]}"; do wait "$pid" 2>/dev/null || true; done

    # Tally results
    T_END=$(date +%s%N)
    ELAPSED_MS=$(( (T_END - T_START) / 1000000 ))
    ELAPSED_S=$(echo "scale=2; $ELAPSED_MS / 1000" | bc)
    EXPECTED=$(echo "scale=0; ${API_BURST:-20} + ${API_RATE_LIMIT:-10} * $ELAPSED_S / 1" | bc 2>/dev/null || echo "~$((${API_BURST:-20} + ${API_RATE_LIMIT:-10} * ELAPSED_MS / 1000))")

    OK=0; RATE_LIMITED=0; OTHER=0
    for f in "$TMPDIR_RL"/*; do
      CODE=$(cat "$f")
      case "$CODE" in
        201) OK=$((OK+1)) ;;
        429) RATE_LIMITED=$((RATE_LIMITED+1)) ;;
        *)   OTHER=$((OTHER+1)) ;;
      esac
    done

    TOTAL=$((OK + RATE_LIMITED + OTHER))
    echo "──────────────────────────────────────────"
    printf "  ✅  201 Created       : %d\n" "$OK"
    printf "  🚫  429 Rate Limited  : %d\n" "$RATE_LIMITED"
    printf "  ❌  Other             : %d\n" "$OTHER"
    printf "  ────────────────────────────────────\n"
    printf "  Total sent            : %d\n" "$TOTAL"
    printf "  Elapsed               : %ss\n" "$ELAPSED_S"
    printf "  Expected 201s ≈ burst(%s) + rate(%s)×%ss ≈ %s\n" \
      "${API_BURST:-20}" "${API_RATE_LIMIT:-10}" "$ELAPSED_S" "$EXPECTED"
    echo "──────────────────────────────────────────"

    if [ "$RATE_LIMITED" -gt 0 ]; then
      echo ""
      echo "  ✔ Rate limiter is working: $RATE_LIMITED request(s) were rejected with 429."
      echo ""
      echo "  Tip: query the $OK created payments with:"
      echo "    curl -s \"$GATEWAY_URL/api/v1/payments?status=pending&limit=${OK}\" | jq 'length'"
    else
      echo ""
      echo "  ⚠ No 429s observed. Try increasing REQUESTS beyond the burst window."
      echo "  Tip: REQUESTS=100 $0 stress-rate-limit"
    fi
    ;;

  *)
    echo "Unknown scenario: $SCENARIO"
    echo "Usage: $0 [mock|stripe|stripe-fail|multi-state|stress|stress-rate-limit]"
    echo ""
    echo "Environment variables:"
    echo "  GATEWAY_URL        (default: http://payment-gateway)"
    echo "  REQUESTS           (default: 5, stress-rate-limit default: 60)"
    echo "  CONCURRENCY        (default: 60, stress-rate-limit only)"
    echo "  DELAY              (default: 0.5s between requests)"
    echo "  TRIGGER_WEBHOOKS   (default: false, stripe mode only)"
    echo "  WEBHOOK_SECRET     (default: auto-detect from cluster secret)"
    echo "  API_RATE_LIMIT     (display only, reads server env)"
    echo "  API_BURST          (display only, reads server env)"
    exit 1
    ;;
esac

echo ""
echo "✔ Done."
