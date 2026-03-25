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
- 持久化模式下优先装配 MySQL ranking 仓储；当 MySQL/Redis 初始化失败或配置缺失时回退内存仓储，保持可用性优先

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
6. `scope/sort` 参数在服务层已统一做空白裁剪（trim），降低调用方参数容错成本。
7. `ranking` 仓储已新增 MySQL 持久化实现，并在 `server/cmd/api/main.go` 持久化模式下完成装配切换。
8. 新增显式 migration 基线文件：`server/migrations/0001_init_schema.up.sql` 与 `0001_init_schema.down.sql`，覆盖当前核心业务表结构。
9. 排行查询已接入 Redis 缓存包装仓储：持久化模式下默认按参数维度缓存 30 秒，Redis 不可用时自动降级为直连仓储。
10. 排行缓存失效策略已扩展到写路径：`comment like/unlike` 成功后也会触发 `InvalidateRankingCache`，与评分写入、评论发布行为保持一致。
11. 启动流程已接入 `server/internal/migration`：MySQL 连接成功后先执行 `server/migrations/*.up.sql`，再初始化仓储，进一步收敛运行时自动建表依赖。
12. `ranking` MySQL 集成测试已补齐：新增真实 SQL 路径下的筛选（`scope/scopeId/foodTypeId`）、`hot_desc` 排序、`days` 时间窗与跨页游标稳定性覆盖。
13. 修复 MySQL ranking 仓储的 `hot_score` 字段语义：无论 `score_desc` 还是 `hot_desc`，返回体中的 `hotScore` 均统一按热度公式计算，避免字段值受排序模式影响。
14. 完成 ranking MySQL 查询构建重构第一阶段：将 `ListRankings` 拆分为“聚合构建/基础查询构建/筛选/游标/排序与映射”多个私有函数，并引入排序策略抽象；在不改变 SQL 语义与返回结构前提下降低函数复杂度，便于后续扩展更多排序模式。

### 4.2 下一步计划（Next Steps）

1. 增加游标浮点边界测试（同分值/高并发分页）并验证跨页稳定性。
2. 将当前“全量前缀失效”策略演进为更细粒度 key 失效，降低高写入场景缓存抖动。
3. 评估 `hot_score` 聚合表达式的可观测性（慢查询与执行计划），为后续索引与查询优化提供基线数据。
4. 在排序策略抽象基础上继续推进第二阶段：把排序表达式、游标比较表达式与 order 子句收敛到独立策略对象，减少后续新增排序时的改动面。

### 4.3 待优化事项（Optimization Backlog）

1. 将 ranking 的排序值/游标构造进一步抽象为公共 helper。
2. 补充 days 时间窗对真实行为的聚合计算（当前内存仓储用于接口闭环）。
3. 增加更细颗粒 service/repository 边界测试（极端同分、空结果、多页边界）。
4. 在持久化仓储落地后评估 sortValue 归一化策略，避免浮点精度导致游标抖动。
5. 为拆分后的查询构建函数补充更细粒度单测（策略选择、cursor where 拼接、filter 组合），降低后续重构回归风险。

### 4.4 风险与注意事项（Risks / Watchouts）

1. ranking 已接入 MySQL 聚合查询路径，需持续关注高并发场景下的查询成本与索引命中。
2. 当前缓存失效使用前缀级批量删除，极端高写入场景需关注 Redis SCAN + DEL 频率与延迟抖动。
3. 排行游标包含浮点排序值，需重点验证持久化路径下的精度与跨页一致性。
4. 启动期迁移执行依赖 `server/migrations` 脚本完整性；若脚本缺失或执行失败会回退内存模式，需在 CI/CD 中提前发现并阻断。
