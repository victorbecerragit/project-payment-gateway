# Copilot Instructions — Puzzle 4: Provider Workflow and Persistence Evolution

The project now has a much better internal structure: domain-owned payment entities, status transition rules, transport DTO separation, mapper helpers, a repository abstraction, and a webhook verification seam are all present in the current scaffold.

Puzzle 4 should build on that foundation by introducing a first real provider workflow and preparing persistence beyond the in-memory adapter, while preserving the existing public API contract.

## Goal

Implement one end-to-end provider-backed payment flow and make the storage layer ready for a non-memory implementation without forcing another transport or domain rewrite.

## Current baseline

The current code already includes these good building blocks:

- A domain `Payment` entity with explicit statuses and legal transition checks.
- Application service methods for `CreatePayment`, `GetPayment`, and `ProcessEvent`.
- HTTP DTOs and mapping helpers separated from the domain layer.
- A `WebhookVerifier` interface and a mock verifier seam.
- An in-memory repository implementing the payment repository contract.

Before starting Puzzle 4, keep in mind two follow-up items from the previous review:

- The mock webhook verifier/package location should be cleaned up so package placement matches imports.
- Event-processing behavior should be reconciled with the domain transition rules, especially if provider callbacks can move a payment directly from `pending` to a terminal state.

## Puzzle 4 objective

Add a provider adapter layer and evolve the persistence design so the application service can drive a realistic payment lifecycle using provider abstractions rather than placeholder logic.

## Scope boundaries

Puzzle 4 should not attempt all PSPs or full payment orchestration at once.

Keep the scope limited to:

- One provider adapter first, ideally Stripe or a mock Stripe-like adapter.
- One primary flow: create payment and observe status update from provider/webhook.
- One non-memory persistence path design, with either an actual first adapter or a migration-ready repository contract.
- No breaking public API changes unless explicitly documented and justified against `openapi.yaml`.

## Implementation principles for Copilot

- Keep the domain model provider-agnostic.
- Keep provider request/response payloads out of transport DTO packages.
- Put PSP-specific code behind interfaces under `internal/provider/...`.
- Prefer one clean happy-path integration over broad but shallow provider support.
- Preserve idempotency behavior and extend it only when persistence is ready.
- Add tests at service and adapter boundaries.

## Target architecture for Puzzle 4

Suggested direction:

```text
internal/provider/
  stripe/
    client.go
    mapper.go
    webhook.go
  webhook/
    verifier.go

internal/application/payment/
  service.go
  provider_workflow.go

internal/storage/
  inmemory/
  postgres/        # optional in this puzzle if implemented
  migrations/      # optional if postgres is introduced
```

## PR plan

### PR 1 — Provider interface and workflow seam

Goal: define how the application service talks to a PSP without coupling to one concrete implementation.

Tasks:

- Introduce a provider-facing interface such as `ProviderClient` or `PaymentProcessor`.
- Define the minimum provider operations needed for the first flow, for example `CreatePaymentIntent`, `GetPaymentStatus`, and `ParseWebhookEvent`.
- Keep provider DTOs local to the provider package.
- Inject the provider interface into the payment application service.

Definition of done:

- Application logic can create or update payments through an interface rather than placeholder-only logic.

### PR 2 — First adapter: Stripe or mock Stripe-like provider

Goal: implement one concrete adapter behind the provider interface.

Tasks:

- Choose either a real Stripe sandbox integration or a mock adapter with realistic fields and webhook semantics.
- Map domain payment creation intent into provider request shape.
- Capture provider reference IDs and persist them on the payment entity or in a provider metadata structure.
- Normalize provider responses back into domain-relevant values.

Definition of done:

- Creating a payment can optionally call the provider adapter and store a provider reference for later reconciliation.

### PR 3 — Payment entity/provider metadata refinement

Goal: carry enough provider metadata to support reconciliation and webhook processing.

Tasks:

- Decide whether to extend `Payment` with provider name and provider reference fields, or introduce a dedicated provider metadata structure.
- Keep the domain generic: `provider`, `provider_payment_id`, `provider_status` are acceptable generic fields; avoid Stripe-specific field names in the core entity.
- Make sure OpenAPI exposure of these fields is deliberate rather than accidental.

Definition of done:

- The system can correlate internal payment IDs with provider references cleanly.

### PR 4 — Webhook workflow refinement

Goal: connect provider webhook semantics to the domain state machine safely.

Tasks:

