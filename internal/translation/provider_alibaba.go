package translation

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/coolerks/Fealdhord/internal/ai"
)

const (
	defaultAlibabaEndpoint = "https://mt.cn-hangzhou.aliyuncs.com/"
	alibabaAPIVersion      = "2018-10-12"
)

type alibabaEngine struct {
	client          *http.Client
	accessKeyID     string
	accessKeySecret string
	endpoint        string
}

func newAlibabaEngine(cfg resolvedProviderConfig, client *http.Client) (*alibabaEngine, error) {
	if strings.TrimSpace(cfg.AppID) == "" || strings.TrimSpace(cfg.Secret) == "" {
		return nil, mtError("ConfigurationError", "缺少阿里云 AccessKey ID 或 AccessKey Secret")
	}
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = defaultAlibabaEndpoint
	}
	return &alibabaEngine{
		client:          client,
		accessKeyID:     strings.TrimSpace(cfg.AppID),
		accessKeySecret: strings.TrimSpace(cfg.Secret),
		endpoint:        endpoint,
	}, nil
}

func (e *alibabaEngine) ID() EngineID { return EngineAlibaba }

func (e *alibabaEngine) Capabilities() EngineCapabilities {
	// 单段 1001..5000 走 TranslateGeneral，<=1000 走 GetBatchTranslate（单批 <=50 条且总计 <=8000 UTF-16 字符）。
	return EngineCapabilities{
		SupportsHTML:       true,
		MaxBatchItems:      50,
		MaxBatchChars:      7800, // 预留余量满足官方 8000 UTF-16 字符上限
		MaxSingleItemChars: 4800, // <=5000 走 TranslateGeneral，>5000 再拆段
		DefaultQPS:         10,
		BackgroundQPS:      2,
	}
}

func mapAlibabaLang(code string) string {
	if strings.TrimSpace(code) == "" {
		return ""
	}
	switch NormalizeTargetLang(code) {
	case "zh-CN":
		return "zh"
	case "zh-TW":
		return "zh-tw"
	case "en", "ja", "ko", "fr", "de", "es", "ru":
		return NormalizeTargetLang(code)
	default:
		return ""
	}
}

func (e *alibabaEngine) SupportsLanguagePair(_, targetLang string) bool {
	return mapAlibabaLang(targetLang) != ""
}

func utf16Len(s string) int {
	return len(utf16.Encode([]rune(s)))
}

func (e *alibabaEngine) TranslateBatch(ctx context.Context, req BatchRequest) (BatchResult, error) {
	if len(req.Texts) == 0 {
		return BatchResult{}, nil
	}
	target := mapAlibabaLang(req.TargetLang)
	if target == "" {
		return BatchResult{}, &ai.ModelError{Code: "UnsupportedLanguage"}
	}
	source := "auto"
	if s := mapAlibabaLang(req.SourceLang); s != "" {
		source = s
	}
	formatType := "text"
	if req.IsHTML {
		formatType = "html"
	}

	// 检查是否存在超过 1000 UTF-16 字符的段落：
	// - 若超过 1000 字符，逐段走 TranslateGeneral（上限 5000 字符），其余 ≤1000 字符打包走 GetBatchTranslate。
	out := make([]string, len(req.Texts))
	failed := make(map[int]error)
	var detected string

	var batchIndices []int
	batchTotal := 0

	flushBatch := func() error {
		if len(batchIndices) == 0 {
			return nil
		}
		batchTexts := make([]string, len(batchIndices))
		for i, origIdx := range batchIndices {
			batchTexts[i] = req.Texts[origIdx]
		}
		subRes, err := e.callGetBatchTranslate(ctx, batchTexts, source, target, formatType)
		if err != nil {
			return err
		}
		if detected == "" && subRes.DetectedLang != "" {
			detected = subRes.DetectedLang
		}
		for i, origIdx := range batchIndices {
			if subRes.FailedIndices != nil && subRes.FailedIndices[i] != nil {
				failed[origIdx] = subRes.FailedIndices[i]
			} else if i < len(subRes.Translations) {
				out[origIdx] = subRes.Translations[i]
			}
		}
		batchIndices = nil
		batchTotal = 0
		return nil
	}

	for i, text := range req.Texts {
		uLen := utf16Len(text)
		if uLen > 1000 {
			if err := flushBatch(); err != nil {
				return BatchResult{}, err
			}
			translated, det, err := e.callTranslateGeneral(ctx, text, source, target, formatType)
			if err != nil {
				return BatchResult{}, err
			}
			out[i] = translated
			if detected == "" && det != "" {
				detected = det
			}
			continue
		}
		if len(batchIndices) >= 50 || batchTotal+uLen > 7800 {
			if err := flushBatch(); err != nil {
				return BatchResult{}, err
			}
		}
		batchIndices = append(batchIndices, i)
		batchTotal += uLen
	}
	if err := flushBatch(); err != nil {
		return BatchResult{}, err
	}

	res := BatchResult{Translations: out, DetectedLang: detected}
	if len(failed) > 0 {
		res.FailedIndices = failed
	}
	return res, nil
}

