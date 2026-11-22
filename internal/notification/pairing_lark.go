package notification

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

// The SDK's wire logs may contain the WebSocket ticket and incoming messages.
type pairingLarkLogger struct{}

func (pairingLarkLogger) Debug(context.Context, ...any) {}
func (pairingLarkLogger) Info(context.Context, ...any)  {}
func (pairingLarkLogger) Warn(context.Context, ...any)  {}
func (pairingLarkLogger) Error(context.Context, ...any) {}

func (t transport) pairLark(ctx context.Context, platform string, secret connectionSecret, ready func(), match func(string)) error {
	domain := "https://open.feishu.cn"
	if platform == "lark" {
		domain = "https://open.larksuite.com"
	}
	started := time.Now().UnixMilli()
	handler := dispatcher.NewEventDispatcher("", "").OnP2MessageReceiveV1(func(_ context.Context, event *larkim.P2MessageReceiveV1) error {
		if event.Event == nil || event.Event.Message == nil || event.Event.Sender == nil {
			return nil
		}
		message, sender := event.Event.Message, event.Event.Sender
		if message.Content == nil || message.CreateTime == nil || message.MessageType == nil || *message.MessageType != "text" ||
			sender.SenderType == nil || *sender.SenderType != "user" || sender.SenderId == nil || sender.SenderId.OpenId == nil {
			return nil
		}
		created, _ := strconv.ParseInt(*message.CreateTime, 10, 64)
		var content struct {
			Text string `json:"text"`
		}
		if created >= started && json.Unmarshal([]byte(*message.Content), &content) == nil && isPairingText(content.Text) {
			match(*sender.SenderId.OpenId)
		}
		return nil
	})
	socket := larkws.NewClient(secret.Address, secret.Signing, larkws.WithDomain(domain), larkws.WithAutoReconnect(false),
		larkws.WithHttpClient(t.client), larkws.WithLogger(pairingLarkLogger{}), larkws.WithEventHandler(handler), larkws.WithOnReady(ready))
	if err := socket.Start(ctx); err != nil {
		return errPairConnection
	}
	return nil
}
