# Ranking 模块实现说明（阶段五）

## 1. 本次实现范围

本次在 `me` 模块后，补齐了 P0 缺失的 `ranking` 领域能力，新增并接入以下接口：

1. `GET /api/v1/rankings?scope=&scopeId=&foodTypeId=&days=&sort=&limit=&cursor=`

并补充了：

- 独立的 `ranking` 分层结构（model/dto/repository/service/controller）
- 排行列表游标分页（稳定排序键 + cursor）
- 多维筛选能力（`scope/scopeId/foodTypeId/days/sort`）
- 路由与服务测试，覆盖成功链路、筛选与关键错误参数

---

## 2. 关键实现点

### 2.1 模块分层

- `server/internal/model/ranking/ranking.go`
- `server/internal/dto/ranking/ranking_dto.go`
- `server/internal/repository/ranking/repository.go`
- `server/internal/service/ranking/ranking_service.go`
- `server/internal/controller/ranking/ranking_handler.go`

### 2.2 路由装配

在 `server/internal/router/router.go` 新增：

- `GET /api/v1/rankings`

并在 router 构造中接入 `ranking` 仓储与 service。

### 2.3 参数与规则对齐

- `scope`：`global|canteen|foodType`
- `scopeId`：当 `scope!=global` 时必填且 >0；`global` 场景必须为 0
- `days`：仅允许 `7|30|90`（默认 30）
- `sort`：仅允许 `score_desc|hot_desc`（默认 `score_desc`）
- `limit`：默认 20，最大 50

### 2.4 游标分页语义

- 响应统一：`items/nextCursor/hasMore`
- 游标携带：`sortValue + lastActiveAt + stallId`
- 同值下通过 `lastActiveAt + stallId` 做稳定去重翻页

### 2.5 启动装配兼容

- `server/cmd/api/main.go` 的仓储构造函数已扩展 ranking 仓储返回值
- 当前 ranking 先使用内存仓储实现，保持与现有持久化回退策略兼容

---

## 3. 测试与验证结果

本次新增测试：

- `server/internal/repository/ranking/repository_test.go`
  - 覆盖分页去重与筛选行为
- `server/internal/service/ranking/ranking_service_test.go`
  - 覆盖成功链路与参数错误语义
- `server/internal/router/router_test.go`
  - 新增 `TestRankingEndpoints`，覆盖：
    - 首屏与游标翻页
    - `score_desc` 与 `hot_desc`
    - `scope` 与 `foodTypeId` 筛选
    - 非法参数返回 `40001`

验证命令与结果由本次交付流程统一执行：

1. `go test ./server/...`
2. `go vet ./server/...`
3. `go build ./server/...`

---

## 4. 模块开发状态同步（用于会话切换无缝衔接）

> 你每次开发完，都要同步更新开发文档，写上你已经完成的功能，下一步要做的，待优化的等等内容，为之后切换对话仍然可以无缝衔接开发做准备。

### 4.1 已完成功能（Done）

1. `GET /api/v1/rankings` 已可用，支持分页与多维筛选。
2. ranking 参数语义已与 `api-spec.md` 对齐（scope/scopeId/days/sort/limit/cursor）。
3. ranking 路由已接入统一响应与错误码风格。
4. 排行游标分页已具备稳定翻页语义（避免重复项）。
5. ranking 模块基础测试与路由级测试已补齐。

### 4.2 下一步计划（Next Steps）

1. 增加 MySQL ranking 仓储实现与集成测试，替换当前内存排名数据源。
2. 引入 Redis 缓存层（按 scope/days/sort 维度）并补充失效策略。

### 4.3 待优化事项（Optimization Backlog）

1. 将 ranking 的排序值/游标构造进一步抽象为公共 helper。
2. 补充 days 时间窗对真实行为的聚合计算（当前内存仓储用于接口闭环）。
3. 增加更细颗粒 service/repository 边界测试（极端同分、空结果、多页边界）。

### 4.4 风险与注意事项（Risks / Watchouts）

1. 当前 ranking 仍是内存实现，不具备生产级实时一致性与持久化能力。
2. 若后续接入持久化查询，需要同步治理索引与查询成本。
3. 排行缓存未接入前，高频查询场景会把压力集中在数据库层。
