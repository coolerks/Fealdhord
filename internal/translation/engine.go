package translation

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/coolerks/Fealdhord/internal/ai"
)

// EngineID 标识文章翻译所使用的后端引擎。
type EngineID string

const (
	EngineAI             EngineID = "ai"
	EngineGoogleFree     EngineID = "google_free"
	EngineGoogleBYOK     EngineID = "google_byok"
	EngineMicrosoftEdge  EngineID = "microsoft_edge"
	EngineMicrosoftAzure EngineID = "microsoft_azure"
	EngineAlibaba        EngineID = "alibaba"
	EngineCaiyun         EngineID = "caiyun"
	EngineBaidu          EngineID = "baidu"
	EngineYoudao         EngineID = "youdao"
)

// ProviderMeta 描述一个翻译方式的展示元信息与能力。
type ProviderMeta struct {
	ID                  string   `json:"id"`
	Label               string   `json:"label"`
	Group               string   `json:"group"` // "ai" | "free" | "byok"
	Aliases             []string `json:"aliases"`
	RequiresCredentials bool     `json:"requiresCredentials"`
	UnofficialFree      bool     `json:"unofficialFree"`
	SupportedLanguages  []string `json:"supportedLanguages"`
}

// EngineCapabilities 声明机器翻译引擎的分批与速率限制能力。
type EngineCapabilities struct {
	SupportsHTML       bool    // 模式 A（原生 HTML）或模式 B（占位符纯文本）
	MaxBatchItems      int     // 单次请求最大段数
	MaxBatchChars      int     // 单次请求最大总字符数（按 rune）
	MaxBatchBytes      int     // 可选：单次请求最大 UTF-8 字节数（如百度 5500、Google v2 JSON 90KB）
	MaxSingleItemChars int     // 单段最大字符数（超出则拆分）
	DefaultQPS         float64 // 前台 QPS 上限
	BackgroundQPS      float64 // 后台自动翻译更保守的 QPS 上限
	CharsPerMinute     int     // 可选：每分钟字符窗口（如 Azure F0 = 30000）
	BackgroundCPM      int     // 可选：后台每分钟字符窗口
}

// BatchRequest 表示发送给机器翻译引擎的一批文本。
type BatchRequest struct {
	Texts      []string
	SourceLang string // "" 表示自动检测
	TargetLang string // 内部标准码（zh-CN / zh-TW / en / ja / ko / fr / de / es / ru）
	IsHTML     bool
}

// BatchResult 表示机器翻译引擎返回的批量结果。
type BatchResult struct {
	Translations []string
	DetectedLang string
	// FailedIndices 可选：记录本批中部分失败但未阻断整批的下标（例如百度单段校验失配）。
	FailedIndices map[int]error
}

// Engine 是所有机器翻译 API 引擎的统一执行接口。
type Engine interface {
	ID() EngineID
	Capabilities() EngineCapabilities
	SupportsLanguagePair(sourceLang, targetLang string) bool
	TranslateBatch(ctx context.Context, req BatchRequest) (BatchResult, error)
}

var allLangCodes = []string{"zh-CN", "zh-TW", "en", "ja", "ko", "fr", "de", "es", "ru"}
var caiyunLangCodes = []string{"zh-CN", "zh-TW", "en", "ja", "ko"}

