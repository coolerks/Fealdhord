# Phase 2 验证记录

日期：2026-11-04，本机 macOS arm64。真实 modernc SQLite v1.61.1，goose v3.28.0，sqlc v1.30.0，gofrs/flock v0.13.1。

## 已执行证据

- database 7 项集成测试：旧库升级／一致快照、WAL 恢复、新目标保护、实例锁、错误迁移回滚、未来版本拒绝、并发身份合并和状态不丢失、分类树／优先级／删除冲突、FTS 中英／扩展 Unicode／增改删／重建／完整性／事务故障回滚／候选边界。
- LibraryService 2 项真实仓储测试：列表有界且不夹带正文、按需完整正文、状态重开恢复、安全错误／取消／目标缺失。
- settings 5 项及 race 继续通过。绑定生成实际为 3 Services／10 Methods／24 Models／2 Events。
- 前端新增连续状态反转、失败回滚、过时失败不覆盖新意图、真实读取失败重试及按需正文回归；最终 5 个文件共 26 项前端测试通过；typecheck／lint 返回成功（0 error，保留 1 项既有 Compiler 兼容提示），Go 全包 test／vet、sqlc 生成、Wails 绑定与原生构建通过。末尾仓储错误分类修正后，重新通过相关 race／vet 和最终 Go 原生编译。

## 验证边界

所有数据故障／恢复测试均在临时目录，不覆盖用户原库。当前生产库无 RSS 文章，未注入 fixture。真实读取／状态通路由 Go 临时库服务集成、前端边界 mock、绑定与原生构建分别验证；本阶段不重复原生 E2E。

前端首批最多 500 篇，明确 has_more 提示；完整分页、全库筛选／全文搜索 UI 在 Phase 4。后端 FTS 候选上限 1000、查询≤512 字符；未验证 100k 帧率／相关性、五平台或断电恢复。原生体验最小实测见 native-experience.md。

## 官方依据

[SQLite VACUUM](https://www.sqlite.org/lang_vacuum.html) 的 INTO 是一致快照；[FTS5](https://www.sqlite.org/fts5.html) 的外部内容索引需触发器维护，integrity-check rank=1 对照内容表。本轮使用实际 Go 驱动执行，不用先前 Python 探针代替应用证据。

最终 Vite 主 JS 564.21 kB（gzip 176.77 kB），保留超过 500 kB 的体积提示，未调整阈值或压制警告。Phase 2 未新增 E2E；文档、源哈希、113 项追踪和链接核验无错误。
