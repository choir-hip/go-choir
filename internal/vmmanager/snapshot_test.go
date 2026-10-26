package vmmanager

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// S3 snapshot hibernate tests. The failure list these cover is in
// docs/reports/s3-snapshot-hibernate-design-2026-10-14.md; each test names
// the items it pins.

const testFCVersion = "Firecracker v1.15.1"

// shortStateDir returns a state dir short enough for unix socket paths on
// macOS (104 bytes).
func shortStateDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "s3")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func fakeFirecrackerBin(t *testing.T, dir string) string {
	t.Helper()
	bin := filepath.Join(dir, "fake-firecracker")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho '"+testFCVersion+"'\n"), 0o755); err != nil {
		t.Fatalf("write fake firecracker: %v", err)
	}
	return bin
}

func serveUnix(t *testing.T, sock string, handler http.Handler) *httptest.Server {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(sock), 0o700); err != nil {
		t.Fatalf("mkdir socket dir: %v", err)
	}
	_ = os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen unix %s: %v", sock, err)
	}
	srv := httptest.NewUnstartedServer(handler)
	_ = srv.Listener.Close()
	srv.Listener = ln
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

type fcCall struct {
	Method string
	Path   string
	Body   map[string]any
}

// fakeFirecrackerAPI records calls and writes snapshot files on create.
type fakeFirecrackerAPI struct {
	mu        sync.Mutex
	calls     []fcCall
	failPath  string
	memBytes  int
	loadState string
	loadMem   string
	// root, when set, is the jail root: request paths are jail-relative and
	// resolve under it, as they do for a jailed Firecracker.
	root string
	// loadFilesPresent records that both load paths existed at load time.
	loadFilesPresent bool
	// onCreate, when set, runs after the snapshot files are written.
	onCreate func(statePath, memPath string)
	// failDiff refuses a Diff create only; diffLen is the length of the
	// sparse memory file a Diff create writes (4 KiB of data at offset 0).
	failDiff bool
	diffLen  int64
	// onCall, when set, runs first for every request (outside the lock).
	onCall func(method, path string)
}

func (f *fakeFirecrackerAPI) hostPath(p string) string {
	if f.root == "" {
		return p
	}
	return filepath.Join(f.root, p)
}

func (f *fakeFirecrackerAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	raw, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(raw, &body)
	if f.onCall != nil {
		f.onCall(r.Method, r.URL.Path)
	}
	f.mu.Lock()
	f.calls = append(f.calls, fcCall{Method: r.Method, Path: r.URL.Path, Body: body})
	fail := f.failPath == r.URL.Path
	diff := r.URL.Path == "/snapshot/create" && body["snapshot_type"] == "Diff"
	if diff && f.failDiff {
		fail = true
	}
	f.mu.Unlock()
	if fail {
		if diff {
			// Firecracker may have created the file before failing.
			mem, _ := body["mem_file_path"].(string)
			writeSparseMem(f.hostPath(mem), f.diffLen)
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"fault_message":"injected failure"}`))
		return
	}
	switch r.URL.Path {
	case "/":
		_, _ = w.Write([]byte(`{"id":"vm","state":"Running"}`))
		return
	case "/snapshot/create":
		state, _ := body["snapshot_path"].(string)
		mem, _ := body["mem_file_path"].(string)
		_ = os.WriteFile(f.hostPath(state), []byte("vmstate"), 0o644)
		if diff {
			writeSparseMem(f.hostPath(mem), f.diffLen)
		} else {
			_ = os.WriteFile(f.hostPath(mem), make([]byte, f.memBytes), 0o644)
		}
		if f.onCreate != nil {
			f.onCreate(f.hostPath(state), f.hostPath(mem))
		}
	case "/snapshot/load":
		f.mu.Lock()
		f.loadState, _ = body["snapshot_path"].(string)
		if backend, ok := body["mem_backend"].(map[string]any); ok {
			f.loadMem, _ = backend["backend_path"].(string)
		}
		_, stateErr := os.Stat(f.hostPath(f.loadState))
		_, memErr := os.Stat(f.hostPath(f.loadMem))
		f.loadFilesPresent = f.loadState != "" && f.loadMem != "" && stateErr == nil && memErr == nil
		f.mu.Unlock()
	}
	w.WriteHeader(http.StatusNoContent)
}

func (f *fakeFirecrackerAPI) paths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.calls))
	for _, c := range f.calls {
		out = append(out, c.Method+" "+c.Path)
	}
	return out
}

// fakeGuest serves the guest's hibernate endpoints and /health.
type fakeGuest struct {
	mu            sync.Mutex
	prepareStatus int
	capExpiresAt  time.Time
	prepares      int
	resumed       int
	hostUnixNano  int64
	// credentialResume is what prepare advertises (re-credential on resume).
	credentialResume bool
	// envelopes records the credential_envelope of each resumed call.
	envelopes []string
	// resumedStatus, when set, is the resumed answer (a refused install).
	resumedStatus int
	// onPrepare, when set, runs while the guest is being fenced.
	onPrepare func()
}

func (g *fakeGuest) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch r.URL.Path {
	case "/health":
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	case guestHibernatePreparePath:
		g.prepares++
		if g.onPrepare != nil {
			g.onPrepare()
		}
		status := g.prepareStatus
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		if status == http.StatusOK {
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "prepared", "capability_expires_at": g.capExpiresAt, "guest_build": "test-build", "credential_resume": g.credentialResume})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "busy", "outcome": "busy", "running_runs": 1})
	case guestHibernateResumedPath:
		g.resumed++
		var body struct {
			HostUnixNano       int64  `json:"host_unix_nano"`
			CredentialEnvelope string `json:"credential_envelope"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		g.hostUnixNano = body.HostUnixNano
		g.envelopes = append(g.envelopes, body.CredentialEnvelope)
		if g.resumedStatus != 0 {
			w.WriteHeader(g.resumedStatus)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "hibernate: install delivered credential after resume: exchange refused"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "resumed", "clock_step_ms": 1234})
	default:
		http.NotFound(w, r)
	}
}

