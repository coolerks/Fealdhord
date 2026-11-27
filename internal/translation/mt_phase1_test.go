package translation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/zalando/go-keyring"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func mockHTTPClient(fn roundTripFunc) *http.Client {
	return &http.Client{Transport: fn}
}

func jsonResponse(status int, payload any) *http.Response {
	data, _ := json.Marshal(payload)
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(data)),
	}
}

type memoryCreds struct {
	data map[string]string
}

func newMemoryCreds() *memoryCreds {
	return &memoryCreds{data: make(map[string]string)}
}

func (m *memoryCreds) Get(ref string) (string, error) {
	v, ok := m.data[ref]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return v, nil
}

func (m *memoryCreds) Set(ref, val string) error {
	m.data[ref] = val
	return nil
}

func (m *memoryCreds) Delete(ref string) error {
	delete(m.data, ref)
	return nil
}

func TestMTHTMLBridgePreservesCodeAndLinks(t *testing.T) {
	linkID := strings.Repeat("a", 64)
	src := `Read <a href="#reader-link" data-reader-link="` + linkID + `">the docs</a> and run <code>go test ./...</code>.`
	hb := encodeHTMLForMT(src)
	if !strings.Contains(hb.encoded, `<span translate="no" class="notranslate">C0</span>`) {
		t.Fatalf("expected notranslate code span, got %q", hb.encoded)
	}
	if !strings.Contains(hb.encoded, `<a id="T0">`) || strings.Contains(hb.encoded, linkID) {
		t.Fatalf("expected stripped link tag, got %q", hb.encoded)
	}

	translated := `阅读 <a id="T0">文档</a> 并运行 <span translate="no" class="notranslate">C0</span>。`
	restored, ok := hb.restore(translated)
	if !ok {
		t.Fatalf("expected exact restore, got ok=false (%q)", restored)
	}
	if !strings.Contains(restored, `data-reader-link="`+linkID+`"`) || !strings.Contains(restored, `<code>go test ./...</code>`) {
		t.Fatalf("restored html missing original link or code: %q", restored)
	}
}

func TestMTTextBridgeToleratesSpacesAndFullWidthBrackets(t *testing.T) {
	src := `Press <kbd>Ctrl+F</kbd> to <strong>find</strong> text.`
	tb := encodeTextForMT(src)
	translated := `按 【 C0 】 来 [[T1]]查找[[T2]] 文本。`
	restored, exact := tb.restore(translated)
	if !exact {
		t.Fatalf("expected exact restore with tolerant brackets, got %q", restored)
	}
	if !strings.Contains(restored, `<kbd>Ctrl+F</kbd>`) || !strings.Contains(restored, `<strong>查找</strong>`) {
		t.Fatalf("unexpected restored html: %q", restored)
	}
}

