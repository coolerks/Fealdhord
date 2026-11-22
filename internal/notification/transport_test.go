package notification

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestWebhookFormatsAndCredentialHost(t *testing.T) {
	addresses := map[string]string{
		"discord":  "https://discord.com/api/webhooks/123/token",
		"slack":    "https://hooks.slack.com/services/T/B/token",
		"dingtalk": "https://oapi.dingtalk.com/robot/send?access_token=token",
		"feishu":   "https://open.feishu.cn/open-apis/bot/v2/hook/token",
		"lark":     "https://open.larksuite.com/open-apis/bot/v2/hook/token",
		"wecom":    "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=token",
	}
	for platform, address := range addresses {
		t.Run(platform, func(t *testing.T) {
			secret := connectionSecret{Address: address, Signing: "key"}
			_, payload, err := makeRequest(platform, "", "", secret, Message{
				Title: "标题", URL: "https://example.com/article",
				Links: []ArticleLink{{Title: "另一篇", URL: "https://example.com/other"}},
			}, time.Unix(1800, 0))
			data, _ := json.Marshal(payload)
			if err != nil || !strings.Contains(string(data), "example.com/article") || !strings.Contains(string(data), "example.com/other") {
				t.Fatalf("payload=%s error=%v", data, err)
			}
		})
	}
	for _, raw := range []string{"https://hooks.slack.com.example.com/services/T/B/token", "http://hooks.slack.com/services/T/B/token", "https://u@hooks.slack.com/services/T/B/token"} {
		if _, err := webhookURL("slack", raw); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}

func TestWeChatBindingKeepsTokenInCredentialStore(t *testing.T) {
	service, _ := newTestNotification(t)
	const botToken = "synthetic@region|opaque"
	var polls int
	service.transport.client.Transport = sender(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/ilink/bot/get_bot_qrcode":
			if r.Method != http.MethodPost || r.Header.Get("iLink-App-Id") != "bot" ||
				r.Header.Get("iLink-App-ClientVersion") != "196609" || r.Header.Get("X-WECHAT-UIN") == "" ||
				r.Header.Get("AuthorizationType") != "ilink_bot_token" || r.Header.Get("Authorization") != "" {
				t.Fatal("invalid QR creation request")
			}
			return response(200, `{"qrcode":"sample-code","qrcode_img_content":"sample-qr-payload"}`), nil
		case "/ilink/bot/get_qrcode_status":
			polls++
			if r.Method != http.MethodGet || r.Header.Get("iLink-App-Id") != "bot" ||
				r.Header.Get("iLink-App-ClientVersion") != "196609" || r.Header.Get("AuthorizationType") != "" {
				t.Fatal("invalid QR status request")
			}
			if polls == 1 {
				return response(200, `{"status":"scaned_but_redirect","redirect_host":"node.weixin.qq.com"}`), nil
			}
			if r.URL.Hostname() != "node.weixin.qq.com" {
				t.Fatal("poll did not use the assigned binding host")
			}
			code := r.URL.Query().Get("verify_code")
			switch polls {
			case 2:
				return response(200, `{"status":"need_verifycode"}`), nil
			case 3:
				if code != "1234" {
					t.Fatal("PIN missing from verification request")
				}
				return response(200, `{"status":"scaned"}`), nil
			}
			if code != "" {
				t.Fatal("poll resent an accepted PIN")
			}
			return response(200, `{"status":"confirmed","ilink_user_id":"owner@im.wechat","bot_token":"`+botToken+`","baseurl":"https://ilinkai.weixin.qq.com"}`), nil
		case "/ilink/bot/getupdates":
			if r.Header.Get("Authorization") != "Bearer "+botToken {
				t.Fatal("getupdates did not preserve opaque token")
			}
			return response(200, `{"ret":0,"msgs":[{"from_user_id":"other","context_token":"wrong"},{"from_user_id":"owner@im.wechat","message_type":1,"context_token":"local_context","item_list":[{"type":1,"text_item":{"text":"配对"}}]}]}`), nil
		case "/ilink/bot/sendmessage":
			if r.Header.Get("Authorization") != "Bearer "+botToken {
				t.Fatal("sendmessage did not use the stored token")
			}
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), "local_context") || strings.Contains(string(body), "wrong") {
				t.Fatalf("incorrect context: %s", body)
			}
			return response(200, `{"ret":0}`), nil
		default:
			t.Fatalf("unrecognized request %s", r.URL.Path)
			return nil, nil
		}
	})
	qr, err := service.startWeChatBinding(t.Context())
	if err != nil || !strings.HasPrefix(qr.Image, "data:image/png;base64,") {
		t.Fatalf("QR error=%v", err)
	}
	for _, step := range []struct{ pin, status string }{{"", "scaned"}, {"", "need_verifycode"}, {"1234", "scaned"}} {
		got, err := service.pollWeChatBinding(t.Context(), qr.ID, step.pin, false)
		if err != nil || got.Status != step.status {
			t.Fatalf("expected %s, got %s, err=%v", step.status, got.Status, err)
		}
	}
	result, err := service.pollWeChatBinding(t.Context(), qr.ID, "1234", true)
	if err != nil || result.Status != "connected" || result.Target != "owner@im.wechat" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	data, _ := json.Marshal(result)
	if strings.Contains(string(data), botToken) || strings.Contains(string(data), "local_context") {
		t.Fatal("credential returned to frontend")
	}
	channels, err := service.ListChannels(t.Context())
	if err != nil || len(channels) != 0 {
		t.Fatalf("pairing wrote a channel before confirmation: %v %v", channels, err)
	}
	channel, err := service.completeWeChatBinding(t.Context(), qr.ID, ChannelInput{Channel: Channel{Name: "我的微信", Enabled: true}})
	if err != nil || channel.Verified {
		t.Fatalf("channel=%+v err=%v", channel, err)
	}
	if err := service.TestChannel(t.Context(), channel.ID); err != nil {
		t.Fatal(err)
	}
}

