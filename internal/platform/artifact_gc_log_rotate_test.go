package platform

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
)

// Failure modes, written before the code
// (docs/problems/platform-artifact-retention-2026-10-22.md section 8, F6):
// a day newer than 3 days is compressed (the active file is rotated under the
// writer); a tape-sweep manifest or the scratch log is compressed (breaks a
// pending apply, which names a manifest by sha256); the original is removed
// before the compressed file round-trips; a crash leaves a corrupt .zst that
// passes for the log; a crash after the rename leaves two copies forever; a
// failed rotation loses the original; a dry-run rotates.

func gcLogLines(day string, n int) []byte {
	var b bytes.Buffer
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, `{"at":"%sT00:00:00Z","run":"r","op":"intent","ns":"og","path":"sha256/og/%064d.bin","bytes":%d}`+"\n", day, i, 100+i)
	}
	return b.Bytes()
}

func decompressFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	dec, err := zstd.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer dec.Close()
	var out bytes.Buffer
	if _, err := out.ReadFrom(dec); err != nil {
		t.Fatalf("%s does not decompress: %v", path, err)
	}
	return out.Bytes()
}

func TestRunArtifactGCRotatesOldLogsToZstd(t *testing.T) {
	store, root := openTestPlatformStore(t)
	ensureGCRouteTables(t, store)
	artifactsRoot := filepath.Join(root, "artifacts")
	service := NewService(store, artifactsRoot, "")
	logDir := filepath.Join(artifactsRoot, artifactGCLogDir)
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		t.Fatal(err)
	}
	dayName := func(ago int) string { return time.Now().UTC().AddDate(0, 0, -ago).Format("2006-01-02") }
	write := func(name string, data []byte) string {
		p := filepath.Join(logDir, name)
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-10 * day)
		_ = os.Chtimes(p, old, old)
		return p
	}

	old3 := gcLogLines(dayName(3), 400)
	old9 := gcLogLines(dayName(9), 400)
	p3 := write(dayName(3)+".jsonl", old3)
	p9 := write(dayName(9)+".jsonl", old9)
	// Not rotated: today, yesterday, exactly two days old.
	keep := map[string][]byte{}
	for _, ago := range []int{0, 1, 2} {
		d := dayName(ago)
		keep[write(d+".jsonl", gcLogLines(d, 10))] = gcLogLines(d, 10)
	}
	// Not rotated: a tape-sweep manifest and the scratch sweep's log.
	manifest := write("tape-sweep-ts-20261001T000000Z-deadbeef.manifest.jsonl", gcLogLines("2026-10-01", 10))
	scratchLog := write("checkpoint-scratch-"+dayName(9)+".jsonl", gcLogLines(dayName(9), 10))
	keep[manifest] = gcLogLines("2026-10-01", 10)
	keep[scratchLog] = gcLogLines(dayName(9), 10)

	// A crash after the rename: both copies exist and the .zst is good.
	both := gcLogLines(dayName(5), 50)
	pBoth := write(dayName(5)+".jsonl", both)
	enc, _ := zstd.NewWriter(nil)
	if err := os.WriteFile(pBoth+".zst", enc.EncodeAll(both, nil), 0o600); err != nil {
		t.Fatal(err)
	}
	// A crash mid-write: a corrupt .zst beside the original, and a stray .tmp.
	torn := gcLogLines(dayName(6), 50)
	pTorn := write(dayName(6)+".jsonl", torn)
	if err := os.WriteFile(pTorn+".zst", []byte("not zstd"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logDir, dayName(7)+".jsonl.zst.tmp"), []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	p7 := write(dayName(7)+".jsonl", gcLogLines(dayName(7), 50))
	// A rotation that cannot finish keeps the original: the destination is a
	// directory, so the compressed file can never be installed.
	blocked := gcLogLines(dayName(8), 50)
	pBlocked := write(dayName(8)+".jsonl", blocked)
	if err := os.MkdirAll(pBlocked+".zst", 0o700); err != nil {
		t.Fatal(err)
	}

	// Dry-run rotates nothing.
	if _, err := service.RunArtifactGC(context.Background(), ArtifactGCConfig{Mode: "dry-run", Grace: time.Hour}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p3 + ".zst"); !os.IsNotExist(err) {
		t.Fatalf("dry-run rotated a log: %v", err)
	}

	if _, err := service.RunArtifactGC(context.Background(), ArtifactGCConfig{Mode: "active", Grace: time.Hour}); err != nil {
		t.Fatal(err)
	}

	for p, want := range map[string][]byte{p3: old3, p9: old9, pBoth: both, pTorn: torn} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("original %s kept after a verified rotation: %v", p, err)
		}
		if got := decompressFile(t, p+".zst"); !bytes.Equal(got, want) {
			t.Fatalf("%s.zst does not round-trip", p)
		}
	}
	if _, err := os.Stat(p7 + ".zst"); err != nil {
		t.Fatalf("a stray .tmp blocked the rotation: %v", err)
	}
	if _, err := os.Stat(filepath.Join(logDir, dayName(7)+".jsonl.zst.tmp")); !os.IsNotExist(err) {
		t.Fatal("stray .tmp left behind")
	}
	if got, err := os.ReadFile(pBlocked); err != nil || !bytes.Equal(got, blocked) {
		t.Fatalf("a failed rotation lost the original: %v", err)
	}
	for p, want := range keep {
		got, err := os.ReadFile(p)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("%s was touched: %v", filepath.Base(p), err)
		}
		if _, err := os.Stat(p + ".zst"); !os.IsNotExist(err) {
			t.Fatalf("%s was compressed", filepath.Base(p))
		}
	}
	if small, _ := os.Stat(p9 + ".zst"); small.Size()*4 > int64(len(old9)) {
		t.Fatalf("compressed size %d is not under a quarter of %d", small.Size(), len(old9))
	}

	// The reader that serves both forms returns the same bytes for each.
	for _, d := range []string{dayName(3), dayName(9)} {
		rc, err := openArtifactGCLog(filepath.Join(logDir, d+".jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(rc); err != nil {
			t.Fatal(err)
		}
		rc.Close()
		if !strings.HasPrefix(buf.String(), `{"at":"`+d) {
			t.Fatalf("openArtifactGCLog(%s) = %.60q", d, buf.String())
		}
	}
	rc, err := openArtifactGCLog(filepath.Join(logDir, dayName(1)+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	rc.Close()
}

func newLogRotateService(t *testing.T) (*Service, *Store, string, string) {
	t.Helper()
	store, root := openTestPlatformStore(t)
	ensureGCRouteTables(t, store)
	artifactsRoot := filepath.Join(root, "artifacts")
	logDir := filepath.Join(artifactsRoot, artifactGCLogDir)
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		t.Fatal(err)
	}
	return NewService(store, artifactsRoot, ""), store, root, logDir
}

func writeAgedLog(t *testing.T, path string, data []byte, age time.Duration) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	when := time.Now().Add(-age)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
}

