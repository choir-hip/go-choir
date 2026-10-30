package yaegikernel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// The out-of-memory leg of the isolation self-test (SDC RES-01, RES-02): the
// deployed proof that a runaway cell is killed by its worker's cgroup and
// not by the kernel's choice of the daemon
// (docs/archive/problems/desk-cell-init-only-for-loop-never-ends-guest-oom-2026-10-16.md).
//
// The deployed desks refuse to run a deliberate runaway on a prompt
// (docs/problems/fp2-step4-deployed-oom-not-drivable-desk-refuses-runaway-2026-10-22.md),
// so the runtime runs it itself: one throwaway worker of the desk's class,
// through the production spawner and under its real budget cgroup in the
// real desks slice, is sent deskOOMSelfTestCell, a constant. Nothing here
// takes a cell from a caller.
//
// Ways it could mislead are O1-O10 in that record, with the review bunch 22
// repairs. The guards in code:
//   - the guest's MemAvailable must cover the budget plus deskOOMHeadroomBytes,
//     before any worker starts and again before the cell is sent;
//   - the cell is sent only after the worker's own cgroup reads a numeric
//     memory.max no larger than its manifest budget (O1);
//   - a sibling of the same class must start and answer before the runaway
//     starts (O5);
//   - the cell's first act is a start marker the host itself records, so a
//     kill before the cell ran is not a proof;
//   - the death cause is the cgroup's oom_kill, read by a path saved before
//     Eval, because Eval's timeout path releases the cgroup (O2, O9);
//   - the daemon's identity is read from /proc/self, never from a step list
//     (O4).
// The judge requires required confinement mode: the leg proves the
// production desk worker, not a dev spawn.

// deskOOMHeadroomBytes is what the guest must still have available after the
// runaway takes its whole budget. 512 MiB is the daemon's MemoryMin (the floor
// its unit reserves). The other 512 MiB covers the growth of the other live
// desk workers and the page cache the daemon reads its store through during
// the seconds the leg runs. On an 8 GiB guest the leg then needs about 1.75
// GiB available.
const deskOOMHeadroomBytes = 1 << 30

// deskOOMMemAvailable reads the guest's MemAvailable; tests replace it.
var deskOOMMemAvailable = readMemAvailable

func readMemAvailable() (uint64, error) {
	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && f[0] == "MemAvailable:" {
			kb, err := strconv.ParseUint(f[1], 10, 64)
			if err != nil {
				return 0, err
			}
			return kb << 10, nil
		}
	}
	return 0, fmt.Errorf("no MemAvailable in /proc/meminfo")
}

// deskOOMStartMarkerCapture is the capture id the cell's first verb asks for.
// The worker sends it to the host as a broker frame mid-cell, and the
// self-test's own egress hook records it: the host sees the cell start.
const deskOOMStartMarkerCapture = "choir-oom-selftest-start"

const deskOOMStartMarker = `_, _ = choir.ReadCapture("` + deskOOMStartMarkerCapture + `", 0, 1)`

// deskOOMSelfTestCell sends the start marker, then allocates 8 MiB at a time
// and writes one byte per page, so the memory is resident and not only
// reserved (O3).
const deskOOMSelfTestCell = deskOOMStartMarker + `
var keep [][]byte
for {
	b := make([]byte, 8<<20)
	for i := 0; i < len(b); i += 4096 {
		b[i] = 1
	}
	keep = append(keep, b)
}`

// deskOOMCheckCell is the sibling's ping.
const (
	deskOOMCheckCell = "import \"fmt\"\nfmt.Println(\"cell\", 6*7)"
	deskOOMCheckWant = "cell 42"
)

// deskOOMStartMarkerEgress is the self-test workers' egress handler: it
// records the start marker and serves nothing else.
func deskOOMStartMarkerEgress(started *atomic.Bool) func(context.Context, BrokerAction, json.RawMessage) (json.RawMessage, error) {
	return func(_ context.Context, action BrokerAction, payload json.RawMessage) (json.RawMessage, error) {
		var p ReadCapturePayload
		if action == ActionReadCapture && json.Unmarshal(payload, &p) == nil && p.CaptureID == deskOOMStartMarkerCapture {
			started.Store(true)
			return json.RawMessage(`{}`), nil
		}
		return nil, fmt.Errorf("the OOM self-test serves only its start marker")
	}
}

