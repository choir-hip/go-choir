package capsule

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func inputBytes(name string, data []byte) InputSource {
	return InputSource{Ref: "obj:choir.web_capture:o:" + name, Name: name, URL: "https://example.test/" + name, MediaType: "text/csv", Size: int64(len(data)),
		Open: func(context.Context) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil }}
}

func readManifest(t *testing.T, dir string) InputsManifest {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, InputsManifestName))
	if err != nil {
		t.Fatal(err)
	}
	var m InputsManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestStageInputsNamedCaptureLandsWithBytesAndManifest(t *testing.T) {
	dir := t.TempDir()
	data := []byte("DATE,UNRATE\n2026-01-01,4.1\n")
	m, err := StageInputs(context.Background(), []InputSource{inputBytes("fredgraph-UNRATE.csv", data)}, dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "fredgraph-UNRATE.csv"))
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("file bytes = %q, %v", got, err)
	}
	sum := sha256.Sum256(data)
	disk := readManifest(t, dir)
	if len(m.Files) != 1 || len(disk.Files) != 1 {
		t.Fatalf("manifest = %+v / %+v", m, disk)
	}
	f := disk.Files[0]
	if f.SHA256 != hex.EncodeToString(sum[:]) || f.Bytes != int64(len(data)) || f.MediaType != "text/csv" || f.URL != "https://example.test/fredgraph-UNRATE.csv" || !strings.HasSuffix(f.Source, "fredgraph-UNRATE.csv") {
		t.Fatalf("manifest file = %+v", f)
	}
}

func TestStageInputsUnnamedCaptureIsNotThere(t *testing.T) {
	dir := t.TempDir()
	if _, err := StageInputs(context.Background(), []InputSource{inputBytes("named.csv", []byte("a,b\n"))}, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "other.csv")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("an unnamed capture is staged: %v", err)
	}
	if m := readManifest(t, dir); len(m.Files) != 1 {
		t.Fatalf("manifest = %+v", m)
	}
}

func TestStageInputsRefusalsAreVisibleNeverAnError(t *testing.T) {
	dir := t.TempDir()
	items := []InputSource{
		{Ref: "obj:choir.web_capture:o:gone", Refusal: "capture not found"},
		{Ref: "obj:choir.mail:o:m1", Refusal: "not a web capture id; mail is never materialized"},
		{Ref: "obj:choir.web_capture:o:err", Name: "err.csv", Size: -1, Open: func(context.Context) (io.ReadCloser, error) { return nil, errors.New("corpusd returned 500") }},
		inputBytes("ok.csv", []byte("x\n")),
	}
	m, err := StageInputs(context.Background(), items, dir)
	if err != nil {
		t.Fatalf("a missing ref failed the stage: %v", err)
	}
	disk := readManifest(t, dir)
	if len(disk.Files) != 1 || len(disk.Refused) != 3 || len(m.Refused) != 3 {
		t.Fatalf("manifest = %+v", disk)
	}
	if !strings.Contains(disk.Refused[0].Reason, "not found") || !strings.Contains(disk.Refused[2].Reason, "500") {
		t.Fatalf("reasons = %+v", disk.Refused)
	}
	// Every ref refused still leaves a manifest the capsule can read.
	empty := t.TempDir()
	if _, err := StageInputs(context.Background(), items[:1], empty); err != nil {
		t.Fatal(err)
	}
	if m := readManifest(t, empty); len(m.Files) != 0 || len(m.Refused) != 1 {
		t.Fatalf("all-refused manifest = %+v", m)
	}
}

