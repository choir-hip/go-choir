package vmmanager

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
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

// 2: a refused pause clones nothing, and a resume is still attempted (a
// refusal may come from a VM that did pause; review 364).
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
	if s := vmStates(api); len(s) < 2 || s[0] != "Paused" || s[1] != "Resumed" {
		t.Fatalf("vm states = %v, want a pause then a resume attempt", s)
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

// Review 364 (DeepSeek, HIGH): a VM is never left paused. More ways to
// fail, written before the code:
//  8. vmctl dies between pause and resume: nothing durable says the VM is
//     paused, and nothing resumes it;
//  9. a pause answered with an error (a 5xx after the VM did pause) is not
//     followed by a resume;
// 10. a clone that hangs holds the guest paused without bound;
// 11. a resume whose answer is lost is reported as a VM left paused when it
//     runs (no probe), or a VM that is paused is reported as running;
// 12. a stop or kill-mode hibernate runs during a capture's pause.

// statefulFC is a Firecracker API that keeps the VM's state: a pause pauses
// it (and may then answer pauseStatus anyway), a resume of a paused VM
// resumes it unless resumeFails, and GET / reports the state.
type statefulFC struct {
	mu          sync.Mutex
	state       string
	pauseStatus int
	resumeFails bool
	onPause     func()
	calls       []string
}

func (f *statefulFC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body map[string]string
	_ = json.NewDecoder(r.Body).Decode(&body)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, r.Method+" "+r.URL.Path+" "+body["state"])
	if r.Method == http.MethodGet && r.URL.Path == "/" {
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "vm", "state": f.state})
		return
	}
	if r.Method != http.MethodPatch || r.URL.Path != "/vm" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	switch body["state"] {
	case "Paused":
		if f.onPause != nil {
			f.onPause()
		}
		f.state = "Paused"
		if f.pauseStatus != 0 {
			w.WriteHeader(f.pauseStatus)
			_, _ = w.Write([]byte(`{"fault_message":"injected"}`))
			return
		}
	case "Resumed":
		if f.resumeFails || f.state != "Paused" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"fault_message":"not resumed"}`))
			return
		}
		f.state = "Running"
	}
	w.WriteHeader(http.StatusNoContent)
}

func (f *statefulFC) now() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state
}

func captureStatefulVM(t *testing.T, fc *statefulFC) *Manager {
	t.Helper()
	mgr := captureTestVM(t, nil)
	serveUnix(t, mgr.apiSocketPath("vm-c"), fc)
	return mgr
}

func pauseMarkerPath(mgr *Manager) string {
	return filepath.Join(mgr.cfg.StateDir, "vm-c", "capture-pause.json")
}

func capturePath(mgr *Manager) string {
	return filepath.Join(mgr.cfg.StateDir, "vm-c", "data.img.capture")
}

// 8: the pause marker is durable before the pause and gone after a
// confirmed resume.
func TestCaptureDataImageMarksThePauseDurably(t *testing.T) {
	fc := &statefulFC{state: "Running"}
	mgr := captureStatefulVM(t, fc)
	markedAtPause := false
	fc.onPause = func() {
		_, err := os.Stat(pauseMarkerPath(mgr))
		markedAtPause = err == nil
	}
	mgr.captureCloneFn = copyClone
	if _, err := mgr.CaptureDataImage("vm-c"); err != nil {
		t.Fatalf("CaptureDataImage: %v", err)
	}
	if !markedAtPause {
		t.Fatal("no pause marker when the pause was sent")
	}
	if _, err := os.Stat(pauseMarkerPath(mgr)); !os.IsNotExist(err) {
		t.Fatalf("pause marker left after a confirmed resume: %v", err)
	}
}

// 9: a pause answered 5xx after the VM did pause is resumed; nothing is
// cloned and no marker stays.
func TestCaptureDataImageResumesAfterAPauseError(t *testing.T) {
	fc := &statefulFC{state: "Running", pauseStatus: http.StatusInternalServerError}
	mgr := captureStatefulVM(t, fc)
	cloned := false
	mgr.captureCloneFn = func(dst, src *os.File) error { cloned = true; return nil }
	if _, err := mgr.CaptureDataImage("vm-c"); err == nil {
		t.Fatal("a pause error returned no error")
	}
	if fc.now() != "Running" {
		t.Fatalf("VM state after a pause error = %s, want Running", fc.now())
	}
	if cloned {
		t.Fatal("cloned after a pause error")
	}
	if _, err := os.Stat(pauseMarkerPath(mgr)); !os.IsNotExist(err) {
		t.Fatalf("pause marker left after a confirmed resume: %v", err)
	}
}

// 10: a clone past its deadline is abandoned: the VM is resumed while the
// clone still runs, the call fails, and the torn clone is removed when the
// clone returns.
func TestCaptureDataImageCloneDeadlineResumes(t *testing.T) {
	fc := &statefulFC{state: "Running"}
	mgr := captureStatefulVM(t, fc)
	mgr.captureCloneDeadline = 50 * time.Millisecond
	release := make(chan struct{})
	mgr.captureCloneFn = func(dst, src *os.File) error {
		<-release
		return copyClone(dst, src)
	}
	start := time.Now()
	_, err := mgr.CaptureDataImage("vm-c")
	if err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("hung clone: err = %v, want a deadline error", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("hung clone held the call %s", time.Since(start))
	}
	if fc.now() != "Running" {
		t.Fatalf("VM state while the clone hangs = %s, want Running", fc.now())
	}
	close(release)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(capturePath(mgr)); os.IsNotExist(err) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("abandoned clone was never removed")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Review 365 (item 6): an abandoned clone keeps the pause marker until the
// clone returns (a vmctl death in between still finds it), and its cleanup
// removes only the files it made: a newer capture and marker written after
// a recovery stay.
func TestAbandonedCloneCleanupRemovesOnlyItsOwnFiles(t *testing.T) {
	fc := &statefulFC{state: "Running"}
	mgr := captureStatefulVM(t, fc)
	mgr.captureCloneDeadline = 50 * time.Millisecond
	release := make(chan struct{})
	finished := make(chan struct{})
	mgr.captureAbandonedDone = func() { close(finished) }
	mgr.captureCloneFn = func(dst, src *os.File) error {
		<-release
		return copyClone(dst, src)
	}
	if _, err := mgr.CaptureDataImage("vm-c"); err == nil {
		t.Fatal("hung clone returned no error")
	}
	if _, err := os.Stat(pauseMarkerPath(mgr)); err != nil {
		t.Fatalf("pause marker while the abandoned clone runs: %v, want it kept", err)
	}
	// A recovery removed both, and a newer capture made its own.
	for _, p := range []string{pauseMarkerPath(mgr), capturePath(mgr)} {
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("newer"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	close(release)
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("abandoned clone cleanup never ran")
	}
	for _, p := range []string{pauseMarkerPath(mgr), capturePath(mgr)} {
		if raw, err := os.ReadFile(p); err != nil || string(raw) != "newer" {
			t.Fatalf("%s after the abandoned cleanup = %q %v, want the newer file kept", p, raw, err)
		}
	}
}

// Review 366 (DeepSeek): a later capture that writes its marker over one a
// failed unlink left gets a new file, so the abandoned cleanup keeps it.
func TestAbandonedCloneCleanupKeepsAMarkerRewrittenInPlace(t *testing.T) {
	fc := &statefulFC{state: "Running"}
	mgr := captureStatefulVM(t, fc)
	mgr.captureCloneDeadline = 50 * time.Millisecond
	release := make(chan struct{})
	finished := make(chan struct{})
	mgr.captureAbandonedDone = func() { close(finished) }
	mgr.captureCloneFn = func(dst, src *os.File) error {
		<-release
		return copyClone(dst, src)
	}
	if _, err := mgr.CaptureDataImage("vm-c"); err == nil {
		t.Fatal("hung clone returned no error")
	}
	vmDir := filepath.Join(mgr.cfg.StateDir, "vm-c")
	if err := os.Remove(capturePath(mgr)); err != nil {
		t.Fatal(err)
	}
	if err := writeCapturePauseMarker(vmDir, capturePauseMarker{VMID: "vm-c", PausedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	close(release)
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("abandoned clone cleanup never ran")
	}
	if _, err := os.Stat(pauseMarkerPath(mgr)); err != nil {
		t.Fatalf("the later capture's marker after the abandoned cleanup: %v, want it kept", err)
	}
}

// Review 365 (item 6): when the marker cannot be removed the capture is not
// handed out (a marker beside a handed-out capture would let a reattach
// delete it).
func TestCaptureDataImageFailsWhenTheMarkerStays(t *testing.T) {
	fc := &statefulFC{state: "Running"}
	mgr := captureStatefulVM(t, fc)
	mgr.captureCloneFn = func(dst, src *os.File) error {
		// Turn the marker into a directory that os.Remove refuses.
		if err := os.Remove(pauseMarkerPath(mgr)); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(pauseMarkerPath(mgr), "pin"), 0o755); err != nil {
			return err
		}
		return copyClone(dst, src)
	}
	if _, err := mgr.CaptureDataImage("vm-c"); err == nil || !strings.Contains(err.Error(), "pause marker") {
		t.Fatalf("err = %v, want a pause marker error", err)
	}
	if _, err := os.Stat(capturePath(mgr)); !os.IsNotExist(err) {
		t.Fatalf("capture handed out beside a marker: %v", err)
	}
	if fc.now() != "Running" {
		t.Fatalf("VM state = %s, want Running", fc.now())
	}
}

// 11: a resume whose answers all fail on a VM that runs is not an outage;
// the probe decides, and the capture stands.
func TestCaptureDataImageProbesBeforeCallingItPaused(t *testing.T) {
	fc := &statefulFC{state: "Running"}
	mgr := captureStatefulVM(t, fc)
	mgr.captureCloneFn = func(dst, src *os.File) error {
		// The resume lands but every answer is lost: here, the VM runs
		// and every resume is refused.
		fc.mu.Lock()
		fc.state = "Running"
		fc.resumeFails = true
		fc.mu.Unlock()
		return copyClone(dst, src)
	}
	got, err := mgr.CaptureDataImage("vm-c")
	if err != nil {
		t.Fatalf("running VM reported as: %v", err)
	}
	if _, err := os.Stat(got.Path); err != nil {
		t.Fatalf("capture: %v", err)
	}
	if _, err := os.Stat(pauseMarkerPath(mgr)); !os.IsNotExist(err) {
		t.Fatalf("pause marker left on a running VM: %v", err)
	}
}

// 11: a VM the probe shows paused is reported loudly and keeps its marker,
// so a restart of vmctl resumes it.
func TestCaptureDataImageLeftPausedKeepsTheMarker(t *testing.T) {
	fc := &statefulFC{state: "Running", resumeFails: true}
	mgr := captureStatefulVM(t, fc)
	mgr.captureCloneFn = copyClone
	_, err := mgr.CaptureDataImage("vm-c")
	if err == nil || !strings.Contains(err.Error(), "LEFT PAUSED") {
		t.Fatalf("err = %v, want VM LEFT PAUSED", err)
	}
	if _, err := os.Stat(pauseMarkerPath(mgr)); err != nil {
		t.Fatalf("pause marker of a paused VM: %v", err)
	}
	if _, err := os.Stat(capturePath(mgr)); !os.IsNotExist(err) {
		t.Fatalf("capture handed out with the VM paused: %v", err)
	}
}

// 8: at reattach, a marker resumes the VM it names and removes the stale
// capture; resuming continues the same guest process, so it is not a boot.
func TestRecoverCapturePauseResumesAndCleans(t *testing.T) {
	fc := &statefulFC{state: "Paused"}
	mgr := captureStatefulVM(t, fc)
	if err := os.WriteFile(pauseMarkerPath(mgr), []byte(`{"vm_id":"vm-c"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(capturePath(mgr), []byte("torn"), 0o600); err != nil {
		t.Fatal(err)
	}
	mgr.recoverCapturePause("vm-c")
	if fc.now() != "Running" {
		t.Fatalf("VM state after recovery = %s, want Running", fc.now())
	}
	for _, p := range []string{pauseMarkerPath(mgr), capturePath(mgr)} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("%s left after recovery: %v", p, err)
		}
	}
}

