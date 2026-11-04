# 原生阅读体验设计

main 装配原生窗口选项与 macOS application Menu，内部 desktop 包定义白名单命令事件。前端 lib/commands 为统一命令入口；macOS event、Windows Menubar、ContextMenu 和 Palette 共享执行器。Windows 保留系统窗口边框，菜单栏在应用中实现，避免伪造原生交通灯。

MacBackdropTranslucent＋HiddenInset＋透明 WebView／HTML 背景，让侧栏区域露出真正 AppKit 材质，主内容仍不透明。顶部预留交通灯与可拖动区，不画第二套标题栏。中性灰／白，导航黑灰图标，不固定绿色强调色。默认 reader_open=false，只有选择文章后显示 Reader；Masonry 支持内容全宽。

选择 lucide-animated（MIT，Motion＋Lucide）官方 registry 按图标导入源码，保留许可及来源；统一适配 ref startAnimation／stopAnimation，父按钮 Hover／Focus 触发，Reduced Motion 不启动内部路径动画。业务按钮尺寸不移动，虚拟列表不逐卡创建新动画循环。

菜单编辑动作通过原生 EditMenu／受控文本操作，浏览器右键由 Radix ContextMenu 与 Wails DefaultContextMenuDisabled 双层阻止。可观察错误不静默吞掉。用户范围覆盖早期绿色及常驻三栏设计，相关规格同步更新。
