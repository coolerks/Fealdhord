# Phase 2：领域数据基础与一致恢复

## 目标

用户已授权原生体验修订完成后自动进入下一阶段。当前 00001 已保存偏好，本任务补齐阅读器核心领域模型，替换生产假空入口为真实 SQLite 查询，为 Phase 3 RSS 接入提供稳定仓储。

## 范围与需求

- DB-01：Feed／Article／Identity／Category／多来源成员／ReadingState／Favorite／Media／ReaderContent 的版本化 DDL、外键、唯一及有效值约束、排序索引、sqlc 常规查询。
- IDENTITY-01：同 Feed 别名合并、跨 Feed 隔离、稳定文章 ID／sort_at，重复更新不重置阅读／收藏／手动排除；冲突原子拒绝。
- CAT-01：手动排除优先于 Feed／AI，来源独立；分类拒环／深度超限／同级重名。
- SEARCH-02：真实 modernc FTS5，外部内容触发器，中文单字／二字 token 与字面查询，更新／删除／回滚／重建一致。
- RECOVERY-01：启动实例锁、旧库升级前一致快照、故障阻止写入并保留原库／备份，备份可恢复到新路径，不覆盖用户当前库。
- DATA-01：有界文章读取与状态操作 Go 服务，Shell 从真实库读取，空库仍为真实空态；失败明确显示，不静默当空集合。

## 验收

真实临时 SQLite 测试空库与旧库升级、WAL 快照恢复、错误迁移回滚、锁竞争、外键／唯一、身份并发、分类来源、FTS 增改删与中文标点。类型／lint／Go test／race／vet／原生构建及中文文档核验通过；不重复原生 E2E。RSS 抓取、分类编辑 UI、OPML 与 AI 不在本阶段。
