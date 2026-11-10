package computerevent

import (
	"fmt"
	"strings"
)

// The shot_reuse event (docs/design/shot-verdict-reuse-2026-11-09.md, section
// 12). A clean rebase re-applies a change management already judged. When the
// source the judged shots depend on is unchanged between the base that pass was
// judged on and the booted base, and a platform liveness pass over the
// candidate is clean, the gate commits this event in place of a new judged
// round. It sits under the same idempotency key as a shot_verdict
// (ShotVerdictKey) so the gate keeps one lookup, and carries the same binding
// fields; the reader accepts either kind for the operation and host build job
// it names and no other.
//
// Failure modes decided first (the full list is section 12.5):
//
//   - A reuse for another operation, job, bundle or plan unlocks this swap
//     (FM2): the payload repeats all four and the reader checks them.
//   - A reuse with no prior pass, no origin or an unverified liveness result is
//     refused at validation; a field that is empty never stands for a match.
//   - A chain of reuses drifts from what was judged: every event records the
//     judged source ids unchanged from its origin, and the origin's event
//     digest and key, so the next hop re-reads the origin from the tape.
//   - The event moves no head, and the reducer binds DecisionRef to the live
//     pending decision like a verdict.

// ShotReuseMediaType is the payload's media type.
const ShotReuseMediaType = "application/vnd.choir.shot-reuse+json"

// ShotReuseReason is the Reason every reuse carries.
const ShotReuseReason = "reused"

// ShotReuseMaxChain bounds how many reuses may stack on one judged origin.
const ShotReuseMaxChain = 32

// ShotInputIDs are the git object ids of the source the shots depend on,
// read at one base commit: the trees of frontend/ and nix/ and of the shot
// driver's two packages, and the blobs of flake.nix and flake.lock. They are
// source ids, never build output: the host build bakes the revision and date
// into the bundle, so an output digest differs on every rebase.
type ShotInputIDs struct {
	Frontend    string `json:"frontend"`
	Nix         string `json:"nix"`
	FlakeNix    string `json:"flake_nix"`
	FlakeLock   string `json:"flake_lock"`
	Shot        string `json:"shot_driver"`
	ShotSandbox string `json:"shot_sandbox"`
	// The rest is what the shot pass runs or renders from the running binary
	// and the repository (review round 2, design 12.7): the module pins
	// (chromedp and friends), the sandbox, files-layer and state-copy code,
	// the package bundler, the first-party desk lists and the style files. An
	// event recorded before they existed lacks them and so is incomplete.
	GoMod            string `json:"go_mod"`
	GoSum            string `json:"go_sum"`
	Capsule          string `json:"capsule"`
	Lease            string `json:"lease"`
	ObjectGraph      string `json:"objectgraph"`
	PkgRuntime       string `json:"pkgruntime"`
	AppdeskFirstpart string `json:"appdesk_firstparty"`
	AppdeskRequired  string `json:"appdesk_required"`
	Styles           string `json:"styles"`
}

// ShotInputPaths are the repository paths ShotInputIDs holds, in List order.
var ShotInputPaths = []string{
	"frontend", "nix", "flake.nix", "flake.lock", "internal/shot", "internal/shotsandbox",
	"go.mod", "go.sum", "internal/capsule", "internal/lease", "internal/objectgraph", "internal/pkgruntime",
	"internal/appdesk/firstparty.json", "internal/appdesk/required.json", "styles",
}

// List is the ids in ShotInputPaths order.
func (i ShotInputIDs) List() []string {
	return []string{i.Frontend, i.Nix, i.FlakeNix, i.FlakeLock, i.Shot, i.ShotSandbox,
		i.GoMod, i.GoSum, i.Capsule, i.Lease, i.ObjectGraph, i.PkgRuntime, i.AppdeskFirstpart, i.AppdeskRequired, i.Styles}
}

// ShotInputIDsFromList is List's inverse; a list of the wrong length is not ids.
func ShotInputIDsFromList(ids []string) (ShotInputIDs, bool) {
	if len(ids) != len(ShotInputPaths) {
		return ShotInputIDs{}, false
	}
	return ShotInputIDs{ids[0], ids[1], ids[2], ids[3], ids[4], ids[5], ids[6], ids[7], ids[8], ids[9], ids[10], ids[11], ids[12], ids[13], ids[14]}, true
}

// Complete says every id is a non-empty git object id.
func (i ShotInputIDs) Complete() bool {
	for _, id := range i.List() {
		if !IsGitObjectID(id) {
			return false
		}
	}
	return true
}

