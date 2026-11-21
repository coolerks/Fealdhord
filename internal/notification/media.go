package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/coolerks/Fealdhord/internal/netpolicy"
)

func (s *Service) SetImageFetcher(fetcher netpolicy.Fetcher, allowed func(context.Context) bool) {
	if s != nil {
		s.transport.images, s.transport.allowImages = fetcher, allowed
	}
}

func (t transport) cover(ctx context.Context, imageURL string) ([]byte, string) {
	if t.images == nil || !strings.HasPrefix(imageURL, "https://") {
		return nil, ""
	}
	response, err := t.images.Fetch(ctx, imageURL, netpolicy.Request{
		MaxBytes: 2 << 20, Timeout: 5 * time.Second, Accept: "image/png, image/jpeg, image/gif",
	})
	if err != nil || response.Status != http.StatusOK {
		return nil, ""
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(response.Body))
	if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width > 4096 || config.Height > 4096 {
		return nil, ""
	}
	return response.Body, "cover." + format
}

func (t transport) putFeishuCover(ctx context.Context, platform string, secret connectionSecret, imageURL string) string {
	if secret.MediaAppID == "" || secret.MediaSecret == "" {
		return ""
	}
	picture, fileName := t.cover(ctx, imageURL)
	if len(picture) == 0 {
		return ""
	}
	base := "https://" + webhooks[platform].host + "/open-apis/"
	response, body, err := t.post(ctx, base+"auth/v3/tenant_access_token/internal", map[string]any{
		"app_id": secret.MediaAppID, "app_secret": secret.MediaSecret,
	}, nil)
	var auth struct {
		Code  int    `json:"code"`
		Token string `json:"tenant_access_token"`
	}
	if err != nil || response.StatusCode != http.StatusOK || json.Unmarshal(body, &auth) != nil || auth.Code != 0 || auth.Token == "" {
		return ""
	}
	var buffer bytes.Buffer
	form := multipart.NewWriter(&buffer)
	if err := form.WriteField("image_type", "message"); err != nil {
		return ""
	}
	part, err := form.CreateFormFile("image", fileName)
	if err != nil {
		return ""
	}
	if _, err := part.Write(picture); err != nil {
		return ""
	}
	if err := form.Close(); err != nil {
		return ""
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"im/v1/images", &buffer)
	if err != nil {
		return ""
	}
	request.Header.Set("Content-Type", form.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+auth.Token)
	response, err = t.client.Do(request)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	body, err = io.ReadAll(io.LimitReader(response.Body, maxWireResponse))
	var uploaded struct {
		Code int `json:"code"`
		Data struct {
			Key string `json:"image_key"`
		} `json:"data"`
	}
	if err != nil || response.StatusCode != http.StatusOK || json.Unmarshal(body, &uploaded) != nil || uploaded.Code != 0 {
		return ""
	}
	return uploaded.Data.Key
}
