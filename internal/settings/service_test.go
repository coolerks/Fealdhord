package settings

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/coolerks/Fealdhord/internal/database"
)

func openTestStore(t *testing.T, filename string) *database.Store {
	t.Helper()
	store, err := database.Open(context.Background(), filename)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	return store
}

func TestPreferencesPersistAndReopen(t *testing.T) {
	ctx := context.Background()
	filename := filepath.Join(t.TempDir(), "偏好目录", "settings.db")
	store, err := database.Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(store)
	got := service.Get(ctx)
	if !got.OK || got.Data.Revision != 0 || got.Data.Preferences.Theme != "system" || got.Data.Preferences.RefreshToastAutoDismiss {
		t.Fatalf("初始偏好错误：%+v", got)
	}
	prefs := got.Data.Preferences
	prefs.Theme = "dark"
	prefs.RefreshToastAutoDismiss = true
	prefs.ArticleViewMode = "masonry"
	prefs.SidebarWidth = 340
	prefs.ScrollAnchors["all:masonry"] = ScrollAnchor{ArticleID: "article-7", Offset: 18}
	saved := service.Update(ctx, UpdateRequest{Preferences: prefs, Revision: 0})
	if !saved.OK || saved.Data.Revision != 1 {
		t.Fatalf("保存失败：%+v", saved)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestStore(t, filename)
	snapshot := NewService(reopened).Get(ctx)
	if !snapshot.OK || snapshot.Data.Preferences.Theme != "dark" || !snapshot.Data.Preferences.RefreshToastAutoDismiss || snapshot.Data.Preferences.SidebarWidth != 340 || snapshot.Data.Preferences.ScrollAnchors["all:masonry"].ArticleID != "article-7" {
		t.Fatalf("重开丢失设置：%+v", snapshot)
	}
}

func TestConcurrentUpdatesDoNotOverwrite(t *testing.T) {
	ctx := context.Background()
	s := NewService(openTestStore(t, filepath.Join(t.TempDir(), "prefs.db")))
	var wg sync.WaitGroup
	results := make(chan Result, 8)
	for range 8 {
		wg.Go(func() { results <- s.Update(ctx, UpdateRequest{Preferences: Defaults(), Revision: 0}) })
	}
	wg.Wait()
	close(results)
	success := 0
	for result := range results {
		if result.OK {
			success++
		} else if result.Error.Code != "Conflict" {
			t.Fatalf("非预期错误：%+v", result)
		}
	}
	if success != 1 {
		t.Fatalf("同 revision 应只成功一次：%d", success)
	}
}

func TestInvalidInputsAndUnavailableStoreDoNotWrite(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, filepath.Join(t.TempDir(), "prefs.db"))
	s := NewService(store)
	prefs := Defaults()
	prefs.Theme = "unknown"
	if got := s.Update(ctx, UpdateRequest{Preferences: prefs}); got.OK || got.Error.Code != "ValidationError" {
		t.Fatal("非法选项未拒绝")
	}
	prefs = Defaults()
	prefs.SidebarWidth = 1000
	if got := s.Update(ctx, UpdateRequest{Preferences: prefs}); got.OK {
		t.Fatal("超宽布局未拒绝")
	}
	prefs = Defaults()
	prefs.ScrollAnchors["all:list"] = ScrollAnchor{Offset: 1e12}
	if got := s.Update(ctx, UpdateRequest{Preferences: prefs}); got.OK {
		t.Fatal("异常锚点未拒绝")
	}
	if got := s.Get(ctx); got.Data.Revision != 0 {
		t.Fatal("非法更新产生写入")
	}
	if got := NewService(nil).Get(ctx); got.OK || got.Data != nil || got.Error.Code != "DatabaseError" {
		t.Fatal("仓储失败应返回安全错误")
	}
}

func TestLegacyValuesNormalizeIndividually(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, filepath.Join(t.TempDir(), "prefs.db"))
	raw := `{"theme":"dark","article_view_mode":"old-grid","sidebar_width":9999,"reader_font_size":19,"reader_full_mode":"web","reader_pinned":true}`
	if ok, err := store.SavePreferences(ctx, raw, 0, 0); err != nil || !ok {
		t.Fatal(err)
	}
	got := NewService(store).Get(ctx)
	if !got.OK || got.Data.Preferences.Theme != "dark" || got.Data.Preferences.ArticleViewMode != "list" || got.Data.Preferences.SidebarWidth != 248 || got.Data.Preferences.ReaderFontSize != 19 {
		t.Fatalf("旧值恢复错误：%+v", got)
	}
	if got.Data.Preferences.ReaderOpenMode != "feed" {
		t.Fatal("旧偏好应继续打开订阅预览")
	}
	if got.Data.Preferences.RefreshToastAutoDismiss {
		t.Fatal("旧设置的订阅刷新通知默认保留")
	}

	prefs := got.Data.Preferences
	if prefs.ReaderLayout != "split" {
		t.Fatal("旧版并排偏好应保留")
	}
	prefs.ReaderLayout = "drawer"
	prefs.ReaderOpenMode = "full"
	service := NewService(store)
	saved := service.Update(ctx, UpdateRequest{Preferences: prefs, Revision: got.Data.Revision})
	if !saved.OK {
		t.Fatalf("保存阅读方式失败：%+v", saved.Error)
	}
	loaded := service.Get(ctx)
	if !loaded.OK || loaded.Data.Preferences.ReaderOpenMode != "full" || loaded.Data.Preferences.ReaderLayout != "drawer" || loaded.Data.Preferences.ReaderPinned {
		t.Fatal("阅读方式没有持久化")
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(b, &envelope); err != nil {
		t.Fatal(err)
	}
	if _, exists := envelope["error"]; exists {
		t.Fatal("成功结果不应带 error")
	}
}

