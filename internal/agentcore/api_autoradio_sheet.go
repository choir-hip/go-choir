package agentcore

// The Autoradio run sheet, server side (slice D1): the custodian's copy.
//
//	GET  /api/autoradio/sheet          the sheet
//	POST /api/autoradio/sheet/ops      a batch of ops; one answer per op
//	GET  /api/autoradio/sheet/stream   SSE: the sheet, then every change
//
// Every applied op is one owner-private notice record on the tape,
// "autoradio-sheet-op:<op_id>", carrying the op and the version the sheet
// took. The sheet in memory is the fold of those records through
// runsheet.Apply and is rebuilt by replay when the process starts. A restart
// resumes nothing: replay only reads. A dropped op writes nothing.
//
// Every record carries a sheet id (subject ref sheet_ops / sheet_snap). The
// listener's sheet is "owner". A second kind of sheet, the public master,
// takes another sheet id and another owner principal and nothing else in the
// records, the store or the reducer changes (record ids of a non-owner sheet
// are prefixed by its id, which an op_id can never contain).
//
// The routes check shape, bounds, authentication and who may send which op.
// They never judge content; the reducer holds the rest.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/runsheet"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	sheetOwnerID = "owner"

	sheetOpRecordPrefix   = "autoradio-sheet-op:"
	sheetSnapRecordPrefix = "autoradio-sheet-snap:"
	kindSheetOp           = "autoradio.sheet_op"
	kindSheetSnapshot     = "autoradio.sheet_snapshot"

	// Bounds on what a signed-in stranger can cause.
	sheetMaxOps  = 64
	sheetMaxBody = 256 * 1024
	// sheetMaxAhead bounds the ahead list for place, which the reducer does
	// not cap (the reducer bounds previous). A place at "now" adds nothing
	// ahead and is not subject to it.
	sheetMaxAhead = runsheet.MaxAhead
	// sheetOpsPerHour bounds the ops an owner makes the process judge per
	// clock hour (ops not already answered); with the entry bounds of
	// internal/runsheet it bounds the tape one owner can add (see the
	// reference page for the byte figures). Whole batches are refused.
	sheetOpsPerHour = 1200
	// sheetMaxStreams bounds the open streams of one sheet.
	sheetMaxStreams = 4

	// Replay and memory.
	sheetSnapshotEvery = 128
	sheetKeepOps       = 256
	sheetKeepResults   = 2048
	sheetKeepSkipped   = 256
	sheetSnapPage      = 8

	sheetHeartbeat    = 20 * time.Second
	sheetWriteTimeout = 15 * time.Second
	// sheetReplayBackoff is how long a failed replay (a store read error) is
	// answered without reading the tape again.
	sheetReplayBackoff = 3 * time.Second
	// sheetIdle is how long a loaded sheet with no stream stays in memory.
	sheetIdle = 15 * time.Minute

	// Dropped reasons the route adds to the reducer's.
	droppedRecordTooLarge   = "record_too_large"
	droppedByDeskRefused    = "by_desk_refused"
	droppedPlacedByMismatch = "placed_by_mismatch"
	droppedAheadFull        = runsheet.DroppedAheadFull
)

// The bounds on what one record may be, checked at the write site against the
// real marshalled record (the figures the reference page states). The
// reducer's bounds keep every valid op and sheet far inside them; these are
// the guarantee that does not depend on that model being right. Variables so a
// test can lower them.
var (
	sheetMaxRecordBytes   = 64 * 1024
	sheetMaxSnapshotBytes = 6 * 1024 * 1024
)

// sheetReplayPage is the first page of op records a replay reads; a page that
// does not reach the snapshot is followed by one four times larger.
var sheetReplayPage = 1024

// sheetReplayMax is the most op records one replay reads; past it the sheet
// is rebuilt from the newest records alone, loudly.
var sheetReplayMax = 200000

// errSheetEvicted says the live sheet was dropped from memory (it was behind
// the tape, or idle): the caller takes the sheet again, which replays it.
var errSheetEvicted = errors.New("autoradio sheet: evicted")

type sheetLogEntry struct {
	version int64
	op      runsheet.Op
}

type sheetEvent struct {
	name string
	data []byte
}

type sheetSub struct{ ch chan sheetEvent }

// sheetEntry is the slot of one owner's sheet in the store. It outlives the
// live sheet (which may be evicted and replayed), so the hourly count does.
type sheetEntry struct {
	mu    sync.Mutex // guards ls, err, until: held while a replay runs
	ls    *liveSheet
	err   error
	until time.Time

	rateMu sync.Mutex
	window time.Time
	used   int
}

type liveSheet struct {
	ownerID, sheetID string
	store            *sheetStore
	entry            *sheetEntry
	lastUsed         atomic.Int64

	mu       sync.Mutex
	evicted  bool
	sheet    runsheet.RunSheet
	log      []sheetLogEntry
	results  map[string]runsheet.Result
	order    []string
	subs     map[*sheetSub]bool
	lastSnap int64
	replayed int
	// skippedOps are the op ids replays have skipped (newest sheetKeepSkipped),
	// carried from snapshot to snapshot so an op older than the newest
	// snapshot is still answered replay_skipped, never applied.
	skippedOps []string
}

