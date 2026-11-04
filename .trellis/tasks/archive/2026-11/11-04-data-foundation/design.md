# Phase 2 技术设计

沿用 modernc／goose／sqlc，新增 00002 核心领域表，不重写历史迁移。常规 CRUD 与固定投影由 sqlc 生成；FTS 特殊语句若生成器不识别，放独立 .fts.sql 并由 database/search 固定 SQL 参数化调用，保持受测而非绕过。

Store 持有进程文件锁和单写连接。启动检测 goose 版本，有待执行迁移时先 VACUUM INTO 一致快照，完成后检查数据库；未来版本拒绝打开。迁移失败关闭并解锁，保留备份。RestoreSnapshot 只创建新目标，不覆盖活动库。每份快照临时文件＋同步＋原子改名，权限 0600，目录 0700。

文章入库在事务内解析别名与稳定身份；所有候选别名必须指向同一文章，碰撞明确拒绝，状态独立表不受刷新覆盖。分类关系通过有效成员视图统一判定 manual 优先。时间毫秒、ID 不透明、sort_at 首次写定。

外部内容 FTS 与 articles.rowid 关联，触发器维护增改删。CJK 单字／二字有前缀 token，混合字面查询编译成绑定 MATCH，候选有上限并做连续片段过滤。LibraryService 以安全 Result 暴露列表／状态，UI 不直接接触 SQL，Query 失败有错误态。未来 Phase 4 在稳定仓储上加完整分页／搜索交互。
