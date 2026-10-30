package agentcore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/recoveryplan"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/snapshot"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// restoreTape is what recovery reads: the computer's own tape, and the sealed
// snapshot chunks the tape names (snapshot v1, internal/snapshot). Projection
// bases are gone; a snapshot plus the tape tail is the only start point other
// than genesis.
type restoreTape interface {
	snapshot.Source
	snapshot.ChunkStore
}

// platformRestoreTape reads the tape and the chunks over the guest's
// authenticated connection to the platform.
type platformRestoreTape struct {
	events *computerevent.HTTPClient
	chunks snapshot.HTTPChunks
}

func (p platformRestoreTape) EventsPage(ctx context.Context, computerID string, after uint64, size int) ([]computerevent.DurableEvent, error) {
	return p.events.EventsPage(ctx, computerID, after, size)
}

func (p platformRestoreTape) FetchPayload(ctx context.Context, computerID, digest string) ([]byte, error) {
	return p.events.FetchPayload(ctx, computerID, digest)
}

func (p platformRestoreTape) PutChunk(ctx context.Context, digest string, sealed []byte) error {
	return p.chunks.PutChunk(ctx, digest, sealed)
}

func (p platformRestoreTape) GetChunk(ctx context.Context, digest string) ([]byte, error) {
	return p.chunks.GetChunk(ctx, digest)
}

// resolveRestoreTape returns the recovery source. An injected fake wins for
// tests; otherwise the platform source is built from the event client the
// runtime already holds. No source means no recovery: callers refuse.
func (rt *Runtime) resolveRestoreTape() (restoreTape, error) {
	if rt == nil || rt.eventAppender == nil {
		return nil, fmt.Errorf("%w: event projection authority is not configured", ErrRematerializeUnavailable)
	}
	if rt.restoreTape != nil {
		return rt.restoreTape, nil
	}
	client, ok := rt.eventPayloadReader.(*computerevent.HTTPClient)
	if !ok || client == nil {
		return nil, fmt.Errorf("%w: restore authority is not configured", ErrRematerializeUnavailable)
	}
	return platformRestoreTape{
		events: client,
		chunks: snapshot.HTTPChunks{BaseURL: client.BaseURL(), ComputerID: strings.TrimSpace(rt.cfg.ComputerID), Capability: client.Capability()},
	}, nil
}

// restorePlan is how a restore reaches its target head: install the newest
// usable snapshot at or below the target and replay the tape after it, or
// replay from genesis when the tape holds none and the chain is short.
type restorePlan struct {
	sequence   uint64
	candidates []snapshot.Candidate
}

// planRestoreTarget resolves the target head to a sequence on the canonical
// chain and finds the snapshots at or below it, in one pass over the tape.
// The tape decides: a target that is not on the chain refuses, and a chain
// with no snapshot longer than the replay bound refuses rather than
// lifetime-replaying.
func planRestoreTarget(ctx context.Context, tape restoreTape, computerID, targetHead string) (restorePlan, error) {
	computerID = strings.TrimSpace(computerID)
	targetHead = strings.ToLower(strings.TrimSpace(targetHead))
	if !computerevent.IsSHA256(targetHead) || tape == nil || computerID == "" {
		return restorePlan{}, fmt.Errorf("%w: target resolution requires computer, source and head", recoveryplan.ErrBaseRefused)
	}
	var target uint64
	var found bool
	candidates, err := snapshot.Scan(ctx, tape, computerID, func(d computerevent.DurableEvent) bool {
		if d.Request.Next.CanonicalEventHead == targetHead {
			target, found = d.Request.Event.Sequence, true
			return true
		}
		return false
	})
	if err != nil {
		return restorePlan{}, err
	}
	if !found {
		return restorePlan{}, fmt.Errorf("%w: rematerialize: recovery target is not on the canonical chain", recoveryplan.ErrBaseRefused)
	}
	if len(candidates) == 0 && target > recoveryplan.MaxRecoveryTailEvents {
		return restorePlan{}, fmt.Errorf("%w: no snapshot at or below recovery target %d, which exceeds the %d-event replay bound", recoveryplan.ErrBaseRefused, target, recoveryplan.MaxRecoveryTailEvents)
	}
	return restorePlan{sequence: target, candidates: candidates}, nil
}

// openStagedStore opens the store replay will reconstruct into, under
// stagingRoot: the newest snapshot installed when the plan has one, a fresh
// store otherwise. The returned record is zero for a genesis replay.
func (rt *Runtime) openStagedStore(ctx context.Context, tape restoreTape, stagingRoot, markerName, computerID string, plan restorePlan) (*choirstore.Store, snapshot.Record, error) {
	marker := filepath.Join(stagingRoot, markerName)
	if len(plan.candidates) == 0 {
		staged, err := choirstore.OpenFresh(marker)
		if err != nil {
			return nil, snapshot.Record{}, fmt.Errorf("rematerialize: open staged workspace: %w", err)
		}
		return staged, snapshot.Record{}, nil
	}
	if rt.privateArtifactCipher == nil {
		return nil, snapshot.Record{}, fmt.Errorf("%w: private artifact cipher is not configured", ErrRematerializeUnavailable)
	}
	record, err := snapshot.Install(ctx, snapshot.RestoreConfig{
		ComputerID:  computerID,
		DestPath:    choirstore.StateDBPath(marker),
		Source:      tape,
		Chunks:      tape,
		Cipher:      rt.privateArtifactCipher,
		MaxSequence: plan.sequence,
		Verify:      verifySnapshotFile,
	}, plan.candidates)
	if err != nil {
		return nil, snapshot.Record{}, fmt.Errorf("%w: install snapshot: %v", recoveryplan.ErrBaseRefused, err)
	}
	staged, err := choirstore.Open(marker)
	if err != nil {
		return nil, snapshot.Record{}, fmt.Errorf("rematerialize: open staged snapshot: %w", err)
	}
	return staged, record, nil
}

