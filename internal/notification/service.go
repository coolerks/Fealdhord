// Package notification manages durable article and task delivery.
package notification

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
	"github.com/coolerks/Fealdhord/internal/schedule"
)

type Credentials interface {
	Get(string) (string, error)
	Set(string, string) error
	Delete(string) error
}

type Summarizer func(context.Context, string, []Message) (string, error)

type Service struct {
	store       *database.Store
	q           *sqlc.Queries
	credentials Credentials
	transport   transport
	summarize   Summarizer
	localNotice func(LocalNotice)
	now         func() time.Time
	wake        chan struct{}
	configMu    sync.Mutex
	wechat      wechatBinding
}

func New(store *database.Store, credentials Credentials, summarize Summarizer) *Service {
	if store == nil {
		return nil
	}
	return &Service{store: store, q: store.NotificationQueries(), credentials: credentials, summarize: summarize,
		transport: newTransport(), now: time.Now, wake: make(chan struct{}, 1)}
}

func (s *Service) Wake() {
	if s == nil {
		return
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

type LocalNotice struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

const EventName = "fealdhord:scheduled-notice"

func (s *Service) SetLocalNotice(send func(LocalNotice)) {
	if s != nil {
		s.localNotice = send
	}
}

func (s *Service) Prepare(ctx context.Context) error {
	if s == nil {
		return nil
	}
	now := s.now().UnixMilli()
	return s.q.ResumeNotificationRuntime(ctx, sqlc.ResumeNotificationRuntimeParams{ResumedAt: now, LastSeen: now})
}

func (s *Service) Start(parent context.Context, initialRefresh <-chan struct{}) func() {
	if s == nil {
		return func() {}
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	heartbeatDone := make(chan struct{})
	tasksDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		lastTick := s.now()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := s.now()
				if now.Sub(lastTick) > 30*time.Second {
					_ = s.Prepare(ctx)
				} else {
					_ = s.q.TouchNotificationRuntime(ctx, now.UnixMilli())
				}
				lastTick = now
			}
		}
	}()
	go func() {
		defer close(tasksDone)
		if err := s.q.RecoverScheduledRuns(ctx); err != nil {
			slog.Warn("恢复定时任务失败")
		}
		if initialRefresh != nil {
			select {
			case <-ctx.Done():
				return
			case <-initialRefresh:
			}
		}
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for ctx.Err() == nil {
			if err := s.recoverTasks(ctx); err != nil && ctx.Err() == nil {
				slog.Warn("恢复中断任务失败")
			}
			if err := s.processTasks(ctx); err != nil && ctx.Err() == nil {
				slog.Warn("处理定时任务失败")
			}
			s.Wake()
			select {
			case <-ctx.Done():
			case <-ticker.C:
			}
		}
	}()
	go func() {
		defer close(done)
		if err := s.q.RecoverNotificationDeliveries(ctx); err != nil {
			slog.Warn("恢复通知队列失败")
		}
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for ctx.Err() == nil {
			if err := s.processRules(ctx); err != nil && ctx.Err() == nil {
				slog.Warn("处理文章推送规则失败")
			}
			for range 30 {
				ok, err := s.deliver(ctx)
				if err != nil && ctx.Err() == nil {
					slog.Warn("处理通知投递失败")
				}
				if !ok || err != nil {
					break
				}
			}
			select {
			case <-ctx.Done():
			case <-ticker.C:
			case <-s.wake:
			}
		}
	}()
	return func() { cancel(); <-done; <-heartbeatDone; <-tasksDone }
}

