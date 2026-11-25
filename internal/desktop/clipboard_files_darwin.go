package desktop

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>
#include <stdlib.h>

static char *copyClipboardFilePaths(void) {
    @autoreleasepool {
        NSPasteboard *board = NSPasteboard.generalPasteboard;
        NSDictionary *options = @{NSPasteboardURLReadingFileURLsOnlyKey: @YES};
        NSArray *urls = [board readObjectsForClasses:@[[NSURL class]] options:options];
        if (urls.count == 0) {
            return NULL;
        }
        NSMutableArray<NSString *> *paths = [NSMutableArray arrayWithCapacity:urls.count];
        for (id item in urls) {
            if ([item isKindOfClass:[NSURL class]] && [item isFileURL]) {
                NSString *path = ((NSURL *)item).path;
                if (path.length > 0) {
                    [paths addObject:path];
                }
            }
        }
        if (paths.count == 0) {
            return NULL;
        }
        NSString *joined = [paths componentsJoinedByString:@"\n"];
        const char *utf8 = joined.UTF8String;
        if (!utf8) {
            return NULL;
        }
        return strdup(utf8);
    }
}
*/
import "C"

import (
	"strings"
	"unsafe"
)

func readClipboardFilePaths(_ unsafe.Pointer) []string {
	raw := C.copyClipboardFilePaths()
	if raw == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(raw))
	text := C.GoString(raw)
	if text == "" {
		return nil
	}
	parts := strings.Split(text, "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
