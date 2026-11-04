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
| DataService | PreviewOPML{dialog_token}；ImportOPML{preview_id,selection}；ExportOPML{dialog_token}；Backup{dialog_token} | 导入报告／文件结果 |

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
