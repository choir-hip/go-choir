package vmmanager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

// A host capture of a running computer's data image: the first projection
// base of a computer the host cannot replay to (S2 of
// docs/problems/paper-never-checkpointed-2026-10-27.md; owner, 2026-10-27).
//
// The guest has no channel by which the host can hold its appends (the
// guest's HoldAppends is in-process, and the hibernate fence refuses a busy
// guest), so the capture pauses the VM instead. A paused VM's disk is what a
// power cut at that instant would leave: the guest's ext4 journal and Dolt's
// chunk journal are both crash safe, so the clone opens as a store at some
// committed head. That head is checked against the tape off the guest
// (projectionbase.PublishCapturedBase); a hold would only buy a head equal
// to the platform's at capture time, which nothing needs.
//
// The clone is a FICLONE reflink on the host's btrfs: proportional to the
// image's extent count, not its size, and it shares every block until one
// side writes. The pause covers only the clone, which has a deadline.
//
// A VM is never left paused (review 364): a durable marker is written before
// the pause and removed only once the VM is seen running; every pause error
// is followed by a resume; the state is probed before a VM is reported
// paused; and a vmctl that dies inside the pause resumes the VM at reattach
// (recoverCapturePause).

const (
	captureImageName       = "data.img.capture"
	capturePauseMarkerName = "capture-pause.json"
	capturePauseTimeout    = 10 * time.Second
	captureResumeTimeout   = 10 * time.Second
	captureResumeTries     = 3
	// captureCloneTimeout bounds the guest's pause. A reflink of the
	// paper's image (about 56 GiB allocated) is expected in seconds; past
	// this the clone is abandoned and the VM resumed.
	captureCloneTimeout = 30 * time.Second
)

// ErrVMOperationInProgress refuses a capture of a VM another operation
// holds (hibernate, refresh, stop, another capture).
var ErrVMOperationInProgress = errors.New("VM operation in progress")

// DataImageCapture is one capture: the clone's path and how long the guest
// was paused (PausedFor) and the clone took (ClonedFor).
type DataImageCapture struct {
	Path      string
	PausedFor time.Duration
	ClonedFor time.Duration
}

// capturePauseMarker is the durable record that a capture may hold a VM
// paused.
type capturePauseMarker struct {
	VMID     string    `json:"vm_id"`
	Socket   string    `json:"socket"`
	PausedAt time.Time `json:"paused_at"`
}

