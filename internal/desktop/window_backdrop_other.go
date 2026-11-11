//go:build !windows

package desktop

import "github.com/wailsapp/wails/v3/pkg/application"

func configureWindowsBackdrop(_ *application.WebviewWindowOptions) {}
