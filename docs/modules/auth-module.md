# Auth 模块实现说明（超详细版）

> 文档目标：帮助你快速且深入理解当前仓库中 **Auth 模块** 的实现思路、代码结构、关键逻辑、每个文件职责、每一段核心代码的含义，以及后续如何扩展到数据库与 Redis。

---

## 1. 本模块解决了什么问题

当前仓库原本只有规范文档，没有可运行代码。基于 `agents.md` / `backend-dev.md` / `api-spec.md`，本次先落地了后端第一个 P0 模块：**鉴权模块 auth**，提供以下最小可运行能力：

1. `POST /api/v1/auth/register`：注册
2. `POST /api/v1/auth/login`：登录并签发 accessToken + refreshToken
3. `POST /api/v1/auth/refresh`：刷新 token（含 refresh token 轮换）

并且满足：

- 统一响应格式：`{code, message, data}`
- 对齐错误码语义：`40001/40101/40901/50000`
- JWT 安全基础：限定 `HS256`，区分 access/refresh token
- refresh token 防重放：使用后即失效（一次性）
- 基础测试闭环：核心流程和错误路径已覆盖

---

## 2. 文件结构与职责（本次新增）

### 2.1 入口与路由

- `server/cmd/api/main.go`
  - 读取 `JWT_SECRET`
  - 构建 Gin 引擎
  - 启动 HTTP 服务

- `server/internal/router/router.go`
  - 装配内存仓储（用户 + refresh token）
  - 装配 `AuthService` 与 `AuthHandler`
  - 注册路由：`/api/v1/auth/register|login|refresh`

### 2.2 auth 领域

- `server/internal/dto/auth/auth_dto.go`
  - 请求/响应 DTO 定义
  - `binding` 标签用于 Gin 参数校验

- `server/internal/model/auth/user.go`
  - 用户实体最小字段定义（ID、昵称、邮箱、密码哈希、状态、创建时间）

- `server/internal/repository/auth/repository.go`
  - 仓储接口定义：`UserRepository`、`RefreshTokenRepository`
  - 内存实现：便于在无数据库阶段先打通业务闭环

- `server/internal/repository/auth/mysql_user_repository.go`
  - MySQL 用户仓储实现（基于 GORM）
  - 启动阶段自动迁移 `users` 表结构，映射统一仓储接口

- `server/internal/repository/auth/redis_refresh_token_repository.go`
  - Redis refresh token 仓储实现
  - 使用 key TTL + 删除消费（DEL）保证 refresh token 一次性消费

- `server/internal/service/auth/auth_service.go`
  - 核心业务逻辑：注册、登录、刷新 token
  - 密码哈希、JWT 签发、refresh token 轮换

- `server/internal/controller/auth/auth_handler.go`
  - HTTP handler：参数绑定、调用 service、错误到 HTTP 映射

### 2.3 通用包

- `server/internal/pkg/errors/code.go`
  - 统一错误码常量
  - `AppError` 结构（携带 code/message/internal err）

- `server/internal/pkg/response/response.go`
  - 统一返回格式封装（成功/失败）

- `server/internal/pkg/auth/jwt.go`
  - JWT claims 定义
  - token 签发与解析（强制 HS256）

- `server/internal/middleware/auth.go`
  - 访问令牌鉴权中间件（后续业务路由可复用）

### 2.4 测试

- `server/internal/router/router_test.go`
  - 端到端风格路由测试：
    - register -> login -> refresh 成功
    - refresh token 重复使用失败
    - 非法参数/重复注册/错误密码/非法 token 等错误路径

---

## 3. 关键实现细节（逐段解释）

## 3.1 go.mod 依赖选择

文件：`server/go.mod`

- `github.com/gin-gonic/gin`
  - Web 框架，负责路由、请求解析、参数校验
- `github.com/golang-jwt/jwt/v5`
  - JWT 签发与解析
- `golang.org/x/crypto/bcrypt`
  - 密码哈希（不可逆）

为什么这么选：

- 与 `backend-dev.md` 技术栈一致
- 先用最小必要依赖，避免空仓库阶段引入过多复杂度

---

## 3.2 统一错误模型

文件：`server/internal/pkg/errors/code.go`

### 常量定义

- `CodeOK = 0`
- `CodeBadRequest = 40001`
- `CodeUnauthorized = 40101`
- `CodeConflict = 40901`
- 以及 `40301/40401/50000`

