package yaegikernel

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

// LR lever 6 (docs/archive/reports/lr-lever6-cell-output-design-2026-10-14.md): a
// desk cell's printed output over the bound is shown cut, and the whole text
// stays in the session behind a handle choir.CellOut reads. Ways this can
// fail, written before the code (numbers follow the note):
//
//  1. The model needs the cut part and does not know it exists: the marker
//     must state the length and the handle, and CellOut must return every
//     byte.
//  3. A cut inside a multi-byte rune: the view must be valid UTF-8, with
//     head and tail counted in runes (CJK and Arabic at the boundary).
//  4. An error hidden by a cut: stderr and error are never cut.
//  5. Session memory grows: only the last 8 cut outputs are kept, the ninth
//     evicts the first, and the first then answers with an error naming its
//     eviction. Reading a handle in small slices never consumes a slot.
//  6. A session reset between cells: a handle from the old worker is refused
//     by instance, and the error names both instances.
//  10. A cell prints past the bound and then panics: the session dies, so
//     the output is not cut (no handle could ever be read).
//  11. A research parent's large findings view is a value view, not a
//     print, and is not cut.
//
// And from the round-2 build notes: nil hooks, or hooks without KeepOutput
// (the capsule path), never cut; choir.CellOut never panics, whatever it is
// given, since a panic would poison the session.

var cellOutHandleRE = regexp.MustCompile(`out, err := choir\.CellOut\("([0-9a-f]{8}:[0-9]+)"\)`)

func handleFromView(t *testing.T, view string) string {
	t.Helper()
	m := cellOutHandleRE.FindStringSubmatch(view)
	if m == nil {
		t.Fatalf("cut view carries no choir.CellOut handle:\n%s", clip(view))
	}
	return m[1]
}

func clip(s string) string {
	if len(s) > 600 {
		return s[:300] + " … " + s[len(s)-300:]
	}
	return s
}

// keepingSession is a desk session whose hooks keep cut output, as the desk
// worker entry wires them.
func keepingSession(t *testing.T, scope *ChoirScope) (*Session, *CellHooks) {
	t.Helper()
	sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), scope.ChoirExports())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sess.Close)
	hooks := scope.BindCell()
	hooks.KeepOutput = scope.keepCellOutput
	return sess, &hooks
}

func runCell(t *testing.T, sess *Session, hooks *CellHooks, source string) SessionResult {
	t.Helper()
	out, err := serveCell(sess, SessionFrame{ID: "cell", Source: source}, nil, hooks)
	if err != nil || out.Error != "" {
		t.Fatalf("cell %q failed: %v %s", clip(source), err, out.Error)
	}
	return out
}

// Failure mode 1 (and 2: the marker sits on its own line, outside the head).
func TestCellOutputCutShowsHandleAndCellOutReturnsTheWhole(t *testing.T) {
	_, _, scope, _ := testChoirFixture(t)
	sess, hooks := keepingSession(t, scope)
	big := strings.Repeat("0123456789", 2000)
	out := runCell(t, sess, hooks, `big := strings.Repeat("0123456789", 2000)`+"\nfmt.Print(big)")
	if len(out.Stdout) >= 9000 {
		t.Fatalf("20,000-char print not cut: %d bytes", len(out.Stdout))
	}
	if !strings.HasPrefix(out.Stdout, big[:cellOutputHead]) || !strings.HasSuffix(out.Stdout, big[len(big)-cellOutputTail:]) {
		t.Fatalf("view is not head %d + tail %d of the print:\n%s", cellOutputHead, cellOutputTail, clip(out.Stdout))
	}
	for _, want := range []string{"20,000 chars", "12,500 not shown", "out, err := choir.CellOut(\""} {
		if !strings.Contains(out.Stdout, want) {
			t.Errorf("marker missing %q:\n%s", want, clip(out.Stdout))
		}
	}
	h := handleFromView(t, out.Stdout)
	head := out.Stdout[:strings.Index(out.Stdout, "choir.CellOut")]
	if !strings.HasPrefix(head[cellOutputHead:], "\n") {
		t.Errorf("marker is not on its own line after the head: %q", head[cellOutputHead:])
	}
	read := runCell(t, sess, hooks, fmt.Sprintf("whole, err := choir.CellOut(%q)\nfmt.Println(len(whole), err == nil, whole == big)", h))
	if read.Stdout != "20000 true true\n" {
		t.Fatalf("choir.CellOut(%q) did not return the whole print: %q", h, read.Stdout)
	}
}

