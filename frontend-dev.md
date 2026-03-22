# 食堂评分系统前端开发文档（TypeScript）

## 1. 文档目标

本规范用于指导前端团队实现 **食堂评分与评价平台** Web 端，确保与后端接口契约一致，并支持以下核心能力：

- 多食堂、多窗口浏览与筛选
- 窗口评分（1~5星）
- 评论（楼中楼）
- 评论点赞
- 基于游标法的列表分页
- 排行榜（按食堂、食物类型、时间范围筛选）

> 本文档是开发约束，不是产品PRD。所有接口字段以 `api-spec.md` 为最终准绳。

---

## 2. 技术栈与工程基线

### 2.1 技术选型

- 框架：React 18 + TypeScript + Vite
- 路由：React Router v6
- 服务端状态：TanStack Query v5
- 客户端状态：Zustand
- UI组件：Ant Design v5
- 图表：Apache ECharts
- 网络请求：Axios（统一拦截器）
- 表单：React Hook Form + Zod
- 测试：Vitest + React Testing Library + Playwright（E2E）
- 代码质量：ESLint + Prettier + Stylelint + Husky + lint-staged

### 2.2 运行环境

- Node.js >= 20
- pnpm >= 9

### 2.3 目录结构（必须遵守）

```txt
web/
  src/
    app/
      router/
      providers/
    pages/
      home/
      stall-list/
      stall-detail/
      ranking/
      profile/
      auth/
    features/
      auth/
      canteen/
      stall/
      comment/
      ranking/
      rating/
    components/
      common/
      biz/
    services/
      http/
      api/
    stores/
    hooks/
    types/
    utils/
    styles/
```

---

## 3. 功能范围（前端）

## 3.1 MVP功能

1. 登录/注册/登出
2. 食堂列表 + 窗口列表（筛选）
3. 窗口详情（评分概览 + 评论区）
4. 用户对窗口评分（单用户可更新）
5. 评论区楼中楼（一级评论 + 二级回复）
6. 评论点赞（支持点赞/取消点赞）
7. 排行榜（全局、按食堂、按食物类型）

## 3.2 非MVP但预留扩展

- 评论举报
- 内容审核状态展示
- 热门评论排序（hot）
- 暗黑模式

---

## 4. 页面与交互规范

## 4.1 页面清单

1. `/login` 登录页
2. `/` 首页（热门窗口、最新评价）
3. `/stalls` 窗口列表页（筛选 + 排序）
4. `/stalls/:stallId` 窗口详情（评分、评论楼）
5. `/rankings` 排行榜页（多维筛选）
6. `/me` 个人页（我的评分、我的评论）

## 4.2 楼中楼评论交互

- 评论区默认展示一级评论列表（按 `latest` 或 `hot`）
- 每条一级评论展示：
  - 用户信息、内容、点赞数、回复数、发布时间
  - 点赞按钮（已点赞高亮）
  - “查看回复”按钮（懒加载二级回复）
- 二级回复展示：
  - `@被回复用户名` + 回复内容
  - 支持二级回复点赞
- 回复发布策略：
  - 回复一级评论：`parentId = rootCommentId`
  - 回复二级评论：`parentId = replyCommentId`，并带 `replyToUserId`

## 4.3 点赞交互规范

- 点赞采用乐观更新：点击后先更新UI，再等待接口结果
- 接口失败需回滚UI并提示错误
- 防抖：500ms 内同一评论重复点击仅发送一次请求

---

## 5. 游标分页规范（前端必须统一）

## 5.1 适用范围

- 窗口列表
- 评论一级列表
- 评论回复列表
- 排行榜列表

## 5.2 请求参数

- `limit`：单次拉取条数（默认20，最大50）
- `cursor`：上一页返回的 `nextCursor`（首屏为空）

## 5.3 响应字段

```ts
type CursorPage<T> = {
  items: T[];
  nextCursor: string | null;
  hasMore: boolean;
};
```

## 5.4 前端实现要求

- Query Key 必须包含筛选条件与 cursor 相关上下文
- 切换筛选项时重置 cursor
- infinite scroll 与 “加载更多” 两种模式统一基于同一 hook
- 禁止使用 offset/pageNo 分页

