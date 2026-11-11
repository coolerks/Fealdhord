# Journal - moyok (Part 1)

> AI development session journal
> Started: 2026-11-04

---



## Session 1: 功能优先规则与 Phase 0 提交收尾
<!-- trellis-session: v=2 fp=b6d6d1810d4f8f02 -->

**Date**: 2026-11-04
**Task**: 功能优先规则与 Phase 0 提交收尾
**Branch**: `codex/desktop-bootstrap`

### Summary

更新 AGENTS：功能优先、非必要不做 E2E、按需单元测试、按完成部分直接提交；提交并归档 Phase 0 桌面工程。

### Main Changes

- 新增持续自动提交授权，覆盖工作流逐次确认要求
- 修复无 dist 首次绑定生成并完成本机构建

### Git Commits

| Hash | Message |
|------|---------|
| `1611640` | docs: 明确功能优先与按阶段自动提交规则 |
| `0b654c3` | feat: 建立折栈桌面基础工程与规格基线 |

### Testing

- [OK] 本轮未执行 E2E；wails3 build 与文档链接检查通过

### Status

[OK] **Completed**

### Next Steps

- 按已批准路线图进入 Phase 1，围绕实际功能编码并按完成部分直接提交


## Session 2: Phase 1 正式阅读空间与偏好恢复完成
<!-- trellis-session: v=2 fp=1b387c5456706cc8 -->

**Date**: 2026-11-04
**Task**: Phase 1 正式阅读空间与偏好恢复完成
**Branch**: `codex/reader-shell`

### Summary

完成三栏 Shell、虚拟双视图、主题与动态模式、键盘及 SQLite 偏好恢复，任务已归档。

### Main Changes

- 按部分提交 Go／SQLite 设置服务和正式前端，不注入生产假文章。

### Git Commits

| Hash | Message |
|------|---------|
| `5a6d949` | feat: 持久化阅读布局与外观偏好 |
| `d942a7c` | feat: 完成正式阅读空间与虚拟双视图 |

### Testing

- [OK] 19 项前端与 5 项真实 SQLite 设置测试通过；race、Go test／vet、原生构建通过，文档核验无错误。

### Status

[OK] **Completed**

### Next Steps

- Phase 2：补齐领域 schema、FTS5、约束与迁移／备份恢复；保留 Compiler 兼容及首包体积提示。


## Session 3: 原生菜单与灵动阅读体验重做
<!-- trellis-session: v=2 fp=cc465a393a9ff7cb -->

**Date**: 2026-11-04
**Task**: 原生菜单与灵动阅读体验重做
**Branch**: `codex/native-reading-experience`

### Summary

完成 macOS 系统菜单、Windows 应用菜单、全局右键、中性色与真实窗口材质、按需内部图标动画；继续 Phase 2。

### Git Commits

| Hash | Message |
|------|---------|
| `942ec94` | feat: 重做原生菜单与灵动阅读体验 |

### Testing

- [OK] 22 项前端、Go test／vet／设置 race、原生构建及最小本机菜单／右键检查通过。

### Status

[OK] **Completed**


## Session 4: Phase 2 领域数据基础与真实阅读接线完成
<!-- trellis-session: v=2 fp=e92f5a271f1cc083 -->

**Date**: 2026-11-04
**Task**: Phase 2 领域数据基础与真实阅读接线完成
**Branch**: `codex/data-foundation`

### Summary

在原生体验修订后自动完成核心领域迁移、稳定身份、分类优先级、FTS、一致快照恢复与真实读取／状态持久化。

### Main Changes

- 原生菜单／视觉／图标动态修订 942ec94 已单独提交归档；本阶段接通真实库，不注入生产示例。

### Git Commits

| Hash | Message |
|------|---------|
| `8253005` | feat: 建立领域数据模型与一致快照恢复 |
| `3cb0bd5` | feat: 接通领域仓储与真实阅读状态持久化 |

### Testing

- [OK] 14 项核心 Go 与 race、26 项前端、Go test／vet、类型／lint、sqlc／绑定、原生构建／本机包签名验证及文档核验通过。

### Status

[OK] **Completed**

### Next Steps

- Phase 3 RSS 安全网络、发现／解析、条件请求调度与首图；完整分页与全库搜索 UI 保持 Phase 4 边界。


## Session 5: 界面减负与三方体验规则落实
<!-- trellis-session: v=2 fp=c112414c1ac5fd4f -->

**Date**: 2026-11-05
**Task**: 界面减负与三方体验规则落实
**Branch**: `codex/interface-clarity`

### Summary

加入用户原文两条规则，删除冗余标语、说明和标签，整合工具区并修正命令面板可访问名称，保留数据与现有交互。

### Git Commits

| Hash | Message |
|------|---------|
| `85c50cb` | docs: 加入界面减负与三方体验规则 |
| `4262340` | refactor: 精简阅读界面并统一操作语义 |

