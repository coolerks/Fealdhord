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
	input := ArticleShare{ArticleID: articles[0].ID, ChannelIDs: channels, Title: true, Link: true, FullText: true}
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
