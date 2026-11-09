package capsule

// Failure modes (an assignment's PDF and chart never reached the owner):
//   - too many files, a file over 50 MB, a set over 200 MB are admitted;
//   - a name with a separator, a dot-file, or ".." escapes the directory;
//   - a symlink in the output directory is followed out of the capsule;
//   - a type outside the list, or content that is not its extension (a
//     script named .png), is delivered;
//   - a secret leaves the capsule in a deliverable;
//   - a refused file vanishes without a reason;
//   - a retry leaves a partial or duplicated file.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

var deliverablePNG = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{1}, 64)...)

func stageFrom(t *testing.T, src string, names []string, links map[string]bool, dest string) ([]DeliverableFile, []DeliverableRefusal) {
	t.Helper()
	open := func(name string) (screenInput, error) { return os.Open(filepath.Join(src, name)) }
	acc, ref, err := stageDeliverableFiles(context.Background(), names, links, open, dest)
	if err != nil {
		t.Fatalf("stage: %v", err)
	}
	return acc, ref
}

func writeSrc(t *testing.T, files map[string][]byte) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func refusalFor(refs []DeliverableRefusal, name string) string {
	for _, r := range refs {
		if r.Name == name {
			return r.Reason
		}
	}
	return ""
}

func TestDeliverablesAcceptTypesAndListRefusals(t *testing.T) {
	src := writeSrc(t, map[string][]byte{
		"report.pdf": []byte("%PDF-1.7 body"),
		"chart.png":  deliverablePNG,
		"data.csv":   []byte("a,b\n1,2\n"),
		"fake.png":   []byte("#!/bin/sh\necho hi\n"),
		"tool.exe":   []byte("MZ"),
		"empty.txt":  {},
		"vec.svg":    []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`),
		"bin.txt":    {0, 1, 2, 3},
	})
	names := []string{"report.pdf", "chart.png", "data.csv", "fake.png", "tool.exe", "empty.txt", "vec.svg", "bin.txt"}
	acc, ref := stageFrom(t, src, names, nil, filepath.Join(t.TempDir(), "stage"))
	got := map[string]string{}
	for _, f := range acc {
		got[f.Name] = f.MediaType
	}
	for name, mt := range map[string]string{"report.pdf": "application/pdf", "chart.png": "image/png", "data.csv": "text/csv", "vec.svg": "image/svg+xml"} {
		if got[name] != mt {
			t.Fatalf("%s: media type %q, want %q (accepted %v)", name, got[name], mt, acc)
		}
	}
	for _, name := range []string{"fake.png", "tool.exe", "empty.txt", "bin.txt"} {
		if refusalFor(ref, name) == "" {
			t.Fatalf("%s was dropped without a reason: %v", name, ref)
		}
		if _, ok := got[name]; ok {
			t.Fatalf("%s was accepted", name)
		}
	}
}

func TestDeliverablesNameSanitization(t *testing.T) {
	for _, bad := range []string{"../x.txt", "a/b.txt", ".hidden.txt", "a..b.txt", "", `a\b.txt`} {
		if _, err := SanitizeDeliverableName(bad); err == nil {
			t.Fatalf("%q was admitted", bad)
		}
	}
	if got, err := SanitizeDeliverableName("Q3 report (final).pdf"); err != nil || got != "Q3 report _final_.pdf" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := SanitizeDeliverableName(strings.Repeat("a", 97) + ".txt"); err == nil {
		t.Fatal("a 101-character name was admitted")
	}
	// A nested path from the diff is refused with its reason, not flattened.
	src := writeSrc(t, map[string][]byte{"ok.txt": []byte("hi")})
	acc, ref := stageFrom(t, src, []string{"sub/x.txt", "ok.txt"}, nil, filepath.Join(t.TempDir(), "stage"))
	if len(acc) != 1 || acc[0].Name != "ok.txt" || refusalFor(ref, "sub/x.txt") == "" {
		t.Fatalf("accepted %v, refused %v", acc, ref)
	}
	// Two names that sanitize to one: the second is refused.
	src = writeSrc(t, map[string][]byte{"a b.txt": []byte("1"), "a?b.txt": []byte("2"), "a_b.txt": []byte("3")})
	acc, ref = stageFrom(t, src, []string{"a b.txt", "a?b.txt", "a_b.txt"}, nil, filepath.Join(t.TempDir(), "stage"))
	if len(acc) != 2 || len(ref) != 1 {
		t.Fatalf("accepted %v, refused %v", acc, ref)
	}
}

func TestDeliverablesSymlinkIsNeverFollowed(t *testing.T) {
	secret := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(secret, []byte("host file"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := t.TempDir()
	if err := os.Symlink(secret, filepath.Join(src, "link.txt")); err != nil {
		t.Fatal(err)
	}
	// Named as a link by the diff.
	dest := filepath.Join(t.TempDir(), "stage")
	acc, ref := stageFrom(t, src, []string{"link.txt"}, map[string]bool{"link.txt": true}, dest)
	if len(acc) != 0 || refusalFor(ref, "link.txt") == "" {
		t.Fatalf("accepted %v, refused %v", acc, ref)
	}
	// Even if the diff missed it, the opener refuses a link (openBeneath uses
	// RESOLVE_NO_SYMLINKS; O_NOFOLLOW stands in for it here).
	open := func(name string) (screenInput, error) {
		return os.OpenFile(filepath.Join(src, name), os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	}
	acc, ref, err := stageDeliverableFiles(context.Background(), []string{"link.txt"}, nil, open, dest)
	if err != nil {
		t.Fatal(err)
	}
	if len(acc) != 0 || refusalFor(ref, "link.txt") == "" {
		t.Fatalf("accepted %v, refused %v", acc, ref)
	}
}

func TestDeliverablesBounds(t *testing.T) {
	files := map[string][]byte{}
	var names []string
	for i := 0; i < MaxDeliverables+3; i++ {
		n := "f" + string(rune('a'+i)) + ".txt"
		files[n] = []byte("x")
		names = append(names, n)
	}
	src := writeSrc(t, files)
	acc, ref := stageFrom(t, src, names, nil, filepath.Join(t.TempDir(), "stage"))
	if len(acc) != MaxDeliverables || len(ref) != 3 {
		t.Fatalf("accepted %d refused %d", len(acc), len(ref))
	}
	for _, r := range ref {
		if !strings.Contains(r.Reason, "limit") {
			t.Fatalf("reason %q", r.Reason)
		}
	}
	// One file over the per-file bound (sparse, so cheap).
	dir := t.TempDir()
	big, err := os.Create(filepath.Join(dir, "big.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = big.WriteString("%PDF-1.7")
	if err := big.Truncate(MaxDeliverableBytes + 1); err != nil {
		t.Fatal(err)
	}
	_ = big.Close()
	acc, ref = stageFrom(t, dir, []string{"big.pdf"}, nil, filepath.Join(t.TempDir(), "stage"))
	if len(acc) != 0 || !strings.Contains(refusalFor(ref, "big.pdf"), "larger than") {
		t.Fatalf("accepted %v refused %v", acc, ref)
	}
	// The set bound: five files of 50 MB fit 200 MB only four times.
	dir = t.TempDir()
	var five []string
	for i := 0; i < 5; i++ {
		n := "p" + string(rune('a'+i)) + ".pdf"
		f, err := os.Create(filepath.Join(dir, n))
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.WriteString("%PDF-1.7")
		if err := f.Truncate(MaxDeliverableBytes); err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
		five = append(five, n)
	}
	if testing.Short() {
		return
	}
	acc, ref = stageFrom(t, dir, five, nil, filepath.Join(t.TempDir(), "stage"))
	if len(acc) != 4 || len(ref) != 1 || !strings.Contains(ref[0].Reason, "exceed") {
		t.Fatalf("accepted %d refused %v", len(acc), ref)
	}
}

func TestDeliverablesIdempotentRestageLeavesNoTemp(t *testing.T) {
	src := writeSrc(t, map[string][]byte{"a.txt": []byte("one"), "b.png": deliverablePNG})
	dest := filepath.Join(t.TempDir(), "stage")
	for i := 0; i < 2; i++ {
		acc, ref := stageFrom(t, src, []string{"a.txt", "b.png"}, nil, dest)
		if len(acc) != 2 || len(ref) != 0 {
			t.Fatalf("round %d: %v %v", i, acc, ref)
		}
	}
	entries, _ := os.ReadDir(dest)
	if len(entries) != 2 {
		t.Fatalf("stage holds %d entries, want 2: %v", len(entries), entries)
	}
}

func TestDeliverableNamesFromDiff(t *testing.T) {
	changes := []FileChange{
		{Path: "var/lib/artifact/deliverables/r.pdf", Kind: ChangeAdded},
		{Path: "/var/lib/artifact/deliverables/gone.pdf", Kind: ChangeDeleted},
		{Path: "var/lib/artifact/deliverables/sub", Kind: ChangeAdded, Mode: os.ModeDir},
		{Path: "var/lib/artifact/deliverables/sub/x.txt", Kind: ChangeAdded},
		{Path: "var/lib/artifact/deliverables/l.txt", Kind: ChangeAdded, Mode: os.ModeSymlink},
		{Path: "var/lib/artifact/screens/s.png", Kind: ChangeAdded},
	}
	names, links := deliverableNames(changes)
	want := []string{"l.txt", "r.pdf", "sub/x.txt"}
	if strings.Join(names, ",") != strings.Join(want, ",") || !links["l.txt"] || links["r.pdf"] {
		t.Fatalf("names %v links %v", names, links)
	}
}
