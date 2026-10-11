package capturesegment

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// W1b (docs/definitions/choir-autopaper-w1-content-in-the-graph-2026-10-11.md):
// capture bodies live in one append-only segment per UTC day. Failure modes
// pinned: a ref names bytes that are not there or that changed; concurrent
// appends interleave; a crash leaves a partial last line that corrupts the
// next append; a ref escapes the captures directory.
func TestAppendAndReadRoundTrip(t *testing.T) {
	root := t.TempDir()
	a := New(root)
	day := time.Date(2026, 10, 11, 23, 59, 0, 0, time.UTC)
	ref1, err := a.Append(day, []byte("first body\nwith a newline"))
	if err != nil {
		t.Fatal(err)
	}
	ref2, err := a.Append(day, []byte("second"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(ref1, "capseg:2026-10-11:") || ref1 == ref2 {
		t.Fatalf("refs = %q %q", ref1, ref2)
	}
	for ref, want := range map[string]string{ref1: "first body\nwith a newline", ref2: "second"} {
		got, err := Read(root, ref)
		if err != nil || string(got) != want {
			t.Fatalf("Read(%q) = %q, %v; want %q", ref, got, err, want)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "captures", "2026", "10", "11.jsonl")); err != nil {
		t.Fatalf("segment not at the day path: %v", err)
	}
}

func TestReadRejectsChangedOrMissingBytes(t *testing.T) {
	root := t.TempDir()
	a := New(root)
	ref, err := a.Append(time.Now().UTC(), []byte("body"))
	if err != nil {
		t.Fatal(err)
	}
	path, _, _, _, err := parseRef(root, ref)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if err := os.WriteFile(path, bytes.Replace(raw, []byte("Ym9keQ"), []byte("Ym9keA"), 1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(root, ref); err == nil {
		t.Fatal("changed bytes read without error")
	}
	parts := strings.Split(ref, ":")
	parts[2] = "999999"
	if _, err := Read(root, strings.Join(parts, ":")); err == nil {
		t.Fatal("missing bytes read without error")
	}
}

func TestConcurrentAppendsDoNotInterleave(t *testing.T) {
	root := t.TempDir()
	a := New(root)
	day := time.Now().UTC()
	var wg sync.WaitGroup
	refs := make([]string, 40)
	for i := range refs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ref, err := a.Append(day, []byte(strings.Repeat(fmt.Sprint(i%10), 5000)))
			if err != nil {
				t.Error(err)
			}
			refs[i] = ref
		}(i)
	}
	wg.Wait()
	for i, ref := range refs {
		got, err := Read(root, ref)
		if err != nil || string(got) != strings.Repeat(fmt.Sprint(i%10), 5000) {
			t.Fatalf("ref %d corrupted: %v", i, err)
		}
	}
}

func TestPartialLastLineDoesNotCorruptTheNextAppend(t *testing.T) {
	root := t.TempDir()
	a := New(root)
	day := time.Now().UTC()
	if _, err := a.Append(day, []byte("before crash")); err != nil {
		t.Fatal(err)
	}
	path := a.segmentPath(day)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(`{"sha256":"abc","body":"trunc`)
	_ = f.Close()
	ref, err := New(root).Append(day, []byte("after crash"))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Read(root, ref); err != nil || string(got) != "after crash" {
		t.Fatalf("append after a partial line = %q, %v", got, err)
	}
}

func TestReadRejectsRefsOutsideTheCapturesDirectory(t *testing.T) {
	root := t.TempDir()
	for _, ref := range []string{
		"capseg:../../etc:0:10:" + strings.Repeat("a", 64),
		"capseg:2026-13-40:0:10:" + strings.Repeat("a", 64),
		"file:/etc/passwd",
	} {
		if _, err := Read(root, ref); err == nil {
			t.Fatalf("Read(%q) accepted", ref)
		}
	}
}
