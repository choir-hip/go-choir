package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/projectionbase"
)

// `checkpointd capture-base -computer <id>`: an operator job that publishes
// a computer's projection base from a capture of its running data image
// instead of a replay of its tape (S2 of
// docs/problems/paper-never-checkpointed-2026-10-27.md; owner, 2026-10-27).
// One named computer per run; nothing schedules it.
//
//  1. vmctl pauses the VM, reflinks data.img to data.img.capture, resumes
//     (POST /internal/vmctl/capture-data-image);
//  2. this process loop-mounts the capture and publishes the store on it
//     (projectionbase.PublishCapturedBase: head checked against the tape,
//     compacted, witnessed, packed, read back), marked live_captured;
//  3. corpusd advertises it as the computer's watermark, after its own check
//     that the base's head is the tape's at that sequence;
//  4. the capture is unmounted and deleted on every path.
//
// A refusal at any step leaves no advertised base and no published one; an
// advertisement whose outcome is uncertain keeps the published base.
// The base is an epoch root (owner, 2026-10-27): replays start from it, and
// the tape below it is not replayed again. Run it with CAP_SYS_ADMIN
// (mount), in its own transient unit at low priority, with the host-internal
// credential: the runbook is the "S2 runbook" section of
// docs/problems/paper-never-checkpointed-2026-10-27.md.

// captureGrant is vmctl's answer to a capture request.
type captureGrant struct {
	VMID        string `json:"vm_id"`
	ComputerID  string `json:"computer_id"`
	UserID      string `json:"user_id"`
	CapturePath string `json:"capture_path"`
	PausedMS    int64  `json:"paused_ms"`
	ClonedMS    int64  `json:"cloned_ms"`
}

// captureDeps are the job's effects, one function each.
type captureDeps struct {
	// precheck returns the computer's advertised watermark, refusing when
	// the tape is not above it: no store head could exceed it, so a pause
	// would be for nothing (review 365).
	precheck   func(ctx context.Context, computerID string) (uint64, error)
	capture    func(ctx context.Context, computerID string) (captureGrant, error)
	mount      func(ctx context.Context, image, dir string) (unmount func() error, err error)
	publish    func(ctx context.Context, cfg projectionbase.CaptureConfig) (*projectionbase.Result, error)
	advertise  func(ctx context.Context, computerID string, sequence uint64, baseRef string) error
	retract    func(artifactsRoot, baseRef string) error
	tapeHeadAt func(ctx context.Context, ownerID, computerID string, sequence uint64) (string, error)
}

type captureOptions struct {
	ComputerID    string
	ArtifactsRoot string
	MountRoot     string
	MinFreeBytes  uint64
	Timeout       time.Duration
}

// captureReport is the job's receipt, printed as one JSON line.
type captureReport struct {
	ComputerID    string `json:"computer_id"`
	VMID          string `json:"vm_id,omitempty"`
	CapturePath   string `json:"capture_path,omitempty"`
	PausedMS      int64  `json:"paused_ms"`
	ClonedMS      int64  `json:"cloned_ms"`
	Sequence      uint64 `json:"sequence,omitempty"`
	CanonicalHead string `json:"canonical_head,omitempty"`
	BaseRef       string `json:"base_ref,omitempty"`
	BlobBytes     int64  `json:"blob_bytes,omitempty"`
	Origin        string `json:"origin,omitempty"`
	Advertised    bool   `json:"advertised"`
	Duration      string `json:"duration"`
	Error         string `json:"error,omitempty"`
}

// errAdvertiseUnknown: the advertisement may have landed (the readback
// failed). The published base is kept, so an advertised base never loses
// its bytes.
var errAdvertiseUnknown = errors.New("advertisement outcome unknown")

// errAdvertiseRefused: the readback shows the base is not advertised and
// cannot be (no watermark, a lower one, or another base at its sequence).
// Only this retracts the published base.
var errAdvertiseRefused = errors.New("advertisement refused")

// captureLockName is the run lock under the mount root: one capture-base
// at a time, and the sweep runs only under it.
const captureLockName = ".capture-base.lock"

// captureSweep is the start of a run (review 364): hold the run lock, then
// remove what a killed run left. isMount and unmount are seams.
type captureSweep struct {
	isMount func(dir string) bool
	unmount func(dir string) error
}

