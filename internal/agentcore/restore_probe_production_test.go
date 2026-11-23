package agentcore

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/dbx/dbxtest"
	"github.com/yusefmosiah/go-choir/internal/snapshot"
)

// Review round 3 of lane/snapshot-by-replay-time. Twice the tests modelled
// production instead of running it: the chunk probe was tested against fakes
// that implement HasChunk, and production's platformRestoreTape did not, so the
// probe was silently skipped and a pruned candidate was downloaded. From here
// each restore-path claim has a test through the production wiring: the real
// platformRestoreTape (built by the same constructor the runtime uses), the real
// computerevent.HTTPClient and snapshot.HTTPChunks, against a fake host that
// only speaks HTTP.
//
// Ways it could fail:
//  a. the production restore tape is not a ChunkProber (the probe is skipped);
//  b. a pruned candidate is downloaded before it fails;
//  c. the next older live candidate is not installed after it;
//  d. the candidate phase has no deadline and failures can starve the replay.

var _ snapshot.ChunkProber = platformRestoreTape{}

type fakeHost struct {
	mu       sync.Mutex
	chunks   map[string][]byte
	payloads map[string][]byte
	gets     int
	heads    int
}

func (h *fakeHost) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch {
	case strings.HasPrefix(r.URL.Path, "/internal/computers/files/chunks/"):
		digest := strings.TrimPrefix(r.URL.Path, "/internal/computers/files/chunks/")
		switch r.Method {
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			h.chunks[digest] = body
			w.WriteHeader(http.StatusCreated)
		case http.MethodHead:
			h.heads++
			if _, ok := h.chunks[digest]; !ok {
				w.WriteHeader(http.StatusNotFound)
			}
		case http.MethodGet:
			h.gets++
			data, ok := h.chunks[digest]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write(data)
		}
	case r.URL.Path == "/internal/computers/files/snapshot-root":
		w.WriteHeader(http.StatusCreated)
	case r.URL.Path == "/internal/computers/events/payload":
		data, ok := h.payloads[r.URL.Query().Get("artifact_digest")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"payload_base64": base64.RawStdEncoding.EncodeToString(data)})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

type hostAppender struct{ host *fakeHost }

func (a hostAppender) HoldAppends(fn func() error) error { return fn() }

func (a hostAppender) AppendNewPrivatePayload(ctx context.Context, event computerevent.Event, _ computerevent.TransitionInput, plaintext []byte, mediaType string, cipher *computerevent.PrivateArtifactCipher) (computerevent.Receipt, string, error) {
	envelope, _, err := cipher.Encrypt(ctx, event.ComputerID, event.EventID, mediaType, "private", plaintext)
	if err != nil {
		return computerevent.Receipt{}, "", err
	}
	digest := computerevent.DigestBytes(envelope)
	a.host.mu.Lock()
	a.host.payloads[digest] = envelope
	a.host.mu.Unlock()
	return computerevent.Receipt{}, digest, nil
}

func TestProductionRestoreTapeIsAChunkProber(t *testing.T) {
	var asInstallSeesIt snapshot.ChunkStore = platformRestoreTape{}
	if _, ok := asInstallSeesIt.(snapshot.ChunkProber); !ok {
		t.Fatal("platformRestoreTape is not a ChunkProber: the install's probe would be skipped")
	}
	var asRuntimeBuildsIt restoreTape = newPlatformRestoreTape(nil, "c")
	if _, ok := asRuntimeBuildsIt.(snapshot.ChunkProber); !ok {
		t.Fatal("the tape the runtime builds is not a ChunkProber")
	}
}

// A pruned candidate is skipped by a probe over HTTP and not downloaded; the
// next older live candidate installs.
func TestProductionRestoreTapeSkipsAPrunedCandidateWithoutDownloadingIt(t *testing.T) {
	ctx := context.Background()
	const computerID = "computer-restore-probe"
	host := &fakeHost{chunks: map[string][]byte{}, payloads: map[string][]byte{}}
	srv := httptest.NewServer(host)
	defer srv.Close()
	client, err := computerevent.NewHTTPClient(srv.URL, srv.Client(), func(context.Context) (string, error) { return "token", nil }, true)
	if err != nil {
		t.Fatal(err)
	}
	tape := newPlatformRestoreTape(client, computerID)

	db := dbxtest.SQLite(t)
	if _, err := db.Write.Exec(`CREATE TABLE blobs (id INTEGER PRIMARY KEY, body BLOB NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	row := make([]byte, 64<<10)
	for i := 0; i < 144; i++ { // about 9 MiB: three chunks
		if _, err := rand.Read(row); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Write.Exec(`INSERT INTO blobs (id, body) VALUES (?, ?)`, i+1, row); err != nil {
			t.Fatal(err)
		}
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	cipher, err := computerevent.NewPrivateArtifactCipher(computerID, key)
	if err != nil {
		t.Fatal(err)
	}
	seq := uint64(40)
	cfg := snapshot.Config{
		ComputerID: computerID,
		DB:         db.Read,
		Head: func(context.Context, *sql.Tx) (*computerevent.Head, error) {
			return &computerevent.Head{ComputerID: computerID, Sequence: seq, CanonicalEventHead: strings.Repeat("e", 64)}, nil
		},
		Cipher:   cipher,
		Chunks:   tape, // the production type takes the snapshot too
		Appender: hostAppender{host},
	}
	first, err := snapshot.TakeSnapshot(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Write.Exec(`UPDATE blobs SET body = x'00' WHERE id = 144`); err != nil {
		t.Fatal(err)
	}
	seq = 80
	second, err := snapshot.TakeSnapshot(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	held := map[string]bool{}
	for _, d := range first.Record.Manifest.Files[0].Chunks {
		held[d] = true
	}
	pruned := 0
	for _, d := range second.Record.Manifest.Files[0].Chunks {
		if !held[d] {
			delete(host.chunks, d) // the host swept what only the newer root named
			pruned++
		}
	}
	if pruned == 0 {
		t.Fatal("test premise: the second snapshot has no unique chunk")
	}
	host.mu.Lock()
	host.gets, host.heads = 0, 0
	host.mu.Unlock()

	candidates := []snapshot.Candidate{
		{EventID: first.EventID, Digest: first.ArtifactDigest, Sequence: 41},
		{EventID: second.EventID, Digest: second.ArtifactDigest, Sequence: 81},
	}
	rcfg := snapshot.RestoreConfig{
		ComputerID: computerID, DestPath: filepath.Join(t.TempDir(), "state.db"),
		Source: tape, Chunks: tape, Cipher: cipher,
		// This test is about the probe; the head tie a version 2 record needs is
		// tested where a real store carries a head row (snapshot_page_digest_test.go).
		Verify: func(context.Context, snapshot.Record, string) error { return nil },
	}
	record, err := snapshot.Install(ctx, rcfg, candidates)
	if err != nil || record.StoreHeadSequence != 40 {
		t.Fatalf("Install = head %d, %v; want the older live snapshot (head 40)", record.StoreHeadSequence, err)
	}
	if want := len(first.Record.Manifest.Files[0].Chunks); host.gets != want {
		t.Fatalf("%d chunk downloads, want %d: the pruned candidate must not be downloaded", host.gets, want)
	}
	if host.heads == 0 {
		t.Fatal("no HEAD reached the host: the probe did not run")
	}
}