### Testing

- [OK] 28 项前端、类型／lint、原生构建／应用包、文档核验通过；未新增 E2E，保留已有 Compiler 与体积提示。

### Status

[OK] **Completed**


## Session 6: 菜单分组与全界面单次图标动效
<!-- trellis-session: v=2 fp=bf7d028fe35645b7 -->

**Date**: 2026-11-05
**Task**: 菜单分组与全界面单次图标动效
**Branch**: `codex/interface-clarity`

### Summary

完成上下文菜单与图标动效一致性任务并归档；保留原有业务功能。

### Main Changes

- 右键与应用菜单分组、官方图标、末尾真实快捷键，统一命令呈现和可访问名称。
- 38 个官方图标单次播放自动归位、完成通知和取消；覆盖设置、阅读器、空态及结构图标，同源生成静态 SVG。

### Git Commits

| Hash | Message |
|------|---------|
| `38805c8` | fix: 统一图标单次播放与自动归位 |
| `9d6376d` | feat: 完善菜单分组快捷键与全局图标动效 |

### Testing

- [OK] 前端 38 项、typecheck、lint 通过；保留既有 1 项 Virtualizer Compiler 提示。
- [OK] 前端生产、macOS 构建及应用包 ad-hoc 签名通过；文档 116 文件／259 本地链接无错误。
- [OK] 未执行 E2E、原生帧率采样或 Windows 实机；手动时钟播放检查不当作帧率证据。

### Status

[OK] **Completed**

### Next Steps

- 按产品路线继续 Phase 3 RSS；保留既有未跟踪框架文件。


## Session 7: 修复 StrictMode 按需挂载图标动画锁死
<!-- trellis-session: v=2 fp=043f0760d0718a84 -->

**Date**: 2026-11-05
**Task**: 修复 StrictMode 按需挂载图标动画锁死
**Branch**: `codex/interface-clarity`

### Summary

按 trellis-break-loop 复现并修复首次悬停动画失效，补齐真实运动回归与实际规范。

### Main Changes

- 启动推迟到 passive effect 后可取消微任务，保留 StrictMode、原版动画和已有菜单。
- 记录跨层生命周期、模拟测试缺口及隐含挂载假设，更新组件／质量规范；仓库没有模板源码目录。

### Git Commits

| Hash | Message |
|------|---------|
| `bbdd6fc` | docs: 记录图标动画失效根因与真实组合回归门禁 |
| `c047117` | fix: 修复严格模式首次悬停动画被取消后锁死 |

### Testing

- [OK] 41 项前端测试、类型、lint（1 项既有提示）、macOS 构建及本地应用包签名通过。
- [OK] 38 个真实图标 StrictMode 首次 hover 运动／归位回归先失败再通过；启动前取消覆盖。
- [OK] Chrome 真实时钟观察 Settings 339／338 个不同 SVG 状态、Layers 117 个，自动归位和重播通过。

### Status

[OK] **Completed**

### Next Steps

- 继续产品路线 Phase 3；未测原生帧率与 Windows 实机，不将浏览器探针当作全平台认证。


## Session 8: 修复命令面板图标禁用误判
<!-- trellis-session: v=2 fp=bb3ab8b5ff7a94af -->

**Date**: 2026-11-05
**Task**: 修复命令面板图标禁用误判
**Branch**: `codex/interface-clarity`

### Summary

排除 cmdk data-disabled=false 的禁用误判，恢复命令行悬停和键盘选中动画。

### Main Changes

- 仅修正共享禁用选择器，保持真正禁用状态、StrictMode 和命令行为。

### Git Commits

| Hash | Message |
|------|---------|
| `ad8480d` | fix: 恢复命令面板可用选项的图标动画 |

### Testing

- [OK] 真实 cmdk＋Motion 两项回归先失败再通过；全部前端 43 项、类型、lint 通过。
- [OK] macOS 构建、应用包签名与文档验证通过；保留既有 Virtualizer 提示，未扩大 E2E。

### Status

[OK] **Completed**

### Next Steps

- 后续按产品路线推进，命令面板回归使用真实控件属性。


## Session 9: 完成 Phase 3 RSS 订阅与文章同步
<!-- trellis-session: v=2 fp=195bc3bf1d017469 -->

**Date**: 2026-11-05
**Task**: 完成 Phase 3 RSS 订阅与文章同步
**Branch**: `codex/interface-clarity`

### Summary

完成安全抓取与三格式解析、订阅事务和刷新调度、受控媒体及既有界面接线；父任务与四个子任务已归档。

### Main Changes

- 订阅发现、显式选择、添加编辑、已有分类关联、归档恢复、刷新取消与图片策略可用；暂停视觉调整。

### Git Commits