type snapshotFixture struct {
	mgr    *Manager
	vmID   string
	cfg    VMConfig
	inst   *VMInstance
	fc     *fakeFirecrackerAPI
	guest  *fakeGuest
	guestS *httptest.Server
	expect VMConfig
}

// newSnapshotFixture builds a snapshot-mode manager with one running VM whose
// launch config was written exactly as launchFirecracker writes it.
func newSnapshotFixture(t *testing.T) *snapshotFixture {
	t.Helper()
	stateDir := shortStateDir(t)
	cfg := DefaultManagerConfig()
	cfg.StateDir = stateDir
	cfg.HostBasePort = 1
	// Guest artifacts live at stable paths, as on Node B; the snapshot binds
	// their identities.
	artifacts := filepath.Join(stateDir, "guest")
	if err := os.MkdirAll(artifacts, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"vmlinux", "initrd", "storedisk.erofs"} {
		if err := os.WriteFile(filepath.Join(artifacts, name), []byte(name+"-a"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg.KernelImagePath = filepath.Join(artifacts, "vmlinux")
	cfg.InitrdPath = filepath.Join(artifacts, "initrd")
	cfg.StoreDiskPath = filepath.Join(artifacts, "storedisk.erofs")
	cfg.KernelParams = "root=fstab init=/nix/store/init-a"
	cfg.FirecrackerBinPath = fakeFirecrackerBin(t, stateDir)
	cfg.HibernateMode = HibernateModeSnapshot
	cfg.BootReadyTimeout = 2 * time.Second
	cfg.HealthCheckTimeout = time.Second
	mgr := NewManager(cfg)
	// The host floor fails a start whose rules do not install (SEC-03):
	// tap networking runs against the fakes, never the host.
	mgr.hostNet = newFakeHostNet(t, newFakeIPTables(t), newFakeIP(nil))
	prevStatfs := statfsAvailable
	statfsAvailable = func(string) (int64, error) { return 1 << 50, nil }
	t.Cleanup(func() { statfsAvailable = prevStatfs })

	guest := &fakeGuest{capExpiresAt: time.Now().Add(30 * time.Minute)}
	guestS := httptest.NewServer(guest)
	t.Cleanup(guestS.Close)

	vmID := "vm-s3"
	vmDir := filepath.Join(stateDir, vmID)
	if err := os.MkdirAll(vmDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vmDir, "data.img"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	credPath := filepath.Join(vmDir, "credential.img")
	if err := os.WriteFile(credPath, []byte("cred"), 0o600); err != nil {
		t.Fatal(err)
	}
	vmCfg := VMConfig{
		VMID: vmID, ComputerID: "computer-s3", RealizationID: "real-7", OwnerID: "owner", DesktopID: "primary",
		Epoch: 7, GuestPort: 8085, MachineCPUCount: 2, MachineMemSizeMib: 64, GatewayToken: "computer-s3:secret",
	}
	mgr.applyLaunchDefaults(&vmCfg)
	vmCfg.credentialDiskPath = credPath
	hostPort, ok := mgr.hostPortFromHostURL(guestS.URL)
	if !ok {
		t.Fatalf("host port from %s", guestS.URL)
	}
	launch, err := json.Marshal(mgr.buildFirecrackerConfig(vmCfg, hostPort))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vmDir, "fc-config.json"), launch, 0o644); err != nil {
		t.Fatal(err)
	}
	inst := &VMInstance{Config: vmCfg, State: StateRunning, HostURL: guestS.URL, Healthy: true, done: make(chan struct{})}
	mgr.vms[vmID] = inst
	fc := &fakeFirecrackerAPI{memBytes: 4096}
	return &snapshotFixture{
		mgr: mgr, vmID: vmID, cfg: vmCfg, inst: inst, fc: fc, guest: guest, guestS: guestS,
		expect: VMConfig{ComputerID: "computer-s3", MachineCPUCount: 2, MachineMemSizeMib: 64},
	}
}

func (f *snapshotFixture) serveLaunchAPI(t *testing.T) {
	serveUnix(t, f.mgr.apiSocketPath(f.vmID), f.fc)
}

// restoreStarter fakes the restoring Firecracker process: it serves the API
// on the socket from the restore args.
func (f *snapshotFixture) restoreStarter(t *testing.T, api *fakeFirecrackerAPI, gotArgs *[]string) {
	f.mgr.firecrackerStarter = func(vmID string, args []string) error {
		*gotArgs = append([]string(nil), args...)
		sock := ""
		for i, arg := range args {
			if arg == "--api-sock" && i+1 < len(args) {
				sock = args[i+1]
			}
		}
		if sock == "" {
			return errors.New("no --api-sock")
		}
		serveUnix(t, sock, api)
		return nil
	}
}

func readHibernateTimeline(t *testing.T, mgr *Manager, vmID string) *BootTimeline {
	t.Helper()
	tl, err := mgr.HibernateTimelineForVM(vmID)
	if err != nil {
		t.Fatalf("hibernate timeline: %v", err)
	}
	return tl
}

// Failure items 20, 22, 24 (mode switch and launch command line).
func TestHibernateModeSwitchAndLaunchArgs(t *testing.T) {
	stateDir := shortStateDir(t)
	cfg := DefaultManagerConfig()
	cfg.StateDir = stateDir
	mgr := NewManager(cfg)
	if got := mgr.hibernateMode(); got != HibernateModeKill {
		t.Fatalf("default mode = %q, want kill", got)
	}
	args := mgr.firecrackerLaunchArgs("vm-a", "/cfg.json")
	if strings.Join(args, " ") != "--no-api --enable-pci --id vm-a --config-file /cfg.json" {
		t.Fatalf("kill-mode launch args changed: %v", args)
	}

	mgr.cfg.HibernateMode = "SNAPSHOT"
	args = mgr.firecrackerLaunchArgs("vm-a", "/cfg.json")
	sock := filepath.Join(stateDir, "vm-a", "api", "fc.sock")
	if strings.Join(args, " ") != "--api-sock "+sock+" --enable-pci --id vm-a --config-file /cfg.json" {
		t.Fatalf("snapshot-mode launch args = %v", args)
	}
	info, err := os.Stat(filepath.Dir(sock))
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("api dir mode = %v, %v; want 0700", info, err)
	}
	if !firecrackerCmdlineBytesMatchVM([]byte("firecracker\x00"+strings.Join(args, "\x00")), "vm-a") {
		t.Fatal("snapshot-mode command line must still identify the VM")
	}

	long := "vm-" + strings.Repeat("x", 120)
	args = mgr.firecrackerLaunchArgs(long, "/cfg.json")
	if args[0] != "--no-api" {
		t.Fatalf("over-long socket path must fall back to --no-api, got %v", args)
	}

	t.Setenv("VM_HIBERNATE_MODE", "snapshot")
	t.Setenv("VM_SNAPSHOT_BUDGET_GIB", "12")
	t.Setenv("VM_SNAPSHOT_MAX_AGE", "2h")
	loaded := LoadConfigFromEnv()
	if loaded.HibernateMode != "snapshot" || loaded.SnapshotBudgetGiB != 12 || loaded.SnapshotMaxAge != 2*time.Hour {
		t.Fatalf("env config = %q %d %s", loaded.HibernateMode, loaded.SnapshotBudgetGiB, loaded.SnapshotMaxAge)
	}
	if got := strings.Join(firecrackerRestoreArgs("vm-a", sock), " "); got != "--api-sock "+sock+" --enable-pci --id vm-a" {
		t.Fatalf("restore args = %s", got)
	}
}

// Failure items 4 (API errors surface) and the API client contract.
func TestFCAPIClientOverUnixSocket(t *testing.T) {
	dir := shortStateDir(t)
	sock := filepath.Join(dir, "fc.sock")
	fc := &fakeFirecrackerAPI{memBytes: 8}
	serveUnix(t, sock, fc)
	api := newFCAPIClient(sock)
	ctx := t.Context()
	if err := api.WaitReady(ctx); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}
	if err := api.Pause(ctx); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	state, mem := filepath.Join(dir, "vmstate"), filepath.Join(dir, "mem")
	if err := api.CreateFullSnapshot(ctx, state, mem); err != nil {
		t.Fatalf("CreateFullSnapshot: %v", err)
	}
	if err := api.LoadSnapshot(ctx, state, mem); err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	fc.mu.Lock()
	calls := append([]fcCall(nil), fc.calls...)
	fc.mu.Unlock()
	want := []string{"GET /", "PATCH /vm", "PUT /snapshot/create", "PUT /snapshot/load"}
	if got := fc.paths(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("calls = %v, want %v", got, want)
	}
	if calls[1].Body["state"] != "Paused" {
		t.Fatalf("pause body = %v", calls[1].Body)
	}
	if calls[2].Body["snapshot_type"] != "Full" || calls[2].Body["snapshot_path"] != state || calls[2].Body["mem_file_path"] != mem {
		t.Fatalf("create body = %v", calls[2].Body)
	}
	backend, _ := calls[3].Body["mem_backend"].(map[string]any)
	if calls[3].Body["resume_vm"] != true || backend["backend_type"] != "File" || backend["backend_path"] != mem || calls[3].Body["snapshot_path"] != state {
		t.Fatalf("load body = %v", calls[3].Body)
	}

	fc.failPath = "/snapshot/load"
	err := api.LoadSnapshot(ctx, state, mem)
	var apiErr *fcAPIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusBadRequest || apiErr.FaultMessage != "injected failure" {
		t.Fatalf("load error = %v", err)
	}
}

