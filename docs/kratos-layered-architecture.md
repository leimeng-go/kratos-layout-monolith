# Kratos 三层架构与领域模型设计

## 概览

Kratos 采用 **Service → Biz → Data** 三层分层架构，每层职责明确、依赖单向，通过接口实现解耦。

```
┌─────────────────────────────────────────────┐
│                  Service                     │
│          (HTTP / gRPC 协议适配层)             │
├─────────────────────────────────────────────┤
│                    Biz                       │
│            (核心业务逻辑层)                   │
├─────────────────────────────────────────────┤
│                   Data                       │
│            (数据存储实现层)                   │
└─────────────────────────────────────────────┘
```

## 为什么要分层？

核心原因：**每一层变化的频率和原因不同**。

| 变化因素 | 影响层 | 示例 |
|---------|--------|------|
| 传输协议变更 | Service | HTTP → gRPC |
| 业务规则调整 | Biz | 注册时增加邮箱验证 |
| 存储方案替换 | Data | MySQL → PostgreSQL |

分层后，改一层不需要动其他两层。

## 各层职责

### Service 层 — 协议适配

**目录：** `internal/model/user/service/`

职责：
- 接收 HTTP / gRPC 请求，返回响应
- 请求参数的转换和校验
- 调用 Biz 层处理业务
- Token 生成、中间件处理等

```go
type UserService struct {
    v1.UnimplementedUserServiceServer
    uc     *biz.UserUsecase
    secret string
    expire int64
}

func (s *UserService) Login(ctx context.Context, req *v1.LoginRequest) (*v1.LoginReply, error) {
    // 1. 调用 biz 层执行业务逻辑
    user, err := s.uc.Login(ctx, req.Username, req.Password)
    if err != nil {
        return nil, err
    }
    // 2. 生成 token，组装响应
    token, _ := auth.GenerateToken(s.secret, user.Id, user.Username, s.expire)
    return &v1.LoginReply{Token: token, User: toPBUser(user)}, nil
}
```

Service 层不包含任何业务判断逻辑，只做"翻译"：把协议层的数据结构翻译成 Biz 层的领域对象。

### Biz 层 — 业务逻辑

**目录：** `internal/model/user/biz/`

职责：
- 定义领域模型（`User` 结构体）
- 定义仓储接口（`UserRepo`）
- 实现业务规则（注册校验、登录验证等）
- 不依赖任何框架、数据库、协议

```go
// 领域模型
type User struct {
    Id        int64
    Username  string
    Password  string
    Email     string
    // ...
}

// 仓储接口 —— 只定义"需要什么能力"，不关心"怎么实现"
type UserRepo interface {
    CreateUser(context.Context, *User) (*User, error)
    GetUserByID(context.Context, int64) (*User, error)
    GetUserByUsername(context.Context, string) (*User, error)
    // ...
}

// 业务逻辑
func (uc *UserUsecase) Register(ctx context.Context, u *User) (*User, error) {
    existing, _ := uc.repo.GetUserByUsername(ctx, u.Username)
    if existing != nil {
        return nil, ErrUsernameExists // 业务规则：用户名不能重复
    }
    return uc.repo.CreateUser(ctx, u)
}
```

Biz 层是整个架构的核心，它通过接口依赖 Data 层，而不是直接依赖具体实现。

### Data 层 — 数据存储

**目录：** `internal/model/user/data/`

职责：
- 实现 `biz.UserRepo` 接口
- 处理数据库操作（GORM / SQL）
- 处理缓存逻辑（Redis）
- 数据模型与领域模型的转换

```go
type userRepo struct {
    db    *gorm.DB
    redis *cache.Redis
    log   *log.Helper
}

func (r *userRepo) GetUserByID(ctx context.Context, id int64) (*biz.User, error) {
    var user User
    err := r.redis.Take(ctx, key, &user, ttl, func() error {
        return r.db.WithContext(ctx).First(&user, id).Error // 缓存未命中，查数据库
    })
    if err != nil {
        return nil, err
    }
    return toBizUser(&user), nil // GORM 模型 → 领域模型
}
```