func TestCellOutputBoundIsEightThousandRunes(t *testing.T) {
	scope := &ChoirScope{}
	at := strings.Repeat("é", cellOutputBound)
	if got := scope.keepCellOutput(at); got != at {
		t.Fatalf("an output of exactly %d runes was cut", cellOutputBound)
	}
	over := at + "x"
	if got := scope.keepCellOutput(over); got == over || !strings.Contains(got, "8,001 chars") {
		t.Fatalf("an output of %d runes was not cut: %q", cellOutputBound+1, clip(got))
	}
}

// Failure mode 3.
func TestCellOutputCutFallsOnRuneBoundaries(t *testing.T) {
	_, _, scope, _ := testChoirFixture(t)
	sess, hooks := keepingSession(t, scope)
	unit := "中文العربية" // 2 three-byte runes + 7 two-byte runes
	big := strings.Repeat(unit, 2000)
	runes := []rune(big)
	out := runCell(t, sess, hooks, fmt.Sprintf("big := strings.Repeat(%q, 2000)\nfmt.Print(big)", unit))
	if !utf8.ValidString(out.Stdout) {
		t.Fatal("cut view is not valid UTF-8")
	}
	if !strings.HasPrefix(out.Stdout, string(runes[:cellOutputHead])) || !strings.HasSuffix(out.Stdout, string(runes[len(runes)-cellOutputTail:])) {
		t.Fatalf("head/tail not counted in runes:\n%s", clip(out.Stdout))
	}
	if !strings.Contains(out.Stdout, fmt.Sprintf("%s chars", groupDigits(len(runes)))) {
		t.Fatalf("marker does not count runes (%d):\n%s", len(runes), clip(out.Stdout))
	}
	h := handleFromView(t, out.Stdout)
	read := runCell(t, sess, hooks, fmt.Sprintf("whole, _ := choir.CellOut(%q)\nfmt.Println(whole == big)", h))
	if read.Stdout != "true\n" {
		t.Fatalf("multi-byte output not returned whole: %q", read.Stdout)
	}
}

// Failure mode 4: only the cell's stdout passes through the cut. Cells in a
// desk session cannot write stderr (println lands on stdout, os is refused),
// so the guarantee for stderr is structural: KeepOutput is handed stdout
// alone, once, and never on an error path. Errors stay whole: a refusal is
// appended after the cut view, and a long panic message is not cut.
func TestCellOutputNeverCutsStderrOrError(t *testing.T) {
	_, _, scope, _ := testChoirFixture(t)
	sess, hooks := keepingSession(t, scope)
	var seen []string
	hooks.KeepOutput = func(stdout string) string {
		seen = append(seen, stdout)
		return scope.keepCellOutput(stdout)
	}
	out := runCell(t, sess, hooks, `fmt.Print(strings.Repeat("o", 20000))`+"\n"+`s, _ := choir.CellOut("not-a-handle")`+"\n_ = s")
	if len(seen) != 1 || seen[0] != strings.Repeat("o", 20000) {
		t.Fatalf("KeepOutput saw %d calls, not the cell's stdout alone", len(seen))
	}
	cut := strings.Index(out.Stdout, `choir.CellOut("`)
	refusal := strings.Index(out.Stdout, "choir.CellOut refused:")
	if cut < 0 || refusal < cut || !strings.Contains(out.Stdout[refusal:], "not a cell output handle") {
		t.Fatalf("the refusal is not shown whole after the cut view:\n%s", clip(out.Stdout))
	}
	seen = nil
	failed, err := serveCell(sess, SessionFrame{ID: "c", Source: `panic(strings.Repeat("p", 20000))`}, nil, hooks)
	if err == nil || strings.Count(failed.Error, "p") < 20000 || len(seen) != 0 {
		t.Fatalf("a long error was cut or lost: err=%v len=%d keep calls=%d", err, len(failed.Error), len(seen))
	}
}

