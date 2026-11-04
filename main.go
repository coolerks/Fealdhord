package main

import (
	"context"
	"embed"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/coolerks/Fealdhord/internal/database"
	desktopintegration "github.com/coolerks/Fealdhord/internal/desktop"
	"github.com/coolerks/Fealdhord/internal/settings"

	fealdhord "github.com/coolerks/Fealdhord/internal/app"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	application.RegisterEvent[fealdhord.OperationEvent](fealdhord.EventName)
	application.RegisterEvent[desktopintegration.CommandEvent](desktopintegration.CommandEventName)
}

func main() {
	var desktop *application.App
	service, stop := fealdhord.NewBootstrapService(context.Background(), func(event fealdhord.OperationEvent) {
		desktop.Event.Emit(fealdhord.EventName, event)
	})
	var settingsRepository settings.Repository
	dataDir, pathErr := os.UserConfigDir()
	if pathErr == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		store, err := database.Open(ctx, filepath.Join(dataDir, "Fealdhord", "fealdhord.db"))
		cancel()
		if err != nil {
			slog.Error("本地设置初始化失败", "error", err)
		} else {
			settingsRepository = store
			defer func() {
				if err := store.Close(); err != nil {
					slog.Error("关闭设置数据库失败", "error", err)
				}
			}()
		}
	} else {
		slog.Error("无法定位用户数据目录", "error", pathErr)
	}
	settingsService := settings.NewService(settingsRepository)
	desktop = application.New(application.Options{
		Name: "Fealdhord", Description: "折栈，本地优先的 RSS 阅读器",
		Services: []application.Service{application.NewService(service), application.NewService(settingsService)},
		Assets:   application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac:      application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	desktop.OnShutdown(stop)
	desktopintegration.InstallMenu(desktop)
	desktop.Window.NewWithOptions(desktopintegration.WindowOptions())
	if err := desktop.Run(); err != nil {
		stop()
		slog.Error("桌面应用启动失败", "error", err)
		os.Exit(1)
	}
}
