package vmmanager

// Guest tool layers (docs/design/guest-tool-layers-2026-10-31.md). Failure
// modes first (design 1):
//
//   T1 the data drive moves, or a tool slot is missing from the drives: every
//      slot comes after the base, in the release's order, read-only, and the
//      data drive stays second (/dev/vdb).
//   T2 the profile picks the wrong file: a named selector attaches the layer,
//      "none" attaches the placeholder, a grant selector follows that grant,
//      and a layer the profile does not name is attached (never taken away).
//   T3 a tool file (layer or placeholder) is missing: the release never
//      loads, and a ring computer cold boots the single image.
//   T4 a schema mismatch: schema 1 naming tools, schema 2 naming none, a bad
//      name, a repeated name or a bad id never loads; a schema 1 layered
//      release (the current layered path) still loads with no tool drives.
//   T5 a resumed VM sees other bytes behind a slot: tool disks are bound to
//      the snapshot (a replaced file refuses), and a profile change between
//      hibernate and wake refuses the wake.
//   T6 a reattach after a profile change binds today's profile instead of
//      what the VM runs: it takes the launch record's tool disks.
//   T7 retention cannot see a tool layer: the in_use answer names every tool
//      id (layer and placeholder) of a release in use.
//   T8 a single-image or schema 1 launch grows tool drives: it has none.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var testToolSlots = []releaseLayerTool{
	{Name: "browser", ID: "d1111111111111111111111111111111", EmptyID: "d2222222222222222222222222222222"},
	{Name: "delegate", ID: "d3333333333333333333333333333333", EmptyID: "d4444444444444444444444444444444"},
	{Name: "office", ID: "d5555555555555555555555555555555", EmptyID: "d6666666666666666666666666666666"},
	{Name: "toolchain", ID: "d7777777777777777777777777777777", EmptyID: "d8888888888888888888888888888888"},
}

func testToolsDir(releases string) string {
	return filepath.Join(filepath.Dir(releases), "guest-tools")
}

func writeTestToolFile(t *testing.T, releases, id string) string {
	t.Helper()
	d := filepath.Join(testToolsDir(releases), id)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(d, "tool.erofs")
	_ = os.Remove(p)
	if err := os.WriteFile(p, []byte("tool-"+id), 0o444); err != nil {
		t.Fatal(err)
	}
	return p
}

// writeTestToolRelease makes id a schema 2 layered release with the test
// tool slots, as node-b-guest-release-materialize does.
func writeTestToolRelease(t *testing.T, releases, id, baseID string, p testRelPolicy) {
	t.Helper()
	writeTestLayeredRelease(t, releases, id, baseID, p)
	raw, err := json.Marshal(releaseLayers{Schema: 2, Layout: "layered", BaseID: baseID, BaseStorePath: "/nix/store/" + baseID + "-guest-store-base.erofs", Tools: testToolSlots})
	if err != nil {
		t.Fatal(err)
	}
	lp := filepath.Join(releases, id, "layers.json")
	_ = os.Remove(lp)
	if err := os.WriteFile(lp, raw, 0o444); err != nil {
		t.Fatal(err)
	}
	for _, s := range testToolSlots {
		writeTestToolFile(t, releases, s.ID)
		writeTestToolFile(t, releases, s.EmptyID)
	}
}

const toolTestProfile = "toolchain=*;office=none;browser=*;delegate=grant:claude-delegate"

