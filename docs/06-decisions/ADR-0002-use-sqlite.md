# ADR-0002：SQLite、驱动与 FTS5

状态：拟定，待本轮规划评审；不是已完成实现。
日期：2026-11-04。

## 背景

本地优先、离线搜索、复杂关系与可靠事务需要统一持久源。

## 决定

使用 SQLite／FTS5；modernc.org/sqlite 为首选候选，实际锁版以 Phase 2 Go 探针通过为准。

## 替代方案

mattn/go-sqlite3 是可选 CGo 方案；文件 JSON 不适合事务与全文查询；云 DB 违背本地基础体验。

## 后果与验证

驱动无 CGo 不表示 Wails 无原生依赖；中文查询需单独索引方案。

依据与后续门禁：[相关规格／研究](../09-research/sqlite-fts5-notes.md)。