| Hash | Message |
|------|---------|
| `24bd2a8` | feat: 实现受控网络抓取与 RSS 三格式解析 |
| `0f02953` | feat: 实现订阅仓储与原子刷新调度核心 |
| `9a37105` | feat: 实现受控首图管线与 RSS 桌面服务装配 |
| `264c3df` | feat: 接通订阅管理刷新与首图设置并完成 RSS 阶段 |

### Testing

- [OK] Go 全包 test／vet、相关七包 race、前端 54 项与类型／lint、Wails production build、macOS 调试签名和文档静态检查通过；未新增 E2E。

### Status

[OK] **Completed**

### Next Steps

- Phase 4：完整分页／全库搜索筛选、分类管理与按源导航、OPML、阅读增强。


## Session 10: Phase 4 基础阅读器完成
<!-- trellis-session: v=2 fp=062c3eee6a95cb34 -->

**Date**: 2026-11-06
**Task**: Phase 4 基础阅读器完成
**Branch**: `codex/interface-clarity`

### Summary

完成全库分页搜索、分类导航、安全正文与Reader提取、原生OPML迁移；修复调试包签名缓存并完成联验与五任务归档。

### Git Commits

| Hash | Message |
|------|---------|
| `17904d2` | docs: 规划 Phase 4 完整阅读器并启动全库查询任务 |
| `c1a3960` | feat: 接通全库分页搜索与跨页文章状态 |
| `a034025` | feat: 完成分类管理订阅导航与全库计数 |
| `47e695a` | feat: 完成安全正文阅读与受控网页提取 |
| `6cf26d3` | fix: 避免覆盖旧可执行文件导致调试包签名核验失败 |
| `af5589e` | feat: 完成 OPML 预览导入导出与有界刷新接线 |

### Testing

- [OK] Go全包test与vet、职责相关race、前端20文件80项、类型与lint、SQLC与Wails绑定、production构建及macOS调试签名通过；未执行E2E或其他平台实机。

### Status

[OK] **Completed**

### Next Steps

- 依据路线图规划AI基础能力；AI/Agent/Skills/MCP尚未实现。


## Session 11: Windows 标题栏与侧栏原生毛玻璃
<!-- trellis-session: v=2 fp=6dc933b95e373f43 -->

**Date**: 2026-11-11
**Task**: Windows 标题栏与侧栏原生毛玻璃
**Branch**: `codex/interface-clarity`

### Summary

Windows 11 22H2 起启用 DWM Acrylic 与透明 WebView2，自绘标题栏及侧栏共用叠色。前端 89 项、macOS 桌面包测试和 Windows x64 生产交叉编译通过，Windows 实机材质与交互待验收。Phase 7 工具运行时继续进行。

### Git Commits

| Hash | Message |
|------|---------|
| `3a04bb1` | feat: 为 Windows 标题栏与侧栏接入原生 Acrylic |

### Status

[OK] **Completed**


## Session 12: 侧栏动效与 Windows 亚克力交互修正
<!-- trellis-session: v=2 fp=aa269bbb7b04cc5d -->

**Date**: 2026-11-11
**Task**: 侧栏动效与 Windows 亚克力交互修正
**Branch**: `codex/interface-clarity`

### Summary

修正亚克力上的菜单及导航状态色，移除菜单栏底线。侧栏滚动分隔按需显示，切换只重排一次。前端 90 项测试、Chromium 布局探针和 Windows x64 交叉编译通过。

### Git Commits

| Hash | Message |
|------|---------|
| `f86ad47` | fix: 优化侧栏切换与 Windows 亚克力交互 |

### Status

[OK] **Completed**


## Session 13: 侧栏与文章区同步平推
<!-- trellis-session: v=2 fp=8fe6db2869c8434f -->

**Date**: 2026-11-11
**Task**: 侧栏与文章区同步平推
**Branch**: `codex/interface-clarity`

### Summary

Chromium 逐帧检查两侧同步水平位移、无右侧露底、宽度只变一次。前端 90 项测试及 Windows x64 交叉编译通过。Phase 7 仍在进行。

### Git Commits

| Hash | Message |
|------|---------|
| `3882147` | fix: 恢复侧栏与文章区同步平推动画 |

### Status

[OK] **Completed**


## Session 14: 阅读工具栏常驻操作
<!-- trellis-session: v=2 fp=248a6ad44880a23e -->

**Date**: 2026-11-11
**Task**: 阅读工具栏常驻操作
**Branch**: `codex/interface-clarity`

### Summary

列表和瀑布流阅读器共用可配置的常驻项与更多菜单。翻译以禁用占位呈现。

### Git Commits

| Hash | Message |
|------|---------|
| `6d412dc` | feat: 支持自定义阅读工具栏常驻操作 |

### Testing

- [OK] 前端 92 项测试、生产构建与 Go 设置测试通过。Lint 有一项既有的 TanStack Virtual 警告。

### Status

[OK] **Completed**
