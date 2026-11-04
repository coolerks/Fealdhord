# ADR-0007：官方 MCP Go SDK 与协议版本

状态：拟定，待本轮规划评审；不是已完成实现。
日期：2026-11-04。

## 背景

需要 Client／Server 和两种标准 transport，协议正在演进。

## 决定

使用 github.com/modelcontextprotocol/go-sdk；v1.8.0 为当前研究候选，Phase 10 复核安全补丁并锁定，依 SDK 协商。

## 替代方案

手写 JSON-RPC 与旧握手复刻增加不兼容风险。

## 后果与验证

版本矩阵必须实测；新协议 HTTP 模式与旧状态会话不可混同。

依据与后续门禁：[相关规格／研究](../09-research/mcp-sdk-notes.md)。
