// Package sourceproj keeps the guest source repository a projection of what
// the computer has applied: the base commit (the booted base), plus the
// effective release's patch as one deterministic commit
// (docs/design/computer-update-architecture-rnd-2026-10-24.md, section 4.6,
// "U1 build notes"). The capsule seeds from that repository's clean HEAD, so
// a candidate is built on the applied source, not on the creation commit.
//
// Nothing here trusts a prior run. Reconcile recomputes the expected HEAD from
// its input every time; the refs and the facts it leaves are observation. It
// runs on every computer (capsule overlay S4,
// docs/design/capsule-overlay-computer-state-2026-11-03.md section 5): there
// is no switch, and it never resets work it has not first saved.
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
	// RefSavedPrefix + a commit id holds what a reset would have lost: a
	// commit of the dirty work tree, or a HEAD the projection did not leave.
	// The name is the commit, and the ref is created, never moved.
	RefSavedPrefix = "refs/choir/saved/"

	// TrailerStackBase names the commit a projection commit's patch was
	// applied to. Freeze reads it from the capsule's pinned commit, so the
	// composition never depends on mutable refs.
	TrailerStackBase = "Choir-Stack-Base"
)

// State is what reconcile found.
type State string

const (
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
	// not upstreamed, and the booted base changed a path the stack's patch
	// touches. The projection is kept; casting refuses until a rebase.
	StateBaseMoved State = "stack_base_moved"
	// StateRematerialized: a live stack on a base that is not the booted one,
	// where the booted base left every path the patch touches unchanged. HEAD
	// is the booted base with the patch's after-image at those paths, byte for
	// byte; casting is admitted (the served build is still the stack on its
	// own base, which the Cast's ledger warning says). With the package lock
	// (S6) a package root the booted base also changed is no bar: its entry
	// (Result.Packages) is retired, grafted, or holds this computer's version
	// whole; only a shell path the booted base changed is stack_base_moved.
	StateRematerialized State = "stack_rematerialized"
	// StateBaseObjectsMissing: the booted base's objects are not in the
	// guest repository (residual R3), so it can neither be moved to nor shown
	// to contain the stack. The cast refuses (U1b); U1b's fetch supplies them.
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
	// FactSaved: reconcile kept what a reset would have lost at a saved ref.
	FactSaved = "source.projection_saved"
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
	// OnSave records what reconcile saved before it resets anything. It is
	// called with every save (created now or found from an earlier attempt),
	// so it must be idempotent on the saved commit. The reset runs only when
	// it returns nil; an error leaves HEAD and the work tree as they were.
	OnSave func(ctx context.Context, saved []Saved) error
}

// Saved kinds.
const (
	// SavedDirty is a commit of the work tree's tracked changes (and of the
	// untracked files a reset would overwrite), its parent the HEAD it was on.
	SavedDirty = "dirty"
	// SavedHead is a HEAD no refs/choir/ ref names (the creation commit, or a
	// commit something else made on the computer branch).
	SavedHead = "head"
)

// Saved is one ref reconcile kept before a reset.
type Saved struct {
	Ref    string `json:"ref"`
	Commit string `json:"commit"`
	Kind   string `json:"kind"`
	// From is the HEAD the reset moves away from.
	From string `json:"from,omitempty"`
	// To is the commit the reset moves to.
	To string `json:"to,omitempty"`
	// Created: this reconcile made the ref (an earlier attempt may have).
	Created bool `json:"created,omitempty"`
}

