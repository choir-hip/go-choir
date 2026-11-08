package agentcore

// R3.1 revert (docs/design/selfdev-apply-and-revert-2026-11-06.md, the R3.1
// amendment). Ways this could fail, written first:
//   - the d@e shape (X's chain prior is an applied platform operation with no
//     checkpoint, X's signed receipt names the booted base as the release it
//     replaced) answers 409 "not a restorable checkpoint" instead of opening a
//     base restore;
//   - the receipt's prior is read from an unsigned or foreign source, so a row
//     or an event the guest cannot vouch for opens a base restore;
//   - a receipt prior that is some other release still restores the base;
//   - the refusal does not name the release X replaced;
//   - a silent deploy leaves a full-evidence chain prior and the revert rolls
//     back to it, reinstalling a dead overlay over the new base;
//   - P names a full-evidence operation that is not the chain prior and the
//     revert follows the chain instead;
//   - the base is matched by digest, which drifts with the route's refs;
//   - a receipt that fails verification is trusted as P.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// importBootedBase makes the booted base release the served one, as a fresh
// computer's first checkpoint does, and returns its content digest.
func (fx *derivableSelfDevFixture) importBootedBase(t *testing.T) string {
	t.Helper()
	root := fx.withBaselineRoot(t)
	manifest, err := fx.rt.servingBaselineManifest(context.Background(), fx.computerID, root)
	if err != nil {
		t.Fatal(err)
	}
	request := updater.BaselineImportRequest{
		ComputerID: fx.computerID, RealizationID: "realization-derivable",
		IdempotencyKey: "r31-baseline", SourceDir: root, Manifest: manifest,
	}
	if request.RequestCommitment, err = updater.ComputeBaselineImportCommitment(request); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.engine.ImportBaseline(request); err != nil {
		t.Fatal(err)
	}
	return manifest.ContentDigest
}

// pointCurrentAt repoints the served release, as a deploy refresh does.
func (fx *derivableSelfDevFixture) pointCurrentAt(t *testing.T, digest string) {
	t.Helper()
	link := filepath.Join(fx.updaterRoot, "current")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(fx.updaterRoot, "releases", digest), link); err != nil {
		t.Fatal(err)
	}
}

func (fx *derivableSelfDevFixture) revertRaw(t *testing.T, operationID string) *httptest.ResponseRecorder {
	t.Helper()
	fx.waitForStableHead(t)
	recorder := httptest.NewRecorder()
	(&APIHandler{rt: fx.rt}).revertSelfDevelopmentOperation(recorder, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}")), "owner", fx.computerID, operationID)
	return recorder
}

func (fx *derivableSelfDevFixture) stripEvidence(t *testing.T, operationID string) {
	t.Helper()
	if _, err := fx.store.DB().ExecContext(context.Background(), `UPDATE self_development_operations SET checkpoint_ref='', route_receipt='' WHERE operation_id=?`, operationID); err != nil {
		t.Fatal(err)
	}
	row, err := fx.operations.Get(context.Background(), fx.computerID, operationID)
	if err != nil || revertTargetHasEvidence(row) {
		t.Fatalf("setup: %s should now lack evidence: %+v err=%v", operationID, row, err)
	}
}

// d@e: S applied (stale, evidence-less), a deploy put the plain base back, X
// applied on the base. X's receipt prior is the booted base.
func TestRevertBaseRestoreFromReceiptWhenChainPriorLacksEvidence(t *testing.T) {
	fx := joinFixture(t, "rv-r31-base", joinTestBaseA, joinTestBaseA)
	base := fx.importBootedBase(t)
	stale := fx.applyPlain(t, "op-rv-r31-stale")
	fx.pointCurrentAt(t, base)
	fx.stripEvidence(t, stale.OperationID)
	x := fx.applyPlain(t, "op-rv-r31-x")
	if prior, _, err := fx.rt.replacedReleaseDigest(context.Background(), x); err != nil || prior != base {
		t.Fatalf("setup: X's receipt prior = %q err=%v; want the booted base %s", prior, err, base)
	}

	code, opened := fx.revert(t, x.OperationID)
	if code != http.StatusCreated || !selfdev.IsBaseRestore(opened) || selfdev.BaseRestoreSubject(opened) != x.OperationID {
		t.Fatalf("revert status=%d operation=%+v; want a base restore of %s", code, opened, x.OperationID)
	}
	// The restore targets the booted base labelled with the route's current
	// refs (as every base restore does), not the stale operation's release.
	now, err := fx.rt.servingBaselineManifest(context.Background(), fx.computerID, fx.rt.packageMigrationBaselineRoot())
	if err != nil || opened.ReleaseDigest != now.ContentDigest || opened.ReleaseDigest == stale.ReleaseDigest {
		t.Fatalf("base restore targets %s; want the booted base %s", opened.ReleaseDigest, now.ContentDigest)
	}
	fx.waitForState(t, opened.OperationID, selfdev.StateRolledBack)
	if served := fx.currentManifest(t); served.ContentDigest != now.ContentDigest || served.LayeringEntrypoint != "" {
		t.Fatalf("served release = %+v; want the booted base with no overlay", served)
	}
}

