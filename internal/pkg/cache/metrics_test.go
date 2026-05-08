package cache

import (
	"math"
	"testing"
)

func TestMetricsHitRate(t *testing.T) {
	m := NewMetrics()
	m.Hit()
	m.Hit()
	m.Miss()
	rate := m.HitRate()
	expected := math.Round((2.0/3.0)*100) / 100
	if rate != expected {
		t.Errorf("expected %.4f, got %.4f", expected, rate)
	}
}

func TestMetricsZeroHits(t *testing.T) {
	m := NewMetrics()
	m.Miss()
	if m.HitRate() != 0 {
		t.Errorf("expected 0, got %.4f", m.HitRate())
	}
}

func TestMetricsCounters(t *testing.T) {
	m := NewMetrics()
	for i := 0; i < 10; i++ {
		m.Hit()
	}
	for i := 0; i < 5; i++ {
		m.Miss()
	}
	for i := 0; i < 3; i++ {
		m.DBOp()
	}
	for i := 0; i < 2; i++ {
		m.SingleFlight()
	}

	if m.HitCount() != 10 {
		t.Errorf("HitCount: want 10, got %d", m.HitCount())
	}
	if m.MissCount() != 5 {
		t.Errorf("MissCount: want 5, got %d", m.MissCount())
	}
	if m.DBOpCount() != 3 {
		t.Errorf("DBOpCount: want 3, got %d", m.DBOpCount())
	}
	if m.SingleFlightCount() != 2 {
		t.Errorf("SingleFlightCount: want 2, got %d", m.SingleFlightCount())
	}
}
