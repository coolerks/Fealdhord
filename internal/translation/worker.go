package translation

import (
	"context"
	"errors"
	"time"

	"github.com/coolerks/Fealdhord/internal/ai"
)

// Start 启动后台自动翻译 Worker，每 15 秒扫描最近 7 天内待自动翻译的新文章。
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
			if time.Now().Before(backoff) || s.store == nil {
				continue
			}
			if delay := s.translateNext(ctx); delay > 0 {
				backoff = time.Now().Add(delay)
			}
		}
	}()
	return func() { cancel(); <-done }
}

func (s *Service) translateNext(ctx context.Context) time.Duration {
	articleID, scope, targetLang, err := s.store.NextArticleForTranslation(ctx, time.Now().Add(-7*24*time.Hour))
	if err != nil || articleID == "" {
		return 0
	}
	sourceMode, deferTask := "summary", "translation"
	if scope == "full" {
		sourceMode, deferTask = "reader", "translation_full"
	}
	_, err = s.Translate(ctx, TranslateRequest{
		ArticleID: articleID, TargetLang: targetLang, SourceMode: sourceMode, Background: true,
	})
	var failure *ai.ModelError
	if !errors.As(err, &failure) {
		return 0
	}
	switch failure.Code {
	case "RateLimitError":
		return max(time.Minute, failure.RetryAfter)
	case "BudgetExceeded":
		return time.Minute
	case "AuthenticationError", "CredentialError", "ConfigurationError", "CredentialRequired", "InvalidEndpoint":
		return 5 * time.Minute
	case "Busy", "Cancelled":
		return 0
	default:
		// Unsupported source languages are specific to the article.
		if err := s.store.DeferArticleAI(ctx, articleID, deferTask, time.Now().Add(30*time.Minute)); err != nil {
			return time.Minute
		}
		return 0
	}
}

// API 暴露给 Wails 前端的翻译服务绑定接口。
type API struct {
	service *Service
}

func NewAPI(service *Service) *API {
	return &API{service: service}
}

func (a *API) Languages() []LanguageOption {
	return SupportedLanguages
}

func (a *API) Get(ctx context.Context, articleID, targetLang, sourceMode string) (Record, error) {
	if a.service == nil {
		return Record{}, &ai.ModelError{Code: "StorageError"}
	}
	return a.service.Get(ctx, articleID, targetLang, sourceMode)
}

func (a *API) Translate(ctx context.Context, input TranslateRequest) (Record, error) {
	if a.service == nil {
		return Record{}, &ai.ModelError{Code: "StorageError"}
	}
	input.Background = false
	return a.service.Translate(ctx, input)
}

func (a *API) Cancel(articleID, targetLang, sourceMode string) {
	if a.service == nil {
		return
	}
	a.service.Cancel(articleID, targetLang, sourceMode)
}

func (a *API) Clear(ctx context.Context, articleID, targetLang string) error {
	if a.service == nil {
		return &ai.ModelError{Code: "StorageError"}
	}
	return a.service.Clear(ctx, articleID, targetLang)
}

func (a *API) Providers() []ProviderMeta {
	return SupportedProviders()
}

func (a *API) GetProviderConfig(ctx context.Context, id string) (ProviderConfig, error) {
	if a.service == nil {
		return ProviderConfig{}, &ai.ModelError{Code: "StorageError"}
	}
	return a.service.GetProviderConfig(ctx, id)
}

func (a *API) SaveProviderConfig(ctx context.Context, input SaveProviderInput) (ProviderConfig, error) {
	if a.service == nil {
		return ProviderConfig{}, &ai.ModelError{Code: "StorageError"}
	}
	return a.service.SaveProviderConfig(ctx, input)
}

func (a *API) TestProvider(ctx context.Context, id string, draft *SaveProviderInput) TestResult {
	if a.service == nil {
		return TestResult{OK: false, ErrorCode: "StorageError"}
	}
	return a.service.TestProvider(ctx, id, draft)
}
