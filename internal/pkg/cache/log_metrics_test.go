package cache

import (
	"bytes"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
)

func TestLogMetricsOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := log.NewHelper(log.NewStdLogger(&buf))
	lm := NewLogMetrics(log.NewStdLogger(&buf))

	lm.Hit("user:1")
	lm.Miss("user:2")
	lm.DbFail("user:3")

	_ = logger

	output := buf.String()
	if output == "" {
		t.Error("expected log output")
	}
}
