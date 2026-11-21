# 调研证据

核验于 2026-11-21。目标是冻结实现所需的字段和兼容性样例，SDK 版本在实现时锁定到通过测试的补丁版本。

| 对象 | 已核实及项目影响 |
| --- | --- |
| openai-go | 官方 SDK 提供 Chat Completions、Responses、流式、模型列表和 HTTP 自定义选项。项目需显式配置服务地址、API 密钥与 HTTP 生命周期，验证 SDK 默认重试和环境变量是否会改变现有语义。见 [官方 SDK](https://github.com/openai/openai-go)。 |
| anthropic-sdk-go | 官方主分支文档当前使用 `/v2`。Messages、计数及结构化工具结果可纳入迁移。针对本机兼容服务验证自定义 Base URL、认证头和输出上限。见 [官方 SDK](https://github.com/anthropics/anthropic-sdk-go) 与 [Claude 计数接口](https://platform.claude.com/docs/en/build-with-claude/token-counting)。 |
| OpenRouter | 目录接口公开上下文长度、输入模态和定价等字段。字段语义与币种单位从服务文档校验，不能按所有兼容网关共享结构。见 [模型目录](https://openrouter.ai/docs/api/api-reference/models/get-models)。 |
| Models.dev | 官方源码说明三种目录：`api.json` 是服务商数据，`models.json` 是基础模型，`catalog.json` 为合并目录。可用 `canonical_model_id` 连接托管和厂商记录。读取 `models.json?type=all` 的公开样本看到了 `limit.context/output`、`modalities.input/output`。网页工具上的两个大型接口超过读取上限；本地直连探测对两个大接口、独立目录及图标返回 403，因此完整响应和图标实际授权还需实施探针。优先拆分读取和使用文件快照；目录不可用时降级。见 [Models.dev API 与源码](https://github.com/anomalyco/models.dev#api)、[模型资料样本](https://models.dev/models.json?type=all)、[Models.dev 软件许可](https://github.com/anomalyco/models.dev/blob/dev/LICENSE)。品牌图标另外核对适用的使用条件。 |
| Pi | 实际公开 GET 成功，约 0.99 MB，顶层按供应商 ID 分组（本次 43 项），再按模型 ID 映射，返回 ETag。样本字段包含 `id/name/provider/input/cost/contextWindow/maxTokens`，部分含 `inputLimits`、`compat` 或 `thinkingLevelMap`。给 Pi 字段定义写独立测试，`maxTokens` 按输出上限处理；精细价格字段从上游类型或样本核验后显示。见 [Pi JSON](https://pi.dev/api/models) 与 [Pi 上游](https://github.com/earendil-works/pi)。 |
| MCP | 官方 Go SDK 支持 Client／Server、stdio 与 Streamable HTTP、协议协商和安全配置。当前与旧客户端的握手行为由 SDK 协商并实测。见 [官方 Go SDK](https://github.com/modelcontextprotocol/go-sdk) 与 [传输规范](https://modelcontextprotocol.io/specification/latest/basic/transports)。 |
| Skills | 沿用仓库既有声明式 SKILL.md、受限附件和工具白名单。导入时以 Markdown／frontmatter 为边界，应用自身执行手册文本。见 [项目 Skills 规格](../../../../docs/04-specs/skills.md) 与 [Agent Skills 规格](https://agentskills.io/specification)。 |

## 仓库定位

- 模型调用与目录在 [internal/ai](../../../../internal/ai)，现有目录只解码 ID、显示名并做同源路径回退。设置、凭据、用途分配在 [internal/aiconfig](../../../../internal/aiconfig)。
- [Chat 服务](../../../../internal/chat/service.go) 当前用固定历史/上下文字节上限和本地流式快照。工具续轮在 [agentrun](../../../../internal/agentrun/stream.go)，现有授权文章工具依赖本轮指定 ID，审计存储已落地。
- [应用工具规格](../../../../docs/04-specs/tools.md) 原列 25 项，增加项对应 Library、Reader、Feed、Navigation、Refresh、ArticleAI、Settings、Transfer 可观察行为，详见本任务 tools.md。
- [聊天编辑器](../../../../frontend/src/features/chat/ChatDialog.tsx) 当前由「＋」打开占据消息区和输入框之间的搜索行。计划迁移到独立附加文章界面，模型常规找文走工具。
