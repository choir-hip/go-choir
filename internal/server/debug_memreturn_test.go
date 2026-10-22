package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// Ways the guest memory-return read could fail (written before the code;
// docs/problems/guest-damon-reclaim-state-unreported-2026-10-22.md):
//  1. An older kernel, or one without CONFIG_DAMON_RECLAIM or
//     CONFIG_PAGE_REPORTING, has no parameter directory: that is absent
//     (present=false), not an error, and the procmem read still succeeds.
//  2. A kernel has only some of the counters (bytes_reclaimed_regions came
//     later than nr_reclaimed_regions): the ones that exist are reported and
//     the missing ones are omitted, never zero.
//  3. A file exists but cannot be read (not root): named in errors, the
//     rest still reported, the route still answers.
//  4. kdamond_pid is -1 (not running): no CPU is read. A pid that now
//     belongs to another process: no CPU is reported for it.
//  5. The balloon driver is not bound (no device): bound=false and no
//     feature claims. A features string too short to hold the bit: the
//     feature is unknown (omitted), not false.
//  6. /proc/buddyinfo is absent: no free-block summary; units are the
//     guest's page size, not the test host's.
//  7. The read writes something: it must leave every parameter file as it
//     found it (read only; no writer, no unit changes parameters).
//  8. A value is not a number: kept raw for inputs; for a counter, named in
//     errors and omitted.
//  9. page_reporting_order reads -1 until a reporting device registers (seen
//     on 6.18.37 without a balloon): no order is reportable then, so no
//     reportable total is claimed.

