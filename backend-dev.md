# 食堂评分系统后端开发文档（Go + MySQL + Redis）

## 1. 文档目标

本规范用于指导后端团队构建企业化可扩展服务，重点保障：

- 评分、楼中楼评论、评论点赞核心链路稳定
- 全站列表统一使用游标分页
- 排行榜查询高性能（MySQL + Redis）
- 前后端接口契约一致（见 `api-spec.md`）

---

## 2. 技术栈与基础设施

## 2.1 技术选型

- 语言：Go 1.22+
- Web框架：Gin
- ORM：GORM（复杂排行场景可用手写 SQL）
- 数据库：MySQL 8.0+
- 缓存：Redis 7+
- 鉴权：JWT + Refresh Token
- 配置：Viper
- 日志：Zap
- 任务调度：robfig/cron
- 文档：Swagger（swaggo）
- 测试：testing + testify

## 2.2 服务形态

- 单体模块化（Modular Monolith），便于面试展示架构能力
- 通过清晰分层保证后续可拆分微服务

---

## 3. 后端目录规范

```txt
server/
  cmd/
    api/main.go
  internal/
    router/
    middleware/
    controller/
      auth/
      stall/
    service/
      auth/
      stall/
    repository/
      auth/
      stall/
    model/
      auth/
      stall/
    dto/
      auth/
      stall/
    cache/
    pkg/
      auth/
      errors/
      logger/
      cursor/
      response/
  migrations/
  scripts/
  configs/
```

---

## 4. 业务域拆分

1. `auth`：注册、登录、token刷新
2. `canteen`：食堂信息
3. `stall`：窗口信息、详情聚合
4. `rating`：用户打分
5. `comment`：楼中楼评论、点赞
6. `ranking`：排行榜计算与查询

---

## 5. 数据模型设计（含楼中楼 + 点赞）

## 5.1 核心表

### users
- `id BIGINT PK`
- `nickname VARCHAR(64)`
- `email VARCHAR(128) UNIQUE`
- `password_hash VARCHAR(255)`
- `status TINYINT`
- `created_at DATETIME`

### canteens
- `id BIGINT PK`
- `name VARCHAR(128)`
- `campus VARCHAR(64)`
- `status TINYINT`

### food_types
- `id BIGINT PK`
- `name VARCHAR(64)`

### stalls
- `id BIGINT PK`
- `canteen_id BIGINT`
- `food_type_id BIGINT`
- `name VARCHAR(128)`
- `status TINYINT`

### ratings
- `id BIGINT PK`
- `user_id BIGINT`
- `stall_id BIGINT`
- `score TINYINT`（1~5）
- `created_at DATETIME`
- `updated_at DATETIME`
- 唯一索引：`uk_user_stall(user_id, stall_id)`

### comments（支持楼中楼）
- `id BIGINT PK`
- `stall_id BIGINT NOT NULL`
- `user_id BIGINT NOT NULL`
- `root_id BIGINT NOT NULL DEFAULT 0`  
  - 一级评论：`root_id = 0`
  - 二级回复：`root_id = 一级评论id`
- `parent_id BIGINT NOT NULL DEFAULT 0`
  - 一级评论：`parent_id = 0`
  - 回复一级：`parent_id = 一级评论id`
  - 回复二级：`parent_id = 被回复评论id`
- `reply_to_user_id BIGINT NOT NULL DEFAULT 0`
- `content VARCHAR(2000)`
- `like_count INT NOT NULL DEFAULT 0`
- `reply_count INT NOT NULL DEFAULT 0`（仅一级评论维护）
- `status TINYINT`（1=正常，2=隐藏）
- `created_at DATETIME`

### comment_likes
- `id BIGINT PK`
- `comment_id BIGINT`
- `user_id BIGINT`
- `created_at DATETIME`
- 唯一索引：`uk_comment_user(comment_id, user_id)`

---

## 6. 游标分页后端规范（强制）

## 6.1 统一游标结构

游标采用 Base64URL 编码 JSON：

```json
{"createdAt":"2026-03-22T10:00:00Z","id":12345}
```

> 所有按时间倒序列表必须使用 `(created_at DESC, id DESC)` 复合排序，避免翻页重复/漏数据。

## 6.2 查询条件模板

```sql
WHERE (created_at < :createdAt)
   OR (created_at = :createdAt AND id < :id)
ORDER BY created_at DESC, id DESC
LIMIT :limit
```

## 6.3 返回体模板

```json
{
  "items": [],
  "nextCursor": "...",
  "hasMore": true
}
```

---

## 7. 评论与点赞业务规则

