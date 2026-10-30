package agentcore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// ErrReplayCompletenessUnavailable means the runtime cannot expose the
// owner-authorized replay probe on this process. It is a capability/configuration
// failure, not a clean replay result.
var ErrReplayCompletenessUnavailable = errors.New("replay completeness probe unavailable")

const replayCompletenessSchemaVersion = 4

// ReplayCompletenessReport is the read-only evidence returned by the product
// replay probe. Replay state is built in a disposable SQLite store; the live
// runtime store is only observed.
type ReplayCompletenessReport struct {
	SchemaVersion      int                               `json:"schema_version"`
	ComputerID         string                            `json:"computer_id"`
	CapturedAt         string                            `json:"captured_at"`
	LiveHead           *computerevent.Head               `json:"live_head"`
	ReplayHead         *computerevent.Head               `json:"replay_head"`
	Live               computerversion.ObservationSet    `json:"live"`
	Replay             computerversion.ObservationSet    `json:"replay"`
	Result             computerversion.EquivalenceResult `json:"result"`
	Eligibility        ReplayEligibility                 `json:"eligibility"`
	ProbeDigest        string                            `json:"probe_digest"`
	RunMemory          ReplayRunMemoryComparison         `json:"run_memory"`
	ObjectGraph        *ReplayObjectGraphComparison      `json:"object_graph,omitempty"`
	DepositMode        string                            `json:"deposit_mode,omitempty"`
	BaseSequence       uint64                            `json:"base_sequence"`
	BaseBlobSHA256     string                            `json:"base_blob_sha256"`
	TailTargetSequence uint64                            `json:"tail_target_sequence"`
	TailEventsApplied  int                               `json:"tail_events_applied"`
	TailPagesFetched   int                               `json:"tail_pages_fetched"`
}

// ReplayRunMemoryComparison summarizes row-level run-memory projection drift
// without returning provider-facing content. Samples are bounded because the
// full equivalence report already carries table-level hashes.
type ReplayRunMemoryComparison struct {
	LiveCount          int                         `json:"live_count"`
	ReplayCount        int                         `json:"replay_count"`
	LiveOnlyCount      int                         `json:"live_only_count"`
	ReplayOnlyCount    int                         `json:"replay_only_count"`
	DifferentCount     int                         `json:"different_count"`
	LiveOnlyByRun      map[string]int              `json:"live_only_by_run,omitempty"`
	LiveOnlyByOwner    map[string]int              `json:"live_only_by_owner,omitempty"`
	LiveOnlyByAgent    map[string]int              `json:"live_only_by_agent,omitempty"`
	LiveOnlySeqMin     int64                       `json:"live_only_seq_min,omitempty"`
	LiveOnlySeqMax     int64                       `json:"live_only_seq_max,omitempty"`
	LiveOnlyCreatedMin string                      `json:"live_only_created_min,omitempty"`
	LiveOnlyCreatedMax string                      `json:"live_only_created_max,omitempty"`
	Samples            []ReplayRunMemoryDifference `json:"samples,omitempty"`
}

type ReplayRunMemoryDifference struct {
	Kind            string   `json:"kind"`
	EntryID         string   `json:"entry_id"`
	RunID           string   `json:"loop_id,omitempty"`
	OwnerID         string   `json:"owner_id,omitempty"`
	AgentID         string   `json:"agent_id,omitempty"`
	Seq             int64    `json:"seq,omitempty"`
	CreatedAt       string   `json:"created_at,omitempty"`
	ReplayRunID     string   `json:"replay_loop_id,omitempty"`
	ReplayOwnerID   string   `json:"replay_owner_id,omitempty"`
	ReplayAgentID   string   `json:"replay_agent_id,omitempty"`
	ReplaySeq       int64    `json:"replay_seq,omitempty"`
	ReplayCreatedAt string   `json:"replay_created_at,omitempty"`
	LiveRowDigest   string   `json:"live_row_digest,omitempty"`
	ReplayRowDigest string   `json:"replay_row_digest,omitempty"`
	DifferentFields []string `json:"different_fields,omitempty"`
}

const replayRunMemorySampleLimit = 32

