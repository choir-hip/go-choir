package agentcore

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// TestFreezeAcceptsAdvancedBookkeepingHead replays the M11 wedge: the
// operation pins BaseHead at trajectory_started's PreviousHead, then its own
// trajectory event (and, on a live tape, every projection batch) advances
// canonical_event_head. Freeze must still pass the head gate: the pin is a
// state surface (desired/effective/pending), not a head digest.
func TestFreezeAcceptsAdvancedBookkeepingHead(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-freeze-drift"
	productStore, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer productStore.Close()
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
	baseHead, err := productStore.Head(ctx, computerID)
	if err != nil || baseHead == nil {
		t.Fatal("genesis head unavailable")
	}
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	operation, err := operations.Start(ctx, selfdev.StartRequest{
		ComputerID: computerID, IdempotencyKey: "freeze-drift-op",
		PromptArtifactRef: "artifact:sha256:" + strings.Repeat("c", 64),
		BaseHead:          baseHead.CanonicalEventHead,
	})
	if err != nil {
		t.Fatal(err)
	}
	// The operation's own trajectory event lands on top of the pin — exactly
	// what the live tape does before the desk's first cell runs.
	trajID, _ := computerevent.NewEventID()
	traj := computerevent.Event{SchemaVersion: 1, EventID: trajID, ComputerID: computerID, EventKind: computerevent.EventTrajectoryStarted, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), IdempotencyKey: "selfdev-start-test", RequestCommitment: strings.Repeat("d", 64), TrajectoryID: operation.TrajectoryID, ActorProfile: "management", AuthorityRef: "platform-control:selfdev", PrivacyClass: "owner", PayloadCommitment: strings.Repeat("a", 64), ProposedEffectRef: strings.Repeat("b", 64), ResultingEffectiveCommitment: strings.Repeat("a", 64), ReducerVersion: 1}
	if _, err := appender.AppendNew(ctx, traj, computerevent.TransitionInput{}, nil); err != nil {
		t.Fatal(err)
	}
	head, err := productStore.Head(ctx, computerID)
	if err != nil || head.CanonicalEventHead == operation.BaseHead {
		t.Fatal("canonical head did not advance past the operation pin")
	}
	if _, err := operations.Transition(ctx, computerID, operation.OperationID, selfdev.StateRequested, selfdev.StateExecuting, nil); err != nil {
		t.Fatal(err)
	}
	rec := &types.RunRecord{TrajectoryID: operation.TrajectoryID}
	toolCtx := &CapsuleToolCtx{
		ComputerID:         computerID,
		UpdaterRoot:        t.TempDir(),
		Executor:           capsule.NewExecutor(t.TempDir(), t.TempDir(), "", 0),
		OperationStore:     operations,
		EventProjection:    freezeTestProjection{store: productStore},
		EventAppender:      appender,
		TransactionBuilder: transaction.NewTransactionBuilder(transaction.NewClassifier()),
	}
	freeze := func() error {
		_, freezeErr := freezeCapsuleEffectBundle(ctx, toolCtx, rec, "handle-1",
			"recipe:sha256:"+strings.Repeat("e", 64),
			[]string{"test:sha256:" + strings.Repeat("f", 64)},
			[]string{"toolchain:sha256:" + strings.Repeat("1", 64)})
		return freezeErr
	}
	if err := freeze(); err != nil && strings.Contains(err.Error(), "base head unavailable, stale, or pending") {
		t.Fatalf("freeze refused on pass-through head advancement: %v", err)
	}
	// A competing state transition (researcher_update moves desired AND
	// effective) between pin and freeze must still refuse.
	updateID, _ := computerevent.NewEventID()
	update := computerevent.Event{SchemaVersion: 1, EventID: updateID, ComputerID: computerID, EventKind: computerevent.EventResearchUpdate, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), IdempotencyKey: "competing-update", ActorProfile: "research", AuthorityRef: "typed-update", PrivacyClass: "owner", PayloadCommitment: strings.Repeat("a", 64), ResultingEffectiveCommitment: strings.Repeat("2", 64), ReducerVersion: 1}
	if _, err := appender.AppendNew(ctx, update, computerevent.TransitionInput{TargetStateCommitment: strings.Repeat("2", 64)}, nil); err != nil {
		t.Fatal(err)
	}
	if err := freeze(); err == nil || !strings.Contains(err.Error(), "base head unavailable, stale, or pending") {
		t.Fatalf("freeze must refuse after a competing transition moved the state surface, got %v", err)
	}
}

type freezeTestProjection struct{ store *choirstore.Store }

func (p freezeTestProjection) Head(ctx context.Context, computerID string) (*computerevent.Head, error) {
	return p.store.Head(ctx, computerID)
}
func (p freezeTestProjection) EventByIdempotency(ctx context.Context, computerID, idempotencyKey string) (computerevent.Event, bool, error) {
	return p.store.EventByIdempotency(ctx, computerID, idempotencyKey)
}
