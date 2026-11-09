package agentcore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/actgate"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/gomemlimit"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// desk_go_eval (R3b): the host desk-cell carrier's sole tool for a non-capsule
// desk. It evaluates model-authored Go source in a killable host session worker
// — never in the daemon's address space — and reduces the cell's staged intents
// through the same ledger path capsule_go_eval uses.
// The worker is re-executed from the daemon binary (autoputer desk-session);
// SpawnDeskSessionWorker supplies the socketpair + process-group hardening.

const deskWorkerArg = "desk-session"

// deskWorkerMemoryLimitBytes is the D2 memory cap applied to every desk
// session worker via RLIMIT_AS (R3r): a model-authored cell cannot grow
// its address space unboundedly and OOM the daemon. The worker re-execs
// the autoputer binary — its linked address space needs headroom far
// above a cell's working set — so the cap binds growth, not startup
// size (8 GiB virtual; Linux staging enforced).
const deskWorkerMemoryLimitBytes uint64 = 8 << 30 // 8 GiB

// The management cell deadline (the deskmanifest rows' CellDeadline, 300 s) bounds a management cell whose source sets no
// timeout_ms (management delegation S1, design round 2 R2-19). Before it, a
// management cell ran until its worker died: run 305a582d's cell 6 ran
// 860,565 ms. The value is max(300 s, 2 x the p99 management cell duration
// S0 measured on the live paper): p99 4.6 s over 295 cells (10-14
// 06:51-09:12 UTC) and 1.0 s over 1,582 cells (10-13 15:00 to 10-14 09:12),
// with one cell over 60 s, the 860 s one. So the 300 s floor governs.

// managementCellDeadlineEnv overrides the management cell deadline with a Go
// duration; "0" (any zero duration) or "off" removes it. A value that does not parse, or a
// negative one, keeps the default, so a typo cannot unbound the cell.
const managementCellDeadlineEnv = "CHOIR_MANAGEMENT_CELL_DEADLINE"

// deskCellHostDeadline is the host's bound on a cell that sets no
// timeout_ms, or 0 for none. Only management has one: other desks wait on
// legitimate long calls (research's choir.SearchWait), and the design scopes
// S1 to management.
func deskCellHostDeadline(deskRole string) time.Duration {
	// The bound is the desk row's cell_deadline (F11); a management task's
	// row carries management's (S2a). A desk with none waits.
	d, ok := deskmanifest.Lookup(deskRole)
	if !ok || d.CellDeadline <= 0 {
		return 0
	}
	raw := strings.TrimSpace(os.Getenv(managementCellDeadlineEnv))
	switch strings.ToLower(raw) {
	case "":
		return d.CellDeadline
	case "0", "off":
		return 0
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed < 0 {
		return d.CellDeadline
	}
	return parsed
}

// A desk's session outlives its run (PD2): the next run of the same agent
// evaluates in the same REPL, so a follow-up continues from the values the
// desk's earlier runs defined. Sessions are process memory: a restart, a
// failed or timed-out cell, or deskSessionRetention without a cell ends one,
// and nothing resumes it.
const (
	deskSessionRetention = 30 * time.Minute
	// deskSessionMaxIdle bounds the idle sessions a computer keeps; beyond
	// it the least recently used idle session is closed.
	deskSessionMaxIdle = 12
	// deskSessionAcquireWait bounds a run's wait for its desk's session
	// while another run's cell holds it. The caller's context usually ends
	// sooner (the activation budget); this bound holds when it has none.
	// A management cell is bounded at its host deadline; a cell of another
	// desk without timeout_ms is not, so a waiter must not wait as long as
	// it runs.
	deskSessionAcquireWait = 10 * time.Minute
)

type deskSession struct {
	worker   *yaegikernel.DeskSessionWorker
	cfg      yaegikernel.SessionWorkerConfig
	started  time.Time
	lastUsed time.Time
	cells    int
	runs     []string
	// busyRun is the run whose cell is evaluating now; "" when idle.
	// busySince is when that cell took the session.
	busyRun   string
	busySince time.Time
	// freed is closed when the busy cell ends, waking runs that wait for
	// this session.
	freed chan struct{}
	// searches is the session's async search table (desk_search.go); its
	// streams end with the session.
	searches *deskSearches
	// labels is the union of the labels of everything any run read in this
	// session (MA3.5): the REPL keeps values across runs, so a later run's
	// acts carry what an earlier run read into them. It lives and dies with
	// the session.
	labels types.LabelSet
	// rowHold keeps the desk row of the worker's epoch retained while the
	// worker lives (deskmanifest.RetainEpoch); close releases it.
	rowHold func()
}

// close ends the session: its searches' streams, then its worker. Runs
// waiting for its busy cell are woken: they find the session gone and
// start a new one. The caller holds the pool mutex, or has removed the
// session from the pool under it (endIdleDeskSession).
func (s *deskSession) close() {
	if s.freed != nil {
		close(s.freed)
		s.freed = nil
	}
	s.searches.close()
	s.worker.Close()
	if s.rowHold != nil {
		s.rowHold()
		s.rowHold = nil
	}
}

// deskReaperTimer is the part of *time.Timer the reaper uses.
type deskReaperTimer interface{ Stop() bool }

type deskSessionWorkers struct {
	mu        sync.Mutex
	sessions  map[string]*deskSession
	retention time.Duration
	maxIdle   int
	// maxWait bounds a wait for a busy session (deskSessionAcquireWait).
	maxWait time.Duration
	now     func() time.Time
	// afterFunc arms the idle reaper (time.AfterFunc unless a test replaces
	// it); reaper is the armed timer, nil when nothing idle is being watched.
	afterFunc func(time.Duration, func()) deskReaperTimer
	reaper    deskReaperTimer
	// spawning marks a session whose worker is starting outside the mutex;
	// its done channel closes when the start ends. release and closeAll
	// cancel it (MA6.4 r4): a start released meanwhile lands no session.
	spawning map[string]*deskSpawn
	// label prefixes the desk_session a trace event names. A choir.Research
	// child's pool is keyed on its root run like its siblings', so without a
	// label two sibling children read as two workers on one desk session.
	label string
}

// newResearchChildDeskWorkers returns the pool one choir.Research child runs
// in, labelled with the child's key for the trace.
func newResearchChildDeskWorkers(role, childKey string) *deskSessionWorkers {
	w := newDeskSessionWorkers()
	w.label = role + ":" + childKey + "/"
	return w
}

// sessionLabel is the desk_session a trace event names for key.
// sessionLabels is the label union of the lease's live session.
func (m *deskSessionWorkers) sessionLabels(lease deskSessionLease) types.LabelSet {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s := m.sessions[lease.key]; s != nil && s.worker == lease.worker {
		return s.labels
	}
	return nil
}

// taintSession folds labels into the lease's live session; a session that
// was replaced or closed is not the lease's and is left alone.
func (m *deskSessionWorkers) taintSession(lease deskSessionLease, labels types.LabelSet) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s := m.sessions[lease.key]; s != nil && s.worker == lease.worker {
		s.labels = s.labels.Union(labels)
	}
}

func (m *deskSessionWorkers) sessionLabel(key string) string {
	return m.label + key
}

func newDeskSessionWorkers() *deskSessionWorkers {
	return &deskSessionWorkers{
		sessions:  map[string]*deskSession{},
		retention: deskSessionRetention,
		maxIdle:   deskSessionMaxIdle,
		maxWait:   deskSessionAcquireWait,
		now:       time.Now,
	}
}

// deskSessionWorkers returns the Runtime's shared worker pool, lazily
// allocated so desk cells never spawn a worker before one is requested.
func (rt *Runtime) deskSessionWorkers() *deskSessionWorkers {
	rt.deskWorkersMu.Lock()
	defer rt.deskWorkersMu.Unlock()
	if rt.deskWorkers == nil {
		rt.deskWorkers = newDeskSessionWorkers()
	}
	return rt.deskWorkers
}

// deskSessionLease is one cell's hold on a session; done releases it.
type deskSessionLease struct {
	key    string
	worker *yaegikernel.DeskSessionWorker
	// notice tells the desk whether its session is new or continuing, on
	// the first cell of each run of a persistent session.
	notice string
	// waitedFor names the run whose cell this one waited for; waited is how
	// long. Both are zero when the session was free.
	waitedFor string
	waited    time.Duration
	// queued is how long the new worker's spawn waited for desks memory.
	queued time.Duration
}

