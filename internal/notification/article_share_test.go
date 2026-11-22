package notification

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/database"
)

func TestArticleSharingQueuesSelectedContentAndKeepsFullText(t *testing.T) {
	s, now := newTestNotification(t)
	ctx := t.Context()
	payloads := map[string][]string{}
	s.transport.client.Transport = sender(func(request *http.Request) (*http.Response, error) {
		data, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		payloads[request.URL.Host] = append(payloads[request.URL.Host], string(data))
		if request.URL.Host == "hooks.slack.com" {
			return response(200, "ok"), nil
		}
		return response(200, `{"ok":true}`), nil
	})
	channels := setupChannels(t, s)
	clear(payloads)
	body := strings.Repeat("正文", 800) + "文章末段"
	_, err := s.store.AddSubscription(ctx,
		database.SubscriptionInput{Title: "RSS", URL: "https://example.com/feed"},
		[]database.ArticleInput{{GUID: "share", Title: "文章题目", URL: "https://example.com/story", Summary: "文章摘要", Text: body}},
		database.FeedCheck{At: now.UnixMilli()},
	)
	if err != nil {
		t.Fatal(err)
	}
	articles, err := s.q.ListArticles(ctx, 1)
	if err != nil || len(articles) != 1 {
		t.Fatalf("article fixture: %v, %v", articles, err)
	}
	input := ArticleShare{ArticleID: articles[0].ID, ChannelIDs: channels, Title: true, Link: true, FullText: true, MessageLimit: 500}
	if err := s.sendArticle(ctx, input, nil); err != nil {
		t.Fatal(err)
	}
	if len(payloads) != 0 {
		t.Fatal("queueing sent network requests")
	}
	*now = now.Add(time.Second)
	for range 8 {
		if got, err := s.deliver(ctx); !got || err != nil {
			t.Fatalf("delivery = %v, %v", got, err)
		}
	}
	for _, host := range []string{"hooks.slack.com", "api.telegram.org"} {
		got := payloads[host]
		if len(got) != 4 || !strings.Contains(got[0], "文章题目") || !strings.Contains(got[0], "https://example.com/story") ||
			!strings.Contains(got[3], "文章末段") || strings.Contains(strings.Join(got, ""), "文章摘要") {
			t.Fatalf("incorrect share on %s", host)
		}
	}
	if !strings.Contains(payloads["hooks.slack.com"][0], "https://example.com/story|文章题目") ||
		!strings.Contains(payloads["api.telegram.org"][0], "href=") {
		t.Fatal("missing linked title")
	}
	clear(payloads)
	input.ChannelIDs = channels[1:]
	input.Title, input.Link, input.FullText, input.Summary = false, false, false, true
	if err := s.sendArticle(ctx, input, nil); err != nil {
		t.Fatal(err)
	}
	if got, err := s.deliver(ctx); !got || err != nil {
		t.Fatalf("delivery = %v, %v", got, err)
	}
	got := payloads["api.telegram.org"]
	if len(got) != 1 || !strings.Contains(got[0], "文章摘要") || strings.Contains(got[0], "文章题目") ||
		strings.Contains(got[0], "https://example.com/story") || len(payloads["hooks.slack.com"]) != 0 {
		t.Fatal("summary-only share included unselected data or targets")
	}
	input.ChannelIDs = []string{channels[0], "unverified"}
	if err := s.sendArticle(ctx, input, nil); err == nil {
		t.Fatal("an unavailable destination was accepted")
	}
	if got, err := s.deliver(ctx); got || err != nil {
		t.Fatalf("rejected share left a delivery = %v, %v", got, err)
	}
}

func TestArticleShareLinkOnlyUsesChannelFormat(t *testing.T) {
	const link = "https://example.com/story"
	message := Message{URL: link}
	if !strings.Contains(markdownMessage(message), "["+link+"]") || !strings.Contains(htmlLink("", link), ">"+link+"</a>") {
		t.Fatal("link-only content should display its URL")
	}
	if plainMessage(message) != link {
		t.Fatal("plain-text channels should receive the raw URL")
	}
}