func (s *Service) deliver(ctx context.Context) (bool, error) {
	row, err := s.q.NextNotificationDelivery(ctx, s.now().UnixMilli())
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	count, err := s.q.BeginNotificationDelivery(ctx, row.ID)
	if err != nil {
		return false, err
	}
	if count != 1 {
		return true, s.q.CancelPendingNotificationDelivery(ctx, row.ID)
	}
	channel, err := s.q.GetNotificationChannel(ctx, row.ChannelID)
	if err != nil {
		return false, err
	}
	result := deliveryResult{state: "cancelled", detail: "configuration_changed"}
	if channel.Enabled != 0 && channel.Verified != 0 {
		var message Message
		if err := json.Unmarshal([]byte(row.Payload), &message); err != nil {
			result = deliveryResult{state: "failed", detail: "invalid_payload"}
		} else if secret, err := s.secret(channel.CredentialRef); err != nil {
			result = deliveryResult{state: "failed", detail: "authentication"}
		} else {
			result = s.transport.send(ctx, channel.Platform, channel.Target, row.ID, secret, message, s.now())
		}
	}
	if result.state == "pending" && row.Attempts >= 4 {
		result.state = "failed"
	}
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return true, s.q.FinishNotificationDelivery(writeCtx, sqlc.FinishNotificationDeliveryParams{
		ID: row.ID, State: result.state, Detail: result.detail, DueAt: s.now().Add(result.wait).UnixMilli(),
	})
}

func (s *Service) secret(ref string) (connectionSecret, error) {
	var secret connectionSecret
	raw, err := s.credentials.Get(ref)
	if err != nil || json.Unmarshal([]byte(raw), &secret) != nil {
		return secret, errors.New("通知凭据不可用")
	}
	return secret, nil
}

func flag(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func checkWrite(rows int64, err error) error {
	if err != nil {
		return errors.New("保存通知设置失败")
	}
	if rows != 1 {
		return errors.New("配置已改变，请重新载入后编辑")
	}
	return nil
}

func parts(text string, size int) []string {
	runes := []rune(text)
	if len(runes) == 0 {
		return []string{""}
	}
	var out []string
	for len(runes) > 0 {
		n := min(len(runes), size)
		out = append(out, string(runes[:n]))
		runes = runes[n:]
	}
	return out
}

func deliveriesFor(source, key string, revision int64, targets []string, messages []Message, now time.Time) []sqlc.CreateNotificationDeliveryParams {
	var deliveries []sqlc.CreateNotificationDeliveryParams
	for _, target := range targets {
		for index, message := range messages {
			for part, body := range parts(message.Text, 700) {
				item := message
				item.Text = body
				if part > 0 {
					item.Image = ""
				}
				payload, _ := json.Marshal(item)
				sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s:%d:%d", source, key, target, index, part)))
				deliveries = append(deliveries, sqlc.CreateNotificationDeliveryParams{
					ID: fmt.Sprintf("%x", sum[:16]), ChannelID: target, SourceID: source, SourceRevision: revision,
					Payload: string(payload), DueAt: now.UnixMilli(), CreatedAt: now.UnixMilli(),
				})
			}
		}
	}
	return deliveries
}

type Channel struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Target   string `json:"target"`
	Enabled  bool   `json:"enabled"`
	Verified bool   `json:"verified"`
	Revision int64  `json:"revision"`
}

type ChannelInput struct {
	Channel
	Address     string `json:"address"`
	Signing     string `json:"signing"`
	Context     string `json:"context"`
	MediaAppID  string `json:"media_app_id"`
	MediaSecret string `json:"media_secret"`
}

func publicChannel(row sqlc.NotificationChannel) Channel {
	return Channel{ID: row.ID, Name: row.Name, Platform: row.Platform, Target: row.Target,
		Enabled: row.Enabled != 0, Verified: row.Verified != 0, Revision: row.Revision}
}

func (s *Service) ListChannels(ctx context.Context) ([]Channel, error) {
	rows, err := s.q.ListNotificationChannels(ctx)
	result := make([]Channel, 0, len(rows))
	for _, row := range rows {
		result = append(result, publicChannel(row))
	}
	return result, err
}