// Density D1 (docs/problems/busy-guest-hibernate-takes-kill-path-2026-10-26.md):
// a guest that answers 409 at the prepare fence is working. The hibernate is
// abandoned and the VM stays running and healthy; it is never killed, so its
// work is not closed as "interrupted by a restart". The error is distinct so
// every caller counts a skip. Other prepare failures keep the kill path
// (TestHibernateSnapshotFallsBackToKill).
func TestHibernateSnapshotGuestBusyLeavesTheVMRunning(t *testing.T) {
	f := newSnapshotFixture(t)
	f.serveLaunchAPI(t)
	f.guest.prepareStatus = http.StatusConflict
	err := f.mgr.HibernateVM(f.vmID)
	var busy interface{ HibernateGuestBusy() bool }
	if err == nil || !errors.As(err, &busy) || !busy.HibernateGuestBusy() {
		t.Fatalf("HibernateVM = %v, want a guest-busy skip error", err)
	}
	f.mgr.mu.Lock()
	state, healthy := f.inst.State, f.inst.Healthy
	f.mgr.mu.Unlock()
	if state != StateRunning || !healthy {
		t.Fatalf("state=%s healthy=%v, want the busy guest left running and healthy", state, healthy)
	}
	for _, call := range f.fc.paths() {
		if call != "GET /" {
			t.Fatalf("busy guest must never be paused or snapshotted; calls=%v", f.fc.paths())
		}
	}
	if _, statErr := os.Stat(f.mgr.snapshotDir(f.vmID)); !os.IsNotExist(statErr) {
		t.Fatalf("no snapshot may be left for a running VM: %v", statErr)
	}
	tl := readHibernateTimeline(t, f.mgr, f.vmID)
	if tl.Snapshot == nil || tl.Snapshot.Outcome != "skip" || tl.Snapshot.Reason != snapshotReasonGuestBusy || tl.BootKind == "hibernate_kill" {
		t.Fatalf("timeline = %+v %+v, want a skip for guest_busy", tl.BootKind, tl.Snapshot)
	}
}

