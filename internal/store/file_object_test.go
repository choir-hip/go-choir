package store

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// The file object, slice 1 (docs/design/file-object-2026-10-24.md). Ways the
// store half can fail, written before the code:
//  1. A rewrite of a path mints a new id instead of keeping the file's.
//  2. A move mints a new id, or leaves the old path resolving.
//  3. A delete removes the object, so a reference resolves to nothing, or
//     leaves the path resolving.
//  4. A new file at a deleted file's path reuses the deleted id.
//  5. A move onto a live path leaves two files claiming one path.
//  6. The list shows deleted files or another computer's files.
//  7. The kinds are unregistered, or registered under a writer the write
//     gate refuses in enforce mode.
//  8. The switch is unknown to the settings API, or defaults on.
//  9. On a bound tape a change writes more than one projection batch, or a
//     tombstoned path view does not project (a replay would resurrect it).

const (
	fileObjOwner    = "owner-file-object"
	fileObjComputer = "computer-file-object"
)

func fileRecord(path, digest string, size int64) FileRecord {
	return FileRecord{Path: path, Digest: digest, Size: size, Mode: 0o644, ModTime: time.Unix(1700000000, 0).UTC(), MediaType: "text/plain"}
}

func enableFileObjects(t *testing.T, s *Store, computerID string) {
	t.Helper()
	if err := s.PutComputerSetting(context.Background(), ComputerSetting{OwnerID: fileObjOwner, ComputerID: computerID, Key: SettingFilesObject, Value: true}); err != nil {
		t.Fatal(err)
	}
}

func applyOne(t *testing.T, s *Store, change FileChange) FileApplyResult {
	t.Helper()
	res, err := s.ApplyFileChanges(context.Background(), fileObjOwner, fileObjComputer, []FileChange{change}, time.Now().UTC())
	if err != nil {
		t.Fatalf("apply %+v: %v", change, err)
	}
	return res
}

func TestFileObjectKeepsItsIDThroughWriteMoveAndDelete(t *testing.T) {
	restore := objectgraph.SetWriterGateMode(objectgraph.WriterGateEnforce)
	defer restore()
	s := openTestStore(t)
	enableFileObjects(t, s, fileObjComputer)
	ctx := context.Background()

	res := applyOne(t, s, FileChange{Op: FileOpWrite, Record: fileRecord("docs/a.txt", "sha256:aa", 3), RecordedBy: "files_api"})
	if res.Created != 1 || len(res.FileIDs) != 1 {
		t.Fatalf("first write = %+v, want one created", res)
	}
	id := res.FileIDs[0]
	got, err := s.GetFileObjectByPath(ctx, fileObjOwner, fileObjComputer, "/docs//a.txt")
	if err != nil || got.FileID != id || got.Digest != "sha256:aa" || got.State != FileStatePresent || got.Name != "a.txt" {
		t.Fatalf("by path = %+v, %v; want id %s present", got, err, id)
	}

	res = applyOne(t, s, FileChange{Op: FileOpWrite, Record: fileRecord("docs/a.txt", "sha256:bb", 4), RecordedBy: "files_api"})
	if res.Updated != 1 || res.FileIDs[0] != id {
		t.Fatalf("rewrite = %+v, want the same id %s updated", res, id)
	}
	got, _ = s.GetFileObject(ctx, fileObjOwner, fileObjComputer, id)
	if got.Digest != "sha256:bb" || got.Size != 4 || got.CreatedAt.IsZero() {
		t.Fatalf("after rewrite = %+v", got)
	}

	res = applyOne(t, s, FileChange{Op: FileOpMove, FromPath: "docs/a.txt", Record: fileRecord("archive/b.txt", "sha256:bb", 4), RecordedBy: "files_api"})
	if res.Moved != 1 || res.FileIDs[0] != id {
		t.Fatalf("move = %+v, want the same id %s moved", res, id)
	}
	if _, err := s.GetFileObjectByPath(ctx, fileObjOwner, fileObjComputer, "docs/a.txt"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old path after move = %v, want ErrNotFound", err)
	}
	got, err = s.GetFileObjectByPath(ctx, fileObjOwner, fileObjComputer, "archive/b.txt")
	if err != nil || got.FileID != id || got.Path != "archive/b.txt" || got.Name != "b.txt" {
		t.Fatalf("new path after move = %+v, %v", got, err)
	}

	res = applyOne(t, s, FileChange{Op: FileOpDelete, Record: FileRecord{Path: "archive/b.txt"}, RecordedBy: "files_api"})
	if res.Deleted != 1 {
		t.Fatalf("delete = %+v", res)
	}
	if _, err := s.GetFileObjectByPath(ctx, fileObjOwner, fileObjComputer, "archive/b.txt"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("path after delete = %v, want ErrNotFound", err)
	}
	got, err = s.GetFileObject(ctx, fileObjOwner, fileObjComputer, id)
	if err != nil || got.State != FileStateDeleted || got.DeletedAt.IsZero() {
		t.Fatalf("deleted object = %+v, %v; want state deleted, kept", got, err)
	}

	res = applyOne(t, s, FileChange{Op: FileOpWrite, Record: fileRecord("archive/b.txt", "sha256:cc", 2), RecordedBy: "files_api"})
	if res.Created != 1 || res.FileIDs[0] == id {
		t.Fatalf("write at a deleted path = %+v, want a new id (not %s)", res, id)
	}
	// A delete of a path with no file writes nothing.
	if res := applyOne(t, s, FileChange{Op: FileOpDelete, Record: FileRecord{Path: "nothing/here"}}); res.Deleted != 0 || res.Unchanged != 1 {
		t.Fatalf("delete of an unknown path = %+v", res)
	}
}

