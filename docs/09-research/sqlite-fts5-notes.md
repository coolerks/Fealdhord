# SQLite、查询生成与全文检索调研

核验日期：2026-11-04。

## 官方事实

[FTS5](https://www.sqlite.org/fts5.html)提供 unicode61、trigram 等 tokenizer；trigram 的全文匹配不能命中不足三个 Unicode 字符的子串。外部内容索引需维护同步。[modernc SQLite](https://pkg.go.dev/modernc.org/sqlite)是无 CGo 驱动，不能据此推断整个 Wails 应用无原生依赖。[sqlc 配置](https://docs.sqlc.dev/en/v1.31.1/reference/config.html)支持 sqlite engine 与迁移输入；[goose](https://github.com/pressly/goose)支持版本化 SQL 迁移。

## 项目决定

采用 database/sql + modernc.org/sqlite 候选、goose 迁移、sqlc 生成非 FTS 查询。英文使用 unicode61；中文索引前生成确定性的二字连续片段，单字独立 token 保证短查询召回，存入专用 `cjk_terms` 列。混合查询由编译器按字面词生成 MATCH，不开放任意原始语法。增加正文片段校验减少二字集合假阳性。单字检索成本、索引体积与排序均须实验，不承诺相关性已达标。

trigram 可作未来子串索引替代，不作为解决所有中文查询的现成方案。驱动、sqlc 处理 FTS 虚表与事务、FTS5 编译能力必须在 Phase 2／4 以实际 Go 项目验证。FTS 特殊查询若生成器不支持，封装在 database/search，白名单固定 SQL 与绑定参数，保留同等测试。

## 已执行离线语义探针

运行 `python3 docs/09-research/probes/sqlite_search_probe.py`，Python sqlite3 3.54.1 内存库：unicode61 对句中“并发”零命中，trigram 双字零命中、三字命中；显式二字／单字 token 命中。三来源独立行能保留 manual excluded。全部断言通过。探针 source／运行输出可重现；不是 modernc 或 100k 性能结果。中文索引拟用“单”／“双”前缀区分单字／二字 token，产品实现必须测完整 Unicode、连续性过滤与中英混合。