// Failure mode 5.
func TestCellOutputKeepsOnlyTheLastEight(t *testing.T) {
	_, _, scope, _ := testChoirFixture(t)
	sess, hooks := keepingSession(t, scope)
	var handles []string
	for i := 0; i < cellOutputKept+1; i++ {
		out := runCell(t, sess, hooks, fmt.Sprintf(`fmt.Print(strings.Repeat("%d", 9000))`, i))
		handles = append(handles, handleFromView(t, out.Stdout))
	}
	first := runCell(t, sess, hooks, fmt.Sprintf("_, err := choir.CellOut(%q)\nfmt.Println(err)", handles[0]))
	if !strings.Contains(first.Stdout, "evicted") || !strings.Contains(first.Stdout, "only the last 8 cut outputs are kept") {
		t.Fatalf("the first handle after a ninth cut does not name its eviction: %q", first.Stdout)
	}
	// Reading slices of a kept handle never consumes a slot.
	for i := 0; i < 3; i++ {
		runCell(t, sess, hooks, fmt.Sprintf("s, _ := choir.CellOut(%q)\nfmt.Println(s[:100])", handles[1]))
	}
	for i, h := range handles[1:] {
		read := runCell(t, sess, hooks, fmt.Sprintf("s, err := choir.CellOut(%q)\nfmt.Println(len(s), err, s[:1])", h))
		if want := fmt.Sprintf("9000 <nil> %d\n", i+1); read.Stdout != want {
			t.Fatalf("kept handle %s = %q, want %q", h, read.Stdout, want)
		}
	}
	// An ignored CellOut error still reaches the result as a refusal.
	refused := runCell(t, sess, hooks, fmt.Sprintf("s, _ := choir.CellOut(%q)\n_ = s", handles[0]))
	if !strings.Contains(refused.Stdout, "choir.CellOut refused:") || !strings.Contains(refused.Stdout, "evicted") {
		t.Fatalf("an ignored CellOut error is not shown: %q", refused.Stdout)
	}
}

// Failure mode 6, on the real desk worker path: the host spawns the worker
// with KeepCellOutput, the worker cuts, the worker is killed, and the fresh
// worker refuses the old handle by instance.
func TestCellOutputHandleFromAKilledWorkerIsRefused(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("desk session worker spawn requires unix socketpair")
	}
	ctx := context.Background()
	cfg := deskWorkerConfig(t)
	cfg.Session.KeepCellOutput = true
	w, err := SpawnDeskSessionWorker(cfg)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer w.Close()
	res, err := w.Eval(ctx, `fmt.Print(strings.Repeat("k", 20000))`)
	if err != nil || res.Error != "" {
		t.Fatalf("eval: %v %s", err, res.Error)
	}
	h := handleFromView(t, res.Stdout)
	oldInstance := strings.SplitN(h, ":", 2)[0]
	w.Kill()
	w2, err := SpawnDeskSessionWorker(cfg)
	if err != nil {
		t.Fatalf("respawn: %v", err)
	}
	defer w2.Close()
	read, err := w2.Eval(ctx, fmt.Sprintf("_, err := choir.CellOut(%q)\nfmt.Println(err)", h))
	if err != nil || read.Error != "" {
		t.Fatalf("eval: %v %s", err, read.Error)
	}
	m := regexp.MustCompile(`handle ([0-9a-f]{8}):1 is from a session that ended; this session is ([0-9a-f]{8}); re-run the cell`).FindStringSubmatch(read.Stdout)
	if m == nil || m[1] != oldInstance || m[2] == oldInstance {
		t.Fatalf("old handle not refused by instance: %q", read.Stdout)
	}
}

// The capsule path spawns its worker without KeepCellOutput: its output and
// receipt digest stay uncut.
func TestCellOutputWorkerWithoutKeepIsUncut(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("desk session worker spawn requires unix socketpair")
	}
	w, err := SpawnDeskSessionWorker(deskWorkerConfig(t))
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer w.Close()
	res, err := w.Eval(context.Background(), `fmt.Print(strings.Repeat("c", 20000))`)
	if err != nil || res.Error != "" {
		t.Fatalf("eval: %v %s", err, res.Error)
	}
	if len(res.Stdout) != 20000 {
		t.Fatalf("worker without KeepCellOutput cut its output: %d bytes", len(res.Stdout))
	}
}

func TestCellOutputUncutWithoutTheHook(t *testing.T) {
	_, _, scope, _ := testChoirFixture(t)
	sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), scope.ChoirExports())
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	src := `fmt.Print(strings.Repeat("n", 20000))`
	out, err := serveCell(sess, SessionFrame{ID: "nil-hooks", Source: src}, nil, nil)
	if err != nil || len(out.Stdout) != 20000 {
		t.Fatalf("nil hooks cut the output: err=%v len=%d", err, len(out.Stdout))
	}
	hooks := scope.BindCell() // no KeepOutput: the capsule path's hooks
	out, err = serveCell(sess, SessionFrame{ID: "no-keep", Source: src}, nil, &hooks)
	if err != nil || len(out.Stdout) != 20000 {
		t.Fatalf("hooks without KeepOutput cut the output: err=%v len=%d", err, len(out.Stdout))
	}
}

