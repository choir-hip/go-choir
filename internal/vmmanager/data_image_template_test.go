package vmmanager

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
)

// A new computer's data image is reflinked from a prebuilt, identity-free,
// seeded template instead of being built (git clone + mkfs.ext4 -d of a
// 32 GiB file) under the global manager lock
// (docs/problems/new-computer-creation-14s-2026-10-26.md, step 1).
//
// Failure modes pinned here, written before the code:
//
//   - switch off: the template path runs at all (a template dir appears, a
//     build happens) when VMCTL_DATA_IMAGE_TEMPLATE is unset;
//   - existing computers: a boot that is not a first creation (refresh,
//     recover, a lost data.img, a recovery's staged image) uses the template;
//   - template missing: a new computer fails, or builds in place under the
//     lock, instead of the template being built and then cloned;
//   - stale seed identity: a template built for an earlier seed (another
//     commit, other refs) is cloned after the seed moved, or a stale template
//     is used when the new build fails; a seed that moves during the build
//     is promoted under the old name;
//   - reflink unsupported: a refused FICLONE fails the boot, or leaves a
//     partial data.img, instead of falling back to today's build;
//   - crash mid-build: a half-built template is ever promoted or cloned; a
//     failed build leaves a final template or its temporary file behind;
//   - concurrent first creations: two creations racing on a missing template
//     build it twice, or one clones a template the other is still writing;
//   - per-computer data: a template whose seed tree holds anything but the
//     clean public checkout (an untracked file, a stray root entry, a
//     per-computer file name, an image root entry mkfs did not get from the
//     seed) is promoted;
//   - lock: the global manager lock is held while the template is built,
//     or not held while the template is cloned into the VM's directory.

const templateTestSizeMB = 8

// templateStubTools puts stub mkfs.ext4, resize2fs and debugfs first on
// PATH (git stays reachable). mkfs fails a template build while the file
// named by failMkfs exists; debugfs prints the listing file the test names
// (or a clean seeded root).
func templateStubTools(t *testing.T) (mkfs, failMkfs, listing string) {
	t.Helper()
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	failMkfs = filepath.Join(dir, "fail-template-mkfs")
	listing = filepath.Join(dir, "debugfs-listing")
	clean := "/2/040755/0/0/./\n/2/040755/0/0/../\n/11/040700/0/0/lost+found/\n/12/040755/0/0/files/\n"
	if err := os.WriteFile(listing, []byte(clean), 0o644); err != nil {
		t.Fatal(err)
	}
	scripts := map[string]string{
		"mkfs.ext4": fmt.Sprintf("#!/bin/sh\ncase \"$*\" in *.building-*) [ -f %q ] && exit 1;; esac\nexit 0\n", failMkfs),
		"resize2fs": "#!/bin/sh\nexit 0\n",
		"debugfs":   fmt.Sprintf("#!/bin/sh\ncat %q\n", listing),
	}
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return filepath.Join(binDir, "mkfs.ext4"), failMkfs, listing
}

func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{"-C", dir, "-c", "user.email=t@example.invalid", "-c", "user.name=t", "-c", "commit.gpgsign=false"}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// seedRepo makes a small source repository with one commit.
func seedRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := filepath.Join(t.TempDir(), "go-choir")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitT(t, repo, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, repo, "add", ".")
	gitT(t, repo, "commit", "-q", "-m", "seed")
	return repo
}

func commitIn(t *testing.T, repo, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, repo, "add", ".")
	gitT(t, repo, "commit", "-q", "-m", "change "+name)
}

func withLocalCommit(t *testing.T) {
	t.Helper()
	old := buildinfo.Commit
	buildinfo.Commit = "local"
	t.Cleanup(func() { buildinfo.Commit = old })
}

// templateManager is a manager with the switch on, a seed repo, stub tools
// and a sparse (non-copying) clone so no test writes a real image.
func templateManager(t *testing.T, enabled bool) (*Manager, string) {
	t.Helper()
	withLocalCommit(t)
	mkfs, _, _ := templateStubTools(t)
	repo := seedRepo(t)
	cfg := DefaultManagerConfig()
	cfg.StateDir = t.TempDir()
	cfg.MkfsExt4Path = mkfs
	cfg.SourceSeedRepoPath = repo
	cfg.StoreDiskPath = "/nonexistent/store.erofs"
	cfg.DataImageTemplate = DataImageTemplateConfig{Enabled: enabled}
	m := NewManager(cfg)
	m.templateCloneFn = sparseCloneForTest
	return m, repo
}

