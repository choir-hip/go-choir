# Parallax Mission: Orchestrator — Mission Suite 2026-06-28

**Date:** 2026-06-28
**Status:** active paradoc
**Source program:** `docs/mission-suite-2026-06-28.md`
**Ledger:** `docs/mission-orchestrator-suite-2026-06-28.ledger.md`

## Mission Conjecture

If the orchestrator delegates all 24 missions to subagents — each in its
own worktree, each with a conjecture to decide and a parallax doc, each
working toward the reward of being mainlined — then the audited computer
vision is materially advanced because the mission suite IS the artifact
program building itself: each mission is a typed transaction with an
author, a conjecture, evidence, and a verdict, and the set of mainlined
missions becomes the tape.

The load-bearing bridge: **delegating conjectures (not tasks) to
subagents in isolated worktrees, with mainlining as the reward condition,
produces evidence that advances the cognitive state of the system.** The
reward condition aligns the subagent's gradient with the orchestrator's:
quality work lands on main; uncertain work stays on a branch.

## Deeper Goal (G)

The audited computer: `computer = choir_code(artifact_program)`, where
the tape is the program, the program is self-authoring, and every state
change is a typed transaction with provenance.

The mission suite IS the first self-authoring cycle: the orchestrator
delegates conjectures, subagents produce evidence, the orchestrator
verifies and mainlines, and the mainlined work becomes the substrate for
the next cycle. This is the tape writing itself through the missions.

## Operating Model

Each mission gets:
1. **Its own worktree** — isolated branch, no contention with other missions
2. **A parallax doc** — the conjecture, spec, invariants, acceptance
   criteria, authority bounds. Pipelined: written just-in-time before
   delegation, not all upfront.
3. **A subagent** — background, works autonomously in the worktree
4. **A reward condition** — if the work is quality, it gets mainlined
   (merged to main and pushed). If uncertain, it stays on a branch or
   becomes a PR. The subagent's gradient is aligned: produce work good
   enough to mainline.

The orchestrator:
- Pipelines parallax docs as missions come up
- Launches subagents in waves, but reorchestrates in real time — doesn't
  wait for a full wave to complete before launching the next
- Verifies each return: conjecture decided? evidence admissible?
  invariants preserved? quality sufficient for main?
- Mainlines confident work, PRs uncertain work, records blocked work as
  open edges
- Updates the checkpoint report after each mission settles
- Keeps moving when blocked — blocked missions produce open edges, not stops

## Invariants / Qualities / Domain Ramp (I/Q/D)

**Invariants (never optimize across):**
- No silent conflict resolution (Base planner preserves both sides)
- No fake-island domain (must build/test on real codebase, not toy)
- No weakening existing auth security (API keys add a path, don't replace
  WebAuthn)
- No production deploy without staging verification (orange+ mutations)
- Problem Documentation First for any new bug discovered
- Each mission works in its own worktree — no cross-mission file contention

**Qualities:**
- Each delegation prompt states the conjecture, not just the task
- Each verification checks the conjecture verdict, not just test passage
- Each commit references the mission and conjecture
- The ledger records every pass with ΔV against prediction
- Work that lands on main must pass CI

**Domain ramp:**
- Wave 1: independent missions (M1, M2, M11, M12, M13, M14)
- Pipeline: as Wave 1 missions settle, launch dependent and review missions
- Critical path: M8 (runtime refactor) is red — PR only, needs deep review
- Target: M10 (choir-in-choir) activates the force multiplier

## Variant (Conjecture Descent) V

```
V = driving conjectures still undecided across all delegated missions
  + conjectures whose evidence class is below settlement tier
  + conjectures with no strong definitive statement yet recorded
```

**Initial conjectures:**

- C1 (M1): "An API key system with Bearer token auth, scoped access, and
  SHA-256 hashed storage can be added to the existing auth service without
  weakening WebAuthn session security." — undecided
- C2 (M2): "A pure three-tree planner with no I/O can represent real sync
  failure cases (concurrent edits, delete-vs-edit, moves, conflicts) while
  preserving both sides of every conflict." — undecided
