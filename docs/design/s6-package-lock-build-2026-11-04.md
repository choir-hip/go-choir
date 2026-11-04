# S6 build: the package lock and package-aware containment (2026-11-04)

Status: build plan, red (the Cast gate's source and the projection).
Builds "### S6" and failure L17 of
[the Library design](library-package-propagation-2026-10-26.md) (10.3, 15.3),
reconciled with what landed since. Evidence that motivates it:
[noop-rebase-never-clears-base-moved](../problems/noop-rebase-never-clears-base-moved-2026-11-04.md)
(8d78b7e78 exported `library-next`; three test computers with their own
`library-next` went `stack_base_moved`, every ordinary Cast was refused for
about 90 minutes, and an install was refused). Owner, 2026-11-04: apps are a
layer, not part of the platform stack.

## What the design assumed that does not exist

- **S5 trailers.** No `package_adopt` job exists, so no projection commit
  carries `Choir-Package-Root` / `Choir-Package-Release`. Those trailers are
  written only by `choir package export` on main's export commits.
- **P8 adoption** is deleted (b5fa1aa82). Package install
  (`package_install.go`) is an ordinary self-development operation whose
  cumulative source patch is confined to `frontend/src/packages/<name>/`.
- **A lock record.** None exists, and none is needed: under U1 the applied
  stack is one cumulative patch on a stack base, so every fact an entry
  needs is a tree id the repository already holds.

## Entries: what they are derived from

An entry is one package root `frontend/src/packages/<name>` that the applied
stack's cumulative patch touches (the first path segment under
`frontend/src/packages/` that is a directory; a file directly there is a
shell path). Local packages (today's case: an engineer's own `library-next`)
and installed ones are the same kind of entry. Per entry, three subtree ids
(absent = empty):

- **P**: the stack base's subtree at the root;
- **R**: the stack's subtree (the patch applied to the stack base);
- **B'**: the booted base's subtree.

Provenance is optional enrichment, not a state input: `release_id` is set
when R equals the `release_tree_hash` of a package install record
(`store.ReleaseAdoption`, kind `package`, same package). No new record.

## States (per entry; `Result.Packages`)

| State | When | Projection at the root |
|---|---|---|
| `adopted` | booted = stack base | R (the ordinary projection) |
| `upstreamed` | B' = R | B' (nothing placed; the entry is retired) |
| `package_base_moved` | B' = P, booted moved | R, byte-identical |
| `package_behind_base` | B' is neither P nor R | the stack's paths grafted onto B' when the base changed none of them (S4 rule, `conflict: false`); otherwise R placed whole (`conflict: true`) |

Shell paths (everything outside a package root) keep S4 exactly: if the
booted base changed one the patch touches, the whole stack is
`stack_base_moved` and U4's rebase runs as today (entries still reported).

The overall state: the whole-stack containment test runs first, as today.
Then, when the shell grafts: the result tree equal to booted is `upstreamed`;
otherwise `stack_rematerialized`, now also when a package root moved under
the stack. The served build does not change (owner Q1: no clean rebase
without approval); approving the next candidate cast on this projection
approves the re-materialization, as S4 already does.

## The Cast gate

No new admission branch. The gate admits `stack_rematerialized` with a ledger
warning; the warning now names every `package_behind_base` entry and says
that approving the candidate keeps this computer's version of that package
over the base's. Today's case (B' at `library-next` is a different app, the
shell untouched) is `package_behind_base` with `conflict: true`, the state
is `stack_rematerialized`, and an ordinary Cast on another path is admitted.
The rebase notice is armed only at `stack_base_moved`, unchanged. A pair
claimed before this deploy is inert: a rebase Cast on it answers
`rebase_not_needed`.

To take the platform's version instead, an engineer restores the root to the
base's in a candidate (an ordinary change), or the owner uninstalls.

## Retirement

Per entry, by subtree equality B' = R: the root is left as booted has it, so
the next candidate's cumulative patch (composed from the booted base) no
longer carries it. Another unpromoted entry stays placed. When every entry
and the shell are in booted, the stack is `upstreamed` (today's rule).

## Rollout and rollback

No switch (owner, 2026-11-03: rollout goes by ring, rollback is a git
revert). The package-aware graft replaces S4's whole-stack graft on every
computer; reverting the commit restores it. What the off path stood in for
stays on its own: a shell path the booted base also changed is still
`stack_base_moved`, and U4's rebase still answers it.

## Restarts

Reconcile recomputes from the tape at boot, apply and Cast; it never
resumes work. A crash between the apply and the projection is reconciled by
the next pass from the same inputs (saved refs first, as S4).

## Failure modes, written before code

| # | Failure | Guard | Test |
|---|---|---|---|
| K1 | A shell conflict hides behind the package logic | shell paths keep S4; any conflict is `stack_base_moved` | mixed stack with a shell conflict |
| K2 | A root is misidentified (a file at `packages/x`, a nested `packages/`) | root = `frontend/src/packages/<seg>/` with more path after it | unit on the splitter |
| K3 | R placed whole silently drops the base's change in that root | `conflict: true` on /health and in the Cast warning; the served build is untouched; approval decides | today's case |
| K4 | Retiring one entry drops another unpromoted one | retirement is per root by subtree id | two-entry test |
| K5 | The placed subtree is not R | the root's tree id is compared after the write; a mismatch is not graftable (`stack_base_moved`) | byte-identity test |
| K6 | The re-materialized commit is not reproducible | the commit is a function of the operation and the tree | rebuild gives the same id |
| K7 | A crash between apply and projection leaves a stale HEAD or dirty tree | reconcile from inputs; S4 saves first | stale HEAD + dirty file |
| K9 | The stack deletes a package, or the base deletes it | an absent subtree is the empty tree; R absent placed means removed | delete case |
| K10 | A directory/file clash at the root | the index write fails: not graftable, `stack_base_moved` | covered by S4's clash test |
| K11 | The placed package fails to build on a base whose SDK moved (L18) | the capsule build fails; nothing is applied | residual (S12 removes it) |
| K12 | A publisher draft of an operation cast on a package-placed projection | the draft's own graft refuses rather than guesses | residual |

## Not in this slice

- `vmctl list` column (reads `effective_release.packages` later);
- a deterministic re-materialization candidate (needs the `package_adopt`
  job, Library 6.4);
- `reverted` / `set_aside` lock transitions (S7, S11).
