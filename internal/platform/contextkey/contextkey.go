package contextkey

type contextKey string

const (
	RequestIDKey contextKey = "requestID"
	LoggerKey    contextKey = "logger"
	TracerKey    contextKey = "tracer"
)
