# Wails v3 调研

核验日期：2026-11-04。

## 官方事实

[安装文档](https://wails.io/quick-start/installation/)要求 Go 1.26+ 构建工具链；[官方仓库](https://github.com/wailsapp/wails/blob/master/README.md)列出 v3.0.0 稳定版。平台使用系统 WebView，原生依赖须按系统检查。

## 项目决定

采用 Wails v3.0.0 首版基线，CLI 和 Go module 同版本；Go 最低 1.26。React + TypeScript 前端通过生成绑定调用应用服务，事件只传折栈 DTO。实际模板名、生成绑定路径和取消 API 必须在 Phase 0 由固定 CLI 帮助和最小实验确认，不能照抄 Wails v2。

## 未验证与实施门禁

doctor 失败原因未完全定位；本轮不改变系统配置。Phase 0 需证明最小窗口、主题、绑定、事件订阅与取消工作；五平台原生构建和玻璃视觉不能以 `GOOS` 交叉编译替代。详见 environment.md 与平台矩阵。

## Phase 0 后续实证

React TS 模板名为 react；事件 application.RegisterEvent + app.Event.Emit；Events.On 返回注销函数；生成命令 wails3 generate bindings -ts -i。已实现一个服务四个方法、七个模型、一个注册事件。doctor 原因为沙箱内 sysctl 禁止，沙箱外 ready=true。本机窗口和取消已通过，见 phase0-validation.md。
