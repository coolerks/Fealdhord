# 多渠道通知与 AI 定时任务

「设置 → 通知」管理渠道、文章推送规则、定时任务和投递历史。关闭窗口后仍在后台运行时继续投递。退出折栈后，重新启动恢复持久队列和到期任务，并将到期刷新发现的文章补入通知。源站在恢复时已不再提供的条目无法找回。

## 渠道

在「通知渠道 → 编辑」配置收件白名单。支持多收件人的连接可填写多个 ID（每行一个，最多 50 个）。「发送测试」逐项验证平台是否接受，只有已通过测试的收件目标会进入推送队列。渠道可以同时有已验证和待验证的收件目标。

| 平台 | 私发或指定收件目标 | 群或频道 Webhook | 内容 |
| --- | --- | --- | --- |
| [Telegram](https://core.telegram.org/bots/api#sendmessage) | Bot Token 与 Chat ID。私聊、群使用数字 ID，公开频道也支持 `@频道用户名` | — | 可点击标题、正文与图片 |
| [Discord](https://docs.discord.com/developers/resources/user#create-dm) | Bot Token 与 Discord 用户 ID，机器人建立私聊 | 频道 Webhook | Embed 标题链接、正文与图片 |
| [Slack](https://docs.slack.dev/reference/methods/conversations.open/) | Bot Token 与工作区成员 ID（`U…` 或 `W…`），应用需有打开单人私聊和发消息的权限 | Incoming Webhook | Block Kit 链接、正文与图片 |
| [钉钉](https://open.dingtalk.com/document/orgapp/chatbots-send-one-on-one-chat-messages-in-batches) | AppKey、AppSecret、可选 RobotCode，以及企业成员 UserID | 群 Webhook 与可选加签密钥 | Markdown 链接、正文与图片外链 |
| [飞书](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/message/create)、[Lark](https://open.larksuite.com/document/server-docs/im-v1/message/create) | 各自区域的 AppID 和 AppSecret，成员 ID 默认使用 `open_id`，也支持 `user_id:…` 和 `union_id:…` | 群机器人 Webhook 与可选加签密钥。Webhook 上传图片另需应用上传凭据 | 富文本链接、正文与上传图片 |
| [企业微信](https://developer.work.weixin.qq.com/document/path/90236) | CorpID、应用 Secret、AgentID，以及应用可见范围内的企业成员 UserID（`touser`） | 群 Webhook | Markdown 链接、正文或图文卡片 |
| [个人微信](https://github.com/Tencent/openclaw-weixin/blob/main/docs/protocol.md) | 每个私聊扫码绑定并主动发一句消息。白名单可从已绑定私聊中多选 | — | 纯文本链接与 iLink 图片上传 |
| [QQ](https://github.com/tencent-connect/bot-docs/blob/main/docs/develop/api-v2/server-inter/message/send-receive/send.md) | AppID、AppSecret 与 `c2c:用户 OpenID`、`group:群 OpenID` 或 `channel:子频道 ID` | — | 文本链接，按目标能力生成图片消息 |

群或频道 Webhook 固定投递给绑定会话。需要个人定向收件时，添加相应的应用机器人连接。规则和 AI 工具选择已配置渠道，每个渠道会投递给其中已验证的收件目标。发送测试也只使用保存的目标。

旧配置中的单一目标和测试状态会迁移至白名单，已排队消息保留原收件目标和投递记录。移除目标或更换连接凭据后，旧配置中尚未开始的投递会取消。重新添加目标需要重新测试，旧投递不会恢复。Webhook、令牌、加签和上传凭据使用系统密钥库。图片根据远程图片偏好和平台能力发送，图片抓取或上传失败时继续处理文字。

平台接口接受请求与接收端实际收到消息是不同状态。九渠道真实送达、微信与 QQ 的隔时主动推送，以及各端实际图片样式待使用授权目标验证。

## 规则、补齐与状态

- 规则可选多个渠道和分类。父分类覆盖其子分类。首次订阅历史、304 和重复更新不产生新文章事件。对最近两天发现的未匹配文章，分类首次匹配时可补入。正文使用已有纯文本，按平台限制分片。
- 逐条规则正常情况下立即排队；退出期间首次发现的文章按恢复批次组织。每日规则按所选时区、收录区间与发送时间组织。跨午夜支持左闭右开区间。夏令时缺失时点顺延，重复时点仅运行一次。恢复文章带有离线区间内可靠的发布时间时，可进入对应的错过窗口。
- SQLite 持久化文章事件、游标、每日条目、运行及逐目标消息片段。为该规则已经投递或已经排队的条目保留稳定标识。网络发送在事务外。一个目的地失败时，其他目的地继续处理。429 有限重试。进入发送阶段后未收到结果的片段标记「结果未知」，重发需要明确操作。投递历史展示最近 100 条，任务运行记录每项展示最近 50 次。

## AI 定时任务

在全局 AI 输入框描述一次性、每日、工作日或指定星期任务。支持固定提醒、按分类的文章列表与 AI 摘要，可指定回看 1 到 30 天。确认卡展示时区、下次时间和渠道。对话中的 `/定时任务` 打开管理页；也可要求 AI 查看、修改、暂停、恢复或删除任务。没有外部渠道时，结果保存在应用的运行记录中并使用应用通知。AI 摘要使用「摘要」用途分配的模型。

后台用已确认的配置执行，只连接用户配置的目的地。摘要模型处理文章内容时使用固定数据范围，不执行文章中的指令。任务结果与待发片段一起提交，恢复已保存的结果时直接续发。周期任务错过多个时点时处理一个合并运行，从下一次未来时间继续。摘要模型仍受实际模型、预算和连接限制。所有 AI 写入和外发走工具确认。MCP 通过现有显式工具配置选择可见项。
