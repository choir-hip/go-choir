package yaegikernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/traefik/yaegi/stdlib"

	"github.com/yusefmosiah/go-choir/internal/gomodule"
)

// MA6.5: a desk cell calls a package-shipped Go module and the host verbs in
// the same yaegi worker (docs/design/ma65-yaegi-go-modules-2026-11-06.md).
//
// Ways this could fail, written first (design section 6):
//  1. the bundle does not cross execve, or crosses altered, and the worker
//     serves cells anyway (without its modules, or with altered ones);
//  2. the worker trusts the bundle without re-checking it;
//  3. a cell imports a module package its desk did not declare;
//  4. the interpreter reads package source from the real filesystem;
//  5. a module and a host verb cannot be used in one cell (the module loads
//     but the choir binding or the egress round trip breaks).

const (
	difflibPkg     = "github.com/pmezard/go-difflib/difflib"
	difflibVersion = "v1.0.1-0.20181226105442-5d4384ee4fb2"
	humanizePath   = "github.com/dustin/go-humanize"
)

// testModule reads a module's content from internal/gomodule's testdata.
func testModule(t testing.TB, path, version string) gomodule.ModuleSource {
	t.Helper()
	m := gomodule.Module{Path: path, Version: version}
	root := filepath.Join("..", "gomodule", "testdata", "gomod", filepath.FromSlash(m.Dir()))
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		b, err := os.ReadFile(p)
		files[filepath.ToSlash(rel)] = b
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return gomodule.ModuleSource{Module: m, Files: files}
}

