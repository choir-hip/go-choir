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

// A lifecycle command used to embed a copy of the revision its own batch
// writes. These tests pin that a new command names the revision by id and
// that every old and new form replays to the same bytes.

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

// storedResultLegacy rewrites one command object into the old form: the full
// revision embedded, no revision id, exactly what older tapes carry.
func storedResultLegacy(t *testing.T, s *Store, start types.StartLifecycleRequest, commandID string) {
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
	if _, slim := stored["revision_id"]; !slim {
		if _, full := stored["revision"]; !full {
			return // the command carries no revision
		}
		return // already legacy
	}
	var revisionID string
	if err := json.Unmarshal(stored["revision_id"], &revisionID); err != nil {
		t.Fatal(err)
	}
	revObj, err := s.lifecycleGetObject(ctx, ogKindTexRev, start.OwnerID, start.ComputerID, revisionID)
	if err != nil {
		t.Fatalf("legacy rewrite: get revision: %v", err)
	}
	delete(stored, "revision_id")
	stored["revision"] = json.RawMessage(revObj.Body)
	receipt["stored_result"], _ = json.Marshal(stored)
	body, _ := json.Marshal(receipt)
	legacy := obj
	legacy.Body = body
	legacy.ContentHash = objectgraph.ContentHash(obj.ObjectKind, body, obj.Metadata)
	if err := s.ogStore.PutObject(ctx, legacy); err != nil {
		t.Fatalf("legacy rewrite: put: %v", err)
	}
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

func TestLifecycleCommandNamesItsRevisionInsteadOfEmbeddingIt(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatal(err)
	}
	commit := storedResultCommit(start, "stored-result-commit-1", "stored-result-rev-1", 1, start.InitialRevision.RevisionID)
	if _, err := s.CommitLifecycleArtifactHead(ctx, commit); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ id, revision string }{
		{start.CommandID, start.InitialRevision.RevisionID},
		{commit.CommandID, commit.Revision.RevisionID},
	} {
		obj := storedResultCommandObject(t, s, start, c.id)
		stored := storedResultKeys(t, obj)
		if _, embedded := stored["revision"]; embedded {
			t.Fatalf("%s still embeds its revision", c.id)
		}
		var named string
		if err := json.Unmarshal(stored["revision_id"], &named); err != nil || named != c.revision {
			t.Fatalf("%s names revision %q, want %q (%v)", c.id, named, c.revision, err)
		}
	}
	// The measured size: the command op body no longer carries the revision body.
	cmd := storedResultCommandObject(t, s, start, commit.CommandID)
	rev, err := s.lifecycleGetObject(ctx, ogKindTexRev, start.OwnerID, start.ComputerID, commit.Revision.RevisionID)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(cmd.Body, []byte(storedResultMarker)) {
		t.Fatal("command body still contains the revision body")
	}
	if len(cmd.Body)*4 > len(rev.Body) {
		t.Fatalf("command body %d bytes is not far below the revision body %d bytes", len(cmd.Body), len(rev.Body))
	}
	t.Logf("command body %d bytes, revision body %d bytes", len(cmd.Body), len(rev.Body))
}

func TestLifecycleReplayAfterLaterRevisionReturnsTheOriginalResult(t *testing.T) {
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
	before := storedResultReplayBytes(t, s, start, &first)
	startBefore := storedResultReplayBytes(t, s, start, nil)
	second := storedResultCommit(start, "stored-result-commit-2", "stored-result-rev-2", 2, first.Revision.RevisionID)
	if _, err := s.CommitLifecycleArtifactHead(ctx, second); err != nil {
		t.Fatal(err)
	}
	if after := storedResultReplayBytes(t, s, start, &first); !bytes.Equal(before, after) {
		t.Fatalf("replay of the first command changed after a later revision:\nbefore %s\nafter  %s", before, after)
	}
	if after := storedResultReplayBytes(t, s, start, nil); !bytes.Equal(startBefore, after) {
		t.Fatal("replay of the start command changed after later revisions")
	}
	var replay types.LifecycleResult
	if err := json.Unmarshal(before, &replay); err != nil || replay.Revision == nil || replay.Revision.RevisionID != first.Revision.RevisionID ||
		replay.Document == nil || replay.Document.CurrentRevisionID != first.Revision.RevisionID {
		t.Fatalf("replay leaked a later head: %+v, %v", replay, err)
	}
}