type sheetStore struct {
	mu        sync.Mutex
	entries   map[string]*sheetEntry
	now       func() time.Time
	lastSweep time.Time
}

func newSheetStore() *sheetStore {
	return &sheetStore{entries: map[string]*sheetEntry{}, now: time.Now}
}

func (rt *Runtime) sheetStore() *sheetStore {
	rt.autoradioSheetsMu.Lock()
	defer rt.autoradioSheetsMu.Unlock()
	if rt.autoradioSheets == nil {
		rt.autoradioSheets = newSheetStore()
	}
	return rt.autoradioSheets
}

type sheetEnv struct {
	led      autoradioLedger
	computer string
}

func (h *APIHandler) sheetEnv() sheetEnv {
	return sheetEnv{led: h.programmeLedger(), computer: h.rt.cfg.ComputerID}
}

// get returns the live sheet, replaying it from the tape when it is not in
// memory. Only a store read error fails, and that is answered again without a
// new read for a few seconds.
func (st *sheetStore) get(ctx context.Context, h *APIHandler, ownerID, sheetID string) (*liveSheet, error) {
	now := st.now()
	st.mu.Lock()
	st.sweepLocked(now)
	key := ownerID + "\x00" + sheetID
	e := st.entries[key]
	if e == nil {
		e = &sheetEntry{}
		st.entries[key] = e
	}
	st.mu.Unlock()

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.ls != nil {
		e.ls.lastUsed.Store(now.UnixNano())
		return e.ls, nil
	}
	if e.err != nil && now.Before(e.until) {
		return nil, e.err
	}
	ls := &liveSheet{ownerID: ownerID, sheetID: sheetID, store: st, entry: e, results: map[string]runsheet.Result{}, subs: map[*sheetSub]bool{}}
	ls.lastUsed.Store(now.UnixNano())
	// A client that goes away does not make the replay fail for everyone.
	if err := ls.replay(context.WithoutCancel(ctx), h.sheetEnv()); err != nil {
		e.err, e.until = err, now.Add(sheetReplayBackoff)
		return nil, err
	}
	e.err, e.ls = nil, ls
	return ls, nil
}

// sweepLocked drops loaded sheets nobody has used for sheetIdle and that hold
// no stream. It never waits for a busy sheet.
func (st *sheetStore) sweepLocked(now time.Time) {
	if now.Sub(st.lastSweep) < time.Minute {
		return
	}
	st.lastSweep = now
	for _, e := range st.entries {
		if !e.mu.TryLock() {
			continue
		}
		if ls := e.ls; ls != nil && now.Sub(time.Unix(0, ls.lastUsed.Load())) >= sheetIdle && ls.mu.TryLock() {
			if len(ls.subs) == 0 {
				ls.evicted = true
				e.ls = nil
			}
			ls.mu.Unlock()
		}
		e.mu.Unlock()
	}
}

// evictLocked drops the sheet from memory (ls.mu held): the next call replays
// it from the tape. Streams are closed; they reconnect with their version.
func (ls *liveSheet) evictLocked() {
	if ls.evicted {
		return
	}
	ls.evicted = true
	for s := range ls.subs {
		delete(ls.subs, s)
		close(s.ch)
	}
	ls.entry.mu.Lock()
	if ls.entry.ls == ls {
		ls.entry.ls = nil
	}
	ls.entry.mu.Unlock()
}

// charge counts n unanswered ops against the owner's hour; it reports how
// long to wait when the batch would pass the bound.
func (e *sheetEntry) charge(now time.Time, n int) time.Duration {
	e.rateMu.Lock()
	defer e.rateMu.Unlock()
	if start := now.Truncate(time.Hour); !start.Equal(e.window) {
		e.window, e.used = start, 0
	}
	if e.used+n > sheetOpsPerHour {
		return e.window.Add(time.Hour).Sub(now)
	}
	e.used += n
	return 0
}

func sheetRecordPrefix(sheetID string) string {
	if sheetID == sheetOwnerID {
		return ""
	}
	return sheetID + ":"
}

func (ls *liveSheet) opRecordID(opID string) string {
	return sheetOpRecordPrefix + sheetRecordPrefix(ls.sheetID) + opID
}

type sheetOpBody struct {
	Version int64       `json:"version"`
	Op      runsheet.Op `json:"op"`
}

func (ls *liveSheet) notice(computer, id, kind string, refs map[string]string, objective, body string) (types.CommitmentRecord, error) {
	label, ok := types.AutoradioKindLabel(kind)
	if !ok {
		return types.CommitmentRecord{}, fmt.Errorf("record kind %q is not in the Autoradio table", kind)
	}
	out := map[string]string{"desk": autoradioDesk, "record_kind": kind}
	for k, v := range refs {
		out[k] = v
	}
	return types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Kind: types.CommitmentKindDirective,
		Discrepancy: types.DiscrepancyUnresolved,
		Directive:   &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNotice, Objective: objective, Body: body},
		Provenance: types.CommitmentProvenance{
			AgentID: autoradioDesk + ":" + ls.ownerID, ModelID: autoradioDesk, RunID: "sheet:" + ls.sheetID,
			CommittedAt: time.Now().UTC().Format(time.RFC3339Nano), Labels: types.NewLabelSet(label).Strings(),
		},
		EmitterKind: types.CommitmentEmitterAgent, ComputerID: computer, SubjectRefs: out,
	}, nil
}

