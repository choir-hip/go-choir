package agentcore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/recoveryplan"
	"github.com/yusefmosiah/go-choir/internal/snapshot"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// The snapshot producer (docs/design/choir-data-system-frame-2026-10-30.md
// sections 5 and 6; build sheet decisions 7 and 12). A computer whose disk is
// lost is rebuilt from its newest snapshot plus a tape tail, so every computer
// takes snapshots: one as soon as boot has settled when it has none, then one
// per interval.
//
// Ways it could fail, written before the code:
//
//  1. the capture runs on the write connection or inside an append hold and
//     blocks every write on a 30 GB store for minutes (it copies the pages of
//     one read transaction on the read pool and holds nothing);
//  2. the first snapshot is judged by scanning the tape from genesis over HTTP
//     (the local computer_event_index answers it);
//  3. two snapshots run at once, or one runs during a boot, a maintenance
//     hold, a self-development apply or while an engineering capsule is live;
//  4. the capture writes a second copy of the store (docs/design/
//     snapshot-one-pass-2026-11-22.md: it streams the pages from its read
//     transaction to the uploader and writes none; a scratch directory an older
//     binary left is removed at boot);
//  5. a long read transaction pins the WAL and the volume fills with WAL while
//     it runs (the pin lasts the stream and the uploader paces it, so it has
//     its own deadline, snapshotPinDeadline; the producer refuses under the
//     floor plus the WAL a pin that long would hold, the stream checks free
//     space every 64 MiB, and the whole capture has a time budget,
//     CHOIR_SNAPSHOT_MAX_DURATION, after which it aborts and says why);
//  6. a snapshot that crashes the computer is retried at every boot (a crash
//     loop: the attempt guard closes it after snapshotAttemptLimit starts);
//  7. a refusal is read as success, or the producer waits forever on one
//     (every refusal is logged with its reason and retried at the next tick).
//
// There is no size cap: a capture needs the floor plus WAL headroom free on the
// data volume, never the store's size; the host stores only the chunks it does
// not already hold. The producer ships ON for every
// computer. A free-space or busy refusal is a correctness refusal, not an off
// switch; CHOIR_SNAPSHOT_DISABLE=1 is the emergency override only.
const (
	envSnapshotInterval    = "CHOIR_SNAPSHOT_INTERVAL"
	envSnapshotMaxDuration = "CHOIR_SNAPSHOT_MAX_DURATION"
	envSnapshotDisable     = "CHOIR_SNAPSHOT_DISABLE"

	// A snapshot bounds the replay a rebuild needs; it is due by tape
	// distance (snapshotDefaultTailEvents), and the interval is only a weekly
	// backstop for a quiet computer. Daily snapshots of the paper re-uploaded
	// about 20 GB each: a fixed page-range cut shares few chunks across a
	// day of writes (owner, 2026-10-31: "we need to reconsider how we do
	// compaction if our current plan has us rewriting gbs every day").
	snapshotDefaultInterval = 7 * 24 * time.Hour
	// Three quarters of the recovery bound: a tail reaches the bound only if
	// the snapshot due here fails for a third as long again (it retries every
	// tick). Not half: each of the paper's snapshots writes about 25 GB of new
	// chunks to the host, and at half it would take two a day.
	snapshotDefaultTailEvents = recoveryplan.MaxRecoveryTailEvents / 4 * 3
	// The capture is bounded as a whole: the stream (which pins the WAL), the
	// uploads and the partitions; past this the attempt aborts with a logged
	// reason and the next attempt waits out its backoff. This is a deadline, not
	// a switch.
	snapshotDefaultMaxDuration = 90 * time.Minute
	// snapshotWALBytesPerSecond is the rate a pinned WAL grows at on the paper:
	// 3.7 GB in 30 minutes, 2026-11-12 (docs/design/
	// snapshot-without-scratch-copy-2026-11-20.md), about 2 MiB/s. Over the pin
	// deadline it is 2 MiB * 1800 s = 3,774,873,600 bytes (3.5 GiB, 3.77 GB).
	// It sizes the headroom the free-space rule asks for.
	snapshotWALBytesPerSecond = int64(2) << 20
	// After a failed attempt the next one waits snapshotBackoffFloor, doubling
	// per consecutive failure up to snapshotBackoffCeiling; a success resets
	// it. Every attempt uploads most of a live store's chunks, which the host
	// keeps until the grace passes, so a failure must not be retried at the
	// tick (6 GiB an hour on the paper, 2026-11-12). At 6 events/s the
	// ceiling adds about 65,000 events to the tail, inside the margin between
	// the trigger and the recovery bound. A backoff after our own failure, not
	// a call limit.
	snapshotBackoffFloor   = 2 * snapshotTickEvery
	snapshotBackoffCeiling = 3 * time.Hour
	// The WAL grows while the stream pins it. A capture needs this floor plus the
	// WAL headroom (snapshotWALHeadroom) free on the data volume; the stream
	// checks the floor every 64 MiB it reads.
	snapshotFreeFloor = uint64(2) << 30

	// The restore request is one blocking request of ten minutes (proxy
	// PROXY_REPLAY_COMPLETENESS_TIMEOUT, nix/node-b.nix; the guest's write
	// deadline). Scan and snapshot install took about four minutes on d@e
	// (2026-11-14), the witness and flip a minute, and a minute is margin, so
	// the tail's replay gets four. At the assumed rate that is 27,120 events.
	restoreRequestBudget = 10 * time.Minute
	snapshotReplayBound  = 4 * time.Minute
	// snapshotAssumedReplayRate is the replay rate used until one is measured
	// on this computer (appender ReplayRate): d@e's latest measurement,
	// 33,332 events in 4 m 54 s (2026-11-14), the lowest of its 113 to 380
	// events/s readings. The paper's replay rate is not in the records; if it
	// is slower than this the bound is optimistic there until a replay has
	// been measured on it.
	snapshotAssumedReplayRate = 113.0
	// The floor between replay-time captures (never between the 540,000-event
	// ceiling or the interval, which fire regardless): the next one is not
	// before the larger of snapshotCaptureWallFraction times the last
	// capture's duration (capturing at most a tenth of wall time) and the last
	// capture's sealed bytes spread over snapshotBytesPerDay (the owner's
	// 2026-10-31 objection to rewriting GBs every day). Inputs are the last
	// capture's own measurements. On d@e (11 s, a small store) it is minutes;
	// on the paper (37 min 40 s, 4.8 GB) it is 13.4 hours.
	snapshotCaptureWallFraction = 10
	snapshotBytesPerDay         = int64(8) << 30
	// snapshotCaptureFileName holds the last capture's own cost beside the
	// store (the data volume, which a restart leaves), so the floor survives
	// a restart.
	snapshotCaptureFileName = "snapshot-producer.json"

	snapshotInitialDelay = 3 * time.Minute
	snapshotTickEvery    = 10 * time.Minute

	snapshotAttemptKey   = "snapshot_producer"
	snapshotAttemptLimit = 3
)

