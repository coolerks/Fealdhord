package desktop

import (
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var errImageClipboard = errors.New("系统剪贴板无法写入图片")

// CopyImageToClipboard 将媒体层验证过的 PNG 写入系统剪贴板。
func CopyImageToClipboard(window *application.WebviewWindow, png []byte) error {
	if window == nil || len(png) == 0 {
		return errImageClipboard
	}
	return application.InvokeSyncWithError(func() error {
		handle := window.NativeWindow()
		if handle == nil || !writeClipboardPNG(handle, png) {
			return errImageClipboard
		}
		return nil
	})
}
