# 最近交接：Phase 2 数据基础

日期：2026-11-04，分支 codex/data-foundation。原生体验修订已归档；Phase 2 已提交并归档到 .trellis/tasks/archive/2026-11/11-04-data-foundation，当前无激活实现任务。

## 本阶段交付

00002 核心领域表与有效成员视图，00003 外部内容 FTS 与触发器；稳定身份别名、跨 Feed 隔离、刷新不丢状态、分类拒环／深度／删除冲突及手动优先；进程锁、升级前一致快照与新路径恢复。LibraryService 提供有界元数据、按需正文、已读／收藏，Shell 接真实库并处理失败／串行保存和回滚。生产不注入示例文章。

## 证据与限制

14 项核心 Go 测试及 race、26 项前端、全包 test／vet、typecheck／lint、生成器与原生构建通过，见[Phase 2 验收](../09-research/phase2-validation.md)。保留 1 项 Virtualizer Compiler 提示与 Vite 主包体积提示，不声称五平台／100k 性能通过；Phase 2 未增加 E2E。

当前最多载入 500 篇且明确 hasMore；完整分页／服务端筛选及全文 UI 仍属 Phase 4。后端 Search 候选 1000、输入≤512 字符；有界读取不能当完整产品搜索。正文安全纯文本，不渲染 raw HTML。

## 必须保持的契约

阅读[数据契约](../04-specs/data-foundation.md)和[原生体验](../04-specs/native-experience.md)。历史迁移不能改写；00003 不传给 sqlc。身份冲突必须回滚，不重置独立状态或稳定 sort_at。分类来源不可合并成单行，手动排除是墓碑。备份用 VACUUM INTO，不复制活跃 WAL 主文件；恢复只创建新目标，绝不覆盖活动库。

## 下一阶段

Phase 3 RSS 纵向路径：安全网络网关、Feed 发现／RSS／Atom／JSON Feed、规范化身份与去重、条件请求／调度、媒体候选与首图。沿用现有 Store 和服务，不重建数据库。没有真实 Feed 前保持空态，不用 fixture 冒充采集。

## 提交与工作协议

942ec94 原生菜单／材质／动态修订；8253005 领域迁移／一致快照，3cb0bd5 领域仓储／真实阅读接线。只本地提交，未推送。主会话独立完成，禁止子代理、inline 不变；文档中文、非必要不做 E2E、完成部分直接提交，不重复征询。已有框架配置的未知未跟踪文件保持原样。