func TestFileObjectMoveOntoALivePathDeletesTheTarget(t *testing.T) {
	s := openTestStore(t)
	enableFileObjects(t, s, fileObjComputer)
	ctx := context.Background()
	a := applyOne(t, s, FileChange{Op: FileOpWrite, Record: fileRecord("a", "sha256:01", 1)}).FileIDs[0]
	b := applyOne(t, s, FileChange{Op: FileOpWrite, Record: fileRecord("b", "sha256:02", 1)}).FileIDs[0]
	applyOne(t, s, FileChange{Op: FileOpMove, FromPath: "a", Record: fileRecord("b", "sha256:01", 1)})
	got, err := s.GetFileObjectByPath(ctx, fileObjOwner, fileObjComputer, "b")
	if err != nil || got.FileID != a {
		t.Fatalf("b after move = %+v, %v; want %s", got, err, a)
	}
	old, _ := s.GetFileObject(ctx, fileObjOwner, fileObjComputer, b)
	if old.State != FileStateDeleted {
		t.Fatalf("overwritten target = %+v, want deleted", old)
	}
	// A move whose source has no file records a write at the destination.
	res := applyOne(t, s, FileChange{Op: FileOpMove, FromPath: "ghost", Record: fileRecord("c", "sha256:03", 1)})
	if res.Created != 1 {
		t.Fatalf("move of an unknown source = %+v, want a created file", res)
	}
}

