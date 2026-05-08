# Kratos 单体应用模板改造实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完善 Wire 依赖注入链路、修复路由注册、添加 health check、修复 Dockerfile/Makefile，使模板达到生产就绪状态。

**Architecture:** 重构 wire 注入链，建立 Config → DB/Cache/Scheduler → Module → Server → App 完整依赖图；在 http.go 添加 health handler；修复构建/迁移配置。

**Tech Stack:** go-kratos/v2, google/wire, GORM, go-redis, go-playground/validator

---

### Task 1: 为 pkg 组件添加 Wire ProviderSet

**Files:**
- Modify: `internal/pkg/cache/cache.go:1-52`
- Modify: `internal/pkg/scheduler/scheduler.go:1-50`

- [ ] **Step 1: 修改 cache.go，添加 ProviderSet**

在 `internal/pkg/cache/cache.go` 文件开头 import 中添加 `github.com/google/wire`，在 `Redis` 类型定义之前添加：

```go
// ProviderSet is cache providers.
var ProviderSet = wire.NewSet(NewRedis)
```

完整文件内容：

```go
package cache

import (
	"github.com/go-kratos/kratos-layout-monolith/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-redis/redis/v8"
	"github.com/google/wire"
)

// ProviderSet is cache providers.
var ProviderSet = wire.NewSet(NewRedis)

// Redis wraps a Redis client.
type Redis struct {
	Client *redis.Client
}

// NewRedis creates a new Redis client.
func NewRedis(c *conf.Redis, logger log.Logger) *Redis {
	if c == nil {
		log.NewHelper(logger).Warn("redis config is nil, skipping redis initialization")
		return &Redis{}
	}

	network := c.Network
	if network == "" {
		network = "tcp"
	}

	client := redis.NewClient(&redis.Options{
		Network:      network,
		Addr:         c.Addr,
		Password:     c.Password,
		DB:           c.DB,
		ReadTimeout:  c.ReadTimeout,
		WriteTimeout: c.WriteTimeout,
	})

	helper := log.NewHelper(log.With(logger, "component", "redis"))
	if err := client.Ping(nil).Err(); err != nil {
		helper.Warnf("redis ping failed: %v", err)
	} else {
		helper.Infof("redis connected at %s", c.Addr)
	}

	return &Redis{Client: client}
}

// Close closes the Redis client.
func (r *Redis) Close() error {
	if r.Client != nil {
		return r.Client.Close()
	}
	return nil
}
```

- [ ] **Step 2: 修改 scheduler.go，添加 ProviderSet**

完整文件内容：

```go
package scheduler

import (
	"github.com/robfig/cron/v3"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// ProviderSet is scheduler providers.
var ProviderSet = wire.NewSet(NewScheduler)

// Scheduler wraps a cron scheduler for managing scheduled jobs.
type Scheduler struct {
	cron   *cron.Cron
	logger *log.Helper
}

// NewScheduler creates a new scheduler.
func NewScheduler(logger log.Logger) *Scheduler {
	return &Scheduler{
		cron:   cron.New(cron.WithSeconds()),
		logger: log.NewHelper(logger),
	}
}

// AddJob adds a cron job.
// spec is a cron expression (e.g., "@every 1h", "0 0 * * *").
func (s *Scheduler) AddJob(spec string, cmd func()) error {
	id, err := s.cron.AddFunc(spec, cmd)
	if err != nil {
		return err
	}
	s.logger.Infof("scheduled job added: %s (id: %d)", spec, id)
	return nil
}

// Start starts the scheduler.
func (s *Scheduler) Start() {
	s.cron.Start()
	s.logger.Info("scheduler started")
}

// Stop stops the scheduler and waits for running jobs to complete.
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	s.logger.Info("scheduler stopped")
}

// Entries returns all scheduled entries.
func (s *Scheduler) Entries() []cron.Entry {
	return s.cron.Entries()
}
```

---

### Task 2: 添加 Health Check 端点

**Files:**
- Modify: `internal/server/http.go:1-54`

- [ ] **Step 1: 修改 http.go，添加 health handler 和路由注册**

完整文件内容（在 NewHTTPServer 返回 srv 之前，添加 health 路由注册）：

