# 最近交接：命令面板图标动画修复

日期：2026-11-05。分支 codex/interface-clarity。

根因是 cmdk 可用选项也有 data-disabled=false，AnimatedGlyph 按属性存在性一律视为禁用。现排除 false，保持原生 disabled、aria-disabled=true、Radix 空属性及 cmdk true 的禁用语义；原有 StrictMode 时序、自动归位和动态偏好保持。

真实 Command／CommandItem＋Motion 的两项回归在旧代码均失败，修复后悬停、键盘选中实际旋转，自动归位并重播，真正禁用项保持静态。全部前端 43 项、类型、lint（1 项既有 Virtualizer 提示）、生产前端、macOS 构建与本地应用包签名通过。规范记录真实生成属性差异，不再以手写 role 节点替代组件。

本轮只修禁用判断，不改命令、可访问名称、菜单架构或业务接口，未扩大 E2E。实际状态见[当前状态](../01-status/CURRENT.md)，原有未跟踪文件保留，不推送。

修复提交 ad8480d；任务已归档。