func (e *alibabaEngine) callTranslateGeneral(ctx context.Context, text, source, target, formatType string) (string, string, error) {
	form := url.Values{
		"FormatType":     {formatType},
		"SourceLanguage": {source},
		"TargetLanguage": {target},
		"SourceText":     {text},
		"Scene":          {"general"},
	}
	raw, err := e.doSignedRequest(ctx, "TranslateGeneral", form)
	if err != nil {
		return "", "", err
	}
	var resp struct {
		Code    string `json:"Code"`
		Message string `json:"Message"`
		Data    struct {
			Translated       string `json:"Translated"`
			DetectedLanguage string `json:"DetectedLanguage"`
		} `json:"Data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", "", &ai.ModelError{Code: "ProviderProtocolError"}
	}
	if resp.Code != "" && resp.Code != "200" {
		return "", "", classifyAlibabaError(resp.Code, resp.Message)
	}
	return resp.Data.Translated, resp.Data.DetectedLanguage, nil
}

func (e *alibabaEngine) callGetBatchTranslate(ctx context.Context, texts []string, source, target, formatType string) (BatchResult, error) {
	srcMap := make(map[string]string, len(texts))
	for i, text := range texts {
		srcMap[strconv.Itoa(i)] = text
	}
	srcJSON, err := json.Marshal(srcMap)
	if err != nil {
		return BatchResult{}, &ai.ModelError{Code: "ValidationError"}
	}
	form := url.Values{
		"ApiType":        {"translate_standard"},
		"FormatType":     {formatType},
		"Scene":          {"general"},
		"SourceLanguage": {source},
		"TargetLanguage": {target},
		"SourceText":     {string(srcJSON)},
	}
	raw, err := e.doSignedRequest(ctx, "GetBatchTranslate", form)
	if err != nil {
		return BatchResult{}, err
	}
	var resp struct {
		Code           string `json:"Code"`
		Message        string `json:"Message"`
		TranslatedList []struct {
			Code             string `json:"code"`
			Index            string `json:"index"`
			Translated       string `json:"translated"`
			DetectedLanguage string `json:"detectedLanguage"`
			ErrorMsg         string `json:"errorMsg"`
		} `json:"TranslatedList"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return BatchResult{}, &ai.ModelError{Code: "ProviderProtocolError"}
	}
	if resp.Code != "" && resp.Code != "200" {
		return BatchResult{}, classifyAlibabaError(resp.Code, resp.Message)
	}
	out := make([]string, len(texts))
	failed := make(map[int]error)
	detected := ""
	seen := make([]bool, len(texts))
	for _, item := range resp.TranslatedList {
		idx, err := strconv.Atoi(strings.TrimSpace(item.Index))
		if err != nil || idx < 0 || idx >= len(texts) {
			continue
		}
		seen[idx] = true
		if item.Code != "" && item.Code != "200" {
			failed[idx] = classifyAlibabaError(item.Code, item.ErrorMsg)
			continue
		}
		out[idx] = item.Translated
		if detected == "" && item.DetectedLanguage != "" {
			detected = item.DetectedLanguage
		}
	}
	for i, ok := range seen {
		if !ok {
			failed[i] = &ai.ModelError{Code: "InvalidModelOutput"}
		}
	}
	res := BatchResult{Translations: out, DetectedLang: detected}
	if len(failed) > 0 {
		res.FailedIndices = failed
	}
	return res, nil
}

