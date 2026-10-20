package vmctl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Failure modes (docs/problems/guest-store-compaction-scheduled-2026-10-20.md):
// the live image is modified before the compacted copy is proven; a refused
// or failed compaction leaves a half-compacted image in place; a stale copy
// or mount from an interrupted run is reused; a busy guest is restarted for a
// compaction; the hook reaches a refresh that does not want it; the sweep
// picks held, inactive, non-interactive or small computers.

type fakeRunner struct {
	mu    sync.Mutex
	calls []string
	// on returns output and error for a command by its base name.
	on func(name string, args []string) ([]byte, error)
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	base := filepath.Base(name)
	f.mu.Lock()
	f.calls = append(f.calls, base+" "+strings.Join(args, " "))
	f.mu.Unlock()
	if f.on != nil {
		return f.on(base, args)
	}
	return nil, nil
}

func (f *fakeRunner) names() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.calls))
	for _, c := range f.calls {
		out = append(out, strings.Fields(c)[0])
	}
	return out
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// compactorFixture returns a compactor over a temp data image holding
// "original". Its fake cp copies the file; its fake projection-compact
// rewrites the copy to "compacted" and prints the given report.
func compactorFixture(t *testing.T, report string, toolErr error) (*hostStoreCompactor, *fakeRunner, string) {
	t.Helper()
	dir := t.TempDir()
	img := filepath.Join(dir, "data.img")
	if err := os.WriteFile(img, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	runner.on = func(name string, args []string) ([]byte, error) {
		switch name {
		case "cp":
			// GNU coreutils refuses a reflink with any sparse mode but
			// auto (staging, 2026-10-20 01:35).
			reflink, sparse := false, false
			for _, a := range args {
				reflink = reflink || strings.HasPrefix(a, "--reflink") && a != "--reflink=auto"
				sparse = sparse || strings.HasPrefix(a, "--sparse") && a != "--sparse=auto"
			}
			if reflink && sparse {
				return []byte("cp: --reflink can be used only with --sparse=auto"), errors.New("exit status 1")
			}
			src, dst := args[len(args)-2], args[len(args)-1]
			b, err := os.ReadFile(src)
			if err != nil {
				return nil, err
			}
			return nil, os.WriteFile(dst, b, 0o600)
		case "systemd-run":
			if isCleanupUnit(args) {
				return nil, nil
			}
			if toolErr != nil {
				return []byte(report), toolErr
			}
			if err := os.WriteFile(img+storeCompactCopySuffix, []byte("compacted"), 0o600); err != nil {
				return nil, err
			}
			return []byte(report), nil
		}
		return nil, nil
	}
	c := &hostStoreCompactor{run: runner, compactBin: "projection-compact", memoryMax: "10G", mountRoot: filepath.Join(dir, "mnt"), bin: func(n string) string { return n }}
	return c, runner, img
}

func isCleanupUnit(args []string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, "--unit=") && strings.HasSuffix(a, "-cleanup") {
			return true
		}
	}
	return false
}

const goodReport = `{"squashed":true,"commits_before":13700,"commits_after":2,"bytes_before":51954497839,"bytes_after":8062682469,"root_before":"6kpd","root_after":"6kpd"}`

func TestStoreCompactorSwapsInTheCompactedCopy(t *testing.T) {
	c, runner, img := compactorFixture(t, "log line\n"+goodReport, nil)
	report, err := c.Compact(context.Background(), "vm-1", img, "computer-1", "head-1")
	if err != nil {
		t.Fatalf("compact: %v", err)
	}
	if got := readFile(t, img); got != "compacted" {
		t.Fatalf("data.img = %q, want the compacted copy swapped in", got)
	}
	if _, err := os.Stat(img + storeCompactCopySuffix); !os.IsNotExist(err) {
		t.Fatalf("copy left behind: %v", err)
	}
	if report.CommitsAfter != 2 || report.RootBefore != report.RootAfter || report.Duration == "" {
		t.Fatalf("report = %+v", report)
	}
	want := []string{"systemctl", "systemd-run", "cp", "systemd-run"}
	if got := runner.names(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("commands = %v, want %v (cleanup, copy, one compaction unit)", got, want)
	}
	compactCall := runner.calls[len(runner.calls)-1]
	for _, want := range []string{"MemorySwapMax=0", "MemoryMax=10G", "mount -o loop", "umount", "e2fsck -n -f", "head-1", "state.texture", img + storeCompactCopySuffix} {
		if !strings.Contains(compactCall, want) {
			t.Fatalf("compaction unit call lacks %q: %s", want, compactCall)
		}
	}
}

