# RSS 与刷新规格

范围：订阅 CRUD、发现、RSS／Atom／JSON Feed、更新、去重和首图；Phase 3 交付。媒体细则见 [媒体设计](../02-design/article-media.md)。

## 行为

输入 Feed URL 时先受控抓取并解析，确认后提交；输入网页 URL 时解析 link rel=alternate，列出多个候选供选择，支持 RSS／Atom 与 application/feed+json。无候选明确提示，不静默添加网页。规范化 scheme／host／默认端口与 fragment，不改写可能有意义的 query。重复 URL 返回已有订阅；已归档则可恢复。

编辑名称／默认分类不更换 ID；改 Feed URL 需重新验证并保留旧文章身份，不隐式覆盖为其他站点。删除为归档，停止后台抓取，文章／收藏／搜索保留；确认文案说明影响。永久擦除另行设计。

刷新使用条件 HTTP；同 Feed 同时最多一个任务，手动刷新与定时刷新合并，返回共同 operation 快照。默认 30 分钟唤醒到期 Feed；网络并发上限 4，每站点上限 2；429 遵循 Retry-After，失败指数退避至 24 小时并加抖动，手动刷新可重试但受站点限流。动态活跃度调整在基线稳定后开启，不改变人工刷新能力。

仅完整解析和事务提交后更新 ETag／Last-Modified。304 更新 last_checked_at／last_success_at、清错误计数，不改文章 fetched_at；解析失败不覆盖旧数据及 validator。停机补查每源一次，不把全部错过的时点回放。取消中止未提交工作，保留已完成 Feed 并显示部分成功。

缺标题显示“无标题”；发布日期缺失用 fetched_at 生成稳定 sort_at；坏日期不导致整源失败。GUID／URL／内容指纹的去重和身份别名规则见数据库规格。解析正常空源是成功，不是错误。

## 验收

- RSS-01：无模型配置时可添加、编辑、归档、恢复、刷新；失败保留原订阅与文章。
- RSS-02：RSS／Atom／JSON Feed fixture 各解析正确，重复刷新不增加条数，不重置已读／收藏／手动排除。
- RSS-03：304 不调用 Parser；200 解析失败不保存新 validator；网络错误有中文提示与重试时间。
- RSS-04：网页多 Feed 显示候选，无候选有真空态；危险 URL／重定向被拒。
- RSS-05：同时刷新不重复写；取消报告完成与未完成；重启不触发积压洪峰。
- MEDIA-01：首图按优先级与安全策略提取，失败不阻塞入库；详细用例见媒体设计。


## Phase 3 实施记录

2026-11-05：已实现 netpolicy 有界抓取与 Feed 本地字节解析、HTML alternate 发现及纯文本清理；相关网络和三格式测试通过。订阅服务、原子刷新、调度、首图及功能接线继续实施，不将解析库可用等同完整 RSS 路径交付。


刷新核心已实现：FeedService 本地 DTO／订阅管理、事务内 article upsert 与整源提交、RefreshService 活动批次合并、4／2 并发边界、304、解析失败保留 validator、Retry-After／退避与取消。00004_feed_refresh 新增 last_error_code，使限流跨重启仍可执行。当前尚未装配到 Wails／前端，媒体后续接入，不宣称完整用户路径已交付。
