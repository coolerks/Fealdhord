package translation

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/coolerks/Fealdhord/internal/ai"
)

const (
	defaultBaiduEndpoint = "https://fanyi-api.baidu.com/api/trans/vip/translate"
	baiduNewlineToken    = "⟦NL⟧"
)

// baiduEngine 对接百度通用文本翻译 API（模式 B 占位符 + 字节上限 + src 校验 + 三档 QPS）。
type baiduEngine struct {
	client    *http.Client
	appID     string
	secretKey string
	endpoint  string
	edition   string // "standard" (1 QPS) | "senior" (10 QPS) | "premium" (100 QPS)
}

func newBaiduEngine(cfg resolvedProviderConfig, client *http.Client) (*baiduEngine, error) {
	if strings.TrimSpace(cfg.AppID) == "" || strings.TrimSpace(cfg.Secret) == "" {
		return nil, mtError("ConfigurationError", "缺少百度翻译 AppID 或密钥")
	}
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = defaultBaiduEndpoint
	}
	edition := strings.TrimSpace(cfg.Edition)
	if edition == "" {
		edition = "standard"
	}
	return &baiduEngine{
		client:    client,
		appID:     strings.TrimSpace(cfg.AppID),
		secretKey: strings.TrimSpace(cfg.Secret),
		endpoint:  endpoint,
		edition:   edition,
	}, nil
}

func (e *baiduEngine) ID() EngineID { return EngineBaidu }

func (e *baiduEngine) Capabilities() EngineCapabilities {
	qps := 0.95 // 标准版 1 QPS，留 50ms 余量降低 54003 触发率
	switch e.edition {
	case "senior":
		qps = 9.5
	case "premium":
		qps = 80
	}
	return EngineCapabilities{
		SupportsHTML:       false,
		MaxBatchItems:      50,
		MaxBatchChars:      5000,
		MaxBatchBytes:      5500, // 严格满足百度官方 6000 字节上限
		MaxSingleItemChars: 1800,
		DefaultQPS:         qps,
		BackgroundQPS:      0.5,
	}
}

func mapBaiduLang(code string) string {
	if strings.TrimSpace(code) == "" {
		return ""
	}
	switch NormalizeTargetLang(code) {
	case "zh-CN":
		return "zh"
	case "zh-TW":
		return "cht"
	case "en":
		return "en"
	case "ja":
		return "jp"
	case "ko":
		return "kor"
	case "fr":
		return "fra"
	case "de":
		return "de"
	case "es":
		return "spa"
	case "ru":
		return "ru"
	default:
		return ""
	}
}

func (e *baiduEngine) SupportsLanguagePair(_, targetLang string) bool {
	return mapBaiduLang(targetLang) != ""
}

func (e *baiduEngine) TranslateBatch(ctx context.Context, req BatchRequest) (BatchResult, error) {
	if len(req.Texts) == 0 {
		return BatchResult{}, nil
	}
	target := mapBaiduLang(req.TargetLang)
	if target == "" {
		return BatchResult{}, &ai.ModelError{Code: "UnsupportedLanguage"}
	}
	source := "auto"
	if s := mapBaiduLang(req.SourceLang); s != "" {
		source = s
	}

	// 段内换行替换为占位符，防止与批间 \n 分隔符冲突。
	prepared := make([]string, len(req.Texts))
	for i, text := range req.Texts {
		normalized := strings.ReplaceAll(text, "\r\n", "\n")
		prepared[i] = strings.ReplaceAll(normalized, "\n", baiduNewlineToken)
	}
	joined := strings.Join(prepared, "\n")

	pairs, detected, err := e.callAPI(ctx, joined, source, target)
	if err != nil {
		return BatchResult{}, err
	}

	out := make([]string, len(req.Texts))
	matched := make([]bool, len(req.Texts))
	if len(pairs) == len(prepared) {
		for i, pair := range pairs {
			if strings.TrimSpace(pair.Src) == strings.TrimSpace(prepared[i]) {
				out[i] = restoreBaiduNewlines(pair.Dst)
				matched[i] = true
			}
		}
	} else {
		// 仅匹配 src 严格一致的段落，避免整批重复计费
		bySrc := make(map[string][]string, len(pairs))
		for _, pair := range pairs {
			k := strings.TrimSpace(pair.Src)
			bySrc[k] = append(bySrc[k], pair.Dst)
		}
		for i, src := range prepared {
			k := strings.TrimSpace(src)
			if list := bySrc[k]; len(list) > 0 {
				out[i] = restoreBaiduNewlines(list[0])
				bySrc[k] = list[1:]
				matched[i] = true
			}
		}
	}

	failed := make(map[int]error)
	for i, ok := range matched {
		if ok {
			continue
		}
		// 仅对无法确认对应关系的单段单独重试一次
		if req.waitForRetry != nil {
			if err := req.waitForRetry(ctx, utf8.RuneCountInString(prepared[i])); err != nil {
				return BatchResult{}, networkError(err)
			}
		}
		singlePairs, singleDet, singleErr := e.callAPI(ctx, prepared[i], source, target)
		if singleErr != nil || len(singlePairs) == 0 {
			if singleErr == nil {
				singleErr = &ai.ModelError{Code: "InvalidModelOutput"}
			}
			failed[i] = singleErr
			continue
		}
		var sb strings.Builder
		for j, p := range singlePairs {
			if j > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(restoreBaiduNewlines(p.Dst))
		}
		out[i] = sb.String()
		if detected == "" && singleDet != "" {
			detected = singleDet
		}
	}

	res := BatchResult{Translations: out, DetectedLang: detected}
	if len(failed) > 0 {
		res.FailedIndices = failed
	}
	return res, nil
}

func restoreBaiduNewlines(s string) string {
	return strings.ReplaceAll(s, baiduNewlineToken, "\n")
}

type baiduPair struct {
	Src string `json:"src"`
	Dst string `json:"dst"`
}

func (e *baiduEngine) callAPI(ctx context.Context, q, from, to string) ([]baiduPair, string, error) {
	salt := rand.Text()
	sum := md5.Sum([]byte(e.appID + q + salt + e.secretKey))
	sign := hex.EncodeToString(sum[:])

	form := url.Values{
		"q":     {q},
		"from":  {from},
		"to":    {to},
		"appid": {e.appID},
		"salt":  {salt},
		"sign":  {sign},
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, "", &ai.ModelError{Code: "ValidationError"}
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, "", networkError(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, "", networkError(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", httpStatusError(resp.StatusCode, resp.Header, truncateRunes(string(raw), 240))
	}

	var parsed struct {
		From        string      `json:"from"`
		TransResult []baiduPair `json:"trans_result"`
		ErrorCode   string      `json:"error_code"`
		ErrorMsg    string      `json:"error_msg"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, "", &ai.ModelError{Code: "ProviderProtocolError"}
	}
	if parsed.ErrorCode != "" && parsed.ErrorCode != "52000" {
		return nil, "", classifyBaiduError(parsed.ErrorCode, parsed.ErrorMsg)
	}
	return parsed.TransResult, parsed.From, nil
}

func classifyBaiduError(code, msg string) error {
	switch code {
	case "52003", "54001", "58000":
		return mtError("AuthenticationError", fmt.Sprintf("%s: %s", code, msg))
	case "54003", "54005":
		return &ai.ModelError{Code: "RateLimitError"}
	case "58001":
		return mtError("UnsupportedLanguage", msg)
	case "52001":
		return &ai.ModelError{Code: "Timeout"}
	default:
		return mtError("ProviderHTTPError", fmt.Sprintf("%s: %s", code, msg))
	}
}
