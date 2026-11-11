package desktop

import (
	"context"
	"errors"
	"log/slog"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

// Notifier limits the frontend bridge to authorization and immediate notices.
type Notifier struct {
	service *notifications.NotificationService
	ready   bool
}

func NewNotifier() *Notifier {
	return &Notifier{service: notifications.New()}
}

func (n *Notifier) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	if err := n.service.ServiceStartup(ctx, options); err != nil {
		slog.Warn("系统通知不可用", "error", err)
		return nil
	}
	n.ready = true
	return nil
}

func (n *Notifier) ServiceShutdown() error {
	if n.ready {
		return n.service.ServiceShutdown()
	}
	return nil
}

func (n *Notifier) Authorize() (bool, error) {
	if !n.ready {
		return false, errors.New("系统通知不可用。")
	}
	return n.service.RequestNotificationAuthorization()
}

func (n *Notifier) Send(id, title, body string) error {
	if !n.ready {
		return errors.New("系统通知不可用。")
	}
	if len(id) == 0 || len(id) > 128 || len(title) == 0 || len(title) > 1024 || len(body) > 4096 {
		return errors.New("通知内容无效。")
	}
	allowed, err := n.service.CheckNotificationAuthorization()
	if err != nil {
		return err
	}
	if !allowed {
		return errors.New("系统通知权限未开启。")
	}
	return n.service.SendNotification(notifications.NotificationOptions{ID: id, Title: title, Body: body})
}
