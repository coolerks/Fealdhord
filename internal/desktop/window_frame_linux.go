//go:build linux && cgo && !gtk3

package desktop

/*
#cgo pkg-config: gtk4
#include <gtk/gtk.h>

#define FLOATING_WINDOW "window.fealdhord-window.csd:not(.maximized):not(.fullscreen)" \
    ":not(.tiled):not(.tiled-top):not(.tiled-right):not(.tiled-bottom):not(.tiled-left)"

static void installGTKWindowFrame(void *handle) {
    if (!handle) return;

    GtkWindow *window = GTK_WINDOW(handle);
    GtkWidget *widget = GTK_WIDGET(window);
    GtkWidget *content = gtk_window_get_child(window);

    // 保留 GTK 客户端装饰，通过隐藏标题栏部件让应用菜单占据第一行。
    GtkWidget *titlebar = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 0);
    gtk_widget_set_visible(titlebar, FALSE);
    gtk_window_set_titlebar(window, titlebar);

    GtkCssProvider *provider = gtk_css_provider_new();
    gtk_css_provider_load_from_string(provider,
        "window.fealdhord-window.csd, "
        "window.fealdhord-window.csd > .fealdhord-window-content { border-radius: 0; }"
        FLOATING_WINDOW ", " FLOATING_WINDOW " > .fealdhord-window-content { border-radius: 12px; }");
    gtk_style_context_add_provider_for_display(gtk_widget_get_display(widget),
        GTK_STYLE_PROVIDER(provider), GTK_STYLE_PROVIDER_PRIORITY_APPLICATION);
    g_object_unref(provider);

    gtk_widget_add_css_class(widget, "fealdhord-window");
    gtk_widget_add_css_class(content, "fealdhord-window-content");
    // 裁切 WebView 内容，保留父窗口的阴影和外沿缩放命中区域。
    gtk_widget_set_overflow(content, GTK_OVERFLOW_HIDDEN);
}
*/
import "C"

import (
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const usesGTKWindowFrame = true

func prepareGTKWindowFrame(window *application.WebviewWindow) {
	var once sync.Once
	window.RegisterHook(events.Common.WindowRuntimeReady, func(*application.WindowEvent) {
		once.Do(func() {
			application.InvokeSync(func() {
				C.installGTKWindowFrame(window.NativeWindow())
			})
		})
	})
}
