//go:build integration

package browsercontrol

import (
	"context"
	"encoding/base64"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

func TestCaptureObscuraCDPScreenshotLive(t *testing.T) {
	if os.Getenv("GO_CHOIR_RUN_OBSCURA_CDP") != "1" {
		t.Skip("set GO_CHOIR_RUN_OBSCURA_CDP=1 to verify live Obscura CDP screenshot capture")
	}
	path := strings.TrimSpace(os.Getenv("CHOIR_OBSCURA_BIN"))
	if path == "" {
		path = strings.TrimSpace(os.Getenv("OBSCURA_BIN"))
	}
	if path == "" {
		path = "/Users/wiz/obscura/target/release/obscura"
	}
	resolved, err := resolveExecutable(path)
	if err != nil {
		t.Fatalf("resolve obscura: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	encoded, err := captureObscuraCDPScreenshot(ctx, resolved, "https://example.com")
	if err != nil {
		t.Fatalf("capture screenshot: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode screenshot: %v", err)
	}
	if len(raw) <= 1000 {
		t.Fatalf("screenshot bytes = %d, want > 1000", len(raw))
	}
	if got := string(raw[:8]); got != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("screenshot magic = %x, want PNG", raw[:8])
	}
}

func TestHandlerControlsObscuraCDPSessionLive(t *testing.T) {
	if os.Getenv("GO_CHOIR_RUN_OBSCURA_CDP") != "1" {
		t.Skip("set GO_CHOIR_RUN_OBSCURA_CDP=1 to verify live Obscura CDP bounded control")
	}
	path := strings.TrimSpace(os.Getenv("CHOIR_OBSCURA_BIN"))
	if path == "" {
		path = strings.TrimSpace(os.Getenv("OBSCURA_BIN"))
	}
	if path == "" {
		path = "/Users/wiz/obscura/target/release/obscura"
	}
	resolved, err := resolveExecutable(path)
	if err != nil {
		t.Fatalf("resolve obscura: %v", err)
	}
	handler := NewHandler(provideriface.Config{}, nil, nil)
	defer handler.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if _, _, err := handler.captureBrowserCDPScreenshot(ctx, "browser-control-live", resolved, "https://httpbin.org/forms/post"); err != nil {
		t.Fatalf("initial capture: %v", err)
	}
	fill, fillScreenshot, fillSessionID, err := handler.controlBrowserCDPSession(ctx, "browser-control-live", "fill", "input[name=custname]", "choir-control-ok")
	if err != nil {
		t.Fatalf("fill control: %v", err)
	}
	if !fill.OK || fill.Value != "choir-control-ok" {
		t.Fatalf("fill result = %+v, want ok value", fill)
	}
	click, clickScreenshot, clickSessionID, err := handler.controlBrowserCDPSession(ctx, "browser-control-live", "click", "input[name=topping]", "")
	if err != nil {
		t.Fatalf("click control: %v", err)
	}
	if !click.OK {
		t.Fatalf("click result = %+v, want ok", click)
	}
	if fillSessionID == "" || fillSessionID != clickSessionID {
		t.Fatalf("control session ids = %q/%q, want stable non-empty id", fillSessionID, clickSessionID)
	}
	for label, encoded := range map[string]string{"fill": fillScreenshot, "click": clickScreenshot} {
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatalf("%s decode screenshot: %v", label, err)
		}
		if len(raw) <= 1000 {
			t.Fatalf("%s screenshot bytes = %d, want > 1000", label, len(raw))
		}
	}
	if !handler.closeBrowserCDPSession("browser-control-live") {
		t.Fatal("closeBrowserCDPSession returned false, want true")
	}
}

func TestHandlerReusesObscuraCDPSessionLive(t *testing.T) {
	if os.Getenv("GO_CHOIR_RUN_OBSCURA_CDP") != "1" {
		t.Skip("set GO_CHOIR_RUN_OBSCURA_CDP=1 to verify live Obscura CDP session reuse")
	}
	path := strings.TrimSpace(os.Getenv("CHOIR_OBSCURA_BIN"))
	if path == "" {
		path = strings.TrimSpace(os.Getenv("OBSCURA_BIN"))
	}
	if path == "" {
		path = "/Users/wiz/obscura/target/release/obscura"
	}
	resolved, err := resolveExecutable(path)
	if err != nil {
		t.Fatalf("resolve obscura: %v", err)
	}
	handler := NewHandler(provideriface.Config{}, nil, nil)
	defer handler.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	first, firstSessionID, err := handler.captureBrowserCDPScreenshot(ctx, "browser-session-live", resolved, "https://example.com")
	if err != nil {
		t.Fatalf("first capture: %v", err)
	}
	second, secondSessionID, err := handler.captureBrowserCDPScreenshot(ctx, "browser-session-live", resolved, "https://example.com/?choir=1")
	if err != nil {
		t.Fatalf("second capture: %v", err)
	}
	if firstSessionID == "" || secondSessionID == "" {
		t.Fatalf("session ids should be non-empty: first=%q second=%q", firstSessionID, secondSessionID)
	}
	if firstSessionID != secondSessionID {
		t.Fatalf("session id changed across navigations: first=%q second=%q", firstSessionID, secondSessionID)
	}
	for label, encoded := range map[string]string{"first": first, "second": second} {
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatalf("%s decode screenshot: %v", label, err)
		}
		if len(raw) <= 1000 {
			t.Fatalf("%s screenshot bytes = %d, want > 1000", label, len(raw))
		}
		if got := string(raw[:8]); got != "\x89PNG\r\n\x1a\n" {
			t.Fatalf("%s screenshot magic = %x, want PNG", label, raw[:8])
		}
	}
	if !handler.closeBrowserCDPSession("browser-session-live") {
		t.Fatal("closeBrowserCDPSession returned false, want true for active session")
	}
	if handler.closeBrowserCDPSession("browser-session-live") {
		t.Fatal("closeBrowserCDPSession returned true for already closed session")
	}
}
