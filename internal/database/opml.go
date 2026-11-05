package database

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

type ImportOutline struct {
	Key       string `json:"key"`
	ParentKey string `json:"parent_key"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	SiteURL   string `json:"site_url"`
	Kind      string `json:"kind"`
	Status    string `json:"status"`
	Reason    string `json:"reason"`
	Depth     int    `json:"depth"`
}
type ImportPlan struct {
	Revision string          `json:"revision"`
	Items    []ImportOutline `json:"items"`
}
type ImportReport struct {
	FeedsAdded        int      `json:"feeds_added"`
	FeedsReused       int      `json:"feeds_reused"`
	FeedsRestored     int      `json:"feeds_restored"`
	CategoriesAdded   int      `json:"categories_added"`
	AssociationsAdded int      `json:"associations_added"`
	Skipped           int      `json:"skipped"`
	FeedIDs           []string `json:"feed_ids"`
}
type OPMLSnapshot struct {
	Categories []sqlc.Category
	Feeds      []sqlc.Feed
	Relations  map[string][]string
}

func opmlSnapshot(ctx context.Context, q *sqlc.Queries) (OPMLSnapshot, error) {
	out := OPMLSnapshot{Relations: map[string][]string{}}
	var err error
	out.Categories, err = q.ListCategories(ctx)
	if err != nil {
		return out, err
	}
	out.Feeds, err = q.ListAllFeeds(ctx)
	if err != nil {
		return out, err
	}
	rows, err := q.ListAllFeedCategories(ctx)
	if err != nil {
		return out, err
	}
	for _, row := range rows {
		out.Relations[row.FeedID] = append(out.Relations[row.FeedID], row.CategoryID)
	}
	return out, nil
}
func importRevision(snapshot OPMLSnapshot) string {
	// 检查身份、配置和树版本，刷新时更新检查时间／validator不会使预览无故失效。
	feeds := make([]string, 0, len(snapshot.Feeds))
	for _, f := range snapshot.Feeds {
		feeds = append(feeds, fmt.Sprintf("%s|%s|%d|%v", f.ID, f.NormalizedUrl, f.Revision, f.ArchivedAt))
	}
	data, _ := json.Marshal(struct {
		Categories []sqlc.Category
		Feeds      []string
		Relations  map[string][]string
	}{snapshot.Categories, feeds, snapshot.Relations})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func (s *Store) OPMLSnapshot(ctx context.Context) (OPMLSnapshot, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return OPMLSnapshot{}, err
	}
	defer tx.Rollback()
	snapshot, err := opmlSnapshot(ctx, s.queries.WithTx(tx))
	if err != nil {
		return snapshot, err
	}
	return snapshot, tx.Commit()
}
func (s *Store) PlanOPML(ctx context.Context, items []ImportOutline) (ImportPlan, error) {
	snapshot, err := s.OPMLSnapshot(ctx)
	if err != nil {
		return ImportPlan{}, err
	}
	out := ImportPlan{Revision: importRevision(snapshot), Items: append([]ImportOutline{}, items...)}
	folders := map[string]string{}
	keys := map[string]string{}
	feeds := map[string]sqlc.Feed{}
	seen := map[string]bool{}
	for _, cat := range snapshot.Categories {
		folders[cat.ParentID.String+"\x00"+cat.NormalizedName] = cat.ID
	}
	for _, f := range snapshot.Feeds {
		feeds[f.NormalizedUrl] = f
	}
	for i, item := range out.Items {
		if item.Status == "invalid" {
			continue
		}
		parent := keys[item.ParentKey]
		if item.Kind == "folder" {
			key := parent + "\x00" + strings.ToLower(item.Title)
			if id, ok := folders[key]; ok {
				out.Items[i].Status = "existing"
				keys[item.Key] = id
			} else {
				id := "new:" + item.Key
				folders[key] = id
				keys[item.Key] = id
				out.Items[i].Status = "new"
			}
		} else {
			if seen[item.URL] {
				out.Items[i].Status = "duplicate"
			} else if old, ok := feeds[item.URL]; ok {
				out.Items[i].Status = "existing"
				if old.ArchivedAt.Valid {
					out.Items[i].Status = "restore"
				}
			} else {
				out.Items[i].Status = "new"
			}
			seen[item.URL] = true
		}
	}
	return out, nil
}
func (s *Store) ImportOPML(ctx context.Context, plan ImportPlan, selected []string, applyExisting bool) (ImportReport, error) {
	out := ImportReport{FeedIDs: []string{}}
	if len(plan.Items) > 100000 || len(selected) > 100000 {
		return out, ErrInvalidInput
	}
	selectedSet := map[string]bool{}
	items := map[string]ImportOutline{}
	for _, item := range plan.Items {
		if _, ok := items[item.Key]; ok || item.Key == "" {
			return out, ErrInvalidInput
		}
		items[item.Key] = item
	}
	for _, key := range selected {
		item, ok := items[key]
		if !ok || item.Status == "invalid" {
			return out, ErrInvalidInput
		}
		selectedSet[key] = true
		parent := item.ParentKey
		for depth := 0; parent != ""; depth++ {
			if depth >= 5 {
				return out, ErrInvalidInput
			}
			ancestor, ok := items[parent]
			if !ok || ancestor.Kind != "folder" || ancestor.Status == "invalid" {
				return out, ErrInvalidInput
			}
			selectedSet[parent] = true
			parent = ancestor.ParentKey
		}
	}
	err := s.transaction(ctx, func(q *sqlc.Queries) error {
		snapshot, err := opmlSnapshot(ctx, q)
		if err != nil {
			return err
		}
		if plan.Revision == "" || importRevision(snapshot) != plan.Revision {
			return ErrConflict
		}
		folders := map[string]string{}
		folderKeys := map[string]string{}
		feeds := map[string]sqlc.Feed{}
		visited := map[string]bool{}
		changed := map[string]bool{}
		relations := map[string]bool{}
		for _, cat := range snapshot.Categories {
			folders[cat.ParentID.String+"\x00"+cat.NormalizedName] = cat.ID
		}
		for _, f := range snapshot.Feeds {
			feeds[f.NormalizedUrl] = f
			for _, categoryID := range snapshot.Relations[f.ID] {
				relations[f.ID+"\x00"+categoryID] = true
			}
		}
		now := time.Now().UTC().UnixMilli()
		for _, item := range plan.Items {
			if !selectedSet[item.Key] || item.Status == "invalid" {
				out.Skipped++
				continue
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			parent := folderKeys[item.ParentKey]
			if item.ParentKey != "" && parent == "" {
				return ErrInvalidInput
			}
			if item.Kind == "folder" {
				if strings.TrimSpace(item.Title) == "" || len(item.Title) > 256 {
					return ErrInvalidInput
				}
				key := parent + "\x00" + strings.ToLower(item.Title)
				id, ok := folders[key]
				if !ok {
					id = rand.Text()
					if err = q.CreateCategory(ctx, sqlc.CreateCategoryParams{ID: id, ParentID: nullString(parent), Name: item.Title, NormalizedName: strings.ToLower(item.Title), CreatedAt: now, UpdatedAt: now}); err != nil {
						return err
					}
					folders[key] = id
					out.CategoriesAdded++
				}
				folderKeys[item.Key] = id
				continue
			}
			if item.Kind != "feed" || len(item.Title) > 512 {
				return ErrInvalidInput
			}
			canonical, err := normalizeURL(item.URL)
			if err != nil {
				return err
			}
			if canonical != item.URL {
				return ErrInvalidInput
			}
			row, exists := feeds[canonical]
			if !exists {
				row = sqlc.Feed{ID: rand.Text(), Title: item.Title, Url: canonical, NormalizedUrl: canonical, SiteUrl: item.SiteURL}
				if err = q.CreateFeed(ctx, sqlc.CreateFeedParams{ID: row.ID, Title: row.Title, Url: canonical, NormalizedUrl: canonical, CreatedAt: now, UpdatedAt: now}); err != nil {
					return err
				}
				if err = q.SetImportedSite(ctx, sqlc.SetImportedSiteParams{SiteUrl: item.SiteURL, ID: row.ID}); err != nil {
					return err
				}
				feeds[canonical] = row
				out.FeedsAdded++
			} else if !visited[row.ID] {
				if row.ArchivedAt.Valid {
					if err = q.RestoreFeed(ctx, sqlc.RestoreFeedParams{ID: row.ID, UpdatedAt: now}); err != nil {
						return err
					}
					row.ArchivedAt = sql.NullInt64{}
					feeds[canonical] = row
					out.FeedsRestored++
				} else {
					out.FeedsReused++
				}
			}
			if !visited[row.ID] {
				visited[row.ID] = true
				out.FeedIDs = append(out.FeedIDs, row.ID)
			}
			if parent != "" && !relations[row.ID+"\x00"+parent] {
				if err = q.SetFeedCategory(ctx, sqlc.SetFeedCategoryParams{FeedID: row.ID, CategoryID: parent, CreatedAt: now}); err != nil {
					return err
				}
				relations[row.ID+"\x00"+parent] = true
				changed[row.ID] = true
				out.AssociationsAdded++
			}
		}
		for id := range changed {
			if err = q.TouchImportedFeed(ctx, sqlc.TouchImportedFeedParams{UpdatedAt: now, ID: id}); err != nil {
				return err
			}
			if applyExisting {
				if err = q.ApplyFeedCategories(ctx, sqlc.ApplyFeedCategoriesParams{FeedID: id, CreatedAt: now, UpdatedAt: now}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return ImportReport{}, err
	}
	return out, nil
}