// Failure mode 10.
func TestCellOutputOfAPoisonedCellIsNotCut(t *testing.T) {
	_, _, scope, _ := testChoirFixture(t)
	sess, hooks := keepingSession(t, scope)
	out, err := serveCell(sess, SessionFrame{ID: "poison", Source: `fmt.Print(strings.Repeat("q", 20000))` + "\npanic(\"after the print\")"}, nil, hooks)
	if err == nil || out.Reuse == ReusePreserve {
		t.Fatalf("panicking cell did not poison the session: err=%v reuse=%q", err, out.Reuse)
	}
	if out.Stdout != strings.Repeat("q", 20000) {
		t.Fatalf("a poisoned cell's output was cut or changed: %d bytes, CellOut marker=%v", len(out.Stdout), strings.Contains(out.Stdout, "CellOut"))
	}
}

// Failure mode 11: a bare choir.Research result is a value view.
func TestCellOutputLeavesTheResearchFindingsViewWhole(t *testing.T) {
	broker, issuer, _, _ := testChoirFixture(t)
	scope, err := NewChoirScope(broker, issuer, "computer-choir", "activation-research", 1, "research", "")
	if err != nil {
		t.Fatal(err)
	}
	findings := strings.Repeat("finding ", 5000) // 40,000 chars
	broker.SetEgress(func(_ context.Context, action BrokerAction, _ json.RawMessage) (json.RawMessage, error) {
		if action != ActionResearch {
			return nil, fmt.Errorf("unexpected %s", action)
		}
		return json.Marshal([]ResearchChildResult{{Objective: "q", Model: "m", Findings: findings}})
	})
	sess, hooks := keepingSession(t, scope)
	out := runCell(t, sess, hooks, `choir.Research("q")`)
	if !strings.Contains(out.Stdout, findings) || strings.Contains(out.Stdout, "choir.CellOut") {
		t.Fatalf("the findings value view was cut: %d bytes\n%s", len(out.Stdout), clip(out.Stdout))
	}
}

func TestCellOutNeverPanics(t *testing.T) {
	scopes := map[string]*ChoirScope{"nil": nil, "zero": {}, "kept": {}}
	scopes["kept"].keepCellOutput(strings.Repeat("x", 9000))
	inputs := []string{"", "x", ":", "::", "zzzzzzzz:1", "abcdef01:-1", "abcdef01:0",
		"abcdef01:99999999999999999999999", "abcdef01:1:2", "ABCDEF01:1", " abcdef01:1 ",
		"中文:1", "\x00\xff:1", strings.Repeat("a", 1<<20) + ":1"}
	for name, scope := range scopes {
		for _, in := range inputs {
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("%s scope: CellOut(%q) panicked: %v", name, clip(in), r)
					}
				}()
				got, err := scope.CellOut(in)
				if err == nil || got != "" {
					t.Errorf("%s scope: CellOut(%q) = %q, %v; want an error", name, clip(in), clip(got), err)
				}
				if err != nil && len(err.Error()) > 400 {
					t.Errorf("%s scope: CellOut error echoes unbounded input: %d bytes", name, len(err.Error()))
				}
			}()
		}
	}
	// A well-formed handle from this session's instance with no such output.
	kept := scopes["kept"]
	h := kept.cellOuts.instance + ":5"
	if _, err := kept.CellOut(h); err == nil || !strings.Contains(err.Error(), "no cut output") {
		t.Errorf("CellOut(%q) beyond the counter = %v", h, err)
	}
	if _, err := kept.CellOut(kept.cellOuts.instance + ":1"); err != nil {
		t.Errorf("CellOut of a kept output failed: %v", err)
	}
}

func TestCellOutputStoreIsSafeUnderConcurrency(t *testing.T) {
	scope := &ChoirScope{}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			view := scope.keepCellOutput(strings.Repeat("g", 9000))
			if m := cellOutHandleRE.FindStringSubmatch(view); m != nil {
				_, _ = scope.CellOut(m[1])
			}
		}()
	}
	wg.Wait()
	if scope.cellOuts.next != 16 || len(scope.cellOuts.kept) != cellOutputKept {
		t.Fatalf("store after 16 concurrent cuts: next=%d kept=%d", scope.cellOuts.next, len(scope.cellOuts.kept))
	}
}

// The value view's 48,000 bound stays in bytes but never splits a rune.
func TestCellValueTextCutsOnARuneBoundary(t *testing.T) {
	text := strings.Repeat("a", maxShownCellValue-1) + strings.Repeat("中", 100)
	got, ok := cellValueText(reflect.ValueOf(text), maxShownCellValue)
	if !ok || !utf8.ValidString(got) {
		t.Fatalf("value view split a rune: ok=%v valid=%v", ok, utf8.ValidString(got))
	}
	if !strings.HasPrefix(got, strings.Repeat("a", maxShownCellValue-1)+"…") {
		t.Fatalf("value view did not cut back to the rune boundary: %q", got[maxShownCellValue-5:])
	}
}
