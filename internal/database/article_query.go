package database

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

var ErrCursorExpired = errors.New("查询游标已过期")

type ArticleScope struct {
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
}
type ArticleQuery struct {
	Scope    ArticleScope `json:"scope"`
	Search   string       `json:"search,omitempty"`
	Read     string       `json:"read,omitempty"`
	Favorite *bool        `json:"favorite,omitempty"`
	Sort     string       `json:"sort"`
	Cursor   string       `json:"cursor,omitempty"`
	Limit    int          `json:"limit,omitempty"`
}
type QueryPage struct {
	Items                  []sqlc.ListArticlesRow
	NextCursor, SnapshotID string
	SearchLimited          bool
}
type queryCursor struct {
	Hash           string  `json:"q"`
	Upper          int64   `json:"u"`
	Generation     int64   `json:"g"`
	At             int64   `json:"a"`
	ID             string  `json:"i"`
	Rank           float64 `json:"r"`
	Since          int64   `json:"since,omitempty"`
	FeatureProfile string  `json:"feature_profile,omitempty"`
	Expires        int64   `json:"e"`
}

func NormalizeArticleQuery(query ArticleQuery) (ArticleQuery, error) {
	query.Search = strings.TrimSpace(query.Search)
	if query.Scope.Kind == "" {
		query.Scope.Kind = "all"
	}
	if query.Limit == 0 {
		query.Limit = 50
	}
	if query.Read == "" {
		query.Read = "all"
	}
	if query.Sort == "" {
		if query.Search != "" {
			query.Sort = "relevance"
		} else {
			query.Sort = "published_desc"
		}
	}
	if query.Search == "" && query.Sort == "relevance" {
		query.Sort = "published_desc"
	}
	if query.Limit < 1 || query.Limit > 200 || len(query.Cursor) > 4096 || len(query.Scope.ID) > 256 {
		return query, ErrInvalidInput
	}
	switch query.Scope.Kind {
	case "all", "unread", "favorites", "featured":
		if query.Scope.ID != "" {
			return query, ErrInvalidInput
		}
	case "feed", "category":
		if query.Scope.ID == "" {
			return query, ErrInvalidInput
		}
	default:
		return query, ErrInvalidInput
	}
	if query.Read != "all" && query.Read != "read" && query.Read != "unread" {
		return query, ErrInvalidInput
	}
	if query.Sort != "published_desc" && query.Sort != "published_asc" && query.Sort != "relevance" {
		return query, ErrInvalidInput
	}
	_, _, err := compileSearch(query.Search)
	return query, err
}
func queryHash(query ArticleQuery) string {
	query.Cursor = ""
	query.Limit = 0
	data, _ := json.Marshal(query)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func (s *Store) encodeCursor(cursor queryCursor) string {
	data, _ := json.Marshal(cursor)
	mac := hmac.New(sha256.New, []byte(s.cursorSecret))
	mac.Write(data)
	return base64.RawURLEncoding.EncodeToString(data) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func (s *Store) decodeCursor(raw string) (queryCursor, error) {
	var cursor queryCursor
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return cursor, ErrCursorExpired
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return cursor, ErrCursorExpired
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return cursor, ErrCursorExpired
	}
	mac := hmac.New(sha256.New, []byte(s.cursorSecret))
	mac.Write(data)
	if !hmac.Equal(signature, mac.Sum(nil)) || json.Unmarshal(data, &cursor) != nil || cursor.Expires < time.Now().Unix() {
		return cursor, ErrCursorExpired
	}
	return cursor, nil
}

// QueryArticles 在同一读事务中读取版本、快照边界和结果，网络绝不进入此事务。
func (s *Store) QueryArticles(ctx context.Context, raw ArticleQuery) (QueryPage, error) {
	out := QueryPage{Items: []sqlc.ListArticlesRow{}}
	query, err := NormalizeArticleQuery(raw)
	if err != nil {
		return out, err
	}
	s.cursorOnce.Do(func() { s.cursorSecret = rand.Text() })
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	var generation, upper int64
	if err = tx.QueryRowContext(ctx, "SELECT generation FROM search_state WHERE id=1").Scan(&generation); err != nil {
		return out, err
	}
	if err = tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(rowid),0) FROM articles").Scan(&upper); err != nil {
		return out, err
	}
	featured := query.Scope.Kind == "featured"
	var featureProfile string
	if featured {
		profile, err := loadArticleAISnapshot(ctx, s.queries.WithTx(tx), "", "featured", false)
		if errors.Is(err, ErrArticleAIDisabled) || errors.Is(err, ErrArticleAIModel) {
			if query.Cursor != "" {
				return out, ErrCursorExpired
			}
			return out, nil
		}
		if err != nil {
			return out, err
		}
		featureProfile = profile.ProfileKey
	}
	cursor := queryCursor{Hash: queryHash(query), Upper: upper, Generation: generation, FeatureProfile: featureProfile, Expires: time.Now().Add(30 * time.Minute).Unix()}
	if featured {
		cursor.Since = time.Now().Add(-7 * 24 * time.Hour).UnixMilli()
	}
	if query.Cursor != "" {
		cursor, err = s.decodeCursor(query.Cursor)
		if err != nil || cursor.Hash != queryHash(query) || cursor.FeatureProfile != featureProfile || ((query.Search != "" || featured) && cursor.Generation != generation) {
			return out, ErrCursorExpired
		}
	}
	out.SnapshotID = fmt.Sprintf("%d:%d", cursor.Upper, cursor.Generation)
	match, segments, err := compileSearch(query.Search)
	if err != nil {
		return out, err
	}
	// 仅标点的非空查询不退化为无界全库查询。
	if query.Search != "" && match == "" {
		return out, nil
	}
	conditions := []string{"a.rowid<=?", "birth.generation<=?"}
	args := []any{cursor.Upper, cursor.Generation}
	if query.Scope.Kind == "feed" {
		conditions = append(conditions, "a.feed_id=?")
		args = append(args, query.Scope.ID)
	}
	if query.Scope.Kind == "category" {
		conditions = append(conditions, "EXISTS(SELECT 1 FROM effective_article_categories ec WHERE ec.article_id=a.id AND ec.category_id=?)")
		args = append(args, query.Scope.ID)
	}
	if featured {
		conditions = append(conditions, "f.archived_at IS NULL", "a.sort_at>=?", "ar.profile_key=?", "ar.importance>=0.7", "ar.relevance>=0.6")
		args = append(args, cursor.Since, featureProfile)
	}
	if query.Scope.Kind == "unread" || query.Read == "unread" {
		conditions = append(conditions, "COALESCE(r.is_read,0)=0")
	}
	if query.Read == "read" {
		conditions = append(conditions, "COALESCE(r.is_read,0)=1")
	}
	if query.Scope.Kind == "favorites" || (query.Favorite != nil && *query.Favorite) {
		conditions = append(conditions, "fav.article_id IS NOT NULL")
	}
	if query.Favorite != nil && !*query.Favorite {
		conditions = append(conditions, "fav.article_id IS NULL")
	}
	join, score, content := "", "0.0", "''"
	if match != "" {
		join = " JOIN article_fts ON article_fts.rowid=a.rowid "
		conditions = append(conditions, "article_fts MATCH ?", "article_fts.rank MATCH 'bm25(5,2,2,1,1)'")
		args = append(args, match)
		score = "article_fts.rank"
		content = "a.title || char(10) || a.author || char(10) || a.summary || char(10) || a.content_text"
	}
	if featured {
		join += ` JOIN article_ai_results ar ON ar.article_id=a.id AND ar.task='featured' AND ar.content_hash=a.content_hash
 JOIN article_ai_settings ai_settings ON ai_settings.id=1 AND ai_settings.featured=1 AND ai_settings.revision=ar.settings_revision
 JOIN model_assignments ma ON ma.task_type='featured' AND ma.model_id=ar.model_id
 JOIN ai_models am ON am.id=ma.model_id AND am.enabled=1 AND am.revision=ar.model_revision
 JOIN ai_providers ap ON ap.id=am.provider_id AND ap.enabled=1 AND ap.revision=ar.provider_revision `
		score = "-(0.6*ar.importance+0.4*ar.relevance)"
	}
	order := "a.sort_at DESC,a.id DESC"
	if query.Sort == "published_asc" {
		order = "a.sort_at ASC,a.id ASC"
	}
	if query.Sort == "relevance" || featured {
		order = score + " ASC,a.sort_at DESC,a.id DESC"
	}
	if query.Cursor != "" {
		switch {
		case query.Sort == "relevance" || featured:
			conditions = append(conditions, "("+score+">? OR ("+score+"=? AND (a.sort_at<? OR (a.sort_at=? AND a.id<?))))")
			args = append(args, cursor.Rank, cursor.Rank, cursor.At, cursor.At, cursor.ID)
		case query.Sort == "published_asc":
			conditions = append(conditions, "(a.sort_at,a.id)>(?,?)")
			args = append(args, cursor.At, cursor.ID)
		default:
			conditions = append(conditions, "(a.sort_at,a.id)<(?,?)")
			args = append(args, cursor.At, cursor.ID)
		}
	}
	scanLimit := query.Limit + 1
	if match != "" {
		scanLimit = 1001
	}
	statement := `SELECT a.id,a.title,a.url,a.author,substr(a.summary,1,500),a.sort_at,a.feed_id,f.title,
 COALESCE(r.is_read,0),CASE WHEN fav.article_id IS NULL THEN 0 ELSE 1 END,
 m.url,m.width,m.height,m.alt,a.revision,` + score + `,` + content + `
 FROM articles a JOIN feeds f ON f.id=a.feed_id JOIN article_query_birth birth ON birth.article_id=a.id
 LEFT JOIN reading_states r ON r.article_id=a.id LEFT JOIN favorites fav ON fav.article_id=a.id
 LEFT JOIN article_media m ON m.article_id=a.id AND m.status='ready' ` + join + `
 WHERE ` + strings.Join(conditions, " AND ") + ` ORDER BY ` + order + ` LIMIT ?`
	args = append(args, scanLimit)
	rows, err := tx.QueryContext(ctx, statement, args...)
	if err != nil {
		return out, err
	}
	scanned := 0
	last := cursor
	for rows.Next() {
		row := sqlc.ListArticlesRow{}
		var rank float64
		var fullText string
		if err = rows.Scan(&row.ID, &row.Title, &row.Url, &row.Author, &row.Summary, &row.SortAt, &row.FeedID, &row.FeedTitle, &row.IsRead, &row.IsFavorite, &row.CoverUrl, &row.CoverWidth, &row.CoverHeight, &row.CoverAlt, &row.Revision, &rank, &fullText); err != nil {
			rows.Close()
			return out, err
		}
		if scanned >= 1000 && match != "" {
			out.SearchLimited = true
			out.NextCursor = s.encodeCursor(last)
			break
		}
		scanned++
		valid := true
		for _, segment := range segments {
			if isCJK([]rune(segment)[0]) && !strings.Contains(strings.ToLower(fullText), segment) {
				valid = false
				break
			}
		}
		if valid && len(out.Items) == query.Limit {
			out.NextCursor = s.encodeCursor(last)
			break
		}
		last.At = row.SortAt
		last.ID = row.ID
		last.Rank = rank
		if valid {
			out.Items = append(out.Items, row)
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return out, err
	}
	if err = rows.Close(); err != nil {
		return out, err
	}
	return out, tx.Commit()
}