// newToolRingFixture: R1 single is the fleet's current; R2 is a schema 2
// layered release with tool slots and the layered link's target; ring
// computer d@e launched from R2 under profile.
func newToolRingFixture(t *testing.T, profile string) *releaseFixture {
	t.Helper()
	f := newSnapshotFixture(t)
	cpu := "cpu-test"
	stubHostCPU(t, &cpu, nil)
	root := f.mgr.cfg.StateDir
	r := &releaseFixture{snapshotFixture: f, releases: filepath.Join(root, "guest-releases"), link: filepath.Join(root, "guest-link")}
	writeTestRelease(t, r.releases, relR1, defaultRelPolicy(2*time.Hour))
	lp := defaultRelPolicy(time.Hour)
	lp.Digest = "sha256:floor-layered"
	writeTestToolRelease(t, r.releases, relR2, baseB1, lp)
	f.mgr.cfg.SnapshotReleaseRetain = true
	f.mgr.cfg.GuestReleasesDir = r.releases
	f.mgr.cfg.GuestReleaseLink = r.link
	f.mgr.cfg.GuestReleaseSupportWindow = 168 * time.Hour
	f.mgr.cfg.GuestLayeredLink = filepath.Join(root, "guest-layered-link")
	f.mgr.cfg.GuestLayeredComputers = []string{ringComputer}
	f.mgr.cfg.GuestToolProfile = parseToolProfile(profile)
	f.mgr.cfg.ClaudeDelegate = ClaudeDelegateConfig{Computers: map[string]bool{}}
	r.point(t, relR1)
	r.pointLayered(t, relR2)

	vmCfg := f.cfg
	vmCfg.ComputerID = ringComputer
	f.expect.ComputerID = ringComputer
	vmCfg.KernelImagePath, vmCfg.InitrdPath, vmCfg.RootfsPath, vmCfg.StoreDiskPath, vmCfg.KernelParams = "", "", "", "", ""
	f.mgr.bindCurrentRelease(&vmCfg)
	f.mgr.applyLaunchDefaults(&vmCfg)
	if vmCfg.GuestRelease != relR2 || len(vmCfg.ToolDisks) != len(testToolSlots) {
		t.Fatalf("setup: tool launch not bound: release %q tools %+v", vmCfg.GuestRelease, vmCfg.ToolDisks)
	}
	hostPort, _ := f.mgr.hostPortFromHostURL(f.guestS.URL)
	launch, err := json.Marshal(f.mgr.buildFirecrackerConfig(vmCfg, hostPort))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.mgr.vmStateDir(f.vmID), "fc-config.json"), launch, 0o644); err != nil {
		t.Fatal(err)
	}
	f.cfg = vmCfg
	f.inst.Config = vmCfg
	if err := f.mgr.writeLaunchRecord(vmCfg); err != nil {
		t.Fatal(err)
	}
	return r
}

func toolChoice(disks []ToolDisk) string {
	var parts []string
	for _, d := range disks {
		state := "empty"
		if d.Attached {
			state = "layer"
		}
		parts = append(parts, d.Name+"="+state)
	}
	return strings.Join(parts, ",")
}

// T1.
func TestToolLayerDriveOrder(t *testing.T) {
	r := newToolRingFixture(t, toolTestProfile)
	hostPort, _ := r.mgr.hostPortFromHostURL(r.guestS.URL)
	cfg := r.cfg
	cfg.credentialDiskPath = filepath.Join(r.mgr.vmStateDir(cfg.VMID), "credential.img")
	fc := r.mgr.buildFirecrackerConfig(cfg, hostPort)
	want := "store,data,credential,base,tool-browser,tool-delegate,tool-office,tool-toolchain"
	if got := strings.Join(driveIDs(fc), ","); got != want {
		t.Fatalf("drives = %s; want %s (data second, tools after base)", got, want)
	}
	drives := fc["drives"].([]map[string]interface{})
	for i, d := range drives[4:] {
		slot := testToolSlots[i]
		if d["is_read_only"] != true || d["is_root_device"] != false {
			t.Fatalf("tool drive %v is not read-only data", d)
		}
		wantPath := filepath.Join(testToolsDir(r.releases), slot.EmptyID, "tool.erofs")
		if slot.Name == "toolchain" || slot.Name == "browser" {
			wantPath = filepath.Join(testToolsDir(r.releases), slot.ID, "tool.erofs")
		}
		if d["path_on_host"] != wantPath {
			t.Fatalf("tool drive %s path = %v; want %s", slot.Name, d["path_on_host"], wantPath)
		}
	}
	// The jail names each slot by its drive id.
	_, files, err := rewriteFirecrackerConfigForJail(fc)
	if err != nil {
		t.Fatalf("jail rewrite: %v", err)
	}
	names := map[string]bool{}
	for _, f := range files {
		names[f.Name] = !f.Writable
	}
	for _, s := range testToolSlots {
		if !names["tool-"+s.Name+".img"] {
			t.Fatalf("jail files %+v lack a read-only tool-%s.img", files, s.Name)
		}
	}
}

