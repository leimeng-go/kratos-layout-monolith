package lock

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos-layout-monolith/internal/pkg/cache"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v8"
	"github.com/google/wire"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ProviderSet is lock providers.
var ProviderSet = wire.NewSet(NewRedisLocker)

// Locker is the distributed lock interface.
// Each bounded context (user, order, etc.) can define its own Locker interface
// with the same signature — this implementation satisfies all of them.
type Locker interface {
	// Lock acquires a distributed lock with the given key and TTL.
	// Returns an unlock function and nil on success.
	// Returns nil and an error if the lock cannot be acquired.
	Lock(ctx context.Context, key string, ttl time.Duration) (unlock func(), err error)
}

// redisLocker implements Locker using Redsync (Redlock algorithm).
type redisLocker struct {
	rs     *redsync.Redsync
	tracer trace.Tracer
}

// NewRedisLocker creates a new Redis-based distributed lock using Redsync.
func NewRedisLocker(rds *cache.Redis) Locker {
	pool := goredis.NewPool(rds.Client)
	return &redisLocker{
		rs:     redsync.New(pool),
		tracer: otel.Tracer("github.com/go-kratos/kratos-layout-monolith/internal/pkg/lock"),
	}
}

// Lock acquires a distributed lock with the given key and TTL.
func (l *redisLocker) Lock(ctx context.Context, key string, ttl time.Duration) (func(), error) {
	ctx, span := l.tracer.Start(ctx, "lock.acquire", trace.WithAttributes(
		attribute.String("lock.key", key),
		attribute.Int64("lock.ttl_ms", ttl.Milliseconds()),
	))
	defer span.End()

	mutex := l.rs.NewMutex(key,
		redsync.WithExpiry(ttl),
		redsync.WithTries(1), // single attempt, don't retry
	)

	if err := mutex.LockContext(ctx); err != nil {
		err = fmt.Errorf("failed to acquire lock %s: %w", key, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	unlock := func() {
		_, unlockSpan := l.tracer.Start(context.Background(), "lock.release", trace.WithAttributes(attribute.String("lock.key", key)))
		defer unlockSpan.End()
		if ok, err := mutex.UnlockContext(context.Background()); !ok || err != nil {
			if err == nil {
				err = fmt.Errorf("failed to release lock %s", key)
			}
			unlockSpan.RecordError(err)
			unlockSpan.SetStatus(codes.Error, err.Error())
		}
	}

	return unlock, nil
}
