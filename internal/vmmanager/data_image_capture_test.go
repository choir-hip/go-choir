package vmmanager

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// A host capture of a running computer's data image (S2 of
// docs/problems/paper-never-checkpointed-2026-10-27.md). Ways it can fail,
// written before the code:
//  1. the clone is taken while the guest still runs: torn image;
//  2. the VM is left paused (clone error, early return): an outage;
//  3. a failed clone leaves a partial capture that a later step publishes;
//  4. an existing capture is overwritten while a processor still reads it;
//  5. a VM that is not running, or has no API socket, is paused anyway;
//  6. the capture lands off the VM's state directory (FICLONE needs one
//     filesystem) or under a name a sweep deletes;
//  7. the pause is not measured.

func captureTestVM(t *testing.T, api *fakeFirecrackerAPI) *Manager {
	t.Helper()
	cfg := DefaultManagerConfig()
	cfg.StateDir = shortStateDir(t)
	mgr := NewManager(cfg)
	mgr.vms["vm-c"] = &VMInstance{
		Config: VMConfig{VMID: "vm-c", ComputerID: "computer-test-c"},
		State:  StateRunning, done: make(chan struct{}),
	}
	if err := os.MkdirAll(filepath.Join(cfg.StateDir, "vm-c"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.StateDir, "vm-c", "data.img"), []byte("ext4 image bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if api != nil {
		serveUnix(t, mgr.apiSocketPath("vm-c"), api)
	}
	return mgr
}

func copyClone(dst, src *os.File) error {
	_, err := io.Copy(dst, src)
	return err
}

func vmStates(api *fakeFirecrackerAPI) []string {
	api.mu.Lock()
	defer api.mu.Unlock()
	var out []string
	for _, c := range api.calls {
		if c.Method == "PATCH" && c.Path == "/vm" {
			s, _ := c.Body["state"].(string)
			out = append(out, s)
		}
	}
	return out
}

// 1, 6, 7: pause, clone while paused, resume; the capture is the image's
// bytes beside it under its own name, and the pause is measured.
func TestCaptureDataImagePausesClonesResumes(t *testing.T) {
	api := &fakeFirecrackerAPI{}
	mgr := captureTestVM(t, api)
	var mu sync.Mutex
	var statesAtClone []string
	mgr.captureCloneFn = func(dst, src *os.File) error {
		mu.Lock()
		statesAtClone = vmStates(api)
		mu.Unlock()
		return copyClone(dst, src)
	}
	got, err := mgr.CaptureDataImage("vm-c")
	if err != nil {
		t.Fatalf("CaptureDataImage: %v", err)
	}
	want := filepath.Join(mgr.cfg.StateDir, "vm-c", "data.img.capture")
	if got.Path != want {
		t.Fatalf("capture path = %q, want %q", got.Path, want)
	}
	raw, err := os.ReadFile(got.Path)
	if err != nil || string(raw) != "ext4 image bytes" {
		t.Fatalf("capture = %q, %v", raw, err)
	}
	if strings.Join(statesAtClone, ",") != "Paused" {
		t.Fatalf("states at clone = %v, want [Paused]", statesAtClone)
	}
	if s := vmStates(api); strings.Join(s, ",") != "Paused,Resumed" {
		t.Fatalf("vm states = %v, want Paused then Resumed", s)
	}
	if got.PausedFor <= 0 {
		t.Fatalf("pause not measured: %+v", got)
	}
	if dataImageCloneTmpRE.MatchString(filepath.Base(got.Path)) {
		t.Fatalf("capture name %q is one the clone sweep deletes", got.Path)
	}
}

// 2, 3: a failed clone still resumes the VM and leaves no capture.
func TestCaptureDataImageCloneFailureResumesAndRemoves(t *testing.T) {
	api := &fakeFirecrackerAPI{}
	mgr := captureTestVM(t, api)
	mgr.captureCloneFn = func(dst, src *os.File) error {
		_, _ = dst.Write([]byte("partial"))
		return errors.New("injected clone failure")
	}
	if _, err := mgr.CaptureDataImage("vm-c"); err == nil {
		t.Fatal("clone failure returned no error")
	}
	if s := vmStates(api); strings.Join(s, ",") != "Paused,Resumed" {
		t.Fatalf("vm states = %v, want Paused then Resumed", s)
	}
	if _, err := os.Stat(filepath.Join(mgr.cfg.StateDir, "vm-c", "data.img.capture")); !os.IsNotExist(err) {
		t.Fatalf("partial capture left behind: %v", err)
	}
}

// 2: a refused pause clones nothing and does not resume a VM it never paused.
func TestCaptureDataImagePauseFailureClonesNothing(t *testing.T) {
	api := &fakeFirecrackerAPI{failPath: "/vm"}
	mgr := captureTestVM(t, api)
	cloned := false
	mgr.captureCloneFn = func(dst, src *os.File) error { cloned = true; return copyClone(dst, src) }
	if _, err := mgr.CaptureDataImage("vm-c"); err == nil {
		t.Fatal("pause failure returned no error")
	}
	if cloned {
		t.Fatal("cloned after a refused pause")
	}
	if _, err := os.Stat(filepath.Join(mgr.cfg.StateDir, "vm-c", "data.img.capture")); !os.IsNotExist(err) {
		t.Fatalf("capture left after a refused pause: %v", err)
	}
}

// 4: an existing capture is never overwritten, and the VM is not paused.
func TestCaptureDataImageRefusesExistingCapture(t *testing.T) {
	api := &fakeFirecrackerAPI{}
	mgr := captureTestVM(t, api)
	mgr.captureCloneFn = copyClone
	existing := filepath.Join(mgr.cfg.StateDir, "vm-c", "data.img.capture")
	if err := os.WriteFile(existing, []byte("in use"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.CaptureDataImage("vm-c"); err == nil {
		t.Fatal("existing capture was not refused")
	}
	if raw, _ := os.ReadFile(existing); string(raw) != "in use" {
		t.Fatalf("existing capture overwritten: %q", raw)
	}
	if s := vmStates(api); len(s) != 0 {
		t.Fatalf("VM paused despite the refusal: %v", s)
	}
}

// 5: no running VM, or no API socket, refuses before any pause.
func TestCaptureDataImageRefusesWithoutRunningVMOrSocket(t *testing.T) {
	mgr := captureTestVM(t, nil)
	mgr.captureCloneFn = copyClone
	if _, err := mgr.CaptureDataImage("vm-missing"); err == nil {
		t.Fatal("missing VM was not refused")
	}
	if _, err := mgr.CaptureDataImage("vm-c"); err == nil {
		t.Fatal("VM without an API socket was not refused")
	}
	if _, err := os.Stat(filepath.Join(mgr.cfg.StateDir, "vm-c", "data.img.capture")); !os.IsNotExist(err) {
		t.Fatalf("capture left behind: %v", err)
	}
}