// begin takes the run lock (refusing when another run holds it) and sweeps:
// every mount under mountRoot is unmounted and its empty mount point
// removed, and every <stateDir>/<vm>/data.img.capture without a vmctl pause
// marker beside it is removed (with a marker, vmctl is inside the pause or
// resumes the VM and removes it at reattach). Only this job creates either.
// The returned function releases the lock.
func (s captureSweep) begin(mountRoot, stateDir string) (func(), error) {
	if err := os.MkdirAll(mountRoot, 0o700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(filepath.Join(mountRoot, captureLockName), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lock.Close()
		return nil, fmt.Errorf("another capture-base run holds %s: %w", lock.Name(), err)
	}
	release := func() {
		_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
		_ = lock.Close()
	}
	entries, err := os.ReadDir(mountRoot)
	if err != nil {
		release()
		return nil, err
	}
	for _, e := range entries {
		dir := filepath.Join(mountRoot, e.Name())
		if !e.IsDir() {
			continue
		}
		if s.isMount(dir) {
			if err := s.unmount(dir); err != nil {
				release()
				return nil, fmt.Errorf("sweep stale capture mount %s: %w", dir, err)
			}
			log.Printf("capture-base: unmounted a stale capture mount %s", dir)
		}
		// Remove only an empty mount point; a directory with files is not ours.
		if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
			log.Printf("capture-base: left %s: %v", dir, err)
		}
	}
	vms, err := os.ReadDir(stateDir)
	if err != nil && !os.IsNotExist(err) {
		release()
		return nil, err
	}
	for _, vm := range vms {
		if !vm.IsDir() {
			continue
		}
		vmDir := filepath.Join(stateDir, vm.Name())
		if _, err := os.Lstat(filepath.Join(vmDir, "capture-pause.json")); err == nil {
			continue
		}
		stale := filepath.Join(vmDir, "data.img.capture")
		if err := os.Remove(stale); err == nil {
			log.Printf("capture-base: removed a stale capture %s", stale)
		} else if !os.IsNotExist(err) {
			release()
			return nil, fmt.Errorf("sweep stale capture %s: %w", stale, err)
		}
	}
	return release, nil
}

// isMountPoint reports whether dir is on another device than its parent.
func isMountPoint(dir string) bool {
	var self, parent syscall.Stat_t
	if syscall.Lstat(dir, &self) != nil || syscall.Lstat(filepath.Dir(dir), &parent) != nil {
		return false
	}
	return self.Dev != parent.Dev
}

