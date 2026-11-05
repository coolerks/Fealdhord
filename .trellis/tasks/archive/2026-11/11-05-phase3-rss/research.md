# Phase 3 实现前检查

日期：2026-11-05。源码与模块元数据核验已完成，尚未实现新能力。

## 真实缺口

internal/database/library.go 的 EnsureFeed 只登记身份，UpsertArticle 为单篇事务。feeds 已有 etag、last_modified、检查／成功／到期时间、错误计数与 revision；article_media 的 status CHECK 缺 blocked。internal/library/service.go 仅输出 /media/ 封面，main 尚无媒体 handler 或 RSS 服务装配。settings.Preferences 无媒体隐私字段。上述缺口必须在各自子任务落地，不能只解锁前端刷新按钮。

## 依赖核验

go list -m -json 的当前结果：gofeed v1.5.0（2026-09-25，Go 1.25）、goquery v1.13.0（2026-08-27，Go 1.25）、bluemonday v1.0.27（Go 1.19）。项目 Go 1.26／工具链 1.27.3 可满足声明要求；这仅证明版本兼容声明，不证明畸形源、安全与维护质量。实现时固定上述已核验版本并运行测试／安全检查，不因 latest 在之后变化静默升级。

## 官方依据

- [gofeed](https://github.com/mmcdole/gofeed)：统一 RSS／Atom／JSON Feed，使用本地字节解析而非库自行联网。
- [Go net/http](https://pkg.go.dev/net/http)：通过 Transport／DialContext／CheckRedirect 构建受控连接与跳转边界。
- [JSON Feed 1.1](https://www.jsonfeed.org/version/1.1/)：区分 content_text／content_html、条目 image／banner_image，不把缺标题视为整源失败。

调度采用单个到期唤醒器和有界 worker，不为每源永久创建 goroutine；原调研列出的 cron 不是必须引入的运行依赖。是否需要迁移先由实际 schema 与原子性决定，新增版本，不修改历史迁移。
