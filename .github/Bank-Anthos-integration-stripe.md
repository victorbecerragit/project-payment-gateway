# Copilot Instructions — Bank of Anthos Integration Ideas with Stripe Demo Track

Bank of Anthos is a Kubernetes-native sample banking application used by Google to demonstrate GKE, Anthos Service Mesh, Anthos Config Management, Cloud Operations, Cloud SQL, Cloud Build, and Cloud Deploy patterns.

The Payment Gateway project should use Bank of Anthos mainly as a packaging, deployment, and platform-demo reference, while Stripe should be treated as the first real provider proof point for the payment-gateway POV.

## Goal

Use Bank of Anthos patterns to turn the Payment Gateway into a stronger demo platform, and use Stripe to prove that the gateway works end to end with a real PSP integration.

## Core rule

Keep the domain and application core provider-agnostic, but make Stripe a first-class demo and validation path in the demo, extras, and docs layers.

## What to reuse from Bank of Anthos

The most useful patterns to reuse are:

- Clear separation between application code, deployment assets, docs, extras, and infrastructure folders.
- Demo-oriented support services such as frontend and load generation.
- Optional platform add-ons packaged under `extras/`.
- Strong environment, troubleshooting, and extensibility documentation.

## What Stripe adds to the story

Stripe gives the project a concrete PSP path for demos, testing, and stakeholder validation.

It allows the POV story to become:

- `frontend/demo client -> payment gateway -> Stripe PaymentIntent -> Stripe webhook -> gateway status update`.
- Local and sandbox webhook testing through Stripe CLI forwarding and event triggering.
- Realistic success, failure, and cancellation scenarios instead of mock-only flows.

## Suggested repo shape

```text
cmd/
internal/
deploy/
  k8s/
extras/
  prometheus/
  otel/
  istio/
  jwt/
  backup/
  postgres/
  stripe-sandbox/
demo/
  frontend/
  loadgenerator/
docs/
  environments.md
  troubleshooting.md
  adding-new-provider.md
  demo-setup.md
  stripe-demo.md
iac/
```

## Stripe demo track

### Stripe demo packaging

Add a dedicated `extras/stripe-sandbox/` package.

Suggested contents:

- Example env file for `STRIPE_API_KEY` and `STRIPE_WEBHOOK_SECRET`.
- README with local and Kubernetes setup steps.
- Example webhook forwarding commands using Stripe CLI.
- Example event trigger commands for `payment_intent.succeeded`, failures, and cancellation-like scenarios.

### Stripe demo documentation

Add `docs/stripe-demo.md` focused on proving the integration works.

Suggested sections:

- Local Docker Compose flow.
- Kubernetes flow.
- How to forward webhooks with Stripe CLI.
- How to trigger test events.
- Expected request path and status transitions inside the gateway.
- Common troubleshooting issues: invalid webhook signature, missing metadata, idempotency mismatch, unknown event type.

### Stripe-aware load generator

Extend `demo/loadgenerator` so it can exercise both mock-provider and Stripe-provider modes.

Suggested scenarios:

- Create payment only.
- Create payment plus successful webhook completion.
- Create payment plus failure event.
- Burst traffic to validate observability and idempotency.

### Stripe demo frontend

The demo frontend should show Stripe-backed flows without becoming provider-specific in architecture.

Suggested capabilities:

- Create a payment using the gateway API.
- Show pending/processing/completed/failed state changes.
- Display transaction/provider reference for demo purposes if exposed by the API.
- Include a small section documenting how webhook completion is simulated or triggered.

## PR plan

### PR 1 — Add Stripe demo pack

Goal: make Stripe setup repeatable for local and cluster demos.

Tasks:

- Create `extras/stripe-sandbox/README.md`.
- Add example environment variable files.
- Add Stripe CLI commands for forwarding and triggering events.
- Keep everything optional and outside the core runtime.

Definition of done:

- A contributor can configure Stripe sandbox support from docs without reading source code.

### PR 2 — Add Stripe demo docs

Goal: explain how to prove the gateway works with Stripe end to end.

Tasks:

- Create `docs/stripe-demo.md`.
- Document expected flow from payment creation to webhook-driven completion.
- Add troubleshooting notes and common failure modes.

Definition of done:

- Someone can run the demo and validate success/failure flows step by step.

### PR 3 — Extend load generator for Stripe mode

Goal: generate demo traffic that exercises a real provider path.

Tasks:

- Add mode selection for mock versus Stripe demos.
- Generate realistic idempotency keys.
- Add helper scripts or docs for webhook event triggering.
- Keep the generator useful even when Stripe is not configured.

Definition of done:

- Demo traffic can show real provider-backed lifecycle updates in dashboards and logs.

### PR 4 — Update demo frontend and README story

Goal: reflect Stripe as the first real provider proof point.

Tasks:

- Update the demo frontend scope to show provider-backed state changes.
- Update project README or demo docs to explain the Stripe-backed POV story.
- Keep the architectural message clear: Stripe is a demo provider, not the domain model.

Definition of done:

- The project tells a clear story: platform demo plus real PSP validation.

## Copilot prompts

### Prompt A — Stripe demo pack

```md
Create an `extras/stripe-sandbox` package for the payment gateway platform demo.

Tasks:
1. Add README, env examples, and Stripe CLI commands.
2. Cover local Docker Compose and Kubernetes usage.
3. Keep Stripe setup optional and outside the core runtime.
4. Optimize for demo repeatability.

Output:
- folder contents
- files to create
- setup flow
- validation steps
```

### Prompt B — Stripe demo docs

```md
Create `docs/stripe-demo.md` for the payment gateway.

Tasks:
1. Document end-to-end Stripe demo flow.
2. Explain how to forward webhooks with Stripe CLI.
3. Show how to trigger success and failure events.
4. Add troubleshooting for signature, metadata, and idempotency issues.

Output:
- doc outline
- commands to include
- expected results
```

### Prompt C — Stripe-aware load generator

```md
Extend the payment gateway load generator to support Stripe demo mode.

Tasks:
1. Add provider mode selection: mock or Stripe.
2. Generate payment creation traffic with realistic idempotency keys.
3. Document webhook replay or Stripe CLI trigger usage.
4. Keep the generator runnable in Kubernetes.

Output:
- runtime flags
- request patterns
- demo scenarios
- test plan
```

### Prompt D — Demo story update

```md
Update the payment gateway demo story so Stripe is the first real provider proof point.

Tasks:
1. Update README/demo docs to explain the flow.
2. Keep the domain model provider-agnostic.
3. Highlight Bank of Anthos-inspired packaging and demo enablement.
4. Make the POV easy for stakeholders to understand.

Output:
- wording changes
- affected files
- diagram suggestions
```

## Acceptance checklist

This integration track is successful when these statements are true:

- Stripe is documented as the first real provider demo path.
- Stripe setup is packaged under `extras/` and docs, not scattered through the repo.
- A contributor can run a local Stripe demo using Stripe CLI webhook forwarding.
- The load generator and/or demo frontend can show a provider-backed payment lifecycle.
- The core payment gateway remains provider-agnostic.
