# 02 auth 模块逐行拆解

本章目标：你可以从 0 手写出 register/login/refresh/logout 全流程，并理解 JWT + Refresh Token 的工程实现。

主文件：

1. server/internal/dto/auth/auth_dto.go
2. server/internal/model/auth/user.go
3. server/internal/repository/auth/repository.go
4. server/internal/repository/auth/mysql_user_repository.go
5. server/internal/repository/auth/redis_refresh_token_repository.go
6. server/internal/pkg/auth/jwt.go
7. server/internal/service/auth/auth_service.go
8. server/internal/controller/auth/auth_handler.go
9. server/internal/testkit/repositories.go（测试夹具）

---

## 1. dto/auth_dto.go 逐行思路

你会看到三类结构：

1. 请求 DTO：RegisterRequest、LoginRequest、RefreshRequest。
2. 响应 DTO：RegisterData、LoginData、RefreshData。
3. 嵌套 VO：LoginUserVO。

### 必学点

1. binding 标签是第一层参数防线。
2. deviceId 是可选，但服务层会补默认值。
3. 统一响应结构由 handler 外层封装，不写在 DTO 里。

你重写时，先写请求 DTO，再写响应 DTO，最后补 binding 规则。

---

## 2. model/auth/user.go 逐行思路

User 实体核心字段：

1. ID
2. Email
3. Nickname
4. PasswordHash
5. Status
6. CreatedAt

### 必学点

1. PasswordHash 永远存哈希，不存明文。
2. Status 用于未来冻结、封禁扩展。
3. CreatedAt 用于审计和排序。

---

## 3. repository/auth/repository.go 逐行拆解

这是 auth 的接口边界定义文件。

### 3.1 错误变量

常见：

1. ErrNotFound
2. ErrAlreadyExists

作用：service 可以根据错误类型映射业务错误码。

### 3.2 UserRepository 接口

关键方法：

1. Create(ctx, user)
2. GetByEmail(ctx, email)
3. GetByID(ctx, id)

### 3.3 RefreshTokenRepository 接口

关键方法：

1. Save(ctx, record)
2. Consume(ctx, userID, jti, deviceID)

Consume 的语义是“读取并删除”，实现一次性消费。

### 3.4 重构后接口扩展

UserRepository 新增批量查询能力：

1. GetByIDs(ctx, ids)

这个方法用于 comment/me 模块消除 N+1 用户查询。

说明：生产仓储不再提供内存实现，测试内存实现迁移到 internal/testkit。

---

## 4. pkg/auth/jwt.go 逐行拆解

### 4.1 Claims 结构

除了标准 claims，还加了业务字段：

1. UserID
2. TokenType（access 或 refresh）
3. JTI（refresh token 唯一标识）
4. DeviceID

### 4.2 SignToken

核心步骤：

1. jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
2. SignedString([]byte(secret))

### 4.3 ParseToken

核心步骤：

1. 校验签名算法必须是 HMAC。
2. 用 claims 结构接收解析结果。
3. 校验 token.Valid。
4. 返回 claims。

必学点：

1. refresh 和 access 共用结构但靠 TokenType 区分。
2. Parse 失败必须返回可判定的错误类型。
3. issuer 与 audience 会在解析后再次校验，防止跨系统 token 误用。

---

## 5. service/auth_service.go 逐行级讲解

这是 auth 章最核心文件。

### 5.1 AuthService 结构体字段

1. users：用户仓储。
2. refreshTokens：refresh token 仓储。
3. secret：JWT 密钥。
4. issuer：签发者。
5. accessTTL、refreshTTL：过期时间。
6. nowFunc、idGen：可注入函数，便于测试。

这两个可注入函数很关键：

1. nowFunc 让你能固定时间测试 token。
2. idGen 让你能固定 jti 测试 refresh 轮换。

### 5.2 NewAuthService

初始化默认值：

1. issuer = authpkg.TokenIssuer
2. accessTTL = 2h
3. refreshTTL = 7d
4. nowFunc = time.Now
5. idGen = defaultID

### 5.3 Register（auth_service.go:47 起）

按逻辑逐行理解：

