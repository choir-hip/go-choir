package vmbackend

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Process is a started VMM. It is the backend-neutral wait/exit half of the
// seam: vmmanager's startFirecrackerProcess does the same today (console
// file, PID, a goroutine that waits), and moves onto this in migration
// step M2.
type Process struct {
	PID    int
	VMID   string
	cmd    *exec.Cmd
	done   chan struct{}
	mu     sync.Mutex
	exitEr error
}

// Start writes the plan's files and execs the VMM. stdout receives the
// guest serial console (both backends write ttyS0 to stdout); it should be a
// file the child writes directly, never a pipe the caller must drain
// (docs/problems/firecracker-sigpipe-deadlock-after-vmctl-restart-2026-10-10.md).
// The caller still owns and closes its copies of plan.ExtraFiles and stdout
// once Start returns.
func Start(plan LaunchPlan, stdout, stderr io.Writer) (*Process, error) {
	if plan.Binary == "" {
		return nil, fmt.Errorf("vmbackend: launch plan for %s has no binary", plan.VMID)
	}
	for _, f := range plan.WriteFiles {
		if err := os.MkdirAll(filepath.Dir(f.Path), 0o755); err != nil {
			return nil, fmt.Errorf("vmbackend: create dir for %s: %w", f.Path, err)
		}
		mode := f.Mode
		if mode == 0 {
			mode = 0o600
		}
		if err := os.WriteFile(f.Path, f.Data, mode); err != nil {
			return nil, fmt.Errorf("vmbackend: write %s: %w", f.Path, err)
		}
	}
	cmd := exec.Command(plan.Binary, plan.Args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.ExtraFiles = plan.ExtraFiles
	if plan.Env != nil {
		cmd.Env = plan.Env
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("vmbackend: start %s for VM %s: %w", plan.Backend, plan.VMID, err)
	}
	p := &Process{PID: cmd.Process.Pid, VMID: plan.VMID, cmd: cmd, done: make(chan struct{})}
	go func() {
		err := cmd.Wait()
		p.mu.Lock()
		p.exitEr = err
		p.mu.Unlock()
		close(p.done)
	}()
	return p, nil
}

// Done is closed when the VMM has exited.
func (p *Process) Done() <-chan struct{} { return p.done }

// ExitErr is the VMM's exit status once Done is closed (nil for exit 0).
func (p *Process) ExitErr() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.exitEr
}

// ErrStopTimeout means the VMM survived SIGKILL for the whole grace period.
var ErrStopTimeout = errors.New("vmbackend: VMM did not exit after SIGKILL")

// Stop kills the VMM and waits up to grace for it to exit. Both backends
// stop the way Firecracker does today: SIGKILL, with the data disk's
// consistency owned by the guest's journaling. vmmanager writes the
// destruction receipt before calling Stop.
func (p *Process) Stop(grace time.Duration) error {
	select {
	case <-p.done:
		return nil
	default:
	}
	if err := p.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("vmbackend: kill VM %s pid %d: %w", p.VMID, p.PID, err)
	}
	select {
	case <-p.done:
		return nil
	case <-time.After(grace):
		return fmt.Errorf("%w: VM %s pid %d", ErrStopTimeout, p.VMID, p.PID)
	}
}

// clockTicksPerSecond is USER_HZ. Linux fixes it at 100 on every
// architecture Choir runs (x86_64, aarch64) regardless of CONFIG_HZ.
const clockTicksPerSecond = 100

// ProcMetrics reads the backend-neutral half of Metrics from /proc.
func ProcMetrics(pid int) (Metrics, error) {
	return procMetricsAt("/proc", pid)
}

func procMetricsAt(root string, pid int) (Metrics, error) {
	if pid <= 0 {
		return Metrics{}, fmt.Errorf("vmbackend: invalid pid %d", pid)
	}
	dir := filepath.Join(root, strconv.Itoa(pid))
	status, err := os.ReadFile(filepath.Join(dir, "status"))
	if err != nil {
		return Metrics{}, fmt.Errorf("vmbackend: read status of pid %d: %w", pid, err)
	}
	stat, err := os.ReadFile(filepath.Join(dir, "stat"))
	if err != nil {
		return Metrics{}, fmt.Errorf("vmbackend: read stat of pid %d: %w", pid, err)
	}
	rss, err := parseStatusRSS(status)
	if err != nil {
		return Metrics{}, err
	}
	utime, stime, err := parseStatCPU(stat)
	if err != nil {
		return Metrics{}, err
	}
	return Metrics{PID: pid, RSSBytes: rss, UserCPU: utime, SystemCPU: stime, At: time.Now()}, nil
}

// parseStatusRSS returns VmRSS from /proc/<pid>/status in bytes.
func parseStatusRSS(status []byte) (int64, error) {
	for _, line := range strings.Split(string(status), "\n") {
		rest, ok := strings.CutPrefix(line, "VmRSS:")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) != 2 || fields[1] != "kB" {
			return 0, fmt.Errorf("vmbackend: malformed VmRSS line %q", line)
		}
		kb, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("vmbackend: malformed VmRSS line %q", line)
		}
		return kb * 1024, nil
	}
	// A zombie has no VmRSS line.
	return 0, fmt.Errorf("vmbackend: no VmRSS in status (process exiting?)")
}

// parseStatCPU returns utime and stime from /proc/<pid>/stat. The comm field
// may contain spaces and parentheses, so fields are counted after the last
// ')'.
func parseStatCPU(stat []byte) (time.Duration, time.Duration, error) {
	s := string(stat)
	i := strings.LastIndexByte(s, ')')
	if i < 0 {
		return 0, 0, fmt.Errorf("vmbackend: malformed stat")
	}
	fields := strings.Fields(s[i+1:])
	// After comm: state(3) ppid ... utime is field 14, stime 15 (1-based in
	// proc(5)); fields[0] is field 3.
	if len(fields) < 13 {
		return 0, 0, fmt.Errorf("vmbackend: short stat")
	}
	u, err1 := strconv.ParseInt(fields[11], 10, 64)
	st, err2 := strconv.ParseInt(fields[12], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("vmbackend: malformed stat cpu fields")
	}
	tick := time.Second / clockTicksPerSecond
	return time.Duration(u) * tick, time.Duration(st) * tick, nil
}
