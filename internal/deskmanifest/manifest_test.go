package deskmanifest

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
)

// The manifest is one table that every desk check reads. These tests pin the
// authority it gives each desk to what the scattered tables granted before (or
// less), so widening a desk is an edit to this file, visible in review.

// systemNames is the manifest without registered app desks, which other tests
// add to the process-wide registry.
func systemNames() []string {
	var out []string
	for _, n := range Names() {
		if d, _ := Lookup(n); d.Kind != KindApp {
			out = append(out, n)
		}
	}
	return out
}

func sorted(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	return out
}

func TestUnknownDeskHasNoAuthority(t *testing.T) {
	for _, name := range []string{"", "root", "Research", "research ", "app:never-registered", "management-task", "verifier", "engineering/impl"} {
		if _, ok := Lookup(name); ok {
			t.Errorf("Lookup(%q) found a desk the manifest does not name", name)
		}
		for _, action := range []string{"read", "product_api", "exec", "read_capture", "emit"} {
			if FrameAllowed(name, action) {
				t.Errorf("FrameAllowed(%q,%q) served an unknown desk", name, action)
			}
		}
	}
}

// Failure mode: a desk added with exec or write_file by omission. Only
// engineering holds them; no desk holds the retired emit and message.
func TestOnlyEngineeringHoldsExecAndWriteFile(t *testing.T) {
	for _, name := range Names() {
		d, _ := Lookup(name)
		for _, a := range d.HandleActions() {
			switch a {
			case "exec", "write_file":
				if name != Engineering {
					t.Errorf("desk %s holds %s; only engineering may", name, a)
				}
			case "emit", "message":
				t.Errorf("desk %s holds the retired %s action", name, a)
			}
		}
		for _, a := range d.FrameActions() {
			if a == "exec" || a == "write_file" || a == "read_file" || a == "list_dir" {
				t.Errorf("desk %s: %s is served by the worker, never over a frame", name, a)
			}
		}
	}
}

// pinnedFrame is each desk's frame table, written out. It equals the table in
// yaegikernel's frame_role_test.go, which is pinned against the module sets.
var pinnedFrame = map[string][]string{
	Management:     {"product_api", "read_capture", "stories"},
	Texture:        {"read_capture"},
	Research:       {"search_start", "read", "search_wire_corpus", "save_evidence", "read_evidence", "list_evidence", "get_run_memory_entry", "research", "poll_source", "read_capture", "system1", "source_api"},
	ResearchChild:  {"search_start", "read", "read_capture", "search_wire_corpus", "read_evidence", "list_evidence"},
	ManagementTask: {"read_capture", "stories"},
	Engineering:    {},
	Conductor:      {},
	Email:          {},
}

func TestFrameActionsArePinned(t *testing.T) {
	for _, name := range systemNames() {
		want, pinned := pinnedFrame[name]
		if !pinned {
			t.Fatalf("desk %s is in the manifest but not in the pinned authority table: add it there, in review", name)
		}
		d, _ := Lookup(name)
		if got := sorted(d.FrameActions()); !reflect.DeepEqual(got, sorted(want)) {
			t.Errorf("desk %s frame actions = %v, pinned %v", name, got, sorted(want))
		}
	}
}

// The message targets and capsule verbs of the tables the manifest replaced.
func TestMessageTargetsAndCapsuleVerbsKeepTheirOldAuthority(t *testing.T) {
	targets := map[string][]string{
		Management: {Texture, Research}, Texture: {Research, Management}, Research: {Texture},
		Engineering: {Management}, Conductor: {}, Email: {},
	}
	for name, want := range targets {
		d, ok := Lookup(name)
		if !ok {
			t.Fatalf("desk %s missing", name)
		}
		if !reflect.DeepEqual(sorted(d.MessageTargets), sorted(want)) {
			t.Errorf("desk %s message targets = %v, want %v", name, d.MessageTargets, want)
		}
	}
	verbs := map[string][]string{
		Management: {},
		Research:   {"read_file", "list_dir", "stat", "go_eval"},
		Engineering: {"exec", "read_file", "write_file", "list_dir", "stat", "mkdir", "remove",
			"kill_session", "go_eval", "init_session", "close_session", "job_start", "job_status", "job_stop"},
	}
	for name, want := range verbs {
		d, _ := Lookup(name)
		if !reflect.DeepEqual(sorted(d.CapsuleVerbs), sorted(want)) {
			t.Errorf("desk %s capsule verbs = %v, want %v", name, d.CapsuleVerbs, want)
		}
	}
}

