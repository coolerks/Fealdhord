package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/coolerks/Fealdhord/internal/netpolicy"
)

const maxWireResponse = 256 << 10

type ArticleLink struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Text  string `json:"text,omitempty"`
}

type Message struct {
	qqReplyID       string
	qqReplySequence int
	Title           string        `json:"title"`
	URL             string        `json:"url,omitempty"`
	Text            string        `json:"text,omitempty"`
	Image           string        `json:"image,omitempty"`
	Links           []ArticleLink `json:"links,omitempty"`
	imageKey        string
}

type deliveryResult struct {
	state, detail string
	wait          time.Duration
	userHint      string
}

type webhook struct {
	host, path string
}

var webhooks = map[string]webhook{
	"discord":  {"discord.com", "/api/webhooks/"},
	"slack":    {"hooks.slack.com", "/services/"},
	"dingtalk": {"oapi.dingtalk.com", "/robot/send"},
	"feishu":   {"open.feishu.cn", "/open-apis/bot/v2/hook/"},
	"lark":     {"open.larksuite.com", "/open-apis/bot/v2/hook/"},
	"wecom":    {"qyapi.weixin.qq.com", "/cgi-bin/webhook/send"},
}

var webhookSegments = map[string]int{"discord": 2, "slack": 3, "feishu": 1, "lark": 1}

var tokenPattern = regexp.MustCompile(`^[0-9A-Za-z_:-]+$`)
var discordTokenPattern = regexp.MustCompile(`^[0-9A-Za-z._~+/:=-]+$`)

// WeChat issues opaque credentials. Reject whitespace and control characters in the Bearer value.
var wechatTokenPattern = regexp.MustCompile(`^[!-~]+$`)
var qqTargetPattern = regexp.MustCompile(`^(group|c2c|channel):[0-9A-Za-z_-]+$`)

type connectionSecret struct {
	Address     string `json:"address"`
	Signing     string `json:"signing"`
	Context     string `json:"context,omitempty"`
	MediaAppID  string `json:"media_app_id,omitempty"`
	MediaSecret string `json:"media_secret,omitempty"`
}

func webhookURL(platform, address string) (*url.URL, error) {
	route, ok := webhooks[platform]
	u, err := url.Parse(address)
	if !ok || err != nil || u.Scheme != "https" || u.Host != route.host || u.User != nil || u.Fragment != "" ||
		!strings.HasPrefix(u.EscapedPath(), route.path) || strings.Contains(u.EscapedPath(), "%") {
		return nil, errors.New("Webhook 地址与渠道不匹配")
	}
	if platform == "dingtalk" || platform == "wecom" {
		key := "access_token"
		if platform == "wecom" {
			key = "key"
		}
		if u.Path != route.path || !tokenPattern.MatchString(u.Query().Get(key)) {
			return nil, errors.New("Webhook 缺少访问令牌")
		}
	} else {
		segments := strings.Split(strings.TrimPrefix(u.Path, route.path), "/")
		if len(segments) != webhookSegments[platform] {
			return nil, errors.New("Webhook 缺少目标标识")
		}
		for _, segment := range segments {
			if !tokenPattern.MatchString(segment) {
				return nil, errors.New("Webhook 目标标识无效")
			}
		}
	}
	return u, nil
}

const qqSandbox = "sandbox"

func validQQMode(mode string) bool { return mode == "" || mode == qqSandbox }

func qqAPIOrigin(mode string) string {
	if mode == qqSandbox {
		return "https://sandbox.api.sgroup.qq.com"
	}
	return "https://api.sgroup.qq.com"
}

func checkConnection(platform, target string, secret connectionSecret) error {
	switch platform {
	case "telegram":
		if target == "" || !tokenPattern.MatchString(secret.Address) {
			return errors.New("请填写机器人令牌与接收目标")
		}
		return nil
	case "wechat":
		if target == "" || secret.Context == "" {
			return errors.New("微信需要已绑定私聊的会话令牌")
		}
		if !wechatTokenPattern.MatchString(secret.Address) {
			return errors.New("微信返回的机器人令牌无效")
		}
		return nil
	case "qq":
		if !qqTargetPattern.MatchString(target) || !tokenPattern.MatchString(secret.Address) || !tokenPattern.MatchString(secret.Signing) {
			return errors.New("请填写 QQ AppID、AppSecret 和 group:、c2c: 或 channel: 接收目标")
		}
		return nil
	default:
		_, err := webhookURL(platform, secret.Address)
		return err
	}
}

func linkURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return ""
	}
	return u.String()
}

func clip(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit-1]) + "…"
}

