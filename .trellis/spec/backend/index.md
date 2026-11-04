# 后端代码规范索引

状态：2026-11-04 整体规范已评审；Phase 0 基础调用链有实际源码和验证，其余领域规范仍需后续阶段落地。用户要求中文优先于旧模板的 English 提示。

| 规范 | 内容 |
| --- | --- |
| [后端目录与依赖边界](directory-structure.md) | 对应行为与验证门禁 |
| [数据库、迁移与身份](database-guidelines.md) | 对应行为与验证门禁 |
| [错误与取消](error-handling.md) | 对应行为与验证门禁 |
| [日志与执行审计](logging-guidelines.md) | 对应行为与验证门禁 |
| [后端质量与验证](quality-guidelines.md) | 对应行为与验证门禁 |

相关产品契约以 [文档入口](../../../docs/README.md) 为索引；若后续实现改变契约，同阶段修改对应规格。禁止子代理，codex.dispatch_mode 保持 inline。
