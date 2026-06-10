package cache

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-kratos/aegis/circuitbreaker/sre"
	"github.com/go-redis/redis/v8"
	"github.com/go-kratos/kratos/v2/log"
)

func newTestCachedDB(t *testing.T) (*CachedDB, *miniredis.Miniredis) {
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
	cdb := &CachedDB{
		cache:  r,
		logger: log.NewHelper(log.NewStdLogger(nil)),
	}
	return cdb, s
}

func TestQueryRowCacheHit(t *testing.T) {
	cdb, s := newTestCachedDB(t)
	defer s.Close()

	s.Set("cache:users:id:1", `{"id":1,"name":"cached"}`)

	var user testUser
	err := cdb.QueryRow(context.Background(), &user, "cache:users:id:1", func() error {
		return errors.New("should not be called")
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.Name != "cached" {
		t.Errorf("expected cached user, got %+v", user)
	}
}

func TestQueryRowCacheMiss(t *testing.T) {
	cdb, s := newTestCachedDB(t)
	defer s.Close()

	var user testUser
	err := cdb.QueryRow(context.Background(), &user, "cache:users:id:2", func() error {
		user = testUser{ID: 2, Name: "fromdb"}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.Name != "fromdb" {
		t.Errorf("expected fromdb, got %+v", user)
	}
	val, _ := s.Get("cache:users:id:2")
	if val == "" {
		t.Error("should be cached after miss")
	}
}

func TestQueryRowIndexCacheHit(t *testing.T) {
	cdb, s := newTestCachedDB(t)
	defer s.Close()

	s.Set("cache:users:username:alice", "cache:users:id:1")
	s.Set("cache:users:id:1", `{"id":1,"name":"alice"}`)

	var user testUser
	err := cdb.QueryRowIndex(context.Background(), &user,
		"cache:users:username:alice",
		func() (string, error) {
			return "", errors.New("should not be called")
		},
		func() error {
			return errors.New("should not be called")
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if user.Name != "alice" {
		t.Errorf("expected alice, got %+v", user)
	}
}

func TestQueryRowIndexMiss(t *testing.T) {
	cdb, s := newTestCachedDB(t)
	defer s.Close()

	var user testUser
	dbCalled := int64(0)
	err := cdb.QueryRowIndex(context.Background(), &user,
		"cache:users:username:bob",
		func() (string, error) {
			atomic.AddInt64(&dbCalled, 1)
			return "cache:users:id:2", nil
		},
		func() error {
			return errors.New("should not be called since data was set in index query")
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt64(&dbCalled) != 1 {
		t.Error("index query should be called once")
	}
	indexVal, _ := s.Get("cache:users:username:bob")
	if indexVal != "cache:users:id:2" {
		t.Errorf("index key should store primary key, got %q", indexVal)
	}
}

func TestExecDeletesCache(t *testing.T) {
	cdb, s := newTestCachedDB(t)
	defer s.Close()

	s.Set("cache:users:id:1", `{"id":1}`)
	s.Set("cache:users:username:alice", "cache:users:id:1")

	err := cdb.Exec(context.Background(), func() error {
		return nil
	}, "cache:users:id:1", "cache:users:username:alice")
	if err != nil {
		t.Fatal(err)
	}

	_, e1 := s.Get("cache:users:id:1")
	_, e2 := s.Get("cache:users:username:alice")
	if e1 == nil || e2 == nil {
		t.Error("cache keys should be deleted after Exec")
	}
}

func TestExecRollbackOnError(t *testing.T) {
	cdb, s := newTestCachedDB(t)
	defer s.Close()

	s.Set("cache:users:id:1", `{"id":1}`)

	err := cdb.Exec(context.Background(), func() error {
		return errors.New("db error")
	}, "cache:users:id:1")
	if err == nil {
		t.Fatal("expected error from execFn")
	}

	val, _ := s.Get("cache:users:id:1")
	if val == "" {
		t.Error("cache should NOT be deleted when execFn fails")
	}
}

func TestQueryRowNoCache(t *testing.T) {
	cdb, s := newTestCachedDB(t)
	defer s.Close()

	var user testUser
	err := cdb.QueryRowNoCache(context.Background(), func() error {
		user = testUser{ID: 1, Name: "nocache"}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.Name != "nocache" {
		t.Errorf("expected nocache, got %+v", user)
	}

	_, err = s.Get("cache:users:id:1")
	if err == nil {
		t.Error("should not cache with QueryRowNoCache")
	}
}

func TestQueryRowIndexNotFound(t *testing.T) {
	cdb, s := newTestCachedDB(t)
	defer s.Close()

	var user testUser
	err := cdb.QueryRowIndex(context.Background(), &user,
		"cache:users:username:notexist",
		func() (string, error) {
			return "", ErrNotFound
		},
		func() error {
			return errors.New("should not be called")
		},
	)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	val, _ := s.Get("cache:users:username:notexist")
	if val != "*" {
		t.Errorf("expected placeholder '*', got %q", val)
	}
}
