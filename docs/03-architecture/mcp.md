# MCP Client 与 Server

Client：用户配置 → 显式启用 → 启动／连接 → SDK 协商版本 → 列举 Tools／Resources／Prompts → 本地权限映射 → Registry。transport 为 stdio 或 Streamable HTTP；不把旧 SSE transport 当成 Streamable HTTP。连接失败只影响该服务，不影响 RSS。

stdio 以 argv 启动程序，不通过 shell 拼接；仅继承最少环境，秘密由 CredentialStore 注入，停止时有界终止进程树。HTTP 校验 TLS、重定向、响应上限与认证，私网服务需针对指定 origin 明确配置。首次配置显示外部程序及权限，不能由文章或 Skill 自动新增服务器。

动态工具列表更新后重新校验冲突和策略；变化不继承更宽授权。超时／断线后工具标不可用，外部写入不能自动重放。Resources／Prompts 作为不可信上下文，按用户选择和大小预算使用。

Server：默认关闭，首版优先本机受认证 Streamable HTTP；stdio 入口作为后续可选部署，必须解决多实例数据库所有权。外部 Client 通过同一应用服务和权限；仅暴露安全白名单。读取和写入分别授权、按客户端审计；没有已确认权限的 unattended 写请求拒绝。服务监听 loopback、校验 Origin、使用随机凭据，不能以“本机”替代认证。

协议版本与 SDK 候选见 [调研](../09-research/mcp-sdk-notes.md)；不能将旧版 session 假设写死到领域层。
