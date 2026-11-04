# 领域数据基础实现契约

## 1. 范围

Phase 2 从最小 app_settings 扩展到核心阅读领域。保留 00001，00002 创建 Feed／Article／Identity／Category／多来源成员／Read／Favorite／Media／ReaderContent 与约束，00003 建外部内容 FTS5 及触发器。AI／对话／MCP 表仍按后续阶段交付。

## 2. 签名

database.Open(ctx,filename)→Store；Store.Backup(ctx,destination)；RestoreSnapshot(ctx,source,destination)。Open 持有 filename.lock 的进程锁直到 Close。普通查询由 sqlc v1.30.0 从 00001／00002 生成；FTS 虚表迁移不送入 sqlc，特殊搜索语句在受测固定 SQL 中绑定参数。

LibraryService.List({limit})→{ok,data:{items,has_more},error?,request_id}，默认 200、最大 500；列表没有完整正文，摘要最多 500 字符。Get({article_id})→{article_id,content_text}，正文按选择单独读取。SetRead／SetFavorite({article_id,value}) 返回同目标及实际成功值，状态可重开恢复。生成绑定位于 internal/library，不手改。

## 3. 契约

所有连接 foreign_keys／WAL／busy_timeout=5000，单写连接。goose 逐迁移事务执行；发现旧库有待执行迁移时先生成 filename.backups/before-v<version>-<UTC>.db。未来 schema 拒绝打开。迁移失败不返回 Store，关闭连接并释放实例锁，保留旧内容和升级前快照；多文件迁移按各自事务，不保证整个批次回退为旧版本。

VACUUM INTO 生成包含 WAL 已提交数据的一致快照；临时文件完整性检查、文件同步，再以同目录硬链接 no-replace 原子发布，Unix 目录同步。备份 0600、创建目录 0700；已有目标拒绝覆盖。恢复只创建新目标，同样验证 quick_check／foreign_key_check，绝不覆盖活动库。

身份按同 Feed GUID／规范化 URL／指纹关联，保存历史别名；所有别名须指向同一文章，GUID 同时出现不同 URL 与不同内容则拒绝覆盖。跨 Feed 独立。sort_at 首次写定，状态独立表不被正文刷新重置；归档订阅保留文章，相同 URL 恢复原 Feed。

分类同父级名称（trim＋小写）唯一，根级独立唯一索引；数据库触发器拒环及超过五级（含移动子树）。有效成员视图优先 manual，Feed included 或达到分类阈值的 AI included 次之。Feed 默认分类可显式选择是否应用历史，更新仅改 feed 来源，保留手动墓碑。删除分类移子项至原父级，同名冲突事务回滚，不删除文章。

外部内容 FTS 由触发器同事务维护，支持完整性核验及重建。CJK 单字／二字用“单”／“双”前缀，混合英文分段用“词”前缀；MATCH 由字面编译器构造绑定参数，最多 512 Unicode 字符、1000 候选并返回截断标志，中文连续片段再校验。权重 5／2／2／1／1，100k 性能未认证；无全文 UI 或 rank 游标交付承诺。

Shell 现在从真实库读取，最多显示最新 500 篇；has_more 时明确提示已加载范围，不冒充全库计数。完整分页和服务端筛选在 Phase 4 接入。正文 Query 按 ID 隔离；列表读取失败显示重试，不能静默显示空库。状态修改每个文章／字段串行落盘，失败回滚到最后真实保存值，旧失败不覆盖新意图。测试 fixture 仍只在测试目录。

## 4. 错误

业务边界：非法长度／数量→ValidationError，缺失文章→NotFound，取消→Cancelled，超时→Timeout，其他 I/O→安全 DatabaseError，原始 SQL／路径不返回 UI。仓储身份碰撞→ErrIdentityConflict，事务回滚。第二实例→ErrDatabaseInUse；未来版本／迁移／快照／完整性失败→启动失败且无可写仓储；目标存在→拒绝覆盖。迁移失败后的恢复由调用方显式选新目标，不自动替换原库或删除备份。无法创建一致快照的文件系统明确失败，不降级复制活跃主文件。

## 5. 案例

正常：旧设置库升级前保留 v1 快照，主题仍在。基础：新库直接创建核心表与 FTS。异常：迁移先 CREATE TABLE 再执行错误 SQL，该文件回滚，旧数据和快照仍可读取。

## 6. 验证

真实 modernc 驱动的恢复测试覆盖旧库升级、WAL 备份／新路径恢复、目标保护、进程锁、故障迁移 DDL 回滚、未来版本拒绝。设置重开与 race 继续通过；领域仓储、身份／分类与 FTS 行为已由真实驱动测试补齐。测试全在临时目录，包含 7 项 database、2 项 LibraryService、5 项 settings；Go race 通过，前端新增串行状态／读取失败／按需正文用例。最终门禁见[阶段记录](../09-research/phase2-validation.md)。

## 7. 实现约束

错误：人工改历史迁移、活跃 WAL 只复制主文件、失败后仍提供写服务。正确：增量版本、受锁升级、一致快照、显式新目标恢复和真实故障测试。不要把这一基础阶段说成 RSS 已抓取或完整搜索 UI 已交付。
