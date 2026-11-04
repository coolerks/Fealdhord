package main

import (
	"context"
	"embed"
	"log/slog"
	"os"

	fealdhord "github.com/coolerks/Fealdhord/internal/app"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	application.RegisterEvent[fealdhord.OperationEvent](fealdhord.EventName)
}

func main() {
	var desktop *application.App
	service, stop := fealdhord.NewBootstrapService(context.Background(), func(event fealdhord.OperationEvent) {
		desktop.Event.Emit(fealdhord.EventName, event)
	})
	desktop = application.New(application.Options{
		Name: "Fealdhord", Description: "折栈，本地优先的 RSS 阅读器",
		Services: []application.Service{application.NewService(service)},
		Assets:   application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac:      application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	desktop.OnShutdown(stop)
	desktop.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "折栈 Fealdhord", Width: 1180, Height: 760, MinWidth: 900, MinHeight: 600,
		BackgroundColour: application.NewRGB(245, 245, 243), URL: "/",
	})
	if err := desktop.Run(); err != nil {
		stop()
		slog.Error("桌面应用启动失败", "error", err)
		os.Exit(1)
	}
}
