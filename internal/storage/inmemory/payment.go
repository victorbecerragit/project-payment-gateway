package inmemory

import (
	"context"
	"sync"

	"github.com/victorbecerragit/project-payment-gateway/internal/domain/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/platform/slogext"
	"github.com/victorbecerragit/project-payment-gateway/internal/platform/tracing"
)

type repository struct {
	mu       sync.RWMutex
	payments map[string]*payment.Payment
	tracer   tracing.Tracer
}

// NewRepository creates a new in-memory payment repository
func NewRepository(tracer tracing.Tracer) payment.Repository {
	if tracer == nil {
		tracer = tracing.NewNoOpTracer()
	}
	return &repository{
		payments: make(map[string]*payment.Payment),
		tracer:   tracer,
	}
}

func (r *repository) Save(ctx context.Context, p *payment.Payment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.payments[p.ID] = p
	return nil
}


func (r *repository) GetByID(ctx context.Context, id string) (*payment.Payment, error) {
	_, span := r.tracer.StartSpan(ctx, "inmemory.GetByID")
	defer span.End()
	span.SetAttribute("payment.id", id)

	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.payments[id]
	if !ok {
		return nil, payment.ErrPaymentNotFound
	}
	return p, nil
}

func (r *repository) GetByIdempotencyKey(ctx context.Context, key string) (*payment.Payment, error) {
	_, span := r.tracer.StartSpan(ctx, "inmemory.GetByIdempotencyKey")
	defer span.End()
	span.SetAttribute("idempotency.key", key)

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, p := range r.payments {
		if p.IdempotencyKey == key {
			span.SetAttribute("payment.id", p.ID)
			return p, nil
		}
	}
	return nil, nil
}

func (r *repository) ListPayments(ctx context.Context, f payment.ListFilter) ([]*payment.Payment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}

	result := make([]*payment.Payment, 0, len(r.payments))
	for _, p := range r.payments {
		if f.Status != "" && string(p.Status) != f.Status {
			continue
		}
		cp := *p
		result = append(result, &cp)
	}

	// Sort descending by CreatedAt
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].CreatedAt.After(result[i].CreatedAt) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (r *repository) GetByProviderRef(ctx context.Context, providerRef string) (*payment.Payment, error) {
	_, span := r.tracer.StartSpan(ctx, "inmemory.GetByProviderRef")
	defer span.End()
	span.SetAttribute("provider.ref", providerRef)

	r.mu.RLock()
	defer r.mu.RUnlock()

	span.SetAttribute("provider.ref", providerRef)
	defer span.End()

	for _, p := range r.payments {
		if p.TransactionID == providerRef {
			span.SetAttribute("payment.id", p.ID)
			return p, nil
		}
	}
	return nil, payment.ErrPaymentNotFound
}

func (r *repository) Close() {
	// No-op for in-memory repository
	slogext.Ctx(context.Background()).Info("in-memory repository closed (no-op)")
}
