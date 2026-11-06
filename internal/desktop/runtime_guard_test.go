package desktop

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRuntimeRejectsEmbeddedPageRequests(t *testing.T) {
	handler := GuardRuntime(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, tc := range []struct {
		name, method, contentType, origin, site string
		want                                    int
	}{
		{"应用调用", "POST", "application/json", "wails://localhost", "same-origin", 204},
		{"原生请求无来源头", "POST", "application/json", "", "", 204},
		{"沙箱框架", "POST", "application/json", "null", "", 403},
		{"外站简单请求", "POST", "text/plain", "https://example.test", "cross-site", 403},
		{"跨站元数据", "POST", "application/json", "", "cross-site", 403},
		{"导航", "GET", "", "", "", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, "wails://localhost/wails/runtime", nil)
			request.URL.Scheme = ""
			request.URL.Host = ""
			request.Header.Set("Content-Type", tc.contentType)
			request.Header.Set("X-Wails-Client-Id", "test-client")
			request.Header.Set("Origin", tc.origin)
			request.Header.Set("Sec-Fetch-Site", tc.site)
			result := httptest.NewRecorder()
			handler.ServeHTTP(result, request)
			if result.Code != tc.want {
				t.Fatalf("状态码 %d，期望 %d", result.Code, tc.want)
			}
		})
	}
}