// deskSessionKey names the session a desk cell evaluates in. A desk run (it
// has a run record) uses its agent's session, so a later run of the same desk
// continues where the earlier one left off. A cell without a run record (a
// choir.Research child) keeps a session of its own activation: a child that
// joined its parent's session would wait on the parent's cell forever.
//
// The key carries the run's desk epoch (@e<N>, the install event sequence of
// the desk row the run started under) for a package desk, so a run activated
// after an update starts a new worker and a run in flight keeps its own.
// A built-in desk is at epoch 0 and its key is unchanged.
func deskSessionKey(execCtx toolregistry.ExecutionContext, deskEpoch uint64) (string, bool) {
	if execCtx.RunRecord != nil {
		if id := strings.TrimSpace(execCtx.RunRecord.AgentID); id != "" {
			if deskEpoch > 0 {
				return fmt.Sprintf("agent:%s@e%d", id, deskEpoch), true
			}
			return "agent:" + id, true
		}
	}
	return deskWorkerActivationID(execCtx), false
}

// acquire returns the session for key, spawning one if it is missing, dead,
// or configured differently, and marks it busy for runID's cell. A desk has
// one REPL: a session that is mid-cell for another run is waited for, outside
// the pool mutex and bounded by ctx, and then serves runID in the same
// worker. The lease's notice names the run it shared with.
func (m *deskSessionWorkers) acquire(ctx context.Context, key string, persistent bool, runID string, cfg yaegikernel.DeskSessionWorkerConfig) (deskSessionLease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sessions == nil {
		m.sessions = map[string]*deskSession{}
	}
	shared := ""
	waitStart := time.Now()
	waitCtx := ctx
	if m.maxWait > 0 {
		var cancel context.CancelFunc
		waitCtx, cancel = context.WithTimeout(ctx, m.maxWait)
		defer cancel()
	}
	var s *deskSession
	var queued time.Duration
	for {
		for {
			s := m.sessions[key]
			if s == nil || s.busyRun == "" || s.busyRun == runID {
				break
			}
			other, since, freed := s.busyRun, s.busySince, s.freed
			m.mu.Unlock()
			select {
			case <-freed:
				m.mu.Lock()
				// A release (a dead worker) also wakes the waiters: the values
				// are then gone, and the next pass starts a new session, so
				// only a session still in place was shared.
				if m.sessions[key] == s {
					shared = other
				}
			case <-waitCtx.Done():
				m.mu.Lock()
				cause := ctx.Err()
				if cause == nil {
					cause = context.DeadlineExceeded
				}
				return deskSessionLease{}, fmt.Errorf("this desk's session is busy with run %s's cell for %s: gave up after %s: %w",
					other, m.clock().Sub(since).Round(time.Second), time.Since(waitStart).Round(time.Millisecond), cause)
			}
		}
		m.reclaimLocked(m.clock())
		s = m.sessions[key]
		if s != nil && (s.worker.Dead() || !sameDeskSessionConfig(s.cfg, cfg.Session)) {
			s.close()
			delete(m.sessions, key)
			s = nil
			// Nothing the run waited for survives: no shared values to claim.
			shared = ""
		}
		if s != nil {
			break
		}
		// Another cell is starting this session's worker: wait for it, then
		// look again.
		if starting := m.spawning[key]; starting != nil {
			m.mu.Unlock()
			select {
			case <-starting.done:
				m.mu.Lock()
				continue
			case <-ctx.Done():
				m.mu.Lock()
				return deskSessionLease{}, fmt.Errorf("this desk's session was starting for another cell: gave up after %s: %w", time.Since(waitStart).Round(time.Millisecond), ctx.Err())
			}
		}
		// The worker holds its epoch's row until it ends, whatever the
		// registry does meanwhile (plan MA4.2, B2).
		rowHold, held := deskmanifest.RetainEpoch(cfg.Session.Role, cfg.Session.DeskEpoch)
		if !held {
			return deskSessionLease{}, fmt.Errorf("desk %q has no row at epoch %d: it was updated or retired before this cell started", cfg.Session.Role, cfg.Session.DeskEpoch)
		}
		// The spawn runs outside the pool mutex: it may wait for desks
		// memory (MA6.4 r3), bounded by the cell's own context, and a wait
		// is relieved by ending idle sessions, which takes this mutex.
		parent := cfg.Context
		if parent == nil {
			parent = ctx
		}
		spawnCtx, cancelSpawn := context.WithCancel(parent)
		starting := &deskSpawn{done: make(chan struct{}), cancel: cancelSpawn}
		if m.spawning == nil {
			m.spawning = map[string]*deskSpawn{}
		}
		m.spawning[key] = starting
		cfg.Context = spawnCtx
		m.mu.Unlock()
		w, err := spawnDeskSessionWorker(cfg)
		m.mu.Lock()
		cancelSpawn()
		delete(m.spawning, key)
		close(starting.done)
		if err == nil && starting.released {
			// Released (or every session closed) while it started: the
			// worker serves nothing.
			w.Close()
			err = errors.New("this desk's session was released while its worker started")
		}
		if err != nil {
			rowHold()
			m.forgetIfEmptyLocked()
			return deskSessionLease{}, err
		}
		queued = w.AdmissionQueued()
		now := m.clock()
		s = &deskSession{worker: w, cfg: cfg.Session, started: now, lastUsed: now, searches: newDeskSearches(), rowHold: rowHold}
		m.sessions[key] = s
		registerDeskPool(m)
		break
	}
	now := m.clock()
	lease := deskSessionLease{key: key, worker: s.worker, queued: queued}
	if shared != "" {
		lease.waitedFor = shared
		lease.waited = time.Since(waitStart)
	}
	if persistent && !slices.Contains(s.runs, runID) {
		if s.cells == 0 {
			lease.notice = "new session: nothing from this desk's earlier runs is defined. A session ends on a restart, a failed or timed-out cell, " + m.retentionLocked().String() + " without a cell, or idle while another desk waits for memory."
		} else {
			lease.notice = fmt.Sprintf("continuing session: values defined by this desk's earlier cells are still defined (%d cells over %d earlier runs, started %s ago). Print one before relying on it.",
				s.cells, len(s.runs), now.Sub(s.started).Round(time.Second))
		}
	}
	if shared != "" {
		if lease.notice != "" {
			lease.notice += " "
		}
		lease.notice += fmt.Sprintf("This cell waited for run %s's cell: one REPL per desk, so values that run defines are visible here and the same names collide.", shared)
	}
	if !slices.Contains(s.runs, runID) {
		s.runs = append(s.runs, runID)
	}
	if s.busyRun != runID {
		s.busySince = m.clock()
	}
	s.busyRun = runID
	// A run that re-enters its own busy session keeps the channel its
	// waiters already wait on.
	if s.freed == nil {
		s.freed = make(chan struct{})
	}
	return lease, nil
}

// done ends a cell's hold on its session and wakes any run waiting for it.
// It acts only on the lease's own session: a cell whose worker died
// released its session, a waiter woken by that release may already hold a
// new session under the same key, and the dead cell's done must not free
// it.
func (m *deskSessionWorkers) done(lease deskSessionLease) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s := m.sessions[lease.key]; s != nil && s.worker == lease.worker {
		s.busyRun, s.busySince = "", time.Time{}
		s.cells++
		s.lastUsed = m.clock()
		if s.freed != nil {
			close(s.freed)
			s.freed = nil
		}
	}
	m.reclaimLocked(m.clock())
	m.armReaperLocked()
	m.forgetIfEmptyLocked()
	// A spawn waiting for desks memory may now end this idle session.
	yaegikernel.NotifyDeskSessionIdle()
}

// armReaperLocked arms one timer for the earliest moment an idle session
// passes retention, so a computer that goes quiet still gives its workers
// back: reclaimLocked alone runs only on acquire and done. It does nothing
// while a timer is armed or no session is idle. A busy session is not
// watched; its done re-arms the reaper when the cell ends.
func (m *deskSessionWorkers) armReaperLocked() {
	if m.reaper != nil {
		return
	}
	now := m.clock()
	wait := time.Duration(-1)
	for _, s := range m.sessions {
		if s.busyRun != "" {
			continue
		}
		left := s.lastUsed.Add(m.retentionLocked()).Sub(now)
		if left < time.Second {
			left = time.Second
		}
		if wait < 0 || left < wait {
			wait = left
		}
	}
	if wait < 0 {
		return
	}
	after := m.afterFunc
	if after == nil {
		after = func(d time.Duration, f func()) deskReaperTimer { return time.AfterFunc(d, f) }
	}
	m.reaper = after(wait, m.reap)
}