// sheetJSON marshals without HTML escaping: the body a record carries as a
// string (runsheet bounds an entry by that carried form).
func sheetJSON(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

// opRecord is the tape record of one applied op and the version it took.
func (ls *liveSheet) opRecord(rt *Runtime, ownerID string, op runsheet.Op, version int64, extra ...map[string]string) (types.CommitmentRecord, error) {
	body, err := sheetJSON(sheetOpBody{Version: version, Op: op})
	if err != nil {
		return types.CommitmentRecord{}, err
	}
	refs := map[string]string{
		"sheet_ops": ls.sheetID, "op_id": op.OpID, "version": strconv.FormatInt(version, 10), "by": string(op.By),
	}
	for _, m := range extra {
		for k, v := range m {
			refs[k] = v
		}
	}
	return ls.notice(rt.cfg.ComputerID, ls.opRecordID(op.OpID), kindSheetOp, refs, "run sheet op "+op.OpID, string(body))
}

// snapshotRecord is the snapshot of the sheet as it stands. A repairing
// snapshot carries the sheet's new epoch in its id, so it can never collide
// with (and be swallowed by) an earlier snapshot at the same version.
func (ls *liveSheet) snapshotRecord(computer string, repair bool) (types.CommitmentRecord, error) {
	body, err := sheetJSON(ls.sheet)
	if err != nil {
		return types.CommitmentRecord{}, err
	}
	v := ls.sheet.Version
	id := sheetSnapRecordPrefix + sheetRecordPrefix(ls.sheetID) + strconv.FormatInt(v, 10)
	if repair {
		id += "-e" + strconv.FormatInt(ls.sheet.SheetEpoch, 10)
	}
	refs := map[string]string{"sheet_snap": ls.sheetID, "version": strconv.FormatInt(v, 10)}
	if len(ls.skippedOps) > 0 {
		refs["skipped_ops"] = strings.Join(ls.skippedOps, ",")
	}
	return ls.notice(computer, id, kindSheetSnapshot, refs, "run sheet snapshot at "+strconv.FormatInt(v, 10), string(body))
}

// recVersion is the version a record's subject ref names, without decoding
// its body.
func recVersion(r types.CommitmentRecord) (int64, bool) {
	v, err := strconv.ParseInt(r.SubjectRefs["version"], 10, 64)
	return v, err == nil && v > 0
}

func recTime(r types.CommitmentRecord) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, r.Provenance.CommittedAt)
	return t
}

