package translation

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/coolerks/Fealdhord/internal/ai"
)

const defaultCaiyunEndpoint = "https://api.interpreter.caiyunai.com/v1/translator"

// caiyunEngine 对接彩云小译 API（模式 B 占位符纯文本 + 星型语种方向矩阵校验）。
type caiyunEngine struct {
	client   *http.Client
	token    string
	endpoint string
}

func newCaiyunEngine(cfg resolvedProviderConfig, client *http.Client) (*caiyunEngine, error) {
	if strings.TrimSpace(cfg.Secret) == "" {
		return nil, mtError("ConfigurationError", "缺少彩云小译 Token")
	}
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = defaultCaiyunEndpoint
	}
	return &caiyunEngine{
		client:   client,
		token:    strings.TrimSpace(cfg.Secret),
		endpoint: endpoint,
	}, nil
}

func (e *caiyunEngine) ID() EngineID { return EngineCaiyun }

func (e *caiyunEngine) Capabilities() EngineCapabilities {
	return EngineCapabilities{
		SupportsHTML:       false, // 官方仅明确定义 media="text"，走模式 B 本地占位符桥接
		MaxBatchItems:      50,
		MaxBatchChars:      4500,
		MaxSingleItemChars: 1000,
		DefaultQPS:         2,
		BackgroundQPS:      0.5,
	}
}

func mapCaiyunLang(code string) string {
	if strings.TrimSpace(code) == "" {
		return ""
	}
	switch NormalizeTargetLang(code) {
	case "zh-CN":
		return "zh"
	case "zh-TW":
		return "zh-Hant"
	case "en", "ja", "ko", "de", "fr", "es", "ru":
		return NormalizeTargetLang(code)
	default:
		return ""
	}
}

func isChineseCode(code string) bool {
	c := NormalizeTargetLang(code)
	return c == "zh-CN" || c == "zh-TW"
}

// SupportsLanguagePair 遵循彩云小译官方星型语言方向表：
// - 任意支持源语言（中/英/日/韩/德/法/西/俄或 auto）→ 简中 / 繁中
// - 简中 / 繁中（或未知 auto）→ 英 / 日 / 韩
// - 非中文之间（如英→日、德→英）不支持。
func (e *caiyunEngine) SupportsLanguagePair(sourceLang, targetLang string) bool {
	target := NormalizeTargetLang(targetLang)
	switch target {
	case "zh-CN", "zh-TW":
		if strings.TrimSpace(sourceLang) == "" {
			return true
		}
		return mapCaiyunLang(sourceLang) != ""
	case "en", "ja", "ko":
		if strings.TrimSpace(sourceLang) == "" {
			return true
		}
		return isChineseCode(sourceLang)
	default:
		return false
	}
}

func (e *caiyunEngine) TranslateBatch(ctx context.Context, req BatchRequest) (BatchResult, error) {
	if len(req.Texts) == 0 {
		return BatchResult{}, nil
	}
	if !e.SupportsLanguagePair(req.SourceLang, req.TargetLang) {
		return BatchResult{}, &ai.ModelError{Code: "UnsupportedLanguage"}
	}
	target := mapCaiyunLang(req.TargetLang)
	source := "auto"
	detect := true
	if strings.TrimSpace(req.SourceLang) != "" {
		if s := mapCaiyunLang(req.SourceLang); s != "" {
			source = s
			detect = false
		}
	}
	payload := struct {
		Source    []string `json:"source"`
		TransType string   `json:"trans_type"`
		Media     string   `json:"media"`
		Detect    bool     `json:"detect"`
	}{
		Source:    req.Texts,
		TransType: source + "2" + target,
		Media:     "text",
		Detect:    detect,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return BatchResult{}, &ai.ModelError{Code: "ValidationError"}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, bytes.NewReader(body))
	if err != nil {
		return BatchResult{}, &ai.ModelError{Code: "ValidationError"}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Authorization", "token "+e.token)

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

	var parsed struct {
		Target  []string `json:"target"`
		Message string   `json:"message"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return BatchResult{}, &ai.ModelError{Code: "ProviderProtocolError"}
	}
	if parsed.Message != "" && len(parsed.Target) == 0 {
		if strings.Contains(strings.ToLower(parsed.Message), "token") || strings.Contains(strings.ToLower(parsed.Message), "auth") {
			return BatchResult{}, mtError("AuthenticationError", parsed.Message)
		}
		return BatchResult{}, mtError("ProviderHTTPError", parsed.Message)
	}
	if len(parsed.Target) != len(req.Texts) {
		return BatchResult{}, &ai.ModelError{Code: "ProviderProtocolError"}
	}
	return BatchResult{Translations: parsed.Target}, nil
}
