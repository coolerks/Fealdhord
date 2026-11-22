# 多渠道通知与 AI 定时任务

「设置 → 通知」管理渠道、文章推送规则和投递历史。关闭窗口后仍在后台运行时继续投递。退出折栈后，重新启动恢复持久队列和到期任务，并将到期刷新发现的文章补入通知。源站在恢复时已不再提供的条目无法找回。

## 渠道

在「通知渠道 → 编辑」配置收件白名单。支持多收件人的连接可填写多个 ID（每行一个，最多 50 个）。只有已验证的收件目标会进入推送队列。新增目标通过四位校验码验证。渠道可以同时有已验证和待验证的收件目标，已有已验证目标可继续使用「发送测试」。

添加定向通知渠道时，先填凭据、配对收件目标、确认配置，再验证收件。配对时给机器人发送「配对」，从识别出的目标中勾选所需收件人。也可以跳过，在确认页手填多个 ID，或保存后再配对。保存后，折栈自动把四位校验码发到各个未验证的目标。在第 4 步填对该目标的码，即可使用。群 Webhook 跳过配对，校验码直接发送至绑定会话。验证码在五分钟内有效，可手动重发。配对后 QQ 优先关联原消息回复校验码，后续主动通知仍受 QQ 平台权限和额度限制。

