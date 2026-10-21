// Package doctool runs the document tools (pdftotext, pdfinfo, pandoc) that
// parse untrusted bytes, sandboxed (SDC SEC-15).
//
// Before it the guest runtime ran them as guest root with the guest's network
// and no limits (internal/content/extract.go). Now, when the mode allows, the
// daemon re-executes itself as a helper (`autoputer doc-tool-sandbox`) that
// it starts as the sandbox user (SandboxUID, no capabilities) in an empty
// network and IPC namespace (under required, also in a cgroup of its own
// under the document tools' cgroup root, never the desks').
// The helper sets rlimits, no_new_privs, Landlock V5 (read and execute the
// tool's roots; read its one input directory and its own /proc entry; write
// nothing; no TCP) and a seccomp deny list, then execs the tool. The tool's
// output comes back through a pipe, capped.
//
// Designed with its failure modes (D1-D11) in
// docs/definitions/choir-launch-fp2-desk-boundary-2026-10-21.md, "Second
// batch".
package doctool

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ModeEnv selects the mode. The guest image sets it.
const ModeEnv = "CHOIR_DOCUMENT_TOOL_SANDBOX"

// Mode is how a document tool runs.
type Mode string

const (
	// ModeOff runs the tool directly, as before SEC-15.
	ModeOff Mode = "off"
	// ModeObserve runs the tool directly, as before: that run is the result,
	// unchanged. A sandboxed shadow run of the same input follows (one at a
	// time, no cgroup, so today's kill domain is unchanged) and only counts:
	// equal, diverged, failed in the sandbox, or not set up (ShadowSnapshot).
	ModeObserve Mode = "observe"
	// ModeRequired sandboxes the tool or does not run it (warning
	// document_tool_sandbox_refused); the extraction falls back to its
	// pure-Go projection, as when a tool is missing.
	ModeRequired Mode = "required"
)

// ModeFromEnv reads the mode: unset or "off" is off; a value it does not know
// is required (a typo strengthens the policy).
func ModeFromEnv(getenv func(string) string) Mode {
	if getenv == nil {
		getenv = os.Getenv
	}
	switch v := Mode(strings.ToLower(strings.TrimSpace(getenv(ModeEnv)))); v {
	case "", ModeOff:
		return ModeOff
	case ModeObserve:
		return ModeObserve
	default:
		return ModeRequired
	}
}

const (
	// SandboxUID is the sandbox user's uid and gid (choir-doctool in the
	// guest image; the spawner uses the number and never looks it up).
	SandboxUID = 61010

	// HelperArg is the daemon's argv[1] for the helper; ProbeArg for the
	// self-test's probe tool (cmd/autoputer/main.go).
	HelperArg = "doc-tool-sandbox"
	ProbeArg  = "doc-tool-probe"

	// cgroupRootEnv names the document tools' own cgroup root, which the
	// daemon makes at start (EnsureToolCgroupRoot). Under required a tool's
	// cgroup is a child named toolCgroupPrefix*, swept from this root alone
	// at the next start. Never the desks cgroup: a converter charged there
	// could get a desk worker killed (review B2).
	cgroupRootEnv    = "CHOIR_DOCTOOL_CGROUP_ROOT"
	toolCgroupPrefix = "t-"
	// toolAggregateMax bounds all sandboxed tools together (two at their
	// budget); a first value, like toolMemoryMax.
	toolAggregateMax = 2 * toolMemoryMax

	// toolMemoryMax is a tool's hard memory budget (a guess: the staging
	// probe reads memory.peak of real runs); toolPidsMax its process cap.
	toolMemoryMax = 1 << 30
	toolPidsMax   = 64

	// toolTimeout bounds one run, as before SEC-15.
	toolTimeout = 30 * time.Second
	// defaultOutputCap bounds what a tool may print into the daemon.
	defaultOutputCap = 64 << 20

	// helperSetupExit is the helper's exit status when it could not set up
	// the sandbox; its stderr then starts with helperSetupPrefix. A tool's
	// own failure never looks like it (the prefix is written before exec).
	helperSetupExit   = 125
	helperSetupPrefix = "doctool-sandbox: "
)

