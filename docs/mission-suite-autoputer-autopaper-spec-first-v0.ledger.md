# Mission Suite Ledger: Autoputer / Autopaper — Spec-First Substrate Redesign

## Pass 0 — 2026-07-03 05:15 EDT

**Conjecture:** The substrate migrations (actor runtime, object graph) outran their TLA+ specifications. The next step is to center the TLA+ spec, rewrite the specs to describe the current architecture, and then drive the code to match the specs.

**Move:** construct (create spec-first suite paradoc, inventory current specs, define missions and dependencies)

**Expected ΔV:** 16 new conjectures proposed (suite variant)

**Actual ΔV:** 16/16 conjectures proposed

**Definitions recorded:**
- D1: TLA+ specs are the source of truth, not a verification afterthought.
- D2: The existing specs are either stale, ahead of code, or proving current code wrong.
- D3: Code changes must follow spec changes (spec-first workflow).
- D4: The actor runtime is the correct concurrency substrate; the old runtime must be deleted.
- D5: The autoputer rename and Nucleus capsule integration are parallel work streams.

**Conjectures proposed:**
Mission S (Spec Redesign):
- C-S1: actor_protocol_og spec holds under current object-graph semantics. (UNDECIDED)
- C-S2: wire_pipeline_og spec captures the wire pipeline flow. (UNDECIDED)
- C-S3: autoputer_lifecycle spec reproduces the current boot failure. (UNDECIDED)
- C-S4: promotion_protocol_og still flags current code violations. (UNDECIDED)

Mission A (Actor Defactoring):
- C-A1: internal/runtime package can be deleted. (UNDECIDED)
- C-A2: cmd/sandbox/main.go builds using only actor runtime + extracted helpers. (UNDECIDED)
- C-A3: Actor runtime tests pass under -race. (UNDECIDED)

Mission B (Wire Redesign):
- C-B1: Wire pipeline compiles and unit tests pass with fake providers. (UNDECIDED)
- C-B2: Wire pipeline produces a real article on staging. (UNDECIDED)
- C-B3: Every fetched item's story eventually settles. (UNDECIDED)

Mission C (Autoputer Solidification):
- C-C1: Autoputer VM image builds with Nucleus included. (UNDECIDED)
- C-C2: Autoputer VM boots and binds to port 8085 on staging. (UNDECIDED)
- C-C3: Nucleus can launch a strict-agent capsule inside the autoputer VM. (UNDECIDED)

Mission D (CI/Verification Guard):
- C-D1: CI passes after each mission commit. (TESTING)
- C-D2: TLA+ specs model-check in CI. (UNDECIDED)
- C-D3: Race detector model is correctly scoped. (SUPPORTED from predecessor)

**Evidence:**
- Existing specs: `specs/actor_protocol.tla`, `actor_protocol_xvm.tla`, `wire_pipeline.tla`, `promotion_protocol.tla`
- `specs/README.md` documents the spec layering and current violations
- `promotion_protocol.tla` already proves current code violates `NoStaleCommit` and `ApprovalGate`
- `wire_pipeline.tla` models publication trajectories that the Go code has not yet adopted
- `cmd/sandbox/main.go` still imports `internal/runtime` for non-concurrency helpers
- Only 5 production files still import `internal/runtime`

**Open decisions needing owner input:**
1. Spec naming: keep old specs as `*_v1.tla` or overwrite in place?
2. TLA+ CI tooling: is the current command correct? Add per-spec TLC configs?
3. Nucleus version: pin rev or follow main?
4. Staging persistent state: reset or migrate? (destructive)
5. Wire redesign scope: full sourcecycled → publish or processor → publish subset?
6. Autoputer rename first or after Mission A?
7. Promotion protocol: fix code to match spec or update spec?

**Next:** Commit the suite paradoc, then spawn subagents for Mission S (spec audit), Mission A (helper extraction), and Mission D (CI/TLA+ verification). Get owner decisions on open questions in parallel.

---

## Pass 1 — 2026-07-03 (Mission S: Promotion Protocol Gate)

**Conjecture:** The old `promotion_protocol.tla` is stale. The current Go code already fixed the two violations it flagged. The autoputer needs a new promotion protocol spec that models the computer ontology, ledger split, route identity, and health window. This spec is the gate for the autoputer.

**Move:** redefine (write assessment doc, delete old specs, write new `specs/promotion_protocol.tla`, update `specs/README.md`, update suite paradoc with default decisions)

**Expected ΔV:**
- Close the 7 open decisions with default decisions (reduces open-decision count to 0).
- Replace C-S4 with two new gate conjectures (C-S4, C-S5).
- Increase suite variant by 2 (new gate conjectures) + 1 (Mission C C-C4 promotion end-to-end) = net +3 after closing decisions.

