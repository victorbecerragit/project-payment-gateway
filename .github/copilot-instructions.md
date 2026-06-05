Copilot Instructions — Project Payment Gateway

This document defines the working rules for reviewing and evolving the `project-payment-gateway` scaffolding repository.

The current repository already includes a Go API entrypoint, basic HTTP routing, OpenAPI specification, Docker/Kubernetes assets, and health/readiness endpoints, which makes it a valid early scaffold but not yet a production-grade payment gateway foundation.

## Current scaffold summary

Observed structure in the repository

- `cmd/api/main.go` starts an HTTP server and registers the current routes.
- `internal/handlers` and `internal/models` appear to hold request handlers and data models.
- `openapi.yaml` exists and should become the contract source of truth.
- `Dockerfile`, `docker-compose.yaml`, `Makefile`, and `k8s/*.yaml` provide local and Kubernetes deployment scaffolding.
- `README.md` frames the project as Kubernetes-ready and OpenAPI-based.

Current routes wired in `cmd/api/main.go`

- `GET /health`.
- `GET /ready`.
- `POST/GET /api/v1/payments`.
- `GET /api/v1/payments/status`.
- `POST /api/v1/webhooks/payment`.

## Working principles for Copilot

Copilot should optimize for small, reviewable refactors instead of broad rewrites.

### Architecture rules

- Keep `cmd/` thin; startup and dependency wiring only.
- Move business logic out of HTTP handlers and into services/use-cases.
- Separate layers clearly: transport, application, domain, provider adapters, and storage.
- Avoid provider-specific logic in generic payment domain models.
- Prefer interfaces at boundaries, especially for repositories, provider clients, clocks, and idempotency storage.

### API and contract rules

- Treat `openapi.yaml` as the intended public API contract and reduce drift between code and spec.
- Standardize request/response schemas, error responses, and payment status vocabulary before expanding endpoints.
- Add idempotency support for payment-creation flows.
- Design webhooks as provider-specific inputs translated into internal domain events.

### Reliability and security rules

- Never log raw secrets, payment credentials, or sensitive webhook payload fields.
- Add request IDs / correlation IDs to logs and responses where relevant.
- Validate webhook signatures before processing provider events.
- Add explicit timeouts around outbound provider calls.
- Keep the service stateless; store durable state externally.

### Open source rules

- Prefer clear package names over framework-heavy abstractions.
- Keep examples and local developer startup simple.
- Every refactor step should preserve a runnable state where possible.
- Add tests together with seams/interfaces, not after the whole refactor.

## Refactor target shape

Target package direction:

```text
cmd/api/
internal/transport/http/
internal/application/
internal/domain/
internal/provider/
internal/storage/
internal/platform/
```

Suggested responsibilities:

- `transport/http`: request parsing, response mapping, middleware, routing.
- `application`: payment orchestration use-cases, command handlers, idempotency coordination.
- `domain`: payment aggregate, status transitions, invariants, value objects.
- `provider`: Stripe/mock/other PSP adapters.
- `storage`: repositories and persistence implementations.
- `platform`: config, logging, tracing, clock, uuid helpers, external clients.

## Puzzle 1 — Architecture baseline review

Goal: understand the scaffold precisely and create a safe refactor seam without changing external behavior.

### Scope

Review these files first:

- `cmd/api/main.go`.
- `internal/handlers/handlers.go`.
- `internal/models/payment.go`.
- `Dockerfile`.
- `Makefile`.
- `docker-compose.yaml`.
- `k8s/*.yaml`.

### Questions to answer

- What logic currently lives in handlers that should move to services?
- Are models transport DTOs, domain models, or mixed concerns?
- Is config loaded centrally or scattered?
- Are health/readiness probes superficial or tied to actual dependencies?
- Are Kubernetes manifests aligned with the application’s real behavior and ports?

### Deliverables

1. A red/yellow/green inventory of the current architecture.
2. A dependency map: entrypoint → router → handlers → models/external calls.
3. A first package split proposal that preserves route compatibility.
4. A list of “safe first refactors” with no behavior change.

### Safe first refactors

- Introduce `PaymentService` and `HealthService` interfaces.
- Keep current endpoints but call service methods from handlers.
- Move response formatting helpers into transport layer helpers.
- Centralize config/env parsing into a single config package.
- Add structured logger injection instead of package-global logging where practical.

### Definition of done

Puzzle 1 is complete when the codebase has a clear dependency direction and the HTTP layer no longer owns business rules.

## Puzzle 2 — OpenAPI review and contract alignment

Goal: make the API contract explicit, coherent, and ready to drive future implementation.

### Scope

Review `openapi.yaml` in detail against the current code paths and expected payment lifecycle behavior.

### Review checklist

- Confirm resource naming and URL consistency for payment operations.
- Decide whether `GET /api/v1/payments/status` should remain separate or become `GET /api/v1/payments/{id}` with status in the resource.
- Define canonical payment statuses and transitions.
- Standardize success and error response envelopes.
- Add schema fields for idempotency key, provider reference, timestamps, and webhook event IDs if missing.
- Clarify webhook endpoint contract and authentication/signature expectations.
- Define which fields are client-facing versus internal-only.

### Deliverables

1. An API contract gap list: current spec vs desired domain behavior.
2. A list of breaking vs non-breaking API changes.
3. A normalized payment schema proposal.
4. A recommendation on whether code should be generated from OpenAPI, validated against it, or both.

### Recommended design direction

- Use OpenAPI as the contract source of truth for public HTTP DTOs.
- Keep generated models isolated from the core domain layer.
- Map transport DTOs to internal domain types explicitly.
- Start with one provider and one happy-path payment lifecycle before adding more PSP complexity.

### Definition of done

Puzzle 2 is complete when the spec can be used as a reliable implementation contract and there is no ambiguity around endpoint purpose, schema ownership, or status vocabulary.

## Suggested next Copilot prompts

### Prompt for Puzzle 1

```md
Review this Go payment gateway scaffold and propose a no-behavior-change refactor.

Focus on:
- cmd/api/main.go
- internal/handlers/handlers.go
- internal/models/payment.go
- config and dependency wiring

Tasks:
1. Identify mixed responsibilities.
2. Propose target package boundaries.
3. Introduce service interfaces for handlers.
4. Suggest the smallest first PR.
5. Do not rewrite everything; keep current routes intact.
```

### Prompt for Puzzle 2

```md
Review openapi.yaml for this payment gateway scaffold and compare it to the currently implemented HTTP routes.

Tasks:
1. Identify mismatches and missing schemas.
2. Normalize payment resource design.
3. Recommend canonical payment status values.
4. Suggest breaking vs non-breaking contract fixes.
5. Propose whether to use code generation or manual transport DTOs.

Constraints:
- Keep OpenAPI as public contract.
- Keep domain models separate from transport models.
- Optimize for incremental refactor, not full rewrite.
```



## Execution order

Work in this order:

1. Puzzle 1 architecture inventory.
2. Puzzle 1 safe seam refactor.
3. Puzzle 2 OpenAPI review.
4. Puzzle 2 contract normalization proposal.
5. Only then start persistence/provider-specific implementation work.

