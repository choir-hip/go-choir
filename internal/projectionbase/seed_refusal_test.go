package projectionbase

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// paper-never-checkpointed S1 review 363: only a refusal of the seed's own
// bytes is ErrSeedRefused (and still ErrBaseRefused); it is what quarantine
// counts.
func TestInstallSeedCandidateCorruptBlobIsSeedRefused(t *testing.T) {
	dir := t.TempDir()
	blob := filepath.Join(dir, "blob")
	if err := os.WriteFile(blob, []byte("not a base"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := installSeedCandidate(context.Background(), SeedCandidate{
		Descriptor: Descriptor{BlobSHA256: strings.Repeat("0", 64)},
		BlobPath:   blob,
	}, filepath.Join(dir, "scratch"))
	if !errors.Is(err, ErrSeedRefused) || !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("err = %v, want both ErrSeedRefused and ErrBaseRefused", err)
	}
}

// A tail-side or target-side refusal is not a verdict on the seed.
func TestTailSideRefusalIsNotSeedRefused(t *testing.T) {
	r := &Rebuilder{cfg: Config{ComputerID: "c", TargetHead: strings.Repeat("a", 64), MaxTailEvents: 1}}
	_, err := r.adoptSeed(context.Background(), nil, t.TempDir(), SeedCandidate{Descriptor: Descriptor{
		ComputerID: "c", CanonicalHead: strings.Repeat("a", 64), Sequence: 1,
	}}, 100)
	if err == nil || errors.Is(err, ErrSeedRefused) {
		t.Fatalf("err = %v, want a non-seed refusal", err)
	}
}
