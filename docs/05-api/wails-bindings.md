# Wails 应用边界契约

## 1. 范围与触发条件

React → Go 服务的所有交互。以下为折栈逻辑 API，实际 Wails 生成语法和文件路径在 Phase 0 最小实验确认；不能把本节示意当成现成绑定。

## 2. 签名

Go 服务方法形如 `ListArticles(req ListArticlesRequest) ListArticlesResult`；每个 Result 是可生成绑定的具体 DTO。服务内部调用 `List(ctx context.Context, query ArticleQuery) (ArticlePage, error)`。不向 JS 暴露可构造的 context。

| 服务 | 方法与请求 | 数据返回 |
| --- | --- | --- |
| FeedService | List；Get{id}；Discover{url}；Add{url,category_ids}；Update{id,revision,patch}；Remove{id,revision} | Feed／列表／发现候选 |
| RefreshService | Start{feed_ids?:string[]} | OperationSnapshot |
| OperationService | Get{operation_id}；Cancel{operation_id} | OperationSnapshot |
| ArticleService | List{query}；Get{id}；SetRead{ids,is_read}；SetFavorite{id,is_favorite} | ArticlePage／ArticleDetail／MutationReceipt |
| CategoryService | List；Create{parent_id?,name,...}；Update{id,revision,patch}；Remove{id,revision}；SetArticle{article_id,category_id,state}；SetFeed{feed_id,category_id,included} | 分类／MutationReceipt |
| SearchService | Search{query} | ArticlePage |
| ReaderService | StartExtract{article_id}；Get{article_id} | OperationSnapshot／ReaderContent |
| SettingsService | Get；Update{revision,patch} | SettingsSnapshot |
| ProviderService | List；Save{config,secret_input?}；StartTest{id}；Delete{id}；ListModels{provider_id}；SaveModel；AssignModel | 脱敏配置／OperationSnapshot／模型 |
| AgentService | Start{conversation_id?,scope,input,model_id?,skill_ids?}；Cancel{run_id}；Get{run_id}；ResolveAction{action_id,decision:approve或deny} | RunSnapshot／ActionReceipt |
| SkillService | List；Get；Save；SetEnabled；Delete | SkillSummary／校验结果 |
| MCPService | List；Save；Connect；Disconnect；Inspect；SetPolicy | 脱敏服务／能力快照 |
| transfer.API（已实现） | ChooseImport／ChooseExport；PreviewOPML{dialog_token}；ImportOPML{preview_id,selected,apply_existing,refresh}；ExportOPML{dialog_token}；Discard{dialog_token,preview_id} | 一次性文件能力／预览／导入报告／文件结果 |
| DataService（后续） | Backup{dialog_token} | 备份文件结果 |

文件路径来自原生对话框授予的一次性 token，前端不能任意写宿主路径；本轮只定义 API，不实现。内置工具不能调用凭据编辑或 DataService 的任意路径操作。

## 3. 请求与响应

所有 JSON 字段使用 snake_case。ID 字符串；时间 RFC3339；不传 Go int64 给 JS 作为可能超精度 ID。逻辑结果：`{ok:true,data:T,request_id:string}` 或 `{ok:false,error:AppError,request_id:string}`，两种不能同时出现。生成绑定的传输异常由 lib/api 映射为 TransportError。

`ArticleQuery={scope:{kind:all|unread|favorites|featured|feed|category,id?},search?:string,read?:all|read|unread,favorite?:boolean,sort:published_desc|published_asc|relevance,cursor?:string,limit?:number}`。limit 默认 50，范围 1..200。普通文章 cursor 绑定查询哈希、快照上界、sort_at 和 id；全文排序增加 rank 与索引 generation，索引变化返回 CursorExpired 后重查并恢复锚点。

`ArticlePage={items:ArticleSummary[],next_cursor?:string,snapshot_id:string}`；Summary 不含完整 HTML。ArticleDetail 返回清理后正文，不包含密钥、原始可执行 HTML或厂商事件。

长操作先返回 operation_id／run_id，再走事件或快照；前端先订阅再启动。变更返回 `{entity_ids,revision}`，执行后端验证后才提交。SetRead／SetFavorite 使用目标值，避免重试 Toggle 两次。

## 4. 校验与错误矩阵

