package cache

import (
	"sort"
	"sync"
	"sync/atomic"
)

type KeyStats struct {
	Key           string
	Hits          int64
	Misses        int64
	DBOps         int64
	DBFails       int64
	SingleFlights int64
}

type perKeyEntry struct {
	hits          atomic.Int64
	misses        atomic.Int64
	dbOps         atomic.Int64
	dbFails       atomic.Int64
	singleFlights atomic.Int64
}

type PerKeyMetrics struct {
	inner   MetricsCollector
	keys    sync.Map
	hits    atomic.Int64
	misses  atomic.Int64
	dbOps   atomic.Int64
	dbFails atomic.Int64
	sfs     atomic.Int64
}

func NewPerKeyMetrics(inner MetricsCollector) *PerKeyMetrics {
	return &PerKeyMetrics{inner: inner}
}

func (p *PerKeyMetrics) getOrCreate(key string) *perKeyEntry {
	if v, ok := p.keys.Load(key); ok {
		return v.(*perKeyEntry)
	}
	e := &perKeyEntry{}
	actual, _ := p.keys.LoadOrStore(key, e)
	return actual.(*perKeyEntry)
}

func (p *PerKeyMetrics) Hit(key string) {
	p.hits.Add(1)
	p.getOrCreate(key).hits.Add(1)
	p.inner.Hit(key)
}

func (p *PerKeyMetrics) Miss(key string) {
	p.misses.Add(1)
	p.getOrCreate(key).misses.Add(1)
	p.inner.Miss(key)
}

func (p *PerKeyMetrics) DBOp(key string) {
	p.dbOps.Add(1)
	p.getOrCreate(key).dbOps.Add(1)
	p.inner.DBOp(key)
}

func (p *PerKeyMetrics) DbFail(key string) {
	p.dbFails.Add(1)
	p.getOrCreate(key).dbFails.Add(1)
	p.inner.DbFail(key)
}

func (p *PerKeyMetrics) SingleFlight(key string) {
	p.sfs.Add(1)
	p.getOrCreate(key).singleFlights.Add(1)
	p.inner.SingleFlight(key)
}

func (p *PerKeyMetrics) HitCount() int64          { return p.hits.Load() }
func (p *PerKeyMetrics) MissCount() int64         { return p.misses.Load() }
func (p *PerKeyMetrics) DBOpCount() int64         { return p.dbOps.Load() }
func (p *PerKeyMetrics) DbFailCount() int64       { return p.dbFails.Load() }
func (p *PerKeyMetrics) SingleFlightCount() int64 { return p.sfs.Load() }

func (p *PerKeyMetrics) HitRate() float64 {
	total := p.hits.Load() + p.misses.Load()
	if total == 0 {
		return 0
	}
	rate := float64(p.hits.Load()) / float64(total)
	return float64(int(rate*100)) / 100
}

func (p *PerKeyMetrics) TopKeys(n int) []KeyStats {
	var stats []KeyStats
	p.keys.Range(func(key, value any) bool {
		e := value.(*perKeyEntry)
		stats = append(stats, KeyStats{
			Key:           key.(string),
			Hits:          e.hits.Load(),
			Misses:        e.misses.Load(),
			DBOps:         e.dbOps.Load(),
			DBFails:       e.dbFails.Load(),
			SingleFlights: e.singleFlights.Load(),
		})
		return true
	})
	sort.Slice(stats, func(i, j int) bool {
		return (stats[i].Hits + stats[i].Misses) > (stats[j].Hits + stats[j].Misses)
	})
	if n < len(stats) {
		stats = stats[:n]
	}
	return stats
}