// T2.
func TestToolLayerProfileChoice(t *testing.T) {
	r := newToolRingFixture(t, toolTestProfile)
	rel, err := r.mgr.releaseByID(relR2)
	if err != nil {
		t.Fatal(err)
	}
	base := VMConfig{VMID: "vm-x", ComputerID: ringComputer}
	// The browser is every computer's (browser=*), with no grant: a primary
	// computer and a test one both get it.
	if got := toolChoice(r.mgr.toolDisksFor(base, rel)); got != "browser=layer,delegate=empty,office=empty,toolchain=layer" {
		t.Fatalf("d@e with no grants = %s", got)
	}
	test := base
	test.WarmnessClass = "test"
	if got := toolChoice(r.mgr.toolDisksFor(test, rel)); got != "browser=layer,delegate=empty,office=empty,toolchain=layer" {
		t.Fatalf("class:test computer = %s", got)
	}
	r.mgr.cfg.ClaudeDelegate = ClaudeDelegateConfig{Computers: map[string]bool{ringComputer: true}}
	if got := toolChoice(r.mgr.toolDisksFor(base, rel)); got != "browser=layer,delegate=layer,office=empty,toolchain=layer" {
		t.Fatalf("delegate-listed computer = %s", got)
	}
	// Unnamed layers are attached; an empty profile attaches everything.
	r.mgr.cfg.GuestToolProfile = parseToolProfile("office=none")
	if got := toolChoice(r.mgr.toolDisksFor(base, rel)); got != "browser=layer,delegate=layer,office=empty,toolchain=layer" {
		t.Fatalf("office=none only = %s", got)
	}
	r.mgr.cfg.GuestToolProfile = parseToolProfile("")
	if got := toolChoice(r.mgr.toolDisksFor(base, rel)); got != "browser=layer,delegate=layer,office=layer,toolchain=layer" {
		t.Fatalf("empty profile = %s", got)
	}
	// Selector tokens: a named computer, class:, an empty selector, and a
	// malformed rule (ignored, so the layer stays attached).
	r.mgr.cfg.GuestToolProfile = parseToolProfile("office=computer-other " + ringComputer + ";toolchain=class:interactive;browser=;Bad Rule=*")
	if got := toolChoice(r.mgr.toolDisksFor(base, rel)); got != "browser=empty,delegate=layer,office=layer,toolchain=empty" {
		t.Fatalf("selector tokens = %s", got)
	}
	inter := base
	inter.WarmnessClass = "interactive"
	if got := toolChoice(r.mgr.toolDisksFor(inter, rel)); !strings.HasSuffix(got, "toolchain=layer") {
		t.Fatalf("class:interactive did not select: %s", got)
	}
	if s := toolProfileSummary(r.mgr.toolDisksFor(inter, rel)); s != "delegate,office,toolchain attached; browser empty" {
		t.Fatalf("summary = %q", s)
	}
}

// T3.
func TestToolLayerFileMissingFallsBackToSingle(t *testing.T) {
	for _, which := range []string{"layer", "placeholder"} {
		t.Run(which, func(t *testing.T) {
			r := newToolRingFixture(t, toolTestProfile)
			id := testToolSlots[2].ID // office, which the profile leaves empty
			if which == "placeholder" {
				id = testToolSlots[3].EmptyID // toolchain's, which the profile does not pick
			}
			if err := os.RemoveAll(filepath.Join(testToolsDir(r.releases), id)); err != nil {
				t.Fatal(err)
			}
			if _, err := r.mgr.releaseByID(relR2); err == nil {
				t.Fatalf("a release missing a tool %s loaded", which)
			}
			cfg := VMConfig{VMID: "vm-fallback", ComputerID: ringComputer, GuestPort: 8085}
			r.mgr.bindCurrentRelease(&cfg)
			if cfg.GuestRelease != relR1 || cfg.BaseDiskPath != "" || len(cfg.ToolDisks) != 0 {
				t.Fatalf("ring computer with a missing tool %s = release %q base %q tools %+v; want the single image R1", which, cfg.GuestRelease, cfg.BaseDiskPath, cfg.ToolDisks)
			}
		})
	}
}