| 条件 | code | 行为 |
| --- | --- | --- |
| 空 ID／非法枚举／超大输入 | ValidationError | 无副作用 |
| 目标不存在 | NotFound | 保留当前列表并提示 |
| revision 冲突 | Conflict | 重查后重新操作 |
| cursor 与查询不匹配／已失效 | CursorExpired | 清 cursor 重新查询并尝试定位原 ID |
| 未配置模型 | ConfigurationError | 配置入口，RSS 正常 |
| 网络／解析／数据库失败 | NetworkError／FeedParseError／DatabaseError | 保留已有数据 |
| 重复取消 | 成功快照 | 不重复改变终态 |
| 未授权／审批失效 | PermissionError／ActionExpired | 不执行工具 |

`AppError={code,message,retryable,operation_id?,details?}`；message 中文，details 仅安全字段。不得把原始 Go 错误或 secret 返回给 UI。

## 5. 案例

良好：List(limit=50) 返回稳定游标；基础：空库返回 items=[]；异常：SetRead 包含不存在 ID，整批验证失败，不部分更新。批量状态上限 500；超限返回 ValidationError，调用方分批并报告每批结果。

## 6. 必需测试

绑定 DTO 序列化、错误互斥、非法 scope、跨查询 cursor、取消、重启 snapshot、批量原子性、旧响应不覆盖新选择、无凭据泄漏；Phase 0 证明真实 Wails 往返，Mock 不能替代。

## 7. 错误与正确做法

错误：组件直接访问 SQLite，或把全量 HTML 列表传入浏览器。正确：窄服务、分页摘要、按需详情、统一错误、后端拥有权限和事务。

验收 API-01：DTO、分页、错误互斥、revision、绑定往返和取消按本节测试，生成器与实际前端保持一致。


## Phase 3 实际边界

internal/feed.Service 已实现 List{include_archived}、Get{id}、Discover{url}、Add{url,title,category_ids}、Update{id,revision,url,title,category_ids}、Remove／Restore{id,revision}，生成绑定位于 internal/feed/service。分类列表随 Feed Page 返回；分类 CRUD 仍属后续完整阅读器。

internal/refresh.API 白名单仅 Start{feed_ids}、Current、Get／Cancel{operation_id}，不绑定协调器 Close／StartScheduler／SetAfterCommit。快照包含 operation_id、generation、sequence、status、completed、total、results；generation 单进程按批次递增，先比较代次再比较 sequence，避免旧结果覆盖新刷新。窗口重载可用 Current 恢复本进程状态；不声称刷新操作跨进程持久化。

新增类型事件 fealdhord:refresh 与 fealdhord:library-changed；原有诊断事件不变。media.API 只绑定无路径参数的 ClearCache，ServeHTTP 由 Wails 内部路由挂载，不是可调用的前端绑定。设置增加 remote_images、media_cache_enabled、web_image_fallback；旧 JSON 默认仅订阅、开启磁盘缓存、关闭网页补图。

## Phase 3 前端消费契约

`rssAPI` 是订阅／刷新／缓存绑定的唯一适配入口；`useRSSSync` 订阅事件后恢复 Current，并仅对活动 operation_id 每 5 秒 Get，卸载清理。`useRefresh` 按 generation 优先、同代次 sequence 递增合并，关闭终态屏蔽迟到事件。`fealdhord:library-changed` 合并失效 feeds 和文章 Query，保持既有状态保存路径。所有设置仍经 SettingsService revision 串行落盘；表单编辑使用打开时 revision，不从新列表借用新 revision。

## Phase 4 查询接口（首部分）

LibraryService.Query(ArticleQuery) 已接真实数据库分页；List 保留兼容调用但应用不再使用固定500条路径。Page 增加 next_cursor／snapshot_id／search_limited；Summary 增加 feed_id／url／title_matches（UTF-16 纯文本范围）。RebuildSearch 显式重建索引并推进代次，返回 ok／error／request_id，前端随后重新查询。时间游标绑定出生代次与 rowid 上界，防 rowid 复用；全文游标额外绑定 generation／rank。游标带进程内 HMAC 和30分钟期限，重启或索引变化返回 CursorExpired。导航分类、阅读增强和 OPML 已按 Phase 4 子任务接线，实际限制见对应规格。

## Phase 4 分类导航接口

navigation.Service 暴露 List／Save／PreviewDelete／Remove／Memberships／SetMembership。分类编辑用 revision CAS，删除复核 tree_revision；成员 UI 仅 manual 来源，state=auto 删除人工行。导航统计来自全库有效关系，Feed 只列活动订阅。偏好增加 selected_scope_id，selected_scope 新增 feed／category；FeedService.Update 的 apply_existing 默认 true，false 仅作用于新入库文章。具体校验和回归见[分类导航规范](../../.trellis/spec/backend/navigation-guidelines.md)。

