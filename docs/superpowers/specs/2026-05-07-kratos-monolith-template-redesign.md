# Kratos 单体应用模板改造设计

## 目标

完善现有单体应用模板的缺失部分，使其更加生产就绪，开发者可以直接基于此模板开始业务开发。

## 改造内容

### 1. 依赖注入链路重构

**问题：** `cmd/app/wire.go` 只包含 `server.ProviderSet` 和 `newApp`，缺少 db、cache、scheduler、业务模块的注入。`internal/pkg/cache`、`scheduler` 没有定义 `wire.ProviderSet`。

**方案：**
- `internal/pkg/cache/cache.go` — 新增 `ProviderSet = wire.NewSet(NewRedis)`
- `internal/pkg/scheduler/scheduler.go` — 新增 `ProviderSet = wire.NewSet(NewScheduler)`
- `cmd/app/wire.go` — 重写，聚合所有 ProviderSet：
  - `db.ProviderSet`、`cache.ProviderSet`、`scheduler.ProviderSet`
  - 模块 ProviderSet（如 `moduser.ProviderSet`）
  - `server.ProviderSet`、`newApp`
- 清理 `internal/pkg/app/app.go`（当前未被使用）

### 2. 模块路由注册

**问题：** `main.go` 中 `RegisterModuleRoutes` 函数不存在，模块路由未注册。

**方案：**
- 在 `cmd/app/main.go` 中定义 `RegisterModuleRoutes(srv *http.Server, uc *biz.UserUsecase, logger log.Logger, jwt *conf.Jwt)`
- 内部调用 `moduser.RegisterHTTP(srv, uc, logger, jwt)`
- 设计为可扩展，后续新增模块只需追加一行
- 在 `main()` 中取消注释并调用

### 3. Health Check 端点

**问题：** 缺少健康检查端点，容器探针和负载均衡无法使用。

**方案：**
- 在 `internal/server/http.go` 的 `NewHTTPServer` 中注册 `GET /health` 路由
- 返回 `{"status": "ok"}` + HTTP 200
- 已存在于 `configs/config.yaml` 的 `auth.whitelist` 中

### 4. Dockerfile Go 版本修复

**问题：** Dockerfile 使用 `golang:1.22`，但 `go.mod` 声明 `go 1.25.0`。

**方案：** 改为 `golang:1.25`。

### 5. Makefile 修复

**问题：** `migrate-up` 和 `migrate-down` 中的变量替换逻辑错误（多次 `go env` 嵌套）。

**方案：** 简化为直接使用配置值，或改用 `go run` 方式执行 migrate 命令。

## 不涉及的范围

- 密码 hash（Login 仅为测试演示，保持原样）
- 新增业务模块（如 order）
- CI/CD 流水线
- 测试框架新增
