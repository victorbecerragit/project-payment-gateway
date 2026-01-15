
# Payment Gateway Integration with OpenAPI & Kubernetes
## A DevOps/SRE Quick Start Guide

**Target Audience:** DevOps/SRE professionals (non-developers)
**Status:** Implementation guide for OpenAPI + Kubernetes + PSP Integration
**Date:** January 2026

---

## 📋 Table of Contents

1. [Overview for DevOps/SRE](#overview-for-devopssre)
2. [Architecture High-Level](#architecture-high-level)
3. [What Developers Will Build](#what-developers-will-build)
4. [Your DevOps/SRE Responsibilities](#your-devopssre-responsibilities)
5. [Quick Setup Guide](#quick-setup-guide)
6. [Kubernetes-Safe Requirements (Your Focus)](#kubernetes-safe-requirements-your-focus)
7. [Docker & Containerization](#docker--containerization)
8. [CI/CD Pipeline Setup](#cicd-pipeline-setup)
9. [Monitoring, Logging & Tracing](#monitoring-logging--tracing)
10. [Common Deployment Patterns](#common-deployment-patterns)
11. [Troubleshooting Checklist](#troubleshooting-checklist)

---

## 🎯 Overview for DevOps/SRE

This is a **Payment Gateway Service** that acts as an abstraction layer between your application and Payment Service Providers (Stripe, PayPal, etc.).

### Why This Matters for DevOps/SRE:

- **Multi-instance Ready:** Must run on Kubernetes with multiple replicas (HA config)
- **Idempotent Operations:** Payments must never be duplicated across pod replicas
- **Distributed Tracing:** Every transaction needs a `correlationId` trackable across your ecosystem
- **Stateless Design:** No local state—all data in persistent storage (database)
- **Event-Driven:** Publishes events via RabbitMQ for asynchronous processing
- **Zero-Downtime Deployments:** Rolling updates with health checks

### Key Acronyms:

- **PSP:** Payment Service Provider (Stripe, PayPal, etc.)
- **OpenAPI:** Standard specification for REST APIs (machine-readable)
- **HA:** High Availability (multiple instances, load balanced)
- **Idempotency:** Same operation run 100 times = same result as running once
- **Correlation ID:** Unique trace ID following a transaction through all services

---

## 🏗️ Architecture High-Level

```
┌─────────────────────────────────────────────────────────────────┐
│                       External World                            │
│         (Client Apps, Stripe, PayPal, RabbitMQ)                │
└──────────────────────────────────────────────────────────────────┘
                              ↑ ↓
┌──────────────────────────────────────────────────────────────────┐
│            Kubernetes Cluster (Your Infrastructure)             │
│                                                                  │
│  ┌────────────────────────────────────────────────────────┐    │
│  │  Ingress / Load Balancer (Routes to pods)            │    │
│  └────────────────────────────────────────────────────────┘    │
│                           ↓                                      │
│  ┌────────────────────────────────────────────────────────┐    │
│  │  Payment Gateway Service (3+ Pods - HA)              │    │
│  │  ├─ Pod 1: payment-gateway-abc123                    │    │
│  │  ├─ Pod 2: payment-gateway-def456                    │    │
│  │  ├─ Pod 3: payment-gateway-ghi789                    │    │
│  │  └─ SpringBoot + OpenAPI + RabbitMQ Client           │    │
│  └────────────────────────────────────────────────────────┘    │
│           ↓ ↓ ↓ (All query same database & RabbitMQ)            │
│  ┌────────────────────────────────────────────────────────┐    │
│  │  Persistent Storage (Outside Kubernetes)             │    │
│  │  ├─ PostgreSQL Database (Transactions, Configs)       │    │
│  │  └─ RabbitMQ Service (Event Broker)                   │    │
│  └────────────────────────────────────────────────────────┘    │
│                           ↑ ↓                                    │
└──────────────────────────────────────────────────────────────────┘
                              ↑ ↓
┌──────────────────────────────────────────────────────────────────┐
│              External Payment Providers & APIs                   │
│    (Stripe, PayPal APIs - called from inside cluster)           │
└──────────────────────────────────────────────────────────────────┘
```

---

## 💻 What Developers Will Build

You don't need to understand all the code, but here's what to expect:

### 1. OpenAPI Specification (YAML file)
```yaml
# This is the "contract" defining the API
openapi: 3.0.0
info:
  title: Payment Gateway API
  version: 1.0.0
paths:
  /api/payments:
    post:
      summary: Create Payment
      operationId: createPayment
      # ... more details
```

**What you care about:** This is the source of truth. Everything (documentation, code generation, testing) comes from this.

### 2. Generated Java Code
The developers use the OpenAPI spec to auto-generate skeleton code. They then implement:
- PSP Adapters (Strategy Pattern) for Stripe, PayPal, NoOp
- Business logic for processing payments
- Database models and migrations
- RabbitMQ event producers

### 3. Configuration (application.yaml)
```yaml
payment-gateway:
  providers:
    - name: stripe
      api-key: ${STRIPE_API_KEY}
      webhook-secret: ${STRIPE_WEBHOOK_SECRET}
    - name: paypal
      client-id: ${PAYPAL_CLIENT_ID}
      client-secret: ${PAYPAL_CLIENT_SECRET}
```

**What you care about:** This goes into Kubernetes Secrets/ConfigMaps.

---

## 🛠️ Your DevOps/SRE Responsibilities

### Primary Focus Areas:

#### 1. **Kubernetes Deployment Configuration**
- Deployments (define replicas, resources)
- Services (internal & external networking)
- ConfigMaps (non-sensitive config)
- Secrets (API keys, credentials)
- PodDisruptionBudgets (for HA)
- NetworkPolicies (if using service mesh)

#### 2. **Idempotency & Distributed Safety**
- Understand how the app prevents duplicate payments
- Ensure database transactions are handled correctly
- Monitor for race conditions in logs
- Setup proper retry logic in API calls

#### 3. **Observability Stack**
- Distributed Tracing (Jaeger/Zipkin) for correlationId tracking
- Prometheus for metrics collection
- ELK/Loki for centralized logging
- RabbitMQ monitoring

#### 4. **Docker & Container Security**
- Review Dockerfile for vulnerabilities
- Setup image scanning (Trivy)
- Security context in K8s (read-only filesystems, non-root)

#### 5. **CI/CD Pipeline**
- GitHub Actions workflows for build/test
- Artifact storage (container registry)
- Deployment automation
- Rollback procedures

#### 6. **Database & Persistence**
- PostgreSQL setup & maintenance
- Flyway migration management
- Backup/recovery procedures
- Connection pooling configuration

---

## 🚀 Quick Setup Guide

### Step 1: Clone & Understand Project Structure

```bash
git clone https://github.com/Labs64/labs64.io-payment-gateway.git
cd labs64.io-payment-gateway

# Project structure
tree -L 2
```

Expected structure:
```
├── src/
│   ├── main/java/          # Java source code
│   ├── main/resources/
│   │   ├── db/migration/   # Flyway SQL scripts
│   │   └── application.yaml # Config template
│   └── test/java/          # Unit tests
├── docker/
│   └── Dockerfile          # Your container spec
├── kubernetes/             # K8s manifests (YOU CREATE THESE)
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── configmap.yaml
│   └── secrets.yaml
├── .github/workflows/      # CI/CD pipelines
├── openapi/
│   └── payment-gateway.yaml # API specification
└── docker-compose.yml      # Local dev environment
```

### Step 2: Local Development Setup with Docker Compose

The developers provide a `docker-compose.yml` for local testing:

```bash
# Start all services locally
docker-compose up -d

# Check services are running
docker ps
# You should see:
# - payment-gateway (Spring Boot app)
# - postgres (Database)
# - rabbitmq (Message broker)

# View logs
docker-compose logs -f payment-gateway

# Stop everything
docker-compose down
```

### Step 3: Review Dockerfile

The developers create a Dockerfile. You need to review it for:

```dockerfile
FROM eclipse-temurin:21-jre-alpine:latest  # Base image

WORKDIR /app
COPY build/libs/payment-gateway.jar app.jar

# Security: Run as non-root
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

# Health check endpoint
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s \
  CMD curl -f http://localhost:8080/actuator/health || exit 1

EXPOSE 8080
ENTRYPOINT ["java", "-jar", "app.jar"]
```

**Your DevOps checklist:**
- [ ] Uses minimal base image (alpine or distroless)
- [ ] Doesn't run as root
- [ ] Has HEALTHCHECK defined
- [ ] Exposes correct ports
- [ ] Multi-stage build (if large)

### Step 4: Create Kubernetes Manifests

Create a `kubernetes/` directory with these files:

#### A. Namespace (optional but recommended)
```yaml
# kubernetes/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: payment-gateway
```

#### B. ConfigMap (non-sensitive config)
```yaml
# kubernetes/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: payment-gateway-config
  namespace: payment-gateway
data:
  application.yaml: |
    spring:
      application:
        name: payment-gateway
      jpa:
        hibernate:
          ddl-auto: validate
      rabbitmq:
        host: rabbitmq-service
        port: 5672
    
    payment-gateway:
      providers:
        - name: stripe
          enabled: true
        - name: paypal
          enabled: true
        - name: noop
          enabled: true
```

#### C. Secret (sensitive data)
```yaml
# kubernetes/secrets.yaml
apiVersion: v1
kind: Secret
metadata:
  name: payment-gateway-secrets
  namespace: payment-gateway
type: Opaque
stringData:
  stripe-api-key: "sk_test_xxxxx"
  stripe-webhook-secret: "whsec_xxxxx"
  paypal-client-id: "AXxxx"
  paypal-client-secret: "ELxxx"
  db-password: "postgres-password-here"
```

**⚠️ Security Note:** Never commit secrets to git! Use:
- Sealed Secrets (kubeseal)
- External Secrets Operator
- HashiCorp Vault
- Cloud provider's secret manager

#### D. Deployment (Your Main Focus)
```yaml
# kubernetes/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: payment-gateway
  namespace: payment-gateway
  labels:
    app: payment-gateway
    version: v1
spec:
  replicas: 3  # HA: 3 instances across cluster
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0  # Zero-downtime deployments
  selector:
    matchLabels:
      app: payment-gateway
  template:
    metadata:
      labels:
        app: payment-gateway
      annotations:
        correlation-id-enabled: "true"  # For distributed tracing
    spec:
      serviceAccountName: payment-gateway
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        fsReadOnlyRootFilesystem: true
      
      containers:
      - name: payment-gateway
        image: your-registry/payment-gateway:v1.0.0
        imagePullPolicy: IfNotPresent
        
        ports:
        - name: http
          containerPort: 8080
          protocol: TCP
        
        # Mount config from ConfigMap
        volumeMounts:
        - name: config
          mountPath: /app/config
          readOnly: true
        - name: tmp
          mountPath: /tmp
        
        # Environment variables (from Secrets)
        env:
        - name: STRIPE_API_KEY
          valueFrom:
            secretKeyRef:
              name: payment-gateway-secrets
              key: stripe-api-key
        - name: PAYPAL_CLIENT_ID
          valueFrom:
            secretKeyRef:
              name: payment-gateway-secrets
              key: paypal-client-id
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: payment-gateway-secrets
              key: db-password
        
        # RabbitMQ connection
        - name: RABBITMQ_HOST
          value: "rabbitmq-service.payment-gateway.svc.cluster.local"
        - name: RABBITMQ_PORT
          value: "5672"
        
        # Database connection
        - name: DB_HOST
          value: "postgres-service.payment-gateway.svc.cluster.local"
        - name: DB_PORT
          value: "5432"
        - name: DB_NAME
          value: "payment_gateway"
        - name: DB_USER
          value: "postgres"
        
        # Resource limits (CRITICAL for Kubernetes)
        resources:
          requests:
            memory: "512Mi"
            cpu: "250m"
          limits:
            memory: "1Gi"
            cpu: "500m"
        
        # Health checks
        livenessProbe:
          httpGet:
            path: /actuator/health/liveness
            port: http
          initialDelaySeconds: 30
          periodSeconds: 10
          timeoutSeconds: 3
          failureThreshold: 3
        
        readinessProbe:
          httpGet:
            path: /actuator/health/readiness
            port: http
          initialDelaySeconds: 10
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 3
        
        # Graceful shutdown (K8s sends SIGTERM, app has 30 seconds)
        lifecycle:
          preStop:
            exec:
              command: ["/bin/sh", "-c", "sleep 15"]
      
      # Volumes
      volumes:
      - name: config
        configMap:
          name: payment-gateway-config
      - name: tmp
        emptyDir:
          sizeLimit: 1Gi
      
      # Pod disruption budget (HA)
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: app
                  operator: In
                  values:
                  - payment-gateway
              topologyKey: kubernetes.io/hostname
```

#### E. Service (Expose to other pods)
```yaml
# kubernetes/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: payment-gateway-service
  namespace: payment-gateway
spec:
  type: ClusterIP  # Internal only, exposed via Ingress
  ports:
  - name: http
    port: 8080
    targetPort: http
    protocol: TCP
  selector:
    app: payment-gateway
  sessionAffinity: ClientIP  # Route same client to same pod
```

#### F. PodDisruptionBudget (HA Protection)
```yaml
# kubernetes/pod-disruption-budget.yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: payment-gateway-pdb
  namespace: payment-gateway
spec:
  minAvailable: 2  # Keep at least 2 pods running during updates
  selector:
    matchLabels:
      app: payment-gateway
```

#### G. ServiceAccount & RBAC (least privilege)
```yaml
# kubernetes/rbac.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: payment-gateway
  namespace: payment-gateway
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: payment-gateway-role
  namespace: payment-gateway
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get"]
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: payment-gateway-rolebinding
  namespace: payment-gateway
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: payment-gateway-role
subjects:
- kind: ServiceAccount
  name: payment-gateway
  namespace: payment-gateway
```

#### H. Ingress (External Access)
```yaml
# kubernetes/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: payment-gateway-ingress
  namespace: payment-gateway
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/rate-limit: "10"
spec:
  ingressClassName: nginx  # Or your ingress controller
  tls:
  - hosts:
    - payment-gateway.example.com
    secretName: payment-gateway-tls
  rules:
  - host: payment-gateway.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: payment-gateway-service
            port:
              number: 8080
```

---

## 🔐 Kubernetes-Safe Requirements (Your Focus)

### 1. Idempotency (Prevent Duplicate Payments)

**The Problem:** If a pod crashes mid-payment, another pod might retry the same payment.

**How the app handles it:**
- Every payment request gets an **idempotency key** (unique ID)
- Database stores: `(tenant_id, payment_id, idempotency_key) → transaction_result`
- If same key arrives again, return cached result instead of processing

**Your DevOps role:**
```yaml
# Monitor for duplicate payment attempts in logs
kubectl logs -n payment-gateway deployment/payment-gateway | grep "DUPLICATE_KEY"

# Alert on these patterns
- "Idempotency key already processed"
- "Transaction already exists"
- "Retry limit exceeded"
```

### 2. Distributed Tracing (Correlation ID)

**The Problem:** Payment starts in app-1 → goes to Stripe → returns to app-2. How do you trace it?

**Solution:** Every request gets a `correlation-id` header:
```
Request 1 (app instance 1)
  ↓ Header: X-Correlation-Id: abc-123-def-456
  Stripe API
  ↓
Request 2 (app instance 2)
  RabbitMQ message with same correlation ID
  ↓
Log entry with correlation ID
```

**Your DevOps role:**

Setup distributed tracing backend (example: Jaeger):

```yaml
# kubernetes/jaeger.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: jaeger-config
data:
  jaeger-agent-host: "jaeger-agent"
  jaeger-agent-port: "6831"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: jaeger
spec:
  replicas: 1
  selector:
    matchLabels:
      app: jaeger
  template:
    metadata:
      labels:
        app: jaeger
    spec:
      containers:
      - name: jaeger
        image: jaegertracing/all-in-one:latest
        ports:
        - name: zipkin
          containerPort: 9411
        - name: jaeger-agent
          containerPort: 6831
          protocol: UDP
```

**Configure app to use it:**
```yaml
# In deployment env vars
- name: OTEL_EXPORTER_OTLP_ENDPOINT
  value: "http://jaeger-agent:6831"
- name: OTEL_SERVICE_NAME
  value: "payment-gateway"
```

### 3. Race Condition Prevention

**The Problem:** Multiple pods process same payment simultaneously.

**How the app handles it:**
- Optimistic locking (database version column)
- Distributed locks (Redis) for critical sections
- Event sourcing pattern (immutable transaction log)

**Your DevOps role:**
```bash
# Monitor database connection pool
kubectl exec -it deployment/payment-gateway -- \
  curl localhost:8080/actuator/metrics/hikaricp.connections

# Alert if: active connections > max connections
# This indicates contention or leak

# Monitor transaction locks
SELECT * FROM pg_locks WHERE NOT granted;

# If many locks, might indicate race conditions
```

### 4. Stateless Design

**The Problem:** If pod-1 crashes, data is lost.

**How the app handles it:**
- All state goes to PostgreSQL (persistent)
- All events go to RabbitMQ (durable)
- Pods are completely stateless (can be killed anytime)

**Your DevOps role:**
```bash
# Verify no local data storage
kubectl describe pod -n payment-gateway payment-gateway-xyz

# Check volumes - should only be:
# - configMap (config)
# - secrets (credentials)
# - emptyDir (temp files only, not for state)

# ✅ Good: volumeMounts only for config/secrets
# ❌ Bad: emptyDir used to store payment history
```

---

## 🐳 Docker & Containerization

### Build the Container

```bash
# From project root
./gradlew clean build -x test  # Build JAR

# Create image
docker build -f docker/Dockerfile -t payment-gateway:v1.0.0 .

# Tag for registry
docker tag payment-gateway:v1.0.0 \
  your-registry.azurecr.io/payment-gateway:v1.0.0

# Push to registry
docker push your-registry.azurecr.io/payment-gateway:v1.0.0
```

### Scan for Vulnerabilities

```bash
# Install Trivy
curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin

# Scan image
trivy image your-registry.azurecr.io/payment-gateway:v1.0.0

# Scan with threshold
trivy image --severity HIGH,CRITICAL \
  your-registry.azurecr.io/payment-gateway:v1.0.0

# Output should have: 0 HIGH, 0 CRITICAL
```

### Security Best Practices

```dockerfile
# ❌ BAD
FROM openjdk:17  # Large, includes dev tools
USER root
COPY app.jar /app/app.jar

# ✅ GOOD
FROM eclipse-temurin:21-jre-alpine:latest  # Minimal
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
COPY --chown=appuser:appgroup app.jar /app/app.jar
USER appuser
HEALTHCHECK --interval=30s CMD curl -f http://localhost:8080/actuator/health
```

---

## 🔄 CI/CD Pipeline Setup

The developers provide `.github/workflows/` files. You need to understand them:

### Pipeline Stages:

```
┌─────────────────────────────────────────────────────┐
│  1. Code Push to GitHub                            │
└────────────────────┬────────────────────────────────┘
                     ↓
┌─────────────────────────────────────────────────────┐
│  2. Build & Test (GitHub Actions)                  │
│     - ./gradlew build                               │
│     - Run unit tests (must pass >80% coverage)      │
│     - Run integration tests                         │
└────────────────────┬────────────────────────────────┘
                     ↓
┌─────────────────────────────────────────────────────┐
│  3. Container Build & Scan                          │
│     - Build Docker image                            │
│     - Scan with Trivy (no HIGH/CRITICAL)            │
│     - Push to registry                              │
└────────────────────┬────────────────────────────────┘
                     ↓
┌─────────────────────────────────────────────────────┐
│  4. Deploy to Dev/Staging (Automatic)               │
│     - Apply K8s manifests                           │
│     - Run smoke tests                               │
│     - Run security tests                            │
└────────────────────┬────────────────────────────────┘
                     ↓
┌─────────────────────────────────────────────────────┐
│  5. Manual Approval → Deploy to Production          │
│     - Blue-green or canary deployment               │
│     - Health checks pass                            │
└────────────────────┬────────────────────────────────┘
                     ↓
┌─────────────────────────────────────────────────────┐
│  6. Post-Deployment                                 │
│     - Smoke tests                                   │
│     - Rollback if needed                            │
└─────────────────────────────────────────────────────┘
```

### Example GitHub Actions Workflow (you create/review):

```yaml
# .github/workflows/build-deploy.yml
name: Build & Deploy Payment Gateway

on:
  push:
    branches: [main]
    paths:
      - 'src/**'
      - 'docker/**'
      - 'kubernetes/**'
      - '.github/workflows/build-deploy.yml'

env:
  REGISTRY: your-registry.azurecr.io
  IMAGE_NAME: payment-gateway

jobs:
  build-and-test:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up JDK 21
      uses: actions/setup-java@v3
      with:
        java-version: '21'
        distribution: 'temurin'
    
    - name: Build with Gradle
      run: |
        ./gradlew clean build \
          --info \
          --stacktrace
    
    - name: Run unit tests
      run: ./gradlew test
    
    - name: Check code coverage
      run: ./gradlew jacocoTestReport
    
    - name: Archive code coverage
      uses: actions/upload-artifact@v3
      with:
        name: code-coverage-report
        path: build/reports/jacoco/

  security-scan:
    runs-on: ubuntu-latest
    needs: build-and-test
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Run Trivy security scan
      uses: aquasecurity/trivy-action@master
      with:
        scan-type: 'fs'
        scan-ref: '.'
        format: 'sarif'
        output: 'trivy-results.sarif'
    
    - name: Upload to GitHub Security
      uses: github/codeql-action/upload-sarif@v2
      with:
        sarif_file: 'trivy-results.sarif'

  build-and-push-image:
    runs-on: ubuntu-latest
    needs: [build-and-test, security-scan]
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Docker Buildx
      uses: docker/setup-buildx-action@v2
    
    - name: Log in to registry
      uses: docker/login-action@v2
      with:
        registry: ${{ env.REGISTRY }}
        username: ${{ secrets.REGISTRY_USERNAME }}
        password: ${{ secrets.REGISTRY_PASSWORD }}
    
    - name: Extract metadata
      id: meta
      uses: docker/metadata-action@v4
      with:
        images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
        tags: |
          type=ref,event=branch
          type=sha,prefix={{branch}}-
          type=semver,pattern={{version}}
          type=semver,pattern={{major}}.{{minor}}
    
    - name: Build and push image
      uses: docker/build-push-action@v4
      with:
        context: .
        file: ./docker/Dockerfile
        push: true
        tags: ${{ steps.meta.outputs.tags }}
        labels: ${{ steps.meta.outputs.labels }}
        cache-from: type=registry,ref=${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:buildcache
        cache-to: type=registry,ref=${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:buildcache,mode=max

  deploy-to-dev:
    runs-on: ubuntu-latest
    needs: build-and-push-image
    if: github.ref == 'refs/heads/main'
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Deploy to DEV cluster
      run: |
        kubectl apply -f kubernetes/ \
          --namespace=payment-gateway-dev \
          --record
    
    - name: Wait for rollout
      run: |
        kubectl rollout status deployment/payment-gateway \
          --namespace=payment-gateway-dev \
          --timeout=5m
    
    - name: Run smoke tests
      run: |
        kubectl run smoke-test \
          --image=curlimages/curl:latest \
          --rm -i --restart=Never \
          -- curl -f http://payment-gateway-service:8080/actuator/health

  deploy-to-production:
    runs-on: ubuntu-latest
    needs: build-and-push-image
    if: github.ref == 'refs/heads/main'
    
    environment:
      name: production
      url: https://payment-gateway.example.com
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Notify Slack - Deployment starting
      uses: slackapi/slack-github-action@v1
      with:
        payload: |
          {
            "text": "🚀 Payment Gateway deployment starting...",
            "blocks": [
              {
                "type": "section",
                "text": {
                  "type": "mrkdwn",
                  "text": "Payment Gateway Production Deployment\nAuthor: ${{ github.actor }}\nCommit: ${{ github.sha }}"
                }
              }
            ]
          }
    
    - name: Deploy to PROD cluster
      run: |
        kubectl apply -f kubernetes/ \
          --namespace=payment-gateway-prod \
          --record
    
    - name: Wait for rollout
      run: |
        kubectl rollout status deployment/payment-gateway \
          --namespace=payment-gateway-prod \
          --timeout=10m
    
    - name: Verify health
      run: |
        kubectl get pods -n payment-gateway-prod -l app=payment-gateway
        kubectl logs -n payment-gateway-prod deployment/payment-gateway --tail=50
    
    - name: Run production smoke tests
      run: |
        kubectl run prod-smoke-test \
          --image=curlimages/curl:latest \
          --rm -i --restart=Never \
          -- curl -f https://payment-gateway.example.com/actuator/health
    
    - name: Notify Slack - Deployment success
      if: success()
      uses: slackapi/slack-github-action@v1
      with:
        payload: |
          {
            "text": "✅ Payment Gateway deployed successfully",
            "attachments": [
              {
                "color": "good",
                "text": "Production deployment completed and healthy"
              }
            ]
          }
    
    - name: Notify Slack - Deployment failed
      if: failure()
      uses: slackapi/slack-github-action@v1
      with:
        payload: |
          {
            "text": "❌ Payment Gateway deployment FAILED",
            "attachments": [
              {
                "color": "danger",
                "text": "Check GitHub Actions logs for details"
              }
            ]
          }
```

---

## 📊 Monitoring, Logging & Tracing

### 1. Prometheus Metrics

```yaml
# kubernetes/prometheus-servicemonitor.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: payment-gateway
  namespace: payment-gateway
spec:
  selector:
    matchLabels:
      app: payment-gateway
  endpoints:
  - port: http
    path: /actuator/prometheus
    interval: 30s
```

**Metrics to alert on:**
```
payment_transaction_total{status="success"}
payment_transaction_total{status="failed"}
payment_processing_duration_seconds{provider="stripe"}
payment_processing_duration_seconds{provider="paypal"}
rabbitmq_messages_published_total
db_connection_pool_active
```

### 2. ELK / Loki Logging

**Fluent Bit DaemonSet** (collect logs):
```yaml
# kubernetes/fluent-bit.yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: fluent-bit
  namespace: logging
spec:
  selector:
    matchLabels:
      k8s-app: fluent-bit
  template:
    metadata:
      labels:
        k8s-app: fluent-bit
    spec:
      containers:
      - name: fluent-bit
        image: fluent/fluent-bit:latest
        volumeMounts:
        - name: varlog
          mountPath: /var/log
        - name: varlibdockercontainers
          mountPath: /var/lib/docker/containers
          readOnly: true
      volumes:
      - name: varlog
        hostPath:
          path: /var/log
      - name: varlibdockercontainers
        hostPath:
          path: /var/lib/docker/containers
```

### 3. Distributed Tracing (OpenTelemetry)

Logs should include:
```json
{
  "timestamp": "2026-01-15T10:30:45.123Z",
  "correlation_id": "abc-123-def-456",
  "span_id": "span-789",
  "trace_id": "trace-456",
  "level": "INFO",
  "logger": "com.example.PaymentService",
  "message": "Processing payment for tenant: abc123",
  "tenant_id": "abc123",
  "payment_id": "pay-456",
  "provider": "stripe"
}
```

**Never log:**
- Credit card numbers
- API keys or secrets
- Personal identification numbers
- Passwords

---

## 🎯 Common Deployment Patterns

### Pattern 1: Blue-Green Deployment (Zero-Downtime)

```bash
# Current: Blue environment (v1.0.0)
# Prepare: Green environment (v1.1.0)

# Deploy v1.1.0 to new pods
kubectl set image deployment/payment-gateway-green \
  payment-gateway=registry/payment-gateway:v1.1.0 \
  --namespace=payment-gateway

# Health checks pass on Green
kubectl rollout status deployment/payment-gateway-green

# Switch traffic: Blue → Green
kubectl patch service payment-gateway-service -p \
  '{"spec":{"selector":{"version":"v1.1.0"}}}'

# Keep Blue running in case rollback needed
# Rollback: just switch selector back
```

### Pattern 2: Canary Deployment (Gradual Rollout)

```bash
# Route 10% traffic to v1.1.0, 90% to v1.0.0
# Monitor metrics for 1 hour
# If healthy: increase to 50%/50%
# Then: 100% v1.1.0

# Using Istio/Flagger:
kubectl apply -f - <<EOF
apiVersion: flagger.app/v1beta1
kind: Canary
metadata:
  name: payment-gateway
  namespace: payment-gateway
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: payment-gateway
  progressDeadlineSeconds: 3600
  service:
    port: 8080
  analysis:
    interval: 1m
    threshold: 5
    maxWeight: 50
    stepWeight: 5
    metrics:
    - name: request-success-rate
      thresholdRange:
        min: 99
      interval: 1m
    - name: request-duration
      thresholdRange:
        max: 500
      interval: 30s
  skipAnalysis: false
EOF
```

### Pattern 3: Rolling Update (Default K8s)

```yaml
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxSurge: 1        # 1 extra pod during update
    maxUnavailable: 0  # Never take pod down (high availability)
```

---

## 🔍 Troubleshooting Checklist

### Pod Not Starting

```bash
# Check pod status
kubectl describe pod -n payment-gateway payment-gateway-xyz

# Check logs
kubectl logs -n payment-gateway payment-gateway-xyz

# Common issues:
# - ImagePullBackOff: Registry credentials wrong
# - CrashLoopBackOff: App crashed (check logs)
# - Pending: Resource requests too high, no nodes available
```

### Payments Failing

```bash
# Check app logs for specific transaction
kubectl logs -n payment-gateway deployment/payment-gateway | \
  grep "correlation_id=abc-123"

# Check RabbitMQ message queue
kubectl exec -it svc/rabbitmq -- \
  rabbitmqctl list_queues name messages

# Check database transactions
kubectl exec -it svc/postgres -- psql -U postgres -d payment_gateway -c \
  "SELECT * FROM payment_transactions WHERE status='FAILED';"
```

### High Memory Usage

```bash
# Check resource usage
kubectl top pods -n payment-gateway

# Check Kubernetes resource limits
kubectl describe deployment payment-gateway -n payment-gateway

# If pod is using >80% of limit:
# Option 1: Increase limit in deployment
# Option 2: Check for memory leak (contact developers)
# Option 3: Check for excessive logging
```

### Database Connection Issues

```bash
# Check connection pool
kubectl exec -it deployment/payment-gateway -n payment-gateway -- \
  curl localhost:8080/actuator/metrics/hikaricp.connections.active

# Should be < max connections (usually 20)

# If stuck: Restart pod
kubectl rollout restart deployment/payment-gateway -n payment-gateway
```

### Duplicate Payments Processing

```bash
# Check for this pattern in logs
kubectl logs -n payment-gateway deployment/payment-gateway | \
  grep "DUPLICATE\|Idempotency"

# Check database for duplicate transactions
SELECT payment_id, COUNT(*) FROM payment_transactions 
GROUP BY payment_id HAVING COUNT(*) > 1;

# If found: Contact developers (idempotency logic issue)
```

### RabbitMQ Message Backlog

```bash
# Check queue depth
kubectl exec -it svc/rabbitmq -- rabbitmqctl list_queues name messages

# If backlog growing:
# 1. Check if payment-gateway pods are healthy
# 2. Check if consumers are processing (check logs)
# 3. Increase replicas temporarily: kubectl scale deployment payment-gateway --replicas=5

# Check RabbitMQ connections
kubectl exec -it svc/rabbitmq -- rabbitmqctl list_connections
```

---

## 📋 Pre-Deployment Checklist

**Before deploying to Production:**

- [ ] All 4 milestones completed and PRs merged
- [ ] Code coverage >80%
- [ ] No HIGH/CRITICAL vulnerabilities in Trivy scan
- [ ] All unit tests pass
- [ ] Dockerfile builds successfully
- [ ] Docker image scanned, zero HIGH/CRITICAL issues
- [ ] K8s manifests validated: `kubectl apply --dry-run=client -f kubernetes/`
- [ ] Secrets properly configured (not in git, using secret manager)
- [ ] Database migrations tested
- [ ] RabbitMQ connection tested
- [ ] Stripe/PayPal sandbox credentials configured
- [ ] Monitoring/logging configured and tested
- [ ] Health checks (liveness/readiness) tested
- [ ] Pod disruption budget defined
- [ ] Resource requests/limits set appropriately
- [ ] Network policies (if using) allow traffic
- [ ] RBAC roles/bindings correct
- [ ] Ingress TLS certificate valid
- [ ] Disaster recovery plan documented
- [ ] Rollback procedure tested
- [ ] On-call runbook prepared
- [ ] Team trained on deployment/troubleshooting

---

## 🚀 Deployment Commands (Quick Reference)

```bash
# Verify cluster connectivity
kubectl cluster-info
kubectl get nodes

# Create namespace
kubectl create namespace payment-gateway

# Apply all K8s manifests
kubectl apply -f kubernetes/ -n payment-gateway

# Check deployment status
kubectl rollout status deployment/payment-gateway -n payment-gateway

# View logs (last 100 lines)
kubectl logs deployment/payment-gateway -n payment-gateway --tail=100

# Stream logs (follow)
kubectl logs deployment/payment-gateway -n payment-gateway -f

# Port forward to test locally
kubectl port-forward svc/payment-gateway-service 8080:8080 -n payment-gateway
# Then: curl http://localhost:8080/actuator/health

# Scale up/down
kubectl scale deployment payment-gateway --replicas=5 -n payment-gateway

# Rolling restart (no downtime)
kubectl rollout restart deployment/payment-gateway -n payment-gateway

# View events (helpful for debugging)
kubectl get events -n payment-gateway --sort-by='.lastTimestamp'

# Update image (for hotfix)
kubectl set image deployment/payment-gateway \
  payment-gateway=registry/payment-gateway:v1.0.1 \
  -n payment-gateway

# Rollback to previous version
kubectl rollout undo deployment/payment-gateway -n payment-gateway

# Delete everything (cleanup)
kubectl delete namespace payment-gateway
```

---

## 📚 Additional Resources

- **Kubernetes Docs:** https://kubernetes.io/docs/
- **Spring Boot:** https://spring.io/projects/spring-boot
- **OpenAPI Specification:** https://spec.openapis.org/
- **Twelve-Factor App:** https://12factor.net/ (principles for cloud-native apps)
- **Google Cloud Best Practices:** https://cloud.google.com/kubernetes-engine/docs/best-practices
- **CNCF Cloud Native:** https://www.cncf.io/
- **Labs64 Payment Gateway:** https://github.com/Labs64/labs64.io-payment-gateway

---

## ❓ Questions & Support

When you encounter issues:

1. **Check application logs first:** `kubectl logs deployment/payment-gateway`
2. **Check Kubernetes events:** `kubectl get events`
3. **Check resource usage:** `kubectl top pods`
4. **Create GitHub Issue:** Tag with `devops` or `sre`
5. **Reference this guide section:** Point others to relevant checklist

---

**Remember:** As DevOps/SRE, you're responsible for:
- Making sure the application runs reliably
- Zero-downtime deployments
- Fast incident response
- Observability (logs, metrics, traces)
- Security posture
- Cost optimization

Good luck with the Payment Gateway project! 🚀