// DaemonIdentity is the running daemon process as it reads itself: a restart
// changes the pid or the start time, a guest reboot the boot id.
type DaemonIdentity struct {
	PID int `json:"pid"`
	// StartTicks is field 22 of /proc/self/stat: clock ticks after boot.
	StartTicks uint64 `json:"start_ticks"`
	BootID     string `json:"boot_id"`
	Error      string `json:"error,omitempty"`
}

// ReadDaemonIdentity reads the calling process's identity from /proc.
func ReadDaemonIdentity() DaemonIdentity {
	id := DaemonIdentity{PID: os.Getpid()}
	raw, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		id.Error = "stat: " + err.Error()
		return id
	}
	if id.StartTicks, err = parseProcStatStartTicks(string(raw)); err != nil {
		id.Error = err.Error()
		return id
	}
	boot, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		id.Error = "boot_id: " + err.Error()
		return id
	}
	id.BootID = strings.TrimSpace(string(boot))
	return id
}

// parseProcStatStartTicks returns field 22 (starttime) of a /proc/<pid>/stat
// line. Fields are counted after the last ')': the comm may hold anything.
func parseProcStatStartTicks(stat string) (uint64, error) {
	i := strings.LastIndexByte(stat, ')')
	if i < 0 {
		return 0, fmt.Errorf("stat: no comm")
	}
	// After ')' the first field is field 3 (state); starttime is field 22.
	fields := strings.Fields(stat[i+1:])
	if len(fields) < 20 {
		return 0, fmt.Errorf("stat: %d fields after comm, want at least 20", len(fields))
	}
	n, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("stat starttime: %w", err)
	}
	return n, nil
}

// DeskOOMCgroup is one cgroup's limits and events as the files read.
type DeskOOMCgroup struct {
	Path       string            `json:"path"`
	MemoryMax  string            `json:"memory_max"`
	MemoryHigh string            `json:"memory_high"`
	Events     map[string]uint64 `json:"memory_events,omitempty"`
}

func readDeskOOMCgroup(path string) DeskOOMCgroup {
	cg := DeskOOMCgroup{Path: path}
	read := func(name string) string {
		raw, err := os.ReadFile(filepath.Join(path, name))
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(raw))
	}
	cg.MemoryMax = read("memory.max")
	cg.MemoryHigh = read("memory.high")
	if ev := read("memory.events"); ev != "" {
		cg.Events = map[string]uint64{}
		for _, line := range strings.Split(ev, "\n") {
			f := strings.Fields(line)
			if len(f) == 2 {
				n, _ := strconv.ParseUint(f[1], 10, 64)
				cg.Events[f[0]] = n
			}
		}
	}
	return cg
}

// DeskOOMWorker is the runaway worker.
type DeskOOMWorker struct {
	PID    int           `json:"pid,omitempty"`
	Cgroup DeskOOMCgroup `json:"cgroup"`
	// CellSent: the cell frame was written to the worker (only after the
	// budget, memory and sibling checks).
	CellSent bool `json:"cell_sent"`
	// CellStarted: the host received the cell's start marker.
	CellStarted bool `json:"cell_started"`
	// Result is how the Eval ended: "transport: ..." when the worker died,
	// "cell error: ...", or "returned".
	Result    string `json:"result,omitempty"`
	Dead      bool   `json:"dead"`
	ExitCause string `json:"exit_cause,omitempty"`
	// OOMKill is memory.events oom_kill, read before the cgroup is removed.
	OOMKill uint64 `json:"oom_kill"`
	Error   string `json:"error,omitempty"`
}

