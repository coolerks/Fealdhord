package notification

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const (
	wecomBotEndpoint = "wss://openws.work.weixin.qq.com"
	wecomBotHost     = "openws.work.weixin.qq.com"
	wecomBotTimeout  = 12 * time.Second
)

var (
	errWeComBotCredentials = errors.New("请填写智能机器人的 Bot ID 和 Secret")
	errWeComBotConnection  = errors.New("无法连接企业微信智能机器人，请检查网络后重试")
	errWeComBotReplaced    = errors.New("企业微信智能机器人的连接被替换，请关闭使用同一 Bot ID 的其他客户端后重试")
)

func isWeComBot(platform, mode string) bool { return platform == "wecom" && mode == "bot" }

func validWeComBotCredentials(secret connectionSecret) bool {
	return tokenPattern.MatchString(secret.Address) && secret.Signing != "" &&
		strings.IndexFunc(secret.Signing, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) == -1
}

type wecomBotFrame struct {
	Command string `json:"cmd,omitempty"`
	Headers struct {
		RequestID string `json:"req_id"`
	} `json:"headers"`
	Body      json.RawMessage `json:"body,omitempty"`
	ErrorCode *int            `json:"errcode,omitempty"`
}

func (f wecomBotFrame) replaced() bool {
	if f.Command != "aibot_event_callback" {
		return false
	}
	var body struct {
		Event struct {
			Type string `json:"eventtype"`
		} `json:"event"`
	}
	return json.Unmarshal(f.Body, &body) == nil && body.Event.Type == "disconnected_event"
}

func wecomBotCommand(ctx context.Context, conn *websocket.Conn, command, requestID string, body map[string]any) error {
	frame := wecomBotFrame{Command: command}
	frame.Headers.RequestID = requestID
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		frame.Body = data
	}
	return wsjson.Write(ctx, conn, frame)
}

func wecomBotAck(ctx context.Context, conn *websocket.Conn, requestID string) (int, error) {
	for {
		var frame wecomBotFrame
		if err := wsjson.Read(ctx, conn, &frame); err != nil {
			return 0, errWeComBotConnection
		}
		if frame.replaced() {
			return 0, errWeComBotReplaced
		}
		if frame.Headers.RequestID == requestID && frame.ErrorCode != nil {
			return *frame.ErrorCode, nil
		}
	}
}

