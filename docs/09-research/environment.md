# 本机与仓库初始基线

以下是规格阶段的初始记录，后续已完成 Phase 0；最新结论见 [Phase 0 验证](phase0-validation.md)。

检查日期：2026-11-04，macOS Apple Silicon。以下为本轮实际命令结果，不是平台认证。

| 检查 | 结果 | 影响 |
| --- | --- | --- |
| Git | main，初始提交 51663f7，已有未跟踪 Trellis 与平台配置 | 保留全部用户文件，不重建仓库 |
| 应用源码 | 无 go.mod、frontend、migrations | 应用构建与测试暂不可运行 |
| go version | go1.27.3 darwin/arm64 | 高于 Wails 官方最低要求 |
| node / npm | v24.19.0 / 11.17.0 | 仅本机可执行，不等于前端兼容验证 |
| pnpm | 11.25.0，路径位于 Codex runtime fallback | 后续必须声明项目包管理器版本，不依赖此私有路径 |
| Wails 二进制元数据 | github.com/wailsapp/wails/v3 v3.0.0 | 由 go version -m 确认，未装新工具 |
| wails3 doctor | 扫描退出 1，仅输出 exit status 1 | Phase 0 前需修复或取得等价分项证据 |
| wails3 doctor -json | 退出 1，无可用报告 | 不能认定环境正常 |
| sqlc version | v1.30.0 | 生成器版本需单独锁定 |
| goose | 未安装 | 后续采用固定版本 CLI 或嵌入式库 |
| sqlite3 --version | 3.54.0 | 系统 CLI，不代表未来 Go 驱动 SQLite 版本 |
| xcode-select -p | /Library/Developer/CommandLineTools | 当前选中 CLT |
| xcodebuild -version | 退出 1，提示需要 Xcode | 已知单项失败；尚不能断言是 doctor 唯一根因 |
| xcrun --show-sdk-path | CLT SDK 路径，退出 0 | SDK 可定位，未验证 Wails 编译 |

## 后续环境门禁

记录 `wails3 doctor` 完整诊断，核对官方平台依赖。不得自动改 xcode-select、安装完整 Xcode 或升级全局工具。项目应固定 Go 工具链、Wails CLI／module、Node／包管理器和生成器；在标准路径之外的本机安装只能算环境事实。

Windows、macOS Intel、Linux amd64／arm64 均未运行。当前没有应用启动截图、跨平台构建或性能结果。

验收 ENV-01：Phase 0 记录固定工具链、doctor 诊断和真实窗口／绑定／事件往返；目前尚未满足。源码检查补充：Wails v3.0.0 的 internal/doctor/doctor_darwin.go 把 xcodebuild 用于可选 iOS 工具信息，失败不直接终止 doctor，因此不能因该命令失败要求安装完整 Xcode。collectSystemInfo 也可能在系统信息获取时返回错误，需后续定位。
