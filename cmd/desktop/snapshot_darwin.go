//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework WebKit

#include <stdlib.h>
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

static WKWebView *choirFindWebView(NSView *view) {
    if ([view isKindOfClass:[WKWebView class]]) return (WKWebView *)view;
    for (NSView *sub in view.subviews) {
        WKWebView *found = choirFindWebView(sub);
        if (found) return found;
    }
    return nil;
}

// choirSnapshotWindow writes a PNG of the window's web content to path. It
// uses WKWebView's own snapshot, so it needs no Screen Recording permission.
static void choirSnapshotWindow(void *nsWindow, const char *path) {
    NSString *target = [NSString stringWithUTF8String:path];
    dispatch_async(dispatch_get_main_queue(), ^{
        NSWindow *window = (__bridge NSWindow *)nsWindow;
        WKWebView *web = choirFindWebView(window.contentView);
        if (!web) {
            NSLog(@"choir: snapshot: no web view in window");
            return;
        }
        // An occluded window's page is "hidden" and WebKit pauses its CSS
        // animations and timers, so bring the window forward and let the
        // page settle before taking the snapshot.
        [window orderFrontRegardless];
        dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(4 * NSEC_PER_SEC)), dispatch_get_main_queue(), ^{
            [web takeSnapshotWithConfiguration:nil completionHandler:^(NSImage *image, NSError *error) {
                if (!image) {
                    NSLog(@"choir: snapshot failed: %@", error);
                    return;
                }
                CGImageRef cg = [image CGImageForProposedRect:NULL context:nil hints:nil];
                NSBitmapImageRep *rep = [[NSBitmapImageRep alloc] initWithCGImage:cg];
                NSData *png = [rep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
                if ([png writeToFile:target atomically:YES]) {
                    NSLog(@"choir: snapshot written to %@", target);
                } else {
                    NSLog(@"choir: snapshot could not write %@", target);
                }
            }];
        });
    });
}
*/
import "C"

import (
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// snapshotWindow is a development aid (CHOIR_DESKTOP_SNAPSHOT): it writes
// the window's rendered page to a PNG as a repeatable launch artifact.
func snapshotWindow(window *application.WebviewWindow, path string) {
	native := window.NativeWindow()
	if native == nil {
		return
	}
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	C.choirSnapshotWindow(native, cPath)
}
