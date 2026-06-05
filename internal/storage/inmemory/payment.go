package inmemory

import (
	"context"
	"fmt"
	"sync"

	"github.com/victorbecerragit/project-payment-gateway/internal/domain/payment"
)

type repository struct {
	mu       sync.RWMutex
	payments map[string]*payment.Payment
}

// NewRepository creates a new in-memory payment repository
func NewRepository() payment.Repository {
	return &repository{
		payments: make(map[string]*payment.Payment),
	}
}

func (r *repository) Save(ctx context.Context, p *payment.Payment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.payments[p.ID] = p
	return nil
}

func (r *repository) GetByID(ctx context.Context, id string) (*payment.Payment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.payments[id]
	if !ok {
		return nil, fmt.Errorf("payment not found")
	}
	return p, nil
}

func (r *repository) GetByIdempotencyKey(ctx context.Context, key string) (*payment.Payment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.payments {
		if p.IdempotencyKey == key {
			return p, nil
		}
	}
	return nil, nil
}