// verifySnapshotFile checks the unsealed file against the witness the
// snapshot recorded, before it is installed.
func verifySnapshotFile(ctx context.Context, record snapshot.Record, path string) error {
	db, err := dbx.OpenSQLite(path, dbx.SQLiteOptions{ReadConns: 1})
	if err != nil {
		return fmt.Errorf("open unsealed snapshot: %w", err)
	}
	defer db.Close()
	witness, err := LiveStoreWitness(ctx, db.Read)
	if err != nil {
		return err
	}
	if witness.ContentRoot != record.WitnessRoot || witness.SchemaHash != record.SchemaHash {
		return fmt.Errorf("unsealed snapshot has witness root %s, the record names %s", witness.ContentRoot, record.WitnessRoot)
	}
	return nil
}

// replayStateExtractor is the one extractor configuration for live and
// replay witnesses (witness v3): a derived index is not state, and the event
// index and projection heads carry wall-clock columns a replay writes afresh.
func replayStateExtractor(db *sql.DB) computerversion.SQLiteStateExtractor {
	return computerversion.SQLiteStateExtractor{
		DB:       db,
		Database: "texture",
		// A derived index is not state (objectgraph.DerivedKeysTable).
		IgnoredTables: map[string]struct{}{objectgraph.DerivedKeysTable: {}},
		IgnoredContentColumns: map[string]map[string]struct{}{
			"computer_event_index": {
				"prepared_at":  {},
				"finalized_at": {},
			},
			"computer_event_projection_heads": {
				"updated_at": {},
			},
		},
	}
}

// LiveStoreWitness is the witness v3 of the store behind db, with the same
// extractor configuration the checkpoint, replay probe and snapshot all use.
func LiveStoreWitness(ctx context.Context, db *sql.DB) (selfdevprotocol.VMLocalContentWitness, error) {
	return storeWitness(ctx, db, replayStateExtractor(db))
}

// storeWitness reads a v3 witness through the extractor's store in one read
// transaction, so every table digest is of one committed state.
func storeWitness(ctx context.Context, db *sql.DB, extractor computerversion.SQLiteStateExtractor) (selfdevprotocol.VMLocalContentWitness, error) {
	var empty selfdevprotocol.VMLocalContentWitness
	if db == nil {
		return empty, errors.New("self-development checkpoint: store is not open")
	}
	digests, err := extractor.Digests(ctx)
	if err != nil {
		return empty, err
	}
	return selfdevprotocol.WitnessFromDigests(digests)
}

// restoreReplayObserver counts recovery-critical replay work for tail-bounded
// cost evidence: every applied sequence must fall in (W,H] exactly once, and
// no page may enumerate from before W. The first tail page legitimately
// starts its cursor at W.
type restoreReplayObserver struct {
	mu              sync.Mutex
	applied         int
	appliedMin      uint64
	appliedMax      uint64
	pages           int
	pageAfterMin    uint64
	pageAfterMinSet bool
	commits         int
}

func (o *restoreReplayObserver) PageFetched(afterSequence uint64, count int) {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.pages++
	if !o.pageAfterMinSet || afterSequence < o.pageAfterMin {
		o.pageAfterMin = afterSequence
		o.pageAfterMinSet = true
	}
}

func (o *restoreReplayObserver) RecordApplied(sequence uint64) {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.applied++
	if o.applied == 1 || sequence < o.appliedMin {
		o.appliedMin = sequence
	}
	if sequence > o.appliedMax {
		o.appliedMax = sequence
	}
}

func (o *restoreReplayObserver) CheckpointCommitted(sequence uint64) {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.commits++
}

// tailReceipt verifies the observed replay against the asserted tail bounds.
// It proves zero prefix work and exactly-once contiguous tail application.
func (o *restoreReplayObserver) tailReceipt(watermarkSequence uint64, targetSequence uint64) (applied int, err error) {
	if o == nil {
		return 0, fmt.Errorf("rematerialize: replay observer is required")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.pageAfterMinSet && o.pageAfterMin < watermarkSequence {
		return 0, fmt.Errorf("%w: replay enumerated before watermark %d", recoveryplan.ErrBaseRefused, watermarkSequence)
	}
	if o.applied > 0 {
		if o.appliedMin <= watermarkSequence {
			return 0, fmt.Errorf("%w: replay applied prefix sequence %d", recoveryplan.ErrBaseRefused, o.appliedMin)
		}
		if o.appliedMax != targetSequence || o.applied != int(targetSequence-watermarkSequence) {
			return 0, fmt.Errorf("%w: tail application is not exactly-once (%d events, [%d,%d], want (%d,%d])", recoveryplan.ErrBaseRefused, o.applied, o.appliedMin, o.appliedMax, watermarkSequence, targetSequence)
		}
	} else if watermarkSequence != targetSequence {
		return 0, fmt.Errorf("%w: non-empty tail after watermark %d applied nothing", recoveryplan.ErrBaseRefused, watermarkSequence)
	}
	return o.applied, nil
}

var _ computerevent.ReplayObserver = (*restoreReplayObserver)(nil)