// Result is what Reconcile did and found.
type Result struct {
	State  State  `json:"state"`
	Reason string `json:"reason,omitempty"`
	// Head is HEAD after reconcile ("" when untouched or unavailable).
	Head string `json:"head,omitempty"`
	// Base is the stack base (the booted base for an empty stack, and for a
	// rematerialized stack).
	Base string `json:"base,omitempty"`
	// ServedBase is the base the served stack's patch is on, set when it is
	// not Base (stack_rematerialized): the served build is still that.
	ServedBase string `json:"served_base,omitempty"`
	// BootedBase is the booted image's build commit as given.
	BootedBase string `json:"booted_base,omitempty"`
	// BootedMissing: the booted base was named but its objects were not in
	// the repository when this reconcile resolved it. A caller deciding
	// whether a refusal passes by waiting for the fetch reads this snapshot
	// for what the result means (problem 30, review r1); a later look at the
	// repository may only turn "missing" into "wait and reconcile again".
	BootedMissing bool `json:"booted_missing,omitempty"`
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
	LocksCleared []string `json:"locks_cleared,omitempty"`
	// From is the HEAD a changed result moved away from: the rollback handle
	// when the projection first leaves the creation commit.
	From string `json:"from,omitempty"`
	// Saved lists what was kept at refs/choir/saved/ before the reset.
	Saved []Saved `json:"saved,omitempty"`
	// Packages are the package roots the stack touches, each with its answer
	// (S6, docs/design/s6-package-lock-build-2026-11-04.md).
	Packages []PackageEntry `json:"packages,omitempty"`
	At       time.Time      `json:"at"`
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
	case StateBaseObjectsMissing:
		// Projection means "build on current main" (FP0, 2026-10-24): with the
		// booted base's objects still missing after the fetch, building on the
		// creation commit is the failure U1 exists to end.
		return "base_objects_missing: " + r.Reason
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
	reconcileMu.Lock()
	defer reconcileMu.Unlock()
	before := ""
	if strings.TrimSpace(in.Repo) != "" {
		before, _ = resolveCommit(ctx, in.Repo, "HEAD")
	}
	out, err := reconcileLocked(ctx, in, res)
	if err == nil && out.Changed && before != out.Head {
		out.From = before
	}
	return out, err
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
		res.BootedMissing = !haveBooted
	}
	_, hadBase := resolveCommit(ctx, repo, RefBase)
	cur := current{head: head, haveHead: haveHead, hadBase: hadBase, onSave: in.OnSave}

	if in.Top == nil || len(in.Top.Patch) == 0 {
		return reconcileEmptyStack(ctx, repo, res, cur, booted, haveBooted)
	}
	return reconcileStack(ctx, repo, in.Top, res, cur, booted, haveBooted)
}

// current is what the repository held when reconcile started, and the hook
// that records a save.
type current struct {
	head              string
	haveHead, hadBase bool
	onSave            func(context.Context, []Saved) error
}

// target is what ensureHead makes the repository: HEAD expected, the base ref
// base, and the applied ref (when appliedDigest names one) appliedCommit.
type target struct {
	expected, base               string
	appliedDigest, appliedCommit string
}

func (res *Result) settle(ctx context.Context, repo string, cur current, t target) error {
	moved, err := ensureHead(ctx, repo, cur, t)
	if err != nil {
		return err
	}
	res.Head, res.Changed, res.Repaired, res.Saved = t.expected, moved.changed, moved.repaired, moved.saved
	return nil
}

func reconcileEmptyStack(ctx context.Context, repo string, res Result, cur current, booted string, haveBooted bool) (Result, error) {
	switch {
	case haveBooted:
		res.State, res.Base = StateBase, booted
	case res.BootedBase == "":
		res.State, res.Reason, res.Head = StateNoBootedBase, "the booted image names no build commit", cur.head
		return res, nil
	default:
		// The booted base's objects are not here (R3). Keep the source as it
		// is, except to drop a retired stack's projection: an empty stack
		// means HEAD is the recorded base.
		res.State, res.Reason = StateBaseObjectsMissing, "the booted base "+res.BootedBase+" is not in the guest source repository"
		recorded, ok := resolveCommit(ctx, repo, RefBase)
		if !ok {
			res.Head = cur.head
			return res, nil
		}
		res.Base = recorded
		booted = recorded
	}
	err := res.settle(ctx, repo, cur, target{expected: booted, base: booted})
	return res, err
}