func TestStoreCompactorKeepsOriginalWhenToolRefuses(t *testing.T) {
	cases := map[string]struct {
		report string
		err    error
	}{
		"tool error":   {`{"error":"refused: root changed"}`, errors.New("exit status 1")},
		"root changed": {`{"root_before":"a","root_after":"b"}`, nil},
		"no report":    {"killed", nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, runner, img := compactorFixture(t, tc.report, tc.err)
			if _, err := c.Compact(context.Background(), "vm-1", img, "computer-1", "head-1"); err == nil {
				t.Fatal("compaction succeeded")
			}
			if got := readFile(t, img); got != "original" {
				t.Fatalf("data.img = %q, want the original untouched", got)
			}
			if _, err := os.Stat(img + storeCompactCopySuffix); !os.IsNotExist(err) {
				t.Fatalf("copy left behind: %v", err)
			}
			if names := strings.Join(runner.names(), ","); !strings.HasSuffix(names, "systemd-run,systemctl,systemd-run") {
				t.Fatalf("commands %s: want the cleanup run again after the refusal", names)
			}
		})
	}
}

func TestStoreCompactorRemovesStaleCopyAndMount(t *testing.T) {
	c, runner, img := compactorFixture(t, goodReport, nil)
	if err := os.WriteFile(img+storeCompactCopySuffix, []byte("stale half-compacted"), 0o600); err != nil {
		t.Fatal(err)
	}
	inner := runner.on
	cleanedBeforeCopy := false
	runner.on = func(name string, args []string) ([]byte, error) {
		if name == "systemd-run" && isCleanupUnit(args) {
			cleanedBeforeCopy = strings.Contains(strings.Join(args, " "), "umount")
		}
		if name == "cp" {
			if _, err := os.Stat(args[len(args)-1]); err == nil {
				t.Fatalf("cp ran over the stale copy")
			}
		}
		return inner(name, args)
	}
	if _, err := c.Compact(context.Background(), "vm-1", img, "computer-1", "head-1"); err != nil {
		t.Fatalf("compact: %v", err)
	}
	if !cleanedBeforeCopy || strings.Join(runner.names()[:3], ",") != "systemctl,systemd-run,cp" {
		t.Fatalf("commands = %v, want the stale unit stopped and its mount removed before the copy", runner.names())
	}
	if got := readFile(t, img); got != "compacted" {
		t.Fatalf("data.img = %q", got)
	}
}

func TestStoreCompactorKeepsOriginalWhenUnitFailsBeforeTheTool(t *testing.T) {
	c, _, img := compactorFixture(t, "mount: wrong fs type", errors.New("exit status 32"))
	if _, err := c.Compact(context.Background(), "vm-1", img, "computer-1", "head-1"); err == nil || !strings.Contains(err.Error(), "wrong fs type") {
		t.Fatalf("compact = %v, want the unit's failure reported", err)
	}
	if got := readFile(t, img); got != "original" {
		t.Fatalf("data.img = %q", got)
	}
	if _, err := os.Stat(img + storeCompactCopySuffix); !os.IsNotExist(err) {
		t.Fatalf("copy left behind: %v", err)
	}
}

type fakeCompactor struct {
	mu    sync.Mutex
	calls []string
	err   error
}

func (f *fakeCompactor) Compact(_ context.Context, vmID, dataImagePath, computerID, head string) (storeCompactionReport, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, vmID+"|"+dataImagePath+"|"+computerID)
	return storeCompactionReport{RootBefore: "r", RootAfter: "r", CommitsAfter: 2}, f.err
}

