// Package shotsandbox runs one candidate-shot pass: a server pair (an
// autoputer over a private state copy and a static frontend server that
// proxies /api to it as the owner) and the Chromium that shoots them, all
// inside a process that has a network namespace with only loopback up and the
// capsule's workload seccomp filter loaded
// (docs/design/source-only-freeze-and-candidate-shots-2026-11-06.md 3.3).
//
// Failure modes, decided before the code:
//
//   - FM3 the sandbox reaches the network: the stage process lives in a new
//     network namespace whose only interface is lo, and loads the capsule's
//     inet workload filter, so a socket is AF_UNIX, AF_INET or AF_INET6 and
//     nothing else; a dial to any address but loopback fails. The servers
//     and the browser are its children and inherit both.
//   - the sandbox writes the owner's live files: it runs as an unprivileged
//     id (capsule.ShotSandboxIDs) over a private state copy; the owner files
//     are hardlinks it cannot write, and everything it creates is under its
//     own work directory.
//   - FM9 the candidate does not start or serve: the stage waits for the
//     autoputer's port with the process watched, and a death or a silence
//     returns a Result naming the cause with the tail of its stderr.
//   - FM6 a hang: the autoputer wait, every view, the plan and the whole
//     stage have deadlines, and the stage's process group is killed when the
//     launcher's context ends.
//   - a port clash with 8082 or another shot: every port is an ephemeral
//     loopback port of the stage's own namespace.
//   - the per-view window reset: the shell restores saved windows on load, so
//     each view starts from an emptied desktop (shot.Options.BeforeView).
package shotsandbox

import (
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/shot"
)

// StageArg is argv[1] of the re-executed stage process. cmd/autoputer calls
// MaybeRunStage first thing in main.
const StageArg = "shot-sandbox-stage"

// StageEnv marks the stage process, so a launcher cannot be mistaken for it.
const StageEnv = "CHOIR_SHOT_SANDBOX_STAGE"

// OwnerUser is the identity the served frontend presents to the autoputer.
const OwnerUser = "owner"

// Spec is one pass: one server pair, one browser, the whole plan.
type Spec struct {
	// Binary is the autoputer to serve; Frontend its (or the running
	// build's) built frontend directory, holding index.html.
	Binary   string `json:"binary"`
	Frontend string `json:"frontend"`
	// StorePath is RUNTIME_STORE_PATH (<dir>/state, over <dir>/state.db) and
	// FilesRoot AUTOPUTER_FILES_ROOT: private copies, never the live ones.
	StorePath string `json:"store_path"`
	FilesRoot string `json:"files_root"`
	// OutDir receives <view>.png (and the video); WorkDir is the stage's
	// HOME and TMPDIR and holds the autoputer's log.
	OutDir  string           `json:"out_dir"`
	WorkDir string           `json:"work_dir"`
	Plan    capsule.ShotPlan `json:"plan"`

	Browser    string `json:"browser"`
	Ffmpeg     string `json:"ffmpeg,omitempty"`
	Fontconfig string `json:"fontconfig,omitempty"`
	// PassEnv is the environment the stage and its children get beside the
	// scrubbed base (PATH, LANG): the launcher's allowlist, never a whole
	// environment.
	PassEnv []string `json:"pass_env,omitempty"`

	ViewTimeoutS   int `json:"view_timeout_s,omitempty"`
	PlanTimeoutS   int `json:"plan_timeout_s,omitempty"`
	EncodeTimeoutS int `json:"encode_timeout_s,omitempty"`
	ReadyTimeoutS  int `json:"ready_timeout_s,omitempty"`

	// RunAsUID and RunAsGID are the ids the stage drops to once loopback is
	// up (set by the launcher; zero keeps the stage as it is).
	RunAsUID int `json:"run_as_uid,omitempty"`
	RunAsGID int `json:"run_as_gid,omitempty"`

	// Probe is a test-only stage action ("dial:<addr>") run in place of the
	// servers; a production Spec never sets it.
	Probe string `json:"probe,omitempty"`
}

// Result is what the stage reports on stdout.
type Result struct {
	Files []shot.Result `json:"files,omitempty"`
	// Error is the cause of a failed pass: a step, a timeout, a candidate
	// that did not start (with its stderr tail in Tail).
	Error string `json:"error,omitempty"`
	// Tail is the last of the autoputer's output, kept for a failure.
	Tail      string `json:"tail,omitempty"`
	ElapsedMS int64  `json:"elapsed_ms"`
	// Probe is the answer to a test probe.
	Probe string `json:"probe,omitempty"`
	// API is the page's /api responses as the stage's frontend proxy counted
	// them, set whenever the servers ran (the liveness evidence of the shot
	// verdict reuse).
	API *APIStats `json:"api,omitempty"`
}

func (s Spec) timeouts() shot.Timeouts {
	return shot.Timeouts{
		View:   time.Duration(s.ViewTimeoutS) * time.Second,
		Plan:   time.Duration(s.PlanTimeoutS) * time.Second,
		Encode: time.Duration(s.EncodeTimeoutS) * time.Second,
	}
}

func (s Spec) readyTimeout() time.Duration {
	if s.ReadyTimeoutS > 0 {
		return time.Duration(s.ReadyTimeoutS) * time.Second
	}
	return 60 * time.Second
}
