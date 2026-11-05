# 命令面板图标动画修复

状态：已提交 ad8480d 并归档。日期：2026-11-05。权威记录：[Trellis 任务](../../../.trellis/tasks/archive/2026-11/11-05-command-icon-motion/task.json)。

cmdk 可用行 data-disabled=false 被误判禁用，现已修正。真实 CommandItem＋Motion 的悬停／键盘运动、归位重播及禁用静态回归先失败再通过。全部前端 43 项、类型、lint、macOS 构建及应用包签名通过，保持已有命令和动态偏好。
