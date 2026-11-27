package translation

import (
	"context"
	"crypto/rand"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/coolerks/Fealdhord/internal/aiconfig"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
	"github.com/zalando/go-keyring"
)

// ProviderConfig 是面向前端展示的非敏感配置（绝不返回明文密钥）。
type ProviderConfig struct {
	ID           string `json:"id"`
	AppID        string `json:"appId"`
	Region       string `json:"region"`
	Endpoint     string `json:"endpoint"`
	Edition      string `json:"edition"`
	HasSecret    bool   `json:"hasSecret"`
	SecretMasked string `json:"secretMasked"`
}

// SaveProviderInput 是保存某个 BYOK 翻译引擎配置时的入参。
type SaveProviderInput struct {
	ID          string `json:"id"`
	AppID       string `json:"appId"`
	Secret      string `json:"secret"`
	ClearSecret bool   `json:"clearSecret"`
	Region      string `json:"region"`
	Endpoint    string `json:"endpoint"`
	Edition     string `json:"edition"`
}

// TestResult 表示翻译方式连通性测试结果。
type TestResult struct {
	OK          bool   `json:"ok"`
	SampleOut   string `json:"sampleOut,omitempty"`
	LatencyMs   int64  `json:"latencyMs"`
	ErrorCode   string `json:"errorCode,omitempty"`
	ErrorDetail string `json:"errorDetail,omitempty"`
}

type resolvedProviderConfig struct {
	ID       EngineID
	AppID    string
	Secret   string
	Region   string
	Endpoint string
	Edition  string
}

func parseProviderEndpoint(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 512 || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return nil, mtError("InvalidEndpoint", "翻译接口地址须为 HTTPS")
	}
	return u, nil
}

func providerOrigin(id EngineID, endpoint string) (string, error) {
	if endpoint == "" {
		switch id {
		case EngineGoogleBYOK:
			endpoint = defaultGoogleCloudV2URL
		case EngineMicrosoftAzure:
			endpoint = defaultMicrosoftAzureURL
		case EngineAlibaba:
			endpoint = defaultAlibabaEndpoint
		case EngineCaiyun:
			endpoint = defaultCaiyunEndpoint
		case EngineBaidu:
			endpoint = defaultBaiduEndpoint
		case EngineYoudao:
			endpoint = defaultYoudaoBatchEndpoint
		}
	}
	u, err := parseProviderEndpoint(endpoint)
	if err != nil {
		return "", err
	}
	return httpsOrigin(u), nil
}

func httpsOrigin(u *url.URL) string {
	port := u.Port()
	if port == "" {
		port = "443"
	}
	return strings.ToLower(u.Hostname()) + ":" + port
}

func validateProviderEndpointChange(id EngineID, previous sqlc.TranslationProvider, endpoint, secret string, clear bool) error {
	origin, err := providerOrigin(id, endpoint)
	if err != nil {
		return err
	}
	if previous.CredentialRef != "" && secret == "" && !clear {
		oldOrigin, err := providerOrigin(id, previous.Endpoint)
		if err != nil || oldOrigin != origin {
			return &ai.ModelError{Code: "CredentialRequired"}
		}
	}
	return nil
}

func maskSecret(secret string) string {
	s := strings.TrimSpace(secret)
	if s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) <= 8 {
		return "****"
	}
	return string(r[:4]) + "****" + string(r[len(r)-4:])
}

func (s *Service) credentialStore() aiconfig.CredentialStore {
	if s.credentials != nil {
		return s.credentials
	}
	return aiconfig.SystemCredentials{}
}

func (s *Service) GetProviderConfig(ctx context.Context, id string) (ProviderConfig, error) {
	s.credentialMu.RLock()
	defer s.credentialMu.RUnlock()
	return s.getProviderConfig(ctx, id)
}

