// Boot timeline recorder (S0a). One record per bootVM invocation captures the
// host-observed boundaries of a Firecracker boot (spawn, tap/network setup,
// process start, first HTTP response, replay detection, first healthy), the
// guest-side receipt fetched from the guest's own /internal/boot/timeline
// endpoint once healthy, and the boot's identity material (kernel/initrd/store
// image paths, memory, epoch, kind). The merged record is persisted to
// <StateDir>/<vmid>/boot-timeline.json so it survives a vmctl restart and can
// be read back through /internal/vmctl/boot-timeline.
package vmmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// BootTimelineMark is one host-observed boundary in a VM boot.
type BootTimelineMark struct {
	Phase    string `json:"phase"`
	At       string `json:"at"`        // RFC3339Nano
	OffsetMS int64  `json:"offset_ms"` // ms since the boot attempt began
	Detail   string `json:"detail,omitempty"`
}

// BootTimeline is the per-boot receipt. It is append-only while the boot is
// in flight and written to disk when the boot reaches a terminal observation
// (healthy or failed).
type BootTimeline struct {
	mu sync.Mutex

	SchemaVersion int    `json:"schema_version"`
	Kind          string `json:"kind"` // "vm_boot_timeline"
	VMID          string `json:"vm_id"`
	ComputerID    string `json:"computer_id,omitempty"`
	BootKind      string `json:"boot_kind"` // cold | refresh | recover | resume
	Epoch         int64  `json:"epoch"`

	// Host-side marks, ordered.
	Marks []BootTimelineMark `json:"marks"`

	// Identity of what was booted.
	KernelImagePath   string `json:"kernel_image_path,omitempty"`
	InitrdPath        string `json:"initrd_path,omitempty"`
	StoreDiskPath     string `json:"store_disk_path,omitempty"`
	DataImagePath     string `json:"data_image_path,omitempty"`
	DataImageBytes    int64  `json:"data_image_bytes,omitempty"`
	DataImageAlloc    int64  `json:"data_image_allocated_bytes,omitempty"`
	MemSizeMiB        int    `json:"mem_size_mib,omitempty"`
	VCPUCount         int    `json:"vcpu_count,omitempty"`
	GuestIP           string `json:"guest_ip,omitempty"`
	HostURL           string `json:"host_url,omitempty"`
	FirecrackerBin    string `json:"firecracker_bin,omitempty"`
	FirecrackerVer    string `json:"firecracker_version,omitempty"`
	HostKernelRelease string `json:"host_kernel_release,omitempty"`

	// Guest receipt: the guest's own /internal/boot/timeline payload, fetched
	// once the guest reports healthy. Kept verbatim so guest-side attribution
	// (systemd units, journal, in-process marks) is not re-mapped host-side.
	GuestReceipt    json.RawMessage `json:"guest_receipt,omitempty"`
	GuestFetchError string          `json:"guest_fetch_error,omitempty"`

	// Terminal state.
	Outcome string `json:"outcome"` // healthy | failed
	Error   string `json:"error,omitempty"`
	// Refusal carries the guest planner's typed refusal witness when the boot
	// failed because materialization was refused. It is the structured copy of
	// the failure; Error remains the human string.
	Refusal   *GuestBootRefusal `json:"refusal,omitempty"`
	StartedAt string            `json:"started_at"`
	EndedAt   string            `json:"ended_at,omitempty"`
	TotalMS   int64             `json:"total_ms,omitempty"`

	// Caller-set summary the receipts key off.
	FirstHealthyOffsetMS int64 `json:"first_healthy_offset_ms,omitempty"`

	start time.Time
}

// bootTimelineJSON is the marshaling view of BootTimeline: identical layout,
// no methods, so encoding/json walks the fields directly instead of
// re-entering MarshalJSON. Pointer conversion only — never copy a struct that
// carries a mutex.
type bootTimelineJSON BootTimeline

// MarshalJSON serializes under t.mu so readers of an in-flight receipt
// (HandleBootTimeline marshals the live record) never race with append marks
// or the guest-receipt write.
func (t *BootTimeline) MarshalJSON() ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return json.Marshal((*bootTimelineJSON)(t))
}

// newBootTimeline begins a record for one boot attempt.
func newBootTimeline(vmID, bootKind string) *BootTimeline {
	now := time.Now()
	return &BootTimeline{
		SchemaVersion: 1,
		Kind:          "vm_boot_timeline",
		VMID:          vmID,
		BootKind:      bootKind,
		StartedAt:     now.UTC().Format(time.RFC3339Nano),
		start:         now,
	}
}

// mark records one boundary.
func (t *BootTimeline) mark(phase string) {
	t.markDetail(phase, "")
}

func (t *BootTimeline) markDetail(phase, detail string) {
	if t == nil {
		return
	}
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Marks = append(t.Marks, BootTimelineMark{
		Phase:    phase,
		At:       now.UTC().Format(time.RFC3339Nano),
		OffsetMS: now.Sub(t.start).Milliseconds(),
		Detail:   detail,
	})
}

