package media

import (
	"context"
	"crypto/rand"
	"github.com/coolerks/Fealdhord/internal/app"
	"net/http"
	"strings"
)

// API 只绑定清理缓存；ServeHTTP 由 Wails 识别为内部服务路由。
type API struct{ service *Service }

func NewAPI(service *Service) *API { return &API{service: service} }

type ClearResult struct {
	OK        bool          `json:"ok"`
	Error     *app.AppError `json:"error,omitempty"`
	RequestID string        `json:"request_id"`
}

func (a *API) ClearCache(context.Context) ClearResult {
	out := ClearResult{RequestID: rand.Text()}
	if a.service == nil || a.service.ClearCache() != nil {
		out.Error = &app.AppError{Code: "MediaError", Message: "无法清理图片缓存，请稍后重试。", Retryable: true}
		return out
	}
	out.OK = true
	return out
}
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if a.service == nil {
		http.NotFound(w, r)
		return
	}
	copy := r.Clone(r.Context())
	u := *r.URL
	copy.URL = &u
	copy.URL.Path = "/media/" + strings.TrimPrefix(r.URL.Path, "/")
	a.service.ServeHTTP(w, copy)
}
