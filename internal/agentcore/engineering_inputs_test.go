package agentcore

// Failure modes (research's captured FRED series never reached the
// engineering capsule, and management pasted whole tables into the Cast):
//   - a capture the Cast names does not arrive, or arrives with other bytes;
//   - a capture the Cast did not name arrives;
//   - an owner-private capture, or a mail id, reaches the capsule;
//   - two series of one file name (fredgraph.csv?id=A and ?id=B) collide;
//   - a missing capture fails the spawn instead of showing in the manifest;
//   - a hostile URL or title makes a name that leaves the directory.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
)

type fakeInputCaptures struct {
	records map[string]map[string]any
	bodies  map[string][]byte
	asked   []string
}

func (f *fakeInputCaptures) ReadCapture(_ context.Context, id string, _, _ int) (json.RawMessage, error) {
	f.asked = append(f.asked, id)
	rec, ok := f.records[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", researchtools.ErrCaptureNotFound, id)
	}
	return json.Marshal(rec)
}

func (f *fakeInputCaptures) OpenCaptureBody(_ context.Context, blob string) (io.ReadCloser, error) {
	b, ok := f.bodies[blob]
	if !ok {
		return nil, fmt.Errorf("no body %s", blob)
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func captureID(t *testing.T, suffix string) string {
	t.Helper()
	id, err := objectgraph.BuildCanonicalID(objectgraph.WebCaptureObjectKind, "owner-1", suffix)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestInputSourcesDeliverOnlyNamedWorldCapturesAsManifestedFiles(t *testing.T) {
	unrate, cpi := captureID(t, "unrate"), captureID(t, "cpi")
	private, other := captureID(t, "private"), captureID(t, "unnamed")
	f := &fakeInputCaptures{
		records: map[string]map[string]any{
			unrate:  {"url": "https://fred.stlouisfed.org/graph/fredgraph.csv?id=UNRATE", "media_type": "text/csv", "content_blob_id": "sha256:u", "raw_bytes": 12},
			cpi:     {"url": "https://fred.stlouisfed.org/graph/fredgraph.csv?id=CPIAUCSL", "media_type": "text/plain", "content_blob_id": "sha256:c", "raw_bytes": 12},
			private: {"url": "https://example.test/p", "media_type": "text/csv", "content_blob_id": "sha256:p", "labels": []string{"owner_private:capture"}},
			other:   {"url": "https://example.test/o.csv", "media_type": "text/csv", "content_blob_id": "sha256:o"},
		},
		bodies: map[string][]byte{"sha256:u": []byte("DATE,UNRATE\n"), "sha256:c": []byte("DATE,CPIAUCSL\n"), "sha256:p": []byte("secret\n"), "sha256:o": []byte("unnamed\n")},
	}
	named := []string{unrate, cpi, private, "obj:choir.mail:owner-1:m1", captureID(t, "gone")}
	dir := t.TempDir()
	manifest, err := capsule.StageInputs(context.Background(), inputSourcesForCaptures(context.Background(), named, f), dir)
	if err != nil {
		t.Fatalf("a refused or missing ref failed the stage: %v", err)
	}
	if len(manifest.Files) != 2 || len(manifest.Refused) != 3 {
		t.Fatalf("manifest = %+v", manifest)
	}
	names := map[string]capsule.InputFile{}
	for _, file := range manifest.Files {
		names[file.Name] = file
		got, err := os.ReadFile(filepath.Join(dir, file.Name))
		if err != nil || int64(len(got)) != file.Bytes {
			t.Fatalf("%s: %q %v", file.Name, got, err)
		}
		if !strings.HasPrefix(file.URL, "https://fred.stlouisfed.org/graph/fredgraph.csv?id=") || !strings.HasSuffix(file.Name, ".csv") {
			t.Fatalf("file = %+v", file)
		}
	}
	if len(names) != 2 {
		t.Fatalf("same-named series collided: %+v", manifest.Files)
	}
	for _, e := range []string{"secret", "unnamed"} {
		entries, _ := os.ReadDir(dir)
		for _, en := range entries {
			if b, _ := os.ReadFile(filepath.Join(dir, en.Name())); strings.Contains(string(b), e) {
				t.Fatalf("%s reached the inputs", e)
			}
		}
	}
	reasons := manifest.Refused[0].Reason + "|" + manifest.Refused[1].Reason + "|" + manifest.Refused[2].Reason
	for _, want := range []string{"owner-private", "mail is never materialized", "not found"} {
		if !strings.Contains(reasons, want) {
			t.Errorf("refusals %q lack %q", reasons, want)
		}
	}
	for _, id := range f.asked {
		if id == other {
			t.Fatal("an unnamed capture was read")
		}
	}
}

func TestInputSourcesWithoutACaptureStoreRefuseVisibly(t *testing.T) {
	dir := t.TempDir()
	manifest, err := capsule.StageInputs(context.Background(), inputSourcesForCaptures(context.Background(), []string{captureID(t, "x")}, nil), dir)
	if err != nil || len(manifest.Files) != 0 || len(manifest.Refused) != 1 {
		t.Fatalf("manifest = %+v, %v", manifest, err)
	}
}

func TestInputFileNameIsFlatUniqueAndTyped(t *testing.T) {
	a := inputFileName("id-a", "https://x.test/graph/fredgraph.csv?id=UNRATE", "text/plain")
	b := inputFileName("id-b", "https://x.test/graph/fredgraph.csv?id=CPIAUCSL", "text/plain")
	if a == b || !strings.HasSuffix(a, ".csv") || !strings.Contains(a, "UNRATE") {
		t.Fatalf("a=%q b=%q", a, b)
	}
	for _, hostile := range []string{"https://x.test/../../etc/passwd", "https://x.test/%2e%2e/%2fa?x=/../b", "::not a url::", ""} {
		name := inputFileName("id", hostile, "application/octet-stream")
		if _, err := capsule.SanitizeDeliverableName(name); err != nil || name != filepath.Base(name) {
			t.Errorf("%q -> %q: %v", hostile, name, err)
		}
	}
	if got := inputFileName("id", "https://x.test/data", "application/json"); !strings.HasSuffix(got, ".json") {
		t.Fatalf("got %q", got)
	}
}
