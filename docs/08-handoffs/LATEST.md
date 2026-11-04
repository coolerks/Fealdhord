# 最近交接：Phase 1 正式阅读 Shell

日期：2026-11-04。分支：codex/reader-shell。当前任务 reader-shell 代码与检查完成，正在提交归档。

## 本阶段交付

正式阅读空间已替换诊断首页，诊断保留于设置／关于。真实虚拟 List 与 Masonry 共用 Query；三栏／紧凑 Reader、玻璃侧栏、主题／布局设置、命令面板、键盘、可拖动及键盘分隔条、ID 锚点与 Reader 位置恢复均已实现。设置与命令按需加载，生产不注入假文章。

偏好通过 SettingsService.Get／Update、SQLite app_settings、goose／sqlc、revision CAS 持久化；保存失败明确显示，队列合并连续更改。首份工作提交 5a6d949 为持久化部分。

## 真实证据与限制

前端 19 项、真实 SQLite 设置 5 项及 race、Go 全包 test／vet、typecheck、lint 返回成功、macOS arm64 原生构建通过。没有重复 E2E／截图。详见[Phase 1 验收](../09-research/phase1-validation.md)。Virtualizer Compiler 兼容警告与 Vite 首包体积提示保留，未声称跨平台／帧率／十万条验收。

## 实际契约

阅读[Shell](../04-specs/reader-shell.md)与[偏好](../04-specs/preferences.md)。生成绑定不手改。query key 当前为完整空集合；后续分页必须纳入查询参数。领域文章不进 Zustand，SQLite 是偏好唯一持久事实。锚点恢复期间不得记录顶部；防抖退出需冲刷。设置冲突显式重载，不能默默覆盖。

## 下一阶段

Phase 2 数据基础：在已有设置迁移之后补 Feed／Article／Category／State／Media／FTS schema 与 sqlc 查询，验证真实驱动 FTS5、唯一／外键约束、迁移失败、事务回滚和备份恢复。Phase 3 接 RSS 与安全网络／首图，Phase 4 接完整领域操作；当前收藏／已读 fixture 仅验证共享缓存，不代表落盘。

## 工作区与持续授权

主会话独立完成，禁止子代理，inline 不变。已有 .agents／.claude／.codex／.pi 和 Trellis 框架配置保持原样且不静默纳入本阶段提交。功能优先，非必要不做 E2E，完成部分直接本地提交，不重复征询、不自动推送。构建与 Go 检查顺序运行，避免 dist 被重建时的嵌入资源竞争。
