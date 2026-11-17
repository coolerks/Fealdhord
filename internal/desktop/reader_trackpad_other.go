//go:build !darwin

package desktop

import "github.com/wailsapp/wails/v3/pkg/application"

func installReaderTrackpadMonitor(*application.App, *application.WebviewWindow) {}
