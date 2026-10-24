package yaegikernel

// Gap H (docs/problems/svelte-cannot-be-written-through-a-go-string-2026-10-24.md):
// a Go raw string cannot hold a backtick, so a Svelte component with template
// literals could not be written from a cell without being minified. WriteFileBT
// writes a raw string whose backticks are spelled @@BT@@.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const svelteWithBackticks = "<script>\n  let n = 1;\n  const label = `total ${n}`;\n</script>\n\n<p class=\"x\">{label}</p>\n\n<style>\n  p {\n    color: var(--choir-fg);\n  }\n</style>\n"

func evalCell(t *testing.T, sess *Session, scope *ChoirScope, source string) SessionResult {
	t.Helper()
	hooks := scope.BindCell()
	out, err := serveCell(sess, SessionFrame{ID: "cell", Source: source}, nil, &hooks)
	if err != nil {
		t.Fatalf("cell failed: %v", err)
	}
	return out
}

func TestWriteFileBTWritesBackticksFromARawStringCell(t *testing.T) {
	_, _, scope, root := testChoirFixture(t)
	sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), scope.ChoirExports())
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	// The model's cell: the file as a raw string, backticks as the placeholder.
	placeholder := strings.ReplaceAll(svelteWithBackticks, "`", "@@BT@@")
	cell := "src := `" + placeholder + "`\nn, err := choir.WriteFileBT(\"lib/X.svelte\", src)\nprintln(n, err)"
	if out := evalCell(t, sess, scope, cell); out.Error != "" {
		t.Fatalf("WriteFileBT cell failed: %s", out.Error)
	}
	got, err := os.ReadFile(filepath.Join(root, "lib", "X.svelte"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != svelteWithBackticks {
		t.Fatalf("file differs from the source:\n got %q\nwant %q", got, svelteWithBackticks)
	}
	if strings.Count(string(got), "\n") != strings.Count(svelteWithBackticks, "\n") {
		t.Fatal("newlines were not preserved")
	}

	// A file with no backticks arrives unchanged, and plain WriteFile keeps
	// writing the placeholder text verbatim.
	if out := evalCell(t, sess, scope, "choir.WriteFileBT(\"plain.txt\", \"no ticks\\n\")\nchoir.WriteFile(\"verbatim.txt\", \"keep @@BT@@\")"); out.Error != "" {
		t.Fatalf("plain cell failed: %s", out.Error)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "plain.txt")); string(b) != "no ticks\n" {
		t.Fatalf("plain.txt = %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "verbatim.txt")); string(b) != "keep @@BT@@" {
		t.Fatalf("WriteFile changed its content: %q", b)
	}
}

func TestWriteFileBTIsGatedExactlyLikeWriteFile(t *testing.T) {
	broker, issuer, scope, _ := testChoirFixture(t)
	if _, ok := scope.ChoirExports()["choir/choir"]["WriteFileBT"]; !ok {
		t.Fatal("engineering lacks WriteFileBT")
	}
	research, err := NewChoirScope(broker, issuer, "computer-choir", "activation-choir", 1, SessionRoleResearch, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := research.ChoirExports()["choir/choir"]["WriteFileBT"]; ok {
		t.Fatal("research exports WriteFileBT")
	}
	if _, err := research.WriteFileBT("x.txt", "x"); err == nil {
		t.Fatal("research WriteFileBT allowed")
	}
	texture := &ChoirScope{desk: "texture"}
	if _, ok := texture.ChoirExports()["choir/choir"]["WriteFileBT"]; ok {
		t.Fatal("texture exports WriteFileBT")
	}
}
