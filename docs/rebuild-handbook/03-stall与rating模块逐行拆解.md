# 03 stall 与 rating 模块逐行拆解

本章目标：你可以自己写出窗口列表/详情/评分更新，并正确处理游标分页和评分聚合。

主文件：

1. server/internal/model/stall/stall.go
2. server/internal/dto/stall/stall_dto.go
3. server/internal/repository/stall/repository.go
4. server/internal/repository/stall/mysql_stall_repository.go
5. server/internal/service/stall/stall_service.go
6. server/internal/controller/stall/stall_handler.go
7. server/internal/testkit/repositories.go（测试夹具）

---

## 1. model 与 dto

### 1.1 stall model

关键字段：

1. ID, Name
2. CanteenID, FoodTypeID
3. AvgRating, RatingCount
4. Status, CreatedAt

评分相关子模型通常还会有 UserRating 结构（userId, stallId, score, updatedAt）。

### 1.2 stall dto

关注两个统一格式：

1. 列表 data：items + nextCursor + hasMore。
2. 详情 data：基础信息 + myRating（可空）。

评分提交响应要返回更新后的 avgRating、ratingCount，方便前端立即刷新。

---

## 2. service/stall_service.go 逐行级讲解

### 2.1 StallService 字段

1. repo：stall repository。
2. rankingInvalidator：可选缓存失效接口。

为什么是可选接口：

1. service 不关心缓存细节。
2. 没缓存时也能工作。

### 2.2 ListCanteens

流程很直：

1. repo.ListCanteens。
2. model -> dto 映射。
3. 返回 CanteenListData。

重点是统一错误包装：repository 错误都被转成业务错误码。

### 2.3 ListStalls（核心）

关键步骤：

1. 规范化 limit：默认 20，最大 50。
2. 规范化 sort：默认 score_desc，只接受 score_desc。
3. decodeStallCursor(cursorText)。
4. 调用 repo.ListStalls(options)。
5. 映射 items 到 dto。
6. 若 hasMore，拿最后一条生成 nextCursor。

### 2.4 decodeStallCursor / encodeStallCursor

游标结构：

1. avgRatingX100
2. id

校验点：

1. id > 0。
2. avgRatingX100 >= 0。

你重写时必须做这个校验，否则恶意 cursor 会造成异常行为。

### 2.5 GetStallDetail

步骤：

1. stallID <= 0 直接 bad request。
2. repo.GetStallByID。
3. not found -> 404。
4. 构造详情 dto。
5. 如果 userID 存在，调用 repo.GetUserRating 填充 myRating。

myRating 只有在登录态下才查，减少不必要读库。

### 2.6 UpsertUserRating（核心）

步骤：

1. 校验 userID、stallID、score 范围 1..5。
2. repo.UpsertUserRating。
3. not found -> 404 stall not found。
4. 成功后触发 rankingInvalidator（如果存在）。
5. 返回新聚合结果。

幂等语义：

1. 同一用户首次评分：ratingCount +1。
2. 同一用户再次评分：ratingCount 不变，只更新 avg。

---

## 3. repository/stall/repository.go（接口）

核心接口方法：

1. ListCanteens
2. ListStalls
3. GetStallByID
4. GetStallsByIDs
4. GetUserRating
5. UpsertUserRating
6. ListUserRatings（给 me 模块复用）

重构后说明：

1. 生产仓储已移除内存实现。
2. testkit 提供测试夹具实现，供 service/router 测试注入。

---

## 4. mysql_stall_repository.go（持久化核心）

### 4.1 列表查询

要点：

1. 过滤 status=1。
2. 支持 canteenId、foodTypeId 条件。
3. 按 avg_rating desc, id desc 稳定排序。
4. cursor 条件要和排序键一致。

典型 cursor 条件：

1. CAST(avg_rating * 100 AS SIGNED) < cursor.avgRatingX100
2. 或 CAST(avg_rating * 100 AS SIGNED) = cursor.avgRatingX100 且 id < cursor.id

新增批量查询：

1. GetStallsByIDs 支持 me/ratings 批量补全窗口名称，避免 N+1。

### 4.2 UpsertUserRating

常见做法：

1. 事务开始。
2. 查 rating 是否存在。
3. 不存在则插入；存在则更新 score。
4. 重新计算目标 stall 的 avg_rating 与 rating_count。
5. 提交事务。

重算 avg 时要注意浮点精度和数据库类型（DECIMAL 推荐）。

---

## 5. controller/stall_handler.go

重点是参数读取：

1. path 参数 stallId。
2. query 参数 limit、cursor、canteenId、foodTypeId、sort。
3. 上下文 userId（可选鉴权场景可能不存在）。

错误映射策略与 auth/comment 保持一致。

---

## 6. 你必须手写的关键片段

1. 游标 encode/decode（avgRatingX100 + id）。
2. ListStalls 的分页模板。
3. UpsertUserRating 的参数校验。
4. GetStallDetail 的 myRating 逻辑。

---

## 7. 本章完成判定

你满足以下条件算过关：

1. 能说明为什么 sort 只接受 score_desc。
2. 能自己写出 cursor 的合法性校验。
3. 能解释评分更新时 ratingCount 何时变化。
4. 能解释 why 评分后要触发 ranking 缓存失效。