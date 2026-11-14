package store

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// A lifecycle command may name the revision its own batch wrote by id instead
// of embedding a copy. These tests pin that the reader returns the same bytes
// from either form, under the enforced reader gate, and fails loudly when a
// named revision is missing.

const storedResultMarker = "STORED-RESULT-REVISION-BODY-MARKER"

func storedResultCommit(start types.StartLifecycleRequest, commandID, revisionID string, version int64, headID string) types.CommitLifecycleArtifactHeadRequest {
	commit := types.CommitLifecycleArtifactHeadRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: commandID, TrajectoryID: start.TrajectoryID,
		ExpectedLifecycleVersion: version, ExpectedHeadRevisionID: headID,
		Revision: types.Revision{
			RevisionID: revisionID, AuthorKind: types.AuthorUser, AuthorLabel: "owner",
			Content: storedResultMarker + " " + revisionID + " " + strings.Repeat("body text ", 2000),
		},
	}
	commit.CommandDigest, _ = ComputeCommitLifecycleArtifactHeadDigest(commit)
	return commit
}

func storedResultCommandObject(t *testing.T, s *Store, start types.StartLifecycleRequest, commandID string) objectgraph.Object {
	t.Helper()
	obj, err := s.lifecycleGetObject(context.Background(), ogKindLifecycleCmd, start.OwnerID, start.ComputerID, commandID)
	if err != nil {
		t.Fatalf("get command %s: %v", commandID, err)
	}
	return obj
}

// storedResultKeys returns the top-level keys of a command's stored_result.
func storedResultKeys(t *testing.T, obj objectgraph.Object) map[string]json.RawMessage {
	t.Helper()
	var receipt map[string]json.RawMessage
	if err := json.Unmarshal(obj.Body, &receipt); err != nil {
		t.Fatalf("decode receipt: %v", err)
	}
	var stored map[string]json.RawMessage
	if err := json.Unmarshal(receipt["stored_result"], &stored); err != nil {
		t.Fatalf("decode stored_result: %v", err)
	}
	return stored
}

// storedResultRewrite rewrites one command object into the slim form (the
// revision named by id) or the old full form (the revision embedded), exactly
// what a tape written by either release carries. Step 1 writes only the full
// form, so every slim command in these tests is built here.
func storedResultRewrite(t *testing.T, s *Store, start types.StartLifecycleRequest, commandID string, slim bool) {
	t.Helper()
	ctx := context.Background()
	obj := storedResultCommandObject(t, s, start, commandID)
	var receipt map[string]json.RawMessage
	if err := json.Unmarshal(obj.Body, &receipt); err != nil {
		t.Fatal(err)
	}
	var stored map[string]json.RawMessage
	if err := json.Unmarshal(receipt["stored_result"], &stored); err != nil {
		t.Fatal(err)
	}
	var revisionID string
	if raw, ok := stored["revision_id"]; ok {
		_ = json.Unmarshal(raw, &revisionID)
	} else if raw, ok := stored["revision"]; ok {
		var rev types.Revision
		if err := json.Unmarshal(raw, &rev); err != nil {
			t.Fatal(err)
		}
		revisionID = rev.RevisionID
	} else {
		t.Fatalf("command %s carries no revision", commandID)
	}
	revObj, err := s.lifecycleGetObject(ctx, ogKindTexRev, start.OwnerID, start.ComputerID, revisionID)
	if err != nil {
		t.Fatalf("rewrite: get revision: %v", err)
	}
	if slim {
		delete(stored, "revision")
		stored["revision_id"], _ = json.Marshal(revisionID)
	} else {
		delete(stored, "revision_id")
		stored["revision"] = json.RawMessage(revObj.Body)
	}
	receipt["stored_result"], _ = json.Marshal(stored)
	body, _ := json.Marshal(receipt)
	rewritten := obj
	rewritten.Body = body
	rewritten.ContentHash = objectgraph.ContentHash(obj.ObjectKind, body, obj.Metadata)
	if err := s.ogStore.PutObject(ctx, rewritten); err != nil {
		t.Fatalf("rewrite: put: %v", err)
	}
}

func storedResultSlim(t *testing.T, s *Store, start types.StartLifecycleRequest, commandID string) {
	t.Helper()
	storedResultRewrite(t, s, start, commandID, true)
}

func storedResultLegacy(t *testing.T, s *Store, start types.StartLifecycleRequest, commandID string) {
	t.Helper()
	storedResultRewrite(t, s, start, commandID, false)
}

