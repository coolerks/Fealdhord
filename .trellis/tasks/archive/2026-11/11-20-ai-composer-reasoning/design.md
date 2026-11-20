# 技术设计

## 界面

沿用 Shell 的双工作区和现有侧栏底部结构。全局侧栏去掉顶部返回项，底部左侧提供阅读模式切换，右侧齿轮进入 AI 设置；收起时提供等价的图标入口。会话标题前使用项目已有的 AI 图标。输入区域使用语义色边线和浅阴影，取消聚焦时的深色外框。使用共享 Button 支持圆形发送与停止图标。给 SearchableSelect 增加可选的无边框触发样式，模型名称和档位并列，均可键盘展开、搜索、选择和关闭。

## 能力和协议

新建模型表单初始化四个 `{label, value}` 条目，按表单最终顺序保存。档位列表随模型的 revision 写入，并通过现有 AI 配置快照读取。后端限制列表、标签和参数值长度，拒绝空值和重复参数值，接受空列表。旧模型以空列表兼容读取，继续使用服务默认；编辑时可添加档位。读取或保存空列表时不重新插入预填值。

全局发送新增可选 `reasoningEffort`，从已选模型保存的档位中取值。「使用服务默认」表示省略字段。按模型 ID 暂存当前工作区内的选择，模型配置变化后重新校验。Go 在持久化消息和发请求前确认显式档位仍属于模型。适配器保留用户保存的参数值，按服务协议生成载荷。OpenAI 兼容 Chat Completions 使用 `reasoning_effort`，Responses 使用 `reasoning.effort`，Claude Messages 使用 `output_config.effort`；OpenRouter Chat Completions 使用 `reasoning.effort` 对象。预填四档是编辑起点，用户可按服务能力调整，例如为 DeepSeek V4 将超高的参数值设为 `max`。

工具续轮沿用同一档位。对于需要回传推理块或服务专用上下文的协议，验证当前轮能正确续传或在发送前明确提示不支持该组合。仅为当前执行保留协议要求的必要数据，回复正文和持久化历史继续沿用现有文本契约。接收上游错误时提示用户调整，禁止静默重发和替换档位。

## 参考

- [OpenAI reasoning guide](https://developers.openai.com/api/docs/guides/reasoning)
- [DeepSeek Thinking Mode](https://api-docs.deepseek.com/guides/thinking_mode/)
- [Claude Effort](https://platform.claude.com/docs/en/build-with-claude/effort)
- [OpenRouter Reasoning Tokens](https://openrouter.ai/docs/guides/best-practices/reasoning-tokens)
