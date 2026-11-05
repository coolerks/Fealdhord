# 后端代码规范索引

状态：2026-11-04 整体规范已评审；Phase 0 基础调用链、原生阅读体验及 Phase 2 领域数据基础有实际源码和验证，其余领域规范仍需后续阶段落地。用户要求中文优先于旧模板的 English 提示。

| 规范 | 内容 |
| --- | --- |
| [后端目录与依赖边界](directory-structure.md) | 对应行为与验证门禁 |
| [数据库、迁移与身份](database-guidelines.md) | 对应行为与验证门禁 |
| [安全正文与登记资源](reader-guidelines.md) | Reader HTML、提取、缓存、取消与本地资源边界 |
| [分类管理与导航](navigation-guidelines.md) | 分类 CAS、删除预览、全库计数及历史继承范围 |
| [全库文章查询与游标](article-query-guidelines.md) | 分页、FTS 版本、候选边界及纯文本命中范围 |
| [受控网络与 Feed 解析](network-guidelines.md) | SSRF、压缩限额与三格式解析契约 |
| [错误与取消](error-handling.md) | 对应行为与验证门禁 |
| [日志与执行审计](logging-guidelines.md) | 对应行为与验证门禁 |
| [后端质量与验证](quality-guidelines.md) | 对应行为与验证门禁 |

相关产品契约以 [文档入口](../../../docs/README.md) 为索引；若后续实现改变契约，同阶段修改对应规格。禁止子代理，codex.dispatch_mode 保持 inline。
