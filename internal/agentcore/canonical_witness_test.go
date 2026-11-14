package agentcore

// The canonical witness (witness version 4), written before the code. Ways
// this could fail (docs/problems/restore-cannot-finish-inside-its-ten-minute-request-2026-11-14.md):
//
//  1. a checkpoint bound on a store whose index holds null pin lists, or a
//     trim backlog, is refused by its own restore (the strict witness read the
//     raw spelling and the raw backlog);
//  2. the canonical form hides a row it must keep: a row above the boundary,
//     a batch row that is not finalized, a row of another kind, a missing
//     row;
//  3. the boundary the checkpoint names is not the one its head defines and
//     the restore believes it;
//  4. a checkpoint in the strict form (version 3) is compared loosely;
//  5. a witness mismatch, or a witness version this release does not judge,
//     answers 500 instead of a refusal;
//  6. the probe's equivalence and the restore's witness disagree.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// boundWitness is the witness the production bind records for the runtime's
// live store at its head (liveCheckpointWitness), not the probe's report.
func boundWitness(t *testing.T, rt *Runtime, computerID string) selfdevprotocol.VMLocalContentWitness {
	t.Helper()
	_, _, w, _, err := rt.liveCheckpointWitness(context.Background(), computerID, true)
	if err != nil {
		t.Fatalf("bind witness: %v", err)
	}
	return w
}

// canonicalLiveWitness is the canonical witness of the runtime's live store at
// its current head, read outside the bind (what a bind at that head records).
func canonicalLiveWitness(ctx context.Context, rt *Runtime) (selfdevprotocol.VMLocalContentWitness, error) {
	var empty selfdevprotocol.VMLocalContentWitness
	head, err := rt.store.Head(ctx, rt.cfg.ComputerID)
	if err != nil || head == nil {
		return empty, fmt.Errorf("live head: %v", err)
	}
	digests, boundary, err := canonicalSnapshotDigests(ctx, rt.store.ReadDB(), func(context.Context, *sql.Tx) (uint64, error) { return head.Sequence, nil })
	if err != nil {
		return empty, err
	}
	return selfdevprotocol.WitnessFromDigestsCanonical(digests, boundary)
}

