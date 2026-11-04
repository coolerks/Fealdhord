# 数据库、迁移与身份

状态：已评审代码契约；Phase 0 已有基础工程，涉及其余领域的规则仍待对应阶段实现。

## 1. 范围与触发条件

新增字段、查询、迁移、FTS 或事务时。

## 2. 签名

`UNIQUE(feed_id,dedup_key)`；`PRIMARY KEY(article_id,category_id,source)`；`source IN (manual,feed,ai)`。

## 3. 契约

database/sql + modernc 候选，goose 迁移，sqlc 常规查询；时间 UTC 毫秒，API RFC3339。写事务不调用网络；每连接 foreign_keys，WAL 和有界 busy_timeout；原始 DDL 待 Phase 2 编写。

## 4. 校验与错误矩阵

重复身份→复用 ID；manual excluded→保持排除；迁移失败→禁止写库；过期 revision→Conflict。

## 5. 良好、基础和异常案例

良好：重刷不变收藏；基础：空库迁移；异常：GUID 碰撞隔离，不覆盖不同文章。

## 6. 必需测试

真实 Go 驱动验证 FTS5、唯一／外键、回滚、迁移故障、FTS 同步、并发去重和备份恢复。

## 7. 错误与正确做法

错误：活跃 WAL 只复制主 db，或人工改表。正确：一致备份＋版本化迁移＋恢复演练。

权威详细设计：[架构](../../../docs/03-architecture/database.md)。

## Phase 1 真实依据

最小 app_settings 已提前实现：migrations/00001_ui_preferences.sql，goose v3.28.0／modernc SQLite v1.61.1，sqlc v1.30.0 生成 internal/database/sqlc。SavePreferences(value,revision,updatedAt) 使用 WHERE revision CAS，返回 RowsAffected 是否为 1。驱动 foreign_keys／WAL／busy_timeout=5000 通过 DSN 每连接设置，单连接；禁止手改生成查询。其余身份／文章／分类／FTS DDL 仍在 Phase 2。完整签名与错误矩阵见[偏好契约](../../../docs/04-specs/preferences.md)。
