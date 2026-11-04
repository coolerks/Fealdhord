# MCP 功能规格

Phase 10 Client，Phase 11 Server。固定 SDK 与协商矩阵在实现前复核，参见 [MCP 调研](../09-research/mcp-sdk-notes.md)。

Client 配置：id、name、transport、command／args 或 url、env credential refs、enabled、permission_policy。stdio 与 HTTP 字段互斥；程序以 argv 启动、不给 shell 解释。UI 显示连接状态、Tools／Resources／Prompts、每工具 auto/ask/deny；外部写默认 ask，未知效果不信任服务注解。

逻辑工具名 `mcp.<server_id>.<tool_name>`，server_id 稳定且唯一。厂商工具名称限制通过映射解决，不能仅用展示名拼接导致重名。断线标不可用，重新连接后重新列举；工具变化不会继承原更宽权限。认证、环境变量与日志脱敏。

Server 初始白名单：search_articles{query}、get_article{id}、list_feeds{cursor?,limit?}、list_categories{cursor?,limit?}、favorite_article{id,is_favorite}、mark_article_read{ids,is_read}。前四只读，后两写；默认读取授权、写入逐客户端策略，拒绝无人批准的危险动作。MCP 外部名称与内置名称独立映射同应用服务。

Resources：`fealdhord://article/{id}`、`fealdhord://feed/{id}`、`fealdhord://category/{id}`，只返回清理且有界内容，不暴露文件路径、数据库或 credential。资源 ID 不存在返回协议规范错误；Prompt 是用户选择的内容模板，不是权限。

## 验收

- MCP-01：stdio／Streamable HTTP 均通过发现、读取、工具、取消、超时与断线测试；两服务器同工具名不冲突。
- MCP-02：secret env 不落普通 DB／日志，恶意 annotation 不提升权限，外部写不自动重放。
- MCP-03：Server 默认不监听；启用后认证、Origin、只读／写策略有效；绕过 UI 仍无法越权，资源不泄漏宿主文件。
- MCP-04：至少当前候选协议与一个仍支持的旧客户端完成互操作；版本协商结果写平台矩阵。
