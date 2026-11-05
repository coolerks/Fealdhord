# 图标动画失效回归修复

状态：规划待评审。日期：2026-11-05。权威任务：[任务记录](../../../.trellis/tasks/11-05-icon-motion-regression/task.json)。

已用真实 AnimatedGlyph＋Motion 复现严格模式首次 hover 不运动且 busy 卡住，移除 StrictMode 对照通过；研究候选调度也通过。实际生产组件尚未修改。

[需求与验收](../../../.trellis/tasks/11-05-icon-motion-regression/prd.md)／[根因与预防机制](../../../.trellis/tasks/11-05-icon-motion-regression/research.md)。已更新前端质量规范；正式修复后需运行真实组合回归与现有检查。
