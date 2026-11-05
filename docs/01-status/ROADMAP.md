# 路线图

沿用原始 Phase 0–12 编号，避免再建立不同的 M 编号。规格已评审，Phase 0 已实现并本机验收，Phase 1 原生体验修订、Phase 2 数据基础与 Phase 3 RSS 完成，后续应用阶段未开始。

| 阶段 | 目标 | 前置依赖 | 完成标准 | 状态 |
| --- | --- | --- | --- | --- |
| 规格阶段 | 研究、契约与验收计划 | 原始指令 | 113 节映射、官方研究、文档核验、最终评审摘要 | 已评审 |
| Phase 0 | 工具链与最小桌面工程 | 规格评审批准 | 固定 Wails／Go／Node、doctor 诊断、窗口／绑定／事件往返 | 已完成（macOS arm64） |
| Phase 1 | 正式 Shell | Phase 0 | 玻璃、双布局、主题、调整宽度、图文／无图、语义动效、Reduced Motion | 已完成（组件测试／原生构建） |
| Phase 2 | 数据基础 | Phase 1 | 真实驱动、goose、sqlc、约束、FTS、迁移恢复 | 已完成（数据层／有界接线） |
| Phase 3 | RSS 纵向路径 | Phase 2 | 三格式、发现、条件请求、调度、去重、首图、分类继承 | 已完成（单元／集成／原生构建） |
| Phase 4 | 完整基础阅读器 | Phase 3 | 虚拟双视图、Reader、FTS、分类、状态、OPML、快捷键 | 已实现并通过相关检查（性能／其他平台实测待验收） |
| Phase 5 | AI 服务与基础 Chat | Phase 4 | 三 Adapter 流式／非流式 contracts、模型分配、凭据 | 未开始 |
| Phase 6 | 自动分类与精选 | Phase 5 | Prompt 测试、缓存、手动覆盖、预算和 Smart View | 未开始 |
| Phase 7 | Runtime | Phase 5–6 | 有限工具循环、Context、审批、审计、取消与恢复 | 未开始 |
| Phase 8 | 内置工具 | Phase 7 | 25 个工具 Schema、权限与真实副作用验证 | 未开始 |
| Phase 9 | Skills | Phase 8 | 声明式文件、CRUD、allowlist、版本与路径安全 | 未开始 |
| Phase 10 | MCP Client | Phase 8–9 | 两 transport、工具／资源／Prompt、权限、互操作 | 未开始 |
| Phase 11 | MCP Server | Phase 10 | 默认关闭、认证、白名单接口与资源、同等权限 | 未开始 |
| Phase 12 | 发布收敛 | Phase 0–11 | 五平台、性能、安全、打包、发布门禁 | 未开始 |

Phase 1 可使用明确标注的测试 fixture 展示动态布局，生产空库不能显示模拟文章。Phase 1 已提前交付最小 SQLite 设置表；其余领域数据库仍为 Phase 2，不把占位数据当持久化。视觉、无障碍和安全随对应阶段实施；Phase 12 不接收推迟到最后的核心 UI。
