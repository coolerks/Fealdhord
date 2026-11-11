package desktop

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
)

func configureWindowsBackdrop(options *application.WebviewWindowOptions) {
	// 旧版系统保留实色，避免 Wails 回退到旧式 AccentPolicy。
	if !w32.SupportsBackdropTypes() {
		return
	}
	options.BackgroundType = application.BackgroundTypeTranslucent
	options.BackgroundColour = application.NewRGBA(0, 0, 0, 0)
	options.Windows.BackdropType = application.Acrylic
}
