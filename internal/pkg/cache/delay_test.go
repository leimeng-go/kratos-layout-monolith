package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-kratos/aegis/circuitbreaker/sre"
	"github.com/go-redis/redis/v8"
	"github.com/go-kratos/kratos/v2/log"
)

func newDelayTestRedis(t *testing.T, s *miniredis.Miniredis) *Redis {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	return &Redis{
		Client:         client,
		collector:      NewLocalMetrics(),
		expiry:         defaultExpiry,
		notFoundExpiry: defaultNotFound,
		breaker:        sre.NewBreaker(),
		logger:         log.NewHelper(log.NewStdLogger(nil)),
	}
}

func TestAsyncDelSuccess(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	r := newDelayTestRedis(t, s)

	s.Set("key1", "val1")
	s.Set("key2", "val2")

	AsyncDel(context.Background(), r, 3, []time.Duration{0, time.Second, 5 * time.Second}, "key1", "key2")

	time.Sleep(100 * time.Millisecond)

	_, e1 := s.Get("key1")
	_, e2 := s.Get("key2")
	if e1 == nil || e2 == nil {
		t.Error("keys should be deleted")
	}
}

func TestAsyncDelRetry(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}

	r := newDelayTestRedis(t, s)

	s.Set("retrykey", "val")

	s.Close()

	AsyncDel(context.Background(), r, 3, []time.Duration{0, 50 * time.Millisecond, 100 * time.Millisecond}, "retrykey")
}
