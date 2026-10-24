// Package sourceproj keeps the guest source repository a projection of what
// the computer has applied: the base commit (the booted base), plus the
// effective release's patch as one deterministic commit
// (docs/design/computer-update-architecture-rnd-2026-10-24.md, section 4.6,
// "U1 build notes"). The capsule seeds from that repository's clean HEAD, so
// a candidate is built on the applied source, not on the creation commit.
//
// Nothing here trusts a prior run. Reconcile recomputes the expected HEAD from
// its input every time; the refs and the facts it leaves are observation.
// Every function is a no-op unless Enabled.
package sourceproj

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// EnvSwitch is the runtime switch. Absent or anything but a true value means
// off, and then nothing in U1 runs.
const EnvSwitch = "CHOIR_SOURCE_PROJECTION"

// Enabled reports whether U1 is switched on for this process.
func Enabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(EnvSwitch))) {
	case "1", "true", "on", "yes":
		return true
	}
	return false
}

// Refs and trailers the projection leaves in the repository.
const (
	// RefBase points at the stack base: the booted base for an empty stack,
	// the base of the stack's patch otherwise. It is the one documented
	// rollback handle: `git reset --hard refs/choir/base`.
	RefBase = "refs/choir/base"
	// RefProjected points at the last commit the projection set as HEAD.
	RefProjected = "refs/choir/projected"
	// RefAppliedPrefix + the applied event digest points at the projection
	// commit of the operation that event applied.
	RefAppliedPrefix = "refs/choir/applied/"
	// Branch is the computer branch HEAD is attached to.
	Branch = "refs/heads/choir/computer"

	// TrailerStackBase names the commit a projection commit's patch was
	// applied to. Freeze reads it from the capsule's pinned commit, so the
	// composition never depends on mutable refs.
	TrailerStackBase = "Choir-Stack-Base"
)

// State is what reconcile found.
type State string

const (
	// StateDisabled: the switch is off; nothing was touched.
	StateDisabled State = "disabled"
	// StateBase: an empty stack; HEAD is the booted base.
	StateBase State = "base"
	// StateProjected: a live stack on the booted base; HEAD is its commit.
	StateProjected State = "projected"
	// StateUpstreamed: the stack's patch is already in the booted base, so it
	// is retired; HEAD is the booted base.
	StateUpstreamed State = "upstreamed"
	// StatePending: an apply is in flight; nothing was touched.
	StatePending State = "pending"
	// StateBaseMoved: a live stack on a base that is not the booted one, and
	// not upstreamed. The projection is kept; casting refuses.
	StateBaseMoved State = "stack_base_moved"
	// StateBaseObjectsMissing: the booted base's objects are not in the
	// guest repository (residual R3), so it can neither be moved to nor shown
	// to contain the stack. With an empty stack the cast proceeds as before.
	StateBaseObjectsMissing State = "base_objects_missing"
	// StateUnrebuildable: the expected commit cannot be built; casting refuses.
	StateUnrebuildable State = "source_projection_unrebuildable"
	// StateRepoUnavailable: no usable repository; nothing was touched.
	StateRepoUnavailable State = "repo_unavailable"
	// StateNoBootedBase: the booted base commit is unknown (a dev build);
	// nothing is moved.
	StateNoBootedBase State = "no_booted_base"
)

// Fact kinds the runtime logs.
const (
	FactBuilt      = "source_projection_built"
	FactRepaired   = "source_projection_repaired"
	FactUpstreamed = "source_projection_upstreamed"
	// FactLockCleared: a stale git lock left by a dead process was removed.
	FactLockCleared = "source_projection_lock_cleared"
)

// Op is the effective release's source operation: the head of the stack.
type Op struct {
	OperationID string
	// BundleDigest, ReleaseDigest and DecisionEvent are trailers; they make
	// the commit self-describing and, with CreatedAt, reproducible.
	BundleDigest  string
	ReleaseDigest string
	DecisionEvent string
	// AppliedEventDigest names the applied event; the ref is built from it.
	AppliedEventDigest string
	// CreatedAt is the commit date. It is the operation's creation time, a
	// tape-projected field, so a rebuild after a restore gets the same commit.
	CreatedAt time.Time
	// Base is the commit the bundle's patch is based on, and Patch its bytes.
	Base  string
	Patch []byte
}

