package articleai

import (
	"context"
	"errors"
	"time"

	"github.com/coolerks/Fealdhord/internal/ai"
)

func (s *Service) Start(ctx context.Context) func() {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		var backoff time.Time
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			if time.Now().Before(backoff) {
				continue
			}
			for _, task := range []string{"classification", "featured"} {
				id, err := s.store.NextArticleAI(ctx, task, time.Now().Add(-7*24*time.Hour))
				if err != nil || id == "" {
					continue
				}
				_, err = s.Evaluate(ctx, Request{ArticleID: id, Task: task})
				var failure *ai.ModelError
				if !errors.As(err, &failure) {
					continue
				}
				switch failure.Code {
				case "RateLimitError":
					backoff = time.Now().Add(max(time.Minute, failure.RetryAfter))
				case "BudgetExceeded":
					backoff = time.Now().Add(time.Minute)
				case "AuthenticationError", "CredentialError":
					backoff = time.Now().Add(5 * time.Minute)
				case "Busy", "Conflict", "Disabled", "Cancelled":
				default:
					_ = s.store.DeferArticleAI(ctx, id, task, time.Now().Add(30*time.Minute))
				}
				if !backoff.IsZero() && time.Now().Before(backoff) {
					break
				}
			}
		}
	}()
	return func() { cancel(); <-done }
}

type API struct{ service *Service }

func NewAPI(service *Service) *API { return &API{service: service} }

func (a *API) Settings(ctx context.Context) (Settings, error) {
	if a.service == nil {
		return Settings{}, code("StorageError")
	}
	return a.service.Settings(ctx)
}

func (a *API) Save(ctx context.Context, input Settings) (Settings, error) {
	if a.service == nil {
		return Settings{}, code("StorageError")
	}
	return a.service.Save(ctx, input)
}

func (a *API) Evaluate(ctx context.Context, input Request) (Result, error) {
	if a.service == nil {
		return Result{}, code("StorageError")
	}
	return a.service.Evaluate(ctx, input)
}
