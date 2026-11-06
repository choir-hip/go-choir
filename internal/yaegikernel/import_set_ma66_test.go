package yaegikernel

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// MA6.6 code mode with per-desk import sets (plan section 12; design
// docs/design/ma66-code-mode-import-sets-2026-11-06.md). Failure modes,
// written before the code (design section 6):
//
//	F1 a desk's set holds a package with effects, or a verb its grants do not
//	   give (the set is not least-privilege);
//	F2 the set moves with something other than the desk's row (authoring order,
//	   a second computation), so its digest is not stable;
//	F3 a carried package list widens the worker past the desk's set, or an
//	   empty one falls back to a wider default;
//	F4 a cell imports outside its set and the static check lets it through;
//	F5 a cell's host calls are unbounded, or the bound refuses a hundred;
//	F6 a cell's parallel host calls are served one at a time;
//	F7 the intent quota is not the deliberate value.

var effectful = []string{"os", "os/exec", "os/signal", "net", "net/http", "net/rpc", "syscall", "unsafe", "reflect", "runtime", "plugin", "io/ioutil", "embed"}

func TestMA66EachDeskComputesItsOwnImportSet(t *testing.T) {
	app, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{Name: "app:ma66-notes", Verbs: []string{"Put", "Get", "List", "Retract"},
		Taint: deskmanifest.TaintUntrustedInput, UID: 62166, WorkerMaxBytes: 128 << 20, OwnedKinds: []string{"choir.app.ma66-notes.note"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := deskmanifest.RegisterApp(app); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deskmanifest.RetireApp(app.Name) })

	cases := []struct {
		desk     string
		has, not []string
	}{
		{deskmanifest.Management, []string{"Cast", "ProductAPI", "Stories", "Delegate", "Records"}, []string{"Write", "WriteFile", "Exec", "Read", "SearchTexture", "Put"}},
		{deskmanifest.Texture, []string{"Write", "ReadDoc", "Done", "Wait", "Records"}, []string{"ProductAPI", "Read", "WriteFile", "Delegate", "Put"}},
		{deskmanifest.Research, []string{"Read", "SearchStart", "SaveEvidence", "Research", "Records"}, []string{"Write", "WriteFile", "Exec", "ProductAPI", "Delegate"}},
		{deskmanifest.Engineering, []string{"WriteFile", "Exec", "Start", "Complete", "Freeze"}, []string{"Cast", "ProductAPI", "Read", "Write", "Delegate"}},
		{deskmanifest.ResearchWeb, []string{"Read", "SearchStart"}, []string{"SearchTexture", "Cast", "Report"}},
		{app.Name, []string{"Put", "Get", "List", "Records"}, []string{"Cast", "ProductAPI", "Read", "WriteFile", "Write"}},
	}
	base := DefaultSafeStdlibPackagesList()
	digests := map[string]string{}
	for _, c := range cases {
		set := DeskImportSet(c.desk, nil)
		// F1: the package part is the pure base for every desk.
		if !slices.Equal(DeskAllowedPackages(c.desk), set.Packages) {
			t.Errorf("%s carried list differs from its set", c.desk)
		}
		if !slices.Equal(set.Packages, base) || !slices.Contains(set.Packages, "choir") || !slices.Contains(set.Packages, "math") {
			t.Errorf("%s packages = %v", c.desk, set.Packages)
		}
		for _, p := range effectful {
			if slices.Contains(set.Packages, p) {
				t.Errorf("%s set holds %s", c.desk, p)
			}
		}
		// F1: the choir part is the desk's grants and no more.
		for _, v := range c.has {
			if !slices.Contains(set.Choir, v) {
				t.Errorf("%s choir lacks %s: %v", c.desk, v, set.Choir)
			}
		}
		for _, v := range c.not {
			if slices.Contains(set.Choir, v) {
				t.Errorf("%s choir holds %s, which its grants do not give", c.desk, v)
			}
		}
		// F2: computed again, the same digest.
		if again := DeskImportSet(c.desk, nil).Digest(); again != set.Digest() {
			t.Errorf("%s digest moved between computations", c.desk)
		}
		digests[c.desk] = set.Digest()
	}
	seen := map[string]string{}
	for d, h := range digests {
		if other, dup := seen[h]; dup {
			t.Errorf("%s and %s have one digest", d, other)
		}
		seen[h] = d
	}
	// An unknown role: the base and observation only.
	unknown := DeskImportSet("no-such-desk", nil)
	if !slices.Equal(unknown.Packages, base) || slices.Contains(unknown.Choir, "Cast") || !slices.Contains(unknown.Choir, "Records") {
		t.Errorf("unknown role set = %+v", unknown)
	}
}

