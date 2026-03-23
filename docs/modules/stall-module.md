# Stall 模块实现说明（阶段一）

## 1. 本次实现范围

本次在 `auth` 模块之后，完成了后端下一部分 `stall` 领域的第一阶段闭环，新增并接入以下接口：

1. `GET /api/v1/canteens`
2. `GET /api/v1/stalls?limit=&cursor=&canteenId=&foodTypeId=&sort=score_desc`
3. `GET /api/v1/stalls/{stallId}`

并补充了：

- 独立的 `stall` 分层结构（model/dto/repository/service/controller）
- 游标编码解码基础包 `internal/pkg/cursor`
- 路由集成测试，覆盖分页、参数错误、详情查询和 not found

---

## 2. 关键实现点

### 2.1 模块分层

- `server/internal/model/stall/stall.go`
- `server/internal/dto/stall/stall_dto.go`
- `server/internal/repository/stall/repository.go`
- `server/internal/service/stall/stall_service.go`
- `server/internal/controller/stall/stall_handler.go`

当前采用内存仓储作为第一阶段实现，确保接口契约可联调、可测试。

### 2.2 游标分页

- 新增 `server/internal/pkg/cursor/cursor.go`（通用 Base64URL 游标编码/解码）
- 新增 `server/internal/pkg/cursor/cursor_test.go`
- `GET /stalls` 实际使用与 `score_desc` 一致的游标键（`avgRating + id`）

### 2.3 详情接口 myRating 行为

- 新增 `server/internal/middleware/optional_auth.go`
- `GET /stalls/{stallId}` 支持“未登录返回基础详情；登录后附带 myRating（若存在）”

---

## 3. 测试与验证结果

已完成并通过：

1. `go test ./server/...`
2. `go vet ./server/...`
3. `go build ./server/...`

新增测试覆盖：

- `server/internal/pkg/cursor/cursor_test.go`
- `server/internal/router/router_test.go`（新增 stall/canteen 相关用例）

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

### 4.2 下一步计划（Next Steps）

1. 进入 `rating` 写接口：`POST /stalls/{stallId}/ratings`，实现单用户评分 upsert。
2. 将 `stall` 模块从内存仓储升级为 MySQL 持久化仓储，并补充仓储层测试。
3. 基于评分数据回填并维护 `avgRating/ratingCount` 的一致性更新链路。

### 4.3 待优化事项（Optimization Backlog）

1. 抽离 `stall` 专用 cursor helper，减少列表游标逻辑分散。
2. 扩展 `sort` 维度（如 `hot_desc`）并同步文档与测试。
3. 补充 service/repository 层单元测试，降低当前对路由级测试的依赖。

### 4.4 风险与注意事项（Risks / Watchouts）

1. 当前内存仓储仅用于开发验证，不适合生产并发与持久化要求。
2. `myRating` 目前由内存数据模拟，后续需与真实 `ratings` 表联动。
3. 排行榜模块尚未接入，当前 `score_desc` 仅反映窗口聚合评分字段。
