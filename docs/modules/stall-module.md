# Stall 模块实现说明（阶段二）

## 1. 本次实现范围

本次在 `auth` 模块之后，完成了后端下一部分 `stall` 领域的第二阶段闭环，新增并接入以下接口：

1. `GET /api/v1/canteens`
2. `GET /api/v1/stalls?limit=&cursor=&canteenId=&foodTypeId=&sort=score_desc`
3. `GET /api/v1/stalls/{stallId}`
4. `POST /api/v1/stalls/{stallId}/ratings`

并补充了：

- 独立的 `stall` 分层结构（model/dto/repository/service/controller）
- 游标编码解码基础包 `internal/pkg/cursor`
- 评分写接口（单用户评分 upsert）与窗口聚合评分字段同步更新
- 路由集成测试，覆盖分页、参数错误、详情查询和 not found

---

## 2. 关键实现点

### 2.1 模块分层

- `server/internal/model/stall/stall.go`
- `server/internal/dto/stall/stall_dto.go`
- `server/internal/repository/stall/repository.go`
- `server/internal/service/stall/stall_service.go`
- `server/internal/controller/stall/stall_handler.go`

当前已支持 MySQL 持久化仓储（并保留内存仓储作为本地/测试回退路径）。

### 2.2 游标分页

- 新增 `server/internal/pkg/cursor/cursor.go`（通用 Base64URL 游标编码/解码）
- 新增 `server/internal/pkg/cursor/cursor_test.go`
- `GET /stalls` 实际使用与 `score_desc` 一致的游标键（`avgRating + id`）

### 2.3 详情接口 myRating 行为

- 新增 `server/internal/middleware/optional_auth.go`
- `GET /stalls/{stallId}` 支持“未登录返回基础详情；登录后附带 myRating（若存在）”

### 2.4 评分写接口 upsert 行为

- 在 `StallRepository` 增加 `UpsertUserRating`，内存仓储实现用户评分的新增/更新
- 新增路由：`POST /api/v1/stalls/:stallId/ratings`（需鉴权）
- 写入规则：同一用户对同一窗口重复提交视为更新，不重复增加 `ratingCount`
- 聚合规则：
  - 新评分：`ratingCount +1`，并重算 `avgRating`
  - 更新评分：`ratingCount` 不变，按“旧分替换新分”重算 `avgRating`
- 详情字段：`myRating` 在详情接口中保持稳定返回；未登录或未评分时返回 `null`

---

## 3. 测试与验证结果

已完成并通过：

1. `go test ./server/...`
2. `go vet ./server/...`
3. `go build ./server/...`

新增测试覆盖：

- `server/internal/pkg/cursor/cursor_test.go`
- `server/internal/router/router_test.go`（新增 stall/canteen 相关用例）
- `server/internal/router/router_test.go`（新增评分 upsert 成功链路与错误路径用例）

---

## 4. 模块开发状态同步（用于会话切换无缝衔接）

> 你每次开发完，都要同步更新开发文档，写上你已经完成的功能，下一步要做的，待优化的等等内容，为之后切换对话仍然可以无缝衔接开发做准备。

### 4.1 已完成功能（Done）

1. `canteen` 列表接口（只返回有效状态项）已接入。
2. `stall` 列表接口已支持：
   - limit 上限约束（最大 50）
   - 基于 `score_desc` 的稳定排序与游标翻页
   - `canteenId` / `foodTypeId` 筛选
3. `stall` 详情接口已支持 `40401` 语义。
4. 详情接口已支持可选鉴权上下文，并在登录场景返回 `myRating`。
5. 路由测试已覆盖成功链路与关键错误路径。
6. 评分写接口已支持单用户 upsert，并可在详情接口返回最新 `myRating`。
7. `stall/rating` 仓储已新增 MySQL 实现：覆盖食堂/窗口查询、评分 upsert 与用户评分分页。
8. 运行时装配已支持在 MySQL 可用时自动切换到持久化仓储。
9. 可选鉴权中间件已优化：当请求携带异常 Authorization 时，`GET /stalls/{stallId}` 回退游客态而非直接 401。

### 4.2 下一步计划（Next Steps）

1. 补充 `stall/rating` MySQL 仓储集成测试（并发 upsert、分页稳定性与精度边界）。
2. 将当前 AutoMigrate 策略升级为显式 migration 文件，提升生产发布可控性。
3. 补充可选鉴权行为的路由测试（异常 Authorization 请求头下仍返回详情与 `myRating=null`）。

### 4.3 待优化事项（Optimization Backlog）

1. 抽离 `stall` 专用 cursor helper，减少列表游标逻辑分散。
2. 扩展 `sort` 维度（如 `hot_desc`）并同步文档与测试。
3. 补充 service/repository 层单元测试，降低当前对路由级测试的依赖。
4. 将可选鉴权策略在 `api-spec.md` 与模块文档中进一步明确（游客回退 vs 严格拒绝）。

### 4.4 风险与注意事项（Risks / Watchouts）

1. 当前仍保留内存回退策略（MySQL 初始化失败回退），生产环境需配合启动门禁。
2. 评分写入已接入 MySQL 持久化，需重点关注索引与并发热点下的事务表现。
3. 排行榜模块尚未接入，当前 `score_desc` 反映实时聚合评分但未引入独立排行缓存层。
4. 可选鉴权回退游客态提升兼容性，但也需结合风控日志观察异常头部流量。