// replay folds the tape into the sheet. A run sheet is a playlist: it is
// always rebuilt, and what cannot be rebuilt is skipped and counted loudly.
//
//   - The base is the newest snapshot that decodes (none: an empty sheet).
//   - Op records at or below the base are not read.
//   - The rest go in version order (ties by record time, then id). The first
//     candidate of a version that decodes and that today's reducer applies
//     takes the version; the others of that version are skipped (dup).
//   - A record that is skipped (it does not decode, or the reducer drops it)
//     still consumes its version: the sheet takes the version the record
//     names, entries unchanged, so no later op reuses it. The same holds for a
//     version gap: the version is the record's, never recomputed.
//   - The op_id of every skipped or deduplicated record is answered
//     replay_skipped, never applied.
//   - A row whose envelope does not decode is counted (it names no version, so
//     it cannot be placed or paid for once); it does not by itself repair.
//   - After a replay that skipped, stepped or deduplicated a version-bearing
//     record, the sheet takes a new sheet_epoch, a snapshot at the resulting
//     version is written at once (damage is paid once) and one line names
//     the counts and the first eight versions and op ids.
//
// Only a failed query fails.
func (ls *liveSheet) replay(ctx context.Context, env sheetEnv) error {
	base, baseSkipped, err := ls.readBase(ctx, env)
	if err != nil {
		return err
	}
	ls.skippedOps = baseSkipped
	for _, id := range baseSkipped {
		ls.remember(runsheet.Result{OpID: id, Dropped: runsheet.DroppedReplaySkipped, Version: base.Version})
	}
	recs, envelopes, truncated, err := ls.readOpsAbove(ctx, env, base.Version)
	if err != nil {
		return err
	}
	type cand struct {
		v   int64
		t   time.Time
		rec types.CommitmentRecord
	}
	var cands []cand
	skipped, nameless := 0, 0
	for _, r := range recs {
		v, ok := recVersion(r)
		if !ok {
			nameless++
			continue
		}
		if v <= base.Version {
			continue
		}
		cands = append(cands, cand{v, recTime(r), r})
	}
	sort.Slice(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if a.v != b.v {
			return a.v < b.v
		}
		if !a.t.Equal(b.t) {
			return a.t.Before(b.t)
		}
		return a.rec.RecordID < b.rec.RecordID
	})
	s, taken, gaps, dup, folded := base, base.Version, 0, 0, 0
	var notes []string
	note := func(v int64, opID, why string) {
		if len(notes) < 8 {
			notes = append(notes, fmt.Sprintf("v%d:%s:%s", v, opID, why))
		}
	}
	// dropped answers the record's op replay_skipped and lets it consume its
	// version (entries unchanged), so no later op reuses it.
	dropped := func(c cand, opID, why string) {
		if opID != "" {
			ls.skippedOps = append(ls.skippedOps, opID)
		}
		ls.remember(runsheet.Result{OpID: opID, Dropped: runsheet.DroppedReplaySkipped, Version: c.v})
		note(c.v, opID, why)
		if c.v > s.Version+1 {
			gaps++
		}
		if c.v > s.Version {
			s.Version = c.v
		}
	}
	for _, c := range cands {
		opID := c.rec.SubjectRefs["op_id"]
		if c.v == taken {
			dup++
			if opID != "" {
				ls.skippedOps = append(ls.skippedOps, opID)
			}
			ls.remember(runsheet.Result{OpID: opID, Dropped: runsheet.DroppedReplaySkipped, Version: c.v})
			note(c.v, opID, "dup")
			continue
		}
		var b sheetOpBody
		if c.rec.Directive == nil || json.Unmarshal([]byte(c.rec.Directive.Body), &b) != nil {
			skipped++
			dropped(c, opID, "undecodable")
			continue
		}
		if b.Op.OpID != "" {
			opID = b.Op.OpID
		}
		next, res, aerr := runsheet.Apply(s, b.Op)
		if aerr != nil || !res.Applied {
			skipped++
			dropped(c, opID, "dropped")
			continue
		}
		if c.v > s.Version+1 {
			gaps++
			note(c.v, opID, "gap")
		}
		next.Version = c.v
		res.Version = c.v
		s, taken = next, c.v
		ls.remember(res)
		ls.log = append(ls.log, sheetLogEntry{version: c.v, op: b.Op})
		folded++
	}
	if n := len(ls.log) - sheetKeepOps; n > 0 {
		ls.log = append([]sheetLogEntry(nil), ls.log[n:]...)
	}
	ls.sheet = s
	ls.lastSnap = base.Version
	ls.replayed = folded
	repaired := skipped+gaps+dup > 0 || truncated
	if repaired {
		epoch := ls.store.now().UnixMilli() // milliseconds: exact in a JSON number of any client
		if epoch <= base.SheetEpoch {
			epoch = base.SheetEpoch + 1
		}
		ls.sheet.SheetEpoch = epoch
	}
	if repaired || nameless+envelopes > 0 {
		log.Printf("autoradio sheet replay repaired owner=%s skipped=%d gaps=%d dup=%d unreadable=%d truncated=%v epoch=%d first=%v",
			ls.ownerID, skipped, gaps, dup, nameless+envelopes, truncated, ls.sheet.SheetEpoch, notes)
	}
	if n := len(ls.skippedOps) - sheetKeepSkipped; n > 0 {
		ls.skippedOps = append([]string(nil), ls.skippedOps[n:]...)
	}
	if repaired {
		ls.writeSnapshot(ctx, env, true)
	}
	return nil
}

// readBase returns the newest snapshot that decodes, paging deeper when the
// newest ones do not.
func (ls *liveSheet) readBase(ctx context.Context, env sheetEnv) (runsheet.RunSheet, []string, error) {
	for limit := sheetSnapPage; ; limit *= 4 {
		recs, _, rows, err := env.led.ListCommitmentRecordsBySubjectRefOfComputer(ctx, ls.ownerID, env.computer, "sheet_snap", ls.sheetID, limit)
		if err != nil {
			return runsheet.RunSheet{}, nil, fmt.Errorf("reading the snapshots: %w", err)
		}
		sort.Slice(recs, func(i, j int) bool {
			vi, _ := recVersion(recs[i])
			vj, _ := recVersion(recs[j])
			if vi != vj {
				return vi > vj
			}
			return recTime(recs[i]).After(recTime(recs[j]))
		})
		for _, r := range recs {
			var s runsheet.RunSheet
			if r.Directive != nil && json.Unmarshal([]byte(r.Directive.Body), &s) == nil && s.Version > 0 {
				var skipped []string
				if v := r.SubjectRefs["skipped_ops"]; v != "" {
					skipped = strings.Split(v, ",")
				}
				return s, skipped, nil
			}
		}
		if rows < limit || limit >= 4096 {
			return runsheet.RunSheet{}, nil, nil
		}
	}
}

