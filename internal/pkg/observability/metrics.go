package observability

import (
	"context"
	"strconv"
	"time"

	"github.com/go-kratos/kratos/v2/middleware"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "app",
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "Total HTTP requests processed by the application.",
	}, []string{"method", "path", "code"})

	httpRequestDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "app",
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "HTTP request duration in seconds.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "path", "code"})
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDurationSeconds)
}

func HTTPMetrics() middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			start := time.Now()
			reply, err := next(ctx, req)

			method, path, code := httpRequestLabels(ctx, err)
			duration := time.Since(start).Seconds()
			httpRequestsTotal.WithLabelValues(method, path, code).Inc()
			httpRequestDurationSeconds.WithLabelValues(method, path, code).Observe(duration)

			return reply, err
		}
	}
}

func httpRequestLabels(ctx context.Context, err error) (string, string, string) {
	method := "unknown"
	path := "unknown"
	code := "200"

	if req, ok := khttp.RequestFromServerContext(ctx); ok {
		method = req.Method
		path = req.URL.Path
	}
	if err != nil {
		code = "error"
	} else if writer, ok := khttp.ResponseWriterFromServerContext(ctx); ok {
		if statusWriter, ok := writer.(interface{ StatusCode() int }); ok {
			code = strconv.Itoa(statusWriter.StatusCode())
		}
	}

	return method, path, code
}
