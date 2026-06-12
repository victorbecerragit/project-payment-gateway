# Secrets Management — External Secrets Operator (ESO)

How this project manages Kubernetes secrets without committing sensitive values to git.

---

## Overview

Plain Kubernetes `Secret` manifests with base64-encoded values must never be committed to version control — base64 is not encryption. This project uses [External Secrets Operator (ESO)](https://external-secrets.io) to pull secret values from an external backend at runtime and create the `payment-gateway-secrets` K8s Secret automatically.

```
Secret Backend (Fake / AWS SM / GCP SM)
        │
        │  ESO polls every refreshInterval
        ▼
ExternalSecret (CRD) ──creates/syncs──▶ K8s Secret "payment-gateway-secrets"
                                                │
                                                ▼
                                     deployment.yaml (unchanged)
```

The `Deployment` manifest never changes between environments — only the `SecretStore` backend changes.

---

## Files

```
k8s/eso/
├── secretstore-literal.yaml   # Local kind/minikube demo (ESO Fake provider)
├── secretstore-aws.yaml       # Production — AWS Secrets Manager via IRSA
├── secretstore-gcp.yaml       # Production — GCP Secret Manager via Workload Identity
├── externalsecret.yaml        # What secrets to pull (same for all backends)
└── README.md                  # This file
```

> `secretstore-literal.yaml` contains real secret values for local demo.
> It is listed in `.gitignore` and must never be committed.

---

## Secrets managed

| K8s Secret key | Source key | Description |
|---|---|---|
| `STRIPE_API_KEY` | `api_key` | Stripe sandbox or live secret key (`sk_test_` / `sk_live_`) |
| `STRIPE_WEBHOOK_SECRET` | `webhook_secret` | Stripe webhook signing secret (`whsec_`) |
| `DATABASE_URL` | `url` | PostgreSQL DSN (`postgres://user:pass@host:5432/db`) |

---

## Installation

### Prerequisites

- Helm 3
- kubectl configured against your cluster
- ESO CRDs not yet installed (first time only)

### Install ESO via Helm

```bash
helm repo add external-secrets https://charts.external-secrets.io
helm repo update

helm install external-secrets external-secrets/external-secrets \
  --namespace external-secrets \
  --create-namespace \
  --set installCRDs=true

# Verify all ESO pods are Running
kubectl get pods -n external-secrets
```

Expected output:
```
NAME                                                READY   STATUS    RESTARTS   AGE
external-secrets-xxxxxxxxx-xxxxx                    1/1     Running   0          30s
external-secrets-cert-controller-xxxxxxxxx-xxxxx    1/1     Running   0          30s
external-secrets-webhook-xxxxxxxxx-xxxxx            1/1     Running   0          30s
```

---

## Setup by environment

### Local — kind / minikube (Fake provider)

The ESO Fake provider stores plain-text values directly in the `SecretStore` manifest.
No cloud account required.

> ⚠ Add to `.gitignore` before filling in real values:
> ```bash
> echo "k8s/eso/secretstore-literal.yaml" >> .gitignore
> ```

**1. Edit `k8s/eso/secretstore-literal.yaml`** — replace placeholders with real values:

```yaml
spec:
  provider:
    fake:
      data:
        - key: "api_key"
          value: "sk_test_YOUR_REAL_KEY"
        - key: "webhook_secret"
          value: "whsec_YOUR_REAL_SECRET"
        - key: "url"
          value: "postgres://pguser:pgpassword@postgres:5432/payments?sslmode=disable"
```

**2. Apply:**

```bash
kubectl apply -f k8s/eso/secretstore-literal.yaml
kubectl apply -f k8s/eso/externalsecret.yaml
```

**3. Verify:**

```bash
# SecretStore should be Valid / Ready=True
kubectl get secretstore payment-gateway-secret-store -n default

# ExternalSecret should be SecretSynced / Ready=True
kubectl get externalsecret payment-gateway-secrets -n default

# K8s Secret should exist with all three keys
kubectl get secret payment-gateway-secrets -n default
kubectl get secret payment-gateway-secrets -o jsonpath='{.data.STRIPE_API_KEY}' | base64 -d
```

**4. Restart the deployment:**

```bash
kubectl rollout restart deployment/payment-gateway
kubectl rollout status deployment/payment-gateway
```

---

### Production — AWS Secrets Manager (IRSA, no static keys)

**1. Create secrets in AWS:**

```bash
aws secretsmanager create-secret --name payment-gateway/stripe \
  --secret-string '{"api_key":"sk_live_YOUR_KEY","webhook_secret":"whsec_YOUR_SECRET"}'

aws secretsmanager create-secret --name payment-gateway/database \
  --secret-string '{"url":"postgres://user:pass@rds-host:5432/payments?sslmode=require"}'
```

**2. Create IAM role with `secretsmanager:GetSecretValue` and annotate the ESO service account (IRSA).**

**3. Apply:**

```bash
kubectl apply -f k8s/eso/secretstore-aws.yaml
kubectl apply -f k8s/eso/externalsecret.yaml
```

> See `k8s/eso/secretstore-aws.yaml` for the full IRSA configuration.

---

### Production — GCP Secret Manager (Workload Identity)

**1. Create secrets in GCP:**

```bash
echo -n '{"api_key":"sk_live_YOUR_KEY","webhook_secret":"whsec_YOUR_SECRET"}' | \
  gcloud secrets create payment-gateway-stripe --data-file=-

echo -n '{"url":"postgres://user:pass@cloud-sql-host:5432/payments"}' | \
  gcloud secrets create payment-gateway-database --data-file=-
```

**2. Grant the GCP Service Account `roles/secretmanager.secretAccessor` and bind Workload Identity.**

**3. Apply:**

```bash
kubectl apply -f k8s/eso/secretstore-gcp.yaml
kubectl apply -f k8s/eso/externalsecret.yaml
```

> See `k8s/eso/secretstore-gcp.yaml` for the full Workload Identity configuration.

---

## Troubleshooting

### `SecretStore not found`

```bash
kubectl get secretstore -n default
```

If empty, the `SecretStore` was not applied or applied to the wrong namespace.
The `SecretStore` and `ExternalSecret` must be in the **same namespace**.

### `Secret does not exist` / `UpdateFailed`

This means the `SecretStore` is reachable but the remote key lookup failed.

Common causes with the Fake provider:
- `property` field set in `remoteRef` but the stored value is a plain string, not JSON → **remove `property`**
- Key name mismatch between `SecretStore.data[].key` and `ExternalSecret.data[].remoteRef.key` → ensure they are identical

```bash
kubectl describe externalsecret payment-gateway-secrets -n default
kubectl describe secretstore payment-gateway-secret-store -n default
```

### Force re-sync

```bash
kubectl annotate externalsecret payment-gateway-secrets \
  force-sync=$(date +%s) --overwrite -n default
```

### Check ESO controller logs

```bash
kubectl logs -n external-secrets -l app.kubernetes.io/name=external-secrets --tail=50
```

---

## Secret rotation

To rotate a secret (e.g. new Stripe key after a leak):

1. Update the value in your backend (Stripe Dashboard, AWS SM, GCP SM, or `secretstore-literal.yaml`)
2. Force a re-sync if you cannot wait for the `refreshInterval`:
   ```bash
   kubectl annotate externalsecret payment-gateway-secrets \
     force-sync=$(date +%s) --overwrite -n default
   ```
3. Restart the deployment to pick up the new value from the mounted secret:
   ```bash
   kubectl rollout restart deployment/payment-gateway
   ```

> With AWS/GCP backends, ESO auto-syncs every `refreshInterval` (default: 1h).
> The deployment restart is only needed if secrets are mounted as env vars
> (as in this project) rather than as files — env vars are not live-reloaded.

---

## Why not Vault?

HashiCorp Vault requires running and operating a stateful service (HA setup, unsealing,
audit backend, token renewal). For this project and for most small-to-medium teams,
the operational overhead is not justified when AWS Secrets Manager or GCP Secret Manager
are already available as managed services. ESO abstracts the backend cleanly — switching
from Fake → AWS SM → GCP SM is a single `SecretStore` manifest change.

---

## References

- [External Secrets Operator docs](https://external-secrets.io/latest/)
- [ESO Fake provider](https://external-secrets.io/latest/provider/fake/)
- [ESO AWS Secrets Manager](https://external-secrets.io/latest/provider/aws-secrets-manager/)
- [ESO GCP Secret Manager](https://external-secrets.io/latest/provider/google-secrets-manager/)
- [Stripe key management](https://docs.stripe.com/keys)
