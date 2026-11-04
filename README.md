# 折栈 Fealdhord

本地优先的 AI RSS 阅读器与 Agent 工作空间，目标支持 Windows、macOS 与 Linux。零 AI 配置也应完整阅读。

当前完成 Phase 0–1：Wails 桌面工程、正式三栏阅读 Shell、虚拟列表／瀑布流、主题／动效／键盘、SQLite 偏好恢复。生产文章库为空；RSS、文章数据库与 AI 能力尚未实现。

[文档入口](docs/README.md) · [当前状态](docs/01-status/CURRENT.md) · [路线图](docs/01-status/ROADMAP.md) · [Phase 1 验收](docs/09-research/phase1-validation.md)

## 开发环境

Go 1.26+（偏好工具链 1.27.3）、Wails CLI／module／runtime 3.0.0、Node 24.19.0，推荐 npm 11.17.0（10.2.5 也通过锁文件安装）。项目依赖固定于 go.mod／go.sum 和 frontend/package-lock.json，不依赖私有工具路径。安装 CLI：`go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0`。各系统原生依赖先执行 `wails3 doctor`；沙箱拒绝 sysctl 不等同于本机环境缺失。

## 安装、构建与运行

```sh
cd frontend
npm ci
cd ..
wails3 build
wails3 task run
```

本机 macOS arm64 已验证。构建输出 bin/Fealdhord；macOS run 创建带 ad-hoc 签名的 bin/Fealdhord.app，窗口是真实原生 WebView。尚不提供正式安装包／签名公证或其他平台通过承诺。当前 macOS 编译目标 13.0，最低版本实机未测。

开发热更新入口：`wails3 task dev`，Vite 监听 127.0.0.1:9245；该配置已建立，热更新交互尚未单独验收。纯浏览器访问没有 Go 绑定，不能作为桌面运行替代。

## 测试

```sh
wails3 task check
go test -race ./internal/app/... ./internal/settings/...
```

check 顺序执行 Go test／vet 和前端 typecheck／lint／test。生成绑定：`wails3 task bindings`，生成输出不可手改。Phase 0 原生绑定往返、进度、取消和完成已经本机窗口验证；Phase 1 使用组件／单元测试与原生构建，不重复 E2E。构建与 Go 全包检查应顺序执行，避免 dist 重建和 go:embed 读取竞争。

说明文档中文、主会话独立完成、禁止子代理。开始工作前读取 AGENTS.md；修改契约同步文档，交接入口 docs/08-handoffs/LATEST.md。

界面偏好位于系统 os.UserConfigDir()/Fealdhord/fealdhord.db，通过 goose 迁移与 sqlc 查询管理。修改查询后执行 `sqlc generate`（v1.30.0），修改 Go DTO 后执行 `wails3 task bindings`。不要直接编辑生成文件或活跃 SQLite 主文件。