// reap is the reaper timer's callback: it closes idle sessions past retention
// (never a busy one, reclaimLocked skips them) and re-arms for the rest.
func (m *deskSessionWorkers) reap() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reaper = nil
	m.reclaimLocked(m.clock())
	m.armReaperLocked()
	m.forgetIfEmptyLocked()
}

// reclaimLocked closes idle sessions past retention, then the least recently
// used idle sessions beyond maxIdle, cancelling their searches. A busy
// session is never reclaimed.
func (m *deskSessionWorkers) reclaimLocked(now time.Time) {
	idle := make([]string, 0, len(m.sessions))
	for key, s := range m.sessions {
		if s.busyRun != "" {
			continue
		}
		if s.worker.Dead() || now.Sub(s.lastUsed) > m.retentionLocked() {
			s.close()
			delete(m.sessions, key)
			continue
		}
		idle = append(idle, key)
	}
	limit := m.maxIdle
	if limit <= 0 {
		limit = deskSessionMaxIdle
	}
	if len(idle) <= limit {
		return
	}
	sort.Slice(idle, func(i, j int) bool { return m.sessions[idle[i]].lastUsed.Before(m.sessions[idle[j]].lastUsed) })
	for _, key := range idle[:len(idle)-limit] {
		m.sessions[key].close()
		delete(m.sessions, key)
	}
}

func (m *deskSessionWorkers) clock() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now()
}

func (m *deskSessionWorkers) retentionLocked() time.Duration {
	if m.retention > 0 {
		return m.retention
	}
	return deskSessionRetention
}

// count reports live sessions.
func (m *deskSessionWorkers) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}

// release terminates and forgets the session's worker and cancels its
// searches.
func (m *deskSessionWorkers) release(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s := m.sessions[key]; s != nil {
		s.close()
		delete(m.sessions, key)
	}
	if sp := m.spawning[key]; sp != nil {
		sp.released = true
		sp.cancel()
	}
	if len(m.sessions) == 0 && m.reaper != nil {
		m.reaper.Stop()
		m.reaper = nil
	}
	m.forgetIfEmptyLocked()
}

// closeAll terminates every session.
func (m *deskSessionWorkers) closeAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.reaper != nil {
		m.reaper.Stop()
		m.reaper = nil
	}
	for key, s := range m.sessions {
		s.close()
		delete(m.sessions, key)
	}
	for _, sp := range m.spawning {
		sp.released = true
		sp.cancel()
	}
	m.forgetIfEmptyLocked()
}

func sameDeskSessionConfig(a, b yaegikernel.SessionWorkerConfig) bool {
	return a.Role == b.Role && a.DeskEpoch == b.DeskEpoch && a.ComputerID == b.ComputerID && a.AllowedRoot == b.AllowedRoot &&
		bytes.Equal(a.GoModules, b.GoModules)
}

