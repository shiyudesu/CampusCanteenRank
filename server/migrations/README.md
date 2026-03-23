# 数据迁移说明（server/migrations）

本目录用于存放后续显式 SQL 迁移脚本，替代运行时 `AutoMigrate`。

## 当前状态

- 现阶段仓库中的 MySQL 仓储在初始化时仍会触发必要表结构初始化。
- 为了满足发布可控性，建议逐步将表结构变更迁移到本目录中的版本化 SQL 文件。

## 迁移文件命名建议

- `0001_init_schema.up.sql`
- `0001_init_schema.down.sql`
- `0002_add_comment_like_index.up.sql`
- `0002_add_comment_like_index.down.sql`

## 最小落地流程（建议）

1. 新增一组 `up/down` SQL 文件并通过本地 MySQL 演练。
2. 在 PR 描述中写明回滚路径（对应 `down` 文件）。
3. 合并后将 CI/CD 接入迁移执行步骤，再去除运行时自动建表依赖。
