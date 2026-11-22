package notification

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestWeComBotPairingVerificationAndStandaloneDelivery(t *testing.T) {
	s, _ := newTestNotification(t)
	api := NewAPI(s, nil)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	input := ChannelInput{
		Channel: Channel{Name: "企业微信", Platform: "wecom", Mode: "bot", Enabled: true},
		Address: "bot_fixture", Signing: "fixture-secret=",
	}
	deliveries := make(chan string, 3)
	connections := make(chan struct{}, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.CloseNow()
		var frame wecomBotFrame
		ack := func() error {
			zero := 0
			return wsjson.Write(ctx, conn, wecomBotFrame{Headers: frame.Headers, ErrorCode: &zero})
		}
		var credentials map[string]string
		if err := wsjson.Read(ctx, conn, &frame); err != nil || frame.Command != "aibot_subscribe" ||
			json.Unmarshal(frame.Body, &credentials) != nil || credentials["bot_id"] != input.Address ||
			credentials["secret"] != input.Signing || frame.Headers.RequestID == "" {
			t.Error("invalid WeCom Bot authentication frame")
			return
		}
		if err := ack(); err != nil {
			t.Error(err)
			return
		}
		select {
		case connections <- struct{}{}:
			for _, message := range []string{
				`{"chattype":"group","from":{"userid":"group-member"},"msgtype":"text","text":{"content":"配对"}}`,
				`{"chattype":"single","from":{"userid":"other"},"msgtype":"text","text":{"content":"你好"}}`,
				`{"chattype":"single","from":{"userid":"employee"},"msgtype":"text","text":{"content":" 配对 "}}`,
			} {
				if err := wsjson.Write(ctx, conn, wecomBotFrame{Command: "aibot_msg_callback", Body: json.RawMessage(message)}); err != nil {
					t.Error(err)
					return
				}
			}
			if err := wsjson.Read(ctx, conn, &frame); err == nil {
				t.Error("pairing connection should close before delivery")
			}
		default:
			var body struct {
				ChatID      string `json:"chatid"`
				MessageType string `json:"msgtype"`
				Markdown    struct {
					Content string `json:"content"`
				} `json:"markdown"`
			}
			if err := wsjson.Read(ctx, conn, &frame); err != nil || frame.Command != "aibot_send_msg" ||
				json.Unmarshal(frame.Body, &body) != nil || body.ChatID != "employee" || body.MessageType != "markdown" {
				t.Error("invalid proactive WeCom Bot send")
				return
			}
			deliveries <- body.Markdown.Content
			if body.Markdown.Content == "回执丢失" {
				return
			}
			if err := ack(); err != nil {
				t.Error(err)
			}
		}
	}))
	defer server.Close()
	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := server.Client()
	underlying := client.Transport
	client.Transport = sender(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != wecomBotHost {
			t.Error("unexpected WebSocket target")
			return response(400, "{}"), nil
		}
		// Check the actual outbound spelling before net/http normalizes received headers.
		if len(req.Header["Sec-WebSocket-Key"]) != 1 || !slices.Equal(req.Header["Sec-WebSocket-Version"], []string{"13"}) {
			t.Error("WeCom gateway requires Sec-WebSocket-* handshake headers")
			return response(http.StatusNotFound, "{}"), nil
		}
		req = req.Clone(req.Context())
		req.URL.Host = endpoint.Host
		return underlying.RoundTrip(req)
	})
	s.transport.client = client

	pairing, err := api.StartRecipientPairing(ctx, input, "")
	if err != nil {
		t.Fatal(err)
	}
	defer api.CancelRecipientPairing(pairing.ID)
	for {
		pairing = api.RecipientPairingStatus(pairing.ID)
		if pairing.State == "failed" {
			t.Fatal(pairing.Detail)
		}
		if len(pairing.Targets) > 0 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("no pair target: %+v", pairing)
		case <-time.After(10 * time.Millisecond):
		}
	}
	if pairing.State != "listening" || !pairing.ReceivedMessage || !slices.Equal(pairing.Targets, []string{"employee"}) {
		t.Fatalf("unexpected pair status: %+v", pairing)
	}
	api.CancelRecipientPairing(pairing.ID)
	input.Targets = pairing.Targets
	channel, err := s.SaveChannel(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	recipient := channel.Recipients[0]
	if err := api.SendRecipientVerification(ctx, channel.ID, channel.Revision, recipient.ID, "", false); err != nil {
		t.Fatal(err)
	}
	code := strings.TrimPrefix(<-deliveries, "配对成功，校验码：")
	if !verificationDigits.MatchString(code) {
		t.Fatal("expected a four-digit verification code")
	}
	if err := s.confirmRecipientVerification(ctx, channel.ID, channel.Revision, recipient.ID, code); err != nil {
		t.Fatal(err)
	}
	if err := s.TestChannel(ctx, channel.ID); err != nil {
		t.Fatal(err)
	}
	if sent := <-deliveries; sent != "折栈通知测试" {
		t.Fatalf("unexpected test notification %q", sent)
	}
	message := Message{Title: "回执丢失"}
	result := s.transport.sendWeComBot(ctx, connectionSecret{Address: input.Address, Signing: input.Signing}, recipient.Target, message)
	if result.state != "unknown" {
		t.Fatalf("send request without acknowledgement has result %+v", result)
	}
}

func TestWeComBotHandshakeReportsSafeHTTPStatus(t *testing.T) {
	tr := newTransport()
	tr.client.Transport = sender(func(req *http.Request) (*http.Response, error) {
		return response(http.StatusForbidden, "upstream response with fixture-secret"), nil
	})
	err := tr.pairWeComBot(t.Context(), connectionSecret{Address: "bot_fixture", Signing: "fixture-secret"},
		func() { t.Error("failed handshake became ready") }, func() {}, func(string) {})
	if err == nil || !strings.Contains(err.Error(), "握手失败（HTTP 403）") || strings.Contains(err.Error(), "fixture-secret") {
		t.Fatalf("expected only a safe handshake diagnostic, got %v", err)
	}
}
