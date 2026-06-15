# OpenAPI Specification Reference

This project uses the [OpenAPI 3.0 Specification](https://spec.openapis.org/oas/v3.0.3) to define the contract between the Payment Gateway and its clients. We follow a **contract-first** approach, meaning the `openapi.yaml` file is the source of truth for the API's behavior.

## Spec Location
The definition file is located at the root of the project:
`openapi.yaml`

## Viewing the API

To view the documentation in a human-readable format, you can use several tools:

1.  **Online Swagger Editor**: Copy the content of `openapi.yaml` and paste it into editor.swagger.io.
2.  **Redocly**: Use Redoc for a clean, documentation-focused layout.
3.  **Local Tools**:
    ```bash
    # If you have redoc-cli installed
    redoc-cli serve openapi.yaml
    ```

## Core Concepts

### Idempotency
All payment creation requests (`POST /api/v1/payments`) require the `X-Idempotency-Key` header (UUID format). This allows clients to safely retry requests in case of network failures without causing duplicate charges.

### Status Vocabulary
Payments follow a strict lifecycle as defined in the schema:
- `pending`: Initial state after creation. (Requires webhook for transition).
- `processing`: The provider is currently handling the transaction.
- `completed`: The payment was successful.
- `failed`: The payment was rejected or encountered an error.
- `cancelled`: The payment was voided before completion.

## Endpoint Summary

| Tag | Method | Path | Description |
| :--- | :--- | :--- | :--- |
| **health** | GET | `/health` | Liveness check for K8s probes. |
| **health** | GET | `/ready` | Readiness check for K8s probes. |
| **payments** | POST | `/api/v1/payments` | Creates a new payment intent. |
| **payments** | GET | `/api/v1/payments` | Lists payments (supports status filtering). |
| **payments** | GET | `/api/v1/payments/{id}` | Retrieves detailed status of a payment. |
| **webhooks** | POST | `/api/v1/webhooks/payment` | Ingests notifications from Stripe/Mock providers. |

## Webhook Verification

The webhook endpoint (`/api/v1/webhooks/payment`) requires signature verification via the `X-Webhook-Signature` header. This ensures that the event was actually sent by the payment provider and has not been tampered with.

## Maintaining the Spec

When modifying the API:
1.  Update `openapi.yaml` first.
2.  Verify the spec is valid using a linter (like `spectral`).
3.  Update the Go transport DTOs in `internal/transport/http/dto/` to align with the new schema.
4.  Ensure health/readiness responses match the spec.

## Server Configuration

In local development, the API is typically served at:
`http://payment-gateway/api` (via Ingress)
`http://localhost:8080/api` (standard port-forward)