package translation

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestAlibabaACS3SignAndBatchGeneralRouting(t *testing.T) {
	ctx := context.Background()
	var actions []string

	client := mockHTTPClient(func(r *http.Request) (*http.Response, error) {
		action := r.Header.Get("x-acs-action")
		actions = append(actions, action)
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "ACS3-HMAC-SHA256 Credential=ak-id,SignedHeaders=") {
			t.Errorf("unexpected alibaba auth header: %q", auth)
		}
		rawBody, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(rawBody))
		switch action {
		case "GetBatchTranslate":
			if vals.Get("ApiType") != "translate_standard" || vals.Get("Scene") != "general" {
				t.Errorf("unexpected alibaba batch params: %v", vals)
			}
			return jsonResponse(http.StatusOK, map[string]any{
				"Code": "200",
				"TranslatedList": []map[string]string{
					{"code": "200", "index": "1", "translated": "短段二", "detectedLanguage": "en"},
					{"code": "200", "index": "0", "translated": "短段一", "detectedLanguage": "en"},
				},
			}), nil
		case "TranslateGeneral":
			if vals.Get("Scene") != "general" || vals.Get("FormatType") != "html" {
				t.Errorf("unexpected alibaba general params: %v", vals)
			}
			return jsonResponse(http.StatusOK, map[string]any{
				"Code": "200",
				"Data": map[string]string{"Translated": "超长段落译文", "DetectedLanguage": "en"},
			}), nil
		default:
			t.Errorf("unexpected action: %q", action)
			return jsonResponse(http.StatusBadRequest, nil), nil
		}
	})

	eng, err := newAlibabaEngine(resolvedProviderConfig{
		ID: EngineAlibaba, AppID: "ak-id", Secret: "ak-secret",
	}, client)
	if err != nil {
		t.Fatal(err)
	}

	longPara := strings.Repeat("a", 1200)
	res, err := eng.TranslateBatch(ctx, BatchRequest{
		Texts:      []string{"Short 1", "Short 2", longPara},
		TargetLang: "zh-CN",
		IsHTML:     true,
	})
	if err != nil {
		t.Fatalf("alibaba translate failed: %v", err)
	}
	if len(actions) != 2 || actions[0] != "GetBatchTranslate" || actions[1] != "TranslateGeneral" {
		t.Fatalf("expected GetBatchTranslate then TranslateGeneral, got %v", actions)
	}
	if len(res.Translations) != 3 || res.Translations[0] != "短段一" || res.Translations[1] != "短段二" || res.Translations[2] != "超长段落译文" {
		t.Fatalf("unexpected alibaba translations: %+v", res)
	}
}

