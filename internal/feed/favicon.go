package feed

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"time"

	"github.com/coolerks/Fealdhord/internal/netpolicy"
	_ "golang.org/x/image/webp"
)

const faviconLimit = 64 << 10

func (s *Service) favicon(ctx context.Context, siteURL, feedURL string) string {
	if !s.iconsAllowed(ctx) {
		return ""
	}
	if siteURL == "" {
		siteURL = feedURL
	}
	target := ResolveURL("/favicon.ico", siteURL)
	if target == "" {
		return ""
	}
	response, err := s.fetcher.Fetch(ctx, target, netpolicy.Request{
		Accept: "image/*", MaxBytes: faviconLimit, Timeout: 3 * time.Second,
	})
	if err != nil || response.Status != http.StatusOK || len(response.Body) > faviconLimit {
		return ""
	}
	kind := faviconType(response.Body)
	if kind == "" {
		return ""
	}
	return "data:" + kind + ";base64," + base64.StdEncoding.EncodeToString(response.Body)
}

func faviconType(body []byte) string {
	if len(body) >= 6 && string(body[:4]) == "\x00\x00\x01\x00" {
		count := int(binary.LittleEndian.Uint16(body[4:6]))
		directoryEnd := 6 + count*16
		if count == 0 || directoryEnd > len(body) {
			return ""
		}
		for pos := 6; pos < directoryEnd; pos += 16 {
			size := binary.LittleEndian.Uint32(body[pos+8:])
			offset := binary.LittleEndian.Uint32(body[pos+12:])
			if size == 0 || offset < uint32(directoryEnd) || uint64(offset)+uint64(size) > uint64(len(body)) {
				return ""
			}
		}
		return "image/x-icon"
	}
	switch kind := http.DetectContentType(body); kind {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		config, _, err := image.DecodeConfig(bytes.NewReader(body))
		if err == nil && config.Width > 0 && config.Width <= 512 && config.Height > 0 && config.Height <= 512 {
			return kind
		}
		return ""
	default:
		return ""
	}
}