func postCompactStore(t *testing.T, h *Handler, body string) (int, storeCompactResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/internal/vmctl/compact-store", bytes.NewBufferString(body))
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	h.HandleCompactStore(rec, req)
	var resp storeCompactResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode %d %s: %v", rec.Code, rec.Body.String(), err)
	}
	return rec.Code, resp
}

func compactFixture(t *testing.T, user string) (*Handler, *mockVMManager, *fakeGuestMarker, *fakeCompactor) {
	t.Helper()
	h, mgr, marker := newDeployRefreshFixture(t, user)
	compactor := &fakeCompactor{}
	h.storeCompact = &storeCompactState{compactor: compactor}
	h.storeCompactOnce.Do(func() {})
	mgr.maintenanceImage = "/state/vm/data.img"
	return h, mgr, marker, compactor
}

func TestCompactStoreRunsAsPlannedRestartWithMaintenance(t *testing.T) {
	h, mgr, marker, compactor := compactFixture(t, "owner-compact")
	marker.outcome = guestMarkWritten
	code, resp := postCompactStore(t, h, `{"user_id":"owner-compact","desktop_id":"primary"}`)
	if code != http.StatusOK || resp.Outcome != deployRefreshRefreshed || !resp.PlannedRestart {
		t.Fatalf("compact: %d %+v, want a planned refresh", code, resp)
	}
	if len(marker.marks) != 1 || !strings.HasSuffix(marker.marks[0], "@"+storeCompactTarget) {
		t.Fatalf("marks = %v", marker.marks)
	}
	if len(compactor.calls) != 1 || !strings.Contains(compactor.calls[0], "|/state/vm/data.img|") {
		t.Fatalf("compactor calls = %v, want one on the data image during the refresh", compactor.calls)
	}
	if resp.Compaction == nil || resp.Compaction.CommitsAfter != 2 {
		t.Fatalf("compaction report = %+v", resp.Compaction)
	}
	if len(mgr.refreshCfgs) != 1 || mgr.refreshCfgs[0].OfflineMaintenance == nil {
		t.Fatalf("refresh config carried no maintenance")
	}
}

func TestCompactStoreDefersBusyGuestWithoutTouchingIt(t *testing.T) {
	h, mgr, marker, compactor := compactFixture(t, "owner-busy-compact")
	marker.outcome, marker.detail = guestMarkBusy, "self-development apply in flight"
	code, resp := postCompactStore(t, h, `{"user_id":"owner-busy-compact","desktop_id":"primary"}`)
	if code != http.StatusConflict || resp.Outcome != deployRefreshDeferred {
		t.Fatalf("busy: %d %+v, want 409 deferred", code, resp)
	}
	if len(mgr.refreshes) != 0 || len(compactor.calls) != 0 {
		t.Fatalf("busy guest was refreshed (%v) or compacted (%v)", mgr.refreshes, compactor.calls)
	}
}

func TestCompactStoreReportsFailedCompactionButStillBoots(t *testing.T) {
	h, mgr, marker, compactor := compactFixture(t, "owner-compact-fails")
	marker.outcome = guestMarkWritten
	compactor.err = errors.New("projection-compact: refused")
	code, resp := postCompactStore(t, h, `{"user_id":"owner-compact-fails","desktop_id":"primary"}`)
	if code != http.StatusOK || resp.Outcome != deployRefreshRefreshed || len(mgr.refreshes) != 1 {
		t.Fatalf("failed compaction: %d %+v; want the computer refreshed on its original image", code, resp)
	}
	if resp.Compaction == nil || !strings.Contains(resp.Compaction.Error, "refused") {
		t.Fatalf("compaction error not reported: %+v", resp.Compaction)
	}
}

func TestPlainDeployRefreshCarriesNoMaintenance(t *testing.T) {
	h, mgr, marker := newDeployRefreshFixture(t, "owner-plain")
	marker.outcome = guestMarkWritten
	if code, _ := postDeployRefresh(t, h, `{"user_id":"owner-plain","desktop_id":"primary","target":"abc"}`); code != http.StatusOK {
		t.Fatalf("deploy refresh: %d", code)
	}
	if len(mgr.refreshCfgs) != 1 || mgr.refreshCfgs[0].OfflineMaintenance != nil {
		t.Fatalf("a deploy refresh carried offline maintenance")
	}
}

