package gatefixture

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/disclosure"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

func TestObserveMovesAllGatesAndRestoresEnforceOnCleanup(t *testing.T) {
	t.Run("inner", func(t *testing.T) {
		Observe(t)
		if objectgraph.CurrentWriterGateMode() != objectgraph.WriterGateObserve ||
			objectgraph.CurrentReaderGateMode() != objectgraph.WriterGateObserve ||
			disclosure.CurrentMode() != disclosure.ModeObserve {
			t.Fatal("Observe did not set all three gates to observe")
		}
	})
	if objectgraph.CurrentWriterGateMode() != objectgraph.WriterGateEnforce ||
		objectgraph.CurrentReaderGateMode() != objectgraph.WriterGateEnforce ||
		disclosure.CurrentMode() != disclosure.ModeEnforce {
		t.Fatal("cleanup did not restore enforce on all three gates")
	}
}

// FM10 at runtime: a product-shaped binary that calls a seam dies at the call.
func TestSeamsPanicOutsideATestBinary(t *testing.T) {
	for _, which := range []string{"objectgraph", "disclosure"} {
		cmd := exec.Command("go", "run", "./testdata/callseam", which)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("%s seam returned in a product binary: %s", which, out)
		}
		if !strings.Contains(string(out), "SetModeForTest called outside a test") {
			t.Fatalf("%s seam failed for another reason: %s", which, out)
		}
		if strings.Contains(string(out), "seam returned") {
			t.Fatalf("%s seam ran past the call: %s", which, out)
		}
	}
}

// FM10 at lint time: the gate script refuses a product file that imports the
// fixture or calls a seam, and accepts a _test.go file that does.
func TestLintRefusesProductImportAndCall(t *testing.T) {
	script, err := filepath.Abs("../../../scripts/check-gatefixture-imports")
	if err != nil {
		t.Fatal(err)
	}
	run := func(files map[string]string) (string, error) {
		root := t.TempDir()
		for name, body := range files {
			p := filepath.Join(root, name)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		out, err := exec.Command("bash", script, root).CombinedOutput()
		return string(out), err
	}
	const imp = "package x\n\nimport \"github.com/yusefmosiah/go-choir/internal/objectgraph/gatefixture\"\n\nvar _ = gatefixture.Observe\n"
	const call = "package x\n\nfunc f() { objectgraph.SetModeForTest(1) }\n"
	if out, err := run(map[string]string{"internal/a/a.go": imp}); err == nil {
		t.Fatalf("lint accepted a product import: %s", out)
	}
	if out, err := run(map[string]string{"internal/a/a.go": call}); err == nil {
		t.Fatalf("lint accepted a product seam call: %s", out)
	}
	if out, err := run(map[string]string{"internal/a/a_test.go": imp + call}); err != nil {
		t.Fatalf("lint refused a _test.go file: %v %s", err, out)
	}
	if out, err := run(map[string]string{"internal/a/a.go": "package x\n\n// SetModeForTest( is named in a comment only\nfunc SetModeForTest(m int) {}\n"}); err != nil {
		t.Fatalf("lint refused a comment and a definition: %v %s", err, out)
	}
}
