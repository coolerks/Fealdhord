# 菜单与动画检查记录

日期：2026-11-05。已检查源码与官方注册表；产品改动及动画运行验证尚未开始。

## 已确认

AnimatedGlyph 通过 mouseenter、focusin、pointerdown 多路启动，mouseleave 才归位且焦点留在目标时直接返回；能解释停在终态，卡顿成因仍需验证。触发器缺 menuitem／option。GlobalContextMenu 位于 Shell 的 IconMotionContext 外，默认 full。

官方 CLI view 已对比 Search（SVG 位移）、BookText（旋转／缩放），本地被改为扫描线／文本描画。Sparkles 多 controls 未返回组合 Promise，Layers 为异步两段，需适配完成通知与取消，不能固定计时复位。

## 覆盖清单

| 位置 | 核对项 |
| --- | --- |
| Sidebar／SemanticButton | 生命周期、语义对应、整个按钮触发 |
| Toolbar | 排序触发器与菜单、搜索输入附加图标、关闭、阅读器方向 |
| GlobalContextMenu／ApplicationMenu | 所有动作、整行 hover、末尾快捷键、分组 |
| CommandPalette | 命令、输入搜索、勾号、键盘选择 |
| SettingsDialog | 分区导航、订阅／AI 空态、Dialog 关闭 |
| Reader | 导航、操作栏、空态 |
| ArticleItem | 收藏／AI 标记、快捷操作，保留虚拟化 |
| LibraryEmpty／App／Shell | 空态、错误、保存失败图标 |
| components/ui | 实际使用的 Dialog X、菜单勾号／箭头、Command 搜索／勾号 |

## 快捷键事实

Shell 有 Windows／Linux Ctrl+,、Ctrl+F、Ctrl+1/2/3、F10，两平台 Mod+K，以及阅读区 /、J/K、F/M、Escape。macOS 原生菜单有 Cmd+N、Cmd+Shift+L 等。展示需匹配平台、启用上下文和右键目标。保留 text-actions 的只读、长度、原值与异步剪贴板校验。

## 官方来源

- [lucide-animated](https://lucide-animated.com/)：Motion／Lucide、MIT；已用 npx shadcn@latest view 读取 https://lucide-animated.com/r/search.json 和 book-text.json。
- [Context Menu](https://ui.shadcn.com/docs/components/radix/context-menu)、[Menubar](https://ui.shadcn.com/docs/components/radix/menubar)、[Dropdown Menu](https://ui.shadcn.com/docs/components/radix/dropdown-menu)、[Command](https://ui.shadcn.com/docs/components/radix/command)：已运行 shadcn docs 并读取官方页面，复用 Group、Separator、Shortcut 及既有焦点机制。

不新增依赖选型问题；不将源码推断写成性能通过。