// DeskOOMSibling is a worker of the same class alive across the kill.
type DeskOOMSibling struct {
	Role           string `json:"role"`
	PID            int    `json:"pid,omitempty"`
	BeforeAnswered bool   `json:"before_answered"`
	AfterAnswered  bool   `json:"after_answered"`
	OutputBefore   string `json:"output_before,omitempty"`
	OutputAfter    string `json:"output_after,omitempty"`
	Error          string `json:"error,omitempty"`
}

// DeskOOMSelfTestReceipt is the leg's receipt, judged.
type DeskOOMSelfTestReceipt struct {
	Kind      string `json:"kind"`
	StartedAt string `json:"started_at"`
	Mode      string `json:"mode"`
	Role      string `json:"role"`
	// BudgetBytes is the manifest's WorkerMaxBytes for Role.
	BudgetBytes uint64 `json:"budget_bytes"`
	// MemAvailableBytes is the guest's last MemAvailable reading before the
	// cell; RequiredAvailableBytes is the budget plus deskOOMHeadroomBytes.
	MemAvailableBytes      uint64         `json:"mem_available_bytes"`
	RequiredAvailableBytes uint64         `json:"required_available_bytes"`
	Slice                  DeskOOMCgroup  `json:"slice"`
	Worker                 DeskOOMWorker  `json:"worker"`
	Sibling                DeskOOMSibling `json:"sibling"`
	DaemonBefore           DaemonIdentity `json:"daemon_before"`
	DaemonAfter            DaemonIdentity `json:"daemon_after"`
	OK                     bool           `json:"ok"`
	Failures               []string       `json:"failures,omitempty"`
}

// DeskOOMSelfTestConfig is the production worker config and the class to
// run. There is no cell field: the leg runs deskOOMSelfTestCell only.
type DeskOOMSelfTestConfig struct {
	// Base is desk_go_eval's worker config; Role is set per worker.
	Base DeskSessionWorkerConfig
	// Role defaults to research, the untrusted-input class.
	Role string
	// CellTimeout bounds the runaway cell; defaults to 60s.
	CellTimeout time.Duration
}

