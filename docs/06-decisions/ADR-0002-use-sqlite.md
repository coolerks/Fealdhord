# ADR-0002：SQLite、驱动与 FTS5

状态：已采纳；Phase 2 真实驱动／FTS／恢复验证通过。
日期：2026-11-04。

## 背景

本地优先、离线搜索、复杂关系与可靠事务需要统一持久源。

## 决定

使用 SQLite／FTS5；modernc.org/sqlite 为首选候选，现固定 modernc v1.61.1、goose v3.28.0、sqlc v1.30.0，真实 Go 测试已通过。

## 替代方案

mattn/go-sqlite3 是可选 CGo 方案；文件 JSON 不适合事务与全文查询；云 DB 违背本地基础体验。

## 后果与验证

驱动无 CGo 不表示 Wails 无原生依赖；中文查询需单独索引方案。

依据与后续门禁：[相关规格／研究](../09-research/sqlite-fts5-notes.md)。

2026-11-04 实施证据：核心迁移、外部内容触发器、并发身份／状态、来源优先级和 VACUUM INTO 快照／恢复均有真实测试，见[Phase 2 验证](../09-research/phase2-validation.md)。100k 与跨平台性能仍未认证。