// CaptureDataImage pauses a running VM, reflinks its data image to
// <state>/<vm>/data.img.capture, and resumes it. An existing capture is
// never overwritten (a processor may still be reading it); the caller
// removes the capture when done. A failed or abandoned clone leaves no
// capture, and a capture is handed out only once the VM is seen running.
func (m *Manager) CaptureDataImage(vmID string) (DataImageCapture, error) {
	unlock, ok := m.tryLockVMOperation(vmID)
	if !ok {
		return DataImageCapture{}, fmt.Errorf("capture data image %s: %w", vmID, ErrVMOperationInProgress)
	}
	defer unlock()
	sock, _, err := m.runningAPISocket(vmID)
	if err != nil {
		return DataImageCapture{}, fmt.Errorf("capture data image: %w", err)
	}
	vmDir := filepath.Join(m.cfg.StateDir, vmID)
	src, err := os.Open(filepath.Join(vmDir, "data.img"))
	if err != nil {
		return DataImageCapture{}, fmt.Errorf("capture data image %s: %w", vmID, err)
	}
	dstPath := filepath.Join(vmDir, captureImageName)
	dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		_ = src.Close()
		if os.IsExist(err) {
			return DataImageCapture{}, fmt.Errorf("capture data image %s: %s exists (a capture is in use or was left behind; remove it first)", vmID, dstPath)
		}
		return DataImageCapture{}, fmt.Errorf("capture data image %s: %w", vmID, err)
	}
	// The discard removes only the file this call made: an abandoned
	// clone's late cleanup must not delete a newer capture made after a
	// recovery removed this one.
	dstInfo, err := dst.Stat()
	if err != nil {
		_ = dst.Close()
		_ = src.Close()
		_ = os.Remove(dstPath)
		return DataImageCapture{}, fmt.Errorf("capture data image %s: %w", vmID, err)
	}
	// The comparison runs while dst is still open: the descriptor pins the
	// inode, so a newer file cannot reuse its number and pass for it.
	discard := func() {
		removeIfSame(dstPath, dstInfo)
		_ = dst.Close()
		_ = src.Close()
	}
	if err := writeCapturePauseMarker(vmDir, capturePauseMarker{VMID: vmID, Socket: sock, PausedAt: time.Now().UTC()}); err != nil {
		discard()
		return DataImageCapture{}, fmt.Errorf("capture data image %s: pause marker: %w", vmID, err)
	}
	// Held open until the cleanup compares it, for the same reason as dst.
	markerFile, markerErr := os.Open(filepath.Join(vmDir, capturePauseMarkerName))
	var markerInfo os.FileInfo
	if markerErr == nil {
		if markerInfo, markerErr = markerFile.Stat(); markerErr != nil {
			_ = markerFile.Close()
			markerFile = nil
		}
	}
	markerHandedOff := false
	defer func() {
		if markerFile != nil && !markerHandedOff {
			_ = markerFile.Close()
		}
	}()

	api := newFCAPIClient(sock)
	clone := m.captureCloneFn
	if clone == nil {
		clone = cloneFile
	}
	pauseCtx, cancel := context.WithTimeout(context.Background(), capturePauseTimeout)
	err = api.Pause(pauseCtx)
	cancel()
	if err != nil {
		// Any error may come from a VM that did pause (a 5xx after the
		// pause, a timeout, a dropped connection): resume regardless.
		discard()
		if resumeErr := confirmResumed(api); resumeErr != nil {
			return DataImageCapture{}, fmt.Errorf("capture data image %s: pause: %v; VM MAY BE LEFT PAUSED: %w (resume by hand: PATCH /vm {\"state\":\"Resumed\"} on %s)", vmID, err, resumeErr, sock)
		}
		if rmErr := removeCapturePauseMarker(vmDir); rmErr != nil {
			log.Printf("vmmanager: data image capture %s: %v", vmID, rmErr)
		}
		return DataImageCapture{}, fmt.Errorf("capture data image %s: pause: %w", vmID, err)
	}
	paused := time.Now()
	deadline := m.captureCloneDeadline
	if deadline <= 0 {
		deadline = captureCloneTimeout
	}
	done := make(chan error, 1)
	go func() { done <- clone(dst, src) }()
	timer := time.NewTimer(deadline)
	var cloneErr error
	abandoned := false
	// resumed tells the abandoned cleanup the VM runs again: a marker of a
	// VM left paused stays for the reattach. The cleanup takes the lock this
	// call holds, so it reads the flag after this call returns.
	var resumed atomic.Bool
	select {
	case cloneErr = <-done:
		timer.Stop()
	case <-timer.C:
		// FICLONE cannot be interrupted: the clone runs on, and the
		// files are released and the torn clone removed when it returns.
		// The marker stays until then, so a vmctl death inside the clone
		// still leaves its record. The cleanup takes the VM's operation
		// lock and removes only the files this call made.
		abandoned = true
		cloneErr = fmt.Errorf("clone past its %s deadline; abandoned", deadline)
		markerHandedOff = true
		go func() {
			<-done
			unlock := m.lockVMOperation(vmID)
			discard()
			if markerErr == nil && resumed.Load() {
				removeIfSame(filepath.Join(vmDir, capturePauseMarkerName), markerInfo)
				_ = syncDirectory(vmDir)
			}
			if markerFile != nil {
				_ = markerFile.Close()
			}
			unlock()
			log.Printf("vmmanager: data image capture %s: removed the abandoned clone", vmID)
			if m.captureAbandonedDone != nil {
				m.captureAbandonedDone()
			}
		}()
	}
	cloned := time.Since(paused)
	resumeErr := confirmResumed(api)
	pausedFor := time.Since(paused)
	log.Printf("vmmanager: data image capture %s: paused %s (clone %s) clone_err=%v resume_err=%v", vmID, pausedFor.Round(time.Millisecond), cloned.Round(time.Millisecond), cloneErr, resumeErr)
	if resumeErr != nil {
		if !abandoned {
			discard()
		}
		return DataImageCapture{}, fmt.Errorf("capture data image %s: VM LEFT PAUSED, resume failed: %w (resume by hand: PATCH /vm {\"state\":\"Resumed\"} on %s; the marker resumes it at vmctl's next reattach)", vmID, resumeErr, sock)
	}
	resumed.Store(true)
	if abandoned {
		return DataImageCapture{}, fmt.Errorf("capture data image %s: reflink: %w", vmID, cloneErr)
	}
	// A marker beside a capture means "never handed out" to a reattach or a
	// boot, which delete the capture: one that stays fails the handoff.
	if err := removeCapturePauseMarker(vmDir); err != nil {
		discard()
		return DataImageCapture{}, fmt.Errorf("capture data image %s: %w; capture discarded", vmID, err)
	}
	if cloneErr != nil {
		discard()
		return DataImageCapture{}, fmt.Errorf("capture data image %s: reflink: %w", vmID, cloneErr)
	}
	// Off the pause: the clone's bytes are fixed once FICLONE returns.
	if err := dst.Sync(); err != nil {
		discard()
		return DataImageCapture{}, fmt.Errorf("capture data image %s: sync clone: %w", vmID, err)
	}
	_ = dst.Close()
	_ = src.Close()
	return DataImageCapture{Path: dstPath, PausedFor: pausedFor, ClonedFor: cloned}, nil
}