// The pin and the free-space guard are variables so a test can shorten them;
// nothing else sets them.
var (
	// The read transaction is bounded on its own: it lasts as long as the
	// uploader holds the stream back, and it pins the WAL. Past this the stream
	// stops, the WAL is released and the attempt fails (snapshot.ErrPinDeadline);
	// the chunks already uploaded are kept and the retry skips them. The mission's
	// target for the paper's 51 GiB store is well under it (docs/definitions/
	// choir-storage-efficiency-2026-11-22.md). A deadline, not a switch.
	snapshotPinDeadline = 30 * time.Minute
	// snapshotFreeCheckEvery is how often the capture's guard reads the free
	// space on its own, apart from the check every 64 MiB the stream reads. The
	// stream's check stops while the reader is blocked behind a stalled upload,
	// and the pinned WAL keeps growing then: the guard is what sees it. It is
	// compaction's own interval (store compactFreeCheckEvery), so a write burst
	// cannot fill the volume inside a window longer than compaction allows.
	snapshotFreeCheckEvery = 10 * time.Second
	// snapshotFreeBytes reads the free space of the volume holding dir.
	snapshotFreeBytes = freeBytesOnVolume
)

type snapshotOutcome string

const (
	snapshotTaken   snapshotOutcome = "taken"
	snapshotNotDue  snapshotOutcome = "not_due"
	snapshotRefused snapshotOutcome = "refused"
	snapshotFailed  snapshotOutcome = "failed"
	snapshotSkipped snapshotOutcome = "skipped"
	// snapshotBackoff: due, but waiting out the backoff after a failure.
	snapshotBackoff snapshotOutcome = "backoff"
)

// snapshotSettings are the producer's tunables, read from the environment.
type snapshotSettings struct {
	Interval    time.Duration
	MaxDuration time.Duration
	Disabled    bool
	// TailEvents fires a snapshot before the interval once the tail since the
	// newest one reaches it, so a busy computer's tail stays inside the
	// recovery bound between snapshots. Zero: the interval alone.
	TailEvents uint64
}

func snapshotSettingsFromEnv(getenv func(string) string) snapshotSettings {
	s := snapshotSettings{Interval: snapshotDefaultInterval, MaxDuration: snapshotDefaultMaxDuration, TailEvents: snapshotDefaultTailEvents}
	if raw := strings.TrimSpace(getenv(envSnapshotInterval)); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			s.Interval = d
		} else {
			log.Printf("runtime: %s=%q is not a positive duration; using %s", envSnapshotInterval, raw, snapshotDefaultInterval)
		}
	}
	if raw := strings.TrimSpace(getenv(envSnapshotMaxDuration)); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			s.MaxDuration = d
		} else {
			log.Printf("runtime: %s=%q is not a positive duration; using %s", envSnapshotMaxDuration, raw, snapshotDefaultMaxDuration)
		}
	}
	switch strings.ToLower(strings.TrimSpace(getenv(envSnapshotDisable))) {
	case "1", "true", "on", "yes":
		s.Disabled = true
	}
	return s
}

// snapshotDeps is what one producer tick reads and does. The runtime fills it
// from its store, appender and platform client; tests substitute seams.
type snapshotDeps struct {
	ComputerID string
	StorePath  string
	// DataDir is the data volume directory beside the store.
	DataDir string
	// Newest is the newest snapshot_recorded on the LOCAL event index.
	Newest func(ctx context.Context) (sequence uint64, at time.Time, found bool, err error)
	// HeadSequence is the store's event head sequence (false for an empty
	// store).
	HeadSequence func(ctx context.Context) (uint64, bool, error)
	// Busy returns a reason when work a snapshot must not run beside is live.
	Busy func(ctx context.Context) (string, error)
	// FreeBytes is the free space on the volume holding dir (the WAL grows
	// there while the stream's read transaction is open).
	FreeBytes func(dir string) (uint64, error)
	// Ready reports whether boot has settled and the runtime is not held.
	Ready func() bool
	// Now is the clock (nil: time.Now). The backoff counts from the time an
	// attempt ENDS, which a 90 minute attempt makes far later than `now`.
	Now func() time.Time
	// ReplayRate is the measured replay rate on this computer (events/s) when
	// one exists (nil or false: snapshotAssumedReplayRate is used).
	ReplayRate func() (float64, bool)
	// Take takes the snapshot.
	Take func(ctx context.Context, maxDuration time.Duration) (snapshot.Ref, error)

	// StagedCopyBytes is the size of state.db.compact, named in a refusal and
	// never removed. afterGates runs between the read-only gates and the free
	// space measurement (a test seam).
	StagedCopyBytes func() int64
	afterGates      func()
}

