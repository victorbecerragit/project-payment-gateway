package health

import "context"

// Service defines the health check operations
type Service interface {
	Check(ctx context.Context) bool
	Ready(ctx context.Context) bool
}
