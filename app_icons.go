package main

import (
	_ "embed"
	"runtime"

	desktopintegration "github.com/coolerks/Fealdhord/internal/desktop"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed build/appicon.png
var appIcon []byte

//go:embed build/windows/appicon.png
var windowsAppIcon []byte

//go:embed build/tray-template.png
var trayTemplate []byte

//go:embed build/tray-contrast.png
var trayContrast []byte

func desktopIcon() []byte {
	if runtime.GOOS == "windows" {
		return windowsAppIcon
	}
	return appIcon
}

func installTray(app *application.App, window *application.WebviewWindow) {
	showWindow := func() { desktopintegration.RestoreMainWindow(window) }

	menu := app.NewMenu()
	menu.Add("显示折栈").OnClick(func(*application.Context) { showWindow() })
	menu.AddSeparator()
	menu.Add("退出折栈").OnClick(func(*application.Context) { app.Quit() })

	tray := app.SystemTray.New()
	tray.OnClick(showWindow)
	tray.SetMenu(menu)
	tray.SetTooltip("折栈 Fealdhord")
	switch runtime.GOOS {
	case "darwin":
		tray.SetTemplateIcon(trayTemplate)
	case "windows":
		tray.SetIcon(windowsAppIcon)
	default:
		tray.SetIcon(trayContrast)
	}
}