// finish records the terminal outcome.
func (t *BootTimeline) finish(outcome string, err error) {
	if t == nil {
		return
	}
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Outcome = outcome
	if err != nil {
		t.Error = err.Error()
	}
	t.EndedAt = now.UTC().Format(time.RFC3339Nano)
	t.TotalMS = now.Sub(t.start).Milliseconds()
}

// firstHealthy records the first-healthy offset.
func (t *BootTimeline) firstHealthy() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.FirstHealthyOffsetMS = time.Since(t.start).Milliseconds()
}

// runCapture executes a read-only probe command with a hard timeout.
func runCapture(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// firecrackerBuildVersion returns the running firecracker binary's version,
// read once from the binary itself (the pinned package path encodes the
// upstream version already, but the binary's own banner is the authoritative
// record for the S0a identity field).
func firecrackerBuildVersion(bin string) string {
	if bin == "" {
		bin = "firecracker"
	}
	out, err := runCapture(3*time.Second, bin, "--version")
	if err != nil {
		return ""
	}
	return firstLine(out)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// hostKernelRelease returns the host kernel release for the receipt's
// hypervisor/kernel pairing.
func hostKernelRelease() string {
	out, err := runCapture(3*time.Second, "uname", "-r")
	if err != nil {
		return ""
	}
	return firstLine(out)
}

// fetchGuestBootTimeline retrieves the guest's own boot receipt once the
// guest is healthy. A fetch failure is recorded, not fatal — the host-side
// marks still stand.
func (t *BootTimeline) fetchGuestBootTimeline(hostURL string) {
	if t == nil || hostURL == "" {
		return
	}
	t.mark("guest_receipt_fetch_begin")
	client := &http.Client{Timeout: 15 * time.Second}
	var lastErr string
	// The guest can answer /health before its earliest window closes — the
	// first post-healthy request observed a connection EOF (listener at the
	// boundary). Retry briefly so a transient startup EOF does not lose the
	// receipt; a persistent failure still lands in guest_fetch_error.
	// After the first successful fetch, keep polling until the receipt's
	// marks include runtime_started (post-rt.Start), so the receipt covers
	// past the health flip — bounded to 15s so a wedged runtime start is
	// recorded as-is rather than waited on forever.
	const maxAttempts = 25
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(800 * time.Millisecond)
		}
		req, err := http.NewRequest(http.MethodGet, hostURL+"/internal/boot/timeline", nil)
		if err != nil {
			lastErr = err.Error()
			break
		}
		req.Header.Set("X-Internal-Caller", "true")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err.Error()
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Sprintf("read body: %v", readErr)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Sprintf("guest returned %d", resp.StatusCode)
			continue
		}
		// Keep the payload verbatim so guest-side attribution is unmodified.
		if !json.Valid(body) {
			lastErr = "guest receipt is not valid JSON"
			continue
		}
		t.mu.Lock()
		t.GuestReceipt = json.RawMessage(body)
		t.mu.Unlock()
		if guestMarkPresent(body, "runtime_started") {
			t.mark("guest_receipt_fetch_done")
			return
		}
		if attempt == 0 {
			// First healthy fetch landed before runtime_started; mark it so the
			// later polls read as waiting-for-runtime-start, not retry noise.
			t.mark("guest_receipt_awaiting_runtime_started")
		}
	}
	// Exit loop on cap: the last successful body is already stored above.
	// Read GuestReceipt under the lock, release, then mark — markDetail
	// re-acquires t.mu, so calling mark inside the critical section is a
	// self-deadlock (docs/problems/s0-fetch-guest-timeline-deadlock-2026-10-01.md).
	t.mu.Lock()
	hasReceipt := t.GuestReceipt != nil
	if !hasReceipt {
		t.GuestFetchError = lastErr
	}
	t.mu.Unlock()
	if hasReceipt {
		t.mark("guest_receipt_fetch_done_partial") // no runtime_started inside window
	}
}

// guestMarkPresent reports whether the receipt JSON marks array contains the
// named phase. The payload is small; a string scan is sufficient and keeps the
// verbatim body unparsed.
func guestMarkPresent(body []byte, phase string) bool {
	return bytes.Contains(body, []byte(`"phase":"`+phase+`"`))
}

// persist writes the receipt to the VM's state directory, atomically.
func (t *BootTimeline) persist(stateDir string) error {
	if t == nil || stateDir == "" {
		return nil
	}
	t.mu.Lock()
	data, err := json.MarshalIndent((*bootTimelineJSON)(t), "", "  ")
	t.mu.Unlock()
	if err != nil {
		return err
	}
	dir := filepath.Join(stateDir, t.VMID)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	final := filepath.Join(dir, "boot-timeline.json")
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, data, 0640); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

// readBootTimeline loads the persisted receipt for a VM, if present.
func readBootTimeline(stateDir, vmID string) (*BootTimeline, error) {
	data, err := os.ReadFile(filepath.Join(stateDir, vmID, "boot-timeline.json"))
	if err != nil {
		return nil, err
	}
	var t BootTimeline
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// goBuildCommit returns the module commit stamped into the running binary, so
// the vmctl-side receipt can carry its own build identity alongside the
// guest's.
func goBuildCommit() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			return s.Value
		}
	}
	return ""
}
