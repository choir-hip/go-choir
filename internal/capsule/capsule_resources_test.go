package capsule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The capsule receipt at destroy carries pids and CPU next to memory
// (docs/problems/engineering-cell-never-returns-2026-11-03.md: the receipt
// could not say whether the job hit pids.max). Failure modes written before
// the code:
//   - pids.events "max" (forks and thread creations refused at pids.max) is
//     not read, so a broker that died for a thread cannot be told apart;
//   - pids.peak is not read, so headroom cannot be judged;
//   - cpu.stat throttling (nr_throttled, throttled_usec) is not read;
//   - a file that cannot be read reads as 0 (a false "never hit") instead of
//     unknown;
//   - the log line lacks them;
//   - a broker that died before destroy is not named in the receipt.

func TestCapsuleReceiptReadsPidsAndCPU(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("memory.events", "low 0\nhigh 0\nmax 0\noom 0\noom_kill 0\n")
	write("memory.peak", "1495269376\n")
	write("pids.events", "max 37\n")
	write("pids.peak", "256\n")
	write("cpu.stat", "usage_usec 9000000\nuser_usec 7000000\nsystem_usec 2000000\nnr_periods 3000\nnr_throttled 1200\nthrottled_usec 450000000\nnr_bursts 0\nburst_usec 0\n")
	const mib = int64(1 << 20)
	r := readCapsuleMemoryReceipt(CapsuleMemoryObserve, capsuleMemoryPlan{Admit: 2048 * mib, Max: 2048 * mib}, 2048*mib, cgroupMemoryDir(dir))
	r.PidsMax = 256
	r.BrokerExit = "exit status 1 at 22:31:05Z"
	if r.PidsEventsMax != 37 || r.PidsPeak != 256 || r.CPUNrThrottled != 1200 || r.CPUThrottledUsec != 450000000 {
		t.Fatalf("receipt %+v", r)
	}
	line := r.String()
	for _, want := range []string{"pids max=256 peak=256 events max=37", "cpu nr_throttled=1200 throttled_usec=450000000", "broker_exit=\"exit status 1 at 22:31:05Z\""} {
		if !strings.Contains(line, want) {
			t.Errorf("receipt line %q lacks %q", line, want)
		}
	}

	// A kernel without pids.peak, a cgroup without the cpu controller: unknown.
	empty := t.TempDir()
	r = readCapsuleMemoryReceipt(CapsuleMemoryObserve, capsuleMemoryPlan{Admit: mib, Max: mib}, 0, cgroupMemoryDir(empty))
	if r.PidsEventsMax != -1 || r.PidsPeak != -1 || r.CPUNrThrottled != -1 || r.CPUThrottledUsec != -1 {
		t.Fatalf("unreadable files must read unknown: %+v", r)
	}
	if line := r.String(); !strings.Contains(line, "pids max=unknown peak=unknown events max=unknown") || strings.Contains(line, "broker_exit") {
		t.Fatalf("receipt line %q", line)
	}
	// A cgroup that offers no resource files at all (a test fake).
	if r := readCapsuleMemoryReceipt(CapsuleMemoryObserve, capsuleMemoryPlan{Admit: mib, Max: mib}, 0, &fakeMemoryFeedback{}); r.PidsPeak != -1 || r.CPUNrThrottled != -1 {
		t.Fatalf("fake cgroup: %+v", r)
	}
}