func TestLookupReturnsCopies(t *testing.T) {
	d, _ := Lookup(Research)
	d.Verbs[0] = "Exec"
	d.CapsuleVerbs[0] = "exec"
	d.MessageTargets = append(d.MessageTargets, Engineering)
	again, _ := Lookup(Research)
	if again.Verbs[0] == "Exec" || again.CapsuleVerbs[0] == "exec" || len(again.MessageTargets) != 1 {
		t.Fatal("a caller changed the manifest through a returned copy")
	}
}

// ARCH-05 and SEC-01: a worker class that must not signal or read another has
// its own uid; the only shared uids are a desk and its own role.
func TestCellUIDsAreDistinctPerWorkerClass(t *testing.T) {
	byUID := map[uint32][]string{}
	for _, name := range systemNames() {
		d, _ := Lookup(name)
		if d.Carrier != CarrierDeskWorker {
			if d.Sandbox.UID != 0 {
				t.Errorf("desk %s has no worker but a uid", name)
			}
			continue
		}
		if d.Sandbox.UID < 61000 {
			t.Errorf("desk %s uid %d is outside the cell range", name, d.Sandbox.UID)
		}
		byUID[d.Sandbox.UID] = append(byUID[d.Sandbox.UID], name)
	}
	for uid, names := range byUID {
		sort.Strings(names)
		joined := strings.Join(names, ",")
		switch joined {
		case Management + "," + ManagementTask, ManagementTask + "," + Management, ResearchChild + "," + Research, Research + "," + ResearchChild, Texture:
		default:
			t.Errorf("uid %d is shared by %s", uid, joined)
		}
	}
	// An untrusted-input desk never shares a uid with a privileged one.
	for _, a := range systemNames() {
		for _, b := range systemNames() {
			da, _ := Lookup(a)
			db, _ := Lookup(b)
			if da.Carrier == CarrierDeskWorker && db.Carrier == CarrierDeskWorker && da.Sandbox.UID == db.Sandbox.UID && da.Taint != db.Taint {
				t.Errorf("%s (%s) and %s (%s) share uid %d", a, da.Taint, b, db.Taint, da.Sandbox.UID)
			}
		}
	}
}

func TestTaintClasses(t *testing.T) {
	want := map[string]Taint{
		Research: TaintUntrustedInput, ResearchChild: TaintUntrustedInput, Email: TaintUntrustedInput,
		Management: TaintPrivileged, Texture: TaintPrivileged, Engineering: TaintPrivileged,
		ManagementTask: TaintPrivileged, Conductor: TaintPrivileged,
	}
	for name, taint := range want {
		d, _ := Lookup(name)
		if d.Taint != taint {
			t.Errorf("desk %s taint = %s, want %s", name, d.Taint, taint)
		}
	}
}

