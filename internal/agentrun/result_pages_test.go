package agentrun

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestArticlePagesReassembleUnicodeAndScope(t *testing.T) {
	raw := strings.Repeat("中文🙂<&\"\\\n长段落\n\n", 4700) + "原文末尾证据"
	source := NewArticleSource("article", "original", raw)
	var results ResultStore
	first, err := results.PublishArticle("owner", "tool", "arguments", source)
	if err != nil {
		t.Fatal(err)
	}
	var body strings.Builder
	for next := first; ; {
		var page ArticlePage
		if err := json.Unmarshal([]byte(next), &page); err != nil || len(next) > ResultPageBytes || page.Offset != body.Len() ||
			page.End != page.Offset+len(page.Text) || !utf8.ValidString(page.Text) || page.Version != source.Version {
			t.Fatalf("invalid page at %d, size %d: %v", body.Len(), len(next), err)
		}
		body.WriteString(page.Text)
		if page.NextCursor == "" {
			break
		}
		for _, denied := range [][2]string{{"other", "tool"}, {"owner", "another-tool"}} {
			if _, err := results.Continue(denied[0], denied[1], page.NextCursor, nil); !errors.Is(err, ErrResultUnavailable) {
				t.Fatalf("wrong owner/tool: %v", err)
			}
		}
		next, err = results.Continue("owner", "tool", page.NextCursor, func(args string) error {
			if args != "arguments" {
				t.Errorf("original arguments: %q", args)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if body.String() != raw {
		t.Fatalf("reassembled %d bytes; want %d", body.Len(), len(raw))
	}

	if _, err := ResumeArticle(source, 1, source.Version); err == nil {
		t.Fatal("offset splits a UTF-8 rune")
	}
	if _, err := ResumeArticle(NewArticleSource("article", "original", "changed"), 10, source.Version); !errors.Is(err, ErrArticleVersion) {
		t.Fatalf("changed source: %v", err)
	}
	for _, entry := range results.entries {
		entry.expires = time.Now().Add(-time.Second)
	}
	var page ArticlePage
	_ = json.Unmarshal([]byte(first), &page)
	if _, err := results.Continue("owner", "tool", page.NextCursor, nil); !errors.Is(err, ErrResultUnavailable) {
		t.Fatalf("expired cursor: %v", err)
	}
}

func TestStructuredPagesRetainStatusItemsAndLargeFields(t *testing.T) {
	items := make([]string, 88)
	for i := range items {
		items[i] = fmt.Sprintf("%d:%s", i, strings.Repeat("中文🙂<>&", 24))
	}
	long := strings.Repeat("超长字段🙂\"<&\n", 2400)
	raw, err := json.Marshal(map[string]any{"ok": true, "data": map[string]any{
		"items": items, "next_cursor": "query-cursor", "has_more": true, "search_limited": true, "long": long,
	}})
	if err != nil {
		t.Fatal(err)
	}
	var results ResultStore
	first, err := results.PublishJSON(t.Context(), "owner", "list", "{}", raw)
	if err != nil {
		t.Fatal(err)
	}
	var body strings.Builder
	count := 0
	for next := first; ; {
		var page ResultPage
		if err := json.Unmarshal([]byte(next), &page); err != nil || !page.Paged || len(next) > ResultPageBytes ||
			!strings.Contains(string(page.Summary), `"next_cursor":"query-cursor"`) {
			t.Fatalf("invalid result page size %d: %v", len(next), err)
		}
		for _, value := range page.Fragments {
			switch {
			case value.Path == "/data/long":
				var part string
				if value.TextOffset == nil || *value.TextOffset != body.Len() || json.Unmarshal(value.Value, &part) != nil {
					t.Fatalf("invalid text fragment %v", value.TextOffset)
				}
				body.WriteString(part)
			case strings.HasPrefix(value.Path, "/data/items/"):
				index, _ := strconv.Atoi(strings.TrimPrefix(value.Path, "/data/items/"))
				var got string
				if index != count || json.Unmarshal(value.Value, &got) != nil || got != items[count] {
					t.Fatalf("unexpected item at %d", index)
				}
				count++
			}
		}
		if page.NextCursor == "" {
			break
		}
		next, err = results.Continue("owner", "list", page.NextCursor, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	if count != len(items) || body.String() != long {
		t.Fatalf("reassembled %d items and %d text bytes", count, body.Len())
	}
}
