package notification

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	ilink "github.com/openilink/openilink-sdk-go"
	qrcode "github.com/skip2/go-qrcode"
)

const wechatBase = "https://ilinkai.weixin.qq.com"
const wechatVersion = "3.0.1"

type wechatBinding struct {
	mu         sync.Mutex
	id         string
	qr         string
	owner      string
	token      string
	base       string
	statusBase string
	cursor     string
	expiry     time.Time
}

type WeChatBinding struct {
	ID      string   `json:"id"`
	Image   string   `json:"image,omitempty"`
	Status  string   `json:"status"`
	Channel *Channel `json:"channel,omitempty"`
}

func trustedWeChatOrigin(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != u.Hostname() || u.User != nil ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return ""
	}
	host := u.Hostname()
	if host != "ilinkai.weixin.qq.com" && !strings.HasSuffix(host, ".weixin.qq.com") {
		return ""
	}
	return "https://" + host
}

func (s *Service) startWeChatBinding(ctx context.Context) (WeChatBinding, error) {
	response, data, err := s.transport.post(ctx, wechatBase+"/ilink/bot/get_bot_qrcode?bot_type=3", map[string]any{"local_token_list": []string{}}, nil)
	var qr ilink.QRCodeResponse
	if err != nil || response.StatusCode != http.StatusOK || json.Unmarshal(data, &qr) != nil ||
		qr.QRCode == "" || qr.QRCodeImgContent == "" || len(qr.QRCodeImgContent) > 2048 {
		return WeChatBinding{}, errors.New("暂时无法获取微信绑定二维码")
	}
	png, err := qrcode.Encode(qr.QRCodeImgContent, qrcode.Medium, 256)
	if err != nil {
		return WeChatBinding{}, errors.New("无法生成微信绑定二维码")
	}
	id := rand.Text()
	s.wechat.mu.Lock()
	defer s.wechat.mu.Unlock()
	s.wechat.id, s.wechat.qr, s.wechat.expiry = id, qr.QRCode, s.now().Add(8*time.Minute)
	s.wechat.owner, s.wechat.token, s.wechat.cursor, s.wechat.base, s.wechat.statusBase = "", "", "", "", wechatBase
	return WeChatBinding{ID: id, Status: "wait", Image: "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)}, nil
}

func (s *Service) cancelWeChatBinding(id string) {
	s.wechat.mu.Lock()
	defer s.wechat.mu.Unlock()
	if s.wechat.id == id {
		s.wechat.id, s.wechat.qr, s.wechat.owner, s.wechat.token, s.wechat.cursor = "", "", "", "", ""
	}
}

func (s *Service) pollWeChatBinding(ctx context.Context, id, pin, name string) (WeChatBinding, error) {
	session := &s.wechat
	session.mu.Lock()
	defer session.mu.Unlock()
	if id == "" || session.id != id || s.now().After(session.expiry) {
		return WeChatBinding{ID: id, Status: "expired"}, nil
	}
	if session.token == "" {
		params := url.Values{"qrcode": {session.qr}}
		if pin != "" {
			if len(pin) > 12 || strings.Trim(pin, "0123456789") != "" {
				return WeChatBinding{}, errors.New("请输入数字验证码")
			}
			params.Set("verify_code", pin)
		}
		endpoint := session.statusBase + "/ilink/bot/get_qrcode_status?" + params.Encode()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return WeChatBinding{}, errors.New("无法读取微信绑定状态")
		}
		response, err := s.transport.client.Do(req)
		if err != nil {
			return WeChatBinding{ID: id, Status: "wait"}, nil
		}
		data, readErr := io.ReadAll(io.LimitReader(response.Body, maxWireResponse))
		_ = response.Body.Close()
		var status ilink.QRStatusResponse
		if readErr != nil || response.StatusCode != http.StatusOK || json.Unmarshal(data, &status) != nil {
			return WeChatBinding{ID: id, Status: "wait"}, nil
		}
		switch status.Status {
		case "confirmed":
			base := trustedWeChatOrigin(status.BaseURL)
			if base == "" || status.BotToken == "" || status.ILinkUserID == "" {
				return WeChatBinding{}, errors.New("微信返回的绑定信息无效")
			}
			session.owner, session.token, session.base = status.ILinkUserID, status.BotToken, base
		case "scaned_but_redirect":
			if base := trustedWeChatOrigin("https://" + status.RedirectHost); base != "" {
				session.statusBase = base
				return WeChatBinding{ID: id, Status: "scaned"}, nil
			}
			return WeChatBinding{}, errors.New("微信绑定节点不可用")
		default:
			return WeChatBinding{ID: id, Status: status.Status}, nil
		}
	}
	client := s.transport.wechatClient(session.token, session.base)
	updates, err := client.GetUpdates(ctx, session.cursor, 8000)
	if err != nil || updates == nil {
		return WeChatBinding{ID: id, Status: "await_message"}, nil
	}
	if updates.GetUpdatesBuf != "" {
		session.cursor = updates.GetUpdatesBuf
	}
	for _, message := range updates.Msgs {
		if message.FromUserID != session.owner || message.ContextToken == "" {
			continue
		}
		channel, err := s.SaveChannel(ctx, ChannelInput{
			Channel: Channel{Name: name, Platform: "wechat", Target: session.owner, Enabled: true},
			Address: session.token, Context: message.ContextToken, Signing: session.base,
		})
		if err != nil {
			return WeChatBinding{}, err
		}
		session.id, session.qr, session.token, session.owner, session.cursor = "", "", "", "", ""
		return WeChatBinding{Status: "connected", Channel: &channel}, nil
	}
	return WeChatBinding{ID: id, Status: "await_message"}, nil
}

