package notification

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	numericID     = regexp.MustCompile("^[0-9]{1,20}$")
	slackMemberID = regexp.MustCompile("^[UW][A-Z0-9]+$")
	memberID      = regexp.MustCompile("^[A-Za-z0-9_.@-]+$")
)

func feishuRecipient(target string) (string, string, bool) {
	kind, value, typed := strings.Cut(target, ":")
	if !typed {
		kind, value = "open_id", target
	}
	switch kind {
	case "open_id":
		return kind, value, strings.HasPrefix(value, "ou_") && memberID.MatchString(value)
	case "union_id":
		return kind, value, strings.HasPrefix(value, "on_") && memberID.MatchString(value)
	case "user_id":
		return kind, value, memberID.MatchString(value)
	default:
		return "", "", false
	}
}

func validAppCredentials(platform string, secret connectionSecret) bool {
	switch platform {
	case "discord":
		return discordTokenPattern.MatchString(secret.Address)
	case "slack":
		return strings.HasPrefix(secret.Address, "xoxb-") && tokenPattern.MatchString(secret.Address)
	case "dingtalk":
		return tokenPattern.MatchString(secret.Address) && tokenPattern.MatchString(secret.Signing) &&
			(secret.Context == "" || tokenPattern.MatchString(secret.Context))
	case "feishu", "lark":
		return tokenPattern.MatchString(secret.Address) && tokenPattern.MatchString(secret.Signing)
	case "wecom":
		agent, _ := strconv.ParseUint(secret.Context, 10, 32)
		return agent > 0 && tokenPattern.MatchString(secret.Address) && tokenPattern.MatchString(secret.Signing)
	default:
		return false
	}
}

func checkAppRecipient(platform, target string, secret connectionSecret) error {
	valid := false
	switch platform {
	case "discord":
		valid = numericID.MatchString(target)
	case "slack":
		valid = slackMemberID.MatchString(target)
	case "dingtalk":
		valid = memberID.MatchString(target)
	case "feishu", "lark":
		_, _, valid = feishuRecipient(target)
	case "wecom":
		valid = memberID.MatchString(target) && !strings.HasPrefix(target, "@")
	}
	if !valid || !validAppCredentials(platform, secret) {
		return errors.New("请检查收件 ID 和应用凭据")
	}
	return nil
}

type appReply struct {
	ID             string   `json:"id"`
	OK             bool     `json:"ok"`
	Code           int      `json:"code"`
	ErrCode        *int     `json:"errcode"`
	InvalidStaff   []string `json:"invalidStaffIdList"`
	ThrottledStaff []string `json:"flowControlledStaffIdList"`
	InvalidUser    string   `json:"invaliduser"`
	AccessToken    string   `json:"accessToken"`
	WeComToken     string   `json:"access_token"`
	TenantToken    string   `json:"tenant_access_token"`
	ProcessKey     string   `json:"processQueryKey"`
	Channel        struct {
		ID string `json:"id"`
	} `json:"channel"`
	Data struct {
		MessageID string `json:"message_id"`
	} `json:"data"`
}

func (t transport) appPost(ctx context.Context, endpoint string, payload map[string]any, headers map[string]string, sending bool) (appReply, deliveryResult) {
	response, body, err := t.post(ctx, endpoint, payload, headers)
	var reply appReply
	if response != nil && response.StatusCode == http.StatusTooManyRequests {
		wait := time.Minute
		if seconds, err := strconv.Atoi(response.Header.Get("Retry-After")); err == nil && seconds > 0 {
			wait = time.Duration(min(seconds, 86400)) * time.Second
		}
		return reply, deliveryResult{state: "pending", detail: "rate_limited", wait: wait}
	}
	if err != nil || response.StatusCode >= 500 {
		if sending {
			return reply, deliveryResult{state: "unknown", detail: "response_unknown"}
		}
		return reply, deliveryResult{state: "failed", detail: "connection"}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return reply, deliveryResult{state: "failed", detail: "platform_rejected"}
	}
	if json.Unmarshal(body, &reply) != nil {
		if sending {
			return reply, deliveryResult{state: "unknown", detail: "invalid_response"}
		}
		return reply, deliveryResult{state: "failed", detail: "invalid_response"}
	}
	if reply.Code != 0 || (reply.ErrCode != nil && *reply.ErrCode != 0) || reply.InvalidUser != "" {
		return reply, deliveryResult{state: "failed", detail: "platform_rejected"}
	}
	return reply, deliveryResult{state: "accepted"}
}

func (t transport) sendApp(ctx context.Context, platform, target, deliveryID string, secret connectionSecret, message Message) deliveryResult {
	if err := checkAppRecipient(platform, target, secret); err != nil {
		return deliveryResult{state: "failed", detail: "configuration"}
	}
	if t.allowImages != nil && !t.allowImages(ctx) {
		message.Image = ""
	}
	message = formatMessage(message)
	switch platform {
	case "discord", "slack":
		return t.sendDirectChat(ctx, platform, target, secret, message)
	case "dingtalk":
		return t.sendDingTalkApp(ctx, target, secret, message)
	case "feishu", "lark":
		return t.sendLarkApp(ctx, platform, target, deliveryID, secret, message)
	case "wecom":
		return t.sendWeComApp(ctx, target, secret, message)
	default:
		return deliveryResult{state: "failed", detail: "configuration"}
	}
}