- Move provider-specific webhook parsing into the provider adapter package.
- Return normalized domain events from provider webhook parsers.
- Reconcile the application `ProcessEvent` logic with the domain transition rules.
- If needed, introduce an application-level policy to move `pending -> processing` before terminal transition when driven by provider events.

Definition of done:

- Webhook processing works end-to-end for the chosen provider model without violating the domain state machine.

### PR 5 — Persistence contract evolution

Goal: prepare for durable storage and stronger idempotency guarantees.

Tasks:

- Review whether the repository interface needs methods for provider-reference lookup, status update, or event deduplication.
- Add repository methods intentionally; do not expose persistence mechanics to handlers.
- Decide whether to implement a first Postgres adapter now or defer it behind a documented contract.
- If Postgres is added, include minimal migrations and a local development path through Docker Compose.

Definition of done:

- Repository boundaries support realistic provider flows and durable idempotency plans.

### PR 6 — Local integration and test strategy

Goal: make the provider flow verifiable in local development and CI.

Tasks:

- Add unit tests for provider mapping logic.
- Add service tests for create-payment plus provider reference persistence.
- Add webhook processing tests that prove domain transitions are respected.
- If a real provider sandbox is used, gate integration tests behind environment variables.

Definition of done:

- The first provider-backed flow can be validated locally and in CI without fragile manual steps.

## Recommended design choices

### Provider abstraction

Suggested interface direction:

```go
type PaymentProcessor interface {
    CreatePayment(ctx context.Context, p *payment.Payment) (*ProviderPaymentResult, error)
    ParseWebhook(ctx context.Context, payload []byte, signature string) (*payment.PaymentEvent, error)
}
```

This keeps the application layer focused on domain orchestration while the provider package owns provider-specific payloads and verification details.

### Persistence direction

If the goal is quick progress, keep `inmemory` for tests and add a first Postgres adapter next. Postgres is the most natural fit for idempotency keys, payment status history, provider references, and webhook deduplication, while DynamoDB can remain a later option if operational goals favor it.

### Minimal happy path

The best first end-to-end flow is:

1. Client calls `POST /api/v1/payments` with `X-Idempotency-Key`.
2. Application creates internal payment record.
3. Provider adapter creates a provider-side payment or intent and returns provider reference.
4. Internal payment is updated with provider metadata and set to `processing` if appropriate.
5. Provider webhook arrives and is parsed into a normalized event.
6. Application updates internal payment state according to the domain rules.

## Copilot prompts

### Prompt A — Provider seam

```md
Introduce a provider abstraction for the payment gateway.

Tasks:
1. Define a provider interface for creating payments and parsing webhooks.
2. Keep provider-specific payloads out of the domain and HTTP DTO packages.
3. Inject the provider dependency into the payment application service.
4. Preserve the current public API.
5. Suggest the smallest safe PR.
```

### Prompt B — First provider adapter

```md
Implement the first payment provider adapter for this project.

Constraints:
- Prefer Stripe or a realistic mock Stripe-like adapter.
- Keep the domain model provider-agnostic.
- Return normalized payment results and events.
- Preserve current route behavior.

Output:
- files to create
- interfaces to add
- mapping strategy
- test cases
```

### Prompt C — Persistence evolution

```md
Review the payment repository contract and evolve it for real provider workflows.

Tasks:
1. Identify missing methods for provider reference lookup and lifecycle updates.
2. Propose the smallest repository contract change.
3. Recommend whether to add a Postgres adapter now.
4. Keep HTTP handlers unchanged.

Output:
- repository diff plan
- storage package structure
- migration plan if Postgres is added
```

### Prompt D — Webhook workflow alignment

```md
Align webhook event processing with the payment domain state machine.

Tasks:
1. Review how provider events map to domain events.
2. Prevent invalid direct terminal transitions unless intentionally supported.
3. Propose an application-level policy for pending -> processing -> terminal flows.
4. Add tests for valid and invalid webhook-driven transitions.
```

## Acceptance checklist

Puzzle 4 can be considered complete when these statements are true:

- The application uses a provider abstraction instead of placeholder-only payment orchestration.
- One concrete provider adapter exists and can drive a basic payment workflow.
- Provider-specific data is isolated from transport DTOs and domain core types.
- The repository contract supports provider references and realistic lifecycle updates.
- Webhook processing is aligned with the domain transition rules.
- Local tests cover create-payment, provider mapping, and webhook-driven state updates.
- Public API behavior remains stable unless an API revision is intentionally documented in `openapi.yaml`.