func TestStageInputsBounds(t *testing.T) {
	t.Run("file count", func(t *testing.T) {
		var items []InputSource
		for i := 0; i < MaxInputs+3; i++ {
			items = append(items, inputBytes(fmt.Sprintf("f%02d.csv", i), []byte("x\n")))
		}
		m, err := StageInputs(context.Background(), items, t.TempDir())
		if err != nil || len(m.Files) != MaxInputs || len(m.Refused) != 3 || !strings.Contains(m.Refused[0].Reason, "20-file") {
			t.Fatalf("files=%d refused=%+v err=%v", len(m.Files), m.Refused, err)
		}
	})
	t.Run("one file declared over", func(t *testing.T) {
		big := InputSource{Ref: "big", Name: "big.csv", Size: MaxInputBytes + 1, Open: func(context.Context) (io.ReadCloser, error) {
			t.Error("an over-size file must not be fetched")
			return nil, errors.New("fetched")
		}}
		m, _ := StageInputs(context.Background(), []InputSource{big}, t.TempDir())
		if len(m.Files) != 0 || len(m.Refused) != 1 || !strings.Contains(m.Refused[0].Reason, "50 MB") {
			t.Fatalf("manifest = %+v", m)
		}
	})
	t.Run("one file over though it declared none", func(t *testing.T) {
		dir := t.TempDir()
		huge := InputSource{Ref: "huge", Name: "huge.csv", Size: -1, Open: func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(io.LimitReader(zeroReader{}, MaxInputBytes+10)), nil
		}}
		m, _ := StageInputs(context.Background(), []InputSource{huge}, dir)
		if len(m.Files) != 0 || len(m.Refused) != 1 || !strings.Contains(m.Refused[0].Reason, "50 MB") {
			t.Fatalf("manifest = %+v", m)
		}
		if _, err := os.Stat(filepath.Join(dir, "huge.csv")); err == nil {
			t.Fatal("over-size bytes were left in the directory")
		}
	})
	t.Run("total", func(t *testing.T) {
		var items []InputSource
		for i := 0; i < 5; i++ {
			i := i
			items = append(items, InputSource{Ref: fmt.Sprintf("r%d", i), Name: fmt.Sprintf("p%d.bin", i), Size: -1, Open: func(context.Context) (io.ReadCloser, error) {
				return io.NopCloser(io.LimitReader(zeroReader{}, 45<<20)), nil
			}})
		}
		m, err := StageInputs(context.Background(), items, t.TempDir())
		var total int64
		for _, f := range m.Files {
			total += f.Bytes
		}
		if err != nil || total > MaxInputsTotalBytes || len(m.Files) != 4 || len(m.Refused) != 1 || !strings.Contains(m.Refused[0].Reason, "200 MB") {
			t.Fatalf("files=%d total=%d refused=%+v err=%v", len(m.Files), total, m.Refused, err)
		}
	})
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func TestStageInputsNamesAreSanitized(t *testing.T) {
	dir := t.TempDir()
	items := []InputSource{
		inputBytes("../../etc/passwd", []byte("x")),
		inputBytes("sub/dir.csv", []byte("x")),
		inputBytes(".hidden.csv", []byte("x")),
		inputBytes("manifest.json", []byte("{}")),
		inputBytes("a b$c.csv", []byte("x")),
		inputBytes("A B_C.CSV", []byte("y")),
	}
	m, err := StageInputs(context.Background(), items, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Files) != 1 || m.Files[0].Name != "a b_c.csv" {
		t.Fatalf("files = %+v", m.Files)
	}
	if len(m.Refused) != 5 {
		t.Fatalf("refused = %+v", m.Refused)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "a b_c.csv" && e.Name() != InputsManifestName {
			t.Fatalf("stray entry %q", e.Name())
		}
	}
	// manifest.json is the host's, never an input's.
	if got := readManifest(t, dir); len(got.Files) != 1 {
		t.Fatalf("manifest overwritten by an input: %+v", got)
	}
}

func TestMaterializeInputsLinksOrCopiesReadOnly(t *testing.T) {
	src := t.TempDir()
	if _, err := StageInputs(context.Background(), []InputSource{inputBytes("a.csv", []byte("1,2\n"))}, src); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, ".stage-leftover"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), filepath.FromSlash(InputsPrefix))
	linked, copied, n, err := materializeInputs(context.Background(), src, dst)
	if err != nil {
		t.Fatal(err)
	}
	if linked+copied != 2 || n == 0 {
		t.Fatalf("linked=%d copied=%d bytes=%d", linked, copied, n)
	}
	if _, err := os.Stat(filepath.Join(dst, ".stage-leftover")); err == nil {
		t.Fatal("a dot-file was materialized")
	}
	info, err := os.Stat(filepath.Join(dst, "a.csv"))
	if err != nil || info.Mode().Perm()&0o222 != 0 {
		t.Fatalf("input is writable: %v %v", info, err)
	}
	if got, _ := os.ReadFile(filepath.Join(dst, "a.csv")); string(got) != "1,2\n" {
		t.Fatalf("bytes = %q", got)
	}
	t.Cleanup(func() { _ = os.Chmod(dst, 0o755) })
}

func TestMaterializeInputsFallsBackToCopy(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "a.csv"), []byte("z"), 0o600); err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	// A destination file that already exists makes the link fail; the copy
	// then fails exclusively too, so the error is surfaced, not swallowed.
	if err := os.WriteFile(filepath.Join(dst, "a.csv"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := materializeInputs(context.Background(), src, dst); err == nil {
		t.Fatal("expected an error when the destination already holds the name")
	}
}

// A staging failure leaves a manifest that refuses every named ref with the
// cause, never an empty or half-filled directory.
func TestStageInputsFailureWritesARefusalManifest(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "in")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "partial.csv"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := StageInputsFailure(dir, []string{"obj:a", "obj:b"}, context.DeadlineExceeded); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "partial.csv")); !os.IsNotExist(err) {
		t.Fatalf("a partial file survived: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, InputsManifestName))
	if err != nil {
		t.Fatal(err)
	}
	var m InputsManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Files) != 0 || len(m.Refused) != 2 || !strings.Contains(m.Refused[0].Reason, "deadline") {
		t.Fatalf("manifest = %+v", m)
	}
}
