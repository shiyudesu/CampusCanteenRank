# 数据迁移说明（server/migrations）

本目录用于存放后续显式 SQL 迁移脚本，替代运行时 `AutoMigrate`。

## 当前状态

- 已提供首个基线迁移：
  - `0001_init_schema.up.sql`
  - `0001_init_schema.down.sql`
- 现阶段仓库中的 MySQL 仓储仍保留 `AutoMigrate` 兜底能力，后续可在 CI/CD 完成迁移接入后逐步移除。

## 迁移文件命名建议

- `0001_init_schema.up.sql`
- `0001_init_schema.down.sql`
- `0002_add_comment_like_index.up.sql`
- `0002_add_comment_like_index.down.sql`

## 最小落地流程（建议）

1. 新增一组 `up/down` SQL 文件并通过本地 MySQL 演练。
2. 在 PR 描述中写明回滚路径（对应 `down` 文件）。
3. 合并后将 CI/CD 接入迁移执行步骤，再去除运行时自动建表依赖。

## 集成测试环境说明

- 仓储层 MySQL 集成测试依赖环境变量 `MYSQL_DSN`。
- 当 `MYSQL_DSN` 未配置或数据库不可达时，测试会按设计自动 `skip`。
- 建议在 CI 中提供独立测试库 DSN，确保集成测试持续执行。

## 0001 基线覆盖范围

- `users`
- `canteens`
- `food_types`
- `stalls`
- `ratings`
- `comments`
- `comment_likes`
