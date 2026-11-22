# 通知收件人配对

## 1. 范围与触发条件

在通知设置中用平台入站文本发现收件目标。专用群 Webhook 的收件位置已由平台选定，直接进入确认配置。微信扫码协议见 [微信通知绑定](wechat-binding.md)。

## 2. 签名

- `StartRecipientPairing(ctx, input ChannelInput, slackAppToken string) (PairingStatus, error)`
- `RecipientPairingStatus(id string) PairingStatus`
- `CancelRecipientPairing(id string)`

## 3. 契约

`PairingStatus` 包含 `id`、`state`、`targets` 和可选 `detail`。状态为 `connecting`、`listening`、`failed` 或 `expired`。配对只在内存运行，同一时间最多一个非微信监听，最长三分钟。前端勾选候选后将收件目标写入确认页的草稿，使用 `SaveChannel` 提交。空收件目标草稿保持待验证。

| 平台 | 事件与目标 |
| --- | --- |
| Telegram | Bot API `getUpdates` 的 `message` 或 `channel_post` → `chat.id` |
| Discord | Gateway 的私聊 `MESSAGE_CREATE` → `author.id` |
| Slack | Socket Mode `message.im` → `user`，与 Bot Token 的工作区一致 |
| 钉钉 | Stream 机器人文本回调 → `senderStaffId` |
| 飞书与 Lark | 长连接 `im.message.receive_v1` → `sender.sender_id.open_id` |
| QQ | `C2C_MESSAGE_CREATE` → `c2c:user_openid`，群 @ → `group:group_openid`，文字子频道 @ → `channel:channel_id` |

## 4. 校验与错误

仅匹配去除首尾空白后的「配对」。QQ @ 事件去除机器人的前置提及，全量群事件必须含机器人的提及。接收目标经过发送侧格式校验，去重并限制为 50 个。已保存渠道使用 revision 校验并可复用凭据。Slack App-Level Token 仅用于当前监听。错误、关闭和超时结束监听。企业微信现有自建应用需要消息接收回调，仍通过手动 UserID 完成设置。

## 5. 案例

- 在三个不同 QQ 会话配对时分别给出三个候选，可任意勾选。
- 收到「你好」或同一个候选的重复事件时，候选列表保持原样。
- 用户跳过并保存空收件目标，渠道可稍后编辑，未验证期间不会加入通知规则的候选。

## 6. 必需测试

断言三种 QQ 事件取对 ID、全量群消息必须提及机器人、Discord 跳过群聊与机器人。模拟 Telegram 初次积压、无关消息、新配对事件、取消网络轮询以及跳过后保存。微信测试覆盖扫码到手动完成、跳过后恢复绑定及回调过期。

## 7. 错误与正确做法

在收到关键词后直接写入白名单会让机器人收到的任意私聊改变通知目的地。将事件解析为候选，由用户勾选并在最后一步保存。