func writeFakeMemoryReturn(t *testing.T) (sysRoot, procRoot string) {
	t.Helper()
	sysRoot, procRoot = t.TempDir(), t.TempDir()
	write := func(root, rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dr := "module/damon_reclaim/parameters/"
	for name, v := range map[string]string{
		"enabled":                     "Y\n",
		"commit_inputs":               "N\n",
		"min_age":                     "300000000\n",
		"wmarks_high":                 "950\n",
		"wmarks_low":                  "5\n",
		"skip_anon":                   "Y\n",
		"kdamond_pid":                 "77\n",
		"nr_reclaim_tried_regions":    "40\n",
		"bytes_reclaim_tried_regions": "167772160\n",
		"nr_reclaimed_regions":        "30\n",
		"bytes_reclaimed_regions":     "125829120\n",
		"nr_quota_exceeds":            "2\n",
	} {
		write(sysRoot, dr+name, v)
	}
	write(sysRoot, "module/page_reporting/parameters/page_reporting_order", "9\n")
	// 64 feature bits as the kernel prints them: bits 1 (stats), 2
	// (deflate on OOM) and 5 (reporting) set, 3 (free page hint) clear.
	features := []byte("0110010000000000000000000000000000000000000000000000000000000000")
	write(sysRoot, "bus/virtio/drivers/virtio_balloon/virtio3/features", string(features)+"\n")
	write(sysRoot, "bus/virtio/drivers/virtio_balloon/uevent", "DRIVER=virtio_balloon\n")

	write(procRoot, "77/comm", "kdamond.0\n")
	// pid (comm) state ppid ... utime is field 14, stime field 15.
	write(procRoot, "77/stat", "77 (kdamond.0) S 2 0 0 0 -1 2129984 0 0 0 0 120 30 0 0 20 0 1 0 500 0 0\n")
	write(procRoot, "vmstat", "nr_free_pages 1000\npgmajfault 812\npgsteal_kswapd 5000\npgsteal_direct 7\nworkingset_refault_file 44\nballoon_inflate 0\nballoon_deflate 0\nunrelated_counter 9\n")
	// Two zones; orders 0..10. Order 9 and 10 are at or above the
	// reporting order.
	write(procRoot, "buddyinfo", "Node 0, zone      DMA      1      1      1      0      2      1      1      0      1      1      3\nNode 0, zone    DMA32     10      4      0      0      0      0      0      0      0      2      1\n")
	return sysRoot, procRoot
}

func TestReadMemoryReturnReportsDamonReportingAndBalloon(t *testing.T) {
	sysRoot, procRoot := writeFakeMemoryReturn(t)
	before, _ := os.ReadFile(filepath.Join(sysRoot, "module/damon_reclaim/parameters/enabled"))
	got := readMemoryReturn(sysRoot, procRoot, 4)

	d := got.DamonReclaim
	if !d.Present || d.Enabled == nil || !*d.Enabled {
		t.Fatalf("damon_reclaim = %+v, want present and enabled", d)
	}
	if d.KdamondPID == nil || *d.KdamondPID != 77 {
		t.Fatalf("kdamond_pid = %v, want 77", d.KdamondPID)
	}
	if d.KdamondCPUTicks == nil || *d.KdamondCPUTicks != 150 {
		t.Fatalf("kdamond cpu ticks = %v, want utime+stime 150", d.KdamondCPUTicks)
	}
	for k, want := range map[string]int64{
		"nr_reclaim_tried_regions": 40, "bytes_reclaim_tried_regions": 167772160,
		"nr_reclaimed_regions": 30, "bytes_reclaimed_regions": 125829120, "nr_quota_exceeds": 2,
	} {
		if v, ok := d.Stats[k]; !ok || v != want {
			t.Fatalf("stats[%s] = %d,%v want %d (stats %v)", k, v, ok, want, d.Stats)
		}
	}
	if d.Parameters["min_age"] != "300000000" || d.Parameters["skip_anon"] != "Y" || d.Parameters["commit_inputs"] != "N" {
		t.Fatalf("parameters = %v", d.Parameters)
	}
	for _, out := range []string{"enabled", "kdamond_pid", "bytes_reclaimed_regions"} {
		if _, dup := d.Parameters[out]; dup {
			t.Fatalf("parameters repeat the output %q: %v", out, d.Parameters)
		}
	}

	if !got.PageReporting.Present || got.PageReporting.Order == nil || *got.PageReporting.Order != 9 || !got.PageReporting.Registered {
		t.Fatalf("page_reporting = %+v", got.PageReporting)
	}
	b := got.Balloon
	if !b.Bound || b.Device != "virtio3" {
		t.Fatalf("balloon = %+v, want bound virtio3", b)
	}
	if b.Reporting == nil || !*b.Reporting || b.StatsVQ == nil || !*b.StatsVQ || b.DeflateOnOOM == nil || !*b.DeflateOnOOM || b.FreePageHint == nil || *b.FreePageHint {
		t.Fatalf("balloon features = %+v", b)
	}

	if got.Vmstat["pgmajfault"] != 812 || got.Vmstat["workingset_refault_file"] != 44 || got.Vmstat["pgsteal_kswapd"] != 5000 {
		t.Fatalf("vmstat = %v", got.Vmstat)
	}
	if _, ok := got.Vmstat["unrelated_counter"]; ok {
		t.Fatalf("vmstat carries an unselected counter: %v", got.Vmstat)
	}
	if _, ok := got.Vmstat["pgsteal_proactive"]; ok {
		t.Fatalf("an absent vmstat counter reads as present: %v", got.Vmstat)
	}

	fb := got.FreeBlocks
	if fb == nil || len(fb.KiBByOrder) != 11 {
		t.Fatalf("free_blocks = %+v", fb)
	}
	// order 0: (1+10) pages * 4 KiB; order 9: (1+2) * 512 pages * 4 KiB.
	if fb.KiBByOrder[0] != 44 || fb.KiBByOrder[9] != 3*512*4 || fb.KiBByOrder[10] != 4*1024*4 {
		t.Fatalf("kib by order = %v", fb.KiBByOrder)
	}
	if fb.ReportableKiB == nil || *fb.ReportableKiB != 3*512*4+4*1024*4 {
		t.Fatalf("reportable kib = %v, want orders >= 9", fb.ReportableKiB)
	}
	if len(got.Errors) != 0 {
		t.Fatalf("errors = %v", got.Errors)
	}
	after, _ := os.ReadFile(filepath.Join(sysRoot, "module/damon_reclaim/parameters/enabled"))
	if string(before) != string(after) {
		t.Fatal("the read changed a parameter file")
	}
}

func TestReadMemoryReturnOlderKernelIsAbsentNotError(t *testing.T) {
	got := readMemoryReturn(t.TempDir(), t.TempDir(), 4)
	if got.DamonReclaim.Present || got.DamonReclaim.Enabled != nil || got.DamonReclaim.KdamondPID != nil || len(got.DamonReclaim.Stats) != 0 {
		t.Fatalf("damon_reclaim on a kernel without it = %+v", got.DamonReclaim)
	}
	if got.PageReporting.Present || got.PageReporting.Order != nil {
		t.Fatalf("page_reporting = %+v", got.PageReporting)
	}
	if got.Balloon.Bound || got.Balloon.Reporting != nil {
		t.Fatalf("balloon = %+v", got.Balloon)
	}
	if got.FreeBlocks != nil || len(got.Vmstat) != 0 {
		t.Fatalf("free_blocks %+v vmstat %v", got.FreeBlocks, got.Vmstat)
	}
	if len(got.Errors) != 0 {
		t.Fatalf("absent files reported as errors: %v", got.Errors)
	}
}

func TestReadMemoryReturnPartialAndStoppedKdamond(t *testing.T) {
	sysRoot, procRoot := writeFakeMemoryReturn(t)
	dr := filepath.Join(sysRoot, "module/damon_reclaim/parameters")
	// An older kernel: no bytes_* counters.
	for _, n := range []string{"bytes_reclaim_tried_regions", "bytes_reclaimed_regions"} {
		if err := os.Remove(filepath.Join(dr, n)); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite := func(p, body string) {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(filepath.Join(dr, "enabled"), "N\n")
	mustWrite(filepath.Join(dr, "kdamond_pid"), "-1\n")
	mustWrite(filepath.Join(dr, "nr_quota_exceeds"), "lots\n")
	// No reporting bit can be read from a short string.
	mustWrite(filepath.Join(sysRoot, "bus/virtio/drivers/virtio_balloon/virtio3/features"), "011\n")

	got := readMemoryReturn(sysRoot, procRoot, 4)
	d := got.DamonReclaim
	if !d.Present || d.Enabled == nil || *d.Enabled {
		t.Fatalf("enabled = %v, want present and false", d.Enabled)
	}
	if d.KdamondPID == nil || *d.KdamondPID != -1 || d.KdamondCPUTicks != nil {
		t.Fatalf("stopped kdamond: pid %v ticks %v", d.KdamondPID, d.KdamondCPUTicks)
	}
	if _, ok := d.Stats["bytes_reclaimed_regions"]; ok {
		t.Fatalf("an absent counter reads as present: %v", d.Stats)
	}
	if _, ok := d.Stats["nr_quota_exceeds"]; ok {
		t.Fatalf("a non-numeric counter reads as a number: %v", d.Stats)
	}
	if d.Stats["nr_reclaimed_regions"] != 30 {
		t.Fatalf("stats = %v", d.Stats)
	}
	if len(got.Errors) != 1 {
		t.Fatalf("errors = %v, want one for nr_quota_exceeds", got.Errors)
	}
	if !got.Balloon.Bound || got.Balloon.StatsVQ == nil || !*got.Balloon.StatsVQ || got.Balloon.Reporting != nil {
		t.Fatalf("short features: %+v (bits 0..2 known, reporting unknown)", got.Balloon)
	}
}

func TestReadMemoryReturnUnregisteredReportingClaimsNothing(t *testing.T) {
	sysRoot, procRoot := writeFakeMemoryReturn(t)
	if err := os.WriteFile(filepath.Join(sysRoot, "module/page_reporting/parameters/page_reporting_order"), []byte("-1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := readMemoryReturn(sysRoot, procRoot, 4)
	if !got.PageReporting.Present || got.PageReporting.Order == nil || *got.PageReporting.Order != -1 || got.PageReporting.Registered {
		t.Fatalf("page_reporting = %+v, want present, order -1, not registered", got.PageReporting)
	}
	if got.FreeBlocks == nil || got.FreeBlocks.ReportableKiB != nil {
		t.Fatalf("free_blocks = %+v, want blocks but no reportable total", got.FreeBlocks)
	}
}

func TestReadMemoryReturnReusedPidIsNotKdamond(t *testing.T) {
	sysRoot, procRoot := writeFakeMemoryReturn(t)
	if err := os.WriteFile(filepath.Join(procRoot, "77/comm"), []byte("bash\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readMemoryReturn(sysRoot, procRoot, 4); got.DamonReclaim.KdamondCPUTicks != nil {
		t.Fatalf("cpu of a non-kdamond pid reported: %v", *got.DamonReclaim.KdamondCPUTicks)
	}
}

func TestReadMemoryReturnUnreadableFileIsNamed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a 0000 file")
	}
	sysRoot, procRoot := writeFakeMemoryReturn(t)
	p := filepath.Join(sysRoot, "module/damon_reclaim/parameters/kdamond_pid")
	if err := os.Chmod(p, 0); err != nil {
		t.Fatal(err)
	}
	got := readMemoryReturn(sysRoot, procRoot, 4)
	if got.DamonReclaim.KdamondPID != nil || got.DamonReclaim.Enabled == nil {
		t.Fatalf("damon_reclaim = %+v", got.DamonReclaim)
	}
	if len(got.Errors) != 1 {
		t.Fatalf("errors = %v, want the unreadable kdamond_pid", got.Errors)
	}
}

func TestProcMemCarriesMemoryReturn(t *testing.T) {
	if _, err := os.Stat("/proc/meminfo"); err != nil {
		t.Skip("no /proc")
	}
	req := httptest.NewRequest(http.MethodGet, DebugPprofPrefix+"procmem", nil)
	req.RemoteAddr = "127.0.0.1:4000"
	rec := httptest.NewRecorder()
	serveDebugPprof(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("procmem %d %q", rec.Code, rec.Body.String())
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	var mr map[string]json.RawMessage
	if err := json.Unmarshal(body["memory_return"], &mr); err != nil {
		t.Fatalf("procmem has no memory_return object: %s", rec.Body.String())
	}
	for _, k := range []string{"damon_reclaim", "page_reporting", "balloon"} {
		if _, ok := mr[k]; !ok {
			t.Fatalf("memory_return lacks %s: %s", k, body["memory_return"])
		}
	}
}
