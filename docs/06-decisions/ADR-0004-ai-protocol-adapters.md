# ADR-0004：三种 AI 协议 Adapter

状态：拟定，待本轮规划评审；不是已完成实现。
日期：2026-11-04。

## 背景

不同服务 wire 协议与工具续轮形状不同。

## 决定

以折栈 ModelRequest／ModelEvent 为边界，三种独立 Adapter，SDK 类型局限内部。

## 替代方案

单 Adapter 模拟 Responses 会丢原生语义；业务直接依赖 SDK 破坏兼容。

## 后果与验证

需要三套 wire contract 测试与可审计的能力协商，明确不支持旧 Completions。

依据与后续门禁：[相关规格／研究](../05-api/model-contract.md)。