// Failure items 1, 3 (fence fallback) and 20 (no API socket): the kill path
// is today's behavior, and the guest is never paused when it refuses.
func TestHibernateSnapshotFallsBackToKill(t *testing.T) {
	t.Run("no api socket", func(t *testing.T) {
		f := newSnapshotFixture(t)
		if err := f.mgr.HibernateVM(f.vmID); err != nil {
			t.Fatalf("HibernateVM: %v", err)
		}
		if f.inst.State != StateHibernated {
			t.Fatalf("state = %s", f.inst.State)
		}
		tl := readHibernateTimeline(t, f.mgr, f.vmID)
		if tl.BootKind != "hibernate_kill" || tl.Snapshot == nil || tl.Snapshot.Reason != snapshotReasonNoAPISocket {
			t.Fatalf("timeline = %+v %+v", tl.BootKind, tl.Snapshot)
		}
		if f.guest.prepares != 0 {
			t.Fatal("guest must not be fenced without an API socket")
		}
	})
	t.Run("old guest without endpoint", func(t *testing.T) {
		f := newSnapshotFixture(t)
		f.serveLaunchAPI(t)
		f.guest.prepareStatus = http.StatusNotFound
		if err := f.mgr.HibernateVM(f.vmID); err != nil {
			t.Fatalf("HibernateVM: %v", err)
		}
		if tl := readHibernateTimeline(t, f.mgr, f.vmID); tl.Snapshot.Reason != snapshotReasonGuestNoEndpoint {
			t.Fatalf("reason = %s", tl.Snapshot.Reason)
		}
	})
	t.Run("snapshot create fails", func(t *testing.T) {
		f := newSnapshotFixture(t)
		f.serveLaunchAPI(t)
		f.fc.failPath = "/snapshot/create"
		if err := f.mgr.HibernateVM(f.vmID); err != nil {
			t.Fatalf("HibernateVM: %v", err)
		}
		if f.inst.State != StateHibernated {
			t.Fatalf("state = %s", f.inst.State)
		}
		if _, err := os.Stat(f.mgr.snapshotPartialDir(f.vmID)); !os.IsNotExist(err) {
			t.Fatalf("partial snapshot must be removed: %v", err)
		}
		if _, err := os.Stat(f.mgr.snapshotDir(f.vmID)); !os.IsNotExist(err) {
			t.Fatalf("no committed snapshot may exist: %v", err)
		}
		if tl := readHibernateTimeline(t, f.mgr, f.vmID); tl.Snapshot.Reason != snapshotReasonCreateFailed {
			t.Fatalf("reason = %s", tl.Snapshot.Reason)
		}
	})
	t.Run("kill mode never fences", func(t *testing.T) {
		f := newSnapshotFixture(t)
		f.mgr.cfg.HibernateMode = HibernateModeKill
		f.serveLaunchAPI(t)
		if err := f.mgr.HibernateVM(f.vmID); err != nil {
			t.Fatalf("HibernateVM: %v", err)
		}
		if f.guest.prepares != 0 || len(f.fc.paths()) != 0 {
			t.Fatalf("kill mode touched the guest or API: prepares=%d calls=%v", f.guest.prepares, f.fc.paths())
		}
	})
}

// Failure items 5, 6, 7, 10, 12, 15, 18: the full round trip, one-shot.
func TestHibernateAndResumeFromSnapshotRoundTrip(t *testing.T) {
	f := newSnapshotFixture(t)
	f.serveLaunchAPI(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatalf("HibernateVM: %v", err)
	}
	if got := strings.Join(f.fc.paths(), ","); got != "GET /,PATCH /vm,PUT /snapshot/create" {
		t.Fatalf("hibernate API calls = %s", got)
	}
	dir := f.mgr.snapshotDir(f.vmID)
	meta, err := readSnapshotMeta(dir)
	if err != nil {
		t.Fatalf("committed snapshot: %v", err)
	}
	if meta.Consumed || meta.Epoch != 7 || meta.FirecrackerVersion != testFCVersion || meta.LaunchConfigSHA256 == "" || len(meta.Disks) != 5 {
		t.Fatalf("meta = %+v", meta)
	}
	if meta.Config.GatewayToken != "" || meta.Config.ComputerCredentialEnvelope != "" {
		t.Fatal("snapshot metadata must not carry bootstrap secrets")
	}
	wantDeadline := f.guest.capExpiresAt.Add(snapshotCredentialRenewalGrace - snapshotCredentialMargin)
	if !meta.ResumeDeadline.Equal(wantDeadline) {
		t.Fatalf("resume deadline = %s, want %s", meta.ResumeDeadline, wantDeadline)
	}
	for _, name := range []string{snapshotStateName, snapshotMemName, snapshotMetaName} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode = %v, %v; want 0600", name, info, err)
		}
	}
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("snapshot dir mode = %v, %v; want 0700", info, err)
	}
	if !f.mgr.snapshotPortHeldByOther("other-vm", meta.HostPort) {
		t.Fatal("hibernated snapshot must reserve its host port")
	}
	if tl := readHibernateTimeline(t, f.mgr, f.vmID); tl.BootKind != "hibernate_snapshot" || tl.Snapshot.Outcome != "snapshot" || tl.Snapshot.Bytes == 0 {
		t.Fatalf("hibernate timeline = %s %+v", tl.BootKind, tl.Snapshot)
	}

	restoreAPI := &fakeFirecrackerAPI{}
	var restoreArgs []string
	f.restoreStarter(t, restoreAPI, &restoreArgs)
	inst, err := f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, nil)
	if err != nil {
		t.Fatalf("ResumeVMFromSnapshot: %v", err)
	}
	if inst.State != StateRunning || inst.Config.Epoch != 7 || inst.HostURL != f.guestS.URL {
		t.Fatalf("resumed instance = state %s epoch %d host %s", inst.State, inst.Config.Epoch, inst.HostURL)
	}
	for _, arg := range restoreArgs {
		if arg == "--config-file" || arg == "--no-api" {
			t.Fatalf("restore must start Firecracker with the API socket only: %v", restoreArgs)
		}
	}
	if restoreAPI.loadState != filepath.Join(dir, snapshotStateName) || restoreAPI.loadMem != filepath.Join(dir, snapshotMemName) {
		t.Fatalf("load paths = %s %s", restoreAPI.loadState, restoreAPI.loadMem)
	}
	if f.guest.resumed != 1 || f.guest.hostUnixNano == 0 {
		t.Fatalf("guest resumed calls = %d host time = %d", f.guest.resumed, f.guest.hostUnixNano)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("snapshot files must be removed after a successful resume: %v", err)
	}
	tl, err := f.mgr.BootTimelineForVM(f.vmID)
	if err != nil {
		t.Fatal(err)
	}
	if tl.BootKind != "snapshot_resume" || tl.Outcome != "healthy" || tl.Snapshot == nil || tl.Snapshot.Outcome != "snapshot" || tl.Snapshot.ClockStepMS != 1234 {
		t.Fatalf("resume timeline = %s %s %+v", tl.BootKind, tl.Outcome, tl.Snapshot)
	}

	// One-shot: nothing left to load.
	f.mgr.mu.Lock()
	inst.State = StateHibernated
	f.mgr.mu.Unlock()
	_, err = f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, nil)
	var fb *SnapshotFallbackError
	if !errors.As(err, &fb) || fb.Reason != snapshotReasonNoSnapshot {
		t.Fatalf("second resume = %v, want no_snapshot fallback", err)
	}
}