// Input is everything Reconcile reads. The caller gathers it from the tape.
type Input struct {
	Repo string
	// BootedBase is the booted image's build commit ("" when unknown).
	BootedBase string
	// Pending: an apply is in flight (current names a release whose operation
	// is not recorded applied yet).
	Pending bool
	// PendingDetail names the operation that holds the apply open and the owner
	// act that clears it; the cast refusal carries it.
	PendingDetail string
	// Top is the effective release's operation with a source patch; nil is an
	// empty stack (a base release, a genesis baseline, a platform release).
	Top *Op
	// TopUnavailable is set instead of Top when the operation exists but its
	// bundle or patch cannot be read; the reason is named in the result.
	TopUnavailable string
}

// Result is what Reconcile did and found.
type Result struct {
	State  State  `json:"state"`
	Reason string `json:"reason,omitempty"`
	// Head is HEAD after reconcile ("" when untouched or unavailable).
	Head string `json:"head,omitempty"`
	// Base is the stack base (the booted base for an empty stack).
	Base string `json:"base,omitempty"`
	// BootedBase is the booted image's build commit as given.
	BootedBase string `json:"booted_base,omitempty"`
	// OperationID is the live (or retired) stack operation.
	OperationID string `json:"operation_id,omitempty"`
	// Diverged: a live stack. A retired (upstreamed) stack is not divergence.
	Diverged bool `json:"diverged"`
	// Changed: HEAD or the work tree was moved. Repaired: it was moved from
	// something this projection did not leave.
	Changed  bool `json:"changed,omitempty"`
	Repaired bool `json:"repaired,omitempty"`
	// LocksCleared lists stale git locks (index.lock, ref locks) removed because
	// a process that died left them and nothing live holds them.
	LocksCleared []string  `json:"locks_cleared,omitempty"`
	At           time.Time `json:"at"`
}

// Refusal names why a cast must be refused, or "" when it may go ahead.
// The first word is the stable reason code.
func (r Result) Refusal() string {
	switch r.State {
	case StatePending:
		detail := r.Reason
		if detail == "" {
			detail = "an apply is in flight"
		}
		return "source_projection_pending: " + detail + "; the capsule source is settled once it records applied"
	case StateBaseMoved:
		return "stack_base_moved: " + r.Reason
	case StateUnrebuildable:
		return "source_projection_unrebuildable: " + r.Reason
	case StateRepoUnavailable:
		return "source_projection_repo_unavailable: " + r.Reason
	}
	return ""
}

var reconcileMu sync.Mutex

// Reconcile makes the repository's HEAD the expected commit and its tracked
// files clean, and says what it found. It returns an error only for a git
// failure it could not name as a state; the states carry their own reasons.
func Reconcile(ctx context.Context, in Input) (Result, error) {
	res := Result{At: time.Now().UTC(), BootedBase: strings.ToLower(strings.TrimSpace(in.BootedBase))}
	if !Enabled() {
		res.State = StateDisabled
		return res, nil
	}
	reconcileMu.Lock()
	defer reconcileMu.Unlock()
	return reconcileLocked(ctx, in, res)
}

func reconcileLocked(ctx context.Context, in Input, res Result) (Result, error) {
	repo := strings.TrimSpace(in.Repo)
	if repo == "" {
		res.State, res.Reason = StateRepoUnavailable, "no source repository is configured"
		return res, nil
	}
	if info, err := os.Stat(filepath.Join(repo, ".git")); err != nil || !info.IsDir() {
		res.State, res.Reason = StateRepoUnavailable, "no git repository at "+repo
		return res, nil
	}
	// A crash can leave git's own lock files; a stale one would wedge every
	// later reconcile. Cleared once, only when its holder is provably dead.
	cleared, lockErr := clearStaleLocks(repo)
	if lockErr != nil {
		return res, lockErr
	}
	res.LocksCleared = cleared
	head, haveHead := resolveCommit(ctx, repo, "HEAD")
	if in.Pending {
		res.State, res.Head, res.Reason = StatePending, head, in.PendingDetail
		return res, nil
	}
	if in.TopUnavailable != "" {
		res.State, res.Reason, res.Head = StateUnrebuildable, in.TopUnavailable, head
		if in.Top != nil {
			res.OperationID = in.Top.OperationID
		}
		return res, nil
	}
	booted, haveBooted := "", false
	if res.BootedBase != "" {
		booted, haveBooted = resolveCommit(ctx, repo, res.BootedBase)
	}
	_, hadBase := resolveCommit(ctx, repo, RefBase)

	if in.Top == nil || len(in.Top.Patch) == 0 {
		return reconcileEmptyStack(ctx, repo, res, head, haveHead, booted, haveBooted, hadBase)
	}
	return reconcileStack(ctx, repo, in.Top, res, head, haveHead, booted, haveBooted, hadBase)
}

