package updater

// The store floor in the updater (event-table design 4.5; review finding A
// on deploy 1, for deploy 2). Once a binary has stamped an op for
// choir_events, its store carries a floor (storeschema.FloorFile, beside the
// epoch receipt) that no binary below it may run on. Ways the updater can
// breach it, written before the code:
//   - restorePrior, after a failed health probe, swaps back to a release
//     below the floor (it never ran the state-compat check);
//   - restorePrior swaps back to a release that declares no epoch on a
//     floored store, where nothing proves it is at or above the floor;
//   - Apply takes a release below the floor because an older binary already
//     rewrote the epoch receipt downward;
//   - Apply takes an undeclared release (a platform-follow offer) on a
//     floored store;
//   - a refused recovery still swaps the pointer or restarts the guest;
//   - with no floor (deploy 1 alone), recovery to an older release is
//     refused, which deploy 1 keeps allowed.

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
)

type floorRig struct {
	engine  *Updater
	service *fakeServiceManager
	prober  *fakeHealthProber
	root    string
	schema  string
}

func newFloorRig(t *testing.T, receipt uint64) floorRig {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })
	service := &fakeServiceManager{}
	prober := &fakeHealthProber{}
	engine, err := New(root, "computer-test", "realization-test", service, prober, testReceiptSigner{key: computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "updater-test"}, PrivateKey: privateKey}})
	if err != nil {
		t.Fatal(err)
	}
	schema := writeSchemaReceipt(t, t.TempDir(), receipt)
	engine.WithStoreSchemaPath(schema)
	return floorRig{engine: engine, service: service, prober: prober, root: root, schema: schema}
}

func (r floorRig) release(t *testing.T, op string, epoch uint64) ApplyRequest {
	t.Helper()
	request := updaterRequestFixture(t, r.root, "computer-test", "realization-test", op, "idem-"+op, "release "+op)
	if epoch != 0 {
		request.Manifest.StoreSchemaVersion = epoch
		request.Manifest.MinStoreSchemaVersion = 1
		if err := refinalizeRequest(&request); err != nil {
			t.Fatal(err)
		}
	}
	return request
}

func (r floorRig) floor(t *testing.T, version uint64) {
	t.Helper()
	if err := storeschema.WriteFloor(filepath.Dir(r.schema), version, "the event-table writer is on"); err != nil {
		t.Fatal(err)
	}
}

func (r floorRig) current(t *testing.T) string {
	t.Helper()
	digest, _, err := r.engine.currentRelease()
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

// recoverTo applies prior (healthy), then a broken next release whose
// health probe fails, so Apply recovers toward prior.
func (r floorRig) recoverTo(t *testing.T, prior ApplyRequest, floorAfterPrior uint64) (ApplyResult, error, ApplyRequest) {
	t.Helper()
	if _, err := r.engine.Apply(context.Background(), prior); err != nil {
		t.Fatalf("apply prior: %v", err)
	}
	if floorAfterPrior != 0 {
		// The next binary opens the store (receipt to this epoch) and
		// switches the writer on (the floor), then fails its probe.
		writeSchemaReceipt(t, filepath.Dir(r.schema), storeschema.Version)
		r.floor(t, floorAfterPrior)
	}
	next := r.release(t, "next", storeschema.Version)
	r.prober.failDigest = next.Manifest.ContentDigest
	result, err := r.engine.Apply(context.Background(), next)
	return result, err, next
}

func TestRestorePriorRefusesAReleaseBelowTheFloor(t *testing.T) {
	r := newFloorRig(t, 1)
	prior := r.release(t, "prior-epoch-1", 1)
	result, err, next := r.recoverTo(t, prior, 2)
	if err == nil || !strings.Contains(err.Error(), "floor") {
		t.Fatalf("recovery below the floor: err=%v, want a refusal naming the floor", err)
	}
	if result.RecoveryReceipt != nil {
		t.Fatal("a refused recovery signed a recovery receipt")
	}
	if got := r.current(t); got != next.Manifest.ContentDigest {
		t.Fatalf("current = %s; a refused recovery swapped the pointer (want the failed release %s kept)", got, next.Manifest.ContentDigest)
	}
	if r.service.recoveries != 0 {
		t.Fatalf("a refused recovery restarted the guest %d times", r.service.recoveries)
	}
}

func TestRestorePriorRefusesAnUndeclaredReleaseOnAFlooredStore(t *testing.T) {
	r := newFloorRig(t, 2)
	prior := r.release(t, "prior-undeclared", 0)
	_, err, _ := r.recoverTo(t, prior, 2)
	if err == nil || !strings.Contains(err.Error(), "floor") {
		t.Fatalf("recovery to an undeclared release on a floored store: err=%v, want a refusal", err)
	}
	if r.service.recoveries != 0 {
		t.Fatal("the guest was restarted into an undeclared release on a floored store")
	}
}

func TestRestorePriorAllowsARecoveryAtOrAboveTheFloor(t *testing.T) {
	r := newFloorRig(t, 2)
	prior := r.release(t, "prior-epoch-2", 2)
	result, err, _ := r.recoverTo(t, prior, 2)
	if err == nil || result.Outcome != "failed" || result.RecoveryReceipt == nil {
		t.Fatalf("recovery at the floor = %+v err=%v; want the failed apply recovered", result, err)
	}
	if strings.Contains(err.Error(), "floor") {
		t.Fatalf("a recovery at the floor was refused: %v", err)
	}
	if got := r.current(t); got != prior.Manifest.ContentDigest {
		t.Fatalf("current = %s, want the prior release back", got)
	}
}

// Deploy 1 alone (no writer, no floor): recovery to a pre-deploy-1 release
// stays allowed, as deploy 1 decided (runtime-safe, not rollback-safe).
func TestRestorePriorWithoutAFloorKeepsDeploy1Behaviour(t *testing.T) {
	r := newFloorRig(t, 1)
	prior := r.release(t, "prior-epoch-1", 1)
	result, err, _ := r.recoverTo(t, prior, 0)
	if err == nil || result.RecoveryReceipt == nil || strings.Contains(err.Error(), "floor") {
		t.Fatalf("recovery with no floor = %+v err=%v; want it allowed", result, err)
	}
	if got := r.current(t); got != prior.Manifest.ContentDigest {
		t.Fatalf("current = %s, want the prior release back", got)
	}
}

func TestApplyRefusesBelowTheFloorEvenWhenTheReceiptWasLowered(t *testing.T) {
	// An older binary already ran and rewrote the receipt to 1; the floor,
	// which it never touches, still says 2.
	r := newFloorRig(t, 1)
	r.floor(t, 2)
	old := r.release(t, "old-epoch-1", 1)
	if _, err := r.engine.Apply(context.Background(), old); err == nil || !strings.Contains(err.Error(), "floor") {
		t.Fatalf("apply of an epoch-1 release on a floored store: err=%v, want a refusal naming the floor", err)
	}
	if _, statErr := os.Lstat(filepath.Join(r.root, "releases", old.Manifest.ContentDigest)); !os.IsNotExist(statErr) {
		t.Fatal("release staged despite the floor refusal")
	}
	undeclared := r.release(t, "undeclared", 0)
	if _, err := r.engine.Apply(context.Background(), undeclared); err == nil || !strings.Contains(err.Error(), "floor") {
		t.Fatalf("apply of an undeclared release on a floored store: err=%v, want a refusal", err)
	}
	ok := r.release(t, "at-floor", 2)
	if _, err := r.engine.Apply(context.Background(), ok); err != nil {
		t.Fatalf("apply of a release at the floor: %v", err)
	}
}
