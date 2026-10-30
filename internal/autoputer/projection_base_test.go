package autoputer

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/dbx/dbxtest"
	"github.com/yusefmosiah/go-choir/internal/recoveryplan"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/snapshot"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// bootServer is a platform stand-in: the canonical head, the replay pages, the
// event payloads and the file CAS chunks the boot restore reads.
type bootServer struct {
	mu       sync.Mutex
	head     *computerevent.Head
	events   []computerevent.DurableEvent
	payloads map[string][]byte
	chunks   map[string][]byte
}

func newBootServer(head *computerevent.Head) *bootServer {
	return &bootServer{head: head, payloads: map[string][]byte{}, chunks: map[string][]byte{}}
}

func (b *bootServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch {
	case r.URL.Path == "/internal/computers/events/head":
		if b.head == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(b.head)
	case r.URL.Path == "/internal/computers/events/replay":
		after, _ := strconv.ParseUint(r.URL.Query().Get("after_sequence"), 10, 64)
		page := []computerevent.DurableEvent{}
		for _, e := range b.events {
			if e.Request.Event.Sequence > after {
				page = append(page, e)
			}
		}
		_ = json.NewEncoder(w).Encode(page)
	case r.URL.Path == "/internal/computers/events/payload":
		data, ok := b.payloads[r.URL.Query().Get("artifact_digest")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"payload_base64": base64.RawStdEncoding.EncodeToString(data)})
	case r.URL.Path == "/internal/computers/files/snapshot-root":
		w.WriteHeader(http.StatusCreated)
	case strings.HasPrefix(r.URL.Path, "/internal/computers/files/chunks/"):
		digest := strings.TrimPrefix(r.URL.Path, "/internal/computers/files/chunks/")
		if r.Method == http.MethodPut {
			body, _ := io.ReadAll(r.Body)
			b.chunks[digest] = body
			w.WriteHeader(http.StatusCreated)
			return
		}
		data, ok := b.chunks[digest]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(data)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

var testCapability = func(ctx context.Context) (string, error) { return "test-cap", nil }

func TestMaterializeBootstrapsNewComputerWithoutSnapshot(t *testing.T) {
	server := httptest.NewServer(newBootServer(nil))
	defer server.Close()

	plan, materialized, err := materializeProjectionBaseIfNeeded(context.Background(), filepath.Join(t.TempDir(), "runtime.db"), "computer-new", server.URL, testCapability, nil)
	if err != nil {
		t.Fatalf("bootstrap refused: %v", err)
	}
	if plan.Action != recoveryplan.RecoveryGenesis {
		t.Fatalf("empty store + no chain must plan genesis, got %s", plan.Action)
	}
	if materialized {
		t.Fatalf("bootstrap must not claim a snapshot was installed")
	}
}

func TestMaterializeReplaysAShortChainFromGenesisWhenNoSnapshotExists(t *testing.T) {
	platform := newBootServer(&computerevent.Head{ComputerID: "computer-short", Sequence: 42, CanonicalEventHead: strings.Repeat("a", 64)})
	server := httptest.NewServer(platform)
	defer server.Close()
	t.Setenv("CHOIR_PRIVACY_KEY_FILE", writePrivacyKey(t, "computer-short"))

	storeDir := t.TempDir()
	plan, materialized, err := materializeProjectionBaseIfNeeded(context.Background(), filepath.Join(storeDir, "runtime.db"), "computer-short", server.URL, testCapability, nil)
	if err != nil {
		t.Fatalf("a short chain with no snapshot must replay from genesis: %v", err)
	}
	if plan.Action != recoveryplan.RecoveryResume || plan.TailEvents != 42 || materialized {
		t.Fatalf("plan = %+v materialized=%t", plan, materialized)
	}
	if entries, _ := os.ReadDir(storeDir); len(entries) != 0 {
		t.Fatalf("the plan mutated the store dir: %v", entries)
	}
}

func TestMaterializeRefusesALongChainWithNoSnapshot(t *testing.T) {
	platform := newBootServer(&computerevent.Head{ComputerID: "computer-long", Sequence: 148333, CanonicalEventHead: strings.Repeat("a", 64)})
	server := httptest.NewServer(platform)
	defer server.Close()
	t.Setenv("CHOIR_PRIVACY_KEY_FILE", writePrivacyKey(t, "computer-long"))

	storeDir := t.TempDir()
	_, _, err := materializeProjectionBaseIfNeeded(context.Background(), filepath.Join(storeDir, "runtime.db"), "computer-long", server.URL, testCapability, nil)
	if !errors.Is(err, recoveryplan.ErrBaseRefused) {
		t.Fatalf("a long chain with no snapshot must refuse, got %v", err)
	}
	var refusal *ProjectionBaseRefusal
	if !errors.As(err, &refusal) || refusal.Kind != BootRefusalKindBaseMissing || refusal.TargetSequence != 148333 {
		t.Fatalf("refusal = %+v", refusal)
	}
	if entries, _ := os.ReadDir(storeDir); len(entries) != 0 {
		t.Fatalf("a refused boot mutated the store dir: %v", entries)
	}
}

type snapshotTape struct {
	platform *bootServer
	mu       sync.Mutex
}

func (s *snapshotTape) HoldAppends(fn func() error) error { return fn() }

func (s *snapshotTape) AppendNewPrivatePayload(ctx context.Context, event computerevent.Event, _ computerevent.TransitionInput, plaintext []byte, mediaType string, cipher *computerevent.PrivateArtifactCipher) (computerevent.Receipt, string, error) {
	envelope, _, err := cipher.Encrypt(ctx, event.ComputerID, event.EventID, mediaType, "private", plaintext)
	if err != nil {
		return computerevent.Receipt{}, "", err
	}
	digest := computerevent.DigestBytes(envelope)
	s.platform.mu.Lock()
	defer s.platform.mu.Unlock()
	event.Sequence = 8
	event.PayloadCommitment = digest
	s.platform.events = append(s.platform.events, computerevent.DurableEvent{Request: computerevent.CASRequest{Event: event}})
	s.platform.payloads[digest] = envelope
	return computerevent.Receipt{}, digest, nil
}

// writePrivacyKey installs a guest privacy key file for computerID and
// returns its path. The same key seals the snapshot in the restore test.
func writePrivacyKey(t *testing.T, computerID string) string {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "keys", "privacy.key")
	if err := computerevent.InstallGuestPrivacyKey(path, computerID, key); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestMaterializeRestoresTheNewestSnapshotWhenTheDiskIsMissing is the boot
// path of snapshot v1: a computer whose disk is gone gets its state file back
// from the tape's snapshot_recorded event, and the plan replays only the tail.
func TestMaterializeRestoresTheNewestSnapshotWhenTheDiskIsMissing(t *testing.T) {
	ctx := context.Background()
	const computerID = "computer-restore"
	keyPath := writePrivacyKey(t, computerID)
	t.Setenv("CHOIR_PRIVACY_KEY_FILE", keyPath)
	cipher, err := computerevent.LoadGuestPrivateArtifactCipher(keyPath, computerID, false)
	if err != nil {
		t.Fatal(err)
	}

	// The computer's former store, snapshotted at sequence 7.
	source := dbxtest.SQLite(t)
	if _, err := source.Write.Exec(`CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT); INSERT INTO notes VALUES (1, 'kept'), (2, 'also kept')`); err != nil {
		t.Fatal(err)
	}
	platform := newBootServer(&computerevent.Head{ComputerID: computerID, Sequence: 12, CanonicalEventHead: strings.Repeat("c", 64)})
	server := httptest.NewServer(platform)
	defer server.Close()
	witnessOf := func(ctx context.Context, db *sql.DB) (selfdevprotocol.VMLocalContentWitness, error) {
		digests, err := computerversion.SQLiteStateExtractor{DB: db, Database: "texture"}.Digests(ctx)
		if err != nil {
			return selfdevprotocol.VMLocalContentWitness{}, err
		}
		return selfdevprotocol.WitnessFromDigests(digests)
	}
	ref, err := snapshot.TakeSnapshot(ctx, snapshot.Config{
		ComputerID: computerID, DB: source.Read, TempDir: t.TempDir(),
		Head: func(context.Context, *sql.DB) (*computerevent.Head, error) {
			return &computerevent.Head{ComputerID: computerID, Sequence: 7, CanonicalEventHead: strings.Repeat("b", 64)}, nil
		},
		Witness:  witnessOf,
		Cipher:   cipher,
		Chunks:   snapshot.HTTPChunks{BaseURL: server.URL, ComputerID: computerID, Capability: testCapability},
		Appender: &snapshotTape{platform: platform},
	})
	if err != nil {
		t.Fatal(err)
	}

	storePath := filepath.Join(t.TempDir(), "runtime.db")
	plan, materialized, err := materializeProjectionBaseIfNeeded(ctx, storePath, computerID, server.URL, testCapability, nil)
	if err != nil {
		t.Fatalf("restore refused: %v", err)
	}
	if !materialized || plan.Action != recoveryplan.RecoveryInstall || plan.WatermarkSequence != 7 || plan.TailEvents != 5 {
		t.Fatalf("plan = %+v materialized=%t", plan, materialized)
	}
	restored, err := dbx.OpenSQLite(choirstore.StateDBPath(storePath), dbx.SQLiteOptions{ReadConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	got, err := witnessOf(ctx, restored.Read)
	if err != nil {
		t.Fatal(err)
	}
	if got.ContentRoot != ref.Record.WitnessRoot {
		t.Fatalf("restored witness root %s, the snapshot recorded %s", got.ContentRoot, ref.Record.WitnessRoot)
	}
}
