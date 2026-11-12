package desktop

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>

static int copyClipboardPNG(const void *bytes, size_t size) {
    @autoreleasepool {
        NSData *data = [NSData dataWithBytes:bytes length:size];
        NSPasteboard *board = NSPasteboard.generalPasteboard;
        [board clearContents];
        return [board setData:data forType:NSPasteboardTypePNG];
    }
}
*/
import "C"

import "unsafe"

func writeClipboardPNG(_ unsafe.Pointer, png []byte) bool {
	return C.copyClipboardPNG(unsafe.Pointer(&png[0]), C.size_t(len(png))) != 0
}