func TestCaiyunLanguageMatrixAndMediaText(t *testing.T) {
	ctx := context.Background()
	client := mockHTTPClient(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("X-Authorization") != "token cy-token" {
			return jsonResponse(http.StatusUnauthorized, map[string]string{"message": "bad token"}), nil
		}
		var req struct {
			Source    []string `json:"source"`
			TransType string   `json:"trans_type"`
			Media     string   `json:"media"`
			Detect    bool     `json:"detect"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Media != "text" || req.TransType != "auto2zh-Hant" || !req.Detect {
			t.Errorf("unexpected caiyun req: %+v", req)
		}
		return jsonResponse(http.StatusOK, map[string]any{
			"target": []string{"繁體中文結果"},
		}), nil
	})

	eng, err := newCaiyunEngine(resolvedProviderConfig{ID: EngineCaiyun, Secret: "cy-token"}, client)
	if err != nil {
		t.Fatal(err)
	}
	if !eng.SupportsLanguagePair("en", "zh-TW") || !eng.SupportsLanguagePair("zh-CN", "ko") {
		t.Fatalf("expected en->zh-TW and zh-CN->ko to be supported")
	}
	if eng.SupportsLanguagePair("en", "ja") || eng.SupportsLanguagePair("de", "en") {
		t.Fatalf("expected non-Chinese pairs en->ja / de->en to be rejected")
	}

	res, err := eng.TranslateBatch(ctx, BatchRequest{Texts: []string{"Hello"}, TargetLang: "zh-TW"})
	if err != nil || len(res.Translations) != 1 || res.Translations[0] != "繁體中文結果" {
		t.Fatalf("unexpected caiyun result: %+v err=%v", res, err)
	}
}

func TestBaiduSignNewlineProtectionAndPartialRetry(t *testing.T) {
	ctx := context.Background()
	calls := 0
	client := mockHTTPClient(func(r *http.Request) (*http.Response, error) {
		calls++
		rawBody, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(rawBody))
		q := vals.Get("q")
		salt := vals.Get("salt")
		wantSum := md5.Sum([]byte("bd-app" + q + salt + "bd-sec"))
		if vals.Get("sign") != hex.EncodeToString(wantSum[:]) {
			t.Errorf("invalid baidu md5 sign")
		}
		if calls == 1 {
			// 模拟第一批只返回第 1 段（第 2 段丢失），验证仅重试第 2 段
			return jsonResponse(http.StatusOK, map[string]any{
				"from": "en",
				"trans_result": []map[string]string{
					{"src": "Line1" + baiduNewlineToken + "Line2", "dst": "行1" + baiduNewlineToken + "行2"},
				},
			}), nil
		}
		return jsonResponse(http.StatusOK, map[string]any{
			"from": "en",
			"trans_result": []map[string]string{
				{"src": "Second", "dst": "第二段"},
			},
		}), nil
	})

	eng, err := newBaiduEngine(resolvedProviderConfig{
		ID: EngineBaidu, AppID: "bd-app", Secret: "bd-sec", Edition: "premium",
	}, client)
	if err != nil {
		t.Fatal(err)
	}
	if eng.Capabilities().DefaultQPS < 50 {
		t.Fatalf("expected premium QPS, got %v", eng.Capabilities().DefaultQPS)
	}
	res, err := eng.TranslateBatch(ctx, BatchRequest{
		Texts:      []string{"Line1\nLine2", "Second"},
		TargetLang: "zh-CN",
	})
	if err != nil {
		t.Fatalf("baidu translate failed: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 1 batch call + 1 single-item retry, got %d", calls)
	}
	if res.Translations[0] != "行1\n行2" || res.Translations[1] != "第二段" {
		t.Fatalf("unexpected baidu translations: %+v", res.Translations)
	}
}

func TestYoudaoV3BatchSignatureUsesConcatenatedTruncate(t *testing.T) {
	ctx := context.Background()
	texts := []string{"Hello, this is the first sentence.", "This is the second sentence."}

	client := mockHTTPClient(func(r *http.Request) (*http.Response, error) {
		rawBody, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(rawBody))
		if vals.Get("signType") != "v3" || vals.Get("to") != "zh-CHS" {
			t.Errorf("unexpected youdao params: %v", vals)
		}
		qs := vals["q"]
		if len(qs) != 2 {
			t.Errorf("expected 2 q params, got %d", len(qs))
		}
		// 严格验证：先拼接再统一 truncate
		joined := strings.Join(texts, "")
		runes := []rune(joined)
		expectedInput := string(runes[:10]) + "62" + string(runes[len(runes)-10:])
		sum := sha256.Sum256([]byte("yd-key" + expectedInput + vals.Get("salt") + vals.Get("curtime") + "yd-secret"))
		if vals.Get("sign") != hex.EncodeToString(sum[:]) {
			t.Errorf("youdao v3 batch sign mismatch: got %s want %s", vals.Get("sign"), hex.EncodeToString(sum[:]))
		}
		return jsonResponse(http.StatusOK, map[string]any{
			"errorCode": "0",
			"translateResults": []map[string]string{
				{"query": texts[0], "translation": "你好，这是第一句。", "type": "en2zh-CHS"},
				{"query": texts[1], "translation": "这是第二句。", "type": "en2zh-CHS"},
			},
		}), nil
	})

	eng, err := newYoudaoEngine(resolvedProviderConfig{
		ID: EngineYoudao, AppID: "yd-key", Secret: "yd-secret", Edition: "text",
	}, client)
	if err != nil {
		t.Fatal(err)
	}
	res, err := eng.TranslateBatch(ctx, BatchRequest{Texts: texts, TargetLang: "zh-CN"})
	if err != nil || len(res.Translations) != 2 || res.Translations[0] != "你好，这是第一句。" || res.DetectedLang != "en" {
		t.Fatalf("unexpected youdao result: %+v err=%v", res, err)
	}
}
