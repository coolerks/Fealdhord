package database

import (
	"context"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

func isCJK(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
}
func SearchTerms(text string) []string { return searchSegments(text) }

func searchSegments(text string) []string {
	var result []string
	var current []rune
	kind := false
	flush := func() {
		if len(current) > 0 {
			result = append(result, string(current))
			current = nil
		}
	}
	for _, r := range strings.ToLower(text) {
		cjk := isCJK(r)
		if !cjk && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			flush()
			continue
		}
		if len(current) > 0 && kind != cjk {
			flush()
		}
		kind = cjk
		current = append(current, r)
	}
	flush()
	return result
}

// CJKTerms 是确定性派生索引，前缀避免单字／二字／拉丁分段互相混淆。
func CJKTerms(text string) string {
	seen := map[string]bool{}
	tokens := []string{}
	add := func(token string) {
		if !seen[token] {
			seen[token] = true
			tokens = append(tokens, token)
		}
	}
	for _, segment := range searchSegments(text) {
		chars := []rune(segment)
		if !isCJK(chars[0]) {
			add("词" + segment)
			continue
		}
		for index, r := range chars {
			add("单" + string(r))
			if index > 0 {
				add("双" + string(chars[index-1:index+1]))
			}
		}
	}
	return strings.Join(tokens, " ")
}
func compileSearch(query string) (string, []string, error) {
	if !utf8.ValidString(query) || utf8.RuneCountInString(query) > 512 {
		return "", nil, ErrInvalidInput
	}
	segments := searchSegments(query)
	clauses := []string{}
	quote := func(text string) string { return `"` + strings.ReplaceAll(text, `"`, `""`) + `"` }
	for _, segment := range segments {
		chars := []rune(segment)
		if !isCJK(chars[0]) {
			clauses = append(clauses, "("+quote(segment)+" OR cjk_terms : "+quote("词"+segment)+")")
			continue
		}
		if len(chars) == 1 {
			clauses = append(clauses, "cjk_terms : "+quote("单"+segment))
			continue
		}
		for index := 1; index < len(chars); index++ {
			clauses = append(clauses, "cjk_terms : "+quote("双"+string(chars[index-1:index+1])))
		}
	}
	return strings.Join(clauses, " AND "), segments, nil
}

type SearchHit struct{ ID, Title string }
type SearchResult struct {
	Items     []SearchHit
	Truncated bool
}

func (s *Store) Search(ctx context.Context, query string, limit int) (SearchResult, error) {
	result := SearchResult{Items: []SearchHit{}}
	if limit < 1 || limit > 1000 {
		return result, ErrInvalidInput
	}
	match, segments, err := compileSearch(strings.TrimSpace(query))
	if err != nil {
		return result, err
	}
	if match == "" {
		return result, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT a.id,a.title,a.author,a.summary,a.content_text FROM article_fts JOIN articles a ON a.rowid=article_fts.rowid WHERE article_fts MATCH ? ORDER BY bm25(article_fts,5,2,2,1,1),a.sort_at DESC,a.id DESC LIMIT 1001`, match)
	if err != nil {
		return result, fmt.Errorf("全文索引查询: %w", err)
	}
	defer rows.Close()
	candidates := 0
	for rows.Next() {
		var id, title, author, summary, text string
		if err := rows.Scan(&id, &title, &author, &summary, &text); err != nil {
			return result, err
		}
		candidates++
		if candidates > 1000 {
			result.Truncated = true
			break
		}
		content := strings.ToLower(strings.Join([]string{title, author, summary, text}, "\n"))
		valid := true
		for _, segment := range segments {
			chars := []rune(segment)
			if isCJK(chars[0]) && !strings.Contains(content, segment) {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		if len(result.Items) < limit {
			result.Items = append(result.Items, SearchHit{ID: id, Title: title})
		} else {
			result.Truncated = true
			break
		}
	}
	return result, rows.Err()
}
func (s *Store) CheckSearchIndex(ctx context.Context) error {
	s.write.Lock()
	defer s.write.Unlock()
	_, err := s.db.ExecContext(ctx, "INSERT INTO article_fts(article_fts,rank) VALUES('integrity-check',1)")
	return err
}
func (s *Store) RebuildSearchIndex(ctx context.Context) error {
	s.write.Lock()
	defer s.write.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "INSERT INTO article_fts(article_fts) VALUES('rebuild')"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE search_state SET generation=generation+1 WHERE id=1"); err != nil {
		return err
	}
	return tx.Commit()
}