func TestWorkerBudgets(t *testing.T) {
	for _, name := range systemNames() {
		d, _ := Lookup(name)
		if d.Carrier != CarrierDeskWorker {
			continue
		}
		m := d.Memory
		if m.WorkerMaxBytes == 0 || (m.WorkerHighBytes != 0 && m.WorkerHighBytes >= m.WorkerMaxBytes) {
			t.Errorf("desk %s: memory.max %d must be set, and memory.high %d (if any) below it", name, m.WorkerMaxBytes, m.WorkerHighBytes)
		}
		if m.WorkerHighBytes != 0 {
			t.Errorf("desk %s: a worker budget is the hard limit alone: memory.high throttles a runaway cell for minutes instead of killing it", name)
		}
		if m.WorkerMaxBytes > 1<<30 {
			t.Errorf("desk %s: a worker budget over 1 GiB defeats the 8 GiB guest's aggregate", name)
		}
	}
	e, _ := Lookup(Engineering)
	if e.Memory.CapsuleRequestedBytes != 2<<30 {
		t.Errorf("engineering capsule request = %d, want the 2 GiB the link was measured against", e.Memory.CapsuleRequestedBytes)
	}
}

var appSeq atomic.Int32

func goodApp() AppDeskSpec {
	return AppDeskSpec{Name: fmt.Sprintf("app:t%d", appSeq.Add(1)), Verbs: []string{"Note", "Ask", "ReadCapture"}, Taint: TaintPrivileged, UID: 62001, WorkerMaxBytes: 256 * mib}
}

func TestAppDeskCannotHoldMoreThanASystemDeskGives(t *testing.T) {
	bad := map[string]func(*AppDeskSpec){
		"exec verb":        func(s *AppDeskSpec) { s.Verbs = append(s.Verbs, "Exec") },
		"write_file verb":  func(s *AppDeskSpec) { s.Verbs = append(s.Verbs, "WriteFile") },
		"product api":      func(s *AppDeskSpec) { s.Verbs = append(s.Verbs, "ProductAPI") },
		"world read":       func(s *AppDeskSpec) { s.Verbs = append(s.Verbs, "Read") },
		"research":         func(s *AppDeskSpec) { s.Verbs = append(s.Verbs, "Research") },
		"jobs":             func(s *AppDeskSpec) { s.Verbs = append(s.Verbs, "Start") },
		"verify":           func(s *AppDeskSpec) { s.Verbs = append(s.Verbs, "Verify") },
		"delegate":         func(s *AppDeskSpec) { s.Verbs = append(s.Verbs, "Delegate") },
		"system name":      func(s *AppDeskSpec) { s.Name = "management" },
		"unprefixed":       func(s *AppDeskSpec) { s.Name = "podcast" },
		"bad slug":         func(s *AppDeskSpec) { s.Name = "app:Pod Cast" },
		"no taint":         func(s *AppDeskSpec) { s.Taint = "" },
		"unknown taint":    func(s *AppDeskSpec) { s.Taint = "trusted" },
		"system-range uid": func(s *AppDeskSpec) { s.UID = 61003 },
		"root":             func(s *AppDeskSpec) { s.UID = 0 },
		"no memory":        func(s *AppDeskSpec) { s.WorkerMaxBytes = 0 },
		"huge memory":      func(s *AppDeskSpec) { s.WorkerMaxBytes = 4 << 30 },
	}
	for name, mutate := range bad {
		spec := goodApp()
		mutate(&spec)
		if _, err := NewAppDesk(spec); err == nil {
			t.Errorf("NewAppDesk accepted %s", name)
		}
	}
	d, err := NewAppDesk(goodApp())
	if err != nil {
		t.Fatalf("a valid app desk was refused: %v", err)
	}
	for _, a := range d.HandleActions() {
		if a != "read_file" && a != "list_dir" && a != "read_capture" {
			t.Errorf("app desk holds %s", a)
		}
	}
}