// IsGitObjectID reports a 40 or 64 digit lowercase hex git object id.
func IsGitObjectID(id string) bool {
	if len(id) != 40 && len(id) != 64 {
		return false
	}
	for _, r := range id {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// ShotDriverEnv is the shot driver's environment as the sandbox sees it: the
// browser, the fontconfig file and ffmpeg (nix store paths).
type ShotDriverEnv struct {
	Browser    string `json:"browser"`
	Fontconfig string `json:"fontconfig"`
	Ffmpeg     string `json:"ffmpeg,omitempty"`
}

// ShotLiveness is the platform's page-level check of the candidate: the app
// reached its ready marker and the page's /api calls were counted.
type ShotLiveness struct {
	Passed bool `json:"passed"`
	// Requests and Failures are the page's /api responses counted by the
	// stage's frontend proxy; a failure is a status of 400 or more.
	Requests int `json:"requests"`
	Failures int `json:"failures"`
	// ByStatus is the status histogram, kept as evidence.
	ByStatus map[string]int `json:"by_status,omitempty"`
	Marker   string         `json:"marker"`
}

// ShotReusePayload is the shot_reuse event's payload.
type ShotReusePayload struct {
	OperationID    string `json:"operation_id"`
	BundleDigest   string `json:"bundle_digest"`
	HostBuildJobID string `json:"host_build_job_id"`
	// ShotJobID names the liveness pass.
	ShotJobID  string `json:"shot_job_id"`
	PlanSHA256 string `json:"plan_sha256"`
	Verdict    string `json:"verdict"`
	Reason     string `json:"reason"`

	// RebaseOf is the operation this one re-applies; PriorEvent the digest of
	// the shot_verdict or shot_reuse event of RebaseOf that this reuse stands
	// on, and PriorKind its kind.
	RebaseOf   string `json:"rebase_of"`
	PriorEvent string `json:"prior_event"`
	PriorKind  string `json:"prior_kind"`
	// OriginEvent and OriginKey name the shot_verdict pass the chain bottoms
	// out in (equal to PriorEvent for a first hop); ChainDepth counts the
	// reuses between it and this one, this one included.
	OriginEvent string `json:"origin_event"`
	OriginKey   string `json:"origin_key"`
	ChainDepth  int    `json:"chain_depth"`

	// JudgedBase and Judged are the base the origin was judged on and the
	// source ids there; BootedBase and Booted the same for the base now
	// booted. Judged == Booted is the reuse condition.
	JudgedBase string       `json:"judged_base"`
	Judged     ShotInputIDs `json:"judged"`
	BootedBase string       `json:"booted_base"`
	Booted     ShotInputIDs `json:"booted"`

	Driver   ShotDriverEnv `json:"driver"`
	Liveness ShotLiveness  `json:"liveness"`
}

// Validate checks the payload's own shape. The binding to an operation, a host
// build and a plan is checked by the reader against the operation it serves.
func (p ShotReusePayload) Validate() error {
	if strings.TrimSpace(p.OperationID) == "" || strings.TrimSpace(p.HostBuildJobID) == "" || strings.TrimSpace(p.ShotJobID) == "" ||
		!IsSHA256(p.BundleDigest) || !IsSHA256(p.PlanSHA256) {
		return fmt.Errorf("shot reuse: operation, bundle digest, host build job, liveness job and plan digest are required")
	}
	if p.Verdict != ShotVerdictPass || p.Reason != ShotReuseReason {
		return fmt.Errorf("shot reuse: a reuse is a pass with the reason %q", ShotReuseReason)
	}
	if strings.TrimSpace(p.RebaseOf) == "" || p.RebaseOf == p.OperationID || !IsSHA256(p.PriorEvent) || !IsSHA256(p.OriginEvent) ||
		strings.TrimSpace(p.OriginKey) == "" || !strings.HasPrefix(p.OriginKey, "shot-verdict:") {
		return fmt.Errorf("shot reuse: the prior operation, prior event and origin are required")
	}
	if p.PriorKind != string(EventShotVerdict) && p.PriorKind != string(EventShotReuse) {
		return fmt.Errorf("shot reuse: the prior event is a %q", p.PriorKind)
	}
	if p.ChainDepth < 1 || p.ChainDepth > ShotReuseMaxChain || (p.PriorKind == string(EventShotVerdict)) != (p.ChainDepth == 1) {
		return fmt.Errorf("shot reuse: chain depth %d does not fit a prior %s", p.ChainDepth, p.PriorKind)
	}
	if !IsGitObjectID(p.JudgedBase) || !IsGitObjectID(p.BootedBase) || !p.Judged.Complete() || !p.Booted.Complete() || p.Judged != p.Booted {
		return fmt.Errorf("shot reuse: the judged and booted source ids must be complete and equal")
	}
	if strings.TrimSpace(p.Driver.Browser) == "" || strings.TrimSpace(p.Driver.Fontconfig) == "" {
		return fmt.Errorf("shot reuse: the driver environment is required")
	}
	if !p.Liveness.Passed || p.Liveness.Requests < 1 || p.Liveness.Failures != 0 || strings.TrimSpace(p.Liveness.Marker) == "" {
		return fmt.Errorf("shot reuse: the liveness pass did not pass")
	}
	return nil
}
