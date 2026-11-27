package translation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/coolerks/Fealdhord/internal/ai"
)

const (
	// 参照 YiiGuxing/TranslationPlugin 的 GoogleTranslateAPI.kt：
	// te_lib 用于 /v1/translateHtml 批量 HTML 翻译。
	defaultGoogleTeLibKey      = "AIzaSyATBXajvzQLTDHEQbcpq0Ihe0vWDHmO520"
	defaultGoogleFreeHTMLURL   = "https://translate-pa.googleapis.com/v1/translateHtml"
	defaultGoogleFreeGTXURL    = "https://translate.googleapis.com/translate_a/single"
	defaultGoogleCloudV2URL    = "https://translation.googleapis.com/language/translate/v2"
	googleCloudV2MaxBodyBytes  = 90 << 10 // 官方硬限制 100 KB，留 10 KB 余量
)

type googleFreeEngine struct {
	client     *http.Client
	htmlURL    string
	gtxURL     string
	teLibKey   string
}

func newGoogleFreeEngine(client *http.Client) *googleFreeEngine {
	return &googleFreeEngine{
		client:   client,
		htmlURL:  defaultGoogleFreeHTMLURL,
		gtxURL:   defaultGoogleFreeGTXURL,
		teLibKey: defaultGoogleTeLibKey,
	}
}

func (e *googleFreeEngine) ID() EngineID { return EngineGoogleFree }

func (e *googleFreeEngine) Capabilities() EngineCapabilities {
	return EngineCapabilities{
		SupportsHTML:       true,
		MaxBatchItems:      50,
		MaxBatchChars:      15000,
		MaxSingleItemChars: 5000,
		DefaultQPS:         2,
		BackgroundQPS:      0.5,
	}
}

func (e *googleFreeEngine) SupportsLanguagePair(_, targetLang string) bool {
	return mapGoogleLang(targetLang) != ""
}

func mapGoogleLang(code string) string {
	if strings.TrimSpace(code) == "" {
		return ""
	}
	switch NormalizeTargetLang(code) {
	case "zh-CN":
		return "zh-CN"
	case "zh-TW":
		return "zh-TW"
	case "en", "ja", "ko", "fr", "de", "es", "ru":
		return NormalizeTargetLang(code)
	default:
		return ""
	}
}

func (e *googleFreeEngine) TranslateBatch(ctx context.Context, req BatchRequest) (BatchResult, error) {
	if len(req.Texts) == 0 {
		return BatchResult{}, nil
	}
	target := mapGoogleLang(req.TargetLang)
	if target == "" {
		return BatchResult{}, &ai.ModelError{Code: "UnsupportedLanguage"}
	}
	source := "auto"
	if s := mapGoogleLang(req.SourceLang); s != "" {
		source = s
	}

	// 普通纯文本单条请求走 GTX 免费通道，HTML 或多段批量走 te_lib /v1/translateHtml。
	if !req.IsHTML && len(req.Texts) == 1 {
		if res, err := e.translateGTX(ctx, req.Texts[0], source, target); err == nil {
			return res, nil
		}
	}
	return e.translateHTML(ctx, req.Texts, source, target)
}

func (e *googleFreeEngine) translateHTML(ctx context.Context, texts []string, source, target string) (BatchResult, error) {
	payload := []any{
		[]any{texts, source, target},
		"te_lib",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return BatchResult{}, &ai.ModelError{Code: "ValidationError"}
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.htmlURL, bytes.NewReader(body))
	if err != nil {
		return BatchResult{}, &ai.ModelError{Code: "ValidationError"}
	}
	httpReq.Header.Set("Content-Type", "application/json+protobuf")
	httpReq.Header.Set("X-Goog-Api-Key", e.teLibKey)
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")

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

	var parsed []json.RawMessage
	if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed) == 0 {
		return BatchResult{}, mtError("ProviderProtocolError", "invalid translateHtml response")
	}
	var translated []string
	if err := json.Unmarshal(parsed[0], &translated); err != nil || len(translated) != len(texts) {
		return BatchResult{}, mtError("ProviderProtocolError", "mismatched translateHtml items")
	}
	detected := ""
	if len(parsed) > 1 {
		var langs []string
		if json.Unmarshal(parsed[1], &langs) == nil && len(langs) > 0 {
			detected = langs[0]
		}
	}
	return BatchResult{Translations: translated, DetectedLang: detected}, nil
}

