package main

import (
	"flag"
	"os"

	"github.com/go-kratos/kratos-layout-monolith/internal/conf"
	"github.com/go-kratos/kratos-layout-monolith/internal/model/user"
	"github.com/go-kratos/kratos-layout-monolith/internal/model/user/biz"
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

	baseLogger := kratoslog.NewStdLogger(os.Stdout)
	logHelper := kratoslog.NewHelper(kratoslog.With(
		baseLogger,
		"ts", kratoslog.DefaultTimestamp,
		"caller", kratoslog.DefaultCaller,
		"service.id", id,
		"service.name", Name,
		"service.version", Version,
	))

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