// sparseCloneForTest stands in for FICLONE: a sparse file of the source's
// size, so tests never copy a 32 GiB image.
func sparseCloneForTest(dst, src *os.File) error {
	info, err := src.Stat()
	if err != nil {
		return err
	}
	return dst.Truncate(info.Size())
}

func templateFiles(t *testing.T, dir string) (final, building []string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		switch {
		case strings.HasPrefix(e.Name(), "data-") && strings.HasSuffix(e.Name(), ".img"):
			final = append(final, e.Name())
		case strings.HasPrefix(e.Name(), ".building-"):
			building = append(building, e.Name())
		}
	}
	return final, building
}

func newDataImgPath(t *testing.T, m *Manager, vmID string) string {
	t.Helper()
	dir := filepath.Join(m.cfg.StateDir, vmID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "data.img")
}

func TestDataImageTemplateSwitchIsOffByDefault(t *testing.T) {
	t.Setenv("VMCTL_DATA_IMAGE_TEMPLATE", "")
	t.Setenv("VMCTL_DATA_IMAGE_TEMPLATE_DIR", "")
	if cfg := LoadConfigFromEnv(); cfg.DataImageTemplate.Enabled {
		t.Fatal("data image template on without VMCTL_DATA_IMAGE_TEMPLATE")
	}
	t.Setenv("VMCTL_DATA_IMAGE_TEMPLATE", "on")
	t.Setenv("VMCTL_DATA_IMAGE_TEMPLATE_DIR", "/srv/templates")
	cfg := LoadConfigFromEnv()
	if !cfg.DataImageTemplate.Enabled || cfg.DataImageTemplate.Dir != "/srv/templates" {
		t.Fatalf("switch not read: %+v", cfg.DataImageTemplate)
	}
	if got := NewManager(ManagerConfig{StateDir: "/var/lib/go-choir/vm-state"}).dataImageTemplateDir(); got != "/var/lib/go-choir/vm-state/.data-image-templates" {
		t.Fatalf("default template dir = %q", got)
	}
}

func TestDataImageTemplateSwitchOffBuildsInPlace(t *testing.T) {
	m, _ := templateManager(t, false)
	builds := 0
	m.templateStagingHook = func(string) error { builds++; return nil }
	pick := m.dataImageTemplateForBoot(VMConfig{VMID: "vm-off", NewComputer: true}, templateTestSizeMB)
	if pick.Path != "" || pick.Reason != "switch_off" {
		t.Fatalf("pick = %+v, want switch_off", pick)
	}
	dataImg := newDataImgPath(t, m, "vm-off")
	source, err := m.prepareNewDataImage(dataImg, templateTestSizeMB, pick)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if !strings.HasPrefix(source, "built") {
		t.Fatalf("source = %q, want built", source)
	}
	if builds != 0 {
		t.Fatal("a template was built with the switch off")
	}
	if _, err := os.Stat(m.dataImageTemplateDir()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("template dir exists with the switch off: %v", err)
	}
	if got := modeOf(t, dataImg); got != 0o600 {
		t.Fatalf("data image mode = %o", got)
	}
}

