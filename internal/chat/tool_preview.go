package chat

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strings"
)

const maxToolPreviewBytes = 16 << 10

// toolPreview formats the local record independently of the model's tool transcript.
func toolPreview(text string) (*string, bool) {
	if data, ok := toolPreviewJSON(text); ok {
		if formatted, err := json.MarshalIndent(maskToolCredentials(data), "", "  "); err == nil {
			// Keep the original structure compact if indentation alone makes it too large.
			if len(formatted) > maxToolPreviewBytes {
				var compact bytes.Buffer
				if json.Compact(&compact, formatted) == nil {
					formatted = compact.Bytes()
				}
			}
			text = string(formatted)
		}
	}
	preview := boundedText(text, maxToolPreviewBytes)
	return &preview, len(preview) != len(text)
}

func toolPreviewJSON(text string) (any, bool) {
	if !json.Valid([]byte(text)) {
		return nil, false
	}
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var data any
	err := decoder.Decode(&data)
	return data, err == nil
}

var credentialKeySeparators = strings.NewReplacer("_", "", "-", "")

func credentialKey(key string) bool {
	switch strings.ToLower(credentialKeySeparators.Replace(key)) {
	case "key", "apikey", "apisecret", "token", "accesstoken", "refreshtoken", "bottoken", "secret",
		"clientsecret", "appsecret", "secretkey", "password", "authorization", "proxyauthorization", "cookie", "setcookie", "privatekey", "credentials":
		return true
	}
	return false
}

func maskToolCredentials(value any) any {
	switch node := value.(type) {
	case map[string]any:
		for key, child := range node {
			if credentialKey(key) {
				node[key] = "[已隐藏]"
			} else {
				node[key] = maskToolCredentials(child)
			}
		}
	case []any:
		for i, child := range node {
			node[i] = maskToolCredentials(child)
		}
	case string:
		trimmed := strings.TrimSpace(node)
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			if data, ok := toolPreviewJSON(trimmed); ok {
				if masked, err := json.Marshal(maskToolCredentials(data)); err == nil {
					return string(masked)
				}
			}
		}
		link, err := url.Parse(node)
		if err != nil || link.Host == "" || (link.Scheme != "http" && link.Scheme != "https") {
			break
		}
		query := link.Query()
		masked := false
		for key := range query {
			if credentialKey(key) {
				query.Set(key, "[已隐藏]")
				masked = true
			}
		}
		if masked {
			link.RawQuery = query.Encode()
		}
		if link.User != nil || masked {
			link.User = nil
			return link.String()
		}
	}
	return value
}