func markdownText(value string) string {
	return strings.NewReplacer("\\", "\\\\", "[", "\\[", "]", "\\]", "*", "\\*", "_", "\\_", "`", "\\`", "<", "&lt;", ">", "&gt;").Replace(value)
}

func plainMessage(m Message) string {
	text := strings.TrimSpace(m.Title + "\n" + m.Text + "\n" + m.URL)
	for _, item := range m.Links {
		text += "\n\n" + plainMessage(Message{Title: item.Title, URL: item.URL, Text: item.Text})
	}
	return strings.TrimSpace(text)
}

func markdownLink(title, raw string) string {
	if title == "" {
		title = linkURL(raw)
	}
	title = markdownText(title)
	if target := linkURL(raw); target != "" {
		target = strings.NewReplacer("(", "%28", ")", "%29").Replace(target)
		return "[" + title + "](" + target + ")"
	}
	return title
}

func markdownMessage(m Message) string {
	text := strings.TrimSpace(markdownLink(m.Title, m.URL) + "\n\n" + markdownText(m.Text))
	for _, item := range m.Links {
		text += "\n\n" + strings.TrimSpace(markdownLink(item.Title, item.URL)+"\n"+markdownText(item.Text))
	}
	return strings.TrimSpace(text)
}

func htmlLink(title, raw string) string {
	if title == "" {
		title = linkURL(raw)
	}
	title = html.EscapeString(title)
	if target := linkURL(raw); target != "" {
		return "<a href=\"" + html.EscapeString(target) + "\">" + title + "</a>"
	}
	return title
}

