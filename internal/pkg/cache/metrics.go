package cache

import (
	"math"
	"sync/atomic"
)

type MetricsCollector interface {
	Hit(key string)
	Miss(key string)
	DBOp(key string)
	DbFail(key string)
	SingleFlight(key string)
}

type MetricsReader interface {
	HitCount() int64
	MissCount() int64
	DBOpCount() int64
	DbFailCount() int64
	SingleFlightCount() int64
	HitRate() float64
}

type LocalMetrics struct {
	hits    atomic.Int64
	misses  atomic.Int64
	dbOps   atomic.Int64
	dbFails atomic.Int64
	sfs     atomic.Int64
}

func NewLocalMetrics() *LocalMetrics {
	return &LocalMetrics{}
}

func (m *LocalMetrics) Hit(string)          { m.hits.Add(1) }
func (m *LocalMetrics) Miss(string)         { m.misses.Add(1) }
func (m *LocalMetrics) DBOp(string)         { m.dbOps.Add(1) }
func (m *LocalMetrics) DbFail(string)       { m.dbFails.Add(1) }
func (m *LocalMetrics) SingleFlight(string) { m.sfs.Add(1) }

func (m *LocalMetrics) HitCount() int64          { return m.hits.Load() }
func (m *LocalMetrics) MissCount() int64         { return m.misses.Load() }
func (m *LocalMetrics) DBOpCount() int64         { return m.dbOps.Load() }
func (m *LocalMetrics) DbFailCount() int64       { return m.dbFails.Load() }
func (m *LocalMetrics) SingleFlightCount() int64 { return m.sfs.Load() }

func (m *LocalMetrics) HitRate() float64 {
	total := m.hits.Load() + m.misses.Load()
	if total == 0 {
		return 0
	}
	rate := float64(m.hits.Load()) / float64(total)
	return math.Round(rate*100) / 100
}
