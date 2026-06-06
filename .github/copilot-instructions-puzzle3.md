# Copilot Instructions — Puzzle 3: Domain Hardening

The project has already improved its structure by introducing application, domain, storage, platform, and transport layers, and by aligning the router more closely with the OpenAPI contract through `POST /api/v1/payments`, `GET /api/v1/payments/{payment_id}`, and `POST /api/v1/webhooks/payment`.

Puzzle 3 focuses on turning that improved scaffold into a safer payment-domain foundation by tightening model ownership, state transitions, webhook processing, and persistence boundaries.

## Current status before Puzzle 3

The current code shows useful progress:

- `cmd/api/main.go` is thinner and mostly composes config, repositories, services, and handlers.
- Transport routing is separated into `internal/transport/http/router.go`.
- Payment creation uses an idempotency key lookup before creating a new record.
- The OpenAPI spec now uses `GET /api/v1/payments/{payment_id}` instead of the older status endpoint split.
- Webhooks require `X-Webhook-Signature` at the HTTP layer.

However, there are still issues to address before deeper PSP integration begins:

- The domain interfaces still depend on transport DTOs (e.g. `internal/transport/http/dto`), so domain ownership is not yet clean.
- Webhook signatures are only checked for presence, not actually verified.
- The `/ready` response still appears to drift from the documented OpenAPI response shape.
- The in-memory repository file appears to contain a duplicated `package inmemory` declaration that should be fixed immediately if present in the repo.

## Puzzle 3 objective

Establish a stable internal domain model for payments that is independent from transport DTOs and ready for provider adapters and real persistence.

## Target outcomes

By the end of Puzzle 3, the project should have:

- A domain-owned `Payment` aggregate or entity.
- Explicit payment status values and legal transitions.
- Transport DTOs separated from domain types.
- Webhook payload translation into internal events instead of direct ad hoc processing.
- Repository interfaces that persist domain entities, not HTTP-facing request/response shapes.

## Implementation principles for Copilot

- Prefer small PRs with no hidden rewrites.
- Keep OpenAPI as the public contract, but do not let generated or transport models leak into the domain core.
- Preserve current endpoints while tightening internals.
- Introduce seams first, behavior changes second.
- Add tests for each new domain rule as it is introduced.

## PR plan

### PR 1 — Immediate fixes

Goal: remove obvious defects and contract drift without architectural expansion.

Tasks:

- Fix duplicated `package inmemory` declaration if it exists in `internal/storage/inmemory/payment.go`.
- Align `/ready` handler response with the OpenAPI schema, including `time` if the spec requires it.
- Review HTTP status codes for webhook signature failures and make them consistent with the documented contract.
- Ensure all JSON responses use the same shape and field naming style as declared in `openapi.yaml`.

Definition of done:

- Project compiles cleanly.
- `openapi.yaml` and implemented response bodies no longer drift on obvious fields.

### PR 2 — Domain-owned payment model

Goal: make the payment domain independent from transport/shared models.

Tasks:

- Create a domain payment entity under `internal/domain/payment`.
- Move status constants or enum-like values into the domain package.
- Move any invariants such as positive amount and required currency into domain constructors or validation methods where appropriate.
- Refactor repository and service interfaces to depend on domain types instead of `internal/models`.

Suggested target shape:

```text
internal/domain/payment/
  entity.go
  status.go
  repository.go
  service.go
```

Definition of done:

- The domain package no longer imports `internal/models`.
- Repositories accept and return domain payment entities.

### PR 3 — Transport DTO split

Goal: make the legacy `internal/models` either transport-only or replace it with `internal/transport/http/dto`.

Tasks:

- Decide whether to keep any legacy `internal/models` temporarily or replace it with `internal/transport/http/dto`.
- Define request and response DTOs that mirror `openapi.yaml` exactly.
- Add explicit mapping functions between DTOs and domain entities.
- Keep card token or provider-specific request fields out of the core payment entity unless they are truly domain-relevant.

