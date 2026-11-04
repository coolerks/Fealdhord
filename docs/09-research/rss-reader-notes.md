# RSS、正文与媒体调研

核验日期：2026-11-04。

## 官方事实

[gofeed](https://github.com/mmcdole/gofeed)当前文档列出 RSS、Atom 和 JSON Feed 1.0／1.1，并保留扩展字段。[JSON Feed 1.1](https://www.jsonfeed.org/version/1.1/)定义条目内容与图片字段。[goquery](https://github.com/PuerkitoBio/goquery)用于 HTML 结构选择；[go-readability](https://github.com/go-shiori/go-readability)提取阅读正文；[bluemonday](https://github.com/microcosm-cc/bluemonday)清理不可信 HTML。这些库不是 SSRF 防火墙。

## 项目决定

统一 HTTP 安全网关先抓取有界字节，再交给 Parser／Readability，避免库自行获取页面绕过网络策略。首图保留候选优先级与来源；清理正文与封面裁剪分开处理。发现、媒体和 Reader 共用 URL／重定向／超时规则。

## 维护状态与未验证

本轮已核对官方仓库能力；仅凭 README 无法证明全部畸形源或长期维护质量。首版保留指定库，不无证替换。Phase 0 锁依赖时检查 release、最近提交、支持 Go 版本、许可证与安全公告；Phase 3 用自制合法 fixture 验证 RSS／Atom／JSON、扩展和日期。go-readability 的中文、代码块与表格保真度需 fixture，失败回退订阅正文，不绕付费墙。

[robfig/cron](https://github.com/robfig/cron)用于周期唤醒，不为每个 Feed 创建永久 goroutine；到期队列由数据库决定。