func (e *alibabaEngine) doSignedRequest(ctx context.Context, action string, form url.Values) ([]byte, error) {
	u, err := url.Parse(e.endpoint)
	if err != nil {
		return nil, &ai.ModelError{Code: "ValidationError"}
	}
	if u.Path == "" {
		u.Path = "/"
	}
	bodyStr := form.Encode()
	bodyBytes := []byte(bodyStr)
	nonce := rand.Text()
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	headers := map[string]string{
		"content-type":          "application/x-www-form-urlencoded",
		"host":                  u.Host,
		"x-acs-action":          action,
		"x-acs-content-sha256":  sha256Hex(bodyBytes),
		"x-acs-date":            now,
		"x-acs-signature-nonce": nonce,
		"x-acs-version":         alibabaAPIVersion,
	}
	authHeader := signAlibabaACS3(http.MethodPost, u.Path, u.Query(), headers, bodyBytes, e.accessKeyID, e.accessKeySecret)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), strings.NewReader(bodyStr))
	if err != nil {
		return nil, &ai.ModelError{Code: "ValidationError"}
	}
	for k, v := range headers {
		if k == "host" {
			continue
		}
		httpReq.Header.Set(k, v)
	}
	httpReq.Header.Set("Authorization", authHeader)

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, networkError(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, networkError(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, httpStatusError(resp.StatusCode, resp.Header, truncateRunes(string(raw), 240))
	}
	return raw, nil
}

// signAlibabaACS3 实现阿里云 OpenAPI V3（ACS3-HMAC-SHA256）请求签名。
func signAlibabaACS3(method, canonicalURI string, query url.Values, headers map[string]string, payload []byte, accessKeyID, accessKeySecret string) string {
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalQuery := canonicalAlibabaQuery(query)

	signedKeys := make([]string, 0, len(headers))
	for k := range headers {
		lk := strings.ToLower(strings.TrimSpace(k))
		if lk == "host" || lk == "content-type" || strings.HasPrefix(lk, "x-acs-") {
			signedKeys = append(signedKeys, lk)
		}
	}
	sort.Strings(signedKeys)

	var canonicalHeaders strings.Builder
	for _, k := range signedKeys {
		canonicalHeaders.WriteString(k)
		canonicalHeaders.WriteByte(':')
		canonicalHeaders.WriteString(strings.TrimSpace(headers[k]))
		canonicalHeaders.WriteByte('\n')
	}
	signedHeaders := strings.Join(signedKeys, ";")
	hashedPayload := sha256Hex(payload)

	canonicalRequest := strings.Join([]string{
		strings.ToUpper(method),
		canonicalURI,
		canonicalQuery,
		canonicalHeaders.String(),
		signedHeaders,
		hashedPayload,
	}, "\n")

	stringToSign := "ACS3-HMAC-SHA256\n" + sha256Hex([]byte(canonicalRequest))
	mac := hmac.New(sha256.New, []byte(accessKeySecret))
	mac.Write([]byte(stringToSign))
	signature := hex.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("ACS3-HMAC-SHA256 Credential=%s,SignedHeaders=%s,Signature=%s",
		accessKeyID, signedHeaders, signature)
}

func canonicalAlibabaQuery(query url.Values) string {
	if len(query) == 0 {
		return ""
	}
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		vals := append([]string(nil), query[k]...)
		sort.Strings(vals)
		for _, v := range vals {
			parts = append(parts, alibabaPercentEncode(k)+"="+alibabaPercentEncode(v))
		}
	}
	return strings.Join(parts, "&")
}

func alibabaPercentEncode(s string) string {
	escaped := url.QueryEscape(s)
	escaped = strings.ReplaceAll(escaped, "+", "%20")
	escaped = strings.ReplaceAll(escaped, "*", "%2A")
	escaped = strings.ReplaceAll(escaped, "%7E", "~")
	return escaped
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func classifyAlibabaError(code, msg string) error {
	switch code {
	case "InvalidAccessKeyId.NotFound", "SignatureDoesNotMatch", "Forbidden", "10008":
		return mtError("AuthenticationError", msg)
	case "Throttling", "Throttling.User", "10007":
		return &ai.ModelError{Code: "RateLimitError"}
	case "10004":
		return mtError("UnsupportedLanguage", msg)
	default:
		return mtError("ProviderHTTPError", fmt.Sprintf("%s: %s", code, msg))
	}
}