type snapshotProducer struct {
	tick sync.Mutex // one snapshot at a time
	// inFlight is true while a snapshot streams. A restart would cut it and
	// the next attempt would repeat the whole read, so a planned restart
	// waits for it (plannedRestartWork.SnapshotInFlight); its time budget
	// bounds the wait. A restart asked for in the instant before a stream
	// starts still cuts it: that costs one attempt, never state.
	inFlight atomic.Bool

	mu       sync.Mutex
	stop     chan struct{}
	lastAt   time.Time
	lastSeq  uint64
	lastKnow bool
	lastWhy  string

	// failures counts consecutive failed attempts; failedHead is the store
	// head the last one began at; retryAt is when the next may begin.
	failures   int
	failedHead uint64
	retryAt    time.Time

	// What the observability block reports (status). eval is the latest
	// evaluation; the streak is how long a due snapshot has gone untaken; the
	// last capture is the floor's input.
	eval                snapshotEval
	evalAt              time.Time
	lastAttemptAt       time.Time
	lastOutcome         snapshotOutcome
	lastDetail          string
	streakSince         time.Time
	streakCount         int
	streakWhy           string
	lastCaptureDuration time.Duration
	lastCaptureBytes    int64
	haveCapture         bool
	lastStats           snapshot.Stats
}

// snapshotEval is one evaluation of whether a snapshot is due.
type snapshotEval struct {
	Head         uint64
	NewestSeq    uint64
	NewestAt     time.Time
	Tail         uint64
	Rate         float64
	RateMeasured bool
	Est          time.Duration
	Due          bool
	Why          string
	// Kind is why it is due: first, ceiling, interval or replay.
	Kind         string
	FloorLimited bool
	Floor        time.Duration
	FloorWhy     string
	// FloorKey names the capture facts behind FloorWhy without its countdown,
	// so the log says it once per capture, not at every tick.
	FloorKey string
}

// snapshotCaptureRecord is the last capture's own cost, persisted beside the
// store. StartedAt is how the next boot knows it belongs to the newest
// snapshot on the tape: that event was appended when the capture ended.
type snapshotCaptureRecord struct {
	// ComputerID is whose capture it is: a copied data directory is not
	// another computer's cost.
	ComputerID      string    `json:"computer_id,omitempty"`
	StartedAt       time.Time `json:"started_at"`
	DurationSeconds float64   `json:"duration_seconds"`
	SealedBytes     int64     `json:"sealed_bytes"`
	HeadSequence    uint64    `json:"head_sequence,omitempty"`
}

func saveSnapshotCapture(dir string, rec snapshotCaptureRecord) error {
	raw, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, snapshotCaptureFileName+".tmp-*")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(raw)
	if serr := tmp.Sync(); werr == nil {
		werr = serr
	}
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		_ = os.Remove(tmp.Name())
		return werr
	}
	if err := os.Chmod(tmp.Name(), 0o600); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, snapshotCaptureFileName)); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

// loadSnapshotCapture reads the persisted capture cost; false when there is
// none or it is unreadable (the cost is then unknown, and a replay-time
// capture is not held back by it).
func loadSnapshotCapture(dir string) (snapshotCaptureRecord, bool) {
	raw, err := os.ReadFile(filepath.Join(dir, snapshotCaptureFileName))
	if err != nil {
		return snapshotCaptureRecord{}, false
	}
	var rec snapshotCaptureRecord
	if json.Unmarshal(raw, &rec) != nil || rec.StartedAt.IsZero() || rec.DurationSeconds <= 0 {
		return snapshotCaptureRecord{}, false
	}
	return rec, true
}

// belongsTo reports whether this record is the cost of the snapshot whose
// event the tape dates at: appended when the capture ended.
func (r snapshotCaptureRecord) belongsTo(eventAt time.Time) bool {
	dur := time.Duration(r.DurationSeconds * float64(time.Second))
	// A minute of slack before the start: the event is stamped by the appender,
	// the start by this clock, and a suspend or a step between them is not a
	// different snapshot.
	return !eventAt.Before(r.StartedAt.Add(-time.Minute)) && eventAt.Sub(r.StartedAt) <= dur+30*time.Minute
}

// snapshotDueInput is what the trigger reads.
type snapshotDueInput struct {
	Tail         uint64
	Since        time.Duration // since the newest snapshot's capture began
	Interval     time.Duration
	TailEvents   uint64 // the event ceiling (0: none)
	Rate         float64
	RateMeasured bool
	HaveCapture  bool
	CaptureDur   time.Duration
	CaptureBytes int64
}

// snapshotCaptureFloor is the least time between two replay-time captures,
// from the last capture's own duration and sealed bytes.
func snapshotCaptureFloor(dur time.Duration, bytes int64) time.Duration {
	floor := dur * snapshotCaptureWallFraction
	if bytes > 0 {
		perDay := time.Duration(float64(bytes) / float64(snapshotBytesPerDay) * float64(24*time.Hour))
		floor = max(floor, perDay)
	}
	return floor
}

