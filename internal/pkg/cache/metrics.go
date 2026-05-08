package cache

import (
	"math"
	"sync/atomic"
)

// Metrics collects cache hit/miss statistics.
type Metrics struct {
	hits   atomic.Int64
	misses atomic.Int64
	dbOps  atomic.Int64
	sfs    atomic.Int64
}

func NewMetrics() *Metrics {
	return &Metrics{}
}

func (m *Metrics) Hit()          { m.hits.Add(1) }
func (m *Metrics) Miss()         { m.misses.Add(1) }
func (m *Metrics) DBOp()         { m.dbOps.Add(1) }
func (m *Metrics) SingleFlight() { m.sfs.Add(1) }

func (m *Metrics) HitCount() int64        { return m.hits.Load() }
func (m *Metrics) MissCount() int64       { return m.misses.Load() }
func (m *Metrics) DBOpCount() int64       { return m.dbOps.Load() }
func (m *Metrics) SingleFlightCount() int64 { return m.sfs.Load() }

func (m *Metrics) HitRate() float64 {
	total := m.hits.Load() + m.misses.Load()
	if total == 0 {
		return 0
	}
	rate := float64(m.hits.Load()) / float64(total)
	return math.Round(rate*100) / 100
}
