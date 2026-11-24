# 工具执行与审批契约

桌面全局聊天按发送时确定的审批模式处理现有需要确认的工具。`ask` 请求一次批准，`bypass` 跳过这次确认，`deny` 返回 `PermissionDenied`。工具注册表、会话和 Skill 范围、MCP 客户端授权及工具自身校验仍独立执行。外部 MCP 请求使用现有审批流程。

## 1. 范围与触发条件

内置工具、MCP 工具与 Runtime 间的输入、输出、权限和审计。

## 2. 签名

```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() json.RawMessage
    Permission() ToolPermission
    Execute(ctx context.Context, input json.RawMessage) (*ToolResult, error)
}
```

Permission 表示静态风险声明；动态目标和数量由 Executor 在执行前计算，不能仅信任工具返回。Registry 禁止重复逻辑名。

## 3. 契约

`ToolPermission={effect:read|write|destructive,external:boolean,scope:string[]}`；策略 auto/ask/deny。外部读也需要服务连接授权；写默认询问、破坏性必须询问，用户 deny 不可被 Skill 覆盖。外部写默认 ask，未分类 MCP 工具按高风险处理。

`ToolResult={content:[{type:text|json,text?,data?}],is_error:boolean,error?:{code,message,retryable},affected_ids?:string[]}`；大结果以分页形式返回，保留本机审计引用。Execute 的基础设施 error 由 Executor 转为安全结果，禁止把 panic 或 secret 直接送模型。

`PendingAction={id,run_id,tool_call_id,tool_name,targets:[{id,title,revision}],args_hash,policy_revision,expires_at,status}`。确认使用 action_id，不重新接受前端自定参数；后端重读目标与 revision，单次 CAS 从 pending 到 executing。审批默认 5 分钟过期且不超过运行许可等待上限。取消／过期／重启后不执行。

同一 run + call_id 只执行一次。只读并行有界；写串行。批量工具显式反馈每项，事务边界内保证原子性，跨系统操作不假装全局事务。

## 4. 校验与错误矩阵

| 输入或状态 | 结果 |
| --- | --- |
| JSON 无效／未知属性／ID 空／数量超限 | ValidationError，无副作用 |
| Skill 不允许该工具 | PermissionError |
| 审批前目标 revision 改变 | ActionStale，重新预览确认 |
| 审批已消费／过期 | 返回既有结果／ActionExpired |
| 外部写超时 | OutcomeUnknown，禁止自动重试 |
| context 取消 | Cancelled；已提交副作用保留记录 |

## 5. 案例

良好：删除 17 个 Feed 前预览 17 个名称与归档影响，确认后只执行这 17 个；基础：article.search 默认只读；异常：确认时第 3 个 Feed 已变化，批次不执行并返回 ActionStale。

## 6. 必需测试

直接后端调用不能绕权限、参数篡改、审批过期／双击／重启、变更 revision、deny 覆盖 auto、MCP 风险声明欺骗、同 call_id 去重、取消后不启动下一工具、外部未知结果不重放。

## 7. 错误与正确做法

错误：模型说“已获得许可”就删除，或 UI 确认后传任意新 IDs。正确：后端持有审批快照、复核 revision、单次执行并审计。

## Chat 只读工具的活动记录

迁移 00013 为 `read_article` 记录调用开始与终态。会话消息通过可选的 `tools` 返回 `id`、`name`、`status` 和 `errorCode`。状态有 running、completed、denied、failed、cancelled、interrupted。调用编号为模型原始编号的 SHA-256，数据库保留开始及结束时间。审计表仅保存活动元数据。本机聊天消息的 `details.work.tools` 另存经结构化凭据脱敏的参数和结果，各限 16 KiB，超限标记截断。同一模型调用编号复用先前结果时不会增加执行记录。应用重启将运行中的消息和工具在一个事务内标记为中断。


## 大结果和长正文

工具结果按实际 JSON 编码计算单页大小，每页最多 12 KiB。小结果沿用原形状。内置工具返回外层 `next_cursor` 时，用同一工具仅传 `result_cursor` 续读。列表查询的 `data.next_cursor`（分页明细中的 `summary.next_cursor`）则用于沿用原过滤条件查询下一批文章。MCP 客户端导入的远端工具可用声明的 `_fealdhord_result_cursor` 继续读取中转结果。

