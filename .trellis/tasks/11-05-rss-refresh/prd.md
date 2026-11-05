# 订阅持久化与刷新服务

## 目标与范围

实现 Phase 3 的独立交付：internal/database、internal/feed、internal/refresh、main 与生成绑定。整体需求与范围见[父任务](../11-05-phase3-rss/prd.md)，保留暂停视觉调整、中文文档、主会话执行和分部分提交授权。

## 验收

整源提交与 validator 原子性、304 不解析、状态不重置；订阅 CRUD revision／归档恢复；刷新合并、限流、退避、取消和关机。具体策略沿用父任务设计与既有 RSS／安全／媒体契约，不把其他子任务规划写成本任务现有能力。

## 依赖与边界

执行前置：rss-ingestion 完成并通过安全／解析验收后。不新增 AI、OPML、完整分页或视觉重设计。当前规划完成，待父阶段最终评审。
