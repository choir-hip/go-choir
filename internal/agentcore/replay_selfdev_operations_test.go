package agentcore

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// selfdevReplayRuntime is a runtime shaped like a production computer for the
// replay probe: the store's projection tape is bound, the chain keeps its
// projection batches (so replay can fetch them), and the probe has no
// advertised base so it replays the whole tape.
func selfdevReplayRuntime(t *testing.T, computerID string) (*Runtime, *selfdev.Store) {
	t.Helper()
	ctx := context.Background()
	storePath := filepath.Join(t.TempDir(), "runtime.db")
	live, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = live.Close() })
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signingKey := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "selfdev-replay"}, PrivateKey: privateKey}
	pinner := &derivableTestPinner{key: signingKey}
	cas := &derivableChainCAS{key: signingKey, projection: live}
	appender, err := computerevent.NewComputerEventAppender(computerID, pinner, live, cas, rollbackTestReceiptVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	appender.SetPayloadResolver(pinnedPayloadReader{pinner: pinner}, nil)
	if err := live.BindProjectionTape(computerID, appender); err != nil {
		t.Fatal(err)
	}
	eventID, _ := computerevent.NewEventID()
	commitment := strings.Repeat("a", 64)
	genesis := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: computerID,
		EventKind: computerevent.EventGenesisImported, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: "genesis", ActorProfile: "management", AuthorityRef: "owner", PrivacyClass: "owner",
		PayloadCommitment: commitment, ProposedEffectRef: strings.Repeat("b", 64),
		ResultingEffectiveCommitment: commitment, ReducerVersion: computerevent.ReducerVersionV1,
	}
	if _, err := appender.AppendNew(ctx, genesis, computerevent.TransitionInput{TargetStateCommitment: commitment}, nil); err != nil {
		t.Fatal(err)
	}
	operations, err := selfdev.NewStore(live, live)
	if err != nil {
		t.Fatal(err)
	}
	operations.BindProjectionSink(live)
	rt := &Runtime{
		cfg:           provideriface.Config{ComputerID: computerID, StorePath: storePath},
		store:         live,
		eventAppender: appender,
	}
	// A fresh probe computer: the tape holds no snapshot, so the probe
	// replays the full chain. The fake serves the chain as it stands when the
	// probe runs.
	rt.restoreTape = newRestoreTapeFake(cas.snapshot)
	return rt, operations
}

// pinnedPayloadReader serves the projection batches the pinner recorded, as
// corpusd serves them to a replaying computer.
type pinnedPayloadReader struct{ pinner *derivableTestPinner }

func (r pinnedPayloadReader) FetchPayload(_ context.Context, _, artifactDigest string) ([]byte, error) {
	if payload, ok := r.pinner.fetch(artifactDigest); ok {
		return payload, nil
	}
	return nil, fmt.Errorf("artifact %s is not pinned", artifactDigest)
}

// TestSelfDevelopmentIntentErrorStampIsNotOnTheTapeAndBreaksReplayEquivalence
// pins docs/problems/selfdev-operations-replay-not-equivalent-2026-10-23.md.
// Store.RecordIntentError used to stamp last_intent_error (and updated_at)
// with raw SQL even when the projection tape is bound, and the operation
// projection (computerevent.SelfDevelopmentOperationProjection, applied by
// store/project.go) had no such column. The live self_development_operations
// table then differed from the table replayed from the tape, so the
// replay-completeness witness reported not_equivalent
// (sqlite:texture:table:self_development_operations and
// sqlite:texture:content_root) and the computer was replay-ineligible.
func TestSelfDevelopmentIntentErrorStampIsNotOnTheTapeAndBreaksReplayEquivalence(t *testing.T) {
	for _, tc := range []struct {
		name  string
		steps []string
	}{
		{"a refused freeze stamps last_intent_error", []string{"freeze: refused (no verifier refs)"}},
		// The success path clears the stamp (rlm_reduce.go): a clear after
		// success is a tape write too, so both columns agree again.
		{"a clear after success", []string{"freeze: refused (no verifier refs)", ""}},
		{"a stamp replaced by another reason", []string{"freeze: refused", "verify: bundle digest mismatch"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			computerID := "computer-selfdev-intent-error-replay"
			rt, operations := selfdevReplayRuntime(t, computerID)
			opened := openExecutingOperation(t, ctx, operations, computerID)

			// Control: operation rows written only through the tape replay
			// equivalent.
			control, err := rt.ReplayCompleteness(ctx, computerID)
			if err != nil {
				t.Fatal(err)
			}
			if !control.Result.Equivalent() {
				t.Fatalf("control: tape-only operation writes are not replay-equivalent: %#v", control.Result.Differences)
			}

			for _, reason := range tc.steps {
				if err := operations.RecordIntentError(ctx, computerID, opened.OperationID, reason); err != nil {
					t.Fatal(err)
				}
			}
			probe, err := rt.openReplayProbe(ctx, computerID)
			if err != nil {
				t.Fatal(err)
			}
			defer probe.close()
			row := func(db *sql.DB) string {
				var lastIntentError, updatedAt string
				if err := db.QueryRow(`SELECT last_intent_error, CAST(updated_at AS TEXT) FROM self_development_operations WHERE operation_id=?`, opened.OperationID).Scan(&lastIntentError, &updatedAt); err != nil {
					t.Fatal(err)
				}
				return fmt.Sprintf("last_intent_error=%q updated_at=%s", lastIntentError, updatedAt)
			}
			t.Logf("live   row: %s", row(rt.store.DB()))
			t.Logf("replay row: %s", row(probe.replayStore.DB()))
			if probe.report.Result.Equivalent() {
				return
			}
			var keys []string
			for _, difference := range probe.report.Result.Differences {
				keys = append(keys, difference.Key)
			}
			t.Fatalf("a RecordIntentError write made the replay not equivalent: differences %v; eligibility %+v", keys, probe.report.Eligibility)
		})
	}
}

