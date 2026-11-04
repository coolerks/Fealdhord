# Phase 0 实施与验证记录

日期：2026-11-04。分支：codex/desktop-bootstrap。任务：.trellis/tasks/archive/2026-11/11-04-desktop-bootstrap。

## 环境结论

沙箱内 `sysctl kern.osproductversion kern.osversion` 返回 Operation not permitted；固定 Wails 源码的系统版本检查直接调用此命令。经授权在沙箱外运行 `wails3 doctor -json`，得到 macOS 27.0、Apple M2、darwin/arm64、ready=true、diagnostics=null。没有安装 Xcode 或修改 xcode-select；iOS／Android／NSIS 缺项不是本次桌面启动阻塞。最初环境记录保留为历史，不覆盖失败证据。

工具链：Go 1.27.3（go.mod 下界 1.26.0，toolchain 1.27.3）、Wails module／CLI／runtime 3.0.0、Node 24.19.0、推荐 npm 11.17.0；实际安装进程的 npm 10.2.5 也已验证，允许范围 >=10.2.5 <12，以 package-lock 固定依赖。React 19.3.0、TypeScript 5.9.3、Vite 8.2.4、Vitest 5.1.0。Vitest 4.0.8 与 Vite 8 的类型冲突已通过固定声明兼容的 5.1.0 解决，没有 any／忽略类型来绕过。

首次 native 链接出现部署目标不一致；Taskfile 与 Info.plist 显式采用 macOS 13.0 作为当前构建目标，重建无该警告。这是工程目标，尚未在 macOS 13 实机验证；不宣称已认证该最低系统版本。

## 自动检查

- go test ./...：通过，应用包编译与基础服务测试通过。Go 会扫描 node_modules 中自带的 Go 文件，当前也通过；核心竞态测试只针对 internal/app。
- go test -race ./internal/app/...：5 项行为测试通过。
- go vet ./...：通过。
- npm run typecheck／lint：通过。
- npm test：5 项交互测试通过，包括 Start 响应前的乱序事件、取消和旧事件、失败恢复及卸载清理。
- npm run build 与 wails3 build：通过，原生可执行文件 bin/Fealdhord。
- wails3 task bundle:mac 与 codesign --verify --deep --strict：通过；仅本机 ad-hoc 签名，不是 Developer ID、公证或发布。

npm ci --offline 使用已缓存包成功安装固定依赖；再次生成绑定的 6 个文件哈希完全一致，生成器报告 1 服务／4 方法／7 模型／1 事件。统一命令 `wails3 task check` 最终通过全部检查。初版命令级 dir 未改变工作目录，已改为独立 frontend:check 任务的任务级 dir；已按 README 实际复核。gofmt -l 与 git diff --check 无输出。

## 真实原生窗口验收

通过本机 Fealdhord.app 与 Computer Use 访问真实 wails://localhost/ 页面，未使用浏览器 Mock：

1. GetInfo 显示桌面服务已连接、0.1.0-dev、darwin/arm64、Wails 3.0.0。
2. 开始检查后显示“正在检查”、1/20，开始按钮禁用、取消按钮启用。
3. 取消后显示“已取消”、16/20，开始恢复、取消禁用。
4. 再次开始后最终“检查完成”、20/20，按钮状态正确。
5. 原生窗口截图与可访问树确认内容完整、无横向溢出；这只验证 Phase 0 工程界面，不算 Phase 1 阅读界面验收。

## 尚未验证

Windows、macOS Intel、Linux 两架构、macOS 最低版本实机均未测；全平台发布与签名仍未完成。开发热更新命令已配置但未单独完成热更新交互验收。RSS、数据库、首图、正式双视图、AI 和 MCP 尚未实现。

文档静态检查结果：[Phase 0 文档检查](phase0-document-validation-result.json)，113 项源章节保持完整；规格阶段历史结果文件不覆盖。

## 提交前干净检出修正

绑定生成前以标准 Node fs API 创建忽略的 frontend/dist/.gitkeep，使 go:embed 在尚未构建前端时也能加载包。真实前端构建随后覆盖 dist，不跟踪生成产物。固定 Wails 生成器末尾空行保持原样；Git 空白检查只排除该 blank-at-eof 提示，不手改生成文件。
