package cache

type CompositeMetrics struct {
	collectors []MetricsCollector
	reader     MetricsReader
}

func NewCompositeMetrics(collectors ...MetricsCollector) *CompositeMetrics {
	m := &CompositeMetrics{}
	for _, collector := range collectors {
		if collector == nil {
			continue
		}
		m.collectors = append(m.collectors, collector)
		if m.reader == nil {
			if reader, ok := collector.(MetricsReader); ok {
				m.reader = reader
			}
		}
	}
	return m
}

func (m *CompositeMetrics) Hit(key string) {
	for _, collector := range m.collectors {
		collector.Hit(key)
	}
}

func (m *CompositeMetrics) Miss(key string) {
	for _, collector := range m.collectors {
		collector.Miss(key)
	}
}

func (m *CompositeMetrics) DBOp(key string) {
	for _, collector := range m.collectors {
		collector.DBOp(key)
	}
}

func (m *CompositeMetrics) DbFail(key string) {
	for _, collector := range m.collectors {
		collector.DbFail(key)
	}
}

func (m *CompositeMetrics) SingleFlight(key string) {
	for _, collector := range m.collectors {
		collector.SingleFlight(key)
	}
}

func (m *CompositeMetrics) HitCount() int64 {
	if m.reader == nil {
		return 0
	}
	return m.reader.HitCount()
}

func (m *CompositeMetrics) MissCount() int64 {
	if m.reader == nil {
		return 0
	}
	return m.reader.MissCount()
}

func (m *CompositeMetrics) DBOpCount() int64 {
	if m.reader == nil {
		return 0
	}
	return m.reader.DBOpCount()
}

func (m *CompositeMetrics) DbFailCount() int64 {
	if m.reader == nil {
		return 0
	}
	return m.reader.DbFailCount()
}

func (m *CompositeMetrics) SingleFlightCount() int64 {
	if m.reader == nil {
		return 0
	}
	return m.reader.SingleFlightCount()
}

func (m *CompositeMetrics) HitRate() float64 {
	if m.reader == nil {
		return 0
	}
	return m.reader.HitRate()
}
