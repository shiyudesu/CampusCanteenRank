# 06 中间件与通用 pkg 逐行拆解

本章目标：你掌握项目真正的工程化底座，能自己写出统一错误、统一响应、鉴权中间件、限流与日志链路。

主文件：

1. server/internal/middleware/auth.go
2. server/internal/middleware/optional_auth.go
3. server/internal/middleware/trace_id.go
4. server/internal/middleware/logger.go
5. server/internal/middleware/recover.go
6. server/internal/middleware/rate_limit.go
7. server/internal/pkg/errors/code.go
8. server/internal/pkg/response/response.go
9. server/internal/pkg/cursor/cursor.go
10. server/internal/pkg/logger/logger.go

---

## 1. errors/code.go

这是“错误语义中心”。

你要看懂三层：

1. 错误码常量（40001/40101/40401/40901/50000）。
2. AppError 结构（code + message + cause）。
3. 帮助函数（New、Wrap、CodeOf 之类）。

原则：

1. repository 返回技术错误。
2. service 转业务错误。
3. handler 再映射 HTTP 状态。

---

## 2. response/response.go

统一响应结构：

1. code
2. message
3. data

核心函数：

1. OK
2. Fail

这样所有接口都一致，前端更易处理。

---

## 3. cursor/cursor.go

通用游标 token：

1. createdAt
2. id

Encode/Decode 核心：

1. JSON 编码。
2. Base64 Raw URL 编码（无 = 填充）。
3. 时间格式优先 RFC3339Nano。
4. 向后兼容旧格式。

这是评论列表等模块复用的基础件。

---

## 4. auth 与 optional_auth 中间件

### 4.1 Auth

流程：

1. 读 Authorization。
2. 校验 Bearer 格式。
3. ParseToken。
4. 校验 TokenType=access。
5. c.Set(userId)。
6. c.Next。

失败就 Fail + Abort。

重构后补充：

1. ParseToken 还会校验 issuer 与 audience。
2. 非本系统签发或 audience 不匹配的 token 会被拒绝。

### 4.2 OptionalAuth

流程：

1. 没 token 直接 Next。
2. token 解析失败也 Next。
3. 解析成功就 Set(userId) 后 Next。

这个中间件用于游客可访问页面附带登录态增强。

---

## 5. trace_id 中间件

目标：每个请求带唯一 trace id，便于日志串联。

关键步骤：

1. 从请求头尝试读取 trace id。
2. 不合法就生成新的。
3. 写回 response header。
4. 放入 context 供后续日志使用。

---

## 6. logger 中间件

典型记录内容：

1. method
2. path
3. status
4. latency
5. client ip
6. trace id

必须放在 recover 前后关系合理，以保证 panic 情况也能记录。

---

## 7. recover 中间件

作用：捕获 panic，返回统一 500，避免进程崩溃。

关键点：

1. defer recover。
2. 打 error 日志并附带 trace id。
3. 返回标准错误 envelope。

---

## 8. rate_limit 中间件

这里常见实现是按客户端 key（IP 或用户维度）限流桶。

你重点理解：

1. key 如何生成。
2. 时间窗口如何计算。
3. 超限时如何返回错误。
4. 并发安全如何保证（锁或原子）。

重构后新增：

1. 限流桶记录 lastSeen。
2. 按窗口周期做过期清理，避免 map 长期无界增长。

写接口一般会挂这个中间件。

---

## 9. pkg/logger/logger.go

这个文件负责封装 zap 并支持敏感字段脱敏。

关键能力：

1. Init(level)
2. SetSensitiveFields
3. With(fields)
4. Info/Error

敏感字段脱敏的价值：

1. 避免 token、password 等泄露到日志。
2. 满足基本安全合规。

---

## 10. 本章完成判定

你满足以下条件即掌握：

1. 能说明 Auth 与 OptionalAuth 的差异和使用场景。
2. 能写出 trace + logger + recover 的中间件顺序。
3. 能说明统一错误和统一响应为什么是大型项目必备。