func (e *googleFreeEngine) translateGTX(ctx context.Context, text, source, target string) (BatchResult, error) {
	u, err := url.Parse(e.gtxURL)
	if err != nil {
		return BatchResult{}, &ai.ModelError{Code: "ValidationError"}
	}
	q := u.Query()
	q.Set("client", "gtx")
	q.Set("sl", source)
	q.Set("tl", target)
	q.Set("dt", "t")
	q.Set("q", text)
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return BatchResult{}, &ai.ModelError{Code: "ValidationError"}
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return BatchResult{}, networkError(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return BatchResult{}, networkError(err)
	}
	if resp.StatusCode != http.StatusOK {
		return BatchResult{}, httpStatusError(resp.StatusCode, resp.Header, truncateRunes(string(raw), 240))
	}

	var outer []json.RawMessage
	if err := json.Unmarshal(raw, &outer); err != nil || len(outer) == 0 {
		return BatchResult{}, &ai.ModelError{Code: "ProviderProtocolError"}
	}
	var segments [][]json.RawMessage
	if err := json.Unmarshal(outer[0], &segments); err != nil {
		return BatchResult{}, &ai.ModelError{Code: "ProviderProtocolError"}
	}
	var out strings.Builder
	for _, seg := range segments {
		if len(seg) > 0 {
			var piece string
			if json.Unmarshal(seg[0], &piece) == nil {
				out.WriteString(piece)
			}
		}
	}
	detected := ""
	if len(outer) > 2 {
		_ = json.Unmarshal(outer[2], &detected)
	}
	return BatchResult{Translations: []string{out.String()}, DetectedLang: detected}, nil
}

// googleBYOKEngine 对接 Google Cloud Translation API v2 Basic。
type googleBYOKEngine struct {
	client   *http.Client
	apiKey   string
	endpoint string
}

func newGoogleBYOKEngine(cfg resolvedProviderConfig, client *http.Client) (*googleBYOKEngine, error) {
	if strings.TrimSpace(cfg.Secret) == "" {
		return nil, mtError("ConfigurationError", "缺少 Google Cloud API Key")
	}
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = defaultGoogleCloudV2URL
	}
	return &googleBYOKEngine{
		client:   client,
		apiKey:   strings.TrimSpace(cfg.Secret),
		endpoint: endpoint,
	}, nil
}

func (e *googleBYOKEngine) ID() EngineID { return EngineGoogleBYOK }

func (e *googleBYOKEngine) Capabilities() EngineCapabilities {
	return EngineCapabilities{
		SupportsHTML:       true,
		MaxBatchItems:      100,
		MaxBatchChars:      25000,
		MaxBatchBytes:      googleCloudV2MaxBodyBytes,
		MaxSingleItemChars: 5000,
		DefaultQPS:         10,
		BackgroundQPS:      2,
	}
}

func (e *googleBYOKEngine) SupportsLanguagePair(_, targetLang string) bool {
	return mapGoogleLang(targetLang) != ""
}

func (e *googleBYOKEngine) TranslateBatch(ctx context.Context, req BatchRequest) (BatchResult, error) {
	if len(req.Texts) == 0 {
		return BatchResult{}, nil
	}
	target := mapGoogleLang(req.TargetLang)
	if target == "" {
		return BatchResult{}, &ai.ModelError{Code: "UnsupportedLanguage"}
	}
	format := "text"
	if req.IsHTML {
		format = "html"
	}
	payload := struct {
		Q      []string `json:"q"`
		Target string   `json:"target"`
		Source string   `json:"source,omitempty"`
		Format string   `json:"format"`
	}{
		Q:      req.Texts,
		Target: target,
		Source: mapGoogleLang(req.SourceLang),
		Format: format,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return BatchResult{}, &ai.ModelError{Code: "ValidationError"}
	}
	if len(body) > 100<<10 {
		return BatchResult{}, mtError("ValidationError", fmt.Sprintf("Google v2 request exceeds 100KB (%d bytes)", len(body)))
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, bytes.NewReader(body))
	if err != nil {
		return BatchResult{}, &ai.ModelError{Code: "ValidationError"}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Goog-Api-Key", e.apiKey)

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
		Data struct {
			Translations []struct {
				TranslatedText         string `json:"translatedText"`
				DetectedSourceLanguage string `json:"detectedSourceLanguage"`
			} `json:"translations"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed.Data.Translations) != len(req.Texts) {
		return BatchResult{}, &ai.ModelError{Code: "ProviderProtocolError"}
	}
	out := make([]string, len(req.Texts))
	detected := ""
	for i, item := range parsed.Data.Translations {
		out[i] = item.TranslatedText
		if detected == "" && item.DetectedSourceLanguage != "" {
			detected = item.DetectedSourceLanguage
		}
	}
	return BatchResult{Translations: out, DetectedLang: detected}, nil
}