// newDeskGoEvalTool evaluates model-authored Go source for a non-capsule desk
// inside a dedicated host session worker. It mirrors newCapsuleGoEvalTool but
// carries no capsule obligation: the desk's authority is its profile/role, and
// the containment boundary is the subprocess, not a guest VM. Staged intents
// reduce through rlmReductionForDeskCall onto the canonical ledger.
func newDeskGoEvalTool(rt *Runtime, workers *deskSessionWorkers, deskRole string) toolregistry.Tool {
	type args struct {
		Source    string `json:"source"`
		TimeoutMS int    `json:"timeout_ms"`
	}
	return toolregistry.Tool{
		Name:        "desk_go_eval",
		Description: "Evaluate model-authored Go source for this desk in a killable host subprocess (restricted stdlib, per-role choir verbs). The desk's session continues across its runs, so values from earlier runs stay defined until a restart, a failed or timed-out cell, or " + deskSessionRetention.String() + " idle; the first cell of each run says whether the session is new or continuing.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"source":     map[string]any{"type": "string", "description": "Raw Go source for one REPL cell; stage desk acts with choir.* verbs."},
			"timeout_ms": map[string]any{"type": "integer", "description": "Optional evaluation timeout in milliseconds; leaving it unset is normal. A cell that reaches it is killed with its session, so values from earlier cells are lost. Host calls such as choir.Read take several seconds each (up to about 10 s), and choir.SearchWait blocks as long as you ask."},
		}, []string{"source"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			execCtx := toolregistry.ExecutionContextFrom(ctx)
			var input args
			if err := json.Unmarshal(raw, &input); err != nil {
				return "", err
			}
			if strings.TrimSpace(input.Source) == "" {
				return "", fmt.Errorf("desk_go_eval: source is required")
			}
			activationID := deskWorkerActivationID(execCtx)
			if activationID == "" {
				return "", fmt.Errorf("desk_go_eval: no activation identity for desk worker")
			}
			deskEpoch := rt.pinDeskEpoch(execCtx.RunID, deskRole)
			sessionKey, persistent := deskSessionKey(execCtx, deskEpoch)
			workerCfg := yaegikernel.DeskSessionWorkerConfig{
				Bin:       deskWorkerBinary(),
				WorkerArg: deskWorkerArg,
				Session: yaegikernel.SessionWorkerConfig{
					AllowedPackages:  yaegikernel.DeskAllowedPackages(deskRole),
					ComputerID:       execCtx.ComputerID,
					ActivationID:     activationID,
					Epoch:            deskWorkerEpoch(execCtx),
					DeskEpoch:        deskEpoch,
					AllowedRoot:      deskWorkerRoot(execCtx),
					Role:             deskRole,
					MemoryLimitBytes: deskWorkerMemoryLimitBytes,
					// A print over the bound reaches the transcript cut; the
					// whole text stays in the session for choir.CellOut (LR
					// lever 6).
					KeepCellOutput: true,
				},
				ProcessGroup: true,
				// The guest image sets CHOIR_DESK_CELL_CONFINEMENT=required:
				// the worker runs as its desk's own user in an empty network
				// namespace under Landlock and seccomp, or does not run.
				Confinement: yaegikernel.CellConfinement{Mode: yaegikernel.DeskCellConfinementModeFromEnv(os.Getenv)},
				// Workers get a clean environment; an operator's worker
				// GOMEMLIMIT override is the one memory setting passed through.
				ExtraEnv: gomemlimit.WorkerEnv(os.Getenv),
				// A refused frame (outside the desk's table, a forged token, a
				// stale epoch) is a trace event, not only a model-visible
				// error: the refusal is evidence (SEC-02).
				Refused: func(refCtx context.Context, r yaegikernel.FrameRefusal) {
					rt.recordDeskRefusal(refCtx, execCtx, workers.sessionLabel(sessionKey), r)
				},
				// Egress services the cell's host-mediated verbs
				// (choir.* → StreamBrokerEgress). Evidence/run-memory actions
				// resolve through rt-bound tools; the management desk's
				// product_api action serves through the canonical route
				// table; network/content actions go to researchDeps.HostEgress
				// — one deps surface, never an open socket.
				// Like Emit, the acting run comes from the evaluating cell's
				// ctx: the session outlives the run that spawned it (PD2).
				Egress: func(egrCtx context.Context, action yaegikernel.BrokerAction, payload json.RawMessage) (json.RawMessage, error) {
					cellExec := toolregistry.ExecutionContextFrom(egrCtx)
					if cellExec.RunID == "" && cellExec.RunRecord == nil {
						cellExec = execCtx
					}
					// The transport already refused any action outside the
					// desk's manifest row (yaegikernel admitFrame). This is the
					// second check, on the host handler itself, so no caller of
					// the handler can serve a desk an action its row lacks
					// (SEC-02, defense in depth).
					if !deskmanifest.FrameAllowedAt(deskRole, deskEpoch, string(action)) {
						return nil, fmt.Errorf("refused: desk %q may not call %s", deskRole, action)
					}
					if err := yaegikernel.ValidatePayload(action, payload); err != nil {
						return nil, fmt.Errorf("refused: %w", err)
					}
					started := time.Now().UTC()
					requestID, pid := yaegikernel.EgressFrameFromContext(egrCtx)
					var out json.RawMessage
					var err error
					if action == yaegikernel.ActionSearchStart {
						// An async search never reaches deskEgress, so its query is
						// gated here, as an egress call is.
						if gateErr := rt.gateEgressCall(egrCtx, deskRole, cellExec, action, payload); gateErr != nil {
							return nil, gateErr
						}
						// The search's results reach the cell from the moment it
						// starts (forwardSearch), so the run is tainted before,
						// not after: a sibling call (SaveEvidence) must not
						// stamp what it writes without world_untrusted (review
						// r1, S1).
						rt.taintRun(cellExec.RunID, worldReadLabels)
						// An async search belongs to this desk session: its
						// stream outlives the cell and ends with the session.
						out, err = workers.startSearch(egrCtx, sessionKey, yaegikernel.EgressWorkerFromContext(egrCtx), rt.searchClient(), payload, func(done searchDone) {
							ev := map[string]any{
								"verb": "search_done", "search_id": done.id, "desk_session": workers.sessionLabel(sessionKey), "worker_pid": pid,
								"run_id": cellExec.RunID, "started_at": done.started.UTC(), "finished_at": time.Now().UTC(),
								"answered": done.answered, "arrived": done.arrived, "total": done.total,
							}
							if len(done.pending) > 0 {
								ev["pending"] = done.pending
							}
							if done.err != "" {
								ev["error"] = done.err
							}
							rt.recordDeskEvent(context.WithoutCancel(egrCtx), cellExec, types.EventDeskEgress, ev)
							if len(done.providers) > 0 {
								rt.recordDeskEvent(context.WithoutCancel(egrCtx), cellExec, types.EventSearchServed, map[string]any{
									"schema": types.SearchServedSchema, "role": cellExec.Role, "providers": done.providers,
								})
							}
						})
						if err == nil {
							// An async search bypasses deskEgress; its read taints here.
							rt.noteEgressRead(egrCtx, cellExec, action, payload, out)
						}
					} else {
						out, err = rt.deskEgress(egrCtx, deskRole, cellExec, action, payload)
					}
					ev := map[string]any{
						"request_id": requestID, "response_request_id": requestID,
						"verb": string(action), "desk_session": workers.sessionLabel(sessionKey), "worker_pid": pid,
						"run_id": cellExec.RunID, "started_at": started, "finished_at": time.Now().UTC(),
						"is_error": err != nil,
					}
					if err != nil {
						ev["error"] = err.Error()
					}
					// The row rides the cell's desk.cell event (journal cut
					// R4); a row with no open cell is appended on its own.
					if !cellEgressLogFrom(egrCtx).add(ev) {
						rt.recordDeskEvent(egrCtx, cellExec, types.EventDeskEgress, ev)
					}
					return out, err
				},
			}
			lease, err := workers.acquire(ctx, sessionKey, persistent, activationID, workerCfg)
			if err != nil {
				return "", fmt.Errorf("desk_go_eval: spawn worker: %w", err)
			}
			defer workers.done(lease)
			// Runs before done: what the run read joins the session's union.
			defer func() { workers.taintSession(lease, rt.runLabelSet(execCtx.RunID)) }()
			w := lease.worker
			reduction := rlmReductionForDeskCall(ctx, rt)
			// A Texture cell works against its frame (Cut 2 2a): the
			// revision the frame showed, its blocks by alias for Write, and
			// the frame's record sources for {{cite}} handles.
			var docSnapshot *yaegikernel.DocSnapshot
			if deskCommits(deskRole, TextureRevisionKind) && execCtx.RunRecord != nil {
				docSnapshot = textureDocSnapshotForRun(ctx, rt, execCtx.RunRecord, reduction.records)
				if docSnapshot != nil {
					// The frame showed the cell the owner's document.
					rt.taintRun(execCtx.RunID, documentReadLabels)
				}
			}
			// R4: the acting desk's commitment pack rides the same frame —
			// score-free by construction (types.ActingPack), so the
			// epistemic boundary holds on the wire.
			pack := actingCommitmentPackForDesk(ctx, rt, execCtx)
			// RLM prompt-as-variable: pending update_coagent records ride the
			// frame so choir.Updates() exposes them inside the cell. The chat
			// wake turn carries only ids.
			updates, updatesRemaining, updatesTooLarge := pendingCellUpdatesWindow(ctx, rt, execCtx)
			// A story's research desk reads its story's entity analysis,
			// built at the run's first cell (slice c0 §4).
			analysis := rt.storyAnalysisFor(ctx, deskRole, execCtx)
			if reduction.active {
				rt.armCellTerminalDeadline(ctx, reduction)
			}
			deskCellFramePrepared()
			// The cell's bound is its own timeout_ms when set, else the host's
			// deadline for the desk (management only, S1). It is armed after
			// the host has prepared the frame, so it measures the cell: armed
			// before, a second of store reads on the paper left a 1000 ms
			// cell 3 ms to run. It goes on the eval context, not ctx:
			// armCellTerminalDeadline keys on the outer context's deadline,
			// which stays the activation's.
			evalCtx := ctx
			hostDeadline := time.Duration(0)
			if input.TimeoutMS <= 0 {
				hostDeadline = deskCellHostDeadline(deskRole)
			}
			if input.TimeoutMS > 0 || hostDeadline > 0 {
				bound := hostDeadline
				if input.TimeoutMS > 0 {
					bound = time.Duration(input.TimeoutMS) * time.Millisecond
				}
				var cancel context.CancelFunc
				evalCtx, cancel = context.WithTimeout(ctx, bound)
				defer cancel()
			}
			egressLog := &cellEgressLog{}
			evalCtx = withCellEgressLog(evalCtx, egressLog)
			evalStart := time.Now()
			untrustedBefore := rt.runUntrustedReads(execCtx.RunID)
			// A run on a session that earlier runs read into starts with
			// their labels: the values they defined are still defined.
			rt.seedRunLabels(execCtx.RunID, workers.sessionLabels(lease))
			shownCalibration, delivery := rt.calibrationReadableForRun(ctx, execCtx.RunRecord)
			res, evalErr := w.EvalFrame(evalCtx, yaegikernel.SessionFrame{Source: input.Source, RunID: execCtx.RunID, Inbox: reduction.inbox, Doc: docSnapshot, Pack: pack,
				Updates: updates, Records: reduction.records.view, RecordsAfter: reduction.records.after, Analysis: analysis, Calibration: shownCalibration,
				Backlog: frameBacklog(reduction.records.remaining, updatesRemaining, updatesTooLarge)})
			// The host consumes only the updates the cell was shown whole.
			// A cell that failed consumes nothing.
			if evalErr == nil {
				sentUpdates := w.LastUpdatesSent()
				rt.noteUpdatesShown(execCtx.RunID, sentUpdates, updatesRemaining > 0 || len(sentUpdates) < len(updates))
			}
			if res.CalibrationRead {
				rt.noteCalibrationRead(ctx, execCtx, shownCalibration, delivery)
			}
			elapsed := time.Since(evalStart)
			// The cell's own bound fired, not the run's cancellation or the
			// activation's deadline.
			ownBoundFired := evalErr != nil && ctx.Err() == nil && evalCtx.Err() == context.DeadlineExceeded
			cellEvent := map[string]any{
				"desk_session": workers.sessionLabel(lease.key), "worker_pid": w.PID(), "run_id": execCtx.RunID,
				"started_at": evalStart.UTC(), "finished_at": time.Now().UTC(),
				"duration_ms":    elapsed.Milliseconds(),
				"waited_for_run": lease.waitedFor, "waited_ms": lease.waited.Milliseconds(), "queued_ms": lease.queued.Milliseconds(), "confinement": w.ConfinementProfile(),
				"error": res.Error, "reuse": res.Reuse, "diag": res.DiagKind, "intents": len(res.Intents),
			}
			if hostDeadline > 0 {
				cellEvent["host_deadline_ms"] = hostDeadline.Milliseconds()
			}
			if rows := egressLog.seal(); len(rows) > 0 {
				cellEvent["egress"] = rows
			}
			// MA6.6: the cell's host-call count, and the rows past the
			// event's bound by verb.
			if calls, omitted := egressLog.counts(); calls > 0 {
				cellEvent["egress_calls"] = calls
				if len(omitted) > 0 {
					cellEvent["egress_omitted"] = omitted
				}
			}
			result := yaegikernel.SessionResult{}
			if evalErr != nil {
				cellEvent["error"] = evalErr.Error()
				// A dead worker is released before the event is recorded, so
				// the event and the log carry how it ended (R2-19, H1).
				workerExit := ""
				if w.Dead() {
					workers.release(lease.key)
					exit := w.ExitCause()
					workerExit = exit
					cellEvent["worker_exit"] = exit
					switch {
					case ownBoundFired && hostDeadline > 0:
						cellEvent["killed_by"] = "host_deadline"
						log.Printf("desk cell: %s cell deadline %s killed the cell of run %s after %d ms (desk session %s, worker pid %d); worker exit: %s",
							deskRole, hostDeadline, execCtx.RunID, elapsed.Milliseconds(), workers.sessionLabel(lease.key), w.PID(), exit)
					case ownBoundFired:
						cellEvent["killed_by"] = "timeout_ms"
						log.Printf("desk cell: %s cell of run %s reached its own timeout_ms=%d after %d ms (worker pid %d); worker exit: %s",
							deskRole, execCtx.RunID, input.TimeoutMS, elapsed.Milliseconds(), w.PID(), exit)
					default:
						log.Printf("desk cell: %s worker of run %s ended after %d ms (worker pid %d): %v; worker exit: %s",
							deskRole, execCtx.RunID, elapsed.Milliseconds(), w.PID(), evalErr, exit)
					}
				}
				result.Error = evalErr.Error()
				if workerExit != "" && !ownBoundFired && ctx.Err() == nil {
					result.Error = cellWorkerDiedError(workerExit, elapsed)
					result.DurationMs = elapsed.Milliseconds()
				}
				if ownBoundFired {
					if hostDeadline > 0 {
						result.Error = cellHostDeadlineError(deskRole, hostDeadline, elapsed)
					} else {
						result.Error = cellTimeoutError(input.TimeoutMS, elapsed)
					}
					result.DurationMs = elapsed.Milliseconds()
				}
			} else {
				result = res
			}
			rt.recordDeskEvent(ctx, execCtx, types.EventDeskCell, cellEvent)
			if reduction.active && result.Error != "" {
				fate := cellFateFailure
				if evalCtx.Err() == context.DeadlineExceeded {
					fate = cellFateTimeout
				}
				if fateErr := reduction.abort(context.Background(), fate, result.Error); fateErr != nil {
					return "", fateErr
				}
			}
			if reduction.active && result.Error == "" {
				if !deskCommits(deskRole, TextureRevisionKind) {
					reduction.records = reduction.records.capToShown(rt.noteRecordsShown(execCtx.RunID, result.RecordsShown))
				}
				// What choir.Records() showed taints the run before its acts
				// are stamped (run_labels.go).
				rt.taintRun(execCtx.RunID, reduction.records.labelsThrough())
				if rerr := reduction.commit(ctx, result.Intents); rerr != nil {
					return "", rerr
				}
				for _, in := range reduction.receipt.Intents {
					receipt := fmt.Sprintf("rlm:%s:%d", in.Kind, in.Seq)
					if in.Inbox != "" {
						receipt += " to " + in.Inbox
					}
					result.Receipts = append(result.Receipts, receipt)
				}
				// A Texture cell records what it staged. The activation's end,
				// not the cell, answers a trigger no cell authored a turn for
				// (answerIdleTextureTrigger): a read-only cell that committed a
				// turn consumed reports the model had not read yet (rerun 12).
				if deskCommits(deskRole, TextureRevisionKind) {
					rt.noteTextureCell(execCtx.RunID, result.Intents)
				}
			}
			// What a cell printed after it read outside content is data, not
			// instructions (run_context.go frameLabelled).
			stdout := result.Stdout
			if rt.runUntrustedReads(execCtx.RunID) != untrustedBefore {
				stdout = frameLabelled(rt.runLabelSet(execCtx.RunID), stdout)
			}
			cell := map[string]any{
				"stdout":   stdout,
				"stderr":   result.Stderr,
				"error":    result.Error,
				"duration": result.DurationMs,
				"reuse":    result.Reuse,
				"diag":     result.DiagKind,
				"receipts": result.Receipts,
				"intents":  len(result.Intents),
			}
			if reduction.active && reduction.restateNotice != "" {
				cell["calibration_restate"] = reduction.restateNotice
			}
			// One count line per async search with new arrivals: the only
			// push of search arrivals to the model.
			if len(result.Searches) > 0 {
				cell["searches"] = result.Searches
			}
			if lease.notice != "" {
				cell["session"] = lease.notice
			}
			out, _ := json.Marshal(cell)
			return string(out), nil
		},
	}
}