// snapshotDue decides whether a snapshot is due: the 540,000-event ceiling, the
// interval, or a tail whose replay over the replay rate exceeds
// snapshotReplayBound, the last only past the capture floor when the last
// capture's cost is known.
func snapshotDue(in snapshotDueInput) snapshotEval {
	// The rate used is the slowest of the assumed one and what was measured: a
	// fast replay never relaxes the bound above the worst the records show.
	rate := snapshotAssumedReplayRate
	ev := snapshotEval{Tail: in.Tail}
	if in.RateMeasured && in.Rate > 0 && in.Rate < rate {
		rate, ev.RateMeasured = in.Rate, true
	}
	ev.Rate = rate
	ev.Est = time.Duration(float64(in.Tail) / rate * float64(time.Second))
	switch {
	case in.TailEvents > 0 && in.Tail >= in.TailEvents:
		ev.Due, ev.Kind, ev.Why = true, "ceiling", fmt.Sprintf("tail of %d events", in.Tail)
	case in.Since >= in.Interval:
		ev.Due, ev.Kind, ev.Why = true, "interval", "interval"
	case ev.Est > snapshotReplayBound:
		ev.Why = fmt.Sprintf("restore replay of %d events would take %s at %.0f events/s, over the %s bound", in.Tail, ev.Est.Round(time.Second), rate, snapshotReplayBound)
		switch {
		case !in.HaveCapture:
			// An existing snapshot whose capture cost is not recorded (taken by a
			// binary that did not persist it): there is no floor to wait out. The
			// capture is taken, and it measures the cost for every later tick and
			// restart.
			ev.Due, ev.Kind = true, "replay"
		case in.Since < snapshotCaptureFloor(in.CaptureDur, in.CaptureBytes):
			ev.Floor = snapshotCaptureFloor(in.CaptureDur, in.CaptureBytes)
			ev.FloorLimited = true
			ev.FloorKey = fmt.Sprintf("%s|%d|%s", in.CaptureDur.Round(time.Second), in.CaptureBytes, ev.Floor.Round(time.Second))
			ev.FloorWhy = fmt.Sprintf("%s; this computer's restore tail exceeds the restore budget because captures are too expensive: the last capture took %s and wrote %d bytes, so the next replay-time capture is not before %s after the last began (%s from now)",
				ev.Why, in.CaptureDur.Round(time.Second), in.CaptureBytes, ev.Floor.Round(time.Second), (ev.Floor - in.Since).Round(time.Second))
		default:
			ev.Due, ev.Kind = true, "replay"
			ev.Floor = snapshotCaptureFloor(in.CaptureDur, in.CaptureBytes)
		}
	}
	return ev
}

// snapshotBackoffWait is the wait after the nth consecutive failure (n >= 1).
func snapshotBackoffWait(n int) time.Duration {
	wait := snapshotBackoffFloor
	for i := 1; i < n && wait < snapshotBackoffCeiling; i++ {
		wait *= 2
	}
	return min(wait, snapshotBackoffCeiling)
}

// run is one evaluation: take a snapshot if one is due and nothing refuses.
// It records the evaluation and its outcome for status.
func (p *snapshotProducer) run(ctx context.Context, d snapshotDeps, set snapshotSettings, now time.Time) (snapshotOutcome, string) {
	outcome, detail := p.evaluate(ctx, d, set, now)
	p.recordOutcome(now, outcome, detail)
	return outcome, detail
}

