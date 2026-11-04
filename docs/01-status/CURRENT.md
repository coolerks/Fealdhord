# 当前状态

最后更新：2026-11-04（Asia/Singapore）。

- 当前阶段：原生阅读体验修订已提交归档（942ec94）；Phase 2 数据基础开发中。
- 已交付：默认双栏／Reader 聚焦与可选三栏、真实 macOS 材质侧栏及跨平台菜单／全局右键、真实虚拟 List／Masonry、主题、分隔条、语义反馈与 Reduced Motion、设置／Command Palette／键盘及浏览位置恢复。
- 持久化：Go SettingsService＋真实 SQLite 最小 app_settings，goose 迁移／sqlc 查询、revision CAS 和串行合并保存；Phase 2 已加入核心表／FTS 迁移、实例锁和升级前一致快照，仓储与真实数据入口正在接入。
- 质量证据：前端 22 项测试、真实 SQLite 设置 5 项及 race、Go 全包 test／vet、typecheck、lint（0 error／1 Compiler 兼容提示）和 macOS 原生 build 通过。保留首包体积提示，详见阶段验收。
- 生产数据：文章入口真实空集合，fixture 仅测试使用。RSS、文章／分类／媒体持久化、首图提取、AI／Agent／Skills／MCP 尚未实现；其他平台与完整性能门禁未测。
- 下一阶段：Phase 2 数据基础，补完整领域 schema、身份／分类／状态约束、FTS5、迁移故障与恢复实验；不重复创建设置表。
- [最新验收证据](../09-research/phase1-validation.md) · [交接](../08-handoffs/LATEST.md)。

本地已有偏好服务提交 5a6d949，正式 Shell 提交 d942a7c；不自动推送。文档中文、主会话独立完成、功能优先、非必要不做 E2E、按完成部分直接提交的授权持续生效。