Definition of done:

- HTTP handlers decode DTOs, map to domain input, call services, then map results back to DTOs.
- Domain types are not reused as API response types.

### PR 4 — Payment lifecycle rules

Goal: define the first real state machine for the payment lifecycle.

Canonical statuses should start simple and remain aligned with the spec unless the contract is intentionally revised.

Suggested initial statuses:

- `pending`.
- `processing`.
- `completed`.
- `failed`.
- `cancelled`.

Tasks:

- Define allowed transitions, for example `pending -> processing -> completed|failed`, and `pending -> cancelled`.
- Prevent invalid transitions in domain logic.
- Add unit tests for transition rules.
- Clarify whether webhook events may move a payment directly from `pending` to `completed` depending on provider semantics.

Definition of done:

- Payment status changes are governed by domain rules, not arbitrary string assignment in handlers or services.

### PR 5 — Webhook normalization

Goal: treat webhooks as provider inputs translated into internal events.

Tasks:

- Introduce a simple internal webhook/domain event structure.
- Map external webhook payload fields into that internal structure.
- Add signature verification interface, even if the first implementation is a mock verifier.
- Keep provider-specific parsing in adapter code, not in generic payment services.

Suggested shape:

```text
internal/domain/payment/event.go
internal/provider/webhook/
internal/application/payment/webhook_service.go
```

Definition of done:

- Webhook handler validates signature through an interface and forwards a normalized event to application logic.

### PR 6 — Repository boundary cleanup

Goal: prepare for Postgres or DynamoDB without changing public API behavior.

Tasks:

- Ensure repository interfaces are domain-oriented.
- Keep `inmemory` as a test/dev adapter only.
- Add comments or ADR notes on future persistence adapters such as Postgres or DynamoDB.
- Introduce repository methods required for lifecycle updates, not just create/get.

Definition of done:

- Persistence layer is ready for a second adapter without refactoring the transport layer again.

## Copilot prompts

### Prompt A — Immediate fixes

```md
Review the payment gateway repo and prepare the smallest fix PR for Puzzle 3.

Tasks:
1. Fix any compile-breaking issue, including duplicate package declarations.
2. Align `/ready` response body with `openapi.yaml`.
3. Check webhook error status and response shape against the spec.
4. Do not change architecture yet unless needed for the fix.

Output:
- exact files to edit
- patch plan
- test plan
```

### Prompt B — Domain separation

```md
Refactor the payment domain so it no longer depends on transport DTOs such as `internal/transport/http/dto`.

Tasks:
1. Create a domain-owned payment entity and status definitions.
2. Update repository and service interfaces to use domain types.
3. Keep HTTP DTOs separate from domain entities.
4. Preserve current API behavior.
5. Suggest the smallest safe PR sequence.
```

### Prompt C — Lifecycle rules

```md
Design the initial payment state machine for this project.

Constraints:
- Start with: pending, processing, completed, failed, cancelled.
- Prevent invalid state transitions.
- Keep compatibility with current OpenAPI contract.
- Propose unit tests for each legal and illegal transition.

Output:
- status model
- transition table
- code placement
- test cases
```

### Prompt D — Webhook normalization

```md
Refactor webhook handling for the payment gateway.

Tasks:
1. Introduce a signature verification interface.
2. Normalize webhook payloads into internal events.
3. Keep provider-specific parsing out of generic handlers.
4. Preserve the existing HTTP endpoint.
5. Suggest mock-first implementation before real Stripe adapter work.
```

## Acceptance checklist

Puzzle 3 can be considered complete when these statements are true:

- The project compiles and runs cleanly.
- The domain layer does not depend on transport/shared DTO packages.
- Payment status changes are enforced by domain logic.
- Webhook handling uses a verification seam and normalized internal events.
- The repository interface is ready for non-memory adapters.
- Public HTTP behavior remains compatible unless intentionally changed through a documented API revision.
