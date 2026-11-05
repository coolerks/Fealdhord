# 菜单与图标动效实施计划

状态：用户已批准并开始实施。播放基础已完成，5 项生命周期测试与原有 App 12 项回归通过；菜单与页面覆盖继续实施。

## 顺序与门禁

1. 评审通过后 task.py start，加载 trellis-before-dev、本目录及前端规范。
2. 官方 CLI 核对并补齐图标，恢复原版 Search／BookText；实现可等待、自动归位、防重复、取消安全的播放和共同偏好边界。
3. 验证持续 hover、单次结束、快速进入、焦点、禁用、关闭及卸载；同步规范，完成此部分直接提交。
4. 共享命令呈现；重组右键和应用菜单，加入图标与末尾平台快捷键；保留文本编辑及右键目标行为。
5. 对照 research.md 补齐设置、阅读器、文章、工具栏、命令面板、空态和结构图标；检查静态／动态同形、空闲实例与冲突 CSS。
6. 相关回归及完整前端检查、macOS 构建，记录 bundle 大小；卡顿若仍无法通过测试确认，进行最小真实运行核验并记录未测边界。
7. 更新 CURRENT、native-experience、LATEST、任务镜像；分批提交，归档和记录会话，不推送。

## 验证

frontend：npm run typecheck、npm run lint、npm test、npm run build。原生：wails3 build；不得与读取 go:embed 的检查并行写 dist。文档：python3 docs/09-research/probes/validate_docs.py，并单独检查本任务无占位和链接有效。

不默认 E2E；生命周期状态测试不代替帧率证据。高风险点为 Portal 偏好边界、多段动画取消、完成通知、编辑快照和虚拟定位。门禁不通过先修当前部分，不扩展 RSS 或业务重构。