func (p *snapshotProducer) evaluate(ctx context.Context, d snapshotDeps, set snapshotSettings, now time.Time) (snapshotOutcome, string) {
	if set.Disabled {
		return snapshotSkipped, envSnapshotDisable + " is set"
	}
	if !p.tick.TryLock() {
		return snapshotSkipped, "a snapshot is already running"
	}
	defer p.tick.Unlock()
	if d.Ready != nil && !d.Ready() {
		return snapshotSkipped, "boot has not settled or the computer is held"
	}

	p.mu.Lock()
	known := p.lastKnow
	p.mu.Unlock()
	if !known {
		seq, at, found, err := d.Newest(ctx)
		if err != nil {
			return snapshotFailed, "read the local snapshot index: " + err.Error()
		}
		// The newest snapshot's own cost, persisted by the producer that took
		// it, when it is this snapshot's (a restart keeps the floor).
		rec, recOK := loadSnapshotCapture(d.DataDir)
		p.mu.Lock()
		p.lastKnow = true
		if found {
			p.lastSeq, p.lastAt = seq, at
			if recOK && (rec.ComputerID == "" || rec.ComputerID == d.ComputerID) && rec.belongsTo(at) && !p.haveCapture {
				// The tail starts at the capture's head, not at the event the
				// append wrote after it (the events of the capture's own length).
				if rec.HeadSequence > 0 && rec.HeadSequence < seq {
					p.lastSeq = rec.HeadSequence
				}
				p.haveCapture = true
				p.lastCaptureDuration = time.Duration(rec.DurationSeconds * float64(time.Second))
				p.lastCaptureBytes = rec.SealedBytes
			}
		}
		p.mu.Unlock()
	}
	p.mu.Lock()
	lastSeq, lastAt := p.lastSeq, p.lastAt
	p.mu.Unlock()

	head, hasHead, err := d.HeadSequence(ctx)
	if err != nil {
		return snapshotFailed, "read the store head: " + err.Error()
	}
	if !hasHead {
		p.setEval(now, snapshotEval{Rate: snapshotAssumedReplayRate})
		return snapshotNotDue, "the store holds no events"
	}
	reason := "first"
	ev := snapshotEval{Head: head, NewestSeq: lastSeq, NewestAt: lastAt, Tail: head - min(head, lastSeq), Due: true, Why: reason, Kind: "first", Rate: snapshotAssumedReplayRate}
	if !lastAt.IsZero() {
		if head <= lastSeq+1 {
			ev.Due = false
			p.setEval(now, ev)
			return snapshotNotDue, "no event since the newest snapshot"
		}
		in := snapshotDueInput{Tail: head - lastSeq, Since: now.Sub(lastAt), Interval: set.Interval, TailEvents: set.TailEvents}
		if d.ReplayRate != nil {
			in.Rate, in.RateMeasured = d.ReplayRate()
		}
		p.mu.Lock()
		in.HaveCapture, in.CaptureDur, in.CaptureBytes = p.haveCapture, p.lastCaptureDuration, p.lastCaptureBytes
		p.mu.Unlock()
		ev = snapshotDue(in)
		ev.Head, ev.NewestSeq, ev.NewestAt = head, lastSeq, lastAt
		p.setEval(now, ev)
		if !ev.Due {
			return snapshotNotDue, ev.FloorWhy
		}
		reason = ev.Why
	} else {
		p.setEval(now, ev)
	}

	// A failed attempt uploaded chunks the host keeps: wait before the next.
	p.mu.Lock()
	failures, failedHead, retryAt := p.failures, p.failedHead, p.retryAt
	p.mu.Unlock()
	if failures > 0 && now.Before(retryAt) {
		return snapshotBackoff, fmt.Sprintf("backing off after %d failed attempt(s), the last at head %d; the next attempt is not before %s", failures, failedHead, retryAt.UTC().Format(time.RFC3339))
	}

	// The gates above only read. From here the attempt acts, in this order: the
	// directory, the correctness refusals, then the free space.
	if d.afterGates != nil {
		d.afterGates()
	}
	if err := os.MkdirAll(d.DataDir, 0o700); err != nil {
		return snapshotFailed, "create the data dir: " + err.Error()
	}

	// Correctness refusals: logged by the caller, retried at the next tick.
	if why, err := d.Busy(ctx); err != nil {
		return snapshotRefused, "cannot tell whether work is live (fails closed): " + err.Error()
	} else if why != "" {
		return snapshotRefused, why
	}
	free, err := d.FreeBytes(d.DataDir)
	if err != nil {
		return snapshotRefused, "cannot measure free space (fails closed): " + err.Error()
	}
	// The capture writes no copy of the store: it needs the floor and the WAL a
	// pinned reader would hold for as long as the pin deadline allows. A staged
	// compact copy needs no term: its bytes are already missing from free.
	p.mu.Lock()
	lastGrowth := p.lastStats.PinWALEnd - p.lastStats.PinWALStart
	p.mu.Unlock()
	headroom := snapshotWALHeadroom(lastGrowth)
	need := headroom + snapshotFreeFloor
	if free < need {
		staged := ""
		if n := d.StagedCopyBytes(); n > 0 {
			staged = fmt.Sprintf("; a staged compact copy of %d bytes (state.db.compact) is on the volume and a capture never removes it", n)
		}
		return snapshotRefused, fmt.Sprintf("free space %d bytes on the data volume is under the %d bytes this capture needs: %d bytes of WAL headroom for a pin of up to %s plus the %d-byte floor%s", free, need, headroom, snapshotPinDeadline, snapshotFreeFloor, staged)
	}

	// A snapshot that crashes the computer is not retried forever.
	if d.StorePath != "" {
		if err := spendGuestAttempt(d.StorePath, snapshotAttemptKey, d.ComputerID, snapshotAttemptLimit); err != nil {
			return snapshotRefused, "closed after crashing the computer: " + err.Error()
		}
	}
	clock := time.Now
	if d.Now != nil {
		clock = d.Now
	}
	started := clock()
	// inFlight holds through the cost record's write below: a planned restart
	// between the end of the capture and the record would lose the cost and cost
	// a second full capture at the next boot.
	p.inFlight.Store(true)
	defer p.inFlight.Store(false)
	ref, err := d.Take(ctx, set.MaxDuration)
	took := clock().Sub(started)
	if d.StorePath != "" {
		_ = clearGuestAttempt(d.StorePath, snapshotAttemptKey, d.ComputerID)
	}
	if err != nil {
		p.mu.Lock()
		p.failures++
		p.failedHead = head
		ended := time.Now()
		if d.Now != nil {
			ended = d.Now()
		}
		p.retryAt = ended.Add(snapshotBackoffWait(p.failures))
		n, next := p.failures, p.retryAt
		p.mu.Unlock()
		cause := err.Error()
		switch {
		case errors.Is(err, snapshot.ErrPinDeadline):
			cause = fmt.Sprintf("the read transaction was released at the %s pin deadline with nothing recorded (the chunks uploaded so far are kept): %s", snapshotPinDeadline, cause)
		case errors.Is(err, snapshot.ErrTimeBudget):
			cause = fmt.Sprintf("aborted at the %s time budget (%s) with nothing recorded: %s", set.MaxDuration, envSnapshotMaxDuration, cause)
		}
		return snapshotFailed, fmt.Sprintf("%s (attempt at head %d; failure %d; next attempt not before %s)", cause, head, n, next.UTC().Format(time.RFC3339))
	}
	p.mu.Lock()
	priorFailures := p.failures
	p.failures, p.failedHead, p.retryAt = 0, 0, time.Time{}
	p.lastAt, p.lastSeq = now, ref.Record.StoreHeadSequence
	// After a failed attempt the retry finds the failed one's uploads already
	// held and writes little: the cost of getting this snapshot is at least
	// what the last known capture was.
	sealed := ref.Stats.BytesUploaded
	if priorFailures > 0 {
		sealed = max(sealed, p.lastCaptureBytes)
	}
	p.haveCapture, p.lastCaptureDuration, p.lastCaptureBytes = true, took, sealed
	p.lastStats = ref.Stats
	// The block is current right after a capture: the newest snapshot is this
	// one and nothing is due until the next evaluation.
	p.eval = snapshotEval{Head: ref.Record.StoreHeadSequence, NewestSeq: ref.Record.StoreHeadSequence, NewestAt: now, Rate: ev.Rate, RateMeasured: ev.RateMeasured}
	p.evalAt = now
	p.mu.Unlock()
	if d.DataDir != "" {
		if err := saveSnapshotCapture(d.DataDir, snapshotCaptureRecord{ComputerID: d.ComputerID, StartedAt: started, DurationSeconds: took.Seconds(), SealedBytes: sealed, HeadSequence: ref.Record.StoreHeadSequence}); err != nil {
			log.Printf("runtime: snapshot: could not record the capture cost beside the store (the cost is unknown after a restart and a replay-time capture is taken again): %v", err)
		}
	}
	st := ref.Stats
	return snapshotTaken, fmt.Sprintf("%s snapshot at head %d: %d bytes (%d pages of %d) in %s, event %s; %d chunks, %d uploaded (%d bytes sealed), %d already held, %d repeated, %d probe errors; pin %s (%.0f pages/s), upload after the pin %s, wal %+d bytes over the pin, %d events during it, no scratch copy",
		reason, ref.Record.StoreHeadSequence, ref.Record.PlaintextSize, st.Pages, st.PageSize, took.Round(time.Millisecond), ref.EventID,
		st.Chunks, st.ChunksUploaded, st.BytesUploaded, st.ChunksHeld, st.ChunksRepeated, st.ProbeErrors,
		st.PinTime.Round(time.Millisecond), pagesPerSecond(st), st.UploadTime.Round(time.Millisecond), st.PinWALEnd-st.PinWALStart, st.PinEvents)
}

