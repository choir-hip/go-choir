package agentcore

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// Gate 2 track D (docs/definitions/choir-appdev-gate2-supervised-self-development-2026-10-09.md):
// self-development is not a special operation. An ordinary engineering
// assignment that freezes a change opens its own promotion candidate.
//
// Ways it can fail:
//  1. A freeze with no candidate opens one while the computer's signed mode
//     is off, so an unarmed computer accumulates promotable changes.
//  2. The opened candidate is not in executing, or not bound to the
//     assignment's trajectory, so freeze's own lookup still refuses it.
//  3. The candidate pins stale heads, so freeze's staleness check refuses.
//  4. A retried freeze opens a second candidate for the same assignment.
//  5. A trajectory that already has a candidate gets another one.
func freezeCandidateTestStore(t *testing.T) (*choirstore.Store, *selfdev.Store, string) {
	t.Helper()
	ctx := context.Background()
	computerID := "computer-freeze-open"
	productStore, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { productStore.Close() })
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signingKey := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "test"}, PrivateKey: privateKey}
	appender, err := computerevent.NewComputerEventAppender(computerID, rollbackTestPinner{signingKey}, productStore, rollbackTestCAS{key: signingKey, projection: productStore}, rollbackTestReceiptVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	genesisID, _ := computerevent.NewEventID()
	genesis := computerevent.Event{SchemaVersion: 1, EventID: genesisID, ComputerID: computerID, EventKind: computerevent.EventGenesisImported, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), IdempotencyKey: "genesis", ActorProfile: "management", AuthorityRef: "owner", PrivacyClass: "owner", PayloadCommitment: strings.Repeat("a", 64), ProposedEffectRef: strings.Repeat("b", 64), ResultingEffectiveCommitment: strings.Repeat("a", 64), ReducerVersion: 1}
	if _, err := appender.AppendNew(ctx, genesis, computerevent.TransitionInput{TargetStateCommitment: strings.Repeat("a", 64)}, nil); err != nil {
		t.Fatal(err)
	}
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	return productStore, operations, computerID
}

func TestFreezeOpensCandidateOnlyWhenModeAuthorizes(t *testing.T) {
	ctx := context.Background()
	_, operations, computerID := freezeCandidateTestStore(t)
	refused := func(context.Context) error { return errors.New("mode is off") }
	if _, err := openFreezeCandidate(ctx, operations, refused, computerID, "trajectory-doc", "engineering:assignment-a", "make an app"); err == nil {
		t.Fatal("candidate opened while the signed mode does not authorize proposals")
	}
	if _, err := operations.GetByTrajectory(ctx, computerID, "trajectory-doc"); err == nil {
		t.Fatal("refused open left a candidate behind")
	}
}

func TestFreezeOpensExecutingCandidateBoundToAssignment(t *testing.T) {
	ctx := context.Background()
	productStore, operations, computerID := freezeCandidateTestStore(t)
	authorized := func(context.Context) error { return nil }
	opened, err := openFreezeCandidate(ctx, operations, authorized, computerID, "trajectory-doc", "engineering:assignment-a", "make an app that does x")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if opened.State != selfdev.StateExecuting || opened.TrajectoryID != "trajectory-doc" {
		t.Fatalf("opened candidate = state %q trajectory %q", opened.State, opened.TrajectoryID)
	}
	head, err := productStore.Head(ctx, computerID)
	if err != nil || head == nil {
		t.Fatalf("head: %v", err)
	}
	if opened.DesiredHead != head.DesiredEventHead || opened.EffectiveHead != head.EffectiveEventHead {
		t.Fatalf("candidate pins stale heads: %+v vs %+v", opened, head)
	}
	found, err := operations.GetByTrajectory(ctx, computerID, "trajectory-doc")
	if err != nil || found.OperationID != opened.OperationID {
		t.Fatalf("freeze lookup by trajectory = %+v, %v", found, err)
	}
	again, err := openFreezeCandidate(ctx, operations, authorized, computerID, "trajectory-doc", "engineering:assignment-a", "make an app that does x")
	if err != nil || again.OperationID != opened.OperationID {
		t.Fatalf("retried open = %+v, %v; want the same candidate", again, err)
	}
	if _, err := openFreezeCandidate(ctx, operations, authorized, computerID, "trajectory-doc", "engineering:assignment-b", "another change"); err == nil {
		t.Fatal("a second candidate opened on a trajectory that already has one")
	}
}
