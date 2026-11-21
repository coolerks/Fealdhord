package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coolerks/Fealdhord/internal/database/sqlc"
	"github.com/coolerks/Fealdhord/internal/schedule"
)

func ruleWindow(rule Rule, at time.Time) (time.Time, time.Time, error) {
	if _, err := time.Parse("15:04", rule.WindowStart); err != nil {
		return time.Time{}, time.Time{}, errors.New("收录开始时间无效")
	}
	if _, err := time.Parse("15:04", rule.WindowEnd); err != nil {
		return time.Time{}, time.Time{}, errors.New("收录结束时间无效")
	}
	endDay := at
	if end := schedule.WallTime(endDay, rule.WindowEnd); end.After(at) {
		endDay = at.AddDate(0, 0, -1)
	}
	startDay := endDay
	if rule.WindowStart >= rule.WindowEnd {
		startDay = endDay.AddDate(0, 0, -1)
	}
	return schedule.WallTime(startDay, rule.WindowStart), schedule.WallTime(endDay, rule.WindowEnd), nil
}

func (s *Service) processRules(ctx context.Context) error {
	if err := s.q.PruneNotificationCandidates(ctx, s.now().UnixMilli()); err != nil {
		return err
	}
	rules, err := s.q.ListNotificationRules(ctx)
	if err != nil {
		return err
	}
	parents, err := s.categoryParents(ctx)
	if err != nil {
		return err
	}
	for _, row := range rules {
		if row.Enabled == 0 {
			continue
		}
		rule, err := ruleView(row)
		if err != nil {
			return err
		}
		for range 64 {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			cursor, due, err := s.processRule(ctx, row, rule, parents)
			if err != nil {
				return err
			}
			if row.EventCursor == cursor && row.NextAt == due {
				break
			}
			row.EventCursor, row.NextAt = cursor, due
		}
	}
	return nil
}

func (s *Service) processRule(ctx context.Context, row sqlc.NotificationRule, rule Rule, parents map[string]string) (int64, int64, error) {
	now := s.now()
	cursor, next := row.EventCursor, row.NextAt
	cal := schedule.Calendar{Kind: "daily", Timezone: rule.Timezone, LocalTime: rule.SendAt}
	if rule.Mode == "daily" && (next == 0 || next <= now.UnixMilli()) {
		next = schedule.Millis(cal.Next(now))
	}
	events, err := s.q.ListNotificationEvents(ctx, sqlc.ListNotificationEventsParams{AfterSeq: cursor, PageSize: 100})
	if err != nil {
		return cursor, next, err
	}
	source := "rule:" + row.ID
	var deliveries []sqlc.CreateNotificationDeliveryParams
	var digestItems []sqlc.CreateNotificationDigestItemParams
	var candidates []sqlc.AddNotificationCandidateParams
	var matchedCandidates []int64
	fresh := len(events)
	if len(rule.Categories) != 0 {
		var included []string
		for category := range parents {
			if matchesCategories([]string{category}, rule.Categories, parents) {
				included = append(included, category)
			}
		}
		data, _ := json.Marshal(included)
		rows, err := s.q.ListMatchedNotificationCandidates(ctx, sqlc.ListMatchedNotificationCandidatesParams{
			RuleID: row.ID, RuleRevision: row.Revision, NowAt: now.UnixMilli(), CategoryIds: string(data),
		})
		if err != nil {
			return cursor, next, err
		}
		for _, candidate := range rows {
			matchedCandidates = append(matchedCandidates, candidate.Seq)
			events = append(events, sqlc.ListNotificationEventsRow(candidate))
		}
	}
	for index, event := range events {
		if index < fresh {
			cursor = event.Seq
			matched, err := s.matchesEvent(ctx, event.ArticleID, rule.Categories, parents)
			if err != nil {
				return cursor, next, err
			}
			if !matched {
				expires := time.UnixMilli(event.DiscoveredAt).AddDate(0, 0, 2)
				if expires.After(now) {
					candidates = append(candidates, sqlc.AddNotificationCandidateParams{
						RuleID: row.ID, RuleRevision: row.Revision, EventSeq: event.Seq, ExpiresAt: expires.UnixMilli(),
					})
				}
				continue
			}
		}
		message := Message{Title: event.Title, URL: event.Url}
		if rule.Body {
			message.Text = event.Body
			if message.Text == "" {
				message.Text = event.Summary
			}
		}
		if rule.Image && strings.HasPrefix(event.Cover, "https://") {
			message.Image = event.Cover
		}
		recovered := event.OfflineSince > 0 && event.OfflineSince < event.ResumedAt
		if rule.Mode == "daily" || recovered {
			at := now
			if rule.Mode == "daily" {
				at = digestOccurrence(rule, cal, event, recovered)
				if at.IsZero() {
					continue
				}
			}
			if rule.Mode == "instant" && recovered {
				at = time.UnixMilli(event.ResumedAt)
			}
			data, _ := json.Marshal(message)
			digestItems = append(digestItems, sqlc.CreateNotificationDigestItemParams{
				RuleID: row.ID, RuleRevision: row.Revision, ArticleID: event.ArticleID,
				Occurrence: at.UnixMilli(), Recovery: flag(recovered && !at.After(now)), Payload: string(data),
			})
		} else {
			deliveries = append(deliveries, deliveriesFor(source, event.ArticleID, row.Revision, rule.Channels, []Message{message}, now)...)
		}
	}
	if row.EventCursor != cursor || row.NextAt != next || len(matchedCandidates) != 0 {
		err = s.store.SaveNotificationBatch(ctx, sqlc.AdvanceNotificationRuleParams{
			ID: row.ID, Revision: row.Revision, EventCursor_2: row.EventCursor, EventCursor: cursor, NextAt: next,
		}, deliveries, digestItems, candidates, matchedCandidates)
		if err != nil {
			return cursor, next, err
		}
	}
	return cursor, next, s.queueDigests(ctx, row, rule, now)
}

