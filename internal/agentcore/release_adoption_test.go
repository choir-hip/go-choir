package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/release"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// P8 failure modes pinned here (docs/archive/reports/p6-p8-release-adopt-2026-10-15.md):
//   A1 a tampered record (a field changed, the id kept) is adopted;
//   A2 a self-consistent record that was never published is adopted;
//   A3 corpusd's stored record or blob was altered after publish;
//   A4 patch bytes that do not hash to the record's sha are staged;
//   A5 a patch that does not apply to the adopter's own tree reaches the desks;
//   A6 a refusal leaves a trace: an adoption object, a run (a prompt), a
//      staged file;
//   A7 the apply check writes to the adopter's source repository;
//   A8 the capsule input is bytes that changed after verification;
//   A9 an assignment on another trajectory receives the patch;
//   A10 the adoption carries a binary (nothing but the record and the patch
//      is staged);
//   A12 a repeated request starts a second request to the desks.

type fakeReleaseSource struct {
	records map[string]json.RawMessage
	blobs   map[string][]byte
}

func (f *fakeReleaseSource) FetchRelease(_ context.Context, computerID, releaseID string) (json.RawMessage, error) {
	if raw, ok := f.records[releaseID]; ok {
		return raw, nil
	}
	return nil, ErrReleaseNotPublished
}

func (f *fakeReleaseSource) FetchReleasePatch(_ context.Context, computerID, sha string) ([]byte, error) {
	if raw, ok := f.blobs[sha]; ok {
		return raw, nil
	}
	return nil, ErrReleasePatchUnavailable
}

type fakePromptSubmitter struct {
	calls []string
}

func (f *fakePromptSubmitter) submit(_ context.Context, ownerID, text, commandID string) (PromptSubmission, error) {
	f.calls = append(f.calls, text)
	return PromptSubmission{SubmissionID: "submission-1", TrajectoryID: "traj-adopt-1", DocID: "doc-adopt-1"}, nil
}

// publisherRecord is the release computer A published: its patch against the
// shared base.
func publisherRecord(t *testing.T, base, tree string, patch []byte) (release.Record, string, json.RawMessage) {
	t.Helper()
	record := release.Record{
		Schema:    release.Schema,
		Publisher: release.Publisher{ComputerID: "computer-a", OwnerID: "owner-a"},
		Operation: release.Operation{
			OperationID: "op-a", BundleDigest: strings.Repeat("b", 64), ReleaseDigest: strings.Repeat("c", 64),
			CodeRef: "code:sha256:" + strings.Repeat("d", 64), DecisionEvent: strings.Repeat("e", 64),
		},
		Source: release.Source{
			BaseCommit: base, PatchSHA256: computerevent.DigestBytes(patch), PatchBytes: len(patch),
			TreeHash: tree, ChangedPaths: []string{"frontend/public/drill.txt"},
		},
		Declarations: release.Declarations{TestCommand: "test -f frontend/public/drill.txt"},
	}
	id, err := record.ReleaseID()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := record.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	return record, id, raw
}

func adoptBody(t *testing.T, id string, record json.RawMessage) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"release_id": id, "record": record})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

type adoptionError struct {
	Error  string `json:"error"`
	Reason string `json:"reason"`
}

