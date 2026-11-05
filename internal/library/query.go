package library

import (
	"context"
	"crypto/rand"
	"time"
	"unicode"
	"unicode/utf16"

	"github.com/coolerks/Fealdhord/internal/app"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
	"strings"
)

type QueryRequest = database.ArticleQuery

type queryRepository interface {
	QueryArticles(context.Context, database.ArticleQuery) (database.QueryPage, error)
}

func projectArticle(row sqlc.ListArticlesRow, covers bool) Article {
	article := Article{ID: row.ID, FeedID: row.FeedID, URL: row.Url, Title: row.Title, FeedTitle: row.FeedTitle, Summary: row.Summary, SortAt: time.UnixMilli(row.SortAt).UTC().Format(time.RFC3339Nano), IsRead: row.IsRead == 1, IsFavorite: row.IsFavorite == 1, TitleMatches: []TextRange{}}
	if covers && row.CoverUrl.Valid && strings.HasPrefix(row.CoverUrl.String, "/media/") {
		cover := &Cover{URL: row.CoverUrl.String, Alt: row.CoverAlt.String}
		if row.CoverWidth.Valid && row.CoverWidth.Int64 > 0 {
			value := row.CoverWidth.Int64
			cover.Width = &value
		}
		if row.CoverHeight.Valid && row.CoverHeight.Int64 > 0 {
			value := row.CoverHeight.Int64
			cover.Height = &value
		}
		article.Cover = cover
	}
	return article
}

// titleMatches 返回 UTF-16 偏移，对应 JavaScript slice；从纯文本生成，绝不注入 snippet HTML。
func titleMatches(title, search string) []TextRange {
	runes := []rune(title)
	matched := make([]bool, len(runes))
	for _, term := range database.SearchTerms(search) {
		needle := []rune(term)
		for start := 0; start+len(needle) <= len(runes); start++ {
			found := true
			for index, r := range needle {
				if unicode.ToLower(runes[start+index]) != r {
					found = false
					break
				}
			}
			if found {
				for index := range needle {
					matched[start+index] = true
				}
			}
		}
	}
	result := []TextRange{}
	offset := 0
	for i := 0; i < len(runes); {
		if !matched[i] {
			offset += len(utf16.Encode(runes[i : i+1]))
			i++
			continue
		}
		start := offset
		for i < len(runes) && matched[i] {
			offset += len(utf16.Encode(runes[i : i+1]))
			i++
		}
		result = append(result, TextRange{Start: start, End: offset})
	}
	return result
}
func (s *Service) Query(ctx context.Context, request QueryRequest) ListResult {
	out := ListResult{RequestID: rand.Text()}
	repository, ok := s.repository.(queryRepository)
	if !ok {
		out.Error = safeError(database.ErrInvalidInput)
		return out
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	page, err := repository.QueryArticles(ctx, request)
	if err != nil {
		out.Error = safeError(err)
		return out
	}
	result := &Page{Items: []Article{}, HasMore: page.NextCursor != "", NextCursor: page.NextCursor, SnapshotID: page.SnapshotID, SearchLimited: page.SearchLimited}
	covers := s.coverAllowed == nil || s.coverAllowed(ctx)
	for _, row := range page.Items {
		article := projectArticle(row, covers)
		if request.Search != "" {
			article.TitleMatches = titleMatches(row.Title, request.Search)
		}
		result.Items = append(result.Items, article)
	}
	out.OK = true
	out.Data = result
	return out
}

type IndexResult struct {
	OK        bool          `json:"ok"`
	Error     *app.AppError `json:"error,omitempty"`
	RequestID string        `json:"request_id"`
}

func (s *Service) RebuildSearch(ctx context.Context) IndexResult {
	out := IndexResult{RequestID: rand.Text()}
	repository, ok := s.repository.(interface{ RebuildSearchIndex(context.Context) error })
	if !ok {
		out.Error = safeError(database.ErrInvalidInput)
		return out
	}
	if err := repository.RebuildSearchIndex(ctx); err != nil {
		out.Error = safeError(err)
		return out
	}
	out.OK = true
	return out
}