- 通用分页返回 `paged`、`page`、`pages`、`next_cursor` 和 `fragments`。按顺序应用片段中的 JSON Pointer `path` 和 `value`。拆分的数组和对象先返回空容器，大字符串按 UTF-8 字节 `text_offset` 拼接。分页保留原始条目和字段。可用时 `summary` 单独提供状态、计数和原查询游标。
- `article.get`、`article.extract`、`read_article` 的正文页返回 `id`、`source`、`version`、`text`、`offset`、`end`、`total_bytes`、`coverage` 和可选 `next_cursor`。偏移是 UTF-8 字节，区间为左闭右开。来源区分订阅正文、提取原文和只有摘要。聊天读全文时自动处理后续页，`page_only` 可显式只读取一页用于核对引文。可以重新授权后用同一文章 ID、版本和偏移恢复。版本已变化返回 `ArticleVersionChanged`。
- 游标关联同一工具、聊天运行或 MCP 授权配置下的客户端。翻页读取已批准的快照，MCP 按范围复核指定目标。游标有效期 20 分钟，聊天运行结束也会释放该运行的快照。无效或过期返回 `ToolResultUnavailable`。一次结果最多 48 MiB，单次运行或 MCP 授权配置最多缓存 64 MiB 和 128 份结果，达到上限时返回 `ToolResultLimit`。写操作的分页只读已生成的结果。
- 聊天用当前模型和上下文配置处理分片，接近预算时在本轮滚动摘要中保留证据、来源和覆盖区间。压缩请求不提供工具，并计入本轮调用次数和用量。明确未完成的自动读正文返回 `ArticleReadIncomplete`，保留范围和恢复位置。网页抓取、提取、安全解析和聊天运行仍有各自的保护上限。

### 内置工具结果审计

| 工具 | 大结果行为 |
| --- | --- |
| `read_article`、`article.get`、`article.extract` | 正文连续分页，聊天自动分批和压缩。 |
| `article.list`、`article.search`、`article.get_metadata` | 保留查询游标、候选上限标记及摘要；结果页保留全部条目和字段。 |
| `feed.list`、`feed.get`、`feed.discover` | 订阅、分类、发现结果和详情字段按结果页返回；工具发现保留受控网页的全部候选。 |
| `feed.add`、`feed.update`、`feed.remove`、`feed.restore`、`category.add_feed`、`category.remove_feed` | 订阅写入一次；详情可分页。 |
| `feed.refresh`、`feed.refresh_all`、`feed.refresh_status`、`feed.cancel_refresh`、`app.get_diagnostics` | 返回运行状态和计数；多条刷新明细分页。 |
| `category.list`、`category.get`、`category.preview_remove`、`category.article_memberships` | 分类、成员与影响详情按结果页返回。 |
| `article.mark_read`、`article.mark_unread`、`article_ai.preview`、`article_ai.evaluate` | 保留批量状态、评估及命中详情，读取后续页时复用已执行结果。 |
| `opml.preview_import`、`opml.import`、`opml.export` | 预览条目和导入报告明细可续取。翻页不重复打开文件选择、不重做导入。导出结果为文件名和大小。 |
| `notification.channel.list`、`notification.rule.list`、`schedule.list`、`schedule.get`、`schedule.create`、`schedule.update`、`schedule.pause`、`schedule.resume` | 列表及任务详情可分页，已保存任务的返回结果可直接续读。 |
| `article.favorite`、`article.unfavorite`、`article.add_category`、`article.remove_category`、`category.create`、`category.update`、`category.remove`、`view.list`、`view.get_counts`、`app.get_settings`、`app.update_reading_settings`、`app.get_refresh_settings`、`app.update_refresh_settings`、`app.get_version`、`notification.send`、`schedule.delete`、`AskUserQuestion` | 通常返回固定字段、单条状态或少量答案。共用分页保护。 |
| `find_app_tools`、`run_app_tool` | 搜索目录返回 `next_offset`，代理调用保持目标工具的权限和分页语义。 |