func TestLifecycleReplayOfOldAndNewCommandsIsByteIdentical(t *testing.T) {
	s := openTestStore(t)
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
	slim := [][]byte{storedResultReplayBytes(t, s, start, nil), storedResultReplayBytes(t, s, start, &first), storedResultReplayBytes(t, s, start, &second)}
	hashes := map[string]string{}
	for _, id := range []string{start.CommandID, first.CommandID, second.CommandID} {
		hashes[id] = storedResultCommandObject(t, s, start, id).ContentHash
	}
	revHash := func(id string) string {
		obj, err := s.lifecycleGetObject(ctx, ogKindTexRev, start.OwnerID, start.ComputerID, id)
		if err != nil {
			t.Fatal(err)
		}
		return obj.ContentHash
	}
	revHashes := map[string]string{}
	for _, id := range []string{start.InitialRevision.RevisionID, first.Revision.RevisionID, second.Revision.RevisionID} {
		revHashes[id] = revHash(id)
	}
	// Mixed tape: start and second stay slim, first becomes the old full form.
	storedResultLegacy(t, s, start, first.CommandID)
	if hashes[first.CommandID] == storedResultCommandObject(t, s, start, first.CommandID).ContentHash {
		t.Fatal("legacy rewrite did not change the command object")
	}
	mixed := [][]byte{storedResultReplayBytes(t, s, start, nil), storedResultReplayBytes(t, s, start, &first), storedResultReplayBytes(t, s, start, &second)}
	for i := range slim {
		if !bytes.Equal(slim[i], mixed[i]) {
			t.Fatalf("mixed-tape replay %d differs:\nslim  %s\nmixed %s", i, slim[i], mixed[i])
		}
	}
	// All old form.
	storedResultLegacy(t, s, start, start.CommandID)
	storedResultLegacy(t, s, start, second.CommandID)
	for i, b := range [][]byte{storedResultReplayBytes(t, s, start, nil), storedResultReplayBytes(t, s, start, &first), storedResultReplayBytes(t, s, start, &second)} {
		if !bytes.Equal(slim[i], b) {
			t.Fatalf("legacy replay %d differs from slim replay", i)
		}
	}
	// Replay writes nothing: revision objects keep their content hashes, and the
	// untouched legacy command objects keep theirs.
	for id, want := range revHashes {
		if got := revHash(id); got != want {
			t.Fatalf("revision %s content hash changed: %s != %s", id, got, want)
		}
	}
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

func TestLifecycleRestoreFromSnapshotPlusTailAcrossTheChange(t *testing.T) {
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
	// The snapshot is taken with the old form (a tape written before the change).
	storedResultLegacy(t, s, start, start.CommandID)
	storedResultLegacy(t, s, start, first.CommandID)
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
	// The tail: a new command after the restore is written in the new form.
	second := storedResultCommit(start, "stored-result-commit-2", "stored-result-rev-2", 2, first.Revision.RevisionID)
	if _, err := restored.CommitLifecycleArtifactHead(ctx, second); err != nil {
		t.Fatal(err)
	}
	if _, slim := storedResultKeys(t, storedResultCommandObject(t, restored, start, second.CommandID))["revision_id"]; !slim {
		t.Fatal("tail command after restore is not in the new form")
	}
	for name, c := range map[string]*types.CommitLifecycleArtifactHeadRequest{"start": nil, "first": &first, "second": &second} {
		var replay types.LifecycleResult
		if err := json.Unmarshal(storedResultReplayBytes(t, restored, start, c), &replay); err != nil {
			t.Fatal(err)
		}
		want := start.InitialRevision.RevisionID
		if c != nil {
			want = c.Revision.RevisionID
		}
		if replay.Revision == nil || replay.Revision.RevisionID != want || replay.Document == nil || replay.Document.CurrentRevisionID != want {
			t.Fatalf("%s replay after restore = %+v", name, replay)
		}
	}
}

func TestTextureTurnCommandNamesItsRevisionAndStillRecoversByCallerRun(t *testing.T) {
	s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnRevision)
	rev := start.InitialRevision
	rev.RevisionID = "stored-result-turn-revision"
	rev.AuthorKind = types.AuthorAppAgent
	rev.AuthorLabel = "appagent"
	rev.ParentRevisionID = start.InitialRevision.RevisionID
	text := storedResultMarker + " " + strings.Repeat("turn body ", 2000)
	bodyDoc, _ := json.Marshal(userAuthoredTextStructuredTextureDoc(rev.DocID, rev.RevisionID, text))
	rev.BodyDoc, rev.Content = bodyDoc, text
	req.Revision = rev
	setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
	applied, err := s.ApplyTextureTurn(ctx, req)
	if err != nil || applied.Revision == nil {
		t.Fatalf("apply turn: %+v, %v", applied, err)
	}
	cmd := storedResultCommandObject(t, s, start, req.CommandID)
	if bytes.Contains(cmd.Body, []byte(storedResultMarker)) {
		t.Fatal("texture turn command body still contains the revision body")
	}
	if _, named := storedResultKeys(t, cmd)["revision_id"]; !named {
		t.Fatal("texture turn command does not name its revision")
	}
	recovered, err := s.GetAppliedTextureTurnByCallerRun(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID, applied.Agent.ActiveRunID)
	if err != nil || recovered.Revision == nil || recovered.Revision.RevisionID != rev.RevisionID ||
		!bytes.Equal(recovered.Revision.BodyDoc, applied.Revision.BodyDoc) || recovered.Revision.Content != applied.Revision.Content {
		t.Fatalf("recovered by caller run = %+v, %v", recovered, err)
	}
	replay, err := s.ApplyTextureTurn(ctx, req)
	if err != nil || !replay.Replay || replay.Revision == nil || replay.Revision.RevisionID != rev.RevisionID {
		t.Fatalf("turn replay = %+v, %v", replay, err)
	}
}
