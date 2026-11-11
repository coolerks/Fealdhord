#import <AppKit/AppKit.h>

@interface FealdhordResizeCursorView : NSView
@end

@implementation FealdhordResizeCursorView

- (NSView *)hitTest:(NSPoint)point {
    return nil;
}

- (void)resetCursorRects {
    [super resetCursorRects];
    NSWindowStyleMask style = self.window.styleMask;
    if (!(style & NSWindowStyleMaskResizable) || (style & NSWindowStyleMaskFullScreen)) {
        return;
    }

    NSRect bounds = self.visibleRect;
    const CGFloat edge = 5;
    if (NSWidth(bounds) <= 2 * edge || NSHeight(bounds) <= 2 * edge) {
        return;
    }

    CGFloat left = NSMinX(bounds);
    CGFloat right = NSMaxX(bounds) - edge;
    CGFloat bottom = NSMinY(bounds);
    CGFloat top = NSMaxY(bounds) - edge;
    CGFloat innerWidth = NSWidth(bounds) - 2 * edge;
    CGFloat innerHeight = NSHeight(bounds) - 2 * edge;

    if (@available(macOS 15.0, *)) {
        const struct {
            NSRect rect;
            NSCursorFrameResizePosition position;
        } regions[] = {
            { NSMakeRect(left, bottom + edge, edge, innerHeight), NSCursorFrameResizePositionLeft },
            { NSMakeRect(right, bottom + edge, edge, innerHeight), NSCursorFrameResizePositionRight },
            { NSMakeRect(left + edge, top, innerWidth, edge), NSCursorFrameResizePositionTop },
            { NSMakeRect(left + edge, bottom, innerWidth, edge), NSCursorFrameResizePositionBottom },
            { NSMakeRect(left, top, edge, edge), NSCursorFrameResizePositionTopLeft },
            { NSMakeRect(right, top, edge, edge), NSCursorFrameResizePositionTopRight },
            { NSMakeRect(left, bottom, edge, edge), NSCursorFrameResizePositionBottomLeft },
            { NSMakeRect(right, bottom, edge, edge), NSCursorFrameResizePositionBottomRight },
        };
        for (NSUInteger i = 0; i < sizeof(regions) / sizeof(regions[0]); i++) {
            [self addCursorRect:regions[i].rect
                         cursor:[NSCursor frameResizeCursorFromPosition:regions[i].position
                                                           inDirections:NSCursorFrameResizeDirectionsAll]];
        }
    } else {
        [self addCursorRect:NSMakeRect(left, bottom, edge, NSHeight(bounds)) cursor:NSCursor.resizeLeftRightCursor];
        [self addCursorRect:NSMakeRect(right, bottom, edge, NSHeight(bounds)) cursor:NSCursor.resizeLeftRightCursor];
        [self addCursorRect:NSMakeRect(left + edge, top, innerWidth, edge) cursor:NSCursor.resizeUpDownCursor];
        [self addCursorRect:NSMakeRect(left + edge, bottom, innerWidth, edge) cursor:NSCursor.resizeUpDownCursor];
    }
}

@end

void installResizeCursors(NSWindow *window) {
    NSView *content = window.contentView;
    FealdhordResizeCursorView *view = [[FealdhordResizeCursorView alloc] initWithFrame:content.bounds];
    view.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    [content addSubview:view positioned:NSWindowAbove relativeTo:nil];
    [window invalidateCursorRectsForView:view];
    [view release];
}
