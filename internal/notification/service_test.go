package notification

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
	"github.com/coolerks/Fealdhord/internal/schedule"
)

type testCredentials map[string]string

func (s testCredentials) Get(ref string) (string, error) { return s[ref], nil }
func (s testCredentials) Set(ref, value string) error    { s[ref] = value; return nil }
func (s testCredentials) Delete(ref string) error        { delete(s, ref); return nil }

type sender func(*http.Request) (*http.Response, error)

func (f sender) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newTestNotification(t *testing.T) (*Service, *time.Time) {
	t.Helper()
	store, err := database.Open(t.Context(), filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	at := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)
	service := New(store, testCredentials{}, nil)
	service.now = func() time.Time { return at }
	return service, &at
}

func response(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func setupChannels(t *testing.T, s *Service) []string {
	t.Helper()
	var ids []string
	for _, input := range []ChannelInput{
		{Channel: Channel{Name: "Slack", Platform: "slack", Enabled: true}, Address: "https://hooks.slack.com/services/T/B/secret"},
		{Channel: Channel{Name: "Telegram", Platform: "telegram", Target: "1234", Enabled: true}, Address: "1234:secret"},
	} {
		channel, err := s.SaveChannel(t.Context(), input)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.TestChannel(t.Context(), channel.ID); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, channel.ID)
	}
	return ids
}

