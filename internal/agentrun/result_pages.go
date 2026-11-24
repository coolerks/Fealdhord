package agentrun

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	ResultPageBytes  = 12 << 10
	maxResultBytes   = 48 << 20
	maxStoreBytes    = 64 << 20
	maxStoredResults = 128
	resultLifetime   = 20 * time.Minute
)

var ErrResultUnavailable = errors.New("ToolResultUnavailable")
var ErrResultLimit = errors.New("ToolResultLimit")
var ErrArticleVersion = errors.New("ArticleVersionChanged")

type resultStoreKey struct{}

func ResultStoreFrom(ctx context.Context) *ResultStore {
	store, _ := ctx.Value(resultStoreKey{}).(*ResultStore)
	return store
}

// ResultStore holds immutable, authorized snapshots. Its owner key is supplied
// by the caller, never by the model or the MCP request.
type ResultStore struct {
	mu      sync.Mutex
	entries map[string]*resultSnapshot
	size    int
}

type resultSnapshot struct {
	pageSize               int
	owner, tool, arguments string
	expires                time.Time
	size                   int
	pages                  [][]ResultFragment
	article                *ArticleSource
	boundaries             map[int]bool
	summary                json.RawMessage
}

type ArticleSource struct {
	Start    int    `json:"-"`
	ID       string `json:"id"`
	Source   string `json:"source"`
	Text     string `json:"-"`
	Version  string `json:"version"`
	PageOnly bool   `json:"page_only,omitempty"`
}

func ResumeArticle(source ArticleSource, offset int, version string) (ArticleSource, error) {
	if version != "" && version != source.Version {
		return ArticleSource{}, ErrArticleVersion
	}
	if offset < 0 || offset > len(source.Text) || (offset != 0 && version == "") ||
		(offset < len(source.Text) && !utf8.RuneStart(source.Text[offset])) {
		return ArticleSource{}, errors.New("ValidationError")
	}
	source.Start = offset
	return source, nil
}

func NewArticleSource(id, source, text string) ArticleSource {
	hash := sha256.Sum256([]byte(text))
	return ArticleSource{ID: id, Source: source, Text: text, Version: hex.EncodeToString(hash[:])}
}

type ResultFragment struct {
	Path       string          `json:"path"`
	Value      json.RawMessage `json:"value"`
	TextOffset *int            `json:"text_offset,omitempty"`
}

type ResultPage struct {
	Paged      bool             `json:"paged"`
	NextCursor string           `json:"next_cursor,omitempty"`
	Page       int              `json:"page"`
	Pages      int              `json:"pages"`
	Fragments  []ResultFragment `json:"fragments"`
	Summary    json.RawMessage  `json:"summary,omitempty"`
}

type ArticlePage struct {
	ArticleSource
	Text       string `json:"text"`
	Offset     int    `json:"offset"`
	End        int    `json:"end"`
	Total      int    `json:"total_bytes"`
	NextCursor string `json:"next_cursor,omitempty"`
	Coverage   string `json:"coverage"`
}

func (s *ResultStore) save(item *resultSnapshot) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for key, previous := range s.entries {
		if !now.Before(previous.expires) {
			delete(s.entries, key)
			s.size -= previous.size
		}
	}
	if item.size > maxResultBytes || item.size > maxStoreBytes-s.size || len(s.entries) >= maxStoredResults {
		return "", ErrResultLimit
	}
	if s.entries == nil {
		s.entries = make(map[string]*resultSnapshot)
	}
	key := rand.Text()
	item.expires = now.Add(resultLifetime)
	s.entries[key] = item
	s.size += item.size
	return key, nil
}

