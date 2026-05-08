package cache

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"time"

	"github.com/go-kratos/kratos-layout-monolith/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-redis/redis/v8"
	"github.com/google/wire"
	"golang.org/x/sync/singleflight"
)

const notFoundPlaceholder = "*"

// ErrNotFound indicates the requested record was not found in the database.
var ErrNotFound = errors.New("not found")

// ProviderSet is cache providers.
var ProviderSet = wire.NewSet(NewRedis)

// Redis wraps a Redis client with cache operations.
type Redis struct {
	Client  *redis.Client
	metrics *Metrics
	sf      singleflight.Group
}

// NewRedis creates a new Redis client with cache capabilities.
func NewRedis(c *conf.Redis, logger log.Logger) *Redis {
	if c == nil {
		log.NewHelper(logger).Warn("redis config is nil, skipping redis initialization")
		return &Redis{metrics: NewMetrics()}
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

	return &Redis{Client: client, metrics: NewMetrics()}
}

// Close closes the Redis client.
func (r *Redis) Close() error {
	if r.Client != nil {
		return r.Client.Close()
	}
	return nil
}

// Metrics returns the cache metrics collector.
func (r *Redis) Metrics() *Metrics { return r.metrics }

// Take checks cache first, if miss calls queryFn to fetch from DB and caches the result.
// Uses singleflight to prevent cache stampede.
func (r *Redis) Take(ctx context.Context, key string, val any, ttl time.Duration, queryFn func() error) error {
	r.metrics.Miss() // pessimistic count; will be corrected on hit path

	// Try cache first
	data, err := r.Client.Get(ctx, key).Bytes()
	if err == nil && len(data) > 0 {
		if string(data) == notFoundPlaceholder {
			r.metrics.Hit()
			return ErrNotFound
		}
		r.metrics.Hit()
		if jerr := json.Unmarshal(data, val); jerr == nil {
			return nil
		}
		// Corrupted cache, delete and fall through to DB
		r.Client.Del(ctx, key)
	}

	// Use singleflight to prevent cache stampede
	type result struct {
		data []byte
		nf   bool
	}

	val2, err, _ := r.sf.Do(key, func() (any, error) {
		r.metrics.DBOp()
		r.metrics.SingleFlight()
		qerr := queryFn()
		if errors.Is(qerr, ErrNotFound) {
			r.Client.Set(ctx, key, notFoundPlaceholder, aroundDuration(60*time.Second))
			return result{nf: true}, nil
		}
		if qerr != nil {
			return nil, qerr
		}

		data, _ := json.Marshal(val)
		r.Client.Set(ctx, key, data, aroundDuration(ttl))
		return result{data: data}, nil
	})
	if err != nil {
		return err
	}

	res := val2.(result)
	if res.nf {
		return ErrNotFound
	}
	return json.Unmarshal(res.data, val)
}

// Set writes a value to cache with the given TTL and random jitter.
func (r *Redis) Set(ctx context.Context, key string, val any, ttl time.Duration) error {
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return r.Client.Set(ctx, key, data, aroundDuration(ttl)).Err()
}

// Del deletes one or more cache keys.
func (r *Redis) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return r.Client.Del(ctx, keys...).Err()
}

func aroundDuration(d time.Duration) time.Duration {
	jitter := time.Duration(float64(d) * 0.05 * (rand.Float64()*2 - 1))
	return d + jitter
}
