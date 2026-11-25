# Linux 桌面图标与窗口栏

## 目标

让通过 DEB 安装的折栈在 Linux 桌面可找到、可辨认，并让 Linux 主窗口与 Windows 使用相同的一行菜单和窗口控件。

## 已确认事实

- 修复前，安装包只把彩色图标放入 hicolor 的 1024×1024 目录（`.github/workflows/release.yml`）。GTK 应用 ID 包含 `.desktop` 后缀，与已安装的桌面入口文件去掉扩展名后的名称不同（`build/linux/Fealdhord.desktop`、`main.go`）。
- 目前只有 Windows 主窗口关闭系统装饰（`internal/desktop/menu.go:32`）。Linux 前端已有同一套应用菜单，窗口控件和标题拖动区仅在 Windows 渲染（`frontend/src/app/ApplicationMenu.tsx:38`）。固定版本的 Wails 在 Linux 用 Common 窗口事件通知最大化与还原。

## 需求与验收

- DEB 在 hicolor 的标准目录安装由现有应用图标生成的 48×48 和 256×256 PNG。安装后，GNOME 应用网格与运行中窗口可使用折栈图标。保留已安装桌面入口文件名与单实例标识，GTK 应用 ID、窗口类别和图标对应入口文件去掉扩展名后的名称。
- Linux 主窗口首行显示侧栏切换、应用菜单、可拖动空白区域与最小化、最大化／还原、关闭按钮。系统标题栏不再额外占一行。
- Linux 菜单、键盘操作和窗口边缘缩放可用。点击控件、双击空白区或通过窗口管理器切换最大化时，按钮的图标与可访问名称跟随状态。
- 以安装包内容检查、前端回归和构建覆盖开发验证。Linux 实机窗口行为与 GNOME 图标显示单独记录实测状态。

## 范围

沿用现有 Windows 菜单与窗口控件，Linux 使用实色背景。范围限于 Linux 应用启动器及主窗口。保持 Windows 窗口材质、macOS 原生菜单及各平台托盘实现。

## 验证记录

前端 App 回归 82 项、生产构建、Go 桌面检查、Windows amd64 交叉编译与 Linux 打包配置核对通过。实际 DEB 构建及 GNOME 桌面验证待执行。详见 [修改与验证记录](../../../../../docs/08-handoffs/history/2026-11-25-linux-desktop-chrome.md)。
