# 原生菜单与阅读体验契约

## 1. 范围

用户于 2026-11-04 明确否定绿色与邮件式常驻三栏，要求参考 macOS Books／Podcasts 的中性材质、留白与清晰内容层级。本契约覆盖此前对应视觉选择；原始需求快照不改动。

## 2. 签名

internal/desktop.WindowOptions／InstallMenu；事件 fealdhord:command={id:string}，已注册生成类型。前端 native 适配 Wails System／Window／Clipboard／Events，commands.runCommand 为白名单执行器。ApplicationMenu、Palette、工具栏和上下文菜单消费同一命令及 Query／UI 状态。

## 3. 契约

macOS 使用系统菜单栏：折栈／文件／编辑／显示／文章／窗口／帮助。编辑、隐藏、退出、全屏等采用原生 role，菜单快捷键遵守 Cmd；Windows／Linux 使用 Radix Menubar，自行实现文件／编辑／显示／文章／窗口／帮助，F10 可聚焦，系统窗口框架保留。OPML 尚未接入，菜单明示禁用。

macOS HiddenInset 隐藏标题文字并保留原生控制，NSVisualEffectView 窗口材质＋透明 HTML／WebView＋侧栏半透明叠色，主内容不透明；不使用私有 API。其他系统默认不透明回退，不声称具备 macOS 材质。顶部预留 52px 控制／拖动区域。

默认双栏浏览，选择文章后打开 Reader 聚焦面板；reader_pinned=false 为新增兼容字段，旧 JSON 缺字段安全默认为 false。设置允许显式启用宽窗口并排三栏。无文章不显示空 Reader 常驻栏。主题采用中性黑白灰。

全局 Radix ContextMenu＋DefaultContextMenuDisabled 禁止原生浏览器右键。文章菜单按目标 ID 操作；文本选择／输入提供复制、剪切、粘贴、全选。剪贴板异步写回前检查输入快照未变，尊重只读和 maxLength；失败明确提示，不能把命令当成功。文本仍可选择。

lucide-animated（MIT）官方 registry 源码按图标保存在 components/icons，并保留 LICENSE；原版轨迹不自创替换。AnimatedGlyph 从父按钮／菜单整行／键盘焦点触发，被动图标响应自身 Hover。播放完成后等待 normal／initial 归位再卸载，持续悬停不循环，移出不截断；同轮事件不重复启动、不排队。Full 才创建实例；Subtle／Off 静态，Provider 覆盖全局菜单。静态 SVG 由 npm run icons:generate 从同份库源生成，保持几何一致；正文与虚拟定位不受影响。

## 4. 校验与错误

未知菜单事件 ID 忽略；窗口／剪贴板失败显示安全错误。菜单未接领域功能明示禁用或跳转已有阶段说明，不展示伪数据。减少动态优先级沿用偏好契约。多阶段内部动画以 generation 中断，防止卸载或模式切换后旧 Promise 再次启动。

## 5. 案例

正常：macOS 显示菜单选择 Masonry，工具栏与持久偏好同步。基础：空白右键展示应用菜单，未出现浏览器后退／重载／检查。异常：选中文本后菜单夺焦导致粘贴目标丢失；必须使用打开前的目标快照。

## 6. 验证

原生体验首次修订时 22 项前端测试通过，Go test／vet、设置 race 与原生 build 通过，lint 0 error／1 项既有 Compiler 提示。组件覆盖 Windows Menubar 和全局右键、defaultPrevented、文本编辑、macOS 菜单事件注册／无重复前端菜单、Reader 聚焦、图标 Full／Off。真实 SQLite 偏好测试继续执行。原生窗口已启动，本机 AX 确认系统菜单 7 组，点击显示→瀑布流后对应 radio 与状态文案同步；右键显示阅读上下文菜单；设置关闭回焦成功。截图确认灰白双栏与顶部原生区域。

未执行 Windows 实机、跨平台性能或动效逐帧／录屏认证；静态截图不作为完整动画验收。真实 AppKit 透明效果强度依赖系统减少透明度、背景及 OS 状态。

## 7. 反例与正确做法

错误：只用 CSS blur 覆盖不透明 NSWindow、画第二套 macOS 交通灯、把所有 SVG 平移两像素冒充语义动画。正确：窗口级材质和透明链路、原生控制、安全回退与内部路径动画，组件行为及本机效果分别验证。

参考：[Wails 菜单](https://v3.wails.io/features/menus/application/)、[窗口选项](https://v3.wails.io/features/windows/options/)、[动画库](https://lucide-animated.com/)、[MIT 源码](https://github.com/pqoqubbw/icons)。

## 2026-11-05 界面减负

侧栏与原生菜单统一使用“命令面板”，文章搜索保留独立入口；命令输入框及列表有可访问名称。阅读工具区合并，窄分栏按容器宽度重排；去掉品牌副标、宣传性空态、重复视图解释和常驻快捷键脚注。订阅与分类、AI 与扩展仅展示真实不可用状态；加载范围与保存失败保留。规则与三方体验判断写入 AGENTS.md，数据服务和命令 ID 不变。


## 2026-11-05 菜单与图标动效一致性

右键按文本编辑／文章操作／视图／应用分组，以分隔线呈现，省去重复标题；每项有图标，已有快捷键按平台格式末尾对齐并提供 aria-keyshortcuts。command-presentation 统一右键、Windows／Linux 应用菜单和命令面板呈现，执行仍走 runCommand／text-actions。右键目标不是当前文章时，不展示针对当前文章的 F／M，避免误导。只读、文本快照及实际文章目标保持。

覆盖设置导航、阅读器、工具栏、命令面板、文章标记、空态，以及 Dialog 关闭／菜单勾号和箭头。恢复官方 Search／BookText，移除 AI 旧 CSS 缩放及卡片 Hover 平移，避免和库轨迹叠加。多 controls 等全部结束，续段有取消代次，cancelAnimation 停止活动任务；cmdk 通过 data-selected 变化支持键盘选择。

新增生命周期／菜单回归，以及 Motion 手动时钟驱动的全部 38 个图标播放＋归位检查。当前前端共 38 项通过，类型、lint（0 error／1 项既有 Virtualizer 提示）、前端与 macOS 构建通过；不将手动时钟测试当作原生帧率认证。未运行 E2E、Windows 实机或逐帧性能采样。