func TestGoogleFreeAndBYOKProviders(t *testing.T) {
	ctx := context.Background()

	freeClient := mockHTTPClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/v1/translateHtml":
			if r.Header.Get("X-Goog-Api-Key") == "" {
				t.Errorf("missing X-Goog-Api-Key")
			}
			return jsonResponse(http.StatusOK, []any{
				[]string{"你好，<b>世界</b>", "第二段"},
				[]string{"en", "en"},
			}), nil
		case "/translate_a/single":
			if r.URL.Query().Get("client") != "gtx" || r.URL.Query().Get("sl") != "auto" {
				t.Errorf("unexpected gtx query: %s", r.URL.RawQuery)
			}
			return jsonResponse(http.StatusOK, []any{
				[]any{[]any{"你好，世界", "Hello, world"}},
				nil,
				"en",
			}), nil
		default:
			return jsonResponse(http.StatusNotFound, map[string]string{"error": "not found"}), nil
		}
	})

	gFree := newGoogleFreeEngine(freeClient)
	htmlRes, err := gFree.TranslateBatch(ctx, BatchRequest{
		Texts:      []string{"Hello, <b>world</b>", "Second"},
		TargetLang: "zh-CN",
		IsHTML:     true,
	})
	if err != nil || len(htmlRes.Translations) != 2 || htmlRes.Translations[0] != "你好，<b>世界</b>" || htmlRes.DetectedLang != "en" {
		t.Fatalf("unexpected google free html result: %+v err=%v", htmlRes, err)
	}
	gtxRes, err := gFree.TranslateBatch(ctx, BatchRequest{
		Texts:      []string{"Hello, world"},
		TargetLang: "zh-CN",
		IsHTML:     false,
	})
	if err != nil || len(gtxRes.Translations) != 1 || gtxRes.Translations[0] != "你好，世界" {
		t.Fatalf("unexpected google free gtx result: %+v err=%v", gtxRes, err)
	}

	byokClient := mockHTTPClient(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("X-Goog-Api-Key") != "gcp-key-123" {
			return jsonResponse(http.StatusForbidden, map[string]string{"error": "forbidden"}), nil
		}
		var req struct {
			Q      []string `json:"q"`
			Target string   `json:"target"`
			Source string   `json:"source"`
			Format string   `json:"format"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Target != "zh-CN" || req.Source != "" || req.Format != "html" || len(req.Q) != 2 {
			t.Errorf("unexpected google v2 req: %+v", req)
		}
		return jsonResponse(http.StatusOK, map[string]any{
			"data": map[string]any{
				"translations": []map[string]string{
					{"translatedText": "甲", "detectedSourceLanguage": "en"},
					{"translatedText": "乙", "detectedSourceLanguage": "en"},
				},
			},
		}), nil
	})

	gBYOK, err := newGoogleBYOKEngine(resolvedProviderConfig{
		ID: EngineGoogleBYOK, Secret: "gcp-key-123",
	}, byokClient)
	if err != nil {
		t.Fatal(err)
	}
	res, err := gBYOK.TranslateBatch(ctx, BatchRequest{Texts: []string{"A", "B"}, TargetLang: "zh-CN", IsHTML: true})
	if err != nil || len(res.Translations) != 2 || res.Translations[1] != "乙" {
		t.Fatalf("unexpected google byok result: %+v err=%v", res, err)
	}
}

func TestMicrosoftEdgeAndAzureProviders(t *testing.T) {
	ctx := context.Background()

	edgeClient := mockHTTPClient(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("from") != "" || r.URL.Query().Get("to") != "zh-Hans" || r.URL.Query().Get("isEnterpriseClient") != "false" {
			t.Errorf("unexpected edge query: %s", r.URL.RawQuery)
		}
		if r.Header.Get("Authorization") != "" {
			t.Errorf("edge translatetext must not send Authorization header")
		}
		var texts []string
		if err := json.NewDecoder(r.Body).Decode(&texts); err != nil || len(texts) != 2 {
			t.Errorf("expected string[] body, got %v err=%v", texts, err)
		}
		return jsonResponse(http.StatusOK, []map[string]any{
			{"translations": []map[string]string{{"text": "第一段"}}},
			{"translations": []map[string]string{{"text": "第二段"}}},
		}), nil
	})

	edge := newMicrosoftEdgeEngine(edgeClient)
	edgeRes, err := edge.TranslateBatch(ctx, BatchRequest{Texts: []string{"First", "Second"}, TargetLang: "zh-CN"})
	if err != nil || len(edgeRes.Translations) != 2 || edgeRes.Translations[0] != "第一段" {
		t.Fatalf("unexpected edge result: %+v err=%v", edgeRes, err)
	}

	azureClient := mockHTTPClient(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Ocp-Apim-Subscription-Key") != "az-key" || r.Header.Get("Ocp-Apim-Subscription-Region") != "eastasia" {
			return jsonResponse(http.StatusUnauthorized, map[string]string{"error": "unauthorized"}), nil
		}
		if r.URL.Query().Get("from") != "" || r.URL.Query().Get("api-version") != "3.0" || r.URL.Query().Get("textType") != "html" || r.URL.Query().Get("to") != "zh-Hant" {
			t.Errorf("unexpected azure query: %s", r.URL.RawQuery)
		}
		return jsonResponse(http.StatusOK, []map[string]any{
			{
				"detectedLanguage": map[string]any{"language": "en"},
				"translations":     []map[string]string{{"text": "繁體譯文"}},
			},
		}), nil
	})

	azure, err := newMicrosoftAzureEngine(resolvedProviderConfig{
		ID: EngineMicrosoftAzure, Secret: "az-key", Region: "eastasia", Edition: "f0",
	}, azureClient)
	if err != nil {
		t.Fatal(err)
	}
	if azure.Capabilities().CharsPerMinute != 30000 {
		t.Fatalf("expected F0 CPM limit 30000, got %d", azure.Capabilities().CharsPerMinute)
	}
	azRes, err := azure.TranslateBatch(ctx, BatchRequest{Texts: []string{"<b>Traditional</b>"}, TargetLang: "zh-TW", IsHTML: true})
	if err != nil || len(azRes.Translations) != 1 || azRes.Translations[0] != "繁體譯文" || azRes.DetectedLang != "en" {
		t.Fatalf("unexpected azure result: %+v err=%v", azRes, err)
	}
}

func TestProviderConfigSaveMaskAndTestProvider(t *testing.T) {
	ctx := context.Background()
	store, err := database.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	creds := newMemoryCreds()
	client := mockHTTPClient(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Ocp-Apim-Subscription-Key") != "secret-azure-9876" {
			return jsonResponse(http.StatusUnauthorized, map[string]string{"error": "bad key"}), nil
		}
		return jsonResponse(http.StatusOK, []map[string]any{
			{"translations": []map[string]string{{"text": "你好，世界！"}}},
		}), nil
	})

	svc := New(store, nil, nil, nil, nil)
	svc.credentials = creds
	svc.httpClient = client

	saved, err := svc.SaveProviderConfig(ctx, SaveProviderInput{
		ID:      string(EngineMicrosoftAzure),
		Secret:  "secret-azure-9876",
		Region:  "eastasia",
		Edition: "f0",
	})
	if err != nil {
		t.Fatalf("save provider config: %v", err)
	}
	if !saved.HasSecret || saved.SecretMasked != "secr****9876" || saved.Region != "eastasia" {
		t.Fatalf("unexpected saved config: %+v", saved)
	}

	testRes := svc.TestProvider(ctx, string(EngineMicrosoftAzure), nil)
	if !testRes.OK || testRes.SampleOut != "你好，世界！" {
		t.Fatalf("expected ok test result, got %+v", testRes)
	}

	cleared, err := svc.SaveProviderConfig(ctx, SaveProviderInput{
		ID:          string(EngineMicrosoftAzure),
		ClearSecret: true,
		Edition:     "f0",
	})
	if err != nil || cleared.HasSecret {
		t.Fatalf("expected cleared secret, got %+v err=%v", cleared, err)
	}
	badRes := svc.TestProvider(ctx, string(EngineMicrosoftAzure), nil)
	if badRes.OK || badRes.ErrorCode != "ConfigurationError" {
		t.Fatalf("expected ConfigurationError after clear, got %+v", badRes)
	}
	var me *ai.ModelError
	if !errors.As(mtError("RateLimitError", "429"), &me) || me.Code != "RateLimitError" {
		t.Fatalf("expected detailError to unwrap to ai.ModelError")
	}
}
