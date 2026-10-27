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
// A refusal at any step leaves no advertised base and no published one.
// Run it as root (mount), in its own unit at low priority, with the
// host-internal credential (see the runbook in the problem doc).

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

// errAdvertiseUnknown: the advertisement may have landed (no answer, and no
// readback). The published base is kept, so an advertised base never loses
// its bytes.
var errAdvertiseUnknown = errors.New("advertisement outcome unknown")

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
		if !errors.Is(err, errAdvertiseUnknown) {
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
	if os.Getenv("GOMEMLIMIT") == "" && *memoryLimit > 0 {
		debug.SetMemoryLimit(*memoryLimit << 20)
	}
	if err := ensureDoltHome(os.Getenv); err != nil {
		fmt.Fprintln(os.Stderr, "capture-base:", err)
		return 1
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	client := &http.Client{Timeout: 2 * time.Minute, Transport: internalTransport{}}
	deps := captureDeps{
		capture:    func(ctx context.Context, id string) (captureGrant, error) { return requestCapture(ctx, client, *vmctlURL, id) },
		mount:      loopMount,
		publish:    projectionbase.PublishCapturedBase,
		advertise:  func(ctx context.Context, id string, seq uint64, ref string) error { return advertiseBase(ctx, client, *platformURL, id, seq, ref) },
		retract:    retractPublishedBase,
		tapeHeadAt: func(ctx context.Context, owner, id string, seq uint64) (string, error) { return tapeHeadAt(ctx, *platformURL, owner, id, seq) },
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

type watermarkAnswer struct {
	Sequence uint64 `json:"watermark_sequence"`
	BaseRef  string `json:"base_ref"`
}

// advertiseBase asks corpusd, the watermark's sole writer, to advertise the
// base. corpusd keeps a higher watermark and answers 200 either way, so the
// answer must name this base. On a lost answer the watermark is read back.
func advertiseBase(ctx context.Context, client *http.Client, platformURL, computerID string, sequence uint64, baseRef string) error {
	base := strings.TrimRight(platformURL, "/")
	var got watermarkAnswer
	status, err := postJSON(ctx, client, base+"/internal/computers/files/watermark", map[string]any{
		"computer_id": computerID, "watermark_sequence": sequence, "base_ref": baseRef,
	}, &got)
	if err != nil && status == 0 {
		req, rerr := http.NewRequestWithContext(ctx, http.MethodGet, base+"/internal/computers/files/watermark?"+url.Values{"computer_id": {computerID}}.Encode(), nil)
		if rerr != nil {
			return fmt.Errorf("%w: %v", errAdvertiseUnknown, err)
		}
		resp, rerr := client.Do(req)
		if rerr != nil {
			return fmt.Errorf("%w: %v; readback: %v", errAdvertiseUnknown, err, rerr)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&got) != nil {
			return fmt.Errorf("%w: %v; readback status %d", errAdvertiseUnknown, err, resp.StatusCode)
		}
		err = nil
	}
	if err != nil {
		return err
	}
	if got.Sequence != sequence || got.BaseRef != baseRef {
		return fmt.Errorf("corpusd advertises %d (%s), not this base %d (%s)", got.Sequence, got.BaseRef, sequence, baseRef)
	}
	return nil
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

// loopMount mounts image read-write on dir (the ext4 journal of a capture
// taken from a running guest is replayed at mount; the capture is
// disposable). The returned function unmounts it.
func loopMount(ctx context.Context, image, dir string) (func() error, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if out, err := exec.CommandContext(ctx, "mount", "-o", "loop", image, dir).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("mount -o loop %s: %v: %s", image, err, strings.TrimSpace(string(out)))
	}
	return func() error {
		if out, err := exec.Command("umount", dir).CombinedOutput(); err != nil {
			return fmt.Errorf("umount %s: %v: %s", dir, err, strings.TrimSpace(string(out)))
		}
		return os.Remove(dir)
	}, nil
}
