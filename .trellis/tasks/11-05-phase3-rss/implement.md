# Phase 3 实施顺序

状态：规划已收敛，待最终评审；尚未编辑产品代码。用户已授权主会话独立开发、分部分提交和暂停视觉调整。

1. rss-ingestion：先网络安全边界，再发现、三格式解析、正文规范化。独立验收离线 HTTP／DNS／fixture；不先用普通 http.Get 绕过网关做功能。
2. rss-refresh：依赖 ingestion 通过。实现事务内 upsert、批量提交、FeedService、刷新快照／合并／取消、条件请求与到期调度，再生成 Wails 绑定。验证真实 SQLite 与可控时钟。
3. rss-media：依赖 ingestion 的网关／规范化及 refresh 提交后钩子。实现候选、隐私偏好与兼容迁移、异步探测、缓存和本地已登记媒体 handler；先通过禁图零请求及代理拒绝。
4. rss-workflow：依赖前述后端服务。接既有订阅页、菜单／工具栏刷新、取消、Query 重取及媒体；保持视觉样式和已有命令／动效行为。
5. 父任务联验 RSS-01–05／MEDIA-01–02／NET-01–02 与分类继承、重复刷新、归档恢复；记录实际限制，归档子任务和父任务，写会话日志。

每个子任务先加载其 PRD／design／implement 与对应前后端规范；inline，不调用子代理。用户批准本阶段最终方案后按顺序启动子任务，不逐个重复确认同一已批范围。若实际产品范围或破坏性兼容策略改变，再做必要评审。

检查：gofmt、相关 go test／race、go vet、sqlc generate、Wails bindings、前端 typecheck／lint／test／build 和 macOS build。构建写 dist 时不并行执行读取 go:embed 的 Go 检查。每部分通过必要检查、更新文档后直接 git commit，不自动推送。

回退点：网关先独立提交；数据事务与迁移独立提交；调度／服务独立提交；媒体及功能接线各自提交。保留生产数据与旧状态，不通过降级 schema 强行回滚。
