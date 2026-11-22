package notification

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

var errPairConnection = errors.New("无法接收配对消息，请检查机器人凭据和平台接收权限")

func (t transport) listenPairing(ctx context.Context, platform string, secret connectionSecret, slackToken string, ready func(), match func(string)) error {
	switch platform {
	case "telegram":
		return t.pairTelegram(ctx, secret.Address, ready, match)
	case "qq", "discord":
		return t.pairGateway(ctx, platform, secret, ready, match)
	case "slack":
		return t.pairSlack(ctx, secret.Address, slackToken, ready, match)
	case "dingtalk":
		return t.pairDingTalk(ctx, secret, ready, match)
	case "feishu", "lark":
		return t.pairLark(ctx, platform, secret, ready, match)
	}
	return errPairConnection
}

func (t transport) pairPost(ctx context.Context, endpoint string, request map[string]any, auth string, result any) error {
	headers := map[string]string{}
	if auth != "" {
		headers["Authorization"] = auth
	}
	resp, data, err := t.post(ctx, endpoint, request, headers)
	if err != nil || resp.StatusCode != http.StatusOK || json.Unmarshal(data, result) != nil {
		return errPairConnection
	}
	return nil
}

func (t transport) pairSocket(ctx context.Context, endpoint, domain string) (*websocket.Conn, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "wss" || u.User != nil || u.Host != u.Hostname() ||
		(u.Host != domain && !strings.HasSuffix(u.Host, "."+domain)) {
		return nil, errPairConnection
	}
	dialCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(dialCtx, endpoint, &websocket.DialOptions{HTTPClient: t.client})
	if err != nil {
		return nil, errPairConnection
	}
	conn.SetReadLimit(maxWireResponse)
	return conn, nil
}

