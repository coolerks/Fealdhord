package notification

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

var telegramTarget = regexp.MustCompile(`^(-?[0-9]{1,20}|@[a-zA-Z0-9_]+)$`)

func isWebhook(platform, mode string) bool {
	_, webhook := webhooks[platform]
	return webhook && mode == ""
}

func (s *Service) checkRecipients(ctx context.Context, channel Channel, secret connectionSecret, targets []string) ([]string, error) {
	if channel.Mode != "" && (channel.Mode != "app" || !isWebhook(channel.Platform, "")) {
		return nil, errors.New("不支持此渠道接入方式")
	}
	if isWebhook(channel.Platform, channel.Mode) {
		if len(targets) > 1 || (len(targets) == 1 && targets[0] != "") {
			return nil, errors.New("群 Webhook 固定发往绑定会话；个人收件请选择应用机器人")
		}
		return []string{""}, checkConnection(channel.Platform, "", secret)
	}
	if channel.Mode == "app" && !validAppCredentials(channel.Platform, secret) {
		return nil, errors.New("请检查应用机器人凭据")
	}
	if len(targets) > 50 {
		return nil, errors.New("每个连接最多添加 50 个收件目标")
	}
	result := make([]string, 0, len(targets))
	seen := map[string]bool{}
	for _, raw := range targets {
		target := strings.TrimSpace(raw)
		if target == "" || len(target) > 300 {
			return nil, errors.New("收件目标无效")
		}
		if channel.Mode == "app" && (channel.Platform == "feishu" || channel.Platform == "lark") {
			target = strings.TrimPrefix(target, "open_id:")
		}
		if seen[target] {
			continue
		}
		switch {
		case channel.Mode == "app":
			if err := checkAppRecipient(channel.Platform, target, secret); err != nil {
				return nil, err
			}
		case channel.Platform == "wechat":
			if target == channel.Target {
				if err := checkConnection(channel.Platform, target, secret); err != nil {
					return nil, err
				}
			} else if _, err := s.boundWeChat(ctx, channel.ID, target); err != nil {
				return nil, err
			}
		default:
			if channel.Platform == "telegram" && target != channel.Target && !telegramTarget.MatchString(target) {
				return nil, errors.New("请填写数字 Chat ID 或 @频道用户名")
			}
			if err := checkConnection(channel.Platform, target, secret); err != nil {
				return nil, err
			}
		}
		seen[target] = true
		result = append(result, target)
	}
	if secret.Address == "" {
		return nil, errors.New("请填写渠道凭据")
	}
	return result, nil
}

func (s *Service) boundWeChat(ctx context.Context, current, target string) (sqlc.NotificationChannel, error) {
	id, ok := strings.CutPrefix(target, "binding:")
	if !ok || id == "" || id == current {
		return sqlc.NotificationChannel{}, errors.New("请选择已绑定的微信私聊")
	}
	bound, err := s.q.GetNotificationChannel(ctx, id)
	if err != nil || bound.Platform != "wechat" || bound.Target == "" || bound.CredentialRef == "" || bound.Enabled == 0 {
		return sqlc.NotificationChannel{}, errors.New("微信私聊绑定不可用")
	}
	return bound, nil
}

func (s *Service) sendRecipient(ctx context.Context, channel sqlc.NotificationChannel, target, id string, message Message) deliveryResult {
	if channel.Platform == "wechat" && target != channel.Target {
		bound, err := s.boundWeChat(ctx, channel.ID, target)
		if err != nil {
			return deliveryResult{state: "cancelled", detail: "wechat_binding_removed"}
		}
		channel, target = bound, bound.Target
	}
	secret, err := s.secret(channel.CredentialRef)
	if err != nil {
		return deliveryResult{state: "failed", detail: "authentication"}
	}
	if channel.Mode == "app" {
		return s.transport.sendApp(ctx, channel.Platform, target, id, secret, message)
	}
	return s.transport.send(ctx, channel.Platform, target, id, secret, message, s.now())
}
