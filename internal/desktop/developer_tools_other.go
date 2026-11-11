//go:build !darwin || !private_mac_apis

package desktop

import "github.com/wailsapp/wails/v3/pkg/application"

func OpenDeveloperTools(window *application.WebviewWindow) {
	window.OpenDevTools()
}
