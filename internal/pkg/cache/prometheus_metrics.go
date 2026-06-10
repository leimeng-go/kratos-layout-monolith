package cache

import (
	"github.com/prometheus/client_golang/prometheus"
)

type PrometheusMetrics struct {
	hits          *prometheus.CounterVec
	misses        *prometheus.CounterVec
	dbOps         *prometheus.CounterVec
	dbFails       *prometheus.CounterVec
	singleFlights *prometheus.CounterVec
}

func NewPrometheusMetrics(namespace string, reg prometheus.Registerer) *PrometheusMetrics {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}

	factory := func(name, help string) *prometheus.CounterVec {
		cv := prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      name,
			Help:      help,
		}, []string{"key"})
		reg.MustRegister(cv)
		return cv
	}

	return &PrometheusMetrics{
		hits:          factory("cache_hit_total", "Total cache hits"),
		misses:        factory("cache_miss_total", "Total cache misses"),
		dbOps:         factory("cache_db_op_total", "Total DB operations"),
		dbFails:       factory("cache_db_fail_total", "Total DB failures"),
		singleFlights: factory("cache_singleflight_total", "Total singleflight shared hits"),
	}
}

func (p *PrometheusMetrics) Hit(key string)          { p.hits.WithLabelValues(key).Inc() }
func (p *PrometheusMetrics) Miss(key string)         { p.misses.WithLabelValues(key).Inc() }
func (p *PrometheusMetrics) DBOp(key string)         { p.dbOps.WithLabelValues(key).Inc() }
func (p *PrometheusMetrics) DbFail(key string)       { p.dbFails.WithLabelValues(key).Inc() }
func (p *PrometheusMetrics) SingleFlight(key string) { p.singleFlights.WithLabelValues(key).Inc() }
