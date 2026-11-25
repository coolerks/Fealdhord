# Linux 应用图标与窗口栏

Linux 主窗口关闭系统标题栏，和 Windows 共用菜单、侧栏切换、窗口控制与空白拖动区。最大化按钮读取当前窗口状态并订阅 Linux Wails 的 `common:WindowMaximise` 和 `common:WindowUnMaximise`。窗口内的最小化、最大化、关闭与双击动作沿用既有命令。Linux 窗口栏使用实色。

Linux DEB 保留 `io.fealdhord.desktop` 入口文件名和单实例标识。GTK 应用 ID、`StartupWMClass` 与 `Icon` 使用 `io.fealdhord`。发布构建从原应用图标生成 48×48 和 256×256 PNG，安装到 hicolor 对应目录。

## 验证

- 前端 App 回归 82 项通过，覆盖 Windows 和 Linux 窗口控件、双击及最大化／还原状态。生产构建包含 TypeScript 检查并通过。Lint 无错误，保留文章列表的 TanStack Virtual 既有警告。
- `go test ./internal/desktop .`、`go vet ./internal/desktop .` 与 Windows amd64 生产交叉编译通过。
- 通过 YAML、Bash 语法及打包配置核对，检查了入口文件名、GTK ID、窗口类别、图标名与两个目标尺寸。
- 当前在 macOS 上完成开发验证，尚未实际生成新版 DEB 或在 GNOME 上安装启动。实机验收覆盖应用网格图标、运行中窗口归组、启动器唤回现有窗口、窗口拖动和边缘缩放，以及外部最大化状态同步。
