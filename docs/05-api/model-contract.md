# 统一模型契约

## 1. 范围与触发条件

三种 Protocol Adapter、Runtime 和 Chat 之间的稳定边界；完整契约仍在实施。当前代码位于 `internal/ai`，已实现 Chat Completions 单轮文本请求、SSE 文本事件、用量归一化和工具调用。

## 2. 签名

```go
type Model interface {
    Generate(ctx context.Context, req *ModelRequest) (*ModelResponse, error)
    Stream(ctx context.Context, req *ModelRequest) (<-chan ModelEvent, error)
}
```

Generate 与 Stream 均只处理一轮模型响应；Tool Loop 属于 Runtime。初始校验／连接错误返回 error；已开启 stream 的失败通过统一 error 事件结束，channel 随后关闭。

## 3. 请求、响应与协议映射

`ModelRequest`：system、messages、tools、tool_choice、temperature?、top_p?、max_output_tokens?、stop?、response_format?、metadata、provider_options。可选数值必须区分未设置与 0。Message：role(system/user/assistant/tool)、content（typed blocks）、tool_calls、tool_call_id?、metadata。响应含完整 message、finish_reason、usage、adapter_state_ref?。

基础 ContentBlock 为 `{type:"text",text:string}`；模型续轮专用块以 `{type:"adapter_state",ref:string}` 引用 Adapter 私有已验证数据，不把厂商签名或 SDK 对象暴露给 UI。vision 能力仅作为模型声明，RSS 默认不发送图片；未来主动图像输入需独立契约与用户选择。`ToolChoice` 为 auto/none/required 或指定注册工具名，指定不存在工具返回 ValidationError。

请求基本头：Chat／Responses 默认 Bearer 认证；Messages 默认 x-api-key 与固定兼容版本头。具体版本头值按 Phase 5 官方文档锁定。兼容服务可显式选择认证映射，秘密只在发请求时注入，重定向不跨 origin 转发。

ToolDefinition 包含 name、description、input_schema；ToolCall 包含 id、name、arguments_json。Tool Result 包含 call_id、content、is_error。供应商名映射保存双向表；不支持工具时直接报 CapabilityError，不模拟 JSON 提示。

| 内部含义 | Chat Completions | Responses | Messages |
| --- | --- | --- | --- |
| endpoint | /v1/chat/completions | /v1/responses | /v1/messages |
| 系统提示 | system message | instructions | top-level system |
| 工具请求 | assistant.tool_calls | function_call items | tool_use blocks |
| 工具结果 | role=tool + tool_call_id | function_call_output + call_id | user 中 tool_result + tool_use_id |
| 参数片段定位 | choice + tool index／id | output item id／index | content block index／id |
| 流结束 | finish reason + 流结束标记 | completed／failed／incomplete 终态 | message_stop + stop_reason |

`ModelEvent`：type、message_id、sequence、call_id?、text?、arguments_delta?、usage?、finish_reason?、error?。type 至少 message_start/text_delta/reasoning_delta/tool_call_start/tool_call_arguments_delta/tool_call_end/usage/message_end/error。同一 message sequence 严格递增；可交错不同 call 的片段。tool_call_end 仅表示参数完成，执行需另做验证。

usage 区分 input_tokens/output_tokens/total_tokens 与可选 cached／reasoning；未知为 null，不能当 0。规范化为单消息累计快照，不把多次累计报告重复相加。预算以实际返回与本地上限双重约束。

仅暴露厂商实际提供且允许显示的 reasoning 摘要；维持续传所需的 opaque 签名块由 Adapter 保存。前端不接触原始 SSE。

## 4. 校验与错误矩阵

| 场景 | 错误与处理 |
| --- | --- |
| 不支持的参数且没有显式禁用 | CapabilityError，不无声忽略 |
| 401／403 | AuthenticationError，不自动重试 |
| 429 | RateLimitError，保留 retry_after，有限退避 |
| SSE 损坏／未闭合参数 | ProviderProtocolError，不执行该工具 |
| 超时／取消 | Timeout／Cancelled，停止读流 |
| EOF 无协议终态 | ProviderProtocolError，不能展示为成功 |
| 多个工具 id 冲突 | ProviderProtocolError，不合并不同调用 |

SSE 事件与工具参数有独立大小上限，UTF-8 和跨分包边界正确累积；不使用默认 Scanner 小 token 上限误截断。仅对无副作用且尚未输出内容的可重试请求有限重试；已执行工具后不能重放整个 run。

## 5. 案例

良好：两个 tool_calls 参数交错，按 ID 拼接，完整校验后各执行一次并把两个结果回传；基础：无工具的纯文本非流式回复；异常：流断在 JSON 半截，保留部分文本，标记失败，无工具副作用。

## 6. 必需测试

三协议各覆盖非流式、文本流、多个／交错工具、结果续轮、拒绝、usage 缺失与累计、畸形 SSE、未知事件、超限、认证、429、取消、超时与 EOF。用 httptest Mock，零收费 API；不能只测试统一模型后漏掉 wire 编解码。

## 7. 错误与正确做法

错误：Messages 结果回传成 role=tool，或 Responses 借 Chat Adapter 模拟。正确：独立映射到各自原生结构，由统一契约测试保证用户结果一致。

验收 MODEL-01：三 Adapter 独立通过本节所有 wire 用例，包括多工具交错、工具结果续轮、EOF 与 usage。

Chat Completions 当前以配置的 API 基础路径追加 `chat/completions`，请求使用标准库 HTTP 并禁止重定向。响应上限为 8 MiB，单个 SSE 事件上限为 256 KiB，请求总时限为两分钟。已取消且无人继续消费的事件通道会直接关闭。

Chat Completions 的流式调用以工具索引累积参数，单轮最多 128 项，单项参数最多 64 KiB。收到 `tool_calls` 结束原因和 `[DONE]`，且整批调用 ID、工具名和 JSON 对象均验证通过后，才发出各项 `tool_call_end`。调用方应等到 `message_end` 再进入 Runtime 的参数 Schema 校验和审批。续轮按 `assistant.tool_calls` 和对应的 `role=tool` 消息编码，缺失、重复或无对应调用的结果在发请求前拒绝。
