# 实施与验收

1. 接入图标、托盘和原生应用打包。
2. 配置标签触发、三平台构建和预发布，核对分支提交不触发构建。
3. 检查本机构建和包结构，提交后推送分支与 Alpha 标签，观察 GitHub Actions 构建及发布结果。

## 验收结果

[v0.1.0-alpha.2](https://github.com/coolerks/Fealdhord/releases/tag/v0.1.0-alpha.2) 已作为预发布上传 Windows x64 ZIP、macOS 通用 ZIP、Linux x64 DEB 和 SHA256SUMS.txt。三份摘要与 GitHub 资产记录一致。首轮 Alpha 构建因修正托盘图标而取消。macOS 使用临时签名，三平台安装与启动未做实机验收。

Go 测试、vet、前端生产构建、actionlint、本机 Windows 交叉编译与 macOS 双架构包签名均通过。远端普通分支提交未触发工作流。
