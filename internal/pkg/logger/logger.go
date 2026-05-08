package logger

import (
	"os"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
)

// NewLogger creates a standard kratos logger.
func NewLogger() log.Logger {
	return log.NewStdLogger(os.Stdout)
}

// NewHelper creates a logger helper with standard fields.
func NewHelper(id, name, version string) *log.Helper {
	logger := NewLogger()
	return log.NewHelper(log.With(
		logger,
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.id", id,
		"service.name", name,
		"service.version", version,
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
	))
}
