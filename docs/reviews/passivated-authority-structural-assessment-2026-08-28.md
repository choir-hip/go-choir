# Passivated Run Authority — Structural Assessment and Escalation, 2026-08-28

- Date: 2026-08-28
- Mutation class: assessment (red context)
- Trigger: AGENTS.md escalation rule — 3+ iterations on the same subsystem without convergence; doctrine-level decision required
- Live incident: staging guest `computer-03335285269bdba4f94377e56879f9e6` crash-looping since the 19:20Z generation on commit `ca01210f` (boot completes, `runtime: started`, then `runtime startup refused: actorruntime: reconcile Texture owner: ambiguous boot Texture recovery run authority`)
- Related receipts: `docs/problems/texture-boot-recovery-passivated-ambiguity-2026-08-28.md`, `docs/problems/selfdev-wake-passivated-super-silent-noop-2026-08-28.md`

## 1. The Problem Class

The 08-28 boot-outage recovery (systemd-restarted generations + boot
passivation) minted a population of **passivated runs that still carry live
authority bindings** (agent mutations, operation bindings, canonical revision
heads). The platform's recovery paths disagree about what those runs mean:

| site | file:line | current semantics | enforced by test |
|---|---|---|---|
| Boot Texture recovery enumeration | `textureowner/texture_controller.go:130` | fail closed if >1 non-terminal candidate | (indirect) |
| Passivated Texture occurrence eligibility | `texture_controller.go:356` | fail closed if >1 eligible passivated candidate | yes (`TestPassivatedTextureWakeFailsClosedOnAmbiguousCanonicalRuns` asserts refusal at wake) |
| Passivated candidate selection | `texture_controller.go:767` | fail closed on second match | yes |
| Self-development Super wake | `agentcore/api_self_development.go` `selfDevelopmentSuperRunTerminal` | passivated = NOT terminal → silent no-op wake | (fixed today in `ca01210f`; landed) |
| Boot passivation itself | `agentcore/runtime.go` | mass-passivates pending/running at every boot | yes |

Both post-outage deaths are this one disagreement:
1. Self-dev wake no-oped silently on a passivated bound Super (fixed:
   `ca01210f` treats passivated as terminal **for that predicate only**).
2. Texture boot reconcile refuses startup on passivated duplicates (unfixed:
   two patch attempts reverted — the first broke the intended
   "pre-repair passivated run" resume recovery
   (`TestAdapterSQLiteBootRecoveryUsesJoinedOccurrenceNotDuplicateInitialDispatch`),
   the second moved the refusal deeper to the `:356` guard, proven by a
   failing boot-path test reproducing the live shape).

## 2. Why This Needs a Doctrine Ruling

`types.RunState` docs say passivated runs are "non-terminal but no longer own
live actor slots" (`Active()` excludes them). The disagreement is about
**authority**, not liveness:

- **Fail-closed reading** (current `:356`/`:767` + tests): multiple
  passivated runs claiming the same canonical revision head is corruption;
  never guess. Cost: any mass-passivation event permanently bricks boot and
  the self-dev retry surface — exactly what happened.
- **Deterministic-disambiguation reading**: passivated duplicates are
  crash-loop residue; among candidates, the newest wins (fresher authority),
  exact ties still refuse, live (active) ambiguity still refuses. Cost: the
  platform activates a run that an older failsafe would have refused; the
  failsafe tests must be re-pinned to the new contract.

There is also a **third option**: a one-time residue-repair migration
(product-path) that deterministically re-binds or unbinds duplicate passivated
authority rows, leaving both guards' fail-closed semantics intact. Higher
ceremony (data migration on the canonical store), but zero doctrine change.

## 3. Dependency Graph

- `RunState.Terminal()` deliberately excludes `passivated` (do not change
  globally — every recovery site keys off it).
- Boot passivation is here to stay (crash-loop hygiene).
- The four guard sites are independent; fixing only `:130` moves the refusal
  to `:356` (verified: a boot-path test that fixes `:130` alone still fails
  with `ambiguous passivated Texture run authority for document ...`).
- The self-dev fix (`ca01210f`) is complete for its site: the wake now
  unbinds passivated bound Supers (test
  `TestSelfDevelopmentStartRevivesPassivatedPersistentSuper`).

## 4. Recommendation (for owner decision)

**Option B (deterministic disambiguation, recommended)**: at all Texture guard
sites (`:130`, `:356`, `:767`), when candidates differ only by being passivated
residue, select the newest `UpdatedAt` candidate; refuse only on
live-active ambiguity or exact `UpdatedAt` ties (different run IDs). Re-pin the
two fail-closed tests to assert: (a) live ambiguity still refuses, (b)
passivated duplicates with distinct timestamps resolve to the newest, (c)
exact-tie passivated duplicates still refuse. One commit, rollback = revert.
Restores boot liveness without touching `Terminal()` or adding migrations.

Option A (keep fail-closed): requires the Option C migration to restore the
computer; otherwise staging stays down.

Option C (residue migration): product-path repair command that deterministically
unbinds/re-binds duplicate passivated authority rows; both guards stay
untouched. Safest semantics, most ceremony, and the residue will re-accumulate
after the next mass-passivation event unless the guards also change — so C
without B is not durable.

## 5. Current State (for the record)

- Staging guest: crash-looping on `ca01210f` (~30s generations, deterministic
  refusal; observed 19:20-19:25Z, Node B journal). Owner-visible: GUI/API
  work only within each ~25s live window.
- `main` @ `93771bcb`: green (texture partial fix reverted; self-dev fix and
  worker-update substrate repair remain landed and deployed).
- Deployed staging/proxy: `ca01210f` (contains the worker-update substrate
  repair + self-dev wake fix; NOT the reverted texture change).
- The self-dev operations retry wake is now functional and will fire once boot
  survives Texture reconcile.
