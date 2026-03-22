# 食堂评分系统 API 接口文档（v1）

## 1. 基础约定

## 1.1 Base URL

`/api/v1`

## 1.2 认证

- Header: `Authorization: Bearer <accessToken>`

## 1.3 通用响应

```json
{
  "code": 0,
  "message": "ok",
  "data": {}
}
```

## 1.4 错误码（示例）

- `0`：成功
- `40001`：参数错误
- `40101`：未登录或token失效
- `40301`：无权限
- `40401`：资源不存在
- `40901`：重复操作
- `50000`：服务内部错误

---

## 2. 游标分页统一规范

## 2.1 请求参数

- `limit`：默认20，最大50
- `cursor`：可选，首屏不传

## 2.2 返回结构

```json
{
  "items": [],
  "nextCursor": "eyJjcmVhdGVkQXQiOiIyMDI2LTAzLTIyVDEwOjAwOjAwWiIsImlkIjoxMjM0NX0",
  "hasMore": true
}
```

> 所有列表接口（窗口、评论、回复、排行榜）统一使用该结构。

---

## 3. 认证模块

## 3.1 注册

`POST /auth/register`

请求：

```json
{
  "email": "user@example.com",
  "nickname": "Tom",
  "password": "Pass@123456"
}
```

响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "userId": 1001
  }
}
```

## 3.2 登录

`POST /auth/login`

请求：

```json
{
  "email": "user@example.com",
  "password": "Pass@123456"
}
```

响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "accessToken": "xxx",
    "refreshToken": "xxx",
    "expiresIn": 7200,
    "user": {
      "id": 1001,
      "nickname": "Tom"
    }
  }
}
```

## 3.3 刷新Token

`POST /auth/refresh`

---

## 4. 食堂与窗口模块

## 4.1 食堂列表

`GET /canteens`

响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "items": [
      {"id": 1, "name": "一食堂", "campus": "主校区"}
    ]
  }
}
```

## 4.2 窗口列表（游标分页）

`GET /stalls?canteenId=1&foodTypeId=2&sort=score_desc&limit=20&cursor=...`

响应 `data`：

```json
{
  "items": [
    {
      "id": 101,
      "name": "川味小炒",
      "canteenId": 1,
      "foodTypeId": 2,
      "avgRating": 4.6,
      "ratingCount": 532
    }
  ],
  "nextCursor": "...",
  "hasMore": true
}
```

## 4.3 窗口详情

`GET /stalls/{stallId}`

响应包含：基础信息、评分统计、我的评分（登录后）

---

## 5. 评分模块

## 5.1 提交/更新评分

`POST /stalls/{stallId}/ratings`

请求：

```json
{
  "score": 5
}
```

规则：

- 单用户对单窗口唯一评分，重复提交视为更新

---

## 6. 评论模块（楼中楼）

## 6.1 发表评论（一级）

`POST /stalls/{stallId}/comments`

请求：

```json
{
  "content": "这家窗口今天鱼香肉丝很不错",
  "rootId": 0,
  "parentId": 0,
  "replyToUserId": 0
}
```

## 6.2 发表评论（回复）

`POST /stalls/{stallId}/comments`

请求：

```json
{
  "content": "同意，性价比很高",
  "rootId": 9001,
  "parentId": 9001,
  "replyToUserId": 1002
}
```

> 回复二级评论时，`parentId` 填二级评论ID，`rootId` 仍填一级评论ID。

## 6.3 一级评论列表（游标分页）

`GET /stalls/{stallId}/comments?sort=latest&limit=20&cursor=...`

响应 `data`：

```json
{
  "items": [
    {
      "id": 9001,
      "stallId": 101,
      "rootId": 0,
      "parentId": 0,
      "replyToUserId": 0,
      "content": "味道稳定，推荐",
      "likeCount": 12,
      "replyCount": 3,
      "createdAt": "2026-03-22T10:00:00Z",
      "author": {
        "id": 1001,
        "nickname": "Tom"
      },
      "likedByMe": true
    }
  ],
  "nextCursor": "...",
  "hasMore": true
}
```

## 6.4 回复列表（某一级评论下，游标分页）

`GET /comments/{rootCommentId}/replies?limit=20&cursor=...`

响应 `data`：

```json
{
  "items": [
    {
      "id": 9101,
      "stallId": 101,
      "rootId": 9001,
      "parentId": 9001,
      "replyToUserId": 1002,
      "content": "确实，分量也足",
      "likeCount": 2,
      "createdAt": "2026-03-22T10:03:00Z",
      "author": {
        "id": 1003,
        "nickname": "Jerry"
      },
      "replyToUser": {
        "id": 1002,
        "nickname": "Alice"
      },
      "likedByMe": false
    }
  ],
  "nextCursor": "...",
  "hasMore": true
}
```

---

## 7. 点赞模块

## 7.1 点赞评论

`POST /comments/{commentId}/like`

响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "liked": true,
    "likeCount": 13
  }
}
```

## 7.2 取消点赞

`DELETE /comments/{commentId}/like`

响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "liked": false,
    "likeCount": 12
  }
}
```

规则：

- 幂等，重复点赞/取消点赞不报错

---

## 8. 排行榜模块（游标分页）

`GET /rankings?scope=global&scopeId=0&foodTypeId=0&days=30&sort=score_desc&limit=20&cursor=...`

参数说明：

- `scope`: `global|canteen|foodType`
- `scopeId`: 当 `scope!=global` 时必填
- `foodTypeId`: 可选二次筛选
- `days`: `7|30|90`
- `sort`: `score_desc|hot_desc`

响应 `data`：

```json
{
  "items": [
    {
      "rank": 1,
      "stallId": 101,
      "stallName": "川味小炒",
      "canteenId": 1,
      "canteenName": "一食堂",
      "foodTypeId": 2,
      "foodTypeName": "川菜",
      "avgRating": 4.61,
      "ratingCount": 532,
      "reviewCount": 221,
      "hotScore": 97.4
    }
  ],
  "nextCursor": "...",
  "hasMore": true
}
```

---

## 9. 个人中心模块（游标分页）

## 9.1 我的评论

`GET /me/comments?limit=20&cursor=...`

## 9.2 我的评分

`GET /me/ratings?limit=20&cursor=...`

---

## 10. 字段与约束补充

1. 所有 ID 使用 `int64`
2. 时间字段统一 ISO8601 UTC
3. 评论内容长度：1~2000
4. 评分范围：1~5
5. `limit` 最大 50，超过按50处理
6. 空列表时：`items=[]`, `nextCursor=null`, `hasMore=false`

---

## 11. 联调验收清单

1. 游标翻页无重复、无漏项
2. 评论楼中楼关系正确（rootId/parentId/replyToUserId）
3. 点赞幂等且计数正确
4. 排行榜筛选与排序符合预期
5. 鉴权接口返回401行为一致