```go
package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-kratos/kratos-layout-monolith/internal/conf"
	"github.com/go-kratos/kratos-layout-monolith/internal/middleware/auth"
	"github.com/go-kratos/kratos-layout-monolith/internal/middleware/cors"
	"github.com/go-kratos/kratos-layout-monolith/internal/middleware/ratelimit"
	"github.com/go-kratos/kratos-layout-monolith/internal/middleware/requestid"
	"github.com/go-kratos/kratos-layout-monolith/internal/middleware/validator"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/google/wire"
)

// ProviderSet is server providers.
var ProviderSet = wire.NewSet(NewHTTPServer)

// NewHTTPServer creates a new HTTP server with common middleware.
func NewHTTPServer(
	c *conf.Server,
	ac *conf.Auth,
	jwt *conf.Jwt,
	logger log.Logger,
) (*kratoshttp.Server, error) {
	opts := []kratoshttp.ServerOption{
		kratoshttp.Middleware(
			recovery.Recovery(),
			requestid.Server(),
			cors.Server(),
			ratelimit.Server(),
			auth.Server(ac, jwt.Secret),
			validator.Server(),
		),
	}
	if c.HTTP != nil {
		if c.HTTP.Network != "" {
			opts = append(opts, kratoshttp.Network(c.HTTP.Network))
		}
		if c.HTTP.Addr != "" {
			opts = append(opts, kratoshttp.Address(c.HTTP.Addr))
		}
		if c.HTTP.Timeout != 0 {
			opts = append(opts, kratoshttp.Timeout(c.HTTP.Timeout))
		}
	}

	srv := kratoshttp.NewServer(opts...)

	// Register health check endpoint
	srv.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	log.NewHelper(logger).Infof("HTTP server listening on %s", c.HTTP.Addr)

	return srv, nil
}
```

关键变化：
- 添加 `encoding/json` 和 `net/http` import
- 使用 `kratoshttp` 别名避免与 `net/http` 冲突
- 在 `srv` 上调用 `HandleFunc` 注册 `/health`
- health 端点不走 kratos middleware 链，直接响应

---

### Task 3: 重写 Wire 注入配置和 main.go

**Files:**
- Modify: `cmd/app/wire.go:1-23`
- Modify: `cmd/app/main.go:1-88`

- [ ] **Step 1: 重写 cmd/app/wire.go**

完整文件内容：

```go
//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"github.com/go-kratos/kratos-layout-monolith/internal/conf"
	"github.com/go-kratos/kratos-layout-monolith/internal/moduser"
	"github.com/go-kratos/kratos-layout-monolith/internal/pkg/cache"
	"github.com/go-kratos/kratos-layout-monolith/internal/pkg/db"
	"github.com/go-kratos/kratos-layout-monolith/internal/server"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// initApp init kratos application.
func initApp(*conf.Bootstrap, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(
		db.ProviderSet,
		cache.ProviderSet,
		moduser.ProviderSet,
		server.ProviderSet,
		newApp,
	))
}
```

关键变化：
- 导入 db、cache、moduser 包
- `wire.Build` 聚合所有 ProviderSet
- 不包含 scheduler（scheduler 在 main.go 中手动 Start/Stop）

- [ ] **Step 2: 重写 cmd/app/main.go**

完整文件内容：

```go
package main

import (
	"flag"
	"os"

	"github.com/go-kratos/kratos-layout-monolith/internal/conf"
	"github.com/go-kratos/kratos-layout-monolith/internal/moduser/biz"
	kratoslog "github.com/go-kratos/kratos/v2/log"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/transport/http"

	_ "go.uber.org/automaxprocs"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	// Name is the name of the compiled software.
	Name string
	// Version is the version of the compiled software.
	Version string
	// flagconf is the config flag.
	flagconf string

	id, _ = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "./configs/config.yaml", "config path, eg: -conf config.yaml")
}

func newApp(logger kratoslog.Logger, hs *http.Server) *kratos.App {
	return kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Logger(logger),
		kratos.Server(hs),
	)
}

func RegisterModuleRoutes(srv *http.Server, uc *biz.UserUsecase, logger kratoslog.Logger, jwt *conf.Jwt) {
	moduser.RegisterHTTP(srv, uc, logger, jwt)
}

func main() {
	flag.Parse()

	// Create base logger
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

	app, cleanup, err := initApp(&bc, baseLogger)
	if err != nil {
		logHelper.Fatalf("failed to init app: %v", err)
	}
	defer cleanup()

	// Register module routes after wire init
	srv := bc.Server
	_ = srv // config available if needed for route registration
	// Note: wire-generated initApp already wires the HTTP server.
	// Modules register routes via the http.Server passed through wire.

	// start and wait for stop signal
	if err := app.Run(); err != nil {
		logHelper.Fatalf("app run error: %v", err)
	}
}
```

等等，这样写有问题。`RegisterModuleRoutes` 需要拿到 `*http.Server` 和 `*biz.UserUsecase`，但这些是 wire 内部构造的，main.go 拿不到。

正确方案是让 wire 的 `initApp` 返回更多信息。重新设计 `newApp` 和 `initApp` 的签名：

**修正后的 main.go**：