// A silent deploy: the chain prior A carries full evidence, but a deploy put
// the plain base back before X. P is the base, so the revert is a base
// restore, not a rollback that reinstalls A's dead overlay.
func TestRevertBaseRestoreWhenPIsBaseEvenIfChainPriorHasFullEvidence(t *testing.T) {
	fx := joinFixture(t, "rv-r31-full", joinTestBaseA, joinTestBaseA)
	base := fx.importBootedBase(t)
	first := fx.applyPlain(t, "op-rv-r31-first")
	fx.pointCurrentAt(t, base)
	x := fx.applyPlain(t, "op-rv-r31-x")
	if prior, _, err := fx.rt.replacedReleaseDigest(context.Background(), x); err != nil || prior != base {
		t.Fatalf("setup: X's receipt prior = %q err=%v; want the booted base", prior, err)
	}
	code, reverted := fx.revert(t, x.OperationID)
	if code != http.StatusCreated || !selfdev.IsBaseRestore(reverted) {
		t.Fatalf("revert status=%d operation=%+v; want a base restore, not a rollback to %s", code, reverted, first.OperationID)
	}
}

// P is the release of a full-evidence operation that is not the chain prior:
// the rollback goes to that operation.
func TestRevertRollsBackToTheOperationPNames(t *testing.T) {
	fx := joinFixture(t, "rv-r31-pop", joinTestBaseA, joinTestBaseA)
	fx.importBootedBase(t)
	a := fx.applyPlain(t, "op-rv-r31-a")
	s := fx.applyPlain(t, "op-rv-r31-s")
	fx.stripEvidence(t, s.OperationID)
	fx.pointCurrentAt(t, a.ReleaseDigest)
	x := fx.applyPlain(t, "op-rv-r31-x")
	if prior, _, err := fx.rt.replacedReleaseDigest(context.Background(), x); err != nil || prior != a.ReleaseDigest {
		t.Fatalf("setup: X's receipt prior = %q err=%v; want A's release", prior, err)
	}
	code, reverted := fx.revert(t, x.OperationID)
	if code != http.StatusCreated || selfdev.IsBaseRestore(reverted) || reverted.BaseHead != a.EffectiveHead {
		t.Fatalf("revert status=%d operation=%+v; want the rollback to %s", code, reverted, a.OperationID)
	}
}

// P is neither the base nor a full-evidence operation's release: 409 naming P
// and the chain prior.
func TestRevertRefusesWhenPIsUnknownAndNamesIt(t *testing.T) {
	fx := joinFixture(t, "rv-r31-other", joinTestBaseA, joinTestBaseA)
	fx.importBootedBase(t)
	stale := fx.applyPlain(t, "op-rv-r31-stale")
	fx.stripEvidence(t, stale.OperationID)
	x := fx.applyPlain(t, "op-rv-r31-x") // replaces the stale overlay, not the base
	recorder := fx.revertRaw(t, x.OperationID)
	body := recorder.Body.String()
	if recorder.Code != http.StatusConflict || !strings.Contains(body, "not a restorable checkpoint") ||
		!strings.Contains(body, stale.ReleaseDigest) || !strings.Contains(body, stale.EffectiveHead) {
		t.Fatalf("revert status=%d body=%s; want 409 naming P %s and the chain prior %s", recorder.Code, body, stale.ReleaseDigest, stale.EffectiveHead)
	}
}

// No P (a first-ever apply has no prior release): the legacy chain order.
func TestRevertWithNoPUsesTheChainOrder(t *testing.T) {
	fx := joinFixture(t, "rv-r31-nop", joinTestBaseA, joinTestBaseA)
	fx.withBaselineRoot(t)
	first := fx.applyPlain(t, "op-rv-r31-first")
	if prior, _, err := fx.rt.replacedReleaseDigest(context.Background(), first); err != nil || prior != "" {
		t.Fatalf("setup: P = %q err=%v; want none", prior, err)
	}
	code, opened := fx.revert(t, first.OperationID)
	if code != http.StatusCreated || !selfdev.IsBaseRestore(opened) {
		t.Fatalf("revert status=%d operation=%+v; want the genesis base restore", code, opened)
	}
}

