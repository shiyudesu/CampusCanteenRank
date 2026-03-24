# Comment 模块实现说明（阶段三）

## 1. 本次实现范围

在 `stall` 模块之后，本次完成了后端下一部分 `comment` 领域的第一阶段闭环，新增并接入以下接口：

1. `POST /api/v1/stalls/{stallId}/comments`
2. `GET /api/v1/stalls/{stallId}/comments?sort=latest&limit=&cursor=`

并补充了：

- 独立的 `comment` 分层结构（model/dto/repository/service/controller）
- 一级评论列表游标分页（`created_at DESC, id DESC` 对应的游标语义）
- 评论发布参数规则校验（一级评论 / 回复评论）
- 一级评论 `replyCount` 的内存一致性维护（回复创建时递增）
- 路由集成测试，覆盖创建、分页、鉴权与关键错误路径

---

## 2. 关键实现点

### 2.1 模块分层

- `server/internal/model/comment/comment.go`
- `server/internal/dto/comment/comment_dto.go`
- `server/internal/repository/comment/repository.go`
- `server/internal/service/comment/comment_service.go`
- `server/internal/controller/comment/comment_handler.go`

当前已支持 MySQL 持久化仓储（保留内存仓储用于无依赖本地联调与测试回退）。

### 2.2 发布评论规则

`POST /stalls/{stallId}/comments` 统一支持一级与回复两类请求：

- 一级评论：`rootId=0,parentId=0,replyToUserId=0`
- 回复评论：`rootId>0,parentId>0,replyToUserId>0`

回复评论额外校验：

1. `rootId` 必须存在且为一级评论
2. `parentId` 必须存在且属于同一 `stall`
3. `parentId` 为一级时需等于 `rootId`；为二级时需满足 `parent.rootId == rootId`
4. `replyToUserId` 必须与 `parent.userId` 一致

### 2.3 一级评论列表（游标分页）

- `GET /stalls/{stallId}/comments` 默认 `sort=latest`
- `limit` 默认 20，最大 50
- `cursor` 复用通用 `internal/pkg/cursor`
- 仅返回一级评论（`rootId=0,parentId=0`）
- 返回结构对齐 `items/nextCursor/hasMore`

### 2.4 路由装配

在 `server/internal/router/router.go` 新增：

- `POST /api/v1/stalls/:stallId/comments`（需鉴权）
- `GET /api/v1/stalls/:stallId/comments`

并在 router 装配阶段接入 `comment` 模块 service/repository。

---

## 3. 测试与验证结果

已完成并通过：

1. `go test ./server/...`
2. `go vet ./server/...`
3. `go build ./server/...`

新增测试覆盖：

- `server/internal/router/router_test.go`
  - 新增 `TestCommentCreateAndListTopLevel`
  - 覆盖：
    - 一级评论创建成功
    - 一级评论列表游标分页（无重复）
    - 回复评论创建成功并驱动根评论 `replyCount` 变化
    - 未登录发布评论（401）
    - 非法 cursor / sort（40001）
    - 回复关系不合法（40001）
    - 窗口不存在（40401）

---

## 4. 模块开发状态同步（用于会话切换无缝衔接）

> 你每次开发完，都要同步更新开发文档，写上你已经完成的功能，下一步要做的，待优化的等等内容，为之后切换对话仍然可以无缝衔接开发做准备。

### 4.1 已完成功能（Done）

1. 评论发布接口已支持一级评论与回复评论两种路径。
2. 回复评论的 `rootId/parentId/replyToUserId` 关系校验已接入。
3. 一级评论列表已支持 `latest` 游标分页（`limit + cursor`）。
4. 评论 author 信息已在响应中返回（基于用户仓储查询昵称）。
5. 根评论 `replyCount` 在回复创建后可正确递增。
6. 评论模块路由级测试已覆盖主要成功/失败链路。
7. 点赞模块已完成：支持 `POST/DELETE /api/v1/comments/{commentId}/like`，返回 `liked + likeCount`。
8. 点赞接口幂等已落地：重复点赞/取消点赞不会报错且计数不重复增减。
9. 回复列表接口已完成：支持 `GET /api/v1/comments/{rootCommentId}/replies`，返回 `items/nextCursor/hasMore`。
10. 评论仓储已新增 MySQL 实现：覆盖创建、查询、分页、点赞幂等与 `likedByMe` 查询能力。
11. 点赞写路径已切换为事务实现（点赞记录与 `like_count` 原子更新）。
12. 评论列表与回复列表的 `likedByMe` 已升级为批量查询，减少逐条查询带来的 N+1 开销。
13. 新增 MySQL 仓储集成测试文件（环境变量 `MYSQL_DSN` 可用时执行）：覆盖点赞/取消点赞幂等、`HasLikedBatch` 边界语义、`ErrNotFound` 行为与同时间戳游标分页稳定性。
14. 已将“回复创建 + 根评论 `replyCount` 递增”改为仓储层原子写入：新增 `CreateReplyAndIncrementRoot`，内存/ MySQL 仓储均已接入，服务层回复路径切换为单次原子调用。
15. 评论点赞写路径已接入 ranking 缓存失效：`LikeComment/UnlikeComment` 成功后触发 `InvalidateRankingCache`，避免排行榜缓存滞后。
16. 评论 MySQL 集成测试初始化流程已接入显式迁移执行（`migration.ApplySQLMigrations`），降低对 `AutoMigrate` 的隐式依赖。

### 4.2 下一步计划（Next Steps）

1. 为 MySQL 评论仓储补充并发冲突场景的强化测试（多协程点赞竞争与事务锁行为观测）。
2. 在 CI/CD 固化迁移执行门禁，确保每次评论相关集成测试都基于最新 SQL schema。
3. 为 `HasLikedBatch` 增加大页性能基线测试（高评论量场景下的耗时与查询次数监控）。
4. 为回复创建原子事务补充 MySQL 集成级回归用例（覆盖 root 不存在与并发回帖场景）。

### 4.3 待优化事项（Optimization Backlog）

1. `likedByMe` 真实态与批量查询已落地；下一步优化为按场景裁剪字段读取与缓存策略。
2. 为评论 service/repository 增加细粒度单元测试，降低对路由测试依赖。

### 4.4 风险与注意事项（Risks / Watchouts）

1. 当前默认仍保留内存回退策略（MySQL 初始化失败会回退），生产环境需配合健康检查与启动门禁。
2. author 信息依赖用户仓储查询；历史脏数据场景下会降级为占位昵称。
3. 当前已接入 MySQL 持久化，主要风险转为索引与迁移脚本版本治理（需确保环境间 schema 一致）。
4. 批量 `likedByMe` 依赖 `comment_likes(comment_id,user_id)` 索引性能，需持续关注大数据量分页场景。
5. MySQL 集成测试依赖 `MYSQL_DSN` 环境；未配置时会跳过相关测试，需在 CI 中显式提供数据库环境避免覆盖盲区。