// pendingCellUpdates loads the pending update_coagent records addressed to
// this desk's agent and maps them into the cell-visible PendingUpdate shape
// (RLM prompt-as-variable: the payload lives on the frame, not in chat).
func pendingCellUpdates(ctx context.Context, rt *Runtime, execCtx toolregistry.ExecutionContext) []yaegikernel.PendingUpdate {
	out, _, _ := pendingCellUpdatesWindow(ctx, rt, execCtx)
	return out
}

// frameBacklog is what a cell's window left out, or nil when it left nothing.
func frameBacklog(records int64, updates, tooLarge int) *yaegikernel.FrameBacklog {
	if records <= 0 && updates <= 0 && tooLarge <= 0 {
		return nil
	}
	return &yaegikernel.FrameBacklog{RecordsRemaining: records, UpdatesRemaining: updates, UpdatesTooLarge: tooLarge}
}

// pendingCellUpdatesWindow is pendingCellUpdates cut to the frame's updates
// budget, oldest first (the store lists them in arrival order), with the
// count left out. The Autopaper's management desk had 1000 pending updates;
// shown whole they overflowed the 4 MiB frame. The host consumes only the
// updates a cell was shown (noteUpdatesShown, wakeWindowIDs), so an update
// past the window stays pending for a later window.
func pendingCellUpdatesWindow(ctx context.Context, rt *Runtime, execCtx toolregistry.ExecutionContext) ([]yaegikernel.PendingUpdate, int, int) {
	packets := pendingCellPackets(ctx, rt, execCtx)
	// An update too large to be shown whole is settled with a visible cause
	// before the window is cut (update_window.go).
	tooLarge := rt.settleUnshowableUpdates(ctx, execCtx.RunRecord, packets)
	shown, remaining := windowUpdatePackets(packets)
	out := make([]yaegikernel.PendingUpdate, 0, len(shown))
	for _, u := range shown {
		out = append(out, pendingUpdateView(u))
	}
	return out, remaining, tooLarge
}

// pendingUpdateView is the update as the cell reads it.
func pendingUpdateView(u types.CoagentSourcePacket) yaegikernel.PendingUpdate {
	return yaegikernel.PendingUpdate{
		UpdateID:        strings.TrimSpace(u.UpdateID),
		FromAgentID:     strings.TrimSpace(u.AgentID),
		FromRole:        strings.TrimSpace(u.Role),
		ChannelID:       strings.TrimSpace(u.ChannelID),
		MessageSeq:      u.MessageSeq,
		WorkItemID:      strings.TrimSpace(u.WorkItemID),
		Packet:          u.Packet,
		HumanProjection: strings.TrimSpace(u.Content),
	}
}

func pendingCellPackets(ctx context.Context, rt *Runtime, execCtx toolregistry.ExecutionContext) []types.CoagentSourcePacket {
	if rt == nil || rt.store == nil || execCtx.RunRecord == nil {
		return nil
	}
	rec := execCtx.RunRecord
	ownerID := strings.TrimSpace(rec.OwnerID)
	agentID := strings.TrimSpace(rec.AgentID)
	if ownerID == "" || agentID == "" || !runSupportsCoagentUpdateInjection(rec) {
		return nil
	}
	updates, err := rt.pendingCoagentUpdatesForRun(ctx, rec, ownerID, agentID, 100)
	if err != nil {
		// The cell still runs, but an empty choir.Updates() must not be
		// indistinguishable from "no reports" (rerun 12).
		log.Printf("desk cell: pending updates for %s run %s unavailable: %v", agentID, rec.RunID, err)
		return nil
	}
	// The cell variable exposes the desk's pending backlog — every
	// deliverable update not yet disposed by a terminal write — regardless
	// of whether a chat wake turn already named it. Seen-ness exists to
	// dedupe chat injection, not to hide the payload from the cell.
	out := make([]types.CoagentSourcePacket, 0, len(updates))
	for _, u := range updates {
		if strings.TrimSpace(u.UpdateID) == "" || !cellUpdateVisible(rec, u) {
			continue
		}
		out = append(out, u)
	}
	return out
}

// cellUpdateVisible reports whether a desk cell's choir.Updates() shows the
// update. A Texture turn's cells see only what its commit consumes: a report
// that arrived after the activation was scheduled waits for the next turn,
// rather than being dispositioned here and refused at commit. Park and chat
// injection keep their own rule (coagentUpdateDeliverableForRun).
func cellUpdateVisible(rec *types.RunRecord, u types.CoagentSourcePacket) bool {
	if !coagentUpdateDeliverableForRun(rec, u) {
		return false
	}
	if isTextureAgentID(rec.AgentID) {
		if scheduled := int64(metadataIntValue(rec.Metadata, "scheduled_message_seq")); scheduled > 0 {
			return u.WithinTextureTurnBoundary(rec.RunID, scheduled)
		}
	}
	return true
}

