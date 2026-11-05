package transfer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

const fixtureXML = `<?xml version="1.0"?><opml version="2.0"><head><title>订阅</title></head><body><outline text="技术"><outline text="源" type="rss" xmlUrl="HTTPS://EXAMPLE.test:443/feed?a=1&amp;b=2#top" htmlUrl="https://example.test/"/><outline text="空分类"/></outline><outline text="其他"><outline text="重复" xmlUrl="https://example.test/feed?a=1&amp;b=2"/></outline><outline text="坏地址" type="rss" xmlUrl="file:///secret"/><outline text="缺地址" type="rss"/></body></opml>`

func testStore(t *testing.T) *database.Store {
	t.Helper()
	store, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "transfer.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}
func testService(t *testing.T, store *database.Store) (*Service, *API, string, string) {
	t.Helper()
	dir := t.TempDir()
	input := filepath.Join(dir, "input.opml")
	output := filepath.Join(dir, "output.opml")
	if err := os.WriteFile(input, []byte(fixtureXML), 0600); err != nil {
		t.Fatal(err)
	}
	service := NewService(store, func(context.Context) (string, error) { return input, nil }, func(context.Context) (string, error) { return output, nil }, nil, nil)
	t.Cleanup(service.Close)
	return service, NewAPI(service), input, output
}
func selected(preview PreviewResult) []string {
	keys := []string{}
	for _, row := range preview.Data.Items {
		if row.Status != "invalid" {
			keys = append(keys, row.Key)
		}
	}
	return keys
}
func previewFixture(t *testing.T, api *API) PreviewResult {
	t.Helper()
	token := api.ChooseImport(context.Background())
	if !token.OK {
		t.Fatal(token.Error)
	}
	preview := api.PreviewOPML(context.Background(), TokenRequest{Token: token.Data.Token})
	if !preview.OK {
		t.Fatal(preview.Error)
	}
	return preview
}
func TestPreviewConfirmAndRoundTrip(t *testing.T) {
	store := testStore(t)
	service, api, input, output := testService(t, store)
	ctx := context.Background()
	queued := 0
	service.queue = func(ids []string) error { queued += len(ids); return nil }
	token := api.ChooseImport(ctx)
	encoded, _ := json.Marshal(token)
	if !token.OK || strings.Contains(string(encoded), filepath.Dir(input)) {
		t.Fatal("文件路径泄漏到绑定", string(encoded))
	}
	if result := api.ExportOPML(ctx, TokenRequest{Token: token.Data.Token}); result.OK {
		t.Fatal("导入能力不能用于导出")
	}
	preview := api.PreviewOPML(ctx, TokenRequest{Token: token.Data.Token})
	if !preview.OK {
		t.Fatal(preview.Error)
	}
	if reused := api.PreviewOPML(ctx, TokenRequest{Token: token.Data.Token}); reused.OK {
		t.Fatal("文件授权可重复使用")
	}
	before, _ := store.Navigation(ctx)
	if len(before.Feeds) != 0 || len(before.Categories) != 0 || queued != 0 {
		t.Fatal("预览产生副作用")
	}
	result := api.ImportOPML(ctx, ImportRequest{PreviewID: preview.Data.ID, Selected: selected(preview), ApplyExisting: true, Refresh: true})
	if !result.OK {
		t.Fatal(result.Error)
	}
	report := result.Data.Report
	if report.FeedsAdded != 1 || report.CategoriesAdded != 3 || report.AssociationsAdded != 2 || report.Skipped != 2 || !result.Data.RefreshQueued || queued != 1 {
		t.Fatal(report, queued)
	}
	if again := api.ImportOPML(ctx, ImportRequest{PreviewID: preview.Data.ID, Selected: selected(preview)}); again.OK {
		t.Fatal("预览不能提交两次")
	}
	nav, _ := store.Navigation(ctx)
	if len(nav.Feeds) != 1 || len(nav.Categories) != 3 {
		t.Fatal(nav)
	}
	source, _ := store.GetSubscription(ctx, nav.Feeds[0].ID)
	if source.Title != "源" || source.URL != "https://example.test/feed?a=1&b=2" || len(source.CategoryIDs) != 2 {
		t.Fatal(source)
	}
	grant := api.ChooseExport(ctx)
	if !grant.OK {
		t.Fatal(grant.Error)
	}
	exported := api.ExportOPML(ctx, TokenRequest{Token: grant.Data.Token})
	if !exported.OK {
		t.Fatal(exported.Error)
	}
	bytes, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := parseOPML(ctx, bytes)
	if err != nil {
		t.Fatal("导出不能再导入", err)
	}
	restored := testStore(t)
	plan, err := restored.PlanOPML(ctx, rows)
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	for _, row := range plan.Items {
		keys = append(keys, row.Key)
	}
	round, err := restored.ImportOPML(ctx, plan, keys, true)
	if err != nil || round.FeedsAdded != 1 || round.CategoriesAdded != 3 || round.AssociationsAdded != 2 {
		t.Fatal(round, err)
	}
	if exported.Data.Bytes != len(bytes) || exported.Data.Name != "output.opml" {
		t.Fatal(exported)
	}
}
func TestCancellationExpiryFileChangeAndAtomicFailure(t *testing.T) {
	store := testStore(t)
	service, api, input, output := testService(t, store)
	ctx := context.Background()
	service.pickImport = func(context.Context) (string, error) { return "", nil }
	cancelled := api.ChooseImport(ctx)
	if !cancelled.OK || !cancelled.Data.Cancelled {
		t.Fatal(cancelled)
	}
	service.pickImport = func(context.Context) (string, error) { return input, nil }
	preview := previewFixture(t, api)
	api.Discard(ctx, DiscardRequest{PreviewID: preview.Data.ID})
	if result := api.ImportOPML(ctx, ImportRequest{PreviewID: preview.Data.ID, Selected: selected(preview)}); result.OK {
		t.Fatal("取消预览仍可导入")
	}
	nav, _ := store.Navigation(ctx)
	if len(nav.Feeds) != 0 {
		t.Fatal("取消写入了库")
	}
	token := api.ChooseImport(ctx)
	if err := os.WriteFile(input, []byte(fixtureXML+"changed"), 0600); err != nil {
		t.Fatal(err)
	}
	changed := api.PreviewOPML(ctx, TokenRequest{Token: token.Data.Token})
	if changed.OK || changed.Error.Code != "Conflict" {
		t.Fatal(changed)
	}
	if err := os.WriteFile(input, []byte(fixtureXML), 0600); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	service.now = func() time.Time { return now }
	token = api.ChooseImport(ctx)
	now = now.Add(11 * time.Minute)
	if result := api.PreviewOPML(ctx, TokenRequest{Token: token.Data.Token}); result.OK || result.Error.Code != "PermissionError" {
		t.Fatal(result)
	}
	if err := os.WriteFile(output, []byte("原文件"), 0600); err != nil {
		t.Fatal(err)
	}
	grant := api.ChooseExport(ctx)
	service.beforeCommit = func() error { return errors.New("模拟写入完成后的提交失败") }
	failed := api.ExportOPML(ctx, TokenRequest{Token: grant.Data.Token})
	if failed.OK {
		t.Fatal(failed)
	}
	kept, _ := os.ReadFile(output)
	if string(kept) != "原文件" {
		t.Fatal("导出失败损坏已有文件")
	}
	temporary, _ := filepath.Glob(filepath.Join(filepath.Dir(output), ".fealdhord-opml-*"))
	if len(temporary) != 0 {
		t.Fatal("临时文件未清理", temporary)
	}
	service.beforeCommit = nil
	grant = api.ChooseExport(ctx)
	if err := os.WriteFile(output, []byte("用户改动"), 0600); err != nil {
		t.Fatal(err)
	}
	failed = api.ExportOPML(ctx, TokenRequest{Token: grant.Data.Token})
	if failed.OK || failed.Error.Code != "Conflict" {
		t.Fatal("覆盖了授权后变化的文件", failed)
	}
	kept, _ = os.ReadFile(output)
	if string(kept) != "用户改动" {
		t.Fatal(string(kept))
	}
}
func TestImportConflictsAndWholeTransactionRollback(t *testing.T) {
	store := testStore(t)
	_, api, _, _ := testService(t, store)
	ctx := context.Background()
	preview := previewFixture(t, api)
	if _, err := store.CreateCategory(ctx, "新增分类", ""); err != nil {
		t.Fatal(err)
	}
	failed := api.ImportOPML(ctx, ImportRequest{PreviewID: preview.Data.ID, Selected: selected(preview)})
	if failed.OK || failed.Error.Code != "Conflict" {
		t.Fatal(failed)
	}
	nav, _ := store.Navigation(ctx)
	if len(nav.Feeds) != 0 || len(nav.Categories) != 1 {
		t.Fatal("过期预览写入了部分内容", nav)
	}
	rows := []database.ImportOutline{{Key: "folder", Title: "不能留下的分类", Kind: "folder", Status: "new"}, {Key: "feed", ParentKey: "folder", Title: "坏源", URL: "invalid://target", Kind: "feed", Status: "new"}}
	plan, err := store.PlanOPML(ctx, rows)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.ImportOPML(ctx, plan, []string{"folder", "feed"}, true); err == nil {
		t.Fatal("无效批次应失败")
	}
	nav, _ = store.Navigation(ctx)
	if len(nav.Categories) != 1 || len(nav.Feeds) != 0 {
		t.Fatal("事务未完整回滚", nav)
	}
}
func TestOPMLSecurityLimitsAndMetadataBoundary(t *testing.T) {
	ctx := context.Background()
	for _, raw := range []string{`<!DOCTYPE opml [<!ENTITY xxe SYSTEM "file:///secret">]><opml version="2.0"><body><outline text="&xxe;"/></body></opml>`, `<opml><body>` + strings.Repeat(`<outline text="深">`, 17) + strings.Repeat(`</outline>`, 17) + `</body></opml>`, strings.Repeat("x", maxFileBytes+1), `<opml><body><outline text="x" text="y"/></body></opml>`} {
		if _, err := parseOPML(ctx, []byte(raw)); !errors.Is(err, ErrDocument) {
			t.Fatal("危险或超限输入未拒绝", err)
		}
	}
	var source strings.Builder
	source.WriteString(`<opml><body>`)
	for i := range 10001 {
		fmt.Fprintf(&source, `<outline text="源" xmlUrl="https://example.test/%d"/>`, i)
	}
	source.WriteString(`</body></opml>`)
	if _, err := parseOPML(ctx, []byte(source.String())); !errors.Is(err, ErrDocument) {
		t.Fatal("超过10000个源未拒绝", err)
	}
	snapshot := database.OPMLSnapshot{Categories: []sqlc.Category{{ID: "cat", Name: "分类", AiPrompt: "secret-prompt"}}, Feeds: []sqlc.Feed{{ID: "feed", Title: "源", NormalizedUrl: "https://example.test/feed", Description: "private-internal", Etag: "secret-validator"}}, Relations: map[string][]string{}}
	data, err := encodeOPML(ctx, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"secret-prompt", "private-internal", "secret-validator"} {
		if strings.Contains(string(data), secret) {
			t.Fatal("导出了内部配置", secret)
		}
	}
}

type waitingRepository struct {
	Repository
	started chan struct{}
}

func (r waitingRepository) PlanOPML(ctx context.Context, _ []database.ImportOutline) (database.ImportPlan, error) {
	close(r.started)
	<-ctx.Done()
	return database.ImportPlan{}, ctx.Err()
}
func TestDiscardCancelsInvisiblePreviewAndPendingExport(t *testing.T) {
	store := testStore(t)
	service, api, _, output := testService(t, store)
	started := make(chan struct{})
	service.repository = waitingRepository{store, started}
	token := api.ChooseImport(context.Background())
	done := make(chan PreviewResult, 1)
	go func() { done <- api.PreviewOPML(context.Background(), TokenRequest{Token: token.Data.Token}) }()
	<-started
	api.Discard(context.Background(), DiscardRequest{Token: token.Data.Token})
	select {
	case result := <-done:
		if result.OK || result.Error.Code != "Cancelled" {
			t.Fatalf("cancel: %+v", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("preview did not stop")
	}
	service.mu.Lock()
	remaining := len(service.previews) + len(service.processing)
	service.mu.Unlock()
	if remaining != 0 {
		t.Fatal("cancel leaked private preview")
	}
	service.repository = store
	ctx, cancel := context.WithCancel(context.Background())
	service.beforeCommit = func() error { cancel(); return nil }
	grant := api.ChooseExport(ctx)
	result := api.ExportOPML(ctx, TokenRequest{Token: grant.Data.Token})
	if result.OK || result.Error.Code != "Cancelled" {
		t.Fatalf("export: %+v", result)
	}
	if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("cancel created target")
	}
}
func TestExporterRejectsOrphanHierarchy(t *testing.T) {
	_, err := encodeOPML(context.Background(), database.OPMLSnapshot{Categories: []sqlc.Category{{ID: "orphan", ParentID: sql.NullString{String: "missing", Valid: true}, Name: "孤立"}}})
	if !errors.Is(err, ErrDocument) {
		t.Fatal("orphan silently omitted", err)
	}
}

func TestReuseRestoreAndFutureOnlyImport(t *testing.T) {
	ctx := context.Background()
	store := testStore(t)
	_, api, _, _ := testService(t, store)
	id, err := store.AddSubscription(ctx, database.SubscriptionInput{URL: "https://example.test/feed?a=1&b=2", Title: "我的自定义源"}, nil, database.FeedCheck{At: 1, Next: 2})
	if err != nil {
		t.Fatal(err)
	}
	article, err := store.UpsertArticle(ctx, database.ArticleInput{FeedID: id, GUID: "old", Title: "已有", Text: "已有文章", FetchedAt: 3})
	if err != nil {
		t.Fatal(err)
	}
	source, _ := store.GetSubscription(ctx, id)
	if err = store.ArchiveFeed(ctx, id, source.Revision); err != nil {
		t.Fatal(err)
	}
	preview := previewFixture(t, api)
	result := api.ImportOPML(ctx, ImportRequest{PreviewID: preview.Data.ID, Selected: selected(preview), ApplyExisting: false, Refresh: true})
	if !result.OK || result.Data.Report.FeedsRestored != 1 || result.Data.Report.FeedsAdded != 0 || result.Data.RefreshQueued || result.Data.RefreshError == "" {
		t.Fatalf("restore: %+v", result)
	}
	source, _ = store.GetSubscription(ctx, id)
	if source.Title != "我的自定义源" || source.Archived || len(source.CategoryIDs) != 2 {
		t.Fatal(source)
	}
	memberships, err := store.ArticleMemberships(ctx, article)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range memberships {
		if m.Effective {
			t.Fatal("future-only assigned old article", m)
		}
	}
	fresh, err := store.UpsertArticle(ctx, database.ArticleInput{FeedID: id, GUID: "new", Title: "新文", Text: "新文", FetchedAt: 4})
	if err != nil {
		t.Fatal(err)
	}
	memberships, err = store.ArticleMemberships(ctx, fresh)
	if err != nil {
		t.Fatal(err)
	}
	effective := 0
	for _, m := range memberships {
		if m.Effective {
			effective++
		}
	}
	if effective != 2 {
		t.Fatal("new article lost categories", memberships)
	}
	preview = previewFixture(t, api)
	result = api.ImportOPML(ctx, ImportRequest{PreviewID: preview.Data.ID, Selected: selected(preview), ApplyExisting: true})
	if !result.OK || result.Data.Report.FeedsReused != 1 || result.Data.Report.AssociationsAdded != 0 {
		t.Fatalf("idempotence: %+v", result)
	}
}
