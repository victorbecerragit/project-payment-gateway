#!/bin/bash
# Payment Gateway Load Generator
# Scenarios: mock | stripe | stripe-fail | stress

set -euo pipefail

SCENARIO="${1:-mock}"
GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"
REQUESTS="${REQUESTS:-5}"
DELAY="${DELAY:-0.5}"          # seconds between requests
TRIGGER_WEBHOOKS="${TRIGGER_WEBHOOKS:-false}"

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

  # ── Scenario 4: Stress (mock, no external deps) ──────────────────────────
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

  # ── Scenario 5: Stress rate-limit (parallel burst, expects 429s) ─────────
  stress-rate-limit)
    REQUESTS="${REQUESTS:-60}"
    CONCURRENCY="${CONCURRENCY:-60}"  # fire all at once to blow past burst=20
    TMPDIR_RL=$(mktemp -d)
    trap 'rm -rf "$TMPDIR_RL"' EXIT

    echo "▶ Stress-rate-limit: $REQUESTS parallel requests against POST /api/v1/payments"
    echo ""
    echo "  Rate limit config (env overrides or server defaults):"
    echo "    API_RATE_LIMIT = ${API_RATE_LIMIT:-10}  req/s"
    echo "    API_BURST      = ${API_BURST:-20}        tokens"
    echo ""
    echo "  Strategy: fire $REQUESTS requests simultaneously from one IP."
    echo "  Burst=${API_BURST:-20} requests should succeed; the rest should return HTTP 429."
    echo ""

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
    echo "──────────────────────────────────────────"

    if [ "$RATE_LIMITED" -gt 0 ]; then
      echo ""
      echo "  ✔ Rate limiter is working: $RATE_LIMITED request(s) were rejected with 429."
    else
      echo ""
      echo "  ⚠ No 429s observed. Try increasing REQUESTS beyond the burst window."
      echo "  Tip: REQUESTS=100 $0 stress-rate-limit"
    fi
    ;;

  *)
    echo "Unknown scenario: $SCENARIO"
    echo "Usage: $0 [mock|stripe|stripe-fail|stress|stress-rate-limit]"
    echo ""
    echo "Environment variables:"
    echo "  GATEWAY_URL        (default: http://localhost:8080)"
    echo "  REQUESTS           (default: 5, stress-rate-limit default: 60)"
    echo "  CONCURRENCY        (default: 60, stress-rate-limit only)"
    echo "  DELAY              (default: 0.5s between requests)"
    echo "  TRIGGER_WEBHOOKS   (default: false, stripe mode only)"
    echo "  API_RATE_LIMIT     (display only, reads server env)"
    echo "  API_BURST          (display only, reads server env)"
    exit 1
    ;;
esac

echo ""
echo "✔ Done."
