package database

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

type CategoryInfo struct {
	ID           string `json:"id"`
	ParentID     string `json:"parent_id"`
	Name         string `json:"name"`
	Icon         string `json:"icon"`
	Color        string `json:"color"`
	Description  string `json:"description"`
	SortPosition int64  `json:"sort_position"`
	Revision     int64  `json:"revision"`
	Total        int64  `json:"total"`
	Unread       int64  `json:"unread"`
}
type NavFeed struct {
	IconData string `json:"icon_data,omitempty"`
	ID       string `json:"id"`
	Title    string `json:"title"`
	Total    int64  `json:"total"`
	Unread   int64  `json:"unread"`
}
type NavTotals struct {
	Total     int64 `json:"total"`
	Unread    int64 `json:"unread"`
	Favorites int64 `json:"favorites"`
}
type Navigation struct {
	Categories []CategoryInfo `json:"categories"`
	Feeds      []NavFeed      `json:"feeds"`
	Totals     NavTotals      `json:"totals"`
}
type CategoryInput struct {
	ID           string `json:"id"`
	Revision     int64  `json:"revision"`
	ParentID     string `json:"parent_id"`
	Name         string `json:"name"`
	Icon         string `json:"icon"`
	Color        string `json:"color"`
	Description  string `json:"description"`
	SortPosition int64  `json:"sort_position"`
}
type CategoryDeletion struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Revision     int64  `json:"revision"`
	TreeRevision string `json:"tree_revision"`
	Children     int    `json:"children"`
	Articles     int64  `json:"articles"`
	Feeds        int64  `json:"feeds"`
}
type MembershipInfo struct {
	CategoryID string `json:"category_id"`
	Effective  bool   `json:"effective"`
	Manual     string `json:"manual"`
}