```go
package main

import (
	"flag"
	"os"

	"github.com/go-kratos/kratos-layout-monolith/internal/conf"
	"github.com/go-kratos/kratos-layout-monolith/internal/moduser"
	"github.com/go-kratos/kratos-layout-monolith/internal/moduser/biz"
	kratoslog "github.com/go-kratos/kratos/v2/log"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/transport/http"

	_ "go.uber.org/automaxprocs"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	Name    string
	Version string
	flagconf string
	id, _   = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "./configs/config.yaml", "config path, eg: -conf config.yaml")
}

// appComponents holds everything wire produces, so main.go can use them.
type appComponents struct {
	app         *kratos.App
	httpServer  *http.Server
	userUsecase *biz.UserUsecase
}

func newAppComponents(
	logger kratoslog.Logger,
	hs *http.Server,
	uc *biz.UserUsecase,
) *appComponents {
	app := kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Logger(logger),
		kratos.Server(hs),
	)
	return &appComponents{
		app:         app,
		httpServer:  hs,
		userUsecase: uc,
	}
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
	moduser.RegisterHTTP(components.httpServer, components.userUsecase, baseLogger, bc.Jwt)

	if err := components.app.Run(); err != nil {
		logHelper.Fatalf("app run error: %v", err)
	}
}
```

**修正后的 wire.go**：

```go
//go:build wireinject
// +build wireinject

package main

import (
	"github.com/go-kratos/kratos-layout-monolith/internal/conf"
	"github.com/go-kratos/kratos-layout-monolith/internal/moduser"
	"github.com/go-kratos/kratos-layout-monolith/internal/moduser/biz"
	"github.com/go-kratos/kratos-layout-monolith/internal/pkg/cache"
	"github.com/go-kratos/kratos-layout-monolith/internal/pkg/db"
	"github.com/go-kratos/kratos-layout-monolith/internal/server"

	kratoslog "github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/google/wire"
)

func initApp(*conf.Bootstrap, kratoslog.Logger) (*appComponents, func(), error) {
	panic(wire.Build(
		db.ProviderSet,
		cache.ProviderSet,
		moduser.ProviderSet,
		server.ProviderSet,
		wire.NewSet(newAppComponents),
	))
}
```

但这有个问题：`wire.NewSet(newAppComponents)` 返回 `*appComponents` 而不是 `(*kratos.App, func(), error)`。Wire 需要返回 cleanup function。

最佳方案：让 `newAppComponents` 返回 `(*appComponents, func(), error)`：

**最终修正后的 main.go**：

```go
package main

import (
	"flag"
	"os"

	"github.com/go-kratos/kratos-layout-monolith/internal/conf"
	"github.com/go-kratos/kratos-layout-monolith/internal/moduser"
	"github.com/go-kratos/kratos-layout-monolith/internal/moduser/biz"
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
	moduser.RegisterHTTP(components.httpServer, components.userUsecase, baseLogger, bc.Jwt)

	if err := components.app.Run(); err != nil {
		logHelper.Fatalf("app run error: %v", err)
	}
}
```

**最终修正后的 wire.go**：

```go
//go:build wireinject
// +build wireinject

package main

import (
	"github.com/go-kratos/kratos-layout-monolith/internal/conf"
	"github.com/go-kratos/kratos-layout-monolith/internal/moduser"
	"github.com/go-kratos/kratos-layout-monolith/internal/pkg/cache"
	"github.com/go-kratos/kratos-layout-monolith/internal/pkg/db"
	"github.com/go-kratos/kratos-layout-monolith/internal/server"

	kratoslog "github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

func initApp(*conf.Bootstrap, kratoslog.Logger) (*appComponents, func(), error) {
	panic(wire.Build(
		db.ProviderSet,
		cache.ProviderSet,
		moduser.ProviderSet,
		server.ProviderSet,
		newAppComponents,
	))
}
```

这应该能工作。Wire 自动解析：
1. `db.NewDatabase` → `*gorm.DB` + cleanup (wire 自动处理 cleanup)
2. `cache.NewRedis` → `*cache.Redis`
3. `moduser.ProviderSet` → `biz.UserUsecase` (需要 `UserRepo` + logger)
4. `server.NewHTTPServer` → `*http.Server` (需要 Server/Auth/Jwt config + logger)
5. `newAppComponents` → `*appComponents` + cleanup (需要 logger + http.Server + UserUsecase)

- [ ] **Step 3: 修改 moduser.RegisterHTTP 签名**

修改 `internal/moduser/wire.go` 中的 `RegisterHTTP` 函数，改为使用 Wire 注入的 `*service.UserService` 而不是重新创建。

完整文件内容：

