# 微信通知绑定

## 1. 范围

修改通知设置的微信扫码、识别私聊中的「配对」文本或绑定状态时。

## 2. 签名

`StartWeChatBinding(ctx)` 返回二维码与本地绑定 ID。`PollWeChatBinding(ctx, id, pin, listen)` 返回状态和已识别的私聊目标。

`ResumeWeChatBinding(ctx, input)` 使用已保存凭据开启配对，`CompleteWeChatBinding(ctx, id, input)` 在用户确认后返回已保存渠道。`CancelWeChatBinding(id)` 清理本地绑定。

## 3. 协议

创建二维码使用 `POST /ilink/bot/get_bot_qrcode?bot_type=3`。创建和状态查询均携带 `iLink-App-Id` 和编码后的 `iLink-App-ClientVersion`。创建请求额外携带 `AuthorizationType` 与 `X-WECHAT-UIN`。状态查询只向获准的微信 HTTPS 域名发送二维码或验证码。服务端以 `scaned` 接受验证码后，后续轮询省略验证码。见 [腾讯微信协议](https://github.com/Tencent/openclaw-weixin/blob/main/docs/protocol_zh_CN.md)。

## 4. 校验与错误

`confirmed` 后由界面进入第二步，使用 SDK 读取扫码用户的私聊消息并匹配「配对」。有非空 `context_token` 时可保存收件目标。扫码后跳过消息配对时，可以保存没有收件目标的待验证渠道。机器人令牌作为不透明字符串处理，拒绝空值、空白和控制字符。确认配置后令牌写入系统凭据库。私聊查询的 `ret` 或 `errcode` 为 `-14` 表示过期，其他非零值作为失败结束本次自动轮询。绑定最长八分钟。

## 5. 案例

扫码并确认后先显示等待私聊消息。无新消息是正常等待。只接受扫码用户的私聊「配对」。查询失败时允许用户重新生成二维码。

## 6. 回归测试

模拟二维码请求、重定向、验证码、确认和私聊入站。断言机器人令牌完整传入 SDK，只有扫码用户的会话令牌可用于发送，两个令牌都不出现在前端数据中。验证过期与前端错误不会再次自动轮询。

## 7. 错误与正确做法

将机器人令牌套入其他平台的字符表会拒绝微信下发的有效凭据。按 HTTP Bearer 头的安全边界处理，原样交给 SDK。
