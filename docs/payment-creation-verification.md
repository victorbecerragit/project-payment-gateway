# Payment Creation Flow — Verification Report

**Date:** 2026-06-10  
**Endpoint:** `POST /api/v1/payments`  
**Provider:** Stripe (test mode)  
**Environment:** Local (`go run`) with in-memory repository

---

## 1. Happy Path

**Request:**
```bash
curl -s -X POST http://localhost:8080/api/v1/payments \
  -H "Content-Type: application/json" \
  -H "X-Idempotency-Key: idem-1749566038" \
  -d '{"amount":99.99,"currency":"USD","description":"Stripe demo payment","customer_id":"cust_123"}'
```

**Response `201 Created`:**
```json
{
    "payment_id": "pay_20260610153358",
    "status": "pending",
    "amount": 99.99,
    "currency": "USD",
    "transaction_id": "pi_3Tgo8hRnzZxdec4K1J3eDziI",
    "created_at": "2026-06-10T15:33:58.562573478Z"
}
```

**Verified:**
- `payment_id` assigned with `pay_` prefix
- `status` is `pending` (correct initial domain state)
- `transaction_id` is a Stripe PaymentIntent ID (`pi_...`) confirming provider was called
- `amount` and `currency` correctly returned

---

## 2. Idempotency — Same Key, Same Request

**Request:** identical to case 1 (same `X-Idempotency-Key`)

**Response `201 Created`:**
```json
{
    "payment_id": "pay_20260610153358",
    "status": "pending",
    "amount": 99.99,
    "currency": "USD",
    "transaction_id": "pi_3Tgo8hRnzZxdec4K1J3eDziI",
    "created_at": "2026-06-10T15:33:58.562573Z"
}
```

**Verified:** Identical `payment_id` and `transaction_id` returned — no duplicate payment created.

---

## 3. Idempotency — Same Key, Different Amount

**Request:** same `X-Idempotency-Key` as case 1 but with `"amount": 1.00`

**Response `201 Created`:**
```json
{
    "payment_id": "pay_20260610153358",
    "status": "pending",
    "amount": 99.99,
    "currency": "USD",
    "transaction_id": "pi_3Tgo8hRnzZxdec4K1J3eDziI",
    "created_at": "2026-06-10T15:33:58.562573Z"
}
```

**Verified:** Original payment returned unchanged — body differences are ignored when the idempotency key matches.

---

## 4. Error — Missing Idempotency Key

**Request:** no `X-Idempotency-Key` header

**Response `400 Bad Request`:**
```json
{
    "error": "Bad Request",
    "message": "X-Idempotency-Key header is required",
    "code": 400
}
```

---

## 5. Error — Negative Amount

**Request:** `"amount": -10.00`

**Response `400 Bad Request`:**
```json
{
    "error": "Bad Request",
    "message": "payment amount must be positive",
    "code": 400
}
```

---

## 6. Error — Unsupported Currency

**Request:** `"currency": "XYZ"`

**Response `400 Bad Request`:**
```json
{
    "error": "Bad Request",
    "message": "unsupported or invalid currency: XYZ",
    "code": 400
}
```

---

## 7. Error — Invalid Customer ID Format

**Request:** `"customer_id": "invalid"` (missing `cust_` prefix)

**Response `400 Bad Request`:**
```json
{
    "error": "Bad Request",
    "message": "customer ID must have 'cust_' prefix and not be empty",
    "code": 400
}
```

---

## 8. Error — Missing Description

**Request:** no `description` field

**Response `400 Bad Request`:**
```json
{
    "error": "Bad Request",
    "message": "description is required",
    "code": 400
}
```

---

## 9. Error — Malformed JSON

**Request:** body is `not-json`

**Response `400 Bad Request`:**
```json
{
    "error": "Bad Request",
    "message": "Invalid request body",
    "code": 400
}
```

---

## Summary

| # | Case | Expected Status | Result |
|---|------|----------------|--------|
| 1 | Valid payment creation | `201` | ✅ |
| 2 | Idempotency — same key, same request | `201` same payment | ✅ |
| 3 | Idempotency — same key, different amount | `201` original payment | ✅ |
| 4 | Missing `X-Idempotency-Key` header | `400` | ✅ |
| 5 | Negative amount | `400` | ✅ |
| 6 | Unsupported currency | `400` | ✅ |
| 7 | Invalid customer ID format | `400` | ✅ |
| 8 | Missing description | `400` | ✅ |
| 9 | Malformed JSON body | `400` | ✅ |

All 9 cases passed. No `500` errors on invalid input — all domain validation errors return descriptive `400 Bad Request` responses.