// forget makes the next evaluation read the newest snapshot, the head and the
// persisted capture cost afresh (the store was replaced by a restore).
func (p *snapshotProducer) forget() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastKnow, p.lastAt, p.lastSeq = false, time.Time{}, 0
	p.haveCapture, p.lastCaptureDuration, p.lastCaptureBytes = false, 0, 0
	p.eval, p.evalAt = snapshotEval{}, time.Time{}
	p.streakCount, p.streakSince, p.streakWhy = 0, time.Time{}, ""
	p.failures, p.failedHead, p.retryAt = 0, 0, time.Time{}
}

func (p *snapshotProducer) floorKey() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.eval.FloorKey
}

// setEval records the latest evaluation for status.
func (p *snapshotProducer) setEval(now time.Time, ev snapshotEval) {
	p.mu.Lock()
	p.eval, p.evalAt = ev, now
	p.mu.Unlock()
}

// recordOutcome keeps the last attempt and how long a due snapshot has gone
// untaken. A refusal, a failure and a backoff all count: the snapshot was due
// and was not taken. A taken snapshot or a snapshot no longer due ends the
// streak; a skipped tick (boot not settled) leaves it.
func (p *snapshotProducer) recordOutcome(now time.Time, outcome snapshotOutcome, detail string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch outcome {
	case snapshotRefused, snapshotFailed, snapshotBackoff:
		if p.streakCount == 0 {
			p.streakSince = now
		}
		p.streakCount++
		p.streakWhy = detail
		p.lastAttemptAt, p.lastOutcome, p.lastDetail = now, outcome, detail
	case snapshotTaken:
		p.streakCount, p.streakSince, p.streakWhy = 0, time.Time{}, ""
		p.lastAttemptAt, p.lastOutcome, p.lastDetail = now, outcome, detail
	case snapshotNotDue:
		p.streakCount, p.streakSince, p.streakWhy = 0, time.Time{}, ""
	}
}

// snapshotProducerStatus is the observability block (GET
// /api/runtime/observability, snapshot_producer): whether a snapshot is due,
// how long a restore's tail would replay, and what has stopped a due snapshot.
type snapshotProducerStatus struct {
	NewestSequence uint64 `json:"newest_sequence"`
	NewestAt       string `json:"newest_at,omitempty"`
	HeadSequence   uint64 `json:"head_sequence"`
	TailEvents     uint64 `json:"tail_events"`
	// ReplayRate is the events/s the estimate uses; ReplayRateSource is
	// "measured" (this computer's last real replay) or "assumed".
	ReplayRate             float64 `json:"replay_rate"`
	ReplayRateSource       string  `json:"replay_rate_source"`
	EstimatedReplaySeconds float64 `json:"estimated_replay_seconds"`
	ReplayBoundSeconds     float64 `json:"replay_bound_seconds"`
	Due                    bool    `json:"due"`
	DueWhy                 string  `json:"due_why,omitempty"`
	// FloorLimited: the replay estimate is over the bound but the last
	// capture's cost holds the next one back; FloorWhy says so.
	FloorLimited bool   `json:"floor_limited"`
	FloorWhy     string `json:"floor_why,omitempty"`
	EvaluatedAt  string `json:"evaluated_at,omitempty"`

	LastAttemptAt string `json:"last_attempt_at,omitempty"`
	LastOutcome   string `json:"last_outcome,omitempty"`
	LastDetail    string `json:"last_detail,omitempty"`
	// Refusal is the current reason a due snapshot was not taken, RefusedSince
	// when that began without a snapshot in between, and ConsecutiveRefusals
	// how many ticks (refused, failed or backing off).
	Refusal             string  `json:"refusal,omitempty"`
	RefusedSince        string  `json:"refused_since,omitempty"`
	ConsecutiveRefusals int     `json:"consecutive_refusals"`
	LastCaptureSeconds  float64 `json:"last_capture_seconds"`
	LastCaptureBytes    int64   `json:"last_capture_bytes"`
	// CaptureKnown: the last capture's cost is known (taken in this process
	// or persisted for the newest snapshot). False after an old snapshot is
	// found with no record: a replay-time capture is then due without a floor,
	// and that capture measures the cost.
	CaptureKnown bool `json:"capture_known"`

	// The last capture's pin and phases: the pin is the interval with an open
	// reader on the state store (the whole stream); the upload is what remained
	// after it; the WAL growth over the pin and the events appended during it
	// normalise it by the write rate. Zero before the first capture of this
	// process.
	LastPinSeconds        float64 `json:"last_pin_seconds"`
	LastUploadSeconds     float64 `json:"last_upload_seconds"`
	LastStateBytes        int64   `json:"last_state_bytes"`
	LastPinWALGrowthBytes int64   `json:"last_pin_wal_growth_bytes"`
	LastPinEvents         uint64  `json:"last_pin_events"`
}

