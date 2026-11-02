package agentcore

import (
	"bytes"
	"errors"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

func reusedBundle(epoch uint64) transaction.CapsuleEffectBundle {
	return transaction.CapsuleEffectBundle{ReleaseBinary: &capsule.ReleaseBinaryProvenance{
		Origin: capsule.ReleaseBinaryReusedRunning, SHA256: "aa", RunningCommit: strings.Repeat("a", 40), StoreSchemaVersion: epoch,
	}}
}

// The release declares the epoch Freeze recorded on the provenance, not the
// materializing process's own.
func TestPlainSelfDevelopmentReleaseDeclaresTheFrozenStoreEpoch(t *testing.T) {
	frozen := storeschema.Version + 7 // a materializer of another epoch than the frozen binary
	if got := plainReleaseStoreEpoch(reusedBundle(frozen)); got != frozen {
		t.Fatalf("declared epoch = %d, want the frozen %d", got, frozen)
	}
	// A bundle frozen by an older binary carries no field: undeclared.
	if got := plainReleaseStoreEpoch(reusedBundle(0)); got != 0 {
		t.Fatalf("bundle without the field declared %d, want 0", got)
	}
	built := transaction.CapsuleEffectBundle{ReleaseBinary: &capsule.ReleaseBinaryProvenance{Origin: capsule.ReleaseBinaryCapsuleBuilt, StoreSchemaVersion: 6}}
	if got := plainReleaseStoreEpoch(built); got != 0 {
		t.Fatalf("capsule-built release epoch = %d, want 0", got)
	}
	if got := plainReleaseStoreEpoch(transaction.CapsuleEffectBundle{}); got != 0 {
		t.Fatalf("unprovenanced release epoch = %d, want 0", got)
	}
}

// The provenance epoch is covered by the bundle digest.
func TestFrozenStoreEpochIsBoundIntoTheBundleDigest(t *testing.T) {
	a, b := reusedBundle(5), reusedBundle(6)
	da, err := a.ComputeContentDigest()
	if err != nil {
		t.Fatal(err)
	}
	db, err := b.ComputeContentDigest()
	if err != nil {
		t.Fatal(err)
	}
	if da == db {
		t.Fatal("bundle digest does not cover the provenance store epoch")
	}
}

func TestMintRefusesAReusedBinaryThatIsNoLongerRunning(t *testing.T) {
	defer func(prev func() (capsule.RunningBinary, error)) { selfdevRunningBinary = prev }(selfdevRunningBinary)
	bundle := reusedBundle(storeschema.Version)
	commit := bundle.ReleaseBinary.RunningCommit
	selfdevRunningBinary = func() (capsule.RunningBinary, error) {
		return capsule.RunningBinary{SHA256: "aa", Commit: commit}, nil
	}
	if err := checkPlainReleaseBinaryStillRunning(bundle); err != nil {
		t.Fatalf("same binary refused: %v", err)
	}
	selfdevRunningBinary = func() (capsule.RunningBinary, error) {
		return capsule.RunningBinary{SHA256: "bb", Commit: commit}, nil
	}
	err := checkPlainReleaseBinaryStillRunning(bundle)
	if err == nil || !errors.Is(err, updater.ErrApplyRefused) {
		t.Fatalf("changed binary: err=%v, want an apply refusal", err)
	}
}

func TestDelegateUseRebaseAssignmentRecordsUseWithoutTheLogLine(t *testing.T) {
	rt, _ := testRuntime(t)
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	rt.noteDelegateCell("r1", 1, "", "", true)
	use := rt.delegateUseFor("r1", 1, true, func() bool { return true })
	if use == nil || !use.Available || !use.SourcePatch || use.Calls != 0 {
		t.Fatalf("use = %+v", use)
	}
	if strings.Contains(buf.String(), "source patch without delegate") {
		t.Fatalf("rebase assignment logged: %q", buf.String())
	}
}
