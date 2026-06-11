package db

import (
	"time"

	"github.com/go-kratos/kratos-layout-monolith/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/plugin/opentelemetry/tracing"
)

// ProviderSet is db providers.
var ProviderSet = wire.NewSet(NewDatabase)

// NewDatabase creates a new database connection and returns the *gorm.DB instance.
func NewDatabase(c *conf.Database, logger log.Logger) (*gorm.DB, func(), error) {
	if c == nil {
		log.NewHelper(logger).Warn("database config is nil, using nil DB")
		cleanup := func() {}
		return nil, cleanup, nil
	}

	maxOpenConns := c.MaxOpenConns
	if maxOpenConns == 0 {
		maxOpenConns = 100
	}
	maxIdleConns := c.MaxIdleConns
	if maxIdleConns == 0 {
		maxIdleConns = 20
	}

	helper := log.NewHelper(log.With(logger, "component", "gorm"))

	gormLogger := gormlogger.New(
		&gormLogWriter{helper: helper},
		gormlogger.Config{
			SlowThreshold:             200 * time.Millisecond,
			Colorful:                  false,
			IgnoreRecordNotFoundError: true,
			LogLevel:                  gormlogger.Warn,
		},
	)

	d, err := gorm.Open(mysql.Open(c.Source), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, nil, err
	}
	if err := d.Use(tracing.NewPlugin(tracing.WithoutMetrics())); err != nil {
		helper.Warnf("gorm tracing plugin setup failed: %v", err)
	}

	sqlDB, err := d.DB()
	if err != nil {
		return nil, nil, err
	}
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)

	cleanup := func() {
		sqlDB.Close()
	}

	helper.Infof("database connected (driver: %s)", c.Driver)
	return d, cleanup, nil
}

type gormLogWriter struct {
	helper *log.Helper
}

func (w *gormLogWriter) Printf(format string, v ...interface{}) {
	w.helper.Infof(format, v...)
}
