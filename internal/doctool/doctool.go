// Package doctool runs the document tools (pdftotext, pdfinfo, pandoc) that
// parse untrusted bytes, sandboxed (SDC SEC-15).
//
// Before it the guest runtime ran them as guest root with the guest's network
// and no limits (internal/content/extract.go). Now, when the mode allows, the
// daemon re-executes itself as a helper (`autoputer doc-tool-sandbox`) that
// it starts as the sandbox user (SandboxUID, no capabilities) in an empty
// network and IPC namespace and a cgroup of its own under the desks cgroup.
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
	"os"
	"os/exec"
	"strings"
	"time"
)

// ModeEnv selects the mode. The guest image sets it.
const ModeEnv = "CHOIR_DOCUMENT_TOOL_SANDBOX"

// Mode is how a document tool runs.
type Mode string

const (
	// ModeOff runs the tool directly, as before SEC-15.
	ModeOff Mode = "off"
	// ModeObserve sandboxes the tool and, when the sandbox cannot be set
	// up, runs it directly with the warning document_tool_sandbox_fallback;
	// a tool that fails only inside the sandbox is run again directly with
	// the warning document_tool_sandbox_divergence.
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

	// cgroupRootEnv is the desks cgroup the daemon makes at start
	// (yaegikernel.DeskCgroupRootEnv). A tool's cgroup is a child named
	// toolCgroupPrefix*, which the desks cgroup's boot sweep removes after
	// a crash, as it removes a desk worker's.
	cgroupRootEnv    = "CHOIR_DESK_CGROUP_ROOT"
	toolCgroupPrefix = "w-doctool-"

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
	if opts.mode == "" || opts.mode == ModeOff {
		return runDirect(ctx, tool, toolPath, prefixArgs, ext, raw, suffixArgs)
	}
	out, warnings, setupErr := runSandboxed(ctx, opts, tool, toolPath, prefixArgs, ext, raw, suffixArgs)
	if setupErr == nil {
		if opts.mode == ModeObserve && len(warnings) != 0 {
			// Observe never makes an extraction worse: a tool that failed
			// inside the sandbox runs again as before, and if that works the
			// divergence is the warning (the sandbox refused something the
			// tool needs, or the input is hostile).
			if direct, directWarnings := runDirect(ctx, tool, toolPath, prefixArgs, ext, raw, suffixArgs); len(directWarnings) == 0 {
				return direct, []string{"document_tool_sandbox_divergence: " + strings.Join(warnings, "; ")}
			}
		}
		return out, warnings
	}
	if opts.mode == ModeObserve {
		out, warnings := runDirect(ctx, tool, toolPath, prefixArgs, ext, raw, suffixArgs)
		return out, append([]string{"document_tool_sandbox_fallback: " + setupErr.Error()}, warnings...)
	}
	return "", []string{"document_tool_sandbox_refused: " + setupErr.Error()}
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