func reconcileStack(ctx context.Context, repo string, top *Op, res Result, cur current, booted string, haveBooted bool) (Result, error) {
	res.OperationID = top.OperationID
	base, ok := resolveCommit(ctx, repo, top.Base)
	if !ok {
		res.State, res.Head = StateUnrebuildable, cur.head
		res.Reason = "the patch base commit " + top.Base + " is not in the guest source repository"
		return res, nil
	}
	tree, err := patchedTree(ctx, repo, base, top.Patch, false)
	if err != nil {
		res.State, res.Head = StateUnrebuildable, cur.head
		res.Reason = "the operation's patch does not apply to its base " + base + ": " + err.Error()
		return res, nil
	}
	upstreamed := func(reason string) (Result, error) {
		res.State, res.Base, res.Diverged, res.Reason = StateUpstreamed, booted, false, reason
		err := res.settle(ctx, repo, cur, target{expected: booted, base: booted})
		return res, err
	}
	if haveBooted {
		contained, cerr := Contained(ctx, repo, booted, base, top.Patch, tree)
		if cerr != nil {
			return res, cerr
		}
		if contained {
			return upstreamed("the operation's patch is already in the booted base " + booted)
		}
	}
	commit, err := projectionCommit(ctx, repo, top, base, tree)
	if err != nil {
		return res, err
	}
	if haveBooted && base != booted {
		// S4: a moved base the patch's paths did not see is grafted, byte for
		// byte (library design 10.3, applied to the whole stack).
		// S6: package roots are units; shell paths keep S4's path graft.
		graft, packages, ok, why, gerr := packageGraft(ctx, repo, base, booted, tree)
		res.Packages = packages
		if gerr != nil {
			return res, gerr
		}
		if ok {
			bootedTree, terr := git(ctx, repo, nil, nil, "rev-parse", booted+"^{tree}")
			if terr != nil {
				return res, terr
			}
			if graft == bootedTree {
				return upstreamed("the operation's patch, grafted onto the booted base " + booted + ", leaves it unchanged")
			}
			grafted, cerr := projectionCommit(ctx, repo, top, booted, graft)
			if cerr != nil {
				return res, cerr
			}
			res.State, res.Base, res.ServedBase, res.Diverged = StateRematerialized, booted, base, true
			res.Reason = "the stack's patch on " + base + " is grafted byte-identically onto the booted base " + booted + ": the booted base changed no path the patch touches; the served build is still the stack on " + base
			if conflicts := packageConflicts(res.Packages); len(conflicts) > 0 {
				res.Reason = "the stack's patch on " + base + " is re-materialized onto the booted base " + booted + ": the booted base changed no shell path the patch touches; it also changed the package roots " + strings.Join(conflicts, ", ") +
					" (package_behind_base), where this computer's version is kept whole; the served build is still the stack on " + base
			}
			err := res.settle(ctx, repo, cur, target{expected: grafted, base: booted, appliedDigest: top.AppliedEventDigest, appliedCommit: commit})
			return res, err
		}
		res.State, res.Base, res.Diverged = StateBaseMoved, base, true
		res.Reason = "the stack is based on " + base + " but this computer booted " + booted + " and the stack's patch is not in it; " + why
		err := res.settle(ctx, repo, cur, target{expected: commit, base: base, appliedDigest: top.AppliedEventDigest, appliedCommit: commit})
		return res, err
	}
	res.Base, res.Diverged = base, true
	if haveBooted {
		entries, perr := packageEntries(ctx, repo, base, tree)
		if perr != nil {
			return res, perr
		}
		res.Packages = entries
	}
	switch {
	case haveBooted:
		res.State = StateProjected
	case res.BootedBase == "":
		res.State = StateProjected
		res.Reason = "the booted base is unknown"
	default:
		res.State = StateBaseMoved
		res.Reason = "the stack is based on " + base + " but this computer booted " + res.BootedBase + "; that commit is not in the guest source repository, so the stack cannot be shown to be upstreamed"
	}
	err = res.settle(ctx, repo, cur, target{expected: commit, base: base, appliedDigest: top.AppliedEventDigest, appliedCommit: commit})
	return res, err
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

type headMove struct {
	changed, repaired bool
	saved             []Saved
}

// ensureHead makes t.expected the attached HEAD with clean tracked files, and
// sets the base ref (and the applied ref when t.appliedDigest is set). It
// reports whether it moved anything, and whether what it moved was something
// the projection did not leave (damage). Before any reset it saves what the
// reset would lose (S4): the dirty work tree as a commit, and a HEAD no
// refs/choir/ ref names, each at refs/choir/saved/<commit>, and hands them to
// cur.onSave; the reset runs only once that has returned nil.
func ensureHead(ctx context.Context, repo string, cur current, t target) (headMove, error) {
	dirty, err := workTreeDirty(ctx, repo)
	if err != nil {
		return headMove{}, err
	}
	moved := !cur.haveHead || cur.head != t.expected
	// Read what the projection left before any ref moves: a HEAD that is none
	// of it was put there by something else (damage once the projection has
	// recorded a base; the creation commit before that).
	foreign := false
	if moved && cur.haveHead {
		left, err := projectionLeft(ctx, repo, cur.head)
		if err != nil {
			return headMove{}, err
		}
		foreign = !left
	}
	if !moved && !dirty {
		if err := setProjectionRefs(ctx, repo, t); err != nil {
			return headMove{}, err
		}
		return headMove{}, nil
	}
	var saved []Saved
	{
		from := ""
		if cur.haveHead {
			from = cur.head
		}
		commit, err := saveWorkTree(ctx, repo, from, t.expected)
		if err != nil {
			return headMove{}, fmt.Errorf("save the work tree before the reset: %w", err)
		}
		if commit != "" {
			created, err := createSavedRef(ctx, repo, commit)
			if err != nil {
				return headMove{}, err
			}
			saved = append(saved, Saved{Ref: RefSavedPrefix + commit, Commit: commit, Kind: SavedDirty, From: from, To: t.expected, Created: created})
		}
		if foreign {
			created, err := createSavedRef(ctx, repo, cur.head)
			if err != nil {
				return headMove{}, err
			}
			saved = append(saved, Saved{Ref: RefSavedPrefix + cur.head, Commit: cur.head, Kind: SavedHead, From: cur.head, To: t.expected, Created: created})
		}
	}
	if len(saved) > 0 && cur.onSave != nil {
		if err := cur.onSave(ctx, saved); err != nil {
			return headMove{}, fmt.Errorf("record the saved source before the reset (nothing was reset): %w", err)
		}
	}
	if err := setProjectionRefs(ctx, repo, t); err != nil {
		return headMove{}, err
	}
	if _, err := git(ctx, repo, nil, nil, "update-ref", Branch, t.expected); err != nil {
		return headMove{}, err
	}
	if _, err := git(ctx, repo, nil, nil, "symbolic-ref", "HEAD", Branch); err != nil {
		return headMove{}, err
	}
	if _, err := git(ctx, repo, nil, nil, "reset", "--hard", "--quiet", t.expected); err != nil {
		return headMove{}, err
	}
	return headMove{changed: true, repaired: dirty || (foreign && cur.hadBase), saved: saved}, nil
}

// setProjectionRefs points the applied, projected and base refs at the
// target. None of them is a reset: they lose nothing.
func setProjectionRefs(ctx context.Context, repo string, t target) error {
	if t.appliedDigest != "" && commitID.MatchString(t.appliedDigest) {
		applied := t.appliedCommit
		if applied == "" {
			applied = t.expected
		}
		if cur, ok := resolveCommit(ctx, repo, RefAppliedPrefix+t.appliedDigest); !ok || cur != applied {
			if _, err := git(ctx, repo, nil, nil, "update-ref", RefAppliedPrefix+t.appliedDigest, applied); err != nil {
				return err
			}
		}
	}
	// The last commit the projection set, whether or not an applied event
	// named it: a HEAD that is this was left by us, not damage.
	if cur, ok := resolveCommit(ctx, repo, RefProjected); !ok || cur != t.expected {
		if _, err := git(ctx, repo, nil, nil, "update-ref", RefProjected, t.expected); err != nil {
			return err
		}
	}
	if cur, ok := resolveCommit(ctx, repo, RefBase); !ok || cur != t.base {
		if _, err := git(ctx, repo, nil, nil, "update-ref", RefBase, t.base); err != nil {
			return err
		}
	}
	return nil
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
