package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

func TestDingTalkPairingUsesOfficialStreamEndpoint(t *testing.T) {
	const topic = "/v1.0/im/bot/messages/get"
	const ticket = "fixture-ticket"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/connect" || r.URL.Query().Get("ticket") != ticket {
			t.Error("missing Stream path or ticket")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.CloseNow()
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		send := func(kind, topic, id, data string) bool {
			if err := wsjson.Write(ctx, conn, map[string]any{
				"type": kind, "headers": map[string]string{"topic": topic, "messageId": id}, "data": data,
			}); err != nil {
				t.Error(err)
				return false
			}
			var ack struct {
				Code    int               `json:"code"`
				Headers map[string]string `json:"headers"`
				Data    string            `json:"data"`
			}
			if err := wsjson.Read(ctx, conn, &ack); err != nil || ack.Code != 200 ||
				ack.Headers["messageId"] != id || (kind == "SYSTEM" && ack.Data != data) {
				t.Errorf("unexpected %s acknowledgment: %+v, %v", kind, ack, err)
				return false
			}
			return true
		}
		if send("SYSTEM", "ping", "1", `{"opaque":"fixture"}`) {
			send("CALLBACK", topic, "2", fmt.Sprintf(`{"senderStaffId":"employee01","msgtype":"text","createAt":%d,"text":{"content":"配对"}}`, time.Now().Add(time.Second).UnixMilli()))
		}
	}))
	defer server.Close()
	client := server.Client()
	loopback, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	underlying := client.Transport
	client.Transport = sender(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "api.dingtalk.com":
			var body struct {
				ClientID      string              `json:"clientId"`
				ClientSecret  string              `json:"clientSecret"`
				Subscriptions []map[string]string `json:"subscriptions"`
			}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil ||
				req.URL.Path != "/v1.0/gateway/connections/open" || req.Header.Get("Accept") != "application/json" ||
				body.ClientID != "fixture-id" || body.ClientSecret != "fixture-secret" ||
				len(body.Subscriptions) != 1 || body.Subscriptions[0]["type"] != "CALLBACK" || body.Subscriptions[0]["topic"] != topic {
				t.Error("incorrect Stream registration request")
				return response(400, "{}"), nil
			}
			return response(200, `{"endpoint":"wss://wss-open-connection.dingtalk.com:443/connect","ticket":"fixture-ticket"}`), nil
		case "wss-open-connection.dingtalk.com:443":
			req = req.Clone(req.Context())
			req.URL.Host = loopback.Host
			return underlying.RoundTrip(req)
		default:
			t.Error("unexpected Stream host")
			return response(500, "{}"), nil
		}
	})

	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
	defer cancel()
	var ready bool
	var target string
	err = (transport{client: client}).pairDingTalk(ctx, connectionSecret{Address: "fixture-id", Signing: "fixture-secret"},
		func() { ready = true }, func(value string) { target = value })
	if !ready || target != "employee01" {
		t.Fatalf("official endpoint did not deliver the pairing message: ready=%t target=%q err=%v", ready, target, err)
	}
}

func TestPairSocketRejectsUntrustedGateway(t *testing.T) {
	client := &http.Client{Transport: sender(func(*http.Request) (*http.Response, error) {
		t.Error("invalid gateway must be rejected before dialing")
		return response(500, "{}"), nil
	})}
	for _, endpoint := range []string{
		"wss://wss-open-connection.dingtalk.com:8443/connect",
		"wss://wss-open-connection.dingtalk.com:/connect",
		"wss://wss-open-connection.dingtalk.com.invalid:443/connect",
		"wss://user@wss-open-connection.dingtalk.com:443/connect",
		"ws://wss-open-connection.dingtalk.com:443/connect",
	} {
		if _, err := (transport{client: client}).pairSocket(t.Context(), endpoint, "dingtalk.com"); !errors.Is(err, errPairConnection) {
			t.Errorf("invalid gateway %q: %v", endpoint, err)
		}
	}
}

func TestDingTalkPairingReportsSafeRegistrationFailure(t *testing.T) {
	client := &http.Client{Transport: sender(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "api.dingtalk.com" {
			t.Error("unexpected connection request")
		}
		return response(403, `{"message":"fixture-secret-and-ticket"}`), nil
	})}
	err := (transport{client: client}).pairDingTalk(t.Context(), connectionSecret{Address: "fixture-id", Signing: "fixture-secret"},
		func() { t.Error("rejected registration must not become ready") }, func(string) { t.Error("unexpected target") })
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") || strings.Contains(err.Error(), "fixture") {
		t.Fatalf("expected only safe registration diagnostics, got %v", err)
	}
}

