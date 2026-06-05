# Payment Gateway Integration with OpenAPI & Kubernetes
## A DevOps/SRE Quick Start Guide

A production-ready payment gateway service built with Go, featuring OpenAPI specification, Docker containerization, and Kubernetes deployment manifests.

Based on guides from https://github.com/Labs64/labs64.io-payment-gateway

## Table of Contents

1. [Overview for DevOps/SRE](#1-overview-for-devopssre)
2. [Architecture High-Level](#2-architecture-high-level)
3. [What Developers Built](#3-what-developers-built)
4. [Your DevOps/SRE Responsibilities](#4-your-devopssre-responsibilities)
5. [Kubernetes-Safe Requirements](#5-kubernetes-safe-requirements)
6. [Docker & Containerization](#6-docker--containerization)
7. [Quick Start](#7-quick-start)
8. [Deployment Guide](#8-deployment-guide)
9. [Monitoring & Operations](#9-monitoring--operations)

---

## 1. Overview for DevOps/SRE

This payment gateway service is a microservice designed for Kubernetes environments. It provides RESTful APIs for payment processing with the following characteristics:

### Key Features
- **Stateless Design**: No local state, fully horizontally scalable
- **Health Checks**: Built-in liveness and readiness probes
- **Cloud-Native**: 12-factor app compliant
- **Security-First**: Non-root containers, read-only filesystem
- **Observable**: Structured logging, metrics-ready endpoints

### Technology Stack
- **Runtime**: Go 1.24
- **API Specification**: OpenAPI 3.0
- **Container**: Docker multi-stage builds
- **Orchestration**: Kubernetes 1.25+
- **Ingress**: NGINX Ingress Controller

---

## 2. Architecture High-Level

```
┌─────────────────────────────────────────────────────────────┐
│                    Internet / Load Balancer                 │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│              Kubernetes Ingress (NGINX)                      │
│          (TLS Termination, Rate Limiting)                    │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                Payment Gateway Service                       │
│                   (ClusterIP Service)                        │
└────────────┬──────────────────────────┬─────────────────────┘
             │                          │
             ▼                          ▼
    ┌────────────────┐        ┌────────────────┐
    │  Pod 1 (8080)  │        │  Pod 2 (8080)  │
    │ ┌────────────┐ │        │ ┌────────────┐ │
    │ │  Gateway   │ │  ...   │ │  Gateway   │ │
    │ │  Container │ │        │ │  Container │ │
    │ └────────────┘ │        │ └────────────┘ │
    └────────────────┘        └────────────────┘
             │                          │
             └──────────┬───────────────┘
                        ▼
        ┌───────────────────────────┐
        │  External Payment APIs    │
        │  (Stripe, PayPal, etc.)   │
        └───────────────────────────┘
```

### Component Responsibilities
- **Ingress**: SSL/TLS termination, routing, rate limiting
- **Service**: Internal load balancing across pods
- **Deployment**: Pod lifecycle management, rolling updates
- **HPA**: Automatic scaling based on CPU/memory metrics
- **ConfigMap**: Environment-specific configuration

---

## 3. What Developers Built

The development team has created a Go-based payment gateway with the following components:

### Application Structure
```
project-payment-gateway/
├── cmd/api/                              # Application entry point
├── internal/
│   ├── domain/payment/                   # Business logic and entities
│   ├── application/payment/              # Use-case orchestration
│   ├── transport/http/                   # HTTP transport layer
│   │   ├── handlers/                     # Request handlers
│   │   ├── dto/                          # Data Transfer Objects
│   │   ├── response/                     # Helper package for responses
│   │   └── middleware/                   # HTTP middlewares
│   └── platform/                         # Infrastructure helpers
```
├── k8s/                 # Kubernetes manifests
├── openapi.yaml         # OpenAPI 3.0 specification
├── Dockerfile           # Multi-stage container build
└── go.mod               # Go module dependencies
```

### API Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/health` | GET | Liveness probe |
| `/ready` | GET | Readiness probe |
| `/api/v1/payments` | POST | Create payment |
| `/api/v1/payments/{payment_id}` | GET | Retrieve payment status and details |
| `/api/v1/webhooks/payment` | POST | Receive payment webhooks |

### OpenAPI Documentation
The complete API specification is available in `openapi.yaml`. You can view it using:
- [Swagger Editor](https://editor.swagger.io/)
- [Redoc](https://github.com/Redocly/redoc)
- Local tools: `swagger-ui` or `redoc-cli`

---

## 4. Your DevOps/SRE Responsibilities

As the DevOps/SRE engineer, your responsibilities include:

### 1. Infrastructure Setup
- [ ] Provision Kubernetes cluster (EKS, GKE, AKS, or on-prem)
- [ ] Configure NGINX Ingress Controller
- [ ] Set up cert-manager for TLS certificates
- [ ] Configure monitoring stack (Prometheus/Grafana)

### 2. CI/CD Pipeline
- [ ] Set up container registry (ECR, GCR, Docker Hub)
- [ ] Configure CI pipeline for building Docker images
- [ ] Implement automated testing in pipeline
- [ ] Set up CD for Kubernetes deployments
- [ ] Configure staging and production environments

### 3. Security & Compliance
- [ ] Enable Pod Security Standards
- [ ] Configure Network Policies
- [ ] Set up secret management (Vault, Sealed Secrets)
- [ ] Implement RBAC policies
- [ ] Enable audit logging

### 4. Monitoring & Observability
- [ ] Configure Prometheus scraping
- [ ] Set up alerting rules
- [ ] Create Grafana dashboards
- [ ] Configure log aggregation (ELK/Loki)
- [ ] Set up distributed tracing (Jaeger/Tempo)

### 5. Operational Tasks
- [ ] Define SLI/SLO/SLA metrics
- [ ] Create runbooks for common issues
- [ ] Set up backup and disaster recovery
- [ ] Implement cost optimization strategies
- [ ] Plan capacity and scaling strategies

---

## 5. Kubernetes-Safe Requirements

This service follows Kubernetes best practices:

### ✅ Stateless Design
- No local storage dependencies
- All state external (databases, caches)
- Safe for horizontal scaling

### ✅ Health Checks
- **Liveness**: `/health` - restarts unhealthy pods
- **Readiness**: `/ready` - removes pods from load balancing if not ready
- Configured with appropriate timeouts and thresholds

### ✅ Resource Management
```yaml
resources:
  requests:
    memory: "64Mi"
    cpu: "100m"
  limits:
    memory: "128Mi"
    cpu: "200m"
```

### ✅ Security Context
- Runs as non-root user (UID 1000)
- Read-only root filesystem
- Drops all capabilities
- No privilege escalation

### ✅ Rolling Updates
```yaml
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxSurge: 1
    maxUnavailable: 0
```
Ensures zero-downtime deployments.

### ✅ Auto-Scaling
- Horizontal Pod Autoscaler (HPA) configured
- Scales from 3 to 10 replicas
- Based on CPU (70%) and memory (80%) utilization

### ✅ Configuration Management
- Environment variables via ConfigMap
- Secrets should use Kubernetes Secrets
- No hardcoded configuration in images

---

## 6. Docker & Containerization

### Multi-Stage Build
The Dockerfile uses multi-stage builds for optimization:

1. **Build Stage**: 
   - Uses `golang:1.24-alpine` for compilation
   - Installs dependencies
   - Produces statically-linked binary

2. **Runtime Stage**:
   - Uses minimal `alpine:3.19` base image
   - Copies only the binary and necessary files
   - ~20MB final image size

### Security Features
- Non-root user (UID 1000)
- CA certificates included for HTTPS
- Health check built into image
- Minimal attack surface

### Building the Image
```bash
# Build the Docker image
docker build -t payment-gateway:latest .

# Build with version tag
docker build -t payment-gateway:v1.0.0 .

# Test the image locally
docker run -p 8080:8080 payment-gateway:latest
```

### Image Registry
```bash
# Tag for registry
docker tag payment-gateway:latest <registry>/payment-gateway:latest

# Push to registry
docker push <registry>/payment-gateway:latest
```

---

## 7. Quick Start

### Local Development

#### Prerequisites
- Go 1.24 or later
- Docker (for containerization)
- kubectl (for Kubernetes)
- A Kubernetes cluster (minikube, kind, or cloud provider)

#### Run Locally
```bash
# Clone the repository
git clone <repository-url>
cd project-payment-gateway

# Install dependencies
go mod download

# Run the application
go run cmd/api/main.go

# Test the API
curl http://localhost:8080/health
```

#### Test API Endpoints
```bash
# Health check
curl http://localhost:8080/health

# Create a payment
curl -X POST http://localhost:8080/api/v1/payments \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 99.99,
    "currency": "USD",
    "description": "Test payment",
    "customer_id": "cust_12345"
  }'

# Get payment status
curl "http://localhost:8080/api/v1/payments/status?payment_id=pay_20240115100000"
```

---

## 8. Deployment Guide

### Step 1: Build and Push Docker Image
```bash
# Build the image
docker build -t <registry>/payment-gateway:v1.0.0 .

# Push to registry
docker push <registry>/payment-gateway:v1.0.0
```

### Step 2: Update Kubernetes Manifests
```bash
# Update the image in k8s/deployment.yaml
# Change: image: payment-gateway:latest
# To: image: <registry>/payment-gateway:v1.0.0
```

### Step 3: Apply Kubernetes Resources
```bash
# Create namespace (optional)
kubectl create namespace payment-gateway

# Apply ConfigMap
kubectl apply -f k8s/configmap.yaml

# Apply Deployment
kubectl apply -f k8s/deployment.yaml

# Apply Service
kubectl apply -f k8s/service.yaml

# Apply HPA (if metrics-server is installed)
kubectl apply -f k8s/hpa.yaml

# Apply Ingress (if ingress controller is installed)
kubectl apply -f k8s/ingress.yaml
```

### Step 4: Verify Deployment
```bash
# Check pod status
kubectl get pods -l app=payment-gateway

# Check service
kubectl get svc payment-gateway

# Check logs
kubectl logs -l app=payment-gateway --tail=50

# Port-forward for testing
kubectl port-forward svc/payment-gateway 8080:80

# Test the service
curl http://localhost:8080/health
```

### Step 5: Configure Ingress (Production)
```bash
# Install NGINX Ingress Controller
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.1/deploy/static/provider/cloud/deploy.yaml

# Install cert-manager for TLS
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Apply ingress
kubectl apply -f k8s/ingress.yaml
```

---

## 9. Monitoring & Operations

### Health Monitoring
```bash
# Check liveness
curl http://<service-url>/health

# Check readiness
curl http://<service-url>/ready
```

### Kubernetes Monitoring
```bash
# Watch pod status
kubectl get pods -l app=payment-gateway -w

# View resource usage
kubectl top pods -l app=payment-gateway

# Check HPA status
kubectl get hpa payment-gateway-hpa

# View events
kubectl get events --sort-by=.metadata.creationTimestamp
```

### Logs
```bash
# View logs
kubectl logs -l app=payment-gateway --tail=100 -f

# View logs from specific pod
kubectl logs <pod-name> -f

# View logs from previous container (after crash)
kubectl logs <pod-name> --previous
```

### Scaling
```bash
# Manual scaling
kubectl scale deployment payment-gateway --replicas=5

# Check HPA metrics
kubectl describe hpa payment-gateway-hpa
```

### Rolling Updates
```bash
# Update image version
kubectl set image deployment/payment-gateway payment-gateway=<registry>/payment-gateway:v1.0.1

# Check rollout status
kubectl rollout status deployment/payment-gateway

# View rollout history
kubectl rollout history deployment/payment-gateway

# Rollback if needed
kubectl rollout undo deployment/payment-gateway
```

### Troubleshooting

#### Pod Crash Loop
```bash
# Check pod status
kubectl describe pod <pod-name>

# View logs
kubectl logs <pod-name> --previous

# Check events
kubectl get events --field-selector involvedObject.name=<pod-name>
```

#### Service Not Accessible
```bash
# Check service endpoints
kubectl get endpoints payment-gateway

# Test service from within cluster
kubectl run -it --rm debug --image=alpine --restart=Never -- sh
# In the pod:
apk add curl
curl http://payment-gateway.default.svc.cluster.local/health
```

#### High Resource Usage
```bash
# Check resource usage
kubectl top pods -l app=payment-gateway

# Review HPA status
kubectl describe hpa payment-gateway-hpa

# Check metrics
kubectl get --raw /apis/metrics.k8s.io/v1beta1/namespaces/default/pods
```

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

For issues, questions, or contributions, please open an issue on the GitHub repository.
