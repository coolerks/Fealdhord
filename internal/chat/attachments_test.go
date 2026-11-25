package chat

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareAttachmentsMultimodalAndPaths(t *testing.T) {
	dir := t.TempDir()
	notePath := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(notePath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	pngBytes := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 1}
	pngB64 := base64.StdEncoding.EncodeToString(pngBytes)

	// Non-multimodal model must reject images with ModelImageUnsupported.
	_, _, _, _, err := prepareAttachments([]SendAttachment{
		{Kind: "image", Name: "shot.png", MIMEType: "image/png", Data: pngB64},
	}, false)
	if err == nil || errorCode(err) != "ModelImageUnsupported" {
		t.Fatalf("expected ModelImageUnsupported, got %v", err)
	}

	// Multimodal model accepts pasted image + file + folder paths.
	blocks, readOnly, persisted, suffix, err := prepareAttachments([]SendAttachment{
		{Kind: "image", Name: "shot.png", MIMEType: "image/png", Data: pngB64},
		{Kind: "file", Name: "note.txt", Path: notePath},
		{Kind: "folder", Name: filepath.Base(dir), Path: dir},
	}, true)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(blocks) != 1 || blocks[0].Type != "image" || blocks[0].MIMEType != "image/png" {
		t.Fatalf("unexpected image blocks: %+v", blocks)
	}
	if len(readOnly) != 2 {
		t.Fatalf("expected 2 read-only paths, got %v", readOnly)
	}
	if len(persisted) != 3 || persisted[0].Preview == "" {
		t.Fatalf("unexpected persisted metadata: %+v", persisted)
	}
	if !strings.Contains(suffix, notePath) || !strings.Contains(suffix, dir) {
		t.Fatalf("prompt suffix missing paths: %q", suffix)
	}
}

func TestSaveExportFileSVGAndPNG(t *testing.T) {
	dir := t.TempDir()
	svgPath := filepath.Join(dir, "flow.svg")
	if err := saveExportFile(svgPath, "svg", "<svg></svg>"); err != nil {
		t.Fatalf("save svg: %v", err)
	}
	if got, _ := os.ReadFile(svgPath); string(got) != "<svg></svg>" {
		t.Fatalf("svg content mismatch: %q", got)
	}

	pngPath := filepath.Join(dir, "flow.png")
	raw := []byte{0x89, 'P', 'N', 'G'}
	if err := saveExportFile(pngPath, "png", "data:image/png;base64,"+base64.StdEncoding.EncodeToString(raw)); err != nil {
		t.Fatalf("save png: %v", err)
	}
	if got, _ := os.ReadFile(pngPath); string(got) != string(raw) {
		t.Fatalf("png content mismatch: %x", got)
	}
}
