package requestid

import (
	"context"

	"github.com/go-kratos/kratos/v2/middleware"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/google/uuid"
)

type contextKey string

const RequestIDKey contextKey = "request_id"

// FromContext extracts the request ID from the context.
func FromContext(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

// Server returns a middleware that adds a unique request ID.
func Server() middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			r, ok := khttp.RequestFromServerContext(ctx)
			if !ok {
				return next(ctx, req)
			}

			reqID := r.Header.Get("X-Request-ID")
			if reqID == "" {
				reqID = uuid.New().String()
			}

			if w, ok := khttp.ResponseWriterFromServerContext(ctx); ok {
				w.Header().Set("X-Request-ID", reqID)
			}

			ctx = context.WithValue(ctx, RequestIDKey, reqID)
			return next(ctx, req)
		}
	}
}
