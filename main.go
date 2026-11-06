package main

import (
	"context"
	"embed"
	"fmt"
	"github.com/coolerks/Fealdhord/internal/feed"
	"github.com/coolerks/Fealdhord/internal/media"
	"github.com/coolerks/Fealdhord/internal/netpolicy"
	"github.com/coolerks/Fealdhord/internal/reader"
	"github.com/coolerks/Fealdhord/internal/refresh"
	"github.com/coolerks/Fealdhord/internal/transfer"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/coolerks/Fealdhord/internal/database"
	desktopintegration "github.com/coolerks/Fealdhord/internal/desktop"
	"github.com/coolerks/Fealdhord/internal/library"
	"github.com/coolerks/Fealdhord/internal/navigation"
	"github.com/coolerks/Fealdhord/internal/settings"

	fealdhord "github.com/coolerks/Fealdhord/internal/app"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

type LibraryChanged struct {
	Sequence uint64 `json:"sequence"`
}

func init() {
	application.RegisterEvent[LibraryChanged]("fealdhord:library-changed")
	application.RegisterEvent[refresh.Event](refresh.EventName)
	application.RegisterEvent[fealdhord.OperationEvent](fealdhord.EventName)
	application.RegisterEvent[desktopintegration.CommandEvent](desktopintegration.CommandEventName)
}

func main() {
	var desktop *application.App
	var mainWindow *application.WebviewWindow
	service, stop := fealdhord.NewBootstrapService(context.Background(), func(event fealdhord.OperationEvent) {
		desktop.Event.Emit(fealdhord.EventName, event)
	})
	var settingsRepository settings.Repository
	var libraryRepository library.Repository
	var navigationRepository navigation.Repository
	var readerRepository reader.Repository
	var transferRepository transfer.Repository
	var feedRepository feed.Repository
	var refreshRepository refresh.Repository
	var mediaRepository media.Repository
	dataDir, pathErr := os.UserConfigDir()
	if pathErr == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		store, err := database.Open(ctx, filepath.Join(dataDir, "Fealdhord", "fealdhord.db"))
		cancel()
		if err != nil {
			slog.Error("本地设置初始化失败", "error", err)
		} else {
			settingsRepository = store
			libraryRepository = store
			navigationRepository = store
			readerRepository = store
			transferRepository = store
			feedRepository = store
			refreshRepository = store
			mediaRepository = store
			defer func() {
				if err := store.Close(); err != nil {
					slog.Error("关闭设置数据库失败", "error", err)
				}
			}()
		}
	} else {
		slog.Error("无法定位用户数据目录", "error", pathErr)
	}
	var sequence atomic.Uint64
	changed := func() {
		if desktop != nil {
			desktop.Event.Emit("fealdhord:library-changed", LibraryChanged{Sequence: sequence.Add(1)})
		}
	}
	client := netpolicy.New()
	var mediaService *media.Service
	settingsService := settings.NewService(settingsRepository, func(ctx context.Context, prefs settings.Preferences) {
		if mediaService != nil {
			if err := mediaService.PolicyChanged(ctx, media.Policy{RemoteImages: prefs.RemoteImages, CacheEnabled: prefs.MediaCacheEnabled, WebFallback: prefs.WebImageFallback}); err != nil {
				slog.Warn("媒体偏好已保存，但缓存状态更新失败")
			}
		}
		changed()
	})
	policy := func(ctx context.Context) media.Policy {
		result := settingsService.Get(ctx)
		if !result.OK || result.Data == nil {
			return media.Policy{RemoteImages: "never"}
		}
		prefs := result.Data.Preferences
		return media.Policy{RemoteImages: prefs.RemoteImages, CacheEnabled: prefs.MediaCacheEnabled, WebFallback: prefs.WebImageFallback}
	}
	cacheDir := ""
	if directory, err := os.UserCacheDir(); err == nil {
		cacheDir = filepath.Join(directory, "Fealdhord", "media")
	}
	var mediaErr error
	mediaService, mediaErr = media.New(context.Background(), mediaRepository, client, cacheDir, policy, changed)
	if mediaErr != nil {
		slog.Warn("磁盘媒体缓存不可用，使用有界内存回退")
		mediaService, _ = media.New(context.Background(), mediaRepository, client, "", policy, changed)
	}
	feedService := feed.NewService(feedRepository, client, changed, func(ctx context.Context, id string, document *feed.Document) {
		if mediaService != nil {
			mediaService.Ingest(ctx, id, document, nil)
		}
	})
	refreshService := refresh.NewService(context.Background(), refreshRepository, client, func(event refresh.Event) {
		if desktop != nil {
			desktop.Event.Emit(refresh.EventName, event)
		}
	}, changed)
	refreshService.SetAfterCommit(func(ctx context.Context, id string, document *feed.Document, ids []string) {
		if mediaService != nil {
			mediaService.Ingest(ctx, id, document, ids)
		}
	})
	readerService := reader.NewService(readerRepository, client, func(ctx context.Context, article database.ReaderArticle) bool {
		value := policy(ctx)
		return value.RemoteImages == "always" || (value.RemoteImages == "subscriptions" && !article.Archived)
	}, func(target string) error {
		if desktop == nil {
			return fmt.Errorf("桌面应用尚未就绪")
		}
		return desktop.Browser.OpenURL(target)
	})
	transferService := transfer.NewService(transferRepository, func(ctx context.Context) (string, error) {
		if desktop == nil || mainWindow == nil || ctx.Err() != nil {
			return "", context.Canceled
		}
		return desktop.Dialog.OpenFile().SetTitle("导入 OPML").AddFilter("OPML 订阅文件", "*.opml;*.xml").AttachToWindow(mainWindow).PromptForSingleSelection()
	}, func(ctx context.Context) (string, error) {
		if desktop == nil || mainWindow == nil || ctx.Err() != nil {
			return "", context.Canceled
		}
		return desktop.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{Title: "导出 OPML", Filename: "Fealdhord.opml", Filters: []application.FileFilter{{DisplayName: "OPML 订阅文件", Pattern: "*.opml;*.xml"}}, Window: mainWindow}).PromptForSingleSelection()
	}, changed, func(ids []string) error {
		result := refreshService.Start(context.Background(), refresh.StartRequest{FeedIDs: ids})
		if !result.OK {
			return fmt.Errorf("刷新队列不可用")
		}
		return nil
	})
	desktop = application.New(application.Options{
		Name: "Fealdhord", Description: "折栈，本地优先的 RSS 阅读器",
		Services: []application.Service{
			application.NewService(service), application.NewService(settingsService),
			application.NewService(readerService),
			application.NewService(transfer.NewAPI(transferService)),
			application.NewService(navigation.NewService(navigationRepository, changed)),
			application.NewService(library.NewService(libraryRepository, func(ctx context.Context) bool { return policy(ctx).RemoteImages != "never" })),
			application.NewService(feedService), application.NewService(refresh.NewAPI(refreshService)),
			application.NewServiceWithOptions(media.NewAPI(mediaService), application.ServiceOptions{Route: "/media/"}),
		},
		Assets: application.AssetOptions{Handler: application.AssetFileServerFS(assets), Middleware: desktopintegration.GuardRuntime},
		Mac:    application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	desktop.OnShutdown(func() {
		transferService.Close()
		stop()
		refreshService.Close()
		if mediaService != nil {
			mediaService.Close()
		}
		client.CloseIdleConnections()
	})
	refreshService.StartScheduler()
	desktopintegration.InstallMenu(desktop)
	mainWindow = desktop.Window.NewWithOptions(desktopintegration.WindowOptions())
	if err := desktop.Run(); err != nil {
		stop()
		slog.Error("桌面应用启动失败", "error", err)
		os.Exit(1)
	}
}
