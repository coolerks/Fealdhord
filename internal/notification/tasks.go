package notification

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coolerks/Fealdhord/internal/database/sqlc"
	"github.com/coolerks/Fealdhord/internal/schedule"
)

type Task struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Action       string            `json:"action"`
	LookbackDays int               `json:"lookback_days"`
	Text         string            `json:"text"`
	Calendar     schedule.Calendar `json:"calendar"`
	Categories   []string          `json:"categories"`
	Channels     []string          `json:"channels"`
	Enabled      bool              `json:"enabled"`
	Revision     int64             `json:"revision"`
	NextAt       int64             `json:"next_at"`
}

func taskView(row sqlc.ScheduledTask) (Task, error) {
	var task Task
	err := json.Unmarshal([]byte(row.Configuration), &task)
	task.ID, task.Revision, task.Enabled, task.NextAt = row.ID, row.Revision, row.Enabled != 0, row.NextAt
	return task, err
}

func (s *Service) ListTasks(ctx context.Context) ([]Task, error) {
	rows, err := s.q.ListScheduledTasks(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Task, 0, len(rows))
	for _, row := range rows {
		task, err := taskView(row)
		if err != nil {
			return nil, err
		}
		result = append(result, task)
	}
	return result, nil
}

func (s *Service) GetTask(ctx context.Context, id string) (Task, error) {
	row, err := s.q.GetScheduledTask(ctx, id)
	if err != nil {
		return Task{}, errors.New("找不到定时任务")
	}
	return taskView(row)
}

func (s *Service) SaveTask(ctx context.Context, task Task) (Task, error) {
	task.Name = strings.TrimSpace(task.Name)
	task.Text = strings.TrimSpace(task.Text)
	if task.Name == "" || len(task.Name) > 200 || len(task.Text) > 4000 || len(task.Categories) > 100 || task.LookbackDays < 0 || task.LookbackDays > 30 || !task.Calendar.Valid() {
		return Task{}, errors.New("任务名称、时间或内容无效")
	}
	switch task.Action {
	case "reminder":
		if task.Text == "" {
			return Task{}, errors.New("请填写提醒内容")
		}
	case "articles", "summary":
	default:
		return Task{}, errors.New("不支持该任务操作")
	}
	if task.Enabled && len(task.Channels) > 0 {
		if err := s.validTargets(ctx, task.Channels); err != nil {
			return Task{}, err
		}
	}
	task.NextAt = schedule.Millis(task.Calendar.Next(s.now()))
	if task.Enabled && task.NextAt == 0 {
		return Task{}, errors.New("请选择未来的执行时间")
	}
	if task.ID == "" {
		task.ID = rand.Text()
		data, _ := json.Marshal(task)
		if err := s.q.CreateScheduledTask(ctx, sqlc.CreateScheduledTaskParams{
			ID: task.ID, Configuration: string(data), Enabled: flag(task.Enabled), NextAt: task.NextAt,
		}); err != nil {
			return Task{}, err
		}
	} else {
		data, _ := json.Marshal(task)
		if err := checkWrite(s.q.UpdateScheduledTask(ctx, sqlc.UpdateScheduledTaskParams{
			ID: task.ID, Revision: task.Revision, Configuration: string(data), Enabled: flag(task.Enabled), NextAt: task.NextAt,
		})); err != nil {
			return Task{}, err
		}
	}
	s.Wake()
	return s.GetTask(ctx, task.ID)
}

func (s *Service) SetTaskEnabled(ctx context.Context, id string, revision int64, enabled bool) (Task, error) {
	task, err := s.GetTask(ctx, id)
	if err != nil {
		return Task{}, err
	}
	if task.Revision != revision {
		return Task{}, errors.New("定时任务已变更")
	}
	task.Enabled = enabled
	return s.SaveTask(ctx, task)
}

func (s *Service) DeleteTask(ctx context.Context, id string, revision int64) error {
	return checkWrite(s.q.DeleteScheduledTask(ctx, sqlc.DeleteScheduledTaskParams{ID: id, Revision: revision}))
}