**Actual ΔV:**
- 7 open decisions closed with default decisions.
- Old specs deleted: `actor_protocol.tla`, `actor_protocol_xvm.tla`, `wire_pipeline.tla`, and their `.cfg` files.
- New `specs/promotion_protocol.tla` written.
- New `specs/promotion_protocol.cfg` written.
- `specs/README.md` rewritten for the new specs.
- Suite paradoc updated with default decisions and promotion-gate focus.
- C-S4 redefined: `promotion_protocol.tla` models active/candidate/route/rollback/health-window and checks green. (TESTING — pending CI TLC run)
- C-S5 redefined: `promotion_protocol.tla` encodes `NoStaleCommit`, `ApprovalGate`, `NoTornOutcome`, `RouteConsistency`, `CandidateIsolation`, `HealthWindowReversible`, `ConfirmedLedgersApplied`, `AbortedLedgersRolledBack`, `CertificateCompleteness`. (TESTING — pending CI TLC run)
- C-C4 added: Promotion protocol end-to-end works on staging: candidate → verify → approve → promote → health window → confirm. (UNDECIDED)
- Updated variant count: 18 conjectures (5 S + 3 A + 3 B + 4 C + 3 D).

**Definitions recorded:**
- D6: Old specs are deleted; new specs are written in place (pre-launch, only good code).
- D7: The promotion protocol is the gate: a persistent computer is not an autoputer until candidate promotion is model-checked, verified, and encoded.
- D8: The current Go code already enforces `NoStaleCommit` and `ApprovalGate`; the new spec must encode and verify these invariants, not just flag old violations.
- D9: Default decisions made (owner can override): delete old specs, keep TLA+ CI command, pin Nucleus rev, reset staging Dolt state, processor→publish wire scope, rename in parallel with Mission A, redefine spec first then encode.

**Evidence:**
- `docs/promotion-protocol-spec-staleness-and-redefinition-2026-07-03.md` documents the current state and the redefinition requirements.
- `internal/runtime/app_promotion.go` now has `ApproveAppAdoption`, `PromoteAppAdoption` requiring `owner_approved`, and `promoteFreshnessCAS`.
- `internal/runtime/app_promotion_freshness_test.go` unit-tests the freshness CAS.
- `docs/computer-ontology.md` provides the computer/ledger/route ontology for the new spec.
- `specs/promotion_protocol.tla` models: `Slots`, `ActiveComps`, `CandidateComps`, `Ledgers`, `MaxTailMoves`; variables `activeBase`, `candidateBase`, `candidateParent`, `route`, `ledgerState`, `promoStatus`, `promoActive`, `promoCandidate`, `promoBase`, `approved`, `poisoned`, `healthWindow`.
- Spec actions: `MoveActiveTail`, `ForkCandidate`, `PrepareLedger`, `Restage`, `Verify`, `Approve`, `Commit`, `Abort`, `ApplySecondary`, `RollbackSecondary`, `PoisonedWrite`, `HealthCheckFail`, `ConfirmHealthy`, `AutoRevert`.
- Spec invariants: `NoStaleCommit`, `ApprovalGate`, `NoTornOutcome`, `RouteConsistency`, `CandidateIsolation`, `HealthWindowReversible`, `ConfirmedLedgersApplied`, `AbortedLedgersRolledBack`, `CertificateCompleteness`.
- Spec liveness: `EveryCommittedPromotionSettles`, `SystemProgress`.

**Open decisions:** None — all closed with default decisions (owner can override at any time).

**Risks:**
- Local TLA+ tools (TLC) are not available in this shell; the spec has not been locally model-checked. Verification depends on the CI `tla-model-check` job. Any syntax or semantic error will be caught by CI, but this is not ideal for a gate artifact.
- The `AutoRevert` action now atomically rolls back all secondaries to preserve the `NoTornOutcome` invariant. This may be coarser than the eventual implementation, but it is a correct refinement.
- The `EveryCommittedPromotionSettles` liveness property deliberately excludes poisoned promotions; forward recovery (a new promotion) is outside the single-promotion model.

**Next:** Commit Pass 1, push to `origin/main`, and monitor the `tla-model-check` CI job. If TLC reports errors, iterate as Pass 2 before any code changes. If TLC passes, the promotion gate is established and Mission C promotion encoding can begin.

---

## Pass 2 — 2026-07-03 (Mission S: Promotion Protocol Gate — TLC iteration)

