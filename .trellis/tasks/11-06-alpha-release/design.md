# 方案

在 build 目录保留 SVG 与运行时 PNG，复用 Wails 图标生成器派生 ICNS 与 Windows 图标资源。macOS 托盘用系统模板，Windows 和 Linux 使用黑白描边图。GitHub Actions 以原生运行器满足 WebView 依赖，macOS 在 Apple Silicon 上交叉编译 Intel 并通过 lipo 合并。发布任务在矩阵全部通过后上传构建物和 SHA-256。