// DeskOOMSelfTest runs the leg. Both workers are closed before it returns.
func DeskOOMSelfTest(ctx context.Context, cfg DeskOOMSelfTestConfig) DeskOOMSelfTestReceipt {
	role := cfg.Role
	if role == "" {
		role = "research"
	}
	timeout := cfg.CellTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	rec := DeskOOMSelfTestReceipt{
		Kind:         "desk_oom_selftest",
		StartedAt:    time.Now().UTC().Format(time.RFC3339),
		Mode:         string(cfg.Base.Confinement.Mode),
		Role:         role,
		Sibling:      DeskOOMSibling{Role: role},
		DaemonBefore: ReadDaemonIdentity(),
	}
	if rec.Mode == "" {
		rec.Mode = "auto"
	}
	finish := func() DeskOOMSelfTestReceipt {
		rec.DaemonAfter = ReadDaemonIdentity()
		rec.Failures = JudgeDeskOOMSelfTest(rec)
		rec.OK = len(rec.Failures) == 0
		return rec
	}
	// O1: never run the runaway without a hard budget.
	root := cfg.Base.CgroupRoot
	if root == "" {
		root = os.Getenv(DeskCgroupRootEnv)
	}
	if root == "" {
		rec.Worker.Error = "refused: no desks cgroup (" + DeskCgroupRootEnv + " unset); the runaway runs only under a hard budget"
		return finish()
	}
	d, known := deskmanifest.Lookup(role)
	if !known || d.Carrier != deskmanifest.CarrierDeskWorker || d.Memory.WorkerMaxBytes == 0 {
		rec.Worker.Error = fmt.Sprintf("refused: the manifest gives desk role %q no worker budget", role)
		return finish()
	}
	rec.BudgetBytes = d.Memory.WorkerMaxBytes
	rec.RequiredAvailableBytes = rec.BudgetBytes + deskOOMHeadroomBytes
	memoryShort := func() bool {
		avail, err := deskOOMMemAvailable()
		rec.MemAvailableBytes = avail
		switch {
		case err != nil:
			rec.Worker.Error = "refused: MemAvailable unread: " + err.Error()
		case avail < rec.RequiredAvailableBytes:
			rec.Worker.Error = fmt.Sprintf("refused: MemAvailable %d MiB is below the budget plus headroom %d MiB", avail>>20, rec.RequiredAvailableBytes>>20)
		default:
			return false
		}
		return true
	}
	if memoryShort() {
		return finish()
	}
	rec.Slice = readDeskOOMCgroup(root)

	var started atomic.Bool
	wcfg := cfg.Base
	wcfg.Session.Role = role
	wcfg.CgroupRoot = root
	wcfg.Egress = deskOOMStartMarkerEgress(&started)

	// The sibling first, so it lives through the kill; no live sibling, no
	// runaway.
	sib, err := SpawnDeskSessionWorker(wcfg)
	if err != nil {
		rec.Sibling.Error = "spawn: " + err.Error()
		return finish()
	}
	defer sib.Close()
	rec.Sibling.PID = sib.PID()
	rec.Sibling.BeforeAnswered, rec.Sibling.OutputBefore = deskOOMPing(ctx, sib)
	if !rec.Sibling.BeforeAnswered {
		return finish()
	}

	w, err := SpawnDeskSessionWorker(wcfg)
	if err != nil {
		rec.Worker.Error = "spawn: " + err.Error()
		return finish()
	}
	defer w.Close()
	rec.Worker.PID = w.PID()
	if w.budget == nil {
		rec.Worker.Error = "refused: the worker has no budget cgroup"
		return finish()
	}
	rec.Worker.Cgroup = readDeskOOMCgroup(w.budget.path)
	limit, perr := strconv.ParseUint(rec.Worker.Cgroup.MemoryMax, 10, 64)
	if perr != nil || limit == 0 || limit > rec.BudgetBytes {
		rec.Worker.Error = fmt.Sprintf("refused: the worker's memory.max reads %q, want a hard limit of at most %d", rec.Worker.Cgroup.MemoryMax, rec.BudgetBytes)
		return finish()
	}
	if memoryShort() {
		return finish()
	}

	runDeskOOMRunaway(ctx, w, deskOOMSelfTestCell, timeout, started.Load, &rec.Worker)

	rec.Sibling.AfterAnswered, rec.Sibling.OutputAfter = deskOOMPing(ctx, sib)
	return finish()
}

// runDeskOOMRunaway sends cell to w and records how the worker ended. The
// cgroup path is saved before Eval: Eval's timeout path reaps the worker and
// releases its cgroup (w.budget = nil), and a released cgroup reads as no
// kill, which the judge refuses.
func runDeskOOMRunaway(ctx context.Context, w *DeskSessionWorker, cell string, timeout time.Duration, started func() bool, out *DeskOOMWorker) {
	if w.budget == nil {
		out.Error = "refused: the worker has no budget cgroup"
		return
	}
	path := w.budget.path
	cellCtx, cancel := context.WithTimeout(ctx, timeout)
	res, evalErr := w.Eval(cellCtx, cell)
	cancel()
	// Sent unless Eval refused before it wrote the frame (deskEvalNotSent).
	out.CellSent = !deskEvalNotSent(evalErr)
	switch {
	case evalErr != nil:
		out.Result = "transport: " + evalErr.Error()
	case res.Error != "":
		out.Result = "cell error: " + res.Error
	default:
		out.Result = "returned"
	}
	out.CellStarted = started()
	out.Dead = w.Dead()
	// O9: the events before Close removes the cgroup.
	out.Cgroup = readDeskOOMCgroup(path)
	out.OOMKill = out.Cgroup.Events["oom_kill"]
	w.Close()
	out.ExitCause = w.ExitCause()
}

// deskEvalNotSent reports whether err is one of Eval's refusals before the
// cell frame reached the worker: the worker was already dead, the frame would
// not marshal, or the frame write failed (desk_session_worker.go, EvalCell).
// Every other Eval error (a read failure, a forged or mismatched frame, the
// context ending) comes after the write.
func deskEvalNotSent(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return msg == "desk session worker dead" ||
		strings.HasPrefix(msg, "desk eval marshal") ||
		strings.HasPrefix(msg, "desk eval write")
}