func digestOccurrence(rule Rule, calendar schedule.Calendar, event sqlc.ListNotificationEventsRow, recovered bool) time.Time {
	observed := time.UnixMilli(event.DiscoveredAt)
	if recovered && event.PublishedAt.Valid && event.PublishedAt.Int64 >= event.OfflineSince &&
		event.PublishedAt.Int64 < event.ResumedAt {
		observed = time.UnixMilli(event.PublishedAt.Int64)
	}
	loc, _ := time.LoadLocation(rule.Timezone)
	at := calendar.Next(observed)
	start, end, _ := ruleWindow(rule, at.In(loc))
	if !observed.Before(start) && observed.Before(end) {
		return at
	}
	if recovered && observed.UnixMilli() == event.DiscoveredAt {
		return time.UnixMilli(event.ResumedAt)
	}
	return time.Time{}
}

func (s *Service) queueDigests(ctx context.Context, row sqlc.NotificationRule, rule Rule, now time.Time) error {
	due, err := s.q.DueNotificationDigest(ctx, sqlc.DueNotificationDigestParams{
		RuleID: row.ID, RuleRevision: row.Revision, Occurrence: now.UnixMilli(),
	})
	if err != nil {
		return err
	}
	for len(due) > 0 {
		first := due[0]
		count := 1
		for count < len(due) && due[count].Occurrence == first.Occurrence && due[count].Recovery == first.Recovery {
			count++
		}
		items := due[:count]
		title := rule.Name
		if first.Recovery != 0 {
			title += " · 补发"
		}
		if loc, err := time.LoadLocation(rule.Timezone); err == nil {
			title += " · " + time.UnixMilli(first.Occurrence).In(loc).Format("2006-01-02")
		}
		messages, err := digestMessages(title, items, rule.Body || rule.Image)
		if err != nil {
			return err
		}
		key := fmt.Sprintf("digest:%d:%d", first.Occurrence, first.ID)
		deliveries := deliveriesFor("rule:"+row.ID, key, row.Revision, rule.Channels, messages, now)
		if err := s.store.QueueNotificationDigest(ctx, items, deliveries); err != nil {
			return err
		}
		due = due[count:]
	}
	return nil
}

func digestMessages(title string, items []sqlc.NotificationDigestItem, full bool) ([]Message, error) {
	var messages []Message
	var digest Message
	if full {
		messages = append(messages, Message{Title: title})
	}
	for _, item := range items {
		var article Message
		if err := json.Unmarshal([]byte(item.Payload), &article); err != nil {
			return nil, err
		}
		if full {
			messages = append(messages, article)
			continue
		}
		if len(digest.Links) == 3 {
			messages = append(messages, digest)
			digest = Message{}
		}
		digest.Title = title
		digest.Links = append(digest.Links, ArticleLink{Title: clip(article.Title, 120), URL: article.URL})
	}
	if len(digest.Links) != 0 {
		messages = append(messages, digest)
	}
	return messages, nil
}

func (s *Service) matchesEvent(ctx context.Context, id string, categories []string, parents map[string]string) (bool, error) {
	if len(categories) == 0 {
		return true, nil
	}
	assigned, err := s.q.ListNotificationArticleCategories(ctx, id)
	return matchesCategories(assigned, categories, parents), err
}