// deskWorkerBinary is the executable the session worker re-executes as
// `autoputer desk-session` — normally the daemon's own os.Executable. The
// override lets tests point at a real compiled autoputer instead of the test
// harness binary.
var deskWorkerBinOverride string

func deskWorkerBinary() string {
	if deskWorkerBinOverride != "" {
		return deskWorkerBinOverride
	}
	if bin, err := os.Executable(); err == nil {
		return deskWorkerBinaryFor(bin)
	}
	return "autoputer"
}

// daemonSelfTestBinary is the daemon's own executable, for the self-test
// legs that run it with an argument only cmd/autoputer dispatches: the
// document-tool sandbox's helper and probe (doctool.HelperArg, ProbeArg)
// and the capsule seccomp child (capsule.SeccompSelfTestArg). Never
// deskWorkerBinary: the dedicated desk-worker serves only desk-session
// (docs/problems/doctool-selftest-runs-desk-worker-2026-10-22.md). The
// override lets tests point at a real compiled autoputer.
var daemonSelfTestBinOverride string

func daemonSelfTestBinary() string {
	if daemonSelfTestBinOverride != "" {
		return daemonSelfTestBinOverride
	}
	if bin, err := os.Executable(); err == nil {
		return bin
	}
	return "autoputer"
}

// deskWorkerBinaryFor is the worker to spawn for a daemon running as exe: the
// dedicated desk-worker binary beside it (no Dolt engine, no gRPC; RES-17) if
// there is an executable regular file, else exe itself, which serves
// `desk-session` too.
//
// A layered release runs the daemon from the updater's private store
// ($CHOIR_UPDATER_ROOT/store/<name>/...), under a root-only 0700 directory.
// The worker execs after it drops to the cell uid, which cannot search that
// directory, so every desk cell failed with EACCES and no Texture turn could
// commit (docs/problems/texture-trigger-not-disposed-loop-2026-11-03.md).
// The guest overlays that store onto /nix/store in the daemon's mount
// namespace, so the same file is at /nix/store/<name>/...; that path is used
// when it exists.
func deskWorkerBinaryFor(exe string) string {
	dedicated := filepath.Join(filepath.Dir(exe), "desk-worker")
	if executableRegularFile(dedicated) {
		return cellReachablePath(dedicated)
	}
	return cellReachablePath(exe)
}

// deskWorkerNixStoreRoot is where the guest's store overlay presents the
// updater's private store (a test seam).
var deskWorkerNixStoreRoot = "/nix/store"

func executableRegularFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular() && st.Mode().Perm()&0o111 != 0
}

// cellReachablePath returns the overlay alias of a path in the updater's
// private store when that alias is an executable file, else path unchanged.
func cellReachablePath(path string) string {
	if alias := layeredStoreAlias(path); alias != "" && executableRegularFile(alias) {
		return alias
	}
	return path
}

// layeredStoreAlias maps $CHOIR_UPDATER_ROOT/store/<name>/<rest> to
// <nix store>/<name>/<rest>. Only one clean store entry name (no dot prefix,
// so never the updater's .staging- dirs) with a path inside it maps.
func layeredStoreAlias(path string) string {
	root := strings.TrimSpace(os.Getenv("CHOIR_UPDATER_ROOT"))
	if root == "" {
		return ""
	}
	storeDir := filepath.Clean(filepath.Join(root, "store")) + string(filepath.Separator)
	clean := filepath.Clean(path)
	if !strings.HasPrefix(clean, storeDir) {
		return ""
	}
	name, rest, ok := strings.Cut(strings.TrimPrefix(clean, storeDir), string(filepath.Separator))
	if !ok || name == "" || rest == "" || strings.HasPrefix(name, ".") {
		return ""
	}
	return filepath.Join(deskWorkerNixStoreRoot, name, rest)
}

// deskWorkerActivationID identifies the desk activation a worker serves. Run
// identity is the stable key; the run's record supplies it when the tool
// context lacks an explicit activation field.
func deskWorkerActivationID(execCtx toolregistry.ExecutionContext) string {
	if execCtx.RunRecord != nil {
		if id := strings.TrimSpace(execCtx.RunRecord.RunID); id != "" {
			return id
		}
	}
	return strings.TrimSpace(execCtx.RunID)
}

// runDeskHold is a run's pin on the desk row it started under.
type runDeskHold struct {
	epoch   uint64
	release func()
}

// pinDeskEpoch fixes the desk epoch a run evaluates under, at its first cell,
// and holds that row until the run ends or parks (releaseRunDeskEpochs). Later
// cells of the run, and every worker they spawn, use the same epoch however
// the desk is updated meanwhile: a run is never cut by an update (plan MA4.2,
// B2). A cell with no run id (a research child's) takes the live epoch.
func (rt *Runtime) pinDeskEpoch(runID, deskRole string) uint64 {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return deskmanifest.CurrentEpoch(deskRole)
	}
	key := runID + "|" + deskRole
	if v, ok := rt.runDeskEpoch.Load(key); ok {
		return v.(*runDeskHold).epoch
	}
	// An update between reading the live epoch and holding it makes the hold
	// fail; read again. Updates are event-ordered, so this settles at once.
	for attempt := 0; attempt < 8; attempt++ {
		epoch := deskmanifest.CurrentEpoch(deskRole)
		release, ok := deskmanifest.RetainEpoch(deskRole, epoch)
		if !ok {
			if deskmanifest.CurrentEpoch(deskRole) != epoch {
				continue
			}
			// No row (an unknown or just-retired desk): nothing to pin; the
			// frame check refuses it.
			return epoch
		}
		if actual, loaded := rt.runDeskEpoch.LoadOrStore(key, &runDeskHold{epoch: epoch, release: release}); loaded {
			release()
			return actual.(*runDeskHold).epoch
		}
		return epoch
	}
	return deskmanifest.CurrentEpoch(deskRole)
}

// releaseRunDeskEpochs drops the run's pins; the run ended or parked.
func (rt *Runtime) releaseRunDeskEpochs(runID string) {
	prefix := strings.TrimSpace(runID) + "|"
	rt.runDeskEpoch.Range(func(k, v any) bool {
		if key, _ := k.(string); strings.HasPrefix(key, prefix) {
			if rt.runDeskEpoch.CompareAndDelete(k, v) {
				v.(*runDeskHold).release()
			}
		}
		return true
	})
}

// deskWorkerEpoch returns the handle-issuer epoch the worker fences on. For a
// non-capsule desk there is no capsule epoch; a positive constant satisfies
// the issuer's epoch>0 constraint. Desk epochs gain a canonical source when
// R3c binds desk activations to a lifecycle version.
func deskWorkerEpoch(execCtx toolregistry.ExecutionContext) uint64 {
	return 1
}

// deskWorkerRoot bounds the worker's filesystem root. Desks without file
// verbs get a scratch dir; the allowed root is the run's working dir.
func deskWorkerRoot(execCtx toolregistry.ExecutionContext) string {
	if wd := strings.TrimSpace(execCtx.WorkingDir); wd != "" {
		return wd
	}
	return os.TempDir()
}

// actingPackMaxItems bounds the commitment pack injected into a desk cell
// frame: enough to cover a working set without flooding model context.
const actingPackMaxItems = 16

