package chat

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/coolerks/Fealdhord/internal/ai"
)

const (
	maxAttachmentsPerMessage = 10
	maxImageAttachmentBytes  = 10 << 20 // 10 MiB
	maxPreviewDataURLBytes   = 512 << 10
)

type SendAttachment struct {
	Kind     string `json:"kind"`               // "image" | "file" | "folder"
	Name     string `json:"name"`
	Path     string `json:"path,omitempty"`
	MIMEType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"` // base64 without data: prefix
}

type MessageAttachment struct {
	Kind     string `json:"kind"`               // "image" | "file" | "folder"
	Name     string `json:"name"`
	Path     string `json:"path,omitempty"`
	MIMEType string `json:"mimeType,omitempty"`
	Preview  string `json:"preview,omitempty"`
}

type AttachmentCandidate struct {
	Kind     string `json:"kind"` // "image" | "file" | "folder"
	Name     string `json:"name"`
	Path     string `json:"path"`
	MIMEType string `json:"mimeType,omitempty"`
	Preview  string `json:"preview,omitempty"`
}

type SteerInput struct {
	ConversationID string           `json:"conversationId"`
	Text           string           `json:"text"`
	Attachments    []SendAttachment `json:"attachments,omitempty"`
}

func supportedImageMIME(mimeType string) bool {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/png", "image/jpeg", "image/webp", "image/gif":
		return true
	default:
		return false
	}
}

func detectImageMIMEByExt(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return ""
	}
}

func inspectPaths(paths []string) []AttachmentCandidate {
	out := make([]AttachmentCandidate, 0, min(len(paths), maxAttachmentsPerMessage))
	seen := make(map[string]bool, len(paths))
	for _, raw := range paths {
		if len(out) >= maxAttachmentsPerMessage {
			break
		}
		if raw == "" || !filepath.IsAbs(raw) || !utf8.ValidString(raw) {
			continue
		}
		clean, err := filepath.EvalSymlinks(filepath.Clean(raw))
		if err != nil {
			clean = filepath.Clean(raw)
		}
		if seen[clean] {
			continue
		}
		info, err := os.Stat(clean)
		if err != nil {
			continue
		}
		seen[clean] = true
		name := filepath.Base(clean)
		if info.IsDir() {
			out = append(out, AttachmentCandidate{Kind: "folder", Name: name, Path: clean})
			continue
		}
		mimeType := detectImageMIMEByExt(clean)
		if mimeType != "" && info.Size() > 0 && info.Size() <= maxImageAttachmentBytes {
			if data, err := os.ReadFile(clean); err == nil {
				detected := http.DetectContentType(data)
				if supportedImageMIME(detected) {
					mimeType = detected
				}
				candidate := AttachmentCandidate{Kind: "image", Name: name, Path: clean, MIMEType: mimeType}
				if len(data) <= maxPreviewDataURLBytes {
					candidate.Preview = "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
				}
				out = append(out, candidate)
				continue
			}
		}
		out = append(out, AttachmentCandidate{Kind: "file", Name: name, Path: clean})
	}
	return out
}

