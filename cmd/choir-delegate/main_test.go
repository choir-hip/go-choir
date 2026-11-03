package main

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/claudedelegate"
)

// docs/design/claude-delegate-slice1-2026-10-14.md F1/F2/F12: in a capsule
// without the delegate (no socket), every verb says it is unavailable with
// exit 3 instead of hanging or pretending; a bad spec is refused before it
// reaches the broker; wait reads the run's status file and never blocks
// past choir.Exec's window.

func TestUnavailableWithoutTheBrokerSocket(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "absent.sock")
	for _, args := range [][]string{{"status"}, {"start", "--spec-json", `{"objective":"x"}`}, {"stop", "--id", "d1"}} {
		var out bytes.Buffer
		if code := run(args, &out, socket, t.TempDir()); code != 3 || !strings.Contains(out.String(), `"available": false`) {
			t.Errorf("%v: code %d out %s", args, code, out.String())
		}
	}
}

func TestStartRefusesABadSpecLocally(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"start", "--spec-json", `{"objective":"x","paths":["../../etc"]}`}, &out, "/nonexistent.sock", t.TempDir()); code != 1 || !strings.Contains(out.String(), "outside") {
		t.Fatalf("code %d out %s", code, out.String())
	}
}

func TestStartAndWaitAgainstAServer(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "runs")
	script := filepath.Join(dir, "claude")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nread p\necho '{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"num_turns\":1,\"result\":\"ok\"}'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	srv := claudedelegate.NewServer(root, "sk-ant-oat01-FAKE-cli-token-0123456789abcdefghijkl", []string{"PATH=" + os.Getenv("PATH")}, func(string) *exec.Cmd { return exec.Command(script) })
	// macOS caps a socket path near 104 bytes; t.TempDir() can exceed it.
	short, err := os.MkdirTemp("/tmp", "cd")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(short)
	socket := filepath.Join(short, "d.sock")
	l, err := net.Listen("unix", socket)
	if err != nil {
		t.Skipf("unix socket: %v", err)
	}
	defer l.Close()
	go func() { _ = srv.Serve(l) }()

	var out bytes.Buffer
	if code := run([]string{"start", "--spec-json", `{"objective":"say ok","max_turns":2,"wall_clock_min":1}`}, &out, socket, root); code != 0 {
		t.Fatalf("start: %d %s", code, out.String())
	}
	var started claudedelegate.Response
	if err := json.Unmarshal(out.Bytes(), &started); err != nil || started.Status == nil || started.Status.ID == "" {
		t.Fatalf("start output %s (%v)", out.String(), err)
	}
	begin := time.Now()
	out.Reset()
	if code := run([]string{"wait", "--id", started.Status.ID, "--seconds", "60"}, &out, socket, root); code != 0 {
		t.Fatalf("wait: %d %s", code, out.String())
	}
	if time.Since(begin) > (claudedelegate.WaitMaxSeconds+2)*time.Second {
		t.Fatal("wait ignored its 25 s ceiling")
	}
	var st claudedelegate.Status
	if err := json.Unmarshal(out.Bytes(), &st); err != nil || st.State != claudedelegate.StateDone || st.Result == nil || st.Result.Subtype != "success" {
		t.Fatalf("wait output %s (%v)", out.String(), err)
	}
	out.Reset()
	if code := run([]string{"wait", "--id", "../../etc"}, &out, socket, root); code != 1 {
		t.Fatalf("wait with an escaping id: %d %s", code, out.String())
	}
}

// Cycle time (docs/evidence/selfdev-round-cycle-time-2026-11-03.md): a claude
// engineer polled `wait` 31 times, 12 of 47 minutes. A parked wait is one job
// that returns when the run ends, however long that takes, in one call.

func writeStatus(t *testing.T, root, id, state string) {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(claudedelegate.Status{ID: id, State: state})
	tmp := filepath.Join(dir, "tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, filepath.Join(dir, claudedelegate.StatusFile)); err != nil {
		t.Fatal(err)
	}
}

func TestParkedWaitOutlastsTheUnparkedCeilingAndReturnsWhenTheRunEnds(t *testing.T) {
	root := t.TempDir()
	writeStatus(t, root, "run1", claudedelegate.StateRunning)
	go func() {
		time.Sleep(700 * time.Millisecond)
		writeStatus(t, root, "run1", claudedelegate.StateDone)
	}()
	var out bytes.Buffer
	begin := time.Now()
	if code := run([]string{"wait", "--id", "run1", "--park"}, &out, "/nonexistent.sock", root); code != 0 {
		t.Fatalf("code %d %s", code, out.String())
	}
	if d := time.Since(begin); d < 600*time.Millisecond || d > 3*time.Second {
		t.Fatalf("parked wait returned after %v, want about 0.7 s (the run's end)", d)
	}
	if !strings.Contains(out.String(), `"state": "done"`) || !strings.Contains(out.String(), "--seconds 0") {
		t.Fatalf("summary %s", out.String())
	}
	if len(out.String()) > 1024 {
		t.Fatalf("summary is %d bytes; a job tail is 4 KiB", out.Len())
	}
}

func TestParkedWaitHonoursItsBoundAndPlainWaitStillCapsAt25(t *testing.T) {
	root := t.TempDir()
	writeStatus(t, root, "run2", claudedelegate.StateRunning)
	var out bytes.Buffer
	begin := time.Now()
	if code := run([]string{"wait", "--id", "run2", "--park", "--seconds", "1"}, &out, "/nonexistent.sock", root); code != 0 {
		t.Fatalf("code %d %s", code, out.String())
	}
	if d := time.Since(begin); d < 900*time.Millisecond || d > 3*time.Second {
		t.Fatalf("bounded park returned after %v", d)
	}
	if !strings.Contains(out.String(), `"state": "running"`) || !strings.Contains(out.String(), "park again") {
		t.Fatalf("summary %s", out.String())
	}
	// Without --park the full status prints and --seconds 0 returns at once.
	out.Reset()
	if code := run([]string{"wait", "--id", "run2", "--seconds", "0"}, &out, "/nonexistent.sock", root); code != 0 || !strings.Contains(out.String(), `"dir"`) {
		t.Fatalf("plain wait: %d %s", code, out.String())
	}
}

func TestHelpPrintsUsage(t *testing.T) {
	for _, a := range []string{"--help", "-h", "help"} {
		var out bytes.Buffer
		if code := run([]string{a}, &out, "/nonexistent.sock", t.TempDir()); code != 0 || !strings.Contains(out.String(), "usage: choir-delegate") {
			t.Errorf("%s: code %d out %s", a, code, out.String())
		}
	}
}