## 7.1 评论发布

- 一级评论：`rootId=0,parentId=0,replyToUserId=0`
- 回复评论：
  - `rootId` 必须指向一级评论
  - `parentId` 可指向一级或二级评论
  - `replyToUserId` 必须存在且与 parent 对应

## 7.2 楼中楼深度控制

- 产品形态为“楼中楼两层展示”
- 数据允许任意 parent 关系，但查询接口只提供：
  - 一级评论列表
  - 指定一级评论下的回复列表

## 7.3 点赞规则

- 幂等：
  - 点赞接口重复调用不重复加1
  - 取消点赞重复调用不报错
- 计数一致性：
  - 事务内插入/删除 like 记录并更新 `like_count`
  - 或采用异步对账任务纠偏

---

## 8. 排行榜实现策略

## 8.1 排行公式（建议）

`score = avg_rating * 0.75 + log10(review_count + 1) * 0.2 + recency_boost * 0.05`

## 8.2 支持筛选

- `scope=global`
- `scope=canteen&scopeId=xx`
- `scope=foodType&scopeId=xx`
- `days=7|30|90`

## 8.3 缓存策略

- Key：`rank:v1:{scope}:{scopeId}:{days}:{sort}:{cursor}`
- TTL：300秒
- 触发失效：评分变更、评论发布（异步批量失效）

---

## 9. 中间件与治理要求

1. `Recover`：panic恢复
2. `TraceID`：请求链路ID
3. `Auth`：JWT鉴权
4. `RateLimit`：评论与点赞接口限流
5. `Logger`：结构化日志（请求耗时、状态码）

---

## 10. 事务与一致性约束

- 评论发布（回复）时需更新一级评论 `reply_count`
- 点赞/取消点赞必须使用事务保障计数一致
- 评分更新后需触发窗口统计更新（异步可接受）

---

## 11. 测试策略

> 模块化开发强制要求：每个后端模块（如 `auth`、`rating`、`comment`、`ranking`）开发完成后，必须先完成该模块相关测试并通过，再允许推送到 GitHub。

## 11.1 单元测试

- 游标编码/解码
- 评论树参数校验
- 点赞幂等逻辑

## 11.2 集成测试

- 发布一级评论 -> 回复 -> 查询楼中楼
- 点赞 -> 取消点赞 -> 计数正确
- 排行榜筛选维度正确

## 11.3 压测建议

- 重点接口：
  - `GET /rankings`
  - `GET /stalls/{id}/comments`
  - `POST /comments/{id}/like`

## 11.4 模块测试通过后的推送规则

- 后端模块测试通过后，当前批次可直接推送 GitHub
- 测试失败时禁止推送，需修复并完成回归验证
- 建议按模块粒度提交，确保问题定位与回滚成本可控

## 11.5 后端与其他任务模型使用规则

- 后端开发及其他工程任务默认使用 GPT-5.3 Codex xhigh（模型：`gpt-5.3-codex`，推理强度：`xhigh`）
- 若涉及额外组件、插件、扩展或 SDK，需自行搜索官方文档并安装，记录来源、版本、用途

## 11.6 后端 GitHub 提交信息规范

- 仅使用前缀：`feat`、`chore`、`fix`、`refactor`、`docs`
- 提交信息使用中文，格式：`<type>(可选scope): <中文描述>`
- 示例：
  - `feat(comment): 添加评论点赞幂等逻辑`
  - `fix(rating): 修复重复评分更新异常`
  - `refactor(repository): 重构游标分页查询`

## 11.7 后端代码注释规范（强制）

- 新增/修改后端代码必须包含适当注释，优先说明实现意图、事务边界、错误语义与一致性约束。
- 对外暴露函数、复杂查询、并发与容错分支需要注释“为什么这样做”，避免仅重复代码字面含义。
- 涉及安全敏感信息（如 token/password/cookie）的处理逻辑，必须在代码附近注明脱敏或防护策略，便于评审与回归。

---

## 12. 安全与风控

- 评论内容长度限制与敏感词过滤
- XSS清洗（存储前或渲染前）
- 登录接口防爆破（IP限流）
- 关键写接口鉴权必开

---

## 13. 发布与运维

- Docker Compose：`mysql + redis + api + web`
- 环境变量分层：`.env.dev/.env.test/.env.prod`
- 启动前自动迁移（可选）

---

## 14. 完成定义（DoD）

- 游标分页接口全面替代 offset 分页
- 评论楼中楼 + 点赞链路全量可用
- 核心接口具备测试覆盖
- 与 `api-spec.md` 字段和错误码完全对齐
