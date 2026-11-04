# ADR-0001：Wails v3 与 Go 工具链

状态：已接受并完成 Phase 0 本机验证；其他平台仍待实测。
日期：2026-11-04。

## 背景

原文要求 Wails 3 正式版与 Go 1.25+，必须核对当前事实。

## 决定

采用 Wails v3.0.0，构建工具链最低 Go 1.26；CLI 与 module 同版。

## 替代方案

Wails v2 不满足目标；Electron 增加独立运行时；预发布版无必要。

## 后果与验证

需要原生平台 SDK 和五平台验证；本机 doctor 失败仍待调查。

依据与后续门禁：[相关规格／研究](../09-research/wails-v3-notes.md)。

Phase 0 工具链偏好固定 Go 1.27.3；本机构建 macOS target=13.0，尚未认证该最低版本实机。