- C3 (M11): "The runtime test shards pass with -race enabled, or the race
  detector finds bugs that must be fixed before enabling." — undecided
- C4 (M12): "The flaky Dolt test can be quarantined without losing
  coverage of the behavior it tests." — undecided
- C5 (M13): "Privacy policy and ToS can be drafted that accurately reflect
  Choir's actual data flows without overclaiming or underdisclosing." — undecided
- C6 (M14): "Per-cycle, per-article LLM API cost can be tracked through
  trace events and aggregated without a separate billing system." — undecided
- C7 (M15): "PR #7 (docs checker cleanup) can be reviewed, improved, and
  merged without introducing retired vocabulary or breaking tests." — undecided
- C8 (M18): "The four worktrees from ~2026-06-23 can be triaged with clear
  recommendations." — undecided
- C9 (M19): "The 27 open_handoff missions can be triaged and consolidated." — undecided
- C10 (M20): "Trace events can be persisted to Dolt as the primary
  observability store without SaaS export." — undecided
- C11 (M22): "Health endpoints and circuit breakers can be added without
  disrupting existing service behavior." — undecided
- C12 (M21): "PII can be redacted from trace events at ingestion by a local
  SLM actor before persistence." — undecided

**V = 12** (all undecided)

Each mission that settles with a typed verdict reduces V by 1. Discovery
of a new conjecture increases V but advances the cognitive state.

## Budget

**Granted:** open-ended (12-24+ hours)
**Spent:** 0
**Remaining:** full
**Solvency:** not estimated — the orchestrator pipelines work and keeps
moving. Blocked missions produce open edges. The session runs as long as
it's producing conjecture descent.

## Authority / Bounds

**Orchestrator authority:**
- Delegate missions to background subagents in worktrees
- Verify subagent returns
- Merge worktree branches to main and push (for confident work)
- Create PRs (for uncertain work or work needing review)
- Run local tests and builds
- Update mission suite, paradoc state, checkpoint report
- Generate checkpoint reports (MD to docs/, PDF to iCloud)
- Reorchestrate in real time — launch new missions as prerequisites settle

**Orchestrator does NOT:**
- Deploy to staging (requires user trigger)
- Force-push or rewrite history
- Merge PRs without deep review — PR #7 and any PRs need review
- Rubber-stamp subagent work — verify before mainlining

**Subagent authority:**
- Create and modify files within the mission's worktree
- Run tests and builds locally
- Create new packages and test files
- Return evidence and conjecture verdicts

**Subagents do NOT:**
- Merge to main or push to main (orchestrator does this after verification)
- Deploy to staging or production
- Modify files outside their worktree
- Touch protected surfaces beyond what the mission spec allows

## Mutation Class / Protected Surfaces

- M1 (API auth): **orange** — new DB tables, endpoints, proxy validation
- M2 (Base kernel): **yellow** — new packages, tests only
- M11 (race detector): **yellow** — CI config change
- M12 (flaky test): **yellow** — test infrastructure
- M13 (privacy policy): **green** — docs only
- M14 (LLM cost tracking): **orange** — trace event schema, provider calls
- M15 (PR7 review): **green/yellow** — docs + test fixes
- M18 (worktree triage): **green** — evaluation only
- M19 (mission graph triage): **green** — docs work
- M20 (trace observability): **orange** — observability infrastructure
- M22 (health checks): **orange** — reliability infrastructure
- M21 (PII retraction): **orange** — privacy infrastructure

## Worktree Allocation

```
/Users/wiz/.windsurf/worktrees/go-choir/
  m1-api-auth        branch: orchestrator/m1-api-auth
  m2-base-kernel     branch: orchestrator/m2-base-kernel
  m11-race-detector  branch: orchestrator/m11-race-detector
  m12-flaky-test     branch: orchestrator/m12-flaky-test
  m13-privacy-policy branch: orchestrator/m13-privacy-policy
  m14-llm-cost       branch: orchestrator/m14-llm-cost
```

Additional worktrees created as needed for Wave 2+ missions.

## Position / Live Conjectures / Open Edges

**Position:** All worktrees created for Wave 1. Parallax docs being
pipelined. No subagents launched yet — user will trigger.

