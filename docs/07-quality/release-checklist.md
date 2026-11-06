# 发布门禁

当前未进入发布阶段。

- RSS 基础版：零 AI 的端到端订阅、刷新、分类、双视图、首图、Reader、搜索、OPML、已读／收藏与主题完成。
- Shell 从早期具备正式视觉，语义动效和 Reduced Motion 有实际证据；发布阶段只收敛已实现体验。
- Go 格式／vet／tests，TypeScript strict／lint／component／build，相关 E2E 和 Adapter contracts 通过。
- 数据迁移、FTS 同步、备份恢复、凭据不可用 fallback、取消和重启恢复通过。
- 平台矩阵五行关键项有证据，打包／安装／启动／卸载、系统 WebView、签名／公证按目标平台记录；无证不标完成。
- 安全清单、权限确认、MCP 认证、密钥和日志扫描通过，无真实收费 API 进入单测。
- 固定依赖、工具链、锁文件和生成代码，许可证与安全公告复核；不依赖 Codex 本机 runtime 路径。
- README 启动／测试／构建命令可复现，CURRENT／ROADMAP／COMPLETED／追踪表与 LATEST 同步。

完成判断 RELEASE-01：上述门禁逐项给出证据或明确不适用理由；阻塞项未消除不发布正式完成声明。

## Alpha 自动构建

推送 `v0.1.0-alpha.1` 这类版本标签会触发 GitHub Actions，普通分支提交不会触发。全部平台构建通过后，工作流会发布安装包和 `SHA256SUMS.txt`。带预发布后缀的标签在 GitHub Releases 标为预发布。

| 下载包 | 适用环境 |
| --- | --- |
| Windows x64 ZIP | 解压运行 `Fealdhord.exe`，系统需要 WebView2 |
| macOS 通用 ZIP | macOS 13 及以上，包含 Apple Silicon 和 Intel 架构，使用临时签名 |
| Linux x64 DEB | Ubuntu 24.04 或具有相应运行库的兼容系统，安装时解析 GTK 4 和 WebKitGTK 6 依赖 |

应用图标源文件在 `build/appicon.svg`。更新对应的 1024px PNG 后，可运行 `wails3 task icons` 派生 ICNS 和 ICO。托盘源文件在 `build/tray.svg` 与 `build/tray-contrast.svg`。更新时同步导出 64px 透明 PNG。macOS 使用模板着色，Windows 和 Linux 使用黑白描边，Linux 托盘显示取决于桌面环境是否支持 StatusNotifierItem。
