# Observability — Prometheus & Grafana

## Architecture

```
┌─────────────────────────────────────┐
│          payment-gateway            │
│  :8080/metrics                      │
│  ┌────────────────────────────┐     │
│  │ payment_events_published   │     │
│  │ http_requests_total        │     │
│  │ http_request_duration_sec  │     │
│  └──────────┬─────────────────┘     │
└─────────────┼───────────────────────┘
              │
┌─────────────┼───────────────────────┐
│  payment-event-consumer             │
│  :9090/metrics                      │
│  ┌────────────────────────────┐     │
│  │ payment_processing_duration│     │
│  │ payment_consumer_retry     │     │
│  │ payment_dlq_total          │     │
│  └──────────┬─────────────────┘     │
└─────────────┼───────────────────────┘
              │
┌─────────────┼───────────────────────┐
│  payment-kafka (Strimzi)            │
│  JMX exporter → :8080/metrics       │
└─────────────┼───────────────────────┘
              │
    ┌─────────▼─────────┐
    │    Prometheus      │
    │  (kube-prometheus) │
    └─────────┬─────────┘
              │
    ┌─────────▼─────────┐
    │      Grafana       │
    │   dashboards +     │
    │   alerts           │
    └───────────────────┘
```

## Metrics

### Application Metrics

| Metric | Type | Labels | Description |
|---|---|---|---|
| `payment_events_published_total` | Counter | `event_type`, `status` | Events published to Kafka |
| `payment_dlq_total` | Counter | `event_type`, `reason` | Events routed to dead letter queue |
| `payment_processing_duration_seconds` | Histogram | `event_type` | Handler execution time in consumer |
| `payment_consumer_retry_total` | Counter | `event_type` | Transient error retries in consumer |

### HTTP Metrics (built-in middleware)

| Metric | Type | Description |
|---|---|---|
| `http_requests_total` | Counter | Per-path/method/status request count |
| `http_request_duration_seconds` | Histogram | Request latency distribution |
| `http_requests_dropped_total` | Counter | Requests dropped by rate limiter |
| `rate_limiter_unique_ips_total` | Gauge | Unique IPs tracked by rate limiter |

### Kafka Broker Metrics (Strimzi JMX Exporter)

| Metric | Description |
|---|---|
| `kafka_server_*` | Broker-level metrics (request rate, partition count) |
| `kafka_network_*` | Network processor metrics |
| `kafka_log_*` | Log segment metrics |
| `kafka_controller_*` | Controller metrics (leader election, etc.) |

## Deploy

```bash
# Full stack install (Prometheus + Grafana Operator + ServiceMonitors)
make monitor-up

# Individual components
make install-prometheus-stack
make install-grafana-operator

# Port-forward
make monitor-port-forward-prometheus  # http://localhost:9090
make monitor-port-forward-grafana     # http://localhost:3000 (admin:admin)

# Teardown
make monitor-down
```

### How it works (Grafana Operator)

1. `install-grafana-operator` installs the operator via Helm (`grafana/grafana-operator`)
2. `kubectl apply -k k8s/monitoring/` creates the Grafana CR (instance), GrafanaDatasource CR (Prometheus source), and GrafanaDashboard CR (payment-ops dashboard)
3. The operator reconciles these CRDs into a running Grafana instance with pre-configured datasource and dashboard

No manual provisioning ConfigMaps needed — everything is declarative CRDs.

## Prometheus Queries

### Event publication rate
```
rate(payment_events_published_total[5m])
```

### Event publication errors
```
payment_events_published_total{status="error"}
```

### DLQ rate
```
rate(payment_dlq_total[5m])
```

### Consumer processing duration (p99)
```
histogram_quantile(0.99, rate(payment_processing_duration_seconds_bucket[5m]))
```

### Retry rate
```
rate(payment_consumer_retry_total[5m])
```

## Grafana Dashboards

The **Payment Operations** dashboard is pre-loaded as a `GrafanaDashboard` CRD by `k8s/monitoring/grafana-dashboard-payment.yaml`. The Grafana Operator reconciles it into the Grafana instance automatically.

### Dashboard Panels

1. **Event Publication Rate** — `rate(payment_events_published_total[5m])`
2. **Publish Errors** — `payment_events_published_total{status="error"}`
3. **Event Processing Duration (p99)** — histogram_quantile
4. **Consumer Retries** — `rate(payment_consumer_retry_total[5m])`
5. **DLQ Volume** — `rate(payment_dlq_total[5m])`

## Testing with Load Generator

The `multi-state` scenario creates payments across all four states (pending, completed, failed, cancelled) without any external dependencies:

```bash
# Default: 8 payments (2 per state)
REQUESTS=8 ./loadgen.sh multi-state

# With port-forward active, run from a second terminal:
make install-prometheus-stack install-grafana-operator deploy-monitoring
make monitor-port-forward-grafana    # Terminal 1: Grafana @ :3000
./demo/loadgenerator/loadgen.sh multi-state  # Terminal 2: load generator
```

This exercises every custom metric:
- `payment_events_published_total` — each payment creation + each webhook event
- `payment_processing_duration_seconds` — consumer processing each webhook event
- `payment_dlq_total` — if any events hit the dead letter queue
- `payment_consumer_retry_total` — if any transient errors occur

## Interview Talking Points

- **Three instrumentation points**: publisher (counter), consumer handler (histogram), retry loop (counter), DLQ (counter)
- **Decision to use `promauto`**: automatic registration avoids manual `prometheus.MustRegister` calls; simpler for application-level metrics
- **Separate metrics port for consumer** (`:9090`): decouples operational metrics from application audit processing; no risk of port conflict
- **JMX exporter built into Strimzi**: no sidecar needed; configured declaratively in `Kafka` CR `metricsConfig`
- **Grafana Operator over Helm chart**: CRD-based approach (`Grafana`, `GrafanaDatasource`, `GrafanaDashboard`) is more Kubernetes-native — no manual ConfigMap provisioning or sidecars for dashboard/datasource injection; the operator handles reconciliation
- **ServiceMonitor over annotations**: CRD-based discovery is the modern Prometheus operator pattern; declarative, version-controlled, supports namespace selectors
