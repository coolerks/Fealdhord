# 多渠道文章通知与 AI 定时任务调研

2026-11-21。以下保留初步调研。已确认本机运行、重开补齐，最终设计建议与技术取舍见 [评审记录](notification-design-review.md)。

## 现有接入点

- `main.go` 已注册 Go 刷新调度器和应用关闭前的清理。应用支持关闭窗口后在后台运行。当前桌面与应用内通知由刷新事件驱动。
- `internal/database/feeds.go` 的 `CommitFeed` 在事务内写文章，其返回 ID 也供媒体摄取使用，不能直接当作首次发现事件。应在文章插入分支生成持久事件，并为初次订阅导入历史、正文更新和重复刷新定义明确的过滤行为。
- `internal/appagent/tools.go` 已有具备参数校验和逐次确认的工具注册表。`internal/aiconfig/credentials.go` 使用系统密钥库，可沿用凭据引用的模式。

## 渠道与待验收能力

| 渠道 | 拟接入方式 | 图文策略与验收 |
| --- | --- | --- |
| Telegram | 已获授权的 Bot Token + Chat ID | 支持点击标题与图片消息，验证私聊已发起或机器人已加入目标会话。[Bot API](https://core.telegram.org/bots/api) |
| Discord | 频道 Webhook | Embed 标题链接、图片或附件。关掉自动提及，按频道限制拆分。[Webhook](https://docs.discord.com/developers/resources/webhook#execute-webhook) |
| Slack | Incoming Webhook | Block Kit 标题链接、图片块。Webhook 一般绑定一个频道，多个目标建多个连接。[Incoming Webhooks](https://docs.slack.dev/messaging/sending-messages-using-incoming-webhooks/) |
| 钉钉 | 群自定义机器人 Webhook，支持加签 | Markdown 或链接卡片。图片取决于所选载荷和公开图片 URL 是否可达。[自定义机器人](https://open.dingtalk.com/document/robots/custom-robot-access) |
| 飞书 | 国内群自定义机器人 Webhook | 富文本或卡片链接。分别验证签名、图片引用和上传所需凭据。[自定义机器人](https://open.feishu.cn/document/client-docs/bot-v3/add-custom-bot) |
| Lark | 国际群自定义机器人 Webhook | 可与飞书共用适配器，分别校验服务地址和凭据，独立实机验收。[Custom bot](https://open.larksuite.com/document/client-docs/bot-v3/add-custom-bot) |
| 企业微信 | 企业微信群机器人 Webhook | Markdown 或图文，按群机器人实际支持范围发送图片。[群机器人配置](https://developer.work.weixin.qq.com/document/path/91770) |
| 微信 | 腾讯 iLink 扫码绑定个人微信，识别已关联私聊 | 文本带原始链接，支持时另发首图。用真实账号验证会话绑定、令牌重启恢复、隔时主动发送和失效提示。拟审查 Go SDK 的许可证、超时控制和持久化接口。[腾讯协议](https://github.com/Tencent/openclaw-weixin/blob/main/docs/protocol.md) [Go SDK](https://github.com/openilink/openilink-sdk-go) |
| QQ | 官方机器人及当前账号获准的主动消息路径 | 通用文档仍列出旧主动接口停止通知，腾讯维护的插件已展示有条件的定时推送。按具体目标实测授权与平台拦截；没有验证主动能力的目标不能启用即时／定时推送。[发送文档](https://github.com/tencent-connect/bot-docs/blob/main/docs/develop/api-v2/server-inter/message/send-receive/send.md) [腾讯官方插件](https://github.com/tencent-connect/openclaw-qqbot/blob/main/README.zh.md) |

对纯发送型 Webhook 使用稳定的公开 API。对扫码、长轮询、鉴权续期和媒体等复杂协议，优先评估维护中的 Go SDK。Hermes 和 OpenClaw 作为协议与能力参照，运行时可直接连接渠道。[Hermes 渠道](https://hermes-agent.nousresearch.com/docs/integrations) [OpenClaw 渠道](https://openclaw.ai/integrations)

## 推送与调度方案

- 「设置 → 通知」增加连接、推送规则和投递记录。保存连接后，用用户主动触发的样例消息验证真实目的地。规则选择多个连接、全部或指定分类、逐条或每日汇总。每日录入发送时间、IANA 时区和文章收录时间段，支持跨午夜。文章以首次入库时间归窗，收藏、阅读、编辑正文等动作不产生通知。同一规则内多分类命中合并。
- 统一文章消息包含标题、可选正文、首图及原文链接。正文使用已保存的安全纯文本，按渠道长度限制拆分或明确截断并保留原文链接。单独控制是否发送首图。使用已授权的媒体来源，经现有网络策略与大小限制；图片不可用仍发送文字。各渠道使用自己的链接、转义和提及规则。
- SQLite 持久化首次入库事件、窗口游标、任务定义、任务运行和逐目标投递状态。写入文章与事件同事务，网络发送在事务外。目标失败独立处理。确定未发送的可恢复故障有限重试并尊重限流；服务器是否已经收件未知时保留该状态，不盲目重发。凭据存系统密钥库，工具和日志只使用连接 ID、状态与脱敏错误。
- AI 输入框可用自然语言创建一次性或重复任务，后续可查看、改时间、暂停、恢复和删除。创建或修改时显示任务、时区、下次执行、数据范围、操作和目标渠道供确认。任务可以执行固定提醒、RSS 查询与摘要等已授权操作，结果写到应用内或选定渠道。管理入口展示下次执行与最近结果。借鉴 [ChatGPT 定时任务](https://help.openai.com/en/articles/10291617-scheduled-tasks-in-chatgpt) 的创建、编辑及暂停流程。
- 工具候选为 `notification.channel.list`、`notification.rule.list`、`notification.send`、`schedule.list`、`schedule.get`、`schedule.create`、`schedule.update`、`schedule.pause`、`schedule.resume`、`schedule.delete`。连接凭据在设置中管理。外发与任务修改沿用确认，任务只在已确认的目的地和数据范围内执行。更改权限、禁用或删除连接可阻止未开始的投递。外部文章内容不能新增任务、修改授权或触发工具写操作。

## 验收与待决项

用临时数据库和受控时钟验证首次入库与重复刷新、多分类和多目标、首次订阅不回灌旧文章、跨午夜和夏令时、重启补偿、并发去重、限流及结果未知。用 HTTP 合约测试验证平台载荷，使用授权账号进行九个渠道的真实接收测试。实测分别记录文本、链接、首图、可否隔时主动发送，不拿 HTTP 200 代替送达证据。

已确认首版随折栈进程运行，应用退出后在下次打开时自动补齐。补发分组和迟到文章处理见评审记录。
