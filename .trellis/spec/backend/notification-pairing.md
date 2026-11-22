# 通知收件人配对

## 1. 范围与触发条件

在通知设置中用平台入站文本发现收件目标。专用群 Webhook 的收件位置已由平台选定，直接进入确认配置。微信扫码协议见 [微信通知绑定](wechat-binding.md)。

## 2. 签名

- `StartRecipientPairing(ctx, input ChannelInput, slackAppToken string) (PairingStatus, error)`
- `RecipientPairingStatus(id string) PairingStatus`
- `CancelRecipientPairing(id string)`

## 3. 契约

`PairingStatus` 包含 `id`、`state`、`targets` 和可选 `detail`。QQ 从 READY 返回 `bot_name`，收到受支持的聊天事件后设置 `received_message`。飞书和 Lark 收到本次监听期间由用户发送的 `p2p` 消息后设置同一个 `received_message`。心跳、机器人进群事件和其他通用帧不设置聊天收件标志。状态为 `connecting`、`listening`、`failed` 或 `expired`。配对只在内存运行，同一时间最多一个非微信监听，最长三分钟。前端勾选候选后将收件目标写入确认页的草稿，使用 `SaveChannel` 提交。空收件目标草稿保持待验证。

| 平台 | 事件与目标 |
| --- | --- |
| Telegram | Bot API `getUpdates` 的 `message` 或 `channel_post` → `chat.id` |
| Discord | Gateway 的私聊 `MESSAGE_CREATE` → `author.id` |
| Slack | Socket Mode `message.im` → `user`，与 Bot Token 的工作区一致 |
| 钉钉 | Stream 机器人文本回调 → `senderStaffId` |
| 飞书与 Lark | 长连接 `im.message.receive_v1`，`message.chat_type=p2p` 且 `sender.sender_type=user` → `sender.sender_id.open_id` |
| QQ | `C2C_MESSAGE_CREATE` → `c2c:user_openid`，群 @ → `group:group_openid`，文字子频道 @ → `channel:channel_id` |

钉钉应用机器人的表单显示 Client ID 和 Client Secret，沿用 `ChannelInput.Address` 和 `ChannelInput.Signing`。Stream 握手使用 `clientId` 和 `clientSecret`，主动发送的 OAuth 接口仍使用 `appKey` 和 `appSecret`。Stream 接收配置与主动发送所需的「企业内机器人发送消息权限」分别在操作步骤中说明。

QQ 渠道复用 `Channel.mode`。空字符串为正式环境，`sandbox` 为沙箱。两种环境共用 `bots.qq.com/app/getAppAccessToken`。正式和沙箱使用腾讯 SDK 的 `api.sgroup.qq.com` 系列地址。网关发现、投递和测试分别请求对应环境地址。渠道环境与验证过的收件目标一起保存，已有渠道更换环境时重新添加并配对。QQ 在 `READY` 后发送心跳并收到 `op=11` 后进入 `listening`，收到消息后才展示收件候选。连续未确认心跳或断线结束本次配对，由用户发起下一次连接。

## 4. 校验与错误

仅匹配去除首尾空白后的「配对」。QQ @ 事件去除机器人的前置提及，全量群事件必须含机器人的提及。接收目标经过发送侧格式校验，去重并限制为 50 个。已保存渠道使用 revision 校验并可复用凭据。Slack App-Level Token 仅用于当前监听。错误、关闭和超时结束监听。企业微信现有自建应用需要消息接收回调，仍通过手动 UserID 完成设置。

飞书与 Lark 以 SDK 握手成功进入 `listening`。只有消息创建时间在本次监听窗口内的用户私聊才能报告收件。只有私聊文本「配对」并携带可用的 `open_id` 才产生候选。单聊权限 `im:message.p2p_msg:readonly` 和接收消息事件需随应用发布。若同一个 AppID 同时有多个长连接，平台可能把事件送给其他客户端。收件校验码需要机器人发送权限。

## 5. 案例

- 在三个不同 QQ 会话配对时分别给出三个候选，可任意勾选。
- 收到「你好」或同一个候选的重复事件时，候选列表保持原样。
- 用户跳过并保存空收件目标，渠道可稍后编辑，未验证期间不会加入通知规则的候选。

## 6. 必需测试

断言三种 QQ 事件取对 ID、全量群消息必须提及机器人、Discord 跳过群聊与机器人。模拟 Telegram 初次积压、无关消息、新配对事件、取消网络轮询以及跳过后保存。微信测试覆盖扫码到手动完成、跳过后恢复绑定及回调过期。QQ 回环 WebSocket 验证正式与沙箱地址、READY 昵称、首个心跳、ACK、收到普通私聊及提取配对候选、无 ACK 时终止。模拟测试投递验证已保存的沙箱环境生效。

飞书与 Lark 回环使用官方 Go SDK 接收事件帧，断言群聊、机器人和旧消息不会产生候选，普通私聊更新收件状态，私聊「配对」提取当前应用的 Open ID。界面测试区分长连接就绪、收到普通私聊和确认候选。

## 7. 错误与正确做法

在收到关键词后直接写入白名单会让机器人收到的任意私聊改变通知目的地。将事件解析为候选，由用户勾选并在最后一步保存。用心跳 ACK 仅确认网关连接，聊天送达由实际事件确认，并在界面区分两者。


## 收件校验码

### 1. 范围与触发条件

保存含未验证收件目标的渠道后自动进入第 4 步，也可从渠道列表继续。群 Webhook 已绑定收件位置，直接验证收件。

### 2. 签名

- `SendRecipientVerification(ctx, channelID, revision, recipientID, pairingID, resend)`
- `ConfirmRecipientVerification(ctx, channelID, revision, recipientID, code)`

### 3. 契约

每个目标生成四位随机数字，校验码仅保留在进程内并通过目标渠道送达。回复所需的 QQ 消息 ID 不进入前端状态或持久化，限定同一个配对会话、AppID、环境和收件目标。发送校验码使用现有投递适配器。QQ 有配对消息时使用被动回复，其他场景沿用渠道本身的投递方式。确认后复用事务更新该目标和所属渠道的验证状态。

### 4. 校验与错误

每码五分钟有效，最多尝试五次。重发间隔至少 30 秒。请求必须匹配处于启用状态的收件目标、渠道和当前 revision。并发发送有单次在途限制。结果未知时允许输入已到达的校验码，平台明确拒绝时必须重发。验证成功后即消费该码。

### 5. 案例

一次配对两个目标后，先收到甲的码并验证，甲可进入推送队列，乙继续待验证。稍后可以重新打开验证页，复用本次运行内未过期的校验码。

### 6. 必需测试

断言验证码送达字段、目标隔离、重复自动发送、过期、修订版变化、验证状态更新和已消费码失效。QQ 用模拟响应断言校验码引用收到的消息。前端断言保存后进入第 4 步并自动发送，再通过四位输入完成。

### 7. 错误与正确做法

用平台接受发送请求直接标记新目标会忽略用户是否收到。自动发送后等待用户回填该目标的校验码，成功后再更新。