// confirmResumed resumes the VM, retrying a refused or timed-out call, and
// when every try fails probes its state: a resume whose answer was lost
// leaves a running VM that a retry's refusal must not call paused.
func confirmResumed(api *fcAPIClient) error {
	err := resumeAfterCapture(api)
	if err == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), captureResumeTimeout)
	state, probeErr := api.State(ctx)
	cancel()
	if probeErr != nil {
		return fmt.Errorf("%v; state probe: %w", err, probeErr)
	}
	if state == "Running" {
		return nil
	}
	return fmt.Errorf("%v; state probe answers %q", err, state)
}

// resumeAfterCapture resumes the VM, retrying a refused or timed-out call: a
// computer left paused is an outage.
func resumeAfterCapture(api *fcAPIClient) error {
	var err error
	for try := 0; try < captureResumeTries; try++ {
		ctx, cancel := context.WithTimeout(context.Background(), captureResumeTimeout)
		err = api.Resume(ctx)
		cancel()
		if err == nil {
			return nil
		}
	}
	return err
}

// writeCapturePauseMarker writes the marker durably (file and directory
// synced) before the pause is sent. It writes a temp file and renames it, so
// every capture's marker is a new inode: an abandoned clone's cleanup, which
// removes only its own marker by inode, never removes a later capture's
// marker written over one a failed unlink left (review 366).
func writeCapturePauseMarker(vmDir string, marker capturePauseMarker) error {
	raw, err := json.Marshal(marker)
	if err != nil {
		return err
	}
	path := filepath.Join(vmDir, capturePauseMarkerName)
	f, err := os.CreateTemp(vmDir, capturePauseMarkerName+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	fail := func(err error) error {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Chmod(0o600); err != nil {
		return fail(err)
	}
	if _, err := f.Write(raw); err != nil {
		return fail(err)
	}
	if err := f.Sync(); err != nil {
		return fail(err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return syncDirectory(vmDir)
}

func removeCapturePauseMarker(vmDir string) error {
	if err := os.Remove(filepath.Join(vmDir, capturePauseMarkerName)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove capture pause marker: %w", err)
	}
	if err := syncDirectory(vmDir); err != nil {
		return fmt.Errorf("remove capture pause marker: sync %s: %w", vmDir, err)
	}
	return nil
}

// removeIfSame removes path only while it is still the file recorded in
// want (same device and inode).
func removeIfSame(path string, want os.FileInfo) {
	got, err := os.Lstat(path)
	if err != nil || !os.SameFile(got, want) {
		return
	}
	_ = os.Remove(path)
}

// recoverCapturePause resumes a VM that a capture held paused when vmctl
// died inside the pause (its marker outlived the capture), then removes the
// marker and the capture, which was never handed out. It runs at reattach.
//
// Resuming a paused VM continues the same guest process from the
// instruction it stopped at: it is not a boot, so the restart rule (a crash
// restart closes open work, CLAUDE.md "Restarts End Work") does not apply.
// The guest sees only a jump of its clock.
func (m *Manager) recoverCapturePause(vmID string) {
	vmDir := filepath.Join(m.cfg.StateDir, vmID)
	if _, err := os.Lstat(filepath.Join(vmDir, capturePauseMarkerName)); err != nil {
		return
	}
	unlock, ok := m.tryLockVMOperation(vmID)
	if !ok {
		log.Printf("vmmanager: capture pause marker of VM %s not recovered: VM operation in progress", vmID)
		return
	}
	defer unlock()
	sock, _, err := m.runningAPISocket(vmID)
	if err != nil {
		log.Printf("vmmanager: capture pause marker of VM %s not recovered: %v", vmID, err)
		return
	}
	if err := confirmResumed(newFCAPIClient(sock)); err != nil {
		log.Printf("vmmanager: VM %s LEFT PAUSED by an interrupted capture; resume failed: %v (resume by hand: PATCH /vm {\"state\":\"Resumed\"} on %s)", vmID, err, sock)
		return
	}
	_ = os.Remove(filepath.Join(vmDir, captureImageName))
	if err := removeCapturePauseMarker(vmDir); err != nil {
		log.Printf("vmmanager: VM %s: %v", vmID, err)
	}
	log.Printf("vmmanager: resumed VM %s after an interrupted data image capture; removed its capture", vmID)
}

// clearCapturePause removes a capture's marker and capture when the VM
// boots: no process of it runs, so nothing is paused, and a capture beside
// a marker was never handed out.
func (m *Manager) clearCapturePause(vmID string) {
	vmDir := filepath.Join(m.cfg.StateDir, vmID)
	if _, err := os.Lstat(filepath.Join(vmDir, capturePauseMarkerName)); err != nil {
		return
	}
	_ = os.Remove(filepath.Join(vmDir, captureImageName))
	if err := removeCapturePauseMarker(vmDir); err != nil {
		log.Printf("vmmanager: VM %s: %v", vmID, err)
	}
	log.Printf("vmmanager: cleared an interrupted data image capture of VM %s at boot", vmID)
}
