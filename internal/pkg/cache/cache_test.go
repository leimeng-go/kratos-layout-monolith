package cache

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-kratos/aegis/circuitbreaker/sre"
	"github.com/go-redis/redis/v8"
	"github.com/go-kratos/kratos/v2/log"
)

func newTestRedis(t *testing.T) (*Redis, *miniredis.Miniredis) {
	t.Helper()
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	r := &Redis{
		Client:         client,
		collector:      NewLocalMetrics(),
		expiry:         defaultExpiry,
		notFoundExpiry: defaultNotFound,
		breaker:        sre.NewBreaker(),
		logger:         log.NewHelper(log.NewStdLogger(nil)),
	}
	return r, s
}

type testUser struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func TestTakeCacheHit(t *testing.T) {
	r, s := newTestRedis(t)
	defer s.Close()

	data, _ := json.Marshal(testUser{ID: 1, Name: "test"})
	s.Set("user:id:1", string(data))

	var user testUser
	callCount := int64(0)
	err := r.Take(context.Background(), "user:id:1", &user, func() error {
		atomic.AddInt64(&callCount, 1)
		return errors.New("should not be called")
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.ID != 1 || user.Name != "test" {
		t.Errorf("unexpected user: %+v", user)
	}
	if atomic.LoadInt64(&callCount) != 0 {
		t.Error("query should not be called on cache hit")
	}
	if r.collector.(MetricsReader).HitCount() != 1 {
		t.Errorf("HitCount: want 1, got %d", r.collector.(MetricsReader).HitCount())
	}
}

func TestTakeCacheMiss(t *testing.T) {
	r, s := newTestRedis(t)
	defer s.Close()

	var user testUser
	err := r.Take(context.Background(), "user:id:2", &user, func() error {
		user = testUser{ID: 2, Name: "fetched"}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.Name != "fetched" {
		t.Errorf("unexpected user: %+v", user)
	}
	val, _ := s.Get("user:id:2")
	if val == "" {
		t.Error("data should be cached")
	}
	if r.collector.(MetricsReader).MissCount() != 1 {
		t.Errorf("MissCount: want 1, got %d", r.collector.(MetricsReader).MissCount())
	}
	if r.collector.(MetricsReader).DBOpCount() != 1 {
		t.Errorf("DBOpCount: want 1, got %d", r.collector.(MetricsReader).DBOpCount())
	}
}

func TestTakeNotFound(t *testing.T) {
	r, s := newTestRedis(t)
	defer s.Close()

	var user testUser
	err := r.Take(context.Background(), "user:id:999", &user, func() error {
		return ErrNotFound
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	val, _ := s.Get("user:id:999")
	if val != "*" {
		t.Errorf("expected placeholder '*', got %q", val)
	}
}

func TestTakeNotFoundUsesSetNX(t *testing.T) {
	r, s := newTestRedis(t)
	defer s.Close()

	var user testUser
	err := r.Take(context.Background(), "user:nf:nx", &user, func() error {
		return ErrNotFound
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}

	val, _ := s.Get("user:nf:nx")
	if val != "*" {
		t.Errorf("expected placeholder '*', got %q", val)
	}

	callCount := int64(0)
	err = r.Take(context.Background(), "user:nf:nx", &user, func() error {
		atomic.AddInt64(&callCount, 1)
		return ErrNotFound
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound from cached placeholder, got %v", err)
	}
	if atomic.LoadInt64(&callCount) != 0 {
		t.Error("query should not be called when placeholder exists in cache")
	}
}

func TestTakeDBFail(t *testing.T) {
	r, s := newTestRedis(t)
	defer s.Close()

	var user testUser
	err := r.Take(context.Background(), "user:dbfail:1", &user, func() error {
		return errors.New("db connection refused")
	})
	if err == nil {
		t.Fatal("expected error from DB failure")
	}
	if r.collector.(MetricsReader).DbFailCount() != 1 {
		t.Errorf("DbFailCount: want 1, got %d", r.collector.(MetricsReader).DbFailCount())
	}
}

func TestDelKeys(t *testing.T) {
	r, s := newTestRedis(t)
	defer s.Close()

	s.Set("user:id:1", `{"id":1}`)
	s.Set("user:username:test", `{"id":1}`)

	err := r.Del(context.Background(), "user:id:1", "user:username:test")
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.Get("user:id:1")
	if err == nil {
		t.Error("key should be deleted")
	}
}

func TestSetCache(t *testing.T) {
	r, s := newTestRedis(t)
	defer s.Close()

	user := testUser{ID: 10, Name: "settest"}
	err := r.Set(context.Background(), "user:id:10", user, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	val, _ := s.Get("user:id:10")
	if val == "" {
		t.Error("key should exist")
	}
}

func TestTakeSingleFlight(t *testing.T) {
	r, s := newTestRedis(t)
	defer s.Close()

	var callCount int64
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			var user testUser
			err := r.Take(context.Background(), "user:sf:1", &user, func() error {
				atomic.AddInt64(&callCount, 1)
				time.Sleep(50 * time.Millisecond)
				user = testUser{ID: 1, Name: "sf"}
				return nil
			})
			if err != nil {
				t.Error(err)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if atomic.LoadInt64(&callCount) != 1 {
		t.Errorf("singleflight: expected 1 call, got %d", atomic.LoadInt64(&callCount))
	}
}
