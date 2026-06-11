package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-kratos/kratos-layout-monolith/internal/conf"
	"github.com/go-kratos/kratos-layout-monolith/internal/middleware/auth"
	"github.com/go-kratos/kratos-layout-monolith/internal/middleware/cors"
	"github.com/go-kratos/kratos-layout-monolith/internal/middleware/ratelimit"
	"github.com/go-kratos/kratos-layout-monolith/internal/middleware/requestid"
	"github.com/go-kratos/kratos-layout-monolith/internal/middleware/validator"
	"github.com/go-kratos/kratos-layout-monolith/internal/pkg/cache"
	"github.com/go-kratos/kratos-layout-monolith/internal/pkg/observability"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/google/wire"
	"gorm.io/gorm"
)

// ProviderSet is server providers.
var ProviderSet = wire.NewSet(NewHTTPServer)

// NewHTTPServer creates a new HTTP server with common middleware.
func NewHTTPServer(
	c *conf.Server,
	ac *conf.Auth,
	jwt *conf.Jwt,
	database *gorm.DB,
	rds *cache.Redis,
	logger log.Logger,
) (*kratoshttp.Server, error) {
	opts := []kratoshttp.ServerOption{
		kratoshttp.Middleware(
			recovery.Recovery(),
			observability.HTTPMetrics(),
			tracing.Server(),
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
	if rds != nil {
		observability.RegisterPoolMetrics(database, rds.Client)
	} else {
		observability.RegisterPoolMetrics(database, nil)
	}

	srv.Handle("/metrics", promhttp.Handler())
	srv.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	healthHandler := newHealthHandler(database, rds)
	srv.HandleFunc("/health", healthHandler)
	srv.HandleFunc("/readyz", healthHandler)

	log.NewHelper(logger).Infof("HTTP server listening on %s", c.HTTP.Addr)

	return srv, nil
}

func newHealthHandler(database *gorm.DB, rds *cache.Redis) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if database != nil {
			sqlDB, err := database.DB()
			if err != nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "database": err.Error()})
				return
			}
			if err := sqlDB.PingContext(ctx); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "database": err.Error()})
				return
			}
		}
		if rds != nil && rds.Client != nil {
			if err := rds.Client.Ping(ctx).Err(); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "redis": err.Error()})
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