// Failure item 6: a consumed snapshot is never loaded, even if its files
// survive (crash after consumption, before load).
func TestConsumedSnapshotIsNeverLoaded(t *testing.T) {
	f := newSnapshotFixture(t)
	f.serveLaunchAPI(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	dir := f.mgr.snapshotDir(f.vmID)
	meta, err := readSnapshotMeta(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := consumeSnapshot(dir, meta, time.Now()); err != nil {
		t.Fatal(err)
	}
	reread, err := readSnapshotMeta(dir)
	if err != nil || !reread.Consumed || reread.ConsumedAt.IsZero() {
		t.Fatalf("consumption not durable: %+v %v", reread, err)
	}
	started := false
	f.mgr.firecrackerStarter = func(string, []string) error { started = true; return nil }
	_, err = f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, nil)
	var fb *SnapshotFallbackError
	if !errors.As(err, &fb) || fb.Reason != snapshotReasonConsumed {
		t.Fatalf("resume = %v, want consumed fallback", err)
	}
	if started {
		t.Fatal("a consumed snapshot must never start Firecracker")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("a consumed snapshot is discarded")
	}
	if got := f.mgr.takeSnapshotFallback(f.vmID); got != snapshotReasonConsumed {
		t.Fatalf("fallback reason for the cold boot = %q", got)
	}
}

// Failure items 7, 8, 9, 11, 21: every bound field refuses before load.
func TestValidateSnapshotRefusesEveryMismatch(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(t *testing.T, f *snapshotFixture, meta *SnapshotMeta, v *snapshotValidation)
		want   string
	}{
		{"valid", func(*testing.T, *snapshotFixture, *SnapshotMeta, *snapshotValidation) {}, ""},
		{"consumed", func(_ *testing.T, _ *snapshotFixture, meta *SnapshotMeta, _ *snapshotValidation) {
			meta.Consumed = true
		}, snapshotReasonConsumed},
		{"version", func(_ *testing.T, _ *snapshotFixture, meta *SnapshotMeta, _ *snapshotValidation) { meta.Version = 99 }, snapshotReasonVersion},
		{"other computer", func(_ *testing.T, _ *snapshotFixture, _ *SnapshotMeta, v *snapshotValidation) {
			v.Expect.ComputerID = "computer-other"
		}, snapshotReasonIdentity},
		{"other realization", func(_ *testing.T, _ *snapshotFixture, meta *SnapshotMeta, v *snapshotValidation) {
			v.Expect.Epoch = meta.Epoch + 1
		}, snapshotReasonIdentity},
		{"firecracker upgraded", func(_ *testing.T, _ *snapshotFixture, _ *SnapshotMeta, v *snapshotValidation) {
			v.FirecrackerVersion = "Firecracker v1.16.0"
		}, snapshotReasonFirecracker},
		{"firecracker patch bump 1.16.1 to 1.16.2", func(_ *testing.T, _ *snapshotFixture, meta *SnapshotMeta, v *snapshotValidation) {
			meta.FirecrackerVersion = "Firecracker v1.16.1"
			v.FirecrackerVersion = "Firecracker v1.16.2"
		}, snapshotReasonFirecracker},
		{"host kernel changed", func(_ *testing.T, _ *snapshotFixture, _ *SnapshotMeta, v *snapshotValidation) {
			v.HostKernelRelease = "6.19.0"
		}, snapshotReasonHostKernel},
		{"deploy moved store disk", func(_ *testing.T, f *snapshotFixture, _ *SnapshotMeta, _ *snapshotValidation) {
			f.mgr.cfg.StoreDiskPath = "/nix/store/store-b.erofs"
		}, snapshotReasonLaunchConfig},
		{"deploy moved kernel", func(_ *testing.T, f *snapshotFixture, _ *SnapshotMeta, _ *snapshotValidation) {
			f.mgr.cfg.KernelImagePath = "/nix/store/kernel-b/bzImage"
		}, snapshotReasonLaunchConfig},
		{"shape changed", func(_ *testing.T, _ *snapshotFixture, _ *SnapshotMeta, v *snapshotValidation) {
			v.Expect.MachineMemSizeMib = 128
		}, snapshotReasonLaunchConfig},
		{"hold placed", func(_ *testing.T, _ *snapshotFixture, _ *SnapshotMeta, v *snapshotValidation) {
			v.Expect.MaintenanceHold = true
		}, snapshotReasonLaunchConfig},
		{"data disk written", func(t *testing.T, f *snapshotFixture, _ *SnapshotMeta, _ *snapshotValidation) {
			path := filepath.Join(f.mgr.vmStateDir(f.vmID), "data.img")
			later := time.Now().Add(time.Minute)
			if err := os.Chtimes(path, later, later); err != nil {
				t.Fatal(err)
			}
		}, snapshotReasonDisk},
		{"data disk replaced", func(t *testing.T, f *snapshotFixture, _ *SnapshotMeta, _ *snapshotValidation) {
			// Same size and mtime, new inode: written beside, renamed over
			// (the old inode is still live, so it cannot be reused).
			path := filepath.Join(f.mgr.vmStateDir(f.vmID), "data.img")
			info, _ := os.Stat(path)
			tmp := path + ".new"
			if err := os.WriteFile(tmp, []byte("data"), 0o644); err != nil {
				t.Fatal(err)
			}
			_ = os.Chtimes(tmp, info.ModTime(), info.ModTime())
			if err := os.Rename(tmp, path); err != nil {
				t.Fatal(err)
			}
		}, snapshotReasonDisk},
		{"deploy replaced store image in place", func(t *testing.T, f *snapshotFixture, _ *SnapshotMeta, _ *snapshotValidation) {
			// Same path, so the launch config hash is unchanged; the bytes
			// under it are another build's.
			path := f.mgr.cfg.StoreDiskPath
			tmp := path + ".new"
			if err := os.WriteFile(tmp, []byte("storedisk.erofs-b"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(tmp, path); err != nil {
				t.Fatal(err)
			}
		}, snapshotReasonDisk},
		{"deploy replaced kernel in place", func(t *testing.T, f *snapshotFixture, _ *SnapshotMeta, _ *snapshotValidation) {
			later := time.Now().Add(time.Minute)
			if err := os.Chtimes(f.mgr.cfg.KernelImagePath, later, later); err != nil {
				t.Fatal(err)
			}
		}, snapshotReasonDisk},
		{"credential disk rewritten", func(t *testing.T, f *snapshotFixture, _ *SnapshotMeta, _ *snapshotValidation) {
			if err := os.WriteFile(f.cfg.credentialDiskPath, []byte("new credential"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, snapshotReasonDisk},
		{"credential window passed", func(_ *testing.T, _ *snapshotFixture, meta *SnapshotMeta, v *snapshotValidation) {
			v.Now = meta.ResumeDeadline
		}, snapshotReasonCredentialWindow},
		{"max age passed", func(_ *testing.T, _ *snapshotFixture, meta *SnapshotMeta, v *snapshotValidation) {
			meta.ResumeDeadline = time.Time{}
			v.Now = meta.CreatedAt.Add(DefaultSnapshotMaxAge)
		}, snapshotReasonMaxAge},
		{"memory file missing", func(t *testing.T, f *snapshotFixture, _ *SnapshotMeta, _ *snapshotValidation) {
			_ = os.Remove(filepath.Join(f.mgr.snapshotDir(f.vmID), snapshotMemName))
		}, snapshotReasonFilesMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newSnapshotFixture(t)
			f.serveLaunchAPI(t)
			if err := f.mgr.HibernateVM(f.vmID); err != nil {
				t.Fatal(err)
			}
			dir := f.mgr.snapshotDir(f.vmID)
			meta, err := readSnapshotMeta(dir)
			if err != nil {
				t.Fatal(err)
			}
			v := snapshotValidation{Now: time.Now(), FirecrackerVersion: meta.FirecrackerVersion, HostKernelRelease: meta.HostKernelRelease, Expect: f.expect}
			tc.mutate(t, f, meta, &v)
			if got := f.mgr.validateSnapshot(dir, meta, v); got != tc.want {
				t.Fatalf("validateSnapshot = %q, want %q", got, tc.want)
			}
		})
	}
}

// Failure item 7 end to end: a disk change refuses, discards, and names the
// reason for the cold boot that follows; that boot discards nothing more.
func TestResumeAfterDiskChangeFallsBackCold(t *testing.T) {
	f := newSnapshotFixture(t)
	f.serveLaunchAPI(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(filepath.Join(f.mgr.vmStateDir(f.vmID), "data.img"), later, later); err != nil {
		t.Fatal(err)
	}
	_, err := f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, nil)
	var fb *SnapshotFallbackError
	if !errors.As(err, &fb) || fb.Reason != snapshotReasonDisk {
		t.Fatalf("resume = %v, want disk fallback", err)
	}
	if _, err := os.Stat(f.mgr.snapshotDir(f.vmID)); !os.IsNotExist(err) {
		t.Fatal("refused snapshot must be discarded")
	}
	if f.mgr.snapshotPortHeldByOther("other", fb2port(t, f)) {
		t.Fatal("discarded snapshot must release its port")
	}
	tl := newBootTimeline(f.vmID, "cold")
	f.mgr.recordSnapshotFallbackOnBoot(f.vmID, tl)
	if tl.Snapshot == nil || tl.Snapshot.Outcome != "fallback_cold" || tl.Snapshot.Reason != snapshotReasonDisk {
		t.Fatalf("cold boot timeline snapshot = %+v", tl.Snapshot)
	}
}

// The 1.16.1 -> 1.16.2 host bump (docs/problems/firecracker-1-16-1-virtio-
// mem-restore-bugs-2026-10-22.md): 1.16.2 changed the snapshot format, so a
// snapshot taken under 1.16.1 must be refused before Firecracker starts,
// discarded, and the wake must cold boot. A patch-level banner change alone
// refuses; nothing parses or tolerates "compatible" versions.
func TestResumeRefusesSnapshotFromOtherFirecrackerPatch(t *testing.T) {
	f := newSnapshotFixture(t)
	root := t.TempDir()
	f.mgr.cfg.FirecrackerBinPath = fakeFirecrackerAt(t, filepath.Join(root, "fc-1.16.1", "bin"), "Firecracker v1.16.1")
	f.serveLaunchAPI(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	dir := f.mgr.snapshotDir(f.vmID)
	meta, err := readSnapshotMeta(dir)
	if err != nil || meta.FirecrackerVersion != "Firecracker v1.16.1" {
		t.Fatalf("snapshot meta version = %+v, %v", meta, err)
	}

	// Deploy: the host's pinned binary is now 1.16.2.
	f.mgr.cfg.FirecrackerBinPath = fakeFirecrackerAt(t, filepath.Join(root, "fc-1.16.2", "bin"), "Firecracker v1.16.2")
	started := false
	f.mgr.firecrackerStarter = func(string, []string) error { started = true; return nil }
	_, err = f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, nil)
	var fb *SnapshotFallbackError
	if !errors.As(err, &fb) || fb.Reason != snapshotReasonFirecracker {
		t.Fatalf("resume = %v, want %s fallback", err, snapshotReasonFirecracker)
	}
	if started {
		t.Fatal("a 1.16.1 snapshot must never start a 1.16.2 Firecracker to load it")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("the refused 1.16.1 snapshot must be discarded")
	}
	if got := f.mgr.GetVM(f.vmID); got == nil || got.State != StateHibernated {
		t.Fatalf("the computer must stay hibernated for the cold boot, got %+v", got)
	}
	tl := newBootTimeline(f.vmID, "cold")
	f.mgr.recordSnapshotFallbackOnBoot(f.vmID, tl)
	if tl.Snapshot == nil || tl.Snapshot.Outcome != "fallback_cold" || tl.Snapshot.Reason != snapshotReasonFirecracker {
		t.Fatalf("cold boot timeline snapshot = %+v", tl.Snapshot)
	}
}

func fb2port(t *testing.T, f *snapshotFixture) int {
	t.Helper()
	port, ok := f.mgr.hostPortFromHostURL(f.guestS.URL)
	if !ok {
		t.Fatal("no port")
	}
	return port
}

// Failure items 23, 6: a failed load kills the restoring process, never
// reloads, and leaves the VM to a cold boot.
func TestResumeLoadFailureFallsBackAndNeverReloads(t *testing.T) {
	f := newSnapshotFixture(t)
	f.serveLaunchAPI(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	restoreAPI := &fakeFirecrackerAPI{failPath: "/snapshot/load"}
	var args []string
	f.restoreStarter(t, restoreAPI, &args)
	_, err := f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, nil)
	var fb *SnapshotFallbackError
	if !errors.As(err, &fb) || fb.Reason != snapshotReasonLoadFailed {
		t.Fatalf("resume = %v, want load_failed", err)
	}
	if got := f.mgr.GetVM(f.vmID); got == nil || got.State != StateHibernated {
		t.Fatalf("manager must restore the hibernated instance for the cold boot, got %+v", got)
	}
	if f.guest.resumed != 0 {
		t.Fatal("guest must not be told it resumed after a failed load")
	}
	if _, err := os.Stat(f.mgr.snapshotDir(f.vmID)); !os.IsNotExist(err) {
		t.Fatal("failed snapshot must be discarded")
	}
}

// Failure item 24: after a rollback to kill mode a snapshot is never loaded.
func TestKillModeNeverLoadsLeftoverSnapshot(t *testing.T) {
	f := newSnapshotFixture(t)
	f.serveLaunchAPI(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	f.mgr.cfg.HibernateMode = HibernateModeKill
	started := false
	f.mgr.firecrackerStarter = func(string, []string) error { started = true; return nil }
	_, err := f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, nil)
	var fb *SnapshotFallbackError
	if !errors.As(err, &fb) || fb.Reason != snapshotReasonModeKill || started {
		t.Fatalf("resume = %v started=%v, want mode_kill fallback without start", err, started)
	}
}

// Failure item 5: an interrupted capture (partial dir only) is never resumable.
func TestPartialSnapshotIsNeverResumable(t *testing.T) {
	f := newSnapshotFixture(t)
	partial := f.mgr.snapshotPartialDir(f.vmID)
	if err := os.MkdirAll(partial, 0o700); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(partial, snapshotStateName), []byte("x"), 0o600)
	_, err := f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, nil)
	var fb *SnapshotFallbackError
	if !errors.As(err, &fb) || fb.Reason != snapshotReasonNoSnapshot {
		t.Fatalf("resume = %v, want no_snapshot", err)
	}
	if _, err := os.Stat(partial); !os.IsNotExist(err) {
		t.Fatal("partial capture must be cleaned up")
	}
}

// Failure item 14 (any cold boot discards the snapshot first).
func TestColdBootDiscardsSnapshot(t *testing.T) {
	f := newSnapshotFixture(t)
	f.serveLaunchAPI(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	tl := newBootTimeline(f.vmID, "recover")
	f.mgr.recordSnapshotFallbackOnBoot(f.vmID, tl)
	if _, err := os.Stat(f.mgr.snapshotDir(f.vmID)); !os.IsNotExist(err) {
		t.Fatal("cold boot must discard the snapshot")
	}
	if tl.Snapshot == nil || tl.Snapshot.Reason != snapshotReasonColdBoot {
		t.Fatalf("timeline = %+v", tl.Snapshot)
	}
}

// Failure item 17: budget eviction takes the least recently created
// snapshots of other VMs, skips VMs mid-operation, and refuses when the
// budget or the disk floor cannot be met.
func TestEnsureSnapshotRoomEvictsOldestFirst(t *testing.T) {
	stateDir := shortStateDir(t)
	cfg := DefaultManagerConfig()
	cfg.StateDir = stateDir
	cfg.SnapshotBudgetGiB = 1
	mgr := NewManager(cfg)
	prevStatfs := statfsAvailable
	free := int64(1) << 50
	statfsAvailable = func(string) (int64, error) { return free, nil }
	t.Cleanup(func() { statfsAvailable = prevStatfs })
	// Incompressible snapshots: on-disk bytes equal logical bytes.
	fakeDiskBytes(t, 100)

	const chunk = int64(400) << 20
	base := time.Now().Add(-time.Hour)
	for i, vm := range []string{"vm-old", "vm-mid", "vm-new"} {
		dir := mgr.snapshotDir(vm)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		mem, err := os.Create(filepath.Join(dir, snapshotMemName))
		if err != nil {
			t.Fatal(err)
		}
		if err := mem.Truncate(chunk); err != nil {
			t.Fatal(err)
		}
		_ = mem.Close()
		meta := &SnapshotMeta{Version: snapshotMetaVersion, VMID: vm, HostPort: 100 + i, CreatedAt: base.Add(time.Duration(i) * time.Minute)}
		if err := writeSnapshotMeta(dir, meta); err != nil {
			t.Fatal(err)
		}
		mgr.reserveSnapshotPort(vm, meta.HostPort)
	}

	// vm-old is mid-operation: it cannot be evicted, so vm-mid and vm-new go.
	unlockOld := mgr.lockVMOperation("vm-old")
	if !mgr.ensureSnapshotRoom("vm-x", chunk) {
		t.Fatal("expected room after evicting unlocked snapshots")
	}
	unlockOld()
	if _, err := os.Stat(mgr.snapshotDir("vm-old")); err != nil {
		t.Fatal("a VM mid-operation must not be evicted")
	}
	if _, err := os.Stat(mgr.snapshotDir("vm-mid")); !os.IsNotExist(err) {
		t.Fatal("vm-mid should be evicted")
	}
	if mgr.snapshotPortHeldByOther("vm-x", 101) {
		t.Fatal("evicted snapshot must release its port")
	}

	// Oldest goes first once it is free.
	for _, vm := range []string{"vm-new"} {
		if _, err := os.Stat(mgr.snapshotDir(vm)); err == nil {
			t.Fatalf("%s should have been evicted to fit", vm)
		}
	}

	if mgr.ensureSnapshotRoom("vm-x", int64(2)<<30) {
		t.Fatal("a snapshot larger than the whole budget must be refused")
	}
	free = snapshotDiskFloorBytes
	if mgr.ensureSnapshotRoom("vm-x", chunk) {
		t.Fatal("the disk floor must refuse a snapshot that would cross it")
	}
}

// Failure item 15: another VM never receives a hibernated VM's subnet.
func TestAllocateHostPortSkipsSnapshotReservation(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.StateDir = shortStateDir(t)
	mgr := NewManager(cfg)
	mgr.reserveSnapshotPort("vm-hibernated", cfg.HostBasePort)
	mgr.mu.Lock()
	port, err := mgr.allocateHostPortLocked("vm-other")
	mgr.mu.Unlock()
	if err != nil || port != cfg.HostBasePort+1 {
		t.Fatalf("allocated %d (%v); want %d", port, err, cfg.HostBasePort+1)
	}
	mgr.mu.Lock()
	mgr.nextPort = cfg.HostBasePort
	own, err := mgr.allocateHostPortLocked("vm-hibernated")
	mgr.mu.Unlock()
	if err != nil || own != cfg.HostBasePort {
		t.Fatalf("owner allocated %d (%v); want its own port", own, err)
	}
}

// Failure item 8: the rebuilt expected config equals the launch config byte
// for byte when nothing moved (otherwise no snapshot would ever load).
func TestExpectedLaunchConfigMatchesLaunch(t *testing.T) {
	f := newSnapshotFixture(t)
	hostPort, _ := f.mgr.hostPortFromHostURL(f.guestS.URL)
	launch, err := os.ReadFile(filepath.Join(f.mgr.vmStateDir(f.vmID), "fc-config.json"))
	if err != nil {
		t.Fatal(err)
	}
	meta := &SnapshotMeta{VMID: f.vmID, HostPort: hostPort, Config: snapshotStoredConfig(f.cfg), CredentialDiskPath: f.cfg.credentialDiskPath}
	_, expected := f.mgr.expectedLaunchConfig(meta, f.expect)
	hash, err := launchConfigHash(expected)
	if err != nil {
		t.Fatal(err)
	}
	if hash != sha256Hex(launch) {
		got, _ := json.Marshal(expected)
		t.Fatalf("expected config differs from launch:\nlaunch:   %s\nexpected: %s", launch, got)
	}
}

func TestResumeDeadlineFromCapability(t *testing.T) {
	if !resumeDeadlineFrom(time.Time{}).IsZero() {
		t.Fatal("no capability means no credential deadline")
	}
	exp := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	if got := resumeDeadlineFrom(exp); !got.Equal(exp.Add(12 * time.Minute)) {
		t.Fatalf("deadline = %s", got)
	}
}