// F2: the set is a function of the row's grants, not of the order they were
// written in; a grant that adds a verb moves it.
func TestMA66ImportSetDigestIsStableUnderGrantOrder(t *testing.T) {
	register := func(name string, uid uint32, verbs ...string) {
		t.Helper()
		d, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{Name: name, Verbs: verbs,
			Taint: deskmanifest.TaintUntrustedInput, UID: uid, WorkerMaxBytes: 128 << 20})
		if err != nil {
			t.Fatal(err)
		}
		if err := deskmanifest.RegisterApp(d); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = deskmanifest.RetireApp(name) })
	}
	register("app:ma66-a", 62167, "Put", "Get", "List", "Retract", "Cast")
	register("app:ma66-b", 62168, "Cast", "List", "Retract", "Get", "Put")
	register("app:ma66-c", 62169, "Cast", "List", "Retract", "Get", "Put", "Report")
	digest := func(name string) string {
		s := DeskImportSet(name, []string{"example.com/z", "example.com/a"})
		s.Desk = ""
		return s.Digest()
	}
	if digest("app:ma66-a") != digest("app:ma66-b") {
		t.Fatal("grant order moved the import set")
	}
	if digest("app:ma66-a") == digest("app:ma66-c") {
		t.Fatal("a grant that adds a verb did not move the import set")
	}
	a := DeskImportSet("app:ma66-a", []string{"example.com/z", "example.com/a"})
	b := DeskImportSet("app:ma66-a", []string{"example.com/a", "example.com/z"})
	if a.Digest() != b.Digest() {
		t.Fatal("go_imports order moved the import set")
	}
}

// F3: the worker's list may only narrow the desk's set.
func TestMA66CarriedPackagesOnlyNarrowTheSet(t *testing.T) {
	if _, err := DeskAllowlist(deskmanifest.Management, []string{"fmt", "strings"}); err != nil {
		t.Fatalf("a narrower list: %v", err)
	}
	for _, p := range []string{"encoding/gob", "os", "net/http", "github.com/pmezard/go-difflib/difflib"} {
		if _, err := DeskAllowlist(deskmanifest.Management, []string{"fmt", p}); err == nil || !strings.Contains(err.Error(), "outside desk") {
			t.Errorf("carried %s: %v", p, err)
		}
	}
	if _, err := DeskAllowlist(deskmanifest.Management, []string{" ", ""}); err == nil {
		t.Error("a blank carried list fell back to a default")
	}
	al, err := DeskAllowlist(deskmanifest.Research, nil)
	if err != nil {
		t.Fatal(err)
	}
	if al.IsAllowed("math") != nil || al.IsAllowed("choir") != nil || al.IsAllowed("encoding/gob") == nil {
		t.Error("an empty carried list is not the desk's own set")
	}
}

func ma66Worker(t *testing.T, role string, egress func(context.Context, BrokerAction, json.RawMessage) (json.RawMessage, error)) *DeskSessionWorker {
	t.Helper()
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("desk session worker spawn requires unix socketpair")
	}
	cfg := deskWorkerConfig(t)
	cfg.Session.Role = role
	cfg.Session.AllowedPackages = DeskAllowedPackages(role)
	cfg.Egress = egress
	w, err := SpawnDeskSessionWorker(cfg)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	t.Cleanup(w.Close)
	return w
}