func openIndexStore(t *testing.T) *choirstore.Store {
	t.Helper()
	s, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func insertIndexRow(t *testing.T, s *choirstore.Store, computerID, digest string, seq int, kind, status, pins string) {
	t.Helper()
	_, err := s.DB().Exec(`INSERT INTO computer_event_index (event_digest, computer_id, sequence, previous_head, event_kind, event_artifact_digest, event_pin_receipt_digest, payload_pin_receipt_digests_json, request_commitment, idempotency_key, status, next_desired_event_head, next_effective_event_head, next_desired_state_commitment, next_effective_state_commitment, next_reducer_version, next_credential_revocation_epoch, prepared_at)
		VALUES (?, ?, ?, 'p', ?, 'a', 'b', ?, 'r', ?, ?, 'd', 'e', 'c', 'f', 1, 0, ?)`, digest, computerID, seq, kind, pins, "key-"+digest, status, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
}

const canonicalTestHead = 50000 // trim boundary 40000

func bindAt(t *testing.T, s *choirstore.Store, head uint64) selfdevprotocol.VMLocalContentWitness {
	t.Helper()
	digests, boundary, err := canonicalSnapshotDigests(context.Background(), s.ReadDB(), func(context.Context, *sql.Tx) (uint64, error) { return head, nil })
	if err != nil {
		t.Fatal(err)
	}
	w, err := selfdevprotocol.WitnessFromDigestsCanonical(digests, boundary)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func restoreGate(t *testing.T, staged *choirstore.Store, want selfdevprotocol.VMLocalContentWitness, head uint64) error {
	t.Helper()
	got, err := reconstructedWitness(context.Background(), staged.ReadDB(), want, head)
	if err != nil {
		return err
	}
	return selfdevprotocol.WitnessContentMatches(got, want)
}

// A live store with null pin lists and a trim backlog, bound; a staged store
// the way a snapshot-and-replay builds it (no backlog, "[]"): the witness
// matches. The strict form of the same two stores does not.
func TestCanonicalWitnessBindThenRestoreGateOnNullAndTrimBacklog(t *testing.T) {
	batch := string(computerevent.EventProjectionBatchRecorded)
	live, staged := openIndexStore(t), openIndexStore(t)
	insertIndexRow(t, live, "c", "kept", 45000, "desk", "finalized", "null")
	insertIndexRow(t, staged, "c", "kept", 45000, "desk", "finalized", "[]")
	insertIndexRow(t, live, "c", "backlog", 100, batch, "finalized", "[]")
	insertIndexRow(t, live, "c", "window", 49000, batch, "finalized", "[]") // above the boundary: kept
	insertIndexRow(t, staged, "c", "window", 49000, batch, "finalized", "[]")

	strictLive, err := LiveStoreWitness(context.Background(), live.ReadDB())
	if err != nil {
		t.Fatal(err)
	}
	if err := restoreGate(t, staged, strictLive, canonicalTestHead); err == nil {
		t.Fatal("precondition: the strict witness must refuse these two stores")
	}

	want := bindAt(t, live, canonicalTestHead)
	if want.WitnessVersion != selfdevprotocol.WitnessVersionCanonical || want.EventIndexTrimBoundary != 40000 {
		t.Fatalf("bound witness = version %d boundary %d", want.WitnessVersion, want.EventIndexTrimBoundary)
	}
	if err := restoreGate(t, staged, want, canonicalTestHead); err != nil {
		t.Fatalf("a restore of a canonical checkpoint refused equivalent stores: %v", err)
	}
}

// What the canonical form must not hide.
func TestCanonicalWitnessRefusesRealDifferences(t *testing.T) {
	batch := string(computerevent.EventProjectionBatchRecorded)
	build := func(extra func(s *choirstore.Store)) *choirstore.Store {
		s := openIndexStore(t)
		insertIndexRow(t, s, "c", "kept", 45000, "desk", "finalized", "[]")
		insertIndexRow(t, s, "c", "window", 49000, batch, "finalized", "[]")
		if extra != nil {
			extra(s)
		}
		return s
	}
	want := bindAt(t, build(nil), canonicalTestHead)
	cases := map[string]func(s *choirstore.Store){
		"an extra row above the boundary": func(s *choirstore.Store) { insertIndexRow(t, s, "c", "extra", 46000, "desk", "finalized", "[]") },
		"an extra batch row above the boundary": func(s *choirstore.Store) {
			insertIndexRow(t, s, "c", "extra-batch", 41000, batch, "finalized", "[]")
		},
		"a prepared batch row at the boundary": func(s *choirstore.Store) {
			insertIndexRow(t, s, "c", "prep", 40000, batch, "prepared", "[]")
		},
		"a finalized row of another kind below the boundary": func(s *choirstore.Store) {
			insertIndexRow(t, s, "c", "old-desk", 100, "desk", "finalized", "[]")
		},
		"a changed column on a kept row": func(s *choirstore.Store) {
			if _, err := s.DB().Exec(`UPDATE computer_event_index SET next_reducer_version = 99 WHERE event_digest = 'kept'`); err != nil {
				t.Fatal(err)
			}
		},
		"a non-null pin list difference": func(s *choirstore.Store) {
			if _, err := s.DB().Exec(`UPDATE computer_event_index SET payload_pin_receipt_digests_json = '["x"]' WHERE event_digest = 'kept'`); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, extra := range cases {
		t.Run(name, func(t *testing.T) {
			err := restoreGate(t, build(extra), want, canonicalTestHead)
			if !errors.Is(err, selfdevprotocol.ErrWitnessMismatch) {
				t.Fatalf("err = %v, want a witness mismatch", err)
			}
		})
	}
	t.Run("a missing row above the boundary", func(t *testing.T) {
		s := openIndexStore(t)
		insertIndexRow(t, s, "c", "kept", 45000, "desk", "finalized", "[]")
		if err := restoreGate(t, s, want, canonicalTestHead); !errors.Is(err, selfdevprotocol.ErrWitnessMismatch) {
			t.Fatalf("err = %v, want a witness mismatch", err)
		}
	})
}

// The boundary is committed and checked against the checkpoint's head; a
// larger one would hide more rows.
func TestCanonicalWitnessBoundaryIsTheHeadsAndIsCommitted(t *testing.T) {
	s := openIndexStore(t)
	insertIndexRow(t, s, "c", "kept", 45000, "desk", "finalized", "[]")
	want := bindAt(t, s, canonicalTestHead)
	if err := restoreGate(t, s, want, canonicalTestHead+10000); !errors.Is(err, selfdevprotocol.ErrWitnessMismatch) {
		t.Fatalf("a head whose boundary is not the checkpoint's: %v", err)
	}
	lie := want
	lie.EventIndexTrimBoundary = 49000
	if err := lie.Validate(); err == nil {
		t.Fatal("a boundary changed after the fact still validates: it is not inside the committed digest")
	}
	if err := restoreGate(t, s, lie, canonicalTestHead); err == nil {
		t.Fatal("restore accepted a witness with an altered boundary")
	}
}

// A strict checkpoint (version 3) is still compared strictly.
func TestStrictCheckpointsStayStrict(t *testing.T) {
	live, staged := openIndexStore(t), openIndexStore(t)
	insertIndexRow(t, live, "c", "kept", 45000, "desk", "finalized", "null")
	insertIndexRow(t, staged, "c", "kept", 45000, "desk", "finalized", "[]")
	strictLive, err := LiveStoreWitness(context.Background(), live.ReadDB())
	if err != nil {
		t.Fatal(err)
	}
	if strictLive.WitnessVersion != selfdevprotocol.WitnessVersionSQLite {
		t.Fatalf("version = %d", strictLive.WitnessVersion)
	}
	if err := restoreGate(t, staged, strictLive, canonicalTestHead); !errors.Is(err, selfdevprotocol.ErrWitnessMismatch) {
		t.Fatalf("a strict checkpoint was compared loosely: %v", err)
	}
	// Identical stores still match strictly.
	same := openIndexStore(t)
	insertIndexRow(t, same, "c", "kept", 45000, "desk", "finalized", "null")
	if err := restoreGate(t, same, strictLive, canonicalTestHead); err != nil {
		t.Fatalf("a strict checkpoint refused an identical store: %v", err)
	}
}

// The filter is not scoped by computer_id; a second computer is refused.
func TestCanonicalWitnessRefusesAStoreOfTwoComputers(t *testing.T) {
	s := openIndexStore(t)
	insertIndexRow(t, s, "c", "one", 45000, "desk", "finalized", "[]")
	insertIndexRow(t, s, "other", "two", 45001, "desk", "finalized", "[]")
	_, _, err := canonicalSnapshotDigests(context.Background(), s.ReadDB(), func(context.Context, *sql.Tx) (uint64, error) { return canonicalTestHead, nil })
	if err == nil || !strings.Contains(err.Error(), "computers") {
		t.Fatalf("err = %v, want the one-computer refusal", err)
	}
}

func TestRematerializeRefusalStatusAnswersWitnessRefusalsAs409(t *testing.T) {
	for name, err := range map[string]error{
		"mismatch": errors.Join(errors.New("rematerialize: x"), selfdevprotocol.ErrWitnessMismatch),
		"version":  selfdevprotocol.RefuseWitnessVersion(9),
	} {
		if got := rematerializeRefusalStatus(err); got != http.StatusConflict {
			t.Fatalf("%s: status %d, want 409", name, got)
		}
	}
	if got := rematerializeRefusalStatus(errors.New("disk on fire")); got != http.StatusInternalServerError {
		t.Fatalf("an internal error answered %d", got)
	}
}

// End to end: a live store holding a null pin list is bound by the production
// bind; the restore (snapshot, replay, witness gate) accepts it. A live store
// with a real difference is refused, with a mismatch.
func TestRestoreAcceptsABoundCheckpointOfAStoreWithNullPinLists(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  string
		wantErr bool
	}{
		{"null pin list on a kept row", `UPDATE computer_event_index SET payload_pin_receipt_digests_json = 'null'`, false},
		{"a real difference", `UPDATE computer_event_index SET next_reducer_version = 99`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			computerID := "computer-canonical-restore-" + strings.ReplaceAll(tc.name, " ", "-")
			storePath := filepath.Join(t.TempDir(), "runtime.db")
			live, err := choirstore.Open(storePath)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if live != nil {
					_ = live.Close()
				}
			})
			rt, cas, acceptedHead := rematerializeTapeRuntime(t, computerID, storePath, live)
			updaterRoot := filepath.Join(t.TempDir(), "updater")
			priorDigest, _ := pinFrontendRelease(t, updaterRoot, computerID, "<html>live</html>")
			targetDigest, targetIdentity := pinFrontendRelease(t, updaterRoot, computerID, "<html>checkpoint</html>")
			pointCurrent(t, updaterRoot, priorDigest)
			rt.selfdevUpdaterRoot = updaterRoot
			ctx := context.Background()
			seedRestoreBase(t, ctx, rt, cas, computerID, 1)
			// The live store drifts from its own base after the snapshot.
			if _, err := live.DB().ExecContext(ctx, tc.mutate); err != nil {
				t.Fatal(err)
			}
			witness := boundWitness(t, rt, computerID)
			if witness.WitnessVersion != selfdevprotocol.WitnessVersionCanonical {
				t.Fatalf("the production bind produced version %d", witness.WitnessVersion)
			}
			checkpoint := rematerializeTestCheckpoint(t, computerID, witness, targetDigest, targetIdentity, acceptedHead)
			report, err := rt.RematerializeFromTape(ctx, computerID, checkpoint)
			if tc.wantErr {
				if !errors.Is(err, selfdevprotocol.ErrWitnessMismatch) {
					t.Fatalf("a real difference: err = %v, want a witness mismatch", err)
				}
				if got := rematerializeRefusalStatus(err); got != http.StatusConflict {
					t.Fatalf("status %d, want 409", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("restore refused its own bound checkpoint: %v", err)
			}
			live = nil
			t.Cleanup(func() { _ = rt.store.Close() })
			if !report.WitnessMatched {
				t.Fatalf("report = %#v", report)
			}
		})
	}
}

// The probe and the witness agree: on the e2e fixture where the probe says
// equivalent, the probe's own live digests are the bound witness's.
func TestProbeEquivalentAndBoundWitnessAreOneComputation(t *testing.T) {
	fx := newProbeFixture(t, "computer-probe-bound-agree", true)
	report, err := fx.rt.ReplayCompleteness(context.Background(), "computer-probe-bound-agree")
	if err != nil || !report.Result.Equivalent() {
		t.Fatalf("probe: %v %v", err, report.Result.Differences)
	}
	bound := boundWitness(t, fx.rt, "computer-probe-bound-agree")
	fromProbe, err := selfdevprotocol.WitnessFromObservationSet(report.Live)
	if err != nil {
		t.Fatal(err)
	}
	if bound.ContentRoot != fromProbe.ContentRoot {
		t.Fatal("the probe's live content root is not the bound witness's: the probe and the restore gate disagree")
	}
}

// The raw column, not the probe's reading of it: a replayed no-payload event
// stores "[]" (red without the Prepare change alone).
func TestReplayStoreWritesEmptyPinListNotNull(t *testing.T) {
	fx := newProbeFixture(t, "computer-replay-raw-pins", true)
	probe, err := fx.rt.openReplayProbe(context.Background(), "computer-replay-raw-pins")
	if err != nil {
		t.Fatal(err)
	}
	defer probe.close()
	var raw string
	if err := probe.replayStore.DB().QueryRow(`SELECT payload_pin_receipt_digests_json FROM computer_event_index WHERE sequence = 2`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if raw != "[]" {
		t.Fatalf("the replay stored %q for an event with no payload pins, want []", raw)
	}
}