func reconcileEmptyStack(ctx context.Context, repo string, res Result, head string, haveHead bool, booted string, haveBooted, hadBase bool) (Result, error) {
	switch {
	case haveBooted:
		res.State, res.Base = StateBase, booted
	case res.BootedBase == "":
		res.State, res.Reason, res.Head = StateNoBootedBase, "the booted image names no build commit", head
		return res, nil
	default:
		// The booted base's objects are not here (R3). Keep the source as it
		// is, except to drop a retired stack's projection: an empty stack
		// means HEAD is the recorded base.
		res.State, res.Reason = StateBaseObjectsMissing, "the booted base "+res.BootedBase+" is not in the guest source repository"
		recorded, ok := resolveCommit(ctx, repo, RefBase)
		if !ok {
			res.Head = head
			return res, nil
		}
		res.Base = recorded
		booted = recorded
	}
	changed, repaired, err := ensureHead(ctx, repo, booted, head, haveHead, hadBase, "", booted)
	if err != nil {
		return res, err
	}
	res.Head, res.Changed, res.Repaired = booted, changed, repaired
	return res, nil
}

func reconcileStack(ctx context.Context, repo string, top *Op, res Result, head string, haveHead bool, booted string, haveBooted, hadBase bool) (Result, error) {
	res.OperationID = top.OperationID
	base, ok := resolveCommit(ctx, repo, top.Base)
	if !ok {
		res.State, res.Head = StateUnrebuildable, head
		res.Reason = "the patch base commit " + top.Base + " is not in the guest source repository"
		return res, nil
	}
	tree, err := patchedTree(ctx, repo, base, top.Patch, false)
	if err != nil {
		res.State, res.Head = StateUnrebuildable, head
		res.Reason = "the operation's patch does not apply to its base " + base + ": " + err.Error()
		return res, nil
	}
	if haveBooted {
		contained, cerr := Contained(ctx, repo, booted, base, top.Patch, tree)
		if cerr != nil {
			return res, cerr
		}
		if contained {
			res.State, res.Base, res.Diverged = StateUpstreamed, booted, false
			res.Reason = "the operation's patch is already in the booted base " + booted
			changed, repaired, err := ensureHead(ctx, repo, booted, head, haveHead, hadBase, "", booted)
			if err != nil {
				return res, err
			}
			res.Head, res.Changed, res.Repaired = booted, changed, repaired
			return res, nil
		}
	}
	commit, err := projectionCommit(ctx, repo, top, base, tree)
	if err != nil {
		return res, err
	}
	res.Base, res.Diverged = base, true
	switch {
	case haveBooted && base == booted:
		res.State = StateProjected
	case haveBooted:
		res.State = StateBaseMoved
		res.Reason = "the stack is based on " + base + " but this computer booted " + booted + " and the stack's patch is not in it"
	case res.BootedBase == "":
		res.State = StateProjected
		res.Reason = "the booted base is unknown"
	default:
		res.State = StateBaseMoved
		res.Reason = "the stack is based on " + base + " but this computer booted " + res.BootedBase + "; that commit is not in the guest source repository, so the stack cannot be shown to be upstreamed"
	}
	changed, repaired, err := ensureHead(ctx, repo, commit, head, haveHead, hadBase, top.AppliedEventDigest, base)
	if err != nil {
		return res, err
	}
	res.Head, res.Changed, res.Repaired = commit, changed, repaired
	return res, nil
}

