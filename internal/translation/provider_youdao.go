package translation

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coolerks/Fealdhord/internal/ai"
)

const (
	defaultYoudaoBatchEndpoint = "https://openapi.youdao.com/v2/api"
	defaultYoudaoHTMLEndpoint  = "https://openapi.youdao.com/translate_html"
)

// youdaoEngine 对接有道智云翻译：
// - 默认走 /v2/api 批量文本翻译（模式 B 占位符，先将全部 q 无分隔符拼接再统一 truncate 计算 v3 签名）；
// - 若用户启用了 edition="html"（已在有道控制台额外开通网页翻译服务），正文可走 /translate_html。
type youdaoEngine struct {
	client    *http.Client
	appKey    string
	appSecret string
	endpoint  string
	edition   string // "text" (默认) | "html"
}

func newYoudaoEngine(cfg resolvedProviderConfig, client *http.Client) (*youdaoEngine, error) {
	if strings.TrimSpace(cfg.AppID) == "" || strings.TrimSpace(cfg.Secret) == "" {
		return nil, mtError("ConfigurationError", "缺少有道智云 AppKey 或 AppSecret")
	}
	edition := strings.TrimSpace(cfg.Edition)
	if edition == "" {
		edition = "text"
	}
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		if edition == "html" {
			endpoint = defaultYoudaoHTMLEndpoint
		} else {
			endpoint = defaultYoudaoBatchEndpoint
		}
	}
	return &youdaoEngine{
		client:    client,
		appKey:    strings.TrimSpace(cfg.AppID),
		appSecret: strings.TrimSpace(cfg.Secret),
		endpoint:  endpoint,
		edition:   edition,
	}, nil
}

func (e *youdaoEngine) ID() EngineID { return EngineYoudao }

func (e *youdaoEngine) Capabilities() EngineCapabilities {
	if e.edition == "html" {
		return EngineCapabilities{
			SupportsHTML:       true,
			MaxBatchItems:      1,
			MaxBatchChars:      50000,
			MaxSingleItemChars: 50000,
			DefaultQPS:         4,
			BackgroundQPS:      1,
		}
	}
	return EngineCapabilities{
		SupportsHTML:       false,
		MaxBatchItems:      20,
		MaxBatchChars:      4500,
		MaxSingleItemChars: 4000,
		DefaultQPS:         4,
		BackgroundQPS:      1,
	}
}

func mapYoudaoLang(code string) string {
	if strings.TrimSpace(code) == "" {
		return ""
	}
	switch NormalizeTargetLang(code) {
	case "zh-CN":
		return "zh-CHS"
	case "zh-TW":
		return "zh-CHT"
	case "en", "ja", "ko", "fr", "de", "es", "ru":
		return NormalizeTargetLang(code)
	default:
		return ""
	}
}

func (e *youdaoEngine) SupportsLanguagePair(_, targetLang string) bool {
	return mapYoudaoLang(targetLang) != ""
}

// youdaoTruncate 计算有道 v3 签名的 input：
// 当长度 <= 20 时返回原串；大于 20 时返回 前10字符 + 总长度 + 后10字符。
func youdaoTruncate(s string) string {
	r := []rune(s)
	n := len(r)
	if n <= 20 {
		return s
	}
	return string(r[:10]) + strconv.Itoa(n) + string(r[n-10:])
}

// signYoudaoV3 严格按官方规范对所有 q 无分隔符拼接后统一 truncate 再计算 SHA256。
func signYoudaoV3(appKey, appSecret, salt, curtime string, texts []string) string {
	joined := strings.Join(texts, "")
	input := youdaoTruncate(joined)
	sum := sha256.Sum256([]byte(appKey + input + salt + curtime + appSecret))
	return hex.EncodeToString(sum[:])
}

