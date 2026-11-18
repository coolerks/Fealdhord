#import <AppKit/AppKit.h>
#import <WebKit/WebKit.h>

static NSString *const trackpadHandler = @"fealdhordReaderTrackpad";

@interface FealdhordReaderTrackpad : NSObject <WKScriptMessageHandler>
- (instancetype)initWithWebView:(WKWebView *)webView;
- (NSEvent *)routeScroll:(NSEvent *)event;
- (void)stop;
@end

@implementation FealdhordReaderTrackpad {
    WKWebView *_webView;
    id _monitor;
    NSRect _viewport;
}

- (instancetype)initWithWebView:(WKWebView *)webView {
    self = [super init];
    if (!self) return nil;
    _webView = webView;
    [webView.configuration.userContentController addScriptMessageHandler:self name:trackpadHandler];
    _monitor = [[NSEvent addLocalMonitorForEventsMatchingMask:NSEventMaskScrollWheel handler:^NSEvent *(NSEvent *event) {
        return [self routeScroll:event];
    }] retain];
    return self;
}

- (void)userContentController:(WKUserContentController *)controller didReceiveScriptMessage:(WKScriptMessage *)message {
    if (!message.frameInfo.isMainFrame) return;
    _viewport = NSZeroRect;
    if (![message.body isKindOfClass:[NSArray class]]) return;
    NSArray *bounds = message.body;
    if (bounds.count != 4) return;
    for (id coordinate in bounds) {
        if (![coordinate isKindOfClass:[NSNumber class]] || !isfinite([coordinate doubleValue])) return;
    }
    _viewport = NSMakeRect([bounds[0] doubleValue], [bounds[1] doubleValue],
        [bounds[2] doubleValue], [bounds[3] doubleValue]);
}

- (NSEvent *)routeScroll:(NSEvent *)event {
    NSWindow *window = event.window ?: NSApp.keyWindow;
    if (NSIsEmptyRect(_viewport) || window != _webView.window) return event;
    // 手势阶段标识滑动或惯性。无阶段的鼠标滚轮和独立的捏合事件继续交给 WebKit。
    if (event.phase == NSEventPhaseNone && event.momentumPhase == NSEventPhaseNone) return event;

    NSPoint location = event.locationInWindow;
    if (!event.window) location = [window convertPointFromScreen:location];
    NSPoint point = [_webView convertPoint:location fromView:nil];
    NSRect bounds = _webView.bounds;
    CGFloat zoom = _webView.pageZoom;
    CGFloat x = (point.x - NSMinX(bounds)) / zoom;
    CGFloat y = (_webView.isFlipped ? point.y - NSMinY(bounds) : NSMaxY(bounds) - point.y) / zoom;
    if (!NSPointInRect(NSMakePoint(x, y), _viewport)) return event;

    CGFloat dx = -event.scrollingDeltaX / zoom;
    CGFloat dy = -event.scrollingDeltaY / zoom;
    if (dx || dy) {
        // 直接转交原始位移，避免同一滑动再次被 WebKit 当成滚轮处理。
        NSString *script = [NSString stringWithFormat:@"window.fealdhordReaderPan?.(%g,%g,%g,%g)", x, y, dx, dy];
        [_webView evaluateJavaScript:script completionHandler:nil];
    }
    return nil;
}

- (void)stop {
    if (_monitor) {
        [NSEvent removeMonitor:_monitor];
        [_monitor release];
        _monitor = nil;
    }
    [_webView.configuration.userContentController removeScriptMessageHandlerForName:trackpadHandler];
}
@end

static WKWebView *findReaderWebView(NSView *view) {
    if ([view isKindOfClass:[WKWebView class]]) return (WKWebView *)view;
    for (NSView *child in view.subviews) {
        WKWebView *webView = findReaderWebView(child);
        if (webView) return webView;
    }
    return nil;
}

void *installReaderTrackpad(void *handle) {
    WKWebView *webView = findReaderWebView([(NSWindow *)handle contentView]);
    return webView ? [[FealdhordReaderTrackpad alloc] initWithWebView:webView] : NULL;
}

void removeReaderTrackpad(void *handle) {
    FealdhordReaderTrackpad *monitor = (FealdhordReaderTrackpad *)handle;
    [monitor stop];
    [monitor release];
}