func TestAdoptionRefusesBeforeAnyStateChangeAndStagesOnlyVerifiedSource(t *testing.T) {
	const computerID = "computer-b"
	rt, _, updaterRoot := releaseTestRuntime(t, computerID)
	repo, base := releaseSourceRepo(t)
	rt.releaseSourceDir = repo
	patch, wantTree := releasePatch(t, repo, map[string]string{"frontend/public/drill.txt": "token\n"})
	record, id, raw := publisherRecord(t, base, wantTree, patch)
	source := &fakeReleaseSource{records: map[string]json.RawMessage{id: raw}, blobs: map[string][]byte{record.Source.PatchSHA256: patch}}
	rt.releaseSource = source
	submitter := &fakePromptSubmitter{}
	rt.SetPromptSubmitter(submitter.submit)
	h := NewAPIHandler(rt)
	adoptions := "/api/computers/" + computerID + "/self-development/adoptions"
	repoBefore := releaseGit(t, repo, "status", "--porcelain") + releaseGit(t, repo, "rev-parse", "HEAD")

	assertNoTrace := func(leg string) {
		t.Helper()
		if len(submitter.calls) != 0 {
			t.Fatalf("A6 %s: the desks were asked (%d prompts)", leg, len(submitter.calls))
		}
		list, err := rt.store.ListReleaseAdoptions(context.Background(), "owner-b", computerID)
		if err != nil || len(list) != 0 {
			t.Fatalf("A6 %s: adoption objects = %d, %v", leg, len(list), err)
		}
		if entries, _ := os.ReadDir(filepath.Join(updaterRoot, "adoptions")); len(entries) != 0 {
			t.Fatalf("A6 %s: staged %d entries", leg, len(entries))
		}
	}
	refused := func(leg string, body []byte, wantStatus int, wantReason string) {
		t.Helper()
		w := releaseRouterCall(t, h, http.MethodPost, adoptions, "owner-b", computerID, body)
		var got adoptionError
		_ = json.Unmarshal(w.Body.Bytes(), &got)
		if w.Code != wantStatus || got.Reason != wantReason {
			t.Fatalf("%s = %d %s, want %d %s", leg, w.Code, w.Body.String(), wantStatus, wantReason)
		}
		assertNoTrace(leg)
	}

	// A1: one field changed, the id kept.
	tampered := record
	tampered.Declarations.TestCommand = "true"
	tamperedRaw, _ := tampered.Canonical()
	refused("A1", adoptBody(t, id, tamperedRaw), http.StatusUnprocessableEntity, "release_digest_mismatch")
	// A2: self-consistent but never published.
	unpublishedID, _ := tampered.ReleaseID()
	refused("A2", adoptBody(t, unpublishedID, tamperedRaw), http.StatusNotFound, "release_not_published")
	// A3: the store answers for the id with bytes that hash elsewhere.
	source.records[unpublishedID] = raw
	refused("A3 record", adoptBody(t, unpublishedID, tamperedRaw), http.StatusBadGateway, "published_record_digest_mismatch")
	delete(source.records, unpublishedID)
	// A3/A4: the stored patch was altered, or is missing.
	source.blobs[record.Source.PatchSHA256] = []byte("diff --git a/frontend/public/drill.txt b/frontend/public/drill.txt\n+evil\n")
	refused("A4 altered", adoptBody(t, id, raw), http.StatusConflict, "patch_digest_mismatch")
	delete(source.blobs, record.Source.PatchSHA256)
	refused("A4 missing", adoptBody(t, id, raw), http.StatusConflict, "patch_unavailable")
	source.blobs[record.Source.PatchSHA256] = patch
	// No owner: refused before anything.
	if w := releaseRouterCall(t, h, http.MethodPost, adoptions, "", computerID, adoptBody(t, id, raw)); w.Code != http.StatusUnauthorized {
		t.Fatalf("no owner = %d", w.Code)
	}
	assertNoTrace("no owner")

	// A5: the adopter's own tree diverged where the patch lands.
	divergedRepo, _ := releaseSourceRepo(t)
	if err := os.WriteFile(filepath.Join(divergedRepo, "frontend", "public", "drill.txt"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	releaseGit(t, divergedRepo, "add", "-A")
	releaseGit(t, divergedRepo, "commit", "-q", "-m", "diverged")
	rt.releaseSourceDir = divergedRepo
	refused("A5", adoptBody(t, id, raw), http.StatusConflict, "patch_does_not_apply")
	rt.releaseSourceDir = repo

	// Adopted: the record and the patch are staged read only, the desks are
	// asked once, and the adoption is recorded with B's own tree.
	w := releaseRouterCall(t, h, http.MethodPost, adoptions, "owner-b", computerID, adoptBody(t, id, raw))
	var adopted ReleaseAdoptionView
	if w.Code != http.StatusCreated || json.Unmarshal(w.Body.Bytes(), &adopted) != nil {
		t.Fatalf("adopt = %d %s", w.Code, w.Body.String())
	}
	if adopted.ReleaseID != id || adopted.TrajectoryID != "traj-adopt-1" || !adopted.SameBase || adopted.AdopterTreeHash != wantTree || adopted.AdopterBaseCommit != base {
		t.Fatalf("adoption = %+v", adopted)
	}
	if len(submitter.calls) != 1 || !strings.Contains(submitter.calls[0], "release:"+id) || !strings.Contains(submitter.calls[0], "/selfdev/adoption/source.patch") {
		t.Fatalf("the desks' request = %q", submitter.calls)
	}
	if after := releaseGit(t, repo, "status", "--porcelain") + releaseGit(t, repo, "rev-parse", "HEAD"); after != repoBefore {
		t.Fatal("A7: the adoption wrote to the adopter's source repository")
	}
	staged := filepath.Join(updaterRoot, "adoptions", id)
	entries, err := os.ReadDir(staged)
	if err != nil || len(entries) != 2 {
		t.Fatalf("A10: staged %v, %v; want exactly release.json and source.patch", entries, err)
	}
	stagedPatch, _ := os.ReadFile(filepath.Join(staged, "source.patch"))
	if computerevent.DigestBytes(stagedPatch) != record.Source.PatchSHA256 {
		t.Fatal("staged patch is not the verified one")
	}
	if info, _ := os.Stat(filepath.Join(staged, "source.patch")); info.Mode().Perm()&0o222 != 0 {
		t.Fatalf("staged patch is writable: %v", info.Mode())
	}

	// A12: a repeat names the same adoption and asks nobody again.
	w = releaseRouterCall(t, h, http.MethodPost, adoptions, "owner-b", computerID, adoptBody(t, id, raw))
	var repeat ReleaseAdoptionView
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &repeat) != nil || repeat.AdoptionID != adopted.AdoptionID || len(submitter.calls) != 1 {
		t.Fatalf("A12 repeat = %d %s, prompts %d", w.Code, w.Body.String(), len(submitter.calls))
	}

	// The capsule input: only the adoption's trajectory gets it (A9), and only
	// while the staged bytes still verify (A8).
	ctx := context.Background()
	dir, err := rt.adoptionInputDir(ctx, "owner-b", computerID, "traj-adopt-1")
	if err != nil || dir != staged {
		t.Fatalf("adoption input = %q, %v", dir, err)
	}
	if other, err := rt.adoptionInputDir(ctx, "owner-b", computerID, "traj-other"); err != nil || other != "" {
		t.Fatalf("A9: another trajectory got %q, %v", other, err)
	}

	// The read derives the operation on the adoption's trajectory and whether
	// B's frozen patch is the release's.
	insertReleaseOperation(t, rt.store, releaseOpRow{operationID: "op-b", computerID: computerID, state: selfdev.StateAwaitingApproval,
		bundleDigest: releaseFrozenBundleOn(t, updaterRoot, computerID, "op-b", "traj-adopt-1", base, patch), trajectoryID: "traj-adopt-1"})
	w = releaseRouterCall(t, h, http.MethodGet, adoptions+"/"+adopted.AdoptionID, "owner-b", computerID, nil)
	var read ReleaseAdoptionView
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &read) != nil || read.Operation == nil || read.Operation.OperationID != "op-b" || !read.PatchMatchesRelease {
		t.Fatalf("adoption read = %d %s", w.Code, w.Body.String())
	}
	w = releaseRouterCall(t, h, http.MethodGet, adoptions, "owner-b", computerID, nil)
	var listed struct {
		Adoptions []ReleaseAdoptionView `json:"adoptions"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &listed) != nil || len(listed.Adoptions) != 1 {
		t.Fatalf("adoption list = %d %s", w.Code, w.Body.String())
	}

	if err := os.Chmod(filepath.Join(staged, "source.patch"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staged, "source.patch"), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.adoptionInputDir(ctx, "owner-b", computerID, "traj-adopt-1"); !errors.Is(err, errAdoptionStagedTampered) {
		t.Fatalf("A8: tampered staged bytes = %v", err)
	}
}

func TestAdoptionWithoutASubmitterOrSourceRefuses(t *testing.T) {
	rt, _, _ := releaseTestRuntime(t, "computer-c")
	h := NewAPIHandler(rt)
	w := releaseRouterCall(t, h, http.MethodPost, "/api/computers/computer-c/self-development/adoptions", "owner-c", "computer-c", []byte(`{"release_id":"`+strings.Repeat("a", 64)+`","record":{}}`))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured adoption = %d %s, want 503", w.Code, w.Body.String())
	}
}
