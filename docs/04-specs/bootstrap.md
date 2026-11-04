# Phase 0 基础调用链

状态：已实现，2026-11-04。本节只描述工程诊断，不代表 RSS／数据库／Agent 已交付。

## 1. 范围与触发条件

React 启动读取应用信息，用户主动启动无网络副作用的五秒检查；验证真实绑定、事件、快照和取消。正式阅读 Shell 在 Phase 1 替换当前工程验证入口。

## 2. 签名

BootstrapService：`GetInfo() InfoResult`、`StartProbe() OperationResult`、`GetOperation(OperationRequest) OperationResult`、`Cancel(OperationRequest) OperationResult`。实现 `internal/app/bootstrap.go`；生成绑定位于 `frontend/bindings/github.com/coolerks/Fealdhord/internal/app/`。

## 3. 契约

结果含 ok、data 或 error、request_id，失败时省略 data。OperationRequest={operation_id:string}。快照含 operation_id、status、completed、total、sequence；running／succeeded／cancelled。20 次本机 tick，每 250ms 一次。Start 同时请求合并为同一个活动操作；仅保存最近 16 项内存快照，重启清空，不承诺未来 Agent 的持久历史能力。

事件 `fealdhord:event`：schema_version=1、event_id、operation_id、sequence、type、occurred_at、payload。type 为 operation.progress／operation.terminal。前端启动前订阅、按 ID 和 sequence 合并，早到事件有界保留；运行中每秒以快照恢复遗漏事件。卸载注销订阅和 interval。取消返回当前事实，worker 随 context 结束并发终态；已完成操作取消不改变成功。

## 4. 校验与错误矩阵

空操作 ID→ValidationError；未知／已回收→NotFound；关闭后 Start→Unavailable。所有错误中文且不含系统错误。关闭与 Start 共用锁，先关闭入口再取消和等待收尾，避免 WaitGroup Add／Wait 竞态。

## 5. 良好、基础和异常案例

良好：真实窗口从 running 递增至 succeeded；基础：GetInfo 返回版本和平台；异常：用户取消后 cancelled，旧 running 事件不能覆盖终态，也不能污染下一操作。

## 6. 必需测试与证据

Go 5 项行为测试覆盖成功／快照、并发合并／取消、关闭、错误载荷和有界历史，竞态检查通过。前端 5 项覆盖读取、取消／旧事件、早到乱序、失败恢复、订阅清理。macOS 原生验收与工具链见 docs/09-research/phase0-validation.md。

## 7. 错误与正确做法

错误：只验证浏览器 Mock 就宣称桌面链路可用；或取消时 UI 自行填终态。正确：生成绑定、真实 Wails 窗口验证，终态以后端事件／快照为准。
