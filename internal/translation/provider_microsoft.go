package translation

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/coolerks/Fealdhord/internal/ai"
)

const (
	// 新版 Microsoft Edge 免费批量翻译接口（无需已失效的 /translate/auth JWT）。
	defaultMicrosoftEdgeURL  = "https://edge.microsoft.com/translate/translatetext"
	defaultMicrosoftAzureURL = "https://api.cognitive.microsofttranslator.com/translate"
)

func mapMicrosoftLang(code string) string {
	if strings.TrimSpace(code) == "" {
		return ""
	}
	switch NormalizeTargetLang(code) {
	case "zh-CN":
		return "zh-Hans"
	case "zh-TW":
		return "zh-Hant"
	case "en", "ja", "ko", "fr", "de", "es", "ru":
		return NormalizeTargetLang(code)
	default:
		return ""
	}
}

// microsoftEdgeEngine 使用新版 Edge translatetext 免费通道（模式 B 纯文本占位符）。
type microsoftEdgeEngine struct {
	client   *http.Client
	endpoint string
}

func newMicrosoftEdgeEngine(client *http.Client) *microsoftEdgeEngine {
	return &microsoftEdgeEngine{
		client:   client,
		endpoint: defaultMicrosoftEdgeURL,
	}
}

func (e *microsoftEdgeEngine) ID() EngineID { return EngineMicrosoftEdge }

func (e *microsoftEdgeEngine) Capabilities() EngineCapabilities {
	return EngineCapabilities{
		SupportsHTML:       false, // Edge translatetext 为纯文本数组，走模式 B 占位符桥接
		MaxBatchItems:      100,
		MaxBatchChars:      20000,
		MaxSingleItemChars: 10000,
		DefaultQPS:         2,
		BackgroundQPS:      0.5,
	}
}

func (e *microsoftEdgeEngine) SupportsLanguagePair(_, targetLang string) bool {
	return mapMicrosoftLang(targetLang) != ""
}

