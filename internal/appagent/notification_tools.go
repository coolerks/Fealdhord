package appagent

import (
	"context"
	"time"

	"github.com/coolerks/Fealdhord/internal/notification"
)

type taskVersion struct {
	ID       string `json:"id" jsonschema:"required"`
	Revision int64  `json:"revision" jsonschema:"required"`
}
type notificationInput struct {
	Channels []string             `json:"channels" jsonschema:"required"`
	Message  notification.Message `json:"message" jsonschema:"required"`
}

func notificationResult(data any, err error) any {
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	return map[string]any{"ok": true, "data": data}
}

func registerNotificationTools(r *Registry, service *notification.Service) {
	Register(r, "notification.channel.list", "列出保存的通知渠道和启用、测试状态；返回 ID 供选择", false, func(ctx context.Context, _ empty) any {
		return notificationResult(service.ListChannels(ctx))
	})
	Register(r, "notification.rule.list", "查看文章推送规则和范围", false, func(ctx context.Context, _ empty) any {
		return notificationResult(service.ListRules(ctx))
	})
	Register(r, "notification.send", "向用户选择且测试过的通知渠道发送消息，需确认", true, func(ctx context.Context, input notificationInput) any {
		return notificationResult(nil, service.Send(ctx, input.Channels, input.Message))
	})
	Register(r, "schedule.list", "列出 AI 定时任务及当前本机时间；next_at 是 Unix 毫秒", false, func(ctx context.Context, _ empty) any {
		tasks, err := service.ListTasks(ctx)
		return notificationResult(map[string]any{"tasks": tasks, "local_time": time.Now().Format(time.RFC3339)}, err)
	})
	Register(r, "schedule.get", "读取定时任务详情与版本", false, func(ctx context.Context, input id) any {
		return notificationResult(service.GetTask(ctx, input.ID))
	})
	Register(r, "schedule.create", "创建任务，id 留空，设置 enabled=true。action 为 reminder、articles 或 summary。calendar 接受 once、daily、weekly，明确 IANA 时区；once_at 用 RFC3339，周日起算 0。文章任务可用 lookback_days 指定回看天数（1–30）。channels 为已配置渠道 ID，空为应用内记录。创建需确认", true, func(ctx context.Context, task notification.Task) any {
		if task.ID != "" || task.Revision != 0 {
			return map[string]any{"ok": false, "error": "创建任务时不能指定 ID 或版本"}
		}
		task.Enabled = true
		return notificationResult(service.SaveTask(ctx, task))
	})
	Register(r, "schedule.update", "按已有 ID 和 revision 修改定时任务，提交完整配置，需确认", true, func(ctx context.Context, task notification.Task) any {
		if task.ID == "" || task.Revision <= 0 {
			return map[string]any{"ok": false, "error": "修改任务时需要任务 ID 和版本"}
		}
		return notificationResult(service.SaveTask(ctx, task))
	})
	Register(r, "schedule.pause", "暂停定时任务，需确认", true, func(ctx context.Context, input taskVersion) any {
		return notificationResult(service.SetTaskEnabled(ctx, input.ID, input.Revision, false))
	})
	Register(r, "schedule.resume", "恢复定时任务，需确认", true, func(ctx context.Context, input taskVersion) any {
		return notificationResult(service.SetTaskEnabled(ctx, input.ID, input.Revision, true))
	})
	Register(r, "schedule.delete", "删除定时任务，需确认", true, func(ctx context.Context, input taskVersion) any {
		return notificationResult(nil, service.DeleteTask(ctx, input.ID, input.Revision))
	})
}
