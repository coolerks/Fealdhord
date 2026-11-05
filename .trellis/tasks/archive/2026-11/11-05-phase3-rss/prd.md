# Phase 3：RSS 订阅与文章同步

## 目标与授权

用户要求暂停 UI 调整，自动建立下一阶段任务并开发新功能，已同意创建本任务。目标是让空库获得真实订阅和文章，无 AI 配置也能添加、发现、刷新、编辑、归档与恢复。既有视觉、菜单和动效保持；只增加功能必需的表单、列表及反馈。

## 已确认事实

Phase 2 已有 SQLite 身份去重、分类继承、独立已读／收藏和 FTS，LibraryService 已接真实有界文章列表。Feed 的 validator／调度字段已存在，但没有安全网络、Parser、FeedService 或 RefreshService。当前 UpsertArticle 自带单条事务，不能直接循环并冒充整源原子提交。article_media 的 CHECK 尚不含 blocked，媒体偏好和本地媒体 handler 未实现。

产品契约沿用 docs/04-specs/rss.md、docs/03-architecture/security.md、docs/02-design/article-media.md、docs/05-api/wails-bindings.md。当前规划不把待实现能力写成完成。

## 需求与验收

- RSS-01：支持 Feed／网页 URL 发现与明确候选选择；重复添加复用订阅身份；添加、编辑名称／分类、归档、恢复和刷新可用，归档保留文章／状态。
- RSS-02：RSS／Atom／JSON Feed 规范化，缺标题／坏日期安全回退；重复刷新不增条数，不重置已读／收藏／人工分类。
- RSS-03：条件请求；304 不解析或改文章 fetched_at；整源解析及单事务提交成功后才更新 ETag／Last-Modified；失败保留原文章和 validator。
- RSS-04／NET-01/02：逐跳校验 URL、全部 DNS 候选及实际连接，拒绝本地／私网／保留地址、重绑定、危险协议和超限解压；最多 5 次重定向。
- RSS-05：同源刷新合并，网络最多 4 并发／每站点 2；取消不回滚已提交源；定时到期补查每源一次、429 Retry-After、退避最长 24 小时。
- MEDIA-01/02：按既有优先级提取、异步探测和有限回退；图片失败不阻塞文章入库；禁图零媒体请求，前端只请求已登记本地媒体 ID。
- 功能接线：真实添加的文章出现在现有阅读空间，刷新后 Query 更新；错误和操作进度明确，无示例数据、不依赖 AI。
- 必要单元／集成、真实 SQLite 事务和 HTTP 边界测试通过；相关绑定、类型、lint、构建及文档同步，分部分直接本地提交。

## 范围边界

本轮不进行配色、布局、标题栏或动效重设计；不实现 AI、MCP、Skills、OPML、完整分页／中文搜索 UI、网页正文提取或完整 HTML Reader。首图登记与受控加载属于本轮，完整阅读器增强仍为 Phase 4。网页补图默认关闭，不默认额外抓原网页。

## 交付顺序

四个子任务按 ingestion → refresh → media → workflow 串行，由主会话独立执行。整个阶段的共同验收由父任务负责；顺序与依赖见 implement.md。本阶段已按获批顺序实现并检查通过，待归档。