func (s *Service) processTasks(ctx context.Context) error {
	rows, err := s.q.ListScheduledTasks(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		now := s.now()
		if row.Enabled == 0 || row.NextAt <= 0 || row.NextAt > now.UnixMilli() {
			continue
		}
		task, err := taskView(row)
		if err != nil {
			return err
		}
		occurrence := time.UnixMilli(row.NextAt)
		next := schedule.Millis(task.Calendar.Next(now))
		if err := s.store.ClaimNotificationTask(ctx, sqlc.ClaimScheduledTaskParams{
			ID: task.ID, Revision: task.Revision, NextAt_2: row.NextAt, NextAt: next,
		}); err != nil {
			return err
		}
		if err := s.runTask(ctx, task, occurrence, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) recoverTasks(ctx context.Context) error {
	rows, err := s.q.ListUnfinishedScheduledRuns(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.State != "interrupted" {
			continue
		}
		count, err := s.q.ResumeScheduledRun(ctx, sqlc.ResumeScheduledRunParams{TaskID: row.TaskID, Occurrence: row.Occurrence})
		if err != nil {
			return err
		}
		if count != 1 {
			continue
		}
		task, err := s.GetTask(ctx, row.TaskID)
		if err != nil || !task.Enabled || task.Revision != row.TaskRevision {
			if _, err := s.q.FinishScheduledRun(ctx, sqlc.FinishScheduledRunParams{
				TaskID: row.TaskID, Occurrence: row.Occurrence, State: "cancelled",
			}); err != nil {
				return err
			}
			continue
		}
		if err := s.runTask(ctx, task, time.UnixMilli(row.Occurrence), s.now()); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) runTask(ctx context.Context, task Task, occurrence, now time.Time) error {
	messages, runErr := s.taskMessages(ctx, task, occurrence, now)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	state, result := "complete", ""
	var deliveries []sqlc.CreateNotificationDeliveryParams
	switch {
	case runErr != nil:
		state, result = "failed", runErr.Error()
	case len(messages) != 0:
		var output []string
		for _, message := range messages {
			output = append(output, plainMessage(message))
		}
		result = strings.Join(output, "\n\n")
		deliveries = deliveriesFor("task:"+task.ID, fmt.Sprint(occurrence.UnixMilli()), task.Revision, task.Channels, messages, now)
	}
	localPending := len(task.Channels) == 0 && result != "" && state == "complete"
	err := s.store.CompleteNotificationTask(ctx, sqlc.FinishScheduledRunParams{
		TaskID: task.ID, Occurrence: occurrence.UnixMilli(), State: state, Result: result, LocalPending: flag(localPending),
	}, deliveries)
	if err == nil && s.localNotice != nil && localPending {
		s.localNotice(LocalNotice{Title: task.Name, Body: clip(result, 500)})
	}
	return err
}

func (s *Service) taskMessages(ctx context.Context, task Task, occurrence, now time.Time) ([]Message, error) {
	if task.Action == "reminder" {
		return []Message{{Title: task.Name, Text: task.Text}}, nil
	}
	loc, _ := time.LoadLocation(task.Calendar.Timezone)
	since := occurrence.In(loc).AddDate(0, 0, -max(task.LookbackDays, 1))
	rows, err := s.q.ListTaskArticles(ctx, sqlc.ListTaskArticlesParams{Since: since.UnixMilli(), Until: now.UnixMilli()})
	if err != nil {
		return nil, errors.New("读取文章失败")
	}
	parents, err := s.categoryParents(ctx)
	if err != nil {
		return nil, errors.New("读取分类失败")
	}
	messages := make([]Message, 0, len(rows))
	for _, row := range rows {
		ok, err := s.matchesEvent(ctx, row.ID, task.Categories, parents)
		if err != nil {
			return nil, errors.New("读取文章分类失败")
		}
		if ok {
			messages = append(messages, Message{Title: row.Title, URL: row.Url, Text: row.Summary})
		}
	}
	if task.Action == "articles" || len(messages) == 0 {
		return messages, nil
	}
	if s.summarize == nil {
		return nil, errors.New("没有可用的摘要模型")
	}
	text, err := s.summarize(ctx, task.Text, messages)
	if err != nil {
		return nil, errors.New("生成文章摘要失败")
	}
	return []Message{{Title: task.Name, Text: text}}, nil
}

func (s *Service) Send(ctx context.Context, channelIDs []string, message Message) error {
	if strings.TrimSpace(message.Title) == "" || len(message.Title) > 800 || len(message.Text) > 12000 {
		return errors.New("通知内容无效")
	}
	if err := s.validTargets(ctx, channelIDs); err != nil {
		return err
	}
	// AI can name saved targets and provide text, but cannot cause an image fetch.
	message.Image = ""
	message.Links = nil
	if err := s.store.EnqueueNotification(ctx, deliveriesFor("manual", rand.Text(), 0, channelIDs, []Message{message}, s.now())); err != nil {
		return err
	}
	s.Wake()
	return nil
}
