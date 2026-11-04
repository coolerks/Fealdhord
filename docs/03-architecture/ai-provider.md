# AI Provider 架构

业务依赖 Model 接口；ChatCompletionsAdapter、ResponsesAdapter、MessagesAdapter 分开实现 wire 映射。SDK 与 HTTP 客户端仅位于 Adapter 包。统一请求、事件及 capability 见 [模型契约](../05-api/model-contract.md)。

Provider 存连接协议与认证引用，Model 存模型名与能力。能力值为 supported／unsupported／unknown，用户覆盖优先于探测；未知不表示支持。五类任务独立分配模型，缺少模型时返回配置错误，不静默调用其他收费模型。

高级选项只作用于当前 Provider：endpoint path、非敏感 extra body、headers 引用、query 引用、timeout、禁用参数。合并顺序为协议默认 → 模型能力裁剪 → 用户选项；model/messages/input/tools/stream 等保留字段不能被 extra body 偷换。UI 明确显示最终主机与路径；日志脱敏。

Base URL 是 API 根，可含 `/v1`；拼接相对 endpoint 时避免丢失已有前缀或出现 `/v1/v1`。绝对自定义 endpoint 不允许跨主机静默转发认证。HTTPS 默认；用户明确设置的本机 HTTP Gateway 可按固定 Provider origin 授权，不扩大为所有 RSS 抓取的私网许可。

多轮上下文保存在本地；Responses 默认 `store=false`（支持时），不依赖远程 conversation 才能运行。需要回传的 opaque reasoning／签名块由 Adapter 私有状态保存，不强制变成可展示文字。流式中断保留未完成标记，不能继续执行未闭合的工具参数。