func TestLarkPairingReceivesPrivateMessageEvents(t *testing.T) {
	for _, platform := range []string{"feishu", "lark"} {
		t.Run(platform, func(t *testing.T) {
			service, _ := newTestNotification(t)
			api := NewAPI(service, nil)
			sendPairing := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := websocket.Accept(w, r, nil)
				if err != nil {
					t.Error(err)
					return
				}
				defer conn.CloseNow()
				ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
				defer cancel()
				write := func(id int, chat, sender, openID, text string, created int64) {
					payload := fmt.Sprintf(`{"schema":"2.0","header":{"event_id":"event-%d","event_type":"im.message.receive_v1"},"event":{"sender":{"sender_id":{"open_id":%q},"sender_type":%q},"message":{"chat_type":%q,"message_type":"text","create_time":"%d","content":%q}}}`,
						id, openID, sender, chat, created, fmt.Sprintf(`{"text":%q}`, text))
					frame := larkws.Frame{Method: int32(larkws.FrameTypeData), Payload: []byte(payload),
						Headers: []larkws.Header{{Key: larkws.HeaderType, Value: "event"}, {Key: larkws.HeaderSum, Value: "1"}}}
					data, err := frame.Marshal()
					if err != nil {
						t.Error(err)
						return
					}
					if err := conn.Write(ctx, websocket.MessageBinary, data); err != nil {
						t.Error(err)
					}
				}
				now := time.Now().UnixMilli()
				write(1, "group", "user", "ou_group", "配对", now)
				write(2, "p2p", "app", "ou_robot", "配对", now)
				write(3, "p2p", "user", "ou_old", "配对", 1)
				write(4, "p2p", "user", "ou_member", "你好", now)
				select {
				case <-ctx.Done():
					return
				case <-sendPairing:
					write(5, "p2p", "user", "ou_member", " 配对 ", time.Now().UnixMilli())
				}
				for ctx.Err() == nil {
					if _, _, err := conn.Read(ctx); err != nil {
						break
					}
				}
			}))
			defer server.Close()
			domain := "open.feishu.cn"
			if platform == "lark" {
				domain = "open.larksuite.com"
			}
			service.transport.client.Transport = sender(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != domain || r.URL.Path != "/callback/ws/endpoint" {
					t.Errorf("unexpected bootstrap URL %s", r.URL)
					return response(500, "{}"), nil
				}
				return response(200, fmt.Sprintf(`{"code":0,"data":{"URL":%q}}`, "ws"+strings.TrimPrefix(server.URL, "http")+"/?device_id=fixture&service_id=1")), nil
			})
			pair, err := api.StartRecipientPairing(t.Context(), ChannelInput{
				Channel: Channel{Platform: platform, Mode: "app"}, Address: "cli_fixture", Signing: "fixture",
			}, "")
			if err != nil {
				t.Fatal(err)
			}
			defer api.CancelRecipientPairing(pair.ID)
			waitForStatus := func(match func(PairingStatus) bool) PairingStatus {
				t.Helper()
				deadline := time.Now().Add(2 * time.Second)
				for time.Now().Before(deadline) {
					status := api.RecipientPairingStatus(pair.ID)
					if match(status) || status.State == "failed" {
						return status
					}
					time.Sleep(5 * time.Millisecond)
				}
				t.Fatalf("timed out waiting for pairing: %+v", api.RecipientPairingStatus(pair.ID))
				return PairingStatus{}
			}
			if got := waitForStatus(func(status PairingStatus) bool { return status.ReceivedMessage }); got.State != "listening" || len(got.Targets) != 0 {
				t.Fatalf("ordinary DM should be observed without adding a recipient: %+v", got)
			}
			close(sendPairing)
			if got := waitForStatus(func(status PairingStatus) bool { return len(status.Targets) > 0 }); !slices.Equal(got.Targets, []string{"ou_member"}) {
				t.Fatalf("private pairing event should provide the sender's open ID: %+v", got)
			}
		})
	}
}

