# Parallax Mission: Orchestrator — Mission Suite 2026-06-28

**Date:** 2026-06-28
**Status:** active paradoc
**Source program:** `docs/mission-suite-2026-06-28.md`
**Ledger:** `docs/mission-orchestrator-suite-2026-06-28.ledger.md`

## Mission Conjecture

If the orchestrator delegates missions to subagents as **conjectures to
decide** (not tasks to complete) with evidence-bounded acceptance criteria,
under the invariants of no silent conflict resolution and no fake-island
domain, over tonight's four independent missions (M1 API auth, M2 Base
kernel, M11 race detector, M12 flaky test quarantine), then the audited
computer vision is materially advanced because each mission decides a
conjecture that unblocks downstream work and produces admissible evidence
the orchestrator can verify.

The load-bearing bridge is: **delegating conjectures (not tasks) to
subagents produces evidence that advances the cognitive state of the
system.** If subagents produce code that passes tests but doesn't decide
the conjecture, the bridge is falsified — motion without descent.

## Deeper Goal (G)

The audited computer: `computer = choir_code(artifact_program)`, where
the tape is the program, the program is self-authoring, and every state
change is a typed transaction with provenance. Tonight's missions are the
first concrete implementation steps:

- M1 (API auth) enables machines to author tape entries — without headless
  auth, only humans in browsers can mutate state, and the self-authoring
  program cannot exist.
- M2 (Base kernel) implements the three-tree reconciliation that IS tape
  consensus for file mutations — without the planner, file mutations are
  opaque accidents, not typed transactions.
- M11 (race detector) defends the execution substrate against the bug class
  that borked the port — without race safety, the tape can be corrupted by
  concurrent access.
- M12 (flaky test quarantine) removes noise from the verification signal —
  without trustworthy CI, conjecture verdicts are unreliable.

The deeper goal is not "four missions completed." The deeper goal is "four
conjectures decided with evidence, unblocking the next layer of the
audited computer."

## Witness / Spec (A/S)

**A:** Four background subagents, each delegated a mission with:
- The conjecture to decide (strong, clear, definitive statement)
- The spec/design doc reference
- The files to create or modify
- The acceptance criterion (what evidence proves the conjecture)
- The authority bounds (what the subagent can and cannot do)

**S:** Each subagent returns:
- The conjecture verdict (supported / weakened / falsified / superseded)
- The evidence (tests, build output, code diffs)
- The strong definitive statement about the system
- Residual risks and open edges

The orchestrator verifies each return at its own observer level:
conjecture decided? evidence admissible? invariants preserved? Then lands
the verified work (commit, push, CI).

## Invariants / Qualities / Domain Ramp (I/Q/D)

**Invariants (never optimize across):**
- No silent conflict resolution (Base planner preserves both sides)
- No fake-island domain (must build/test on real codebase, not toy)
- No weakening existing auth security (API keys add a path, don't replace
  WebAuthn)
- No production deploy without staging verification (orange+ mutations)
- Problem Documentation First for any new bug discovered

**Qualities:**
- Each delegation prompt states the conjecture, not just the task
- Each verification checks the conjecture verdict, not just test passage
- Each commit references the mission and conjecture
- The ledger records every pass with ΔV against prediction

**Domain ramp:**
- Tonight: 4 independent missions, no dependencies between them
- Next: M3 (Base journal) depends on M2, M7 (auth recovery) depends on M1
- Later: M8 (runtime refactor) is serial critical path
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
  Choir's actual data flows (source captures, LLM processing, trace events,
  Dolt storage) without overclaiming or underdisclosing." — undecided
- C6 (M14): "Per-cycle, per-article LLM API cost can be tracked through
  trace events and aggregated without adding a separate billing system." — undecided
- C7 (M15): "PR #7 (docs checker cleanup) can be reviewed, improved if
  needed, and merged without introducing retired vocabulary or breaking
  existing tests." — undecided
- C8 (M18): "The four worktrees from ~2026-06-23 can be triaged
  (merge/hold/discard) with clear recommendations." — undecided
- C9 (M19): "The 27 open_handoff missions in the mission graph can be
  triaged and consolidated." — undecided
- C10 (M20): "Trace events can be persisted to Dolt as the primary
  observability store without SaaS export." — undecided

**V = 10** (all undecided)

Each mission that settles with a typed verdict reduces V by 1. A mission
that discovers a new conjecture increases V but advances the cognitive
state (discovery, not zero progress). Blocked missions don't reduce V
but produce open edges that inform the next cycle.

## Budget

**Granted:** one session (overnight, ~8+ hours of wall-clock)
**Spent:** 0
**Remaining:** full session
**Ambition level:** EVERYTHING. All 24 missions. Partial credit for
partial completion. Blocked missions produce open edges, not stops —
move to the next mission immediately. Confident work goes to main;
uncertain work goes to PRs. The goal is maximum conjecture descent, not
minimum risk.