func (t transport) pairTelegram(ctx context.Context, token string, ready func(), match func(string)) error {
	type message struct {
		Text string `json:"text"`
		Date int64  `json:"date"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		From struct {
			Bot bool `json:"is_bot"`
		} `json:"from"`
	}
	type update struct {
		ID      int64    `json:"update_id"`
		Message *message `json:"message"`
		Post    *message `json:"channel_post"`
	}
	endpoint := "https://api.telegram.org/bot" + token + "/getUpdates"
	started := time.Now().Unix()
	var offset int64
	first := true
	for ctx.Err() == nil {
		request := map[string]any{"limit": 100, "allowed_updates": []string{"message", "channel_post"}}
		if !first {
			request["offset"], request["timeout"] = offset, 8
		}
		var response struct {
			OK     bool     `json:"ok"`
			Result []update `json:"result"`
		}
		if err := t.pairPost(ctx, endpoint, request, "", &response); err != nil {
			return err
		}
		if !response.OK {
			return errPairConnection
		}
		for _, item := range response.Result {
			offset = max(offset, item.ID+1)
			event := item.Message
			if event == nil {
				event = item.Post
			}
			if !first && event != nil && event.Date >= started && !event.From.Bot && event.Chat.ID != 0 && isPairingText(event.Text) {
				match(strconv.FormatInt(event.Chat.ID, 10))
			}
		}
		if first {
			first = false
			ready()
		}
		// Empty long polls may return immediately when the provider is busy.
		if len(response.Result) == 0 {
			select {
			case <-ctx.Done():
			case <-time.After(500 * time.Millisecond):
			}
		}
	}
	return ctx.Err()
}

type pairingGatewayFrame struct {
	Op    int             `json:"op"`
	Event string          `json:"t"`
	Seq   *int64          `json:"s"`
	Data  json.RawMessage `json:"d"`
}

func (t transport) qqAccessToken(ctx context.Context, secret connectionSecret) (string, error) {
	var reply struct {
		Token string `json:"access_token"`
	}
	if t.pairPost(ctx, "https://api.bot.qq.com/app/getAppAccessToken", map[string]any{"appId": secret.Address, "clientSecret": secret.Signing}, "", &reply) != nil || reply.Token == "" {
		return "", errPairConnection
	}
	return reply.Token, nil
}

func (t transport) pairGateway(ctx context.Context, platform string, secret connectionSecret, ready func(), match func(string)) error {
	endpoint, domain := "wss://gateway.discord.gg/?v=10&encoding=json", "gateway.discord.gg"
	token, intents := secret.Address, 1<<12 // Discord: direct messages to the bot.
	if platform == "qq" {
		access, err := t.qqAccessToken(ctx, secret)
		if err != nil {
			return err
		}
		token, intents = "QQBot "+access, 1<<25|1<<30
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.bot.qq.com/gateway", nil)
		if err != nil {
			return errPairConnection
		}
		req.Header.Set("Authorization", token)
		resp, err := t.client.Do(req)
		if err != nil {
			return errPairConnection
		}
		defer resp.Body.Close()
		var gateway struct {
			URL string `json:"url"`
		}
		if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, maxWireResponse)).Decode(&gateway) != nil {
			return errors.New("QQ 未提供网关连接，请检查机器人的事件接收方式")
		}
		endpoint, domain = gateway.URL, "qq.com"
	}

	conn, err := t.pairSocket(ctx, endpoint, domain)
	if err != nil {
		return err
	}
	defer conn.CloseNow()
	var hello pairingGatewayFrame
	var interval struct {
		Millis int64 `json:"heartbeat_interval"`
	}
	handshake, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	if wsjson.Read(handshake, conn, &hello) != nil || hello.Op != 10 || json.Unmarshal(hello.Data, &interval) != nil ||
		interval.Millis < 1000 || interval.Millis > 120000 {
		return errPairConnection
	}
	identify := map[string]any{"token": token, "intents": intents}
	if platform == "qq" {
		identify["shard"] = []int{0, 1}
	} else {
		identify["properties"] = map[string]string{"os": "desktop", "browser": "Fealdhord", "device": "Fealdhord"}
	}
	if wsjson.Write(handshake, conn, map[string]any{"op": 2, "d": identify}) != nil {
		return errPairConnection
	}
	heartbeatCtx, stopHeartbeat := context.WithCancel(ctx)
	defer stopHeartbeat()
	var seq atomic.Pointer[int64]
	if platform == "qq" {
		var zero int64
		seq.Store(&zero)
	}
	sendHeartbeat := func() error {
		ctx, cancel := context.WithTimeout(heartbeatCtx, 5*time.Second)
		defer cancel()
		return wsjson.Write(ctx, conn, map[string]any{"op": 1, "d": seq.Load()})
	}
	go func() {
		ticker := time.NewTicker(time.Duration(interval.Millis) * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				if sendHeartbeat() != nil {
					_ = conn.CloseNow()
					return
				}
			}
		}
	}()
	readCtx := handshake
	var botID string
	for ctx.Err() == nil {
		var frame pairingGatewayFrame
		if wsjson.Read(readCtx, conn, &frame) != nil {
			return errPairConnection
		}
		if frame.Seq != nil {
			seq.Store(frame.Seq)
		}
		switch {
		case frame.Op == 7 || frame.Op == 9:
			return errPairConnection
		case frame.Op == 1:
			if sendHeartbeat() != nil {
				return errPairConnection
			}
		case frame.Event == "READY":
			var identity struct {
				User struct {
					ID string `json:"id"`
				} `json:"user"`
			}
			_ = json.Unmarshal(frame.Data, &identity)
			botID = identity.User.ID
			readCtx = ctx
			cancel()
			ready()
		case frame.Op == 0 && readCtx == ctx:
			if target := gatewayPairingTarget(platform, botID, frame); target != "" {
				match(target)
			}
		}
	}
	return ctx.Err()
}

type gatewayMention struct {
	ID string `json:"id"`
}

func gatewayPairingTarget(platform, botID string, frame pairingGatewayFrame) string {
	var event struct {
		Content  string           `json:"content"`
		Channel  string           `json:"channel_id"`
		Guild    string           `json:"guild_id"`
		Group    string           `json:"group_openid"`
		Webhook  string           `json:"webhook_id"`
		Mentions []gatewayMention `json:"mentions"`
		Author   struct {
			ID     string `json:"id"`
			OpenID string `json:"user_openid"`
			Bot    bool   `json:"bot"`
		} `json:"author"`
	}
	if json.Unmarshal(frame.Data, &event) != nil || event.Author.Bot || event.Webhook != "" {
		return ""
	}
	content := strings.TrimSpace(event.Content)
	if platform == "qq" && botID != "" && (frame.Event == "GROUP_AT_MESSAGE_CREATE" ||
		frame.Event == "GROUP_MESSAGE_CREATE" || frame.Event == "AT_MESSAGE_CREATE") {
		withoutMention := strings.TrimPrefix(content, "<@!"+botID+">")
		if withoutMention == content {
			withoutMention = strings.TrimPrefix(content, "<@"+botID+">")
		}
		// Full group events may arrive for every message, so require the platform's mention.
		if frame.Event == "GROUP_MESSAGE_CREATE" && withoutMention == content &&
			!slices.ContainsFunc(event.Mentions, func(mention gatewayMention) bool { return mention.ID == botID }) {
			return ""
		}
		content = withoutMention
	} else if frame.Event == "GROUP_MESSAGE_CREATE" {
		return ""
	}
	if !isPairingText(content) {
		return ""
	}
	if platform == "discord" {
		if frame.Event == "MESSAGE_CREATE" && event.Guild == "" && event.Author.ID != botID {
			return event.Author.ID
		}
		return ""
	}
	switch frame.Event {
	case "C2C_MESSAGE_CREATE":
		if event.Author.OpenID != "" {
			return "c2c:" + event.Author.OpenID
		}
	case "GROUP_AT_MESSAGE_CREATE", "GROUP_MESSAGE_CREATE":
		if event.Group != "" {
			return "group:" + event.Group
		}
	case "AT_MESSAGE_CREATE":
		if event.Channel != "" {
			return "channel:" + event.Channel
		}
	}
	return ""
}

func (t transport) pairSlack(ctx context.Context, botToken, appToken string, ready func(), match func(string)) error {
	var identity struct {
		OK   bool   `json:"ok"`
		Team string `json:"team_id"`
	}
	if t.pairPost(ctx, "https://slack.com/api/auth.test", nil, "Bearer "+botToken, &identity) != nil || !identity.OK || identity.Team == "" {
		return errPairConnection
	}
	var socket struct {
		OK  bool   `json:"ok"`
		URL string `json:"url"`
	}
	if t.pairPost(ctx, "https://slack.com/api/apps.connections.open", nil, "Bearer "+appToken, &socket) != nil || !socket.OK {
		return errPairConnection
	}
	conn, err := t.pairSocket(ctx, socket.URL, "slack.com")
	if err != nil {
		return err
	}
	defer conn.CloseNow()
	started := time.Now().Unix()
	for ctx.Err() == nil {
		var frame struct {
			Type     string `json:"type"`
			Envelope string `json:"envelope_id"`
			Payload  struct {
				Team  string `json:"team_id"`
				Event struct {
					Type        string `json:"type"`
					Subtype     string `json:"subtype"`
					ChannelType string `json:"channel_type"`
					Bot         string `json:"bot_id"`
					User        string `json:"user"`
					Text        string `json:"text"`
					TS          string `json:"ts"`
				} `json:"event"`
			} `json:"payload"`
		}
		if wsjson.Read(ctx, conn, &frame) != nil || frame.Type == "disconnect" {
			return errPairConnection
		}
		if frame.Type == "hello" {
			ready()
		}
		if frame.Envelope != "" && wsjson.Write(ctx, conn, map[string]string{"envelope_id": frame.Envelope}) != nil {
			return errPairConnection
		}
		event := frame.Payload.Event
		seconds, _, _ := strings.Cut(event.TS, ".")
		stamp, _ := strconv.ParseInt(seconds, 10, 64)
		if frame.Type == "events_api" && frame.Payload.Team == identity.Team && event.Type == "message" &&
			event.ChannelType == "im" && event.Subtype == "" && event.Bot == "" && stamp >= started && isPairingText(event.Text) {
			match(event.User)
		}
	}
	return ctx.Err()
}

func (t transport) pairDingTalk(ctx context.Context, secret connectionSecret, ready func(), match func(string)) error {
	const topic = "/v1.0/im/bot/messages/get"
	var socket struct {
		Endpoint string `json:"endpoint"`
		Ticket   string `json:"ticket"`
	}
	if t.pairPost(ctx, "https://api.dingtalk.com/v1.0/gateway/connections/open", map[string]any{
		"clientId": secret.Address, "clientSecret": secret.Signing, "userAgent": "Fealdhord",
		"subscriptions": []map[string]string{{"type": "CALLBACK", "topic": topic}},
	}, "", &socket) != nil || socket.Ticket == "" {
		return errPairConnection
	}
	u, err := url.Parse(socket.Endpoint)
	if err != nil {
		return errPairConnection
	}
	query := u.Query()
	query.Set("ticket", socket.Ticket)
	u.RawQuery = query.Encode()
	conn, err := t.pairSocket(ctx, u.String(), "dingtalk.com")
	if err != nil {
		return err
	}
	defer conn.CloseNow()
	started := time.Now().UnixMilli()
	ready()
	for ctx.Err() == nil {
		var frame struct {
			Type    string            `json:"type"`
			Headers map[string]string `json:"headers"`
			Data    string            `json:"data"`
		}
		if wsjson.Read(ctx, conn, &frame) != nil || frame.Headers["topic"] == "disconnect" {
			return errPairConnection
		}
		data := ""
		if frame.Type == "SYSTEM" && frame.Headers["topic"] == "ping" {
			data = frame.Data
		}
		ack := map[string]any{"code": 200, "headers": map[string]string{"messageId": frame.Headers["messageId"], "contentType": "application/json"}, "message": "OK", "data": data}
		if wsjson.Write(ctx, conn, ack) != nil {
			return errPairConnection
		}
		if frame.Type == "CALLBACK" && frame.Headers["topic"] == topic {
			var event struct {
				Staff   string `json:"senderStaffId"`
				Type    string `json:"msgtype"`
				Created int64  `json:"createAt"`
				Text    struct {
					Content string `json:"content"`
				} `json:"text"`
			}
			if json.Unmarshal([]byte(frame.Data), &event) == nil && event.Created >= started &&
				event.Type == "text" && isPairingText(event.Text.Content) {
				match(event.Staff)
			}
		}
	}
	return ctx.Err()
}
