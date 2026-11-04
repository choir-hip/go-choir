package capsule

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

// The broker's own log, readable on the product route
// (docs/problems/engineering-cell-never-returns-2026-11-03.md, "Next
// probes"). Failure modes written before the code:
//   - the tail grows without bound (a goroutine dump is thousands of lines);
//   - one long line (a panic value, a stack line) is kept whole;
//   - a line longer than the reader's buffer stops the reader, the pipe
//     fills and the broker blocks writing its own stderr;
//   - the first fatal line ("runtime: failed to create new OS thread",
//     "panic:") scrolls out under the dump that follows it;
//   - the journal stops getting the lines it got before;
//   - a destroyed capsule's tail is gone before anyone reads it, or every
//     destroyed capsule's tail is kept forever.

func TestBrokerLogTailKeepsTheLastLinesBounded(t *testing.T) {
	tail := newBrokerLogTail(3)
	for i := 0; i < 10; i++ {
		tail.addLine(fmt.Sprintf("line %d", i))
	}
	tail.addLine(strings.Repeat("x", 4*brokerLogLineMax))
	fatal, lines := tail.snapshot()
	if len(lines) != 3 || lines[0] != "line 8" || lines[1] != "line 9" {
		t.Fatalf("lines %q", lines)
	}
	if got := len(lines[2]); got > brokerLogLineMax+len("...") {
		t.Fatalf("a long line was kept at %d bytes", got)
	}
	if len(fatal) != 0 {
		t.Fatalf("no fatal line was written, got %q", fatal)
	}
}

func TestBrokerLogTailPinsTheFirstFatalLines(t *testing.T) {
	tail := newBrokerLogTail(5)
	tail.addLine("capsule-broker: listening on /run/capsule/broker.sock (merged=/)")
	tail.addLine("runtime: failed to create new OS thread (have 255 already; errno=11)")
	tail.addLine("fatal error: newosproc")
	for i := 0; i < 1000; i++ {
		tail.addLine(fmt.Sprintf("goroutine %d [running]:", i))
	}
	for i := 0; i < 100; i++ {
		tail.addLine("panic: again")
	}
	fatal, lines := tail.snapshot()
	if len(lines) != 5 {
		t.Fatalf("lines %d", len(lines))
	}
	if len(fatal) < 2 || fatal[0] != "runtime: failed to create new OS thread (have 255 already; errno=11)" || fatal[1] != "fatal error: newosproc" {
		t.Fatalf("fatal %q", fatal)
	}
	if len(fatal) > brokerLogFatalMax {
		t.Fatalf("fatal lines unbounded: %d", len(fatal))
	}
}

func TestCopyBrokerLogDrainsALongLineAndEchoes(t *testing.T) {
	tail := newBrokerLogTail(10)
	var echo bytes.Buffer
	in := "first\n" + strings.Repeat("y", 1<<20) + "\nlast without newline"
	done := make(chan struct{})
	go func() {
		copyBrokerLog(strings.NewReader(in), tail, &echo)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the reader stopped on a long line")
	}
	_, lines := tail.snapshot()
	if len(lines) != 3 || lines[0] != "first" || lines[2] != "last without newline" || len(lines[1]) > brokerLogLineMax+3 {
		t.Fatalf("lines %d %q", len(lines), lines[0])
	}
	if echo.String() != in+"\n" {
		t.Fatalf("the journal echo lost bytes: %d of %d", echo.Len(), len(in)+1)
	}
	// A reader that fails part way still returns.
	copyBrokerLog(io.MultiReader(strings.NewReader("a\n"), errReader{}), tail, io.Discard)
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, fakeErr("broken pipe") }

func TestBrokerLogRegistryKeepsLiveAndTheLastEnded(t *testing.T) {
	var reg brokerLogRegistry
	for i := 0; i < brokerLogsEndedKept+3; i++ {
		id := fmt.Sprintf("capsule-%d", i)
		reg.register(id).addLine("hello " + id)
		reg.end(id, "")
	}
	reg.register("capsule-live").addLine("serving")
	reg.end("capsule-unknown", "") // never registered: no entry
	logs := reg.snapshot()
	if len(logs) != brokerLogsEndedKept+1 {
		t.Fatalf("kept %d logs", len(logs))
	}
	last := logs[len(logs)-1]
	if last.CapsuleID != "capsule-live" || !last.Live || len(last.Lines) != 1 {
		t.Fatalf("live log %+v", last)
	}
	first := logs[0]
	if first.Live || first.CapsuleID != fmt.Sprintf("capsule-%d", 3) {
		t.Fatalf("oldest kept %+v", first)
	}
	reg.setExit("capsule-live", "exit status 1")
	if logs := reg.snapshot(); logs[len(logs)-1].Exit != "exit status 1" {
		t.Fatalf("exit not recorded: %+v", logs[len(logs)-1])
	}
}
