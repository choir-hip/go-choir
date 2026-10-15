//go:build linux

package capsule

import (
	"os"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/visualcapture"
)

// Visual capture, guest side (docs/design/visual-evidence-slice-2026-10-15.md).
// The executor reads the grant vmctl put on the credential disk and, only
// when it is on and the image names a Chromium, gives each capsule broker the
// Chromium and fontconfig paths. The broker passes them to jobs alone
// (CaptureJobEnv); cells and direct exec never see them (V1).

// SetVisualCaptureSource names the credential directory holding the grant,
// the image's Chromium and its fontconfig file. It applies to capsules
// spawned afterwards.
func (e *Executor) SetVisualCaptureSource(dir, browser, fontconfig string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.visualCaptureDir = strings.TrimSpace(dir)
	e.captureBrowser = strings.TrimSpace(browser)
	e.captureFontconfig = strings.TrimSpace(fontconfig)
}

// VisualCaptureStatus reports whether capture jobs may launch Chromium on
// this computer, and why not.
func (e *Executor) VisualCaptureStatus() visualcapture.Status {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return visualcapture.ReadGrant(e.visualCaptureDir, e.captureBrowser)
}

// teardownReclaimEnabled is the report's control-run switch:
// CHOIR_CAPSULE_TEARDOWN_RECLAIM=off leaves a capsule's page cache to be
// reparented, as before this slice.
func teardownReclaimEnabled() bool {
	return strings.TrimSpace(strings.ToLower(os.Getenv("CHOIR_CAPSULE_TEARDOWN_RECLAIM"))) != "off"
}

// visualCaptureBrokerEnvLocked is what a new broker carries for capture
// jobs: nothing unless the grant is on. Callers hold e.mu.
func (e *Executor) visualCaptureBrokerEnvLocked() []string {
	st := visualcapture.ReadGrant(e.visualCaptureDir, e.captureBrowser)
	if !st.Granted {
		return nil
	}
	env := []string{visualcapture.BrowserEnv + "=" + st.Browser}
	if e.captureFontconfig != "" {
		env = append(env, visualcapture.FontconfigEnv+"="+e.captureFontconfig)
	}
	return env
}