// difflibBundle is the verified bundle of the curated go-difflib, as the
// publish gate builds it.
func difflibBundle(t testing.TB) gomodule.Bundle {
	t.Helper()
	cur, err := gomodule.Curated()
	if err != nil {
		t.Fatal(err)
	}
	b, err := gomodule.Resolve(cur, []gomodule.ModuleSource{testModule(t, "github.com/pmezard/go-difflib", difflibVersion)}, []string{difflibPkg})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func encodedBundle(t testing.TB, b gomodule.Bundle) []byte {
	t.Helper()
	raw, err := b.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// moduleCell uses the module and a host verb in one cell: it diffs the text
// a verb returned against a local revision.
const moduleCell = `r, err := choir.ReadCapture("cap-1", 0, 100)
if err != nil { panic(err) }
var c struct{ Text string }
if err := json.Unmarshal(r, &c); err != nil { panic(err) }
d, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{A: difflib.SplitLines("a\nfrom the host\n"), B: difflib.SplitLines("a\n" + c.Text + "\n"), FromFile: "old", ToFile: "new", Context: 1})
fmt.Println(d, err)`

const moduleCellWant = "@@ -1,3 +1,3 @@\n a\n-from the host\n+from the desk's host\n"

func moduleWorkerConfig(t *testing.T) (DeskSessionWorkerConfig, *[]BrokerAction) {
	t.Helper()
	cfg := deskWorkerConfig(t)
	cfg.Session.Role = "research"
	cfg.Session.GoModules = encodedBundle(t, difflibBundle(t))
	var mu sync.Mutex
	var seen []BrokerAction
	cfg.Egress = func(_ context.Context, action BrokerAction, _ json.RawMessage) (json.RawMessage, error) {
		mu.Lock()
		seen = append(seen, action)
		mu.Unlock()
		return json.RawMessage(`{"text":"from the desk's host"}`), nil
	}
	return cfg, &seen
}

func requireSocketpair(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("desk session worker spawn requires unix socketpair")
	}
}

// Failure mode 5: the proof of the plan row.
func TestDeskWorkerCellCallsAGoModuleAndAHostVerb(t *testing.T) {
	requireSocketpair(t)
	cfg, seen := moduleWorkerConfig(t)
	w, err := SpawnDeskSessionWorker(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	res, err := w.Eval(context.Background(), moduleCell)
	if err != nil || res.Error != "" || !strings.Contains(res.Stdout, moduleCellWant) {
		t.Fatalf("cell: %v stdout=%q error=%q", err, res.Stdout, res.Error)
	}
	if len(*seen) != 1 || (*seen)[0] != ActionReadCapture {
		t.Fatalf("host saw %v", *seen)
	}
	// A cell's own import of the module is accepted (dropped as a repeat).
	res, err = w.Eval(context.Background(), "import \""+difflibPkg+"\"\nfmt.Println(len(difflib.SplitLines(\"x\\ny\")))")
	if err != nil || res.Error != "" || !strings.Contains(res.Stdout, "2") {
		t.Fatalf("explicit import: %v %q %q", err, res.Stdout, res.Error)
	}
	// Failure mode 3: a module the desk did not declare.
	res, err = w.Eval(context.Background(), "import \""+humanizePath+"\"\nfmt.Println(humanize.Comma(2))")
	if err != nil || res.Error == "" {
		t.Fatalf("undeclared package served: %v %q", err, res.Stdout)
	}
}

// Failure modes 1 and 2: an altered or partial carry, or a bundle that fails
// the re-check, and the worker never becomes ready.
func TestDeskWorkerRefusesABadGoModuleCarry(t *testing.T) {
	requireSocketpair(t)
	evil := difflibBundle(t)
	for i := range evil.Modules[0].Files {
		if f := &evil.Modules[0].Files[i]; f.Name == "difflib/difflib.go" {
			f.Data = []byte(strings.Replace(string(f.Data), "import (", "import (\n\t_ \"os/exec\"", 1))
		}
	}
	// Encode checks nothing: the worker's re-check is what refuses it.
	evilRaw := encodedBundle(t, evil)
	cases := map[string]func(cfg *DeskSessionWorkerConfig){
		"digest mismatch": func(cfg *DeskSessionWorkerConfig) {
			cfg.ExtraEnv = append(cfg.ExtraEnv, "CHOIR_DESK_SESSION_GO_MODULES_SHA256="+strings.Repeat("0", 64))
		},
		"no digest": func(cfg *DeskSessionWorkerConfig) {
			cfg.ExtraEnv = append(cfg.ExtraEnv, "CHOIR_DESK_SESSION_GO_MODULES_SHA256=")
		},
		"a part missing": func(cfg *DeskSessionWorkerConfig) {
			cfg.ExtraEnv = append(cfg.ExtraEnv, "CHOIR_DESK_SESSION_GO_MODULES_PARTS=2")
		},
		"a banned import with a matching digest": func(cfg *DeskSessionWorkerConfig) {
			cfg.Session.GoModules = evilRaw
		},
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			cfg, _ := moduleWorkerConfig(t)
			mut(&cfg)
			w, err := SpawnDeskSessionWorker(cfg)
			if err == nil {
				w.Close()
				t.Fatal("a worker with a bad go module carry became ready")
			}
			if !strings.Contains(err.Error(), "go modules") {
				t.Fatalf("refusal does not name the go modules: %v", err)
			}
		})
	}
}

func TestGoModulesEnvRoundTrip(t *testing.T) {
	raw := encodedBundle(t, difflibBundle(t))
	// Force several parts.
	big := []byte(strings.Repeat("x", 3*goModulesPartBytes))
	for _, body := range [][]byte{raw, big} {
		env := goModulesEnv("P_", body)
		got := map[string]string{}
		for _, e := range env {
			k, v, _ := strings.Cut(e, "=")
			got[k] = v
			if len(e) >= 128<<10 {
				t.Fatalf("an env string of %d bytes passes execve's per-string limit", len(e))
			}
		}
		back, sum, err := readGoModulesEnv(func(k string) string { return got[k] }, "P_")
		want := sha256.Sum256(body)
		if err != nil || string(back) != string(body) || sum != hex.EncodeToString(want[:]) {
			t.Fatalf("round trip: %v", err)
		}
	}
	if err := checkGoModulesCarry(make([]byte, gomodule.MaxEncodedBundleBytes+1)); err == nil {
		t.Fatal("an over-cap bundle would be carried")
	}
}

