package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Ways the per-process memory read could fail (written before the code):
//  1. It leaks a secret: command lines carry flags and tokens, so only
//     /proc/<pid>/comm (the executable name) is read, never cmdline.
//  2. It is served to a guest caller: it rides the pprof prefix and its
//     host-sourced guard.
//  3. A process exits mid-scan: a missing or unreadable pid is skipped, not
//     an error.
//  4. Kernel threads (no RSS line) inflate the list: they are skipped.
//  5. Units are misread: VmRSS and meminfo are kB; the read reports KiB
//     as the kernel does and names the unit in the field.
//  6. Workers are not grouped: a per-name total (count and RSS) lets one
//     read show what the yaegi desk workers hold together.
//  7. Non-Linux hosts have no /proc: the read says so instead of
//     returning an empty success.

func writeFakeProc(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("meminfo", "MemTotal:        8192000 kB\nMemFree:          100000 kB\nMemAvailable:    3000000 kB\nBuffers:           10000 kB\nCached:          2500000 kB\nShmem:              1000 kB\nSlab:              90000 kB\nAnonPages:       4000000 kB\n")
	write("1/comm", "autoputer\n")
	write("1/cmdline", "autoputer\x00--token\x00SECRET-VALUE\x00")
	write("1/status", "Name:\tautoputer\nPPid:\t0\nVmHWM:\t 1600000 kB\nVmRSS:\t 1500000 kB\nRssAnon:\t 1400000 kB\nRssFile:\t  100000 kB\n")
	write("20/comm", "desk-worker\n")
	write("20/status", "Name:\tdesk-worker\nPPid:\t1\nVmHWM:\t 400000 kB\nVmRSS:\t 300000 kB\n")
	write("21/comm", "desk-worker\n")
	write("21/status", "Name:\tdesk-worker\nPPid:\t1\nVmHWM:\t 500000 kB\nVmRSS:\t 200000 kB\n")
	write("2/comm", "kthreadd\n")
	write("2/status", "Name:\tkthreadd\nPPid:\t0\n")
	write("33/comm", "gone\n") // exited mid-scan: no status file
	write("self/comm", "ignored\n")
	return root
}

func TestReadProcMemSummarizesProcessesWithoutCmdline(t *testing.T) {
	got, err := readProcMem(writeFakeProc(t))
	if err != nil {
		t.Fatal(err)
	}
	if got.Meminfo["MemAvailable_kib"] != 3000000 || got.Meminfo["Cached_kib"] != 2500000 {
		t.Fatalf("meminfo = %v", got.Meminfo)
	}
	if len(got.Processes) != 3 {
		t.Fatalf("processes = %+v, want autoputer and two workers (kernel thread and exited pid skipped)", got.Processes)
	}
	if p := got.Processes[0]; p.Name != "autoputer" || p.RSSKiB != 1500000 || p.HWMKiB != 1600000 || p.AnonKiB != 1400000 {
		t.Fatalf("largest process first = %+v", p)
	}
	w := got.ByName["desk-worker"]
	if w.Count != 2 || w.RSSKiB != 500000 {
		t.Fatalf("desk-worker group = %+v", w)
	}
	if got.TotalRSSKiB != 2000000 {
		t.Fatalf("total rss = %d", got.TotalRSSKiB)
	}
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "SECRET-VALUE") {
		t.Fatal("cmdline leaked into the read")
	}
}

func TestReadProcMemWithoutProcSaysSo(t *testing.T) {
	if _, err := readProcMem(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatal("a missing /proc must be an error, not an empty success")
	}
}

func TestProcMemServedOnlyToHostCallers(t *testing.T) {
	get := func(remote string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, DebugPprofPrefix+"procmem", nil)
		req.RemoteAddr = remote
		rec := httptest.NewRecorder()
		serveDebugPprof(rec, req)
		return rec
	}
	if rec := get("203.0.113.9:4000"); rec.Code != http.StatusForbidden {
		t.Fatalf("guest caller got %d", rec.Code)
	}
	rec := get("127.0.0.1:4000")
	if _, err := os.Stat("/proc/meminfo"); err != nil {
		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("no /proc: got %d, want 501", rec.Code)
		}
		return
	}
	if rec.Code != http.StatusOK || !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("host caller got %d %q", rec.Code, rec.Body.String())
	}
}
