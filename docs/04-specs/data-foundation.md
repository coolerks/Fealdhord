# 领域数据基础实现契约

## 1. 范围

Phase 2 从最小 app_settings 扩展到核心阅读领域。保留 00001，00002 创建 Feed／Article／Identity／Category／多来源成员／Read／Favorite／Media／ReaderContent 与约束，00003 建外部内容 FTS5 及触发器。AI／对话／MCP 表仍按后续阶段交付。

## 2. 签名

database.Open(ctx,filename)→Store；Store.Backup(ctx,destination)；RestoreSnapshot(ctx,source,destination)。Open 持有 filename.lock 的进程锁直到 Close。普通查询由 sqlc v1.30.0 从 00001／00002 生成；FTS 虚表迁移不送入 sqlc，特殊搜索语句在受测固定 SQL 中绑定参数。

## 3. 契约

所有连接 foreign_keys／WAL／busy_timeout=5000，单写连接。goose 逐迁移事务执行；发现旧库有待执行迁移时先生成 filename.backups/before-v<version>-<UTC>.db。未来 schema 拒绝打开。迁移失败不返回 Store，关闭连接并释放实例锁，保留旧内容和升级前快照；多文件迁移按各自事务，不保证整个批次回退为旧版本。

VACUUM INTO 生成包含 WAL 已提交数据的一致快照；临时文件完整性检查、文件同步，再以同目录硬链接 no-replace 原子发布，Unix 目录同步。备份 0600、创建目录 0700；已有目标拒绝覆盖。恢复只创建新目标，同样验证 quick_check／foreign_key_check，绝不覆盖活动库。

## 4. 错误

第二实例→ErrDatabaseInUse；未来版本／迁移／快照／完整性失败→启动失败且无可写仓储；目标存在→拒绝覆盖。迁移失败后的恢复由调用方显式选新目标，不自动替换原库或删除备份。无法创建一致快照的文件系统明确失败，不降级复制活跃主文件。

## 5. 案例

正常：旧设置库升级前保留 v1 快照，主题仍在。基础：新库直接创建核心表与 FTS。异常：迁移先 CREATE TABLE 再执行错误 SQL，该文件回滚，旧数据和快照仍可读取。

## 6. 验证

真实 modernc 驱动的恢复测试覆盖旧库升级、WAL 备份／新路径恢复、目标保护、进程锁、故障迁移 DDL 回滚、未来版本拒绝。设置重开与 race 继续通过；后续领域仓储、身份／分类与 FTS 行为测试随本阶段同任务补齐。测试全在临时目录。

## 7. 实现约束

错误：人工改历史迁移、活跃 WAL 只复制主文件、失败后仍提供写服务。正确：增量版本、受锁升级、一致快照、显式新目标恢复和真实故障测试。不要把这一基础阶段说成 RSS 已抓取或完整搜索 UI 已交付。
