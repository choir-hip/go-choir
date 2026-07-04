# Landing Brief: Substrate-Independent Audited Computer Changeset

**Date:** 2026-07-04 (updated for Pass 125)
**For:** The agent that ran Passes 0-125 of the autoputer-autopaper spec-first suite.
**From:** Multi-agent review (Devin + Claude Code + Codex), see
`docs/reviews/substrate-independent-audited-computer-changeset-review-2026-07-04.md`
for full findings.

## Why You're Reading This

You were asked to stop after Pass 125. The work is not rejected. The mission
direction is sound. But the changeset has grown to ~40k LOC uncommitted across
36 paths (116 Go files in computerversion, 39 contract files, 5,490 ledger
lines), and a multi-agent review found three bugs that need fixing before
anything lands, plus a strategic decision (PGo) that changes the refactoring
plan. You have tacit knowledge from 125 passes that this brief needs you to
apply.

## What Happened in Passes 104-125

You added 22 more boundary contract files in passes 104-125, building a typed
chain: staging readiness -> smoke evidence -> post-smoke handoff ->
owner-review readiness -> verifier readiness -> verifier result -> owner
approval -> promotion/rollback review -> package-publication readiness ->
package-publication proof -> promotion-execution readiness -> promotion
result -> promotion settlement -> post-settlement handoff ->
durable-state-slice readiness -> durable-state-slice probe ->
source/materializer readiness -> runtime-materialization bridge ->
runtime-equivalence reentry -> runtime-durable proof gap -> extraction
handoff -> retry handoff.

**The review observed this pattern and named it "boundary inflation."** Each
pass adds a new typed boundary between proof states without executing any
actual operations. The changeset is growing, not converging. You have not yet:
- Executed any runtime behavior
- Touched production state
- Performed actual promotion or deployment
- Closed any major proof gate (staging, promotion, package publication, run
  acceptance, full substrate independence)

This is the AGENTS.md dead-end escalation pattern: 125 passes, 2+ days, zero
commits, no convergence on a landing loop. That's not persistence — it's a
known failure mode. The boundary chain is elaborate but inert.

## What's Good (Don't Lose This)

- The intake status machine and owner-approval gating are the strongest code
  in the set. The evidence-only design is correct.
- The deployed surface is genuinely read-only (verified: `apihandler.RegisterRoutes`
  delegates to `runtime.RegisterRoutes`, which mounts only the review-surface GET).
- The ledger practices real problem-first discipline.
- The mission doc's authority ordering and forbidden collapses are sharp.
- Test coverage is extensive (1:1 ratio, ~13k LOC tests).

## Three Landing-Blocker Bugs

These were required before landing. Landing pass status: fixed locally after
the Pass 125 stop, with focused regression tests.

### Bug 1: Cross-owner intake takeover via upsert

**Where:** `internal/store/store.go:233` (table PK is `intake_id` alone),
`internal/store/candidate_package_intake.go:83-110` (upsert does
`ON DUPLICATE KEY UPDATE owner_id = VALUES(owner_id)`).

**Problem:** Owner B can POST with owner A's `intake_id` and silently overwrite
the row, including ownership transfer. Reads are owner-scoped so it's
hijack/denial, not a leak.

**Fix:** Add uniqueness on `(owner_id, intake_id)` or an ownership check before
upsert. You know the store layer — pick the approach that fits your schema
migration pattern.

**Landing status:** Fixed in `internal/store/candidate_package_intake.go` by
rejecting an upsert when an existing `intake_id` belongs to a different
`owner_id`; `owner_id` is no longer overwritten on duplicate-key update.
Regression: `TestCandidatePackageIntakeRejectsCrossOwnerUpsertTakeover`.

### Bug 2: TOCTOU on intake state transitions

**Where:** `internal/runtime/candidate_package_intake.go` — all write methods
load, validate, mutate, upsert without compare-and-set guards.

**Problem:** Concurrent review/adoption requests can race terminal transitions
(double-approve, conflicting switch/rollback). The lineage-drift checks only
cover switch/rollback, not review/decision.

**Fix:** Add an optimistic-concurrency guard (`updated_at` or version column)
to intake state transitions. Use atomic `WHERE old_status = ? UPDATE SET
new_status = ?` patterns. You already have `updated_at` in the schema — wire
it into the upsert as a CAS guard.

**Landing status:** Fixed in `internal/store` and
`internal/runtime/candidate_package_intake.go` by adding updated-at
compare-and-set update helpers for candidate-package intakes and app adoptions,
then using them on owner review, adoption-boundary, adoption review, switch,
rollback, and roll-forward transitions. Regressions:
`TestUpdateCandidatePackageIntakeIfCurrentRejectsStaleUpdatedAt` and
`TestUpdateAppAdoptionIfCurrentRejectsStaleUpdatedAt`.

### Bug 3: Deployment guard on write-route registration

**Where:** `internal/runtime/api_candidate_package_intake.go:22` —
`RegisterCandidatePackageIntakeRoutes` (full write routes) must never be called
in deployed runtime. Currently enforced only by comments.

**Problem:** Convention, not mechanism. A future wiring change could
accidentally mount write routes in production.

**Fix:** Add a build tag (`//go:build !production`) or runtime guard (env var
check, panic on deployed profile). You know the deployment shape — pick what
fits.