**Solvency:** Wave 1: 6 parallel independent missions. Wave 2: 3
review/eval missions. Wave 3: M20 + any Wave 1 missions that need
retry. Wave 4: dependent missions (M3, M7, M22) if prerequisites land.
Wave 5: critical path (M8) if confident. Each wave: ~30-90 min subagent
work + ~15-30 min verification. Blocked = move on, not stop.

## Authority / Bounds

**Orchator authority:**
- Delegate missions to background subagents
- Verify subagent returns
- Commit and push verified work
- Create PRs for work that needs review
- Run local tests and builds
- Update mission suite, paradoc state, and checkpoint report
- Generate checkpoint reports (MD to docs/, PDF to iCloud)

**Orchestrator does NOT:**
- Deploy to staging (requires user trigger)
- Modify production routes
- Force-push or rewrite history
- Start missions on the critical path (M8-M10) without user approval
- Approve orange+ mutations for production — only for local verification
- Merge PRs without deep review — PR #7 and any PRs created tonight need
  review, not rubber-stamping

**Subagent authority:**
- Create and modify files within the mission scope
- Run tests and builds locally
- Create new packages and test files
- Return evidence and conjecture verdicts

**Subagents do NOT:**
- Commit or push (orchestrator does this after verification)
- Deploy to staging or production
- Modify files outside their mission scope
- Touch protected surfaces (auth WebAuthn flows, proxy existing routes,
  runtime execution substrate) beyond what the mission spec allows

## Mutation Class / Protected Surfaces

- M1 (API auth): **orange** — new DB tables, new endpoints, proxy
  validation change. Protected: existing WebAuthn flows, existing cookie
  auth, existing proxy routes. Rollback: remove `api_keys` table and
  revert proxy changes.
- M2 (Base kernel): **yellow** — new packages, tests only. No protected
  surfaces. Rollback: remove `internal/base/` directory.
- M11 (race detector): **yellow** — CI config change. Protected: existing
  test shard balance. Rollback: remove `-race` flag.
- M12 (flaky test): **yellow** — test infrastructure. Protected: the
  behavior the flaky test covers. Rollback: un-quarantine the test.

## Evidence Packet

For each mission, the evidence packet contains:
- Mutation class and protected surfaces touched
- Conjecture verdict (supported/weakened/falsified/superseded)
- Tests/probes run and results
- Code diff summary
- Rollback ref or blocker
- Heresy delta (discovered/introduced/repaired)
- Residual risks
- Strong definitive statement about the system

## Heresy Delta

