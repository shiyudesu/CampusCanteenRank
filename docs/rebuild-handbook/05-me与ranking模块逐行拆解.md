# 05 me 与 ranking 模块逐行拆解

本章目标：你可以独立实现个人中心查询与排行榜查询，并理解缓存装饰器模式。

主文件：

1. server/internal/dto/me/me_dto.go
2. server/internal/service/me/me_service.go
3. server/internal/controller/me/me_handler.go
4. server/internal/model/ranking/ranking.go
5. server/internal/dto/ranking/ranking_dto.go
6. server/internal/repository/ranking/repository.go
7. server/internal/repository/ranking/mysql_ranking_repository.go
8. server/internal/repository/ranking/cached_repository.go
9. server/internal/service/ranking/ranking_service.go
10. server/internal/controller/ranking/ranking_handler.go
11. server/internal/testkit/repositories.go（测试夹具）

---

## 1. me_service.go 逐行拆解

### 1.1 ListMyComments

关键流程：

1. userID 必须 > 0（未登录直接 unauthorized）。
2. 规范化 limit（默认 20，最大 50）。
3. decode cursor。
4. comments.ListByUser。
5. 收集作者和 replyToUser 的 userID。
6. 通过 users.GetByIDs 批量拉取昵称。
7. 批量计算 likedByMe。
8. 组装 CommentItem。
9. 生成 nextCursor。

你会发现它复用了 comment 模块的数据结构，这是常见的“跨模块复用 DTO”策略。

### 1.2 ListMyRatings

关键流程：

1. userID 校验。
2. decodeMyRatingCursor。
3. stalls.ListUserRatings。
4. 收集 stallID 并通过 stalls.GetStallsByIDs 批量补名称。
5. 生成 nextCursor（updatedAt + stallId）。

游标字段为什么是 updatedAt + stallId：

1. 用户可能多次改分，updatedAt 能反映最新操作时间。
2. stallId 作为稳定 tie-breaker。

### 1.3 decodeMyRatingCursor / encodeMyRatingCursor

校验点：

1. stallId > 0。
2. updatedAt 非零时间。

---

## 2. ranking_service.go 逐行拆解

### 2.1 ListRankings 参数校验

这是最容易面试被问到的地方。

校验规则：

1. scope 默认 global，只允许 global/canteen/foodType。
2. scope != global 时 scopeId 必须 > 0。
3. scope == global 时 scopeId 必须是 0。
4. days 默认 30，只允许 7/30/90。
5. sort 默认 score_desc，只允许 score_desc/hot_desc。
6. limit 默认 20，最大 50。

### 2.2 游标处理

游标 payload：

1. sortValue
2. lastActiveAt
3. stallId

为什么不用只有 id：

1. 排行排序不仅按 id。
2. 需要携带当前排序主键值。

### 2.3 结果构造

repo 返回 ranking item 后，service 负责：

1. 给每条数据补 rank（当前页内序号）。
2. 根据最后一条和 sort 构建 nextCursor。

---

## 3. ranking repository 与缓存装饰器

### 3.1 repository/ranking/repository.go

接口核心是 ListRankings(options)。

options 通常包含：

1. limit
2. cursor
3. filter(scope/scopeId/foodTypeId/days/sort)

重构后说明：

1. 生产 ranking repository 只保留接口与 MySQL 实现。
2. 测试中的内存 ranking 仓储迁移到 internal/testkit。

### 3.2 mysql_ranking_repository.go

关键点：

1. 构建动态 where 条件。
2. 支持 scope 和 foodType 的组合筛选。
3. 支持 days 时间窗口。
4. 支持 score_desc/hot_desc 排序。
5. 按游标追加翻页条件。

这是项目中 SQL 相对复杂的文件之一。

### 3.3 cached_repository.go（重点）

这是典型的 Decorator 模式：

1. 持有 next repository（真实数据源）。
2. ListRankings 先查 redis。
3. miss 时查 next，再写缓存。
4. 提供 InvalidateRankingCache 批量失效。

优势：

1. 不改业务 service 代码就能加缓存。
2. 缓存逻辑可独立测试。

---

## 4. handler 层

### 4.1 me_handler

都是强制鉴权路由：

1. 从上下文拿 userId。
2. 解析 limit/cursor。
3. 调 service 返回。

### 4.2 ranking_handler

通常是公开查询路由：

1. 解析 query 参数。
2. 调 ranking service。
3. 统一响应。

---

## 5. 易错点

1. scope=global 却传了 scopeId>0。
2. days 允许任意值导致 SQL 不可控。
3. hot_desc 下 cursor 仍用 avgRating。
4. 缓存键没包含完整过滤参数导致脏数据。
5. me/ratings 用 createdAt 做游标造成排序错乱。

---

## 6. 本章完成判定

你满足以下条件算掌握：

1. 能解释 ranking 参数约束为什么这么严格。
2. 能画出 cached repository 的调用链图。
3. 能手写 me/comments 与 me/ratings 的分页逻辑。