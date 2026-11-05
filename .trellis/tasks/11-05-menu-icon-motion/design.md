# 菜单与图标动效技术方案

## 边界与所有权

行为位于 AnimatedGlyph 和菜单组合层，复用 SemanticButton、Radix、commandLabels／runCommand 与 text-actions。CommandID、RPC、领域状态不变。通过 shadcn CLI 核对／安装用户指定 lucide-animated，保留 MIT；恢复 Search、BookText 原版轨迹。

## 单次播放

适配为可等待的播放／归位操作，明确 Promise 返回。大部分 controls.start 已返回 Promise；多 controls／多阶段图标只适配完成通知、取消与代次，不改轨迹。不用固定时间猜测结束。

状态 idle → playing → resetting → idle。同一轮合并鼠标、焦点等重复事件；移出不截断，不排队；完成后主动调用官方 normal／initial 并等待归位，持续停留不循环。模式切换和卸载使旧代次失效，快速移出再入只保留一条链。

常态同形静态 SVG，仅交互目标运行 Motion。验证挂载首帧和归位切换；避免每卡常驻复杂实例、交互时网络加载、循环计时和尺寸变化。不用全局 transform:none 修卡顿，保留虚拟定位 transform。

## 触发与偏好

支持按钮、radio、menuitem／menuitemradio／menuitemcheckbox、option 及显式区域；被动图标自身响应 hover，不改变可访问树。尊重 disabled、aria-disabled、Radix data-disabled。键盘聚焦只播放一次，不与鼠标造成的焦点重复。

将图标偏好放到覆盖 GlobalContextMenu 与 Shell 的共同边界，复用 effectiveMotion；Portal 继承同一有效模式。偏好读取前保守降级，Off／Subtle 不启动路径动画。保留 Shell 面板 MotionConfig。

## 菜单呈现

文本组按快照／只读启用；文章组操作右键目标；视图组包含搜索、列表、瀑布流、侧栏；应用组包含设置。Group + Separator 分组，无重复标题。每项图标 + 文字 + 可选 Shortcut，选中勾号独立占位。

共享命令呈现元数据供右键、应用菜单、命令面板使用，执行仍走 runCommand。快捷键核对 Shell 和 macOS 原生菜单，按平台和上下文展示；F／M 针对当前文章，不冒充针对任意右键文章的绑定。文本动作使用标准平台绑定，未绑定不展示。

## 修改区域

- components/animated-glyph、icons、semantic-button：生命周期、库来源和同形渲染。
- app 菜单／命令面板／工具栏／侧栏、App／Shell：接入、分组、共享呈现和偏好边界。
- features/settings、reader、articles 及实际渲染的 components/ui 结构图标：补齐覆盖，保留角色、标签与焦点。
- style.css：菜单对齐、图标占位和冲突 transform 清理，列表几何不变。
- 播放生命周期／菜单测试、现有 App 回归、原生体验契约与前端规范。

## 风险与回退

官方图标完成通知不同，需要最小适配；不靠延长时间掩盖。原版较大位移需核对裁剪／重排，不自创轨迹。重复触发是否造成可见卡顿仍是推断，代码检查不能证明帧率。按播放基础、菜单、页面覆盖分批提交，发现回归修复对应部分；保留生产数据与其他已有工作。