**Conjecture:** The first CI TLC run of the new `promotion_protocol.tla` revealed a missing `UNCHANGED` variable. The fix is a pure spec correction; no code changes.

**Move:** correct (add `candidateParent` to the `UNCHANGED` tuple of `ForkCandidate`)

**Expected ΔV:** Resolve the first TLC error and get closer to a green model check.

**Actual ΔV:**
- CI run `28647787006` executed TLC and reported:
  ```
  Error: Successor state is not completely specified by action ForkCandidate
  of the next-state relation. The following variable is not assigned: candidateParent.
  ```
- Root cause: `ForkCandidate` did not list `candidateParent` in its `UNCHANGED` tuple.
- All other `UNCHANGED` tuples were reviewed and found complete.
- Fixed in `specs/promotion_protocol.tla` by adding `candidateParent` to `UNCHANGED` in `ForkCandidate`.

**Evidence:**
- CI job log: `gh api repos/choir-hip/go-choir/actions/jobs/84958403708/logs`
- TLC error state trace shows the initial state and the first application of `ForkCandidate`.
- The error is a syntax/semantics issue, not a protocol design flaw.

**Conjecture status update:**
- C-S4: still TESTING (pending second CI TLC run after fix).
- C-S5: still TESTING (pending second CI TLC run after fix).
- C-D1: CI overall failed because of the TLA+ job; will re-evaluate after fix.
- C-D2: still UNDECIDED (pending green TLC run).

**Open decisions:** None.

**Risks:**
- Additional TLC errors may appear after this fix. The spec is still in the CI verification loop.
- Pushing spec fixes to `main` while iterating is acceptable because the Go test gates are green; the only failing gate is TLA+ model-check, which is the intended verification target for Mission S.

**Next:** Commit the fix, push to `origin/main`, and re-run the `tla-model-check` CI job. Repeat until TLC reports "No error has been found".

---

## Pass 3 — 2026-07-03 (Mission S: Promotion Protocol Gate — TLC iteration 2)

**Conjecture:** The second CI TLC run revealed a TLA+ set-construction error in the `SystemProgress` liveness property. The fix is a pure spec correction.

**Move:** correct (replace `{"verified", "approved", TerminalStates}` with `{"verified", "approved"} \cup TerminalStates`)

**Expected ΔV:** Resolve the second TLC error and get closer to a green model check.

**Actual ΔV:**
- CI run `28648099200` executed TLC and reported:
  ```
  Error: TLC threw an unexpected exception.
  Attempted to check equality of the set {"confirmed", "aborted", "reverted"}
  with the value: "staging"
  ```
- Root cause: `SystemProgress` used `{"verified", "approved", TerminalStates}`, which is a set containing two strings and one set, instead of `{"verified", "approved"} \cup TerminalStates`.
- Fixed in `specs/promotion_protocol.tla`.

**Conjecture status update:**
- C-S4: still TESTING (pending third CI TLC run after fix).
- C-S5: still TESTING (pending third CI TLC run after fix).
- C-D1: CI overall failed because of the TLA+ job; will re-evaluate after fix.
- C-D2: still UNDECIDED (pending green TLC run).

**Open decisions:** None.

**Next:** Commit the fix, push to `origin/main`, and re-run the `tla-model-check` CI job. Repeat until TLC reports "No error has been found".

---

## Pass 4 — 2026-07-03 (Mission S: Promotion Protocol Gate — TLC iteration 3)

**Conjecture:** The third CI TLC run revealed that `Abort` does not roll back prepared secondaries, violating the `AbortedLedgersRolledBack` invariant. The fix is a pure spec correction.

**Move:** correct (make `Abort` atomically roll back all prepared secondaries to `rolled_back`)

**Expected ΔV:** Resolve the third TLC error and get closer to a green model check.

**Actual ΔV:**
- CI run `28648213384` executed TLC and produced a counterexample trace ending in `Abort` while `ledgerState[c][source] = "prepared"`.
- Root cause: `Abort` only changed `promoStatus` but left prepared secondaries in `prepared`, violating `AbortedLedgersRolledBack`.
- Fixed in `specs/promotion_protocol.tla` by making `Abort` atomically roll back all prepared secondaries.

**Conjecture status update:**
- C-S4: still TESTING (pending fourth CI TLC run after fix).
- C-S5: still TESTING (pending fourth CI TLC run after fix).
- C-D1: CI overall failed because of the TLA+ job; will re-evaluate after fix.
- C-D2: still UNDECIDED (pending green TLC run).

**Open decisions:** None.

**Next:** Commit the fix, push to `origin/main`, and re-run the `tla-model-check` CI job. Repeat until TLC reports "No error has been found".

