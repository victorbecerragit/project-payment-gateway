.PHONY: help build build-consumer run test clean docker-build docker-run docker-build-consumer k8s-deploy k8s-delete demo-up demo-down monitor-up monitor-down monitor-port-forward-grafana monitor-port-forward-prometheus

# Variables
APP_NAME := payment-gateway
DOCKER_IMAGE := $(APP_NAME):latest
REGISTRY ?= 
VERSION ?= latest

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the Go application
	@echo "Building $(APP_NAME)..."
	go build -o bin/$(APP_NAME) ./cmd/api

build-consumer: ## Build the event consumer binary
	@echo "Building payment-event-consumer..."
	go build -o bin/payment-event-consumer ./cmd/payment-event-consumer

run: ## Run the application locally
	@echo "Running $(APP_NAME)..."
	go run ./cmd/api/main.go

test: ## Run all tests with race detector and coverage
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

test-unit: ## Run unit tests only (no race, fast feedback)
	@echo "Running unit tests..."
	go test ./...

test-handlers: ## Run transport/http/handlers tests only
	@echo "Running handlers tests..."
	go test -v -run . ./internal/transport/http/handlers/...

test-watch: ## Re-run tests on file changes (requires entr: brew install entr)
	@echo "Watching for changes..."
	find . -name '*.go' | entr -c go test ./...

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE) .
	@if [ ! -z "$(REGISTRY)" ]; then \
		docker tag $(DOCKER_IMAGE) $(REGISTRY)/$(DOCKER_IMAGE); \
	fi

docker-build-consumer: ## Build consumer Docker image
	@echo "Building consumer Docker image..."
	docker build -t payment-event-consumer:latest -f cmd/payment-event-consumer/Dockerfile .
	@if [ ! -z "$(REGISTRY)" ]; then \
		docker tag payment-event-consumer:latest $(REGISTRY)/payment-event-consumer:latest; \
	fi

docker-run: ## Run Docker container locally
	@echo "Running Docker container..."
	docker run -p 8080:8080 --rm $(DOCKER_IMAGE)

docker-push: ## Push Docker image to registry
	@if [ -z "$(REGISTRY)" ]; then \
		echo "Error: REGISTRY not set. Use: make docker-push REGISTRY=your-registry"; \
		exit 1; \
	fi
	@echo "Pushing to $(REGISTRY)..."
	docker push $(REGISTRY)/$(DOCKER_IMAGE)

demo-up: ## Deploy full demo stack to kind (ESO secrets first, then app). See docs/DEMO.md.
	@echo "[demo-up] Step 1/3 — applying ESO secrets..."
	kubectl apply -k k8s/kustomize/eso/
	@echo "[demo-up] Step 2/3 — waiting for ExternalSecret to sync..."
	kubectl wait --for=condition=Ready externalsecret/payment-gateway-secrets --timeout=30s
	@echo "[demo-up] Step 3/3 — deploying full stack (postgres + gateway + frontend + stripe-listener)..."
	kubectl apply -k k8s/kustomize/base/
	kubectl wait --for=condition=ready pod -l app=payment-gateway --timeout=90s
	@echo ""
	@echo "✅ Demo stack is up. Open http://payment-gateway in your browser."
	@echo "   Run 'make demo-status' to check all pods."

demo-down: ## Tear down the full demo stack (app + ESO secrets)
	@echo "[demo-down] Deleting app stack..."
	kubectl delete job stripe-trigger-demo --ignore-not-found
	kubectl delete -k k8s/kustomize/base/ --ignore-not-found
	@echo "[demo-down] Deleting ESO secrets..."
	kubectl delete -k k8s/kustomize/eso/ --ignore-not-found
	@echo ""
	@echo "✅ Demo stack removed."