// T4.
func TestToolLayerSchemaChecks(t *testing.T) {
	slots := func(mut func([]releaseLayerTool) []releaseLayerTool) string {
		s := append([]releaseLayerTool(nil), testToolSlots...)
		raw, _ := json.Marshal(mut(s))
		return string(raw)
	}
	same := func(s []releaseLayerTool) []releaseLayerTool { return s }
	for name, layers := range map[string]string{
		"schema 1 with tools":    fmt.Sprintf(`{"schema":1,"layout":"layered","base_id":%q,"tools":%s}`, baseB1, slots(same)),
		"schema 2 without tools": fmt.Sprintf(`{"schema":2,"layout":"layered","base_id":%q}`, baseB1),
		"schema 3":               fmt.Sprintf(`{"schema":3,"layout":"layered","base_id":%q,"tools":%s}`, baseB1, slots(same)),
		"bad name": fmt.Sprintf(`{"schema":2,"layout":"layered","base_id":%q,"tools":%s}`, baseB1, slots(func(s []releaseLayerTool) []releaseLayerTool {
			s[0].Name = "../x"
			return s
		})),
		"repeated name": fmt.Sprintf(`{"schema":2,"layout":"layered","base_id":%q,"tools":%s}`, baseB1, slots(func(s []releaseLayerTool) []releaseLayerTool {
			s[1].Name = s[0].Name
			return s
		})),
		"bad id": fmt.Sprintf(`{"schema":2,"layout":"layered","base_id":%q,"tools":%s}`, baseB1, slots(func(s []releaseLayerTool) []releaseLayerTool {
			s[0].ID = "../../etc/passwd"
			return s
		})),
		"layer is its own placeholder": fmt.Sprintf(`{"schema":2,"layout":"layered","base_id":%q,"tools":%s}`, baseB1, slots(func(s []releaseLayerTool) []releaseLayerTool {
			s[0].EmptyID = s[0].ID
			return s
		})),
	} {
		t.Run(name, func(t *testing.T) {
			r := newToolRingFixture(t, toolTestProfile)
			p := filepath.Join(r.releases, relR2, "layers.json")
			_ = os.Remove(p)
			if err := os.WriteFile(p, []byte(layers), 0o444); err != nil {
				t.Fatal(err)
			}
			if rel, err := r.mgr.releaseByID(relR2); err == nil {
				t.Fatalf("layers.json %s loaded: %+v", layers, rel)
			}
		})
	}
	// A tool layer that is a symlink is not a tool layer.
	r := newToolRingFixture(t, toolTestProfile)
	p := filepath.Join(testToolsDir(r.releases), testToolSlots[0].ID, "tool.erofs")
	_ = os.Remove(p)
	if err := os.Symlink("/etc/hostname", p); err != nil {
		t.Fatal(err)
	}
	if _, err := r.mgr.releaseByID(relR2); err == nil {
		t.Fatal("a symlinked tool layer loaded")
	}
}

// T8 and the schema 1 half of T4: the current layered path is unchanged.
func TestSchemaOneLayeredAndSingleHaveNoToolDrives(t *testing.T) {
	r := newLayeredFixture(t)
	r.mgr.cfg.GuestToolProfile = parseToolProfile(toolTestProfile)
	if len(r.cfg.ToolDisks) != 0 {
		t.Fatalf("schema 1 layered launch has tool disks: %+v", r.cfg.ToolDisks)
	}
	hostPort, _ := r.mgr.hostPortFromHostURL(r.guestS.URL)
	cfg := r.cfg
	cfg.credentialDiskPath = "/cred.img"
	if got := strings.Join(driveIDs(r.mgr.buildFirecrackerConfig(cfg, hostPort)), ","); got != "store,data,credential,base" {
		t.Fatalf("schema 1 layered drives = %s", got)
	}
	s := newReleaseFixture(t)
	s.mgr.cfg.GuestToolProfile = parseToolProfile(toolTestProfile)
	if len(s.cfg.ToolDisks) != 0 {
		t.Fatalf("single-image launch has tool disks: %+v", s.cfg.ToolDisks)
	}
	raw, _ := json.Marshal(s.cfg)
	if strings.Contains(string(raw), "ToolDisks") {
		t.Fatalf("a single-image config writes the tool field: %s", raw)
	}
	// A refresh never carries tool disks forward.
	if c := refreshConfigForCurrentDeploy(VMConfig{StoreDiskPath: "/a", ToolDisks: []ToolDisk{{Name: "x", Path: "/x"}}}, ManagerConfig{StoreDiskPath: "/s"}); len(c.ToolDisks) != 0 {
		t.Fatalf("refresh kept tool disks: %+v", c.ToolDisks)
	}
}