// status is the producer's state for the observability route.
func (p *snapshotProducer) status(_ time.Time) snapshotProducerStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	ev := p.eval
	st := snapshotProducerStatus{
		NewestSequence: ev.NewestSeq, HeadSequence: ev.Head, TailEvents: ev.Tail,
		ReplayRate: ev.Rate, ReplayRateSource: "assumed", EstimatedReplaySeconds: ev.Est.Seconds(),
		ReplayBoundSeconds: snapshotReplayBound.Seconds(),
		Due:                ev.Due, FloorLimited: ev.FloorLimited, FloorWhy: ev.FloorWhy,
		ConsecutiveRefusals: p.streakCount, LastCaptureSeconds: p.lastCaptureDuration.Seconds(), LastCaptureBytes: p.lastCaptureBytes,
		CaptureKnown: p.haveCapture,
	}
	if p.haveCapture {
		ls := p.lastStats
		st.LastPinSeconds, st.LastUploadSeconds = ls.PinTime.Seconds(), ls.UploadTime.Seconds()
		st.LastStateBytes, st.LastPinEvents = ls.StateBytes, ls.PinEvents
		st.LastPinWALGrowthBytes = ls.PinWALEnd - ls.PinWALStart
	}
	if ev.Rate == 0 {
		st.ReplayRate = snapshotAssumedReplayRate
	}
	if ev.RateMeasured {
		st.ReplayRateSource = "measured"
	}
	if ev.Due {
		st.DueWhy = ev.Why
	}
	if !ev.NewestAt.IsZero() {
		st.NewestAt = ev.NewestAt.UTC().Format(time.RFC3339)
	}
	if !p.evalAt.IsZero() {
		st.EvaluatedAt = p.evalAt.UTC().Format(time.RFC3339)
	}
	if !p.lastAttemptAt.IsZero() {
		st.LastAttemptAt = p.lastAttemptAt.UTC().Format(time.RFC3339)
		st.LastOutcome, st.LastDetail = string(p.lastOutcome), p.lastDetail
	}
	if p.streakCount > 0 {
		st.Refusal = p.streakWhy
		st.RefusedSince = p.streakSince.UTC().Format(time.RFC3339)
	}
	return st
}

// pagesPerSecond is the stream's rate: the state file's pages (its size over
// the page size; Stats.Pages also counts the partitions) over the pin.
func pagesPerSecond(st snapshot.Stats) float64 {
	if st.PinTime <= 0 || st.PageSize <= 0 {
		return 0
	}
	return float64(st.StateBytes/int64(st.PageSize)) / st.PinTime.Seconds()
}

// snapshotWALHeadroom is the room a capture asks for beyond the floor: the WAL
// a pinned reader holds. It is the rate the paper's WAL grew at while pinned
// over the pin deadline, or twice the last capture's own growth when that is
// larger.
func snapshotWALHeadroom(lastGrowth int64) uint64 {
	planned := snapshotWALBytesPerSecond * int64(snapshotPinDeadline/time.Second)
	if lastGrowth > 0 {
		planned = max(planned, 2*lastGrowth)
	}
	return uint64(planned)
}

