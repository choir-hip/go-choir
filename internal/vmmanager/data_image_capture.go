package vmmanager

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
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
// side writes. The pause covers only the clone.

const (
	captureImageName     = "data.img.capture"
	capturePauseTimeout  = 10 * time.Second
	captureResumeTimeout = 10 * time.Second
	captureResumeTries   = 3
)

// DataImageCapture is one capture: the clone's path and how long the guest
// was paused (PausedFor) and the clone took (ClonedFor).
type DataImageCapture struct {
	Path      string
	PausedFor time.Duration
	ClonedFor time.Duration
}

// CaptureDataImage pauses a running VM, reflinks its data image to
// <state>/<vm>/data.img.capture, and resumes it. An existing capture is
// never overwritten (a processor may still be reading it); the caller
// removes the capture when done. A failed clone leaves no capture. The VM is
// resumed on every path after a successful pause.
func (m *Manager) CaptureDataImage(vmID string) (DataImageCapture, error) {
	unlock, ok := m.tryLockVMOperation(vmID)
	if !ok {
		return DataImageCapture{}, fmt.Errorf("capture data image %s: VM operation in progress", vmID)
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
	defer src.Close()
	dstPath := filepath.Join(vmDir, captureImageName)
	dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return DataImageCapture{}, fmt.Errorf("capture data image %s: %s exists (a capture is in use or was left behind; remove it first)", vmID, dstPath)
		}
		return DataImageCapture{}, fmt.Errorf("capture data image %s: %w", vmID, err)
	}
	keep := false
	defer func() {
		_ = dst.Close()
		if !keep {
			_ = os.Remove(dstPath)
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
		// Firecracker's own refusal means the VM was not paused. Any other
		// error (a timeout, a dropped connection) may have paused it.
		var refused *fcAPIError
		if !errors.As(err, &refused) {
			if resumeErr := resumeAfterCapture(api); resumeErr != nil {
				log.Printf("vmmanager: data image capture %s: resume after an unanswered pause: %v", vmID, resumeErr)
			}
		}
		return DataImageCapture{}, fmt.Errorf("capture data image %s: pause: %w", vmID, err)
	}
	paused := time.Now()
	cloneErr := clone(dst, src)
	cloned := time.Since(paused)
	resumeErr := resumeAfterCapture(api)
	pausedFor := time.Since(paused)
	log.Printf("vmmanager: data image capture %s: paused %s (clone %s) clone_err=%v resume_err=%v", vmID, pausedFor.Round(time.Millisecond), cloned.Round(time.Millisecond), cloneErr, resumeErr)
	if resumeErr != nil {
		return DataImageCapture{}, fmt.Errorf("capture data image %s: VM LEFT PAUSED, resume failed: %w (resume by hand: PATCH /vm {\"state\":\"Resumed\"} on %s)", vmID, resumeErr, sock)
	}
	if cloneErr != nil {
		return DataImageCapture{}, fmt.Errorf("capture data image %s: reflink: %w", vmID, cloneErr)
	}
	keep = true
	return DataImageCapture{Path: dstPath, PausedFor: pausedFor, ClonedFor: cloned}, nil
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
