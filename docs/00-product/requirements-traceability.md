# 原始需求逐章追踪

来源：[v2.0 不可变原文](source-requirements-v2.md)。108 个原始章节 + 五十五-A 至 E，共 113 项。原文中的示例 Markdown 标题不计章节。完整字段见 [机器可核验清单](requirements-manifest.json)。

下表是章节级覆盖索引，不意味着一个章节只有一个行为；细项以目标规格内行为／错误／验收为准。验收 ID 可跨文档引用，统一由 docs 下对应 ID 定义。Phase 0 工具链与基础调用、Phase 1 正式 Shell 与偏好恢复已实现，其余应用功能未实现；单项当前状态见机器清单，不能把基础链路当完整领域功能。源第七十三节的多 AI 调度被根目录禁止子代理约束取代，仅保留交接文档。

| 需求 ID | 原节 | 内容 | 权威规格 | 验收 ID | 阶段 |
| --- | --- | --- | --- | --- | --- |
| REQ-001 | 一 | 产品原则 | [规格](product-principles.md) | RSS-01 | 全阶段 |
| REQ-002 | 二 | 目标平台 | [规格](../09-research/wails-v3-notes.md) | ENV-01 | Phase 0 |
| REQ-003 | 三 | 技术栈 | [规格](../03-architecture/overview.md) | ARCH-01 | Phase 0 起 |
| REQ-004 | 四 | 不要过度依赖 SDK | [规格](../03-architecture/ai-provider.md) | MODEL-01 | Phase 5 |
| REQ-005 | 五 | AI API 协议 | [规格](../05-api/model-contract.md) | MODEL-01 | Phase 5 |
| REQ-006 | 六 | 明确不支持旧 Completions API | [规格](../05-api/model-contract.md) | MODEL-01 | Phase 5 |
| REQ-007 | 七 | Responses API | [规格](../05-api/model-contract.md) | MODEL-01 | Phase 5 |
| REQ-008 | 八 | Messages API | [规格](../05-api/model-contract.md) | MODEL-01 | Phase 5 |
| REQ-009 | 九 | 统一 Model Interface | [规格](../05-api/model-contract.md) | MODEL-01 | Phase 5 |
| REQ-010 | 十 | 统一 Streaming Event | [规格](../05-api/model-contract.md) | MODEL-01 | Phase 5 |
| REQ-011 | 十一 | Provider 配置系统 | [规格](../04-specs/providers.md) | PROVIDER-01 | Phase 5 |
| REQ-012 | 十二 | 兼容第三方 Provider | [规格](../04-specs/providers.md) | PROVIDER-01 | Phase 5 |
| REQ-013 | 十三 | Model 管理 | [规格](../04-specs/providers.md) | PROVIDER-01 | Phase 5 |
| REQ-014 | 十四 | 任务可以使用不同模型 | [规格](../04-specs/providers.md) | PROVIDER-01 | Phase 5 |
| REQ-015 | 十五 | Agent Runtime | [规格](../03-architecture/agent-runtime.md) | AGENT-01 | Phase 7 |
| REQ-016 | 十六 | Agent 最大循环限制 | [规格](../03-architecture/agent-runtime.md) | AGENT-01 | Phase 7 |
| REQ-017 | 十七 | Tool 是核心能力抽象 | [规格](../04-specs/tools.md) | TOOL-01 | Phase 7／8 |
| REQ-018 | 十八 | Built in Tools | [规格](../04-specs/tools.md) | TOOL-01 | Phase 7／8 |
| REQ-019 | 十九 | Tool 权限系统 | [规格](../04-specs/tools.md) | TOOL-02 | Phase 7／8 |
| REQ-020 | 二十 | 危险操作确认 | [规格](../04-specs/tools.md) | TOOL-02 | Phase 7／8 |
| REQ-021 | 二十一 | Tool Execution History | [规格](../04-specs/tools.md) | TOOL-01 | Phase 7／8 |
| REQ-022 | 二十二 | MCP | [规格](../04-specs/mcp.md) | MCP-01 | Phase 10／11 |
| REQ-023 | 二十三 | MCP Client | [规格](../04-specs/mcp.md) | MCP-01 | Phase 10／11 |
| REQ-024 | 二十四 | MCP Tool Integration | [规格](../04-specs/mcp.md) | MCP-01 | Phase 10／11 |
| REQ-025 | 二十五 | MCP 权限 | [规格](../04-specs/mcp.md) | MCP-01 | Phase 10／11 |
| REQ-026 | 二十六 | MCP Server | [规格](../04-specs/mcp.md) | MCP-03 | Phase 10／11 |
| REQ-027 | 二十七 | Skill 系统 | [规格](../04-specs/skills.md) | SKILL-01 | Phase 9 |
| REQ-028 | 二十八 | Skill 目录 | [规格](../04-specs/skills.md) | SKILL-01 | Phase 9 |
| REQ-029 | 二十九 | Skill 安全 | [规格](../04-specs/skills.md) | SKILL-02 | Phase 9 |
| REQ-030 | 三十 | Skill UI | [规格](../04-specs/skills.md) | SKILL-01 | Phase 9 |
| REQ-031 | 三十一 | AI Chat | [规格](../04-specs/chat.md) | CHAT-01 | Phase 5／7 |
| REQ-032 | 三十二 | Reader AI Panel | [规格](../04-specs/chat.md) | CHAT-01 | Phase 5／7 |
| REQ-033 | 三十三 | Context Builder | [规格](../04-specs/chat.md) | CHAT-01 | Phase 5／7 |
| REQ-034 | 三十四 | RSS 数据结构 | [规格](../03-architecture/database.md) | DB-01 | Phase 2／3 |
| REQ-035 | 三十五 | Feed | [规格](../03-architecture/database.md) | DB-01 | Phase 2／3 |
| REQ-036 | 三十六 | Article | [规格](../03-architecture/database.md) | DB-01 | Phase 2／3 |
| REQ-037 | 三十七 | RSS 格式 | [规格](../04-specs/rss.md) | RSS-02 | Phase 3 |
| REQ-038 | 三十八 | Feed Discovery | [规格](../04-specs/rss.md) | RSS-04 | Phase 3 |
| REQ-039 | 三十九 | Feed 更新机制 | [规格](../04-specs/rss.md) | RSS-03 | Phase 3 |
| REQ-040 | 四十 | 自动分类 | [规格](../04-specs/categories.md) | CAT-01 | Phase 2／4／6 |
| REQ-041 | 四十一 | 分类来源 | [规格](../04-specs/categories.md) | CAT-01 | Phase 2／4／6 |
| REQ-042 | 四十二 | Manual Override | [规格](../04-specs/categories.md) | CAT-01 | Phase 2／4／6 |
| REQ-043 | 四十三 | AI Classification | [规格](../04-specs/ai-classification.md) | AI-01 | Phase 6 |
| REQ-044 | 四十四 | AI Classification Cache | [规格](../04-specs/ai-classification.md) | AI-01 | Phase 6 |
| REQ-045 | 四十五 | 精选 | [规格](../04-specs/smart-views.md) | VIEW-01 | Phase 4／6 |
| REQ-046 | 四十六 | Prompt 测试 | [规格](../04-specs/ai-classification.md) | AI-03 | Phase 6 |
| REQ-047 | 四十七 | 搜索 | [规格](../04-specs/search.md) | SEARCH-01 | Phase 4 |
| REQ-048 | 四十八 | Reader | [规格](../04-specs/reader.md) | READER-01 | Phase 4 |
| REQ-049 | 四十九 | 阅读体验 | [规格](../04-specs/reader.md) | READER-03 | Phase 4 |
| REQ-050 | 五十 | 主界面 | [规格](../02-design/app-shell.md) | SHELL-01 | Phase 1 |
| REQ-051 | 五十一 | Sidebar 毛玻璃效果 | [规格](../02-design/sidebar.md) | SIDEBAR-01 | Phase 1／4 |
| REQ-052 | 五十二 | 视觉风格 | [规格](../02-design/design-system.md) | DS-01 | Phase 1 |
| REQ-053 | 五十三 | Theme | [规格](../02-design/design-system.md) | DS-01 | Phase 1 |
| REQ-054 | 五十四 | Sidebar 信息架构 | [规格](../02-design/sidebar.md) | SIDEBAR-01 | Phase 1／4 |
| REQ-055 | 五十五 | Article List | [规格](../02-design/article-list.md) | LIST-01 | Phase 1／4 |
| REQ-055-A | 五十五-A | 双阅读视图：列表与瀑布流 | [规格](../02-design/article-views.md) | VIEWS-01 | Phase 1／4 |
| REQ-055-B | 五十五-B | RSS 首图提取与媒体管理 | [规格](../02-design/article-media.md) | MEDIA-01 | Phase 3／4 |
| REQ-055-C | 五十五-C | 统一 Motion Design System | [规格](../02-design/motion-system.md) | MOTION-01 | Phase 1 起 |
| REQ-055-D | 五十五-D | Hover 与图标语义动效矩阵 | [规格](../02-design/motion-system.md) | MOTION-01 | Phase 1 起 |
| REQ-055-E | 五十五-E | 动画、图片与瀑布流的性能和测试 | [规格](../07-quality/performance.md) | VIEWS-03 | Phase 1／4／12 |
| REQ-056 | 五十六 | 状态管理 | [规格](../03-architecture/frontend.md) | VIEWS-01 | Phase 1／4 |
| REQ-057 | 五十七 | Wails Boundary | [规格](../05-api/wails-bindings.md) | API-01 | Phase 0 起 |
| REQ-058 | 五十八 | Go Backend Modules | [规格](../03-architecture/backend.md) | ARCH-01 | Phase 0 起 |
| REQ-059 | 五十九 | 数据库 Migration | [规格](../03-architecture/database.md) | DB-01 | Phase 2 |
| REQ-060 | 六十 | Secret Storage | [规格](../09-research/credential-notes.md) | PROVIDER-02 | Phase 5 |
| REQ-061 | 六十一 | OPML | [规格](../04-specs/opml.md) | OPML-01 | Phase 4 |
| REQ-062 | 六十二 | 错误体验 | [规格](../05-api/wails-bindings.md) | API-01 | 全阶段 |
| REQ-063 | 六十三 | Cancellation | [规格](../03-architecture/data-flow.md) | AGENT-01 | Phase 3 起 |
| REQ-064 | 六十四 | Logs | [规格](../07-quality/security-checklist.md) | LOG-01 | 全阶段 |
| REQ-065 | 六十五 | 多 AI 协作文档体系 | [规格](../README.md) | DOC-01 | 规格阶段起 |
| REQ-066 | 六十六 | docs/README.md | [规格](../README.md) | DOC-01 | 规格阶段起 |
| REQ-067 | 六十七 | 00-product | [规格](vision.md) | DOC-01 | 规格阶段起 |
| REQ-068 | 六十八 | 01-status | [规格](../01-status/CURRENT.md) | DOC-01 | 规格阶段起 |
| REQ-069 | 六十九 | ROADMAP.md | [规格](../01-status/ROADMAP.md) | DOC-01 | 规格阶段起 |
| REQ-070 | 七十 | BACKLOG.md | [规格](../01-status/BACKLOG.md) | DOC-01 | 规格阶段起 |
| REQ-071 | 七十一 | RISKS.md | [规格](../01-status/RISKS.md) | DOC-01 | 规格阶段起 |
| REQ-072 | 七十二 | active Task 文档 | [规格](../01-status/active/TASK-0001-fealdhord-spec.md) | DOC-01 | 规格阶段起 |
| REQ-073 | 七十三 | 多 AI Soft Ownership | [规格](../README.md) | DOC-01 | 规格阶段起 |
| REQ-074 | 七十四 | COMPLETED.md | [规格](../01-status/COMPLETED.md) | DOC-01 | 规格阶段起 |
| REQ-075 | 七十五 | 02-design | [规格](../02-design/design-system.md) | DOC-01 | 规格阶段起 |
| REQ-076 | 七十六 | 03-architecture | [规格](../03-architecture/overview.md) | DOC-01 | 规格阶段起 |
| REQ-077 | 七十七 | ADR | [规格](../06-decisions/README.md) | DOC-01 | 规格阶段起 |
| REQ-078 | 七十八 | 08-handoffs | [规格](../08-handoffs/LATEST.md) | DOC-01 | 规格阶段起 |
| REQ-079 | 七十九 | 09-research | [规格](../09-research/README.md) | DOC-01 | 规格阶段起 |
| REQ-080 | 八十 | 根目录 AGENTS.md | [规格](../../AGENTS.md) | DOC-01 | 规格阶段起 |
| REQ-081 | 八十一 | 新 Agent 工作协议 | [规格](../README.md) | DOC-01 | 规格阶段起 |
| REQ-082 | 八十二 | 文档不得脱离代码 | [规格](../README.md) | DOC-01 | 规格阶段起 |
| REQ-083 | 八十三 | 测试 | [规格](../07-quality/testing-strategy.md) | DOC-01 | 对应实现阶段 |
| REQ-084 | 八十四 | AI Adapter Contract Tests | [规格](../07-quality/testing-strategy.md) | MODEL-01 | 对应实现阶段 |
| REQ-085 | 八十五 | RSS Fixture Tests | [规格](../07-quality/testing-strategy.md) | RSS-02 | 对应实现阶段 |
| REQ-086 | 八十六 | 跨平台检查 | [规格](../07-quality/platform-matrix.md) | RELEASE-01 | Phase 0／12 |
| REQ-087 | 八十七 | 性能 | [规格](../07-quality/performance.md) | VIEWS-03 | Phase 4／12 |
| REQ-088 | 八十八 | Accessibility | [规格](../02-design/interaction.md) | INPUT-01 | Phase 1 起 |
| REQ-089 | 八十九 | 键盘操作 | [规格](../02-design/interaction.md) | INPUT-01 | Phase 4 |
| REQ-090 | 九十 | Command Palette | [规格](../02-design/interaction.md) | INPUT-01 | Phase 4 |
| REQ-091 | 九十一 | Settings | [规格](../02-design/settings.md) | SETTINGS-01 | 对应功能阶段 |
| REQ-092 | 九十二 | 项目目录建议 | [规格](../03-architecture/backend.md) | ARCH-01 | Phase 0 起 |
| REQ-093 | 九十三 | README | [规格](../../README.md) | DOC-01 | 规格阶段起 |
| REQ-094 | 九十四 | Git Hygiene | [规格](../07-quality/repository-hygiene.md) | REPO-01 | Phase 0 起 |
| REQ-095 | 九十五 | 代码质量 | [规格](../07-quality/testing-strategy.md) | REPO-01 | 全阶段 |
| REQ-096 | 九十六 | 错误处理 | [规格](../07-quality/testing-strategy.md) | API-01 | 全阶段 |
| REQ-097 | 九十七 | 不要提前复杂化 | [规格](scope.md) | DOC-01 | 全阶段 |
| REQ-098 | 九十八 | 第一阶段实施顺序 | [规格](../01-status/ROADMAP.md) | SHELL-01 | Phase 0 起 |
| REQ-099 | 九十九 | 第一次执行必须做什么 | [规格](../01-status/ROADMAP.md) | SHELL-01 | Phase 0 起 |
| REQ-100 | 一百 | 第一批 ADR | [规格](../06-decisions/README.md) | DOC-01 | 规格阶段 |
| REQ-101 | 一百零一 | UI 第一版验收 | [规格](../02-design/app-shell.md) | SHELL-01 | Phase 1 |
| REQ-102 | 一百零二 | 代码与文档完成定义 | [规格](../07-quality/release-checklist.md) | RELEASE-01 | 每个实现阶段 |
| REQ-103 | 一百零三 | 设计修改规则 | [规格](../06-decisions/README.md) | DOC-01 | 每次重大变更 |
| REQ-104 | 一百零四 | 外部信息验证规则 | [规格](../09-research/README.md) | DOC-01 | 相关阶段实施前 |
| REQ-105 | 一百零五 | AI Provider 命名规则 | [规格](../04-specs/providers.md) | PROVIDER-03 | Phase 5 |
| REQ-106 | 一百零六 | 最终产品体验目标 | [规格](vision.md) | CHAT-01 | Phase 4–11 |
| REQ-107 | 一百零七 | 最终架构目标 | [规格](../03-architecture/overview.md) | ARCH-01 | 全阶段 |
| REQ-108 | 一百零八 | 开始执行 | [规格](../08-handoffs/LATEST.md) | DOC-01 | 规格阶段起 |

## 跨章节必须共同验收的路径

- 零 AI：RSS-01／RSS-02／CAT-01／SEARCH-01／READER-01／OPML-01／VIEWS-01。
- 新增体验：MEDIA-01 至 03／MOTION-01 至 02／VIEWS-01 至 03／INPUT-01。
- Agent 安全：MODEL-01／AGENT-01／TOOL-02／SEC-01／MCP-02 至 04／SKILL-02。
- 持续交付：DOC-01／REPO-01／RELEASE-01；阶段状态与平台证据必须真实。