func TestFileObjectListShowsPresentFilesOfThisComputer(t *testing.T) {
	s := openTestStore(t)
	enableFileObjects(t, s, fileObjComputer)
	ctx := context.Background()
	applyOne(t, s, FileChange{Op: FileOpWrite, Record: fileRecord("keep.txt", "sha256:01", 1)})
	applyOne(t, s, FileChange{Op: FileOpWrite, Record: fileRecord("gone.txt", "sha256:02", 1)})
	applyOne(t, s, FileChange{Op: FileOpDelete, Record: FileRecord{Path: "gone.txt"}})
	enableFileObjects(t, s, "another-computer")
	if _, err := s.ApplyFileChanges(ctx, fileObjOwner, "another-computer", []FileChange{{Op: FileOpWrite, Record: fileRecord("other.txt", "sha256:03", 1)}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListFileObjects(ctx, fileObjOwner, fileObjComputer)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Path != "keep.txt" {
		t.Fatalf("list = %+v, want only keep.txt", list)
	}
}

func TestFileObjectBatchAppliesManyChangesInOneWrite(t *testing.T) {
	s := openTestStore(t)
	enableFileObjects(t, s, fileObjComputer)
	ctx := context.Background()
	changes := []FileChange{
		{Op: FileOpWrite, Record: fileRecord("x/1", "sha256:01", 1)},
		{Op: FileOpWrite, Record: fileRecord("x/2", "sha256:02", 1)},
		{Op: FileOpWrite, Record: fileRecord("x/1", "sha256:03", 2)},
	}
	res, err := s.ApplyFileChanges(ctx, fileObjOwner, fileObjComputer, changes, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 2 || res.Updated != 1 || res.FileIDs[0] != res.FileIDs[2] {
		t.Fatalf("batch = %+v; the rewrite in the same batch must keep the id", res)
	}
	got, _ := s.GetFileObjectByPath(ctx, fileObjOwner, fileObjComputer, "x/1")
	if got.Digest != "sha256:03" {
		t.Fatalf("x/1 = %+v, want the last write", got)
	}
}

func TestFileObjectRefusesEscapingPaths(t *testing.T) {
	s := openTestStore(t)
	enableFileObjects(t, s, fileObjComputer)
	for _, p := range []string{"", "/", "../etc/passwd", "a/../../b"} {
		if _, err := s.ApplyFileChanges(context.Background(), fileObjOwner, fileObjComputer, []FileChange{{Op: FileOpWrite, Record: fileRecord(p, "sha256:01", 1)}}, time.Now()); err == nil {
			t.Errorf("path %q accepted", p)
		}
	}
}

func TestFileObjectKindsAndSwitchAreRegistered(t *testing.T) {
	reg := objectgraph.DefaultRegistry()
	for _, kind := range []objectgraph.ObjectKind{ogKindFile, ogKindFilePath} {
		k, err := reg.LookupKind(kind)
		if err != nil || k.Writer != objectgraph.WriterRuntime || k.IdentityMode != objectgraph.IdentityExternalKey || k.Versioned {
			t.Fatalf("%s = %+v, %v; want external_key, unversioned, system:runtime", kind, k, err)
		}
	}
	def, known := KnownComputerSettings[SettingFilesObject]
	if !known || def {
		t.Fatalf("%s known=%v default=%v; want known and off", SettingFilesObject, known, def)
	}
}

// On a bound tape every ApplyFileChanges call is one projection batch, and
// the projected rows (the moved file, the new view, the tombstoned old view)
// are what the reads see: the path a replay takes.
func TestFileObjectChangesRideTheTapeAsOneBatchEach(t *testing.T) {
	ctx := context.Background()
	productStore := openProjectStore(t)
	computerID := "computer-file-object-tape"
	prepareGenesis(t, productStore, computerID, "genesis-file-object")
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "test"}, PrivateKey: privateKey}
	appender, err := computerevent.NewComputerEventAppender(computerID, liveTapePinner{signer: signer}, productStore, liveTapeCAS{signer: signer, projection: productStore}, liveTapeVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	if err := productStore.BindProjectionTape(computerID, appender); err != nil {
		t.Fatal(err)
	}
	countBatches := func() int {
		var n int
		if err := productStore.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM computer_event_index WHERE computer_id=? AND event_kind=?`, computerID, string(computerevent.EventProjectionBatchRecorded)).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	enableFileObjects(t, productStore, computerID)
	before := countBatches()
	apply := func(change FileChange) FileApplyResult {
		t.Helper()
		res, err := productStore.ApplyFileChanges(ctx, fileObjOwner, computerID, []FileChange{change}, time.Now())
		if err != nil {
			t.Fatalf("apply %+v: %v", change, err)
		}
		return res
	}
	id := apply(FileChange{Op: FileOpWrite, Record: fileRecord("a.txt", "sha256:01", 1)}).FileIDs[0]
	apply(FileChange{Op: FileOpMove, FromPath: "a.txt", Record: fileRecord("b.txt", "sha256:01", 1)})
	if got := countBatches() - before; got != 2 {
		t.Fatalf("projection batches = %d, want 2 (one per change)", got)
	}
	if _, err := productStore.GetFileObjectByPath(ctx, fileObjOwner, computerID, "a.txt"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old view after a taped move = %v, want ErrNotFound", err)
	}
	got, err := productStore.GetFileObjectByPath(ctx, fileObjOwner, computerID, "b.txt")
	if err != nil || got.FileID != id {
		t.Fatalf("new view after a taped move = %+v, %v; want %s", got, err, id)
	}
	apply(FileChange{Op: FileOpDelete, Record: FileRecord{Path: "b.txt"}})
	if f, _ := productStore.GetFileObject(ctx, fileObjOwner, computerID, id); f.State != FileStateDeleted {
		t.Fatalf("taped delete = %+v", f)
	}
}

// Review round 1 (bunch 208). Ways the fixes can fail, written before them:
//  10. NormalizeFilePath rewrites a backslash the filesystem keeps literal, so
//      two different files share one object path.
//  11. ApplyFileChanges writes while the files.object setting is off.
//  12. GetFileObject answers for another computer's file.
//  13. A replay from zero does not reproduce the live rows (a tombstoned view
//      resurrects, a deleted file loses its state).

func TestFileObjectPathsKeepABackslashLiteral(t *testing.T) {
	lit, err := NormalizeFilePath(`a\b.txt`)
	if err != nil || lit != `a\b.txt` {
		t.Fatalf(`NormalizeFilePath(a\b.txt) = %q, %v; want it unchanged`, lit, err)
	}
	dir, err := NormalizeFilePath("a/b.txt")
	if err != nil || dir == lit {
		t.Fatalf("a/b.txt = %q, %v; want a path distinct from the backslash name", dir, err)
	}
	s := openTestStore(t)
	enableFileObjects(t, s, fileObjComputer)
	a := applyOne(t, s, FileChange{Op: FileOpWrite, Record: fileRecord(`a\b.txt`, "hmac-sha256:01", 1)}).FileIDs[0]
	b := applyOne(t, s, FileChange{Op: FileOpWrite, Record: fileRecord("a/b.txt", "hmac-sha256:02", 1)}).FileIDs[0]
	if a == b {
		t.Fatal("two different paths share one file")
	}
}

func TestFileObjectApplyRefusesWhileTheSwitchIsOff(t *testing.T) {
	s := openTestStore(t)
	_, err := s.ApplyFileChanges(context.Background(), fileObjOwner, fileObjComputer, []FileChange{{Op: FileOpWrite, Record: fileRecord("a.txt", "hmac-sha256:01", 1)}}, time.Now())
	if !errors.Is(err, ErrFileObjectsOff) {
		t.Fatalf("apply with the switch unset = %v, want ErrFileObjectsOff", err)
	}
	if err := s.PutComputerSetting(context.Background(), ComputerSetting{OwnerID: fileObjOwner, ComputerID: fileObjComputer, Key: SettingFilesObject, Value: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyFileChanges(context.Background(), fileObjOwner, fileObjComputer, []FileChange{{Op: FileOpWrite, Record: fileRecord("a.txt", "hmac-sha256:01", 1)}}, time.Now()); !errors.Is(err, ErrFileObjectsOff) {
		t.Fatalf("apply with the switch false = %v, want ErrFileObjectsOff", err)
	}
	if list, _ := s.ListFileObjects(context.Background(), fileObjOwner, fileObjComputer); len(list) != 0 {
		t.Fatalf("objects written while off: %+v", list)
	}
}

func TestFileObjectGetByIDIsComputerScoped(t *testing.T) {
	s := openTestStore(t)
	enableFileObjects(t, s, "computer-two")
	res, err := s.ApplyFileChanges(context.Background(), fileObjOwner, "computer-two", []FileChange{{Op: FileOpWrite, Record: fileRecord("a.txt", "hmac-sha256:01", 1)}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	id := res.FileIDs[0]
	if _, err := s.GetFileObject(context.Background(), fileObjOwner, fileObjComputer, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another computer's file = %v, want ErrNotFound", err)
	}
	if got, err := s.GetFileObject(context.Background(), fileObjOwner, "computer-two", id); err != nil || got.FileID != id {
		t.Fatalf("own file = %+v, %v", got, err)
	}
}

// tapeRecorder is a CAS that keeps its events (an EventSource) and a pinner
// that keeps the payloads it pins, so a second store can replay the tape.
type tapeRecorder struct {
	liveTapeCAS
	mu     sync.Mutex
	events []computerevent.DurableEvent
}

func (c *tapeRecorder) CompareAndSwap(ctx context.Context, request computerevent.CASRequest) (computerevent.Receipt, error) {
	receipt, err := c.liveTapeCAS.CompareAndSwap(ctx, request)
	if err != nil {
		return receipt, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, computerevent.DurableEvent{Request: request, Receipt: receipt})
	return receipt, nil
}

func (c *tapeRecorder) Events(_ context.Context, _ string, after uint64) ([]computerevent.DurableEvent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []computerevent.DurableEvent
	for _, e := range c.events {
		if e.Request.Event.Sequence > after {
			out = append(out, e)
		}
	}
	return out, nil
}

type payloadKeepingPinner struct {
	liveTapePinner
	reader *computerevent.MemoryArtifactReader
}

func (p payloadKeepingPinner) PinNonPrivatePayload(ctx context.Context, computerID string, payload []byte, mediaType, privacyClass, intent string) (computerevent.PinResult, error) {
	p.reader.Put(payload)
	return p.liveTapePinner.PinNonPrivatePayload(ctx, computerID, payload, mediaType, privacyClass, intent)
}

func TestFileObjectsReplayFromZeroReproducesTheLiveRows(t *testing.T) {
	ctx := context.Background()
	live := openProjectStore(t)
	computerID := "computer-file-object-replay"
	genesis := prepareGenesisRequest(t, live, computerID, "genesis-file-object-replay")
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "test"}, PrivateKey: privateKey}
	cas := &tapeRecorder{liveTapeCAS: liveTapeCAS{signer: signer, projection: live}}
	cas.events = append(cas.events, genesis)
	reader := computerevent.NewMemoryArtifactReader()
	appender, err := computerevent.NewComputerEventAppender(computerID, payloadKeepingPinner{liveTapePinner{signer: signer}, reader}, live, cas, liveTapeVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	appender.SetPayloadResolver(reader, nil)
	if err := live.BindProjectionTape(computerID, appender); err != nil {
		t.Fatal(err)
	}
	enableFileObjects(t, live, computerID)
	apply := func(changes ...FileChange) FileApplyResult {
		t.Helper()
		res, err := live.ApplyFileChanges(ctx, fileObjOwner, computerID, changes, time.Now())
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		return res
	}
	a := apply(FileChange{Op: FileOpWrite, Record: fileRecord("a.txt", "hmac-sha256:01", 1)}).FileIDs[0]
	b := apply(FileChange{Op: FileOpWrite, Record: fileRecord("b.txt", "hmac-sha256:02", 2)}).FileIDs[0]
	apply(FileChange{Op: FileOpMove, FromPath: "a.txt", Record: fileRecord("c.txt", "hmac-sha256:01", 1)})
	apply(FileChange{Op: FileOpDelete, Record: FileRecord{Path: "b.txt"}})
	apply(FileChange{Op: FileOpWrite, Record: fileRecord("b.txt", "hmac-sha256:03", 3)})

	replayed := openProjectStore(t)
	if err := appender.ReconstructInto(ctx, replayed); err != nil {
		t.Fatalf("replay from zero: %v", err)
	}
	liveList, err := live.ListFileObjects(ctx, fileObjOwner, computerID)
	if err != nil {
		t.Fatal(err)
	}
	gotList, err := replayed.ListFileObjects(ctx, fileObjOwner, computerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(liveList) != 2 || len(gotList) != len(liveList) {
		t.Fatalf("live %+v vs replayed %+v; want the same two present files", liveList, gotList)
	}
	for i := range liveList {
		if liveList[i].FileID != gotList[i].FileID || liveList[i].Path != gotList[i].Path || liveList[i].Digest != gotList[i].Digest {
			t.Errorf("row %d: live %+v, replayed %+v", i, liveList[i], gotList[i])
		}
	}
	if _, err := replayed.GetFileObjectByPath(ctx, fileObjOwner, computerID, "a.txt"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("moved-from view resurrected on replay: %v", err)
	}
	if got, err := replayed.GetFileObjectByPath(ctx, fileObjOwner, computerID, "c.txt"); err != nil || got.FileID != a {
		t.Fatalf("moved file on replay = %+v, %v; want id %s", got, err, a)
	}
	if got, err := replayed.GetFileObject(ctx, fileObjOwner, computerID, b); err != nil || got.State != FileStateDeleted {
		t.Fatalf("deleted file on replay = %+v, %v; want state deleted", got, err)
	}
}

// prepareGenesisRequest is prepareGenesis, returning the durable event so a
// replay source can serve the chain from sequence 1.
func prepareGenesisRequest(t *testing.T, productStore *Store, computerID, idempotency string) computerevent.DurableEvent {
	t.Helper()
	event, digest := prepareGenesis(t, productStore, computerID, idempotency)
	input := computerevent.TransitionInput{TargetStateCommitment: storeTestDigest('b')}
	pinIntent, err := computerevent.ComputePinIntentCommitment(event, input)
	if err != nil {
		t.Fatal(err)
	}
	next, err := computerevent.Reduce(nil, event, input)
	if err != nil {
		t.Fatal(err)
	}
	return computerevent.DurableEvent{
		Request: computerevent.CASRequest{
			Event: event, EventDigest: digest, EventArtifactDigest: digest,
			EventPinReceiptDigest: storeTestDigest('c'), Input: input, Next: next, PinIntentCommitment: pinIntent,
		},
		Receipt: signedReceipt(t, computerID, digest, 1),
	}
}
