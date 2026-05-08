package cache

import (
	"context"

	"github.com/go-kratos/kratos-layout-monolith/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-redis/redis/v8"
	"github.com/google/wire"
)

// ProviderSet is cache providers.
var ProviderSet = wire.NewSet(NewRedis)

// Redis wraps a Redis client.
type Redis struct {
	Client *redis.Client
}

// NewRedis creates a new Redis client.
func NewRedis(c *conf.Redis, logger log.Logger) *Redis {
	if c == nil {
		log.NewHelper(logger).Warn("redis config is nil, skipping redis initialization")
		return &Redis{}
	}

	network := c.Network
	if network == "" {
		network = "tcp"
	}

	client := redis.NewClient(&redis.Options{
		Network:      network,
		Addr:         c.Addr,
		Password:     c.Password,
		DB:           c.DB,
		ReadTimeout:  c.ReadTimeout,
		WriteTimeout: c.WriteTimeout,
	})

	helper := log.NewHelper(log.With(logger, "component", "redis"))
	if err := client.Ping(context.Background()).Err(); err != nil {
		helper.Warnf("redis ping failed: %v", err)
	} else {
		helper.Infof("redis connected at %s", c.Addr)
	}

	return &Redis{Client: client}
}

// Close closes the Redis client.
func (r *Redis) Close() error {
	if r.Client != nil {
		return r.Client.Close()
	}
	return nil
}
