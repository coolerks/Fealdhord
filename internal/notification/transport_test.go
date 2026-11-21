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
			_, payload, err := makeRequest(platform, "", secret, Message{
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
			return response(200, `{"ret":0,"msgs":[{"from_user_id":"other","context_token":"wrong"},{"from_user_id":"owner@im.wechat","context_token":"local_context"}]}`), nil
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
		got, err := service.pollWeChatBinding(t.Context(), qr.ID, step.pin, "我的微信")
		if err != nil || got.Status != step.status {
			t.Fatalf("expected %s, got %s, err=%v", step.status, got.Status, err)
		}
	}
	result, err := service.pollWeChatBinding(t.Context(), qr.ID, "1234", "我的微信")
	if err != nil || result.Status != "connected" || result.Channel == nil || result.Channel.Verified {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	data, _ := json.Marshal(result)
	if strings.Contains(string(data), botToken) || strings.Contains(string(data), "local_context") {
		t.Fatal("credential returned to frontend")
	}
	if err := service.TestChannel(t.Context(), result.Channel.ID); err != nil {
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
	status, err := service.pollWeChatBinding(t.Context(), qr.ID, "", "微信")
	if err != nil || status.Status != "expired" || status.Channel != nil {
		t.Fatalf("expected terminal expired status, got %s, err=%v", status.Status, err)
	}
}
