package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// GET /api/computers/<id>/self-development/operations/<op>/visual-evidence
// (docs/design/visual-evidence-slice-2026-10-15.md V22, V26): the thin
// operation-scoped listing of which evidence digests belong to an operation,
// the drill's instrument. The bytes come from the asset route, not here.
// Ways it could fail, written before the route:
//   - it lists captures before the candidate is frozen, or for a bundle it
//     does not bind (the source-patch export's rules);
//   - it answers an empty 200 for a candidate with no captures;
//   - it claims a capture is resolvable when the image store does not hold it
//     (the drill's check for a failed image store write, V22);
//   - it accepts a write.
func TestSelfDevelopmentVisualEvidenceListing(t *testing.T) {
	ctx := context.Background()
	runtime, productStore := testRuntime(t)
	computerID := "computer-selfdev-visual"
	runtime.cfg.ComputerID = computerID
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	runtime.selfdevOperations = operations
	updaterRoot := t.TempDir()
	runtime.selfdevUpdaterRoot = updaterRoot

	stored, _, err := runtime.imageStore().Put(assetPNG(t))
	if err != nil {
		t.Fatal(err)
	}
	missingDigest := strings.Repeat("e", 64)
	receipt := "capsule-exec:sha256:" + strings.Repeat("9", 64)

	writeBundle := func(operationID string, final bool, visual *capsule.VisualEvidence) string {
		t.Helper()
		bundle := transaction.CapsuleEffectBundle{
			BundleVersion: 1, ComputerID: computerID, BaseEventHead: strings.Repeat("a", 64),
			TrajectoryRef: "trajectory-" + operationID, CapsuleIdentity: "capsule-" + operationID,
			VerifierReceipts: []string{}, VisualEvidence: visual,
		}
		bundle.ContentDigest, err = bundle.ComputeContentDigest()
		if err != nil {
			t.Fatal(err)
		}
		name, digest := "bundle.draft.json", bundle.ContentDigest
		if final {
			bundle.VerifierReceipts = []string{strings.Repeat("f", 64)}
			name = "bundle.json"
		}
		raw, err := computerevent.CanonicalJSON(bundle)
		if err != nil {
			t.Fatal(err)
		}
		if final {
			digest = computerevent.DigestBytes(raw)
		}
		dir := filepath.Join(updaterRoot, "incoming", digest)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), raw, 0o400); err != nil {
			t.Fatal(err)
		}
		return digest
	}
	visual := &capsule.VisualEvidence{
		ManifestSHA256: strings.Repeat("8", 64), FrontendSHA256: strings.Repeat("7", 64), CaptureReceipt: receipt,
		Captures: []capsule.VisualEvidenceCapture{
			{Artifact: stored, Route: "/choir-drill/x.html", MediaType: "image/png", Bytes: 99, Width: 1280, Height: 800},
			{Artifact: "artifact:sha256:" + missingDigest, Route: "/", MediaType: "image/png", Bytes: 77, Width: 1280, Height: 800, Blank: true},
		},
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	insert := func(operationID, owner, state, bundleDigest string) {
		t.Helper()
		if _, err := productStore.DB().ExecContext(ctx, `INSERT INTO self_development_operations (operation_id,computer_id,idempotency_key,request_commitment,trajectory_id,capsule_id,base_head,prompt_artifact_ref,bundle_digest,verifier_refs_json,desired_head,effective_head,state,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,'[]',?,?,?,?,?)`,
			operationID, owner, "key-"+operationID, strings.Repeat("d", 64), "trajectory-"+operationID, "capsule-"+operationID, strings.Repeat("a", 64),
			"artifact:sha256:"+strings.Repeat("b", 64), bundleDigest, strings.Repeat("c", 64), strings.Repeat("c", 64), state, now, now); err != nil {
			t.Fatal(err)
		}
	}
	insert("op-executing", computerID, selfdev.StateExecuting, "")
	insert("op-frozen", computerID, selfdev.StateFrozen, writeBundle("op-frozen", false, visual))
	insert("op-awaiting", computerID, selfdev.StateAwaitingApproval, writeBundle("op-awaiting", true, visual))
	insert("op-plain", computerID, selfdev.StateFrozen, writeBundle("op-plain", false, nil))
	insert("op-gone", computerID, selfdev.StateApplied, strings.Repeat("e", 64))
	insert("op-elsewhere", "computer-other", selfdev.StateFrozen, writeBundle("op-elsewhere", false, visual))

	handler := &APIHandler{rt: runtime}
	get := func(method, operationID string) (int, map[string]any, string) {
		request := httptest.NewRequest(method, "/api/computers/"+computerID+"/self-development/operations/"+operationID+"/visual-evidence", nil)
		request.Header.Set("X-Authenticated-User", "owner")
		request.Header.Set("X-Authenticated-Computer", computerID)
		response := httptest.NewRecorder()
		handler.HandleComputersRouter(response, request)
		var body map[string]any
		_ = json.Unmarshal(response.Body.Bytes(), &body)
		return response.Code, body, response.Body.String()
	}

	for _, operationID := range []string{"op-frozen", "op-awaiting"} {
		code, body, raw := get(http.MethodGet, operationID)
		if code != http.StatusOK {
			t.Fatalf("%s: status=%d body=%s", operationID, code, raw)
		}
		if body["operation_id"] != operationID || body["computer_id"] != computerID || body["bundle_digest"] == "" ||
			body["capture_receipt"] != receipt || body["manifest_sha256"] != visual.ManifestSHA256 || body["frontend_sha256"] != visual.FrontendSHA256 {
			t.Fatalf("%s: binding fields %s", operationID, raw)
		}
		captures, _ := body["captures"].([]any)
		if len(captures) != 2 {
			t.Fatalf("%s: captures %s", operationID, raw)
		}
		first, _ := captures[0].(map[string]any)
		if first["artifact"] != stored || first["digest"] != strings.TrimPrefix(stored, "artifact:sha256:") ||
			first["route"] != "/choir-drill/x.html" || first["resolvable"] != true ||
			first["asset_url"] != "/api/assets/sha256/"+strings.TrimPrefix(stored, "artifact:sha256:") ||
			first["width"] != float64(1280) || first["height"] != float64(800) {
			t.Fatalf("%s: first capture %v", operationID, first)
		}
		second, _ := captures[1].(map[string]any)
		if second["resolvable"] != false || second["blank"] != true {
			t.Fatalf("%s: a capture the store does not hold must say so: %v", operationID, second)
		}
	}
	refusals := map[string]int{
		"op-executing": http.StatusConflict, // not frozen yet
		"op-plain":     http.StatusNotFound, // no captures: not an empty 200
		"op-gone":      http.StatusGone,     // bundle no longer retained
		"op-elsewhere": http.StatusNotFound, // another computer's operation
		"op-missing":   http.StatusNotFound,
	}
	for operationID, want := range refusals {
		if code, body, raw := get(http.MethodGet, operationID); code != want || body["captures"] != nil {
			t.Fatalf("%s: status=%d body=%s, want %d and no captures", operationID, code, raw, want)
		}
	}
	// The Texture evidence embed (station TE) reads the same listing, with
	// the operation's trajectory, which the route does not answer.
	listing, err := runtime.CandidateVisualEvidence(ctx, computerID, "op-frozen")
	if err != nil || listing.TrajectoryID != "trajectory-op-frozen" || len(listing.Captures) != 2 || listing.Captures[1].Resolvable {
		t.Fatalf("CandidateVisualEvidence = %+v, %v", listing, err)
	}
	if _, _, raw := get(http.MethodGet, "op-frozen"); strings.Contains(raw, "trajectory") {
		t.Fatalf("the route answers the trajectory: %s", raw)
	}
	if _, err := runtime.CandidateVisualEvidence(ctx, computerID, "op-elsewhere"); err == nil {
		t.Fatal("another computer's operation resolved")
	}
	if code, _, raw := get(http.MethodPost, "op-frozen"); code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status=%d body=%s, want 405", code, raw)
	}
}