- **Discovered:** any new heresy found during implementation (e.g., "the
  proxy auth validation has an edge case where..." is a discovery)
- **Introduced:** any heresy the implementation creates (e.g., "the API
  key validation doesn't check scope on first pass" is an introduction)
- **Repaired:** any heresy the implementation fixes (e.g., "the flaky
  test was flaky because of X, now fixed" is a repair)

Discovery counts as epistemic progress, not regression. Introduction counts
as regression. Repair counts as progress.

## Position / Live Conjectures / Open Edges

**Position:** All four missions are ready to delegate. Design docs and
specs are complete. The orchestrator is at the starting position — no
missions delegated yet, no evidence collected, V=4.

**Live conjectures:** C1, C2, C3, C4 (all undecided, see Variant section).

**Open edges:**
- **Independence edge:** M1 and M2 are independent but both touch the auth
  proxy conceptually (M1 modifies proxy auth, M2 doesn't touch proxy but
  its future API will use M1's auth). No conflict tonight, but the
  dependency will matter for M4.
- **Resource edge:** running 4 background subagents in parallel may
  contend on the same files (flake.lock, go.mod). M2 and M11/M12 are
  unlikely to conflict. M1 touches auth/proxy, M2 touches internal/base/,
  M11 touches .github/workflows, M12 touches test files. No file
  conflicts expected.
- **Frame-lock edge:** the orchestrator must not confuse "subagent
  returned code" with "conjecture decided." A subagent that writes
  500 lines of passing tests but doesn't state the conjecture verdict
  has not advanced V. The forcing rule applies: if verification produces
  no conjecture verdict, the next move is a shift (re-examine the
  delegation prompt, not re-run the subagent).

## Next Move

**Pass 1: Launch Wave 1 — six parallel background subagents.**

Wave 1 (independent, no dependencies):
- M1 (API auth) — implement API key system
- M2 (Choir Base kernel) — implement pure planner + model + testkit
- M11 (race detector) — add -race to CI runtime shards
- M12 (flaky test) — quarantine flaky Dolt test
- M13 (privacy policy) — draft privacy policy + ToS
- M14 (LLM cost tracking) — implement cost tracking via trace events

Each subagent receives:
1. The conjecture to decide (strong, clear, definitive statement)
2. The spec/design doc reference
3. The files to create or modify
4. The acceptance criterion (what evidence proves the conjecture)
5. The authority bounds

Expected ΔV: -6 (all six conjectures decided)
Risk: some may return falsified or weakened. Both are progress.

After Wave 1 returns, verify each at the orchestrator level, then land
verified work: confident work to main, uncertain work to PRs. Update
checkpoint report. Move immediately to Wave 2.

**Pass 2: Launch Wave 2 — review/eval missions.**

Wave 2 (can overlap with Wave 1 if no file conflicts):
- M15/PR7 (docs cleanup) — deep review of PR #7, improve if needed
- M18 (worktree triage) — evaluate 4 worktrees
- M19 (mission graph triage) — triage 27 open_handoff missions

Expected ΔV: -3

**Pass 3: Launch Wave 3.**

- M20 (trace observability) — persist trace events to Dolt
- M22 (health checks) — health endpoints + circuit breakers
- M21 (PII retraction) — SLM redaction pipeline (may be too complex for
  one pass; partial credit for design + scaffolding)

Expected ΔV: -3

**Pass 4: Dependent missions (if prerequisites landed).**

- M3 (Base journal) — if M2 landed
- M7 (auth recovery) — if M1 landed
- M23 (bounded inbox) — if M8 not needed (may be independent enough)

**Pass 5: Critical path (if confident).**

- M8 (runtime refactor) — only if confident and budget remains. This is
  red mutation class — PR, not main.

**Throughout:** update checkpoint report after each wave, save MD to docs/
and PDF to iCloud. Blocked = move on. Partial credit. Maximum descent.

## Ledger File

`docs/mission-orchestrator-suite-2026-06-28.ledger.md`

## Version / Lineage

- v1: initial paradoc, 2026-06-28
- Source: `docs/mission-suite-2026-06-28.md` (24 missions compiled)
- Design docs: `docs/memo-headless-auth-choir-base-artifact-program-2026-06-28.md`,
  `docs/memo-artifact-program-doctrine-2026-06-28.md`
- Predecessor: `docs/mission-choir-base-reconciliation-kernel-v0.md` (Base
  mission doc, now activated as M2)

## Learning State

- Cognitive transforms applied: Depth Extraction (delegate conjectures not
  tasks), Principal-agent (information asymmetry between orchestrator and
  subagents), Observer hierarchy (verify at orchestrator level, not
  subagent level), Feedback loop (delay between delegation and
  verification is budget risk), Fixed point (orchestrator improves its
  own infrastructure through missions).
- Key learning: the variant is system-wide conjecture descent, not
  per-mission completion. A mission that passes all tests but doesn't
  decide its conjecture has not advanced V.
- Promoted outward: the "delegate conjectures not tasks" principle is
  generalizable to any multi-agent orchestration, not just tonight.

## Settlement

The mission settles when:
- All delegated missions have returned with a typed conjecture
  verdict (supported/weakened/falsified/superseded)
- Each verdict has admissible evidence (tests, build output, code diffs)
- Verified work is committed and pushed, or PRs created for review
- CI passes (or failures are diagnosed and documented)
- The paradoc state is updated with final V and conjecture verdicts
- The ledger records every pass
- The checkpoint report is updated after each wave (MD + PDF)

If budget runs out before all settle: handoff with current state,
remaining conjectures, open edges, and next-move instructions. Blocked
missions are recorded as open edges, not failures.

## Checkpoint Report

After each wave, update `docs/orchestrator-checkpoint-report-2026-06-28.md`
and generate a PDF copy to iCloud (`~/Library/Mobile Documents/com~apple~CloudDocs/Choir Reports/orchestrator-checkpoint-report-2026-06-28.pdf`).

Report format:
- Current V and conjecture verdicts
- Missions delegated, returned, verified, landed
- Missions blocked with open edges
- Next wave plan
- Strong definitive statements produced
- Heresy delta
- Budget spent / remaining

## Suggested Goal String

```text
/goal Run docs/mission-orchestrator-suite-2026-06-28.md as the orchestrator
of the mission suite. Delegate missions to background subagents as
conjectures to decide, not tasks to complete. Wave 1: M1 (API auth), M2
(Choir Base kernel), M11 (race detector), M12 (flaky test), M13 (privacy
policy), M14 (LLM cost tracking). Wave 2: M15 (PR7 review), M18 (worktree
triage), M19 (mission graph triage). Wave 3: M20 (trace observability).
Each subagent receives the conjecture, spec, files, acceptance criterion,
and authority bounds. Verify each return at the orchestrator level:
conjecture decided? evidence admissible? invariants preserved? Land
verified work in separate commits or PRs. Update checkpoint report after
each wave (MD to docs/, PDF to iCloud). Variant V=10. Budget: overnight
session. Settlement: all conjectures decided with typed verdicts and
admissible evidence, verified work committed or PR'd, CI passes or
failures diagnosed, checkpoint report final. Ledger:
docs/mission-orchestrator-suite-2026-06-28.ledger.md.
```