func TestGatewayPairingTargets(t *testing.T) {
	tests := []struct {
		name, platform, event, data, want string
	}{
		{"QQ private", "qq", "C2C_MESSAGE_CREATE", `{"content":" 配对 ","author":{"user_openid":"direct_012"}}`, "c2c:direct_012"},
		{"QQ group", "qq", "GROUP_AT_MESSAGE_CREATE", `{"content":"<@!123> 配对","group_openid":"group_012","author":{"member_openid":"different"}}`, "group:group_012"},
		{"QQ full group with mention", "qq", "GROUP_MESSAGE_CREATE", `{"content":"配对","group_openid":"group_012","mentions":[{"id":"123"}]}`, "group:group_012"},
		{"QQ full group without mention", "qq", "GROUP_MESSAGE_CREATE", `{"content":"配对","group_openid":"group_012"}`, ""},
		{"QQ guild", "qq", "AT_MESSAGE_CREATE", `{"content":"<@123>配对","channel_id":"42","guild_id":"12"}`, "channel:42"},
		{"QQ unrelated mention", "qq", "AT_MESSAGE_CREATE", `{"content":"<@999> 配对","channel_id":"42"}`, ""},
		{"QQ member without group", "qq", "GROUP_AT_MESSAGE_CREATE", `{"content":"配对","author":{"member_openid":"different"}}`, ""},
		{"QQ ordinary text", "qq", "C2C_MESSAGE_CREATE", `{"content":"怎么配对？","author":{"user_openid":"direct_012"}}`, ""},
		{"Discord DM", "discord", "MESSAGE_CREATE", `{"content":"配对","author":{"id":"42"}}`, "42"},
		{"Discord guild", "discord", "MESSAGE_CREATE", `{"content":"配对","guild_id":"12","author":{"id":"42"}}`, ""},
		{"Discord bot", "discord", "MESSAGE_CREATE", `{"content":"配对","author":{"id":"42","bot":true}}`, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := gatewayPairingTarget(test.platform, "123", pairingGatewayFrame{Event: test.event, Data: json.RawMessage(test.data)})
			if got != test.want {
				t.Fatalf("target = %q, want %q", got, test.want)
			}
		})
	}
}