func TestArticleShareBatchUsesRequestedOrderAndTruncatesSummary(t *testing.T) {
	s, now := newTestNotification(t)
	ctx := t.Context()
	var sent []string
	s.transport.client.Transport = sender(func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		sent = append(sent, string(body))
		if request.URL.Host == "hooks.slack.com" {
			return response(200, "ok"), nil
		}
		return response(200, `{"ok":true}`), nil
	})
	targets := setupChannels(t, s)
	sent = nil
	_, err := s.store.AddSubscription(ctx, database.SubscriptionInput{Title: "RSS", URL: "https://example.com/feed"}, []database.ArticleInput{
		{GUID: "a", Title: "带摘要", URL: "https://example.com/story", Summary: "第一段中文说明"},
		{GUID: "b", Title: "纯标题"},
	}, database.FeedCheck{At: now.UnixMilli()})
	if err != nil {
		t.Fatal(err)
	}
	articles, err := s.q.ListArticles(ctx, 2)
	if err != nil || len(articles) != 2 {
		t.Fatalf("expected two articles, got %d, %v", len(articles), err)
	}
	ids := make(map[string]string, len(articles))
	for _, article := range articles {
		ids[article.Title] = article.ID
	}
	input := ArticleShare{
		ArticleIDs: []string{ids["纯标题"], ids["带摘要"]}, ChannelIDs: targets[1:],
		Title: true, Summary: true, Link: true, SummaryLimit: 4,
	}
	if err := s.sendArticle(ctx, input, nil); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Second)
	if delivered, err := s.deliver(ctx); !delivered || err != nil {
		t.Fatalf("deliver = %t, %v", delivered, err)
	}
	if len(sent) != 1 || !strings.Contains(sent[0], "第一段中…") || !strings.Contains(sent[0], "href=") ||
		strings.Index(sent[0], "纯标题") >= strings.Index(sent[0], "带摘要") || strings.Contains(sent[0], "文说明") {
		t.Fatalf("unexpected combined message: %v", sent)
	}
	if delivered, err := s.deliver(ctx); delivered || err != nil {
		t.Fatalf("merged batch queued more than one delivery = %t, %v", delivered, err)
	}
	sent = nil
	input.SplitArticles = true
	if err := s.sendArticle(ctx, input, nil); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Second)
	for range 2 {
		if delivered, err := s.deliver(ctx); !delivered || err != nil {
			t.Fatalf("deliver = %t, %v", delivered, err)
		}
	}
	if len(sent) != 2 || !strings.Contains(sent[0], "纯标题") || !strings.Contains(sent[1], "第一段中…") ||
		strings.Contains(sent[1], "文说明") {
		t.Fatalf("unexpected batch messages: %v", sent)
	}
	input.ArticleIDs[1] = input.ArticleIDs[0]
	if err := s.sendArticle(ctx, input, nil); err == nil {
		t.Fatal("duplicate article accepted")
	}
	if delivered, err := s.deliver(ctx); delivered || err != nil {
		t.Fatalf("rejected batch queued a delivery = %t, %v", delivered, err)
	}
}

func TestArticleSharePacksAtArticleBoundariesAndHonorsChannelLimits(t *testing.T) {
	articles := []ArticleLink{
		{Title: "第一篇", Text: strings.Repeat("甲", 45)},
		{Title: "第二篇", Text: strings.Repeat("乙", 55)},
		{Title: "第三篇", Text: "丙"},
	}
	messages, err := packArticleShare(articles, 100, false, "wechat")
	if err != nil || len(messages) != 2 || len(messages[0].Links) != 1 || len(messages[1].Links) != 2 {
		t.Fatalf("expected to split between articles, got %v, %v", messages, err)
	}
	body := strings.Repeat("中文字😀", 260)
	messages, err = packArticleShare([]ArticleLink{{Text: body}}, 1500, false, "qq")
	if err != nil || len(messages) < 2 {
		t.Fatalf("expected automatic splitting for multi-byte text, got %d, %v", len(messages), err)
	}
	var restored strings.Builder
	for _, message := range messages {
		text := plainMessage(message)
		if len(text) > 2000 {
			t.Fatalf("message exceeds byte budget: %d", len(text))
		}
		restored.WriteString(text)
	}
	if restored.String() != body {
		t.Fatal("splitting lost or reordered text")
	}
}
