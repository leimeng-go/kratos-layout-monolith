package cache

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestPrometheusMetricsRecords(t *testing.T) {
	reg := prometheus.NewRegistry()
	pm := NewPrometheusMetrics("test_cache", reg)

	pm.Hit("user:1")
	pm.Hit("user:1")
	pm.Miss("user:2")
	pm.DBOp("user:2")
	pm.DbFail("user:3")
	pm.SingleFlight("user:1")

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	if len(mfs) == 0 {
		t.Error("expected metrics to be gathered")
	}
}

func TestPrometheusMetricsNilRegistry(t *testing.T) {
	pm := NewPrometheusMetrics("test_cache_nil", nil)
	pm.Hit("key")
}