// 8: without a marker, recovery touches nothing (a capture in use stays).
func TestRecoverCapturePauseWithoutMarkerIsANoop(t *testing.T) {
	fc := &statefulFC{state: "Running"}
	mgr := captureStatefulVM(t, fc)
	if err := os.WriteFile(capturePath(mgr), []byte("in use"), 0o600); err != nil {
		t.Fatal(err)
	}
	mgr.recoverCapturePause("vm-c")
	fc.mu.Lock()
	calls := len(fc.calls)
	fc.mu.Unlock()
	if calls != 0 {
		t.Fatalf("recovery without a marker called the API %d times", calls)
	}
	if _, err := os.Stat(capturePath(mgr)); err != nil {
		t.Fatalf("capture in use removed: %v", err)
	}
}

// 12: StopVM and a kill-mode hibernate wait for a VM's operation lock, so
// neither runs inside a capture's pause.
func TestStopAndKillHibernateWaitForTheOperationLock(t *testing.T) {
	for name, op := range map[string]func(*Manager) error{
		"stop":      func(m *Manager) error { return m.StopVM("vm-c") },
		"hibernate": func(m *Manager) error { return m.HibernateVM("vm-c") },
	} {
		t.Run(name, func(t *testing.T) {
			mgr := captureTestVM(t, nil)
			unlock := mgr.lockVMOperation("vm-c")
			done := make(chan error, 1)
			go func() { done <- op(mgr) }()
			select {
			case err := <-done:
				unlock()
				t.Fatalf("%s ran while the operation lock was held (err=%v)", name, err)
			case <-time.After(100 * time.Millisecond):
			}
			unlock()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}
			case <-time.After(10 * time.Second):
				t.Fatalf("%s never ran after the lock was released", name)
			}
		})
	}
}