// snapshotDepsForRuntime binds the producer to this runtime. It returns false
// when the runtime lacks what a snapshot needs (no store, appender, key or
// platform connection): a local or test runtime, not a production guest.
func (rt *Runtime) snapshotDepsForRuntime() (snapshotDeps, bool) {
	if rt == nil || rt.store == nil || rt.eventAppender == nil || rt.privateArtifactCipher == nil {
		return snapshotDeps{}, false
	}
	computerID := strings.TrimSpace(rt.cfg.ComputerID)
	client, ok := rt.eventPayloadReader.(*computerevent.HTTPClient)
	if computerID == "" || !ok || client == nil {
		return snapshotDeps{}, false
	}
	chunks := snapshot.HTTPChunks{BaseURL: client.BaseURL(), ComputerID: computerID, Capability: client.Capability()}
	store := rt.store
	dataDir := filepath.Dir(store.StatePath())
	return snapshotDeps{
		ComputerID: computerID,
		StorePath:  rt.cfg.StorePath,
		DataDir:    dataDir,
		Newest: func(ctx context.Context) (uint64, time.Time, bool, error) {
			return store.NewestEventOfKind(ctx, computerID, computerevent.EventSnapshotRecorded)
		},
		HeadSequence: func(ctx context.Context) (uint64, bool, error) {
			head, err := store.Head(ctx, computerID)
			if err != nil || head == nil {
				return 0, false, err
			}
			return head.Sequence, true, nil
		},
		Busy: func(ctx context.Context) (string, error) {
			work, err := rt.plannedRestartBusy(ctx, true)
			if err != nil {
				return "", err
			}
			if work.SelfdevActive > 0 {
				return fmt.Sprintf("%d self-development operation(s) in flight", work.SelfdevActive), nil
			}
			if work.LiveCapsules > 0 {
				return fmt.Sprintf("%d engineering capsule(s) live", work.LiveCapsules), nil
			}
			return "", nil
		},
		FreeBytes:  freeBytesOnVolume,
		ReplayRate: rt.eventAppender.ReplayRate,
		Ready: func() bool {
			return !rt.bootInProgress.Load() && !rt.maintenanceHeld()
		},
		StagedCopyBytes: func() int64 { return choirstore.CompactStagedBytes(rt.cfg.StorePath) },
		Take: func(ctx context.Context, maxDuration time.Duration) (snapshot.Ref, error) {
			// releasedHead is the head sequence when the read transaction ended,
			// for the events appended during the pin.
			var releasedHead atomic.Uint64
			ref, err := snapshot.TakeSnapshot(ctx, snapshot.Config{
				ComputerID: computerID,
				DB:         store.ReadDB(),
				// The head is the first statement of the read transaction that
				// streams the pages.
				Head: func(ctx context.Context, tx *sql.Tx) (*computerevent.Head, error) {
					return store.HeadTx(ctx, tx, computerID)
				},
				// The pin on the WAL ends when the stream does; the bound backfills it
				// then (on every path), not when the whole capture returns.
				ReadReleased: func(ctx context.Context) {
					if head, err := store.Head(ctx, computerID); err == nil && head != nil {
						releasedHead.Store(head.Sequence)
					}
					store.CheckpointAfterLongRead(ctx, "snapshot capture")
				},
				WALBytes: func() int64 { return store.WALStats().WALBytes },
				// The WAL the pin holds grows on the data volume; the stream checks
				// this every 64 MiB it reads and the capture every minute.
				FreeSpace: func() error {
					free, err := snapshotFreeBytes(dataDir)
					if err != nil {
						return fmt.Errorf("snapshot: cannot measure free space during the capture (fails closed): %w", err)
					}
					if free < snapshotFreeFloor {
						return fmt.Errorf("snapshot: free space %d bytes on the data volume fell under the %d-byte floor during the capture (the pinned WAL)", free, snapshotFreeFloor)
					}
					return nil
				},
				Partitions:  telemetryPartitionSource(store),
				Cipher:      rt.privateArtifactCipher,
				Chunks:      chunks,
				Appender:    rt.eventAppender,
				MaxDuration: maxDuration,
				PinDeadline: snapshotPinDeadline,
			})
			if err == nil {
				if released := releasedHead.Load(); released > ref.Record.StoreHeadSequence {
					ref.Stats.PinEvents = released - ref.Record.StoreHeadSequence
				}
			}
			return ref, err
		},
	}, true
}

// snapshotProducerTick is one tick on the runtime, with its logging.
func (rt *Runtime) snapshotProducerTick(ctx context.Context, now time.Time) {
	deps, ok := rt.snapshotDepsForRuntime()
	if !ok {
		return
	}
	p := &rt.snapshots
	outcome, detail := p.run(ctx, deps, snapshotSettingsFromEnv(os.Getenv), now)
	switch outcome {
	case snapshotTaken:
		log.Printf("runtime: snapshot taken: %s", detail)
	case snapshotRefused, snapshotFailed, snapshotBackoff:
		// A refusal repeats at every tick until it clears; say it when it
		// changes so the log stays readable.
		key := string(outcome) + ": " + detail
		p.mu.Lock()
		changed := p.lastWhy != key
		p.lastWhy = key
		p.mu.Unlock()
		if changed {
			switch outcome {
			case snapshotFailed, snapshotBackoff:
				log.Printf("runtime: snapshot %s: %s", outcome, detail)
			default:
				log.Printf("runtime: snapshot %s (retry in %s): %s", outcome, snapshotTickEvery, detail)
			}
		}
	default:
		// Not due, but the replay bound is exceeded and the capture floor holds
		// the next capture back: say so when it starts, not at every tick.
		if st := p.status(now); outcome == snapshotNotDue && st.FloorLimited {
			key := "floor: " + p.floorKey()
			p.mu.Lock()
			changed := p.lastWhy != key
			p.lastWhy = key
			p.mu.Unlock()
			if changed {
				log.Printf("runtime: snapshot floor-limited: %s", st.FloorWhy)
			}
			return
		}
		p.mu.Lock()
		p.lastWhy = ""
		p.mu.Unlock()
	}
}

// startSnapshotProducer starts the standing intent: after the boot settles,
// and then at every tick, take a snapshot if one is due. Like the checkpoint
// cadence it binds nothing at boot: a restart resumes no work, the next tick
// is new work.
func (rt *Runtime) startSnapshotProducer(ctx context.Context) {
	if rt == nil {
		return
	}
	p := &rt.snapshots
	p.mu.Lock()
	if p.stop != nil {
		p.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	p.stop = stop
	p.mu.Unlock()
	rt.wg.Add(1)
	go func() {
		defer rt.wg.Done()
		timer := time.NewTimer(snapshotInitialDelay)
		defer timer.Stop()
		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		ticker := time.NewTicker(snapshotTickEvery)
		defer ticker.Stop()
		for {
			rt.snapshotProducerTick(ctx, time.Now().UTC())
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (rt *Runtime) stopSnapshotProducer() {
	if rt == nil {
		return
	}
	p := &rt.snapshots
	p.mu.Lock()
	stop := p.stop
	p.stop = nil
	p.mu.Unlock()
	if stop != nil {
		close(stop)
	}
}
