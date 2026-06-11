package observability

import (
	"database/sql"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
)

var poolRegisterOnce sync.Once

func RegisterPoolMetrics(database *gorm.DB, rds *redis.Client) {
	poolRegisterOnce.Do(func() {
		if database != nil {
			if sqlDB, err := database.DB(); err == nil {
				registerCollector(NewDBStatsCollector(sqlDB))
			}
		}
		if rds != nil {
			registerCollector(NewRedisStatsCollector(rds))
		}
	})
}

func registerCollector(collector prometheus.Collector) {
	if err := prometheus.Register(collector); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}
}

type DBStatsCollector struct {
	db                *sql.DB
	open              *prometheus.Desc
	inUse             *prometheus.Desc
	idle              *prometheus.Desc
	wait              *prometheus.Desc
	waitDuration      *prometheus.Desc
	maxIdleClosed     *prometheus.Desc
	maxIdleTimeClosed *prometheus.Desc
	maxLifetimeClosed *prometheus.Desc
}

func NewDBStatsCollector(db *sql.DB) *DBStatsCollector {
	return &DBStatsCollector{
		db:                db,
		open:              prometheus.NewDesc("app_db_open_connections", "The number of established database connections.", nil, nil),
		inUse:             prometheus.NewDesc("app_db_in_use_connections", "The number of database connections currently in use.", nil, nil),
		idle:              prometheus.NewDesc("app_db_idle_connections", "The number of idle database connections.", nil, nil),
		wait:              prometheus.NewDesc("app_db_wait_count_total", "The total number of database connection waits.", nil, nil),
		waitDuration:      prometheus.NewDesc("app_db_wait_duration_seconds_total", "The total time blocked waiting for a database connection.", nil, nil),
		maxIdleClosed:     prometheus.NewDesc("app_db_max_idle_closed_total", "The total number of database connections closed due to SetMaxIdleConns.", nil, nil),
		maxIdleTimeClosed: prometheus.NewDesc("app_db_max_idle_time_closed_total", "The total number of database connections closed due to SetConnMaxIdleTime.", nil, nil),
		maxLifetimeClosed: prometheus.NewDesc("app_db_max_lifetime_closed_total", "The total number of database connections closed due to SetConnMaxLifetime.", nil, nil),
	}
}

func (c *DBStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.open
	ch <- c.inUse
	ch <- c.idle
	ch <- c.wait
	ch <- c.waitDuration
	ch <- c.maxIdleClosed
	ch <- c.maxIdleTimeClosed
	ch <- c.maxLifetimeClosed
}

func (c *DBStatsCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.db.Stats()
	ch <- prometheus.MustNewConstMetric(c.open, prometheus.GaugeValue, float64(stats.OpenConnections))
	ch <- prometheus.MustNewConstMetric(c.inUse, prometheus.GaugeValue, float64(stats.InUse))
	ch <- prometheus.MustNewConstMetric(c.idle, prometheus.GaugeValue, float64(stats.Idle))
	ch <- prometheus.MustNewConstMetric(c.wait, prometheus.CounterValue, float64(stats.WaitCount))
	ch <- prometheus.MustNewConstMetric(c.waitDuration, prometheus.CounterValue, stats.WaitDuration.Seconds())
	ch <- prometheus.MustNewConstMetric(c.maxIdleClosed, prometheus.CounterValue, float64(stats.MaxIdleClosed))
	ch <- prometheus.MustNewConstMetric(c.maxIdleTimeClosed, prometheus.CounterValue, float64(stats.MaxIdleTimeClosed))
	ch <- prometheus.MustNewConstMetric(c.maxLifetimeClosed, prometheus.CounterValue, float64(stats.MaxLifetimeClosed))
}

type RedisStatsCollector struct {
	client     *redis.Client
	hits       *prometheus.Desc
	misses     *prometheus.Desc
	timeouts   *prometheus.Desc
	totalConns *prometheus.Desc
	idleConns  *prometheus.Desc
	staleConns *prometheus.Desc
}

func NewRedisStatsCollector(client *redis.Client) *RedisStatsCollector {
	return &RedisStatsCollector{
		client:     client,
		hits:       prometheus.NewDesc("app_redis_pool_hits_total", "The total number of Redis connection pool hits.", nil, nil),
		misses:     prometheus.NewDesc("app_redis_pool_misses_total", "The total number of Redis connection pool misses.", nil, nil),
		timeouts:   prometheus.NewDesc("app_redis_pool_timeouts_total", "The total number of Redis connection pool timeouts.", nil, nil),
		totalConns: prometheus.NewDesc("app_redis_pool_total_connections", "The number of Redis connections in the pool.", nil, nil),
		idleConns:  prometheus.NewDesc("app_redis_pool_idle_connections", "The number of idle Redis connections in the pool.", nil, nil),
		staleConns: prometheus.NewDesc("app_redis_pool_stale_connections_total", "The total number of stale Redis connections removed from the pool.", nil, nil),
	}
}

func (c *RedisStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.hits
	ch <- c.misses
	ch <- c.timeouts
	ch <- c.totalConns
	ch <- c.idleConns
	ch <- c.staleConns
}

func (c *RedisStatsCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.client.PoolStats()
	ch <- prometheus.MustNewConstMetric(c.hits, prometheus.CounterValue, float64(stats.Hits))
	ch <- prometheus.MustNewConstMetric(c.misses, prometheus.CounterValue, float64(stats.Misses))
	ch <- prometheus.MustNewConstMetric(c.timeouts, prometheus.CounterValue, float64(stats.Timeouts))
	ch <- prometheus.MustNewConstMetric(c.totalConns, prometheus.GaugeValue, float64(stats.TotalConns))
	ch <- prometheus.MustNewConstMetric(c.idleConns, prometheus.GaugeValue, float64(stats.IdleConns))
	ch <- prometheus.MustNewConstMetric(c.staleConns, prometheus.CounterValue, float64(stats.StaleConns))
}