demo-status: ## Show status of all demo pods, services, and HPA
	@echo "=== Pods ==="
	kubectl get pods
	@echo ""
	@echo "=== Services ==="
	kubectl get svc
	@echo ""
	@echo "=== HPA ==="
	kubectl get hpa 2>/dev/null || echo "No HPA found"
	@echo ""
	@echo "=== ExternalSecret ==="
	kubectl get externalsecret payment-gateway-secrets 2>/dev/null || echo "Not found"

k8s-deploy: ## Deploy to Kubernetes (legacy — prefer make demo-up)
	@echo "Deploying to Kubernetes..."
	kubectl apply -f k8s/configmap.yaml
	kubectl apply -f k8s/deployment.yaml
	kubectl apply -f k8s/service.yaml
	kubectl apply -f k8s/hpa.yaml
	@echo "Deployment complete!"
	@echo "Waiting for pods to be ready..."
	kubectl wait --for=condition=ready pod -l app=payment-gateway --timeout=60s

k8s-delete: ## Delete Kubernetes resources (legacy — prefer make demo-down)
	@echo "Deleting Kubernetes resources..."
	kubectl delete -f k8s/ --ignore-not-found=true

k8s-logs: ## View Kubernetes logs
	kubectl logs -l app=payment-gateway --tail=100 -f

k8s-status: ## Check Kubernetes deployment status
	@echo "Pods:"
	kubectl get pods -l app=payment-gateway
	@echo ""
	@echo "Service:"
	kubectl get svc payment-gateway
	@echo ""
	@echo "HPA:"
	kubectl get hpa payment-gateway-hpa 2>/dev/null || echo "HPA not found"

lint: ## Run linter
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed" && exit 1)
	golangci-lint run ./...

fmt: ## Format code
	go fmt ./...
	go vet ./...

deps: ## Download dependencies
	go mod download
	go mod tidy

compose-up: ## Start with docker-compose
	docker-compose up -d

compose-down: ## Stop docker-compose
	docker-compose down

compose-logs: ## View docker-compose logs
	docker-compose logs -f

kafka-up: ## Deploy Strimzi operator + Kafka cluster + topic
	kubectl apply -k k8s/kafka/

kafka-down: ## Remove Kafka resources
	kubectl delete -k k8s/kafka/ --ignore-not-found

kafka-list-topics: ## List Kafka topics on the cluster
	kubectl exec -it payment-kafka-payment-kafka-brokers-0 -n payment-system -- \
	  bin/kafka-topics.sh --bootstrap-server localhost:9092 --list

kafka-consume-raw: ## Read all events from payment-events topic (debug)
	kubectl exec -it payment-kafka-payment-kafka-brokers-0 -n payment-system -- \
	  bin/kafka-console-consumer.sh \
	  --bootstrap-server localhost:9092 \
	  --topic payment-events \
	  --from-beginning

consumer-logs: ## Tail payment-event-consumer logs
	kubectl logs -l app=payment-event-consumer -n payment-system -f

consumer-lag: ## Show consumer group lag for payment-audit-consumer
	kubectl exec -it payment-kafka-payment-kafka-brokers-0 -n payment-system -- \
	  bin/kafka-consumer-groups.sh \
	  --bootstrap-server localhost:9092 \
	  --describe \
	  --group payment-audit-consumer

kafka-dlq-watch: ## Monitor dead letter queue in real time
	kubectl exec -it payment-kafka-payment-kafka-brokers-0 -n payment-system -- \
	  bin/kafka-console-consumer.sh \
	  --bootstrap-server localhost:9092 \
	  --topic payment-events-dlq \
	  --from-beginning

kafka-dlq-count: ## Show partition details for DLQ topic
	kubectl exec -it payment-kafka-payment-kafka-brokers-0 -n payment-system -- \
	  bin/kafka-topics.sh \
	  --bootstrap-server localhost:9092 \
	  --describe \
	  --topic payment-events-dlq

