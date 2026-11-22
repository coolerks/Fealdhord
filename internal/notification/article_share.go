package notification

import (
	"context"
	"crypto/rand"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
	"github.com/coolerks/Fealdhord/internal/reader"
	"github.com/coolerks/Fealdhord/internal/settings"
)

type ArticleShare struct {
	ArticleID     string   `json:"article_id,omitempty"`
	ArticleIDs    []string `json:"article_ids,omitempty"`
	ChannelIDs    []string `json:"channel_ids"`
	Title         bool     `json:"title"`
	Summary       bool     `json:"summary"`
	Link          bool     `json:"link"`
	FullText      bool     `json:"full_text"`
	SummaryLimit  int      `json:"summary_limit,omitempty"`
	SplitArticles bool     `json:"split_articles,omitempty"`
	MessageLimit  int      `json:"message_limit,omitempty"`
}

const (
	minArticleMessageLimit     = 100
	defaultArticleMessageLimit = 1500
	maxArticleMessageLimit     = 2000
)

type articleExtractor interface {
	Extract(context.Context, reader.Request) reader.Result
}

func (s *Service) sendArticle(ctx context.Context, input ArticleShare, extractor articleExtractor) error {
	ids := input.ArticleIDs
	if input.ArticleID != "" && len(ids) == 0 {
		ids = []string{input.ArticleID}
	}
	if len(ids) == 0 || len(ids) > settings.MaxArticleShareCount || !(input.Title || input.Summary || input.Link || input.FullText) {
		return errors.New("请选择文章和要发送的内容")
	}
	if input.SummaryLimit == 0 {
		input.SummaryLimit = settings.DefaultArticleShareSummaryLimit
	}
	if input.SummaryLimit < 1 || input.SummaryLimit > settings.MaxArticleShareSummaryLimit {
		return errors.New("摘要字数超出允许范围")
	}
	if input.MessageLimit == 0 {
		input.MessageLimit = defaultArticleMessageLimit
	}
	if input.MessageLimit < minArticleMessageLimit || input.MessageLimit > maxArticleMessageLimit {
		return errors.New("单条消息字数须在 100 至 2000 之间")
	}
	channels, err := s.articleShareChannels(ctx, input.ChannelIDs)
	if err != nil {
		return err
	}
	articles := make([]ArticleLink, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id == "" || len(id) > 256 || seen[id] {
			return errors.New("请选择不同的有效文章")
		}
		seen[id] = true
		message, err := s.articleShareMessage(ctx, id, input, extractor)
		if err != nil {
			return err
		}
		articles = append(articles, message)
	}
	now, key := s.now(), rand.Text()
	var deliveries []sqlc.CreateNotificationDeliveryParams
	for _, channel := range channels {
		fragments, err := packArticleShare(articles, input.MessageLimit, input.SplitArticles, channel.Platform)
		if err != nil {
			return err
		}
		for index, part := range fragments {
			deliveries = append(deliveries, deliveriesFor("manual", key+":"+strconv.Itoa(index), 0, []string{channel.ID},
				[]Message{part}, now.Add(time.Duration(index)*time.Millisecond))...)
		}
	}
	s.configMu.Lock()
	defer s.configMu.Unlock()
	// Recheck after extraction, since the user may have changed a channel while it ran.
	if _, err := s.articleShareChannels(ctx, input.ChannelIDs); err != nil {
		return err
	}
	if err := s.store.EnqueueNotification(ctx, deliveries); err != nil {
		return errors.New("无法将文章加入通知队列")
	}
	s.Wake()
	return nil
}

func (s *Service) articleShareMessage(ctx context.Context, id string, input ArticleShare, extractor articleExtractor) (ArticleLink, error) {
	article, err := s.q.GetArticle(ctx, id)
	if err != nil {
		return ArticleLink{}, errors.New("无法读取选中的文章")
	}
	var message ArticleLink
	if input.Title {
		message.Title = strings.TrimSpace(article.Title)
	}
	if input.Link {
		message.URL = linkURL(article.Url)
	}
	var sections []string
	if input.Summary {
		if summary := strings.TrimSpace(article.Summary); summary != "" {
			runes := []rune(summary)
			if len(runes) > input.SummaryLimit {
				summary = string(runes[:input.SummaryLimit]) + "…"
			}
			sections = append(sections, summary)
		}
	}
	if input.FullText {
		fullText, err := s.articleShareFullText(ctx, article, extractor)
		if err != nil {
			return ArticleLink{}, err
		}
		if len(sections) == 0 || sections[0] != fullText {
			sections = append(sections, fullText)
		}
	}
	message.Text = strings.Join(sections, "\n\n")
	if message.Title == "" && message.URL == "" && message.Text == "" {
		return ArticleLink{}, errors.New("有文章没有可发送的所选内容")
	}
	return message, nil
}

