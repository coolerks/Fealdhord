package notification

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"sync"
	"time"

	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

const (
	verificationLifetime = 5 * time.Minute
	verificationCooldown = 30 * time.Second
	verificationAttempts = 5
)

var verificationDigits = regexp.MustCompile(`^[0-9]{4}$`)

type recipientChallenge struct {
	channelID string
	revision  int64
	code      string
	sentAt    time.Time
	attempts  int
	sending   bool
}

type recipientVerifications struct {
	mu      sync.Mutex
	pending map[string]*recipientChallenge
}

func (a *API) SendRecipientVerification(ctx context.Context, channelID string, revision int64, recipientID, pairingID string, resend bool) error {
	_, err := available(a, func(s *Service) (bool, error) {
		return true, s.sendRecipientVerification(ctx, channelID, revision, recipientID, pairingID, resend)
	})
	return err
}

func (a *API) ConfirmRecipientVerification(ctx context.Context, channelID string, revision int64, recipientID, code string) error {
	_, err := available(a, func(s *Service) (bool, error) {
		return true, s.confirmRecipientVerification(ctx, channelID, revision, recipientID, code)
	})
	return err
}

func (s *Service) sendRecipientVerification(ctx context.Context, channelID string, revision int64, recipientID, pairingID string, resend bool) error {
	channel, err := s.q.GetNotificationChannel(ctx, channelID)
	if err != nil || channel.CredentialRef == "" || channel.Revision != revision {
		return errors.New("渠道配置已变化，请重新打开设置")
	}
	recipient, err := s.q.GetNotificationRecipient(ctx, recipientID)
	if err != nil || recipient.ChannelID != channelID || recipient.Active == 0 {
		return errors.New("找不到收件目标")
	}
	if recipient.Verified != 0 {
		return nil
	}
	challenge, err := s.verifications.begin(s.now(), channelID, revision, recipientID, resend)
	if err != nil || challenge == nil {
		return err
	}
	message := Message{Title: "配对成功，校验码：" + challenge.code}
	if channel.Platform == "qq" && pairingID != "" {
		if secret, err := s.secret(channel.CredentialRef); err == nil {
			message.qqReplyID, message.qqReplySequence = s.pairing.nextQQReply(pairingID, secret.Address, channel.Mode, recipient.Target, s.now())
		}
	}
	result := s.sendRecipient(ctx, channel, recipient.Target, rand.Text(), message)

	s.verifications.mu.Lock()
	defer s.verifications.mu.Unlock()
	// A transport may have delivered a message even when its response was lost.
	challenge.sending = false
	if result.state == "accepted" || result.state == "unknown" {
		return nil
	}
	challenge.code = ""
	return errors.New("校验码发送失败，请确认渠道配置后重发")
}

func (v *recipientVerifications) begin(now time.Time, channelID string, revision int64, recipientID string, resend bool) (*recipientChallenge, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.pending == nil {
		v.pending = make(map[string]*recipientChallenge)
	}
	for id, challenge := range v.pending {
		if !challenge.sending && now.Sub(challenge.sentAt) >= verificationLifetime {
			delete(v.pending, id)
		}
	}
	if previous := v.pending[recipientID]; previous != nil && previous.revision == revision {
		if previous.sending {
			return nil, errors.New("校验码正在发送，请稍候")
		}
		if !resend && previous.code != "" && previous.attempts < verificationAttempts {
			return nil, nil
		}
		if now.Sub(previous.sentAt) < verificationCooldown {
			return nil, errors.New("两次发送需间隔 30 秒，请稍后重发")
		}
	}
	if len(v.pending) >= 500 && v.pending[recipientID] == nil {
		return nil, errors.New("待验证目标过多，请稍后重发")
	}
	// Keep active codes distinct across targets in the same channel.
	used := make(map[string]bool)
	for _, challenge := range v.pending {
		if challenge.channelID == channelID {
			used[challenge.code] = true
		}
	}
	number, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return nil, errors.New("无法生成校验码")
	}
	n := number.Int64()
	code := fmt.Sprintf("%04d", n)
	for used[code] {
		n = (n + 1) % 10000
		code = fmt.Sprintf("%04d", n)
	}

	challenge := &recipientChallenge{channelID: channelID, revision: revision, code: code, sentAt: now, sending: true}
	v.pending[recipientID] = challenge
	return challenge, nil
}

func (s *Service) confirmRecipientVerification(ctx context.Context, channelID string, revision int64, recipientID, code string) error {
	if !verificationDigits.MatchString(code) {
		return errors.New("请输入四位数字校验码")
	}
	v := &s.verifications
	v.mu.Lock()
	defer v.mu.Unlock()
	challenge := v.pending[recipientID]
	if challenge == nil || challenge.channelID != channelID || challenge.revision != revision ||
		challenge.code == "" || s.now().Sub(challenge.sentAt) >= verificationLifetime {
		return errors.New("校验码已失效，请重发")
	}
	if challenge.sending {
		return errors.New("校验码正在发送，请稍候")
	}
	if challenge.attempts >= verificationAttempts {
		return errors.New("错误次数已达上限，请重发")
	}
	if subtle.ConstantTimeCompare([]byte(challenge.code), []byte(code)) != 1 {
		challenge.attempts++
		if challenge.attempts == verificationAttempts {
			return errors.New("错误次数已达上限，请重发")
		}
		return errors.New("校验码不正确")
	}
	if err := s.store.VerifyNotificationRecipient(ctx, channelID, sqlc.VerifyNotificationRecipientParams{ID: recipientID, Revision: revision}); err != nil {
		return errors.New("渠道配置已变化，请重新打开设置")
	}
	delete(v.pending, recipientID)
	s.Wake()
	return nil
}
