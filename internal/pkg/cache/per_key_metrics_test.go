package cache

import (
	"sync"
	"testing"
)

func TestPerKeyMetricsCounts(t *testing.T) {
	inner := NewLocalMetrics()
	pk := NewPerKeyMetrics(inner)

	pk.Hit("key1")
	pk.Hit("key1")
	pk.Hit("key2")
	pk.Miss("key3")

	if pk.HitCount() != 3 {
		t.Errorf("HitCount: want 3, got %d", pk.HitCount())
	}
	if pk.MissCount() != 1 {
		t.Errorf("MissCount: want 1, got %d", pk.MissCount())
	}

	top := pk.TopKeys(2)
	if len(top) != 2 {
		t.Fatalf("TopKeys(2): want 2 entries, got %d", len(top))
	}
	if top[0].Key != "key1" || top[0].Hits != 2 {
		t.Errorf("top key: want key1 with 2 hits, got %s with %d hits", top[0].Key, top[0].Hits)
	}
}

func TestPerKeyMetricsConcurrent(t *testing.T) {
	inner := NewLocalMetrics()
	pk := NewPerKeyMetrics(inner)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pk.Hit("concurrent_key")
		}()
	}
	wg.Wait()

	if pk.HitCount() != 100 {
		t.Errorf("HitCount: want 100, got %d", pk.HitCount())
	}
}

func TestPerKeyMetricsDelegatesToInner(t *testing.T) {
	inner := NewLocalMetrics()
	pk := NewPerKeyMetrics(inner)

	pk.Hit("key1")
	pk.Miss("key2")

	if inner.HitCount() != 1 {
		t.Errorf("inner HitCount: want 1, got %d", inner.HitCount())
	}
	if inner.MissCount() != 1 {
		t.Errorf("inner MissCount: want 1, got %d", inner.MissCount())
	}
}
