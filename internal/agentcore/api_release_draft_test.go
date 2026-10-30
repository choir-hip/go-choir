package agentcore

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/release"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// P7 guest side, failure modes pinned (docs/archive/reports/p6-p8-release-adopt-2026-10-15.md):
//   R5 an unapplied operation is drafted, or a patch with secret-bearing
//      added lines;
//   R6 the record carries anything but source (host build digests only,
//      packages empty);
//   the draft is read-only (no state moves; GET only).

func releaseRouterCall(t *testing.T, h *APIHandler, method, path, owner, computer string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body != nil {
		reader = strings.NewReader(string(body))
	} else {
		reader = strings.NewReader("")
	}
	r := httptest.NewRequest(method, path, reader)
	if owner != "" {
		r.Header.Set("X-Authenticated-User", owner)
	}
	r.Header.Set("X-Authenticated-Computer", computer)
	w := httptest.NewRecorder()
	h.HandleComputersRouter(w, r)
	return w
}

type releaseDraftResponse struct {
	ReleaseID   string          `json:"release_id"`
	Record      json.RawMessage `json:"record"`
	PatchSHA256 string          `json:"patch_sha256"`
	PatchBase64 string          `json:"patch_base64"`
}

func TestReleaseDraftIsTheAppliedOperationAsSource(t *testing.T) {
	const computerID = "computer-publisher"
	rt, productStore, updaterRoot := releaseTestRuntime(t, computerID)
	repo, base := releaseSourceRepo(t)
	rt.releaseSourceDir = repo
	patch, wantTree := releasePatch(t, repo, map[string]string{"frontend/public/drill.txt": "token\n"})
	h := NewAPIHandler(rt)

	// The applied release is layered (P5's mint): the draft names the host
	// build's digests and nothing else of the build (R6, the P5 seam).
	layered := writeCurrentRelease(t, updaterRoot, computerID, "selfdev-layered", func(m *updater.ReleaseManifest) {
		m.BuilderReceiptDigest, m.ClosureDigest, m.CodeCommit = strings.Repeat("4", 64), strings.Repeat("5", 64), strings.Repeat("6", 40)
	})
	insertReleaseOperation(t, productStore, releaseOpRow{operationID: "op-applied", computerID: computerID, state: selfdev.StateApplied,
		bundleDigest: releaseFrozenBundle(t, updaterRoot, computerID, "op-applied", base, patch), releaseDigest: layered})
	insertReleaseOperation(t, productStore, releaseOpRow{operationID: "op-awaiting", computerID: computerID, state: selfdev.StateAwaitingApproval,
		bundleDigest: releaseFrozenBundle(t, updaterRoot, computerID, "op-awaiting", base, patch)})
	secret := []byte("diff --git a/x.env b/x.env\nnew file mode 100644\n--- /dev/null\n+++ b/x.env\n@@ -0,0 +1 @@\n+GITHUB_TOKEN=ghp_" + strings.Repeat("A", 36) + "\n")
	insertReleaseOperation(t, productStore, releaseOpRow{operationID: "op-secret", computerID: computerID, state: selfdev.StateApplied,
		bundleDigest: releaseFrozenBundle(t, updaterRoot, computerID, "op-secret", base, secret), releaseDigest: strings.Repeat("3", 64)})

	draft := func(op, query string) *httptest.ResponseRecorder {
		return releaseRouterCall(t, h, http.MethodGet, "/api/computers/"+computerID+"/self-development/operations/"+op+"/release-draft"+query, "owner-a", computerID, nil)
	}
	if w := draft("op-awaiting", ""); w.Code != http.StatusConflict {
		t.Fatalf("R5 unapplied = %d %s, want 409", w.Code, w.Body.String())
	}
	if w := draft("op-secret", ""); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("R5 secret = %d %s, want 422", w.Code, w.Body.String())
	}
	if w := releaseRouterCall(t, h, http.MethodPost, "/api/computers/"+computerID+"/self-development/operations/op-applied/release-draft", "owner-a", computerID, []byte("{}")); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("draft write = %d, want 405", w.Code)
	}

	w := draft("op-applied", "?test_command="+url.QueryEscape("test -f frontend/public/drill.txt"))
	var got releaseDraftResponse
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &got) != nil {
		t.Fatalf("draft = %d %s", w.Code, w.Body.String())
	}
	record, err := release.Decode(got.Record)
	if err != nil || release.VerifyID(record, got.ReleaseID) != nil {
		t.Fatalf("draft record does not verify against its id: %v", err)
	}
	gotPatch, err := base64.StdEncoding.DecodeString(got.PatchBase64)
	if err != nil || release.VerifyPatch(record, gotPatch) != nil || got.PatchSHA256 != computerevent.DigestBytes(patch) {
		t.Fatal("draft patch is not the record's")
	}
	if record.Publisher != (release.Publisher{ComputerID: computerID, OwnerID: "owner-a"}) || record.Operation.OperationID != "op-applied" ||
		record.Operation.ReleaseDigest != layered || record.Source.BaseCommit != base || record.Source.TreeHash != wantTree ||
		strings.Join(record.Source.ChangedPaths, ",") != "frontend/public/drill.txt" || record.Declarations.TestCommand != "test -f frontend/public/drill.txt" {
		t.Fatalf("draft record = %+v", record)
	}
	if record.HostBuild == nil || record.HostBuild.ClosureDigest != strings.Repeat("5", 64) || record.HostBuild.CodeCommit != strings.Repeat("6", 40) {
		t.Fatalf("host build = %+v, want the layered manifest's digests", record.HostBuild)
	}
	// Deterministic: a second draft is the same release (a republish names
	// the same object, R8).
	var again releaseDraftResponse
	if w := draft("op-applied", "?test_command="+url.QueryEscape("test -f frontend/public/drill.txt")); json.Unmarshal(w.Body.Bytes(), &again) != nil || again.ReleaseID != got.ReleaseID {
		t.Fatalf("second draft id %s, first %s", again.ReleaseID, got.ReleaseID)
	}
	// Read-only: the bundle directory and the operation are unchanged.
	op, err := rt.selfdevOperations.Get(context.Background(), computerID, "op-applied")
	if err != nil || op.State != selfdev.StateApplied {
		t.Fatalf("operation after draft = %+v %v", op, err)
	}
	if entries, _ := os.ReadDir(filepath.Join(updaterRoot, "incoming", op.BundleDigest)); len(entries) != 2 {
		t.Fatalf("bundle dir after draft has %d entries", len(entries))
	}
}

func TestReleasePrecommitmentsCarryTheRuleSettlement(t *testing.T) {
	yes := 0.8
	precommit := types.CommitmentRecord{
		RecordID: "pc-1",
		Precommit: &types.CommitmentPrecommit{
			Question:     "Do the tests pass?",
			Distribution: map[string]float64{types.PrecommitAnswerYes: yes, types.PrecommitAnswerNo: 1 - yes},
			Rule:         &types.PrecommitRule{Kind: types.PrecommitRuleTestsPass},
		},
	}
	resolve := &types.CommitmentRecord{RecordID: types.RuleResolveRecordID("pc-1"), Resolve: &types.CommitmentResolve{Verdict: types.PrecommitAnswerYes}}
	got := releasePrecommitment(precommit, resolve)
	if got.RecordID != "pc-1" || got.Kind != "tests_pass" || got.PYesPermille != 800 || got.Settled != "yes" || got.ResolveRecordID != "pc-1:resolve:rule" {
		t.Fatalf("precommitment = %+v", got)
	}
	if open := releasePrecommitment(precommit, nil); open.Settled != "" || open.ResolveRecordID != "" {
		t.Fatalf("an unsettled precommitment reads as settled: %+v", open)
	}
}
