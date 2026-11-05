# 图标动画失效回归修复

状态：修复完成，检查通过，正在归档。日期：2026-11-05。权威任务：[任务记录](../../../.trellis/tasks/11-05-icon-motion-regression/task.json)。

已用真实 AnimatedGlyph＋Motion 复现严格模式首次 hover 不运动且 busy 卡住，移除 StrictMode 对照通过；研究候选调度也通过。生产组件已修正调度，保留 StrictMode。全部 41 项测试通过，包含 38 个图标的真实挂载／运动／归位／重播与启动前取消；Chrome 实际时钟验证通过。

[需求与验收](../../../.trellis/tasks/11-05-icon-motion-regression/prd.md)／[根因与预防机制](../../../.trellis/tasks/11-05-icon-motion-regression/research.md)。已更新前端质量规范；类型、lint 与 macOS 构建通过，原有功能回归保留。