```go
//go:build wireinject
// +build wireinject

package moduser

import (
	"github.com/go-kratos/kratos-layout-monolith/internal/conf"
	"github.com/go-kratos/kratos-layout-monolith/internal/moduser/biz"
	"github.com/go-kratos/kratos-layout-monolith/internal/moduser/data"
	"github.com/go-kratos/kratos-layout-monolith/internal/moduser/service"

	v1 "github.com/go-kratos/kratos-layout-monolith/api/user/v1"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/google/wire"
)

// ProviderSet provides all dependencies for the user module.
var ProviderSet = wire.NewSet(
	biz.ProviderSet,
	data.ProviderSet,
	service.NewUserService,
)

// RegisterHTTP registers the user module's HTTP routes on the given server.
// Call this from main.go after wire initialization.
func RegisterHTTP(
	s *http.Server,
	uc *biz.UserUsecase,
	logger log.Logger,
	jwt *conf.Jwt,
) {
	srv := service.NewUserService(uc, jwt.Secret, jwt.Expire)
	v1.RegisterUserServiceHTTPServer(s, srv)
	log.NewHelper(logger).Infof("[moduser] routes registered")
}
```

注意：保持现有 `RegisterHTTP` 不变（它内部调用 `service.NewUserService`），wire.go 的 `ProviderSet` 包含 `service.NewUserService` 是因为 biz 层的 usecase 不依赖 service，所以 wire 不会自动创建 service。

实际上等一下 — 再看一下依赖链。`moduser.ProviderSet` 中有 `service.NewUserService`，但 `newAppComponents` 并不依赖 `*service.UserService`，所以 wire 不会创建它。只有 `RegisterHTTP`（非 wire 函数）才需要它。所以 `service.NewUserService` 可以从 ProviderSet 中移除，或者保留也没害处（wire 会忽略未使用的 provider）。

保持 `service.NewUserService` 在 ProviderSet 中（无害，保持一致性）。

---

### Task 4: 修复 Dockerfile Go 版本

**Files:**
- Modify: `Dockerfile:1`

- [ ] **Step 1: 修改 Dockerfile builder 镜像版本**

将第一行从 `golang:1.22-alpine` 改为 `golang:1.25-alpine`，与 `go.mod` 保持一致。

完整文件内容：

```dockerfile
FROM golang:1.25-alpine AS builder

COPY . /src
WORKDIR /src

RUN GOPROXY=https://goproxy.cn make build

FROM alpine:latest

RUN apk add --no-cache ca-certificates

COPY --from=builder /src/bin /app
COPY --from=builder /src/configs /app/configs
COPY --from=builder /src/migrations /app/migrations

WORKDIR /app

EXPOSE 8000

CMD ["./app", "-conf", "/app/configs/config.yaml"]
```

---

### Task 5: 修复 Makefile migrate 命令

**Files:**
- Modify: `Makefile:69-80`

- [ ] **Step 1: 修复 migrate-up 和 migrate-down 命令**

将这两行：
```makefile
migrate-up:
	migrate -database $(shell go env go env go env go env go env go env | grep "source" | sed 's/.*source: //') -path ./migrations up

migrate-down:
	migrate -database $(shell go env | grep "source" | sed 's/.*source: //') -path ./migrations down 1
```

替换为（使用 yq 解析 YAML，或直接使用配置值）：

```makefile
migrate-up:
	@DB_URL=$$(grep 'source:' configs/config.yaml | sed 's/.*source: *//' | tr -d '"' | head -1); \
	if [ -z "$$DB_URL" ]; then echo "ERROR: could not find database source in configs/config.yaml"; exit 1; fi; \
	echo "Running migrations up with: $$DB_URL"; \
	migrate -path ./migrations -database "$$DB_URL" up

migrate-down:
	@DB_URL=$$(grep 'source:' configs/config.yaml | sed 's/.*source: *//' | tr -d '"' | head -1); \
	if [ -z "$$DB_URL" ]; then echo "ERROR: could not find database source in configs/config.yaml"; exit 1; fi; \
	echo "Running migrations down with: $$DB_URL"; \
	migrate -path ./migrations -database "$$DB_URL" down 1

migrate-create:
	migrate create -ext sql -dir ./migrations -seq $(name)
```

---

### Task 6: 验证构建

**Files:**
- N/A (verification only)

- [ ] **Step 1: 运行 wire 生成代码**

```bash
cd /home/Ramon/myspace/kratos-layout-monolith && make generate
```

Expected: wire 生成 `cmd/app/wire_gen.go` 和 `internal/moduser/wire_gen.go`，无错误。

- [ ] **Step 2: 验证编译通过**

```bash
cd /home/Ramon/myspace/kratos-layout-monolith && go build ./...
```

Expected: 无编译错误。

- [ ] **Step 3: 运行测试**

```bash
cd /home/Ramon/myspace/kratos-layout-monolith && go test ./...
```

Expected: 现有测试通过（validator_test.go）。
