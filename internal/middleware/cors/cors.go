package cors

import (
	"context"
	"net/http"

	"github.com/go-kratos/kratos/v2/middleware"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

// Server returns a CORS middleware for HTTP server.
func Server() middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			r, ok := khttp.RequestFromServerContext(ctx)
			if !ok {
				return next(ctx, req)
			}

			w, _ := khttp.ResponseWriterFromServerContext(ctx)

			origin := r.Header.Get("Origin")
			if origin != "" && w != nil {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}

			// Handle preflight
			if r.Method == http.MethodOptions {
				if w != nil {
					w.WriteHeader(http.StatusNoContent)
				}
				return nil, nil
			}

			return next(ctx, req)
		}
	}
}
