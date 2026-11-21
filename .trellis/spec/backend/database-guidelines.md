# 数据库、迁移与身份

状态：已评审代码契约；Phase 0 已有基础工程，涉及其余领域的规则仍待对应阶段实现。

## 1. 范围与触发条件

新增字段、查询、迁移、FTS 或事务时。

## 2. 签名

`UNIQUE(feed_id,dedup_key)`；`PRIMARY KEY(article_id,category_id,source)`；`source IN (manual,feed,ai)`。

## 3. 契约

database/sql + modernc v1.61.1，goose 迁移，sqlc 常规查询；时间 UTC 毫秒，API RFC3339。写事务不调用网络；每连接 foreign_keys，WAL 和有界 busy_timeout；核心 DDL 已在 00002／00003 实现，AI 扩展表待对应阶段。

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

## Phase 2 真实依据

Open 持有进程文件锁，有待升级时先一致快照；禁止覆盖现有备份／恢复目标，禁止失败后返回可写 Store。UpsertArticle 通过同一事务解析全部别名，冲突拒绝，不更新稳定 sort_at 或独立状态。effective_article_categories 统一 manual 优先与 AI 阈值，不能各查询重写一套优先级。FTS 特殊 SQL 仅放 database/search.go，查询参数绑定且有候选上限，触发器增改删同事务；00003 不传给 sqlc。实际签名／错误／测试详见[数据契约](../../../docs/04-specs/data-foundation.md)。


Phase 3：UpsertArticle 与 CommitFeed 复用同一个事务内 upsertArticle；整源文章、别名、FTS、继承分类与 validator 同一事务。CommitFeed 复核未归档及预期 normalized_url，避免旧 URL 请求污染新配置。304 只更新源检查状态，不更新文章或清空 validator。00004_feed_refresh 新增 last_error_code，429 的重试窗口不得被手动刷新或重启绕过；sqlc schema 列表需包含新增迁移，生成文件不手改。

通知迁移 00019：`CommitFeed` 在已有刷新基线的新增文章事务内记事件，媒体摄取继续消费原 ID 列表。通知规则游标与入队同事务，任务领取和唯一运行记录同事务，结果与待发片段同事务。外部调用在事务外，发送中断转结果未知。相关恢复及平台行为见 [通知契约](../../../docs/04-specs/notifications.md)。


## 已登记迁移的开发库修复

- 对已经分发并被 Goose 登记的版本发现结构差异时，追加有版本号的事务迁移。改写旧文件不会修复已登记版本的库。
- 通知第 21 版通过 `Open` 注册，在旧库走既有数据库快照路径后运行。检查 `pragma_table_info` 再添加字段，保留已有行和已补齐字段的值。
- 覆盖完整库与「版本已登记但对象缺失」的升级场景。回归应调用实际的受影响查询，验证历史保留及备份生成。
- 已登记版本发生结构差异时，对照同一代码构建的新库与受影响库的只读结构，覆盖表、字段、索引、视图和触发器。以受影响库结构构造回归，并用 SQLC 实际查询或 SQLite `EXPLAIN` 验证依赖；`quick_check` 通过只代表文件结构检查通过。