// Failure mode 4: the source filesystem is the bundle, never the real one,
// even for a path the allowlist admits and the working directory holds.
func TestSessionNeverReadsPackageSourceFromDisk(t *testing.T) {
	withModules := difflibBundle(t)
	dir := t.TempDir()
	pkg := filepath.Join(dir, "src", "example.com", "ondisk")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "x.go"), []byte("package ondisk\nfunc X() int { return 7 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	for _, bundle := range []*gomodule.Bundle{nil, &withModules} {
		s, err := NewSessionWithGoModules(NewAllowlist(append(DefaultSafeStdlibPackagesList(), "example.com/ondisk")...), nil, bundle)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Eval(context.Background(), "import \"example.com/ondisk\"\nfmt.Println(ondisk.X())"); err == nil {
			t.Fatal("the interpreter read a package from the real filesystem")
		}
		s.Close()
	}
}

func TestSessionLoadsAGoModuleInProcess(t *testing.T) {
	b := difflibBundle(t)
	s, err := NewSessionWithGoModules(NewDefaultSafeAllowlist(), nil, &b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	res, err := s.Eval(context.Background(), "m := difflib.NewMatcher(difflib.SplitLines(\"a\\nb\\nc\"), difflib.SplitLines(\"a\\nc\"))\nfmt.Printf(\"%.2f\\n\", m.Ratio())")
	if err != nil || !strings.Contains(res.Stdout, "0.80") {
		t.Fatalf("%v %q", err, res.Stdout)
	}
}

// yaegi's limits bound what a module can be (R42): go-humanize v1.0.1 passes
// the closure check but yaegi v0.16.1 cannot interpret one of its constant
// expressions. The session refuses at start, so its worker never becomes
// ready, and no cell runs without the module it was promised.
func TestAModuleYaegiCannotInterpretRefusesTheSession(t *testing.T) {
	cur := []gomodule.Module{{Path: humanizePath, Version: "v1.0.1", Sum: "h1:GzkhY7T5VNhEkwH0PVJgjz+fX1rhBrR7pRT3mDkpeCY="}}
	useCurated(t, cur)
	b, err := gomodule.Resolve(cur, []gomodule.ModuleSource{testModule(t, humanizePath, "v1.0.1")}, []string{humanizePath})
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewSessionWithGoModules(NewDefaultSafeAllowlist(), nil, &b)
	if err == nil {
		s.Close()
		t.Fatal("a session started without the module it was given")
	}
	if !strings.Contains(err.Error(), "mismatched types") {
		t.Fatalf("refused for another reason: %v", err)
	}
}

// useCurated loads fixture modules against list instead of the curated one.
func useCurated(t *testing.T, list []gomodule.Module) {
	t.Helper()
	old := curatedGoModules
	curatedGoModules = func() ([]gomodule.Module, error) { return list, nil }
	t.Cleanup(func() { curatedGoModules = old })
}

// fixtureBundle builds a one-module bundle from files and admits it.
func fixtureBundle(t *testing.T, path string, files map[string]string, imports ...string) gomodule.Bundle {
	t.Helper()
	src := gomodule.ModuleSource{Module: gomodule.Module{Path: path, Version: "v0.1.0"}, Files: map[string][]byte{}}
	for k, v := range files {
		src.Files[k] = []byte(v)
	}
	sum, err := gomodule.Hash1(src.Module.String(), src.Files)
	if err != nil {
		t.Fatal(err)
	}
	cur := []gomodule.Module{{Path: path, Version: "v0.1.0", Sum: sum}}
	useCurated(t, cur)
	b, err := gomodule.Resolve(cur, []gomodule.ModuleSource{src}, imports)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// D4: a module whose init does not finish refuses the session by name, inside
// a bound shorter than the spawner's ready timeout.
func TestModuleInitIsBoundedAtSessionStart(t *testing.T) {
	old := goModuleLoadTimeout
	goModuleLoadTimeout = 300 * time.Millisecond
	t.Cleanup(func() { goModuleLoadTimeout = old })
	if goModuleLoadTimeout >= sessionReadyTimeout || old >= sessionReadyTimeout {
		t.Fatal("the module load bound is not below the ready timeout")
	}
	b := fixtureBundle(t, "example.com/slow", map[string]string{"slow.go": "package slow\nimport \"time\"\nfunc init() { time.Sleep(time.Hour) }\n"}, "example.com/slow")
	start := time.Now()
	_, err := NewSessionWithGoModules(NewDefaultSafeAllowlist(), nil, &b)
	if err == nil || !strings.Contains(err.Error(), "init did not finish") {
		t.Fatalf("%v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("refusal took %s", time.Since(start))
	}
}

// S2: a module package at the path of a binary symbol table (here yaegi's own
// unfiltered stdlib table) is refused, and the bundle never selects a binary
// table: symbols come from the base allowlist.
func TestModuleAtABinaryTablePathIsRefused(t *testing.T) {
	b := fixtureBundle(t, "github.com/traefik/yaegi", map[string]string{"stdlib/s.go": "package stdlib\nvar X = 1\n"}, "github.com/traefik/yaegi/stdlib")
	if s, err := NewSessionWithGoModules(NewDefaultSafeAllowlist(), nil, &b); err == nil || !strings.Contains(err.Error(), "binary symbol table") {
		if s != nil {
			s.Close()
		}
		t.Fatalf("%v", err)
	}
	if syms := buildFilteredSymbols(NewDefaultSafeAllowlist(), nil); syms["github.com/traefik/yaegi/stdlib/stdlib"] != nil {
		t.Fatal("the base set binds yaegi's own table")
	}
}

// S1: no bound symbol of any allowed package mentions unsafe.Pointer, checked
// here independently of the filter (by the reflect type's own rendering).
func TestNoBoundSymbolMentionsUnsafePointer(t *testing.T) {
	syms := buildFilteredSymbols(NewDefaultSafeAllowlist(), nil)
	mentions := func(rt reflect.Type) string {
		var walkT func(reflect.Type, int) string
		seen := map[reflect.Type]bool{}
		walkT = func(t reflect.Type, depth int) string {
			if t == nil || seen[t] || depth > 8 {
				return ""
			}
			seen[t] = true
			if strings.Contains(t.String(), "unsafe.Pointer") {
				return t.String()
			}
			for i := 0; i < t.NumMethod(); i++ {
				if m := t.Method(i); strings.Contains(m.Type.String(), "unsafe.Pointer") {
					return t.String() + "." + m.Name
				}
			}
			switch t.Kind() {
			case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Chan:
				return walkT(t.Elem(), depth+1)
			case reflect.Struct:
				for i := 0; i < t.NumField(); i++ {
					if f := t.Field(i); f.IsExported() {
						if w := walkT(f.Type, depth+1); w != "" {
							return w
						}
					}
				}
			}
			return ""
		}
		return walkT(rt, 0)
	}
	n := 0
	for pkg, table := range syms {
		for name, v := range table {
			n++
			if w := mentions(v.Type()); w != "" {
				t.Errorf("%s.%s mentions unsafe.Pointer: %s", pkg, name, w)
			}
		}
	}
	if n < 1000 {
		t.Fatalf("scanned only %d symbols", n)
	}
	// The four that motivated the filter are absent, and present unfiltered.
	for _, name := range []string{"LoadPointer", "StorePointer", "SwapPointer", "CompareAndSwapPointer"} {
		if _, ok := syms["sync/atomic/atomic"][name]; ok {
			t.Errorf("atomic.%s is bound", name)
		}
		if _, ok := stdlib.Symbols["sync/atomic/atomic"][name]; !ok {
			t.Errorf("atomic.%s is not in yaegi's table: the test proves nothing", name)
		}
	}
}

func TestACellReferencingAtomicLoadPointerIsRefused(t *testing.T) {
	s, err := NewSession(NewDefaultSafeAllowlist(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, src := range []string{
		"import \"sync/atomic\"\nvar p *int\n_ = atomic.LoadPointer",
		"import \"sync/atomic\"\nf := atomic.SwapPointer\n_ = f",
	} {
		if _, err := s.Eval(context.Background(), src); err == nil {
			t.Errorf("cell accepted: %s", src)
		}
	}
	// The rest of sync/atomic stays.
	if res, err := s.Eval(context.Background(), "var n atomic.Int64\nn.Add(2)\nfmt.Println(n.Load())"); err != nil || !strings.Contains(res.Stdout, "2") {
		t.Fatalf("%v %q", err, res.Stdout)
	}
}

// Every curated module's packages that pass the closure check load in a
// session: a curated entry yaegi cannot interpret is caught here, at review.
func TestEveryCuratedModuleLoadsInYaegi(t *testing.T) {
	cur, err := gomodule.Curated()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cur {
		src := testModule(t, c.Path, c.Version)
		pkgs := map[string]bool{}
		for rel := range src.Files {
			if strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go") {
				dir := filepath.ToSlash(filepath.Dir(rel))
				p := c.Path
				if dir != "." {
					p += "/" + dir
				}
				pkgs[p] = true
			}
		}
		loaded := 0
		for p := range pkgs {
			b, err := gomodule.Resolve(cur, []gomodule.ModuleSource{src}, []string{p})
			if err != nil {
				t.Logf("%s: not loadable by a desk: %v", p, err)
				continue
			}
			s, err := NewSessionWithGoModules(NewDefaultSafeAllowlist(), nil, &b)
			if err != nil {
				t.Errorf("curated %s: package %s does not load in yaegi: %v", c, p, err)
				continue
			}
			s.Close()
			loaded++
		}
		if loaded == 0 {
			t.Errorf("curated %s: no package of it loads", c)
		}
	}
}

// The widened set: a cell uses the packages MA6.5 added (code mode's fan-out
// and join among them).
func TestSessionWidenedStdlib(t *testing.T) {
	s, err := NewSession(NewDefaultSafeAllowlist(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	src := `import ("sync"; "context"; "slices"; "net/url"; "crypto/sha256"; "encoding/csv"; "bytes"; "compress/gzip"; "io"; "container/list"; "sort"; "slices")
var wg sync.WaitGroup
var mu sync.Mutex
got := []int{}
for i := 0; i < 8; i++ { wg.Add(1); go func(n int) { defer wg.Done(); mu.Lock(); got = append(got, n*n); mu.Unlock() }(i) }
wg.Wait()
sort.Ints(got)
ctx, cancel := context.WithCancel(context.Background()); cancel()
u, _ := url.Parse("https://example.com/a?b=1")
sum := sha256.Sum256([]byte("x"))
recs, _ := csv.NewReader(bytes.NewBufferString("a,b\n1,2\n")).ReadAll()
var zb bytes.Buffer
zw := gzip.NewWriter(&zb); zw.Write([]byte("packed")); zw.Close()
zr, _ := gzip.NewReader(&zb); unz, _ := io.ReadAll(zr)
l := list.New(); l.PushBack(2); l.PushBack(5)
fmt.Println(got[7], slices.Contains(got, 49), ctx.Err() != nil, u.Host, sum[0], len(recs), string(unz), l.Front().Value)`
	res, err := s.Eval(context.Background(), src)
	if err != nil || !strings.Contains(res.Stdout, "49 true true example.com 45 2 packed 2") {
		t.Fatalf("%v %q", err, res.Stdout)
	}
	for _, p := range []string{"os", "path/filepath", "text/template", "log", "crypto/rand", "runtime/debug"} {
		if _, err := s.Eval(context.Background(), "import \""+p+"\""); err == nil {
			t.Errorf("%s was admitted", p)
		}
	}
}
