# 原生窗口与动画研究

2026-11-04。本地固定 Wails v3.0.0 源码确认 MacTitleBarHiddenInset、MacBackdropTranslucent、BackgroundTypeTranslucent、DefaultContextMenuDisabled 和 Menu.AddRole／SetApplicationMenu。官方文档：https://v3.wails.io/features/menus/application/ ，https://v3.wails.io/features/windows/options/ 。

lucide-animated 官方 https://lucide-animated.com/index.md 、https://github.com/pqoqubbw/icons ：MIT，基于 Motion／Lucide，按图标 registry 安装，源码可审查。优先采用路径分段／描边／局部旋转和弹簧，避免只给整个 SVG 位移。与 Lottie／Rive 相比无需引入大播放器或制作二进制资产，适合现有统一线性图标与 React Motion；复杂插画将来按实际需要另选。

用户截图中窗口控制在侧栏顶端，主内容留白充足、内容标题与工具操作分开；半透明来自真实窗口材质及透光背景，CSS blur 不能透过不透明 NSWindow。