---

## 6. 状态管理与数据流

## 6.1 Query 管理

- 所有列表类数据使用 `useInfiniteQuery`
- 关键 Query Keys：
  - `['stalls', filters]`
  - `['stall-detail', stallId]`
  - `['comments', stallId, sort]`
  - `['replies', rootCommentId]`
  - `['rankings', scope, scopeId, foodTypeId, days, sort]`

## 6.2 本地状态（Zustand）

- `authStore`：token、用户信息
- `uiStore`：筛选抽屉状态、主题状态
- `draftStore`：评论输入草稿（避免刷新丢失）

---

## 7. 组件规范

## 7.1 关键业务组件

- `StallCard`
- `RatingPanel`
- `CommentComposer`
- `CommentList`
- `CommentItem`
- `ReplyList`
- `RankingFilterBar`
- `RankingTable`

## 7.2 CommentItem Props 约束

```ts
type CommentItemProps = {
  comment: CommentVO;
  onLike: (commentId: number, liked: boolean) => Promise<void>;
  onReply: (payload: ReplyPayload) => Promise<void>;
  onLoadReplies: (rootCommentId: number, cursor?: string) => Promise<void>;
};
```

---

## 8. 网络层规范

## 8.1 Axios拦截器

- 请求拦截：自动注入 `Authorization: Bearer <token>`
- 响应拦截：
  - `401` 自动尝试 refresh token
  - refresh失败 -> 清理登录态并跳转登录页

## 8.2 错误处理

- 后端错误统一通过 `code/message` 处理
- 用户可见提示由 `message.error` 统一发出

---

## 9. 性能与体验指标

- 首屏可交互时间（TTI）目标：< 2.5s（校园网络）
- 关键列表渲染：虚拟滚动（评论回复数较大时启用）
- 图片与头像：懒加载
- 评论提交成功后：局部刷新，不全页刷新

---

## 10. 测试与质量门禁

> 模块化开发强制要求：每个前端模块（例如 `features/*`、`pages/*`）开发完成后，必须先完成该模块测试并通过，再允许进入推送流程。

## 10.1 单元测试

- 覆盖 `services/api`、`hooks`、评论点赞与游标拼接逻辑

## 10.2 组件测试

- 评论楼中楼展开/收起
- 点赞乐观更新及回滚
- 游标加载更多行为

## 10.3 E2E

- 用户登录 -> 打分 -> 评论 -> 回复 -> 点赞 -> 查看排行榜

## 10.4 必过检查

```bash
pnpm lint
pnpm test
pnpm build
```

## 10.5 模块测试通过后的推送规则

- 前端模块测试通过后，需立即执行 GitHub 推送（若当前批次仅含该模块变更）
- 禁止“未测试先推送”或“测试失败仍推送”
- 建议按模块粒度提交，便于回滚与追踪

## 10.6 前端模型使用规则

- 前端开发默认使用 Gemini 3.1 Pro Preview（标识：`gemini-3.1-pro-preview`）
- 若需要组件、插件、扩展或 SDK，需自行检索官方文档并完成安装；同时记录来源与版本

## 10.7 前端 GitHub 提交信息规范

- 仅使用前缀：`feat`、`chore`、`fix`、`refactor`、`docs`
- 提交信息使用中文，格式：`<type>(可选scope): <中文描述>`
- 示例：
  - `feat(comment): 添加评论回复交互`
  - `fix(ranking): 修复筛选条件重置问题`
  - `docs(readme): 更新前端启动说明`

---

## 11. 前端开发里程碑（建议）

1. 第1阶段：脚手架 + 登录鉴权 + 基础路由
2. 第2阶段：窗口列表/详情 + 评分链路
3. 第3阶段：楼中楼评论 + 点赞 + 游标分页
4. 第4阶段：排行榜筛选 + 稳定性优化 + E2E

---

## 12. 完成定义（DoD）

- 业务功能：评分、楼中楼评论、点赞、排行榜、游标分页全部可用
- 代码质量：lint/test/build 全绿
- 联调质量：与 `api-spec.md` 100% 对齐
- 文档质量：README 包含启动方式、环境变量、调试说明