func (e *youdaoEngine) TranslateBatch(ctx context.Context, req BatchRequest) (BatchResult, error) {
	if len(req.Texts) == 0 {
		return BatchResult{}, nil
	}
	target := mapYoudaoLang(req.TargetLang)
	if target == "" {
		return BatchResult{}, &ai.ModelError{Code: "UnsupportedLanguage"}
	}
	source := "auto"
	if s := mapYoudaoLang(req.SourceLang); s != "" {
		source = s
	}

	if e.edition == "html" && req.IsHTML && len(req.Texts) == 1 {
		return e.callTranslateHTML(ctx, req.Texts[0], source, target)
	}
	return e.callBatchText(ctx, req.Texts, source, target)
}

func (e *youdaoEngine) callBatchText(ctx context.Context, texts []string, source, target string) (BatchResult, error) {
	salt := rand.Text()
	curtime := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	sign := signYoudaoV3(e.appKey, e.appSecret, salt, curtime, texts)

	form := url.Values{
		"from":     {source},
		"to":       {target},
		"appKey":   {e.appKey},
		"salt":     {salt},
		"sign":     {sign},
		"signType": {"v3"},
		"curtime":  {curtime},
	}
	for _, text := range texts {
		form.Add("q", text)
	}

	endpoint := e.endpoint
	if e.edition == "html" && endpoint == defaultYoudaoHTMLEndpoint {
		endpoint = defaultYoudaoBatchEndpoint
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return BatchResult{}, &ai.ModelError{Code: "ValidationError"}
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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
		ErrorCode        string `json:"errorCode"`
		TranslateResults []struct {
			Query       string `json:"query"`
			Translation string `json:"translation"`
			Type        string `json:"type"`
		} `json:"translateResults"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return BatchResult{}, &ai.ModelError{Code: "ProviderProtocolError"}
	}
	if parsed.ErrorCode != "" && parsed.ErrorCode != "0" {
		return BatchResult{}, classifyYoudaoError(parsed.ErrorCode)
	}
	if len(parsed.TranslateResults) != len(texts) {
		return BatchResult{}, &ai.ModelError{Code: "ProviderProtocolError"}
	}
	out := make([]string, len(texts))
	detected := ""
	for i, item := range parsed.TranslateResults {
		out[i] = item.Translation
		if detected == "" && item.Type != "" {
			if before, _, ok := strings.Cut(item.Type, "2"); ok {
				detected = before
			}
		}
	}
	return BatchResult{Translations: out, DetectedLang: detected}, nil
}

func (e *youdaoEngine) callTranslateHTML(ctx context.Context, htmlText, source, target string) (BatchResult, error) {
	salt := rand.Text()
	curtime := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	sign := signYoudaoV3(e.appKey, e.appSecret, salt, curtime, []string{htmlText})

	form := url.Values{
		"q":        {htmlText},
		"from":     {source},
		"to":       {target},
		"appKey":   {e.appKey},
		"salt":     {salt},
		"sign":     {sign},
		"signType": {"v3"},
		"curtime":  {curtime},
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return BatchResult{}, &ai.ModelError{Code: "ValidationError"}
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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
		ErrorCode string `json:"errorCode"`
		Data      struct {
			Translation string `json:"translation"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return BatchResult{}, &ai.ModelError{Code: "ProviderProtocolError"}
	}
	if parsed.ErrorCode != "" && parsed.ErrorCode != "0" {
		return BatchResult{}, classifyYoudaoError(parsed.ErrorCode)
	}
	return BatchResult{Translations: []string{parsed.Data.Translation}}, nil
}

func classifyYoudaoError(code string) error {
	switch code {
	case "108", "202", "206", "207", "401":
		return mtError("AuthenticationError", fmt.Sprintf("errorCode=%s", code))
	case "411", "412":
		return &ai.ModelError{Code: "RateLimitError"}
	case "102":
		return &ai.ModelError{Code: "UnsupportedLanguage"}
	default:
		return mtError("ProviderHTTPError", fmt.Sprintf("errorCode=%s", code))
	}
}
