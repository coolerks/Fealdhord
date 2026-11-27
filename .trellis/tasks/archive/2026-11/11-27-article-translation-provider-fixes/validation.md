# 修复与验证

实现提交为 `3439e50`。

| 需求 | 回归证据 |
| --- | --- |
| TR-01 | `TestAlibabaACS3SignAndBatchGeneralRouting` 使用两个官方响应形状和数字顶层状态码。 |
| TR-02 | `TestProviderCredentialsFollowHTTPSOriginAndCommit` 覆盖同源、换域名与端口、旧 HTTP 地址、写库失败和旧密钥回收。`TestTranslationClientRejectsCredentialRedirects` 验证拦截跨来源和 HTTPS 降级。 |
| TR-03 | `TestIncompleteMTSummaryRemainsRetryable` 验证仅标题可展示、部分或空响应不完成、旧缓存和后台候选仍可重试。 |
| TR-04 | `TestMTBodyRestartsAtMissingFragmentWithMarkup` 用真实数据库重开服务，验证只请求缺失片段并还原链接和代码。`TestMTInlineMarkersRejectDuplicatesAndReordering` 与 `TestMTBodyRejectsDroppedMarkersAndRepairsLegacyCache` 验证占位符拒绝和旧缓存修复。 |
| TR-05 | `TestBaiduSingleRetriesShareTheServiceRateLimit` 验证标准版的真实请求间隔及取消后没有补译请求。 |
| TR-06 | 阅读器组件验证 AI 模型缺配置时的两个入口和机器翻译缺少凭据时的文章 AI 设置入口。 |
| TR-07 | `TestMTBodyDoesNotRestoreTranslationAfterClear` 从服务入口执行全目标语言清除，验证晚到结果不写回或发布。 |
| TR-08 | `TestWorkerContinuesAfterUnsupportedArticle` 用临时数据库确认后台下一轮处理较旧的合格文章。 |

Go 全量、相关包竞态和 `go vet ./...` 通过。阅读器和文章 AI 设置 45 项组件测试通过。前端类型检查、lint 和生产构建通过，保留现有虚拟列表 lint 与构建体积警告。

前端默认并行全量首跑 233/235 通过，动态图标帧数和会话菜单各一项失败。两项单独复跑均通过，`npm test -- --maxWorkers=3 --retry=1` 全量 235 项通过。

验证使用模拟厂商响应、内存密钥库和临时 SQLite。真实厂商凭据、桌面实机和发布待后续验证。
