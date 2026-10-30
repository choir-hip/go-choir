package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// The source-patch export (docs/archive/reports/selfdev-readiness-2026-10-15.md B3)
// serves a frozen candidate's source.patch so an agent outside the computer
// can open a draft PR. Ways it could fail, written before the route:
//   - it serves a patch before the candidate is frozen (requested or
//     executing), when no bundle binds the bytes;
//   - it serves bytes that are not the bundle's: a source.patch whose sha256
//     differs from the bundle's recorded source_patch_sha256, or a bundle
//     whose own digest does not match the operation's bundle_digest (a
//     tampered draft or final bundle);
//   - it serves another computer's operation, or a bundle that names a
//     different computer, trajectory or capsule;
//   - after verification the operation's digest is the final bundle digest
//     (bundle.json), not the draft content digest, so a route that reads only
//     bundle.draft.json fails from verified onward;
//   - the bundle directory is gone (retention, cleanup): the route must say
//     so, not answer an empty patch;
//   - the candidate changed no workspace/platform source: there is no patch,
//     and an empty 200 would read as "the change is empty";
//   - the patch carries a credential the outside agent would push to GitHub;
//   - a symlinked source.patch reads a file outside the bundle;
//   - the patch is larger than the proxy relays (4 MiB response cap), so it
//     arrives truncated;
//   - the route accepts a write.
func TestSelfDevelopmentSourcePatchExport(t *testing.T) {
	ctx := context.Background()
	runtime, productStore := testRuntime(t)
	computerID := "computer-selfdev-patch"
	runtime.cfg.ComputerID = computerID
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	runtime.selfdevOperations = operations
	updaterRoot := t.TempDir()
	runtime.selfdevUpdaterRoot = updaterRoot

	baseCommit := strings.Repeat("1", 40)
	patch := []byte("diff --git a/frontend/src/App.svelte b/frontend/src/App.svelte\n--- a/frontend/src/App.svelte\n+++ b/frontend/src/App.svelte\n@@ -1 +1 @@\n-old\n+new\n")
	patchSum := sha256.Sum256(patch)
	patchSHA := hex.EncodeToString(patchSum[:])

	// writeBundle stages incoming/<digest>/ the way Freeze (draft) and
	// Verify (final) leave it, and returns the digest the operation records.
	writeBundle := func(operationID string, final bool, patchBytes []byte, recordedSHA string) string {
		t.Helper()
		bundle := transaction.CapsuleEffectBundle{
			BundleVersion: 1, ComputerID: computerID, BaseEventHead: strings.Repeat("a", 64),
			TrajectoryRef: "trajectory-" + operationID, CapsuleIdentity: "capsule-" + operationID,
			SourcePatchSHA256: recordedSHA, SourcePatchBaseCommit: baseCommit,
			VerifierReceipts: []string{},
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
		if patchBytes != nil {
			if err := os.WriteFile(filepath.Join(dir, "source.patch"), patchBytes, 0o444); err != nil {
				t.Fatal(err)
			}
		}
		return digest
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
	frozenDigest := writeBundle("op-frozen", false, patch, patchSHA)
	insert("op-frozen", computerID, selfdev.StateFrozen, frozenDigest)
	insert("op-awaiting", computerID, selfdev.StateAwaitingApproval, writeBundle("op-awaiting", true, patch, patchSHA))
	insert("op-tampered", computerID, selfdev.StateFrozen, writeBundle("op-tampered", false, append([]byte(nil), append(patch, []byte("+extra\n")...)...), patchSHA))
	insert("op-nosource", computerID, selfdev.StateFrozen, writeBundle("op-nosource", false, nil, ""))
	insert("op-gone", computerID, selfdev.StateApplied, strings.Repeat("e", 64))
	secretPatch := []byte("diff --git a/x.env b/x.env\n--- a/x.env\n+++ b/x.env\n@@ -0,0 +1 @@\n+GITHUB_TOKEN=ghp_" + strings.Repeat("A", 36) + "\n")
	secretSum := sha256.Sum256(secretPatch)
	insert("op-secret", computerID, selfdev.StateFrozen, writeBundle("op-secret", false, secretPatch, hex.EncodeToString(secretSum[:])))
	// An operation naming a bundle frozen for another operation must not serve.
	insert("op-crossbound", computerID, selfdev.StateFrozen, frozenDigest)
	insert("op-elsewhere", "computer-other", selfdev.StateFrozen, writeBundle("op-elsewhere", false, patch, patchSHA))
	hugePatch := []byte(strings.Repeat("+x\n", (sourcePatchExportMaxBytes/3)+1))
	hugeSum := sha256.Sum256(hugePatch)
	insert("op-huge", computerID, selfdev.StateFrozen, writeBundle("op-huge", false, hugePatch, hex.EncodeToString(hugeSum[:])))
	// A symlinked source.patch must not be followed.
	symDigest := writeBundle("op-symlink", false, nil, patchSHA)
	outside := filepath.Join(t.TempDir(), "outside.patch")
	if err := os.WriteFile(outside, patch, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(updaterRoot, "incoming", symDigest, "source.patch")); err != nil {
		t.Fatal(err)
	}
	insert("op-symlink", computerID, selfdev.StateFrozen, symDigest)

	handler := &APIHandler{rt: runtime}
	get := func(method, operationID string) (int, map[string]any, string) {
		request := httptest.NewRequest(method, "/api/computers/"+computerID+"/self-development/operations/"+operationID+"/source-patch", nil)
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
		served, err := base64.StdEncoding.DecodeString(body["patch_base64"].(string))
		if err != nil || string(served) != string(patch) {
			t.Fatalf("%s: served patch %q (%v)", operationID, served, err)
		}
		if body["source_patch_sha256"] != patchSHA || body["source_patch_base_commit"] != baseCommit ||
			body["operation_id"] != operationID || body["computer_id"] != computerID || body["bundle_digest"] == "" ||
			body["state"] == "" || body["patch_bytes"] != float64(len(patch)) {
			t.Fatalf("%s: binding fields %s", operationID, raw)
		}
	}
	refusals := map[string]int{
		"op-executing":  http.StatusConflict,              // not frozen yet
		"op-tampered":   http.StatusConflict,              // bytes do not hash to the bundle's sha256
		"op-nosource":   http.StatusNotFound,              // no workspace/platform change
		"op-gone":       http.StatusGone,                  // bundle no longer retained
		"op-secret":     http.StatusUnprocessableEntity,   // credential in an added line
		"op-crossbound": http.StatusConflict,              // bundle binds another operation
		"op-elsewhere":  http.StatusNotFound,              // another computer's operation
		"op-symlink":    http.StatusConflict,              // not a regular file in the bundle
		"op-missing":    http.StatusNotFound,              // no such operation
		"op-huge":       http.StatusRequestEntityTooLarge, // over what the proxy relays
	}
	for operationID, want := range refusals {
		code, body, raw := get(http.MethodGet, operationID)
		if code != want || body["patch_base64"] != nil {
			t.Fatalf("%s: status=%d body=%s, want %d and no patch", operationID, code, raw, want)
		}
	}
	if code, _, raw := get(http.MethodPost, "op-frozen"); code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status=%d body=%s, want 405", code, raw)
	}
}
