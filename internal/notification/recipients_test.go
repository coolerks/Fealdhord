package notification

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestRecipientAllowlistRetiresQueuedDestination(t *testing.T) {
	s, _ := newTestNotification(t)
	ctx := t.Context()
	var sends []string
	rejectThird := true
	s.transport.client.Transport = sender(func(request *http.Request) (*http.Response, error) {
		var payload struct {
			Target string `json:"chat_id"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		sends = append(sends, payload.Target)
		if payload.Target == "300" && rejectThird {
			return response(403, `{"ok":false}`), nil
		}
		return response(200, `{"ok":true}`), nil
	})
	channel, err := s.SaveChannel(ctx, ChannelInput{
		Channel: Channel{Name: "Telegram", Platform: "telegram", Enabled: true},
		Address: "1234:secret", Targets: []string{"100", "200", "300", " 100 "},
	})
	if err != nil || len(channel.Recipients) != 3 {
		t.Fatalf("saved recipients: %+v, %v", channel.Recipients, err)
	}
	if err := s.TestChannel(ctx, channel.ID); err == nil || !strings.Contains(err.Error(), "300") {
		t.Fatalf("partial test result: %v", err)
	}
	if err := s.Send(ctx, []string{channel.ID}, Message{Title: "新文章"}); err != nil {
		t.Fatal(err)
	}
	channel, err = s.SaveChannel(ctx, ChannelInput{Channel: channel, Targets: []string{"100", "300"}})
	if err != nil {
		t.Fatal(err)
	}
	channel, err = s.SaveChannel(ctx, ChannelInput{Channel: channel, Targets: []string{"100", "200", "300"}})
	if err != nil {
		t.Fatal(err)
	}
	rejectThird = false
	if err := s.TestChannel(ctx, channel.ID); err != nil {
		t.Fatal(err)
	}
	sends = nil
	for range 2 {
		if progress, err := s.deliver(ctx); !progress || err != nil {
			t.Fatalf("deliver = %t, %v", progress, err)
		}
	}
	if !reflect.DeepEqual(sends, []string{"100"}) {
		t.Fatal(sends)
	}
	rows, err := s.q.ListNotificationDeliveries(ctx)
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows = %+v, %v", rows, err)
	}
	states := map[string]string{}
	for _, row := range rows {
		states[row.RecipientTarget] = row.State
	}
	if !reflect.DeepEqual(states, map[string]string{"100": "accepted", "200": "cancelled"}) {
		t.Fatal(states)
	}
}

func TestAppPrivateDeliveryRoutesRecipients(t *testing.T) {
	for _, test := range []struct {
		platform, target                              string
		secret                                        connectionSecret
		authURL, authReply, authTargetKey             string
		sendURL, sendReply, sendTargetKey, sendTarget string
	}{
		{"discord", "1234", connectionSecret{Address: "bot.token.value"},
			"https://discord.com/api/v10/users/@me/channels", `{"id":"5678"}`, "recipient_id",
			"https://discord.com/api/v10/channels/5678/messages", `{"id":"0001"}`, "", ""},
		{"slack", "U012", connectionSecret{Address: "xoxb-bot-token"},
			"https://slack.com/api/conversations.open", `{"ok":true,"channel":{"id":"DABC"}}`, "users",
			"https://slack.com/api/chat.postMessage", `{"ok":true}`, "channel", "DABC"},
		{"dingtalk", "employee-12", connectionSecret{Address: "app-key", Signing: "app-secret"},
			"https://api.dingtalk.com/v1.0/oauth2/accessToken", `{"accessToken":"abc"}`, "",
			"https://api.dingtalk.com/v1.0/robot/oToMessages/batchSend", `{"processQueryKey":"abc"}`, "userIds", "employee-12"},
		{"feishu", "user_id:employee-12", connectionSecret{Address: "cli_app", Signing: "app-secret"},
			"https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal", `{"code":0,"tenant_access_token":"abc"}`, "",
			"https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=user_id", `{"code":0,"data":{"message_id":"om_123"}}`, "receive_id", "employee-12"},
		{"lark", "ou_123", connectionSecret{Address: "cli_app", Signing: "app-secret"},
			"https://open.larksuite.com/open-apis/auth/v3/tenant_access_token/internal", `{"code":0,"tenant_access_token":"abc"}`, "",
			"https://open.larksuite.com/open-apis/im/v1/messages?receive_id_type=open_id", `{"code":0,"data":{"message_id":"om_123"}}`, "receive_id", "ou_123"},
		{"wecom", "employee-12", connectionSecret{Address: "corp", Signing: "app-secret", Context: "10001"},
			"https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=corp&corpsecret=app-secret", `{"errcode":0,"access_token":"abc"}`, "",
			"https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=abc", `{"errcode":0}`, "touser", "employee-12"},
	} {
		t.Run(test.platform, func(t *testing.T) {
			transport := newTransport()
			calls := 0
			transport.client.Transport = sender(func(request *http.Request) (*http.Response, error) {
				calls++
				endpoint, body, key, target := test.authURL, test.authReply, test.authTargetKey, test.target
				if calls == 2 {
					endpoint, body, key, target = test.sendURL, test.sendReply, test.sendTargetKey, test.sendTarget
				}
				if calls > 2 || request.URL.String() != endpoint {
					t.Fatalf("request %d: %s", calls, request.URL)
				}
				if key != "" {
					var payload map[string]any
					if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
						t.Fatal(err)
					}
					expected := any(target)
					if key == "userIds" {
						expected = []any{target}
					}
					if !reflect.DeepEqual(payload[key], expected) {
						t.Fatalf("expected %s = %v, received %v", key, expected, payload[key])
					}
				}
				return response(200, body), nil
			})
			result := transport.sendApp(t.Context(), test.platform, test.target, "delivery", test.secret, Message{Title: "文章"})
			if result.state != "accepted" || calls != 2 {
				t.Fatalf("result = %+v, calls = %d", result, calls)
			}
		})
	}
}