Data 层封装了所有存储细节，对上层完全透明。

## 依赖关系：依赖倒置

```
Service  ──→  Biz  ←──  Data
               ↑
         定义 UserRepo 接口
                       ↑
                 Data 实现该接口
```

**关键设计：** 依赖方向是 `Data → Biz`，而不是 `Biz → Data`。

- Biz 层定义 `UserRepo` 接口
- Data 层实现 `UserRepo` 接口
- Biz 层不知道 Data 层的存在

这样做的好处：
1. Biz 层可以独立做单元测试，mock 一个假 repo
2. 替换存储方案（MySQL → MongoDB）只需改 Data 层
3. 业务逻辑不会被存储框架（GORM）的细节污染

## 领域模型设计（DDD）

Kratos 的三层架构与 DDD（领域驱动设计）高度契合。Biz 层本质上就是 DDD 中的**领域层**，是整个系统的核心。

### DDD 概念映射

```
DDD 概念              →  Kratos 实现
─────────────────────────────────────────────
Bounded Context        →  internal/model/user/  （用户限界上下文）
Entity (实体)          →  biz.User              （有唯一标识的领域对象）
Repository (仓储)      →  biz.UserRepo 接口      （定义在领域层）
                        data.userRepo 实现       （实现在数据层）
Application Service    →  biz.UserUsecase        （编排用例，协调领域对象）
Ubiquitous Language    →  biz 层的命名            （Register、Login 等业务术语）
Anti-Corruption Layer  →  toBizUser / toPBUser   （模型转换，隔离不同层）
```

### 限界上下文（Bounded Context）

每个业务模块是一个独立的限界上下文，拥有自己的领域模型、业务规则和存储实现。模块之间通过接口交互，不共享内部实现。

```
┌──────────────────────┐     ┌──────────────────────┐
│    User Context       │     │    Order Context      │
│  (用户限界上下文)      │     │  (订单限界上下文)      │
│                       │     │                       │
│  biz.User             │     │  biz.Order            │
│  biz.UserUsecase      │     │  biz.OrderUsecase     │
│  biz.UserRepo         │     │  biz.OrderRepo        │
│  data.userRepo        │     │  data.orderRepo       │
└──────────────────────┘     └──────────────────────┘
```

不同上下文中的 `User` 可以是不同的模型。订单上下文中的"用户"可能只需要 `UserId` 和 `Username`，不需要完整的用户信息。这就是 DDD 所说的"同一个概念在不同上下文中含义不同"。

### 实体（Entity）

实体是具有**唯一标识**的领域对象，通过标识区分，而不是通过属性值。

```go
// biz/user.go — 用户实体
type User struct {
    Id        int64   // 唯一标识，决定实体身份
    Username  string  // 业务属性
    Password  string
    Email     string
    Phone     string
    Nickname  string
    Avatar    string
    Status    int32
    CreatedAt string
    UpdatedAt string
}
```

实体的关键特征：
- **有唯一标识**（`Id`）：两个 `Username` 相同的 User 是不同的实体
- **有生命周期**：创建 → 更新 → 删除
- **属于领域层**：不携带任何 GORM tag、JSON tag 等技术细节

### 仓储（Repository）

仓储是领域对象的**持久化抽象**，让业务逻辑不需要关心数据存在哪里、怎么存。

```
┌─────────────────────────────────────────────────────┐
│                   biz 层 (领域层)                     │
│                                                      │
│  type UserRepo interface {                           │
│      CreateUser(ctx, *User) (*User, error)           │
│      GetUserByID(ctx, int64) (*User, error)          │
│      GetUserByUsername(ctx, string) (*User, error)   │
│      ListUsers(ctx, int32, int32) ([]*User, int32)   │
│      UpdateUser(ctx, *User) (*User, error)           │
│      DeleteUser(ctx, int64) error                    │
│  }                                                   │
│                       ↑                               │
│                       │ 实现                           │
├───────────────────────┼─────────────────────────────┤
│                   data 层 (基础设施层)                 │
│                                                      │
│  type userRepo struct {                              │
│      db    *gorm.DB                                  │
│      redis *cache.Redis                              │
│  }                                                   │
│                                                      │
│  func (r *userRepo) GetUserByID(...) {               │
│      // 缓存 + 数据库的具体实现                        │
│  }                                                   │
└─────────────────────────────────────────────────────┘
```

