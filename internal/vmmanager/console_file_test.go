package vmmanager

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// docs/problems/firecracker-sigpipe-deadlock-after-vmctl-restart-2026-10-10.md:
// firecracker's console went to a pipe read by the launching vmctl; once
// that vmctl exited, the next console write raised SIGPIPE and deadlocked a
// vCPU. Failure modes pinned:
//   - the console sink handed to firecracker is a pipe (needs a reader);
//   - bounding the log renames the live file (firecracker keeps writing the
//     renamed one) or drops what was written;
//   - bounding grows generations without limit;
//   - reattach leaves a legacy console pipe without a reader;
//   - reattach opens a reader on a console that is already a file.

func TestConsoleSinkIsAFileThatOutlivesTheLauncher(t *testing.T) {
	path := filepath.Join(t.TempDir(), "console.log")
	sink, err := openConsoleSink(path, 64, 2)
	if err != nil {
		t.Fatal(err)
	}
	info, err := sink.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("console sink mode %v, want a regular file", info.Mode())
	}
	// The child keeps its own descriptor; the launcher closing its copy
	// must not stop the child's writes.
	child, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	_ = sink.Close()
	if _, err := child.WriteString("after launcher exit\n"); err != nil {
		t.Fatalf("write after the launcher closed its copy: %v", err)
	}
	_ = child.Close()
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "after launcher exit") {
		t.Fatalf("console = %q", got)
	}
}

func TestBoundConsoleLogCopiesThenTruncatesInPlace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "console.log")
	live, err := openConsoleSink(path, 16, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()
	for round := 1; round <= 4; round++ {
		if _, err := live.WriteString(fmt.Sprintf("round-%d-0123456789\n", round)); err != nil {
			t.Fatal(err)
		}
		if err := boundConsoleLog(path, 16, 2); err != nil {
			t.Fatal(err)
		}
	}
	// The live descriptor still writes the live path after truncation.
	if _, err := live.WriteString("tail\n"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "tail\n" {
		t.Fatalf("live console = %q, want only the post-bound write", got)
	}
	first, _ := os.ReadFile(path + ".1")
	second, _ := os.ReadFile(path + ".2")
	if !strings.Contains(string(first), "round-4") || !strings.Contains(string(second), "round-3") {
		t.Fatalf("generations: .1=%q .2=%q", first, second)
	}
	if _, err := os.Stat(path + ".3"); !os.IsNotExist(err) {
		t.Fatalf("generation beyond the limit exists: %v", err)
	}
}

func TestReattachDrainsALegacyConsolePipe(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("reattach reads /proc")
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	path := filepath.Join(t.TempDir(), "console.log")
	fdPath := fmt.Sprintf("/proc/self/fd/%d", w.Fd())
	// The launcher's reader is gone, as after a vmctl restart.
	_ = r.Close()
	drained, err := drainLegacyConsolePipe(fdPath, path, 1<<20, 2)
	if err != nil || !drained {
		t.Fatalf("drain = %v, %v; want a reader on the legacy pipe", drained, err)
	}
	if _, err := w.WriteString("console after reattach\n"); err != nil {
		t.Fatalf("write to a drained pipe: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got, _ := os.ReadFile(path); strings.Contains(string(got), "console after reattach") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	got, _ := os.ReadFile(path)
	t.Fatalf("drained console = %q", got)
}

func TestReattachLeavesAFileConsoleAlone(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("reattach reads /proc")
	}
	path := filepath.Join(t.TempDir(), "console.log")
	sink, err := openConsoleSink(path, 1<<20, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close()
	drained, err := drainLegacyConsolePipe(fmt.Sprintf("/proc/self/fd/%d", sink.Fd()), path, 1<<20, 2)
	if err != nil || drained {
		t.Fatalf("drain on a file console = %v, %v; want untouched", drained, err)
	}
}