func deskOOMPing(ctx context.Context, w *DeskSessionWorker) (bool, string) {
	cellCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	res, err := w.Eval(cellCtx, deskOOMCheckCell)
	switch {
	case err != nil:
		return false, "transport: " + err.Error()
	case res.Error != "":
		return false, "cell error: " + res.Error
	}
	out := strings.TrimSpace(res.Stdout)
	return strings.Contains(out, deskOOMCheckWant), out
}

// JudgeDeskOOMSelfTest returns every way the receipt falls short; empty
// means the worker was killed at its budget and nothing else was.
func JudgeDeskOOMSelfTest(rec DeskOOMSelfTestReceipt) []string {
	var fails []string
	fail := func(format string, args ...any) { fails = append(fails, fmt.Sprintf(format, args...)) }
	if rec.Worker.Error != "" {
		fail("worker: %s", rec.Worker.Error)
	}
	if rec.Mode != string(CellConfinementRequired) {
		fail("confinement mode %q, want required (the guest's production mode)", rec.Mode)
	}
	switch {
	case rec.RequiredAvailableBytes != rec.BudgetBytes+deskOOMHeadroomBytes:
		fail("required available %d is not the budget plus the %d headroom", rec.RequiredAvailableBytes, uint64(deskOOMHeadroomBytes))
	case rec.MemAvailableBytes == 0 || rec.MemAvailableBytes < rec.RequiredAvailableBytes:
		fail("MemAvailable %d below the required %d", rec.MemAvailableBytes, rec.RequiredAvailableBytes)
	}
	if rec.BudgetBytes == 0 {
		fail("no manifest budget for %q", rec.Role)
	}
	if n, err := strconv.ParseUint(rec.Slice.MemoryMax, 10, 64); err != nil || n == 0 {
		fail("desks slice memory.max = %q, want a hard limit", rec.Slice.MemoryMax)
	}
	if got := rec.Worker.Cgroup.MemoryMax; rec.BudgetBytes != 0 && got != strconv.FormatUint(rec.BudgetBytes, 10) {
		fail("worker memory.max = %q, want the manifest's %d", got, rec.BudgetBytes)
	}
	if !rec.Worker.CellSent {
		fail("the runaway cell was not sent")
	} else {
		if !rec.Worker.CellStarted {
			fail("the cell's start marker never reached the host: a kill before the cell ran proves nothing")
		}
		if rec.Worker.Result == "returned" {
			fail("the runaway cell returned")
		}
		if !rec.Worker.Dead {
			fail("the runaway worker is alive")
		}
		if rec.Worker.OOMKill < 1 {
			fail("worker memory.events oom_kill = %d: the cgroup did not kill it (result %q, exit %q)", rec.Worker.OOMKill, rec.Worker.Result, rec.Worker.ExitCause)
		}
		if !strings.Contains(rec.Worker.ExitCause, "killed by its memory budget") {
			fail("exit cause does not name the memory budget: %q", rec.Worker.ExitCause)
		}
	}
	if rec.Sibling.Error != "" {
		fail("sibling: %s", rec.Sibling.Error)
	}
	if !rec.Sibling.BeforeAnswered {
		fail("the sibling did not answer before the runaway")
	}
	if !rec.Sibling.AfterAnswered {
		fail("the sibling did not answer after the runaway: %q", rec.Sibling.OutputAfter)
	}
	b, a := rec.DaemonBefore, rec.DaemonAfter
	switch {
	case b.Error != "" || a.Error != "":
		fail("daemon identity unread: before %q after %q", b.Error, a.Error)
	case b.PID <= 0 || b.StartTicks == 0 || b.BootID == "":
		fail("daemon identity incomplete: %+v", b)
	case a != b:
		fail("daemon identity changed: before %+v after %+v", b, a)
	}
	return fails
}