// readOpsAbove pages through the op records, newest first, until a page
// reaches the base version or the tape is exhausted. Past sheetReplayMax
// records it stops and says so (truncated): the sheet is then rebuilt from
// the newest records alone, loudly, rather than not at all.
func (ls *liveSheet) readOpsAbove(ctx context.Context, env sheetEnv, base int64) (recs []types.CommitmentRecord, undecodable int, truncated bool, err error) {
	for limit := sheetReplayPage; ; limit *= 4 {
		if limit > sheetReplayMax {
			limit = sheetReplayMax
		}
		var rows int
		recs, undecodable, rows, err = env.led.ListCommitmentRecordsBySubjectRefOfComputer(ctx, ls.ownerID, env.computer, "sheet_ops", ls.sheetID, limit)
		if err != nil {
			return nil, 0, false, fmt.Errorf("reading the ops: %w", err)
		}
		if rows < limit {
			return recs, undecodable, false, nil
		}
		oldest := int64(-1)
		for _, r := range recs {
			if v, ok := recVersion(r); ok && (oldest < 0 || v < oldest) {
				oldest = v
			}
		}
		if oldest >= 0 && oldest <= base {
			return recs, undecodable, false, nil
		}
		if limit >= sheetReplayMax {
			return recs, undecodable, true, nil
		}
	}
}

func (ls *liveSheet) remember(r runsheet.Result) {
	if _, ok := ls.results[r.OpID]; !ok {
		ls.order = append(ls.order, r.OpID)
	}
	ls.results[r.OpID] = r
	if n := len(ls.order) - sheetKeepResults; n > 0 {
		for _, id := range ls.order[:n] {
			delete(ls.results, id)
		}
		ls.order = append([]string(nil), ls.order[n:]...)
	}
}

func (ls *liveSheet) aheadCount() int {
	n := 0
	for _, e := range ls.sheet.Entries {
		if e.State == runsheet.StateAhead {
			n++
		}
	}
	return n
}

// unanswered counts the ops of a batch the sheet has no answer for (ls.mu held).
func (ls *liveSheet) unanswered(ops []runsheet.Op) int {
	n := 0
	for _, op := range ops {
		if _, ok := ls.results[op.OpID]; !ok {
			n++
		}
	}
	return n
}

type sheetBatch struct {
	results []runsheet.Result
	version int64
	epoch   int64         // the sheet's epoch (nonzero after a repair)
	wait    time.Duration // non-zero: refused, over the hourly bound
}

// applyBatch applies ops in order under the sheet's lock. A tape failure
// stops the batch and evicts the sheet (the tape may or may not hold the
// record, so memory is no longer to be trusted): the ops before it are applied
// and durable, the failing op and the rest are not acknowledged.
func (ls *liveSheet) applyBatch(ctx context.Context, env sheetEnv, rt *Runtime, ops []runsheet.Op, trusted, replayedHere bool) (out sheetBatch, err error) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	if ls.evicted {
		return out, errSheetEvicted
	}
	startVersion := ls.sheet.Version
	var applied []runsheet.Op
	defer func() {
		out.version = ls.sheet.Version
		out.epoch = ls.sheet.SheetEpoch
		if len(applied) > 0 && !ls.evicted {
			ls.broadcast("sheet_ops", map[string]any{"from_version": startVersion, "to_version": ls.sheet.Version, "ops": applied})
		}
	}()
	seen := map[string]bool{}
	for _, op := range ops {
		if runsheet.ValidID(op.OpID) {
			if seen[op.OpID] {
				out.results = append(out.results, runsheet.Result{OpID: op.OpID, Dropped: runsheet.DroppedDuplicate, Version: ls.sheet.Version})
				continue
			}
			seen[op.OpID] = true
		}
		res, ok := ls.results[op.OpID]
		if !ok && runsheet.ValidID(op.OpID) {
			// Not remembered in memory: the tape is the memory of an applied op.
			rec, gerr := env.led.GetCommitmentRecord(ctx, ls.ownerID, env.computer, ls.opRecordID(op.OpID))
			if gerr != nil {
				return out, gerr
			}
			if rec != nil {
				v, vok := recVersion(*rec)
				switch {
				case !vok:
					res = runsheet.Result{OpID: op.OpID, Dropped: runsheet.DroppedReplaySkipped, Version: ls.sheet.Version}
				case v > ls.sheet.Version && !replayedHere:
					// The tape is ahead of memory: answer only from a sheet
					// that holds the record.
					ls.evictLocked()
					return out, errSheetEvicted
				case v > ls.sheet.Version:
					// Already replayed for this call and the record still is
					// not in the sheet: it did not fold.
					log.Printf("autoradio sheet: op %s is on the tape at version %d, above the replayed sheet (%d)", op.OpID, v, ls.sheet.Version)
					res = runsheet.Result{OpID: op.OpID, Dropped: runsheet.DroppedReplaySkipped, Version: ls.sheet.Version}
				default:
					res = runsheet.Result{OpID: op.OpID, Applied: true, Version: v}
				}
				ok = true
				ls.remember(res)
			}
		}
		if ok {
			if !res.Applied && res.Version < ls.sheet.Version {
				res.Version = ls.sheet.Version
			}
			out.results = append(out.results, res)
			continue
		}
		res = runsheet.Result{OpID: op.OpID, Version: ls.sheet.Version}
		switch {
		case op.By == runsheet.ByDesk && !trusted:
			res.Dropped = droppedByDeskRefused
		case op.Op == runsheet.OpPlace && op.Entry != nil && op.Entry.PlacedBy != op.By:
			res.Dropped = droppedPlacedByMismatch
		case op.Op == runsheet.OpPlace && op.At != runsheet.AtNow && op.Entry != nil && op.Entry.State == runsheet.StateAhead && ls.aheadCount() >= sheetMaxAhead:
			res.Dropped = droppedAheadFull
		default:
			next, r, aerr := runsheet.Apply(ls.sheet, op)
			if aerr != nil {
				res.Dropped = runsheet.DroppedInvalid
			} else if !r.Applied {
				res = r
			} else {
				var extra map[string]string
				if op.Op == runsheet.OpPrevious {
					// previous may drop desk entries to stay within the ahead
					// bound: the record names them (forecast evidence).
					if gone := droppedEntryIDs(ls.sheet, next); len(gone) > 0 {
						extra = map[string]string{"trimmed": strings.Join(gone, ",")}
					}
				}
				rec, rerr := ls.opRecord(rt, ls.ownerID, op, next.Version, extra)
				if rerr == nil {
					if raw, merr := json.Marshal(rec); merr == nil && len(raw) > sheetMaxRecordBytes {
						// Nothing is applied or appended: the op is refused whole.
						log.Printf("autoradio sheet: op %s would be a %d byte record (bound %d); dropped", op.OpID, len(raw), sheetMaxRecordBytes)
						res.Dropped = droppedRecordTooLarge
						if runsheet.ValidID(op.OpID) {
							ls.remember(res)
						}
						out.results = append(out.results, res)
						continue
					}
					// The settlement comes first: the settlement is written once by
					// its fixed id, so a failed or repeated append of the op is
					// harmless, whereas an op recorded with its settlement lost
					// could never be settled again (a replayed op is answered
					// from the tape and reaches no rule).
					rerr = ls.settleEnded(ctx, env, ls.sheet, next, op, settleNow(), trusted)
					if rerr == nil {
						_, rerr = env.led.AppendCommitmentRecord(ctx, ls.ownerID, env.computer, rec)
					}
				}
				if rerr != nil {
					ls.evictLocked()
					return out, rerr
				}
				ls.sheet = next
				ls.log = append(ls.log, sheetLogEntry{version: next.Version, op: op})
				if n := len(ls.log) - sheetKeepOps; n > 0 {
					ls.log = append([]sheetLogEntry(nil), ls.log[n:]...)
				}
				applied = append(applied, op)
				res = r
				ls.maybeSnapshot(ctx, env)
			}
		}
		if runsheet.ValidID(op.OpID) {
			ls.remember(res)
		}
		out.results = append(out.results, res)
	}
	return out, nil
}