// Continue verifies ownership and the original target before reading a page.
func (s *ResultStore) Continue(owner, tool, cursor string, verify func(string) error) (string, error) {
	if s == nil {
		return "", ErrResultUnavailable
	}
	key, raw, found := strings.Cut(cursor, ".")
	index, err := strconv.Atoi(raw)
	if !found || err != nil || index < 0 {
		return "", ErrResultUnavailable
	}
	s.mu.Lock()
	item := s.entries[key]
	s.mu.Unlock()
	if item == nil || item.owner != owner || item.tool != tool || !time.Now().Before(item.expires) {
		return "", ErrResultUnavailable
	}
	if verify != nil {
		if err := verify(item.arguments); err != nil {
			return "", err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if item.article != nil {
		if !item.boundaries[index] {
			return "", ErrResultUnavailable
		}
		return item.articlePage(key, index), nil
	}
	if index >= len(item.pages) {
		return "", ErrResultUnavailable
	}
	return item.resultPage(key, index), nil
}

func (s *ResultStore) PublishArticle(owner, tool, arguments string, source ArticleSource) (string, error) {
	if len(source.Text) > maxResultBytes || !utf8.ValidString(source.Text) || source.Start < 0 || source.Start > len(source.Text) || len(source.ID) > 256 {
		return "", ErrResultLimit
	}
	item := &resultSnapshot{owner: owner, tool: tool, arguments: arguments, article: &source,
		size: len(source.Text) + len(arguments) + len(source.ID) + len(source.Version) + 256, boundaries: map[int]bool{source.Start: true}}
	// A one-page response needs no snapshot.
	first := item.articlePage("", source.Start)
	if len(item.boundaries) == 1 {
		return first, nil
	}
	key, err := s.save(item)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return item.articlePage(key, source.Start), nil
}

func (item *resultSnapshot) articlePage(key string, start int) string {
	source := *item.article
	page := ArticlePage{ArticleSource: source, Offset: start, Total: len(source.Text)}
	remaining := source.Text[start:]
	// Reserve the full cursor length when determining the page size.
	page.NextCursor = strings.Repeat("x", 64)
	page.Coverage = "range_complete"
	end := fittingText(remaining, func(chunk string) bool {
		page.Text = chunk
		page.End = start + len(chunk)
		data, _ := json.Marshal(page)
		return len(data) <= ResultPageBytes
	})
	if end < len(remaining) {
		if paragraph := strings.LastIndex(remaining[:end], "\n\n"); paragraph >= end/2 {
			end = paragraph + 2
		}
	}
	page.Text = remaining[:end]
	page.End = start + end
	page.NextCursor = ""
	if page.End < page.Total {
		page.Coverage = "partial"
		item.boundaries[page.End] = true
		page.NextCursor = fmt.Sprintf("%s.%d", key, page.End)
	} else if page.Source == "summary_only" {
		page.Coverage = "summary_only"
	} else if start == 0 {
		page.Coverage = "complete"
	}
	data, _ := json.Marshal(page)
	return string(data)
}

func fittingText(value string, fits func(string) bool) int {
	low, high := 0, min(len(value), ResultPageBytes)
	for low < high {
		mid := (low + high + 1) / 2
		if fits(value[:mid]) {
			low = mid
		} else {
			high = mid - 1
		}
	}
	for low < len(value) && !utf8.RuneStart(value[low]) {
		low--
	}
	return low
}

func (s *ResultStore) PublishJSON(ctx context.Context, owner, tool, arguments string, data []byte) (string, error) {
	if len(data) <= ResultPageBytes {
		return string(data), nil
	}
	if len(data) > maxResultBytes {
		return "", ErrResultLimit
	}
	item := &resultSnapshot{owner: owner, tool: tool, arguments: arguments, size: len(arguments)}
	item.summary = resultSummary(data)
	if err := item.addFragment(ctx, 0, "", data); err != nil {
		return "", err
	}
	key, err := s.save(item)
	if err != nil {
		return "", err
	}
	return item.resultPage(key, 0), nil
}

func (item *resultSnapshot) resultPage(key string, index int) string {
	page := ResultPage{Paged: true, Page: index + 1, Pages: len(item.pages), Fragments: item.pages[index], Summary: item.summary}
	if index+1 < len(item.pages) {
		page.NextCursor = fmt.Sprintf("%s.%d", key, index+1)
	}
	data, _ := json.Marshal(page)
	return string(data)
}

// Keep operation status and collection navigation visible on every page.
// The fragments contain the original values and can reconstruct the full result.
func resultSummary(data []byte) json.RawMessage {
	var root map[string]json.RawMessage
	if json.Unmarshal(data, &root) != nil {
		var items []struct {
			OK *bool `json:"ok"`
		}
		if json.Unmarshal(data, &items) != nil || len(items) == 0 {
			return nil
		}
		succeeded, failed := 0, 0
		for _, item := range items {
			if item.OK != nil {
				if *item.OK {
					succeeded++
				} else {
					failed++
				}
			}
		}
		counts := map[string]int{"total": len(items)}
		if succeeded+failed > 0 {
			counts["success_count"], counts["failed_count"] = succeeded, failed
		}
		raw, _ := json.Marshal(counts)
		return raw
	}
	out := make(map[string]json.RawMessage)
	for _, key := range []string{"ok", "isError", "error", "request_id"} {
		if value := root[key]; len(value) > 0 && len(value) <= 512 {
			out[key] = value
		}
	}
	var nested map[string]json.RawMessage
	_ = json.Unmarshal(root["data"], &nested)
	for _, key := range []string{"next_cursor", "has_more", "search_limited", "preview_id", "operation_id", "status", "completed", "total", "success_count", "failed_count", "skipped_count", "cancelled_count", "report"} {
		if value := nested[key]; len(value) > 0 && len(value) <= 512 {
			out[key] = value
		}
	}
	var report map[string]json.RawMessage
	if json.Unmarshal(nested["report"], &report) == nil {
		delete(report, "feed_ids")
		if summary, err := json.Marshal(report); err == nil && len(summary) <= 512 {
			out["report"] = summary
		}
	}
	result, _ := json.Marshal(out)
	if len(out) == 0 || len(result) > 2<<10 {
		return nil
	}
	return result
}

func (item *resultSnapshot) appendFragment(fragment ResultFragment) bool {
	data, _ := json.Marshal(fragment)
	const fragmentBytes = 9 << 10
	if len(data) > fragmentBytes {
		return false
	}
	index := len(item.pages) - 1
	if index >= 0 {
		current := item.pages[index]
		if item.pageSize+len(data)+1 <= fragmentBytes {
			item.pages[index] = append(current, fragment)
			item.pageSize += len(data) + 1
			item.size += len(data) + 64
			return true
		}
	}
	item.pages = append(item.pages, []ResultFragment{fragment})
	item.pageSize = len(data)
	item.size += len(data) + 64
	return true
}

func (item *resultSnapshot) addFragment(ctx context.Context, depth int, path string, value json.RawMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if item.size > maxResultBytes || len(path) > 4096 || depth > 48 {
		return ErrResultLimit
	}
	if len(value) < ResultPageBytes && item.appendFragment(ResultFragment{Path: path, Value: value}) {
		return nil
	}
	switch value[0] {
	case '{':
		item.appendFragment(ResultFragment{Path: path, Value: json.RawMessage(`{}`)})
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(value, &fields); err != nil {
			return err
		}
		keys := make([]string, 0, len(fields))
		for key := range fields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if err := item.addFragment(ctx, depth+1, path+"/"+strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1"), fields[key]); err != nil {
				return err
			}
		}
	case '[':
		item.appendFragment(ResultFragment{Path: path, Value: json.RawMessage(`[]`)})
		var items []json.RawMessage
		if err := json.Unmarshal(value, &items); err != nil {
			return err
		}
		for i, value := range items {
			if err := item.addFragment(ctx, depth+1, path+"/"+strconv.Itoa(i), value); err != nil {
				return err
			}
		}
	case '"':
		var valueText string
		if err := json.Unmarshal(value, &valueText); err != nil {
			return err
		}
		for offset := 0; offset < len(valueText); {
			if err := ctx.Err(); err != nil {
				return err
			}
			start := offset
			fragment := ResultFragment{Path: path, TextOffset: &start}
			length := fittingText(valueText[offset:], func(part string) bool {
				fragment.Value, _ = json.Marshal(part)
				data, _ := json.Marshal(fragment)
				return len(data) <= 9<<10
			})
			if length == 0 {
				return ErrResultLimit
			}
			offset += length
			fragment.Value, _ = json.Marshal(valueText[start:offset])
			item.appendFragment(fragment)
		}
	default:
		return ErrResultLimit
	}
	return nil
}