// projectionCommit writes the operation's deterministic commit: tree is the
// patch applied to base; the parent is base; identity, date and message are
// functions of the operation, so rebuilding it gives the same commit id.
func projectionCommit(ctx context.Context, repo string, op *Op, base, tree string) (string, error) {
	when := op.CreatedAt.UTC().Unix()
	if when < 1 {
		when = 1
	}
	date := strconv.FormatInt(when, 10) + " +0000"
	env := []string{
		"GIT_AUTHOR_NAME=Choir Projection", "GIT_AUTHOR_EMAIL=projection@choir.local", "GIT_AUTHOR_DATE=" + date,
		"GIT_COMMITTER_NAME=Choir Projection", "GIT_COMMITTER_EMAIL=projection@choir.local", "GIT_COMMITTER_DATE=" + date,
	}
	var msg strings.Builder
	fmt.Fprintf(&msg, "choir: apply %s\n\n", op.OperationID)
	fmt.Fprintf(&msg, "%s: %s\n", TrailerStackBase, base)
	fmt.Fprintf(&msg, "Choir-Operation: %s\n", op.OperationID)
	if op.BundleDigest != "" {
		fmt.Fprintf(&msg, "Choir-Bundle: %s\n", op.BundleDigest)
	}
	if op.ReleaseDigest != "" {
		fmt.Fprintf(&msg, "Choir-Release: %s\n", op.ReleaseDigest)
	}
	if op.DecisionEvent != "" {
		fmt.Fprintf(&msg, "Choir-Decision-Event: %s\n", op.DecisionEvent)
	}
	out, err := git(ctx, repo, env, []byte(msg.String()), "-c", "commit.gpgsign=false", "commit-tree", tree, "-p", base, "-F", "-")
	if err != nil {
		return "", err
	}
	if !commitID.MatchString(out) {
		return "", fmt.Errorf("git wrote a malformed commit id %q", out)
	}
	return out, nil
}

// ensureHead makes expected the attached HEAD with clean tracked files, and
// sets the base ref (and the applied ref when appliedDigest is set). It
// reports whether it moved anything, and whether what it moved was something
// the projection did not leave (damage).
func ensureHead(ctx context.Context, repo, expected, head string, haveHead, hadBase bool, appliedDigest, base string) (changed, repaired bool, err error) {
	dirty, err := workTreeDirty(ctx, repo)
	if err != nil {
		return false, false, err
	}
	moved := !haveHead || head != expected
	// Read what the projection left before any ref moves: a HEAD that is none
	// of it was put there by something else (damage).
	known := true
	if moved && haveHead && hadBase {
		if known, err = projectionLeft(ctx, repo, head); err != nil {
			return false, false, err
		}
	}
	if appliedDigest != "" && commitID.MatchString(appliedDigest) {
		if _, err := git(ctx, repo, nil, nil, "update-ref", RefAppliedPrefix+appliedDigest, expected); err != nil {
			return false, false, err
		}
	}
	// The last commit the projection set, whether or not an applied event
	// named it: a HEAD that is this was left by us, not damage.
	if cur, ok := resolveCommit(ctx, repo, RefProjected); !ok || cur != expected {
		if _, err := git(ctx, repo, nil, nil, "update-ref", RefProjected, expected); err != nil {
			return false, false, err
		}
	}
	if cur, ok := resolveCommit(ctx, repo, RefBase); !ok || cur != base {
		if _, err := git(ctx, repo, nil, nil, "update-ref", RefBase, base); err != nil {
			return false, false, err
		}
	}
	if !moved && !dirty {
		return false, false, nil
	}
	repaired = dirty || !known
	if _, err := git(ctx, repo, nil, nil, "update-ref", Branch, expected); err != nil {
		return false, false, err
	}
	if _, err := git(ctx, repo, nil, nil, "symbolic-ref", "HEAD", Branch); err != nil {
		return false, false, err
	}
	if _, err := git(ctx, repo, nil, nil, "reset", "--hard", "--quiet", expected); err != nil {
		return false, false, err
	}
	return true, repaired, nil
}

// projectionLeft reports whether commit is one the projection left: a commit
// under refs/choir/ (the base it recorded, or an operation it projected). The
// computer branch is deliberately not in the set, so a commit made on it by
// something else reads as damage.
func projectionLeft(ctx context.Context, repo, commit string) (bool, error) {
	out, err := git(ctx, repo, nil, nil, "for-each-ref", "--format=%(objectname)", "refs/choir/")
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == commit {
			return true, nil
		}
	}
	return false, nil
}

// workTreeDirty reports whether a tracked file differs from HEAD in the work
// tree or the index: the same two questions the capsule preflight asks.
func workTreeDirty(ctx context.Context, repo string) (bool, error) {
	if _, ok := resolveCommit(ctx, repo, "HEAD"); !ok {
		return false, nil
	}
	for _, args := range [][]string{{"diff", "--quiet", "--"}, {"diff", "--cached", "--quiet", "--"}} {
		if _, err := git(ctx, repo, nil, nil, args...); err != nil {
			if exitCode1(err) {
				return true, nil
			}
			return false, err
		}
	}
	return false, nil
}
