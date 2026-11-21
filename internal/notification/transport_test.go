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
	service.transport.client.Transport = sender(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/ilink/bot/get_bot_qrcode":
			if r.Method != http.MethodPost {
				t.Fatal(r.Method)
			}
			return response(200, `{"qrcode":"sample-code","qrcode_img_content":"sample-qr-payload"}`), nil
		case "/ilink/bot/get_qrcode_status":
			return response(200, `{"status":"confirmed","ilink_user_id":"owner@im.wechat","bot_token":"local_token","baseurl":"https://ilinkai.weixin.qq.com"}`), nil
		case "/ilink/bot/getupdates":
			return response(200, `{"ret":0,"msgs":[{"from_user_id":"other","context_token":"wrong"},{"from_user_id":"owner@im.wechat","context_token":"local_context"}]}`), nil
		case "/ilink/bot/sendmessage":
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
	result, err := service.pollWeChatBinding(t.Context(), qr.ID, "", "我的微信")
	if err != nil || result.Status != "connected" || result.Channel == nil || result.Channel.Verified {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	data, _ := json.Marshal(result)
	if strings.Contains(string(data), "local_token") || strings.Contains(string(data), "local_context") {
		t.Fatal("credential returned to frontend")
	}
	if err := service.TestChannel(t.Context(), result.Channel.ID); err != nil {
		t.Fatal(err)
	}
}