// N7: a pass that failed does not rotate.
func TestRunArtifactGCDoesNotRotateAfterAFailedPass(t *testing.T) {
	service, store, root, logDir := newLogRotateService(t)
	day5 := time.Now().UTC().AddDate(0, 0, -5).Format("2006-01-02")
	p := filepath.Join(logDir, day5+".jsonl")
	writeAgedLog(t, p, gcLogLines(day5, 20), 5*day)
	breakCorpusStore(t, store, root)
	if _, err := service.RunArtifactGC(context.Background(), ArtifactGCConfig{Mode: "active", Grace: time.Hour}); err == nil {
		t.Fatal("expected the live-set failure")
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("a failed pass rotated a log: %v", err)
	}
	if _, err := os.Stat(p + ".zst"); !os.IsNotExist(err) {
		t.Fatalf("a failed pass wrote a .zst: %v", err)
	}
}

// N8: a file whose name says old but whose mtime is recent (a clock step, or a
// log still being written) is not rotated.
func TestRunArtifactGCDoesNotRotateAFileModifiedRecently(t *testing.T) {
	service, _, _, logDir := newLogRotateService(t)
	day5 := time.Now().UTC().AddDate(0, 0, -5).Format("2006-01-02")
	p := filepath.Join(logDir, day5+".jsonl")
	writeAgedLog(t, p, gcLogLines(day5, 20), time.Minute)
	if _, err := service.RunArtifactGC(context.Background(), ArtifactGCConfig{Mode: "active", Grace: time.Hour}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("a recently modified log was rotated: %v", err)
	}
}

// N9: a well-formed .zst that differs from the plain file may be the only
// archived copy of earlier records. Both are kept and the pass warns.
func TestRunArtifactGCKeepsBothWhenAValidZstDiffersFromThePlainFile(t *testing.T) {
	service, _, _, logDir := newLogRotateService(t)
	day5 := time.Now().UTC().AddDate(0, 0, -5).Format("2006-01-02")
	p := filepath.Join(logDir, day5+".jsonl")
	archived := gcLogLines(day5, 30)
	enc, _ := zstd.NewWriter(nil)
	writeAgedLog(t, p+".zst", enc.EncodeAll(archived, nil), 5*day)
	appended := gcLogLines(day5, 5) // a later, different plain file
	writeAgedLog(t, p, appended, 5*day)

	report, err := service.RunArtifactGC(context.Background(), ArtifactGCConfig{Mode: "active", Grace: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if got := decompressFile(t, p+".zst"); !bytes.Equal(got, archived) {
		t.Fatal("the archived .zst was replaced or removed")
	}
	if got, err := os.ReadFile(p); err != nil || !bytes.Equal(got, appended) {
		t.Fatalf("the plain file was removed: %v", err)
	}
	warned := false
	for _, w := range report.Warnings {
		if strings.Contains(w, "differs") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("no divergence warning in %v", report.Warnings)
	}
}
