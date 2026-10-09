package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixture = `package fixture

import (
	"net/http"
	"sync"
	"time"
)

type svc struct {
	mu     sync.Mutex
	cond   *sync.Cond
	client *http.Client
	ch     chan int
}

// mode 1: deferred unlock holds to the end.
func (s *svc) deferred(req *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.client.Do(req)
}

// mode 2: goroutines and func literals run later.
func (s *svc) later(req *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	go s.client.Do(req)
	f := func() { time.Sleep(time.Second) }
	_ = f
}

// mode 3: unlock ends the span.
func (s *svc) released() {
	s.mu.Lock()
	s.ch = nil
	s.mu.Unlock()
	time.Sleep(time.Second)
}

// mode 4: unlock in an early-return branch does not release the rest.
func (s *svc) branch(ok bool) {
	s.mu.Lock()
	if !ok {
		s.mu.Unlock()
		return
	}
	<-s.ch
	s.mu.Unlock()
}

// mode 7: Cond.Wait under its lock is correct.
func (s *svc) condWait() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cond.Wait()
}
`

func scanFixture(t *testing.T, padding string) []Finding {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "fixture")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := strings.Replace(fixture, "package fixture\n", "package fixture\n"+padding, 1)
	if err := os.WriteFile(filepath.Join(dir, "fixture.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	// mode 6: a test file with a violation is ignored.
	testSrc := "package fixture\nimport \"time\"\nfunc (s *svc) inTest() { s.mu.Lock(); defer s.mu.Unlock(); time.Sleep(1) }\n"
	if err := os.WriteFile(filepath.Join(dir, "fixture_test.go"), []byte(testSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err := Scan(root, []string{"internal"})
	if err != nil {
		t.Fatal(err)
	}
	return findings
}

func TestLockscopeFailureModes(t *testing.T) {
	findings := scanFixture(t, "")
	got := map[string]bool{}
	for _, f := range findings {
		got[f.Func+" "+f.Class] = true
	}
	want := []string{"svc.deferred net", "svc.branch chan"}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing finding %q in %+v", w, findings)
		}
	}
	for _, f := range findings {
		switch f.Func {
		case "svc.later", "svc.released", "svc.condWait", "svc.inTest":
			t.Errorf("false finding: %+v", f)
		}
	}
	if len(findings) != len(want) {
		t.Errorf("findings = %d, want %d: %+v", len(findings), len(want), findings)
	}
}

// mode 5: keys survive edits that move lines.
func TestLockscopeKeysIgnoreLineNumbers(t *testing.T) {
	before := scanFixture(t, "")
	after := scanFixture(t, "\n\n\n// moved\n")
	if len(before) != len(after) {
		t.Fatalf("finding count changed: %d vs %d", len(before), len(after))
	}
	for i := range before {
		if before[i].Line == after[i].Line {
			t.Fatalf("fixture did not move lines")
		}
		if before[i].Key() != after[i].Key() {
			t.Fatalf("key changed with line: %q vs %q", before[i].Key(), after[i].Key())
		}
	}
}
