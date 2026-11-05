# 最近交接：菜单与图标动效一致性

日期：2026-11-05。分支 codex/interface-clarity。

右键按文本编辑、文章操作、视图、应用分组，各项显示官方动画图标，真实快捷键按平台格式末尾对齐。Windows／Linux 应用菜单、命令面板复用命令呈现；CommandID、runCommand 与数据契约不变。右键目标不是当前文章时不显示 F／M，保留实际目标操作、文本快照、只读限制与焦点恢复。

AnimatedGlyph 单次播放完自动归位，持续悬停不循环，移出不截断，重复事件不排队；多 controls 等待全部完成，多段续播可取消。Provider 覆盖全局菜单并遵从动态设置，cmdk 的键盘选择也能触发。恢复 Search／BookText 原版，所有页面和结构图标接入统一播放；移除旧 AI 缩放和卡片上移。

38 个官方图标仅交互时挂载 Motion，静态 SVG 由 npm run icons:generate 从同份已审阅源生成。维护说明见 frontend/src/components/icons/README.md。代码规范记录 Promise／取消、快捷键上下文及 Portal 边界，防止再次只修首页或以固定延迟复位。

前端 38 项测试、类型、lint（0 error／1 项既有 Virtualizer Compiler 提示）、生产前端和 macOS 构建通过；bin/Fealdhord.app 已同步并通过 ad-hoc 签名验证。最大 JS 块 417.04kB、共享块 308.37kB，不将单块大小或 Motion 手动时钟测试当作总首屏体积／原生帧率认证。未运行 E2E、原生逐帧采样或 Windows 实机。

用户获得清晰菜单与自动归位；开发者复用类型化呈现和播放控制；AI Agent 保留可访问名称、分组和真实 aria-keyshortcuts。后续仍为 Phase 3 RSS，实际范围见[当前状态](../01-status/CURRENT.md)。原有框架配置与其他未跟踪文件保留，不推送。

功能提交：38805c8、9d6376d；[任务记录](../../.trellis/tasks/archive/2026-11/11-05-menu-icon-motion/prd.md)已归档。