func (s *Service) getProviderConfig(ctx context.Context, id string) (ProviderConfig, error) {
	if s.store == nil {
		return ProviderConfig{}, &ai.ModelError{Code: "StorageError"}
	}
	eid := EngineID(strings.TrimSpace(id))
	if !IsBYOKProvider(eid) {
		return ProviderConfig{}, &ai.ModelError{Code: "ValidationError"}
	}
	out := ProviderConfig{ID: string(eid)}
	row, err := s.store.GetTranslationProvider(ctx, string(eid))
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return ProviderConfig{}, &ai.ModelError{Code: "StorageError"}
	}
	if err == nil {
		out.AppID = row.AppID
		out.Region = row.Region
		out.Endpoint = row.Endpoint
		out.Edition = row.Edition
		if row.CredentialRef != "" {
			if secret, secErr := s.credentialStore().Get(row.CredentialRef); secErr == nil && strings.TrimSpace(secret) != "" {
				out.HasSecret = true
				out.SecretMasked = maskSecret(secret)
			}
		}
	}
	if out.Edition == "" {
		out.Edition = defaultProviderEdition(eid)
	}
	return out, nil
}

func defaultProviderEdition(id EngineID) string {
	switch id {
	case EngineMicrosoftAzure:
		return "f0"
	case EngineBaidu:
		return "standard"
	default:
		return ""
	}
}

func validateProviderEdition(id EngineID, edition string) (string, bool) {
	e := strings.TrimSpace(edition)
	switch id {
	case EngineMicrosoftAzure:
		if e == "" {
			return "f0", true
		}
		return e, e == "f0" || e == "s1"
	case EngineBaidu:
		if e == "" {
			return "standard", true
		}
		return e, e == "standard" || e == "senior" || e == "premium"
	case EngineYoudao:
		if e == "" {
			return "text", true
		}
		return e, e == "text" || e == "html"
	default:
		return "", e == ""
	}
}

func (s *Service) SaveProviderConfig(ctx context.Context, input SaveProviderInput) (ProviderConfig, error) {
	if s.store == nil {
		return ProviderConfig{}, &ai.ModelError{Code: "StorageError"}
	}
	eid := EngineID(strings.TrimSpace(input.ID))
	if !IsBYOKProvider(eid) {
		return ProviderConfig{}, &ai.ModelError{Code: "ValidationError"}
	}
	appID := strings.TrimSpace(input.AppID)
	region := strings.TrimSpace(input.Region)
	endpoint := strings.TrimSpace(input.Endpoint)
	secret := strings.TrimSpace(input.Secret)
	edition, ok := validateProviderEdition(eid, input.Edition)
	if !ok || len(appID) > 256 || len(region) > 64 || len(secret) > 512 {
		return ProviderConfig{}, &ai.ModelError{Code: "ValidationError"}
	}

	s.credentialMu.Lock()
	defer s.credentialMu.Unlock()
	existing, err := s.store.GetTranslationProvider(ctx, string(eid))
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return ProviderConfig{}, &ai.ModelError{Code: "StorageError"}
	}
	if err := validateProviderEndpointChange(eid, existing, endpoint, secret, input.ClearSecret); err != nil {
		return ProviderConfig{}, err
	}
	credRef := existing.CredentialRef
	creds := s.credentialStore()

	switch {
	case input.ClearSecret:
		credRef = ""
	case secret != "":
		credRef = "translation:" + string(eid) + ":" + rand.Text()
		if setErr := creds.Set(credRef, secret); setErr != nil {
			return ProviderConfig{}, &ai.ModelError{Code: "CredentialError"}
		}
	}

	err = s.store.SaveTranslationProvider(ctx, sqlc.SaveTranslationProviderParams{
		ID:            string(eid),
		AppID:         appID,
		Region:        region,
		Endpoint:      endpoint,
		Edition:       edition,
		CredentialRef: credRef,
		UpdatedAt:     time.Now().UTC().UnixMilli(),
	})
	if err != nil {
		if credRef != "" && credRef != existing.CredentialRef {
			_ = creds.Delete(credRef)
		}
		return ProviderConfig{}, &ai.ModelError{Code: "StorageError"}
	}
	if existing.CredentialRef != "" && existing.CredentialRef != credRef {
		_ = creds.Delete(existing.CredentialRef)
	}
	return s.getProviderConfig(ctx, string(eid))
}

