package main

import (
	_ "embed"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed build/appicon.png
var appIcon []byte

//go:embed build/tray-template.png
var trayTemplate []byte

//go:embed build/tray-contrast.png
var trayContrast []byte

func installTray(app *application.App, window *application.WebviewWindow) {
	showWindow := func() {
		if window.IsMinimised() {
			window.UnMinimise()
		}
		window.Show()
		window.Focus()
	}

	menu := app.NewMenu()
	menu.Add("显示折栈").OnClick(func(*application.Context) { showWindow() })
	menu.AddSeparator()
	menu.Add("退出折栈").OnClick(func(*application.Context) { app.Quit() })

	tray := app.SystemTray.New()
	tray.SetMenu(menu)
	tray.SetTooltip("折栈 Fealdhord")
	if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(trayTemplate)
	} else {
		tray.SetIcon(trayContrast)
	}
}
