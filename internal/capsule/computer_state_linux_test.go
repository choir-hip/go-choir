//go:build linux

package capsule

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/ownerfiles"
)

// The capsule overlay's files layer (S2 failure modes 1 to 5,
// docs/design/capsule-overlay-computer-state-2026-11-03.md).

func unprivilegedLayerOwner(t *testing.T) {
	t.Helper()
	owner, group := computerLayerOwner, computerLayerGroup
	computerLayerOwner, computerLayerGroup = os.Getuid(), os.Getgid()
	t.Cleanup(func() { computerLayerOwner, computerLayerGroup = owner, group })
}

func writeLayerTree(t *testing.T, root string, files map[string]os.FileMode) {
	t.Helper()
	for rel, mode := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("content of "+rel), mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}
}

func receiptReasons(r ComputerStateReceipt) map[string]string {
	out := map[string]string{}
	for _, l := range r.LeftOut {
		out[l.Path] = l.Reason
	}
	return out
}

func TestComputerStateLayerLinksKeptFilesAndNamesEveryLeaveOut(t *testing.T) {
	unprivilegedLayerOwner(t)
	root := filepath.Join(t.TempDir(), "files")
	writeLayerTree(t, root, map[string]os.FileMode{
		"notes/a.md":                            0o644,
		"deep/nested/dir/b.txt":                 0o644,
		"private/canary.txt":                    0o600,
		"Source/platform/README.md":             0o644,
		"Source/other/kept.go":                  0o644,
		"mail/new/1.eml":                        0o644,
		"notes/.a.md.tmp-123":                   0o644,
		"Self-development/op/screens/.screen-9": 0o644,
	})
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside the root"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "notes", "link.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(outside), filepath.Join(root, "linkdir")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, "notes", "fifo"), 0o644); err != nil {
		t.Fatal(err)
	}
	sock := filepath.Join(root, "s.sock")
	if l, err := net.Listen("unix", sock); err == nil {
		t.Cleanup(func() { _ = l.Close() })
	}
	base := t.TempDir()
	e := &Executor{stateDir: t.TempDir()}
	e.SetComputerFilesRoot(root, filepath.Join(root, "Source", "platform"))
	layer, receipt := e.buildComputerStateLayer(context.Background(), base)
	if layer == "" || !receipt.Files {
		t.Fatalf("layer refused: %q", receipt.Refused)
	}
	files := filepath.Join(layer, "computer", "files")
	for _, rel := range []string{"notes/a.md", "deep/nested/dir/b.txt", "Source/other/kept.go"} {
		src, _ := os.Stat(filepath.Join(root, rel))
		dst, err := os.Stat(filepath.Join(files, rel))
		if err != nil {
			t.Fatalf("%s not linked: %v", rel, err)
		}
		if !os.SameFile(src, dst) {
			t.Fatalf("%s is a copy, not a link", rel)
		}
	}
	for _, rel := range []string{"private/canary.txt", "Source/platform", "mail", "notes/.a.md.tmp-123", "Self-development/op/screens/.screen-9", "notes/link.txt", "linkdir", "notes/fifo"} {
		if _, err := os.Lstat(filepath.Join(files, rel)); err == nil {
			t.Fatalf("%s reached the layer", rel)
		}
	}
	want := map[string]string{
		"Source/platform":                       LeaveOutSource,
		"mail":                                  LeaveOutMail,
		"notes/.a.md.tmp-123":                   LeaveOutTemporary,
		"Self-development/op/screens/.screen-9": LeaveOutTemporary,
		"notes/link.txt":                        LeaveOutSymlink,
		"linkdir":                               LeaveOutSymlink,
		"notes/fifo":                            LeaveOutSpecial,
	}
	got := receiptReasons(receipt)
	if _, named := got["private/canary.txt"]; named || receipt.LeftOutCounts[LeaveOutNotReadable] != 1 {
		t.Fatalf("a not-o+r file must be counted, never named: %+v %v", receipt.LeftOut, receipt.LeftOutCounts)
	}
	for rel, reason := range want {
		if got[rel] != reason {
			t.Fatalf("leave-out %s = %q, want %q (receipt %+v)", rel, got[rel], reason, receipt.LeftOut)
		}
	}
	// Directories are 0755 and owned by the layer owner; the canary's
	// bytes are nowhere in the layer.
	_ = filepath.Walk(files, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			t.Fatal(err)
		}
		if info.IsDir() && info.Mode().Perm() != 0o755 {
			t.Fatalf("%s mode %v, want 0755", path, info.Mode().Perm())
		}
		if info.Mode().IsRegular() {
			raw, _ := os.ReadFile(path)
			if bytes.Contains(raw, []byte("content of private/canary.txt")) {
				t.Fatalf("canary reached %s", path)
			}
		}
		return nil
	})
	if receipt.LinkedFiles != 3 || receipt.LockedAt.IsZero() {
		t.Fatalf("receipt = %+v", receipt)
	}
	upper := t.TempDir()
	if err := writeComputerStateReceipt(upper, receipt); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(upper, "computer", "receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	var onDisk ComputerStateReceipt
	if err := json.Unmarshal(raw, &onDisk); err != nil || onDisk.LinkedFiles != 3 {
		t.Fatalf("receipt.json = %s (%v)", raw, err)
	}
}