**Live conjectures:** C1-C12 (all undecided, see Variant section).

**Open edges:**
- **Independence edge:** M1 and M2 are independent but both conceptually
  touch auth (M1 modifies proxy auth, M2's future API will use M1's auth).
  No conflict tonight — separate worktrees.
- **Resource edge:** parallel subagents may contend on nix store / go
  build cache. Separate worktrees prevent file conflicts.
- **Frame-lock edge:** the orchestrator must not confuse "subagent
  returned code" with "conjecture decided." Verification checks the
  conjecture verdict, not just test passage.

## Next Move

**Pass 1: Pipeline parallax docs for Wave 1 missions, then launch.**

Wave 1 worktrees are ready. Write parallax docs for each mission
(pipelined — not all upfront), then launch subagents.

The user triggers the launch. The orchestrator stands ready to:
- Verify returns as they come in
- Mainline confident work
- PR uncertain work
- Launch Wave 2 as Wave 1 settles
- Update checkpoint report after each settlement
- Reorchestrate in real time

## Ledger File

`docs/mission-orchestrator-suite-2026-06-28.ledger.md`

## Version / Lineage

- v1: initial paradoc, 4 missions
- v2: expanded to 10 missions, 3 waves
- v3: expanded to all 24 missions, worktree-per-mission, reward condition,
  pipelined docs, real-time reorchestration, no estimates
- Source: `docs/mission-suite-2026-06-28.md`
- Design docs: `docs/memo-headless-auth-choir-base-artifact-program-2026-06-28.md`,
  `docs/memo-artifact-program-doctrine-2026-06-28.md`

## Learning State

- Cognitive transforms applied: Depth Extraction (delegate conjectures not
  tasks), Principal-agent (reward condition aligns gradients), Observer
  hierarchy (verify at orchestrator level), Feedback loop (pipeline don't
  batch), Fixed point (orchestrator improves its own infrastructure).
- Key learning: the reward condition (mainlining) is the gradient
  alignment mechanism. The subagent's incentive is to produce work good
  enough to mainline. The orchestrator's incentive is to only mainline
  quality work. This creates a cooperative game with aligned gradients.
- The worktree-per-mission model eliminates file contention and allows
  real-time reorchestration without coordination overhead.

## Settlement

The mission settles when:
- All delegated missions have returned with a typed conjecture verdict
- Each verdict has admissible evidence
- Confident work is mainlined; uncertain work is PR'd; blocked work is
  recorded as open edges
- CI passes on mainlined work (or failures are diagnosed)
- The paradoc state is updated with final V and conjecture verdicts
- The ledger records every pass
- The checkpoint report is final (MD + PDF to iCloud)

## Checkpoint Report

After each mission settles, update
`docs/orchestrator-checkpoint-report-2026-06-28.md` and generate a PDF
copy to `~/Library/Mobile Documents/com~apple~CloudDocs/Choir Reports/orchestrator-checkpoint-report-2026-06-28.pdf`.

Report format:
- Current V and conjecture verdicts
- Missions delegated, returned, verified, mainlined, PR'd, blocked
- Strong definitive statements produced
- Heresy delta
- Next missions to launch

## Suggested Goal String

```text
/goal Run docs/mission-orchestrator-suite-2026-06-28.md as the orchestrator
of the mission suite. Each mission gets its own worktree at
/Users/wiz/.windsurf/worktrees/go-choir/<mission-name>. Write parallax docs
pipelined (just-in-time before delegation). Launch subagents as background
agents. Verify each return: conjecture decided? evidence admissible?
invariants preserved? quality sufficient for main? Mainline confident work
(merge to main + push). PR uncertain work. Record blocked work as open
edges and move to the next mission. Reorchestrate in real time — launch
new missions as prerequisites settle. Update checkpoint report after each
mission settles (MD to docs/, PDF to iCloud). Variant V=12. Budget:
open-ended (12-24+ hours). Settlement: all conjectures decided, verified
work mainlined or PR'd, checkpoint report final. Ledger:
docs/mission-orchestrator-suite-2026-06-28.ledger.md.
```
