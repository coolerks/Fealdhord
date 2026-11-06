package desktop

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>

static void installSidebarMaterial(void *handle) {
    NSWindow *window = (NSWindow *)handle;
    NSView *content = window.contentView;
    if (!content) {
        return;
    }

    NSVisualEffectView *material = [[NSVisualEffectView alloc] initWithFrame:content.bounds];
    material.material = NSVisualEffectMaterialSidebar;
    material.blendingMode = NSVisualEffectBlendingModeBehindWindow;
    material.state = NSVisualEffectStateFollowsWindowActiveState;
    material.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    [content addSubview:material positioned:NSWindowBelow relativeTo:nil];
    [material release];
}
*/
import "C"

import "github.com/wailsapp/wails/v3/pkg/application"

func installWindowSidebarMaterial(window *application.WebviewWindow) {
	application.InvokeSync(func() {
		C.installSidebarMaterial(window.NativeWindow())
	})
}