func TestRegisterAppFailsClosed(t *testing.T) {
	d, err := NewAppDesk(goodApp())
	if err != nil {
		t.Fatal(err)
	}
	if FrameAllowed(d.Name, "read_capture") {
		t.Fatal("an app desk was served before it registered")
	}
	if err := RegisterApp(d); err != nil {
		t.Fatalf("register: %v", err)
	}
	if !FrameAllowed(d.Name, "read_capture") || FrameAllowed(d.Name, "product_api") || FrameAllowed(d.Name, "read") {
		t.Fatal("registered app desk's frame table is wrong")
	}
	if err := RegisterApp(d); err == nil {
		t.Fatal("registering the same app desk twice must fail")
	}
	sys, _ := Lookup(Management)
	if err := RegisterApp(sys); err == nil {
		t.Fatal("RegisterApp replaced a system desk")
	}
	widened := d
	widened.Name = "app:other"
	widened.Verbs = append(append([]string{}, d.Verbs...), "ProductAPI")
	if err := RegisterApp(widened); err == nil {
		t.Fatal("RegisterApp accepted an app desk with ProductAPI")
	}
	forged := d
	forged.Name = "app:forged"
	forged.Kind = KindSystem
	if err := RegisterApp(forged); err == nil {
		t.Fatal("RegisterApp accepted a non-app desk")
	}
}

func TestRestartContractIsUniform(t *testing.T) {
	for _, name := range systemNames() {
		d, _ := Lookup(name)
		if d.Restart != RestartContract {
			t.Errorf("desk %s restart contract %q", name, d.Restart)
		}
	}
}

// The guest image declares one user per desk worker class so operators see
// names; the spawner uses the manifest's number. They must be the same
// number, or an operator reading `ps` is told the wrong desk.
func TestGuestImageUsersMatchTheManifest(t *testing.T) {
	raw, err := os.ReadFile("../../nix/autoputer-vm.nix")
	if err != nil {
		t.Skipf("nix/autoputer-vm.nix not readable from the package: %v", err)
	}
	nix := string(raw)
	for _, desk := range []string{Management, Texture, Research} {
		d, _ := Lookup(desk)
		name := "choir-cell-" + desk
		uidRe := regexp.MustCompile(`users\.users\.` + name + ` = \{[^}]*uid = (\d+);`)
		gidRe := regexp.MustCompile(`users\.groups\.` + name + ` = \{ gid = (\d+); \}`)
		m := uidRe.FindStringSubmatch(nix)
		g := gidRe.FindStringSubmatch(nix)
		if m == nil || g == nil {
			t.Errorf("guest image does not declare user and group %s with numeric ids", name)
			continue
		}
		want := fmt.Sprint(d.Sandbox.UID)
		if m[1] != want || g[1] != want {
			t.Errorf("%s: image uid %s gid %s, manifest uid %s", name, m[1], g[1], want)
		}
	}
	if !strings.Contains(nix, `CHOIR_DESK_CELL_CONFINEMENT = "required";`) {
		t.Error("the guest image does not set CHOIR_DESK_CELL_CONFINEMENT=required")
	}
	// Required mode refuses a worker with no memory budget; the image must
	// therefore name the desks cgroup the daemon makes at start.
	if !strings.Contains(nix, `CHOIR_DESK_CGROUP_ROOT = "/sys/fs/cgroup/choir-desks";`) {
		t.Error("the guest image does not name the desks cgroup (CHOIR_DESK_CGROUP_ROOT)")
	}
	if !strings.Contains(nix, `MemoryMin = "512M";`) {
		t.Error("the guest image does not protect the daemon's memory (MemoryMin)")
	}
}

