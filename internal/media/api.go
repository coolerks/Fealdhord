package media

import (
	"bytes"
	"context"
	"crypto/rand"
	"image"
	"image/png"
	"net/http"
	"strings"

	"github.com/coolerks/Fealdhord/internal/app"
)

// ServeHTTP 由 Wails 识别为内部服务路由。
type API struct {
	service        *Service
	writeClipboard func([]byte) error
}

func NewAPI(service *Service, writeClipboard func([]byte) error) *API {
	return &API{service: service, writeClipboard: writeClipboard}
}

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

type CopyImageResult struct {
	OK        bool          `json:"ok"`
	Error     *app.AppError `json:"error,omitempty"`
	RequestID string        `json:"request_id"`
}

func (a *API) CopyInlineImage(ctx context.Context, id string) CopyImageResult {
	result := CopyImageResult{RequestID: rand.Text()}
	if !validInlineID(id) {
		result.Error = &app.AppError{Code: "ValidationError", Message: "图片标识无效。"}
		return result
	}
	if a.copyInlineImage(ctx, id) != nil {
		result.Error = &app.AppError{Code: "MediaError", Message: "无法复制图片，请重试。", Retryable: true}
		return result
	}
	result.OK = true
	return result
}

func (a *API) copyInlineImage(ctx context.Context, id string) error {
	if a.service == nil || a.writeClipboard == nil {
		return errUnavailable
	}
	body, mime, err := a.service.loadInline(ctx, id)
	if err != nil {
		return err
	}
	if mime != "image/png" {
		// 沿用媒体解码并发限额，防止并发复制放大内存占用。
		select {
		case a.service.slots <- struct{}{}:
			defer func() { <-a.service.slots }()
		case <-ctx.Done():
			return ctx.Err()
		}
		decoded, _, err := image.Decode(bytes.NewReader(body))
		if err != nil {
			return err
		}
		var encoded bytes.Buffer
		encoder := png.Encoder{CompressionLevel: png.BestSpeed}
		if err := encoder.Encode(&encoded, decoded); err != nil {
			return err
		}
		body = encoded.Bytes()
	}
	if !a.service.currentInlineAllowed(ctx, id) || ctx.Err() != nil {
		return errUnavailable
	}
	return a.writeClipboard(body)
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
