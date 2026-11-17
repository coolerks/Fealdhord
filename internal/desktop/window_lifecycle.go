package desktop

import (
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// WindowClosePreference tracks the latest committed preference for native close events.
type WindowClosePreference struct {
	mu       sync.Mutex
	revision int64
	quit     bool
}

func (p *WindowClosePreference) Update(quit bool, revision int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if revision >= p.revision {
		p.revision = revision
		p.quit = quit
	}
}

func (p *WindowClosePreference) shouldQuit() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.quit
}

func (p *WindowClosePreference) Install(app *application.App, window *application.WebviewWindow) {
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		event.Cancel()
		if app.Context().Err() != nil {
			return
		}
		if p.shouldQuit() {
			app.Quit()
			return
		}
		application.InvokeSync(func() { window.Hide() })
	})
}

func RestoreMainWindow(window *application.WebviewWindow) {
	application.InvokeSync(func() {
		if window.IsMinimised() {
			window.UnMinimise()
		}
		window.Show()
		window.Focus()
	})
}