func (s *Service) articleShareChannels(ctx context.Context, ids []string) ([]sqlc.NotificationChannel, error) {
	rows, err := s.q.ListNotificationChannels(ctx)
	if err != nil {
		return nil, errors.New("无法读取通知渠道")
	}
	enabled := make(map[string]sqlc.NotificationChannel, len(rows))
	for _, row := range rows {
		if row.Enabled != 0 && row.Verified != 0 {
			enabled[row.ID] = row
		}
	}
	if len(ids) == 0 {
		return nil, errors.New("请选择通知渠道")
	}
	channels := make([]sqlc.NotificationChannel, 0, len(ids))
	for _, id := range ids {
		channel, exists := enabled[id]
		if !exists {
			return nil, errors.New("请选择不同的、已启用且通过测试的通知渠道")
		}
		channels = append(channels, channel)
		delete(enabled, id)
	}
	return channels, nil
}

func packArticleShare(articles []ArticleLink, limit int, split bool, platform string) ([]Message, error) {
	var messages []Message
	var group []ArticleLink
	flush := func() {
		if len(group) > 0 {
			messages = append(messages, Message{Links: group})
			group = nil
		}
	}
	fits := func(entries []ArticleLink) bool {
		return articleShareFits(Message{Links: entries}, limit, platform)
	}
	for _, article := range articles {
		if split || !fits(append(group, article)) {
			flush()
		}
		if fits([]ArticleLink{article}) {
			group = append(group, article)
			continue
		}
		// Preserve the heading and link together unless their combined size exceeds the limit.
		heading := ArticleLink{Title: article.Title, URL: article.URL}
		if !fits([]ArticleLink{heading}) {
			heading.Title = ""
			if !fits([]ArticleLink{heading}) {
				return nil, errors.New("有原文链接超过单条消息长度限制，请提高字数上限或取消勾选链接")
			}
			article.Text = strings.TrimSpace(article.Title + "\n" + article.Text)
		}
		body := []rune(article.Text)
		for len(body) != 0 {
			n := sort.Search(min(len(body), limit)+1, func(n int) bool {
				candidate := heading
				candidate.Text = strings.TrimSpace(string(body[:n]))
				return !fits([]ArticleLink{candidate})
			}) - 1
			heading.Text = strings.TrimSpace(string(body[:n]))
			if heading != (ArticleLink{}) {
				group = []ArticleLink{heading}
			}
			body = body[n:]
			heading = ArticleLink{}
			if len(body) > 0 {
				flush()
			}
		}
	}
	flush()
	return messages, nil
}

func articleShareFits(message Message, limit int, platform string) bool {
	plain := plainMessage(message)
	if utf8.RuneCountInString(plain) > limit {
		return false
	}
	switch platform {
	case "qq":
		return len(plain) <= 2000
	case "wecom":
		return len(markdownMessage(message)) <= 2048
	case "discord":
		return len(utf16.Encode([]rune(markdownMessage(message)))) <= 4096
	case "slack":
		blocks := 0
		for _, article := range message.Links {
			heading := slackArticleHeading(article)
			if len(utf16.Encode([]rune(heading))) > 3000 {
				return false
			}
			if heading != "" {
				blocks++
			}
			if article.Text != "" {
				blocks++
			}
		}
		return blocks <= 50
	}
	return true
}

func (s *Service) articleShareFullText(ctx context.Context, article sqlc.Article, extractor articleExtractor) (string, error) {
	cached, err := s.store.ReaderDocument(ctx, article.ID)
	if err == nil && strings.TrimSpace(cached.Text) != "" {
		return strings.TrimSpace(cached.Text), nil
	}
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return "", errors.New("无法读取已保存的正文")
	}
	if text := strings.TrimSpace(article.ContentText); text != "" && text != strings.TrimSpace(article.Summary) {
		return text, nil
	}
	if extractor != nil && article.Url != "" {
		result := extractor.Extract(ctx, reader.Request{ArticleID: article.ID, Mode: "reader"})
		if result.OK && result.Data != nil && strings.TrimSpace(result.Data.Text) != "" {
			return strings.TrimSpace(result.Data.Text), nil
		}
	}
	return "", errors.New("无法获取文章全文，请重试或改选其他内容")
}
