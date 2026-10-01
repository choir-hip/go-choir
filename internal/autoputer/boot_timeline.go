// Boot timeline observer (S0a). Records the guest-side boot phases of the
// autoputer process with CLOCK_MONOTONIC-relative offsets so they line up with
// systemd's monotonic unit timestamps, then serves one consolidated receipt at
// GET /internal/boot/timeline. Also serves GET /internal/diag/tcp-dial, a
// bounded guest-egress reachability probe used to measure tap->tap isolation.
// Both endpoints require the X-Internal-Caller marker; they emit presence
// flags and timings only — never token or credential material.
package autoputer

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
)

// bootTimelineClock is the process-local monotonic base. time.Now() in Go
// carries CLOCK_MONOTONIC, the same clock systemd exports as
// *TimestampMonotonic fields and journal __MONOTONIC_TIMESTAMP values, so
// marks recorded with it are directly comparable to systemd boot-relative
// timestamps after a one-time /proc/uptime offset calibration.
var bootTimelineClock = struct {
	sync.Mutex
	start      time.Time // wall clock at process init
	uptimeBase float64   // /proc/uptime seconds at process init
	marks      []bootTimelineMark
	replay     map[string]any // replay volume captured at replay_done
}{
	start:      time.Now(),
	uptimeBase: readProcUptimeSeconds(),
}

type bootTimelineMark struct {
	Phase        string `json:"phase"`
	At           string `json:"at"`             // RFC3339Nano wall clock
	SinceStartMS int64  `json:"since_start_ms"` // ms since process init
	SinceBootMS  int64  `json:"since_boot_ms"`  // ms since guest boot (monotonic)
	Detail       string `json:"detail,omitempty"`
}

// bootMark records one boot phase boundary. Marks after the first for a given
// phase suffix are kept distinct via caller-supplied unique phase names; a
// repeated name overwrites nothing and is appended (marks are append-only).
func bootMark(phase string) {
	bootMarkDetail(phase, "")
}

func bootMarkDetail(phase, detail string) {
	now := time.Now()
	bootTimelineClock.Lock()
	defer bootTimelineClock.Unlock()
	mark := bootTimelineMark{
		Phase:        phase,
		At:           now.UTC().Format(time.RFC3339Nano),
		SinceStartMS: now.Sub(bootTimelineClock.start).Milliseconds(),
		Detail:       detail,
	}
	if bootTimelineClock.uptimeBase > 0 {
		sinceBoot := bootTimelineClock.uptimeBase*1000 + float64(mark.SinceStartMS)
		mark.SinceBootMS = int64(sinceBoot)
	}
	bootTimelineClock.marks = append(bootTimelineClock.marks, mark)
}

// bootSetReplay records the post-replay snapshot so the receipt carries head
// position, committed position, and — separately — rows actually applied this
// boot (applied_rows), which is what boot cost is made of.
func bootSetReplay(seq, committed, applied uint64) {
	bootTimelineClock.Lock()
	defer bootTimelineClock.Unlock()
	bootTimelineClock.replay = map[string]any{
		"sequence":           seq,
		"committed_sequence": committed,
		"applied_rows":       applied,
	}
}

// readProcUptimeSeconds reads /proc/uptime's first field (seconds since boot,
// CLOCK_MONOTONIC-compatible on Linux). Returns 0 when unavailable (non-Linux
// dev machines), which degrades since_boot_ms to process-relative only.
func readProcUptimeSeconds() float64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return v
}

