package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/go-kratos/kratos-layout-monolith/internal/middleware/requestid"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
)

type JSONLogger struct {
	writer io.Writer
	mu     sync.Mutex
}

func NewLogger() log.Logger {
	return NewJSONLogger(os.Stdout)
}

func NewJSONLogger(writer io.Writer) log.Logger {
	if writer == nil {
		writer = io.Discard
	}
	return &JSONLogger{writer: writer}
}

func NewHelper(id, name, version string) *log.Helper {
	return log.NewHelper(WithService(NewLogger(), id, name, version))
}

func WithService(logger log.Logger, id, name, version string) log.Logger {
	return log.With(
		logger,
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.id", id,
		"service.name", name,
		"service.version", version,
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
		"request_id", RequestID(),
	)
}

func RequestID() log.Valuer {
	return func(ctx context.Context) any {
		return requestid.FromContext(ctx)
	}
}

func (l *JSONLogger) Log(level log.Level, keyvals ...any) error {
	if len(keyvals) == 0 {
		return nil
	}
	if len(keyvals)%2 != 0 {
		keyvals = append(keyvals, "KEYVALS UNPAIRED")
	}

	fields := make(map[string]any, len(keyvals)/2+1)
	fields[log.LevelKey] = level.String()
	for i := 0; i < len(keyvals); i += 2 {
		key := fmt.Sprint(keyvals[i])
		value := keyvals[i+1]
		if value == "" || value == nil {
			continue
		}
		fields[key] = value
	}

	line, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	line = append(line, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()
	_, err = l.writer.Write(line)
	return err
}