// Destroying the layer must not change the live file a link shares
// (failure mode 1: removePrivateTree used to chmod every file 0600).
func TestRemovingTheLayerLeavesLiveFilesUnchanged(t *testing.T) {
	unprivilegedLayerOwner(t)
	root := filepath.Join(t.TempDir(), "files")
	writeLayerTree(t, root, map[string]os.FileMode{"notes/a.md": 0o644, "b/c.txt": 0o444})
	base := t.TempDir()
	e := &Executor{stateDir: t.TempDir()}
	e.SetComputerFilesRoot(root, filepath.Join(root, "Source", "platform"))
	layer, receipt := e.buildComputerStateLayer(context.Background(), base)
	if layer == "" {
		t.Fatalf("refused: %s", receipt.Refused)
	}
	if err := removePrivateTree(base); err != nil {
		t.Fatal(err)
	}
	for rel, mode := range map[string]os.FileMode{"notes/a.md": 0o644, "b/c.txt": 0o444} {
		info, err := os.Stat(filepath.Join(root, rel))
		if err != nil || info.Mode().Perm() != mode {
			t.Fatalf("%s after destroy: mode %v err %v, want %v", rel, info.Mode().Perm(), err, mode)
		}
		raw, _ := os.ReadFile(filepath.Join(root, rel))
		if string(raw) != "content of "+rel {
			t.Fatalf("%s bytes changed: %q", rel, raw)
		}
	}
}