func storedResultReplayBytes(t *testing.T, s *Store, start types.StartLifecycleRequest, commit *types.CommitLifecycleArtifactHeadRequest) []byte {
	t.Helper()
	ctx := context.Background()
	var result types.LifecycleResult
	var err error
	if commit == nil {
		result, err = s.StartLifecycle(ctx, start)
	} else {
		result, err = s.CommitLifecycleArtifactHead(ctx, *commit)
	}
	if err != nil || !result.Replay {
		t.Fatalf("replay = %+v, %v", result, err)
	}
	b, _ := json.Marshal(result)
	return b
}

func storedResultCopyDir(t *testing.T, from, to string) {
	t.Helper()
	if err := os.MkdirAll(to, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(from)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			storedResultCopyDir(t, filepath.Join(from, e.Name()), filepath.Join(to, e.Name()))
			continue
		}
		in, err := os.Open(filepath.Join(from, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out, err := os.Create(filepath.Join(to, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(out, in); err != nil {
			t.Fatal(err)
		}
		in.Close()
		out.Close()
	}
}

func storedResultTwoCommits(t *testing.T, s *Store) (types.StartLifecycleRequest, types.CommitLifecycleArtifactHeadRequest, types.CommitLifecycleArtifactHeadRequest) {
	t.Helper()
	ctx := context.Background()
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatal(err)
	}
	first := storedResultCommit(start, "stored-result-commit-1", "stored-result-rev-1", 1, start.InitialRevision.RevisionID)
	second := storedResultCommit(start, "stored-result-commit-2", "stored-result-rev-2", 2, first.Revision.RevisionID)
	for _, c := range []types.CommitLifecycleArtifactHeadRequest{first, second} {
		if _, err := s.CommitLifecycleArtifactHead(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	return start, first, second
}

func TestLifecycleReplayOfSlimAndEmbeddedCommandsIsByteIdentical(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start, first, second := storedResultTwoCommits(t, s)
	ids := []string{start.CommandID, first.CommandID, second.CommandID}
	replays := func() [][]byte {
		return [][]byte{storedResultReplayBytes(t, s, start, nil), storedResultReplayBytes(t, s, start, &first), storedResultReplayBytes(t, s, start, &second)}
	}
	for _, id := range ids {
		storedResultLegacy(t, s, start, id) // the whole-write reference: every command embeds its revision
	}
	embedded := replays()
	cmdHashes := map[string]string{}
	for _, id := range ids {
		cmdHashes[id] = storedResultCommandObject(t, s, start, id).ContentHash
	}
	revHashes := map[string]string{}
	for _, id := range []string{start.InitialRevision.RevisionID, first.Revision.RevisionID, second.Revision.RevisionID} {
		obj, err := s.lifecycleGetObject(ctx, ogKindTexRev, start.OwnerID, start.ComputerID, id)
		if err != nil {
			t.Fatal(err)
		}
		revHashes[id] = obj.ContentHash
	}
	// Mixed tape: the first command slim, the others embedded.
	storedResultSlim(t, s, start, first.CommandID)
	if cmdHashes[first.CommandID] == storedResultCommandObject(t, s, start, first.CommandID).ContentHash {
		t.Fatal("slim rewrite did not change the command object")
	}
	for i, b := range replays() {
		if !bytes.Equal(embedded[i], b) {
			t.Fatalf("mixed-tape replay %d differs:\nembedded %s\nmixed    %s", i, embedded[i], b)
		}
	}
	// All slim.
	storedResultSlim(t, s, start, start.CommandID)
	storedResultSlim(t, s, start, second.CommandID)
	for i, b := range replays() {
		if !bytes.Equal(embedded[i], b) {
			t.Fatalf("slim replay %d differs from the embedded reference", i)
		}
	}
	// Replay writes nothing: revision objects keep their content hashes.
	for id, want := range revHashes {
		obj, err := s.lifecycleGetObject(ctx, ogKindTexRev, start.OwnerID, start.ComputerID, id)
		if err != nil || obj.ContentHash != want {
			t.Fatalf("revision %s content hash changed: %v", id, err)
		}
	}
}

func TestLifecycleSlimReplayAfterLaterRevisionReturnsTheOriginalResult(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatal(err)
	}
	first := storedResultCommit(start, "stored-result-commit-1", "stored-result-rev-1", 1, start.InitialRevision.RevisionID)
	if _, err := s.CommitLifecycleArtifactHead(ctx, first); err != nil {
		t.Fatal(err)
	}
	storedResultLegacy(t, s, start, first.CommandID)
	storedResultLegacy(t, s, start, start.CommandID)
	before := storedResultReplayBytes(t, s, start, &first)
	startBefore := storedResultReplayBytes(t, s, start, nil)
	storedResultSlim(t, s, start, first.CommandID)
	storedResultSlim(t, s, start, start.CommandID)
	second := storedResultCommit(start, "stored-result-commit-2", "stored-result-rev-2", 2, first.Revision.RevisionID)
	if _, err := s.CommitLifecycleArtifactHead(ctx, second); err != nil {
		t.Fatal(err)
	}
	if after := storedResultReplayBytes(t, s, start, &first); !bytes.Equal(before, after) {
		t.Fatalf("slim replay changed after a later revision:\nbefore %s\nafter  %s", before, after)
	}
	if after := storedResultReplayBytes(t, s, start, nil); !bytes.Equal(startBefore, after) {
		t.Fatal("slim start replay changed after later revisions")
	}
}

func TestLifecycleRestoreFromSnapshotPlusTailWithSlimCommands(t *testing.T) {
	ctx := context.Background()
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatal(err)
	}
	first := storedResultCommit(start, "stored-result-commit-1", "stored-result-rev-1", 1, start.InitialRevision.RevisionID)
	if _, err := s.CommitLifecycleArtifactHead(ctx, first); err != nil {
		t.Fatal(err)
	}
	storedResultLegacy(t, s, start, start.CommandID)
	storedResultLegacy(t, s, start, first.CommandID)
	startRef, firstRef := storedResultReplayBytes(t, s, start, nil), storedResultReplayBytes(t, s, start, &first)
	// The snapshot holds slim commands (a tape written by the writer release).
	storedResultSlim(t, s, start, start.CommandID)
	storedResultSlim(t, s, start, first.CommandID)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	snapDir := filepath.Join(filepath.Dir(filepath.Dir(path)), t.Name()+"-snapshot")
	t.Cleanup(func() { _ = os.RemoveAll(snapDir) })
	storedResultCopyDir(t, filepath.Dir(path), snapDir)
	restored, err := Open(filepath.Join(snapDir, filepath.Base(path)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = restored.Close() })
	// The tail: a command after the restore, then replay of every command.
	second := storedResultCommit(start, "stored-result-commit-2", "stored-result-rev-2", 2, first.Revision.RevisionID)
	if _, err := restored.CommitLifecycleArtifactHead(ctx, second); err != nil {
		t.Fatal(err)
	}
	if got := storedResultReplayBytes(t, restored, start, nil); !bytes.Equal(got, startRef) {
		t.Fatal("start replay after restore differs")
	}
	if got := storedResultReplayBytes(t, restored, start, &first); !bytes.Equal(got, firstRef) {
		t.Fatal("first replay after restore differs")
	}
	var replay types.LifecycleResult
	if err := json.Unmarshal(storedResultReplayBytes(t, restored, start, &second), &replay); err != nil || replay.Revision == nil || replay.Revision.RevisionID != second.Revision.RevisionID {
		t.Fatalf("tail replay after restore = %+v, %v", replay, err)
	}
}

// slimTextureTurn applies a Texture revision turn, then rewrites its command
// into the slim form, returning the request and the result read while the
// command still embedded its revision.
func slimTextureTurn(t *testing.T) (*Store, types.StartLifecycleRequest, types.ApplyTextureTurnRequest, types.LifecycleResult, types.LifecycleResult) {
	t.Helper()
	s, start, req, applied := applyTextureRevisionTurn(t)
	// The reference is the embedded form, whatever form the writer stored.
	storedResultLegacy(t, s, start, req.CommandID)
	recovered, err := s.GetAppliedTextureTurnByCallerRun(context.Background(), start.OwnerID, start.ComputerID, start.TrajectoryID, applied.Agent.ActiveRunID)
	if err != nil || recovered.Revision == nil {
		t.Fatalf("recover embedded turn: %+v, %v", recovered, err)
	}
	storedResultSlim(t, s, start, req.CommandID)
	return s, start, req, applied, recovered
}

// applyTextureRevisionTurn applies a Texture revision turn through the real
// transaction, in whatever form the writer stores it.
func applyTextureRevisionTurn(t *testing.T) (*Store, types.StartLifecycleRequest, types.ApplyTextureTurnRequest, types.LifecycleResult) {
	t.Helper()
	s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnRevision)
	rev := start.InitialRevision
	rev.RevisionID = "stored-result-turn-revision"
	rev.AuthorKind = types.AuthorAppAgent
	rev.AuthorLabel = "appagent"
	rev.ParentRevisionID = start.InitialRevision.RevisionID
	text := storedResultMarker + " " + strings.Repeat("turn body ", 200)
	bodyDoc, _ := json.Marshal(userAuthoredTextStructuredTextureDoc(rev.DocID, rev.RevisionID, text))
	rev.BodyDoc, rev.Content = bodyDoc, text
	req.Revision = rev
	setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
	applied, err := s.ApplyTextureTurn(ctx, req)
	if err != nil || applied.Revision == nil {
		t.Fatalf("apply turn: %+v, %v", applied, err)
	}
	return s, start, req, applied
}

func TestSlimTextureTurnReadsUnderTheEnforcedReaderGate(t *testing.T) {
	s, start, req, applied, embedded := slimTextureTurn(t)
	defer objectgraph.SetModeForTest(objectgraph.WriterGateEnforce)()
	runID := applied.Agent.ActiveRunID
	// GetAppliedTextureTurnByCallerRun hands back the receipt as stored, so
	// Receipt.StoredResult reflects the stored form (no reader uses it: the
	// readers take Revision and TextureTurn). Compare the result without it.
	embedded.Receipt.StoredResult = nil
	embeddedBytes, _ := json.Marshal(embedded)
	// The command is kernel-owned, so a background context reads it; the
	// revision it names is Texture's, and the rebuild must still succeed.
	for name, ctx := range map[string]context.Context{
		"background":            context.Background(),
		"wrong desk (research)": objectgraph.WithReader(context.Background(), objectgraph.Writer("desk:research")),
		"texture custodian":     objectgraph.WithCustodianReader(context.Background(), ogKindTexRev),
	} {
		got, err := s.GetAppliedTextureTurnByCallerRun(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID, runID)
		if err != nil {
			t.Fatalf("%s: GetAppliedTextureTurnByCallerRun: %v", name, err)
		}
		got.Receipt.StoredResult = nil
		if b, _ := json.Marshal(got); !bytes.Equal(b, embeddedBytes) {
			t.Fatalf("%s: slim read differs from the embedded read", name)
		}
		replay, err := s.ApplyTextureTurn(ctx, req)
		if err != nil || !replay.Replay || replay.Revision == nil || replay.Revision.RevisionID != applied.Revision.RevisionID {
			t.Fatalf("%s: turn replay = %+v, %v", name, replay, err)
		}
	}
	// The gate itself is unchanged: a direct read of the revision on a
	// background context is still refused.
	if _, err := s.GetLifecycleRevision(context.Background(), start.OwnerID, start.ComputerID, applied.Revision.RevisionID); err == nil {
		t.Fatal("a background read of a Texture revision was admitted under enforce")
	}
}

func TestSlimCommandWithMissingRevisionFailsLoudly(t *testing.T) {
	s, start, req, applied, _ := slimTextureTurn(t)
	ctx := context.Background()
	obj := storedResultCommandObject(t, s, start, req.CommandID)
	var receipt map[string]json.RawMessage
	_ = json.Unmarshal(obj.Body, &receipt)
	var stored map[string]json.RawMessage
	_ = json.Unmarshal(receipt["stored_result"], &stored)
	stored["revision_id"], _ = json.Marshal("stored-result-no-such-revision")
	receipt["stored_result"], _ = json.Marshal(stored)
	obj.Body, _ = json.Marshal(receipt)
	obj.ContentHash = objectgraph.ContentHash(obj.ObjectKind, obj.Body, obj.Metadata)
	if err := s.ogStore.PutObject(ctx, obj); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetAppliedTextureTurnByCallerRun(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID, applied.Agent.ActiveRunID)
	if err == nil || got.Revision != nil || !strings.Contains(err.Error(), req.CommandID) {
		t.Fatalf("applied turn with a missing revision = %+v, %v", got, err)
	}
	if _, err := s.ApplyTextureTurn(ctx, req); err == nil {
		t.Fatal("turn replay with a missing revision returned no error")
	}
}
