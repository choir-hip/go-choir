//go:build linux

package capsule

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/childthread"
)

// A dead capsule broker must not look hung
// (docs/problems/engineering-cell-never-returns-2026-11-03.md). The host
// holds its own copy of the broker's listener, so after the broker exits a
// dial still lands in the backlog and a call fails only at its read bound,
// as "i/o timeout". The executor therefore watches the broker process after
// readiness: on an exit it did not ask for, it logs the status, the signal
// and the broker's last lines, marks the broker client so every call fails
// at once with ErrBrokerExited, closes the host's listener copy, and the
// job watcher ends the capsule's running job with the broker's death.

// brokerLogPipe carries a broker's stderr into its capsule's log tail and
// the journal.
type brokerLogPipe struct {
	r, w *os.File
	tail *brokerLogTail
	echo io.Writer
}

// attachBrokerLog points cmd's stderr at a pipe. A *os.File, not a writer:
// cmd.Wait then never waits on a copy goroutine, so a destroy's wait is
// unchanged. Call started after cmd.Start, abort if it failed.
func attachBrokerLog(cmd *exec.Cmd, tail *brokerLogTail, echo io.Writer) (*brokerLogPipe, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("capsule broker log pipe: %w", err)
	}
	cmd.Stderr = w
	return &brokerLogPipe{r: r, w: w, tail: tail, echo: echo}, nil
}

// started closes the parent's write end and reads the pipe until every
// writer (the launcher, the broker and its children) is gone.
func (p *brokerLogPipe) started() {
	_ = p.w.Close()
	go func() {
		defer p.tail.endStream()
		defer p.r.Close()
		copyBrokerLog(p.r, p.tail, p.echo)
	}()
}

func (p *brokerLogPipe) abort() {
	_ = p.w.Close()
	_ = p.r.Close()
	p.tail.endStream()
}

// brokerLogDrainWait bounds how long a death's report waits for the broker's
// last stderr lines to be read.
const brokerLogDrainWait = 500 * time.Millisecond

// startBrokerProcess starts cmd, the broker launcher, and reaps it. The
// fork runs on a thread held until the launcher exits (childthread): the
// launcher's Pdeathsig fires when its parent THREAD exits, and the autoputer
// ends threads on purpose (LockOSThread without unlock in guestmem reclaim,
// tapestore compaction, capfloor). One of those landing on the thread that
// forked the launcher SIGKILLed the launcher, the broker and every process
// of the capsule (docs/problems/engineering-cell-never-returns-2026-11-03.md).
func (c *Capsule) startBrokerProcess(ctx context.Context, timeout time.Duration, cmd *exec.Cmd) error {
	if err := waitForCommandStart(ctx, timeout, func() error { return childthread.Start(cmd) }); err != nil {
		return err
	}
	done := make(chan struct{})
	c.Process = cmd.Process
	c.PID = cmd.Process.Pid
	c.processDone = done
	c.wait = func() error {
		<-done
		return c.processErr
	}
	go func() {
		c.processErr = cmd.Wait()
		close(done)
	}()
	return nil
}

// closeListener closes the host's copy of the broker listener, once.
func (c *Capsule) closeListener() error {
	c.listenerMu.Lock()
	defer c.listenerMu.Unlock()
	if c.listener == nil {
		return nil
	}
	err := c.listener.Close()
	c.listener = nil
	return err
}

func (c *Capsule) recordBrokerDeath(status string) {
	c.deathMu.Lock()
	defer c.deathMu.Unlock()
	if c.death == "" {
		c.death = status
	}
}

// brokerDeath is the broker's exit status and time when it died before the
// capsule's destroy, or "".
func (c *Capsule) brokerDeath() string {
	c.deathMu.Lock()
	defer c.deathMu.Unlock()
	return c.death
}

// describeBrokerExit names how the broker launcher ended: its exit status or
// the signal that killed it.
func describeBrokerExit(err error) string {
	if err == nil {
		return "exit status 0"
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			switch {
			case ws.Signaled():
				text := fmt.Sprintf("killed by signal %d (%s)", int(ws.Signal()), ws.Signal())
				if ws.CoreDump() {
					text += ", core dumped"
				}
				return text
			case ws.Exited():
				return fmt.Sprintf("exit status %d", ws.ExitStatus())
			}
		}
	}
	return err.Error()
}

// watchBrokerExit waits for the capsule's broker to exit. An exit a destroy
// asked for is not a death. A death is logged with the broker's last lines,
// fails every broker call at once, closes the host's listener copy, and is
// recorded for the job watcher (which ends the running job) and the
// capsule's receipt.
func (e *Executor) watchBrokerExit(caps *Capsule) {
	<-caps.processDone
	if caps.stopping.Load() {
		return
	}
	status := describeBrokerExit(caps.processErr) + " at " + time.Now().UTC().Format("15:04:05Z")
	tail := e.brokerLogs.tailOf(caps.ID)
	tail.waitStream(brokerLogDrainWait)
	fatal, lines := tail.snapshot()
	if last := tail.last(); last != "" {
		status += "; last broker log line: " + last
	}
	caps.recordBrokerDeath(status)
	e.brokerLogs.setExit(caps.ID, status)
	if caps.broker != nil {
		caps.broker.MarkExited(status)
	}
	if err := caps.closeListener(); err != nil {
		log.Printf("capsule %s: close the dead broker's listener: %v", caps.ID, err)
	}
	if len(lines) > 10 {
		lines = lines[len(lines)-10:]
	}
	log.Printf("capsule %s: the broker exited before the capsule's destroy: %s; its broker calls now fail at once and its running job ends; first fatal lines: %q; last lines: %q",
		caps.ID, status, fatal, lines)
}

// BrokerLogs returns the live capsules' broker logs and the last few ended
// ones, for the observability route. It takes no executor lock.
func (e *Executor) BrokerLogs() []CapsuleBrokerLog {
	return e.brokerLogs.snapshot()
}