// The walk holds the files write lock: a writer replacing a file through
// the helper never shows half a version in the layer (failure mode 4).
func TestComputerStateLayerIsOneInstantUnderConcurrentWrites(t *testing.T) {
	unprivilegedLayerOwner(t)
	root := filepath.Join(t.TempDir(), "files")
	if err := os.MkdirAll(filepath.Join(root, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	const n = 50
	for i := 0; i < n; i++ {
		if err := ownerfiles.WriteFile(filepath.Join(root, "d", fmt.Sprintf("f%02d", i)), bytes.Repeat([]byte{'0'}, 4096), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var stop atomic.Bool
	var wg sync.WaitGroup
	var generation atomic.Int64
	wg.Add(1)
	go func() {
		defer wg.Done()
		for g := byte('1'); !stop.Load(); g = '1' + (g-'0')%9 {
			// One generation writes every file, holding one Tx, so a
			// snapshot between generations sees all files equal.
			tx := ownerfiles.Begin()
			for i := 0; i < n; i++ {
				_ = tx.WriteFile(filepath.Join(root, "d", fmt.Sprintf("f%02d", i)), bytes.Repeat([]byte{g}, 4096), 0o644)
			}
			tx.End()
			generation.Add(1)
		}
	}()
	for round := 0; round < 20; round++ {
		base := t.TempDir()
		e := &Executor{stateDir: t.TempDir()}
		e.SetComputerFilesRoot(root, "")
		layer, receipt := e.buildComputerStateLayer(context.Background(), base)
		if layer == "" {
			t.Fatalf("refused: %s", receipt.Refused)
		}
		var first []byte
		for i := 0; i < n; i++ {
			raw, err := os.ReadFile(filepath.Join(layer, "computer", "files", "d", fmt.Sprintf("f%02d", i)))
			if err != nil || len(raw) != 4096 {
				t.Fatalf("round %d f%02d: %d bytes, %v", round, i, len(raw), err)
			}
			if first == nil {
				first = raw
			} else if !bytes.Equal(first, raw) {
				t.Fatalf("round %d: the layer mixes two generations", round)
			}
		}
		time.Sleep(time.Millisecond)
	}
	stop.Store(true)
	wg.Wait()
	if generation.Load() == 0 {
		t.Fatal("the writer never ran")
	}
}

// A layer it cannot build is removed and named; the spawn goes on
// (failure mode 5).
func TestComputerStateLayerRefusalLeavesNoLayer(t *testing.T) {
	base := t.TempDir()
	e := &Executor{stateDir: t.TempDir()}
	e.SetComputerFilesRoot(filepath.Join(t.TempDir(), "missing"), "")
	layer, receipt := e.buildComputerStateLayer(context.Background(), base)
	if layer != "" || receipt.Files || receipt.Refused == "" {
		t.Fatalf("layer=%q receipt=%+v", layer, receipt)
	}
	if _, err := os.Lstat(filepath.Join(base, computerLowerName)); err == nil {
		t.Fatal("a refused layer was left on disk")
	}
	e.SetComputerFilesRoot("", "")
	if layer, receipt := e.buildComputerStateLayer(context.Background(), base); layer != "" || receipt.Refused == "" {
		t.Fatalf("no files root: layer=%q receipt=%+v", layer, receipt)
	}
}

// Writes under /computer in the upper are scratch: the manifest walk never
// reports them (failure mode 7).
func TestWalkUpperdirSkipsTheComputerStateLayer(t *testing.T) {
	upper := t.TempDir()
	writeLayerTree(t, upper, map[string]os.FileMode{
		"computer/files/notes/a.md":  0o644,
		"computer/state.db":          0o644,
		"workspace/platform/main.go": 0o644,
	})
	manifests, err := walkUpperdir(context.Background(), upper)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range manifests {
		if m.Path == "computer" || filepath.Dir(m.Path) == "computer" || len(m.Path) > 9 && m.Path[:9] == "computer/" {
			t.Fatalf("walk reported %s", m.Path)
		}
	}
	found := false
	for _, m := range manifests {
		found = found || m.Path == "workspace/platform/main.go"
	}
	if !found {
		t.Fatalf("the source change is missing: %+v", manifests)
	}
}

type fakeStoreSource struct {
	estimate int64
	err      error
	wrote    bool
}

func (f *fakeStoreSource) SnapshotEstimate(context.Context) (int64, error) { return f.estimate, nil }

func (f *fakeStoreSource) SnapshotForCapsule(_ context.Context, dir string, _ []string) (ComputerStoreCopy, error) {
	f.wrote = true
	if err := os.WriteFile(filepath.Join(dir, "state.db"), []byte("copy"), 0o600); err != nil {
		return ComputerStoreCopy{}, err
	}
	if err := os.Mkdir(filepath.Join(dir, "state.texture"), 0o755); err != nil {
		return ComputerStoreCopy{}, err
	}
	if f.err != nil {
		return ComputerStoreCopy{}, f.err
	}
	return ComputerStoreCopy{Sequence: 7, Head: "h7", Bytes: 4}, nil
}

// The store copy is charged against the state volume's floor, owned by the
// capsule's mapped uid, and a refused or failed copy leaves no file
// (S3 failure modes 3, 5 and 9).
func TestComputerStoreCopyIsChargedOwnedAndCleanedUp(t *testing.T) {
	unprivilegedLayerOwner(t)
	restore := computerStoreFreeBytes
	t.Cleanup(func() { computerStoreFreeBytes = restore })

	// Fits: free is the floor plus the copy and the allowance.
	computerStoreFreeBytes = func(string) (int64, error) { return StateFloorBytes + 3*computerStoreAllowance, nil }
	upper := t.TempDir()
	source := &fakeStoreSource{estimate: computerStoreAllowance / 2}
	e := &Executor{stateDir: t.TempDir()}
	e.SetComputerStore(source)
	receipt := &ComputerStateReceipt{}
	e.writeComputerStoreCopy(context.Background(), upper, SpawnSpec{StoreKinds: []string{"choir.run"}, StoreLeaseID: "lease-x"}, receipt)
	if receipt.Store == nil || receipt.Store.Sequence != 7 || receipt.StoreRefused != "" {
		t.Fatalf("receipt = %+v", receipt)
	}
	for _, rel := range []string{"computer", "computer/state.db", "computer/state.texture"} {
		info, err := os.Lstat(filepath.Join(upper, rel))
		if err != nil {
			t.Fatal(err)
		}
		if st := info.Sys().(*syscall.Stat_t); int(st.Uid) != computerLayerOwner || int(st.Gid) != computerLayerGroup {
			t.Fatalf("%s owned by %d:%d", rel, st.Uid, st.Gid)
		}
	}

	if receipt.Store.LeaseID != "lease-x" {
		t.Fatalf("the copy does not name its lease: %+v", receipt.Store)
	}

	// No lease names store kinds: no copy (MA3.4, X6).
	upper = t.TempDir()
	source = &fakeStoreSource{estimate: 1}
	e.SetComputerStore(source)
	receipt = &ComputerStateReceipt{}
	e.writeComputerStoreCopy(context.Background(), upper, SpawnSpec{}, receipt)
	if receipt.Store != nil || receipt.StoreRefused == "" || source.wrote {
		t.Fatalf("a copy was taken without a lease: %+v wrote=%t", receipt, source.wrote)
	}

	// Does not fit: refused before any copy, nothing written.
	computerStoreFreeBytes = func(string) (int64, error) { return StateFloorBytes + computerStoreAllowance, nil }
	upper = t.TempDir()
	source = &fakeStoreSource{estimate: 1 << 30}
	e.SetComputerStore(source)
	receipt = &ComputerStateReceipt{}
	e.writeComputerStoreCopy(context.Background(), upper, SpawnSpec{StoreKinds: []string{"choir.run"}, StoreLeaseID: "lease-x"}, receipt)
	if receipt.Store != nil || receipt.StoreRefused == "" || source.wrote {
		t.Fatalf("an oversize copy was taken: %+v wrote=%t", receipt, source.wrote)
	}
	if _, err := os.Lstat(filepath.Join(upper, "computer", "state.db")); err == nil {
		t.Fatal("a refused copy left a file")
	}

	// Fails mid-copy: the partial files are removed.
	computerStoreFreeBytes = func(string) (int64, error) { return StateFloorBytes + 3*computerStoreAllowance, nil }
	upper = t.TempDir()
	source = &fakeStoreSource{estimate: 1, err: fmt.Errorf("a blanking was refused")}
	e.SetComputerStore(source)
	receipt = &ComputerStateReceipt{}
	e.writeComputerStoreCopy(context.Background(), upper, SpawnSpec{StoreKinds: []string{"choir.run"}, StoreLeaseID: "lease-x"}, receipt)
	if receipt.Store != nil || receipt.StoreRefused == "" {
		t.Fatalf("receipt = %+v", receipt)
	}
	for _, rel := range []string{"computer/state.db", "computer/state.texture"} {
		if _, err := os.Lstat(filepath.Join(upper, rel)); err == nil {
			t.Fatalf("a failed copy left %s", rel)
		}
	}
}

// A walk that cannot build a kept entry fails whole: a layer is whole or
// absent (round-1 review, DeepSeek 1).
func TestSnapshotWalkFailsWholeWhenAnEntryCannotBeBuilt(t *testing.T) {
	unprivilegedLayerOwner(t)
	root := filepath.Join(t.TempDir(), "files")
	writeLayerTree(t, root, map[string]os.FileMode{"a/b.txt": 0o644, "c.txt": 0o644})
	dest := t.TempDir()
	if err := os.Chmod(dest, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dest, 0o755) })
	if os.Geteuid() == 0 {
		t.Skip("root ignores the read-only destination")
	}
	receipt := &ComputerStateReceipt{}
	if err := snapshotFilesTree(context.Background(), root, dest, nil, receipt); err == nil {
		t.Fatalf("the walk reported success over an unwritable layer: %+v", receipt)
	}
	// A cancelled context stops the walk.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := snapshotFilesTree(ctx, root, t.TempDir(), nil, &ComputerStateReceipt{}); err == nil {
		t.Fatal("a cancelled walk succeeded")
	}
}

// The files root and the capsule state must be disjoint, and the source
// leave-out follows the configured source root, not a literal.
func TestComputerStateLayerRootRules(t *testing.T) {
	unprivilegedLayerOwner(t)
	root := filepath.Join(t.TempDir(), "files")
	writeLayerTree(t, root, map[string]os.FileMode{"code/src/main.go": 0o644, "Source/platform/kept.md": 0o644})
	e := &Executor{stateDir: filepath.Join(root, "inner-state")}
	e.SetComputerFilesRoot(root, filepath.Join(root, "code", "src"))
	if layer, receipt := e.buildComputerStateLayer(context.Background(), t.TempDir()); layer != "" || !strings.Contains(receipt.Refused, "overlap") {
		t.Fatalf("an overlapping state dir was accepted: %q %+v", layer, receipt)
	}
	e.stateDir = t.TempDir()
	layer, receipt := e.buildComputerStateLayer(context.Background(), t.TempDir())
	if layer == "" {
		t.Fatalf("refused: %s", receipt.Refused)
	}
	if receiptReasons(receipt)["code/src"] != LeaveOutSource {
		t.Fatalf("the configured source root was not left out: %+v", receipt.LeftOut)
	}
	if _, err := os.Stat(filepath.Join(layer, "computer", "files", "Source", "platform", "kept.md")); err != nil {
		t.Fatalf("a path that is not the configured source was dropped: %v", err)
	}
}

// The spawn's decision (problem 26): a capsule holding the owner's raw
// model credential gets no files layer and no store copy, and its receipt
// says withheld; an implementation capsule and a gateway claude-mode
// capsule (relay secret only) get both, with the same leave-outs.
func TestPrepareComputerStateWithholdsOnlyARawCredential(t *testing.T) {
	unprivilegedLayerOwner(t)
	restore := computerStoreFreeBytes
	t.Cleanup(func() { computerStoreFreeBytes = restore })
	computerStoreFreeBytes = func(string) (int64, error) { return StateFloorBytes + 3*computerStoreAllowance, nil }
	root := filepath.Join(t.TempDir(), "files")
	writeLayerTree(t, root, map[string]os.FileMode{
		"notes/a.md":         0o644,
		"private/canary.txt": 0o600,
		"mail/cur/1":         0o644,
		"Source/platform/x":  0o644,
	})
	relay, err := NewClaudeRelaySecret()
	if err != nil {
		t.Fatal(err)
	}
	const envSentinel = "p26-env-sentinel-opaque-value"
	var provided []ComputerStateReceipt
	for _, c := range []struct {
		name  string
		spec  SpawnSpec
		token string
		held  bool
	}{
		{"implementation", SpawnSpec{ComputerState: true, Egress: true}, "", false},
		{"gateway claude mode", SpawnSpec{ComputerState: true, Egress: true, ClaudeDelegate: true}, relay, false},
		{"raw token for the broker", SpawnSpec{ComputerState: true, Egress: true, ClaudeDelegate: true}, "sk-ant-oat01-FAKE-0123456789abcdef", true},
		{"raw token in the env", SpawnSpec{ComputerState: true, Egress: true, ClaudeDelegate: true, Env: []string{"CLAUDE_CODE_OAUTH_TOKEN=" + envSentinel}}, relay, true},
	} {
		base, upper := t.TempDir(), t.TempDir()
		source := &fakeStoreSource{estimate: 1}
		e := &Executor{stateDir: t.TempDir()}
		e.SetComputerFilesRoot(root, filepath.Join(root, "Source", "platform"))
		e.SetComputerStore(source)
		receipt, layer := e.prepareComputerState(context.Background(), c.spec, c.token, base)
		if receipt.ComputerState == ComputerStateProvided {
			e.writeComputerStoreCopy(context.Background(), upper, SpawnSpec{StoreKinds: []string{"choir.run"}, StoreLeaseID: "lease-x"}, receipt)
		}
		if err := writeComputerStateReceipt(upper, *receipt); err != nil {
			t.Fatal(err)
		}
		_, layerErr := os.Lstat(filepath.Join(base, computerLowerName))
		raw, _ := os.ReadFile(filepath.Join(upper, "computer", "receipt.json"))
		if strings.Contains(string(raw), relay) || strings.Contains(string(raw), "sk-ant-") || strings.Contains(string(raw), envSentinel) {
			t.Fatalf("%s: the receipt names a credential (%d bytes)", c.name, len(raw))
		}
		if c.held {
			if layer != "" || layerErr == nil || source.wrote || receipt.ComputerState != ComputerStateWithheldRawCredential || !strings.Contains(string(raw), ComputerStateWithheldRawCredential) {
				t.Fatalf("%s: layer=%q layerOnDisk=%t storeCopied=%t receipt=%s", c.name, layer, layerErr == nil, source.wrote, raw)
			}
			continue
		}
		if layer == "" || !source.wrote || receipt.ComputerState != ComputerStateProvided {
			t.Fatalf("%s: layer=%q storeCopied=%t receipt=%s", c.name, layer, source.wrote, raw)
		}
		files := filepath.Join(layer, "computer", "files")
		if _, err := os.Lstat(filepath.Join(files, "notes", "a.md")); err != nil {
			t.Fatalf("%s: a kept file is missing: %v", c.name, err)
		}
		for _, rel := range []string{"private/canary.txt", "mail", "Source/platform"} {
			if _, err := os.Lstat(filepath.Join(files, rel)); err == nil {
				t.Fatalf("%s: %s reached the layer", c.name, rel)
			}
		}
		provided = append(provided, *receipt)
	}
	// The leave-outs are exactly those of an implementation capsule.
	if len(provided) != 2 {
		t.Fatalf("provided %d receipts, want 2", len(provided))
	}
	impl, claude := provided[0], provided[1]
	if fmt.Sprint(receiptReasons(impl)) != fmt.Sprint(receiptReasons(claude)) || fmt.Sprint(impl.LeftOutCounts) != fmt.Sprint(claude.LeftOutCounts) || impl.LinkedFiles != claude.LinkedFiles {
		t.Fatalf("claude-mode leave-outs differ: implementation %v %v, claude %v %v", impl.LeftOut, impl.LeftOutCounts, claude.LeftOut, claude.LeftOutCounts)
	}
	if want := map[string]string{"mail": LeaveOutMail, "Source/platform": LeaveOutSource}; receiptReasons(claude)["mail"] != want["mail"] || receiptReasons(claude)["Source/platform"] != want["Source/platform"] || claude.LeftOutCounts[LeaveOutNotReadable] != 1 {
		t.Fatalf("claude-mode leave-outs = %v %v", claude.LeftOut, claude.LeftOutCounts)
	}
}
