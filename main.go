package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/coolerks/Fealdhord/internal/aiconfig"
	fealdhord "github.com/coolerks/Fealdhord/internal/app"
	"github.com/coolerks/Fealdhord/internal/appagent"
	"github.com/coolerks/Fealdhord/internal/articleai"
	"github.com/coolerks/Fealdhord/internal/articlesummary"
	"github.com/coolerks/Fealdhord/internal/chat"
	"github.com/coolerks/Fealdhord/internal/database"
	desktopintegration "github.com/coolerks/Fealdhord/internal/desktop"
	"github.com/coolerks/Fealdhord/internal/feed"
	"github.com/coolerks/Fealdhord/internal/library"
	"github.com/coolerks/Fealdhord/internal/media"
	"github.com/coolerks/Fealdhord/internal/navigation"
	"github.com/coolerks/Fealdhord/internal/netpolicy"
	"github.com/coolerks/Fealdhord/internal/notification"
	"github.com/coolerks/Fealdhord/internal/reader"
	"github.com/coolerks/Fealdhord/internal/refresh"
	"github.com/coolerks/Fealdhord/internal/settings"
	"github.com/coolerks/Fealdhord/internal/transfer"
	"github.com/coolerks/Fealdhord/internal/translation"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
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
	application.RegisterEvent[chat.UpdateEvent](chat.UpdateEventName)
	application.RegisterEvent[notification.LocalNotice](notification.EventName)
}