// T5.
func TestToolLayerSnapshotBinding(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate func(t *testing.T, r *releaseFixture)
		want   string
	}{
		"attached layer replaced": {func(t *testing.T, r *releaseFixture) {
			later := time.Now().Add(time.Hour)
			if err := os.Chtimes(filepath.Join(testToolsDir(r.releases), testToolSlots[3].ID, "tool.erofs"), later, later); err != nil {
				t.Fatal(err)
			}
		}, snapshotReasonDisk},
		"placeholder replaced": {func(t *testing.T, r *releaseFixture) {
			later := time.Now().Add(time.Hour)
			if err := os.Chtimes(filepath.Join(testToolsDir(r.releases), testToolSlots[2].EmptyID, "tool.erofs"), later, later); err != nil {
				t.Fatal(err)
			}
		}, snapshotReasonDisk},
		"profile now attaches office": {func(t *testing.T, r *releaseFixture) {
			r.mgr.cfg.GuestToolProfile = parseToolProfile("toolchain=*;office=*;browser=none;delegate=none")
		}, ""},
		"profile now drops toolchain": {func(t *testing.T, r *releaseFixture) {
			r.mgr.cfg.GuestToolProfile = parseToolProfile("toolchain=none;office=none;browser=none;delegate=none")
		}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			r := newToolRingFixture(t, toolTestProfile)
			dir, meta := r.hibernate(t)
			if len(meta.Config.ToolDisks) != len(testToolSlots) {
				t.Fatalf("meta carries tool disks %+v", meta.Config.ToolDisks)
			}
			bound := map[string]bool{}
			for _, d := range meta.Disks {
				bound[d.Path] = true
			}
			for _, td := range meta.Config.ToolDisks {
				if !bound[td.Path] {
					t.Fatalf("tool disk %s is not snapshot-bound: %+v", td.Path, meta.Disks)
				}
			}
			if got := r.validate(dir, meta); got != "" {
				t.Fatalf("untouched tool snapshot = %q, want a pass", got)
			}
			tc.mutate(t, r)
			got := r.validate(dir, meta)
			if tc.want != "" && got != tc.want {
				t.Fatalf("validateSnapshot = %q, want %s", got, tc.want)
			}
			if tc.want == "" && got == "" {
				t.Fatal("a snapshot whose tool choice moved still passed")
			}
		})
	}
}

// T5, the pass half: an unchanged profile resumes onto the same slots.
func TestToolLayerSnapshotResumesSameSlots(t *testing.T) {
	r := newToolRingFixture(t, toolTestProfile)
	dir, meta := r.hibernate(t)
	if got := r.validate(dir, meta); got != "" {
		t.Fatalf("validateSnapshot = %q", got)
	}
	var args []string
	r.restoreStarter(t, &fakeFirecrackerAPI{}, &args)
	inst, err := r.mgr.ResumeVMFromSnapshot(r.vmID, r.expect, nil)
	if err != nil {
		t.Fatalf("ResumeVMFromSnapshot: %v", err)
	}
	if got := toolChoice(inst.Config.ToolDisks); got != "browser=layer,delegate=empty,office=empty,toolchain=layer" {
		t.Fatalf("resumed tool slots = %s", got)
	}
}

// T6.
func TestToolLayerReattachKeepsLaunchedSlots(t *testing.T) {
	r := newToolRingFixture(t, toolTestProfile)
	r.mgr.cfg.GuestToolProfile = parseToolProfile("toolchain=none;office=*")
	cfg := r.mgr.reattachConfig(r.vmID, r.cfg.Epoch, VMConfig{ComputerID: ringComputer})
	if got := toolChoice(cfg.ToolDisks); got != "browser=layer,delegate=empty,office=empty,toolchain=layer" {
		t.Fatalf("reattached tool slots = %s; want the launch record's", got)
	}
	// A record that does not match the release is not trusted.
	rec, ok := r.mgr.readLaunchRecord(r.vmID, r.cfg.Epoch)
	if !ok {
		t.Fatal("no launch record")
	}
	rec.ToolDisks[0].Path = "/elsewhere"
	if err := r.mgr.storeLaunchRecord(r.vmID, rec); err != nil {
		t.Fatal(err)
	}
	cfg = r.mgr.reattachConfig(r.vmID, r.cfg.Epoch, VMConfig{ComputerID: ringComputer})
	if got := toolChoice(cfg.ToolDisks); got != "browser=layer,delegate=layer,office=layer,toolchain=empty" {
		t.Fatalf("reattach with a mismatched record = %s; want today's profile", got)
	}
}

// T7.
func TestGuestReleaseUsageNamesToolLayers(t *testing.T) {
	r := newToolRingFixture(t, toolTestProfile)
	r.hibernate(t)
	use := r.mgr.GuestReleaseUsage()
	for _, e := range use.InUse {
		if e.ID != relR2 {
			continue
		}
		want := []string{}
		for _, s := range testToolSlots {
			want = append(want, s.ID, s.EmptyID)
		}
		if strings.Join(e.ToolIDs, ",") != strings.Join(want, ",") || e.BaseID != baseB1 {
			t.Fatalf("in_use entry %+v; want tool_ids %v", e, want)
		}
		raw, _ := json.Marshal(e)
		if !strings.Contains(string(raw), `"tool_ids"`) {
			t.Fatalf("answer lacks tool_ids: %s", raw)
		}
		return
	}
	t.Fatalf("usage lacks R2: %+v", use)
}