func (t transport) wechatClient(token, origin string) *ilink.Client {
	if origin == "" {
		origin = wechatBase
	}
	return ilink.NewClient(token, ilink.WithBaseURL(origin), ilink.WithVersion(wechatVersion),
		ilink.WithHTTPClient(wechatHTTP{client: t.client}))
}

type wechatHTTP struct{ client *http.Client }

func (w wechatHTTP) Do(req *http.Request) (*http.Response, error) {
	if trustedWeChatOrigin(req.URL.Scheme+"://"+req.URL.Host) == "" || req.URL.User != nil {
		return nil, errors.New("微信返回了不受支持的服务地址")
	}
	response, err := w.client.Do(req)
	if err == nil {
		response.Body = &boundedBody{Reader: io.LimitReader(response.Body, maxWireResponse), close: response.Body.Close}
	}
	return response, err
}

type boundedBody struct {
	io.Reader
	close func() error
}

func (b *boundedBody) Close() error { return b.close() }

func (t transport) sendWeChat(ctx context.Context, target, deliveryID string, secret connectionSecret, message Message) deliveryResult {
	client := t.wechatClient(secret.Address, trustedWeChatOrigin(secret.Signing))
	items := []ilink.MessageItem{{Type: ilink.ItemText, TextItem: &ilink.TextItem{Text: plainMessage(message)}}}
	if data, _ := t.cover(ctx, message.Image); len(data) != 0 {
		upload, err := client.UploadFile(ctx, data, target, ilink.MediaImage)
		if err == nil {
			items = append(items, ilink.MessageItem{Type: ilink.ItemImage, ImageItem: &ilink.ImageItem{
				Media: &ilink.CDNMedia{EncryptQueryParam: upload.DownloadEncryptedQueryParam,
					AESKey: base64.StdEncoding.EncodeToString([]byte(upload.AESKey)), EncryptType: ilink.EncryptAES128ECB},
				MidSize: upload.CiphertextSize,
			}})
		}
	}
	err := client.SendMessage(ctx, &ilink.SendMessageReq{Msg: &ilink.WeixinMessage{
		ToUserID: target, ClientID: deliveryID, ContextToken: secret.Context,
		MessageType: 2, MessageState: 2,
		ItemList: items,
	}})
	if err != nil {
		return deliveryResult{state: "unknown", detail: "wechat_response"}
	}
	return deliveryResult{state: "accepted"}
}
