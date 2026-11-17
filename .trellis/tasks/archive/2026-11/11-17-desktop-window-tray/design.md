# 技术设计

## 窗口生命周期

目前 `main.go` 启用 macOS 关闭最后窗口后退出，Windows 和 Linux 使用 Wails 的默认退出行为。前端自绘关闭按钮通过 `Window.Close()` 进入相同的 Wails 关闭事件。

主窗口通过 `events.Common.WindowClosing` hook 取消默认销毁。关闭窗口时按已保存偏好隐藏原窗口或调用 `App.Quit()`。三端关闭自动退出选项，由应用负责决定何时退出，以免 macOS 隐藏最后一个可见窗口后进入退出流程。后台运行保留 WebView 和刷新服务。macOS 保持普通激活策略，Dock 运行标记由系统显示。现有显式退出入口沿用应用级清理流程。

现有托盘「显示折栈」操作先还原最小化窗口，再显示并聚焦。将同一恢复动作绑定至托盘主按钮。Wails 3.0.0 已处理 macOS Dock 的 `ApplicationShouldHandleReopen` 并显示隐藏窗口，验证原生恢复路径。Linux GTK 使用非唯一激活模式，在应用启动入口接入 Wails 单实例通知，使启动器重新启动命令时唤回现有窗口。GTK 应用标识与已安装的 `io.fealdhord.desktop` 启动项对齐。数据库锁继续保护本地存储。

## 设置

在「外观与浏览」显示「关闭窗口时」，沿用现有的共享分段控件提供「在后台运行」和「退出应用」。通过 `SettingsService` 和现有 revision 校验保存布尔字段 `quit_on_window_close`。缺字段时使用布尔零值，旧 JSON 无需迁移。原生关闭事件使用启动时加载、保存成功后按 revision 更新的偏好，避免窗口关闭时访问数据库；同时防止迟到的保存通知覆盖更新值。

## 图标资源

`app_icons.go` 当前仅给 macOS 使用模板，其余平台使用黑白资源。Windows 改用彩色应用图标；Linux 沿用现有托盘资源。Windows 任务栏使用 PE 图标资源；调整 Windows 专用图标的透明留白，使白色底板与折页整体放大。把 Windows 专用资源接入本机和 CI 的 ICO／SYSo 生成，从同一图案派生。macOS Dock 与状态栏继续使用现有资源。

交互和打包不要求用户配置迁移。Linux 桌面环境决定托盘可见性、应用分组与聚焦策略。macOS 运行指示器的显示取决于系统 Dock 设置。
