package tracing

import (
	"context"
	"log/slog"
	"time"

	"github.com/victorbecerragit/project-payment-gateway/internal/platform/contextkey"
)

type Span interface {
	End()
	SetAttribute(key string, value interface{})
}

type Tracer interface {
	StartSpan(ctx context.Context, name string) (context.Context, Span)
}

type noOpSpan struct{}

func (s *noOpSpan) End() {}
func (s *noOpSpan) SetAttribute(key string, value interface{}) {}

type noOpTracer struct{}

func NewNoOpTracer() Tracer {
	return &noOpTracer{}
}

func (t *noOpTracer) StartSpan(ctx context.Context, name string) (context.Context, Span) {
	return ctx, &noOpSpan{}
}

type loggerSpan struct {
	ctx        context.Context
	name       string
	start      time.Time
	logger     *slog.Logger
	attributes map[string]interface{}
}

func (s *loggerSpan) End() {
	s.logger.Info("span_end",
		"span_name", s.name,
		"duration_ms", time.Since(s.start).Milliseconds(),
		"attributes", s.attributes,
	)
}

func (s *loggerSpan) SetAttribute(key string, value interface{}) {
	s.attributes[key] = value
}

type loggerTracer struct {
	logger *slog.Logger
}

func NewLoggerTracer(logger *slog.Logger) Tracer {
	return &loggerTracer{logger: logger}
}

func (t *loggerTracer) StartSpan(ctx context.Context, name string) (context.Context, Span) {
	spanLogger := t.logger.With("span_name", name)
	spanLogger.Info("span_start", "span_name", name)
	span := &loggerSpan{
		ctx:        ctx,
		name:       name,
		start:      time.Now(),
		logger:     spanLogger,
		attributes: make(map[string]interface{}),
	}
	return context.WithValue(ctx, contextkey.TracerKey, t), span
}

func CtxTracer(ctx context.Context) Tracer {
	if t, ok := ctx.Value(contextkey.TracerKey).(Tracer); ok {
		return t
	}
	return NewNoOpTracer()
}
