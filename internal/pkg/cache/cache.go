package cache

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"time"

	"github.com/go-kratos/aegis/circuitbreaker"
	"github.com/go-kratos/aegis/circuitbreaker/sre"
	"github.com/go-kratos/kratos-layout-monolith/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-redis/redis/extra/redisotel/v8"
	"github.com/go-redis/redis/v8"
	"github.com/google/wire"

	"golang.org/x/sync/singleflight"
)

const notFoundPlaceholder = "*"

var (
	ErrNotFound     = errors.New("not found")
	errPlaceholder  = errors.New("placeholder")
	defaultExpiry   = 2 * time.Hour
	defaultNotFound = 1 * time.Minute
	expiryDeviation = 0.05
)

var ProviderSet = wire.NewSet(NewRedis)

type Redis struct {
	Client         *redis.Client
	collector      MetricsCollector
	sf             singleflight.Group
	expiry         time.Duration
	notFoundExpiry time.Duration
	breaker        circuitbreaker.CircuitBreaker
	logger         *log.Helper
	cancel         context.CancelFunc
}

func NewRedis(c *conf.Redis, logger log.Logger) *Redis {
	helper := log.NewHelper(log.With(logger, "component", "cache"))

	r := &Redis{
		collector:      NewCompositeMetrics(NewLocalMetrics(), NewPrometheusMetrics("app", nil)),
		expiry:         defaultExpiry,
		notFoundExpiry: defaultNotFound,
		breaker: sre.NewBreaker(
			sre.WithSuccess(0.6),
			sre.WithRequest(100),
			sre.WithWindow(3*time.Second),
			sre.WithBucket(10),
		),
		logger: helper,
	}

	if c == nil {
		helper.Warn("redis config is nil, skipping redis initialization")
		return r
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
	client.AddHook(redisotel.NewTracingHook())

	if err := client.Ping(context.Background()).Err(); err != nil {
		helper.Warnf("redis ping failed: %v", err)
	} else {
		helper.Infof("redis connected at %s", c.Addr)
	}

	r.Client = client

	statCtx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	go r.statLoop(statCtx)

	return r
}

func (r *Redis) Close() error {
	if r.cancel != nil {
		r.cancel()
	}
	if r.Client != nil {
		return r.Client.Close()
	}
	return nil
}

func (r *Redis) Metrics() MetricsCollector { return r.collector }

func (r *Redis) Expiry() time.Duration         { return r.expiry }
func (r *Redis) NotFoundExpiry() time.Duration { return r.notFoundExpiry }

func (r *Redis) accept() bool {
	return r.breaker.Allow() == nil
}

func (r *Redis) Take(ctx context.Context, key string, val any, queryFn func() error) error {
	r.collector.Miss(key)

	data, err := r.cacheGet(ctx, key)
	if err == nil && len(data) > 0 {
		if string(data) == notFoundPlaceholder {
			r.collector.Hit(key)
			return ErrNotFound
		}
		r.collector.Hit(key)
		if jerr := json.Unmarshal(data, val); jerr == nil {
			return nil
		}
		r.cacheDel(ctx, key)
	}

	type result struct {
		data []byte
		nf   bool
	}

	val2, err, _ := r.sf.Do(key, func() (any, error) {
		r.collector.DBOp(key)
		r.collector.SingleFlight(key)
		qerr := queryFn()
		if errors.Is(qerr, ErrNotFound) {
			r.cacheSetNotFound(ctx, key)
			return result{nf: true}, nil
		}
		if qerr != nil {
			r.collector.DbFail(key)
			return nil, qerr
		}

		data, _ := json.Marshal(val)
		r.cacheSet(ctx, key, data, r.expiry)
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

func (r *Redis) Set(ctx context.Context, key string, val any, ttl time.Duration) error {
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return r.cacheSet(ctx, key, data, ttl)
}

func (r *Redis) Del(ctx context.Context, keys ...string) error {
	return r.cacheDel(ctx, keys...)
}

func (r *Redis) cacheGet(ctx context.Context, key string) ([]byte, error) {
	if r.Client == nil {
		return nil, nil
	}

	if !r.accept() {
		return nil, nil
	}

	data, err := r.Client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			r.breaker.MarkSuccess()
			return nil, nil
		}
		r.breaker.MarkFailed()
		r.logger.Warnf("redis get %s failed: %v", key, err)
		return nil, nil
	}
	r.breaker.MarkSuccess()
	return data, nil
}

func (r *Redis) cacheSet(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	if r.Client == nil {
		return nil
	}

	if !r.accept() {
		return nil
	}

	err := r.Client.Set(ctx, key, data, aroundDuration(ttl)).Err()
	if err != nil {
		r.breaker.MarkFailed()
		r.logger.Warnf("redis set %s failed: %v", key, err)
		return nil
	}
	r.breaker.MarkSuccess()
	return nil
}

func (r *Redis) cacheSetNotFound(ctx context.Context, key string) {
	if r.Client == nil {
		return
	}

	if !r.accept() {
		return
	}

	seconds := int64(aroundDuration(r.notFoundExpiry).Seconds())
	ok, err := r.Client.SetNX(ctx, key, notFoundPlaceholder, time.Duration(seconds)*time.Second).Result()
	if err != nil {
		r.breaker.MarkFailed()
		r.logger.Warnf("redis setnx not-found %s failed: %v", key, err)
		return
	}
	r.breaker.MarkSuccess()
	if !ok {
		r.logger.Infof("not-found placeholder already exists for key %s", key)
	}
}

func (r *Redis) cacheDel(ctx context.Context, keys ...string) error {
	if r.Client == nil || len(keys) == 0 {
		return nil
	}

	if !r.accept() {
		return nil
	}

	err := r.Client.Del(ctx, keys...).Err()
	if err != nil {
		r.breaker.MarkFailed()
		r.logger.Warnf("redis del %v failed: %v", keys, err)
		return err
	}
	r.breaker.MarkSuccess()
	return nil
}

func (r *Redis) statLoop(ctx context.Context) {
	reader, ok := r.collector.(MetricsReader)
	if !ok {
		return
	}

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.logger.Infof("cache stats - hit: %d, miss: %d, hit_rate: %.2f, db_ops: %d, db_fails: %d, singleflight: %d",
				reader.HitCount(),
				reader.MissCount(),
				reader.HitRate(),
				reader.DBOpCount(),
				reader.DbFailCount(),
				reader.SingleFlightCount(),
			)
		}
	}
}

func aroundDuration(d time.Duration) time.Duration {
	jitter := time.Duration(float64(d) * expiryDeviation * (rand.Float64()*2 - 1))
	return d + jitter
}