// An unchanged reason is no event: a refusal that retries with the same text
// must not grow the tape, and must not move updated_at (idle time keeps its
// meaning for a retry that changes nothing). A new reason is one event.
func TestSelfDevelopmentIntentErrorUnchangedReasonAppendsNothing(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-selfdev-intent-error-unchanged"
	rt, operations := selfdevReplayRuntime(t, computerID)
	opened := openExecutingOperation(t, ctx, operations, computerID)
	head := func() uint64 {
		h, err := rt.store.Head(ctx, computerID)
		if err != nil || h == nil {
			t.Fatalf("head = %+v, %v", h, err)
		}
		return h.Sequence
	}

	// A clear on a row with no stamp is no change either.
	before := head()
	if err := operations.RecordIntentError(ctx, computerID, opened.OperationID, ""); err != nil {
		t.Fatal(err)
	}
	if got := head(); got != before {
		t.Fatalf("clearing an empty stamp appended events: sequence %d -> %d", before, got)
	}

	if err := operations.RecordIntentError(ctx, computerID, opened.OperationID, "freeze: refused"); err != nil {
		t.Fatal(err)
	}
	stamped := head()
	if stamped != before+1 {
		t.Fatalf("a new reason appended %d events, want 1", stamped-before)
	}
	first, err := operations.Get(ctx, computerID, opened.OperationID)
	if err != nil || first.LastIntentError != "freeze: refused" || first.State != selfdev.StateExecuting {
		t.Fatalf("stamped operation = %+v, %v", first, err)
	}
	if first.UpdatedAt == opened.UpdatedAt {
		t.Fatalf("a new stamp left updated_at at %s; the stamp keeps setting it", first.UpdatedAt)
	}

	if err := operations.RecordIntentError(ctx, computerID, opened.OperationID, "  freeze: refused  "); err != nil {
		t.Fatal(err)
	}
	if got := head(); got != stamped {
		t.Fatalf("an unchanged reason appended events: sequence %d -> %d", stamped, got)
	}
	again, err := operations.Get(ctx, computerID, opened.OperationID)
	if err != nil || again.UpdatedAt != first.UpdatedAt {
		t.Fatalf("an unchanged reason moved updated_at %s -> %s (%v)", first.UpdatedAt, again.UpdatedAt, err)
	}

	// A missing operation is still not found.
	if err := operations.RecordIntentError(ctx, computerID, "selfdev-missing", "x"); !errors.Is(err, selfdev.ErrOperationNotFound) {
		t.Fatalf("missing operation error = %v, want ErrOperationNotFound", err)
	}
}

