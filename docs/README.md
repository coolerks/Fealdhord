# 折栈文档入口

更新日期：2026-11-04。本文档集是 v2.0 需求的首版可评审规格，Phase 0 基础工程与 Phase 1 正式阅读 Shell 已实现，RSS 与 AI 等后续能力尚未实现；“必须”表示实现契约，不代表功能已交付。

## 阅读顺序

1. 根目录 AGENTS.md 与 [当前状态](01-status/CURRENT.md)。
2. [范围](00-product/scope.md)、[需求追踪](00-product/requirements-traceability.md)。
3. [需求分析](09-research/requirements-analysis.md)、[架构](03-architecture/overview.md)、对应功能规格与 ADR。
4. [实施计划](../.trellis/tasks/11-04-fealdhord-spec/implement.md) 和 [交接](08-handoffs/LATEST.md)。

## 目录职责

| 目录 | 权威内容 |
| --- | --- |
| 00-product | 产品目标、术语、范围、原始材料和需求追踪 |
| 01-status | 当前实际状态、路线图、风险和任务镜像 |
| 02-design | 视觉、布局、交互、动效与无障碍契约 |
| 03-architecture | 边界、数据模型、安全与设计理由 |
| 04-specs | 功能行为、失败语义与验收用例 |
| 05-api | 跨层签名、载荷、事件、错误和权限契约 |
| 06-decisions | 重大技术选择及替代方案 |
| 07-quality | 测试、性能、平台和发布门禁 |
| 08-handoffs | 最近交接及有意义的阶段历史 |
| 09-research | 官方事实、环境记录、实验与待证结论 |

## 维护规则

说明文档始终中文；代码标识符、协议字段、路径和不可变原文除外。原始材料归档后不改写，派生规格优先采用本轮明确范围；冲突见 scope.md。CURRENT 是开发状态唯一权威，任务状态以 `.trellis/tasks/.../task.json` 为工作流记录，active 文档只作镜像，禁止维护两套生命周期。

新增大任务先按 Trellis 获准创建，PRD 写目标与验收，design 写方案，implement 写顺序与验证；规划评审后才进入代码实现。主会话独立研究、实现和检查，禁止子代理。

架构、Schema、API、权限或交互变化时，同阶段更新相关规格、追踪表和 CURRENT。ADR 按连续编号创建，记录背景、决定、替代与后果；重大改变通过新 ADR 替代旧 ADR，不改写历史理由。每个有意义阶段更新 LATEST，旧版本进入 history；不能把未运行的检查写成通过。

[质量门禁](07-quality/testing-strategy.md)区分文档验证、技术探针和应用验证。文档完成不等于应用完成。