// droppedEntryIDs are the ids on before that are not on after (at most 20).
func droppedEntryIDs(before, after runsheet.RunSheet) []string {
	have := map[string]bool{}
	for _, e := range after.Entries {
		have[e.ID] = true
	}
	var out []string
	for _, e := range before.Entries {
		if !have[e.ID] && len(out) < 20 {
			out = append(out, e.ID)
		}
	}
	return out
}

// maybeSnapshot keeps replay short. A failed snapshot is only logged: the
// ops are the record, and the next op tries again.
func (ls *liveSheet) maybeSnapshot(ctx context.Context, env sheetEnv) {
	if ls.sheet.Version-ls.lastSnap < sheetSnapshotEvery {
		return
	}
	ls.writeSnapshot(ctx, env, false)
}

func (ls *liveSheet) writeSnapshot(ctx context.Context, env sheetEnv, repair bool) {
	rec, err := ls.snapshotRecord(env.computer, repair)
	if err == nil {
		if raw, merr := json.Marshal(rec); merr == nil && len(raw) > sheetMaxSnapshotBytes {
			// Not written: the sheet stays correct from the previous snapshot
			// plus the ops.
			log.Printf("autoradio sheet: snapshot at %d would be a %d byte record (bound %d); not written", ls.sheet.Version, len(raw), sheetMaxSnapshotBytes)
			return
		}
		_, err = env.led.AppendCommitmentRecord(ctx, ls.ownerID, env.computer, rec)
	}
	if err != nil {
		log.Printf("autoradio sheet: snapshot at %d not written: %v", ls.sheet.Version, err)
		return
	}
	ls.lastSnap = ls.sheet.Version
}

// wire is the sheet as the phone sees it: no reducer memory, never a null list.
func wireSheet(s runsheet.RunSheet) runsheet.RunSheet {
	s.AppliedOps = nil
	if s.Entries == nil {
		s.Entries = []runsheet.Entry{}
	}
	return s
}

func (ls *liveSheet) broadcast(name string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("autoradio sheet: marshal %s: %v", name, err)
		return
	}
	for s := range ls.subs {
		select {
		case s.ch <- sheetEvent{name, data}:
		default: // a reader that cannot keep up reconnects with its version
			delete(ls.subs, s)
			close(s.ch)
		}
	}
}