1. trim + 判空：防止空字符串穿透。
2. users.GetByEmail：查重邮箱。
3. 若找到用户 -> 409 冲突。
4. 若错误不是 ErrNotFound -> 500。
5. bcrypt.GenerateFromPassword：密码哈希。
6. 构造 user 并 users.Create。
7. Create 若重复冲突 -> 409。
8. 其他错误 -> 500。
9. 返回 user.ID。

你重写时，不要把“是否重复”放到 handler。应在 service 保持业务语义。

### 5.4 Login（auth_service.go:79 起）

关键步骤：

1. 读取并规整 deviceId；空则 default。
2. 按邮箱查用户，不存在返回 unauthorized。
3. bcrypt.CompareHashAndPassword 校验密码。
4. buildAccessToken(user.ID)。
5. buildRefreshToken(user.ID, deviceID)。
6. refreshTokens.Save(record)。
7. 返回 accessToken、refreshToken、expiresIn、user。

必学点：

1. 登录失败统一返回 invalid credentials，避免泄漏账号存在性。
2. refresh token 要落存储，不是只靠 JWT 自身。

### 5.5 Refresh（auth_service.go:121 起）

关键步骤：

1. ParseToken 解析 refresh token。
2. token 过期 -> unauthorized(token expired)。
3. TokenType 不是 refresh 或 claims 异常 -> unauthorized。
4. 如果请求带 deviceId，必须和 claims 对上。
5. refreshTokens.Consume(旧 jti)：一次性消费。
6. 再签发新 access + 新 refresh。
7. Save 新 refresh 记录。
8. 返回新令牌。

这就是“轮换 + 防重放”的标准实现。

### 5.6 Logout（auth_service.go:207 起）

本质：消费 refresh token。

步骤：

1. Parse refresh token。
2. 校验类型与 claims。
3. 计算 deviceID（claims 优先，请求次之，最后 default）。
4. refreshTokens.Consume。

这样登出后同一个 refresh token 不可再用。

### 5.7 buildAccessToken / buildRefreshToken

1. access token 不需要 jti。
2. refresh token 需要 jti，并写入 RegisteredClaims.ID。
3. subject 用 userID 字符串。
4. issuer、iat、exp 全写入。
5. audience 写入 canteen-client，并在 ParseToken 时强校验。
6. defaultID 使用 crypto/rand 生成随机 jti（失败时才回退时间戳）。

---

## 6. controller/auth_handler.go 逐行思路

每个 handler 的模板都类似：

1. var req DTO
2. c.ShouldBindJSON 或 ShouldBind
3. 参数错误 -> 40001
4. 调用 service
5. service 错误 -> writeError
6. 成功 -> response.OK

writeError 的意义：

1. 集中把 AppError 映射为 HTTP 状态。
2. 避免每个 handler 重复 switch。

---

## 7. mysql_user_repository 与 redis_refresh_repository

### 7.1 mysql_user_repository.go

关键点：

1. 用 GORM 模型映射 users 表。
2. Create 处理唯一键冲突。
3. GetByEmail / GetByID 把 gorm.ErrRecordNotFound 转为 ErrNotFound。
4. GetByIDs 支持去重与非法 id 过滤，供上层批量昵称查询。

### 7.2 redis_refresh_token_repository.go

关键点：

1. key 结构通常包含 prefix + userId + deviceId + jti。
2. Save 使用 TTL（过期自动清理）。
3. Consume 要做到“存在才删”，删后不可重复消费。

---

## 8. 你应该立即做的手写练习

1. 不看代码，默写 AuthService 的四个公开方法签名。
2. 手写 Refresh 流程图。
3. 手写一个最小 testkit 版 RefreshTokenRepository。
4. 给 Login 增加一条“用户状态禁用不可登录”的规则（自己扩展）。

---

## 9. 本章完成判定

你满足以下条件就算吃透：

1. 能解释为什么 refresh token 需要落 Redis。
2. 能解释为什么 refresh 要先 consume 再签发新 token。
3. 能解释 deviceId 在登录和刷新中的作用。
4. 能不看代码写出 register/login 的 service 主流程。