# 三种 AI 协议调研

核验日期：2026-11-04。只使用官方文档，不调用真实模型。

## 官方事实

[OpenAI 工具调用](https://developers.openai.com/api/docs/guides/function-calling)描述模型发起调用、应用执行、返回结果的闭环，可能包含多项调用。[Responses 迁移说明](https://developers.openai.com/api/docs/guides/migrate-to-responses)说明 Responses 与 Chat Completions 的工具结构和事件不同。[流式文档](https://developers.openai.com/api/docs/guides/streaming-responses)定义语义事件。[Claude 流式文档](https://platform.claude.com/docs/en/build-with-claude/streaming)通过 content block 和 JSON 参数片段传输工具输入。

## 项目决定

三种独立 Adapter 归一为 ModelRequest／ModelEvent；工具参数片段结束并通过 Schema 校验前不执行。原始模型返回格式及厂商续传块只在对应 Adapter 内保存，业务不依赖 SDK 类型。只转发服务实际返回且允许展示的推理／思考内容，不推断或请求隐藏思维链。

优先评估[OpenAI Go SDK](https://github.com/openai/openai-go)和[Anthropic Go SDK](https://github.com/anthropics/anthropic-sdk-go)，兼容选项无法表达时允许适配层内使用 HTTP，不把协议退化为 Prompt JSON。SDK 具体 major 与厂商 API 参数在 Phase 5 锁定；兼容服务的工具和结构化输出能力需 Mock 与用户主动探测，不能凭品牌名称判断。
