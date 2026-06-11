# Payment Gateway Demo Frontend

A single-file HTML dashboard that makes payment status transitions visible in real time during a demo.

## How to run

```bash
# Open directly in a browser (gateway must be on localhost:8080)
open demo/frontend/index.html

# Or serve it if you need to avoid CORS issues
python3 -m http.server 3000 --directory demo/frontend
# then open http://localhost:3000
```

The gateway must be reachable at `http://localhost:8080`. Start it first:

```bash
unset STRIPE_API_KEY STRIPE_WEBHOOK_SECRET
docker compose up --build -d
stripe listen --forward-to localhost:8080/api/v1/webhooks/payment
```

## What stakeholders see

- **Create Payment** form — fills in amount, customer, description; generates idempotency key automatically
- **Live Metrics** — pending / processing / completed / failed counters update every 2 seconds
- **Payments list** — each row shows payment ID, animated status badge, amount, Stripe PI reference, and last-updated time; rows flash green when status changes
- **Activity Log** — every action logged with timestamp; after creation, the exact `stripe trigger` command is printed ready to copy-paste

## Status badge colours

| Status | Colour | Animation |
|--------|--------|-----------|
| `pending` | Blue | — |
| `processing` | Amber | Pulsing |
| `completed` | Green | — |
| `failed` | Red | — |
| `cancelled` | Grey | — |

## Demo risks and mitigations

| Risk | Mitigation |
|------|-----------|
| CORS blocked if opened as `file://` | Serve with `python3 -m http.server 3000` |
| `stripe listen` not running → webhook never arrives | Pre-start listener before opening frontend |
| Stale `STRIPE_API_KEY` in shell → payment creation 500 | Run `unset STRIPE_API_KEY STRIPE_WEBHOOK_SECRET` before `docker compose up` |
| Terminal payments (completed/failed) stop polling | By design — no wasted requests once in terminal state |
