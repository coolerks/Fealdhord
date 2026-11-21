package notification

import (
	"context"
	"errors"

	"github.com/coolerks/Fealdhord/internal/database/sqlc"
	"github.com/coolerks/Fealdhord/internal/schedule"
)

type API struct{ service *Service }

func NewAPI(service *Service) *API { return &API{service: service} }

func available[T any](a *API, run func(*Service) (T, error)) (T, error) {
	if a.service == nil {
		var result T
		return result, errors.New("本地通知存储不可用")
	}
	return run(a.service)
}

func (a *API) ListChannels(ctx context.Context) ([]Channel, error) {
	return available(a, func(s *Service) ([]Channel, error) { return s.ListChannels(ctx) })
}

func (a *API) SaveChannel(ctx context.Context, input ChannelInput) (Channel, error) {
	return available(a, func(s *Service) (Channel, error) { return s.SaveChannel(ctx, input) })
}

func (a *API) TestChannel(ctx context.Context, id string) error {
	_, err := available(a, func(s *Service) (bool, error) { return true, s.TestChannel(ctx, id) })
	return err
}

func (a *API) ListRules(ctx context.Context) ([]Rule, error) {
	return available(a, func(s *Service) ([]Rule, error) { return s.ListRules(ctx) })
}

func (a *API) SaveRule(ctx context.Context, input Rule) (Rule, error) {
	return available(a, func(s *Service) (Rule, error) { return s.SaveRule(ctx, input) })
}

func (a *API) ListDeliveries(ctx context.Context) ([]sqlc.ListNotificationDeliveriesRow, error) {
	return available(a, func(s *Service) ([]sqlc.ListNotificationDeliveriesRow, error) {
		return s.q.ListNotificationDeliveries(ctx)
	})
}

func (a *API) ListTasks(ctx context.Context) ([]Task, error) {
	return available(a, func(s *Service) ([]Task, error) { return s.ListTasks(ctx) })
}

func (a *API) SaveTask(ctx context.Context, input Task) (Task, error) {
	return available(a, func(s *Service) (Task, error) { return s.SaveTask(ctx, input) })
}

func (a *API) ListRuns(ctx context.Context, id string) ([]sqlc.ScheduledRun, error) {
	return available(a, func(s *Service) ([]sqlc.ScheduledRun, error) { return s.q.ListScheduledRuns(ctx, id) })
}

func (a *API) DeleteTask(ctx context.Context, id string, revision int64) error {
	_, err := available(a, func(s *Service) (bool, error) { return true, s.DeleteTask(ctx, id, revision) })
	return err
}

func (a *API) StartWeChatBinding(ctx context.Context) (WeChatBinding, error) {
	return available(a, func(s *Service) (WeChatBinding, error) { return s.startWeChatBinding(ctx) })
}

func (a *API) PollWeChatBinding(ctx context.Context, id, pin, name string) (WeChatBinding, error) {
	return available(a, func(s *Service) (WeChatBinding, error) { return s.pollWeChatBinding(ctx, id, pin, name) })
}

func (a *API) CancelWeChatBinding(id string) {
	if a.service != nil {
		a.service.cancelWeChatBinding(id)
	}
}

func (a *API) DeleteChannel(ctx context.Context, id string, revision int64) error {
	_, err := available(a, func(s *Service) (bool, error) { return true, s.deleteChannel(ctx, id, revision) })
	return err
}

func (a *API) DeleteRule(ctx context.Context, id string, revision int64) error {
	_, err := available(a, func(s *Service) (bool, error) {
		return true, checkWrite(s.q.DeleteNotificationRule(ctx, sqlc.DeleteNotificationRuleParams{ID: id, Revision: revision}))
	})
	return err
}

func (a *API) PreviewTask(ctx context.Context, input Task) (int64, error) {
	return available(a, func(s *Service) (int64, error) {
		value := schedule.Millis(input.Calendar.Next(s.now()))
		if value == 0 {
			return 0, errors.New("任务没有未来可执行的时间")
		}
		return value, nil
	})
}

func (a *API) RetryDelivery(ctx context.Context, id string) error {
	_, err := available(a, func(s *Service) (bool, error) {
		if err := checkWrite(s.q.RetryNotificationDelivery(ctx, sqlc.RetryNotificationDeliveryParams{ID: id, DueAt: s.now().UnixMilli()})); err != nil {
			return false, err
		}
		s.Wake()
		return true, nil
	})
	return err
}

type RulePreview struct {
	NextAt int64 `json:"next_at"`
	Start  int64 `json:"start"`
	End    int64 `json:"end"`
}

func (a *API) PreviewRule(ctx context.Context, input Rule) (RulePreview, error) {
	return available(a, func(s *Service) (RulePreview, error) {
		if err := ctx.Err(); err != nil {
			return RulePreview{}, err
		}
		cal := schedule.Calendar{Kind: "daily", Timezone: input.Timezone, LocalTime: input.SendAt}
		if input.Mode != "daily" || !cal.Valid() {
			return RulePreview{}, errors.New("请选择有效的发送时间和时区")
		}
		at := cal.Next(s.now())
		start, end, err := ruleWindow(input, at)
		if err != nil {
			return RulePreview{}, err
		}
		return RulePreview{NextAt: at.UnixMilli(), Start: start.UnixMilli(), End: end.UnixMilli()}, nil
	})
}

func (a *API) ListLocalNotices(ctx context.Context) ([]sqlc.ListPendingLocalNoticesRow, error) {
	return available(a, func(s *Service) ([]sqlc.ListPendingLocalNoticesRow, error) {
		return s.q.ListPendingLocalNotices(ctx)
	})
}

func (a *API) AcknowledgeLocalNotice(ctx context.Context, id string, occurrence int64) error {
	_, err := available(a, func(s *Service) (bool, error) {
		return true, s.q.AcknowledgeLocalNotice(ctx, sqlc.AcknowledgeLocalNoticeParams{TaskID: id, Occurrence: occurrence})
	})
	return err
}