func main() {
	var reopenWindow atomic.Pointer[application.WebviewWindow]
	options := application.Options{
		Name: "Fealdhord", Description: "折栈，本地优先的 RSS 阅读器",
		Icon: desktopIcon(),
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets), Middleware: desktopintegration.GuardRuntime,
		},
		Windows: application.WindowsOptions{DisableQuitOnLastWindowClosed: true},
		Linux:   application.LinuxOptions{DisableQuitOnLastWindowClosed: true},
	}
	if runtime.GOOS == "linux" {
		const appID = "io.fealdhord.desktop"
		options.Linux.ApplicationID = appID
		options.SingleInstance = &application.SingleInstanceOptions{
			UniqueID: appID,
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if window := reopenWindow.Load(); window != nil {
					desktopintegration.RestoreMainWindow(window)
				}
			},
		}
	}
	// Linux 启动器的重复启动在打开数据库前交给现有进程。
	desktop := application.New(options)
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
	var sequence atomic.Uint64
	changed := func() {
		desktop.Event.Emit("fealdhord:library-changed", LibraryChanged{Sequence: sequence.Add(1)})
	}
	var articleAIService *articleai.Service
	var aiConfig *aiconfig.Service
	var chatService *chat.Service
	var toolStore *database.Store
	dataDir, pathErr := os.UserConfigDir()
	if pathErr == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		store, err := database.Open(ctx, filepath.Join(dataDir, "Fealdhord", "fealdhord.db"))
		cancel()
		if err != nil {
			slog.Error("本地设置初始化失败", "error", err)
		} else {
			toolStore = store
			settingsRepository = store
			libraryRepository = store
			navigationRepository = store
			readerRepository = store
			transferRepository = store
			feedRepository = store
			refreshRepository = store
			mediaRepository = store
			aiConfig = aiconfig.New(store.AIProviderQueries(), aiconfig.SystemCredentials{})
			catalog := ai.NewCatalogIndex(filepath.Join(dataDir, "Fealdhord", "model-catalog"))
			aiConfig.SetCatalog(catalog)
			go catalog.Load(context.Background())
			defer func() {
				if err := store.Close(); err != nil {
					slog.Error("关闭设置数据库失败", "error", err)
				}
			}()
			chatService, err = chat.New(context.Background(), store, aiConfig, func(event chat.UpdateEvent) {
				desktop.Event.Emit(chat.UpdateEventName, event)
			})
			if err != nil {
				slog.Error("本地对话初始化失败")
			} else {
				defer chatService.Close()
			}
			articleAIService = articleai.New(store, aiConfig, changed)
		}
	} else {
		slog.Error("无法定位用户数据目录", "error", pathErr)
	}
	notifications := notification.New(toolStore, aiconfig.SystemCredentials{}, notification.ArticleSummarizer(aiConfig))
	var stopNotifications func()
	notifications.SetLocalNotice(func(event notification.LocalNotice) { desktop.Event.Emit(notification.EventName, event) })
	defer func() {
		if stopNotifications != nil {
			stopNotifications()
		}
	}()
	client := netpolicy.New()
	var mediaService *media.Service
	var setDeveloperMenu func(bool, int64)
	var windowClose desktopintegration.WindowClosePreference
	settingsService := settings.NewService(settingsRepository, settings.Hooks{MediaChanged: func(ctx context.Context, prefs settings.Preferences) {
		if mediaService != nil {
			if err := mediaService.PolicyChanged(ctx, media.Policy{RemoteImages: prefs.RemoteImages, CacheEnabled: prefs.MediaCacheEnabled, WebFallback: prefs.WebImageFallback}); err != nil {
				slog.Warn("媒体偏好已保存，但缓存状态更新失败")
			}
		}
		changed()
	}, DeveloperModeChanged: func(enabled bool, revision int64) {
		if setDeveloperMenu != nil {
			setDeveloperMenu(enabled, revision)
		}
	}, WindowCloseSaved: windowClose.Update})
	policy := func(ctx context.Context) media.Policy {
		result := settingsService.Get(ctx)
		if !result.OK || result.Data == nil {
			return media.Policy{RemoteImages: "never"}
		}
		prefs := result.Data.Preferences
		return media.Policy{RemoteImages: prefs.RemoteImages, CacheEnabled: prefs.MediaCacheEnabled, WebFallback: prefs.WebImageFallback}
	}
	notifications.SetImageFetcher(client, func(ctx context.Context) bool { return policy(ctx).RemoteImages != "never" })
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
	}, func(ctx context.Context) bool { return policy(ctx).RemoteImages != "never" })
	refreshService := refresh.NewService(context.Background(), refreshRepository, client, func(event refresh.Event) {
		desktop.Event.Emit(refresh.EventName, event)
	}, changed)
	refreshService.SetIntervalProvider(func(ctx context.Context) time.Duration {
		result := settingsService.Get(ctx)
		if result.OK && result.Data != nil {
			return time.Duration(result.Data.Preferences.RefreshIntervalMinutes) * time.Minute
		}
		return time.Duration(settings.Defaults().RefreshIntervalMinutes) * time.Minute
	})
	refreshService.SetAfterCommit(func(ctx context.Context, id string, document *feed.Document, ids []string) {
		if mediaService != nil {
			mediaService.Ingest(ctx, id, document, ids)
		}
		notifications.Wake()
	})
	readerService := reader.NewService(readerRepository, client, func(ctx context.Context, article database.ReaderArticle) bool {
		value := policy(ctx)
		return value.RemoteImages == "always" || (value.RemoteImages == "subscriptions" && !article.Archived)
	}, func(target string) error {
		return desktop.Browser.OpenURL(target)
	})
	transferService := transfer.NewService(transferRepository, func(ctx context.Context) (string, error) {
		if mainWindow == nil || ctx.Err() != nil {
			return "", context.Canceled
		}
		return desktop.Dialog.OpenFile().SetTitle("导入 OPML").AddFilter("OPML 订阅文件", "*.opml;*.xml").AttachToWindow(mainWindow).PromptForSingleSelection()
	}, func(ctx context.Context) (string, error) {
		if mainWindow == nil || ctx.Err() != nil {
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
	libraryService := library.NewService(libraryRepository, func(ctx context.Context) bool { return policy(ctx).RemoteImages != "never" })
	navigationService := navigation.NewService(navigationRepository, changed)
	transferAPI := transfer.NewAPI(transferService)
	articleAIAPI := articleai.NewAPI(articleAIService)
	var translationService *translation.Service
	if toolStore != nil && aiConfig != nil {
		translationService = translation.New(toolStore, aiConfig, readerService, func(event translation.ProgressEvent) {
			desktop.Event.Emit(translation.ProgressEventName, event)
		}, changed)
	}
	translationAPI := translation.NewAPI(translationService)
	var platformDirectory string
	if pathErr == nil {
		platformDirectory = filepath.Join(dataDir, "Fealdhord", "ai-platform")
	}
	toolRegistry := appagent.Builtins(appagent.Services{
		Store: toolStore, Library: libraryService, Feed: feedService, Navigation: navigationService, Reader: readerService,
		Refresh: refreshService, Settings: settingsService, ArticleAI: articleAIAPI, Transfer: transferAPI, Notifications: notifications,
	})
	platform := appagent.NewPlatform(toolRegistry, platformDirectory)
	if chatService != nil {
		chatService.SetToolSource(platform.RunTools)
		chatService.SetToolPermissions(platform)
	}
	for _, boundService := range []application.Service{
		application.NewService(appagent.NewAPI(platform, func(ctx context.Context) (string, error) {
			if mainWindow == nil || ctx.Err() != nil {
				return "", context.Canceled
			}
			return desktop.Dialog.OpenFile().SetTitle("导入 Skill").AddFilter("Skill（Markdown 或 ZIP）", "*.md;*.zip").AttachToWindow(mainWindow).PromptForSingleSelection()
		})),
		application.NewService(service), application.NewService(settingsService), application.NewService(notification.NewAPI(notifications, readerService)),
		application.NewService(desktopintegration.NewNotifier()),
		application.NewService(readerService),
		application.NewService(aiconfig.NewAPI(aiConfig)),
		application.NewService(chat.NewAPI(chatService, func(ctx context.Context) ([]string, error) {
			if mainWindow == nil || ctx.Err() != nil {
				return nil, context.Canceled
			}
			return desktop.Dialog.OpenFile().SetTitle("添加文件夹").CanChooseFiles(false).CanChooseDirectories(true).
				AttachToWindow(mainWindow).PromptForMultipleSelection()
		})),
		application.NewService(articleAIAPI),
		application.NewService(articlesummary.New(toolStore, aiConfig, readerService)),
		application.NewService(translationAPI),
		application.NewService(transferAPI),
		application.NewService(navigationService),
		application.NewService(libraryService),
		application.NewService(feedService), application.NewService(refresh.NewAPI(refreshService)),
		application.NewServiceWithOptions(media.NewAPI(mediaService, func(png []byte) error {
			return desktopintegration.CopyImageToClipboard(mainWindow, png)
		}), application.ServiceOptions{Route: "/media/"}),
	} {
		desktop.RegisterService(boundService)
	}
	desktop.OnShutdown(func() {
		reopenWindow.Store(nil)
		platform.Shutdown()
		transferService.Close()
		stop()
		refreshService.Close()
		stopNotifications()
		if mediaService != nil {
			mediaService.Close()
		}
		client.CloseIdleConnections()
	})
	if err := notifications.Prepare(context.Background()); err != nil {
		slog.Warn("本地通知恢复标记不可用")
	}
	stopNotifications = notifications.Start(context.Background(), refreshService.StartScheduler())
	if articleAIService != nil {
		defer articleAIService.Start(context.Background())()
	}
	if translationService != nil {
		defer translationService.Start(context.Background())()
	}
	mainWindow = desktop.Window.NewWithOptions(desktopintegration.WindowOptions())
	initialSettings := settingsService.Get(context.Background())
	if initialSettings.OK {
		windowClose.Update(initialSettings.Data.Preferences.QuitOnWindowClose, initialSettings.Data.Revision)
	}
	windowClose.Install(desktop, mainWindow)
	if runtime.GOOS == "linux" {
		mainWindow.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) {
			reopenWindow.Store(mainWindow)
		})
	}
	developerMode := initialSettings.OK && initialSettings.Data.Preferences.DeveloperMode
	setDeveloperMenu = desktopintegration.InstallMenu(desktop, developerMode, func() {
		result := settingsService.Get(context.Background())
		if result.OK && result.Data.Preferences.DeveloperMode {
			desktopintegration.OpenDeveloperTools(mainWindow)
		}
	})
	desktopintegration.ShowMainWindow(desktop, mainWindow)
	installTray(desktop, mainWindow)
	if err := desktop.Run(); err != nil {
		stop()
		slog.Error("桌面应用启动失败", "error", err)
		os.Exit(1)
	}
}
