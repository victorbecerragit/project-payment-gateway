package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	EventsPublished = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payment_events_published_total",
			Help: "Total number of payment events published to Kafka.",
		},
		[]string{"event_type", "status"},
	)

	DLQTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payment_dlq_total",
			Help: "Total number of events routed to the dead letter queue.",
		},
		[]string{"event_type", "reason"},
	)

	ProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "payment_processing_duration_seconds",
			Help:    "Duration of payment event processing in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"event_type"},
	)

	ConsumerRetries = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payment_consumer_retry_total",
			Help: "Total number of consumer retries for payment events.",
		},
		[]string{"event_type"},
	)
)
