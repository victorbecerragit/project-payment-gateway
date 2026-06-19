.PHONY: help build run test clean docker-build docker-run k8s-deploy k8s-delete demo-up demo-down

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
