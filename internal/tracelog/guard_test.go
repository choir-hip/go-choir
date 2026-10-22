package tracelog

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// No production package imports internal/tracelog yet (S1 ships the library
// with no caller; S2 is the first slice that wires it). The guard fails when
// a non-test source outside this package names the import path, the same way
// tapestore guarded its fixture writer. S2 changes the allow-list in the same
// commit that adds its caller.
func TestNoProductionPackageImportsTracelogYet(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("module root not found")
		}
		dir = parent
	}
	self := filepath.Join(dir, "internal", "tracelog")
	allowed := map[string]bool{} // S2 adds internal/store here
	var offenders []string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case "vendor", "node_modules", ".git":
				return filepath.SkipDir
			}
			if path == self {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if bytes.Contains(raw, []byte("go-choir/internal/tracelog")) {
			rel, _ := filepath.Rel(dir, filepath.Dir(path))
			if !allowed[rel] {
				offenders = append(offenders, path)
			}
		}
		return nil
	})
	if len(offenders) > 0 {
		t.Fatalf("no production package may import internal/tracelog before S2; found %v", offenders)
	}
}

// The library uses only the standard library, so the offline readers and
// the store can import it without a cycle.
func TestTracelogImportsOnlyTheStandardLibrary(t *testing.T) {
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		raw, _ := os.ReadFile(f)
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "\"") && strings.Contains(line, ".") && strings.Contains(line, "/") {
				if !strings.HasPrefix(line, "\"golang.org/x/sys") {
					t.Errorf("%s imports %s", f, line)
				}
			}
		}
	}
}
