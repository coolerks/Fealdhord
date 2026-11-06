package desktop

import (
	"mime"
	"net/http"
	"strings"
)

// GuardRuntime 只接受应用自身的 JSON 绑定请求。子框架不能借跨源简单
// 请求调用桌面方法，沙箱产生的 null Origin 也不受信任。
func GuardRuntime(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wails/runtime" {
			mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
			origin := r.Header.Get("Origin")
			// Wails 模拟服务端请求时会清除 URL 的 scheme。
			scheme := r.URL.Scheme
			if scheme == "" {
				if r.Host == "wails.localhost" || strings.HasPrefix(r.Host, "wails.localhost:") {
					scheme = "http"
				} else {
					scheme = "wails"
				}
			}
			fetchSite := r.Header.Get("Sec-Fetch-Site")
			if r.Method != http.MethodPost || mediaType != "application/json" || r.Header.Get("X-Wails-Client-Id") == "" ||
				(origin != "" && origin != scheme+"://"+r.Host) ||
				(fetchSite != "" && fetchSite != "same-origin") {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
