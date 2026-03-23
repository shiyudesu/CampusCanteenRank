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

当前采用内存仓储作为阶段实现，目标是先打通接口契约与规则校验闭环。

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

### 4.2 下一步计划（Next Steps）

1. 进入回复列表接口：`GET /comments/{rootCommentId}/replies`（游标分页）。
2. 将评论与点赞计数逻辑迁移到 MySQL 持久化仓储，并补充仓储层测试。
3. 在评论列表中接入 `likedByMe` 的真实态计算，打通“列表 + 点赞态”一致性。

### 4.3 待优化事项（Optimization Backlog）

1. 将“创建回复 + 根评论 replyCount 递增”升级为事务边界，避免跨步骤不一致。
2. 增加 `likedByMe` 的真实态计算（当前固定返回 false）。
3. 为评论 service/repository 增加细粒度单元测试，降低对路由测试依赖。

### 4.4 风险与注意事项（Risks / Watchouts）

1. 当前评论仓储为内存实现，不具备持久化与生产并发能力。
2. author 信息依赖用户仓储查询；历史脏数据场景下会降级为占位昵称。
3. 当前已完成点赞接口，但回复列表与持久化仓储仍待落地。
