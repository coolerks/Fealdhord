# ADR-0013：Reader 提取器与登记式正文资源

状态：已接受，实施中。日期：2026-11-05。

## 背景

原技术调研选定的 go-shiori/go-readability 已归档，官方 README 指向后继模块。完整 HTML 阅读又要求图片和链接不能绕过桌面权限边界。

## 决定

固定 codeberg.org/readeck/go-readability/v2 v2.1.3（MIT，模块 Go 1.23），仅调用 Parser.ParseAndMutate 操作统一网关抓取的有界 DOM；MaxElemsToParse=100000，不使用 FromURL。输入最多5MiB／20秒，解析前后检查取消，同一服务最多2个提取；第三方异常转安全错误，失败保留订阅正文。

订阅和提取正文分开缓存；正文资源登记 article_id／content_hash／mode，客户端只获得安全 HTML 中的本地图片 ID 与链接 ID。Go 再验证当前文章和图片策略；网页补图开关不等于原文提取，提取必须用户显式触发。原文／正文链接通过系统浏览器，不在有 Wails 绑定的 WebView 中导航。

## 替代方案

不新引入已归档旧库；不直接把远端 HTML 或图片 src 交给 React。资源登记和增量迁移增加少量存储，但为禁图、取消、陈旧正文隔离与审阅提供统一边界。最多32张正文图／256个链接，超出保留文字且不发请求；无需 AI。

## 后果与验证

[原官方仓库](https://github.com/go-shiori/go-readability)给出归档／迁移指引；本机通过 Go proxy 核验 v2.1.3 模块、源码与许可证。Codeberg 网页本轮未能打开，因此接口事实以下载的固定版本源码为准。使用自制中英文／代码／表格、安全 HTML、取消／版本失效与禁图 fixture 验证，不以库 README 代替功能证据。
