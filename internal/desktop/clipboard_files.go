package desktop

import "github.com/wailsapp/wails/v3/pkg/application"

// ReadClipboardFiles 读取系统剪贴板中复制的本地文件与文件夹路径。
func ReadClipboardFiles(window *application.WebviewWindow) []string {
	if window == nil {
		return nil
	}
	var paths []string
	_ = application.InvokeSyncWithError(func() error {
		handle := window.NativeWindow()
		if handle == nil {
			return nil
		}
		paths = readClipboardFilePaths(handle)
		return nil
	})
	return paths
}
