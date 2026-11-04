# 工具执行与审批契约

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

`ToolResult={content:[{type:text|json,text?,data?}],is_error:boolean,error?:{code,message,retryable},affected_ids?:string[]}`；结果限长且标记截断，保留本机审计引用。Execute 的基础设施 error 由 Executor 转为安全结果，禁止把 panic 或 secret 直接送模型。

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