func compareReplayRunMemory(live, replay []choirstore.RunMemoryEntryFingerprint) ReplayRunMemoryComparison {
	comparison := ReplayRunMemoryComparison{
		LiveCount:   len(live),
		ReplayCount: len(replay),
	}
	liveByID := make(map[string]choirstore.RunMemoryEntryFingerprint, len(live))
	for _, entry := range live {
		liveByID[entry.EntryID] = entry
	}
	replayByID := make(map[string]choirstore.RunMemoryEntryFingerprint, len(replay))
	for _, entry := range replay {
		replayByID[entry.EntryID] = entry
	}
	ids := make([]string, 0, len(liveByID)+len(replayByID))
	seen := make(map[string]struct{}, len(liveByID)+len(replayByID))
	for id := range liveByID {
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	for id := range replayByID {
		if _, ok := seen[id]; !ok {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)

	for _, id := range ids {
		liveEntry, liveOK := liveByID[id]
		replayEntry, replayOK := replayByID[id]
		switch {
		case liveOK && !replayOK:
			comparison.LiveOnlyCount++
			comparison.recordLiveOnly(liveEntry)
			comparison.addSample(ReplayRunMemoryDifference{
				Kind: "live_only", EntryID: id, RunID: liveEntry.RunID, OwnerID: liveEntry.OwnerID,
				AgentID: liveEntry.AgentID, Seq: liveEntry.Seq, CreatedAt: liveEntry.CreatedAt,
				LiveRowDigest: liveEntry.RowDigest,
			})
		case !liveOK && replayOK:
			comparison.ReplayOnlyCount++
			comparison.addSample(ReplayRunMemoryDifference{
				Kind: "replay_only", EntryID: id, RunID: replayEntry.RunID, OwnerID: replayEntry.OwnerID,
				AgentID: replayEntry.AgentID, Seq: replayEntry.Seq, CreatedAt: replayEntry.CreatedAt,
				ReplayRowDigest: replayEntry.RowDigest,
			})
		case liveOK && replayOK:
			differentFields := differingReplayRunMemoryFields(liveEntry.FieldDigests, replayEntry.FieldDigests)
			if liveEntry.RowDigest == replayEntry.RowDigest && len(differentFields) == 0 {
				continue
			}
			if len(differentFields) == 0 {
				differentFields = []string{"row"}
			}
			comparison.DifferentCount++
			comparison.addSample(ReplayRunMemoryDifference{
				Kind: "different", EntryID: id,
				RunID: liveEntry.RunID, OwnerID: liveEntry.OwnerID, AgentID: liveEntry.AgentID,
				Seq: liveEntry.Seq, CreatedAt: liveEntry.CreatedAt,
				ReplayRunID: replayEntry.RunID, ReplayOwnerID: replayEntry.OwnerID, ReplayAgentID: replayEntry.AgentID,
				ReplaySeq: replayEntry.Seq, ReplayCreatedAt: replayEntry.CreatedAt,
				LiveRowDigest: liveEntry.RowDigest, ReplayRowDigest: replayEntry.RowDigest,
				DifferentFields: differentFields,
			})
		}
	}
	return comparison
}

func (c *ReplayRunMemoryComparison) recordLiveOnly(entry choirstore.RunMemoryEntryFingerprint) {
	if c.LiveOnlyByRun == nil {
		c.LiveOnlyByRun = make(map[string]int)
	}
	if c.LiveOnlyByOwner == nil {
		c.LiveOnlyByOwner = make(map[string]int)
	}
	if c.LiveOnlyByAgent == nil {
		c.LiveOnlyByAgent = make(map[string]int)
	}
	c.LiveOnlyByRun[entry.RunID]++
	c.LiveOnlyByOwner[entry.OwnerID]++
	c.LiveOnlyByAgent[entry.AgentID]++
	if c.LiveOnlySeqMin == 0 || entry.Seq < c.LiveOnlySeqMin {
		c.LiveOnlySeqMin = entry.Seq
	}
	if entry.Seq > c.LiveOnlySeqMax {
		c.LiveOnlySeqMax = entry.Seq
	}
	if c.LiveOnlyCreatedMin == "" || entry.CreatedAt < c.LiveOnlyCreatedMin {
		c.LiveOnlyCreatedMin = entry.CreatedAt
	}
	if entry.CreatedAt > c.LiveOnlyCreatedMax {
		c.LiveOnlyCreatedMax = entry.CreatedAt
	}
}

func (c *ReplayRunMemoryComparison) addSample(sample ReplayRunMemoryDifference) {
	if len(c.Samples) < replayRunMemorySampleLimit {
		c.Samples = append(c.Samples, sample)
	}
}

func differingReplayRunMemoryFields(live, replay map[string]string) []string {
	keys := make([]string, 0, len(live)+len(replay))
	seen := make(map[string]struct{}, len(live)+len(replay))
	for key := range live {
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	for key := range replay {
		if _, ok := seen[key]; !ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	different := make([]string, 0, len(keys))
	for _, key := range keys {
		if live[key] != replay[key] {
			different = append(different, key)
		}
	}
	return different
}

// ReplayCompleteness performs the effects Definition's pre-drop measurement.
// It reconstructs the canonical event chain into a fresh workspace, hashes
// that projection and the live store through SQLiteStateExtractor, and
// returns the exact deterministic diff. No event is appended and the live
// store is never used as the replay projection.
func (rt *Runtime) ReplayCompleteness(ctx context.Context, computerID string) (ReplayCompletenessReport, error) {
	probe, err := rt.openReplayProbe(ctx, computerID)
	if err != nil {
		return ReplayCompletenessReport{}, err
	}
	probe.close()
	return probe.report, nil
}

// replayProbe is a completed replay-completeness probe whose disposable
// replay store is still open, so a checkpoint can advance it past the
// captured head by the tail only (bindCheckpointAtHead). close discards it.
type replayProbe struct {
	report      ReplayCompletenessReport
	replayStore *choirstore.Store
	tempRoot    string
	// ctx is the caller's context guarded by the scratch watch: replay work
	// on the probe's store runs under it, so a volume running short stops
	// the replay by name (replay_scratch.go). stopWatch ends the watch.
	ctx       context.Context
	stopWatch func()
}

// scratchErr names a replay error the scratch watch caused.
func (p *replayProbe) scratchErr(err error) error {
	if p == nil {
		return err
	}
	return replayScratchCause(p.ctx, err)
}

func (p *replayProbe) close() {
	if p == nil {
		return
	}
	if p.replayStore != nil {
		_ = p.replayStore.Close()
		p.replayStore = nil
	}
	if p.stopWatch != nil {
		p.stopWatch()
		p.stopWatch = nil
	}
	if p.tempRoot != "" {
		_ = os.RemoveAll(p.tempRoot)
		activeReplayScratch.Delete(p.tempRoot)
		p.tempRoot = ""
	}
}

// openReplayProbe runs the probe and keeps its replay store open. The caller
// closes the returned probe.
func (rt *Runtime) openReplayProbe(ctx context.Context, computerID string) (_ *replayProbe, err error) {
	if rt == nil || rt.store == nil || rt.eventAppender == nil {
		return nil, fmt.Errorf("%w: event projection authority is not configured", ErrReplayCompletenessUnavailable)
	}
	computerID = strings.TrimSpace(computerID)
	if computerID == "" || computerID != strings.TrimSpace(rt.cfg.ComputerID) {
		return nil, fmt.Errorf("%w: computer binding does not match runtime", ErrReplayCompletenessUnavailable)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// The scratch goes beside the live store, and only when the volume can
	// hold it and keep the live store's reserve: checked before the capture
	// holds appends (replay_scratch.go).
	scratchParent := rt.replayScratchParent()
	if scratchParent == "" {
		return nil, fmt.Errorf("%w: live store path is not configured", ErrReplayCompletenessUnavailable)
	}
	if err := os.MkdirAll(scratchParent, 0o700); err != nil {
		return nil, fmt.Errorf("replay completeness: create scratch parent: %w", err)
	}
	if err := rt.admitReplayScratch(ctx, computerID, scratchParent); err != nil {
		return nil, fmt.Errorf("replay completeness: %w", err)
	}

	// Capture the head and the live state together, from one read snapshot:
	// appends wait only while the snapshot's first read (the head) runs, so
	// the state belongs to exactly this head; the scan and the run-memory
	// fingerprints then read that snapshot on the read pool while appends and
	// writes proceed. The replay below stops at the head, and desks keep
	// working meanwhile
	// (docs/problems/m11-rerun-12-restore-refused-and-texture-reports-no-change-2026-10-10.md).
	var (
		liveHead      *computerevent.Head
		live          computerversion.ObservationSet
		liveRunMemory []choirstore.RunMemoryEntryFingerprint
		version       computerversion.ComputerVersion
	)
	extractor := replayStateExtractor(rt.store.ReadDB())
	extractor.SnapshotHook = func(ctx context.Context, tx *sql.Tx) error {
		if err := rt.eventAppender.HoldAppends(func() error {
			var err error
			if liveHead, err = rt.store.HeadTx(ctx, tx, computerID); err != nil {
				return fmt.Errorf("replay completeness: read live event head: %w", err)
			}
			return nil
		}); err != nil {
			return err
		}
		version = replayCompletenessVersion(rt, liveHead)
		var err error
		if liveRunMemory, err = rt.store.ListRunMemoryEntryFingerprintsTx(ctx, tx); err != nil {
			return fmt.Errorf("replay completeness: fingerprint live run memory: %w", err)
		}
		return nil
	}
	liveDigests, err := extractor.Digests(ctx)
	if err != nil {
		return nil, fmt.Errorf("replay completeness: extract live store state: %w", err)
	}
	if live, err = computerversion.ObservationSetFromDigests(liveDigests, computerversion.ExtractRequest{Name: "live-sqlite", Version: version}); err != nil {
		return nil, fmt.Errorf("replay completeness: extract live store state: %w", err)
	}
	if rt.replayProbeAfterCapture != nil {
		rt.replayProbeAfterCapture()
	}

	tempRoot, err := os.MkdirTemp(scratchParent, replayScratchPrefix)
	if err != nil {
		return nil, fmt.Errorf("replay completeness: create disposable workspace: %w", err)
	}
	activeReplayScratch.Store(tempRoot, struct{}{})
	guarded, stopWatch := rt.watchReplayScratch(ctx, scratchParent)
	probe := &replayProbe{tempRoot: tempRoot, ctx: guarded, stopWatch: stopWatch}
	opened := false
	defer func() {
		if !opened {
			err = probe.scratchErr(err)
			probe.close()
		}
	}()
	ctx = guarded
	replayStore, baseSequence, baseBlob, targetSequence, observer, err := rt.openProbeReplayStore(ctx, computerID, tempRoot, liveHead)
	if err != nil {
		return nil, err
	}
	probe.replayStore = replayStore
	depositMode, err := matchLiveDepositMode(rt.store, replayStore)
	if err != nil {
		return nil, fmt.Errorf("replay completeness: %w", err)
	}
	rt.eventAppender.SetReplayObserver(observer)
	var reconstructErr error
	if liveHead != nil && liveHead.Sequence > 0 {
		reconstructErr = rt.eventAppender.ReconstructThroughTarget(ctx, replayStore, liveHead.CanonicalEventHead)
	} else {
		reconstructErr = rt.eventAppender.ReconstructInto(ctx, replayStore)
	}
	rt.eventAppender.SetReplayObserver(nil)
	if reconstructErr != nil {
		return nil, fmt.Errorf("replay completeness: reconstruct event chain: %w", reconstructErr)
	}
	tailApplied := 0
	if targetSequence > 0 {
		tailApplied, err = observer.tailReceipt(baseSequence, targetSequence)
		if err != nil {
			return nil, err
		}
	}
	replayHead, err := replayStore.Head(ctx, computerID)
	if err != nil {
		return nil, fmt.Errorf("replay completeness: read replay event head: %w", err)
	}
	replayExtractor := replayStateExtractor(replayStore.ReadDB())
	replay, err := replayExtractor.Extract(ctx, computerversion.ExtractRequest{Name: "event-replay-projection", Version: version})
	if err != nil {
		return nil, fmt.Errorf("replay completeness: extract replay store state: %w", err)
	}

	replayRunMemory, err := replayStore.ListRunMemoryEntryFingerprints(ctx)
	if err != nil {
		return nil, fmt.Errorf("replay completeness: fingerprint replay run memory: %w", err)
	}
	runMemory := compareReplayRunMemory(liveRunMemory, replayRunMemory)

	result := (computerversion.EquivalenceChecker{}).CheckObservationSets(live, replay)
	var objectGraph *ReplayObjectGraphComparison
	if !result.Equivalent() {
		// Diagnostic only: name the object-graph rows replay cannot
		// reproduce. Failure to list leaves the report as it was.
		liveObjects, liveErr := rt.store.ListObjectGraphFingerprints(ctx)
		replayObjects, replayErr := replayStore.ListObjectGraphFingerprints(ctx)
		if liveErr == nil && replayErr == nil {
			objectGraph = compareReplayObjectGraph(liveObjects, replayObjects)
		}
	}
	report := ReplayCompletenessReport{
		SchemaVersion:      replayCompletenessSchemaVersion,
		ComputerID:         computerID,
		CapturedAt:         time.Now().UTC().Format(time.RFC3339Nano),
		LiveHead:           liveHead,
		ReplayHead:         replayHead,
		Live:               live,
		Replay:             replay,
		Result:             result,
		Eligibility:        replayEligibility(liveHead, replayHead, live, replay, result),
		RunMemory:          runMemory,
		ObjectGraph:        objectGraph,
		DepositMode:        depositMode,
		BaseSequence:       baseSequence,
		BaseBlobSHA256:     baseBlob,
		TailTargetSequence: targetSequence,
		TailEventsApplied:  tailApplied,
		TailPagesFetched:   observer.pages,
	}
	raw, err := computerevent.CanonicalJSON(report)
	if err != nil {
		return nil, fmt.Errorf("replay completeness: encode report: %w", err)
	}
	report.ProbeDigest = computerevent.DigestBytes(raw)
	probe.report = report
	opened = true
	return probe, nil
}

// openProbeReplayStore builds the disposable replay projection for the probe.
// A live chain replays from the newest snapshot the tape names (tail-only);
// absent one, the probe replays the full tape from genesis —
// the tape is the truth, a snapshot is only a cost bound, and the refusal
// posture for unbounded replay is preserved by MaxRecoveryTailEvents. An
// empty store probes the explicit bootstrap path without one. The caller
// closes the store and removes the temp root.
const (
	replayDepositModeRetained = "retained"
	replayDepositModeUpcast   = "upcast"
	// replayDepositModeBaseUpcast: the installed base was built by an
	// upcasting replay while the live store never upcast; recorded V1
	// spellings may still differ (residual base-upcast-vs-live).
	replayDepositModeBaseUpcast = "base_upcast"
)

// matchLiveDepositMode puts the staged replay store in the live store's
// deposit mode before any event replays into it. A live store without an
// upcast ledger kept every deposit's recorded bytes (including recorded
// tool-call content no writer can respell), so the replay compared with it
// and the restore that replaces it must keep them too
// (problems/selfdev-apply-checkpoint-starved-by-resumed-work-2026-10-10.md).
// An unreadable ledger fails closed.
func matchLiveDepositMode(live, staged *choirstore.Store) (string, error) {
	liveUpcast, err := live.DepositsUpcast()
	if err != nil {
		return "", fmt.Errorf("read live deposit mode: %w", err)
	}
	if liveUpcast {
		return replayDepositModeUpcast, nil
	}
	stagedUpcast, err := staged.DepositsUpcast()
	if err != nil {
		return "", fmt.Errorf("read staged deposit mode: %w", err)
	}
	if stagedUpcast {
		return replayDepositModeBaseUpcast, nil
	}
	if err := staged.RetainReplayDeposits(); err != nil {
		return "", err
	}
	return replayDepositModeRetained, nil
}

func (rt *Runtime) openProbeReplayStore(ctx context.Context, computerID, tempRoot string, liveHead *computerevent.Head) (*choirstore.Store, uint64, string, uint64, *restoreReplayObserver, error) {
	observer := &restoreReplayObserver{}
	if liveHead == nil || liveHead.Sequence == 0 {
		replayStore, err := choirstore.OpenFresh(filepath.Join(tempRoot, "runtime.db"))
		if err != nil {
			return nil, 0, "", 0, nil, fmt.Errorf("replay completeness: open disposable workspace: %w", err)
		}
		return replayStore, 0, "", 0, observer, nil
	}
	tape, err := rt.resolveRestoreTape()
	if err != nil {
		return nil, 0, "", 0, nil, err
	}
	// The newest snapshot at or below the live head starts the replay; with
	// none, the whole chain replays from genesis — deterministic reduction
	// makes the full replay the strongest completeness evidence — inside the
	// recovery bound planRestoreTarget keeps.
	plan, err := planRestoreTarget(ctx, tape, computerID, liveHead.CanonicalEventHead)
	if err != nil {
		return nil, 0, "", 0, nil, err
	}
	staged, record, err := rt.openStagedStore(ctx, tape, tempRoot, "runtime.db", computerID, plan)
	if err != nil {
		return nil, 0, "", 0, nil, err
	}
	return staged, record.StoreHeadSequence, record.Manifest.Root, plan.sequence, observer, nil
}

func replayCompletenessVersion(rt *Runtime, head *computerevent.Head) computerversion.ComputerVersion {
	codeRef := strings.TrimSpace(rt.startupManifest().Digest)
	if codeRef == "" {
		codeRef = "runtime:" + strings.TrimSpace(rt.cfg.ComputerID)
	}
	artifactRef := "event-chain:none"
	if head != nil && strings.TrimSpace(head.CanonicalEventHead) != "" {
		artifactRef = "event-chain:" + head.CanonicalEventHead
	}
	return computerversion.ComputerVersion{
		CodeRef:            computerversion.CodeRef(codeRef),
		ArtifactProgramRef: computerversion.ArtifactProgramRef(artifactRef),
	}
}