func openExecutingOperation(t *testing.T, ctx context.Context, operations *selfdev.Store, computerID string) selfdev.Operation {
	t.Helper()
	opened, err := operations.Start(ctx, selfdev.StartRequest{
		ComputerID: computerID, IdempotencyKey: "idem-intent-error",
		PromptArtifactRef: "artifact:sha256:" + strings.Repeat("c", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	executing, err := operations.Transition(ctx, computerID, opened.OperationID, selfdev.StateRequested, selfdev.StateExecuting, nil)
	if err != nil {
		t.Fatal(err)
	}
	return executing
}

// RematerializeFromTape reopens the flipped realization on the live store
// pointer (rematerialize.go:262). After it, the store must still route
// object-graph writes through the tape (Store.Reopen carries the mutation
// interceptor over), and the rebound operation store must still stamp through
// the tape, so a replay of the reopened computer is equivalent.
func TestRematerializeFromTapeKeepsObjectGraphAndOperationWritesOnTheTape(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-rematerialize-tape-writes"
	rt, _ := selfdevReplayRuntime(t, computerID)
	live := rt.store
	setting := func(owner string) {
		t.Helper()
		if err := live.PutComputerSetting(ctx, choirstore.ComputerSetting{
			OwnerID: owner, ComputerID: computerID, Key: choirstore.SettingIntakeLabels, Value: true,
		}); err != nil {
			t.Fatal(err)
		}
	}
	setting("owner-before")

	updaterRoot := filepath.Join(t.TempDir(), "updater")
	priorDigest, _ := pinFrontendRelease(t, updaterRoot, computerID, "<html>live</html>")
	targetDigest, targetIdentity := pinFrontendRelease(t, updaterRoot, computerID, "<html>checkpoint</html>")
	pointCurrent(t, updaterRoot, priorDigest)
	rt.selfdevUpdaterRoot = updaterRoot
	report, err := rt.ReplayCompleteness(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Result.Equivalent() {
		t.Fatalf("control: before the rematerialize the replay is not equivalent: %#v", report.Result.Differences)
	}
	witness, err := selfdevprotocol.WitnessFromObservationSets(report.Live, report.Replay, report.Result)
	if err != nil {
		t.Fatal(err)
	}
	head, err := live.Head(ctx, computerID)
	if err != nil || head == nil {
		t.Fatalf("head: %v %#v", err, head)
	}
	checkpoint := rematerializeTestCheckpoint(t, computerID, witness, targetDigest, targetIdentity, head.CanonicalEventHead)
	if _, err := rt.RematerializeFromTape(ctx, computerID, checkpoint); err != nil {
		t.Fatalf("rematerialize: %v", err)
	}

	sequence := func() uint64 {
		t.Helper()
		h, err := live.Head(ctx, computerID)
		if err != nil || h == nil {
			t.Fatalf("head: %v %#v", err, h)
		}
		return h.Sequence
	}
	if !live.ProjectionTapeBound() {
		t.Fatal("the reopened store lost its projection tape")
	}
	before := sequence()
	setting("owner-after")
	if got := sequence(); got != before+1 {
		t.Fatalf("an object-graph write after the rematerialize moved the tape %d -> %d, want one event", before, got)
	}
	// The rematerialize cut the staged workspace over before the flip, so the
	// reopened store is post-cutover and its vocabulary fence must be armed.
	agent := func(role string) types.AgentRecord {
		return types.AgentRecord{AgentID: "agent-fence-" + role, OwnerID: "owner-fence", ComputerID: computerID, Profile: role, Role: role, ChannelID: "channel-fence"}
	}
	if err := live.UpsertAgentOG(ctx, agent("boss")); err == nil {
		t.Fatal("after the rematerialize the vocabulary write fence accepted an unknown role")
	}
	if err := live.UpsertAgentOG(ctx, agent("engineering")); err != nil {
		t.Fatalf("after the rematerialize the fence refused a live role: %v", err)
	}
	if rt.selfdevOperations == nil {
		t.Fatal("rematerialize left no operation store")
	}
	opened := openExecutingOperation(t, ctx, rt.selfdevOperations, computerID)
	if err := rt.selfdevOperations.RecordIntentError(ctx, computerID, opened.OperationID, "freeze: refused after rematerialize"); err != nil {
		t.Fatal(err)
	}
	after, err := rt.ReplayCompleteness(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	if !after.Result.Equivalent() {
		t.Fatalf("after the rematerialize the reopened computer is not replay-equivalent: %#v", after.Result.Differences)
	}
}

// The reason is cut at 512 bytes and the cut can land inside a multi-byte
// rune; a model-written reason can also carry invalid bytes. The stored value
// is valid UTF-8 and equals what a repeat of the same text produces, so the
// repeat appends nothing.
func TestSelfDevelopmentIntentErrorReasonIsValidUTF8AfterTheCutAndRepeatsAppendNothing(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-selfdev-intent-error-utf8"
	rt, operations := selfdevReplayRuntime(t, computerID)
	opened := openExecutingOperation(t, ctx, operations, computerID)
	head := func() uint64 {
		h, err := rt.store.Head(ctx, computerID)
		if err != nil || h == nil {
			t.Fatalf("head = %+v, %v", h, err)
		}
		return h.Sequence
	}
	// 511 ASCII bytes then a 2-byte rune: byte 512 is half of the rune.
	cut := strings.Repeat("x", 511) + "\u00e9 tail"
	invalid := "freeze: bad \xff\xfe bytes"
	for name, reason := range map[string]string{"cut inside a rune": cut, "invalid bytes": invalid} {
		before := head()
		if err := operations.RecordIntentError(ctx, computerID, opened.OperationID, reason); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		stamped, err := operations.Get(ctx, computerID, opened.OperationID)
		if err != nil {
			t.Fatal(err)
		}
		if !utf8.ValidString(stamped.LastIntentError) || len(stamped.LastIntentError) > 512 || stamped.LastIntentError == "" {
			t.Fatalf("%s: stored reason is not valid UTF-8 within 512 bytes: %q", name, stamped.LastIntentError)
		}
		if head() != before+1 {
			t.Fatalf("%s: a new reason appended %d events, want 1", name, head()-before)
		}
		mid := head()
		if err := operations.RecordIntentError(ctx, computerID, opened.OperationID, reason); err != nil {
			t.Fatal(err)
		}
		if head() != mid {
			t.Fatalf("%s: a repeat of the same reason appended %d events", name, head()-mid)
		}
	}
	report, err := rt.ReplayCompleteness(ctx, computerID)
	if err != nil || !report.Result.Equivalent() {
		t.Fatalf("replay after the cut stamps: %v %#v", err, report.Result.Differences)
	}
}