func TestSelectStoreCompaction(t *testing.T) {
	big := &dataImageResponse{FileBytes: 40 << 30}
	small := &dataImageResponse{FileBytes: 3 << 30}
	cp, pp := string(WarmnessClassCriticalProtected), string(WarmnessClassPublicPlatform)
	ownerships := []ownershipResponse{
		{VMID: "vm-paper", State: string(VMStateActive), Kind: VMKindInteractive, WarmnessClass: cp, DataImage: big},
		{VMID: "vm-public", State: string(VMStateActive), Kind: VMKindInteractive, WarmnessClass: pp, DataImage: big},
		{VMID: "vm-small", State: string(VMStateActive), Kind: VMKindInteractive, WarmnessClass: cp, DataImage: small},
		{VMID: "vm-held", State: string(VMStateActive), Kind: VMKindInteractive, WarmnessClass: cp, Held: true, DataImage: big},
		{VMID: "vm-stopped", State: string(VMStateStopped), Kind: VMKindInteractive, WarmnessClass: cp, DataImage: big},
		{VMID: "vm-worker", State: string(VMStateActive), Kind: "worker", WarmnessClass: cp, DataImage: big},
		{VMID: "vm-nostats", State: string(VMStateActive), Kind: VMKindInteractive, WarmnessClass: cp},
		// An account's computer whose untrimmed image sits far above the
		// threshold: never squashed by the sweep (staging, 2026-10-20 01:20).
		{VMID: "vm-premium", State: string(VMStateActive), Kind: VMKindInteractive, WarmnessClass: string(WarmnessClassPremiumAlwaysOn), DataImage: big},
		{VMID: "vm-primary", State: string(VMStateActive), Kind: VMKindInteractive, WarmnessClass: string(WarmnessClassPrimary), DataImage: big},
		{VMID: "vm-unclassed", State: string(VMStateActive), Kind: VMKindInteractive, DataImage: big},
	}
	selected, skipped, picks := selectStoreCompaction(ownerships, 32<<30)
	if len(selected) != 2 || selected[0].VMID != "vm-paper" || selected[1].VMID != "vm-public" || len(picks) != 2 {
		t.Fatalf("selected = %+v", selected)
	}
	reasons := map[string]string{}
	for _, s := range skipped {
		reasons[s.VMID] = s.Reason
	}
	if reasons["vm-small"] != "below threshold" || reasons["vm-held"] != "held" || reasons["vm-nostats"] != "no data image stats" ||
		reasons["vm-premium"] != "not a platform class" || reasons["vm-primary"] != "not a platform class" || reasons["vm-unclassed"] != "not a platform class" || len(skipped) != 6 {
		t.Fatalf("skipped = %+v", skipped)
	}
}

// The ownership list carries no data image stats; the sweep must read them,
// or it never selects a computer (staging, 2026-10-20 01:03).
func TestCompactSweepReadsDataImageStats(t *testing.T) {
	h, _, _, compactor := compactFixture(t, "owner-sweep")
	makePlatformComputer(t, h, "owner-sweep")
	h.dataImageStatsFor = func(string) (DataImageStats, bool) {
		return DataImageStats{FileBytes: 40 << 30, CapBytes: 128 << 30}, true
	}
	req := httptest.NewRequest(http.MethodPost, "/internal/vmctl/compact-sweep", bytes.NewBufferString(`{"dry_run":true}`))
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	h.HandleCompactSweep(rec, req)
	var resp storeCompactSweepResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode %d %s: %v", rec.Code, rec.Body.String(), err)
	}
	if rec.Code != http.StatusOK || len(resp.Selected) != 1 || resp.Selected[0].FileBytes != 40<<30 {
		t.Fatalf("sweep = %d %+v, want the 40 GiB computer selected", rec.Code, resp)
	}
	if resp.Selected[0].Result != nil || len(compactor.calls) != 0 {
		t.Fatalf("a dry run compacted: %+v %v", resp.Selected[0].Result, compactor.calls)
	}
}