// F4: a cell importing outside its desk's set is refused at the static check,
// before anything runs, in a real worker.
func TestMA66CellImportingOutsideItsSetIsRefusedAtTheStaticCheck(t *testing.T) {
	w := ma66Worker(t, deskmanifest.Management, nil)
	for _, imp := range []string{"encoding/gob", "os/exec", "net", "github.com/pmezard/go-difflib/difflib"} {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		res, err := w.Eval(ctx, fmt.Sprintf("import %q\nprintln(\"ran\")", imp))
		cancel()
		if err != nil {
			t.Fatalf("%s: %v", imp, err)
		}
		if !strings.Contains(res.Error, "refused") || strings.Contains(res.Stdout, "ran") {
			t.Fatalf("%s was not refused at the static check: %+v", imp, res)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	res, err := w.Eval(ctx, "import (\"math\"; \"sort\")\nxs := []float64{3, 1}\nsort.Float64s(xs)\nfmt.Println(math.Sqrt(xs[1]))")
	if err != nil || res.Error != "" || strings.TrimSpace(res.Stdout) != "1.7320508075688772" {
		t.Fatalf("a pure import in the set: %+v %v", res, err)
	}
}

// F5: a hundred read-verb host calls in one cell are served; the bound holds
// past MaxFrameCallsPerCell, refusing the calls over it and no others.
func TestMA66AHundredHostCallsInOneCellAndTheBound(t *testing.T) {
	var calls atomic.Int64
	w := ma66Worker(t, deskmanifest.Research, func(_ context.Context, _ BrokerAction, _ json.RawMessage) (json.RawMessage, error) {
		calls.Add(1)
		return json.RawMessage(`{"items":[]}`), nil
	})
	run := func(n int) (SessionResult, int64) {
		t.Helper()
		calls.Store(0)
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		res, err := w.Eval(ctx, fmt.Sprintf(`ok, refused := 0, 0
for i := 0; i < %d; i++ {
	if _, err := choir.ListEvidence("", 5); err == nil {
		ok++
	} else if strings.Contains(err.Error(), "host calls") {
		refused++
	}
}
fmt.Println(ok, refused)`, n))
		if err != nil {
			t.Fatal(err)
		}
		return res, calls.Load()
	}
	res, host := run(100)
	if strings.TrimSpace(res.Stdout) != "100 0" || host != 100 {
		t.Fatalf("100 calls: %+v, host served %d", res, host)
	}
	res, host = run(MaxFrameCallsPerCell + 10)
	// A refused verb also prints its refusal (recordRefusals); the counts
	// are the first line.
	if first, _, _ := strings.Cut(res.Stdout, "\n"); first != fmt.Sprintf("%d 10", MaxFrameCallsPerCell) || host != MaxFrameCallsPerCell {
		t.Fatalf("past the bound: %q, host served %d", res.Stdout, host)
	}
	// The bound is per cell: the next cell starts at zero.
	if res, host = run(100); strings.TrimSpace(res.Stdout) != "100 0" || host != 100 {
		t.Fatalf("the next cell: %+v, host served %d", res, host)
	}
}

// F6: parallel host calls in one cell run in parallel on the host, up to
// MaxConcurrentFrameCalls, and every call is answered before the cell's
// result.
func TestMA66ParallelHostCallsRunInParallel(t *testing.T) {
	var inflight, peak, done atomic.Int64
	w := ma66Worker(t, deskmanifest.Research, func(_ context.Context, _ BrokerAction, _ json.RawMessage) (json.RawMessage, error) {
		n := inflight.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
		inflight.Add(-1)
		done.Add(1)
		return json.RawMessage(`{"items":[]}`), nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	start := time.Now()
	res, err := w.Eval(ctx, `import "sync"
var wg sync.WaitGroup
for i := 0; i < 16; i++ {
	wg.Add(1)
	go func() { defer wg.Done(); choir.ListEvidence("", 1) }()
}
wg.Wait()
fmt.Println("done")`)
	elapsed := time.Since(start)
	if err != nil || res.Error != "" {
		t.Fatalf("%+v %v", res, err)
	}
	if done.Load() != 16 {
		t.Fatalf("answered %d of 16 before the cell result", done.Load())
	}
	if p := peak.Load(); p < 2 || p > MaxConcurrentFrameCalls {
		t.Fatalf("peak in-flight host calls = %d, want 2..%d", p, MaxConcurrentFrameCalls)
	}
	// Serial would be 1.6 s; eight at a time is two rounds of 100 ms.
	if elapsed > 1200*time.Millisecond {
		t.Fatalf("16 x 100 ms calls took %v", elapsed)
	}
	t.Logf("16 x 100 ms host calls in %v, peak %d in flight", elapsed, peak.Load())
}

// F7: the intent quota is the value design section 5 sets.
func TestMA66IntentQuotaIsDeliberate(t *testing.T) {
	if MaxIntentsPerCell != 64 || MaxIntentsPerCell*4<<10 != MaxTrayBytes {
		t.Fatalf("MaxIntentsPerCell = %d", MaxIntentsPerCell)
	}
	var tray Tray
	for i := range MaxIntentsPerCell {
		if _, err := tray.Escalate("peer", fmt.Sprintf("x%d", i), testRequest()); err != nil {
			t.Fatalf("intent %d: %v", i+1, err)
		}
	}
	if _, err := tray.Escalate("peer", "over", testRequest()); err == nil {
		t.Fatal("an intent past the quota was staged")
	}
}