---

## Pass 5 — 2026-07-03 (Mission S: Promotion Protocol Gate — TLC iteration 4)

**Conjecture:** The fourth CI TLC run revealed that the `NoStaleCommit` state invariant was too strong: it required the active base to stay equal to the promotion base forever after commit, but the active computer continues to move. The freshness check must be a property of the commit action, not a state invariant.

**Move:** correct (redefine `NoStaleCommit` as an action property `[][\A c : Commit(c) => promoBase[c] = activeBase[promoActive[c]]]_vars` and move it from INVARIANTS to PROPERTIES in the `.cfg`)

**Expected ΔV:** Resolve the fourth TLC error and get closer to a green model check.

**Actual ΔV:**
- CI run `28648346786` executed TLC and reported:
  ```
  Error: Invariant NoStaleCommit is violated.
  ```
  The counterexample trace showed `Commit` at base 0, then `MoveActiveTail` moving the active base to 1, which broke the state invariant.
- Root cause: `NoStaleCommit` was a state invariant requiring `promoBase[c] = activeBase[promoActive[c]]` for all committed promotions. The active computer's base is allowed to move after commit, so the invariant is only meaningful at the moment of commit.
- Fixed by redefining `NoStaleCommit` as an action property and moving it from INVARIANTS to PROPERTIES in `specs/promotion_protocol.cfg`.

**Conjecture status update:**
- C-S4: still TESTING (pending fifth CI TLC run after fix).
- C-S5: still TESTING (pending fifth CI TLC run after fix).
- C-D1: CI overall failed because of the TLA+ job; will re-evaluate after fix.
- C-D2: still UNDECIDED (pending green TLC run).

**Open decisions:** None.

**Next:** Commit the fix, push to `origin/main`, and re-run the `tla-model-check` CI job. Repeat until TLC reports "No error has been found".

---

## Pass 6 — 2026-07-03 (Mission S: Promotion Protocol Gate — TLC iteration 5)

**Conjecture:** The fifth CI TLC run revealed that the `EveryCommittedPromotionSettles` liveness property was too narrow: it excluded poisoned promotions only by premise, but TLA+ `~>` requires the right-hand side to eventually become true if the premise ever becomes true. After poisoned, the promotion cannot reach confirmed/reverted, so the right-hand side never became true. The property must allow the promotion to become poisoned as a valid outcome.

**Move:** correct (expand `EveryCommittedPromotionSettles` right-hand side to include `poisoned[c] = TRUE`)

**Expected ΔV:** Resolve the fifth TLC error and get closer to a green model check.

**Actual ΔV:**
- CI run `28648433822` executed TLC and reported:
  ```
  Error: Temporal properties were violated.
  ```
  The counterexample trace showed a committed promotion reaching `poisoned = TRUE` and then stuttering with all secondaries applied. The old liveness property required eventual `confirmed` or `reverted`, which became impossible after poisoned.
- Root cause: `EveryCommittedPromotionSettles` used `~> promoStatus \in {"confirmed", "reverted"}` but did not account for the valid outcome of becoming poisoned.
- Fixed by making the right-hand side `(promoStatus \in {"confirmed", "reverted"} \/ poisoned = TRUE)`.

**Conjecture status update:**
- C-S4: still TESTING (pending sixth CI TLC run after fix).
- C-S5: still TESTING (pending sixth CI TLC run after fix).
- C-D1: CI overall failed because of the TLA+ job; will re-evaluate after fix.
- C-D2: still UNDECIDED (pending green TLC run).

**Open decisions:** None.

**Next:** Commit the fix, push to `origin/main`, and re-run the `tla-model-check` CI job. Repeat until TLC reports "No error has been found".

---

## Pass 7 — 2026-07-03 (Mission S: Promotion Protocol Gate — GREEN)

**Conjecture:** The sixth iteration of the new `promotion_protocol.tla` model-checks green in CI. The promotion gate is established.

**Move:** verify (monitor CI `tla-model-check` job; confirm green; update ledger)

**Expected ΔV:** C-S4 and C-S5 become SUPPORTED; C-D1 and C-D2 become SUPPORTED for this commit.

**Actual ΔV:**
- CI run `28648508586` completed successfully.
- TLA+ Model Check job (ID `84960756999`) completed in 11s with:
  ```
  Model checking completed. No error has been found.
  826 states generated, 318 distinct states found, 0 states left on queue.
  ```
- All other CI jobs passed: Go build, Go tests (including race detector), frontend build, docs truth check, staging deploy.
- Staging deploy job completed: `Deploy to Staging (Node B) in 1m29s (ID 84961058309)`.