## Phase 4 Reader 接口

reader.Service.Get／Extract 使用 article_id和mode，返回安全本地资源HTML及text／has_cached／has_original。OpenLink仅接当前文章的resource_id（空表示原文），不接受任意地址。ArticleSummary新增revision用于正文缓存键；Settings新增reader_auto_read=true和reader_font_family=sans，旧JSON从默认值恢复。正文RPC采用CancellablePromise上下文取消，提取缓存独立于订阅正文。具体限额与错误见[Reader规范](../../.trellis/spec/backend/reader-guidelines.md)。

`media.API.CopyInlineImage(id)` 接收 64 位十六进制正文图片资源 ID，返回 `{ok,error?,request_id}`。调用沿用内联媒体策略和资源版本校验，成功时向系统剪贴板写入 PNG。非法 ID 返回 `ValidationError`，资源不可用或剪贴板写入失败返回 `MediaError`。

Settings 的 Get／Update 偏好对象包含 `reader_code_theme`。默认值为 `app`。更新接受匹配 `^(base16/)?[a-z0-9][a-z0-9-]{0,70}$` 的标识，非法格式返回 `ValidationError`。

### Phase 4 OPML 与刷新事件

transfer.API 的 Choose 返回 token／name／cancelled；Preview 返回 preview_id／name／items，outline 含 key／parent_key／title／url／site_url／kind／status／reason／depth，Go保留私有计划与版本。Import返回 report（feeds_added／feeds_reused／feeds_restored／categories_added／associations_added／skipped／feed_ids）、refresh_queued／refresh_error。Export仅返回name／bytes；Discard返回ok／error／request_id。其余均为统一ok／data／error／request_id，禁止前端传宿主路径或修改计划内容。绑定方法可取消，提交完成后取消不能承诺回滚。

刷新Snapshot增加 success_count／cancelled_count／skipped_count／failed_count及results_truncated。运行中事件最多每200ms广播一次，明细最近100项；终态立即广播，Get／Current保留完整结果。累计数与截断明细独立，UI不能仅据明细判断无失败；每次操作最多10000目标，网络并发仍4／每站点2。

阅读默认方式沿用 reader_open_mode（feed／full），分别打开订阅预览和阅读模式。旧设置中的 reader_full_mode 读取时忽略。

## AI 设置

`aiconfig.API` 提供 `List`、`SaveProvider`、`DeleteProvider`、`SaveModel`、`DeleteModel`、`Assign`、`TestConnection`。密钥仅通过 `SaveProvider` 的 `apiKey` 进入 Go，`null` 保留，空字符串清除。读取和保存结果只包含 `hasCredential`。测试连接仅接收已保存的模型 ID，取消沿绑定的请求上下文传递。详细行为见 [Provider 与模型设置规格](../04-specs/providers.md)。

## 基础 Chat

`chat.API` 提供 `List`、`Create`、`Messages`、`Model`、`Send` 与 `Cancel`。`Send` 在本地事务提交后启动后台流，`Messages` 返回当前文本及运行状态。会话固定在全局、分类或文章范围，附加上下文只用于当前发送。详见 [Chat 与上下文规格](../04-specs/chat.md)。

## 文章 AI

`articleai.API.Settings` 返回启用位、分类提示词、每小时预算、精选阈值与 revision；`Save` 按 revision 条件写入。 `Evaluate` 接收 `{articleId, task, preview}`，task 为 classification 或 featured。preview 返回评估而不写文章结果，且仍占用调用预算。成功返回命中分类、评分或缓存标志，已缓存时不重新请求。并发运行、预算、模型输出异常分别返回 Busy、BudgetExceeded、InvalidModelOutput。详情见 [自动分类](../04-specs/ai-classification.md)。

`Chat.Send` 的 `allowArticleReads` 默认 false。启用时，文章对话只允许读取会话当前文章，全局对话只允许读取该次请求的已选文章。其他范围返回校验错误。授权仅用于本轮。

模型目录通过 `aiconfig.API.FetchModels(providerID)` 查询。返回服务当前响应中去重后的 `{id, displayName}`，保存仍使用既有模型写入接口。密钥由后端从已保存的服务配置读取，列表查询有 15 秒总时限并拒绝重定向。
