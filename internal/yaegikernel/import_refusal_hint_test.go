package yaegikernel

import (
	"strings"
	"testing"
)

// Ways this could fail, written first: the hint does not name the choir
// replacement; a cell that still uses os. is "corrected" by dropping its
// import (it would then fail to compile); an unmapped import gets a made-up
// hint; the corrected cell does not run; a block import is left behind.
func TestImportRefusalNamesWhatToUseInstead(t *testing.T) {
	hint := func(src string) string {
		sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer sess.Close()
		_, err = sess.Eval(t.Context(), src)
		if err == nil {
			t.Fatalf("%q was not refused", src)
		}
		return sess.ErrorHint(src, err.Error())
	}

	// Used: a precise hint, no rewritten cell.
	h := hint("import \"os\"\nb, _ := os.ReadFile(\"/x\")\nprintln(len(b))")
	if !strings.Contains(h, "choir.ReadFile") || !strings.Contains(h, "choir.WriteFile") || strings.Contains(h, "Send this cell instead") {
		t.Errorf("used os: %q", h)
	}

	// Unused (single and block forms): the cell comes back without the import.
	for _, src := range []string{
		"import \"os\"\nfmt.Println(\"hi\")",
		"import (\n\t\"fmt\"\n\t\"os\"\n)\nfmt.Println(\"hi\")",
	} {
		h := hint(src)
		at := strings.Index(h, "Send this cell instead:\n")
		if at < 0 {
			t.Fatalf("no corrected cell for %q: %q", src, h)
		}
		fixed := h[at+len("Send this cell instead:\n"):]
		if strings.Contains(fixed, `"os"`) || !strings.Contains(fixed, `fmt.Println("hi")`) {
			t.Errorf("corrected = %q", fixed)
		}
		sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := sess.Eval(t.Context(), fixed); err != nil {
			t.Errorf("corrected cell %q failed: %v", fixed, err)
		}
		sess.Close()
	}

	if h := hint("import \"os/exec\"\n_ = exec.Command"); !strings.Contains(h, "choir.Exec") {
		t.Errorf("os/exec: %q", h)
	}
	if h := hint("import \"unsafe\"\n_ = unsafe.Sizeof(1)"); h != "" {
		t.Errorf("unmapped import got a hint: %q", h)
	}
}
