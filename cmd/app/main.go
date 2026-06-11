package main

import (
	"context"
	"flag"
	"os"
	"time"

	"github.com/go-kratos/kratos-layout-monolith/internal/conf"
	"github.com/go-kratos/kratos-layout-monolith/internal/model/user"
	"github.com/go-kratos/kratos-layout-monolith/internal/model/user/biz"
	internallogger "github.com/go-kratos/kratos-layout-monolith/internal/pkg/logger"
	internaltracing "github.com/go-kratos/kratos-layout-monolith/internal/pkg/tracing"

	kratoslog "github.com/go-kratos/kratos/v2/log"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/transport/http"

	_ "go.uber.org/automaxprocs"
)

var (
	Name     string
	Version  string
	flagconf string
	id, _    = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "./configs/config.yaml", "config path, eg: -conf config.yaml")
}

type appComponents struct {
	app         *kratos.App
	httpServer  *http.Server
	userUsecase *biz.UserUsecase
}

func newAppComponents(
	logger kratoslog.Logger,
	hs *http.Server,
	uc *biz.UserUsecase,
) (*appComponents, func(), error) {
	app := kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Logger(logger),
		kratos.Server(hs),
	)
	cleanup := func() {
		app.Stop()
	}
	return &appComponents{
		app:         app,
		httpServer:  hs,
		userUsecase: uc,
	}, cleanup, nil
}

func main() {
	flag.Parse()

	baseLogger := internallogger.WithService(internallogger.NewLogger(), id, Name, Version)
	logHelper := kratoslog.NewHelper(baseLogger)

	c := config.New(
		config.WithSource(
			file.NewSource(flagconf),
		),
	)
	defer c.Close()

	if err := c.Load(); err != nil {
		logHelper.Fatalf("failed to load config: %v", err)
	}

	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		logHelper.Fatalf("failed to scan config: %v", err)
	}

	traceShutdown, err := internaltracing.Init(context.Background(), bc.Tracing, Name, Version, id)
	if err != nil {
		logHelper.Fatalf("failed to init tracing: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := traceShutdown(ctx); err != nil {
			logHelper.Errorf("failed to shutdown tracing: %v", err)
		}
	}()

	components, cleanup, err := initApp(&bc, baseLogger)
	if err != nil {
		logHelper.Fatalf("failed to init app: %v", err)
	}
	defer cleanup()

	// Register module routes
	user.RegisterHTTP(components.httpServer, components.userUsecase, baseLogger, bc.Jwt)

	if err := components.app.Run(); err != nil {
		logHelper.Fatalf("app run error: %v", err)
	}
}
