#import <AppKit/AppKit.h>
#import <WebKit/WebKit.h>

static NSString *const trackpadHandler = @"fealdhordReaderTrackpad";

@interface FealdhordReaderTrackpad : NSObject <WKScriptMessageHandler>
- (instancetype)initWithWebView:(WKWebView *)webView;
- (void)stop;
@end

@implementation FealdhordReaderTrackpad {
    WKWebView *_webView;
    id _monitor;
    BOOL _enabled;
    BOOL _pending;
    BOOL _trackpad;
}

- (instancetype)initWithWebView:(WKWebView *)webView {
    self = [super init];
    if (!self) return nil;
    _webView = webView;
    [webView.configuration.userContentController addScriptMessageHandler:self name:trackpadHandler];
    _monitor = [[NSEvent addLocalMonitorForEventsMatchingMask:NSEventMaskScrollWheel handler:^NSEvent *(NSEvent *event) {
        [self observeScroll:event];
        return event;
    }] retain];
    return self;
}

- (void)userContentController:(WKUserContentController *)controller didReceiveScriptMessage:(WKScriptMessage *)message {
    if (!message.frameInfo.isMainFrame || ![message.body isKindOfClass:[NSNumber class]]) return;
    _enabled = [(NSNumber *)message.body boolValue];
    if (!_enabled) {
        _pending = NO;
        _trackpad = NO;
    }
}

- (void)observeScroll:(NSEvent *)event {
    if (!_enabled || event.window != _webView.window) return;

    // 触控板双指接触会先发出 MayBegin。精细滚轮也能报告像素增量，
    // 所以仅用 AppKit 的手势阶段识别这次滑动。
    NSEventPhase phase = event.phase;
    if (phase & NSEventPhaseMayBegin) _pending = YES;
    if (phase & NSEventPhaseBegan) {
        _trackpad = _pending;
        _pending = NO;
    }
    if (phase & NSEventPhaseCancelled) {
        _pending = NO;
        _trackpad = NO;
    }

    NSEventPhase momentum = event.momentumPhase;
    if (!event.scrollingDeltaX && !event.scrollingDeltaY) return;
    BOOL trackpad = _trackpad && (phase != NSEventPhaseNone || momentum != NSEventPhaseNone);
    // 只提供输入来源和原始时间戳。真实事件继续进入 WebKit 的滚动链路。
    double time = (NSDate.date.timeIntervalSince1970 + event.timestamp - NSProcessInfo.processInfo.systemUptime) * 1000;
    NSString *script = [NSString stringWithFormat:@"window.fealdhordReaderScrollSource?.(%.3f, %@)",
        time, trackpad ? @"true" : @"false"];
    [_webView evaluateJavaScript:script completionHandler:nil];
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

void *installReaderTrackpad(void *handle) {
    NSWindow *window = (NSWindow *)handle;
    for (NSView *view in window.contentView.subviews) {
        if ([view isKindOfClass:[WKWebView class]]) {
            return [[FealdhordReaderTrackpad alloc] initWithWebView:(WKWebView *)view];
        }
    }
    return NULL;
}

void removeReaderTrackpad(void *handle) {
    FealdhordReaderTrackpad *monitor = (FealdhordReaderTrackpad *)handle;
    [monitor stop];
    [monitor release];
}