DDD 仓储原则在本项目中的体现：
1. **接口定义在领域层**：`UserRepo` 在 `biz/` 中，用业务语言描述能力
2. **实现在基础设施层**：`userRepo` 在 `data/` 中，处理 GORM + Redis 细节
3. **仓储操作实体**：入参和返回值都是 `biz.User`，不暴露数据模型

### 应用服务（Application Service）

`UserUsecase` 对应 DDD 中的应用服务，负责**编排业务流程**，协调领域对象和仓储完成用例。

```go
type UserUsecase struct {
    repo UserRepo       // 仓储依赖（接口注入）
    log  *log.Helper    // 日志
}

// Register 注册用例：编排"查重 → 创建"的流程
func (uc *UserUsecase) Register(ctx context.Context, u *User) (*User, error) {
    existing, _ := uc.repo.GetUserByUsername(ctx, u.Username)
    if existing != nil {
        return nil, ErrUsernameExists
    }
    return uc.repo.CreateUser(ctx, u)
}
```

应用服务的职责边界：
- **做**：编排业务流程、调用仓储、执行业务校验、返回领域对象
- **不做**：处理 HTTP 请求/响应（Service 层）、操作数据库（Data 层）

### 三模型隔离

系统中存在**三种不同的 User 模型**，各司其职，通过转换函数隔离：

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   v1.User     │     │  biz.User     │     │  data.User    │
│  (API 契约)   │     │  (领域模型)   │     │  (持久化模型) │
│               │     │               │     │               │
│ Protobuf 生成 │     │ 纯 Go struct  │     │ GORM 模型     │
│ 携带 JSON tag │     │ 无框架依赖    │     │ 携带 gorm tag │
│ 用于网络传输  │     │ 承载业务语义  │     │ 映射数据库表  │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       │  toPBUser()        │  toBizUser()       │
       │ ←─────────────────→│←──────────────────→│
       │   service 层       │    data 层          │
```

```go
// data 层：持久化模型 → 领域模型
func toBizUser(u *User) *biz.User {
    return &biz.User{
        Id: u.Id, Username: u.Username, // ...
        CreatedAt: u.CreatedAt.Format("2006-01-02 15:04:05"), // time.Time → string
    }
}

// service 层：领域模型 → API 模型
func toPBUser(u *biz.User) *v1.User {
    return &v1.User{
        Id: u.Id, Username: u.Username, // ...
    }
}
```

为什么要三个模型而不是一个？

| 场景 | 单一模型的问题 | 三模型的解法 |
|------|---------------|-------------|
| 数据库字段变更 | GORM tag 泄漏到 API 响应 | 只改 `data.User`，其他层不受影响 |
| API 字段变更 | 修改 Protobuf 会影响数据库映射 | 只改 `v1.User`，其他层不受影响 |
| 业务规则变更 | 领域逻辑和存储逻辑混在一起 | 只改 `biz.User` 或 `UserUsecase` |

### 领域错误（Domain Error）

错误定义在领域层，用业务语义命名，携带 HTTP 状态码信息：

```go
var (
    ErrUserNotFound       = errors.NotFound("USER_NOT_FOUND", "user not found")
    ErrUsernameExists     = errors.Conflict("USERNAME_EXISTS", "username already exists")
    ErrInvalidCredentials = errors.Unauthorized("INVALID_CREDENTIALS", "invalid credentials")
)
```

领域错误的设计原则：
- **定义在 biz 层**：错误是业务规则的一部分，不是技术细节
- **用业务语言命名**：`ErrUsernameExists` 而不是 `ErrDuplicateKey`
- **跨层传播**：Data 层抛出 → Biz 层包装 → Service 层透传给客户端

### DDD 分层总览

```
api/user/v1/                    ← API 契约层（Protobuf）
  ├── user.proto                   接口定义
  ├── user.pb.go                   生成的 DTO
  └── user.pb.validate.go          参数校验