// SupportedProviders 返回界面可选择的全部翻译方式列表。
func SupportedProviders() []ProviderMeta {
	return []ProviderMeta{
		{
			ID: string(EngineAI), Label: "AI 模型", Group: "ai",
			Aliases: []string{"ai", "llm", "model", "大模型", "模型"}, SupportedLanguages: allLangCodes,
		},
		{
			ID: string(EngineMicrosoftEdge), Label: "Microsoft Edge（免费）", Group: "free",
			Aliases: []string{"edge", "microsoft", "bing", "微软", "免费"}, UnofficialFree: true, SupportedLanguages: allLangCodes,
		},
		{
			ID: string(EngineGoogleFree), Label: "Google 翻译（免费）", Group: "free",
			Aliases: []string{"google", "gtx", "te_lib", "谷歌", "免费"}, UnofficialFree: true, SupportedLanguages: allLangCodes,
		},
		{
			ID: string(EngineMicrosoftAzure), Label: "Microsoft Azure", Group: "byok",
			Aliases: []string{"azure", "microsoft", "微软", "translator"}, RequiresCredentials: true, SupportedLanguages: allLangCodes,
		},
		{
			ID: string(EngineGoogleBYOK), Label: "Google Cloud", Group: "byok",
			Aliases: []string{"google", "gcp", "cloud", "谷歌"}, RequiresCredentials: true, SupportedLanguages: allLangCodes,
		},
		{
			ID: string(EngineAlibaba), Label: "阿里云机器翻译", Group: "byok",
			Aliases: []string{"alibaba", "aliyun", "ali", "阿里", "阿里云"}, RequiresCredentials: true, SupportedLanguages: allLangCodes,
		},
		{
			ID: string(EngineCaiyun), Label: "彩云小译", Group: "byok",
			Aliases: []string{"caiyun", "lingocloud", "彩云", "彩云小译"}, RequiresCredentials: true, SupportedLanguages: caiyunLangCodes,
		},
		{
			ID: string(EngineBaidu), Label: "百度翻译", Group: "byok",
			Aliases: []string{"baidu", "fanyi", "百度"}, RequiresCredentials: true, SupportedLanguages: allLangCodes,
		},
		{
			ID: string(EngineYoudao), Label: "有道智云", Group: "byok",
			Aliases: []string{"youdao", "netease", "有道", "网易有道"}, RequiresCredentials: true, SupportedLanguages: allLangCodes,
		},
	}
}

func NormalizeEngineID(raw string) EngineID {
	switch EngineID(strings.TrimSpace(raw)) {
	case EngineAI, EngineGoogleFree, EngineGoogleBYOK, EngineMicrosoftEdge, EngineMicrosoftAzure,
		EngineAlibaba, EngineCaiyun, EngineBaidu, EngineYoudao:
		return EngineID(strings.TrimSpace(raw))
	default:
		return EngineAI
	}
}

func IsBYOKProvider(id EngineID) bool {
	switch id {
	case EngineGoogleBYOK, EngineMicrosoftAzure, EngineAlibaba, EngineCaiyun, EngineBaidu, EngineYoudao:
		return true
	default:
		return false
	}
}

func defaultHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyFromEnvironment
	transport.MaxIdleConns = 16
	transport.MaxIdleConnsPerHost = 4
	transport.IdleConnTimeout = 90 * time.Second
	return &http.Client{
		Transport: transport,
		Timeout:   45 * time.Second,
	}
}

func parseRetryAfter(header string) time.Duration {
	v := strings.TrimSpace(header)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

type detailError struct {
	base   *ai.ModelError
	detail string
}

func (e *detailError) Error() string {
	if e.detail != "" {
		return e.base.Code + ": " + e.detail
	}
	return e.base.Code
}

func (e *detailError) Unwrap() error { return e.base }

func mtError(code, detail string) error {
	base := &ai.ModelError{Code: code}
	if strings.TrimSpace(detail) == "" {
		return base
	}
	return &detailError{base: base, detail: strings.TrimSpace(detail)}
}

func httpStatusError(status int, header http.Header, detail string) error {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return mtError("AuthenticationError", detail)
	case status == http.StatusTooManyRequests:
		base := &ai.ModelError{Code: "RateLimitError", RetryAfter: parseRetryAfter(header.Get("Retry-After"))}
		if strings.TrimSpace(detail) == "" {
			return base
		}
		return &detailError{base: base, detail: strings.TrimSpace(detail)}
	case status == http.StatusRequestTimeout || status == http.StatusGatewayTimeout:
		return mtError("Timeout", detail)
	default:
		return mtError("ProviderHTTPError", detail)
	}
}

func networkError(err error) error {
	if err == nil {
		return nil
	}
	if err == context.Canceled {
		return &ai.ModelError{Code: "Cancelled"}
	}
	if err == context.DeadlineExceeded {
		return &ai.ModelError{Code: "Timeout"}
	}
	return mtError("ProviderConnectionError", err.Error())
}