func TestNewArticleFanoutPersistsAndDoesNotRepeat(t *testing.T) {
	s, at := newTestNotification(t)
	ctx := t.Context()
	rejectSlack := false
	var payloads []string
	s.transport.client.Transport = sender(func(r *http.Request) (*http.Response, error) {
		data, _ := io.ReadAll(r.Body)
		payloads = append(payloads, string(data))
		if r.URL.Hostname() == "hooks.slack.com" {
			if rejectSlack {
				return response(403, "denied"), nil
			}
			return response(200, "ok"), nil
		}
		return response(200, `{"ok":true}`), nil
	})
	targets := setupChannels(t, s)
	rejectSlack = true
	if _, err := s.SaveRule(ctx, Rule{Name: "逐条", Mode: "instant", Channels: targets, Enabled: true, Body: true}); err != nil {
		t.Fatal(err)
	}
	feed := database.SubscriptionInput{Title: "RSS", URL: "https://example.com/feed"}
	id, err := s.store.AddSubscription(ctx, feed, []database.ArticleInput{{GUID: "old", Title: "初次订阅历史"}}, database.FeedCheck{At: at.UnixMilli()})
	if err != nil {
		t.Fatal(err)
	}
	fresh := database.ArticleInput{GUID: "fresh", Title: "新文章", URL: "https://example.com/fresh", Text: "正文"}
	*at = at.Add(time.Minute)
	if _, err := s.store.CommitFeed(ctx, id, feed.URL, []database.ArticleInput{fresh}, database.FeedCheck{At: at.UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	fresh.Title = "更新标题"
	if _, err := s.store.CommitFeed(ctx, id, feed.URL, []database.ArticleInput{fresh}, database.FeedCheck{At: at.UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	if err := s.processRules(ctx); err != nil {
		t.Fatal(err)
	}
	// 新服务实例只读持久队列。重复扫描不重建已经记录的投递。
	reopened := New(s.store, s.credentials, nil)
	reopened.now, reopened.transport = s.now, s.transport
	if err := reopened.processRules(ctx); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if delivered, err := reopened.deliver(ctx); !delivered || err != nil {
			t.Fatalf("deliver = %t, %v", delivered, err)
		}
	}
	if delivered, err := reopened.deliver(ctx); delivered || err != nil {
		t.Fatalf("extra delivery = %t, %v", delivered, err)
	}
	rows, err := s.q.ListNotificationDeliveries(ctx)
	if err != nil || len(rows) != 2 {
		t.Fatalf("deliveries: %+v %v", rows, err)
	}
	states := map[string]string{}
	for _, row := range rows {
		states[row.ChannelID] = row.State
	}
	if states[targets[0]] != "failed" || states[targets[1]] != "accepted" {
		t.Fatal(states)
	}
	if len(payloads) != 4 || !strings.Contains(strings.Join(payloads, ""), "正文") || strings.Contains(strings.Join(payloads, ""), "初次订阅历史") {
		t.Fatal(payloads)
	}
}

func TestScheduledRunRecoveryAndDigestBoundaries(t *testing.T) {
	s, at := newTestNotification(t)
	ctx := t.Context()
	var notices []LocalNotice
	s.SetLocalNotice(func(n LocalNotice) { notices = append(notices, n) })
	task, err := s.SaveTask(ctx, Task{Name: "提醒", Text: "喝水", Action: "reminder", Enabled: true, Calendar: schedule.Calendar{
		Kind: "once", Timezone: "UTC", OnceAt: at.Add(time.Minute).Format(time.RFC3339),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.ClaimNotificationTask(ctx, sqlc.ClaimScheduledTaskParams{
		ID: task.ID, Revision: task.Revision, NextAt_2: task.NextAt,
	}); err != nil {
		t.Fatal(err)
	}
	if err = s.q.RecoverScheduledRuns(ctx); err != nil {
		t.Fatal(err)
	}
	*at = at.Add(4 * time.Hour)
	if err = s.recoverTasks(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.processTasks(ctx); err != nil {
		t.Fatal(err)
	}
	rows, err := s.q.ListScheduledRuns(ctx, task.ID)
	if err != nil || len(rows) != 1 || rows[0].State != "complete" || len(notices) != 1 {
		t.Fatalf("runs=%+v notices=%+v err=%v", rows, notices, err)
	}
	queued, err := s.q.ListPendingLocalNotices(ctx)
	if err != nil || len(queued) != 1 || queued[0].Title != "提醒" {
		t.Fatalf("pending local notice: %+v %v", queued, err)
	}
	if err := s.q.AcknowledgeLocalNotice(ctx, sqlc.AcknowledgeLocalNoticeParams{TaskID: queued[0].TaskID, Occurrence: queued[0].Occurrence}); err != nil {
		t.Fatal(err)
	}
	if queue, err := s.q.ListPendingLocalNotices(ctx); err != nil || len(queue) != 0 {
		t.Fatalf("local notice was not acknowledged: %+v %v", queue, err)
	}
	rule := Rule{Timezone: "UTC", SendAt: "08:00", WindowStart: "20:00", WindowEnd: "07:00"}
	cal := schedule.Calendar{Timezone: "UTC", Kind: "daily", LocalTime: "08:00"}
	beforeReopen := time.Date(2026, 11, 20, 21, 0, 0, 0, time.UTC)
	if err = s.q.TouchNotificationRuntime(ctx, beforeReopen.UnixMilli()); err != nil {
		t.Fatal(err)
	}
	if err = s.Prepare(context.Background()); err != nil {
		t.Fatal(err)
	}
	published := beforeReopen.Add(time.Hour)
	atWindow := digestOccurrence(rule, cal, sqlc.ListNotificationEventsRow{
		DiscoveredAt: at.UnixMilli(), OfflineSince: beforeReopen.UnixMilli(), ResumedAt: at.UnixMilli(),
		PublishedAt: sql.NullInt64{Int64: published.UnixMilli(), Valid: true},
	}, true)
	if got := atWindow.UTC().Format(time.RFC3339); got != "2026-11-21T08:00:00Z" {
		t.Fatalf("补发窗口=%s", got)
	}
}

func TestUnknownSubmissionIsNotRetriedAfterRestart(t *testing.T) {
	s, _ := newTestNotification(t)
	ctx := t.Context()
	s.transport.client.Transport = sender(func(r *http.Request) (*http.Response, error) {
		if r.URL.Hostname() == "hooks.slack.com" {
			return response(200, "ok"), nil
		}
		return response(200, `{"ok":true}`), nil
	})
	targets := setupChannels(t, s)
	if err := s.Send(ctx, targets[:1], Message{Title: "测试"}); err != nil {
		t.Fatal(err)
	}
	pending, err := s.q.NextNotificationDelivery(ctx, s.now().UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.q.BeginNotificationDelivery(ctx, pending.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.q.RecoverNotificationDeliveries(ctx); err != nil {
		t.Fatal(err)
	}
	if delivered, err := s.deliver(ctx); delivered || err != nil {
		t.Fatalf("%t %v", delivered, err)
	}
	history, err := s.q.ListNotificationDeliveries(ctx)
	if err != nil || len(history) != 1 || history[0].State != "unknown" {
		t.Fatalf("%+v %v", history, err)
	}
}

func TestLateCategoryMatchDoesNotRepeatDelivery(t *testing.T) {
	s, at := newTestNotification(t)
	ctx := t.Context()
	s.transport.client.Transport = sender(func(r *http.Request) (*http.Response, error) {
		if r.URL.Hostname() == "hooks.slack.com" {
			return response(200, "ok"), nil
		}
		return response(200, `{"ok":true}`), nil
	})
	targets := setupChannels(t, s)
	parent, err := s.store.CreateCategory(ctx, "科技", "")
	if err != nil {
		t.Fatal(err)
	}
	child, err := s.store.CreateCategory(ctx, "开发", parent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveRule(ctx, Rule{Name: "科技", Mode: "instant", Categories: []string{parent, child}, Channels: targets[:1], Enabled: true}); err != nil {
		t.Fatal(err)
	}
	feed := database.SubscriptionInput{Title: "RSS", URL: "https://example.com/feed"}
	id, err := s.store.AddSubscription(ctx, feed, nil, database.FeedCheck{At: at.UnixMilli()})
	if err != nil {
		t.Fatal(err)
	}
	*at = at.Add(time.Minute)
	if _, err := s.store.CommitFeed(ctx, id, feed.URL, []database.ArticleInput{{GUID: "new", Title: "新文章"}}, database.FeedCheck{At: at.UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	if err := s.processRules(ctx); err != nil {
		t.Fatal(err)
	}
	if rows, err := s.q.ListNotificationDeliveries(ctx); err != nil || len(rows) != 0 {
		t.Fatalf("unexpected initial deliveries: %+v, %v", rows, err)
	}
	if err := s.store.SetFeedCategories(ctx, id, []string{child}, true); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.processRules(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if rows, err := s.q.ListNotificationDeliveries(ctx); err != nil || len(rows) != 1 {
		t.Fatalf("expected one late match: %+v, %v", rows, err)
	}
}
