# 04 comment 与 like 模块逐行拆解

本章目标：你能独立实现楼中楼评论、回复列表、点赞幂等，并理解事务一致性。

主文件：

1. server/internal/model/comment/comment.go
2. server/internal/dto/comment/comment_dto.go
3. server/internal/repository/comment/repository.go
4. server/internal/repository/comment/mysql_comment_repository.go
5. server/internal/service/comment/comment_service.go
6. server/internal/controller/comment/comment_handler.go
7. server/internal/testkit/repositories.go（测试夹具）

---

## 1. Comment 数据模型必须先吃透

关键字段语义：

1. rootId
一级评论为 0；回复评论指向一级评论 id。

2. parentId
一级评论为 0；回复评论指向直接被回复的评论 id。

3. replyToUserId
回复目标用户 id，一级评论为 0。

4. likeCount
当前评论点赞总数。

5. replyCount
只有一级评论使用，表示其下回复数。

---

## 2. service/comment_service.go 逐行级讲解

### 2.1 CreateComment（最关键函数）

你按这个顺序理解每个分支：

1. userID/stallID 判定。
2. content trim 判空。
3. stall 是否存在（不存在直接 404）。
4. author 是否存在（防脏用户 id）。
5. 判定请求类型：
- 一级评论：rootId=0,parentId=0,replyToUserId=0
- 回复评论：三者都 > 0

6. 如果不是以上两类 -> 400。

7. 回复路径下做强校验：
- root 评论必须存在且是一级。
- parent 评论必须存在且同 stall。
- 如果 parent 是一级，则 parent.id 必须等于 rootId。
- 如果 parent 是回复，则 parent.rootId 必须等于 rootId。
- parent.userId 必须等于 replyToUserId。

8. 创建评论：
- 回复走 CreateReplyAndIncrementRoot（原子更新 reply_count）。
- 一级走 Create。

9. 再次查询新评论，组装返回 dto。

10. 触发 ranking 缓存失效。

这个函数是楼中楼正确性的核心。

### 2.2 ListTopLevelComments

关键步骤：

1. 校验 stallId、limit、sort。
2. stall 存在性校验。
3. decode cursor（createdAt + id）。
4. repo.ListTopLevelByStall。
5. 先收集 userID，再通过 users.GetByIDs 批量查作者昵称。
6. 批量查 likedByMe（viewerUserId > 0 时）。
7. 组装 items。
8. 根据最后一条生成 nextCursor。

### 2.3 ListReplies

流程类似一级列表，但额外点：

1. rootCommentId 必须是一级评论。
2. 需要收集作者和 replyToUser 的全部 userID。
3. 使用 users.GetByIDs 批量查昵称后再组装返回。

### 2.4 LikeComment 与 UnlikeComment

两个函数模板一样：

1. 参数校验。
2. repo.Like 或 repo.Unlike。
3. not found -> 404。
4. 触发 ranking 缓存失效。
5. 返回 liked + likeCount。

---

## 3. repository/comment/repository.go

你需要关注接口抽象而不是具体实现细节。

核心方法：

1. Create
2. CreateReplyAndIncrementRoot
3. GetByID
4. ListTopLevelByStall
5. ListRepliesByRoot
6. ListByUser
7. Like
8. Unlike
9. HasLiked
10. HasLikedBatch

HasLikedBatch 是性能关键，避免列表场景 N+1 查询。

重构后说明：

1. 生产仓储不再提供内存实现。
2. 测试中的内存实现由 internal/testkit 提供。

---

## 4. mysql_comment_repository.go（事务与幂等）

### 4.1 点赞事务

典型流程：

1. 开启事务。
2. SELECT ... FOR UPDATE 锁住目标评论行。
3. 检查是否已点赞。
4. 若已点赞，直接返回当前计数（幂等）。
5. 插入 comment_likes。
6. 更新 comments.like_count。
7. 提交事务。

### 4.2 取消点赞事务

逻辑对称：

1. 锁评论。
2. 查点赞记录。
3. 无记录时直接返回当前计数（幂等）。
4. 删除点赞记录。
5. like_count - 1（下限控制）。

### 4.3 回复创建 + 根评论计数

CreateReplyAndIncrementRoot 的重点是单事务：

1. 插入回复评论。
2. root 评论 reply_count +1。

保证两步要么都成功，要么都回滚。

---

## 5. controller/comment_handler.go

每个接口要做三件固定工作：

1. 解析 path/query/json 参数。
2. 从上下文读 userId（强制或可选）。
3. 调 service 并统一写响应。

你在 handler 不要写复杂业务判断。

---

## 6. 易错点总结

1. 回复评论时只校验 rootId，不校验 parentId。
2. 把 likedByMe 做成逐条查询导致性能抖动。
3. 点赞只插入明细，不更新 like_count。
4. 取消点赞重复调用直接报错（违反幂等）。
5. cursor 只用 createdAt 不用 id（导致同秒重复/漏数）。

---

## 7. 本章完成判定

你达到以下能力就算掌握：

1. 能口述 CreateComment 的所有校验分支。
2. 能写出点赞幂等事务的伪代码。
3. 能解释 HasLikedBatch 为什么必要。
4. 能解释 reply_count 为什么必须跟回复创建同事务。