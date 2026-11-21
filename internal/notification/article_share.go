package notification

import (
	"context"
	"crypto/rand"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
	"github.com/coolerks/Fealdhord/internal/reader"
)

type ArticleShare struct {
	ArticleID  string   `json:"article_id"`
	ChannelIDs []string `json:"channel_ids"`
	Title      bool     `json:"title"`
	Summary    bool     `json:"summary"`
	Link       bool     `json:"link"`
	FullText   bool     `json:"full_text"`
}

type articleExtractor interface {
	Extract(context.Context, reader.Request) reader.Result
}

func (s *Service) sendArticle(ctx context.Context, input ArticleShare, extractor articleExtractor) error {
	if input.ArticleID == "" || len(input.ArticleID) > 256 || !(input.Title || input.Summary || input.Link || input.FullText) {
		return errors.New("请选择文章和要发送的内容")
	}
	if err := s.validArticleShareChannels(ctx, input.ChannelIDs); err != nil {
		return err
	}
	article, err := s.q.GetArticle(ctx, input.ArticleID)
	if err != nil {
		return errors.New("无法读取这篇文章")
	}
	var message Message
	if input.Title {
		message.Title = strings.TrimSpace(article.Title)
	}
	if input.Link {
		message.URL = linkURL(article.Url)
		if message.URL == "" {
			return errors.New("这篇文章没有可发送的原文链接")
		}
	}
	var sections []string
	if input.Summary {
		if summary := strings.TrimSpace(article.Summary); summary != "" {
			sections = append(sections, summary)
		} else {
			return errors.New("这篇文章没有摘要")
		}
	}
	if input.FullText {
		fullText, err := s.articleShareFullText(ctx, article, extractor)
		if err != nil {
			return err
		}
		if len(sections) == 0 || sections[0] != fullText {
			sections = append(sections, fullText)
		}
	}
	message.Text = strings.Join(sections, "\n\n")
	if message.Title == "" && message.URL == "" && message.Text == "" {
		return errors.New("这篇文章没有所选内容")
	}

	now, key := s.now(), rand.Text()
	var deliveries []sqlc.CreateNotificationDeliveryParams
	for index, text := range parts(message.Text, 500) {
		part := Message{Text: text}
		if index == 0 {
			part.Title, part.URL = message.Title, message.URL
		}
		deliveries = append(deliveries, deliveriesFor("manual", key+":"+strconv.Itoa(index), 0, input.ChannelIDs,
			[]Message{part}, now.Add(time.Duration(index)*time.Millisecond))...)
	}
	s.configMu.Lock()
	defer s.configMu.Unlock()
	// Recheck after extraction, since the user may have changed a channel while it ran.
	if err := s.validArticleShareChannels(ctx, input.ChannelIDs); err != nil {
		return err
	}
	if err := s.store.EnqueueNotification(ctx, deliveries); err != nil {
		return errors.New("无法将文章加入通知队列")
	}
	s.Wake()
	return nil
}

func (s *Service) validArticleShareChannels(ctx context.Context, ids []string) error {
	rows, err := s.q.ListNotificationChannels(ctx)
	if err != nil {
		return errors.New("无法读取通知渠道")
	}
	enabled := make(map[string]bool, len(rows))
	for _, row := range rows {
		enabled[row.ID] = row.Enabled != 0 && row.Verified != 0
	}
	if len(ids) == 0 {
		return errors.New("请选择通知渠道")
	}
	for _, id := range ids {
		if !enabled[id] {
			return errors.New("请选择不同的、已启用且通过测试的通知渠道")
		}
		delete(enabled, id)
	}
	return nil
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