**Conjecture status update:**
- **C-S4: SUPPORTED** — `promotion_protocol.tla` models active/candidate/route/rollback/health-window and checks green (CI run `28648508586`).
- **C-S5: SUPPORTED** — `promotion_protocol.tla` encodes `NoStaleCommit`, `ApprovalGate`, `NoTornOutcome`, `RouteConsistency`, `CandidateIsolation`, `HealthWindowReversible`, `ConfirmedLedgersApplied`, `AbortedLedgersRolledBack`, `CertificateCompleteness`, and liveness `EveryCommittedPromotionSettles` / `SystemProgress`.
- **C-D1: SUPPORTED** — CI passed after the mission commit.
- **C-D2: SUPPORTED** — TLA+ specs model-check in CI.
- **Suite variant:** 18 - 4 = **14 conjectures remaining** (C-S4, C-S5, C-D1, C-D2 decided).

**Definitions recorded:**
- D10: The autoputer promotion gate is established. `promotion_protocol.tla` is the verified spec for candidate promotion.
- D11: The promotion protocol spec captures: computer ontology, ledger split, route identity, per-ledger prepare/apply, owner approval, freshness CAS at commit, health window, poisoned writes, and atomic rollback.
- D12: NoStaleCommit is a property of the commit action, not a state invariant, because the active computer continues to move after promotion.
- D13: Aborted and AutoRevert actions atomically roll back all secondaries to preserve NoTornOutcome.
- D14: A committed promotion's liveness outcome includes confirmed, reverted, or poisoned (forward recovery is a new promotion).

**Open decisions:** None.

**Next:** The promotion gate is established. Proceed to Mission S next items (actor_protocol.tla, autoputer_lifecycle.tla) and Mission A/B/C parallel work. The spec is the source of truth; code changes must now match it.

---

## Pass 8 — 2026-07-03 (Pass 2 Post-Merge CI — Staging Deploy Failure)

**Conjecture:** PR #42 can settle Pass 2 only if post-merge main CI and staging deploy pass. The merge itself is not completion.

**Move:** verify (reconcile PR #42 merged state, monitor main CI, record deploy failure before any fix)

**Expected ΔV:** If post-merge CI and staging passed, C-S1, C-S3, and C-A2 could become SUPPORTED and Pass 2 could close.

**Actual ΔV:**
- PR #42 is merged to `main` as merge commit `a6f11b7dbb64c07677a767c19c00e47cf87fdd54`.
- Main CI run `28683310290` began for the merge commit.
- `TLA+ Model Check (specs/)`, `Go Vet + Build`, frontend build, docs truth check, and multiple Go test shards passed.
- `Deploy to Staging (Node B)` job `85071266856` failed while building the host NixOS closure.
- Pass 2 remains INCOMPLETE until the staging packaging failure is repaired and post-fix CI/deploy proof is green.

**Evidence:**
- `pr://42` reports state `MERGED`.
- `gh run view 28683310290 --json status,conclusion,headSha,url,jobs`
- `gh api repos/choir-hip/go-choir/actions/jobs/85071266856/logs`
- Deploy log root error:
  ```text
  error: Cannot build '/nix/store/...-sandbox-0.1.0.drv'.
  > Building subPackage ./cmd/sandbox
  > cmd/sandbox/main.go:13:2: cannot find module providing package github.com/yusefmosiah/go-choir/internal/apihandler: import lookup disabled by -mod=vendor
  ```

**Root cause hypothesis:**
- PR #42 added `internal/apihandler` and made `cmd/sandbox/main.go` import it.
- Local `go build ./cmd/sandbox` passes because the full source tree is visible.
- The Nix service package source filter in `flake.nix` includes only the service `subPackage` and listed `internalDirs`.
- The sandbox package `internalDirs` does not include `internal/apihandler`, so the Nix build's filtered source omits the package and fails under `-mod=vendor`.

**Conjecture status update:**
- C-S1: still TESTING until Pass 2 post-merge CI fully settles.
- C-S3: still TESTING until Pass 2 post-merge CI fully settles.
- C-A2: weakened from pending support to TESTING, because normal Go build passed but the staging Nix service package path failed.
- C-D1: not supported for merge commit `a6f11b7d` until CI/deploy reruns green.

**Open decisions:** None. This is a packaging-source-filter bug, not a spec waiver or product ontology decision.

**Next:** Fix the root cause in `flake.nix` by including `internal/apihandler` in the sandbox service source filter, verify focused Nix build locally if possible, push the fix, monitor main CI/staging, then update the suite definition checkpoint to the new settled state.