func prepareAttachments(attachments []SendAttachment, supportsImage bool) (imageBlocks []ai.ContentBlock, readOnlyPaths []string, persisted []MessageAttachment, promptSuffix string, err error) {
	if len(attachments) == 0 {
		return nil, nil, nil, "", nil
	}
	if len(attachments) > maxAttachmentsPerMessage {
		return nil, nil, nil, "", modelError("ValidationError")
	}
	var pathLines []string
	seenPaths := make(map[string]bool)
	for _, att := range attachments {
		switch att.Kind {
		case "image":
			if !supportsImage {
				return nil, nil, nil, "", modelError("ModelImageUnsupported")
			}
			var rawBytes []byte
			mimeType := strings.ToLower(strings.TrimSpace(att.MIMEType))
			name := strings.TrimSpace(att.Name)
			cleanPath := ""
			if att.Data != "" {
				decoded, decErr := base64.StdEncoding.DecodeString(att.Data)
				if decErr != nil || len(decoded) == 0 || len(decoded) > maxImageAttachmentBytes {
					return nil, nil, nil, "", modelError("ValidationError")
				}
				rawBytes = decoded
				if !supportedImageMIME(mimeType) {
					mimeType = http.DetectContentType(rawBytes)
				}
				if name == "" {
					name = "粘贴的图片"
				}
			} else if att.Path != "" && filepath.IsAbs(att.Path) {
				resolved, rErr := filepath.EvalSymlinks(filepath.Clean(att.Path))
				if rErr != nil {
					return nil, nil, nil, "", modelError("AttachmentUnavailable")
				}
				info, statErr := os.Stat(resolved)
				if statErr != nil || info.IsDir() || info.Size() == 0 || info.Size() > maxImageAttachmentBytes {
					return nil, nil, nil, "", modelError("AttachmentUnavailable")
				}
				data, readErr := os.ReadFile(resolved)
				if readErr != nil {
					return nil, nil, nil, "", modelError("AttachmentUnavailable")
				}
				rawBytes = data
				cleanPath = resolved
				if !supportedImageMIME(mimeType) {
					mimeType = http.DetectContentType(rawBytes)
				}
				if name == "" {
					name = filepath.Base(resolved)
				}
			} else {
				return nil, nil, nil, "", modelError("ValidationError")
			}
			if !supportedImageMIME(mimeType) {
				return nil, nil, nil, "", modelError("ValidationError")
			}
			encoded := base64.StdEncoding.EncodeToString(rawBytes)
			imageBlocks = append(imageBlocks, ai.ContentBlock{
				Type:     "image",
				MIMEType: mimeType,
				Data:     encoded,
			})
			meta := MessageAttachment{Kind: "image", Name: name, Path: cleanPath, MIMEType: mimeType}
			if len(rawBytes) <= maxPreviewDataURLBytes {
				meta.Preview = "data:" + mimeType + ";base64," + encoded
			}
			persisted = append(persisted, meta)

		case "file", "folder":
			if att.Path == "" || !filepath.IsAbs(att.Path) || !utf8.ValidString(att.Path) {
				return nil, nil, nil, "", modelError("ValidationError")
			}
			resolved, rErr := filepath.EvalSymlinks(filepath.Clean(att.Path))
			if rErr != nil {
				return nil, nil, nil, "", modelError("AttachmentUnavailable")
			}
			info, statErr := os.Stat(resolved)
			if statErr != nil {
				return nil, nil, nil, "", modelError("AttachmentUnavailable")
			}
			kind := "file"
			kindLabel := "文件"
			if info.IsDir() {
				kind = "folder"
				kindLabel = "文件夹"
			}
			name := strings.TrimSpace(att.Name)
			if name == "" {
				name = filepath.Base(resolved)
			}
			if !seenPaths[resolved] {
				seenPaths[resolved] = true
				readOnlyPaths = append(readOnlyPaths, resolved)
				pathLines = append(pathLines, "- ["+kindLabel+"] "+resolved)
			}
			persisted = append(persisted, MessageAttachment{Kind: kind, Name: name, Path: resolved})

		default:
			return nil, nil, nil, "", modelError("ValidationError")
		}
	}
	if len(pathLines) > 0 {
		promptSuffix = "\n\n[用户附加的本地路径（可通过 read 工具只读查看文本文件内容）]\n" + strings.Join(pathLines, "\n")
	}
	return imageBlocks, readOnlyPaths, persisted, promptSuffix, nil
}

func marshalAttachments(items []MessageAttachment) string {
	if len(items) == 0 {
		return ""
	}
	data, err := json.Marshal(items)
	if err != nil {
		return ""
	}
	return string(data)
}

func unmarshalAttachments(raw string) []MessageAttachment {
	if raw == "" {
		return nil
	}
	var items []MessageAttachment
	if json.Unmarshal([]byte(raw), &items) != nil {
		return nil
	}
	return items
}

func saveExportFile(path, format, data string) error {
	var payload []byte
	switch format {
	case "svg":
		payload = []byte(data)
	case "png":
		trimmed := data
		if idx := strings.Index(trimmed, ","); idx >= 0 && strings.HasPrefix(trimmed[:idx], "data:") {
			trimmed = trimmed[idx+1:]
		}
		decoded, err := base64.StdEncoding.DecodeString(trimmed)
		if err != nil || len(decoded) == 0 {
			return modelError("ValidationError")
		}
		payload = decoded
	default:
		return modelError("ValidationError")
	}
	return os.WriteFile(path, payload, 0o644)
}