func TestDataImageTemplateNeverServesAnExistingComputer(t *testing.T) {
	m, _ := templateManager(t, true)
	builds := 0
	m.templateStagingHook = func(string) error { builds++; return nil }
	for _, kind := range []string{"", "refresh", "recover", "resume"} {
		pick := m.dataImageTemplateForBoot(VMConfig{VMID: "vm-existing", BootKind: kind}, templateTestSizeMB)
		if pick.Path != "" || pick.Reason != "not_new_computer" {
			t.Fatalf("boot kind %q: pick = %+v, want not_new_computer", kind, pick)
		}
	}
	// A recovery's staged successor image is today's build, never a clone.
	root := m.cfg.StateDir
	if err := os.MkdirAll(filepath.Join(root, "vm-recover"), 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := m.StageSparseImage(root, "vm-recover", 2, "optemplate", templateTestSizeMB); err != nil {
		t.Fatalf("StageSparseImage: %v", err)
	}
	if builds != 0 {
		t.Fatalf("%d template builds for existing computers", builds)
	}
	if final, building := templateFiles(t, m.dataImageTemplateDir()); len(final)+len(building) != 0 {
		t.Fatalf("templates appeared for existing computers: %v %v", final, building)
	}
}

func TestDataImageTemplateMissingIsBuiltThenCloned(t *testing.T) {
	m, _ := templateManager(t, true)
	lockFree := false
	m.templateStagingHook = func(staging string) error {
		if m.mu.TryLock() {
			lockFree = true
			m.mu.Unlock()
		}
		return nil
	}
	pick := m.dataImageTemplateForBoot(VMConfig{VMID: "vm-new", NewComputer: true}, templateTestSizeMB)
	if pick.Path == "" {
		t.Fatalf("no template: %+v", pick)
	}
	if !lockFree {
		t.Fatal("the template was built while the global manager lock was held")
	}
	if len(pick.ID) != 64 || filepath.Base(pick.Path) != "data-"+pick.ID+".img" || filepath.Dir(pick.Path) != m.dataImageTemplateDir() {
		t.Fatalf("template is not content-addressed beside the state: %+v", pick)
	}
	if got := modeOf(t, pick.Path); got != 0o400 {
		t.Fatalf("template mode = %o, want 400", got)
	}
	want, err := m.dataImageSeedIdentity(templateTestSizeMB)
	if err != nil || want != pick.ID {
		t.Fatalf("template id %s, seed identity %s (%v)", pick.ID, want, err)
	}
	dataImg := newDataImgPath(t, m, "vm-new")
	source, err := m.prepareNewDataImage(dataImg, templateTestSizeMB, pick)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if !strings.HasPrefix(source, "template") || !strings.Contains(source, pick.ID[:12]) {
		t.Fatalf("source = %q, want the template", source)
	}
	info, err := os.Stat(dataImg)
	if err != nil || info.Size() != int64(templateTestSizeMB)*1024*1024 {
		t.Fatalf("cloned image: %v %v", info, err)
	}
	if got := modeOf(t, dataImg); got != 0o600 {
		t.Fatalf("cloned image mode = %o, want 600", got)
	}
	// A second creation reuses the template without building.
	m.templateStagingHook = func(string) error { t.Fatal("rebuilt a valid template"); return nil }
	if again := m.dataImageTemplateForBoot(VMConfig{VMID: "vm-new-2", NewComputer: true}, templateTestSizeMB); again.Path != pick.Path {
		t.Fatalf("second pick = %+v", again)
	}
}

func TestDataImageTemplateStaleSeedIdentityIsNotUsed(t *testing.T) {
	m, repo := templateManager(t, true)
	_, failMkfs, _ := templateStubToolsPaths(t, m)
	first := m.dataImageTemplateForBoot(VMConfig{VMID: "vm-a", NewComputer: true}, templateTestSizeMB)
	if first.Path == "" {
		t.Fatalf("first: %+v", first)
	}

	// The seed moves (a deploy updated the source checkout): a new
	// identity, a new template, and the old one is pruned.
	commitIn(t, repo, "NEXT.md", "next\n")
	second := m.dataImageTemplateForBoot(VMConfig{VMID: "vm-b", NewComputer: true}, templateTestSizeMB)
	if second.Path == "" || second.ID == first.ID {
		t.Fatalf("a moved seed reused the old template: %+v vs %+v", second, first)
	}
	if final, _ := templateFiles(t, m.dataImageTemplateDir()); len(final) != 1 || final[0] != filepath.Base(second.Path) {
		t.Fatalf("templates after a rebuild = %v, want only %s", final, filepath.Base(second.Path))
	}

	// The seed moves again and the build fails: the previous template is
	// stale and is not used; the creation builds in place.
	commitIn(t, repo, "THIRD.md", "third\n")
	if err := os.WriteFile(failMkfs, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	third := m.dataImageTemplateForBoot(VMConfig{VMID: "vm-c", NewComputer: true}, templateTestSizeMB)
	if third.Path != "" || !strings.HasPrefix(third.Reason, "build_failed") {
		t.Fatalf("failed build after a seed move: pick = %+v", third)
	}
	if err := os.Remove(failMkfs); err != nil {
		t.Fatal(err)
	}

	// A picked template that vanished or changed before the clone is not
	// cloned: today's build runs.
	fourth := m.dataImageTemplateForBoot(VMConfig{VMID: "vm-d", NewComputer: true}, templateTestSizeMB)
	if fourth.Path == "" {
		t.Fatalf("fourth: %+v", fourth)
	}
	if err := os.Chmod(fourth.Path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(fourth.Path); err != nil {
		t.Fatal(err)
	}
	dataImg := newDataImgPath(t, m, "vm-d")
	source, err := m.prepareNewDataImage(dataImg, templateTestSizeMB, fourth)
	if err != nil || !strings.HasPrefix(source, "built") {
		t.Fatalf("vanished template: source=%q err=%v", source, err)
	}
}

// A seed that moves while the template builds must not be promoted under
// the identity read before the build.
func TestDataImageTemplateSeedMovingDuringBuildIsDiscarded(t *testing.T) {
	m, repo := templateManager(t, true)
	moved := false
	m.templateStagingHook = func(string) error {
		if !moved {
			moved = true
			commitIn(t, repo, "RACE.md", "race\n")
		}
		return nil
	}
	pick := m.dataImageTemplateForBoot(VMConfig{VMID: "vm-race", NewComputer: true}, templateTestSizeMB)
	if pick.Path != "" || !strings.Contains(pick.Reason, "seed moved") {
		t.Fatalf("pick = %+v, want a discarded build", pick)
	}
	if final, building := templateFiles(t, m.dataImageTemplateDir()); len(final)+len(building) != 0 {
		t.Fatalf("a build whose seed moved left %v %v", final, building)
	}
}

func templateStubToolsPaths(t *testing.T, m *Manager) (mkfs, failMkfs, listing string) {
	t.Helper()
	dir := filepath.Dir(filepath.Dir(m.cfg.MkfsExt4Path))
	return m.cfg.MkfsExt4Path, filepath.Join(dir, "fail-template-mkfs"), filepath.Join(dir, "debugfs-listing")
}

func TestDataImageTemplateReflinkUnsupportedFallsBack(t *testing.T) {
	m, _ := templateManager(t, true)
	pick := m.dataImageTemplateForBoot(VMConfig{VMID: "vm-noreflink", NewComputer: true}, templateTestSizeMB)
	if pick.Path == "" {
		t.Fatalf("pick: %+v", pick)
	}
	m.templateCloneFn = func(dst, src *os.File) error {
		_, _ = dst.Write([]byte("partial"))
		return errors.ErrUnsupported
	}
	dataImg := newDataImgPath(t, m, "vm-noreflink")
	source, err := m.prepareNewDataImage(dataImg, templateTestSizeMB, pick)
	if err != nil {
		t.Fatalf("a refused reflink failed the creation: %v", err)
	}
	if !strings.HasPrefix(source, "built") || !strings.Contains(source, "reflink") {
		t.Fatalf("source = %q, want built with the reflink reason", source)
	}
	info, err := os.Stat(dataImg)
	if err != nil || info.Size() != int64(templateTestSizeMB)*1024*1024 {
		t.Fatalf("fallback image: %v %v", info, err)
	}
}

func TestDataImageTemplateCrashMidBuildLeavesNoTemplate(t *testing.T) {
	m, _ := templateManager(t, true)
	_, failMkfs, _ := templateStubToolsPaths(t, m)
	dir := m.dataImageTemplateDir()

	// A build that dies in mkfs promotes nothing and leaves nothing.
	if err := os.WriteFile(failMkfs, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	pick := m.dataImageTemplateForBoot(VMConfig{VMID: "vm-crash", NewComputer: true}, templateTestSizeMB)
	if pick.Path != "" || !strings.HasPrefix(pick.Reason, "build_failed") {
		t.Fatalf("failed build: pick = %+v", pick)
	}
	if final, building := templateFiles(t, dir); len(final)+len(building) != 0 {
		t.Fatalf("failed build left %v %v", final, building)
	}
	if err := os.Remove(failMkfs); err != nil {
		t.Fatal(err)
	}

	// A crashed process's temporary file (a half template) is never used,
	// even when it carries the current identity, and is swept by the next
	// build.
	id, err := m.dataImageSeedIdentity(templateTestSizeMB)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	half := filepath.Join(dir, ".building-"+id+"-123.img")
	if err := os.WriteFile(half, []byte("half"), 0o600); err != nil {
		t.Fatal(err)
	}
	pick = m.dataImageTemplateForBoot(VMConfig{VMID: "vm-after-crash", NewComputer: true}, templateTestSizeMB)
	if pick.Path == "" || pick.Path == half {
		t.Fatalf("after a crash: pick = %+v", pick)
	}
	if _, err := os.Stat(half); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("half template not swept: %v", err)
	}

	// A final-named template of the wrong size (torn) is refused and rebuilt.
	if err := os.Chmod(pick.Path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(pick.Path, 4096); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(pick.Path, 0o400); err != nil {
		t.Fatal(err)
	}
	rebuilt := m.dataImageTemplateForBoot(VMConfig{VMID: "vm-torn", NewComputer: true}, templateTestSizeMB)
	if rebuilt.Path == "" {
		t.Fatalf("torn template: %+v", rebuilt)
	}
	if info, err := os.Stat(rebuilt.Path); err != nil || info.Size() != int64(templateTestSizeMB)*1024*1024 {
		t.Fatalf("torn template not rebuilt: %v %v", info, err)
	}
}

func TestDataImageTemplateConcurrentFirstCreationsBuildOnce(t *testing.T) {
	m, _ := templateManager(t, true)
	var builds atomic.Int32
	m.templateStagingHook = func(string) error {
		builds.Add(1)
		time.Sleep(150 * time.Millisecond)
		return nil
	}
	const n = 8
	picks := make([]dataImageTemplatePick, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			picks[i] = m.dataImageTemplateForBoot(VMConfig{VMID: fmt.Sprintf("vm-race-%d", i), NewComputer: true}, templateTestSizeMB)
		}(i)
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("concurrent picks did not finish")
	}
	if got := builds.Load(); got != 1 {
		t.Fatalf("%d builds for %d racing first creations, want 1", got, n)
	}
	for i, p := range picks {
		if p.Path == "" || p.Path != picks[0].Path {
			t.Fatalf("pick %d = %+v, want %s", i, p, picks[0].Path)
		}
	}
}

// A waiter is bounded: a build that outlasts the wait falls the creation
// back to today's path rather than blocking it.
func TestDataImageTemplateWaitIsBounded(t *testing.T) {
	m, _ := templateManager(t, true)
	m.templateWait = 50 * time.Millisecond
	release := make(chan struct{})
	m.templateStagingHook = func(string) error {
		select {
		case <-release:
		case <-time.After(10 * time.Second):
		}
		return nil
	}
	pick := m.dataImageTemplateForBoot(VMConfig{VMID: "vm-slow", NewComputer: true}, templateTestSizeMB)
	close(release)
	if pick.Path != "" || pick.Reason != "wait_timeout" {
		t.Fatalf("pick = %+v, want wait_timeout", pick)
	}
	// The build goes on for the next creation; let it finish before the
	// test's cleanup.
	deadline := time.Now().Add(20 * time.Second)
	for {
		m.templateMu.Lock()
		inFlight := len(m.templateFlights)
		m.templateMu.Unlock()
		if inFlight == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the abandoned build never finished")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if final, _ := templateFiles(t, m.dataImageTemplateDir()); len(final) != 1 {
		t.Fatalf("the build a waiter gave up on did not finish: %v", final)
	}
}

func TestDataImageTemplateRefusesPerComputerData(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(t *testing.T, m *Manager, repo string)
		staging func(staging string) error
		reason  string
	}{
		{
			name: "untracked file in the seed checkout",
			staging: func(staging string) error {
				return os.WriteFile(filepath.Join(staging, "files", "Source", "platform", "notes.txt"), []byte("x"), 0o600)
			},
			reason: "unsafe",
		},
		{
			name: "per-computer file at the image root",
			staging: func(staging string) error {
				if err := os.MkdirAll(filepath.Join(staging, "choir-credentials"), 0o700); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(staging, "choir-credentials", "privacy-key"), []byte("k"), 0o600)
			},
			reason: "unsafe",
		},
		{
			name: "per-computer file name tracked in the seed",
			mutate: func(t *testing.T, m *Manager, repo string) {
				commitIn(t, repo, "gateway-token", "tok\n")
			},
			reason: "unsafe",
		},
		{
			name: "image root entry not from the seed",
			mutate: func(t *testing.T, m *Manager, repo string) {
				_, _, listing := templateStubToolsPaths(t, m)
				extra := "/2/040755/0/0/./\n/2/040755/0/0/../\n/11/040700/0/0/lost+found/\n/12/040755/0/0/files/\n/13/040711/0/0/choir-signers/\n"
				if err := os.WriteFile(listing, []byte(extra), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			reason: "unsafe",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, repo := templateManager(t, true)
			if tc.mutate != nil {
				tc.mutate(t, m, repo)
			}
			if tc.staging != nil {
				m.templateStagingHook = tc.staging
			}
			pick := m.dataImageTemplateForBoot(VMConfig{VMID: "vm-dirty", NewComputer: true}, templateTestSizeMB)
			if pick.Path != "" || !strings.Contains(pick.Reason, tc.reason) {
				t.Fatalf("pick = %+v, want refusal %q", pick, tc.reason)
			}
			if final, building := templateFiles(t, m.dataImageTemplateDir()); len(final)+len(building) != 0 {
				t.Fatalf("a template with per-computer data was kept: %v %v", final, building)
			}
		})
	}
}

// The whole creation, through BootVM: the template is built with the global
// manager lock free, and the clone into the VM's directory runs with it held.
func TestBootVMHoldsTheGlobalLockOnlyForTheClone(t *testing.T) {
	m, _ := templateManager(t, true)
	m.cfg.StoreDiskPath = "/nonexistent/store.erofs"
	m.cfg.KernelImagePath = "/nonexistent/kernel"
	m.cfg.RootfsPath = "/nonexistent/rootfs"
	h, _ := newFakeHostNetWithEgress(t, newFakeIPTables(t), newFakeIP(nil))
	m.hostNet = h
	m.firecrackerStarter = func(string, []string) error { return errors.New("test: no firecracker") }

	entered := make(chan struct{})
	release := make(chan struct{})
	m.templateStagingHook = func(string) error {
		close(entered)
		select {
		case <-release:
		case <-time.After(20 * time.Second):
		}
		return nil
	}
	var cloned, heldDuringClone bool
	m.templateCloneFn = func(dst, src *os.File) error {
		cloned = true
		if m.mu.TryLock() {
			m.mu.Unlock()
		} else {
			heldDuringClone = true
		}
		return sparseCloneForTest(dst, src)
	}

	bootDone := make(chan error, 1)
	go func() {
		_, err := m.BootVM(VMConfig{VMID: "vm-locktest", NewComputer: true})
		bootDone <- err
	}()
	select {
	case <-entered:
	case <-time.After(20 * time.Second):
		t.Fatal("template build never started")
	}
	if !m.mu.TryLock() {
		close(release)
		t.Fatal("the global manager lock is held while the template builds")
	}
	m.mu.Unlock()
	close(release)
	select {
	case <-bootDone:
	case <-time.After(30 * time.Second):
		t.Fatal("BootVM did not return")
	}
	if !cloned {
		t.Fatal("the new computer's data image was not cloned from the template")
	}
	if !heldDuringClone {
		t.Fatal("the clone ran without the global manager lock")
	}
	if _, err := os.Stat(filepath.Join(m.cfg.StateDir, "vm-locktest", "data.img")); err != nil {
		t.Fatalf("data image after the clone: %v", err)
	}
}

// With the real tools (git, mkfs.ext4, debugfs) the template holds the seed
// and nothing else, and a real FICLONE (or, where the filesystem refuses
// it, today's build) gives the computer its image.
func TestDataImageTemplateWithRealTools(t *testing.T) {
	mkfs, err1 := exec.LookPath("mkfs.ext4")
	_, err2 := exec.LookPath("debugfs")
	if err1 != nil || err2 != nil {
		t.Skip("mkfs.ext4/debugfs not available")
	}
	withLocalCommit(t)
	repo := seedRepo(t)
	cfg := DefaultManagerConfig()
	cfg.StateDir = t.TempDir()
	cfg.MkfsExt4Path = mkfs
	cfg.SourceSeedRepoPath = repo
	cfg.StoreDiskPath = "/nonexistent/store.erofs"
	cfg.DataImageTemplate = DataImageTemplateConfig{Enabled: true}
	m := NewManager(cfg)
	pick := m.dataImageTemplateForBoot(VMConfig{VMID: "vm-real", NewComputer: true}, 64)
	if pick.Path == "" {
		t.Fatalf("real build: %+v", pick)
	}
	dataImg := newDataImgPath(t, m, "vm-real")
	source, err := m.prepareNewDataImage(dataImg, 64, pick)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	t.Logf("real tools: data image source %s", source)
	out, err := exec.Command("debugfs", "-R", "cat /files/Source/platform/README.md", dataImg).Output()
	if err != nil || strings.TrimSpace(string(out)) != "seed" {
		t.Fatalf("seed not in the computer's image: %q %v", out, err)
	}
}