添加或编辑渠道时，所有平台均可通过「操作步骤」查看当前接入方式的配置说明。「查看官方文档」在系统默认浏览器打开对应指引，QQ 的正式环境和沙箱共用 [QQ 机器人文档](https://bot.qq.com/)，个人微信只显示「操作步骤」。

新建渠道时，同时支持机器人私发和群 Webhook 的平台默认选中机器人私发。企业微信默认选中「智能机器人（长连接）」。需要群或频道收件时，在「接入方式」选择「群或频道 Webhook」。

- QQ 已上线时选「正式环境」。未上线测试时选「沙箱」，在 QQ 开放平台的「沙箱配置」核对账号，再通过对应测试入口打开会话。在私聊发「配对」，在普通群或文字子频道 @机器人后发送「配对」。同一个渠道的配对、测试和后续投递使用同一环境。配对页显示 QQ 网关返回的机器人昵称，并区分连接与聊天事件收件。手机提示「未连接灵魂」且折栈未收到消息时，用手机上的「查看指引」核对当前会话对应的机器人及其接入状态。[QQ 沙箱配置](https://github.com/tencent-connect/bot-docs/blob/main/docs/README.md#71-配置沙箱环境) [QQ 事件订阅与通知](https://bot.q.qq.com/wiki/develop/api-v2/dev-prepare/interface-framework/event-emit.html)
- Telegram 和 Discord 可私聊机器人发送「配对」。Telegram 也可绑定能接收该消息的群聊或频道。已使用 Telegram Webhook 的机器人可以手填收件 ID。[Telegram Bot API](https://core.telegram.org/bots/api#getupdates) [Discord Gateway](https://docs.discord.com/developers/events/gateway)
- Slack 要启用 Socket Mode、订阅 `message.im`，填写与 Bot Token 对应的 App-Level Token。飞书和 Lark 需启用长连接并订阅 `im.message.receive_v1`。[Slack Socket Mode](https://docs.slack.dev/apis/events-api/using-socket-mode/) [飞书接收消息事件](https://open.larkenterprise.com/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/message/events/receive)
- 钉钉私聊配对在企业内部应用的「应用能力 → 机器人」配置消息接收模式为 Stream，发布版本并把操作者纳入应用可见范围。折栈通过机器人消息回调接收「配对」，无需添加独立的业务事件订阅。主动发送通知和收件校验码时，在「权限管理」开通「企业内机器人发送消息权限」并发布变更。[钉钉 Stream 机器人配置](https://open-dingtalk.github.io/developerpedia/docs/explore/tutorials/stream/bot/nodejs/create-bot/) [钉钉 Stream 协议](https://open-dingtalk.github.io/developerpedia/docs/learn/stream/protocol/) [钉钉单聊发送接口](https://open.dingtalk.com/document/orgapp/chatbots-send-one-on-one-chat-messages-in-batches)
- 微信先扫码授权，然后在「微信 ClawBot」私聊里发送「配对」。跳过后可从已有渠道继续配对。
- 企业微信新建渠道默认使用「智能机器人（长连接）」。在企业微信创建 API 模式的智能机器人，选择长连接，复制 Bot ID 和 Secret。通过机器人链接或二维码打开私聊。在折栈点「开始配对」，等连接就绪后在私聊发送「配对」，勾选目标，保存并回填四位校验码。该方式无需配置域名、可信 IP 或回调服务器。请为折栈使用独立的机器人，同一个 Bot ID 只能建立一个连接。[企业微信配置指引](https://open.work.weixin.qq.com/help2/pc/cat?doc_id=21657) [企业微信智能机器人 SDK](https://github.com/WecomTeam/aibot-node-sdk)

| 平台 | 私发或指定收件目标 | 群或频道 Webhook | 内容 |
| --- | --- | --- | --- |
| [Telegram](https://core.telegram.org/bots/api#sendmessage) | Bot Token 与收件 ID。私聊填数字用户 ID，群填会话 ID，公开频道也支持 `@频道用户名` | — | 可点击标题、正文与图片 |
| [Discord](https://docs.discord.com/developers/resources/user#create-dm) | Bot Token 与 Discord 用户 ID，机器人建立私聊 | 频道 Webhook | Embed 标题链接、正文与图片 |
| [Slack](https://docs.slack.dev/reference/methods/conversations.open/) | Bot Token 与工作区成员 ID（`U…` 或 `W…`），应用需有打开单人私聊和发消息的权限 | Incoming Webhook | Block Kit 链接、正文与图片 |
| [钉钉](https://open.dingtalk.com/document/orgapp/chatbots-send-one-on-one-chat-messages-in-batches) | Client ID（原 AppKey）、Client Secret（原 AppSecret）、可选 RobotCode，以及企业成员 UserID | 群 Webhook 与可选加签密钥 | Markdown 链接、正文与图片外链 |
| [飞书](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/message/create)、[Lark](https://open.larksuite.com/document/server-docs/im-v1/message/create) | 各自区域的 AppID 和 AppSecret，成员 ID 默认使用 `open_id`，也支持 `user_id:…` 和 `union_id:…` | 群机器人 Webhook 与可选加签密钥。Webhook 上传图片另需应用上传凭据 | 富文本链接、正文与上传图片 |
| [企业微信](https://open.work.weixin.qq.com/help2/pc/cat?doc_id=21657) | 智能机器人长连接使用 Bot ID、Secret 与成员 UserID，可通过私聊配对发现 | 群 Webhook | 智能机器人发送 Markdown 链接、正文与封面链接 |
| [个人微信](https://github.com/Tencent/openclaw-weixin/blob/main/docs/protocol.md) | 每个私聊扫码授权后发送「配对」。白名单可从已绑定私聊中多选 | — | 纯文本链接与 iLink 图片上传 |
| [QQ](https://github.com/tencent-connect/bot-docs/blob/main/docs/develop/api-v2/server-inter/message/send-receive/send.md) | AppID、AppSecret 与 `c2c:用户 OpenID`、`group:群 OpenID` 或 `channel:子频道 ID` | — | 文本链接，按目标能力生成图片消息 |

Telegram 私聊可直接使用数字用户 ID，每行一个。每位收件人首次接收前，需打开用于推送的机器人并点「开始」或发一条消息。详见 [Telegram 会话 ID](https://core.telegram.org/api/bots/ids#user-ids)和[机器人私聊规则](https://core.telegram.org/bots#how-are-bots-different-from-humans)。

飞书和 Lark 私聊配对需要开通「获取用户发给机器人的单聊消息」（`im:message.p2p_msg:readonly`），订阅「接收消息」（`im.message.receive_v1`），并发布应用。发送收件校验码需要「以应用的身份发消息」（`im:message:send_as_bot`）。如果界面显示长连接已建立但一直收不到私聊，请在开放平台核对权限及事件日志，并暂停使用同一 AppID 的其他长连接客户端。[飞书长连接说明](https://open.feishu.cn/document/ukTMukTMukTM/uYDNxYjL2QTM24iN0EjN/event-subscription-configure-)

个人微信在手机确认之后，需要扫码账号在「微信 ClawBot」私聊里发送「配对」。桌面端发现会话后可确认保存，从渠道列表发送测试。绑定过期或上游报错时停止自动轮询，可重新生成二维码。

群或频道 Webhook 固定投递给绑定会话。需要个人定向收件时，添加相应的应用机器人连接。规则和 AI 工具选择已配置渠道，每个渠道会投递给其中已验证的收件目标。发送测试也只使用保存的目标。

企业微信已保存的自建应用继续按原配置运行。切换到智能机器人时新建渠道，并在使用旧渠道的规则里改选新渠道。编辑旧应用时填写通讯录中的成员「账号」，并确认成员在应用可见范围内。旧应用出现 `60020` 时，将当前折栈请求所用的公网出口 IP 加入该应用的「企业可信 IP」。出现 `81013` 时核对成员 UserID 和可见范围。[企业微信自建应用](https://hiflow.tencent.com/docs/applications/wwx-app/)

旧配置中的单一目标和测试状态会迁移至白名单，已排队消息保留原收件目标和投递记录。移除目标或更换连接凭据后，旧配置中尚未开始的投递会取消。重新添加目标需要重新测试，旧投递不会恢复。Webhook、令牌、加签和上传凭据使用系统密钥库。图片根据远程图片偏好和平台能力发送，图片抓取或上传失败时继续处理文字。

平台接口接受请求与接收端实际收到消息是不同状态。九渠道真实送达、微信与 QQ 的隔时主动推送，以及各端实际图片样式待使用授权目标验证。

## 手动发送文章

在阅读页「更多」或列表、瀑布流各文章的「发送到」打开单篇弹窗。列表右上角的发送按钮支持批量发送，按当前筛选和排序取前 10 篇，本次发送条数也可在弹窗调整。默认勾选标题、链接和摘要，可独立勾选全文。摘要默认保留 50 字，截断时追加省略号。「设置 → 通知 → 文章发送」可修改默认发送条数和默认摘要字数。

发送弹窗的渠道多选框支持搜索当前已启用且通过测试的渠道，已选渠道显示为可移除标签。「全部渠道」勾选时选中全部渠道，取消时清空。逐项选满时自动勾选，移除任意渠道后自动取消勾选。消息按已验证的收件目标投递。

批量发送默认将多篇合并为一条消息，也可以在弹窗打开「逐篇发送」。单条消息默认最多 1,500 字，本次可设为 100–2,000 字。超过字数或渠道自身的限制时自动分段，优先在文章之间断开。单篇长文也会分段。

同时选中标题和链接时，支持富文本的渠道使用可点击标题。纯文本渠道在消息中附原文网址。仅发送链接时显示原文网址。全文优先采用本地提取缓存或订阅正文。只有摘要时尝试提取原文，失败则提示。排队成功和实际送达分别由弹窗确认及投递历史反映。

## 规则、补齐与状态

- 规则可选多个渠道和分类。父分类覆盖其子分类。首次订阅历史、304 和重复更新不产生新文章事件。对最近两天发现的未匹配文章，分类首次匹配时可补入。正文使用已有纯文本，按平台限制分片。
- 逐条规则正常情况下立即排队；退出期间首次发现的文章按恢复批次组织。每日规则按所选时区、收录区间与发送时间组织。跨午夜支持左闭右开区间。夏令时缺失时点顺延，重复时点仅运行一次。恢复文章带有离线区间内可靠的发布时间时，可进入对应的错过窗口。
- SQLite 持久化文章事件、游标、每日条目、运行及逐目标消息片段。为该规则已经投递或已经排队的条目保留稳定标识。网络发送在事务外。一个目的地失败时，其他目的地继续处理。429 有限重试。进入发送阶段后未收到结果的片段标记「结果未知」，重发需要明确操作。投递历史展示最近 100 条，任务运行记录每项展示最近 50 次。

早期开发库可能已登记通知迁移，但缺少运行标记、规则候选、每日汇总表和部分字段。第 21 版迁移在备份后补齐缺项，保留任务和历史。无法核对任务版本的旧运行记录按失效处理，避免重复执行。通知后台对同一持续故障只记录首次告警，成功后再发生故障时重新记录。

## AI 定时任务

AI 侧栏在「新对话」下方提供「定时任务」入口。右侧管理页显示任务、下次执行时间和运行记录，可以手动添加、编辑、暂停、恢复或删除。侧栏收起后，AI 工具栏保留入口。返回对话恢复此前的会话和草稿。「定时任务」管理入口与 ChatGPT 的 Scheduled 页面采用同类操作流程。 [ChatGPT 定时任务](https://help.openai.com/en/articles/10291617-scheduled-tasks-in-chatgpt)

在全局 AI 输入框描述一次性、每日、工作日或指定星期任务。支持固定提醒、按分类的文章列表与 AI 摘要，可指定回看 1 到 30 天。确认卡展示时区、下次时间和渠道。对话中的 `/定时任务` 打开管理页；也可要求 AI 查看、修改、暂停、恢复或删除任务。没有外部渠道时，结果保存在应用的运行记录中并使用应用通知。AI 摘要使用「摘要」用途分配的模型。

后台用已确认的配置执行，只连接用户配置的目的地。摘要模型处理文章内容时使用固定数据范围，不执行文章中的指令。任务结果与待发片段一起提交，恢复已保存的结果时直接续发。周期任务错过多个时点时处理一个合并运行，从下一次未来时间继续。摘要模型仍受实际模型、预算和连接限制。所有 AI 写入和外发走工具确认。MCP 通过现有显式工具配置选择可见项。
