package notification

import (
	"context"
	"crypto/rand"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"
)

const pairingTimeout = 3 * time.Minute

type PairingStatus struct {
	ID              string   `json:"id"`
	State           string   `json:"state"`
	Targets         []string `json:"targets"`
	Detail          string   `json:"detail,omitempty"`
	BotName         string   `json:"bot_name,omitempty"`
	ReceivedMessage bool     `json:"received_message,omitempty"`
}

type recipientPairing struct {
	mu     sync.Mutex
	status PairingStatus
	cancel context.CancelFunc
}

func (a *API) StartRecipientPairing(ctx context.Context, input ChannelInput, slackAppToken string) (PairingStatus, error) {
	return available(a, func(s *Service) (PairingStatus, error) {
		return s.startRecipientPairing(ctx, input, strings.TrimSpace(slackAppToken))
	})
}

func (a *API) RecipientPairingStatus(id string) PairingStatus {
	if a.service == nil {
		return PairingStatus{ID: id, State: "expired"}
	}
	pair := &a.service.pairing
	pair.mu.Lock()
	defer pair.mu.Unlock()
	if id == "" || pair.status.ID != id {
		return PairingStatus{ID: id, State: "expired"}
	}
	status := pair.status
	status.Targets = slices.Clone(status.Targets)
	return status
}

func (a *API) CancelRecipientPairing(id string) {
	if a.service != nil {
		a.service.pairing.stop(id)
	}
}

func (p *recipientPairing) stop(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if id != "" && id != p.status.ID {
		return
	}
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	p.status = PairingStatus{}
}

func (s *Service) startRecipientPairing(ctx context.Context, input ChannelInput, slackToken string) (PairingStatus, error) {
	if err := ctx.Err(); err != nil {
		return PairingStatus{}, err
	}
	if len(input.Address) > 2048 || len(input.Signing) > 512 || len(input.Context) > 4096 {
		return PairingStatus{}, errors.New("渠道凭据格式无效")
	}
	secret := connectionSecret{Address: strings.TrimSpace(input.Address), Signing: input.Signing, Context: input.Context}
	if isWebhook(input.Platform, input.Mode) || input.Platform == "wechat" || input.Platform == "wecom" {
		return PairingStatus{}, errors.New("此接入方式无法启动消息配对")
	}
	if input.ID != "" {
		channel, err := s.q.GetNotificationChannel(ctx, input.ID)
		if err != nil || channel.Revision != input.Revision || channel.Platform != input.Platform || channel.Mode != input.Mode {
			return PairingStatus{}, errors.New("渠道配置已变化")
		}
		if secret.Address == "" {
			secret, err = s.secret(channel.CredentialRef)
			if err != nil {
				return PairingStatus{}, err
			}
		}
	}
	switch input.Platform {
	case "telegram":
		if err := checkConnection("telegram", "1", secret); err != nil {
			return PairingStatus{}, errors.New("请填写有效的 Telegram Bot Token")
		}
	case "qq":
		if !validQQMode(input.Mode) {
			return PairingStatus{}, errors.New("QQ 接入环境无效")
		}
		if err := checkConnection("qq", "c2c:1", secret); err != nil {
			return PairingStatus{}, errors.New("请填写有效的 QQ AppID 和 AppSecret")
		}
	case "discord", "slack", "dingtalk", "feishu", "lark":
		if input.Mode != "app" || !validAppCredentials(input.Platform, secret) {
			return PairingStatus{}, errors.New("请填写有效的应用机器人凭据")
		}
	default:
		return PairingStatus{}, errors.New("此平台不支持消息配对")
	}
	if input.Platform == "slack" && (!strings.HasPrefix(slackToken, "xapp-") || len(slackToken) > 512 || !tokenPattern.MatchString(slackToken)) {
		return PairingStatus{}, errors.New("请填写 Slack App-Level Token")
	}
	p := &s.pairing
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		return PairingStatus{}, errors.New("已有渠道正在配对")
	}
	listen, cancel := context.WithTimeout(context.Background(), pairingTimeout)
	status := PairingStatus{ID: rand.Text(), State: "connecting"}
	p.status, p.cancel = status, cancel
	platform := input.Platform
	go func() {
		defer cancel()
		update := func(apply func(*PairingStatus)) {
			p.mu.Lock()
			defer p.mu.Unlock()
			if p.status.ID == status.ID {
				apply(&p.status)
			}
		}
		ready := func() { update(func(value *PairingStatus) { value.State = "listening" }) }
		match := func(target string) {
			if _, err := s.checkRecipients(listen, Channel{Platform: platform, Mode: input.Mode}, secret, []string{target}); err != nil {
				return
			}
			update(func(value *PairingStatus) {
				if len(value.Targets) < 50 && !slices.Contains(value.Targets, target) {
					value.Targets = append(value.Targets, target)
				}
			})
		}
		observeQQ := func(event qqPairingObservation) {
			update(func(value *PairingStatus) {
				if event.botName != "" {
					value.BotName = event.botName
				}
				value.ReceivedMessage = value.ReceivedMessage || event.receivedMessage
			})
		}
		err := s.transport.listenPairing(listen, platform, input.Mode, secret, slackToken, ready, match, observeQQ)
		update(func(value *PairingStatus) {
			p.cancel = nil
			if listen.Err() == context.DeadlineExceeded {
				value.State = "expired"
			} else {
				value.State = "failed"
				value.Detail = "配对连接已断开，请检查平台的消息接收配置后重试"
				if err != nil {
					value.Detail = err.Error()
				}
			}
		})
	}()
	return status, nil
}

func isPairingText(text string) bool { return strings.TrimSpace(text) == "配对" }