// subscribe returns what a reader must be sent first, and registers it.
// evicted means take the sheet again.
func (ls *liveSheet) subscribe(after int64, hasAfter bool, epoch int64) (sub *sheetSub, first []sheetEvent, ok, evicted bool) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	if ls.evicted {
		return nil, nil, false, true
	}
	if len(ls.subs) >= sheetMaxStreams {
		return nil, nil, false, false
	}
	cur := ls.sheet.Version
	// A repaired sheet (nonzero epoch) is announced to every reader that does
	// not hold that epoch: its history changed under it, whatever its version.
	changed := ls.sheet.SheetEpoch != 0 && epoch != ls.sheet.SheetEpoch
	var ops []runsheet.Op
	if hasAfter && !changed && after < cur {
		for _, e := range ls.log {
			if e.version > after {
				ops = append(ops, e.op)
			}
		}
	}
	switch {
	case hasAfter && !changed && after == cur:
	// ops must be every version after+1..cur (a stepped-over version leaves a
	// hole the phone cannot fill): else the full sheet.
	case hasAfter && !changed && after < cur && int64(len(ops)) == cur-after:
		data, _ := json.Marshal(map[string]any{"from_version": after, "to_version": cur, "ops": ops})
		first = []sheetEvent{{"sheet_ops", data}}
	default:
		data, _ := json.Marshal(wireSheet(ls.sheet))
		first = []sheetEvent{{"sheet", data}}
	}
	s := &sheetSub{ch: make(chan sheetEvent, 64)}
	ls.subs[s] = true
	return s, first, true, false
}

func (ls *liveSheet) unsubscribe(s *sheetSub) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	if ls.subs[s] {
		delete(ls.subs, s)
		close(s.ch)
	}
}

// runBatch applies a batch, charging the owner's hour once for a stranger, and
// takes the sheet again if it was evicted on the way.
func (h *APIHandler) runBatch(ctx context.Context, ownerID, sheetID string, ops []runsheet.Op, trusted bool) (sheetBatch, error) {
	charged := trusted
	for attempt := 0; attempt < 3; attempt++ {
		ls, err := h.rt.sheetStore().get(ctx, h, ownerID, sheetID)
		if err != nil {
			return sheetBatch{}, err
		}
		if !charged {
			ls.mu.Lock()
			n := ls.unanswered(ops)
			ls.mu.Unlock()
			if wait := ls.entry.charge(ls.store.now(), n); wait > 0 {
				return sheetBatch{wait: wait}, nil
			}
			charged = true
		}
		b, err := ls.applyBatch(ctx, h.sheetEnv(), h.rt, ops, trusted, attempt > 0)
		if errors.Is(err, errSheetEvicted) {
			continue
		}
		return b, err
	}
	return sheetBatch{}, errors.New("autoradio sheet: could not settle the sheet")
}

// applySheetOps is the in-process door, for the desk (slice D2): a trusted
// caller may send ops by the desk, and its batches are not charged against the
// hourly bound of a stranger.
func (h *APIHandler) applySheetOps(ctx context.Context, ownerID, sheetID string, ops []runsheet.Op, trusted bool) ([]runsheet.Result, error) {
	b, err := h.runBatch(ctx, ownerID, sheetID, ops, trusted)
	return b.results, err
}

// ---- routes ----

func (h *APIHandler) sheetOwner(w http.ResponseWriter, r *http.Request, method string) (string, bool) {
	if r.Method != method {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return "", false
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return "", false
	}
	if h.rt == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "the runtime is not ready"})
		return "", false
	}
	return ownerID, true
}

// sheetServe fails a request whose sheet cannot be read from the tape (a
// store read error, or a sheet still being rebuilt after three tries): a 503.
func (h *APIHandler) sheetServe(w http.ResponseWriter, ctx context.Context, ownerID string) (*liveSheet, bool) {
	ls, err := h.rt.sheetStore().get(ctx, h, ownerID, sheetOwnerID)
	if err != nil {
		log.Printf("autoradio sheet: reading the tape for %s: %v", ownerID, err)
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "the run sheet cannot be read from the tape right now"})
		return nil, false
	}
	return ls, true
}

// HandleAutoradioSheet is GET /api/autoradio/sheet.
func (h *APIHandler) HandleAutoradioSheet(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := h.sheetOwner(w, r, http.MethodGet)
	if !ok {
		return
	}
	for attempt := 0; attempt < 3; attempt++ {
		ls, ok := h.sheetServe(w, r.Context(), ownerID)
		if !ok {
			return
		}
		ls.mu.Lock()
		gone := ls.evicted
		s := wireSheet(ls.sheet)
		ls.mu.Unlock()
		if gone {
			continue
		}
		writeAPIJSON(w, http.StatusOK, s)
		return
	}
	writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "the run sheet is being rebuilt; try again"})
}

