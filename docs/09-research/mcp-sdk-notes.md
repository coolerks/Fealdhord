# MCP 官方 Go SDK 调研

核验日期：2026-11-04。

## 官方事实

[官方 Go SDK](https://github.com/modelcontextprotocol/go-sdk)同时支持 Client 和 Server。[release](https://github.com/modelcontextprotocol/go-sdk/releases)当前列出 v1.8.0；v1.7.0 起支持 2026-07-28，并兼容若干旧协议。[当前 transport 规范](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports)包含 stdio 和 Streamable HTTP。新旧协议的交互模式不能混写。

## 项目决定

以 v1.8.0 为调研候选，不在未测试前声称已锁定。Phase 10 选择经安全复核的兼容补丁版并记录协议协商矩阵；采用 SDK 的版本协商和 transport 实现，业务只消费统一 Tool／Resource／Prompt。2026-07-28 的 HTTP 模式与旧有会话模式需分别验证，不在规格中强制套用旧 initialize 握手。

Client 和 Server 分开交付；本地 MCP Server 默认关闭。stdio 启动的是用户明确配置的外部 MCP 程序，不等于允许产品 Skill 执行代码，更不意味着允许开发会话启动辅助 AI 代理。

## 未验证

新版本 OAuth、认证失效、重连、取消与旧客户端互操作需 Phase 10／11 实验；资源和 Prompt 仍受内容信任、大小和权限限制。
