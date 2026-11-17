package desktop

/*
#cgo LDFLAGS: -framework AppKit -framework WebKit
void *installReaderTrackpad(void *window);
void removeReaderTrackpad(void *monitor);
*/
import "C"

import (
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func installReaderTrackpadMonitor(app *application.App, window *application.WebviewWindow) {
	monitor := application.InvokeSyncWithResult(func() unsafe.Pointer {
		return C.installReaderTrackpad(window.NativeWindow())
	})
	if monitor != nil {
		app.OnShutdown(func() { C.removeReaderTrackpad(monitor) })
	}
}