func signature(secret, message string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func imagePayload(m Message) map[string]any {
	if m.Image == "" {
		return nil
	}
	return map[string]any{"url": m.Image}
}

func formatMessage(m Message) Message {
	m.URL = linkURL(m.URL)
	m.Image = linkURL(m.Image)
	m.Title = clip(m.Title, 250)
	m.Text = clip(m.Text, 1300)
	return m
}

func discordPayload(m Message) map[string]any {
	embed := map[string]any{}
	if m.Title != "" {
		embed["title"] = m.Title
	}
	if m.Text != "" {
		embed["description"] = m.Text
	}
	if m.URL != "" {
		if m.Title == "" {
			embed["description"] = strings.TrimSpace(markdownLink("", m.URL) + "\n\n" + m.Text)
		} else {
			embed["url"] = m.URL
		}
	}
	if len(m.Links) != 0 {
		embed["description"] = markdownMessage(Message{Links: m.Links})
	}
	if m.Image != "" {
		embed["image"] = imagePayload(m)
	}
	return map[string]any{"embeds": []any{embed}, "allowed_mentions": map[string]any{"parse": []string{}}}
}

func slackPayload(m Message) map[string]any {
	escape := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	title := escape.Replace(m.Title)
	if m.URL != "" {
		if title == "" {
			title = escape.Replace(m.URL)
		}
		title = "<" + strings.ReplaceAll(escape.Replace(m.URL), "|", "%7C") + "|" + strings.ReplaceAll(title, "|", "｜") + ">"
	}
	var blocks []any
	if title != "" {
		blocks = append(blocks, map[string]any{"type": "section", "text": map[string]string{"type": "mrkdwn", "text": title}})
	}
	if m.Text != "" {
		blocks = append(blocks, map[string]any{"type": "section", "text": map[string]string{"type": "plain_text", "text": m.Text}})
	}
	for _, item := range m.Links {
		if heading := slackArticleHeading(item); heading != "" {
			blocks = append(blocks, map[string]any{"type": "section", "text": map[string]string{"type": "mrkdwn", "text": heading}})
		}
		if item.Text != "" {
			blocks = append(blocks, map[string]any{"type": "section", "text": map[string]string{"type": "plain_text", "text": item.Text}})
		}
	}
	if m.Image != "" {
		blocks = append(blocks, map[string]string{"type": "image", "image_url": m.Image, "alt_text": m.Title})
	}
	return map[string]any{"text": escape.Replace(plainMessage(m)), "blocks": blocks, "unfurl_links": false, "unfurl_media": false}
}

func slackArticleHeading(item ArticleLink) string {
	escape := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	title := escape.Replace(item.Title)
	if target := linkURL(item.URL); target != "" {
		target = escape.Replace(target)
		if title == "" {
			title = target
		}
		return "<" + strings.ReplaceAll(target, "|", "%7C") + "|" + strings.ReplaceAll(title, "|", "｜") + ">"
	}
	return title
}

func larkPost(m Message) map[string]any {
	title := map[string]string{"tag": "text", "text": m.Title}
	if m.URL != "" {
		title["tag"], title["href"] = "a", m.URL
		if m.Title == "" {
			title["text"] = m.URL
		}
	}
	var content [][]map[string]string
	if title["text"] != "" {
		content = append(content, []map[string]string{title})
	}
	if m.Text != "" {
		content = append(content, []map[string]string{{"tag": "text", "text": m.Text}})
	}
	for _, item := range m.Links {
		link := map[string]string{"tag": "text", "text": item.Title}
		if target := linkURL(item.URL); target != "" {
			link["tag"], link["href"] = "a", target
			if item.Title == "" {
				link["text"] = target
			}
		}
		if link["text"] != "" {
			content = append(content, []map[string]string{link})
		}
		if item.Text != "" {
			content = append(content, []map[string]string{{"tag": "text", "text": item.Text}})
		}
	}
	if m.imageKey != "" {
		content = append(content, []map[string]string{{"tag": "img", "image_key": m.imageKey}})
	}
	return map[string]any{"zh_cn": map[string]any{"content": content}}
}

func wecomPayload(m Message) map[string]any {
	if m.Image != "" && m.URL != "" {
		return map[string]any{"msgtype": "news", "news": map[string]any{"articles": []map[string]string{{"title": m.Title, "description": m.Text, "url": m.URL, "picurl": m.Image}}}}
	}
	return map[string]any{"msgtype": "markdown", "markdown": map[string]string{"content": markdownMessage(m)}}
}

func makeRequest(platform, mode, target string, secret connectionSecret, m Message, now time.Time) (string, map[string]any, error) {
	m = formatMessage(m)
	switch platform {
	case "telegram":

		operation, textKey := "sendMessage", "text"
		payload := map[string]any{"chat_id": target, "parse_mode": "HTML"}
		if m.Image != "" {
			operation, textKey = "sendPhoto", "caption"
			payload["photo"] = m.Image
			m.Text = clip(m.Text, 700)
		} else {
			payload["link_preview_options"] = map[string]bool{"is_disabled": true}
		}
		text := strings.TrimSpace(htmlLink(m.Title, m.URL) + "\n\n" + html.EscapeString(m.Text))
		for _, link := range m.Links {
			text += "\n\n" + strings.TrimSpace(htmlLink(link.Title, link.URL)+"\n"+html.EscapeString(link.Text))
		}
		payload[textKey] = strings.TrimSpace(text)
		return "https://api.telegram.org/bot" + secret.Address + "/" + operation, payload, nil
	case "qq":
		kind, id, _ := strings.Cut(target, ":")
		path := map[string]string{"c2c": "/v2/users/", "group": "/v2/groups/", "channel": "/channels/"}[kind]
		payload := map[string]any{"content": plainMessage(m)}
		if m.qqReplyID != "" {
			payload["msg_id"] = m.qqReplyID
			if kind != "channel" {
				payload["msg_seq"] = m.qqReplySequence
			}
		}
		if kind != "channel" {
			payload["msg_type"] = 0
		} else if m.Image != "" {
			payload["image"] = m.Image
		}
		return qqAPIOrigin(mode) + path + id + "/messages", payload, nil
	}
	u, err := webhookURL(platform, secret.Address)
	if err != nil {
		return "", nil, err
	}
	switch platform {
	case "discord":
		query := u.Query()
		query.Set("wait", "true")
		u.RawQuery = query.Encode()
		return u.String(), discordPayload(m), nil
	case "slack":
		return u.String(), slackPayload(m), nil
	case "dingtalk":
		if secret.Signing != "" {
			timestamp := strconv.FormatInt(now.UnixMilli(), 10)
			query := u.Query()
			query.Set("timestamp", timestamp)
			query.Set("sign", signature(secret.Signing, timestamp+"\n"+secret.Signing))
			u.RawQuery = query.Encode()
		}
		text := markdownMessage(m)
		if m.Image != "" {
			text += "\n\n![](" + m.Image + ")"
		}
		return u.String(), map[string]any{"msgtype": "markdown", "markdown": map[string]string{"title": dingTalkTitle(m), "text": text}}, nil
	case "feishu", "lark":
		payload := map[string]any{"msg_type": "post", "content": map[string]any{"post": larkPost(m)}}
		if secret.Signing != "" {
			timestamp := strconv.FormatInt(now.Unix(), 10)
			payload["timestamp"], payload["sign"] = timestamp, signature(timestamp+"\n"+secret.Signing, "")
		}
		return u.String(), payload, nil
	case "wecom":
		return u.String(), wecomPayload(m), nil
	default:
		return "", nil, errors.New("未知通知渠道")
	}
}

func dingTalkTitle(m Message) string {
	if m.Title == "" {
		return "文章"
	}
	return clip(m.Title, 200)
}

type transport struct {
	client      *http.Client
	images      netpolicy.Fetcher
	allowImages func(context.Context) bool
}

func newTransport() transport {
	return transport{client: &http.Client{
		Timeout:       15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (t transport) post(ctx context.Context, endpoint string, params map[string]any, headers map[string]string) (*http.Response, []byte, error) {
	body, err := json.Marshal(params)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	response, err := t.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer response.Body.Close()
	body, err = io.ReadAll(io.LimitReader(response.Body, maxWireResponse+1))
	if len(body) > maxWireResponse {
		err = errors.New("响应过大")
	}
	return response, body, err
}

func (t transport) send(ctx context.Context, platform, mode, target, deliveryID string, secret connectionSecret, m Message, now time.Time) deliveryResult {
	if err := checkConnection(platform, target, secret); err != nil {
		return deliveryResult{state: "failed", detail: "configuration"}
	}
	if t.allowImages != nil && !t.allowImages(ctx) {
		m.Image = ""
	}
	if platform == "wechat" {
		return t.sendWeChat(ctx, target, deliveryID, secret, m)
	}
	if (platform == "feishu" || platform == "lark") && m.Image != "" {
		m.imageKey = t.putFeishuCover(ctx, platform, secret, m.Image)
	}
	endpoint, payload, err := makeRequest(platform, mode, target, secret, m, now)
	if err != nil {
		return deliveryResult{state: "failed", detail: "configuration"}
	}
	headers := map[string]string{}
	switch platform {
	case "qq":
		token, err := t.qqAccessToken(ctx, secret)
		if err != nil {
			return deliveryResult{state: "failed", detail: "authentication"}
		}
		headers["Authorization"] = "QQBot " + token
		if strings.HasPrefix(m.Image, "https://") && !strings.HasPrefix(target, "channel:") {
			files := strings.TrimSuffix(endpoint, "/messages") + "/files"
			upload, data, err := t.post(ctx, files, map[string]any{"file_type": 1, "url": m.Image, "srv_send_msg": false}, headers)
			var attachment struct {
				FileInfo string `json:"file_info"`
			}
			if err == nil && upload.StatusCode == http.StatusOK && json.Unmarshal(data, &attachment) == nil && attachment.FileInfo != "" {
				payload["msg_type"] = 7
				payload["media"] = map[string]string{"file_info": attachment.FileInfo}
			}
		}
	}
	response, data, err := t.post(ctx, endpoint, payload, headers)
	if err != nil {
		return deliveryResult{state: "unknown", detail: "response_unknown"}
	}
	if platform == "telegram" && m.Image != "" && response.StatusCode == http.StatusBadRequest {
		m.Image = ""
		return t.send(ctx, platform, mode, target, deliveryID, secret, m, now)
	}
	if response.StatusCode == http.StatusTooManyRequests {
		retry := time.Minute
		if seconds, err := strconv.Atoi(response.Header.Get("Retry-After")); err == nil && seconds > 0 {
			retry = time.Duration(min(seconds, 86400)) * time.Second
		}
		return deliveryResult{state: "pending", detail: "rate_limited", wait: retry}
	}
	if response.StatusCode >= 500 {
		return deliveryResult{state: "unknown", detail: fmt.Sprintf("http_%d", response.StatusCode)}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return deliveryResult{state: "failed", detail: fmt.Sprintf("http_%d", response.StatusCode)}
	}
	if platform == "slack" && strings.TrimSpace(string(data)) != "ok" {
		return deliveryResult{state: "failed", detail: "platform_rejected"}
	}
	if platform != "slack" {
		var result struct {
			ID         string `json:"id"`
			OK         *bool  `json:"ok"`
			Code       int    `json:"code"`
			StatusCode int    `json:"StatusCode"`
			ErrCode    int    `json:"errcode"`
			Ret        int    `json:"ret"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return deliveryResult{state: "unknown", detail: "invalid_response"}
		}
		if result.Code != 0 || result.StatusCode != 0 || result.ErrCode != 0 || result.Ret != 0 ||
			(platform == "telegram" && (result.OK == nil || !*result.OK)) ||
			((platform == "qq" || platform == "discord") && result.ID == "") {
			return deliveryResult{state: "failed", detail: "platform_rejected"}
		}
	}
	return deliveryResult{state: "accepted"}
}