// actingCommitmentPackForDesk builds the acting desk's score-free
// commitment pack (R4) from the commitment ledger: the desk's own
// committed acts plus acts addressed to it, joined with their resolution
// observations and discrepancies. The pack rides the cell frame so
// choir.Pack() reads it inside the worker without a network roundtrip.
// Nil on any ledger failure — the pack is context, never a gate.
func actingCommitmentPackForDesk(ctx context.Context, rt *Runtime, execCtx toolregistry.ExecutionContext) *types.ActingPack {
	if rt == nil || rt.store == nil {
		return nil
	}
	agentID := strings.TrimSpace(execCtx.AgentID)
	ownerID := strings.TrimSpace(execCtx.OwnerID)
	computerID := strings.TrimSpace(execCtx.ComputerID)
	if agentID == "" || ownerID == "" || computerID == "" {
		return nil
	}
	// Directives (including every Texture control's cast, P1) are excluded
	// in the query so they cannot crowd the desk's own acts out of the window.
	records, err := rt.store.ListCommitmentClaimRecords(ctx, ownerID, computerID, 256)
	if err != nil {
		log.Printf("desk pack: commitment record list for %s: %v", agentID, err)
		return nil
	}
	pack := types.BuildActingPack(records, agentID, actingPackMaxItems)
	// The desk's calibration digest, under the same setting and arm rule
	// as the digest turn (icrl-calibration-experiment-2026-10-14 P3).
	pack.Calibration = rt.calibrationDigestForRun(ctx, execCtx.RunRecord)
	// Management sees each engineering assignment's typed wait and its
	// minutes waited by kind (engineering_waits.go).
	if deskmanifest.AdmitsAssignments(agentID) || deskmanifest.IsAdmitterInbox(agentID) {
		pack.Waits = rt.assignmentWaitViews(ownerID, computerID)
		// And each frozen candidate's wait for its decision, from durable
		// state (retained_candidate.go).
		pack.Waits = append(pack.Waits, rt.candidateDecisionWaitViews(ctx, ownerID, computerID)...)
		// And each Verify it owes (verify_owed.go), and each shot verdict.
		pack.Waits = append(pack.Waits, rt.verifyOwedWaitViews(ctx, ownerID, computerID, agentID)...)
		pack.Waits = append(pack.Waits, rt.shotsOwedWaitViews(ctx, ownerID, computerID)...)
	} else if deskmanifest.IsAssignmentInbox(agentID) {
		// An engineer sees its own assignment's wait, the deferred deploy
		// refresh included (engineering_refresh_wait.go).
		pack.Waits = rt.engineerWaitViews(ownerID, computerID, strings.TrimPrefix(agentID, deskmanifest.PerformerName()+":"))
	}
	// Every desk sees the computer's runs in an every-model quota incident
	// (provider_quota_incident.go).
	pack.ProviderWaits = rt.providerWaitViews(ownerID, computerID)
	return &pack
}

// textureActivationCells is what one Texture activation's completed cells
// staged: a turn (a texture_turn, or a Cast or Ask, which commits as
// a Wait) or any other act.
type textureActivationCells struct {
	applied bool
	staged  bool
}

// noteTextureCell records one completed Texture cell's staged intents.
func (rt *Runtime) noteTextureCell(runID string, intents []yaegikernel.StagedIntent) {
	runID = strings.TrimSpace(runID)
	if rt == nil || runID == "" {
		return
	}
	rt.textureCellsMu.Lock()
	defer rt.textureCellsMu.Unlock()
	if rt.textureCells == nil {
		rt.textureCells = map[string]textureActivationCells{}
	}
	cells := rt.textureCells[runID]
	for _, in := range intents {
		switch {
		case in.Kind == yaegikernel.IntentTextureTurn, in.Kind == yaegikernel.IntentCast, in.Kind == yaegikernel.IntentAsk:
			cells.applied = true
		case strings.TrimSpace(in.Kind) != "":
			cells.staged = true
		}
	}
	rt.textureCells[runID] = cells
}

// takeTextureActivationCells returns and forgets the activation's cell record;
// ok is false when no cell completed.
func (rt *Runtime) takeTextureActivationCells(runID string) (textureActivationCells, bool) {
	rt.textureCellsMu.Lock()
	defer rt.textureCellsMu.Unlock()
	cells, ok := rt.textureCells[strings.TrimSpace(runID)]
	delete(rt.textureCells, strings.TrimSpace(runID))
	return cells, ok
}

// The runtime's Idle turn says which of two activations it answers. With
// TextureIdleNoActReason the activation staged nothing, so a Texture request
// it leaves open has no next actor unless something else works on it; the
// commit then gives that request a visible fate
// (docs/problems/texture-write-leaves-owner-request-unattended-2026-11-04.md).
const (
	TextureIdleNoActReason  = "desk activation completed with no authoring act; consuming the owner revision"
	TextureIdleOffDocReason = "desk activation committed an act off the texture doc (no revision needed); consuming the owner revision"
)

// answerIdleTextureTrigger disposes the activation's trigger when its cells
// completed but none authored a Texture turn. Without it the owner-revision
// trigger never consumes: texture_turn_committed needs an artifactRefs[1] ==
// the trigger head, which only a turn commit writes (defect #5, a08defc0).
// The desk observed its inputs and chose to change nothing, which is the
// no_semantic_change turn.
//
// It runs once, at the end of the activation. Run per cell (a08defc0 until
// rerun 12) it committed after the model's first read-only cell, and the
// commit's consume-at-commit default delivered every pending producer report
// before the model had read or acted on it.
//
// The decision_kind says what the activation actually did. A staged act
// (Ask/Resolve/Report) is real work routed off the doc, so the wake is
// disposed as delegation_skipped; recording no_worker_needed for it would
// launder a real act into "no act" (the s0m idle-mask defect). Only an
// activation that staged nothing is a true no_worker_needed.
func (rt *Runtime) answerIdleTextureTrigger(ctx context.Context, rec *types.RunRecord) error {
	if rt == nil || rec == nil {
		return nil
	}
	cells, ran := rt.takeTextureActivationCells(rec.RunID)
	if !ran || cells.applied {
		return nil
	}
	if _, bound := rt.Committers().Lookup(TextureRevisionKind); !bound {
		return nil
	}
	// Only an activation of a desk that commits Texture turns (it holds
	// commit.texture_revision) owes the trigger a turn; a failed or
	// cancelled one is settled by the occurrence's own failure path.
	if !deskCommits(agentProfileForRun(rec), TextureRevisionKind) || rec.State == types.RunFailed || rec.State == types.RunCancelled {
		return nil
	}
	docID := strings.TrimSpace(metadataStringValue(rec.Metadata, "doc_id"))
	if docID == "" {
		docID = strings.TrimSpace(rec.ChannelID)
	}
	if docID == "" {
		return nil
	}
	// The activation ended: the host reads the document on Texture's behalf,
	// outside the cell's context, so it declares the custodian as the reader
	// (undeclared-texture-reads-at-settlement-2026-11-05.md).
	doc, err := rt.store.GetLifecycleDocument(objectgraph.WithCustodianReader(ctx, "choir.texture_document"), rec.OwnerID, rec.ComputerID, docID)
	if err != nil {
		// The doc moved out from under the desk: the trigger either consumed
		// or the head is gone.
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return err
	}
	if doc.CurrentRevisionID == "" {
		return nil
	}
	reason := TextureIdleNoActReason
	if cells.staged {
		reason = TextureIdleOffDocReason
	}
	body, err := json.Marshal(yaegikernel.TextureTurn{BaseRevisionID: doc.CurrentRevisionID, Idle: reason})
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(rec.RunID + "\x1f" + docID + "\x1f" + doc.CurrentRevisionID))
	_, err = rt.commitDeskTurn(ctx, agentProfileForRun(rec), TextureRevisionKind, rec, string(body), "rlm-texture-idle:"+hex.EncodeToString(sum[:8]))
	if err != nil {
		// A non-pending mutation or mismatched doc means the trigger was already
		// consumed through another path. That is success for our purpose.
		if strings.Contains(err.Error(), "not pending") || strings.Contains(err.Error(), "does not match authenticated") {
			return nil
		}
		return err
	}
	return nil
}

// deskCellFramePrepared runs when the host has prepared a cell's frame, just
// before the cell's bound is armed; tests use it to make preparation slow.
var deskCellFramePrepared = func() {}

// cellTimeoutError explains a cell stopped by the timeout_ms its desk chose.
// A bare "context deadline exceeded" left research shortening the limit or
// retrying the same cell, while its web searches needed 3-10 s each
// (searches are async since Cut 2; fetches and SearchWait still take time).
func cellTimeoutError(timeoutMS int, elapsed time.Duration) string {
	return fmt.Sprintf("cell stopped by its own timeout_ms=%d after %d ms; the session was killed, so values defined in earlier cells are gone. "+
		"Host calls such as choir.Read take several seconds each (up to about 10 s), and choir.SearchWait blocks as long as you ask: leave timeout_ms unset or allow for them",
		timeoutMS, elapsed.Milliseconds())
}

