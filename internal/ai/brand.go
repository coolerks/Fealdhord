package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var brandName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

func (index *CatalogIndex) BrandIcon(ctx context.Context, brand string) (string, error) {
	if index == nil || !brandName.MatchString(brand) {
		return "", errors.New("InvalidBrand")
	}
	path := filepath.Join(index.directory, "logos", brand+".svg")
	if info, err := os.Stat(path); err == nil && info.Size() <= 64<<10 {
		if data, err := os.ReadFile(path); err == nil && safeLogo(data) {
			return logoData(data), nil
		}
	}
	client := http.Client{Timeout: 4 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	for _, prefix := range []string{"logos/labs/", "logos/"} {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://models.dev/"+prefix+brand+".svg", nil)
		if err != nil {
			return "", err
		}
		response, err := client.Do(request)
		if err != nil {
			return "", errors.New("BrandUnavailable")
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, (64<<10)+1))
		response.Body.Close()
		if response.StatusCode != http.StatusOK || err != nil || len(data) > 64<<10 || !safeLogo(data) {
			continue
		}
		if os.MkdirAll(filepath.Dir(path), 0700) == nil {
			_ = os.WriteFile(path, data, 0600)
		}
		return logoData(data), nil
	}
	return "", errors.New("BrandUnavailable")
}

func logoData(data []byte) string {
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(data)
}

func safeLogo(data []byte) bool {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	elements := map[string]bool{"svg": true, "path": true, "g": true, "rect": true, "circle": true, "ellipse": true, "polygon": true, "polyline": true, "line": true, "defs": true, "linearGradient": true, "radialGradient": true, "stop": true, "mask": true, "clipPath": true, "title": true, "desc": true}
	depth, roots := 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return roots == 1 && depth == 0
		}
		if err != nil {
			return false
		}
		switch token := token.(type) {
		case xml.Directive, xml.ProcInst:
			return false
		case xml.StartElement:
			if !elements[token.Name.Local] || token.Name.Space != "" && token.Name.Space != "http://www.w3.org/2000/svg" {
				return false
			}
			if depth == 0 {
				if token.Name.Local != "svg" {
					return false
				}
				roots++
			}
			depth++
			for _, attribute := range token.Attr {
				name, value := strings.ToLower(attribute.Name.Local), strings.ToLower(strings.TrimSpace(attribute.Value))
				if strings.HasPrefix(name, "on") || name == "style" || name == "href" || name == "src" ||
					strings.Contains(value, "url(") && !strings.HasPrefix(value, "url(#") {
					return false
				}
			}
		case xml.EndElement:
			depth--
		}
	}
}
