package yaegikernel

// Cycle time (docs/evidence/selfdev-round-cycle-time-2026-11-03.md): an
// engineer re-emitted whole files to change a line. choir.Edit and
// choir.EditMulti change a file by exact string replacement, so the model
// emits the changed lines only. Failure modes first: no match, several
// matches, an empty old, a partial multi-edit, a read-only desk.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, root, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFixture(t *testing.T, root, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestEditRefusesNoMatchManyMatchesAndEmptyOldWithoutWriting(t *testing.T) {
	_, _, scope, root := testChoirFixture(t)
	const body = "a := 1\nb := 2\na := 1\n"
	writeFixture(t, root, "f.go", body)

	if _, err := scope.Edit("f.go", "zzz", "y"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("no match: err = %v", err)
	}
	if _, err := scope.Edit("f.go", "a := 1", "a := 9"); err == nil || !strings.Contains(err.Error(), "2 times") {
		t.Fatalf("two matches: err = %v", err)
	}
	if _, err := scope.Edit("f.go", "", "y"); err == nil {
		t.Fatal("empty old accepted")
	}
	if _, err := scope.Edit("f.go", "b := 2", "b := 2"); err == nil {
		t.Fatal("old == new accepted")
	}
	if got := readFixture(t, root, "f.go"); got != body {
		t.Fatalf("a refused edit changed the file: %q", got)
	}
}

func TestEditReplacesExactlyOneMatchAndReturnsAShortDiff(t *testing.T) {
	_, _, scope, root := testChoirFixture(t)
	writeFixture(t, root, "f.go", "one\ntwo\nthree\n")
	diff, err := scope.Edit("f.go", "two", "TWO")
	if err != nil {
		t.Fatal(err)
	}
	if got := readFixture(t, root, "f.go"); got != "one\nTWO\nthree\n" {
		t.Fatalf("file = %q", got)
	}
	if !strings.Contains(diff, "-two") || !strings.Contains(diff, "+TWO") || strings.Count(diff, "\n") > 12 {
		t.Fatalf("diff = %q", diff)
	}
}

func TestEditMultiIsAllOrNothing(t *testing.T) {
	_, _, scope, root := testChoirFixture(t)
	const body = "alpha\nbeta\ngamma\n"
	writeFixture(t, root, "f.txt", body)

	// The second pair does not match: nothing is written, the error names it.
	_, err := scope.EditMulti("f.txt", [][]string{{"alpha", "A"}, {"nope", "N"}})
	if err == nil || !strings.Contains(err.Error(), "edit 2") {
		t.Fatalf("err = %v", err)
	}
	if got := readFixture(t, root, "f.txt"); got != body {
		t.Fatalf("a refused multi-edit wrote: %q", got)
	}
	if _, err := scope.EditMulti("f.txt", [][]string{{"alpha"}}); err == nil {
		t.Fatal("a pair of one accepted")
	}
	if _, err := scope.EditMulti("f.txt", nil); err == nil {
		t.Fatal("no edits accepted")
	}

	// Pairs apply in order, so a later pair can match an earlier one's output.
	if _, err := scope.EditMulti("f.txt", [][]string{{"alpha", "A"}, {"gamma", "G"}, {"A\nbeta", "AB"}}); err != nil {
		t.Fatal(err)
	}
	if got := readFixture(t, root, "f.txt"); got != "AB\nG\n" {
		t.Fatalf("file = %q", got)
	}
}

func TestEditSpellsBackticksAsTheWriteFileBTPlaceholder(t *testing.T) {
	_, _, scope, root := testChoirFixture(t)
	writeFixture(t, root, "X.svelte", "const l = `a`;\n")
	// A literal backtick in old matches; the placeholder in new becomes one.
	if _, err := scope.Edit("X.svelte", "`a`", "@@BT@@b${n}@@BT@@"); err != nil {
		t.Fatal(err)
	}
	if got := readFixture(t, root, "X.svelte"); got != "const l = `b${n}`;\n" {
		t.Fatalf("file = %q", got)
	}
	if _, err := scope.Edit("X.svelte", "@@BT@@b${n}@@BT@@", "`c`"); err != nil {
		t.Fatalf("placeholder in old: %v", err)
	}
}

func TestEditIsExportedWhereWriteFileIs(t *testing.T) {
	broker, issuer, scope, _ := testChoirFixture(t)
	for _, name := range []string{"Edit", "EditMulti"} {
		if _, ok := scope.ChoirExports()["choir/choir"][name]; !ok {
			t.Fatalf("engineering lacks %s", name)
		}
		if _, ok := choirFunctionDocs[name]; !ok {
			t.Fatalf("%s has no surface doc", name)
		}
	}
	research, err := NewChoirScope(broker, issuer, "computer-choir", "activation-choir", 1, SessionRoleResearch, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Edit", "EditMulti"} {
		if _, ok := research.ChoirExports()["choir/choir"][name]; ok {
			t.Fatalf("research exports %s", name)
		}
	}
	if _, err := research.Edit("x.txt", "a", "b"); err == nil {
		t.Fatal("research Edit allowed")
	}
}

func TestEditRunsFromAModelCell(t *testing.T) {
	_, _, scope, root := testChoirFixture(t)
	writeFixture(t, root, "c.txt", "x1\nx2\n")
	sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), scope.ChoirExports())
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	cell := "d, err := choir.EditMulti(\"c.txt\", [][]string{{\"x1\", \"y1\"}, {\"x2\", \"y2\"}})\nprintln(d, err)\n_, err = choir.Edit(\"c.txt\", \"nope\", \"z\")\nprintln(err)"
	if out := evalCell(t, sess, scope, cell); out.Error != "" {
		t.Fatalf("cell failed: %s", out.Error)
	}
	if got := readFixture(t, root, "c.txt"); got != "y1\ny2\n" {
		t.Fatalf("file = %q", got)
	}
}
