package appagent

import (
	"context"
	"net/url"
	"runtime/debug"
	"slices"

	"github.com/coolerks/Fealdhord/internal/articleai"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/feed"
	"github.com/coolerks/Fealdhord/internal/library"
	"github.com/coolerks/Fealdhord/internal/navigation"
	"github.com/coolerks/Fealdhord/internal/notification"
	"github.com/coolerks/Fealdhord/internal/reader"
	"github.com/coolerks/Fealdhord/internal/refresh"
	"github.com/coolerks/Fealdhord/internal/settings"
	"github.com/coolerks/Fealdhord/internal/transfer"
)

type Services struct {
	Store         *database.Store
	Transfer      *transfer.API
	Library       *library.Service
	Feed          *feed.Service
	Navigation    *navigation.Service
	Reader        *reader.Service
	Refresh       *refresh.Service
	Settings      *settings.Service
	ArticleAI     *articleai.API
	Notifications *notification.Service
}

type empty struct{}
type id struct {
	ID string `json:"id" jsonschema:"required"`
}
type ids struct {
	IDs []string `json:"ids" jsonschema:"required"`
}

func Builtins(s Services) *Registry {
	r := NewRegistry()
	if s.Notifications != nil {
		registerNotificationTools(r, s.Notifications)
	registerAskUserQuestionTool(r)
	}
	if s.Store != nil {
		r.scopeDB = s.Store.ChatQueries()
	}
	query := func(ctx context.Context, input library.QueryRequest) library.ListResult {
		if input.Scope.Kind == "" {
			input.Scope.Kind = "all"
		}
		if input.Sort == "" {
			input.Sort = "relevance"
			if input.Search == "" {
				input.Sort = "newest"
			}
		}
		if input.Limit == 0 {
			input.Limit = 12
		}
		input.Limit = min(input.Limit, 50)
		result := s.Library.Query(ctx, input)
		if result.Data != nil {
			for i := range result.Data.Items {
				result.Data.Items[i].Cover = nil
				result.Data.Items[i].URL = cleanToolURL(result.Data.Items[i].URL)
			}
		}
		return result
	}
	Register(r, "article.list", "按范围、状态或订阅列出文章摘要", false, query)
	Register(r, "article.search", "按关键词搜索本地文章。查询使用 search 字段，返回稳定文章 ID 和摘要", false, query)
	Register(r, "article.get", "读取指定文章的纯文本正文，需用户确认", true, func(ctx context.Context, input id) any {
		return s.Library.Get(ctx, library.IDRequest{ArticleID: input.ID})
	})
	Register(r, "article.extract", "重新提取已保存文章的原文，需确认", true, func(ctx context.Context, input id) any {
		result := s.Reader.Extract(ctx, reader.Request{ArticleID: input.ID, Mode: "reader"})
		if result.Data != nil {
			result.Data.HTML = ""
		}
		return result
	})
	for _, pair := range []struct {
		name, description string
		value             bool
	}{{"mark_read", "将指定文章批量标为已读", true}, {"mark_unread", "将指定文章批量标为未读", false}} {
		Register(r, "article."+pair.name, pair.description, true, func(ctx context.Context, input ids) any {
			if len(input.IDs) == 0 || len(input.IDs) > 100 {
				return "ValidationError"
			}
			results := make([]library.StateResult, 0, len(input.IDs))
			for _, id := range input.IDs {
				if ctx.Err() != nil {
					break
				}
				results = append(results, s.Library.SetRead(ctx, library.StateRequest{ArticleID: id, Value: pair.value}))
			}
			return results
		})
	}
	for _, pair := range []struct {
		name, description string
		value             bool
	}{{"favorite", "将指定文章加入收藏", true}, {"unfavorite", "将指定文章从收藏移除", false}} {
		Register(r, "article."+pair.name, pair.description, true, func(ctx context.Context, input id) library.StateResult {
			return s.Library.SetFavorite(ctx, library.StateRequest{ArticleID: input.ID, Value: pair.value})
		})
	}
	for _, pair := range []struct{ name, state, description string }{
		{"add_category", "included", "将指定文章加入分类"},
		{"remove_category", "excluded", "将指定文章从分类移除"},
	} {
		Register(r, "article."+pair.name, pair.description, true, func(ctx context.Context, input navigation.MembershipRequest) navigation.Result {
			input.State = pair.state
			return s.Navigation.SetMembership(ctx, input)
		})
	}
	Register(r, "feed.list", "列出已保存订阅", false, func(ctx context.Context, input feed.ListRequest) feed.ListResult {
		result := s.Feed.List(ctx, input)
		if result.Data != nil {
			for i := range result.Data.Items {
				redactFeed(&result.Data.Items[i])
			}
		}
		return result
	})
	Register(r, "feed.get", "读取订阅详情", false, func(ctx context.Context, input feed.IDRequest) feed.Result {
		result := s.Feed.Get(ctx, input)
		if result.Data != nil {
			redactFeed(result.Data)
		}
		return result
	})
	Register(r, "feed.discover", "从指定公共网页发现订阅", true, s.Feed.Discover)
	Register(r, "feed.add", "添加订阅", true, s.Feed.Add)
	Register(r, "feed.update", "按版本更新订阅", true, s.Feed.Update)
	Register(r, "feed.remove", "归档订阅并保留历史文章", true, s.Feed.Remove)
	Register(r, "feed.restore", "恢复已归档订阅", true, s.Feed.Restore)
	Register(r, "feed.refresh", "刷新指定订阅", true, func(ctx context.Context, input id) any {
		if input.ID == "" {
			return "ValidationError"
		}
		return s.Refresh.Start(ctx, refresh.StartRequest{FeedIDs: []string{input.ID}})
	})
	Register(r, "feed.refresh_all", "刷新全部订阅", true, func(ctx context.Context, _ empty) any { return s.Refresh.Start(ctx, refresh.StartRequest{}) })
	Register(r, "feed.refresh_status", "查询刷新任务进度", false, s.Refresh.Get)
	Register(r, "feed.cancel_refresh", "取消刷新任务", true, s.Refresh.Cancel)
	Register(r, "category.list", "列出分类及阅读计数", false, func(ctx context.Context, _ empty) any {
		result := s.Navigation.List(ctx)
		if result.OK {
			return result.Data.Categories
		}
		return result.Error
	})
	Register(r, "category.get", "查找指定分类详情", false, func(ctx context.Context, input id) any {
		result := s.Navigation.List(ctx)
		if !result.OK {
			return result.Error
		}
		i := slices.IndexFunc(result.Data.Categories, func(value database.CategoryInfo) bool { return value.ID == input.ID })
		if i < 0 {
			return "NotFound"
		}
		return result.Data.Categories[i]
	})
	Register(r, "category.create", "创建分类", true, func(ctx context.Context, input database.CategoryInput) navigation.Result {
		input.ID = ""
		return s.Navigation.Save(ctx, input)
	})
	Register(r, "category.update", "按版本更新分类", true, func(ctx context.Context, input database.CategoryInput) any {
		if input.ID == "" {
			return "ValidationError"
		}
		return s.Navigation.Save(ctx, input)
	})
	Register(r, "category.preview_remove", "预览移除分类的影响", false, s.Navigation.PreviewDelete)
	Register(r, "category.remove", "按预览版本移除分类，保留文章", true, s.Navigation.Remove)
	Register(r, "category.article_memberships", "查看文章分类关系", false, s.Navigation.Memberships)
	Register(r, "view.list", "列出应用内置的文章视图", false, func(_ context.Context, _ empty) []string {
		return []string{"all", "unread", "favorites", "featured"}
	})
	Register(r, "view.get_counts", "获取文章和已读状态计数", false, func(ctx context.Context, _ empty) any {
		result := s.Navigation.List(ctx)
		if result.OK {
			return result.Data.Totals
		}
		return result.Error
	})
	Register(r, "app.get_settings", "查看公开的阅读偏好", false, func(ctx context.Context, _ empty) any {
		result := s.Settings.Get(ctx)
		if !result.OK {
			return result.Error
		}
		prefs := result.Data.Preferences
		return map[string]any{"theme": prefs.Theme, "reader_font_size": prefs.ReaderFontSize, "reader_layout": prefs.ReaderLayout, "revision": result.Data.Revision}
	})
	for _, preview := range []bool{true, false} {
		name := "article_ai.evaluate"
		if preview {
			name = "article_ai.preview"
		}
		Register(r, name, "运行文章分类或精选评估，可能消耗模型额度", true, func(ctx context.Context, input articleai.Request) any {
			input.Preview = preview
			result, err := s.ArticleAI.Evaluate(ctx, input)
			if err != nil {
				return map[string]string{"error": err.Error()}
			}
			return result
		})
	}
	Register(r, "article.get_metadata", "按 ID 获取文章标题和摘要", false, func(ctx context.Context, input id) any {
		if s.Store == nil {
			return map[string]string{"error": "StorageError"}
		}
		item, err := s.Store.ChatQueries().ChatArticleContext(ctx, input.ID)
		if err != nil {
			return map[string]string{"error": "ArticleUnavailable"}
		}
		return map[string]string{"id": input.ID, "title": item.Title, "summary": item.Summary}
	})
	type categoryFeed struct {
		FeedID     string `json:"feed_id"`
		CategoryID string `json:"category_id"`
		Revision   int64  `json:"revision"`
	}
	for _, add := range []bool{true, false} {
		name := "category.remove_feed"
		if add {
			name = "category.add_feed"
		}
		Register(r, name, "更新订阅继承的分类关系", true, func(ctx context.Context, input categoryFeed) any {
			current := s.Feed.Get(ctx, feed.IDRequest{ID: input.FeedID})
			if !current.OK {
				return current.Error
			}
			value := current.Data
			if input.Revision != value.Revision || input.CategoryID == "" {
				return "Conflict"
			}
			categories := slices.Clone(value.CategoryIDs)
			exists := slices.Contains(categories, input.CategoryID)
			if add && !exists {
				categories = append(categories, input.CategoryID)
			}
			if !add {
				categories = slices.DeleteFunc(categories, func(value string) bool { return value == input.CategoryID })
			}
			if categories == nil {
				categories = []string{}
			}
			return s.Feed.Update(ctx, feed.UpdateRequest{ID: value.ID, Revision: value.Revision, Title: value.Title, URL: value.URL, CategoryIDs: categories})
		})
	}
	type readingUpdate struct {
		Revision int64   `json:"revision"`
		Theme    *string `json:"theme,omitempty"`
		FontSize *int    `json:"reader_font_size,omitempty"`
		Layout   *string `json:"reader_layout,omitempty"`
	}
	Register(r, "app.update_reading_settings", "按版本修改主题和阅读排版", true, func(ctx context.Context, input readingUpdate) any {
		result := s.Settings.Get(ctx)
		if !result.OK {
			return result.Error
		}
		prefs := result.Data.Preferences
		if input.Theme != nil {
			prefs.Theme = *input.Theme
		}
		if input.FontSize != nil {
			prefs.ReaderFontSize = *input.FontSize
		}
		if input.Layout != nil {
			prefs.ReaderLayout = *input.Layout
		}
		saved := s.Settings.Update(ctx, settings.UpdateRequest{Preferences: prefs, Revision: input.Revision})
		if !saved.OK {
			return saved.Error
		}
		return map[string]any{"revision": saved.Data.Revision}
	})
	Register(r, "app.get_refresh_settings", "查看订阅自动刷新间隔", false, func(ctx context.Context, _ empty) any {
		result := s.Settings.Get(ctx)
		if !result.OK {
			return result.Error
		}
		return map[string]any{"revision": result.Data.Revision, "minutes": result.Data.Preferences.RefreshIntervalMinutes}
	})
	type interval struct {
		Revision int64 `json:"revision"`
		Minutes  int   `json:"minutes"`
	}
	Register(r, "app.update_refresh_settings", "按版本修改订阅刷新间隔", true, func(ctx context.Context, input interval) any {
		result := s.Settings.Get(ctx)
		if !result.OK {
			return result.Error
		}
		prefs := result.Data.Preferences
		prefs.RefreshIntervalMinutes = input.Minutes
		saved := s.Settings.Update(ctx, settings.UpdateRequest{Preferences: prefs, Revision: input.Revision})
		if !saved.OK {
			return saved.Error
		}
		return map[string]any{"revision": saved.Data.Revision, "minutes": input.Minutes}
	})
	Register(r, "app.get_version", "读取应用版本", false, func(_ context.Context, _ empty) any {
		version := "开发版本"
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
		return map[string]string{"name": "Fealdhord", "version": version}
	})
	Register(r, "app.get_diagnostics", "查看脱敏的刷新状态", false, func(ctx context.Context, _ empty) any { return refresh.NewAPI(s.Refresh).Current(ctx) })
	Register(r, "opml.preview_import", "打开桌面文件选择器，预览 OPML；需本机用户完成选择", true, func(ctx context.Context, _ empty) any {
		chosen := s.Transfer.ChooseImport(ctx)
		if !chosen.OK || chosen.Data == nil || chosen.Data.Cancelled {
			return chosen
		}
		return s.Transfer.PreviewOPML(ctx, transfer.TokenRequest{Token: chosen.Data.Token})
	})
	Register(r, "opml.import", "导入已预览并由用户确认的 OPML 项", true, s.Transfer.ImportOPML)
	Register(r, "opml.export", "请用户选择保存位置后导出 OPML", true, func(ctx context.Context, _ empty) any {
		chosen := s.Transfer.ChooseExport(ctx)
		if !chosen.OK || chosen.Data == nil || chosen.Data.Cancelled {
			return chosen
		}
		return s.Transfer.ExportOPML(ctx, transfer.TokenRequest{Token: chosen.Data.Token})
	})
	return r
}

func cleanToolURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Hostname() == "" {
		return ""
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	return parsed.String()
}

func redactFeed(value *feed.Feed) {
	value.CustomIconURL = ""
	value.IconData = ""
	value.URL = cleanToolURL(value.URL)
	value.SiteURL = cleanToolURL(value.SiteURL)
}