// SDC SEC-15 / SEC-09 / SEC-10, guest side: the guest image sets kernel
// hardening sysctls, does not claim KSM deduplicates guests, and takes its
// resolver from the host when the host names one. Pinned here because the
// image is built by Nix on the host and not exercised by Go tests.
func TestGuestImageHardeningIsDeclared(t *testing.T) {
	raw, err := os.ReadFile("../../nix/autoputer-vm.nix")
	if err != nil {
		t.Skipf("nix/autoputer-vm.nix not readable from the package: %v", err)
	}
	nix := string(raw)
	for _, key := range []string{"kernel.kptr_restrict", "kernel.dmesg_restrict", "kernel.unprivileged_bpf_disabled",
		"net.core.bpf_jit_harden", "kernel.kexec_load_disabled", "kernel.sysrq", "net.ipv4.conf.all.rp_filter",
		"net.ipv4.conf.all.accept_redirects", "net.ipv4.conf.all.send_redirects"} {
		if !strings.Contains(nix, `"`+key+`" =`) {
			t.Errorf("guest image does not set sysctl %s", key)
		}
	}
	// Capsules need user namespaces; the runtime creates one per capsule.
	if strings.Contains(nix, `"user.max_user_namespaces"`) {
		t.Error("the guest image restricts user namespaces, which every capsule needs")
	}
	if strings.Contains(nix, "with KSM deduplication") || strings.Contains(nix, "Combined with KSM") {
		t.Error("the guest image still claims KSM deduplicates guests (SEC-09: KSM is off)")
	}
	if !strings.Contains(nix, "choir.dns=*)") || !strings.Contains(nix, `echo "DNS=$dns_ip"`) {
		t.Error("the guest network unit does not take choir.dns from the kernel command line (SEC-10)")
	}
	// SEC-10 step 3: a guest with the host resolver drops resolved's
	// compiled-in public fallback, and only that guest: every other guest
	// has no link DNS= and resolves through the fallback today.
	dnsBranch := ""
	if i := strings.Index(nix, `if [ -n "$dns_ip" ]; then`); i >= 0 {
		dnsBranch = nix[i:]
		if j := strings.Index(dnsBranch, "\n      fi\n"); j > 0 {
			dnsBranch = dnsBranch[:j]
		}
	}
	if !strings.Contains(dnsBranch, "/run/systemd/resolved.conf.d/") || !strings.Contains(dnsBranch, "FallbackDNS=\n") {
		t.Error("a guest given choir.dns keeps resolved's public fallback servers (SEC-10 step 3)")
	}
	// resolved is already running when the drop-in lands; without a restart
	// it keeps the fallback it may have picked for the whole boot.
	if !strings.Contains(dnsBranch, "systemctl restart --no-block systemd-resolved.service") {
		t.Error("the choir.dns branch writes the no-fallback drop-in but never restarts resolved to read it")
	}
	if strings.Contains(nix, "FallbackDNS") && !strings.Contains(dnsBranch, "FallbackDNS") {
		t.Error("FallbackDNS is set outside the choir.dns branch, which would strip the only resolver of an unselected guest")
	}
}

func TestRetireAppRemovesOnlyAppDesks(t *testing.T) {
	d, err := NewAppDesk(goodApp())
	if err != nil {
		t.Fatal(err)
	}
	if err := RegisterApp(d); err != nil {
		t.Fatal(err)
	}
	if err := RetireApp(d.Name); err != nil {
		t.Fatalf("retire: %v", err)
	}
	if _, ok := Lookup(d.Name); ok || FrameAllowed(d.Name, "read_capture") {
		t.Fatal("a retired app desk is still known")
	}
	if err := RetireApp(d.Name); err == nil {
		t.Fatal("retiring an unknown desk must fail")
	}
	if err := RetireApp(Management); err == nil {
		t.Fatal("RetireApp removed a system desk")
	}
	if _, ok := Lookup(Management); !ok {
		t.Fatal("management is gone")
	}
}

// The worker carry (ARCH-03): a registered app desk renders back to the spec
// that made it, so the worker can rebuild it through NewAppDesk.
func TestAppSpecRoundTrips(t *testing.T) {
	spec := goodApp()
	spec.OwnedKinds = []string{"choir.app.x.note"}
	d, err := NewAppDesk(spec)
	if err != nil {
		t.Fatal(err)
	}
	back, err := NewAppDesk(d.AppSpec())
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if !back.Equal(d) {
		t.Fatalf("round trip changed the desk: %+v vs %+v", back, d)
	}
	other := back
	other.Verbs = append(other.Verbs, "Cast")
	if other.Equal(d) {
		t.Fatal("Equal ignores verbs")
	}
}