func (e *microsoftEdgeEngine) TranslateBatch(ctx context.Context, req BatchRequest) (BatchResult, error) {
	if len(req.Texts) == 0 {
		return BatchResult{}, nil
	}
	target := mapMicrosoftLang(req.TargetLang)
	if target == "" {
		return BatchResult{}, &ai.ModelError{Code: "UnsupportedLanguage"}
	}
	u, err := url.Parse(e.endpoint)
	if err != nil {
		return BatchResult{}, &ai.ModelError{Code: "ValidationError"}
	}
	q := u.Query()
	q.Set("from", mapMicrosoftLang(req.SourceLang))
	q.Set("to", target)
	q.Set("isEnterpriseClient", "false")
	u.RawQuery = q.Encode()

	body, err := json.Marshal(req.Texts)
	if err != nil {
		return BatchResult{}, &ai.ModelError{Code: "ValidationError"}
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return BatchResult{}, &ai.ModelError{Code: "ValidationError"}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 Edg/131.0.0.0")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return BatchResult{}, networkError(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return BatchResult{}, networkError(err)
	}
	if resp.StatusCode != http.StatusOK {
		return BatchResult{}, httpStatusError(resp.StatusCode, resp.Header, truncateRunes(string(raw), 240))
	}

	var parsed []struct {
		DetectedLanguage *struct {
			Language string `json:"language"`
		} `json:"detectedLanguage"`
		Translations []struct {
			Text string `json:"text"`
		} `json:"translations"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed) != len(req.Texts) {
		return BatchResult{}, &ai.ModelError{Code: "ProviderProtocolError"}
	}
	out := make([]string, len(req.Texts))
	detected := ""
	for i, item := range parsed {
		if len(item.Translations) == 0 {
			return BatchResult{}, &ai.ModelError{Code: "ProviderProtocolError"}
		}
		out[i] = item.Translations[0].Text
		if detected == "" && item.DetectedLanguage != nil && item.DetectedLanguage.Language != "" {
			detected = item.DetectedLanguage.Language
		}
	}
	return BatchResult{Translations: out, DetectedLang: detected}, nil
}

// microsoftAzureEngine 对接官方 Azure AI Translator v3.0（模式 A 原生 HTML + 字符速率窗口限流）。
type microsoftAzureEngine struct {
	client   *http.Client
	apiKey   string
	region   string
	endpoint string
	edition  string // "f0" | "s1"
}

func newMicrosoftAzureEngine(cfg resolvedProviderConfig, client *http.Client) (*microsoftAzureEngine, error) {
	if strings.TrimSpace(cfg.Secret) == "" {
		return nil, mtError("ConfigurationError", "缺少 Azure Translator 密钥")
	}
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = defaultMicrosoftAzureURL
	}
	edition := strings.TrimSpace(cfg.Edition)
	if edition == "" {
		edition = "f0"
	}
	return &microsoftAzureEngine{
		client:   client,
		apiKey:   strings.TrimSpace(cfg.Secret),
		region:   strings.TrimSpace(cfg.Region),
		endpoint: endpoint,
		edition:  edition,
	}, nil
}

func (e *microsoftAzureEngine) ID() EngineID { return EngineMicrosoftAzure }

func (e *microsoftAzureEngine) Capabilities() EngineCapabilities {
	if e.edition == "s1" {
		return EngineCapabilities{
			SupportsHTML:       true,
			MaxBatchItems:      100,
			MaxBatchChars:      45000,
			MaxSingleItemChars: 45000,
			DefaultQPS:         10,
			BackgroundQPS:      2,
		}
	}
	// F0 免费层：官方每小时 200 万字符均匀分布（约 33,300 字符/分钟），按 30,000 字符/分钟限流避免 429。
	return EngineCapabilities{
		SupportsHTML:       true,
		MaxBatchItems:      100,
		MaxBatchChars:      25000,
		MaxSingleItemChars: 25000,
		DefaultQPS:         4,
		BackgroundQPS:      1,
		CharsPerMinute:     30000,
		BackgroundCPM:      15000,
	}
}

func (e *microsoftAzureEngine) SupportsLanguagePair(_, targetLang string) bool {
	return mapMicrosoftLang(targetLang) != ""
}

func (e *microsoftAzureEngine) TranslateBatch(ctx context.Context, req BatchRequest) (BatchResult, error) {
	if len(req.Texts) == 0 {
		return BatchResult{}, nil
	}
	target := mapMicrosoftLang(req.TargetLang)
	if target == "" {
		return BatchResult{}, &ai.ModelError{Code: "UnsupportedLanguage"}
	}
	u, err := url.Parse(e.endpoint)
	if err != nil {
		return BatchResult{}, &ai.ModelError{Code: "ValidationError"}
	}
	q := u.Query()
	q.Set("api-version", "3.0")
	q.Set("to", target)
	if src := mapMicrosoftLang(req.SourceLang); src != "" {
		q.Set("from", src)
	}
	if req.IsHTML {
		q.Set("textType", "html")
	} else {
		q.Set("textType", "plain")
	}
	u.RawQuery = q.Encode()

	items := make([]struct {
		Text string `json:"Text"`
	}, len(req.Texts))
	for i, text := range req.Texts {
		items[i].Text = text
	}
	body, err := json.Marshal(items)
	if err != nil {
		return BatchResult{}, &ai.ModelError{Code: "ValidationError"}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return BatchResult{}, &ai.ModelError{Code: "ValidationError"}
	}
	httpReq.Header.Set("Content-Type", "application/json; charset=UTF-8")
	httpReq.Header.Set("Ocp-Apim-Subscription-Key", e.apiKey)
	if e.region != "" {
		httpReq.Header.Set("Ocp-Apim-Subscription-Region", e.region)
	}

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return BatchResult{}, networkError(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return BatchResult{}, networkError(err)
	}
	if resp.StatusCode != http.StatusOK {
		return BatchResult{}, httpStatusError(resp.StatusCode, resp.Header, truncateRunes(string(raw), 240))
	}

	var parsed []struct {
		DetectedLanguage *struct {
			Language string `json:"language"`
		} `json:"detectedLanguage"`
		Translations []struct {
			Text string `json:"text"`
		} `json:"translations"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed) != len(req.Texts) {
		return BatchResult{}, &ai.ModelError{Code: "ProviderProtocolError"}
	}
	out := make([]string, len(req.Texts))
	detected := ""
	for i, item := range parsed {
		if len(item.Translations) == 0 {
			return BatchResult{}, &ai.ModelError{Code: "ProviderProtocolError"}
		}
		out[i] = item.Translations[0].Text
		if detected == "" && item.DetectedLanguage != nil && item.DetectedLanguage.Language != "" {
			detected = item.DetectedLanguage.Language
		}
	}
	return BatchResult{Translations: out, DetectedLang: detected}, nil
}