func (s *Store) Navigation(ctx context.Context) (Navigation, error) {
	out := Navigation{Categories: []CategoryInfo{}, Feeds: []NavFeed{}}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(ctx, `SELECT COUNT(CASE WHEN feed.archived_at IS NULL THEN 1 END),COUNT(CASE WHEN feed.archived_at IS NULL AND COALESCE(r.is_read,0)=0 THEN 1 END),COUNT(f.article_id) FROM articles a JOIN feeds feed ON feed.id=a.feed_id LEFT JOIN reading_states r ON r.article_id=a.id LEFT JOIN favorites f ON f.article_id=a.id`).Scan(&out.Totals.Total, &out.Totals.Unread, &out.Totals.Favorites)
	if err != nil {
		return out, err
	}
	rows, err := tx.QueryContext(ctx, `WITH counts AS(SELECT ec.category_id,COUNT(*) AS total,SUM(CASE WHEN COALESCE(r.is_read,0)=0 THEN 1 ELSE 0 END) AS unread FROM effective_article_categories ec JOIN articles a ON a.id=ec.article_id JOIN feeds feed ON feed.id=a.feed_id LEFT JOIN reading_states r ON r.article_id=ec.article_id WHERE feed.archived_at IS NULL GROUP BY ec.category_id)
 SELECT c.id,COALESCE(c.parent_id,''),c.name,c.icon,c.color,c.description,c.sort_position,c.revision,COALESCE(n.total,0),COALESCE(n.unread,0) FROM categories c LEFT JOIN counts n ON n.category_id=c.id ORDER BY c.sort_position,c.name,c.id`)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var item CategoryInfo
		if err = rows.Scan(&item.ID, &item.ParentID, &item.Name, &item.Icon, &item.Color, &item.Description, &item.SortPosition, &item.Revision, &item.Total, &item.Unread); err != nil {
			rows.Close()
			return out, err
		}
		out.Categories = append(out.Categories, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return out, err
	}
	rows.Close()
	rows, err = tx.QueryContext(ctx, `WITH counts AS(SELECT a.feed_id,COUNT(*) AS total,SUM(CASE WHEN COALESCE(r.is_read,0)=0 THEN 1 ELSE 0 END) AS unread FROM articles a LEFT JOIN reading_states r ON r.article_id=a.id GROUP BY a.feed_id)
 SELECT f.id,f.title,f.icon_url,COALESCE(n.total,0),COALESCE(n.unread,0) FROM feeds f LEFT JOIN counts n ON n.feed_id=f.id WHERE f.archived_at IS NULL ORDER BY f.sort_position,f.title,f.id`)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var item NavFeed
		if err = rows.Scan(&item.ID, &item.Title, &item.IconData, &item.Total, &item.Unread); err != nil {
			rows.Close()
			return out, err
		}
		out.Feeds = append(out.Feeds, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return out, err
	}
	rows.Close()
	return out, tx.Commit()
}
func validCategory(input CategoryInput) bool {
	if strings.TrimSpace(input.Name) == "" || len(input.Name) > 256 || len(input.Description) > 2048 || len(input.ParentID) > 256 || len(input.ID) > 256 || input.SortPosition < -1000000 || input.SortPosition > 1000000 {
		return false
	}
	switch input.Icon {
	case "", "folder", "book", "radio-tower", "favorite", "ai":
	default:
		return false
	}
	if input.Color != "" {
		if len(input.Color) != 7 || input.Color[0] != '#' {
			return false
		}
		if _, err := hex.DecodeString(input.Color[1:]); err != nil {
			return false
		}
	}
	return true
}
func (s *Store) SaveCategory(ctx context.Context, input CategoryInput) (string, error) {
	input.Name = strings.TrimSpace(input.Name)
	if !validCategory(input) {
		return "", ErrInvalidInput
	}
	id := input.ID
	now := time.Now().UTC().UnixMilli()
	err := s.transaction(ctx, func(q *sqlc.Queries) error {
		if id == "" {
			id = rand.Text()
			if err := q.CreateCategory(ctx, sqlc.CreateCategoryParams{ID: id, ParentID: nullString(input.ParentID), Name: input.Name, NormalizedName: strings.ToLower(input.Name), CreatedAt: now, UpdatedAt: now}); err != nil {
				return err
			}
			input.Revision = 0
		}
		affected, err := q.SaveCategory(ctx, sqlc.SaveCategoryParams{ID: id, Revision: input.Revision, ParentID: nullString(input.ParentID), Name: input.Name, NormalizedName: strings.ToLower(input.Name), Icon: input.Icon, Color: input.Color, Description: input.Description, SortPosition: input.SortPosition, UpdatedAt: now})
		if err != nil {
			return err
		}
		if affected != 1 {
			return ErrConflict
		}
		return nil
	})
	return id, err
}
func treeHash(categories []sqlc.Category) string {
	data, _ := json.Marshal(categories)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func (s *Store) PreviewCategoryDeletion(ctx context.Context, id string) (CategoryDeletion, error) {
	out := CategoryDeletion{ID: id}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	q := s.queries.WithTx(tx)
	category, err := q.GetCategory(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			err = ErrNotFound
		}
		return out, err
	}
	categories, err := q.ListCategories(ctx)
	if err != nil {
		return out, err
	}
	out.Name = category.Name
	out.Revision = category.Revision
	out.TreeRevision = treeHash(categories)
	for _, row := range categories {
		if row.ParentID.Valid && row.ParentID.String == id {
			out.Children++
		}
	}
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM effective_article_categories WHERE category_id=?", id).Scan(&out.Articles); err != nil {
		return out, err
	}
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM feed_categories WHERE category_id=?", id).Scan(&out.Feeds); err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func (s *Store) DeleteCategoryChecked(ctx context.Context, input CategoryDeletion) error {
	return s.transaction(ctx, func(q *sqlc.Queries) error {
		categories, err := q.ListCategories(ctx)
		if err != nil {
			return err
		}
		if input.TreeRevision == "" || treeHash(categories) != input.TreeRevision {
			return ErrConflict
		}
		category, err := q.GetCategory(ctx, input.ID)
		if err != nil {
			return err
		}
		if category.Revision != input.Revision {
			return ErrConflict
		}
		if err = q.MoveChildrenToParent(ctx, sqlc.MoveChildrenToParentParams{ParentID: category.ParentID, ParentID_2: nullString(input.ID), UpdatedAt: time.Now().UTC().UnixMilli()}); err != nil {
			return err
		}
		_, err = q.DeleteCategory(ctx, input.ID)
		return err
	})
}
func (s *Store) ArticleMemberships(ctx context.Context, id string) ([]MembershipInfo, error) {
	if _, err := s.queries.GetArticle(ctx, id); err != nil {
		if err == sql.ErrNoRows {
			err = ErrNotFound
		}
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT c.id,CASE WHEN ec.article_id IS NULL THEN 0 ELSE 1 END,COALESCE(ac.state,'auto') FROM categories c LEFT JOIN effective_article_categories ec ON ec.category_id=c.id AND ec.article_id=? LEFT JOIN article_categories ac ON ac.category_id=c.id AND ac.article_id=? AND ac.source='manual' ORDER BY c.sort_position,c.name,c.id`, id, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MembershipInfo{}
	for rows.Next() {
		var item MembershipInfo
		if err = rows.Scan(&item.CategoryID, &item.Effective, &item.Manual); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