func parseSheetOps(body []byte) ([]runsheet.Op, int64, error) {
	var top struct {
		Ops        json.RawMessage `json:"ops"`
		SheetEpoch int64           `json:"sheet_epoch"`
	}
	if err := json.Unmarshal(body, &top); err != nil {
		return nil, 0, errors.New("body: not valid JSON")
	}
	if len(top.Ops) == 0 || string(top.Ops) == "null" {
		return nil, 0, errors.New("ops: required")
	}
	var raws []json.RawMessage
	if err := json.Unmarshal(top.Ops, &raws); err != nil {
		return nil, 0, errors.New("ops: must be an array")
	}
	if len(raws) == 0 {
		return nil, 0, errors.New("ops: at least one op")
	}
	if len(raws) > sheetMaxOps {
		return nil, 0, fmt.Errorf("ops: at most %d ops", sheetMaxOps)
	}
	ops := make([]runsheet.Op, len(raws))
	for i, raw := range raws {
		if err := json.Unmarshal(raw, &ops[i]); err != nil {
			return nil, 0, fmt.Errorf("ops[%d]: %v", i, err)
		}
		switch {
		case ops[i].Op == "":
			return nil, 0, fmt.Errorf("ops[%d].op: required", i)
		case ops[i].By == "":
			return nil, 0, fmt.Errorf("ops[%d].by: required", i)
		case !runsheet.ValidID(ops[i].OpID):
			return nil, 0, fmt.Errorf("ops[%d].op_id: must match [A-Za-z0-9][A-Za-z0-9_.-]{0,63}", i)
		}
	}
	return ops, top.SheetEpoch, nil
}

// HandleAutoradioSheetOps is POST /api/autoradio/sheet/ops.
func (h *APIHandler) HandleAutoradioSheetOps(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := h.sheetOwner(w, r, http.MethodPost)
	if !ok {
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, sheetMaxBody))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeAPIJSON(w, http.StatusRequestEntityTooLarge, apiError{Error: fmt.Sprintf("body exceeds %d bytes", sheetMaxBody)})
			return
		}
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "body: cannot read"})
		return
	}
	ops, heldEpoch, err := parseSheetOps(body)
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	// A disconnect does not stop a batch half way: it is applied or refused whole.
	ctx := context.WithoutCancel(r.Context())
	b, err := h.runBatch(ctx, ownerID, sheetOwnerID, ops, false)
	if err != nil {
		log.Printf("autoradio sheet: batch for %s: %v", ownerID, err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "cannot record the ops"})
		return
	}
	if b.wait > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(b.wait.Seconds())+1))
		writeAPIJSON(w, http.StatusTooManyRequests, apiError{Error: fmt.Sprintf("more than %d ops this hour", sheetOpsPerHour)})
		return
	}
	ans := map[string]any{"version": b.version, "results": b.results}
	if b.epoch != 0 && b.epoch != heldEpoch {
		// The history was repaired and the sender does not hold the epoch (it
		// may send the one it holds as sheet_epoch in the body): it must take
		// GET /api/autoradio/sheet.
		ans["sheet_epoch"] = b.epoch
	}
	writeAPIJSON(w, http.StatusOK, ans)
}

// HandleAutoradioSheetStream is GET /api/autoradio/sheet/stream?after=<version>.
func (h *APIHandler) HandleAutoradioSheetStream(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := h.sheetOwner(w, r, http.MethodGet)
	if !ok {
		return
	}
	var after, epoch int64
	if raw := r.URL.Query().Get("epoch"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v < 0 {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "epoch: must be a number"})
			return
		}
		epoch = v
	}
	_, hasAfter := r.URL.Query()["after"]
	if hasAfter {
		v, err := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
		if err != nil || v < 0 {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "after: must be a version number"})
			return
		}
		after = v
	}
	var ls *liveSheet
	var sub *sheetSub
	var first []sheetEvent
	for attempt := 0; ; attempt++ {
		var ok bool
		if ls, ok = h.sheetServe(w, r.Context(), ownerID); !ok {
			return
		}
		var evicted bool
		sub, first, ok, evicted = ls.subscribe(after, hasAfter, epoch)
		if ok {
			break
		}
		if evicted && attempt < 3 {
			continue
		}
		if evicted {
			writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "the run sheet is being rebuilt; reconnect"})
			return
		}
		writeAPIJSON(w, http.StatusTooManyRequests, apiError{Error: fmt.Sprintf("at most %d open streams", sheetMaxStreams)})
		return
	}
	defer ls.unsubscribe(sub)

	hd := w.Header()
	hd.Set("Content-Type", "text/event-stream")
	hd.Set("Cache-Control", "no-cache")
	hd.Set("X-Accel-Buffering", "no")
	rc := http.NewResponseController(w)
	w.WriteHeader(http.StatusOK)
	flush := func() {
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	write := func(s string) bool {
		// The server-wide write timeout would cut a stream; each event gets
		// its own deadline, 15 s plus a second per 32 KB (a full sheet can be
		// 5 MB, which a slow link needs minutes for), so a half-open client
		// is released at the next event or heartbeat instead of holding a
		// slot for ever.
		_ = rc.SetWriteDeadline(time.Now().Add(sheetWriteTimeout + time.Duration(len(s)/(32*1024))*time.Second))
		_, err := io.WriteString(w, s)
		flush()
		return err == nil
	}
	if !write(": autoradio\n\n") {
		return
	}
	emit := func(e sheetEvent) bool { return write("event: " + e.name + "\ndata: " + string(e.data) + "\n\n") }
	for _, e := range first {
		if !emit(e) {
			return
		}
	}
	beat := time.NewTicker(sheetHeartbeat)
	defer beat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case e, open := <-sub.ch:
			if !open || !emit(e) {
				return
			}
		case <-beat.C:
			if !write(": ping\n\n") {
				return
			}
		}
	}
}