// The platform allows a single live connection per robot. Serialize pairing and delivery in this process.
func (t transport) lockWeComBot(ctx context.Context, botID string) (func(), error) {
	// The service shares the transport across pairing, verification, and the delivery worker.
	value, _ := t.wecomBotSessions.LoadOrStore(botID, make(chan struct{}, 1))
	gate := value.(chan struct{})
	select {
	case gate <- struct{}{}:
		return func() { <-gate }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (t transport) connectWeComBot(ctx context.Context, secret connectionSecret) (*websocket.Conn, int, error) {
	conn, err := t.pairSocket(ctx, wecomBotEndpoint, wecomBotHost)
	if err != nil {
		return nil, 0, errWeComBotConnection
	}
	reqID := "aibot_subscribe_" + rand.Text()
	if err = wecomBotCommand(ctx, conn, "aibot_subscribe", reqID, map[string]any{
		"bot_id": secret.Address, "secret": secret.Signing,
	}); err != nil {
		_ = conn.CloseNow()
		return nil, 0, errWeComBotConnection
	}
	code, err := wecomBotAck(ctx, conn, reqID)
	if err != nil || code != 0 {
		_ = conn.CloseNow()
		return nil, code, err
	}
	return conn, 0, nil
}

func (t transport) pairWeComBot(ctx context.Context, secret connectionSecret, ready, received func(), match func(string)) error {
	connect, cancelConnect := context.WithTimeout(ctx, wecomBotTimeout)
	defer cancelConnect()
	unlock, err := t.lockWeComBot(connect, secret.Address)
	if err != nil {
		return errors.New("企业微信智能机器人正在发送消息，请稍后重新配对")
	}
	defer unlock()
	conn, code, err := t.connectWeComBot(connect, secret)
	cancelConnect()
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("企业微信拒绝智能机器人连接（%d），请检查 Bot ID 和 Secret", code)
	}
	defer conn.CloseNow()
	listen, stop := context.WithCancel(ctx)
	defer stop()
	ready()
	var missed atomic.Int32
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-listen.Done():
				return
			case <-ticker.C:
				if missed.Add(1) > 2 || wecomBotCommand(listen, conn, "ping", "ping_"+rand.Text(), nil) != nil {
					stop()
					return
				}
			}
		}
	}()
	for {
		var frame wecomBotFrame
		if err := wsjson.Read(listen, conn, &frame); err != nil {
			return errWeComBotConnection
		}
		if frame.replaced() {
			return errWeComBotReplaced
		}
		if strings.HasPrefix(frame.Headers.RequestID, "ping_") && frame.ErrorCode != nil && *frame.ErrorCode == 0 {
			missed.Store(0)
		}
		if frame.Command != "aibot_msg_callback" {
			continue
		}
		var body struct {
			ChatType    string `json:"chattype"`
			MessageType string `json:"msgtype"`
			From        struct {
				UserID string `json:"userid"`
			} `json:"from"`
			Text struct {
				Content string `json:"content"`
			} `json:"text"`
		}
		if json.Unmarshal(frame.Body, &body) != nil || body.ChatType != "single" {
			continue
		}
		received()
		if body.MessageType == "text" && isPairingText(body.Text.Content) {
			match(body.From.UserID)
		}
	}
}

func (t transport) sendWeComBot(ctx context.Context, secret connectionSecret, target string, message Message) deliveryResult {
	if t.allowImages != nil && !t.allowImages(ctx) {
		message.Image = ""
	}
	message = formatMessage(message)
	content := markdownMessage(message)
	if cover := markdownLink("查看封面", message.Image); message.Image != "" && len(content)+2+len(cover) <= 2048 {
		content += "\n\n" + cover
	}

	ctx, cancel := context.WithTimeout(ctx, wecomBotTimeout)
	defer cancel()
	unlock, err := t.lockWeComBot(ctx, secret.Address)
	if err != nil {
		return deliveryResult{state: "pending", detail: "wecom_bot_busy", wait: 30 * time.Second,
			userHint: "企业微信智能机器人正在配对，请结束配对后重发"}
	}
	defer unlock()
	conn, code, err := t.connectWeComBot(ctx, secret)
	if err != nil {
		return deliveryResult{state: "pending", detail: "wecom_bot_connection", wait: 30 * time.Second, userHint: err.Error()}
	}
	if code != 0 {
		return deliveryResult{state: "failed", detail: fmt.Sprintf("wecom_bot_auth_%d", code),
			userHint: fmt.Sprintf("企业微信拒绝智能机器人连接（%d），请检查 Bot ID 和 Secret", code)}
	}
	defer conn.CloseNow()

	reqID := "aibot_send_msg_" + rand.Text()
	err = wecomBotCommand(ctx, conn, "aibot_send_msg", reqID, map[string]any{
		"chatid": target, "msgtype": "markdown", "markdown": map[string]string{"content": content},
	})
	if err != nil {
		return deliveryResult{state: "unknown", detail: "wecom_bot_send_unconfirmed"}
	}
	code, err = wecomBotAck(ctx, conn, reqID)
	if err != nil {
		return deliveryResult{state: "unknown", detail: "wecom_bot_send_unconfirmed"}
	}
	if code != 0 {
		return deliveryResult{state: "failed", detail: fmt.Sprintf("wecom_bot_send_%d", code),
			userHint: fmt.Sprintf("企业微信智能机器人未能发送消息（%d），请确认收件人可使用该机器人并已打开过私聊", code)}
	}
	return deliveryResult{state: "accepted"}
}