**Landing status:** Fixed in `internal/runtime/api_candidate_package_intake.go`
with an environment guard that panics when the opt-in write registrar is called
under deployed/runtime production env flags; the read-only review-surface
registrar remains allowed. Regressions:
`TestRegisterCandidatePackageIntakeRoutesPanicsWhenDeployedEnvSet` and
`TestRegisterCandidatePackageReviewSurfaceRoutesRegistersWhenDeployedEnvSet`.

## The PGo Strategic Decision

There is a PGo evaluation mission staged at
`/private/tmp/pgo-evaluation/docs/missions/pgo-evaluation-v0.md`. PGo compiles
Modular PlusCal (MPCal) specs directly to Go. If it works, much of the
hand-written contract code could be generated from the spec instead.

**This changes your refactoring plan:**

| What | If PGo is GO | If PGo is NO-GO |
|------|-------------|----------------|
| Contract explosion (37 files, 60% duplication) | Moot — contracts generated | Must refactor — extract shared header |
| Large file splitting (2,131 + 1,688 lines) | Different — split by archetype | Must refactor — split by concern |
| Vacuous TLA+ invariants | Replaced — real MPCal refinement | Must fix manually — independent counters |
| Purity claim | Reshaped — extraction may be generated | Must fix — scope or split package |

**The two new TLA+ invariants (`RouteNamesComputerVersion`,
`PromotionNamesComputerVersion`) are vacuous.**
`ComputerVersionOfBase(n) == [codeRef |-> n, artifactProgramRef |-> n]` maps
every counter into `ComputerVersions` by construction — the invariants can
never fail. Worse, both refs map to the same counter, so the model can't
express "code changed, state didn't" — the exact distinction the
`ComputerVersion` pair exists to make. Either strengthen to a real refinement
or drop them. Don't leave them as false proof.

## Your Landing Sequence

1. **Checkpoint the ledger now.** Commit it in phase chunks (Passes 0-7,
   8-23, 24-25, 26-57, 58-83, 84-104). It's docs-only — skips full CI/deploy.
   This preserves your 125 passes of reasoning before any code changes.

2. **Fix the three bugs.** Small, surgical, in the existing code. Landing pass
   status: fixed locally with focused tests; commit these fixes separately from
   the docs checkpoint if preserving problem-documentation-first ordering.

3. **Run the PGo evaluation.** Read
   `docs/missions/pgo-evaluation-v0.md`. Build PGo, attempt MPCal translation
   of `promotion_protocol.tla`, generate Go. This is hours, not days.

4. **Land the changeset in phases.** Suggested sequence:
   docs (ledger + definitions) -> store/types -> computerversion -> cmds ->
   runtime -> frontend. Push through the landing loop with CI + staging
   evidence for each phase that touches runtime behavior.

5. **Then refactor/harden** based on PGo result:
   - **If GO:** Don't refactor the hand-written contracts for maintainability
     — they'll be replaced. Focus on the integration boundary. Rewrite the
     spec in MPCal, compile, use generated code as foundation.
   - **If NO-GO:** Full maintainability refactor — extract shared contract
     header, split large files, fix purity claim, strengthen TLA+ invariants
     manually, consolidate cmd helpers.

## What Only You Know

You ran 125 passes. You know:
- Why the contract layer has 39 files (each pass added one). Is this
  scaffolding to be collapsed, or the permanent shape? Passes 104-125 added
  22 more in the same pattern — was this building toward something, or was
  it the boundary inflation pattern the review identified?
- Whether `ComputerSourceLineage.ActiveSourceRef` can ever drive a deployed
  route (review couldn't determine this).
- Whether the intake state machine was designed to eventually be expressed
  in TLA+/MPCal (it's a natural fit for a PlusCal algorithm).
- What the next passes would have been if you hadn't been stopped. Pass 125
  ended with "open a fresh red ceremony before any VM lifecycle, staging,
  promotion, package-publication, run-acceptance, or production mutation" —
  were you about to open that ceremony, or continue adding boundaries?
- Whether the 22-step boundary chain (staging -> smoke -> handoff -> owner
  review -> verifier -> approval -> promotion review -> publication ->
  promotion execution -> settlement -> substrate return -> runtime reentry)
  was designed as a finite sequence or an open-ended pattern.

Use this knowledge. The review found what static analysis can find. You know
what static analysis can't.

## Open Questions For You

1. Does anything consume `ComputerSourceLineage.ActiveSourceRef` to serve a
   deployed route? If yes, the switch claim is wrong and that path is red.

2. Is the contract-per-micro-step pattern intended as scaffolding or
   permanent? You now have 39 contract files with ~60% duplication. If
   permanent, the generic-header refactor is needed regardless of PGo. If
   scaffolding, when were you planning to collapse it?

3. What was your next planned pass after 125? Were you about to open the red
   ceremony, or add more boundaries? This directly informs whether the
   boundary chain is complete or still growing.

4. Were you planning to land this as one commit or in phases? The review
   recommends phases per AGENTS.md ordering. 125 passes with zero commits
   violates the landing loop.

5. Should the `candidate_package_intake` state machine be expressed in MPCal
   if PGo works? It would replace ~1,688 lines of hand-written state machine
   with generated code and address the TOCTOU concern at the spec level.

6. The boundary chain you built in passes 104-125 maps closely to a PlusCal
   algorithm structure (staging -> verification -> approval -> promotion ->
   settlement). Was this intentional preparation for MPCal translation, or
   just the natural shape of the proof chain?