func (s *Service) SaveChannel(ctx context.Context, input ChannelInput) (Channel, error) {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 200 || len(input.Address) > 2048 || len(input.Signing) > 512 || len(input.Context) > 4096 || len(input.Target) > 300 || len(input.MediaAppID) > 256 || len(input.MediaSecret) > 512 || (input.MediaAppID == "") != (input.MediaSecret == "") {
		return Channel{}, errors.New("通知渠道配置无效")
	}
	input.Name = name
	input.Address = strings.TrimSpace(input.Address)
	channel := input.Channel
	old := sqlc.NotificationChannel{}
	secret := connectionSecret{Address: input.Address, Signing: input.Signing, Context: input.Context, MediaAppID: input.MediaAppID, MediaSecret: input.MediaSecret}
	if channel.ID != "" {
		var err error
		old, err = s.q.GetNotificationChannel(ctx, channel.ID)
		if err != nil || old.CredentialRef == "" || old.Revision != channel.Revision {
			return Channel{}, errors.New("渠道配置已变化")
		}
		if old.Platform != channel.Platform || old.Target != channel.Target {
			return Channel{}, errors.New("更换平台或收件目标时，请创建新渠道")
		}
		if previous, loadErr := s.secret(old.CredentialRef); input.Address == "" {
			if loadErr != nil {
				return Channel{}, loadErr
			}
			secret = previous
			if input.MediaAppID != "" {
				secret.MediaAppID, secret.MediaSecret = input.MediaAppID, input.MediaSecret
			}
		} else if _, webhook := webhooks[channel.Platform]; webhook &&
			(loadErr != nil || previous.Address != input.Address) {
			return Channel{}, errors.New("更换 Webhook 时请创建新渠道")
		}
	} else {
		channel.ID = rand.Text()
	}
	if err := checkConnection(channel.Platform, channel.Target, secret); err != nil {
		return Channel{}, err
	}
	ref := old.CredentialRef
	verified := old.Verified
	if ref == "" || input.Address != "" || input.MediaAppID != "" {
		ref = "notification:" + rand.Text()
		data, _ := json.Marshal(secret)
		if err := s.credentials.Set(ref, string(data)); err != nil {
			return Channel{}, errors.New("系统凭据存储不可用")
		}
		verified = 0
	}
	var err error
	if old.ID == "" {
		err = s.q.CreateNotificationChannel(ctx, sqlc.CreateNotificationChannelParams{
			ID: channel.ID, Name: name, Platform: channel.Platform, Target: channel.Target,
			CredentialRef: ref, Enabled: flag(channel.Enabled), Verified: verified,
		})
	} else {
		err = checkWrite(s.q.UpdateNotificationChannel(ctx, sqlc.UpdateNotificationChannelParams{
			ID: old.ID, Name: name, Platform: old.Platform, Target: old.Target, Revision: old.Revision,
			CredentialRef: ref, Enabled: flag(channel.Enabled), Verified: verified,
		}))
	}
	if err != nil {
		if ref != old.CredentialRef {
			_ = s.credentials.Delete(ref)
		}
		return Channel{}, errors.New("保存通知渠道失败")
	}
	if old.CredentialRef != "" && old.CredentialRef != ref {
		_ = s.credentials.Delete(old.CredentialRef)
	}
	row, err := s.q.GetNotificationChannel(ctx, channel.ID)
	s.Wake()
	return publicChannel(row), err
}

func (s *Service) TestChannel(ctx context.Context, id string) error {
	channel, err := s.q.GetNotificationChannel(ctx, id)
	if err != nil {
		return errors.New("找不到通知渠道")
	}
	secret, err := s.secret(channel.CredentialRef)
	if err != nil {
		return err
	}
	result := s.transport.send(ctx, channel.Platform, channel.Target, rand.Text(), secret, Message{Title: "折栈通知测试"}, s.now())
	if result.state != "accepted" {
		return fmt.Errorf("通知渠道未确认接收：%s", result.detail)
	}
	return checkWrite(s.q.UpdateNotificationChannel(ctx, sqlc.UpdateNotificationChannelParams{
		ID: id, Name: channel.Name, Platform: channel.Platform, Target: channel.Target, Revision: channel.Revision,
		CredentialRef: channel.CredentialRef, Enabled: channel.Enabled, Verified: 1,
	}))
}

func (s *Service) validTargets(ctx context.Context, ids []string) error {
	if len(ids) == 0 || len(ids) > 20 {
		return errors.New("请选择通知渠道，最多 20 个")
	}
	seen := map[string]bool{}
	for _, id := range ids {
		row, err := s.q.GetNotificationChannel(ctx, id)
		if err != nil || seen[id] || row.Enabled == 0 || row.Verified == 0 {
			return errors.New("请选择已启用且完成测试的不同通知渠道")
		}
		seen[id] = true
	}
	return nil
}

