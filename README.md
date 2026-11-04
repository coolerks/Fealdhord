# 折栈 Fealdhord

本地优先的 AI RSS 阅读器与 Agent 工作空间，目标支持 Windows、macOS 与 Linux。零 AI 配置也应完整阅读。

当前完成 Phase 0：Wails 桌面工程、React／TypeScript 前端、真实生成绑定、事件／快照／取消与本机窗口验收。正式阅读 Shell、RSS、数据库和 AI 能力尚未实现。

[文档入口](docs/README.md) · [当前状态](docs/01-status/CURRENT.md) · [路线图](docs/01-status/ROADMAP.md) · [Phase 0 验收](docs/09-research/phase0-validation.md)

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
go test -race ./internal/app/...
```

check 顺序执行 Go test／vet 和前端 typecheck／lint／test。生成绑定：`wails3 task bindings`，生成输出不可手改。原生绑定往返、进度、取消和完成另经本机窗口实际验证。

说明文档中文、主会话独立完成、禁止子代理。开始工作前读取 AGENTS.md；修改契约同步文档，交接入口 docs/08-handoffs/LATEST.md。