// runOptions is one run's configuration; the zero value of the test fields is
// production.
type runOptions struct {
	mode      Mode
	helperBin string // "" is os.Executable()
	outputCap int    // 0 is defaultOutputCap
	// testUserNamespace replaces the identity switch with a user namespace
	// that maps only the caller's uid, so a non-root test can run the
	// sandbox; testNoCgroup runs it without a cgroup. Production sets
	// neither.
	testUserNamespace bool
	testNoCgroup      bool
	// syncShadow makes observe wait for its shadow run (tests).
	syncShadow bool
}

// RunOnTempFile runs tool with prefixArgs, the input's path, and suffixArgs,
// the input being raw written to a file with extension ext, and returns its
// stdout and the warnings for the extraction. The mode comes from ModeEnv.
func RunOnTempFile(ctx context.Context, tool string, prefixArgs []string, ext string, raw []byte, suffixArgs []string) (string, []string) {
	return run(ctx, runOptions{mode: ModeFromEnv(nil)}, tool, prefixArgs, ext, raw, suffixArgs)
}

func run(ctx context.Context, opts runOptions, tool string, prefixArgs []string, ext string, raw []byte, suffixArgs []string) (string, []string) {
	toolPath, err := exec.LookPath(tool)
	if err != nil {
		return "", []string{tool + "_unavailable"}
	}
	switch opts.mode {
	case "", ModeOff:
		return runDirect(ctx, tool, toolPath, prefixArgs, ext, raw, suffixArgs)
	case ModeObserve:
		// Today's run is the result (review B3): nothing the shadow sees
		// reaches the extraction.
		out, warnings := runDirect(ctx, tool, toolPath, prefixArgs, ext, raw, suffixArgs)
		observeShadow(opts, tool, toolPath, prefixArgs, ext, raw, suffixArgs, out, len(warnings) == 0)
		return out, warnings
	}
	out, warnings, _, setupErr := runSandboxed(ctx, opts, true, tool, toolPath, prefixArgs, ext, raw, suffixArgs)
	if setupErr != nil {
		return "", []string{"document_tool_sandbox_refused: " + setupErr.Error()}
	}
	return out, warnings
}

// ShadowStats counts observe's shadow runs since start: what required mode
// would have returned compared with what the tool returned directly.
type ShadowStats struct {
	Runs          int64 `json:"runs"`
	Equal         int64 `json:"equal"`          // same output, or both failed
	Diverged      int64 `json:"diverged"`       // both succeeded, outputs differ
	SandboxFailed int64 `json:"sandbox_failed"` // succeeded directly, failed in the sandbox
	SetupFailed   int64 `json:"setup_failed"`   // the sandbox could not be set up
	Skipped       int64 `json:"skipped"`        // a shadow was already running
	// MaxRSSBytes is the largest shadow's peak RSS; OverBudget counts shadows
	// whose peak passed toolMemoryMax (required mode would have killed them).
	MaxRSSBytes int64 `json:"max_rss_bytes"`
	OverBudget  int64 `json:"over_budget"`
}

// Sub is s minus an earlier snapshot (MaxRSSBytes is s's).
func (s ShadowStats) Sub(o ShadowStats) ShadowStats {
	return ShadowStats{Runs: s.Runs - o.Runs, Equal: s.Equal - o.Equal, Diverged: s.Diverged - o.Diverged,
		SandboxFailed: s.SandboxFailed - o.SandboxFailed, SetupFailed: s.SetupFailed - o.SetupFailed,
		Skipped: s.Skipped - o.Skipped, MaxRSSBytes: s.MaxRSSBytes, OverBudget: s.OverBudget - o.OverBudget}
}

var (
	shadowMu    sync.Mutex
	shadowStats ShadowStats
	// shadowSlot lets one shadow run at a time: observe adds at most one
	// converter's memory to the daemon's cgroup, never a pile of them.
	shadowSlot = make(chan struct{}, 1)
)

// ShadowSnapshot returns observe's shadow counts since start.
func ShadowSnapshot() ShadowStats {
	shadowMu.Lock()
	defer shadowMu.Unlock()
	return shadowStats
}