作用：

- 避免在业务代码中“魔法数字”散落
- 后续前端可稳定依赖 code 做业务提示/跳转

### `AppError`

字段：

- `Code`：业务错误码
- `Message`：可返回给客户端的错误文案
- `Err`：底层错误（用于日志排查）

设计目的：

- 在 service 层保留业务语义
- 在 controller 层统一映射 HTTP 状态码

---

## 3.3 统一响应封装

文件：`server/internal/pkg/response/response.go`

### `Envelope`

```go
type Envelope struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data"`
}
```

说明：

- 严格对齐 `api-spec.md` 的通用响应结构

### `OK`

- 返回 HTTP 200
- `code=0, message="ok"`
- `data` 填入业务对象

### `Fail`

- 可指定 HTTP 状态码和业务错误码
- `data` 统一返回空对象 `{} `（保持结构稳定）

---

## 3.4 DTO 与参数校验

文件：`server/internal/dto/auth/auth_dto.go`

### 请求 DTO

1. `RegisterRequest`
   - `email`：必填 + email 格式
   - `nickname`：必填，1~64
   - `password`：必填，8~128

2. `LoginRequest`
   - `email/password` 校验同上

3. `RefreshRequest`
   - `refreshToken` 必填

为什么使用 `binding` 标签：

- 让参数校验前置到 handler 入参阶段
- 异常输入在 controller 直接返回 40001，不进入业务逻辑

### 响应 DTO

- `RegisterData{userId}`
- `LoginData{accessToken, refreshToken, expiresIn, user}`
- `RefreshData{accessToken, refreshToken, expiresIn}`

其中登录返回结构已与 `api-spec.md` 明确对齐。

---

## 3.5 JWT 工具层

文件：`server/internal/pkg/auth/jwt.go`

### `Claims`

扩展字段：

- `uid`：用户 ID
- `typ`：token 类型（`access` / `refresh`）
- `jti`：refresh token 唯一 ID（用于轮换和防重放）
- 内嵌 `jwt.RegisteredClaims`：`sub/iss/iat/exp`

### `SignToken`

- 使用 HS256 签名
- 输入 claims 输出 token 字符串

### `ParseToken`

安全点：

1. 显式校验签名算法必须为 HMAC
2. `WithValidMethods([]string{"HS256"})` 再次限制
3. 设置少量 `leeway` 缓冲时钟偏差

防止：

- 算法混淆攻击
- 异常 token 被错误解析为合法

---

## 3.6 仓储层（内存实现）

文件：`server/internal/repository/auth/repository.go`

### 接口设计

1. `UserRepository`
   - `Create`
   - `GetByEmail`

2. `RefreshTokenRepository`
   - `Save`
   - `Consume`

为什么先定义接口：

- 业务层只依赖抽象，不直接绑死存储实现
- 后续切换 MySQL/Redis 时，service 代码可基本不改

### `MemoryUserRepository`

- `map[email]*User`
- `RWMutex` 保证并发安全
- 自增 ID 起始于 1000

### `MemoryRefreshTokenRepository`

- `map[userID:jti]record`
- `Consume` 行为：
  - 不存在 -> `ErrNotFound`
  - 已过期 -> 删除并返回 `ErrNotFound`
  - 存在且有效 -> 删除并成功

这就是一次性 refresh token 的关键。

### `MySQLUserRepository`

- 使用 `gorm` + MySQL 驱动实现 `UserRepository`
- `Create` 负责用户落库并回填 `ID/CreatedAt`
- 唯一键冲突（email）统一映射到 `ErrAlreadyExists`，由 service 映射为 `40901`

### `RedisRefreshTokenRepository`

- 使用 `go-redis/v9` 实现 `RefreshTokenRepository`
- `Save` 根据 token 过期时间设置 key TTL（过期即自动清理）
- `Consume` 通过 `DEL` 判断是否存在，删除成功即消费成功，删除 0 行视为 `ErrNotFound`

这实现了“持久化 + 一次性消费”语义，支持服务重启后的 token 状态一致性。

---

## 3.7 业务层（核心）

文件：`server/internal/service/auth/auth_service.go`

### 构造函数字段

- `users` / `refreshTokens`：仓储接口
- `secret`：JWT 密钥
- `issuer`：固定 `canteen-api`
- `accessTTL`：2h
- `refreshTTL`：7d
- `nowFunc` / `idGen`：可测试性钩子

说明：

- 时间和 ID 生成解耦，后续可单测控制时钟/随机性

### `Register`

流程：

1. 基础空值保护（双保险）
2. 查重邮箱：存在 -> `40901`
3. bcrypt 哈希密码
4. 创建用户
5. 返回 userID

错误语义：

- 参数问题 -> `40001`
- 邮箱冲突 -> `40901`
- 其他未知异常 -> `50000`

### `Login`

流程：

1. 按邮箱查询用户，不存在 -> `40101`
2. 比对密码哈希，不匹配 -> `40101`
3. 签发 access token（短期）
4. 签发 refresh token（长期 + jti）
5. 保存 refresh token 记录（用于轮换）
6. 返回登录数据

为什么凭证错误用 401 而不是 404：

- 不泄漏“邮箱是否存在”的信息，减少枚举风险

### `Refresh`

流程：

1. 解析 refresh token
2. 校验类型必须是 `refresh` 且 `uid/jti` 有效
3. `Consume` 旧 token 记录（保证一次性）
4. 签发新 access + 新 refresh
5. 保存新 refresh 记录
6. 返回新 token 对

关键价值：

- 防重放：旧 refresh token 不能再次使用

### `buildAccessToken` / `buildRefreshToken`

- 两者都带 `iss/iat/exp`
- refresh 额外带 `jti`
- `ExpiresIn` 返回 access TTL 秒数，方便前端倒计时与刷新策略

---

## 3.8 Handler 层（HTTP 映射）

文件：`server/internal/controller/auth/auth_handler.go`

### 三个入口方法

1. `Register`
2. `Login`
3. `Refresh`

统一步骤：

1. `ShouldBindJSON` 绑定 + 校验
2. 调 service
3. 用 `response.OK/Fail` 回包

### `writeError`

- 识别 `*AppError`
- 映射 HTTP 状态：
  - `40001 -> 400`
  - `40101 -> 401`
  - `40901 -> 409`
  - 其他 -> `500`

意义：

- 把“业务错误码”和“HTTP 状态”映射规则集中管理

---

## 3.9 Router 装配

文件：`server/internal/router/router.go`

### `NewEngine(secret)`

1. `gin.New()`
2. `TraceID` 中间件（透传/生成 `X-Trace-ID`，写入上下文与响应头）
3. `RequestLogger` 中间件（记录 `trace_id/method/path/status/latency/client_ip`）
4. `Recover` 中间件（panic 统一转换为标准错误 envelope，并记录 panic + stack）
5. 构造仓储 + service + handler
6. 挂路由到 `/api/v1/auth/*`

这样好处：

- main 只做启动
- 业务组件装配逻辑集中到 router

### `NewEngineWithRepositories(secret, userRepo, refreshRepo)`

- 提供可注入仓储的装配入口，便于在生产使用 MySQL/Redis，在测试保持内存仓储
- 若传入 nil，则自动回退到内存实现，保持兼容性

### 启动装配（`server/cmd/api/main.go`）

- 新增环境驱动仓储选择逻辑：
  - 当 `MYSQL_DSN` 与 `REDIS_ADDR` 均存在：启用 MySQL + Redis 持久化仓储
  - 任一缺失或初始化失败：自动回退到内存仓储
- 支持环境变量：
  - `MYSQL_DSN`
  - `REDIS_ADDR`
  - `REDIS_PASSWORD`（可选）
  - `REDIS_DB`（可选，默认 0）
  - `REDIS_REFRESH_PREFIX`（可选，默认 `auth:refresh`）

---

## 3.10 中间件准备

文件：`server/internal/middleware/auth.go`

逻辑：

1. 读取 `Authorization`
2. 检查 `Bearer <token>` 格式
3. 解析 token 且必须是 access token
4. 注入 `userId` 到上下文

虽然当前 auth 三个接口不需要它，但后续评分/评论/点赞写接口会直接复用。

---

## 4. 测试说明（已实现）

文件：`server/internal/router/router_test.go`

## 4.1 成功链路测试

`TestAuthRegisterLoginRefreshFlow`

- 注册成功
- 登录成功
- 刷新成功
- 重复使用同一个 refresh token -> 40101（验证轮换）

## 4.2 错误场景测试

`TestAuthErrorCases`

- 非法邮箱注册 -> 40001
- 重复邮箱注册 -> 40901
- 错误密码登录 -> 40101
- 非法 refresh token -> 40101

## 4.3 包装器一致性检查

`decodeEnvelope` 强制每个响应都包含：

- `code`
- `message`
- `data`

这可以避免后续某些 handler 忘记走统一响应封装。

---

## 5. 与规范文档的对齐情况

## 5.1 对齐点

- `api-spec.md`：
  - `POST /auth/register|login|refresh`
  - 响应 envelope 结构
  - 认证体系（Bearer + JWT）

- `backend-dev.md`：
  - Gin + 分层结构
  - JWT + Refresh Token
  - 模块化开发闭环（先模块实现+测试）

- `agents.md`：
  - P0 优先级中 auth 优先
  - 测试门禁、文档同步要求

## 5.2 当前有意“最小化”部分

1. 目前仓储为内存实现，不是 MySQL/Redis
2. 未接入 Viper/Zap/TraceID（后续基础设施模块可补）
3. refresh 接口请求体在 `api-spec.md` 未明确，本实现采用 `{"refreshToken":"..."}`

这些都是为了先把模块闭环跑通，后续再逐步替换持久化实现。

> 更新：当前已完成 Auth 持久化升级（MySQL + Redis）并在 `api-spec.md` 补齐 refresh 请求/响应示例；该段用于保留第一阶段实现背景。

---

## 6. 后续演进建议（按优先级）

1. **中间件治理（高优先）**
   - TraceID
   - 结构化日志（Zap）
   - Recover 统一错误格式

2. **安全增强（中优先）**
   - 登录限流
   - 密码复杂度更细规则
   - refresh token 设备维度管理

3. **测试增强（中优先）**
   - service 层单测
   - middleware 单测
   - 过期 token 边界测试

4. **数据层治理（中优先）**
   - 将当前 AutoMigrate 升级为显式 migration 文件与版本管理
   - 在 CI 中补齐 MySQL/Redis 仓储集成测试

---

## 7. 快速运行说明

在 `server/` 目录执行：

```bash
go mod tidy
go test ./...
go vet ./...
go build ./...
```

启动服务：

```bash
JWT_SECRET="your-strong-secret" go run ./cmd/api
```

启用持久化模式（MySQL + Redis）：

```bash
JWT_SECRET="your-strong-secret" \
MYSQL_DSN="user:pass@tcp(127.0.0.1:3306)/canteen?charset=utf8mb4&parseTime=True&loc=Local" \
REDIS_ADDR="127.0.0.1:6379" \
go run ./cmd/api
```

若仅设置 `JWT_SECRET`（未设置 `MYSQL_DSN/REDIS_ADDR`），服务会自动进入内存模式。

默认监听：`localhost:8080`

---

## 8. 你最关心的“读代码路径建议”

如果你要快速理解整个 auth 模块，建议按这个顺序读：

1. `router/router.go`（先看总装配）
2. `auth_handler.go`（看接口入口和错误映射）
3. `auth_service.go`（看核心业务）
4. `jwt.go`（看 token 细节）
5. `repository.go`（看当前存储策略）
6. `router_test.go`（看行为预期）

这样会先建立“系统全貌”，再进入细节，理解成本最低。

---

## 9. 本模块完成定义（当前状态）

- [x] 已实现 register/login/refresh 三个接口
- [x] 已实现统一响应结构
- [x] 已实现错误码映射
- [x] 已实现 access/refresh token 签发与解析
- [x] 已实现 refresh token 一次性消费（轮换）
- [x] 已完成 auth 仓储持久化升级（MySQL 用户仓储 + Redis refresh token 仓储）
- [x] 已支持环境变量驱动的持久化/内存自动回退装配
- [x] 已在 `api-spec.md` 补全 refresh 请求/响应示例
- [x] 已提供模块级测试并通过
- [x] 已补充模块级详细文档

本模块已达到“可运行、可验证、可理解、可扩展”的第一阶段交付目标。

---

## 10. 模块开发状态同步（用于会话切换无缝衔接）

> 你每次开发完，都要同步更新开发文档，写上你已经完成的功能，下一步要做的，待优化的等等内容，为之后切换对话仍然可以无缝衔接开发做准备。

### 10.1 已完成功能（Done）

1. 后端 `server/` 基础骨架初始化完成（可编译、可测试）。
2. Auth 模块接口完成并可用：
   - `POST /api/v1/auth/register`
   - `POST /api/v1/auth/login`
   - `POST /api/v1/auth/refresh`
3. 统一响应封装已接入：`{code,message,data}`。
4. 错误码语义已接入并与 API 规范对齐（核心：40001/40101/40901/50000）。
5. JWT 签发与解析完成，access/refresh token 类型分离。
6. refresh token 一次性消费（轮换）逻辑完成，防止重放。
7. Auth 持久化升级完成：
   - `UserRepository` 支持 MySQL（GORM）实现并接入唯一键冲突语义。
   - `RefreshTokenRepository` 支持 Redis（TTL + DEL consume）实现。
8. 启动装配支持环境变量驱动：
   - 有 `MYSQL_DSN + REDIS_ADDR` 时走持久化模式。
   - 初始化失败或缺失时自动回退内存模式。
9. `api-spec.md` 已补全 `POST /auth/refresh` 请求/响应示例，联调字段歧义已消除。
10. 路由级测试完成并覆盖核心成功/失败路径。
11. 中间件治理第一阶段已完成：
   - 新增 `TraceID` 中间件（透传/生成 `X-Trace-ID`，并写入 Gin Context）。
   - 新增统一请求日志中间件（输出 trace_id、method、path、status、latency、client_ip）。
   - 新增统一 Recover 中间件（panic 统一返回 `50000/internal error` 的标准 envelope）。
12. Router 已接入上述中间件链路，并补充中间件行为测试（TraceID 生成/透传、Recover 统一返回）。
13. 中间件治理第二阶段已完成（Zap + 脱敏）：
   - 请求日志升级为 Zap 结构化日志，字段包含 `trace_id/request_id/user_id/method/path/status/latency/client_ip`。
   - 增加日志配置能力：支持 `LOG_LEVEL` 调整日志级别。
   - 增加敏感字段脱敏：支持 `LOG_SENSITIVE_FIELDS` 自定义脱敏字段，默认覆盖 `authorization/cookie/password/token/refreshToken`。

### 10.2 下一步计划（Next Steps）

1. **进入下一个业务模块**（按 P0）
   - 推荐顺序：`stall list/detail` -> `rating` -> `comment` -> `like` -> `ranking`。
2. **补充仓储层测试**
   - 增加 MySQL/Redis 仓储集成测试（可通过 docker compose 或 testcontainers）。

### 10.3 待优化事项（Optimization Backlog）

1. 密码策略增强（复杂度、黑名单词、历史密码策略可扩展）。
2. 登录安全增强（IP + 账号维度限流、失败次数惩罚）。
3. refresh token 增加设备维度管理（deviceId/sessionId）。
4. service 层补充更细颗粒单测（当前仍以路由测试为主）。
5. 错误信息国际化或统一错误消息字典。
6. 配置管理升级（Viper + `.env.dev/.env.test/.env.prod`）。
7. 将当前 AutoMigrate 迁移策略升级为显式 migration 文件，便于生产可控发布。

### 10.4 风险与注意事项（Risks / Watchouts）

1. 持久化模式依赖 MySQL/Redis 可用性；当前为“失败即回退内存”的可用性优先策略，生产建议引入启动失败策略与健康检查门禁。
2. 当前尚未接入审计日志与指标上报，不适合直接作为生产安全基线。
3. 仓储层尚未加入真实 MySQL/Redis 集成测试，需要在 CI 中补齐。

### 10.5 下一会话接手指引（Handoff Quick Start）

1. 先读：`agents.md` -> `api-spec.md` -> 本文档第 10 章。
2. 运行校验：
   - `cd server`
   - `go test ./...`
   - `go vet ./...`
   - `go build ./...`
3. 从 `stall list/detail` 模块开始推进，并保持“开发完成即同步文档”的规则。

### 10.6 模型使用规则（会话继承）

- 前端开发：使用 Gemini 3.1 Pro Preview（`gemini-3.1-pro-preview`）。
- 除前端开发外的所有任务：统一使用 GPT-5.3 Codex xhigh（`gpt-5.3-codex`，`xhigh`），不因 agent 或模式变化而改变。