func TestWeChatBindingStopsOnRevokedBotSession(t *testing.T) {
	service, _ := newTestNotification(t)
	service.transport.client.Transport = sender(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/ilink/bot/get_bot_qrcode":
			return response(200, `{"qrcode":"sample-code","qrcode_img_content":"sample-qr-payload"}`), nil
		case "/ilink/bot/get_qrcode_status":
			return response(200, `{"status":"confirmed","ilink_user_id":"owner@im.wechat","bot_token":"sample","baseurl":"https://ilinkai.weixin.qq.com"}`), nil
		case "/ilink/bot/getupdates":
			return response(200, `{"errcode":-14}`), nil
		default:
			t.Fatalf("unexpected request %s", r.URL.Path)
			return nil, nil
		}
	})
	qr, err := service.startWeChatBinding(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	status, err := service.pollWeChatBinding(t.Context(), qr.ID, "", true)
	if err != nil || status.Status != "expired" || status.Target != "" {
		t.Fatalf("expected terminal expired status, got %s, err=%v", status.Status, err)
	}
}

func TestWeChatSkippedPairingCanResume(t *testing.T) {
	service, _ := newTestNotification(t)
	updateCalls := 0
	service.transport.client.Transport = sender(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/ilink/bot/get_bot_qrcode":
			return response(200, `{"qrcode":"sample-code","qrcode_img_content":"sample-qr-payload"}`), nil
		case "/ilink/bot/get_qrcode_status":
			return response(200, `{"status":"confirmed","ilink_user_id":"owner@im.wechat","bot_token":"sample","baseurl":"https://ilinkai.weixin.qq.com"}`), nil
		case "/ilink/bot/getupdates":
			updateCalls++
			if updateCalls == 1 {
				return response(200, `{"ret":0,"msgs":[{"from_user_id":"owner@im.wechat","context_token":"ignored","message_type":1,"item_list":[{"type":1,"text_item":{"text":"请配对"}}]}]}`), nil
			}
			return response(200, `{"ret":0,"msgs":[{"from_user_id":"owner@im.wechat","context_token":"paired","message_type":1,"item_list":[{"type":1,"text_item":{"text":"配对"}}]}]}`), nil
		default:
			t.Errorf("unrecognized request %s", r.URL.Path)
			return response(500, "{}"), nil
		}
	})
	ctx := t.Context()
	qr, err := service.startWeChatBinding(ctx)
	if err != nil {
		t.Fatal(err)
	}
	status, err := service.pollWeChatBinding(ctx, qr.ID, "", false)
	if err != nil || status.Status != "await_message" || updateCalls != 0 {
		t.Fatalf("QR confirmation status=%+v, err=%v, update calls=%d", status, err, updateCalls)
	}
	input := ChannelInput{Channel: Channel{Name: "微信", Enabled: true}}
	channel, err := service.completeWeChatBinding(ctx, qr.ID, input)
	if err != nil || channel.Verified || len(channel.Recipients) != 0 {
		t.Fatalf("skipped channel=%+v, err=%v", channel, err)
	}
	input.Channel = channel
	pair, err := service.resumeWeChatBinding(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	status, err = service.pollWeChatBinding(ctx, pair.ID, "", true)
	if err != nil || status.Status != "await_message" {
		t.Fatalf("non-matching message returned %+v, err=%v", status, err)
	}
	status, err = service.pollWeChatBinding(ctx, pair.ID, "", true)
	if err != nil || status.Status != "connected" {
		t.Fatalf("matching message returned %+v, err=%v", status, err)
	}
	channel, err = service.completeWeChatBinding(ctx, pair.ID, input)
	if err != nil || channel.ID != input.ID || len(channel.Recipients) != 1 || channel.Recipients[0].Target != status.Target {
		t.Fatalf("repaired channel=%+v, err=%v", channel, err)
	}
}

func TestQQSandboxDeliveryUsesSavedMode(t *testing.T) {
	service, _ := newTestNotification(t)
	service.transport.client.Transport = sender(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.bot.qq.com/app/getAppAccessToken":
			return response(200, `{"access_token":"sample"}`), nil
		case "https://sandbox.api.sgroup.qq.com/v2/users/paired_user/messages":
			if r.Header.Get("Authorization") != "QQBot sample" {
				t.Error("incorrect QQ authorization")
			}
			return response(200, `{"id":"message_id"}`), nil
		default:
			t.Errorf("incorrect QQ delivery URL %s", r.URL)
			return response(500, "{}"), nil
		}
	})
	channel, err := service.SaveChannel(t.Context(), ChannelInput{
		Channel: Channel{Name: "QQ", Platform: "qq", Mode: qqSandbox, Enabled: true},
		Address: "app", Signing: "secret", Targets: []string{"c2c:paired_user"},
	})
	if err != nil || channel.Mode != qqSandbox {
		t.Fatalf("saved QQ mode: %q, error: %v", channel.Mode, err)
	}
	if err := service.TestChannel(t.Context(), channel.ID); err != nil {
		t.Fatal(err)
	}
}
