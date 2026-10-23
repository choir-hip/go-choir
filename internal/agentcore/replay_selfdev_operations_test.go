package agentcore

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
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
	// A fresh probe computer: the platform has published no base, so the
	// probe replays the full chain. The fake serves the chain as it stands
	// when the probe runs.
	rt.restoreBaseSource = &lazyChainBaseSource{cas: cas}
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

// lazyChainBaseSource is restoreBaseFake with no advertised base whose tail is
// whatever the chain holds when asked.
type lazyChainBaseSource struct {
	restoreBaseFake
	cas *derivableChainCAS
}

func (s *lazyChainBaseSource) Watermark(context.Context, string) (uint64, string, error) {
	return 0, "", nil
}

func (s *lazyChainBaseSource) TailPage(ctx context.Context, computerID string, afterSequence uint64, pageSize int) ([]computerevent.DurableEvent, error) {
	s.restoreBaseFake.events = s.cas.snapshot()
	return s.restoreBaseFake.TailPage(ctx, computerID, afterSequence, pageSize)
}

// TestSelfDevelopmentIntentErrorStampIsNotOnTheTapeAndBreaksReplayEquivalence
// pins docs/problems/selfdev-operations-replay-not-equivalent-2026-10-23.md.
// Store.RecordIntentError stamps last_intent_error (and updated_at) with raw
// SQL even when the projection tape is bound, and the operation projection
// (computerevent.SelfDevelopmentOperationProjection, applied by
// store/project.go) has no such column. The live self_development_operations
// table then differs from the table replayed from the tape, so the
// replay-completeness witness reports not_equivalent
// (dolt:texture:table:self_development_operations and
// dolt:texture:content_root) and the computer is replay-ineligible.
//
// The test fails today. It passes when the stamp is on the tape (or the
// column is classified live-only in the witness).
func TestSelfDevelopmentIntentErrorStampIsNotOnTheTapeAndBreaksReplayEquivalence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		reason string
	}{
		{"a refused freeze stamps last_intent_error", "freeze: refused (no verifier refs)"},
		// The success path clears a prior stamp (rlm_reduce.go); the clear is
		// the same raw UPDATE and still moves updated_at off the tape's.
		{"a clearing write moves updated_at off the tape", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			computerID := "computer-selfdev-intent-error-replay"
			rt, operations := selfdevReplayRuntime(t, computerID)

			opened, err := operations.Start(ctx, selfdev.StartRequest{
				ComputerID: computerID, IdempotencyKey: "idem-intent-error",
				PromptArtifactRef: "artifact:sha256:" + strings.Repeat("c", 64),
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := operations.Transition(ctx, computerID, opened.OperationID, selfdev.StateRequested, selfdev.StateExecuting, nil); err != nil {
				t.Fatal(err)
			}

			// Control: operation rows written only through the tape replay
			// equivalent.
			control, err := rt.ReplayCompleteness(ctx, computerID)
			if err != nil {
				t.Fatal(err)
			}
			if !control.Result.Equivalent() {
				t.Fatalf("control: tape-only operation writes are not replay-equivalent: %#v", control.Result.Differences)
			}

			// The defect: the raw-SQL intent stamp, as rlm_reduce.go writes it.
			if err := operations.RecordIntentError(ctx, computerID, opened.OperationID, tc.reason); err != nil {
				t.Fatal(err)
			}
			probe, err := rt.openReplayProbe(ctx, computerID)
			if err != nil {
				t.Fatal(err)
			}
			defer probe.close()
			row := func(db *sql.DB) string {
				var lastIntentError, updatedAt string
				if err := db.QueryRow(`SELECT last_intent_error, CAST(updated_at AS CHAR) FROM self_development_operations WHERE operation_id=?`, opened.OperationID).Scan(&lastIntentError, &updatedAt); err != nil {
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