func readBootID() string {
	data, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// guestInternalCaller gates the diagnostics surface the same way the runtime
// API gates internal routes: the vmctl host caller and the autoputer-proxy
// forwarder both set X-Internal-Caller.
func guestInternalCaller(r *http.Request) bool {
	return r.Header.Get("X-Internal-Caller") == "true"
}

// runGuestDiagTool executes a read-only observer command with a hard timeout
// and bounded output. Every collection failure is evidence: the receipt
// records missing observers rather than substituting silence.
func runGuestDiagTool(timeout time.Duration, outputCap int64, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("%s timed out after %s", name, timeout)
	}
	out := stdout.String()
	if int64(len(out)) > outputCap {
		out = out[:outputCap]
	}
	if err != nil {
		return out, fmt.Errorf("%s: %v: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// goChoirSystemdUnits are the units whose monotonic timestamps attribute the
// guest service chain. Order is the declaration order in
// nix/autoputer-vm.nix; the receipt reports effective ordering, not intent.
var goChoirSystemdUnits = []string{
	"go-choir-configure-network.service",
	"systemd-networkd.service",
	"systemd-networkd-wait-online.service",
	"network-online.target",
	"go-choir-extract-cmdline.service",
	"go-choir-guest-receipt-signer-state-migration.service",
	"go-choir-guest-receipt-signer.service",
	"go-choir-verifier-signer.service",
	"go-choir-kernel-capability-probe.service",
	"go-choir-updater.service",
	"go-choir-autoputer.service",
	"multi-user.target",
}

// parseSystemdAnalyzeTime parses `systemd-analyze time` text output:
//
//	Startup finished in 1.234s (kernel) + 2.345s (initrd) + 5.678s (userspace) = 9.257s
//	multi-user.target reached after 9.256s in userspace
func parseSystemdAnalyzeTime(raw string) map[string]any {
	out := map[string]any{}
	// The summary line is "<n>s (kernel) + <n>s (initrd) + <n>s (userspace)
	// = <n>s." — each duration precedes its label, so walk backward.
	parseBefore := func(label string) (float64, bool) {
		idx := strings.Index(raw, label)
		if idx < 0 {
			return 0, false
		}
		fields := strings.Fields(raw[:idx])
		if len(fields) == 0 {
			return 0, false
		}
		v, err := strconv.ParseFloat(strings.TrimSuffix(fields[len(fields)-1], "s"), 64)
		if err != nil {
			return 0, false
		}
		return v, true
	}
	if v, ok := parseBefore("(kernel)"); ok {
		out["kernel_s"] = v
	}
	if v, ok := parseBefore("(initrd)"); ok {
		out["initrd_s"] = v
	}
	if v, ok := parseBefore("(userspace)"); ok {
		out["userspace_s"] = v
	}
	if eq := strings.LastIndex(raw, "="); eq >= 0 {
		fields := strings.Fields(raw[eq+1:])
		if len(fields) > 0 {
			// "= 10.947s." — strip the trailing period before the suffix.
			tok := strings.TrimSuffix(strings.TrimSuffix(fields[0], "."), "s")
			if v, err := strconv.ParseFloat(tok, 64); err == nil {
				out["total_s"] = v
			}
		}
	}
	if idx := strings.Index(raw, "reached after "); idx >= 0 {
		rest := raw[idx+len("reached after "):]
		tok := strings.Fields(rest)
		if len(tok) > 0 {
			if v, err := strconv.ParseFloat(strings.TrimSuffix(tok[0], "s"), 64); err == nil {
				out["target_reached_s"] = v
			}
		}
		// "<unit> reached after" — extract exactly the line containing idx.
		lineStart := strings.LastIndex(raw[:idx], "\n") + 1 // 0 when no newline
		lineEnd := len(raw)
		if nl := strings.Index(raw[idx:], "\n"); nl >= 0 {
			lineEnd = idx + nl
		}
		line := raw[lineStart:lineEnd]
		if sp := strings.Index(line, " reached after"); sp > 0 {
			out["target_unit"] = strings.TrimSpace(line[:sp])
		}
	}
	return out
}

// collectSystemdUnits queries the monotonic activation timestamps for the
// go-choir unit chain in one systemctl call per property class. systemctl on
// systemd 258 supports --output=json; when unavailable the key=value fallback
// parses the same fields from text.
func collectSystemdUnits() ([]map[string]any, string) {
	units := make([]map[string]any, 0, len(goChoirSystemdUnits))
	var firstErr string
	for _, unit := range goChoirSystemdUnits {
		raw, err := runGuestDiagTool(5*time.Second, 16384, "systemctl", "show", unit,
			"-p", "Id,LoadState,ActiveState,SubState,Result,"+
				"ConditionTimestampMonotonic,ActiveEnterTimestampMonotonic,"+
				"ExecMainStartTimestampMonotonic,ExecMainExitTimestampMonotonic,"+
				"InactiveEnterTimestampMonotonic")
		entry := map[string]any{"unit": unit}
		if err != nil {
			entry["error"] = err.Error()
			if firstErr == "" {
				firstErr = err.Error()
			}
			units = append(units, entry)
			continue
		}
		for _, line := range strings.Split(raw, "\n") {
			kv := strings.SplitN(line, "=", 2)
			if len(kv) != 2 {
				continue
			}
			key, val := kv[0], strings.TrimSpace(kv[1])
			if val == "" || val == "0" {
				continue
			}
			switch key {
			case "Id", "LoadState", "ActiveState", "SubState", "Result":
				entry[strings.ToLower(key)] = val
			default:
				// Monotonic timestamps arrive in microseconds.
				if n, convErr := strconv.ParseInt(val, 10, 64); convErr == nil && strings.HasSuffix(key, "Monotonic") {
					name := strings.TrimSuffix(key, "Monotonic")
					name = strings.TrimSuffix(name, "Timestamp")
					outKey := lowerFirst(name) + "_ms"
					entry[outKey] = n / 1000
				}
			}
		}
		units = append(units, entry)
	}
	return units, firstErr
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// collectJournalBootEvents reads this boot's journal for the go-choir units
// and systemd lifecycle boundaries, returning monotonic timestamped entries.
// journalctl reads journal files directly; no D-Bus is required.
func collectJournalBootEvents() ([]map[string]any, string) {
	args := []string{
		"-b", "-o", "json", "--no-pager", "-n", "20000",
		"_COMM=systemd",
	}
	for _, unit := range goChoirSystemdUnits {
		args = append(args, "-u", unit)
	}
	raw, err := runGuestDiagTool(15*time.Second, 1<<20, "journalctl", args...)
	if err != nil {
		return nil, err.Error()
	}
	events := make([]map[string]any, 0, 256)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var rec map[string]any
		if jsonErr := json.Unmarshal([]byte(line), &rec); jsonErr != nil {
			continue
		}
		msg, _ := rec["MESSAGE"].(string)
		unit, _ := rec["UNIT"].(string)
		if unit == "" {
			unit, _ = rec["OBJECT_UNIT"].(string)
		}
		// Keep only lifecycle-bearing lines: unit state transitions and
		// systemd's own phase announcements.
		if !isBootJournalEvent(msg) {
			continue
		}
		var monoUS int64
		switch v := rec["__MONOTONIC_TIMESTAMP"].(type) {
		case string:
			monoUS, _ = strconv.ParseInt(v, 10, 64)
		case float64:
			monoUS = int64(v)
		}
		rt, _ := rec["__REALTIME_TIMESTAMP"].(string)
		events = append(events, map[string]any{
			"unit":         unit,
			"message":      msg,
			"monotonic_ms": monoUS / 1000,
			"realtime_us":  rt,
		})
	}
	return events, ""
}

func isBootJournalEvent(msg string) bool {
	for _, pat := range []string{
		"Starting ", "Started ", "Finished ", "Reached target",
		"Startup finished", "Stopped ", "Failed", "failed",
		"Listening on", "Reached target", "Activating",
	} {
		if strings.Contains(msg, pat) {
			return true
		}
	}
	return false
}

// cmdlineDigest returns /proc/cmdline parameters with every value redacted to
// presence flags. It answers "which params exist" (gateway_token presence is a
// finding) without ever serializing secrets into evidence.
func cmdlinePresence() map[string]any {
	raw, err := os.ReadFile("/proc/cmdline")
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	params := strings.Fields(string(raw))
	presence := map[string]any{"count": len(params)}
	for _, p := range params {
		name := p
		if i := strings.IndexByte(p, '='); i >= 0 {
			name = p[:i]
		}
		presence[name] = true
	}
	return presence
}

// mountEntry is one /proc/mounts row relevant to the store/data layout.
type mountEntry struct {
	Device   string `json:"device"`
	Point    string `json:"point"`
	FSType   string `json:"fstype"`
	ReadOnly bool   `json:"read_only"`
}

func collectMounts() []mountEntry {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return nil
	}
	interesting := map[string]bool{
		"/": true, "/nix/store": true, "/nix": true,
		"/mnt/persistent": true, "/run": true, "/run/choir-bootstrap": true,
		"/mnt": true, "/tmp": true, "/dev": true, "/proc": true, "/sys": true,
	}
	var out []mountEntry
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		if !interesting[fields[1]] {
			continue
		}
		ro := false
		for _, opt := range strings.Split(fields[3], ",") {
			if opt == "ro" {
				ro = true
			}
		}
		out = append(out, mountEntry{Device: fields[0], Point: fields[1], FSType: fields[2], ReadOnly: ro})
	}
	return out
}

func fileStatEvidence(path string) map[string]any {
	info, err := os.Lstat(path)
	if err != nil {
		return map[string]any{"path": path, "present": false, "error": err.Error()}
	}
	entry := map[string]any{
		"path":    path,
		"present": true,
		"mode":    fmt.Sprintf("%#o", info.Mode().Perm()),
		"size":    info.Size(),
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		entry["uid"] = int(st.Uid)
		entry["gid"] = int(st.Gid)
	}
	return entry
}

// collectGuestLayout gathers the store/mount/process evidence named by the
// s0a-guest-layout receipt: mount table, store path identity, disk usage, and
// the token surfaces' modes. Values are structural only.
func collectGuestLayout() map[string]any {
	layout := map[string]any{
		"mounts":   collectMounts(),
		"boot_id":  readBootID(),
		"uptime_s": readProcUptimeSeconds(),
		"cmdline":  cmdlinePresence(),
	}
	// Nix store identity: /run/current-system symlink + /nix/store entry count.
	if target, err := os.Readlink("/run/current-system"); err == nil {
		layout["current_system"] = target
	}
	if entries, err := os.ReadDir("/nix/store"); err == nil {
		layout["nix_store_entries"] = len(entries)
	} else {
		layout["nix_store_error"] = err.Error()
	}
	// Disk usage of the persistent data mount.
	var st syscall.Statfs_t
	if err := syscall.Statfs("/mnt/persistent", &st); err == nil {
		layout["persistent_bytes_total"] = int64(st.Blocks) * int64(st.Bsize)
		layout["persistent_bytes_free"] = int64(st.Bavail) * int64(st.Bsize)
	}
	layout["files"] = []map[string]any{
		fileStatEvidence("/run/go-choir-autoputer.env"),
		fileStatEvidence("/run/go-choir-updater.env"),
		fileStatEvidence("/run/go-choir-guest-deploy-receipt.json"),
		fileStatEvidence("/mnt/persistent/gateway-token"),
		fileStatEvidence("/mnt/persistent/choir-credentials/privacy-key"),
	}
	// Process env: presence flags for the credential-bearing variables only.
	env := os.Environ()
	presence := map[string]bool{}
	for _, name := range []string{"RUNTIME_GATEWAY_TOKEN", "CHOIR_COMPUTER_CREDENTIAL_FILE", "CHOIR_PRIVACY_KEY_FILE"} {
		for _, e := range env {
			if strings.HasPrefix(e, name+"=") {
				presence[name] = true
			}
		}
	}
	layout["env_presence"] = presence
	return layout
}

// guestIdentity returns the runtime + image identity records the receipt must
// carry: build info (ldflags), the deployed receipt the guest activated, the
// image manifest digest, and the running executable path.
func guestIdentity() map[string]any {
	id := map[string]any{
		"build":   buildinfo.Snapshot("autoputer"),
		"pid":     os.Getpid(),
		"boot_id": readBootID(),
	}
	if exe, err := os.Executable(); err == nil {
		id["executable"] = exe
		if resolved, resErr := filepath.EvalSymlinks(exe); resErr == nil {
			id["executable_resolved"] = resolved
		}
	}
	if raw, err := os.ReadFile("/run/go-choir-guest-deploy-receipt.json"); err == nil {
		var receipt map[string]any
		if json.Unmarshal(raw, &receipt) == nil {
			id["deploy_receipt"] = receipt
		}
	}
	if manifest := strings.TrimSpace(os.Getenv("CHOIR_GUEST_IMAGE_MANIFEST")); manifest != "" {
		if raw, err := os.ReadFile(manifest); err == nil {
			var m map[string]any
			if json.Unmarshal(raw, &m) == nil {
				id["image_manifest"] = m
			}
		}
	}
	if kr, err := runGuestDiagTool(3*time.Second, 4096, "uname", "-srmo"); err == nil {
		id["kernel"] = strings.TrimSpace(kr)
	}
	return id
}

// handleBootTimeline serves GET /internal/boot/timeline. It is registered on
// the server mux before Start, so it is available during and after replay;
// callers get marks even when /health is still gated 503.
func handleBootTimeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !guestInternalCaller(r) {
		http.Error(w, "internal caller marker required", http.StatusForbidden)
		return
	}
	bootTimelineClock.Lock()
	marks := make([]bootTimelineMark, len(bootTimelineClock.marks))
	copy(marks, bootTimelineClock.marks)
	var replay map[string]any
	if bootTimelineClock.replay != nil {
		replay = make(map[string]any, len(bootTimelineClock.replay))
		for k, v := range bootTimelineClock.replay {
			replay[k] = v
		}
	}
	processStart := bootTimelineClock.start
	uptimeBase := bootTimelineClock.uptimeBase
	bootTimelineClock.Unlock()

	units, unitsErr := collectSystemdUnits()
	journal, journalErr := collectJournalBootEvents()
	analyzeRaw, analyzeErr := runGuestDiagTool(10*time.Second, 8192, "systemd-analyze", "time")
	var analyze map[string]any
	if analyzeErr == nil {
		analyze = parseSystemdAnalyzeTime(analyzeRaw)
	}

	missing := []string{}
	if unitsErr != "" {
		missing = append(missing, "systemctl-unit-timestamps")
	}
	if journalErr != "" {
		missing = append(missing, "journal-boot-events")
	}
	if analyzeErr != nil {
		missing = append(missing, "systemd-analyze-time")
	}
	if uptimeBase <= 0 {
		missing = append(missing, "proc-uptime")
	}

	resp := map[string]any{
		"schema_version":     1,
		"kind":               "guest_boot_timeline",
		"collected_at":       time.Now().UTC().Format(time.RFC3339Nano),
		"process_started_at": processStart.UTC().Format(time.RFC3339Nano),
		"uptime_at_start_s":  uptimeBase,
		"marks":              marks,
		"replay":             replay,
		"systemd_analyze":    analyze,
		"units":              units,
		"journal_events":     journal,
		"identity":           guestIdentity(),
		"layout":             collectGuestLayout(),
		"missing_observers":  missing,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// handleDiagTCPDial serves GET /internal/diag/tcp-dial?addr=host:port. It
// performs one bounded TCP connect from inside the guest and reports the
// outcome. This is the tap->tap isolation oracle: a capsule-vs-host failure
// here is reachability evidence, a success is an openness finding. Bounded to
// IP literals + a 2s timeout; DNS names are refused so the probe can't be
// turned into a resolver oracle.
func handleDiagTCPDial(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !guestInternalCaller(r) {
		http.Error(w, "internal caller marker required", http.StatusForbidden)
		return
	}
	addr := strings.TrimSpace(r.URL.Query().Get("addr"))
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		http.Error(w, "addr must be host:port", http.StatusBadRequest)
		return
	}
	if ip := net.ParseIP(host); ip == nil {
		http.Error(w, "addr host must be an IP literal", http.StatusBadRequest)
		return
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		http.Error(w, "invalid port", http.StatusBadRequest)
		return
	}
	started := time.Now()
	dialer := net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.Dial("tcp", addr)
	latency := time.Since(started)
	result := map[string]any{
		"schema_version": 1,
		"kind":           "guest_tcp_dial",
		"addr":           addr,
		"latency_ms":     latency.Milliseconds(),
		"ok":             err == nil,
	}
	if err != nil {
		result["error"] = err.Error()
	} else {
		local := conn.LocalAddr().String()
		_ = conn.Close()
		result["local_addr"] = local
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
