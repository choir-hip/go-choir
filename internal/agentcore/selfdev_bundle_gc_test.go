package agentcore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRemoveUnnamedBundlesKeepsWhatAnOpenOperationNamesAndWhatIsYoung(t *testing.T) {
	incoming := t.TempDir()
	now := time.Now()
	make := func(name, marker string, age time.Duration) string {
		dir := filepath.Join(incoming, name)
		if err := os.MkdirAll(filepath.Join(dir, "evidence"), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, p := range []string{filepath.Join(dir, marker), filepath.Join(dir, "evidence", "shot.png")} {
			if marker == "" && strings.HasSuffix(p, "/") {
				continue
			}
			if err := os.WriteFile(p, []byte("12345"), 0o444); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(p, now.Add(-age), now.Add(-age)); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Chtimes(filepath.Join(dir, "evidence"), now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(filepath.Join(dir, "evidence"), 0o555); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(dir, now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	settled := make("1"+strings.Repeat("a", 63), "bundle.json", 48*time.Hour)
	settledDraft := make("2"+strings.Repeat("a", 63), "bundle.draft.json", 48*time.Hour)
	open := make("3"+strings.Repeat("a", 63), "bundle.json", 48*time.Hour)
	young := make("4"+strings.Repeat("a", 63), "bundle.json", time.Minute)
	payload := filepath.Join(incoming, "5"+strings.Repeat("a", 63))
	if err := os.MkdirAll(payload, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, d := range []string{settled, settledDraft, open, young} {
			_ = os.Chmod(filepath.Join(d, "evidence"), 0o755)
		}
	})
	removed, freed := removeUnnamedBundles(incoming, map[string]bool{filepath.Base(open): true}, now, 10*time.Minute)
	if removed != 2 || freed != 20 {
		t.Fatalf("removed=%d freed=%d", removed, freed)
	}
	for _, gone := range []string{settled, settledDraft} {
		if _, err := os.Lstat(gone); err == nil {
			t.Fatalf("%s survived", gone)
		}
	}
	for _, kept := range []string{open, young, payload} {
		if _, err := os.Lstat(kept); err != nil {
			t.Fatalf("%s was removed: %v", kept, err)
		}
	}
}