func TestTelegramPairingIsTemporaryAndRequiresConfirmation(t *testing.T) {
	service, _ := newTestNotification(t)
	api := NewAPI(service, nil)
	requests := 0
	cancelled := make(chan struct{})
	service.transport.client.Transport = sender(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.telegram.org" || !strings.HasSuffix(r.URL.Path, "/getUpdates") {
			t.Errorf("unexpected request %s", r.URL)
			return response(500, "{}"), nil
		}
		var request struct {
			Offset int64 `json:"offset"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		requests++
		switch requests {
		case 1:
			return response(200, `{"ok":true,"result":[{"update_id":1,"message":{"text":"配对","date":4102444800,"chat":{"id":100}}}]}`), nil
		case 2:
			if request.Offset != 2 {
				t.Errorf("offset = %d, want 2", request.Offset)
			}
			return response(200, `{"ok":true,"result":[{"update_id":2,"message":{"text":"配对","date":4102444800,"chat":{"id":42}}},{"update_id":3,"message":{"text":"配对","date":10,"chat":{"id":50}}},{"update_id":4,"message":{"text":"如何配对","date":4102444800,"chat":{"id":51}}}]}`), nil
		default:
			<-r.Context().Done()
			close(cancelled)
			return nil, r.Context().Err()
		}
	})
	input := ChannelInput{Channel: Channel{Platform: "telegram", Name: "Telegram", Enabled: true}, Address: "42:token"}
	pair, err := api.StartRecipientPairing(t.Context(), input, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { api.CancelRecipientPairing(pair.ID) })
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && len(api.RecipientPairingStatus(pair.ID).Targets) == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if got := api.RecipientPairingStatus(pair.ID); got.State != "listening" || !slices.Equal(got.Targets, []string{"42"}) {
		t.Fatalf("unexpected status %+v", got)
	}
	channels, err := api.ListChannels(t.Context())
	if err != nil || len(channels) != 0 {
		t.Fatalf("pairing must not persist a channel: %v, %v", channels, err)
	}
	api.CancelRecipientPairing(pair.ID)
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("cancelling did not end the active long poll")
	}
	if got := api.RecipientPairingStatus(pair.ID); got.State != "expired" || len(got.Targets) != 0 {
		t.Fatalf("cancelled status %+v", got)
	}
	input.Targets = []string{}
	channel, err := api.SaveChannel(t.Context(), input)
	if err != nil || channel.Verified || len(channel.Recipients) != 0 {
		t.Fatalf("skipped channel %+v, %v", channel, err)
	}
}

func TestQQPairingGateway(t *testing.T) {
	for _, test := range []struct {
		name, mode, host string
		ack              bool
	}{
		{"production receives a private message", "", "api.sgroup.qq.com", true},
		{"sandbox receives a private message", qqSandbox, "sandbox.api.sgroup.qq.com", true},
		{"fails without a heartbeat acknowledgment", qqSandbox, "sandbox.api.sgroup.qq.com", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			ack := test.ack
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := websocket.Accept(w, r, nil)
				if err != nil {
					t.Error(err)
					return
				}
				defer conn.CloseNow()
				ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
				defer cancel()
				interval := 30000
				if !ack {
					interval = 1000
				}
				write := func(frame string) bool {
					if err := conn.Write(ctx, websocket.MessageText, []byte(frame)); err != nil {
						t.Error(err)
						return false
					}
					return true
				}
				if !write(fmt.Sprintf(`{"op":10,"d":{"heartbeat_interval":%d}}`, interval)) {
					return
				}
				var identify, heartbeat pairingGatewayFrame
				if err := wsjson.Read(ctx, conn, &identify); err != nil || identify.Op != 2 ||
					!strings.Contains(string(identify.Data), `"token":"QQBot sample"`) {
					t.Errorf("bad identify: %+v, %v", identify, err)
					return
				}
				if !write(`{"op":0,"t":"READY","s":1,"d":{"user":{"id":"123","username":"文章通知机器人"}}}`) {
					return
				}
				if err := wsjson.Read(ctx, conn, &heartbeat); err != nil || heartbeat.Op != 1 || string(heartbeat.Data) != "1" {
					t.Errorf("bad heartbeat: %+v, %v", heartbeat, err)
					return
				}
				if ack && write(`{"op":11}`) &&
					write(`{"op":0,"t":"C2C_MESSAGE_CREATE","s":2,"d":{"content":"你好","author":{"user_openid":"other_user"}}}`) {
					write(`{"op":0,"t":"C2C_MESSAGE_CREATE","s":3,"d":{"content":"配对","author":{"user_openid":"paired_user"}}}`)
				}
				_, _, _ = conn.Read(ctx)
			}))
			defer server.Close()

			client := server.Client()
			loopback, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			underlying := client.Transport
			client.Transport = sender(func(req *http.Request) (*http.Response, error) {
				switch req.URL.String() {
				case "https://bots.qq.com/app/getAppAccessToken":
					return response(200, `{"access_token":"sample"}`), nil
				case "https://" + test.host + "/gateway":
					return response(200, `{"url":"wss://`+test.host+`/websocket"}`), nil
				case "https://" + test.host + "/websocket":
					req = req.Clone(req.Context())
					req.URL.Host = loopback.Host
					return underlying.RoundTrip(req)
				default:
					t.Errorf("incorrect QQ pairing URL %s", req.URL)
					return response(500, "{}"), nil
				}
			})

			ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
			defer cancel()
			ready := false
			var target, botName string
			var received int
			err = (transport{client: client}).pairGateway(ctx, "qq", test.mode,
				connectionSecret{Address: "app", Signing: "secret"}, func() { ready = true }, func(value string) {
					target = value
					cancel()
				}, func(observation qqPairingObservation) {
					if observation.botName != "" {
						botName = observation.botName
					}
					if observation.receivedMessage {
						received++
					}
				})
			if botName != "文章通知机器人" || (ack && received != 1) || (!ack && received != 0) {
				t.Fatalf("unexpected observation: name=%q inbox=%d", botName, received)
			}
			if ack && (!ready || target != "c2c:paired_user") {
				t.Fatalf("ready=%t target=%q error=%v", ready, target, err)
			}
			if !ack && (ready || !errors.Is(err, errQQHeartbeat)) {
				t.Fatalf("gateway must stay unconfirmed: ready=%t error=%v", ready, err)
			}
		})
	}
}