type Rule struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Mode        string   `json:"mode"`
	Categories  []string `json:"categories"`
	Channels    []string `json:"channels"`
	Body        bool     `json:"body"`
	Image       bool     `json:"image"`
	Timezone    string   `json:"timezone"`
	SendAt      string   `json:"send_at"`
	WindowStart string   `json:"window_start"`
	WindowEnd   string   `json:"window_end"`
	Enabled     bool     `json:"enabled"`
	Revision    int64    `json:"revision"`
	NextAt      int64    `json:"next_at"`
}

func ruleView(row sqlc.NotificationRule) (Rule, error) {
	var result Rule
	err := json.Unmarshal([]byte(row.Configuration), &result)
	result.ID, result.Enabled, result.Revision, result.NextAt = row.ID, row.Enabled != 0, row.Revision, row.NextAt
	return result, err
}

func (s *Service) ListRules(ctx context.Context) ([]Rule, error) {
	rows, err := s.q.ListNotificationRules(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Rule, 0, len(rows))
	for _, row := range rows {
		item, err := ruleView(row)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *Service) SaveRule(ctx context.Context, rule Rule) (Rule, error) {
	rule.Name = strings.TrimSpace(rule.Name)
	if rule.Name == "" || len(rule.Name) > 200 || len(rule.Categories) > 100 || (rule.Mode != "instant" && rule.Mode != "daily") {
		return Rule{}, errors.New("推送规则配置无效")
	}
	if rule.Enabled {
		if err := s.validTargets(ctx, rule.Channels); err != nil {
			return Rule{}, err
		}
	}
	nextAt := int64(0)
	if rule.Mode == "daily" {
		cal := schedule.Calendar{Kind: "daily", Timezone: rule.Timezone, LocalTime: rule.SendAt}
		if !cal.Valid() {
			return Rule{}, errors.New("请选择有效的发送时间和时区")
		}
		next := cal.Next(s.now())
		_, _, err := ruleWindow(rule, next)
		if err != nil {
			return Rule{}, err
		}
		nextAt = next.UnixMilli()
	}
	cursor, err := s.q.NotificationCursor(ctx)
	if err != nil {
		return Rule{}, err
	}
	if rule.ID == "" {
		rule.ID = rand.Text()
		data, _ := json.Marshal(rule)
		err = s.q.CreateNotificationRule(ctx, sqlc.CreateNotificationRuleParams{
			ID: rule.ID, Configuration: string(data), Enabled: flag(rule.Enabled), EventCursor: cursor, NextAt: nextAt,
		})
	} else {
		if previous, err := s.q.GetNotificationRule(ctx, rule.ID); err == nil && previous.Enabled != 0 && rule.Enabled {
			cursor = previous.EventCursor
		}
		data, _ := json.Marshal(rule)
		err = checkWrite(s.q.UpdateNotificationRule(ctx, sqlc.UpdateNotificationRuleParams{
			ID: rule.ID, Revision: rule.Revision, Configuration: string(data), Enabled: flag(rule.Enabled), EventCursor: cursor, NextAt: nextAt,
		}))
	}
	if err != nil {
		return Rule{}, err
	}
	s.Wake()
	row, err := s.q.GetNotificationRule(ctx, rule.ID)
	if err != nil {
		return Rule{}, err
	}
	return ruleView(row)
}

func (s *Service) categoryParents(ctx context.Context) (map[string]string, error) {
	rows, err := s.q.ListCategories(ctx)
	result := map[string]string{}
	for _, row := range rows {
		result[row.ID] = row.ParentID.String
	}
	return result, err
}

func matchesCategories(categories, expected []string, parents map[string]string) bool {
	if len(expected) == 0 {
		return true
	}
	for _, category := range categories {
		for depth := 0; category != "" && depth <= len(parents); depth++ {
			if slices.Contains(expected, category) {
				return true
			}
			category = parents[category]
		}
	}
	return false
}

func (s *Service) deleteChannel(ctx context.Context, id string, revision int64) error {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	row, err := s.q.GetNotificationChannel(ctx, id)
	if err != nil || row.Revision != revision {
		return errors.New("渠道配置已变化")
	}
	if err := checkWrite(s.q.DeleteNotificationChannel(ctx, sqlc.DeleteNotificationChannelParams{ID: id, Revision: revision})); err != nil {
		return err
	}
	_ = s.credentials.Delete(row.CredentialRef)
	s.Wake()
	return nil
}
