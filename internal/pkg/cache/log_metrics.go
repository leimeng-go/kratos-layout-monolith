package cache

import (
	"github.com/go-kratos/kratos/v2/log"
)

type LogMetrics struct {
	logger *log.Helper
}

func NewLogMetrics(logger log.Logger) *LogMetrics {
	return &LogMetrics{
		logger: log.NewHelper(log.With(logger, "component", "cache_metrics")),
	}
}

func (l *LogMetrics) Hit(key string) {
	l.logger.Infof("cache hit key=%s", key)
}

func (l *LogMetrics) Miss(key string) {
	l.logger.Infof("cache miss key=%s", key)
}

func (l *LogMetrics) DBOp(key string) {
	l.logger.Infof("cache db_op key=%s", key)
}

func (l *LogMetrics) DbFail(key string) {
	l.logger.Warnf("cache db_fail key=%s", key)
}

func (l *LogMetrics) SingleFlight(key string) {
	l.logger.Infof("cache singleflight key=%s", key)
}
