//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AuthenticationServices -framework Cocoa

#import <AuthenticationServices/AuthenticationServices.h>
#import <Cocoa/Cocoa.h>

@interface AuthPresentationContext : NSObject <ASWebAuthenticationPresentationContextProviding>
@end

@implementation AuthPresentationContext
- (NSWindow *)presentationAnchorForWebAuthenticationSession:(ASWebAuthenticationSession *)session {
    NSWindow *window = [[NSApplication sharedApplication] keyWindow];
    if (!window) {
        for (NSWindow *w in [[NSApplication sharedApplication] windows]) {
            if (w.isVisible) { window = w; break; }
        }
    }
    return window;
}
@end

// startWebAuthSessionSync opens an ASWebAuthenticationSession to the given URL
// and waits for the callback redirect to the specified URL scheme.
// Returns the callback URL (caller must free) or NULL with an error message
// (caller must free) on failure.
static const char *startWebAuthSessionSync(const char *urlStr, const char *schemeStr, char **errOut) {
    static AuthPresentationContext *ctx = nil;
    if (!ctx) {
        ctx = [[AuthPresentationContext alloc] init];
    }

    // Keep a strong reference to the session so it isn't deallocated
    // while waiting for the callback. Without this, ARC may release
    // the session before ASWebAuthenticationSession fires its completion.
    static ASWebAuthenticationSession *activeSession = nil;
    activeSession = nil;

    __block const char *result = NULL;
    __block const char *errMsg = NULL;
    dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);

    NSURL *url = [NSURL URLWithString:[NSString stringWithUTF8String:urlStr]];
    NSString *scheme = [NSString stringWithUTF8String:schemeStr];

    activeSession = [[ASWebAuthenticationSession alloc]
        initWithURL:url
        callbackURLScheme:scheme
        completionHandler:^(NSURL *callbackURL, NSError *error) {
            if (error) {
                errMsg = strdup([[error localizedDescription] UTF8String]);
            } else {
                result = strdup([[callbackURL absoluteString] UTF8String]);
            }
            activeSession = nil; // release reference
            dispatch_semaphore_signal(semaphore);
        }];

    activeSession.presentationContextProvider = ctx;
    activeSession.prefersEphemeralWebBrowserSession = NO;

    dispatch_async(dispatch_get_main_queue(), ^{
        BOOL started = [activeSession start];
        if (!started) {
            if (!errMsg) errMsg = strdup("Failed to start authentication session");
            activeSession = nil;
            dispatch_semaphore_signal(semaphore);
        }
    });

    long waitResult = dispatch_semaphore_wait(semaphore, dispatch_time(DISPATCH_TIME_NOW, 60 * NSEC_PER_SEC));
    if (waitResult != 0) {
        // Cancel the session on timeout.
        [activeSession cancel];
        activeSession = nil;
        if (errOut) *errOut = strdup("Authentication session timed out");
        return NULL;
    }

    if (errMsg) {
        if (errOut) *errOut = (char *)errMsg;
        else free((void *)errMsg);
        return NULL;
    }

    return result;
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

func startWebAuthSession(authURL, callbackScheme string) (string, error) {
	cURL := C.CString(authURL)
	defer C.free(unsafe.Pointer(cURL))

	cScheme := C.CString(callbackScheme)
	defer C.free(unsafe.Pointer(cScheme))

	var cErr *C.char
	result := C.startWebAuthSessionSync(cURL, cScheme, &cErr)

	if cErr != nil {
		errMsg := C.GoString(cErr)
		C.free(unsafe.Pointer(cErr))
		return "", errors.New(errMsg)
	}

	if result != nil {
		callbackURL := C.GoString(result)
		C.free(unsafe.Pointer(result))
		return callbackURL, nil
	}

	return "", errors.New("authentication session returned no result")
}
