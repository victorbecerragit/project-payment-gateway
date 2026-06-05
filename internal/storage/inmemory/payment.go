package inmemory
package inmemory

import (
	"context"
	"fmt"
	"sync"

	"github.com/victorbecerragit/project-payment-gateway/internal/domain/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/models"
)

type repository struct {
	mu       sync.RWMutex
	payments map[string]*models.Payment
}

// NewRepository creates a new in-memory payment repository
func NewRepository() payment.Repository {
	return &repository{
		payments: make(map[string]*models.Payment),
	}
}

func (r *repository) Save(ctx context.Context, p *models.Payment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.payments[p.ID] = p
	return nil
}

func (r *repository) GetByID(ctx context.Context, id string) (*models.Payment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.payments[id]
	if !ok {
		return nil, fmt.Errorf("payment not found")
	}
	return p, nil
}

func (r *repository) GetByIdempotencyKey(ctx context.Context, key string) (*models.Payment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.payments {
		if p.IdempotencyKey == key {
			return p, nil
		}
	}
	return nil, nil
}
