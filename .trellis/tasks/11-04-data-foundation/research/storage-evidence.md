# 存储研究依据

SQLite 官方 [VACUUM](https://www.sqlite.org/lang_vacuum.html) 确认 INTO 生成事务一致快照；不使用活跃 WAL 主文件复制。 [FTS5](https://www.sqlite.org/fts5.html) 确认外部内容触发器及 integrity-check rank=1 可对照内容表。中文二字／单字方案沿用 docs/09-research/sqlite-fts5-notes.md，但本轮必须以真实 modernc 驱动验证，不沿用 Python 探针充数。

实例锁采用跨平台 gofrs/flock 的 TryLockContext，不用永久 PID 文件代替进程锁。非 FTS SQL 由固定 sqlc v1.30.0 生成，FTS 例外固定 SQL 与绑定参数保留独立测试。
