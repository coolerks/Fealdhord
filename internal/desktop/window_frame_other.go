//go:build !linux || !cgo || gtk3

package desktop

import "github.com/wailsapp/wails/v3/pkg/application"

const usesGTKWindowFrame = false

func prepareGTKWindowFrame(_ *application.WebviewWindow) {}