func (t transport) sendDirectChat(ctx context.Context, platform, target string, secret connectionSecret, message Message) deliveryResult {
	base, open, auth := "https://discord.com/api/v10/", "users/@me/channels", "Bot "+secret.Address
	arguments := map[string]any{"recipient_id": target}
	if platform == "slack" {
		base, open, auth = "https://slack.com/api/", "conversations.open", "Bearer "+secret.Address
		arguments = map[string]any{"users": target}
	}
	headers := map[string]string{"Authorization": auth}
	reply, result := t.appPost(ctx, base+open, arguments, headers, false)
	if result.state != "accepted" {
		return result
	}
	channel := reply.ID
	if platform == "slack" {
		channel = reply.Channel.ID
		if !reply.OK || !strings.HasPrefix(channel, "D") || !memberID.MatchString(channel) {
			return deliveryResult{state: "failed", detail: "recipient_not_available"}
		}
	} else if !numericID.MatchString(channel) {
		return deliveryResult{state: "failed", detail: "recipient_not_available"}
	}
	destination := "channels/" + channel + "/messages"
	payload := discordPayload(message)
	if platform == "slack" {
		destination = "chat.postMessage"
		payload = slackPayload(message)
		payload["channel"] = channel
	}
	reply, result = t.appPost(ctx, base+destination, payload, headers, true)
	if result.state == "accepted" && ((platform == "slack" && !reply.OK) || (platform == "discord" && reply.ID == "")) {
		return deliveryResult{state: "failed", detail: "platform_rejected"}
	}
	return result
}

func (t transport) sendDingTalkApp(ctx context.Context, target string, secret connectionSecret, message Message) deliveryResult {
	const base = "https://api.dingtalk.com/v1.0/"
	reply, result := t.appPost(ctx, base+"oauth2/accessToken", map[string]any{"appKey": secret.Address, "appSecret": secret.Signing}, nil, false)
	if result.state != "accepted" {
		return result
	}
	if reply.AccessToken == "" {
		return deliveryResult{state: "failed", detail: "authentication"}
	}
	robot := secret.Context
	if robot == "" {
		robot = secret.Address
	}
	text := markdownMessage(message)
	if message.Image != "" {
		text += "\n\n![](" + strings.ReplaceAll(message.Image, ")", "%29") + ")"
	}
	param, _ := json.Marshal(map[string]string{"title": clip(message.Title, 200), "text": text})
	reply, result = t.appPost(ctx, base+"robot/oToMessages/batchSend", map[string]any{
		"robotCode": robot, "userIds": []string{target}, "msgKey": "sampleMarkdown", "msgParam": string(param),
	}, map[string]string{"x-acs-dingtalk-access-token": reply.AccessToken}, true)
	if result.state == "accepted" && len(reply.ThrottledStaff) > 0 {
		return deliveryResult{state: "pending", detail: "rate_limited", wait: time.Minute}
	}
	if result.state == "accepted" && (reply.ProcessKey == "" || len(reply.InvalidStaff) > 0) {
		return deliveryResult{state: "failed", detail: "platform_rejected"}
	}
	return result
}

func (t transport) sendLarkApp(ctx context.Context, platform, target, id string, secret connectionSecret, message Message) deliveryResult {
	base := "https://" + webhooks[platform].host + "/open-apis/"
	reply, result := t.appPost(ctx, base+"auth/v3/tenant_access_token/internal",
		map[string]any{"app_id": secret.Address, "app_secret": secret.Signing}, nil, false)
	if result.state != "accepted" {
		return result
	}
	if reply.TenantToken == "" {
		return deliveryResult{state: "failed", detail: "authentication"}
	}
	if message.Image != "" {
		secret.MediaAppID, secret.MediaSecret = secret.Address, secret.Signing
		message.imageKey = t.putFeishuCover(ctx, platform, secret, message.Image)
	}
	body, _ := json.Marshal(larkPost(message))
	kind, value, _ := feishuRecipient(target)
	reply, result = t.appPost(ctx, base+"im/v1/messages?receive_id_type="+kind,
		map[string]any{"receive_id": value, "msg_type": "post", "content": string(body), "uuid": id},
		map[string]string{"Authorization": "Bearer " + reply.TenantToken}, true)
	if result.state == "accepted" && reply.Data.MessageID == "" {
		return deliveryResult{state: "failed", detail: "platform_rejected"}
	}
	return result
}

func (t transport) sendWeComApp(ctx context.Context, target string, secret connectionSecret, message Message) deliveryResult {
	const base = "https://qyapi.weixin.qq.com/cgi-bin/"
	query := url.Values{"corpid": {secret.Address}, "corpsecret": {secret.Signing}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"gettoken?"+query.Encode(), nil)
	if err != nil {
		return deliveryResult{state: "failed", detail: "authentication"}
	}
	response, err := t.client.Do(req)
	if err != nil {
		return deliveryResult{state: "failed", detail: "authentication"}
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxWireResponse+1))
	var token appReply
	if response.StatusCode != http.StatusOK || err != nil || len(data) > maxWireResponse ||
		json.Unmarshal(data, &token) != nil || token.ErrCode == nil || *token.ErrCode != 0 || token.WeComToken == "" {
		return deliveryResult{state: "failed", detail: "authentication"}
	}
	payload := wecomPayload(message)
	agent, _ := strconv.ParseUint(secret.Context, 10, 32)
	payload["agentid"], payload["touser"] = agent, target
	reply, result := t.appPost(ctx, base+"message/send?access_token="+url.QueryEscape(token.WeComToken), payload, nil, true)
	if result.state == "accepted" && reply.ErrCode == nil {
		return deliveryResult{state: "unknown", detail: "invalid_response"}
	}
	return result
}
