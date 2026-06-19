package events

import "context"

// NoOpPublisher discards all events. Used in tests and local dev without Kafka.
type NoOpPublisher struct{}

func NewNoOpPublisher() *NoOpPublisher { return &NoOpPublisher{} }

func (n *NoOpPublisher) Publish(_ context.Context, _ PaymentEvent) error { return nil }
func (n *NoOpPublisher) Close() error                                     { return nil }
