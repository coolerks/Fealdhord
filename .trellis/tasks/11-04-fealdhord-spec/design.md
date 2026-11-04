# 技术设计摘要

## 定位与范围

本任务输出需求与技术规格；应用实现等待后续评审批准。详细权威契约在 docs，本文件是规划入口，不复制全文。

## 架构与边界

React → Wails DTO → Application Services → Domain／Runtime → Repository／受控网络。业务自有 Model／Tool／Skill 协议；三 Adapter、MCP 与凭据只在各自边界。依据 [总架构](../../../docs/03-architecture/overview.md)。

## 数据流与契约

SQLite 事实源；Query 管领域快照，Zustand 管 UI。刷新事务后发失效事件；媒体异步；模型工具参数完成且通过 Schema／权限后执行。事件可丢，快照可重查。详见 [数据模型](../../../docs/03-architecture/database.md)、[Wails](../../../docs/05-api/wails-bindings.md)、[模型](../../../docs/05-api/model-contract.md)、[工具](../../../docs/05-api/tool-contract.md)、[事件](../../../docs/05-api/events.md)。

## 重要取舍

Go 基线提升 1.26；纯 Go SQLite 候选不消除原生 GUI 依赖；分类来源独立存证；共享虚拟双视图；默认不额外抓网页补图；生产不明文降级凭据；MCP 依 SDK 版本协商。12 份拟定 ADR 记录替代方案与代价。

## 兼容性、迁移与恢复

无旧产品 schema 需迁移，但新工程从首版使用 goose。单实例锁、迁移前一致备份、失败禁止写入。后端与 DTO 用版本化契约，设置旧枚举安全回退。五平台矩阵与原生 WebView 验证不能用本机浏览器替代。

## 研究未知与阶段门禁

本机 doctor 失败、实际 Go SQLite FTS5／sqlc、中文索引体积与召回、动态 lanes 锚点、凭据平台与 MCP 互操作均未通过产品测试；分别阻塞对应阶段验收，不阻塞本轮规格交付。详见 RISKS 与研究文件。

## 运维与回滚

日志显式白名单脱敏；长操作可取消且保留已提交事实。外部写未知结果不重放，重启 run 为 interrupted。发布回退走兼容备份／前向修复，不使用未验证破坏性 Down。