// A receipt that fails verification chooses no target: the revert refuses (409)
// rather than falling back to the chain order.
func TestRevertRefusesAReceiptThatFailsVerification(t *testing.T) {
	fx := joinFixture(t, "rv-r31-bad", joinTestBaseA, joinTestBaseA)
	base := fx.importBootedBase(t)
	stale := fx.applyPlain(t, "op-rv-r31-stale")
	fx.pointCurrentAt(t, base)
	fx.stripEvidence(t, stale.OperationID)
	x := fx.applyPlain(t, "op-rv-r31-x")
	// The row no longer names the receipt the signed event carries.
	if _, err := fx.store.DB().ExecContext(context.Background(), `UPDATE self_development_operations SET materialization_receipt=? WHERE operation_id=?`, strings.Repeat("0", 64), x.OperationID); err != nil {
		t.Fatal(err)
	}
	row, _ := fx.operations.Get(context.Background(), fx.computerID, x.OperationID)
	if prior, status, err := fx.rt.replacedReleaseDigest(context.Background(), row); err == nil || status != http.StatusConflict {
		t.Fatalf("a receipt the row does not name was accepted: P=%q status=%d", prior, status)
	}
	recorder := fx.revertRaw(t, x.OperationID)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "cannot be decided") {
		t.Fatalf("revert status=%d body=%s; want a 409 refusal that chooses no target", recorder.Code, recorder.Body.String())
	}
	if len(fx.rollbackOps(t)) != 0 {
		t.Fatal("a refused revert opened an operation")
	}
}

type failingPayloadReader struct{}

func (failingPayloadReader) FetchPayload(context.Context, string, string) ([]byte, error) {
	return nil, errors.New("store unavailable")
}

// A payload fetch error is a retryable 503, not a fall back to the chain order.
func TestRevertRefusesRetryablyWhenThePayloadCannotBeRead(t *testing.T) {
	fx := joinFixture(t, "rv-r31-503", joinTestBaseA, joinTestBaseA)
	base := fx.importBootedBase(t)
	stale := fx.applyPlain(t, "op-rv-r31-stale")
	fx.pointCurrentAt(t, base)
	fx.stripEvidence(t, stale.OperationID)
	x := fx.applyPlain(t, "op-rv-r31-x")
	fx.configure(func(rt *Runtime) { rt.eventPayloadReader = failingPayloadReader{} })
	recorder := fx.revertRaw(t, x.OperationID)
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "retry") {
		t.Fatalf("revert status=%d body=%s; want a retryable 503", recorder.Code, recorder.Body.String())
	}
	if len(fx.rollbackOps(t)) != 0 {
		t.Fatal("a refused revert opened an operation")
	}
}

// P's pinned manifest is gone: the refusal says so instead of calling P
// "neither a base import nor ...".
func TestRevertNamesAnUnreadablePManifest(t *testing.T) {
	fx := joinFixture(t, "rv-r31-gone", joinTestBaseA, joinTestBaseA)
	base := fx.importBootedBase(t)
	stale := fx.applyPlain(t, "op-rv-r31-stale")
	fx.pointCurrentAt(t, base)
	fx.stripEvidence(t, stale.OperationID)
	x := fx.applyPlain(t, "op-rv-r31-x")
	releaseDir := filepath.Join(fx.updaterRoot, "releases", base)
	if err := os.Chmod(releaseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(releaseDir, 0o755) })
	if err := os.Remove(filepath.Join(releaseDir, "release-manifest.json")); err != nil {
		t.Fatal(err)
	}
	recorder := fx.revertRaw(t, x.OperationID)
	body := recorder.Body.String()
	if (recorder.Code != http.StatusConflict && recorder.Code != http.StatusServiceUnavailable) || !strings.Contains(body, "unreadable") || !strings.Contains(body, base) || strings.Contains(body, "neither") {
		t.Fatalf("revert status=%d body=%s; want a refusal naming P's unreadable manifest", recorder.Code, body)
	}
}

// A deploy after X rebuilt the baseline root. P is still the old base import,
// so the revert restores the CURRENT booted base with no overlay.
func TestRevertRestoresTheCurrentBaseAfterADeployChangedIt(t *testing.T) {
	fx := joinFixture(t, "rv-r31-deploy", joinTestBaseA, joinTestBaseA)
	base := fx.importBootedBase(t)
	x := fx.applyPlain(t, "op-rv-r31-x")
	if prior, _, err := fx.rt.replacedReleaseDigest(context.Background(), x); err != nil || prior != base {
		t.Fatalf("setup: P = %q err=%v; want the old base import %s", prior, err, base)
	}
	index := filepath.Join(fx.rt.packageMigrationBaselineRoot(), "frontend", "index.html")
	if err := os.Chmod(index, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(index, []byte("<html>booted base after a deploy</html>"), 0o444); err != nil {
		t.Fatal(err)
	}
	code, opened := fx.revert(t, x.OperationID)
	if code != http.StatusCreated || !selfdev.IsBaseRestore(opened) {
		t.Fatalf("revert status=%d operation=%+v; want a base restore", code, opened)
	}
	now, err := fx.rt.servingBaselineManifest(context.Background(), fx.computerID, fx.rt.packageMigrationBaselineRoot())
	if err != nil || opened.ReleaseDigest != now.ContentDigest || opened.ReleaseDigest == base {
		t.Fatalf("restore targets %s; want the new booted base %s (old import %s)", opened.ReleaseDigest, now.ContentDigest, base)
	}
	fx.waitForState(t, opened.OperationID, selfdev.StateRolledBack)
	if served := fx.currentManifest(t); served.ContentDigest != now.ContentDigest || served.LayeringEntrypoint != "" {
		t.Fatalf("served release = %+v; want the new booted base", served)
	}
}
