//go:build darwin && private_mac_apis

package desktop

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit -framework WebKit
#import <AppKit/AppKit.h>
#import <WebKit/WebKit.h>

static WKWebView *findWebView(NSView *view) {
    if ([view isKindOfClass:[WKWebView class]]) {
        return (WKWebView *)view;
    }
    for (NSView *child in view.subviews) {
        WKWebView *webView = findWebView(child);
        if (webView) {
            return webView;
        }
    }
    return nil;
}

static void enableLocalInspector(void *handle) {
    NSWindow *window = (NSWindow *)handle;
    WKWebView *webView = findWebView(window.contentView);
    @try {
        [webView.configuration.preferences setValue:@YES forKey:@"developerExtrasEnabled"];
    } @catch (NSException *exception) {
        NSLog(@"[Fealdhord] Unable to enable the Web Inspector: %@", exception.reason);
    }
}
*/
import "C"

import "github.com/wailsapp/wails/v3/pkg/application"

func OpenDeveloperTools(window *application.WebviewWindow) {
	// Wails 3.0.0 在 macOS 13.3+ 先启用 Safari 检查，跳过了本地检查器所需的偏好。
	application.InvokeSync(func() { C.enableLocalInspector(window.NativeWindow()) })
	window.OpenDevTools()
}
