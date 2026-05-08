# Kratos 单体应用模板

基于 [go-kratos/kratos](https://github.com/go-kratos/kratos) 的单体应用项目模板。

与官方 `kratos-layout`（面向微服务）不同，本模板专为单体应用设计：

| 特性 | 微服务 (kratos-layout) | 单体 (本模板) |
|------|------|------|
| 模块组织 | 单一 service | 多 bounded context (moduser, modorder...) |
| 服务间通信 | gRPC + 服务发现 | 直接 Go 函数调用 |
| 暴露端口 | HTTP + gRPC | 仅 HTTP |
| 服务发现 | etcd/consul | 不需要 |
| 数据库 | 每个服务独立 | 共享数据库 |
| 定时任务 | 无 | 内置 cron scheduler |
| 中间件 | 基础 recovery | auth, CORS, rate limit, request_id |
| 数据库迁移 | 无 | golang-migrate 集成 |

## 目录结构

```
├── api/                        # API 协议定义 (按业务域)
│   └── user/v1/
│       ├── user.proto          # UserService 定义
│       └── error_reason.proto  # 错误码
├── cmd/app/                    # 唯一入口
│   ├── main.go                 # 应用启动
│   ├── wire.go                 # Wire 注入定义
│   └── wire_gen.go             # Wire 生成代码
├── configs/
│   └── config.yaml             # 应用配置
├── internal/
│   ├── server/
│   │   └── http.go             # HTTP server + 公共中间件
│   ├── middleware/              # 中间件层
│   │   ├── auth/               # JWT 认证
│   │   ├── cors/               # CORS
│   │   ├── ratelimit/          # 限流
│   │   └── requestid/          # RequestID
│   ├── pkg/                    # 共享基础设施
│   │   ├── app/                # 应用生命周期
│   │   ├── db/                 # GORM 数据库
│   │   ├── cache/              # Redis 缓存
│   │   ├── logger/             # 日志
│   │   └── scheduler/          # 定时任务
│   ├── conf/
│   │   └── conf.go             # 配置结构体
│   └── moduser/                # 用户模块 (按业务域命名)
│       ├── data/               # 数据访问层
│       │   ├── data.go         # 模型定义 (gorm)
│       │   └── user.go         # 仓库实现
│       ├── biz/                # 业务逻辑层
│       │   ├── biz.go          # ProviderSet
│       │   └── user.go         # 领域模型 + UseCase
│       ├── service/
│       │   └── user.go         # HTTP handler 实现
│       ├── wire.go             # Wire 注入定义
│       └── wire_gen.go         # Wire 生成代码
├── migrations/                 # 数据库迁移脚本
│   ├── 000001_create_users_table.up.sql
│   └── 000001_create_users_table.down.sql
├── third_party/                # Proto 依赖
├── Makefile
├── Dockerfile
├── go.mod
└── openapi.yaml                # API 文档 (自动生成)
```

## 快速开始

### 1. 环境准备

```bash
# 安装开发工具
make init
```

需要安装:
- Go 1.22+
- protoc (Protocol Buffers 编译器)
- MySQL / PostgreSQL
- Redis

### 2. 创建项目

```bash
# 方式1: 使用此模板作为起点
# 复制模板代码到你的项目目录

# 方式2: 如果注册为 kratos template (未来)
kratos new myapp -r https://github.com/go-kratos/kratos-layout-monolith
```

### 3. 配置

编辑 `configs/config.yaml`:

```yaml
server:
  http:
    addr: 0.0.0.0:8000
    timeout: 10s

data:
  database:
    driver: mysql
    source: root:password@tcp(127.0.0.1:3306)/myapp?parseTime=True&loc=Local
  redis:
    addr: 127.0.0.1:6379

jwt:
  secret: "your-secret-key"
  expire: 7200
```

### 4. 数据库迁移

```bash
# 执行 up 迁移
make migrate-up

# 回滚迁移
make migrate-down

# 创建新的迁移文件
make migrate-create name=add_orders_table
```

### 5. 运行

```bash
# 开发模式运行
make run

# 编译
make build

# 运行编译后的程序
./bin/app -conf ./configs/config.yaml
```

## 添加新模块

### 1. 创建模块目录

```bash
mkdir -p internal/modorder/{data,biz,service}
mkdir -p api/order/v1
```

### 2. 定义 API (proto)

在 `api/order/v1/order.proto` 中定义服务:

```protobuf
syntax = "proto3";
package order.v1;

import "google/api/annotations.proto";

option go_package = "github.com/go-kratos/kratos-layout-monolith/api/order/v1;v1";

service OrderService {
  rpc CreateOrder (CreateOrderRequest) returns (CreateOrderReply) {
    option (google.api.http) = {
      post: "/api/order/v1/orders"
      body: "*"
    };
  }
}
```

### 3. 实现模块

按照 DDD 三层结构实现:
- `data/` - 数据访问 (GORM model + repo)
- `biz/` - 业务逻辑 (domain model + usecase)
- `service/` - HTTP handler

### 4. 注册模块

在 `cmd/app/wire_gen.go` 的 `RegisterModuleRoutes` 中添加:

```go
func RegisterModuleRoutes(srv *http.Server, bootstrap *conf.Bootstrap) {
    // 添加你的模块
    modorder.RegisterHTTP(srv, bootstrap)
}
```

### 5. 生成代码

```bash
make all
```

## 技术栈

| 组件 | 技术选型 |
|------|------|
| 框架 | [go-kratos](https://github.com/go-kratos/kratos) |
| 数据库 ORM | [GORM](https://gorm.io/) |
| 缓存 | [go-redis](https://github.com/go-redis/redis) |
| 认证 | [golang-jwt](https://github.com/golang-jwt/jwt) |
| 数据库迁移 | [golang-migrate](https://github.com/golang-migrate/migrate) |
| 定时任务 | [robfig/cron](https://github.com/robfig/cron) |
| 依赖注入 | [Wire](https://github.com/google/wire) |

## License

MIT