kafka-dlq-replay: ## Replay DLQ original_event fields back into payment-events
	@echo "Extract original events and replay — run manually after confirming root cause fixed:"
	@echo "kubectl exec -it payment-kafka-payment-kafka-brokers-0 -n payment-system -- \\"
	@echo "  bin/kafka-console-consumer.sh --bootstrap-server localhost:9092 \\"
	@echo "  --topic payment-events-dlq --from-beginning \\"
	@echo "  | jq -c '.original_event' \\"
	@echo "  | kubectl exec -i payment-kafka-payment-kafka-brokers-0 -n payment-system -- \\"
	@echo "  bin/kafka-console-producer.sh --bootstrap-server localhost:9092 \\"
	@echo "  --topic payment-events"

demo-kafka: ## Full Kafka demo: deploy stack, trigger payment, show events
	@echo "==> Starting Kafka stack..."
	kubectl apply -k k8s/kafka/
	@echo "==> Waiting for Kafka to be ready..."
	kubectl wait kafka/payment-kafka --for=condition=Ready --timeout=300s -n payment-system
	@echo "==> Kafka ready. Deploy consumer and trigger a payment:"
	@echo "    docker build -t payment-event-consumer:latest -f cmd/payment-event-consumer/Dockerfile ."
	@echo "    kind load docker-image payment-event-consumer:latest --name payment-demo"
	@echo "    kubectl apply -k k8s/event-consumer/"
	@echo "    make stripe-trigger"
	@echo "    make kafka-consume-raw"

# ──────────────────────────────────────────────
# Monitoring (Puzzle 6)
# ──────────────────────────────────────────────

MONITORING_NAMESPACE := monitoring

.PHONY: install-prometheus-stack install-grafana-operator

install-prometheus-stack: ## Install kube-prometheus-stack (Prometheus + Alertmanager + CRDs)
	helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
	helm repo update
	helm upgrade --install prometheus-stack prometheus-community/kube-prometheus-stack \
	  --namespace $(MONITORING_NAMESPACE) \
	  --create-namespace \
	  --set grafana.enabled=false \
	  --wait

install-grafana-operator: ## Install Grafana Operator (CRD-based Grafana management)
	helm repo add grafana https://grafana.github.io/helm-charts
	helm repo update
	helm upgrade --install grafana-operator grafana/grafana-operator \
	  --namespace $(MONITORING_NAMESPACE) \
	  --create-namespace \
	  --wait

monitor-up: install-prometheus-stack install-grafana-operator ## Deploy full monitoring stack
	@echo "==> Waiting for Grafana Operator CRDs to be established..."
	kubectl wait --for=condition=Established crd/grafanas.grafana.integreatly.org --timeout=60s 2>/dev/null || true
	sleep 5
	kubectl apply -k k8s/monitoring/
	@echo "==> Waiting for Grafana instance to be ready..."
	kubectl wait --for=condition=Ready grafana/grafana -n $(MONITORING_NAMESPACE) --timeout=120s 2>/dev/null || true
	@echo ""
	@echo "==> Monitoring stack deployed."
	@echo "    Prometheus: make monitor-port-forward-prometheus"
	@echo "    Grafana:    make monitor-port-forward-grafana"
	@echo "    Grafana admin password: admin"

monitor-down: ## Remove monitoring stack (Helm releases + k8s manifests)
	-helm uninstall grafana-operator --namespace $(MONITORING_NAMESPACE) 2>/dev/null
	-helm uninstall prometheus-stack --namespace $(MONITORING_NAMESPACE) 2>/dev/null
	kubectl delete -k k8s/monitoring/ --ignore-not-found 2>/dev/null

monitor-port-forward-prometheus: ## Port-forward Prometheus to localhost:9090
	@echo "Prometheus UI: http://localhost:9090"
	kubectl port-forward svc/prometheus-stack-kube-prom-prometheus 9090:9090 -n $(MONITORING_NAMESPACE)

monitor-port-forward-grafana: ## Port-forward Grafana to localhost:3000
	@echo "Grafana UI: http://localhost:3000 (admin:admin)"
	kubectl port-forward svc/grafana-service 3000:3000 -n $(MONITORING_NAMESPACE)
