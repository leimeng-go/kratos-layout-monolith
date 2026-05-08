package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-kratos/kratos-layout-monolith/internal/conf"
	"github.com/go-kratos/kratos-layout-monolith/internal/middleware/auth"
	"github.com/go-kratos/kratos-layout-monolith/internal/middleware/cors"
	"github.com/go-kratos/kratos-layout-monolith/internal/middleware/ratelimit"
	"github.com/go-kratos/kratos-layout-monolith/internal/middleware/requestid"
	"github.com/go-kratos/kratos-layout-monolith/internal/middleware/validator"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/google/wire"
)

// ProviderSet is server providers.
var ProviderSet = wire.NewSet(NewHTTPServer)

// NewHTTPServer creates a new HTTP server with common middleware.
func NewHTTPServer(
	c *conf.Server,
	ac *conf.Auth,
	jwt *conf.Jwt,
	logger log.Logger,
) (*kratoshttp.Server, error) {
	opts := []kratoshttp.ServerOption{
		kratoshttp.Middleware(
			recovery.Recovery(),
			requestid.Server(),
			cors.Server(),
			ratelimit.Server(),
			auth.Server(ac, jwt.Secret),
			validator.Server(),
		),
	}
	if c.HTTP != nil {
		if c.HTTP.Network != "" {
			opts = append(opts, kratoshttp.Network(c.HTTP.Network))
		}
		if c.HTTP.Addr != "" {
			opts = append(opts, kratoshttp.Address(c.HTTP.Addr))
		}
		if c.HTTP.Timeout != 0 {
			opts = append(opts, kratoshttp.Timeout(c.HTTP.Timeout))
		}
	}

	srv := kratoshttp.NewServer(opts...)

	// Register health check endpoint
	srv.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	log.NewHelper(logger).Infof("HTTP server listening on %s", c.HTTP.Addr)

	return srv, nil
}
