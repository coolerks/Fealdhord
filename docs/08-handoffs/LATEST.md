# 最近交接：Phase 0 桌面工程

日期：2026-11-04。分支：codex/desktop-bootstrap。当前任务：desktop-bootstrap，in_progress，代码与本机验收通过，已获持续授权，按完成部分直接提交。

## 本阶段交付

Wails 3.0.0／React 19.3／TS／Vite 8 工程，internal/app 基础服务，四个绑定方法、七模型、一个注册事件；单活动有界诊断、取消／关闭、快照与早到乱序合并。Taskfile、锁文件、忽略规则、本机应用包和中文运行文档已建立。

## 真实证据

Go test／race／vet、前端 typecheck／lint／5 项交互、前端 build、Wails native build、ad-hoc 签名校验通过。真实窗口读取版本和平台，开始后收到进度，取消显示 cancelled，重新运行最终 20/20 succeeded。详见 docs/09-research/phase0-validation.md。

## 关键经验

- doctor 沙箱内 sysctl 被拒，沙箱外 ready=true；不需要为可选 iOS 项安装完整 Xcode。
- 绑定生成器可能警告后以 0 返回且 0 服务；必须检查实际服务／方法数量，不能只看退出码。
- Vite 8 与旧 Vitest 4 类型不兼容，现固定 Vitest 5.1.0；不能用 any 绕过。
- macOS 构建目标 13.0；最低版本与其他平台未认证。应用包仅临时签名，不是发布签名。
- 此前整体规格仍是产品目标，只有 Phase 0 已实现；诊断页不是正式阅读 Shell，诊断内存历史不代表 Agent 持久执行历史。

## 下一阶段

Phase 1 依已有规划建立三栏、玻璃 Sidebar、真实双视图、主题、分栏和 Motion／Reduced Motion；同时验证动态 lanes、锚点与键盘顺序。RSS／数据库后续 Phase 2–4，不能用假文章当真实交付。

## 工作区与提交

保留已有 .agents／.claude／.codex／.pi 与 Trellis 框架文件，禁止子代理，inline 不变。AGENTS.md 已记录持续提交授权，覆盖工作流逐次确认要求。只提交本会话代码／文档／规范／两个任务记录，不静默包含其他既有配置；随后归档与记日志。功能优先，非必要不做 E2E，按风险选择单元与必要检查。
