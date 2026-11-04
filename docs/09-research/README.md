# 技术研究入口

核验日期：2026-11-04。每份记录区分官方事实、项目决定与未验证项。外部链接是核验来源，不保证未来版本永不改变；实施前涉及 API 必须重新核验。优先官方文档／仓库／release／源码，不以第三方文章作为契约。

本机检查无修改全局工具、无真实 API 调用。离线 SQLite 探针仅验证 token 与关系语义，不能证明实际 Go 驱动／产品性能。

- [ai-protocol-notes](ai-protocol-notes.md)
- [Phase 0 实际验收](phase0-validation.md)
- [需求分析与首版决策](requirements-analysis.md)
- [文档与探针核验](validation.md)
- [credential-notes](credential-notes.md)
- [environment](environment.md)
- [frontend-notes](frontend-notes.md)
- [mcp-sdk-notes](mcp-sdk-notes.md)
- [rss-reader-notes](rss-reader-notes.md)
- [sqlite-fts5-notes](sqlite-fts5-notes.md)
- [wails-v3-notes](wails-v3-notes.md)

- [SQLite 离线语义探针](probes/sqlite_search_probe.py)
- [文档核验程序](probes/validate_docs.py)

依赖维护检查：本轮已阅读官方能力资料，未批量安装。锁定每个依赖前记录精确版本、Go／Node 下界、release／提交活跃度、许可证和公告；若证据显示长期无人维护，先调研替代并新增 ADR，不直接照原文安装。