func (s *Service) loadResolvedConfig(ctx context.Context, id EngineID, draft *SaveProviderInput) (resolvedProviderConfig, error) {
	cfg := resolvedProviderConfig{ID: id, Edition: defaultProviderEdition(id)}
	if !IsBYOKProvider(id) {
		return cfg, nil
	}
	s.credentialMu.RLock()
	defer s.credentialMu.RUnlock()
	var existing sqlc.TranslationProvider
	if s.store != nil {
		row, err := s.store.GetTranslationProvider(ctx, string(id))
		if err != nil && !errors.Is(err, database.ErrNotFound) {
			return cfg, &ai.ModelError{Code: "StorageError"}
		}
		if err == nil {
			existing = row
			cfg.AppID = row.AppID
			cfg.Region = row.Region
			cfg.Endpoint = row.Endpoint
			if row.Edition != "" {
				cfg.Edition = row.Edition
			}
		}
	}
	if draft != nil {
		cfg.AppID = strings.TrimSpace(draft.AppID)
		cfg.Region = strings.TrimSpace(draft.Region)
		cfg.Endpoint = strings.TrimSpace(draft.Endpoint)
		secret := strings.TrimSpace(draft.Secret)
		if err := validateProviderEndpointChange(id, existing, cfg.Endpoint, secret, draft.ClearSecret); err != nil {
			return cfg, err
		}
		if ed := strings.TrimSpace(draft.Edition); ed != "" {
			cfg.Edition = ed
		}
		if draft.ClearSecret {
			cfg.Secret = ""
			return cfg, nil
		}
		if secret != "" {
			cfg.Secret = secret
			return cfg, nil
		}
	}
	if _, err := providerOrigin(id, cfg.Endpoint); err != nil {
		return cfg, err
	}
	if existing.CredentialRef != "" {
		secret, err := s.credentialStore().Get(existing.CredentialRef)
		if err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return cfg, &ai.ModelError{Code: "CredentialError"}
		}
		cfg.Secret = strings.TrimSpace(secret)
	}
	return cfg, nil
}

func (s *Service) resolveEngine(ctx context.Context, id EngineID, draft *SaveProviderInput) (Engine, error) {
	if s.engineFactory != nil {
		return s.engineFactory(ctx, id)
	}
	cfg, err := s.loadResolvedConfig(ctx, id, draft)
	if err != nil {
		return nil, err
	}
	client := s.httpClient
	if client == nil {
		client = defaultHTTPClient()
	}
	return buildEngine(cfg, client)
}

// TestProvider 发送一句短文本测试指定翻译方式的连通性与凭据有效性。
func (s *Service) TestProvider(ctx context.Context, id string, draft *SaveProviderInput) TestResult {
	start := time.Now()
	eid := NormalizeEngineID(id)
	if eid == EngineAI {
		return TestResult{OK: false, ErrorCode: "ValidationError", ErrorDetail: "请在「服务与模型」中测试 AI 模型连接。"}
	}
	targetLang := s.defaultTargetLang(ctx)
	sample := "Hello, world!"
	if strings.EqualFold(targetLang, "en") {
		sample = "你好，世界！"
	}

	engine, err := s.resolveEngine(ctx, eid, draft)
	if err != nil {
		return TestResult{OK: false, LatencyMs: time.Since(start).Milliseconds(), ErrorCode: modelErrorCode(err), ErrorDetail: err.Error()}
	}
	if !engine.SupportsLanguagePair("", targetLang) {
		return TestResult{OK: false, LatencyMs: time.Since(start).Milliseconds(), ErrorCode: "UnsupportedLanguage"}
	}

	testCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	res, err := s.callEngineWithLimit(testCtx, engine, BatchRequest{
		Texts:      []string{sample},
		TargetLang: targetLang,
		IsHTML:     false,
	}, false)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return TestResult{OK: false, LatencyMs: latency, ErrorCode: modelErrorCode(err), ErrorDetail: err.Error()}
	}
	out := ""
	if len(res.Translations) > 0 {
		out = strings.TrimSpace(res.Translations[0])
	}
	if out == "" {
		return TestResult{OK: false, LatencyMs: latency, ErrorCode: "InvalidModelOutput"}
	}
	return TestResult{OK: true, SampleOut: out, LatencyMs: latency}
}