// cellWorkerDiedError is what a model reads when its cell's worker died
// mid-cell with no deadline behind it: how it ended (without the stderr
// tail), the lost session, and the usual cause. On the paper a bare "frame:
// read header: EOF" told the Texture desk nothing, and it re-ran the same
// runaway loop until the guest OOM killer took the daemon
// (docs/archive/problems/desk-cell-init-only-for-loop-never-ends-guest-oom-2026-10-16.md).
func cellWorkerDiedError(exit string, elapsed time.Duration) string {
	if i := strings.Index(exit, "; stderr:"); i >= 0 {
		exit = exit[:i]
	}
	return fmt.Sprintf("cell's worker died after %d ms (%s); the session was killed, so values defined in earlier cells are gone. "+
		"A worker killed by a signal, or exiting with status 2, has usually run out of memory: check that every loop ends and that nothing grows without bound, and do one step per cell",
		elapsed.Milliseconds(), exit)
}

// cellHostDeadlineError is what a model reads when the host's cell deadline
// (S1) killed its cell: the bound, where it comes from, the time spent, the
// lost session, and what to do instead.
func cellHostDeadlineError(deskRole string, deadline, elapsed time.Duration) string {
	return fmt.Sprintf("cell stopped by the host's %s cell deadline (%s, set by %s) after %d ms; the session was killed, so values defined in earlier cells are gone. "+
		"Keep each cell short: do one step per cell, and end loops over records or captures on a bound you check",
		deskRole, deadline, managementCellDeadlineEnv, elapsed.Milliseconds())
}

// deskEgress resolves one host-mediated choir.* call from a desk cell.
// Evidence/run-memory actions resolve through rt-bound tools; the management
// desk's product_api action serves through the canonical route table;
// network/content actions go to researchDeps.HostEgress — one deps surface,
// never an open socket.
func (rt *Runtime) deskEgress(egrCtx context.Context, deskRole string, cellExec toolregistry.ExecutionContext, action yaegikernel.BrokerAction, payload json.RawMessage) (json.RawMessage, error) {
	// A call that carries what the run composed to a destination is the
	// egress sink of the disclosure path (disclosure_gate.go).
	if err := rt.gateEgressCall(egrCtx, deskRole, cellExec, action, payload); err != nil {
		return nil, err
	}
	out, err := rt.deskEgressRead(egrCtx, deskRole, cellExec, action, payload)
	if err == nil {
		if action == yaegikernel.ActionReadCapture {
			if gateErr := gateCaptureRead(egrCtx, deskRole, out); gateErr != nil {
				return nil, gateErr
			}
		}
		// Every read point the cell reaches taints the run (run_labels.go).
		rt.noteEgressRead(egrCtx, cellExec, action, payload, out)
	}
	return out, err
}

// deskEgressRead dispatches one egress call; deskEgress labels its result.
func (rt *Runtime) deskEgressRead(egrCtx context.Context, deskRole string, cellExec toolregistry.ExecutionContext, action yaegikernel.BrokerAction, payload json.RawMessage) (json.RawMessage, error) {
	if action == yaegikernel.ActionSystem1 {
		return rt.system1Egress(egrCtx, deskRole, cellExec, payload)
	}
	if action == yaegikernel.ActionRelateStories {
		return rt.relateStoriesEgress(egrCtx, deskRole, cellExec, payload)
	}
	if action == yaegikernel.ActionRelate {
		return rt.relateEgress(egrCtx, deskRole, cellExec, payload)
	}
	if action == yaegikernel.ActionRequestLease {
		return rt.requestLeaseEgress(egrCtx, deskRole, cellExec, payload)
	}
	if action == yaegikernel.ActionReadMail {
		return rt.readMailEgress(egrCtx, deskRole, cellExec, payload)
	}
	if action == yaegikernel.ActionObjectGet {
		return rt.objectGetEgress(egrCtx, deskRole, cellExec, payload)
	}
	if action == yaegikernel.ActionObjectList {
		return rt.objectListEgress(egrCtx, deskRole, cellExec, payload)
	}
	if action == yaegikernel.ActionProductAPI {
		return rt.productAPIEgress(egrCtx, deskRole, cellExec.OwnerID, cellExec.OwnerEmail, payload)
	}
	if action == yaegikernel.ActionStories {
		return rt.storiesEgress(egrCtx, deskRole, cellExec, payload)
	}
	if action == yaegikernel.ActionShotStart {
		return rt.shotStartEgress(egrCtx, deskRole, cellExec, payload)
	}
	if action == yaegikernel.ActionScreens {
		return rt.screensEgress(egrCtx, deskRole, cellExec, payload)
	}
	if action == yaegikernel.ActionInstallPacket {
		return rt.installPacketEgress(egrCtx, deskRole, cellExec, payload)
	}
	if action == yaegikernel.ActionShotWait {
		return rt.shotWaitEgress(egrCtx, deskRole, cellExec, payload)
	}
	if action == yaegikernel.ActionResearch {
		return rt.runResearchChildren(egrCtx, cellExec, payload)
	}
	if action == yaegikernel.ActionTextureSearch {
		return rt.textureSearchEgress(egrCtx, cellExec, payload)
	}
	if fn, ok := rt.egressEvidenceTool()[string(action)]; ok {
		out, err := fn(egrCtx, payload)
		return json.RawMessage(out), err
	}
	if rt.researchDeps == nil {
		return nil, fmt.Errorf("%s host unavailable", action)
	}
	if action == yaegikernel.ActionPollSource {
		// One poll of a source at a time within a run: a cell's parallel
		// calls would otherwise race on the source's cursor (MA6.6 review
		// r1, D3).
		defer rt.lockPollSource(cellExec.RunID, payload)()
	}
	return rt.researchDeps.HostEgress(egrCtx, string(action), payload)
}

// recordDeskRefusal records a frame the daemon refused from a desk worker as a
// desk.egress event with refused=true, on the run the evaluating cell belongs
// to. The refusal is also logged: a forged frame ends the worker.
func (rt *Runtime) recordDeskRefusal(ctx context.Context, fallback toolregistry.ExecutionContext, session string, r yaegikernel.FrameRefusal) {
	exec := toolregistry.ExecutionContextFrom(ctx)
	if exec.RunID == "" && exec.RunRecord == nil {
		exec = fallback
	}
	level := "refused"
	if r.Forged {
		level = "forged"
	}
	log.Printf("desk frame %s: desk %q action %s (worker pid %d, run %s): %s", level, r.Role, r.Action, r.WorkerPID, exec.RunID, r.Reason)
	if r.Forged {
		// A forged frame is a fidelity event: the worker used authority the
		// host did not give it. The breaker drops every class of the desk at
		// once and ends its leases (act_gate.go, MA7.8).
		rt.tripBreaker(ctx, exec.OwnerID, exec.ComputerID, r.Role, actgate.Wild, breakerCauseForgedFrame, exec.RunID,
			fmt.Sprintf("Its worker (pid %d) sent a %s frame the host could not verify: %s.", r.WorkerPID, r.Action, r.Reason))
	}
	rt.recordDeskEvent(context.WithoutCancel(ctx), exec, types.EventDeskEgress, map[string]any{
		"request_id": r.RequestID, "response_request_id": r.RequestID,
		"verb": string(r.Action), "desk_session": session, "worker_pid": r.WorkerPID,
		"run_id": exec.RunID, "finished_at": time.Now().UTC(),
		"is_error": true, "refused": true, "forged": r.Forged, "desk": r.Role, "error": r.Reason,
	})
}

// recordDeskEvent persists a desk.cell or desk.egress run event for the trace
// export. It is recorded on the cell's run; a choir.Research child has no run
// record, so its events land on the root run its RunID names. The event is
// not published on the live bus, and a failure to record never fails the
// cell.
func (rt *Runtime) recordDeskEvent(ctx context.Context, exec toolregistry.ExecutionContext, kind types.EventKind, payload map[string]any) {
	if rt == nil || rt.store == nil {
		return
	}
	rec := exec.RunRecord
	if rec == nil {
		if exec.RunID == "" || exec.OwnerID == "" {
			return
		}
		loaded, err := rt.getRunForComputer(ctx, exec.OwnerID, exec.RunID)
		if err != nil {
			return
		}
		rec = &loaded
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	ev := &types.EventRecord{
		EventID:      uuid.New().String(),
		RunID:        rec.RunID,
		AgentID:      rec.AgentID,
		ChannelID:    rec.ChannelID,
		OwnerID:      rec.OwnerID,
		TrajectoryID: trajectoryIDForRun(rec),
		Timestamp:    time.Now().UTC(),
		Kind:         kind,
		Payload:      raw,
	}
	if err := rt.store.AppendEvent(context.WithoutCancel(ctx), ev); err != nil {
		log.Printf("desk_go_eval: record %s event: %v", kind, err)
	}
}
