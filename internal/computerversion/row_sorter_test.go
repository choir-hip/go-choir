package computerversion

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"testing"
)

// referenceRowHash is the in-memory digest the witness has always used:
// sort the row encodings, then hash each followed by a newline.
func referenceRowHash(rows []string) string {
	sorted := append([]string(nil), rows...)
	sort.Strings(sorted)
	hasher := sha256.New()
	for _, r := range sorted {
		hasher.Write([]byte(r))
		hasher.Write([]byte("\n"))
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}

// A table bigger than memory must hash to the same digest as the in-memory
// sort, or every existing witness stops re-verifying.
func TestRowSorterSpilledDigestMatchesInMemory(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	rows := make([]string, 0, 1000)
	for i := 0; i < 1000; i++ {
		rows = append(rows, fmt.Sprintf(`{"id":%d,"v":"%x"}`, rng.Intn(200), rng.Int63()))
	}
	rows = append(rows, rows[:50]...) // duplicate rows are kept, not merged
	rows = append(rows, "")
	want := referenceRowHash(rows)

	for _, limit := range []int{1, 64, 4096, 1 << 30} {
		dir := t.TempDir()
		s := newRowSorter(dir, limit)
		for _, r := range rows {
			if err := s.Add(r); err != nil {
				t.Fatalf("limit %d: add: %v", limit, err)
			}
		}
		got, err := s.Sum()
		if err != nil {
			t.Fatalf("limit %d: sum: %v", limit, err)
		}
		if got != want {
			t.Fatalf("limit %d: digest %s, want %s", limit, got, want)
		}
		s.Close()
		left, _ := os.ReadDir(dir)
		if len(left) != 0 {
			t.Fatalf("limit %d: %d spill files left behind", limit, len(left))
		}
	}
}

func TestRowSorterEmptyTable(t *testing.T) {
	s := newRowSorter(t.TempDir(), 16)
	defer s.Close()
	got, err := s.Sum()
	if err != nil {
		t.Fatal(err)
	}
	if want := referenceRowHash(nil); got != want {
		t.Fatalf("digest %s, want %s", got, want)
	}
}
