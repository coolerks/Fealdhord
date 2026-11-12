//go:build linux && cgo

package desktop

/*
#cgo gtk3 pkg-config: gtk+-3.0
#cgo !gtk3 pkg-config: gtk4
#include <gtk/gtk.h>

static gboolean copyClipboardPNG(const void *bytes, size_t size) {
    GdkDisplay *display = gdk_display_get_default();
    if (!display) return FALSE;
#if GTK_MAJOR_VERSION >= 4
    GBytes *data = g_bytes_new(bytes, size);
    GdkContentProvider *provider = gdk_content_provider_new_for_bytes("image/png", data);
    gboolean written = gdk_clipboard_set_content(gdk_display_get_clipboard(display), provider);
    g_object_unref(provider);
    g_bytes_unref(data);
    return written;
#else
    GdkPixbufLoader *loader = gdk_pixbuf_loader_new_with_type("png", NULL);
    if (!loader) return FALSE;
    gboolean loaded = gdk_pixbuf_loader_write(loader, bytes, size, NULL);
    gboolean closed = gdk_pixbuf_loader_close(loader, NULL);
    GdkPixbuf *image = gdk_pixbuf_loader_get_pixbuf(loader);
    if (loaded && closed && image) {
        GtkClipboard *board = gtk_clipboard_get_default(display);
        gtk_clipboard_set_image(board, image);
        gtk_clipboard_store(board);
    }
    g_object_unref(loader);
    return loaded && closed && image != NULL;
#endif
}
*/
import "C"

import "unsafe"

func writeClipboardPNG(_ unsafe.Pointer, png []byte) bool {
	return C.copyClipboardPNG(unsafe.Pointer(&png[0]), C.size_t(len(png))) != 0
}