func umountDir(dir string) error {
	if out, err := exec.Command("umount", dir).CombinedOutput(); err != nil {
		return fmt.Errorf("umount %s: %v: %s", dir, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// loopMountArgs mounts the guest's image so nothing on it can run, carry a
// set-id bit, or open a device node.
func loopMountArgs(image, dir string) []string {
	return []string{"-o", "loop,nosuid,nodev,noexec", image, dir}
}

func runCaptureBase(ctx context.Context, deps captureDeps, opts captureOptions) (report captureReport, err error) {
	started := time.Now()
	report.ComputerID = opts.ComputerID
	defer func() {
		report.Duration = time.Since(started).Round(time.Second).String()
		if err != nil {
			report.Error = err.Error()
		}
	}()
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}
	watermark, err := deps.precheck(ctx, opts.ComputerID)
	if err != nil {
		return report, fmt.Errorf("precheck (nothing paused): %w", err)
	}
	grant, err := deps.capture(ctx, opts.ComputerID)
	if err != nil {
		return report, fmt.Errorf("capture: %w", err)
	}
	report.VMID, report.CapturePath, report.PausedMS, report.ClonedMS = grant.VMID, grant.CapturePath, grant.PausedMS, grant.ClonedMS
	log.Printf("capture-base computer=%s vm=%s paused_ms=%d cloned_ms=%d capture=%s", opts.ComputerID, grant.VMID, grant.PausedMS, grant.ClonedMS, grant.CapturePath)
	defer func() {
		if rmErr := os.Remove(grant.CapturePath); rmErr != nil && !os.IsNotExist(rmErr) {
			log.Printf("capture-base: remove capture %s: %v", grant.CapturePath, rmErr)
		}
	}()
	mountDir := filepath.Join(opts.MountRoot, grant.VMID)
	unmount, err := deps.mount(ctx, grant.CapturePath, mountDir)
	if err != nil {
		return report, fmt.Errorf("mount capture: %w", err)
	}
	defer func() {
		if umErr := unmount(); umErr != nil {
			log.Printf("capture-base: unmount %s: %v", mountDir, umErr)
			if err == nil {
				err = fmt.Errorf("unmount capture: %w", umErr)
			}
		}
	}()
	result, err := deps.publish(ctx, projectionbase.CaptureConfig{
		ComputerID:    opts.ComputerID,
		StoreMarker:   filepath.Join(mountDir, "state"),
		ArtifactsRoot: opts.ArtifactsRoot,
		CaptureFS:     filepath.Dir(grant.CapturePath),
		AboveSequence: watermark,
		MinFreeBytes:  opts.MinFreeBytes,
		TapeHeadAt: func(ctx context.Context, computerID string, sequence uint64) (string, error) {
			return deps.tapeHeadAt(ctx, grant.UserID, computerID, sequence)
		},
	})
	if err != nil {
		return report, fmt.Errorf("publish: %w", err)
	}
	d := result.Descriptor
	report.Sequence, report.CanonicalHead, report.BaseRef, report.BlobBytes, report.Origin = d.Sequence, d.CanonicalHead, d.BlobSHA256, d.BlobSizeBytes, d.Origin
	if err := deps.advertise(ctx, opts.ComputerID, d.Sequence, d.BlobSHA256); err != nil {
		// Only a definitive refusal retracts: an advertised base never
		// loses its bytes, so anything less certain keeps it.
		if errors.Is(err, errAdvertiseRefused) {
			if rerr := deps.retract(opts.ArtifactsRoot, d.BlobSHA256); rerr != nil {
				log.Printf("capture-base: retract %s: %v", d.BlobSHA256, rerr)
			}
		}
		return report, fmt.Errorf("advertise: %w", err)
	}
	report.Advertised = true
	return report, nil
}

// retractPublishedBase removes a published, unadvertised base: its blob and
// its descriptor sidecar.
func retractPublishedBase(artifactsRoot, baseRef string) error {
	if !computerevent.IsSHA256(baseRef) {
		return fmt.Errorf("retract: base ref must be lowercase SHA-256")
	}
	dir := filepath.Join(filepath.Clean(artifactsRoot), "sha256", projectionbase.Namespace)
	var errs []error
	for _, name := range []string{projectionbase.DescriptorSidecarName(baseRef), baseRef} {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// captureBaseMain is `checkpointd capture-base`.
func captureBaseMain(args []string) int {
	fs := flag.NewFlagSet("capture-base", flag.ContinueOnError)
	computerID := fs.String("computer", "", "computer to capture (computer-...)")
	artifactsRoot := fs.String("artifacts-root", "/var/lib/go-choir/platform-artifacts", "platform artifacts root")
	mountRoot := fs.String("mount-root", "/var/lib/go-choir/capture-base", "where the capture is mounted")
	stateDir := fs.String("state-dir", "/var/lib/go-choir/vm-state", "vmctl's VM state directory (stale captures are swept from it)")
	vmctlURL := fs.String("vmctl-url", "http://127.0.0.1:8083", "vmctl internal URL")
	platformURL := fs.String("platform-url", "http://127.0.0.1:8086", "corpusd internal URL")
	minFreeGiB := fs.Uint64("min-free-gib", 24, "free space the artifacts filesystem keeps after each large write")
	memoryLimit := fs.Int64("memory-limit-mib", 8192, "Go soft memory limit when GOMEMLIMIT is unset; run under a MemoryMax without swap")
	timeout := fs.Duration("timeout", 3*time.Hour, "deadline for the whole job")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*computerID) == "" {
		fmt.Fprintln(os.Stderr, "capture-base: -computer is required")
		return 2
	}
	if strings.TrimSpace(*stateDir) == "" {
		fmt.Fprintln(os.Stderr, "capture-base: -state-dir is required (the stale-capture sweep reads it)")
		return 2
	}
	if os.Getenv("GOMEMLIMIT") == "" && *memoryLimit > 0 {
		debug.SetMemoryLimit(*memoryLimit << 20)
	}
	if err := ensureDoltHome(os.Getenv); err != nil {
		fmt.Fprintln(os.Stderr, "capture-base:", err)
		return 1
	}
	release, err := captureSweep{isMount: isMountPoint, unmount: umountDir}.begin(*mountRoot, *stateDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capture-base:", err)
		return 1
	}
	defer release()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	client := &http.Client{Timeout: 2 * time.Minute, Transport: internalTransport{}}
	deps := captureDeps{
		precheck: func(ctx context.Context, id string) (uint64, error) {
			return precheckWatermark(ctx, client, *vmctlURL, *platformURL, id)
		},
		capture: func(ctx context.Context, id string) (captureGrant, error) {
			return requestCapture(ctx, client, *vmctlURL, id)
		},
		mount:   loopMount,
		publish: projectionbase.PublishCapturedBase,
		advertise: func(ctx context.Context, id string, seq uint64, ref string) error {
			return advertiseBase(ctx, client, *platformURL, id, seq, ref)
		},
		retract: retractPublishedBase,
		tapeHeadAt: func(ctx context.Context, owner, id string, seq uint64) (string, error) {
			return tapeHeadAt(ctx, *platformURL, owner, id, seq)
		},
	}
	report, err := runCaptureBase(ctx, deps, captureOptions{
		ComputerID: strings.TrimSpace(*computerID), ArtifactsRoot: *artifactsRoot, MountRoot: *mountRoot,
		MinFreeBytes: *minFreeGiB << 30, Timeout: *timeout,
	})
	_ = json.NewEncoder(os.Stdout).Encode(report)
	if err != nil {
		log.Printf("capture-base: %v", err)
		return 1
	}
	return 0
}

// internalTransport marks every request host-internal (the marker header,
// and the credential on a loopback service port).
type internalTransport struct{ owner string }

func (t internalTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	hostauth.MarkInternal(req)
	if t.owner != "" {
		req.Header.Set("X-Authenticated-User", t.owner)
	}
	return http.DefaultTransport.RoundTrip(req)
}

func postJSON(ctx context.Context, client *http.Client, endpoint string, body any, out any) (int, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Choir-Lifecycle-Caller", "checkpointd-capture-base")
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	answer, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, fmt.Errorf("%s: status %d: %s", endpoint, resp.StatusCode, strings.TrimSpace(string(answer)))
	}
	if out != nil {
		if err := json.Unmarshal(answer, out); err != nil {
			return resp.StatusCode, fmt.Errorf("%s: decode: %w", endpoint, err)
		}
	}
	return resp.StatusCode, nil
}

func requestCapture(ctx context.Context, client *http.Client, vmctlURL, computerID string) (captureGrant, error) {
	var grant captureGrant
	_, err := postJSON(ctx, client, strings.TrimRight(vmctlURL, "/")+"/internal/vmctl/capture-data-image", map[string]string{"computer_id": computerID}, &grant)
	if err != nil {
		return grant, err
	}
	if grant.ComputerID != computerID || grant.CapturePath == "" || grant.VMID == "" {
		return grant, fmt.Errorf("vmctl answered an incomplete capture: %+v", grant)
	}
	return grant, nil
}

// precheckWatermark asks vmctl, without pausing, for the computer's owner
// (a dry run), reads the tape head as that owner and the advertised
// watermark, and refuses when the head is not above the watermark: no
// captured store head could exceed it. It returns the watermark (0: none).
func precheckWatermark(ctx context.Context, client *http.Client, vmctlURL, platformURL, computerID string) (uint64, error) {
	var who captureGrant
	if _, err := postJSON(ctx, client, strings.TrimRight(vmctlURL, "/")+"/internal/vmctl/capture-data-image", map[string]any{"computer_id": computerID, "dry_run": true}, &who); err != nil {
		return 0, err
	}
	if who.CapturePath != "" {
		return 0, fmt.Errorf("vmctl did not honor dry_run: it captured %s (deploy vmctl with checkpointd; the next run sweeps the capture)", who.CapturePath)
	}
	if who.ComputerID != computerID || who.UserID == "" {
		return 0, fmt.Errorf("vmctl answered an incomplete dry run: %+v", who)
	}
	events, err := computerevent.NewHTTPClient(platformURL, &http.Client{Timeout: time.Minute, Transport: internalTransport{owner: who.UserID}},
		func(context.Context) (string, error) { return "internal", nil }, true)
	if err != nil {
		return 0, err
	}
	head, err := events.Head(ctx, computerID)
	if err != nil {
		return 0, fmt.Errorf("tape head: %w", err)
	}
	if head == nil {
		return 0, fmt.Errorf("computer %s has no tape", computerID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(platformURL, "/")+"/internal/computers/files/watermark?"+url.Values{"computer_id": {computerID}}.Encode(), nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("watermark: %w", err)
	}
	defer resp.Body.Close()
	var wm watermarkAnswer
	switch {
	case resp.StatusCode == http.StatusNotFound:
	case resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&wm) != nil:
		return 0, fmt.Errorf("watermark: status %d", resp.StatusCode)
	}
	if head.Sequence <= wm.Sequence {
		return 0, fmt.Errorf("tape head %d is not above the advertised watermark %d (%s): a capture could not become the root; nothing to do", head.Sequence, wm.Sequence, wm.BaseRef)
	}
	return wm.Sequence, nil
}

type watermarkAnswer struct {
	Sequence uint64 `json:"watermark_sequence"`
	BaseRef  string `json:"base_ref"`
}

// advertiseBase asks corpusd, the watermark's sole writer, to advertise the
// base. Only a clean answer naming this base is success without a second
// look; on any other answer (an error status, an undecodable body, another
// watermark) the watermark is read back (review 364):
//   - it names this base: advertised;
//   - it is absent (404), lower, higher (corpusd records only a higher
//     sequence, so this base never will be), or another base at this
//     sequence: errAdvertiseRefused, and the caller retracts the base;
//   - the readback fails: errAdvertiseUnknown, and the base is kept.
func advertiseBase(ctx context.Context, client *http.Client, platformURL, computerID string, sequence uint64, baseRef string) error {
	base := strings.TrimRight(platformURL, "/")
	var got watermarkAnswer
	_, err := postJSON(ctx, client, base+"/internal/computers/files/watermark", map[string]any{
		"computer_id": computerID, "watermark_sequence": sequence, "base_ref": baseRef,
	}, &got)
	if err == nil && got.Sequence == sequence && got.BaseRef == baseRef {
		return nil
	}
	answer := err
	if answer == nil {
		answer = fmt.Errorf("corpusd answered watermark %d (%s)", got.Sequence, got.BaseRef)
	}
	req, rerr := http.NewRequestWithContext(ctx, http.MethodGet, base+"/internal/computers/files/watermark?"+url.Values{"computer_id": {computerID}}.Encode(), nil)
	if rerr != nil {
		return fmt.Errorf("%w: %v; readback: %v", errAdvertiseUnknown, answer, rerr)
	}
	resp, rerr := client.Do(req)
	if rerr != nil {
		return fmt.Errorf("%w: %v; readback: %v", errAdvertiseUnknown, answer, rerr)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w: %v; readback: no watermark", errAdvertiseRefused, answer)
	}
	var back watermarkAnswer
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&back) != nil {
		return fmt.Errorf("%w: %v; readback status %d", errAdvertiseUnknown, answer, resp.StatusCode)
	}
	switch {
	case back.Sequence == sequence && back.BaseRef == baseRef:
		return nil
	case back.Sequence > sequence:
		// corpusd records only a higher sequence (file_cas_http.go), so this
		// base will never be advertised. A kept live_captured sidecar would
		// still raise the host rebuilder's floor and wedge the computer's
		// jobs (review 366, Devin 1): retract it.
		return fmt.Errorf("%w: corpusd advertises a higher watermark %d (%s), not this base %d (%s): %v", errAdvertiseRefused, back.Sequence, back.BaseRef, sequence, baseRef, answer)
	default:
		return fmt.Errorf("%w: corpusd advertises %d (%s), not this base %d (%s): %v", errAdvertiseRefused, back.Sequence, back.BaseRef, sequence, baseRef, answer)
	}
}

// tapeHeadAt reads the computer's event at sequence from corpusd's replay
// endpoint and returns its digest, the canonical head at that sequence.
func tapeHeadAt(ctx context.Context, platformURL, ownerID, computerID string, sequence uint64) (string, error) {
	if sequence == 0 {
		return "", fmt.Errorf("sequence must be positive")
	}
	c, err := computerevent.NewHTTPClient(platformURL, &http.Client{Timeout: time.Minute, Transport: internalTransport{owner: ownerID}},
		func(context.Context) (string, error) { return "internal", nil }, true)
	if err != nil {
		return "", err
	}
	page, err := c.EventsPage(ctx, computerID, sequence-1, 1)
	if err != nil {
		return "", err
	}
	if len(page) != 1 || page[0].Request.Event.Sequence != sequence || page[0].Request.Event.ComputerID != computerID {
		return "", fmt.Errorf("tape has no event %d for %s", sequence, computerID)
	}
	return page[0].Request.EventDigest, nil
}

// loopMount mounts image read-write on dir, nosuid,nodev,noexec (the ext4
// journal of a capture taken from a running guest is replayed at mount; the
// capture is disposable). The returned function unmounts it.
func loopMount(ctx context.Context, image, dir string) (func() error, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if out, err := exec.CommandContext(ctx, "mount", loopMountArgs(image, dir)...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("mount %s: %v: %s", image, err, strings.TrimSpace(string(out)))
	}
	return func() error {
		if err := umountDir(dir); err != nil {
			return err
		}
		return os.Remove(dir)
	}, nil
}