func TestReaderToolbarPreferences(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, filepath.Join(t.TempDir(), "prefs.db"))
	if ok, err := store.SavePreferences(ctx, `{"theme":"dark"}`, 0, 0); err != nil || !ok {
		t.Fatalf("写入旧设置失败：%t %v", ok, err)
	}
	service := NewService(store)
	got := service.Get(ctx)
	if !got.OK || !slices.Equal(got.Data.Preferences.ReaderToolbarPinned, []string{"refresh", "favorite", "translate", "ai"}) {
		t.Fatalf("旧设置未使用默认常驻操作：%+v", got)
	}
	prefs := got.Data.Preferences
	for _, invalid := range [][]string{{"unknown"}, {"favorite", "favorite"}} {
		prefs.ReaderToolbarPinned = invalid
		if result := service.Update(ctx, UpdateRequest{Preferences: prefs, Revision: got.Data.Revision}); result.OK || result.Error.Code != "ValidationError" {
			t.Fatalf("无效操作未拒绝：%+v", result)
		}
	}
	prefs.ReaderToolbarPinned = []string{}
	saved := service.Update(ctx, UpdateRequest{Preferences: prefs, Revision: got.Data.Revision})
	if !saved.OK {
		t.Fatalf("允许将全部操作移入更多：%+v", saved.Error)
	}
	loaded := service.Get(ctx)
	if !loaded.OK || loaded.Data.Preferences.ReaderToolbarPinned == nil || len(loaded.Data.Preferences.ReaderToolbarPinned) != 0 {
		t.Fatalf("读取时未保留空数组：%+v", loaded)
	}
}

func TestDeveloperModeUpdatesMenuOnlyAfterCommit(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, filepath.Join(t.TempDir(), "prefs.db"))
	type change struct {
		enabled  bool
		revision int64
	}
	var changes []change
	service := NewService(store, Hooks{DeveloperModeChanged: func(enabled bool, revision int64) {
		changes = append(changes, change{enabled, revision})
	}})
	initial := service.Get(ctx)
	if !initial.OK || initial.Data.Preferences.DeveloperMode {
		t.Fatal("开发者模式默认关闭")
	}
	prefs := initial.Data.Preferences
	prefs.DeveloperMode = true
	saved := service.Update(ctx, UpdateRequest{Preferences: prefs, Revision: initial.Data.Revision})
	if !saved.OK || !service.Get(ctx).Data.Preferences.DeveloperMode {
		t.Fatalf("开发者模式未保存：%+v", saved)
	}
	prefs.DeveloperMode = false
	conflict := service.Update(ctx, UpdateRequest{Preferences: prefs, Revision: initial.Data.Revision})
	if conflict.OK {
		t.Fatal("旧版本不应覆盖设置")
	}
	disabled := service.Update(ctx, UpdateRequest{Preferences: prefs, Revision: saved.Data.Revision})
	if !disabled.OK || !slices.Equal(changes, []change{{true, saved.Data.Revision}, {false, disabled.Data.Revision}}) {
		t.Fatalf("菜单应跟随实际保存的状态：%+v", changes)
	}
}

func TestFutureSchemaCannotBeReadOrOverwritten(t *testing.T) {
	ctx := context.Background()
	filename := filepath.Join(t.TempDir(), "future.db")
	store := openTestStore(t, filename)
	db, err := sql.Open("sqlite", filename)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, "UPDATE app_settings SET schema_version=2 WHERE key='ui_preferences'"); err != nil {
		t.Fatal(err)
	}
	s := NewService(store)
	for _, got := range []Result{s.Get(ctx), s.Update(ctx, UpdateRequest{Preferences: Defaults()})} {
		if got.OK || got.Error.Code != "ConfigurationError" {
			t.Fatalf("未来版本未保护：%+v", got)
		}
	}
	record, err := store.LoadPreferences(ctx)
	if err != nil || record.Revision != 0 || record.SchemaVersion != 2 {
		t.Fatalf("未来版本被改写：%+v %v", record, err)
	}
}

func TestLegacyMediaDefaultsAndScopedNotification(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, filepath.Join(t.TempDir(), "prefs.db"))
	if _, err := store.SavePreferences(ctx, `{"theme":"system"}`, 0, 100); err != nil {
		t.Fatal(err)
	}
	calls := 0
	service := NewService(store, Hooks{MediaChanged: func(context.Context, Preferences) { calls++ }})
	got := service.Get(ctx)
	if !got.OK || got.Data.Preferences.RemoteImages != "subscriptions" || !got.Data.Preferences.MediaCacheEnabled || got.Data.Preferences.WebImageFallback {
		t.Fatal("旧设置媒体默认值错误")
	}
	prefs := got.Data.Preferences
	prefs.Theme = "dark"
	saved := service.Update(ctx, UpdateRequest{Preferences: prefs, Revision: got.Data.Revision})
	if !saved.OK || calls != 0 {
		t.Fatal("界面偏好触发了全媒体重置")
	}
	prefs.RemoteImages = "never"
	saved = service.Update(ctx, UpdateRequest{Preferences: prefs, Revision: saved.Data.Revision})
	if !saved.OK || calls != 1 {
		t.Fatal("媒体策略未通知")
	}
}