internal/model/user/
  ├── service/                    ← 接口适配层 (Interface / Adapter)
  │   └── user.go                    DTO ↔ 领域模型转换，协议处理
  ├── biz/                        ← 领域层 (Domain)
  │   └── user.go                    实体、仓储接口、应用服务、领域错误
  └── data/                       ← 基础设施层 (Infrastructure)
      ├── user_model_gen.go          持久化模型（GORM）
      └── user_cache_gen.go          仓储实现（DB + Cache）
```

## 请求流转示例

以"用户登录"为例，完整请求链路：

```
客户端 POST /v1/users/login {username, password}
  │
  ▼
Service (接口适配层):
  解析 v1.LoginRequest (API DTO)
  调用 uc.Login()
  │
  ▼
Biz / UserUsecase (应用服务):
  通过 repo.GetUserByUsername() 查询实体
  校验密码（业务规则）
  返回 biz.User（领域实体）
  │
  ▼
Data / userRepo (仓储实现):
  先查 Redis 缓存
  未命中则查 MySQL
  返回 data.User（持久化模型）
  通过 toBizUser() 转为 biz.User（领域实体）
  │
  ▼
Biz: 返回 biz.User 给 Service
  │
  ▼
Service:
  生成 JWT Token
  通过 toPBUser() 转为 v1.User (API DTO)
  组装 v1.LoginReply → 返回客户端
```

## 依赖注入：Wire

Kratos 使用 Google Wire 进行依赖注入，将三层在启动时组装起来：

```go
// wire.go — 声明各层的 Provider
var AppProviderSet = wire.NewSet(
    data.ProviderSet,    // Data 层: NewUserRepo
    biz.ProviderSet,     // Biz 层: NewUserUsecase
    service.ProviderSet, // Service 层: NewUserService
)
```

Wire 在编译期自动生成组装代码（`wire_gen.go`），避免手动传递依赖。

## 目录结构总结

```
internal/model/user/
├── biz/
│   ├── biz.go          # Wire ProviderSet
│   └── user.go         # 领域模型 + UserRepo 接口 + 业务逻辑
├── data/
│   ├── data.go         # Wire ProviderSet + NewUserRepo
│   ├── user_model_gen.go   # GORM 数据模型（表映射）
│   └── user_cache_gen.go   # UserRepo 实现（DB + Redis）
├── service/
│   └── user.go         # HTTP/gRPC 接口实现
├── wire.go             # 依赖注入声明
└── wire_gen.go         # Wire 自动生成
```

## 设计原则

### 架构原则

1. **单向依赖：** Service → Biz ← Data，禁止反向依赖
2. **接口隔离：** Biz 定义接口，Data 实现接口
3. **模型分离：** 领域模型（biz.User）与数据模型（data.User）独立
4. **协议无关：** Biz 层不感知 HTTP/gRPC 等传输协议
5. **存储无关：** Biz 层不感知 MySQL/Redis 等存储细节

### DDD 原则

6. **限界上下文隔离：** 每个业务模块（user、order 等）是独立的限界上下文，拥有自己的领域模型和仓储，模块之间不共享内部实现
7. **领域模型纯净：** `biz.User` 是纯 Go struct，不携带 GORM tag、JSON tag 或 Protobuf 注解，不依赖任何框架
8. **充血模型优先：** 业务规则尽量放在实体或应用服务中，而不是散落在 Service 或 Data 层
9. **统一语言：** 命名使用业务术语（`Register`、`Login`、`UserRepo`），而非技术术语（`InsertRow`、`CheckPassword`、`UserDAO`）
10. **防腐层转换：** 不同层之间通过显式的转换函数（`toBizUser`、`toPBUser`）隔离，避免某一层的数据结构泄漏到其他层
