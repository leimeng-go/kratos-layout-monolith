package lock

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos-layout-monolith/internal/pkg/cache"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v8"
	"github.com/google/wire"
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
	rs *redsync.Redsync
}

// NewRedisLocker creates a new Redis-based distributed lock using Redsync.
func NewRedisLocker(rds *cache.Redis) Locker {
	pool := goredis.NewPool(rds.Client)
	return &redisLocker{
		rs: redsync.New(pool),
	}
}

// Lock acquires a distributed lock with the given key and TTL.
func (l *redisLocker) Lock(ctx context.Context, key string, ttl time.Duration) (func(), error) {
	mutex := l.rs.NewMutex(key,
		redsync.WithExpiry(ttl),
		redsync.WithTries(1), // single attempt, don't retry
	)

	if err := mutex.LockContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to acquire lock %s: %w", key, err)
	}

	unlock := func() {
		mutex.UnlockContext(context.Background())
	}

	return unlock, nil
}
