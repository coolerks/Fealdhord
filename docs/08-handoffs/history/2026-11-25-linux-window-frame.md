# Linux 窗口阴影、圆角与菜单栏配色

GTK4 主窗口保留 GTK 客户端装饰，在首次显示前设置隐藏的标题栏部件。应用菜单仍为窗口首行。GTK 负责窗口阴影和外沿缩放，应用继续提供拖动、双击最大化和窗口按钮。GTK 同时裁切原生窗口和内部 WebView，浮动窗口的四角为 12px 圆角，最大化、全屏和平铺时贴齐边缘。

Linux 菜单栏和侧栏共享随应用主题切换的实色。GTK3 构建沿用既有无边框窗口。Windows 和 macOS 沿用各自的窗口行为和样式。

## 验证

- Windows、Linux 单行菜单和 macOS 主机菜单的 3 项前端回归通过，前端生产构建和 TypeScript 检查通过。Lint 无错误，保留文章列表的 TanStack Virtual 既有警告。
- macOS 上的 `go test ./internal/desktop .`、`go vet ./internal/desktop .` 与 Windows amd64 生产交叉编译通过。
- Linux 实机外观及交互由用户验证，覆盖浅色／深色配色、普通窗口阴影及四角、最大化后贴边、菜单栏拖动和外沿缩放。