// observeShadow runs the sandboxed shadow of a direct run that already
// returned out (ok: without warnings), after the request, and counts the
// outcome. It never touches the result.
func observeShadow(opts runOptions, tool, toolPath string, prefixArgs []string, ext string, raw []byte, suffixArgs []string, out string, ok bool) {
	select {
	case shadowSlot <- struct{}{}:
	default:
		shadowMu.Lock()
		shadowStats.Skipped++
		shadowMu.Unlock()
		return
	}
	shadow := func() {
		defer func() { <-shadowSlot }()
		sOut, sWarnings, maxRSS, setupErr := runSandboxed(context.Background(), opts, false, tool, toolPath, prefixArgs, ext, raw, suffixArgs)
		outcome := "equal"
		switch {
		case setupErr != nil:
			outcome = "setup_failed: " + setupErr.Error()
		case ok && len(sWarnings) != 0:
			outcome = "sandbox_failed: " + strings.Join(sWarnings, "; ")
		case ok && sOut != out:
			outcome = "diverged"
		}
		shadowMu.Lock()
		shadowStats.Runs++
		switch {
		case setupErr != nil:
			shadowStats.SetupFailed++
		case strings.HasPrefix(outcome, "sandbox_failed"):
			shadowStats.SandboxFailed++
		case outcome == "diverged":
			shadowStats.Diverged++
		default:
			shadowStats.Equal++
		}
		if maxRSS > shadowStats.MaxRSSBytes {
			shadowStats.MaxRSSBytes = maxRSS
		}
		if maxRSS > toolMemoryMax {
			shadowStats.OverBudget++
		}
		shadowMu.Unlock()
		if outcome != "equal" {
			log.Printf("doctool: observe: %s shadow %s (peak %d MiB)", tool, truncate(outcome, 300), maxRSS>>20)
		}
	}
	if opts.syncShadow {
		shadow()
		return
	}
	go shadow()
}

// errSetup marks a sandbox that could not be set up (as opposed to a tool
// that failed inside it).
type errSetup struct{ err error }

func (e errSetup) Error() string { return e.err.Error() }
func (e errSetup) Unwrap() error { return e.err }

func setupErrorf(format string, args ...any) error {
	return errSetup{fmt.Errorf(format, args...)}
}

// runDirect is the path before SEC-15: the tool as the daemon's own user,
// on a temp file.
func runDirect(ctx context.Context, tool, toolPath string, prefixArgs []string, ext string, raw []byte, suffixArgs []string) (string, []string) {
	tmp, err := os.CreateTemp("", "choir-document-*"+ext)
	if err != nil {
		return "", []string{"tempfile_create_failed: " + err.Error()}
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return "", []string{"tempfile_write_failed: " + err.Error()}
	}
	if err := tmp.Close(); err != nil {
		return "", []string{"tempfile_close_failed: " + err.Error()}
	}
	toolCtx, cancel := context.WithTimeout(ctx, toolTimeout)
	defer cancel()
	cmd := exec.CommandContext(toolCtx, toolPath, toolArgs(prefixArgs, tmpPath, suffixArgs)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", []string{toolFailure(tool, stderr.String(), err)}
	}
	return string(out), nil
}

func toolArgs(prefixArgs []string, input string, suffixArgs []string) []string {
	args := append([]string{}, prefixArgs...)
	args = append(args, input)
	return append(args, suffixArgs...)
}

func toolFailure(tool, stderr string, err error) string {
	warning := tool + "_failed"
	if msg := strings.TrimSpace(stderr); msg != "" {
		return warning + ": " + truncate(msg, 500)
	}
	return warning + ": " + err.Error()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// cappedBuffer collects a tool's stdout up to limit bytes; a write past it
// fails, which ends the copy from the pipe, and calls onOverflow (the run's
// cancel, which kills the tool).
type cappedBuffer struct {
	buf        bytes.Buffer
	limit      int
	overflow   bool
	onOverflow func()
}

var errOutputCap = errors.New("tool output exceeds its cap")

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b.overflow || b.buf.Len()+len(p) > b.limit {
		if !b.overflow && b.onOverflow != nil {
			b.onOverflow()
		}
		b.overflow = true
		return 0, errOutputCap
	}
	return b.buf.Write(p)
}
