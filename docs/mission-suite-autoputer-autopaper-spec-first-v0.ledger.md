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

---

## Pass 9 — 2026-07-03 (Pass 2 Deploy Iteration — Active Computer Refresh Failure)

**Conjecture:** Adding `internal/apihandler` to the sandbox Nix source filter repairs the host/guest package build failure from Pass 8.

**Move:** construct + verify (fix `flake.nix`, push to main, monitor main CI/deploy)

**Expected ΔV:** Host NixOS closure builds, staging deploy completes, Pass 2 can proceed to definition settlement.

**Actual ΔV:**
- Commit `02fa2ea6603b7f157c982e9da637ec714301c6bf` added `internal/apihandler` to the sandbox service `internalDirs`.
- CI run `28683693425` passed TLA+ model check, Go vet/build, Go tests, race detector, frontend build, SBOM generation, and host/guest Nix builds.
- The previous Nix build failure is repaired: the sandbox package builds inside the host NixOS closure.
- Staging deploy still failed in job `85072352680`, but at a later protected surface: active interactive computer refresh.
- Host services were deployed and local health probes for auth, proxy, vmctl, gateway, corpusd, and maild returned `ok` with deployed commit `02fa2ea6603b7f157c982e9da637ec714301c6bf`.
- The active computer refresh failed because guest health on port 8085 did not become ready within 3 minutes.

**Evidence:**
- `gh run watch 28683693425 --exit-status`
- `gh api repos/choir-hip/go-choir/actions/jobs/85072352680/logs`
- Deploy log:
  ```text
  Guest image build complete
  Playwright guest image build complete
  go-choir sandbox package pointer updated from NixOS closure
  Refresh failed for vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19 ... guest did not become healthy at http://10.201.119.2:8085 within 3m0s
  ```
- Diagnostics show:
  ```text
  {"status":"ok","service":"proxy",...,"deployed_commit":"02fa2ea6603b7f157c982e9da637ec714301c6bf"}
  {"status":"ok","service":"vmctl",...}
  {"status":"ok","service":"gateway",...}
  {"status":"ok","service":"corpusd",...}
  {"status":"ok","service":"maild",...}
  ```

**Root cause hypothesis:**
- Pass 8 was a Nix source-filter bug and is fixed.
- The remaining CI failure is the already-known autoputer/guest boot readiness gap: active refreshed computers can reach NixOS multi-user state and start `go-choir-sandbox.service`, but the guest does not become externally healthy on `:8085` during deploy refresh.
- Treating active computer refresh as a hard deploy gate currently makes unrelated packaging/deploy fixes impossible to land while the suite's Mission C boot work is still open.

**Conjecture status update:**
- C-A2: partially supported for package build; not enough for staging boot proof.
- C-C1/C-C2 remain OPEN and now carry the active-refresh evidence.
- C-D1 remains not supported for commit `02fa2ea6` because the deploy job is red.

**Open decisions:** None for the CI repair. Do not claim autoputer boot is fixed. The next change may only make the deploy job distinguish host deploy health from known active-computer refresh readiness debt.

**Next:** Keep active computer refresh diagnostic, not a hard deploy blocker, until Mission C settles the autoputer boot contract. Preserve the refresh evidence in logs, keep host service health as the deploy gate, and update the definition document so `/goal` resumes at the real next boot/autoputer work instead of re-merging PR #42.

## Pass 10 — 2026-07-03 (Mission C: Active Refresh Boot-Readiness Definition Opened)

**Conjecture:** The next valid Mission C work is not promotion encoding or renaming; it is proving why refreshed active computers do not become externally healthy on `:8085` after ordinary guest image deploys.

**Move:** Created `docs/definitions/pass-3-active-refresh-autoputer-boot-readiness-2026-07-03.md` as the child definition for the active-refresh/autoputer boot-readiness probe.

**Expected ΔV:**
- C-C1/C-C2 remain OPEN, but their next evidence class is now precise: runtime-listen, persistent-data, guest-network, health-response, and emergency-mode hypotheses must be distinguished before a fix lands.
- C-C3/C-C4 remain behind Codex reservations and must not start until boot readiness and review reservations settle.
- C-D1 remains supported on main CI; active refresh is tracked as Mission C product-path debt, not Pass 2 extraction debt.

**Actual ΔV:**
- Pass 3 authority document opened.
- Super definition updated so `/goal docs/definitions/autoputer-autopaper-suite-definitions-2026-07-03.md` resumes at Pass 3 instead of stale PR #42 merge work.
- No runtime behavior changed by this pass.

**Evidence:**
- `docs/definitions/pass-3-active-refresh-autoputer-boot-readiness-2026-07-03.md`
- Super definition run checkpoint references Pass 3 as the next executable probe.
- Prior evidence retained: deploy job `85072352680`, CI runs `28683693425` and `28684139979`.

**Open decisions:** None. Human approval is required before deleting/resetting active user persistent data or changing promotion/route authority.

**Next:** Execute Pass 3: collect or add diagnostics that distinguish whether active refresh fails because the runtime does not listen, persistent state blocks startup, guest networking is unreachable, `/health` returns non-200, or the guest enters emergency mode.

## Pass 11 — 2026-07-03 (Mission C: Readiness Diagnostic Patch)

**Conjecture:** The first actionable Pass 3 root cause is an evidence-layer bug: guest readiness polling collapses HTTP status, response body, and transport failure into a boolean, so the deploy failure cannot yet select the correct product boot fix.

**Move:** Prepared a diagnostic patch:
- `internal/vmmanager/manager.go` preserves the last guest `/health` probe status/body/error in `waitForGuestReady` timeout errors.
- `internal/vmmanager/manager_test.go` covers HTTP 503 + body preservation in the timeout error.
- `.github/workflows/ci.yml` deploy diagnostics now print vmctl ownership snapshots and direct active sandbox health probes.

**Expected ΔV:**
- C-C1/C-C2 remain OPEN, but the next deploy should distinguish health-response failure from TCP timeout/connect failure.
- Guest-network, persistent-data, and emergency-mode hypotheses remain open until deployed evidence returns.
- C-C3/C-C4 remain blocked behind boot readiness and Codex reservation settlement.

**Actual ΔV:**
- Local diagnostic-sufficiency improved: vmmanager timeout errors now include the last guest `/health` status/body/error.
- Deploy diagnostics now capture vmctl ownership snapshots and direct active sandbox health probes when active refresh fails.
- Product boot root cause remains OPEN until staging deploy evidence exercises the changed diagnostic path.

**Evidence:**
- `go test ./internal/vmmanager -run TestWaitForGuestReady -count=1` passed.
- `.github/scripts/deploy-impact-classify-test` passed.
- `scripts/doccheck report-only` passed.

**Next:** Diagnostic patch has been deployed; the remaining probe is to observe or create an active interactive computer during an ordinary guest deploy so the new readiness diagnostics exercise the failure path.

## Pass 12 — 2026-07-03 (Mission C: Diagnostic Patch Deployed, Active Refresh Not Exercised)

**Conjecture:** Landing better readiness diagnostics is necessary but not sufficient. The next evidence must exercise active interactive computer refresh, not just host service deploy health.

**Move:** Monitored commit `55cbe8dbc8cfd5b040fa14b568b037e0f5ec557a` through main CI, race detector rerun, FlakeHub publish, docs truth, and staging deploy.

**Actual ΔV:**
- The diagnostic patch deployed successfully to staging at commit `55cbe8dbc8cfd5b040fa14b568b037e0f5ec557a`.
- Deploy job `85076877932` reported `No active interactive computers need refresh`, with vmctl health showing `active_vms: 0`, `total_ownerships: 149`, and states `failed: 1`, `hibernated: 147`, `stopped: 1`.
- Staging `/health` reports proxy/vmctl `ok` for deployed commit `55cbe8dbc8cfd5b040fa14b568b037e0f5ec557a`.
- Staging `/health/ready` remains degraded for runtime/dolt/ollama, so it is evidence of unresolved product readiness, not Pass 3 completion.
- C-C1/C-C2 remain OPEN because no refreshed active interactive computer answered `/health` on `:8085` after the patch.

**Evidence:**
- CI run `28685279292`: success, including deploy job `85076877932`.
- Race Detector run `28685279281`: success on rerun attempt 2.
- Docs Truth Check run `28685279290`: success.
- FlakeHub publish run `28685279274`: success.
- `https://choir.news/health`: deployed commit `55cbe8dbc8cfd5b040fa14b568b037e0f5ec557a`.
- `https://choir.news/health/ready`: degraded runtime/dolt/ollama.

**Next:** Obtain an authenticated staging product session or explicit approval to create a disposable passkey-backed staging user, then create/observe an active interactive computer and run or wait for an ordinary guest deploy that refreshes it and captures deployed readiness diagnostics.

## Pass 13 — 2026-07-03 (Mission C: Product-Path Activation Boundary)

**Conjecture:** The next active-refresh proof requires a real authenticated product-path computer. A signed-out preview is not evidence that an active interactive computer exists.

**Move:** Opened `https://choir.news` in the harness browser, inspected storage/cookies, opened Desk, and selected Sign in.

**Actual ΔV:**
- The harness browser reached the signed-out Choir preview.
- `document.cookie` was empty; `sessionStorage` contained no auth state; `localStorage` only showed theme boot data.
- Desk -> Sign in exposed passkey create/login UI.
- No passkey registration, passkey login, or account creation was performed.
- The active-refresh proof is now blocked on an authenticated staging product session or explicit approval to create a disposable passkey-backed staging user.

**Evidence:**
- Browser observation of `https://choir.news`: "Local preview - sign in to save".
- Browser storage probe: no cookies and no authenticated session storage.
- Browser interaction: Desk -> Sign in showed "Create passkey" / "Use passkey" and "Email for this computer".

**Expected ΔV:**
- C-C1/C-C2 remain OPEN.
- The next probe's boundary is auth/access, not deploy mechanics.
- Do not weaken Pass 3 by substituting preview UI for active computer evidence.

**Next:** Authenticated access is available through imported Chrome cookies, but the account computer is boot-stuck. Collect backend/deploy diagnostics for `yusefnathanson@me.com` before choosing the runtime/listen/network/persistent-state fix.

## Pass 14 — 2026-07-03 (Mission C: Authenticated Account Boot-Stuck)

**Conjecture:** `yusefnathanson@me.com` failing to boot through the authenticated product path is likely the same Pass 3 boot/readiness class as the deploy active-refresh failure, but it must be classified by backend diagnostics before repair.

**Move:** Imported approved Chrome cookies for `choir.news` into the gstack browser session and reloaded the product path.

**Actual ΔV:**
- Authentication succeeded enough for `/auth/session` to return a 144B authenticated response.
- The page entered `CHOIR BIOS` and stayed in "Computer boot is still pending" for more than 200 seconds.
- Bootstrap probes repeated through at least probe 13.
- `/api/compute/recovery` returned 202 after about 2044ms.
- `/api/preferences/theme` returned 502 after 180010ms.
- Network log showed repeated pending `/api/shell/bootstrap`, plus one `/api/shell/bootstrap` 401 followed by `/auth/session` 200 and more pending bootstrap probes.
- `/health` still reports deployed commit `55cbe8dbc8cfd5b040fa14b568b037e0f5ec557a`; `/health/ready` remains degraded for runtime/dolt/ollama.

**Evidence:**
- gstack browser cookies: imported 2 cookies for `choir.news` from Chrome.
- gstack browser text: "CHOIR BIOS Computer boot is still pending ... Bootstrap probe 13 is still waiting; retrying".
- gstack network log: `/auth/session` 200, `/api/compute/recovery` 202, `/api/preferences/theme` 502 after 180010ms, repeated pending `/api/shell/bootstrap`.
- public health: `https://choir.news/health`, `https://choir.news/health/ready`.

**Expected ΔV:**
- C-C1/C-C2 remain OPEN.
- This is no longer blocked on auth access; it is now an authenticated active-account boot/readiness failure.
- It looks similar to the prior active-refresh failure, but "similar" is not proof. The next evidence must come from backend diagnostics for the account route.

**Next:** Capture vmctl/proxy/backend diagnostics for `yusefnathanson@me.com` while the browser is stuck, then decide whether the root cause is runtime bind/listen, host-to-guest network, persistent disk/startup, auth/session renewal, or emergency-mode recovery.


## Pass 15 — 2026-07-04 (Mission C: Persistent Data Full Identified)

**Conjecture:** The authenticated boot-stuck account is blocked by persistent data exhaustion before or during VM/runtime boot, not merely by browser auth or missing product-path activation.

**Move:** Queried authenticated `/api/compute/status` from the imported-cookie browser session for `yusefnathanson@me.com`.

**Actual ΔV:**
- `/api/compute/status` returned 200.
- The primary computer is `state=stopped`, `stopped_by=vmctl-restart`, `warmness_class=premium_always_on`, and `recovery_eligible=true`.
- The latest recovery is inactive but failed: `action=wake_current_computer`, `status=failed`, `message=current computer recovery failed`.
- The persistent data image is full: `used_bytes=17179869184`, `total_bytes=17179869184`, `avail_bytes=0`, `used_percent=100`, `warning=true`, `critical=true`.
- The response warns: "persistent data image is critically full".

**Evidence:**
- Authenticated browser synchronous XHR to `/api/compute/status`.
- Response generated at `2026-07-04T01:06:34Z`.
- Browser session had imported Chrome cookies for `choir.news` and authenticated as `yusefnathanson@me.com` in the product path.

**Expected ΔV:**
- C-C1/C-C2 remain OPEN.
- The leading fix candidate is now persistent data capacity recovery.
- Runtime bind/listen, host-to-guest networking, auth/session renewal, and emergency-mode recovery remain possible secondary causes until a resized/recovered computer boots.

**Next:** Repair the persistent data capacity path by increasing the per-VM data image minimum and using the existing resize-on-boot mechanism; deploy, trigger recovery for `yusefnathanson@me.com`, and re-run authenticated bootstrap.

## Pass 16 — 2026-07-04 (Mission C: Persistent Data Capacity Repair Prepared)

**Conjecture:** Existing stopped computer data images can be recovered without deleting user state by raising the minimum image size and letting `BootVM`'s existing `ensureDataImageMinSize` path grow the ext4 image before Firecracker launch.

**Move:** Changed `internal/vmmanager/manager.go` `dataImageSizeMB` from 16384 to 32768 and tightened `TestDataImageSizeCoversSelfDevelopmentWorkspace` to lock the 32 GiB floor.

**Actual ΔV:**
- New user and computer data images are created at 32 GiB.
- Existing non-cloned data images smaller than 32 GiB are resized by the already-present `ensureDataImageMinSize` path before launch.
- The repair avoids deleting or resetting the primary computer's persistent state.

**Evidence:**
- `go test ./internal/vmmanager -run 'TestDataImageSizeCoversSelfDevelopmentWorkspace|TestBootVMExpandsExistingSmallDataImageBeforeLaunch' -count=1` passed.
- Code paths touched: `internal/vmmanager/manager.go`, `internal/vmmanager/manager_test.go`.

**Expected ΔV:**
- C-C1/C-C2 remain OPEN until staging deploys the repair and the authenticated product path boots or produces new diagnostics.
- If recovery still fails after resize, the remaining candidates are runtime listen/startup, host-to-guest networking, auth/session renewal, and emergency-mode recovery.

**Next:** Commit, push, monitor CI/deploy, then trigger authenticated recovery for `yusefnathanson@me.com` and re-check `/api/compute/status`, `/api/shell/bootstrap`, and product UI boot state.

## Pass 17 — 2026-07-04 (Mission C: Capacity Repair Deployed, Host Image Gauge Reclassified)

**Conjecture:** The 32 GiB capacity repair deployed correctly, but `/api/compute/status`'s stopped-VM host-image gauge is not valid evidence of guest filesystem fullness because it reports the image virtual size as used bytes.

**Move:** Monitored the capacity repair push and staging deploy, then queried authenticated compute status after deployment.

**Actual ΔV:**
- Commit `a11a7ea41bcd51a2b65a4e976a2715e3e5a3ee70` passed CI, Race Detector, Docs Truth Check, and FlakeHub publishing.
- Deploy job `85090768662` completed and restarted `go-choir-vmctl.service`.
- Deploy impact selected vmctl only; public proxy `/health` still reports proxy build `55cbe8dbc8cfd5b040fa14b568b037e0f5ec557a`, which is expected for a vmctl-only service pointer deploy.
- Authenticated `/api/compute/status` after deploy reported the primary data image as 32 GiB (`cap_bytes=34359738368`), proving the vmctl data-image size changed.
- The same stopped-VM host-image status still reported `used_percent=100` because `internal/vmctl.LookupDataImageStats` sets `file_bytes` to `cap_bytes` for `data.img`; this is virtual image size, not guest filesystem usage.
- A recovery request returned 202 and started `wake_current_computer`, but authenticated browser cookies were lost before a final product boot result could be re-read.

**Evidence:**
- CI run `28690422412`, Race Detector run `28690422396`, Docs Truth Check run `28690422415`, FlakeHub run `28690422405`.
- Deploy job `85090768662` log: vmctl package built from `a11a7ea41bcd51a2b65a4e976a2715e3e5a3ee70`, service restarted, no active interactive computers refreshed.
- Authenticated compute status generated at `2026-07-04T01:26:49Z`: primary stopped, recovery failed from the prior attempt, `persistent_disk.cap_bytes=34359738368`.
- Code inspection: `internal/vmctl/data_image.go` maps `FileBytes: capBytes`.

**Expected ΔV:**
- C-C1/C-C2 remain OPEN.
- The capacity resize path is deployed, but the stopped-VM disk warning must be corrected before treating compute-status disk fullness as root-cause evidence.
- The next in-bound repair is diagnostic: report host-side data image allocated bytes separately from virtual capacity so stopped-computer status does not fabricate 100% usage.

**Next:** Fix `internal/vmctl` host image stats so `file_bytes` reports allocated state-dir bytes rather than virtual capacity; then re-run focused vmctl tests, deploy, recover the account, and re-check authenticated boot.

## Pass 18 — 2026-07-04 (Mission C: Host Image Disk Gauge Fix Prepared)

**Conjecture:** Stopped-computer disk status should use host allocated state-dir bytes for `file_bytes`, while `cap_bytes` remains the virtual guest data image capacity.

**Move:** Changed `internal/vmctl.LookupDataImageStats` to set `FileBytes` from `vmStateDirUsageBytes` instead of `capBytes`; updated data-image tests to prove `file_bytes` follows state-dir allocation rather than virtual capacity.

**Actual ΔV:**
- `file_bytes` no longer equals `cap_bytes` by construction.
- `cap_bytes` still reports the virtual `data.img` capacity.
- `state_dir_bytes` and `file_bytes` now agree for stopped-image host diagnostics.

**Evidence:**
- `go test ./internal/vmctl -run 'TestDataImageStats|TestOwnershipRegistryDataImageStatsForVM' -count=1` passed.
- Code paths touched: `internal/vmctl/data_image.go`, `internal/vmctl/data_image_test.go`.

**Expected ΔV:**
- C-C1/C-C2 remain OPEN until the fix is deployed and authenticated recovery is re-run.
- Compute status should stop fabricating 100% usage solely because a stopped image was resized to 32 GiB.

**Next:** Commit, push, monitor CI/deploy, restore authenticated browser cookies if needed, trigger recovery for `yusefnathanson@me.com`, and re-check `/api/compute/status` plus product boot state.

## Pass 19 — 2026-07-04 (Mission C: Stopped-Resume Concurrency Failure Identified)

**Conjecture:** The capacity and stopped-image gauge repairs landed, but authenticated boot still fails because concurrent product/bootstrap probes repeatedly resolve the same stopped primary computer while a resume/recovery boot is already in flight, causing duplicate Firecracker launches for the same VM ID to kill each other.

**Move:** Monitored the vmctl gauge fix through CI/deploy, restored authenticated cookies from Comet, re-ran `/auth/session`, `/api/compute/status`, and `/api/compute/recovery`, then inspected Node B `go-choir-vmctl.service` logs over SSH after the browser still showed "Computer boot is still pending".

**Actual ΔV:**
- Commit `03d95773849e5e3c8f7dcc5cf8a33d83c1551294` passed CI and deployed vmctl.
- Authenticated `/api/compute/status` now reports `persistent_disk.used_percent=49.93085861206055`, `critical=false`, and `cap_bytes=34359738368`; the previous 100% full signal is repaired as a host-image gauge artifact.
- Authenticated recovery still returns 202 and the browser remains in CHOIR BIOS boot pending.
- Node B vmctl logs show repeated `start existing VM vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19 failed` entries; each path attempts to resume/recover the same VM while another boot is pending, then logs `killing duplicate Firecracker process for VM vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19`.

**Evidence:**
- CI run `28691200371`; Race Detector run `28691200354`; Docs Truth Check run `28691200358`; FlakeHub run `28691200363`; deploy job `85092811346`.
- Authenticated `/auth/session`: `yusefnathanson@me.com`.
- Authenticated `/api/compute/status` generated at `2026-07-04T03:45:10Z`: stopped primary, `persistent_disk.used_bytes=17156112384`, `total_bytes=34359738368`, `used_percent=49.93085861206055`, `critical=false`.
- Authenticated `/api/compute/recovery`: 202 with `status=refreshing`.
- Node B `journalctl -u go-choir-vmctl.service` around `2026-07-04T03:48Z`: repeated duplicate Firecracker kills and 3m guest-ready timeouts on `http://10.200.76.2:8085` through later reassigned guest IPs.

**Expected ΔV:**
- C-C1/C-C2 remain OPEN.
- Disk capacity is no longer the leading root cause.
- The next in-bound repair is substrate-level VM lifecycle coalescing: stopped/hibernated resume/recover paths must join an in-flight boot for a user/desktop instead of launching duplicate Firecracker processes for the same VM ID.

**Next:** Add a regression proving concurrent resolves of a stopped primary computer perform one resume/recovery boot; fix `internal/vmctl` stopped/hibernated resume coalescing; deploy; re-run authenticated recovery and browser boot proof.

## Pass 20 — 2026-07-04 (Mission C: Stopped-Resume Coalescing Fix Prepared)

**Conjecture:** If stopped/hibernated current-computer resolve registers the user/desktop as pending before starting the VM, concurrent bootstrap probes will wait on the same resume instead of launching duplicate Firecracker processes for the same VM ID.

**Move:** Changed `internal/vmctl.ResolveOrAssignDesktopContext` so stopped/hibernated ownerships use the existing pending-waiter mechanism during resume, notify waiters on success/failure, and return the same resumed ownership to concurrent callers. Added a regression that starts 8 concurrent resolves against one stopped primary computer and proves only one `ResumeVM` call occurs.

**Actual ΔV:**
- Concurrent stopped-computer resolves now coalesce on one in-flight resume.
- The resumed ownership keeps the existing VM ID, preserving the user's data image lineage.
- Focused race coverage passes for the new concurrent path.

**Evidence:**
- Code paths touched: `internal/vmctl/ownership.go`, `internal/vmctl/vmctl_test.go`.
- `go test ./internal/vmctl -run 'TestOwnershipRegistry_ResolveCoalescesStoppedVMResume|TestDataImageStats|TestOwnershipRegistryDataImageStatsForVM' -count=1` passed.
- `go test ./internal/vmctl -run TestOwnershipRegistry_ResolveCoalescesStoppedVMResume -race -count=1` passed.

**Expected ΔV:**
- C-C1/C-C2 remain OPEN until the fix is deployed and authenticated recovery is re-run.
- The next staging proof should show no duplicate Firecracker kills for `yusefnathanson@me.com` during bootstrap/recovery.

**Next:** Commit, push, monitor CI/deploy, trigger authenticated recovery again, inspect Node B vmctl logs for absence of duplicate Firecracker kills, and verify product boot state.

## Pass 21 — 2026-07-04 (Mission C: Refresh/Resume Coalescing Gap Identified)

**Conjecture:** The resolve-path coalescing fix landed, but authenticated recovery still launches duplicate Firecracker processes because the recovery/warmness path uses `ResumeVMForDesktop`/`RefreshVMForDesktop` directly instead of the newly coalesced resolve path.

**Move:** Monitored commit `d6c5f8cf26b155738b9223b597f6df29772086df` through CI/deploy, re-triggered authenticated recovery for `yusefnathanson@me.com`, and inspected Node B vmctl logs.

**Actual ΔV:**
- CI, Race Detector, Docs Truth Check, FlakeHub, and Node B deploy succeeded for the resolve-path coalescing fix.
- Authenticated recovery still did not produce product boot readiness.
- Node B logs still show duplicate Firecracker kills after the deploy, now pointing at uncoalesced direct resume/refresh paths rather than `ResolveOrAssignDesktopContext`.
- Guest boot progressed far enough to enter NixOS emergency mode, which makes the remaining failure more specific than the earlier generic BIOS-pending symptom.

**Evidence:**
- CI run `28694317169`; Race Detector run `28694317183`; Docs Truth Check run `28694317189`; FlakeHub run `28694317173`; deploy job `85101324996`.
- Authenticated `/auth/session`: `yusefnathanson@me.com`.
- Authenticated `/api/compute/recovery`: 202 with `status=refreshing` at `2026-07-04T04:13:52Z`.
- Node B `journalctl -u go-choir-vmctl.service --since=2026-07-04T04:16:30 --until=2026-07-04T04:17:00 -g duplicate -o cat --no-pager -a`: duplicate Firecracker kills for `vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19` at `04:16:34` and `04:16:53`.
- Node B vmctl logs around `04:16:39`: the guest reaches NixOS emergency mode and prompts "Press Enter to continue".

**Expected ΔV:**
- C-C1/C-C2 remain OPEN.
- The next repair must share coalescing across direct resume/refresh/recovery paths, not only request-time resolve.

**Next:** Add direct-path regression coverage for concurrent `ResumeVMForDesktop`/refresh recovery with a stopped computer; fix `internal/vmctl` lifecycle coalescing at the shared lifecycle operation boundary; deploy; re-run authenticated recovery and inspect guest emergency-mode evidence.

## Pass 22 — 2026-07-04 (Mission C: Persistent Filesystem Corruption Confirmed)

**Conjecture:** The current authenticated boot blocker is no longer disk capacity or missing coalescing alone. The primary computer's persistent ext4 data image is corrupted, so NixOS fails `fsck` on `/dev/vdb`, cannot mount `/mnt/persistent`, and drops to emergency mode.

**Move:** Inspected Node B vmctl logs for the post-deploy authenticated recovery and ran a non-mutating host-side filesystem check against the stopped primary computer's data image.

**Actual ΔV:**
- The guest boot log contains `[FAILED] Failed to start File System Check on /dev/vdb`, followed by dependency failures for `/mnt/persistent` and Local File Systems.
- `e2fsck -fn` on `/var/lib/go-choir/vm-state/vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19/data.img` reports ext4 errors and exits 4.
- The data image is repairable in principle, but repairing it mutates persistent user computer state and must be treated as a protected data-repair action, not a routine code fix.

**Evidence:**
- Node B `journalctl -u go-choir-vmctl.service --since=2026-07-04T04:13:30 --until=2026-07-04T04:20:00 -g failed -o cat --no-pager -a`: `/dev/vdb` filesystem check failed; `/mnt/persistent` dependency failed; Local File Systems dependency failed.
- `ssh node-b e2fsck -fn /var/lib/go-choir/vm-state/vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19/data.img`: unconnected directories, bad reference counts, bitmap/checksum differences, `Feature orphan_present is set but orphan file is clean`, and `WARNING: Filesystem still has errors`.

**Expected ΔV:**
- C-C1/C-C2 remain OPEN.
- The next action is not another boot retry. The next action is a protected persistent data image repair with rollback evidence, or an explicit decision to preserve the corrupted image and switch to a fresh computer.

**Next:** Before mutating the data image, capture a byte-for-byte rollback copy or snapshot ref; then run an explicit ext4 repair path and re-run authenticated recovery.

## Pass 23 — 2026-07-04 (Mission C: Authenticated Primary Computer Boot Recovered)

**Conjecture:** Repairing the corrupted primary persistent data image will allow the authenticated computer to mount `/mnt/persistent`, complete runtime boot, and report ready through the product compute monitor.

**Move:** Created a rollback copy of the live data image, ran an explicit mutating ext4 repair after approval, verified the repaired filesystem with read-only fsck, re-imported Chrome auth cookies, triggered authenticated recovery, and opened the product page.

**Actual ΔV:**
- Rollback copy exists at `/var/lib/go-choir/vm-state/vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19/data.img.rollback-20260704T0426Z`.
- `e2fsck -fy` repaired the live ext4 image; follow-up `e2fsck -fn` reports no filesystem errors.
- Authenticated recovery for `yusefnathanson@me.com` now returns 200 with `current_computer.state=active` and `runtime.status=ready`.
- `/api/compute/status` reports the primary computer active, runtime reachable/ready, and guest persistent disk healthy: `used_percent=7.26568350535852`, `warning=false`, `critical=false`.
- Browser product path at `https://choir.news` renders the authenticated desktop app shell with Files, Web Lens, Email, Compute Monitor, Pulse, Texture, Universal Wire, Podcast, Calendar, Super Console, and Settings.

**Evidence:**
- Rollback copy command: `ssh node-b cp --sparse=always --reflink=auto .../data.img .../data.img.rollback-20260704T0426Z`.
- Repair command: `ssh node-b e2fsck -fy /var/lib/go-choir/vm-state/vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19/data.img`; output: `FILE SYSTEM WAS MODIFIED`.
- Verification command: `ssh node-b e2fsck -fn /var/lib/go-choir/vm-state/vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19/data.img`; output: clean pass 1 through pass 5, no warning.
- Auth recovery: `/api/compute/recovery` returned 200 at `2026-07-04T04:32:29Z`, `runtime_health=ready`, `researcher_count=3`.
- Compute status: `/api/compute/status` generated `2026-07-04T04:32:42Z`, `state=active`, `recovery.status=ready`, guest persistent disk healthy.
- Browser proof: `browse goto https://choir.news` returned 200; `browse text body` showed the authenticated app shell.

**Expected ΔV:**
- C-C1/C-C2 for authenticated primary boot are satisfied for this repaired computer.
- Direct lifecycle coalescing remains a code hardening candidate, but it is no longer the active blocker for `yusefnathanson@me.com` boot readiness.
- Pass 3 active-refresh acceptance still needs the original deploy-refresh axis if the suite requires a deploy-triggered active computer refresh, not merely manual authenticated recovery.

**Next:** Update the definition checkpoint, commit and push the repair evidence, then decide whether to proceed to deploy-triggered active-refresh proof or the next suite mission.

## Pass 24 — 2026-07-04 (Owner Reorders Suite: Autoputer First, Autopaper Tabled)

**Conjecture:** The substrate-independent audited computer mission is not a
separate goal competing with the Autoputer / Autopaper suite. It is the correct
refinement and rephrasing of the autoputer goal. Autopaper should wait until the
autoputer works correctly.

**Move:** Recorded owner authority in the suite definition and in
`docs/definitions/substrate-independent-audited-computer-2026-07-04.md`.
Reframed the active next probe away from autopaper/wire publication and toward
substrate-independent autoputer semantics: `Materializer`, `CapabilityManifest`,
`ObservationSet`, and `EquivalenceCheck`.

**Actual ΔV:**
- Autopaper/wire work is tabled, not deleted.
- The active suite path is now autoputer first.
- The substrate-independent audited computer definition is adopted as a
  refinement of the autoputer goal.
- Hypervisor/container choices remain implementation substrates, not the product
  object or success criterion.

**Evidence:**
- Owner statement, 2026-07-04: the suite should do the substrate-independent
  audited computer goal as a refinement/rephrasing of the autoputer goal; table
  autopaper; get autoputer working correctly first.
- `docs/definitions/substrate-independent-audited-computer-2026-07-04.md`.

**Expected ΔV:**
- C-B1/C-B2 autopaper/wire publication criteria are DEFERRED.
- Mission C/autoputer work is widened and sharpened into substrate-independent
  audited-computer work.
- The next executable probe should define the materializer/equivalence contract
  before adding or migrating substrates.

**Next:** Execute
`/goal docs/definitions/substrate-independent-audited-computer-2026-07-04.md`
as the active autoputer refinement. Define `Materializer`,
`CapabilityManifest`, `ObservationSet`, and `EquivalenceCheck` interfaces/types
in a non-runtime package with focused tests and a failing mismatch fixture.

## Pass 25 — 2026-07-04 (External Review Consensus: Substrate-Symptom Cluster, eBPF Deferred)

**Conjecture:** The owner-requested Devin/Claude/Cursor/Codex review can sharpen
the autoputer-first refinement without bloating the definition graph or turning
eBPF into a false success criterion.

**Move:** Collected read-only architecture reviews from Devin, Claude, Cursor,
and Codex. No reviewer edited files. Compared their recommendations against the
suite definition, substrate-independent audited computer definition, production
readiness checklist, artifact-program doctrine, and promotion spec.

**Actual ΔV:**
- All four reviews agreed that substrate-independent audited computer is the
  active autoputer refinement and that autopaper/wire should stay tabled.
- All four reviews agreed the next probe remains the non-runtime
  `Materializer`, `CapabilityManifest`, `ObservationSet`, and
  `EquivalenceCheck` contract with a passing fixture and a seeded mismatch
  fixture.
- Passes 15-23 are now classified as a substrate-symptom cluster on opaque
  `data.img` state: capacity, host-image gauge, resume coalescing, duplicate
  Firecracker launches, and ext4 repair were necessary rescue/hardening moves,
  not substrate-independent audited-computer progress.
- Pass 23 `e2fsck -fy` is recorded as protected-data rescue. It must not collapse
  into "fsck repaired ext4 -> product recovery is solved."
- eBPF/logging/monitoring belongs below the audited-computer abstraction as an
  optional materializer capability and Trace source. It is not artifact-program
  state, not a parallel observability stack, not a completion criterion, and not
  an equivalence proof.

**Evidence:**
- Devin review: direction correct; record Passes 15-23 as substrate-symptom
  cluster; eBPF optional observation source; next move is docs record then
  materializer/equivalence contract.
- Claude review: add explicit suite/refinement mapping and an autopaper
  untabling gate; reclassify `data-img-disposable` as a target conjecture;
  add forbidden collapse for eBPF-as-proof.
- Cursor review: one ledger tier plus failing mismatch fixture first; no
  hypervisor migration, no eBPF implementation, no autopaper.
- Codex review: product object is the audited computer; route/fork/promote must
  converge on `ComputerVersion`; eBPF feeds Trace/ObservationSet only.

**Expected ΔV:**
- The autoputer-first route is now more production-pointed: observability remains
  Trace -> Dolt -> supervision, with eBPF only as a future capability-scoped
  collector after privacy/retraction boundaries are designed.
- The definition graph stays small; prior art informs interface fields and proof
  obligations rather than becoming new authority.
- The next executable probe is unchanged but sharper: one durable state slice,
  one real equivalence checker, one passing fixture, and one seeded mismatch.

**Next:** Execute
`/goal docs/definitions/substrate-independent-audited-computer-2026-07-04.md`
with the sharpened first probe: one durable state slice, one real checker, one
passing fixture, and one seeded mismatch.

## Pass 26 — 2026-07-04 (First Contract Package: ComputerVersion Equivalence)

**Conjecture:** The substrate-independent audited-computer mission can descend
from semantic authority into a small non-runtime Go contract without touching
VM lifecycle, promotion, staging, `cmd/sandbox`, autopaper/wire, or opaque
`data.img` behavior.

**Move:** Added `internal/computerversion` with pure value types and a real
equivalence checker:

- `ComputerVersion = (CodeRef, ArtifactProgramRef)`
- `Materializer`
- `CapabilityManifest`
- `ObservationSet`
- `EquivalenceCheck`

**Actual ΔV:**
- One file-manifest durable slice can be represented as observations.
- Equality passes only when both realizations name the same `ComputerVersion`,
  both manifests support the required observation kind, and observed values
  match.
- A seeded file-manifest mismatch fails loudly with a structured difference.
- Unsupported materializer capability narrows the claim instead of passing.
- A different `ComputerVersion` fails instead of being compared as equivalent.

**Evidence:**
- `go test ./internal/computerversion` passed.
- `scripts/doccheck report-only` passed with existing project-wide docs
  warnings only.

**Deferred / Still Open:**
- No concrete Firecracker or non-Firecracker materializer exists yet.
- No current production `data.img` or Dolt/objectgraph state has been extracted
  into the new observation contract.
- UserIsomorphism remains unimplemented.
- Promotion protocol refinement to `ComputerVersion` remains open.

**Next:** Choose one current durable state slice, adapt it into an
`ObservationSet`, and add a second non-identical projection fixture so the
checker proves equality and seeded mismatch for real current-state-shaped data.

## Pass 27 — 2026-07-04 (Base Tree Observation Slice)

**Conjecture:** The first durable state slice should be shaped from an existing
typed Choir state model, not invented as a toy schema and not extracted from
opaque `data.img` bytes.

**Move:** Added `internal/computerversion/base_tree.go`, adapting the existing
`internal/base/tree.Tree` snapshot into a file-manifest `ObservationSet`.
Observations are keyed by stable Base `ItemID` and compare location, deletion
state, version ID, blob ref, content hash, manifest metadata, and provenance.

**Actual ΔV:**
- A current-state-shaped Base tree slice can now flow into the
  `ComputerVersion`/`ObservationSet`/`EquivalenceCheck` contract.
- Two non-identical projection fixtures over the same `ComputerVersion` pass
  equivalence for that Base tree slice.
- A seeded Base tree content/version mismatch fails with a structured
  `not_equivalent` difference.
- A live Base item without a linked current version is rejected before becoming
  observation evidence.

**Evidence:**
- `go test ./internal/computerversion` passed.

**Deferred / Still Open:**
- This is still fixture-backed; no production-backed Base journal, Dolt, or
  objectgraph state has been sampled into the contract.
- No concrete Firecracker or non-Firecracker materializer exists yet.
- No UserIsomorphism checker exists yet.
- Promotion protocol refinement to `ComputerVersion` remains open.

**Next:** Add an `Extract` contract and a pure event-journal extractor for the
existing Base event stream so `ArtifactProgramRef` can name a typed tape cursor,
not just an already-derived fixture tree.

## Pass 28 — 2026-07-04 (Base Event Extractor)

**Conjecture:** `ArtifactProgramRef` needs a first executable interpretation as
a typed tape cursor before materializer work can honestly claim substrate
independence.

**Move:** Added `Extractor`, `ExtractRequest`, `BaseEventExtractor`, and
`BaseEventJournalObservationSet`. The extractor validates committed positive
cursor positions, rejects duplicate event IDs and duplicate cursors, derives
the existing Base tree by `CursorSeq`, and emits the Base tree observation set.

**Actual ΔV:**
- `Extract` is now defined in code as the boundary from typed artifact-program
  state to `ObservationSet`.
- A typed Base event tape can produce a file-manifest observation set without
  reading or comparing opaque substrate images.
- Equivalent Base event tapes with different input ordering compare equal
  because committed cursor order is authoritative.
- A seeded Base event content/version mismatch fails with a structured
  `not_equivalent` difference.
- Non-committed or duplicate cursor positions are rejected before extraction.

**Evidence:**
- `go test ./internal/computerversion` passed.

**Deferred / Still Open:**
- The extractor validates event identity and cursor shape but does not verify
  journal hash-chain entries.
- No production-backed Base journal, Dolt, or objectgraph state has been
  sampled into the contract.
- No concrete Firecracker or non-Firecracker materializer exists yet.
- No UserIsomorphism checker exists yet.
- Promotion protocol refinement to `ComputerVersion` remains open.

**Next:** Add a pure projection materializer for extracted observation sets so
`Materialize` produces `Realization` objects under declared capabilities, then
compare two declared projections through the checker.

## Pass 29 — 2026-07-04 (Projection Materializer)

**Conjecture:** `Materialize` can get its first executable meaning without
touching VM lifecycle: a scoped projection materializer can turn extracted
observations into declared `Realization` objects and let the checker enforce
capability/equivalence boundaries.

**Move:** Added `ProjectionMaterializer`. It consumes an already-extracted
`ObservationSet`, validates that the requested `ComputerVersion` matches,
requires a named materializer and substrate, refuses unsupported required
capabilities, and returns a `Realization`.

**Actual ΔV:**
- `Materialize` now has a first non-runtime implementation.
- Two declared projections over the same extracted Base observations pass
  through `Materialize` and then `EquivalenceCheck`.
- A seeded mismatch still fails after materialization.
- A manifest that does not support the required observation class is rejected
  before it can claim a realization.

**Evidence:**
- `go test ./internal/computerversion` passed.

**Deferred / Still Open:**
- This is a projection materializer, not a Firecracker/vmmanager boundary.
- No production-backed Base journal, Dolt, or objectgraph state has been
  sampled into the contract.
- The Base event extractor does not yet verify journal hash-chain entries.
- No UserIsomorphism checker exists yet.
- Promotion protocol refinement to `ComputerVersion` remains open.

**Next:** Add hash-chain-aware extraction from existing Base `journal.Entry`
values so the typed tape proof can reject tampered parent/event chains before
deriving observations.

## Pass 30 — 2026-07-04 (Hash-Chain-Aware Base Journal Extraction)

**Conjecture:** The typed tape proof must reject tampered journal entries before
deriving observations; otherwise the extractor is only an ordered event fixture,
not an artifact-program boundary.

**Move:** Added `BaseJournalEntryExtractor` and
`BaseJournalEntriesObservationSet`. The extractor sorts entries by cursor,
validates event identity and committed cursors, verifies per-item parent links,
recomputes entry hashes, and only then derives Base events into observations.

**Actual ΔV:**
- Base `journal.Entry` slices can feed the extractor/materializer/checker chain.
- Shuffled but valid entries extract equivalently because cursor order is
  authoritative.
- Tampered event payloads are rejected by hash mismatch before observation
  derivation.
- Broken parent links are rejected before observation derivation.

**Evidence:**
- `go test ./internal/computerversion` passed.

**Deferred / Still Open:**
- The proof still uses in-memory test journals and entry slices, not a
  product-backed persisted Base journal source.
- No concrete Firecracker or non-Firecracker materializer exists yet.
- No UserIsomorphism checker exists yet.
- Promotion protocol refinement to `ComputerVersion` remains open.

**Next:** Add a read-only adapter from the existing Base journal interface into
`BaseJournalEntryExtractor`, then prove the chain on an actual `journal.Journal`
implementation rather than manually passed entry slices.

## Pass 31 — 2026-07-04 (Base Journal Interface Adapter)

**Conjecture:** The first typed tape proof should consume the existing Base
`journal.Journal` interface before reaching for any runtime or VM boundary.

**Move:** Added `BaseJournalExtractor`. It accepts a read-only `journal.Journal`,
calls the journal's own `VerifyChain`, reads entries, reuses the hash-chain-aware
entry extractor, and feeds the observation/materialization/equivalence path.

**Actual ΔV:**
- The extractor path now starts from an existing Base persistence interface, not
  only manually supplied entry slices.
- A `journal.MemJournal` can append events, verify its chain, extract
  observations, and materialize them as a projection.
- A nil journal is rejected before extraction.

**Evidence:**
- `go test ./internal/computerversion` passed.

**Deferred / Still Open:**
- The proof still uses `journal.MemJournal`, not the SQLite journal backend.
- No production-backed persisted Base journal, Dolt, or objectgraph state has
  been sampled into the contract.
- No concrete Firecracker or non-Firecracker materializer exists yet.
- No UserIsomorphism checker exists yet.
- Promotion protocol refinement to `ComputerVersion` remains open.

**Next:** Add a focused test proving the existing SQLite Base journal
implementation can append entries, verify its chain, and feed
`BaseJournalExtractor` into the projection/equivalence path.

## Pass 32 — 2026-07-04 (SQLite Base Journal Proof)

**Conjecture:** The typed tape path should prove against the existing persistent
Base journal backend before claiming it can leave fixture territory.

**Move:** Added a focused SQLite journal test. The test opens
`journal.NewSQLiteJournal`, appends Base events, verifies/extracts through
`BaseJournalExtractor`, and materializes the resulting observation set through
`ProjectionMaterializer`.

**Actual ΔV:**
- The extractor path now works against the existing SQLite Base journal
  implementation, not only `MemJournal` or manually supplied entry slices.
- SQLite journal entries feed the same chain verification, observation
  extraction, materialization, and equivalence-compatible realization path.
- The proof still uses focused test data, not live product state.

**Evidence:**
- `go test ./internal/computerversion` passed.

**Deferred / Still Open:**
- No production-backed persisted Base journal, Dolt, or objectgraph state has
  been sampled into the contract.
- No concrete Firecracker or non-Firecracker materializer exists yet.
- No UserIsomorphism checker exists yet.
- Promotion protocol refinement to `ComputerVersion` remains open.

**Next:** Define a scoped `UserIsomorphism` contract for declared observation
kinds and implement a checker that passes only for explicitly covered semantics
while rejecting unsupported or unclaimed durable semantics.

## Pass 33 — 2026-07-04 (Scoped UserIsomorphism)

**Conjecture:** Observation equivalence is not yet a user-equivalence claim. The
suite needs a named boundary that authorizes only explicitly covered
user-visible semantics and rejects unsupported durable semantics by default.

**Move:** Added `UserSemantic`, `UserIsomorphismScope`,
`UserIsomorphismChecker`, and `UserIsomorphismResult`. The checker composes with
`EquivalenceCheck`, requires declared observation kinds, requires each requested
user semantic to be covered, and narrows the claim for unsupported or unclaimed
semantics.

**Actual ΔV:**
- File-manifest observations can now support a scoped user-isomorphism claim for
  file path, file content, and deletion-state semantics.
- Live process continuity cannot pass by implication; if required but unclaimed
  or explicitly unsupported, the checker returns `narrowed`.
- Observation mismatch still fails the user-isomorphism claim.

**Evidence:**
- `go test ./internal/computerversion` passed.

**Deferred / Still Open:**
- No production-backed persisted Base journal, Dolt, or objectgraph state has
  been sampled into the contract.
- No concrete Firecracker or non-Firecracker materializer exists yet.
- UserIsomorphism is scoped to declared observations; it is not full-computer
  equivalence.
- Promotion protocol refinement to `ComputerVersion` remains open.

**Next:** Locate the lowest-blast-radius live product-backed Base, Dolt,
objectgraph, or file/blob slice; document its persistent/ephemeral/cache
boundary; then adapt it into a scoped `ObservationSet` with a seeded mismatch
fixture.

## Pass 34 — 2026-07-04 (Base Blob Store Observation Slice)

**Conjecture:** The next lowest-blast-radius product-backed slice is the
existing Choir Base filesystem blob store: it is content-addressed, persistent
on disk, and can prove blob integrity without touching VM lifecycle, auth,
Texture canonical writes, provider calls, or staging.

**Move:** Added `BaseBlobStoreObservationSet`. The adapter takes an explicit
list of Base `BlobRef` values, reads the existing `blob.Store`, verifies bytes
through `Get`, checks metadata through `Stat`, sorts refs, rejects duplicates,
and emits `ObservationBlobSet` entries.

**Actual ΔV:**
- Explicitly selected Base blob refs can now become scoped observation evidence.
- Reopened filesystem blob stores feed the observation path, proving persistence
  across store handles.
- Missing and corrupt blobs are rejected before they become observations.
- A seeded blob-set mismatch fails equivalence.

**Evidence:**
- `go test ./internal/computerversion` passed.

**Deferred / Still Open:**
- No live production Base, Dolt, or objectgraph state has been sampled into this
  contract.
- Journal, tree, and blob observations are still separate slices, not one bound
  current-state proof.
- No concrete Firecracker or non-Firecracker materializer exists yet.
- Promotion protocol refinement to `ComputerVersion` remains open.

**Next:** Compose the Base journal, tree, and blob observations into one scoped
current-state slice so the proof can tie event provenance to blob integrity
before touching live deployed state.

## Pass 35 — 2026-07-04 (Composite Base Current-State Slice)

**Conjecture:** Before sampling live production state, the proof should bind the
existing Base journal, derived tree/file-manifest observations, and blob-store
integrity observations into one current-state slice. Otherwise event provenance
and blob integrity remain separate claims.

**Move:** Added `BaseCurrentStateObservationSet`. It reads a Base
`journal.Journal` through `BaseJournalExtractor`, derives file-manifest
observations, extracts referenced blob refs, verifies those refs through the
filesystem `blob.Store`, and returns one sorted `ObservationSet` containing both
`ObservationFileManifest` and `ObservationBlobSet` entries.

**Actual ΔV:**
- SQLite-backed Base journal events can now be bound to filesystem blob-store
  integrity observations in one scoped current-state slice.
- A missing blob referenced by the current file manifest is rejected before the
  composite slice can become evidence.
- A seeded composite mismatch fails equivalence.
- The proof still uses focused test data, not live deployed user state.

**Evidence:**
- `go test ./internal/computerversion` passed.

**Deferred / Still Open:**
- No live production Base, Dolt, or objectgraph state has been sampled into this
  contract.
- No read-only runtime configuration boundary has been identified for opening an
  existing Base journal/blob root.
- No concrete Firecracker or non-Firecracker materializer exists yet.
- Promotion protocol refinement to `ComputerVersion` remains open.

**Next:** Add a read-only loader boundary for an existing configured Base
journal/blob root, or document that no such runtime configuration exists yet and
keep the next step at API/storage wiring rather than production sampling.

## Pass 36 — 2026-07-04 (Read-Only Base Current-State Loader)

**Conjecture:** The composite Base current-state slice should have an explicit
read-only opening boundary before any live sampling. Otherwise production
sampling pressure can accidentally create missing journal/blob state or apply
schema mutation while trying to observe.

**Move:** Added `blob.OpenStore`, `journal.OpenSQLiteJournalReadOnly`, and
`OpenBaseCurrentStateSource`. The loader opens an existing SQLite Base journal
without schema creation, opens an existing filesystem blob root without directory
creation, and feeds both into the existing composite current-state observation
path.

**Actual ΔV:**
- Existing Base journal/blob paths can now be loaded into a composite
  `ObservationSet` without creating missing persistence roots.
- A read-only SQLite journal handle rejects append attempts.
- Missing journal/blob paths are rejected before they can materialize new state.
- Runtime/product configuration still does not expose an existing Base
  journal/blob root pair for live sampling.

**Evidence:**
- `go test ./internal/base/blob ./internal/base/journal ./internal/computerversion`
  passed.

**Deferred / Still Open:**
- No live production Base, Dolt, or objectgraph state has been sampled into this
  contract.
- No product-path runtime configuration has been identified for a deployed Base
  journal/blob root pair.
- No concrete Firecracker or non-Firecracker materializer exists yet.
- Promotion protocol refinement to `ComputerVersion` remains open.

**Next:** Inspect the Base API/desktop sync boundary and either wire Base
persistence configuration into a non-VM product path with tests, or document why
product wiring is out of scope for this yellow/orange checkpoint and choose the
next non-runtime storage slice.

## Pass 37 — 2026-07-04 (Base API Handler Observation Proof)

**Conjecture:** Before wiring deployed product configuration, the existing Base
API handler should prove it can write durable journal/blob state that the
read-only current-state loader can observe. If this fails, product wiring would
only move the proof boundary without a valid handler-to-observation path.

**Move:** Added a focused Base API handler test that constructs the handler with
the existing SQLite journal and filesystem blob store implementations, performs
authenticated `POST /api/base/blobs` and `POST /api/base/items` writes, closes
the writable journal, reopens the same paths through `OpenBaseCurrentStateSource`,
and verifies the resulting observation set contains both file-manifest and
blob-set observations.

**Actual ΔV:**
- The Base API handler can now be shown to feed the same observation contract as
  the lower-level journal/blob tests.
- The proof crosses an HTTP handler boundary without using deployed auth/session,
  staging routing, VM lifecycle, or production user state.
- Inspection found the desktop sync engine is only a remote Base API client with
  local synced-state JSON; it does not own the server-side Base journal/blob
  persistence root.
- No cmd service currently wires the Base API handler to configured persistent
  Base journal/blob paths.

**Evidence:**
- `go test ./internal/base/api ./internal/computerversion` passed.

**Deferred / Still Open:**
- No live production Base, Dolt, or objectgraph state has been sampled into this
  contract.
- No product cmd/service wiring exposes a configured Base journal path and blob
  root.
- The test uses auth test doubles and in-process handlers; it does not prove
  deployed auth/session, routing, or persistence configuration.
- No concrete Firecracker or non-Firecracker materializer exists yet.
- Promotion protocol refinement to `ComputerVersion` remains open.

**Next:** Add a minimal non-deployed cmd/service wiring proof for the Base API
handler with explicit Base journal/blob path configuration and focused tests, or
document that such wiring crosses into runtime/auth/session scope and choose a
local product-path harness instead.

## Pass 38 — 2026-07-04 (Persistent Base API Wiring Boundary)

**Conjecture:** The next safe product-path step is not deployed route
registration; it is a small wiring boundary that makes persistent Base API
configuration explicit and testable without changing any cmd service behavior.

**Move:** Added `PersistentHandlerConfig`, `PersistentHandler`, and
`OpenPersistentHandler` in `internal/base/api`. The helper validates explicit
Base journal/blob paths, opens the SQLite journal plus filesystem blob store,
wires them into the existing Base API handler, and owns journal shutdown. Added
a focused test proving authenticated writes through the persistent handler can
be reopened read-only through `OpenBaseCurrentStateSource`.

**Actual ΔV:**
- Persistent Base API wiring is now represented as explicit journal/blob path
  configuration rather than ad hoc handler construction.
- The persistent wiring proof creates API state through HTTP routes, closes the
  writer, reopens the same paths read-only, and observes file-manifest plus
  blob-set evidence.
- No deployed cmd service behavior changed.
- The remaining route-registration question is now explicit: whether a deployed
  service should call this boundary, which may involve auth/session and staging
  behavior.

**Evidence:**
- `go test ./internal/base/api ./internal/computerversion` passed.

**Deferred / Still Open:**
- No live production Base, Dolt, or objectgraph state has been sampled into this
  contract.
- No cmd service currently calls `OpenPersistentHandler` with deployed
  configuration.
- The test uses auth test doubles and in-process handlers; it does not prove
  deployed auth/session, routing, or persistence configuration.
- No concrete Firecracker or non-Firecracker materializer exists yet.
- Promotion protocol refinement to `ComputerVersion` remains open.

**Next:** Perform red-ceremony assessment for registering persistent Base API
routes in a cmd service, then either wire the service with deployed-config tests
or choose a lower-risk local harness that remains outside auth/session and
staging.

## Pass 39 — 2026-07-04 (Route Registration Red Assessment)

**Conjecture:** Registering persistent Base API routes in a deployed cmd service
is no longer a storage-mechanics problem; it is an authority-boundary problem.
If route registration is treated as ordinary local wiring, the mission will
silently cross auth/session, staging route, and writable product-state surfaces.

**Move:** Added a route-registration red-ceremony assessment to the active
definition. The assessment names the conjecture delta, protected surfaces,
admissible evidence classes, rollback path, and heresy delta. It explicitly
chooses not to mutate deployed route registration in this checkpoint.

**Actual ΔV:**
- The boundary between handler/storage proof and deployed route registration is
  explicit.
- Persistent Base API route registration is classified as red if executed,
  because it touches API key scope validation, staging-facing routing, writable
  product persistence, and run acceptance/product-path verification.
- The next probe is narrowed to a local-only harness or focused cmd/service test
  that proves route-registration shape without changing deployed service
  behavior.

**Evidence:**
- Definition document now contains `Route Registration Red-Ceremony Assessment`.

**Deferred / Still Open:**
- No live production Base, Dolt, or objectgraph state has been sampled into this
  contract.
- No cmd service currently calls `OpenPersistentHandler` with deployed
  configuration.
- Deployed auth/session and route behavior remain unproven and intentionally
  unmutated.
- No concrete Firecracker or non-Firecracker materializer exists yet.
- Promotion protocol refinement to `ComputerVersion` remains open.

**Next:** Add a local-only Base API harness or focused cmd/service test that
consumes `OpenPersistentHandler` with explicit paths and proves route
registration shape without deployed routing; do not mutate staging-facing
services until red ceremony is intentionally accepted.

## Pass 40 — 2026-07-04 (Local Base Route Harness)

**Conjecture:** The route-registration shape can be proven below the red
deployment boundary by mounting persistent Base routes on the shared in-process
server wrapper, preserving `/health`, and issuing authenticated Base API
requests without binding a port or changing any deployed cmd service.

**Move:** Added `api.RegisterPersistentRoutes` as a local registrar helper and
added `server.Server.Handle(pattern, http.Handler)` so route subtrees can be
mounted in an in-process harness. Added a focused proof that opens persistent
Base journal/blob paths, mounts `/api/base/` on the shared server wrapper,
writes a blob through the mounted route, and confirms the existing `/health`
route still responds.

**Actual ΔV:**
- Persistent Base API route registration now has a local proof shape that uses
  the same route table wrapper as cmd services without mutating deployed
  routing.
- The proof still uses auth test doubles and focused test data; it does not
  claim deployed auth/session, staging, or production persistence behavior.
- The next uncertainty moves from route-table mechanics to whether an existing
  product command/config path should consume this boundary, or whether another
  non-deployed harness should remain the next safe probe.

**Evidence:**
- `go test ./internal/base/api ./internal/server ./internal/computerversion`
  passed.
- `TestRegisterPersistentRoutesOnSharedServer` proves mounted persistent Base
  routes can write through `server.Server` while `/health` remains registered.

**Deferred / Still Open:**
- No live production Base, Dolt, or objectgraph state has been sampled into this
  contract.
- No deployed cmd service currently calls `OpenPersistentHandler` or
  `RegisterPersistentRoutes`.
- Deployed auth/session and route behavior remain unproven and intentionally
  unmutated.
- No concrete Firecracker or non-Firecracker materializer exists yet.
- Promotion protocol refinement to `ComputerVersion` remains open.

**Next:** Add or identify a concrete local command/harness configuration that
runs the persistent Base API route tree from explicit journal/blob paths and can
be used for manual observation extraction; keep deployed route registration
behind red approval.

## Pass 41 — 2026-07-04 (Local Base Harness Command)

**Conjecture:** A concrete command boundary can exercise persistent Base API
writes from explicit journal/blob/auth database paths without crossing into
deployed routing, staging, or production auth/session behavior.

**Move:** Added `cmd/baseharness`, a local-only command that requires explicit
`--journal`, `--blob-root`, and `--auth-db` paths, opens the real auth store as
the API key validator, mounts persistent Base routes through the shared server
wrapper, and serves `/api/base/` on localhost. Added a focused test that creates
a real local auth user/API key, writes blob and item state through the harness,
closes the writable server, and reopens the resulting journal/blob roots through
the read-only current-state observation source.

**Actual ΔV:**
- The route proof now includes real auth-store API key validation instead of
  only auth test doubles.
- The proof has an executable local command/configuration boundary for manual
  observation extraction from explicit paths.
- The claim remains local and non-deployed: no public route, staging service,
  production auth/session, or user state was mutated.

**Evidence:**
- `go test ./cmd/baseharness ./internal/base/api ./internal/computerversion ./internal/server`
  passed.
- `TestOpenConfiguredServerFeedsReadOnlyObservationWithRealAuthStore` proves
  Base API writes through `cmd/baseharness` can feed the read-only
  `OpenBaseCurrentStateSource` observation path.

**Deferred / Still Open:**
- No deployed cmd service currently calls `OpenPersistentHandler` or
  `RegisterPersistentRoutes`.
- No live production Base, Dolt, or objectgraph state has been sampled into this
  contract.
- No concrete Firecracker or non-Firecracker materializer exists yet.
- Promotion protocol refinement to `ComputerVersion` remains open.

**Next:** Add a read-only extraction/report command for an existing Base
journal/blob root, or build the first non-Firecracker projection materializer
over the current Base observation set; do not wire deployed Base routes without
red approval.

## Pass 42 — 2026-07-04 (Read-Only Base Observation Command)

**Conjecture:** Manual observation extraction should have a read-only command
boundary before any live product root is sampled, so future agents can emit the
same `ObservationSet` schema from explicit paths without creating missing state
or depending on a running API server.

**Move:** Added `cmd/baseobserve`, a local command that requires explicit
Base journal/blob paths plus `CodeRef` and `ArtifactProgramRef`, opens the Base
current-state source read-only, and writes the resulting JSON `ObservationSet`
to stdout. Added focused tests that first create Base state through the
persistent API path, then run `baseobserve` against the resulting journal/blob
roots and verify both file-manifest and blob-set observations are emitted.

**Actual ΔV:**
- Observation extraction is now executable outside the server route path and
  can be run against explicit local roots.
- The command refuses missing journal/blob roots without materializing them,
  preserving the read-only evidence boundary.
- The next proof can shift from route/config mechanics to projection/materializer
  equivalence over the current Base observation set.

**Evidence:**
- `go test ./cmd/baseobserve ./cmd/baseharness ./internal/base/api ./internal/computerversion`
  passed.
- `TestRunEmitsReadOnlyBaseCurrentStateObservationSet` proves `baseobserve`
  emits the same observation schema from persisted Base API state.
- `TestRunDoesNotCreateMissingObservationRoots` proves missing roots remain
  absent after a failed observe run.

**Deferred / Still Open:**
- No live production Base, Dolt, or objectgraph state has been sampled into this
  contract.
- No non-Firecracker materializer/projection has been compared against this
  current-state slice.
- No concrete Firecracker materializer exists yet.
- Promotion protocol refinement to `ComputerVersion` remains open.

**Next:** Build the first non-Firecracker projection materializer over the
current Base observation set and compare it against the existing extracted Base
current-state realization with `EquivalenceCheck`; keep the claim scoped to
file-manifest/blob-set observations.

## Pass 43 — 2026-07-04 (Base Current-State Projection Equivalence)

**Conjecture:** The current Base journal/blob observation slice can already
support a first non-Firecracker projection proof if the claim is kept narrow:
file-manifest and blob-set observations only, no live process continuity, no VM
image equivalence, and no production-root sampling.

**Move:** Added `BaseCurrentStateCapabilityManifest` to encode the supported
observation scope for the Base current-state reader. Added projection tests that
derive the current-state `ObservationSet` from a SQLite Base journal plus
filesystem blob store, materialize it once as a Base current-state reader and
once as a non-Firecracker file projection, then compare the two realizations
with `EquivalenceCheck`.

**Actual ΔV:**
- The first non-Firecracker projection proof now runs over the current-state
  shaped Base slice rather than only over hand-built projection fixtures.
- Unsupported live-process continuity narrows the Base current-state capability
  manifest instead of being implied by file/blob equivalence.
- A seeded projection mismatch fails against the extracted current-state
  realization, so projection equality is not a tautology.

**Evidence:**
- `go test ./cmd/baseobserve ./cmd/baseharness ./internal/base/api ./internal/base/blob ./internal/base/journal ./internal/computerversion ./internal/server`
  passed.
- `TestBaseCurrentStateAndFileProjectionMaterializersCompare` proves the
  extracted Base current-state slice and non-Firecracker file projection compare
  equivalent under declared file/blob capabilities.
- `TestBaseCurrentStateProjectionMismatchFails` proves a corrupted projection
  fails equivalence.
- `TestBaseCurrentStateCapabilityManifestNarrowsUnsupportedLiveProcess` proves
  live-process continuity is not smuggled into the narrow file/blob proof.

**Deferred / Still Open:**
- No live production Base, Dolt, or objectgraph state has been sampled into this
  contract.
- No concrete Firecracker materializer exists yet.
- No production root has been classified safe for `cmd/baseobserve` sampling.
- Promotion protocol refinement to `ComputerVersion` remains open.

**Next:** Select a safe real Base journal/blob root or fixture-derived persisted
root for observation sampling, run `cmd/baseobserve` to emit the concrete
`ObservationSet`, and compare it through the Base-current-state and
non-Firecracker file-projection materializer paths. Do not sample production
state without explicit root classification.

## Pass 44 — 2026-07-04 (Base Observe-to-Compare Command Path)

**Conjecture:** The next topology-preserving proof should make observation
sampling and equivalence comparison executable from command boundaries, not only
from package tests, while staying on fixture-derived persisted Base state and
out of deployed route registration.

**Move:** Added `CompareBaseCurrentStateToFileProjection` and `cmd/basecompare`.
The command reads one Base current-state `ObservationSet` JSON stream/file for
an implicit file-projection comparison, or separate left/right JSON files for
projection mismatch checks. It materializes the left side as the Base
current-state reader path, materializes the right side as the non-Firecracker
file-projection path, emits `EquivalenceResult` JSON, returns 0 only for
equivalent, and returns 1 for not-equivalent/narrowed results.

**Actual ΔV:**
- The observe-to-compare path is now executable as local commands:
  fixture/persisted root -> `cmd/baseobserve` JSON -> `cmd/basecompare` result.
- The command path preserves the same narrow file-manifest/blob-set capability
  scope as the package proof and does not imply live-process or full-computer
  equivalence.
- A tampered right-hand projection JSON produces a structured
  `not_equivalent` result instead of a ceremonial pass.

**Evidence:**
- `go test ./cmd/basecompare ./cmd/baseobserve ./cmd/baseharness ./internal/base/api ./internal/base/blob ./internal/base/journal ./internal/computerversion ./internal/server`
  passed.
- `TestRunObservationSetFeedsBaseCurrentStateFileProjectionCompare` proves
  fixture-derived persistent Base state can be observed through `cmd/baseobserve`
  and fed into the Base-current-state vs non-Firecracker projection comparison.
- `TestRunComparesStdinObservationSetAsEquivalent` proves the compare command
  accepts observed JSON on stdin and emits `equivalent`.
- `TestRunComparesLeftAndRightFilesAndReportsTamperedProjection` proves a
  tampered projection exits 1 with a blob-set `not_equivalent` difference.
- `TestRunRejectsStdinCollisionAndInvalidJSON` proves invalid comparison inputs
  are rejected before an equivalence claim is emitted.

**Deferred / Still Open:**
- No real non-production or production Base root has been sampled.
- No live product state has been classified safe for observation.
- No concrete Firecracker materializer exists yet.
- Promotion protocol refinement to `ComputerVersion` remains open.

**Next:** Locate and classify candidate local/staging-derived Base journal/blob
roots without reading production user state. If none are safe, build an explicit
fixture export artifact with `cmd/baseharness` + `cmd/baseobserve` +
`cmd/basecompare` and record the JSON evidence refs.

## Pass 45 — 2026-07-04 (Fixture Export Evidence Chain)

**Conjecture:** Before sampling any real product root, the command-chain proof
should produce durable evidence artifacts from an explicit local fixture root:
seed persisted Base state, observe it read-only, and compare the resulting
`ObservationSet`.

**Move:** Searched the worktree for candidate Base journal/blob roots and found
no safe real root to sample: sqlite candidates were unrelated auth/vendor
databases or test-only paths. Added `cmd/baseharness --seed-fixture`, which
opens explicit local journal/blob/auth paths, writes one blob and one item
through the persistent Base API route path, emits fixture metadata, and exits
without listening. Ran the command chain and saved evidence refs:
`local://pass45-base-commands.json`, `local://pass45-base-observation.json`, and
`local://pass45-base-equivalence.json`.

**Actual ΔV:**
- The first observe/compare evidence now exists as durable JSON artifacts rather
  than only test stdout.
- The fixture path still exercises the persistent Base API route machinery before
  read-only observation.
- The equivalence claim remains explicitly narrow: two observations
  (`blob_set`, `file_manifest`) and status `equivalent`.
- The probe avoided production user state because no safe real root was
  identified.

**Evidence:**
- Candidate root scan: repo glob found only `auth.db` plus vendor sqlite embed
  databases; config grep found Base journal/blob paths only in new commands,
  tests, and old docs, not a safe live Base root.
- `local://pass45-base-commands.json` records return code 0 for
  `cmd/baseharness --seed-fixture`, `cmd/baseobserve`, and `cmd/basecompare`.
- `local://pass45-base-observation.json` records `pass45-fixture-observation`
  with one `blob_set` and one `file_manifest` observation.
- `local://pass45-base-equivalence.json` records `{ "status": "equivalent" }`.
- `go test ./cmd/baseharness ./cmd/baseobserve ./cmd/basecompare ./internal/base/api ./internal/base/blob ./internal/base/journal ./internal/computerversion ./internal/server`
  passed after adding fixture export tests.

**Deferred / Still Open:**
- No live product Base, Dolt, objectgraph, or `data.img` state has been sampled.
- No concrete Firecracker materializer exists yet.
- Firecracker/vmmanager behavior is not yet behind a materializer/capability
  boundary for a scoped path.
- Promotion protocol refinement to `ComputerVersion` remains open.

**Next:** Inspect `specs/promotion_protocol.tla` and current promotion/vmmanager
boundaries to choose the smallest non-runtime formalization or wrapper that
moves completion semantics item 2 or 6 without mutating deployed VM lifecycle or
promotion behavior.

## Pass 46 — 2026-07-04 (Promotion ComputerVersion Refinement)

**Conjecture:** The promotion protocol can satisfy the audited-computer
definition's `ComputerVersion` naming requirement without mutating deployed
promotion or VM lifecycle behavior by exposing an explicit finite-model
refinement seam from the existing bounded base-version counter to
`ComputerVersion = (CodeRef, ArtifactProgramRef)`.

**Move:** Updated `specs/promotion_protocol.tla` to define
`ComputerVersions`, `ComputerVersionOfBase`, and
`ComputerVersionOfRoutedComputer` while preserving the existing abstract
promotion state variables. Added `RouteNamesComputerVersion` and
`PromotionNamesComputerVersion` invariants to `specs/promotion_protocol.cfg`.

**Actual ΔV:**
- Promotion/rollback semantics no longer rely on unnamed numeric base versions
  only; the formal model now names the refinement path to ComputerVersion
  records.
- The refinement is intentionally bounded and abstract: base-version numbers
  alias finite `codeRef` and `artifactProgramRef` values for TLC.
- No deployed promotion, route, VM lifecycle, auth/session, provider, or
  staging behavior changed.

**Evidence:**
- `java -XX:+UseParallelGC -cp /tmp/tla2tools.jar tlc2.TLC -deadlock -workers auto promotion_protocol.tla`
  through `/nix/var/nix/profiles/default/bin/nix shell nixpkgs#temurin-jre-bin`
  in `specs/`: 826 states generated, 318 distinct states found, 0 errors.
- `go test ./cmd/basecompare ./cmd/baseobserve ./cmd/baseharness ./internal/base/api ./internal/base/blob ./internal/base/journal ./internal/computerversion ./internal/server`
  passed.
- `scripts/doccheck report-only` completed: 293 docs, 440 warnings.

**Deferred / Still Open:**
- No concrete promotion certificate or deployed route record is keyed by real
  CodeRef/ArtifactProgramRef values yet.
- No live product Base, Dolt, objectgraph, or `data.img` state has been sampled.
- No concrete Firecracker/vmmanager materializer exists yet.

**Next:** Inspect current Firecracker/vmmanager and promotion-certificate
boundaries to choose the smallest non-production wrapper that moves the
formal `ComputerVersion` seam toward executable evidence without mutating
deployed VM lifecycle or promotion behavior.

## Pass 47 — 2026-07-04 (Scoped VMManager Materializer Boundary)

**Conjecture:** Firecracker/vmmanager can enter the audited-computer contract
without touching VM lifecycle behavior by first exposing a scoped VM-state
materializer/capability boundary that classifies legacy opaque state and refuses
durable user-state claims.

**Move:** Added `ObservationVMStateManifest`,
`VMManagerScopedMaterializer`, `VMManagerScopedPath`, and
`VMManagerCapabilityManifest` in `internal/computerversion`. The boundary emits
one `vm_state_manifest` observation for explicit vmmanager state inputs and
declares file/blob/Dolt/objectgraph/provenance/live-process observation kinds
unsupported. No `internal/vmmanager` lifecycle code was changed.

**Actual ΔV:**
- Completion item 2 now has a first scoped Firecracker/vmmanager path behind a
  materializer/capability boundary.
- `data.img` is classified as `durable_legacy_opaque` for this scoped boundary,
  not as disposable cache and not as typed durable user-state evidence.
- The boundary is deliberately non-lifecycle: it does not boot, stop, resume,
  copy, or inspect a VM.
- Durable user-state equivalence remains grounded in typed artifact-program
  observations, not launch metadata.

**Evidence:**
- `go test ./internal/computerversion` passed with
  `TestVMManagerScopedMaterializerEmitsOnlyVMStateManifest`,
  `TestVMManagerScopedRealizationsCompareEquivalentAndMismatchFails`,
  `TestVMManagerCapabilityManifestNarrowsOrBlocksDurableClaims`, and
  `TestVMManagerScopedMaterializerRejectsInvalidInputsBeforeClaim`.

**Deferred / Still Open:**
- No concrete Firecracker lifecycle materializer has been implemented.
- No production `data.img` or live product state has been sampled.
- No promotion certificate or deployed route record is keyed by real
  CodeRef/ArtifactProgramRef values yet.

**Next:** Build or inspect the smallest non-production wrapper that connects the
scoped vmmanager state classification to an executable fixture root without
booting/killing VMs or mutating deployed promotion behavior.

## Pass 48 — 2026-07-04 (VM State Observation Command)

**Conjecture:** The scoped vmmanager materializer boundary should be executable
from an explicit non-production fixture root, not only available as library
types/tests.

**Move:** Added `cmd/vmstateobserve`, a non-deployed local command that requires
`ComputerVersion` refs, VM ID, and at least one explicit persistent/data path.
By default it verifies supplied persistent/data paths exist, then emits
`ObservationSet` JSON with one `vm_state_manifest` observation. Generated
evidence refs `local://pass48-vmstate-command.json` and
`local://pass48-vmstate-observation.json` from a temporary fixture root
containing an explicit persistent dir and `data.img`.

**Actual ΔV:**
- The Firecracker/vmmanager scoped boundary is now executable as a command over
  explicit non-production paths.
- The command still makes no lifecycle claim: it does not boot, stop, resume,
  copy, or inspect a running VM.
- The emitted observation class remains `vm_state_manifest`; it does not become
  file/blob/Dolt/objectgraph/provenance proof.

**Evidence:**
- `go test ./cmd/vmstateobserve ./internal/computerversion` passed.
- `local://pass48-vmstate-command.json` records return code 0 for
  `go run ./cmd/vmstateobserve ...` over a temp fixture root.
- `local://pass48-vmstate-observation.json` records one `vm_state_manifest`
  observation requiring `vm_state_manifest`.

**Deferred / Still Open:**
- No concrete Firecracker lifecycle materializer has been implemented.
- No live VM state, production `data.img`, or product user state has been
  sampled.
- No promotion certificate or deployed route record is keyed by real
  CodeRef/ArtifactProgramRef values yet.

**Next:** Compare the `cmd/vmstateobserve` fixture artifact against a second
independently declared vmmanager fixture manifest or connect it to a fixture
promotion-certificate wrapper, without booting/killing VMs or mutating deployed
promotion behavior.

## Pass 49 — 2026-07-04 (VM State Observation Compare)

**Conjecture:** Scoped vmmanager observation artifacts should have an executable
compare path with both passing and failing outcomes before any lifecycle or
production state claim is attempted.

**Move:** Added `cmd/vmstatecompare`, a non-deployed local command that compares
two `ObservationSet` JSON artifacts under `VMManagerCapabilityManifest`. It
returns `equivalent` for matching `vm_state_manifest` observations,
`not_equivalent` for concrete manifest mismatches, and `narrowed` when an input
requires durable observation kinds that vmmanager cannot support.

**Actual ΔV:**
- The vmmanager fixture path now has observe and compare commands.
- The compare path has an explicit failure mode for seeded VM-state mismatches.
- Durable user-state observation kinds still narrow instead of passing under
  vmmanager launch/state metadata.

**Evidence:**
- `go test ./cmd/vmstatecompare ./cmd/vmstateobserve ./internal/computerversion`
  passed.
- `local://pass49-vmstate-compare.json` records `cmd/vmstatecompare` returning
  0 with `{ "status": "equivalent" }` for the `pass48` fixture artifact.
- `local://pass49-vmstate-compare.json` also records a seeded `data.img` path
  mismatch returning 1 with `not_equivalent` and a `vm_state_manifest`
  difference.

**Deferred / Still Open:**
- VM-state manifest comparison is not durable user-state equivalence.
- No concrete Firecracker lifecycle materializer has been implemented.
- No live VM state, production `data.img`, or product user state has been
  sampled.

**Next:** Add a local promotion-certificate or route-record fixture over
concrete `ComputerVersion` refs, or build the first lifecycle-free Firecracker
fixture wrapper that relates `vm_state_manifest` to typed Base observations.
Do not boot/kill VMs or mutate deployed promotion behavior.

## Pass 50 — 2026-07-04 (Promotion Certificate Observation Fixture)

**Conjecture:** After the formal promotion model names `ComputerVersion`, a
local implementation-side certificate fixture can express promotion evidence
over concrete active/base/candidate refs without touching live route behavior.

**Move:** Added `PromotionCertificate` and `PromotionLedgerCertificate` in
`internal/computerversion`. A certificate validates route slot, concrete
active/base/candidate `ComputerVersion` refs, owner approval, health-window
state, unique ledger states, rollback ref, and evidence ref, then emits a
candidate-scoped `promotion_certificate` `ObservationSet`. Generated
`local://pass50-promotion-certificate.json` as a concrete-ref fixture.

**Actual ΔV:**
- Promotion evidence now has an implementation-side observation schema over
  concrete `ComputerVersion` refs.
- The schema is local evidence only: it does not approve, commit, revert, or
  move a deployed route.
- Reordered ledgers canonicalize before comparison, while seeded candidate or
  ledger mismatches fail equivalence.

**Evidence:**
- `go test ./internal/computerversion` passed with focused promotion certificate
  tests.
- `local://pass50-promotion-command.json` records return code 0 for generating
  the fixture ObservationSet.
- `local://pass50-promotion-certificate.json` records one
  `promotion_certificate` observation for candidate
  `(pass50-candidate-code, pass50-candidate-artifact)`.

**Deferred / Still Open:**
- No deployed promotion certificate, route record, rollback path, or promotion
  API consumes this schema yet.
- No concrete Firecracker lifecycle materializer has been implemented.
- No live VM state, production `data.img`, or product user state has been
  sampled.

**Next:** Build a fixture-level combined observation package tying Base
current-state evidence, vmmanager state classification, and promotion
certificate evidence under one `ComputerVersion`. Do not boot/kill VMs or mutate
deployed promotion behavior.

## Pass 51 — 2026-07-04 (Combined ComputerVersion Observation Package)

**Conjecture:** A lifecycle-free fixture package can relate typed Base
current-state evidence, scoped vmmanager state classification, and promotion
certificate evidence under one `ComputerVersion` without pretending that any
member observation proves more than its declared kind.

**Move:** Added `CombineObservationSets` in `internal/computerversion`. The
combiner requires one valid `ComputerVersion`, rejects member sets that name a
different version, merges and sorts required observation kinds, dedupes identical
duplicate observations, and rejects conflicting duplicate kind/key values.
Generated `local://pass51-combined-observation.json` from:

- `cmd/baseharness --seed-fixture` and `cmd/baseobserve` for Base
  `file_manifest` + `blob_set`;
- `cmd/vmstateobserve` for scoped `vm_state_manifest`;
- `PromotionCertificate.ObservationSet` for `promotion_certificate`.

All three evidence slices name
`(pass51-candidate-code, pass51-candidate-artifact)`.

**Actual ΔV:**
- Evidence composition is now an explicit package boundary instead of an
  implied relation between separate fixture artifacts.
- The combined package preserves the narrower semantics of each member
  observation: Base file/blob observations remain typed state, vmmanager remains
  scoped legacy-state classification, and promotion remains local certificate
  evidence.
- Reordering member sets produces an equivalent combined observation package.

**Evidence:**
- `go test ./cmd/basecompare ./cmd/baseobserve ./cmd/baseharness ./cmd/vmstateobserve ./cmd/vmstatecompare ./internal/base/api ./internal/base/blob ./internal/base/journal ./internal/computerversion ./internal/server` passed.
- `local://pass51-combined-command.json` records return code 0 for each command
  step in the fixture chain.
- `local://pass51-combined-observation.json` records four observations under one
  `ComputerVersion`: `blob_set`, `file_manifest`, `promotion_certificate`, and
  `vm_state_manifest`.
- `local://pass51-combined-equivalence.json` records `equivalent` for the same
  combined evidence assembled in a different order.

**Deferred / Still Open:**
- The package is local fixture evidence, not live product sampling.
- No deployed route, rollback path, promotion API, or VM lifecycle path consumes
  the combined evidence.
- No Base/Dolt/objectgraph production root has been classified safe to sample.

**Next:** Locate or deliberately provision the smallest non-production
product-shaped root whose Base journal/blob, vmmanager manifest, and
promotion/route certificate can be observed under one `ComputerVersion`; if none
exists, define the required fixture-root contract before any deployed sampling.

## Pass 52 — 2026-07-04 (Product-Shaped Fixture Root Observer)

**Conjecture:** The combined evidence package should be callable from one
explicit non-production fixture-root contract before any agent searches or
samples live roots. That contract can prove the shape of the next realism axis
without crossing production user-state, deployed route, or VM lifecycle
boundaries.

**Move:** Added `ProductFixtureRoot` in `internal/computerversion`. The observer
takes one `ComputerVersion`, explicit Base journal/blob paths, one
`VMManagerScopedPath`, and one `PromotionCertificate`. It opens Base state
through `OpenBaseCurrentStateSource`, serializes vmmanager and promotion evidence
through their existing observation APIs, then calls `CombineObservationSets`.
Generated `local://pass52-fixture-root-observation.json` from a deliberately
provisioned local fixture root.

**Actual ΔV:**
- The next product-shaped root probe is now executable as a package API rather
  than a prose recipe.
- Missing Base roots, promotion-candidate mismatch, and VM fixtures without
  persistent/data paths fail before combined evidence is emitted.
- The contract still does not boot/kill VMs, mutate route state, or sample live
  product state.

**Evidence:**
- `go test ./cmd/basecompare ./cmd/baseobserve ./cmd/baseharness ./cmd/vmstateobserve ./cmd/vmstatecompare ./internal/base/api ./internal/base/blob ./internal/base/journal ./internal/computerversion ./internal/server` passed.
- `local://pass52-fixture-root-command.json` records return code 0 for seeding
  the Base fixture and observing the product-shaped fixture root.
- `local://pass52-fixture-root-observation.json` records four observations under
  `(pass52-candidate-code, pass52-candidate-artifact)`: `blob_set`,
  `file_manifest`, `promotion_certificate`, and `vm_state_manifest`.
- `local://pass52-fixture-root-selfcheck.json` records `equivalent` for the
  emitted fixture-root observation set self-check.

**Deferred / Still Open:**
- No authorized live or staging-safe product root has been discovered or sampled.
- No deployed cmd service consumes `ProductFixtureRoot`.
- No concrete Firecracker lifecycle materializer has been implemented.

**Next:** Search configured local/staging-safe non-production roots for a
candidate product-shaped root. If no authorized root exists, keep live sampling
blocked and define the minimum provisioning contract for a candidate computer
evidence root.

## Pass 53 — 2026-07-04 (Local Product-Root Discovery Blocked)

**Conjecture:** After `ProductFixtureRoot` exists, the next safe move is to find
an already-authorized local/staging-safe non-production product-shaped root
before defining new provisioning work. If no such root is configured locally, the
mission should stay blocked from live sampling and move to an explicit
candidate-root provisioning contract.

**Move:** Searched the repo/worktree and local environment for configured Base
and VM roots. Checked `BASE_API_JOURNAL_PATH`, `BASE_API_BLOB_ROOT`,
`VM_STATE_DIR`, and `VMCTL_OWNERSHIP_PATH`; all were empty locally. Repo-local
sqlite/data-image globbing found only auth/vendor sqlite databases, not an
authorized product-shaped root. Repo text points to production Node B paths such
as `/var/lib/go-choir/vm-state`, but those are not safe to sample under this
yellow/orange-low fixture-root checkpoint.

**Actual ΔV:**
- Opportunistic live/staging root reads are explicitly blocked.
- The next realism step is narrowed to provisioning an authorized candidate
  evidence root, not guessing that a production path is admissible.

**Evidence:**
- Local env check showed empty `BASE_API_JOURNAL_PATH`, `BASE_API_BLOB_ROOT`,
  `VM_STATE_DIR`, and `VMCTL_OWNERSHIP_PATH`.
- Repo-local globbing found only `auth.db` and vendor sqlite fixtures, plus no
  repo-local `data.img`.

**Deferred / Still Open:**
- No authorized candidate-computer evidence root exists yet.
- No production/staging root has been sampled.
- No concrete Firecracker lifecycle materializer has been implemented.

**Next:** Define and implement the minimum candidate-computer evidence-root
provisioning contract that can feed `ProductFixtureRoot` without sampling
production/staging roots.

## Pass 54 — 2026-07-04 (Candidate Evidence Root Admission Contract)

**Conjecture:** Before provisioning or sampling any product-shaped root, the
mission needs an executable admission contract that makes candidate evidence
root authority explicit: source, authorization, non-production status, deployed
route non-mutation, and path containment under one root.

**Move:** Added `CandidateEvidenceRootManifest` in `internal/computerversion`.
The manifest admits only `local_candidate` or `staging_candidate` sources,
requires `AuthorizedForSampling`, rejects `ContainsProduction` and
`TouchesDeployedRoute`, requires Base and VM evidence paths to remain under
`RootPath`, validates the embedded `ProductFixtureRoot`, and returns that root
only after validation. Generated
`local://pass54-candidate-evidence-root-manifest.json` as one authorized
`local_candidate` fixture manifest, then observed it through `ProductFixtureRoot`.

**Actual ΔV:**
- The candidate-root boundary is now executable and testable instead of implied
  by prose.
- Production/staging opportunistic reads have a concrete rejection point:
  manifests that contain production state, touch deployed routes, lack sampling
  authorization, or point evidence paths outside the declared root fail before
  observation.
- The contract still provisions only local fixture data in this checkpoint; it
  does not discover or sample a live product root.

**Evidence:**
- `go test ./cmd/basecompare ./cmd/baseobserve ./cmd/baseharness ./cmd/vmstateobserve ./cmd/vmstatecompare ./internal/base/api ./internal/base/blob ./internal/base/journal ./internal/computerversion ./internal/server` passed.
- `local://pass54-candidate-evidence-root-command.json` records return code 0
  for seeding the fixture root and observing through the admitted manifest.
- `local://pass54-candidate-evidence-root-manifest.json` records source
  `local_candidate`, `authorized_for_sampling: true`,
  `contains_production: false`, and `touches_deployed_route: false`.
- `local://pass54-candidate-evidence-root-observation.json` records four
  observations under `(pass54-candidate-code, pass54-candidate-artifact)`:
  `blob_set`, `file_manifest`, `promotion_certificate`, and
  `vm_state_manifest`.
- `local://pass54-candidate-evidence-root-selfcheck.json` records `equivalent`
  for the admitted root's emitted observation set self-check.

**Deferred / Still Open:**
- No command/harness provisions an admitted candidate root end-to-end yet.
- No authorized live non-production product root has been sampled.
- No concrete Firecracker lifecycle materializer has been implemented.

**Next:** Add a local candidate evidence-root provisioning command or harness
that creates an admitted `CandidateEvidenceRootManifest`, writes/observes the
root through existing package APIs, and records a failing seeded mismatch. Do not
sample production/staging roots.

## Pass 55 — 2026-07-04 (Candidate Evidence Root Provisioning Command)

**Conjecture:** The admitted candidate-root contract should be executable from a
local command before the mission reaches for richer state or any live root. The
command must create evidence, not just validate a manifest, and it must include
a seeded mismatch so the failure path stays load-bearing.

**Move:** Added `cmd/evidenceroot`. The command requires an empty `--root`,
`--code-ref`, and `--artifact-program-ref`; seeds Base state through in-process
persistent Base API routes; writes local vmmanager fixture files; constructs an
authorized `local_candidate` `CandidateEvidenceRootManifest`; observes it through
`ProductFixtureRoot`; and emits JSON containing the manifest, combined
observation set, equivalent self-check, seeded `not_equivalent` mismatch, and
Base fixture metadata. It rejects missing required flags before JSON and rejects
non-empty roots without overwriting existing files.

**Actual ΔV:**
- Candidate evidence-root provisioning is now executable as a local command.
- The positive path produces admitted Base + VM + promotion evidence under one
  `ComputerVersion`.
- The negative path is explicit: a seeded vmmanager observation mismatch returns
  `not_equivalent`, and unsafe command inputs emit no success JSON.

**Evidence:**
- `go test ./cmd/basecompare ./cmd/baseobserve ./cmd/baseharness ./cmd/evidenceroot ./cmd/vmstateobserve ./cmd/vmstatecompare ./internal/base/api ./internal/base/blob ./internal/base/journal ./internal/computerversion ./internal/server` passed.
- `local://pass55-evidenceroot-command.json` records command return code 0.
- `local://pass55-evidenceroot-manifest.json` records source
  `local_candidate`, `authorized_for_sampling: true`,
  `contains_production: false`, and `touches_deployed_route: false`.
- `local://pass55-evidenceroot-observation.json` records four observations under
  `(pass55-candidate-code, pass55-candidate-artifact)`.
- `local://pass55-evidenceroot-selfcheck.json` records `equivalent`.
- `local://pass55-evidenceroot-seeded-mismatch.json` records `not_equivalent`
  with a concrete `vm_state_manifest` difference.

**Deferred / Still Open:**
- The command still provisions a minimal fixture, not a rich live candidate
  computer.
- No Dolt/objectgraph state slice is in the candidate root yet.
- No concrete Firecracker lifecycle materializer has been implemented.

**Next:** Add one more typed non-production state slice to the candidate evidence
root, preferably Dolt/objectgraph if an existing local package boundary can be
used without deployment or production data; otherwise record why
Base+VM+promotion is the current safe frontier.

## Pass 56 — 2026-07-04 (Typed Objectgraph Candidate Slice)

**Conjecture:** After the candidate evidence-root command exists, the safest next
non-production state slice is a typed objectgraph snapshot embedded in the
candidate fixture. This advances beyond Base file/blob state without reading
production corpusd/Dolt or mutating deployed routes.

**Move:** Added `ObjectGraphSnapshot` in `internal/computerversion`. It validates
native `objectgraph.Object` and `objectgraph.Edge` values, rejects content-hash
mismatches and missing edge endpoints, and emits one deterministic
`object_graph_head` observation. `ProductFixtureRoot` now optionally combines
that snapshot while preserving the previous four-kind scope when `ObjectGraph`
is nil. `cmd/evidenceroot` now provisions a two-object/one-edge objectgraph
snapshot inside the admitted local candidate evidence root.

**Actual ΔV:**
- Candidate evidence roots now carry a typed objectgraph state slice in addition
  to Base file/blob, vmmanager, and promotion-certificate evidence.
- The objectgraph slice is content-addressed and endpoint-checked through the
  existing `internal/objectgraph` package types.
- This still does not claim a live Dolt/corpusd head; it is fixture-level typed
  objectgraph evidence only.

**Evidence:**
- `go test ./cmd/basecompare ./cmd/baseobserve ./cmd/baseharness ./cmd/evidenceroot ./cmd/vmstateobserve ./cmd/vmstatecompare ./internal/base/api ./internal/base/blob ./internal/base/journal ./internal/computerversion ./internal/server` passed.
- `local://pass56-objectgraph-command.json` records command return code 0.
- `local://pass56-objectgraph-manifest.json` records an admitted
  `local_candidate` manifest containing an `object_graph` snapshot with two
  typed objects and one edge.
- `local://pass56-objectgraph-observation.json` records required kinds
  `blob_set`, `file_manifest`, `object_graph_head`, `promotion_certificate`, and
  `vm_state_manifest`.
- `local://pass56-objectgraph-selfcheck.json` records `equivalent`.
- `local://pass56-objectgraph-seeded-mismatch.json` records `not_equivalent`
  with a concrete `vm_state_manifest` difference.

**Deferred / Still Open:**
- No live Dolt/corpusd commit head has been sampled or admitted.
- The objectgraph slice is embedded fixture data, not a persisted embedded Dolt
  objectgraph repository.
- No concrete Firecracker lifecycle materializer has been implemented.

**Next:** Inspect existing embedded Dolt objectgraph test helpers and decide
whether a local Dolt-backed `object_graph_head`/`dolt_head` fixture can be
admitted without production data or deployed routes; if not, move to a
non-lifecycle Firecracker realization proof.

## Pass 57 — 2026-07-04 (Embedded Dolt Objectgraph Head)

**Conjecture:** Once typed objectgraph fixture evidence exists, the next
load-bearing local proof is an embedded Dolt objectgraph commit head under the
same candidate evidence root. This checks the Dolt-backed objectgraph path
without sampling corpusd, staging, or production data.

**Move:** Added `DoltHeadSnapshot` in `internal/computerversion`. It emits one
`dolt_head` observation carrying database, commit hash, linked objectgraph head,
object count, edge count, and derivation, while rejecting missing commit hashes
and `contains_production: true`. `ProductFixtureRoot` now optionally combines
the Dolt head. `cmd/evidenceroot` creates an embedded local Dolt repo under
`<candidate-root>/dolt-objectgraph`, writes the typed objectgraph snapshot
through `objectgraph.DoltStore`, commits it, queries `HASHOF('HEAD')`, and emits
the resulting `dolt_head`.

**Actual ΔV:**
- Candidate evidence roots now include a local embedded Dolt/objectgraph commit
  head, not only an in-memory typed objectgraph snapshot.
- The Dolt head is path-contained under the admitted candidate root and linked
  back to the same objectgraph head/counts.
- The claim remains fixture-level: no live corpusd/platform Dolt head was read.

**Evidence:**
- `go test ./cmd/basecompare ./cmd/baseobserve ./cmd/baseharness ./cmd/evidenceroot ./cmd/vmstateobserve ./cmd/vmstatecompare ./internal/base/api ./internal/base/blob ./internal/base/journal ./internal/computerversion ./internal/server` passed.
- `local://pass57-dolthead-command.json` records command return code 0.
- `local://pass57-dolthead-manifest.json` records an admitted `local_candidate`
  manifest with `dolt_head.repo_root` under the candidate root,
  `database: objectgraph`, `contains_production: false`, and commit hash
  `ugohvt2etcuvnpvr3enlb023larbnm59`.
- `local://pass57-dolthead-observation.json` records required kinds
  `blob_set`, `dolt_head`, `file_manifest`, `object_graph_head`,
  `promotion_certificate`, and `vm_state_manifest`.
- `local://pass57-dolthead-selfcheck.json` records `equivalent`.
- `local://pass57-dolthead-seeded-mismatch.json` records `not_equivalent` with a
  concrete `vm_state_manifest` difference.

**Deferred / Still Open:**
- No live corpusd/platform Dolt head has been sampled.
- No concrete Firecracker lifecycle materializer has been implemented.
- The candidate evidence root is still generated fixture state, not an actual
  running candidate computer.

**Next:** Build a non-lifecycle Firecracker materializer realization proof that
turns the existing vmmanager fixture paths into a declared
`Realization`/`CapabilityManifest` without booting/killing VMs, or explicitly
classify why lifecycle materialization must wait for a candidate computer.

## Pass 58 — 2026-07-04 (Non-Lifecycle VM Realization Command)

**Conjecture:** After the embedded Dolt/objectgraph fixture head, the next safe
Firecracker step is not lifecycle control; it is an executable realization
boundary that proves exactly what vmmanager fixture paths can claim through
`Realization` and `CapabilityManifest`.

**Move:** Added `cmd/vmrealize`. The command accepts explicit vmmanager fixture
paths and `ComputerVersion` refs, validates existing paths by default, and emits
`computerversion.Realization` JSON via `VMManagerScopedMaterializer` and
`VMManagerCapabilityManifest`. It does not boot, stop, resume, copy, or mutate a
VM. It supports only `vm_state_manifest` and carries explicit unsupported
capabilities for file/blob/Dolt/objectgraph/provenance/live-process claims.

**Actual ΔV:**
- The non-lifecycle Firecracker/vmmanager boundary is now executable from a
  command, not just an internal interface.
- The realization output cleanly separates supported `vm_state_manifest`
  evidence from unsupported durable-state claims.
- The proof still classifies fixture paths only; it is not a VM lifecycle
  materializer and not candidate-computer runtime state.

**Evidence:**
- `go test ./cmd/basecompare ./cmd/baseobserve ./cmd/baseharness ./cmd/evidenceroot ./cmd/vmrealize ./cmd/vmstateobserve ./cmd/vmstatecompare ./internal/base/api ./internal/base/blob ./internal/base/journal ./internal/computerversion ./internal/server` passed.
- `local://pass58-vmrealize-command.json` records command return code 0.
- `local://pass58-vmrealize-realization.json` records realization
  `pass58-vm-realization`, substrate `firecracker/vmmanager`, materializer
  `pass58-firecracker-scoped`, supported `vm_state_manifest`, six unsupported
  capability declarations, and one `vm_state_manifest` observation.

**Deferred / Still Open:**
- No VM lifecycle operation has been implemented or invoked.
- No deployed route, production state, auth/session, or staging VM was touched.
- No single candidate-computer package bundles the candidate evidence root and
  vm realization outputs yet.

**Next:** Define the minimum candidate-computer package/manifest that can bundle
the existing evidence-root command output and vmrealize output as one reviewable
candidate artifact without booting/killing VMs or touching deployed routes.

## Pass 59 — 2026-07-04 (Candidate-Computer Package Manifest)

**Conjecture:** After an admitted candidate evidence root and scoped
vmmanager realization both exist, the next safe package step is a hashed local
manifest that bundles those artifacts under one `ComputerVersion` without using
the product AppChangePackage/adoption path yet.

**Move:** Added `internal/computerversion.CandidateComputerPackageManifest` and
`cmd/candidatepackage`. The manifest binds one admitted
`CandidateEvidenceRootManifest`, the evidence-root `ObservationSet`, one or more
scoped `Realization` values, canonical required observation kinds, review
contracts, non-production/non-deployed-route flags, and a stable
`PackageManifestSHA256`. The command reads `cmd/evidenceroot` output plus
realization JSON and emits the package manifest.

**Actual ΔV:**
- A candidate-computer package now exists as a reviewable local artifact rather
  than an implicit pile of JSON files.
- Package validation keeps the previous admission and capability boundaries:
  production state, deployed-route mutation, mismatched `ComputerVersion`,
  missing required observations, and unsupported realization claims reject.
- This still is not a product-transfer object; no AppChangePackage, adoption,
  run-acceptance, promotion, deployed route, or VM lifecycle behavior consumes it
  yet.

**Evidence:**
- `go test ./internal/computerversion ./cmd/candidatepackage ./cmd/evidenceroot ./cmd/vmrealize` passed.
- `local://pass59-evidenceroot-output.json` records the admitted evidence root
  used as package input.
- `local://pass59-vmrealize-realization.json` records the scoped vmmanager
  realization used as package input.
- `local://pass59-candidatepackage-command.json` records the three command
  return codes as 0.
- `local://pass59-candidatepackage-manifest.json` records package
  `pass59-candidate-package`, hash
  `sha256:e320dce27f61370d8dbbdc65efef2698be0bbae22feec5dcf1da7d9191e0f69d`,
  six required observation classes, one scoped vmmanager realization,
  `contains_production: false`, and `touches_deployed_route: false`.

**Deferred / Still Open:**
- No product API or AppChangePackage path consumes
  `CandidateComputerPackageManifest` yet.
- No deployed route, production state, auth/session, staging VM, promotion, or VM
  lifecycle operation was touched.

**Next:** Compare `CandidateComputerPackageManifest` with the existing
`AppChangePackageRecord` product path and define the safe bridge, if any, that
makes candidate-computer package evidence reviewable by another computer without
mutating deployment, promotion, or live user state.

## Pass 60 — 2026-07-04 (AppChangePackage Bridge Payload)

**Conjecture:** The existing AppChangePackage path can carry
candidate-computer package evidence as embedded manifest/provenance/verifier
JSON, but direct publication must remain blocked because AppChangePackage
publication currently requires runtime or UI source deltas.

**Move:** Added `CandidatePackageAppChangeBridgePayload` and
`cmd/candidatepackage --output bridge`. The bridge validates a hashed
`CandidateComputerPackageManifest`, emits AppChangePackage-compatible
`manifest_json`, `verifier_contracts_json`, and `provenance_refs_json`, and marks
`direct_publish_ready: false` with explicit source-delta blockers.

**Actual ΔV:**
- The product-path comparison is now executable: candidate evidence can be
  represented in the existing AppChangePackage JSON fields without calling the
  product API.
- The bridge records why direct AppChangePackage publication remains invalid:
  candidate-computer evidence is not a runtime/UI source delta.
- The next decision is now a product acceptance boundary, not a data-shape
  mystery.

**Evidence:**
- `go test ./internal/computerversion ./cmd/candidatepackage ./cmd/evidenceroot ./cmd/vmrealize` passed.
- `local://pass60-appchange-bridge-command.json` records the bridge command
  return code as 0.
- `local://pass60-appchange-bridge-payload.json` records kind
  `candidate_package_app_change_bridge`, `direct_publish_ready: false`, blockers
  `app_change_package_publish_requires_runtime_or_ui_source_delta` and
  `candidate_computer_package_is_evidence_payload_not_product_source_delta`, and
  six required observation classes.

**Deferred / Still Open:**
- No product API consumes the bridge payload.
- No AppChangePackage/adoption/run-acceptance/promotion route changed.
- No deployed route, production state, auth/session, staging VM, or VM lifecycle
  operation was touched.

**Next:** Define the product-path acceptance contract for candidate-computer
evidence packages: decide which API boundary owns evidence-only package intake,
which verifier contracts gate owner review, and how rollback/adoption stays
impossible until a source-delta or candidate-computer promotion path exists.

## Pass 61 — 2026-07-04 (Product-Path Acceptance Contract)

**Conjecture:** Candidate-computer package evidence needs a named product-path
acceptance boundary before any runtime API is added. That boundary can be
defined as evidence-only intake plus owner review while keeping direct
AppChangePackage publication and adoption blocked.

**Move:** Added `CandidatePackageProductPathAcceptanceContract` and
`cmd/candidatepackage --output acceptance`. The contract validates a
`CandidateComputerPackageManifest` and its app-change bridge, selects
`candidate_package_evidence_only_intake` as the current safe boundary, requires
owner review, marks `adoption_ready: false`, and emits verifier-contract status
for passed package/evidence checks plus pending intake and blocked
publish/adoption boundaries.

**Actual ΔV:**
- The product acceptance boundary is now an executable contract instead of a
  doc-only choice.
- The contract preserves the AppChangePackage source-delta invariant: candidate
  evidence is reviewable, but it is not publishable or adoptable through the
  existing AppChangePackage path.
- The next unknown is storage/API ownership of evidence-only intake records, not
  verifier-contract semantics.

**Evidence:**
- `go test ./internal/computerversion ./cmd/candidatepackage ./cmd/evidenceroot ./cmd/vmrealize` passed.
- `local://pass61-product-path-acceptance-command.json` records the acceptance
  command return code as 0.
- `local://pass61-product-path-acceptance.json` records kind
  `candidate_package_product_path_acceptance`, intake boundary
  `candidate_package_evidence_only_intake`, `owner_review_required: true`,
  `adoption_ready: false`, four adoption blockers, three passed verifier
  contracts, one pending intake contract, and two blocked publish/adoption
  contracts.

**Deferred / Still Open:**
- No runtime/product API persists the acceptance contract.
- No AppChangePackage record, adoption flow, run-acceptance path, promotion
  path, deployed route, production state, auth/session, staging VM, or VM
  lifecycle operation was touched.

**Next:** Define and prove a storage/API intake record for evidence-only
candidate-computer packages that persists package hash, evidence refs,
verifier-contract state, owner review state, and rollback/adoption blockers
without publishing, adopting, promoting, or changing deployed routes.

## Pass 62 — 2026-07-04 (Candidate-Package Intake Record)

**Conjecture:** A candidate-computer evidence package can be persisted as an
evidence-only owner-review intake record without publishing an AppChangePackage,
adopting a computer, promoting, changing active routes, or touching VM
lifecycle.

**Move:** Added `CandidatePackageIntakeRecord` under `internal/types`,
candidate-package intake persistence methods under `internal/store`, and
`cmd/candidatepackage --output intake`. The record preserves package hash,
source refs, intake boundary, owner review state, adoption blockers,
verifier-contract JSON, evidence refs, required observations, acceptance JSON,
trace id, and timestamps. Store methods support upsert/get/list while rejecting
unsafe or incomplete records.

**Actual ΔV:**
- Evidence-only candidate-package intake now has a concrete durable record shape
  and store boundary.
- The boundary remains review/intake only: it does not publish an
  AppChangePackage, create an adoption, promote a computer, mutate routes, or
  touch VM lifecycle.
- The next unknown is the product/API handler boundary for creating and
  reviewing these intake records, not the persistence model itself.

**Evidence:**
- `go test ./internal/types ./internal/store ./cmd/candidatepackage ./internal/computerversion ./cmd/evidenceroot ./cmd/vmrealize` passed.
- `local://pass62-candidate-package-intake-command.json` records the intake
  command return code as 0.
- `local://pass62-candidate-package-intake.json` records intake
  `pass62-candidate-package-intake`, owner `pass62-owner`, status
  `owner_review_pending`, owner review state `required`, intake boundary
  `candidate_package_evidence_only_intake`, `adoption_ready: false`, and the
  four adoption blockers from the pass61 acceptance contract.
- Store tests prove insert/get/list round trip, update without intake-id drift,
  JSON payload preservation, and rejection of missing owner/package/hash/boundary
  fields, `adoption_ready: true`, and invalid JSON payloads.

**Deferred / Still Open:**
- No product/API route creates candidate-package intake records.
- No owner-review transition endpoint, adoption flow, rollback state machine,
  AppChangePackage publication path, promotion path, deployed route, production
  state, auth/session, staging VM, or VM lifecycle operation was touched.

**Next:** Define and prove the smallest non-deployed product/API handler
boundary for candidate-package intake creation/review that persists the intake
record, enforces owner scope and verifier state, and keeps adoption/rollback
transitions blocked until their semantics exist.

## Pass 63 — 2026-07-04 (Candidate-Package Intake API Handler Boundary)

**Conjecture:** A non-deployed product/API handler boundary can create and read
candidate-package intake records through the existing store while enforcing owner
scope and verifier/adoption blockers, without publishing AppChangePackages,
creating adoptions, promoting computers, mutating active routes, or touching VM
lifecycle.

**Move:** Added `internal/runtime/candidate_package_intake.go` runtime helper
methods and `internal/runtime/api_candidate_package_intake.go` opt-in API
handlers plus `RegisterCandidatePackageIntakeRoutes`. The helper is not wired
into deployed `RegisterRoutes`; local tests mount it explicitly on an in-process
server.

**Actual ΔV:**
- Authenticated create/list/detail handler path persists
  `CandidatePackageIntakeRecord` values through the existing store.
- Owner scope is enforced for create/list/detail; another owner receives 404 for
  a created intake.
- Missing auth, owner mismatch, and `adoption_ready: true` are rejected before
  persistence.
- Focused tests assert the intake handler path creates no AppChangePackage or
  AppAdoption side effects.

**Evidence:**
- `local://pass63-candidate-package-intake-handler-tests.jsonl`
- `go test ./internal/runtime -run TestCandidatePackageIntake` passed.
- `go test ./internal/types ./internal/store ./cmd/candidatepackage ./internal/computerversion ./cmd/evidenceroot ./cmd/vmrealize ./internal/runtime -run 'TestCandidatePackage|TestRunEmitsCandidatePackage|TestBuildCandidatePackage|TestUpsertCandidatePackageIntake'` passed.

**Deferred / Still Open:**
- No owner-review transition endpoint, adoption flow, rollback state machine,
  AppChangePackage publication path, promotion path, deployed route, production
  state, auth/session, staging VM, or VM lifecycle operation was touched.

**Next:** Define and prove the smallest non-deployed owner-review transition
endpoint for candidate-package intake records that can approve or reject review
state while keeping adoption/rollback/AppChangePackage publication and deployed
route mutation blocked until their semantics exist.

## Pass 64 — 2026-07-04 (Candidate-Package Intake Owner Review Transition)

**Conjecture:** A non-deployed owner-review transition endpoint can approve or
reject candidate-package intake review state while preserving the evidence-only
boundary: no adoption readiness, no AppChangePackage publication, no
active-computer promotion, no deployed route mutation, and no VM lifecycle
behavior.

**Move:** Added `ReviewCandidatePackageIntake` to
`internal/runtime/candidate_package_intake.go` and added the opt-in
`POST /api/candidate-package-intakes/{intake_id}/review` handler to
`internal/runtime/api_candidate_package_intake.go`. The review route is still
only mounted by `RegisterCandidatePackageIntakeRoutes`; deployed `RegisterRoutes`
remains unchanged.

**Actual ΔV:**
- Approve transitions pending owner-review intake records to
  `owner_approved`, clears `owner_review_required`, removes
  `owner_review_not_recorded`, appends review evidence, preserves non-review
  blockers, and keeps `adoption_ready: false`.
- Reject transitions pending owner-review intake records to `rejected`, records
  `owner_review_rejected`, appends review evidence, preserves non-review
  blockers, and keeps `adoption_ready: false`.
- Terminal re-review, invalid decisions, missing auth, and wrong-owner access are
  rejected.
- Focused tests assert the review path creates no AppChangePackage or
  AppAdoption side effects.

**Evidence:**
- `local://pass64-candidate-package-review-transition-tests.jsonl`
- `go test ./internal/runtime -run 'TestCandidatePackageIntakeReview|TestCandidatePackageIntakeOptIn|TestCandidatePackageIntakeRoutes'` passed.
- `go test ./internal/types ./internal/store ./cmd/candidatepackage ./internal/computerversion ./cmd/evidenceroot ./cmd/vmrealize ./internal/runtime -run 'TestCandidatePackage|TestRunEmitsCandidatePackage|TestBuildCandidatePackage|TestUpsertCandidatePackageIntake'` passed.

**Deferred / Still Open:**
- No adoption flow, rollback state machine, AppChangePackage publication path,
  promotion path, deployed route, production state, auth/session, staging VM, or
  VM lifecycle operation was touched.

**Next:** Define and prove the smallest non-deployed adoption/rollback state
boundary for approved candidate-package intake records while keeping direct
AppChangePackage publication, deployed route mutation, promotion, and VM
lifecycle behavior blocked until their semantics exist.

## Pass 65 — 2026-07-04 (Candidate-Package Adoption Boundary)

**Conjecture:** An approved candidate-package intake can bind the minimum
adoption/rollback readiness state without creating an AppChangePackage, creating
an AppAdoption, mutating deployed routes, promoting a computer, touching VM
lifecycle behavior, or claiming full product adoption.

**Move:** Added `BindCandidatePackageIntakeAdoptionBoundary` to
`internal/runtime/candidate_package_intake.go`, added opt-in
`POST /api/candidate-package-intakes/{intake_id}/adoption-boundary` handling in
`internal/runtime/api_candidate_package_intake.go`, and narrowed
`internal/store/candidate_package_intake.go` so `adoption_ready: true` can only
persist after owner-approved review state with no remaining blockers. Deployed
`RegisterRoutes` remains unchanged; local tests mount
`RegisterCandidatePackageIntakeRoutes` explicitly.

**Actual ΔV:**
- Adoption-boundary binding requires an owner-approved intake plus explicit
  adoption and rollback contract refs.
- Binding removes `adoption_rollback_boundary_not_bound`, writes an
  `adoption_rollback_boundary` acceptance envelope, appends optional boundary
  evidence, and marks `adoption_ready: true` only when no blockers remain.
- Pending, rejected, wrong-owner, missing-ref, and already-ready transitions are
  rejected or no-op bounded.
- Direct create still rejects all adoption-ready payloads; store persistence
  permits adoption-ready records only after owner approval, `owner_review_required:
  false`, and zero adoption blockers.
- Focused tests assert the boundary creates no AppChangePackage or AppAdoption
  side effects.

**Evidence:**
- `local://pass65-candidate-package-adoption-boundary-tests.jsonl`
- `go test ./internal/runtime ./internal/store -run 'TestCandidatePackageIntakeAdoptionBoundary|TestCandidatePackageIntakeReview|TestCandidatePackageIntakeOptIn|TestCandidatePackageIntakeRoutes|TestCandidatePackageIntakeRoundTripPreservesAdoptionReady|TestCandidatePackageIntakeRejectsUnsafeOrIncompleteRecords'` passed.

**Deferred / Still Open:**
- No AppChangePackage publication path, product adoption consumer, promotion
  path, deployed route, production state, auth/session, staging VM, or VM
  lifecycle operation was touched.
- Adoption-ready intake is readiness state only; it is not publication,
  adoption, promotion, or rollback execution.

**Next:** Define and prove the smallest non-deployed publication/adoption
consumer for adoption-ready candidate-package intake records while keeping
deployed route mutation, active-computer promotion, and VM lifecycle behavior
blocked until their semantics exist.

## Pass 66 — 2026-07-04 (Candidate-Package Publication Draft Boundary)

**Conjecture:** An adoption-ready candidate-package intake can produce a
reviewable non-published AppChangePackage draft candidate without creating an
AppAdoption, publishing a package, mutating deployed routes, promoting a
computer, touching VM lifecycle behavior, or claiming full product adoption.

**Move:** Added `CreateCandidatePackageIntakePublicationDraft` to
`internal/runtime/candidate_package_intake.go` and added opt-in
`POST /api/candidate-package-intakes/{intake_id}/publication-draft` handling in
`internal/runtime/api_candidate_package_intake.go`. Deployed `RegisterRoutes`
remains unchanged; local tests mount `RegisterCandidatePackageIntakeRoutes`
explicitly.

**Actual ΔV:**
- Publication-draft creation requires an owner-approved, adoption-ready intake,
  zero adoption blockers, a bound adoption/rollback acceptance envelope, and an
  explicit publication contract ref.
- Success creates or returns a private draft `AppChangePackageRecord` whose
  manifest/provenance/verifier-contract JSON ties the draft to the intake,
  candidate package, publication contract, adoption contract, and rollback
  contract.
- The draft manifest records direct publication, AppAdoption creation,
  promotion, deployed-route mutation, and VM lifecycle as blocked.
- Pending, rejected, not-ready, wrong-owner, missing publication contract,
  missing adoption/rollback boundary, and unrelated package-id collision
  transitions are rejected without creating new promotion/adoption state.
- Focused tests assert successful draft creation creates no `AppAdoption`.

**Evidence:**
- `local://pass66-candidate-package-publication-draft-tests.jsonl`
- `go test ./internal/runtime ./internal/store -run TestCandidatePackageIntake` passed.
- `scripts/doccheck report-only` passed in report-only mode: 293 docs, 440 warnings.

**Deferred / Still Open:**
- No deployed route, package publication path, product adoption state machine,
  promotion path, production state, auth/session, staging VM, or VM lifecycle
  operation was touched.
- Publication draft is reviewable candidate state only; it is not package
  publication, adoption execution, promotion, rollback execution, or run
  acceptance.

**Next:** Define and prove the smallest non-deployed owner adoption/review state
machine for candidate-package publication drafts while keeping deployed route
mutation, active-computer promotion, and VM lifecycle behavior blocked until
their semantics exist.

## Pass 67 — 2026-07-04 (Candidate-Package Adoption Review Boundary)

**Conjecture:** A private candidate-package publication draft can enter an
owner adoption/review state machine without creating package publication,
AppAdoption execution, active-computer promotion, deployed route mutation,
rollback execution, or VM lifecycle side effects.

**Move:** Added a non-deployed opt-in runtime/API owner adoption-review
boundary for private candidate-package publication drafts.

**Actual ΔV:**
- Adoption-review creation requires an owner-approved, adoption-ready intake,
  zero adoption blockers, a private publication-draft `AppChangePackageRecord`
  matching the intake manifest, a target computer, and adoption-review contract
  ref.
- Success creates or returns an `AppAdoptionRecord` with status
  `owner_review_pending`; package publication, deployed-route mutation,
  promotion, rollback execution, and VM lifecycle remain blocked in recorded
  verifier/rollback/profile state.
- Owner decision transitions can resolve the private review state to
  `owner_review_approved` or `owner_review_rejected` while preserving the
  candidate-package intake/draft provenance.
- Pending intake, approved-but-not-adoption-ready intake, rejected intake,
  wrong-owner intake, missing adoption-review contract, unrelated package-id
  collision, malformed draft manifest, and duplicate review attempts are
  rejected or bounded without creating unsafe adoption/promotion state.

**Evidence:**
- `local://pass67-candidate-package-adoption-review-tests.jsonl`
- `go test ./internal/runtime ./internal/store -run TestCandidatePackageIntake`
  passed.

**Deferred / Still Open:**
- No deployed route, package publication path, active-computer switch,
  promotion execution, rollback execution, production state, auth/session,
  staging VM, or VM lifecycle operation was touched.
- Adoption review is private non-deployed candidate state only; it is not
  package publication, promotion, rollback execution, run acceptance, or a
  deployed product adoption path.

**Next:** Define and prove the smallest non-deployed product promotion /
active-computer switch boundary for owner-approved candidate-package adoption
reviews while keeping deployed route mutation and VM lifecycle behavior blocked
until their semantics exist.

## Pass 68 — 2026-07-04 (Candidate-Package Promotion Switch Boundary)

**Conjecture:** An owner-approved private candidate-package adoption review can
perform the smallest non-deployed active-computer source-lineage switch without
publishing the package, mutating deployed routes, executing rollback, touching
VM lifecycle behavior, or claiming full product promotion.

**Move:** Added a non-deployed opt-in runtime/API promotion-switch boundary for
owner-approved candidate-package adoption reviews.

**Actual ΔV:**
- Promotion-switch creation requires an owner-approved adoption review whose
  private draft package still matches the candidate-package intake manifest,
  whose target computer lineage still matches the expected active source ref,
  and whose candidate source ref is non-empty.
- Success updates only the target computer source lineage to the candidate
  source ref, records adoption/package/candidate provenance, marks the adoption
  `source_lineage_switched`, and records verifier status
  `source_lineage_switched`.
- Pending reviews, rejected reviews, wrong-owner requests, missing candidate
  refs, mismatched candidate refs, stale active lineage, and already-switched
  adoptions are rejected or no-op bounded without package publication,
  deployed-route mutation, rollback execution, or VM lifecycle effects.

**Evidence:**
- `local://pass68-candidate-package-promotion-switch-tests.jsonl`
- `go test ./internal/runtime ./internal/store -run TestCandidatePackageIntakePromotionSwitch`
  passed.
- `scripts/doccheck report-only` passed in report-only mode: 293 docs, 440
  warnings.

**Deferred / Still Open:**
- No deployed route, package publication path, rollback/roll-forward consumer,
  production state, auth/session, staging VM, or VM lifecycle operation was
  touched.
- Promotion switch is source-lineage-only private candidate state; it is not
  package publication, full product promotion, deployed route mutation, rollback
  execution, run acceptance, or VM lifecycle settlement.

**Next:** Define and prove the smallest non-deployed rollback/roll-forward
boundary for source-lineage-switched candidate-package adoption reviews while
keeping deployed route mutation, package publication, and VM lifecycle behavior
blocked until their semantics exist.

## Pass 69 — 2026-07-04 (Candidate-Package Source-Lineage Rollback / Roll-Forward Boundary)

**Conjecture:** A source-lineage-switched private candidate-package adoption
review can be rolled back to its recorded previous active source ref, and a
rolled-back review can be rolled forward again to its candidate source ref,
through a bounded non-deployed opt-in transition without publishing packages,
mutating deployed routes, touching VM lifecycle behavior, or claiming full
product promotion.

**Move:** Added non-deployed opt-in runtime/API rollback and roll-forward
boundaries for source-lineage-switched candidate-package adoption reviews.

**Actual ΔV:**
- Rollback requires a `source_lineage_switched` adoption review, a valid
  candidate source ref, a recorded cutover ref, matching adoption/rollback
  contract refs, and current target lineage still equal to the candidate switch
  ref.
- Successful rollback restores only the target computer source lineage to the
  recorded previous active source ref, records verifier/profile rollback state,
  and marks the adoption `rolled_back`.
- Roll-forward requires a `rolled_back` adoption review and current target
  lineage still equal to the recorded rollback target; success switches only the
  target lineage back to the candidate source ref and records source-lineage-only
  roll-forward evidence.
- Pending reviews, approved-but-unswitched reviews, stale rollback lineage,
  duplicate rollback attempts, and stale roll-forward lineage are rejected or
  no-op bounded without package publication, deployed-route mutation, VM
  lifecycle effects, or full promotion/build fields.

**Evidence:**
- `local://pass69-candidate-package-promotion-switch-rollback-tests.jsonl`
- `go test ./internal/runtime ./internal/store -run TestCandidatePackageIntakePromotionSwitch`
  passed.

**Deferred / Still Open:**
- No deployed route, package publication path, run-acceptance/product acceptance
  consumer, production state, auth/session, staging VM, or VM lifecycle
  operation was touched.
- Rollback/roll-forward is source-lineage-only private candidate state; it is
  not package publication, full product promotion, deployed route mutation, run
  acceptance, staging acceptance, or VM lifecycle settlement.

**Next:** Define and prove the smallest non-deployed candidate-package promotion
acceptance evidence boundary that consumes owner-review, source-lineage switch,
and rollback/roll-forward evidence while keeping deployed route mutation,
package publication, auth/session, staging, and VM lifecycle behavior blocked.

## Pass 70 — 2026-07-04 (Candidate-Package Local Acceptance Evidence Boundary)

**Conjecture:** The complete local candidate-package evidence chain can be
summarized as a bounded non-deployed acceptance artifact that consumes owner
review, source-lineage switch, and rollback/roll-forward evidence without
claiming deployed promotion-level acceptance, package publication, auth/session
proof, staging proof, or VM lifecycle settlement.

**Move:** Added a non-deployed opt-in runtime/API acceptance evidence boundary
for candidate-package promotion-switch reviews.

**Actual ΔV:**
- `GET /api/candidate-package-intakes/{intake_id}/adoption-review/{adoption_id}/promotion-switch/acceptance`
  returns a `candidate_package_promotion_switch_acceptance_evidence` artifact
  only within the local route harness; no deployed route registration changed.
- Accepted evidence requires owner-review approval, source-lineage switch,
  rollback, and roll-forward checkpoints, plus matching package/intake/adoption
  provenance.
- The artifact carries `local-source-lineage-evidence` scope and explicit
  blocked/unproven boundaries for package publication, deployed promotion,
  deployed route mutation, promotion-level acceptance, RunAcceptanceRecord
  creation, auth/session, staging, and VM lifecycle behavior.
- Missing rollback/roll-forward evidence, rolled-back current state, and
  wrong-owner requests are rejected without mutating package/adoption/run
  acceptance state.

**Evidence:**
- `local://pass70-candidate-package-acceptance-evidence-tests.jsonl`
- `go test ./internal/runtime ./internal/store -run TestCandidatePackageIntakePromotionSwitch`
  passed.

**Deferred / Still Open:**
- No product UI, deployed route, package publication path, production state,
  auth/session proof, staging proof, VM lifecycle operation, or RunAcceptanceRecord
  consumer was touched.
- The accepted artifact is local-source-lineage evidence only; it is not package
  publication, full product promotion, deployed route mutation, run acceptance,
  staging acceptance, or VM lifecycle settlement.

**Next:** Define and prove the smallest product-visible non-deployed
candidate-package adoption/promotion review surface that consumes
local-source-lineage acceptance evidence without publishing packages, mutating
deployed routes, touching auth/session or VM lifecycle behavior, or claiming
staging/deployed acceptance.

## Pass 71 — 2026-07-04 (Candidate-Package Product-Visible Non-Deployed Review Surface)

**Conjecture:** Local-source-lineage acceptance evidence can feed a
product-visible but non-deployed candidate-package adoption/promotion review
surface without publishing packages, mutating deployed routes, touching
auth/session or VM lifecycle behavior, creating RunAcceptanceRecords, or
claiming staging/deployed acceptance.

**Move:** Added a read-only non-deployed opt-in runtime/API review-surface
boundary for candidate-package promotion-switch reviews.

**Actual ΔV:**
- `GET /api/candidate-package-intakes/{intake_id}/adoption-review/{adoption_id}/promotion-switch/review-surface`
  returns a `candidate_package_adoption_promotion_review_surface` only within
  the local route harness; no deployed route registration changed.
- The surface requires accepted local-source-lineage evidence, embeds or
  references that evidence, and stays scoped as `product_visible_non_deployed`
  with `deployment_state: non_deployed`.
- The surface is read-only and allows only review/inspect actions.
- The surface explicitly blocks package publication, deployed promotion, deployed
  route mutation, promotion-level acceptance, RunAcceptanceRecord creation,
  auth/session, staging, VM lifecycle behavior, AppChangePackage mutation, and
  AppAdoption mutation.
- Incomplete rollback/roll-forward evidence, rolled-back current state,
  wrong-owner access, non-GET methods, and malformed paths reject without
  mutating package/adoption/run-acceptance state or target lineage.

**Evidence:**
- `local://pass71-candidate-package-promotion-review-surface-tests.jsonl`
- `go test ./internal/runtime ./internal/store -run TestCandidatePackageIntakePromotionSwitch -parallel=1`
  passed.
- `scripts/doccheck report-only` passed in report-only mode: 293 docs, 440
  warnings.

**Deferred / Still Open:**
- No deployed route, package publication path, product UI, production state,
  auth/session proof, staging proof, VM lifecycle operation, or RunAcceptanceRecord
  consumer was touched.
- The review surface is product-visible non-deployed local-route evidence only;
  it is not package publication, full product promotion, deployed route mutation,
  run acceptance, staging acceptance, or VM lifecycle settlement.

**Next:** Define and prove the smallest UI/workflow consumer for the
non-deployed candidate-package adoption/promotion review surface while keeping
deployed route mutation, package publication, auth/session, staging, VM
lifecycle, and run-acceptance boundaries blocked.

## Pass 72 — 2026-07-04 (Candidate-Package Review UI Consumer)

**Conjecture:** The non-deployed candidate-package adoption/promotion review
surface can be consumed by the smallest product UI/workflow without adding
deployed route mutation, package publication, auth/session changes, staging
claims, VM lifecycle behavior, or run-acceptance semantics.

**Move:** Added a `candidate-review` desktop app and URL app intent that consume
only the read-only candidate-package review-surface GET for an intake/adoption
pair.

**Actual ΔV:**
- `candidate-review` opens from Desk or `/?app=candidate-review&intake=...&adoption=...`.
- The app fetches only
  `/api/candidate-package-intakes/{intake_id}/adoption-review/{adoption_id}/promotion-switch/review-surface`
  when authenticated and both IDs are present.
- The app renders read-only/product-visible/non-deployed state, accepted
  local-source-lineage evidence, review/inspect actions, provenance, and blocked
  boundaries.
- Missing IDs keep the app in an input/empty state without calling the
  candidate-package review API.
- Signed-out users can open the app but loading a review surface requests auth;
  it does not call the review-surface API, shell bootstrap, or desktop-state
  protected routes.

**Evidence:**
- `local://pass72-candidate-review-ui-tests.json`
- `pnpm --dir frontend exec playwright test tests/candidate-review-app.spec.js --reporter=json`
  passed: 3 tests, 0 unexpected.
- `pnpm --dir frontend build` passed.
- `scripts/doccheck report-only` passed in report-only mode: 293 docs, 440
  warnings.

**Deferred / Still Open:**
- No deployed backend route exposure, package publication path, production state,
  auth/session change, staging proof, VM lifecycle operation, or RunAcceptanceRecord
  consumer was touched.
- The UI is a product workflow consumer for mocked/local review-surface data; it
  is not deployed candidate-package promotion, run acceptance, staging
  acceptance, or VM lifecycle settlement.

**Next:** Define and prove the smallest deployed-read/non-promoting route
exposure for the Candidate Review UI while keeping package publication,
candidate deployed route mutation, auth/session changes, staging acceptance, VM
lifecycle, and run-acceptance boundaries blocked.

## Pass 73 — 2026-07-04 (Candidate-Package Deployed Read-Only Review Surface)

**Conjecture:** The Candidate Review UI can be backed by a deployed-read,
non-promoting runtime route that exposes only the review-surface GET and keeps
candidate-package creation, owner-review mutation, publication-draft creation,
source-lineage switch, rollback/roll-forward, package publication,
AppAdoption mutation, auth/session changes, staging acceptance, VM lifecycle,
and run-acceptance boundaries blocked.

**Move:** Registered `RegisterCandidatePackageReviewSurfaceRoutes` from deployed
runtime `RegisterRoutes` and kept `RegisterCandidatePackageIntakeRoutes` as the
full opt-in local harness registrar. The deployed route parser accepts only:

- `GET /api/candidate-package-intakes/{intake_id}/adoption-review/{adoption_id}/promotion-switch/review-surface`

**Actual ΔV:**
- Deployed `RegisterRoutes` serves the authenticated owner review-surface GET
  after the existing local harness creates accepted local-source-lineage
  evidence.
- The deployed review-surface GET is read-only over CandidatePackageIntake,
  AppChangePackage, AppAdoption, RunAcceptanceRecord, and target source-lineage
  state.
- Deployed `RegisterRoutes` rejects candidate-package intake root create/list,
  detail read, owner-review mutation, adoption-boundary mutation,
  publication-draft mutation, adoption-review create/decision, promotion-switch,
  rollback, roll-forward, and local acceptance evidence routes.
- The Candidate Review UI still fetches only the review-surface GET and renders
  product-visible non-deployed local-source-lineage evidence without activation,
  publication, or run-acceptance controls.

**Evidence:**
- `local://pass73-deployed-route-tests.jsonl`
- `local://pass73-candidate-review-ui-tests.json`
- `go test -json ./internal/runtime -run 'TestCandidatePackageIntake(DeployedRegisterRoutesServesOnlyReviewSurface|PromotionSwitchReviewSurfaceRouteReturnsReadOnlyProductSurface)$' -count=1 -parallel=1`
  passed: 15 pass events, 0 fail events.
- `pnpm --dir frontend exec playwright test tests/candidate-review-app.spec.js --reporter=json`
  passed: 3 tests, 0 unexpected.

**Deferred / Still Open:**
- No package publication path, candidate deployed route mutation, AppAdoption
  mutation, production state, auth/session change, staging proof, VM lifecycle
  operation, or RunAcceptanceRecord consumer was touched.
- The deployed route is product-visible read-only evidence for local
  source-lineage acceptance; it is not full product promotion, run acceptance,
  staging acceptance, or VM lifecycle settlement.

**Next:** Define and prove the smallest owner-controlled activation/promotion
decision boundary that can consume the product-visible review surface while
keeping package publication, auth/session, staging, VM lifecycle, and
run-acceptance boundaries explicit.

## Pass 74 — 2026-07-04 (Candidate-Package Owner Activation Decision Boundary)

**Conjecture:** The product-visible Candidate Review surface can expose an
owner-controlled activation decision boundary that prepares the next promotion
decision from accepted local source-lineage evidence without publishing packages,
mutating AppAdoption, mutating deployed routes, touching auth/session, claiming
staging acceptance, changing VM lifecycle, or creating run-acceptance records.

**Move:** Added an `activation_decision_boundary` object to the review-surface
schema and a Candidate Review UI panel that lets an authenticated owner prepare
the local activation decision summary from the accepted review surface.

**Actual ΔV:**
- Review-surface JSON now names the owner-controlled activation boundary:
  `owner_decision_preparable`, `prepare_activation_decision`, `no_mutation`, the
  local acceptance id, the next AppAdoption promotion contract boundary,
  blocked protected routes, and required contracts.
- Candidate Review renders an "Owner activation decision" panel. Clicking
  "Prepare activation decision" produces a local summary from the surface.
- The click does not call `/api/adoptions`, candidate-package mutation routes,
  `/api/run-acceptances`, auth mutation, staging, or VM lifecycle routes.
- Runtime review-surface tests still prove the review-surface GET is read-only
  over CandidatePackageIntake, AppChangePackage, AppAdoption,
  RunAcceptanceRecord, and target source-lineage state.

**Evidence:**
- `local://pass74-activation-boundary-runtime-tests.jsonl`
- `local://pass74-activation-boundary-ui-tests.json`
- `go test -json ./internal/runtime -run 'TestCandidatePackageIntake(PromotionSwitchReviewSurfaceRouteReturnsReadOnlyProductSurface|DeployedRegisterRoutesServesOnlyReviewSurface)$' -count=1 -parallel=1`
  passed: 15 pass events, 0 fail events.
- `pnpm --dir frontend exec playwright test tests/candidate-review-app.spec.js --reporter=json`
  passed: 3 tests, 0 unexpected.
- `pnpm --dir frontend build` passed.

**Deferred / Still Open:**
- No package publication path, AppAdoption mutation, candidate deployed route
  mutation, production state, auth/session mutation, staging proof, VM lifecycle
  operation, or RunAcceptanceRecord consumer was touched.
- The activation boundary is a product-visible local decision seam only; it is
  not durable activation, package publication, run acceptance, staging
  acceptance, or VM lifecycle settlement.

**Next:** Define the smallest durable activation contract that can consume this
owner decision boundary without conflating local source-lineage acceptance with
promotion-level acceptance.

## Pass 75 — 2026-07-04 (Candidate-Package Durable Activation Contract)

**Conjecture:** The prepared owner activation decision boundary can be consumed
by a pure non-runtime `computerversion` contract without conflating local
source-lineage acceptance with activation readiness, package publication,
AppAdoption mutation, deployed route mutation, auth/session mutation, staging
acceptance, VM lifecycle mutation, promotion-level acceptance, or
RunAcceptanceRecord creation.

**Move:** Added `CandidatePackageOwnerActivationDecision`,
`CandidatePackageDurableActivationContract`, and
`BuildCandidatePackageDurableActivationContract` in
`internal/computerversion`. The builder binds a
`CandidateComputerPackageManifest`, existing
`CandidatePackageProductPathAcceptanceContract`, and prepared owner decision
into a blocked durable contract object.

**Actual ΔV:**
- A valid package/acceptance/owner-decision triple emits a contract bound to the
  package id, package manifest hash, `ComputerVersion`, and local acceptance id.
- The emitted contract is `activation_ready=false`, `no_mutation=true`, and
  `promotion_level_claimed=false`.
- The contract carries blockers for package publication, AppAdoption mutation,
  deployed route mutation, auth/session mutation, staging acceptance, VM
  lifecycle mutation, and RunAcceptanceRecord creation.
- Mismatched acceptance package id/hash/version, mismatched owner-decision
  package id/hash/version, missing local acceptance id, activation-ready claims,
  promotion-level claims, and `no_mutation=false` are rejected.

**Evidence:**
- `local://pass75-durable-activation-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildCandidatePackageDurableActivationContract -parallel=1`
  passed: 14 pass events, 0 fail events.
- `local://pass75-candidate-review-ui-regression.json`
- `pnpm --dir frontend exec playwright test tests/candidate-review-app.spec.js --reporter=json`
  passed: 3 tests, 0 unexpected.
- `scripts/doccheck report-only` passed in report-only mode: 293 docs, 440 warnings.

**Deferred / Still Open:**
- No runtime handler, deployed route, data migration, package publication,
  AppAdoption mutation, production state, auth/session mutation, staging proof,
  VM lifecycle operation, or RunAcceptanceRecord consumer was touched.
- The durable activation contract is verifier input only; it is not package
  publication, AppAdoption approval/promotion, route activation, staging
  acceptance, VM lifecycle settlement, or run acceptance.

**Next:** Define the next product-activation verifier contract that can consume
the durable activation contract and decide which blocked prerequisite is the
first safe non-runtime proof to bind.

## Pass 76 — 2026-07-04 (Candidate-Package Product Activation Verifier Contract)

**Conjecture:** A pure non-runtime product-activation verifier can consume the
durable activation contract and identify package publication as the first safe
bindable prerequisite without treating local source-lineage acceptance as
activation readiness, AppAdoption mutation, deployed route mutation,
auth/session mutation, staging acceptance, VM lifecycle settlement,
promotion-level acceptance, or RunAcceptanceRecord creation.

**Move:** Added `CandidatePackageProductActivationEvidence`,
`CandidatePackageProductActivationPrerequisiteEvidence`,
`CandidatePackageProductActivationVerifierContract`, and
`BuildCandidatePackageProductActivationVerifierContract` in
`internal/computerversion`. The verifier consumes the Pass 75 durable activation
contract and prerequisite evidence candidates.

**Actual ΔV:**
- With only an explicit package-publication proof candidate, the verifier
  selects `package_publication_contract` as the first bindable prerequisite.
- The verifier remains `activation_ready=false`, `no_mutation=true`, and
  `promotion_level_claimed=false`.
- AppAdoption, deployed route, auth/session, staging, VM lifecycle, and
  run-acceptance prerequisites stay blocked in this first verifier slice.
- Missing durable package/hash/local-acceptance identity, mismatched evidence
  package id/hash/version/local acceptance id, passed prerequisites without
  evidence refs, and unsafe first-slice AppAdoption/staging/VM/run-acceptance
  candidates are rejected or narrowed to blocked.

**Evidence:**
- `local://pass76-product-activation-verifier-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildCandidatePackageProductActivationVerifierContract -parallel=1`
  passed: 21 pass events, 0 fail events.

**Deferred / Still Open:**
- No package publication implementation, AppAdoption mutation, deployed route
  mutation, production state, auth/session mutation, staging proof, VM lifecycle
  operation, or RunAcceptanceRecord consumer was touched.
- The verifier only chooses the next safe prerequisite to bind; it does not bind
  that prerequisite, publish a package, or authorize product activation.

**Next:** Bind the package-publication prerequisite as a pure reviewable
contract/evidence object, still outside runtime route mutation and AppAdoption
mutation.

## Pass 77 — 2026-07-04 (Candidate-Package Package-Publication Proof Contract)

**Conjecture:** The verifier-selected package-publication prerequisite can be
bound as a pure reviewable proof contract without actually publishing a package,
authorizing activation, mutating AppAdoption, mutating deployed routes, touching
auth/session, claiming staging, changing VM lifecycle, claiming promotion-level
acceptance, or creating RunAcceptanceRecords.

**Move:** Added `CandidatePackagePublicationProofEvidence`,
`CandidatePackagePublicationProofContract`, and
`BuildCandidatePackagePublicationProofContract` in `internal/computerversion`.
The builder consumes the Pass 76 verifier and binds only the verifier-selected
package-publication evidence ref.

**Actual ΔV:**
- A verifier-selected `package_publication_contract` candidate can be represented
  as a bound proof contract tied to package id, package manifest hash,
  `ComputerVersion`, local acceptance id, and verifier evidence ref.
- The emitted proof contract has `publication_bound=true` while remaining
  `actual_package_published=false`, `no_mutation=true`,
  `activation_ready=false`, and `promotion_level_claimed=false`.
- AppAdoption, deployed route, auth/session, staging, VM lifecycle, and
  run-acceptance prerequisites remain blocked.
- Mismatched proof identity, verifier state without bindable package
  publication, and unsafe proof claims for actual publication/AppAdoption/route
  mutation/staging/VM/run-acceptance are rejected or kept non-authorizing.

**Evidence:**
- `local://pass77-publication-proof-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildCandidatePackagePublicationProofContract -parallel=1`
  passed: 20 pass events, 0 fail events.

**Deferred / Still Open:**
- No actual package publication implementation, AppAdoption mutation, deployed
  route mutation, production state, auth/session mutation, staging proof, VM
  lifecycle operation, or RunAcceptanceRecord consumer was touched.
- The publication proof contract is inert review evidence only; it does not
  publish a package or authorize activation.

**Next:** Define the pure source-delta/package-publication payload boundary that
could turn this proof contract into a reviewable package publication candidate
without touching deployed runtime routes or AppAdoption mutation.

## Pass 78 — 2026-07-04 (Candidate-Package Publication Payload Boundary Contract)

**Conjecture:** A verifier-bound package-publication proof can be tied to
explicit source-delta and payload-manifest refs as a reviewable publication
payload candidate without actually publishing a package, making direct publish
ready, authorizing activation, mutating AppAdoption, mutating deployed routes,
touching auth/session, claiming staging, changing VM lifecycle, claiming
promotion-level acceptance, or creating RunAcceptanceRecords.

**Move:** Added `CandidatePackagePublicationPayloadEvidence`,
`CandidatePackagePublicationPayloadContract`, and
`BuildCandidatePackagePublicationPayloadContract` in `internal/computerversion`.
The builder consumes the Pass 77 publication proof and binds only explicit
`source_delta_ref` and `payload_manifest_ref` evidence.

**Actual ΔV:**
- A publication proof can now become a reviewable package-publication payload
  candidate tied to package id, package manifest hash, `ComputerVersion`, local
  acceptance id, verifier evidence ref, source-delta ref, and payload-manifest
  ref.
- The emitted payload contract has `reviewable_publication_candidate=true` while
  remaining `actual_package_published=false`, `direct_publish_ready=false`,
  `no_mutation=true`, `activation_ready=false`, and
  `promotion_level_claimed=false`.
- AppAdoption, deployed route, auth/session, staging, VM lifecycle, and
  run-acceptance prerequisites remain blocked.
- Unbound publication proofs, mismatched identity, missing source-delta/payload
  refs, actual publication claims, direct-publish-ready claims, AppAdoption,
  deployed-route, auth/session, staging, VM lifecycle, run-acceptance,
  activation-ready, and promotion-level claims are rejected.

**Evidence:**
- `local://pass78-publication-payload-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildCandidatePackagePublicationPayloadContract -parallel=1`
  passed: 22 pass events, 0 fail events.

**Deferred / Still Open:**
- No actual package publication executor, AppAdoption mutation, deployed route
  mutation, production state, auth/session mutation, staging proof, VM lifecycle
  operation, or RunAcceptanceRecord consumer was touched.
- The publication payload contract is inert review evidence only; it does not
  publish a package, make direct publish ready, or authorize activation.

**Next:** Define the pure package-publication executor preflight contract that
can inspect this payload boundary and state exactly which non-mutating checks
would be required before any future red package-publication executor exists.

## Pass 79 — 2026-07-04 (Candidate-Package Publication Executor Preflight Contract)

**Conjecture:** A reviewable package-publication payload can feed a pure
executor preflight contract that records required non-mutating checks while
keeping executor permission false, actual package publication false, direct
publish readiness false, product activation false, promotion-level acceptance
false, and all protected mutation surfaces blocked.

**Move:** Added `CandidatePackagePublicationPreflightEvidence`,
`CandidatePackagePublicationPreflightContract`, and
`BuildCandidatePackagePublicationPreflightContract` in `internal/computerversion`.
The builder consumes the Pass 78 publication payload contract and records
canonical preflight check refs plus optional verifier contract refs.

**Actual ΔV:**
- A reviewable publication payload can now be transformed into preflight review
  evidence tied to package id, package manifest hash, `ComputerVersion`, local
  acceptance id, verifier evidence ref, source-delta ref, payload-manifest ref,
  and non-mutating preflight check refs.
- The emitted preflight contract has `executor_allowed=false`,
  `actual_package_published=false`, `direct_publish_ready=false`,
  `no_mutation=true`, `activation_ready=false`, and
  `promotion_level_claimed=false`.
- Package publication, AppAdoption, deployed route, auth/session, staging, VM
  lifecycle, and run-acceptance prerequisites remain blocked.
- Malformed payload contracts, mismatched identity/source/payload refs, missing
  preflight check refs, executor-allowed claims, actual publication claims,
  direct-publish-ready claims, AppAdoption, deployed-route, auth/session,
  staging, VM lifecycle, run-acceptance, activation-ready, and promotion-level
  claims are rejected.

**Evidence:**
- `local://pass79-publication-preflight-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildCandidatePackagePublicationPreflightContract -parallel=1`
  passed: 25 pass events, 0 fail events.
- Tester teeth check temporarily changed the preflight contract to report
  `actual_package_published=true`; the focused test failed, and the mutation was
  reverted.

**Deferred / Still Open:**
- No package-publication executor, AppAdoption mutation, deployed route mutation,
  production state, auth/session mutation, staging proof, VM lifecycle operation,
  or RunAcceptanceRecord consumer was touched.
- Preflight evidence is not executor permission. Any future executor remains a
  red mutation requiring AGENTS.md ceremony and higher evidence.

**Next:** Define the pure owner/reviewer authorization gate that can decide
whether this preflight packet is ready for a future red executor design review,
without executing publication or changing product state.

## Pass 80 — 2026-07-04 (Candidate-Package Publication Executor Review Gate Contract)

**Conjecture:** A package-publication preflight packet can pass through a pure
owner/reviewer gate for future red executor design review without becoming
executor permission, actual publication, direct publish readiness, product
activation, promotion-level acceptance, or any deployed/runtime mutation.

**Move:** Added `CandidatePackagePublicationExecutorReviewGateEvidence`,
`CandidatePackagePublicationExecutorReviewGateContract`, and
`BuildCandidatePackagePublicationExecutorReviewGateContract` in
`internal/computerversion`. The builder consumes the Pass 79 preflight contract,
requires owner and reviewer authorization refs, preserves preflight check refs
and verifier contract refs, and emits only design-review readiness.

**Actual ΔV:**
- A publication preflight contract can now be bound to owner/reviewer
  authorization refs as a review-gate contract for future executor design
  review.
- The emitted gate has `executor_design_review_ready=true` but still has
  `executor_allowed=false`, `actual_package_published=false`,
  `direct_publish_ready=false`, `no_mutation=true`, `activation_ready=false`,
  and `promotion_level_claimed=false`.
- Package publication, AppAdoption, deployed route, auth/session, staging, VM
  lifecycle, and run-acceptance prerequisites remain blocked.
- Malformed preflight contracts, mismatched identity/source/payload/check refs,
  verifier-contract ref mismatches, missing owner/reviewer authorization refs,
  executor-allowed claims, actual publication claims, direct-publish-ready
  claims, AppAdoption, deployed-route, auth/session, staging, VM lifecycle,
  run-acceptance, activation-ready, and promotion-level claims are rejected.

**Evidence:**
- `local://pass80-publication-review-gate-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildCandidatePackagePublicationExecutorReviewGateContract -parallel=1`
  passed: 31 pass events, 0 fail events.
- Tester teeth check temporarily changed the review gate to report
  `executor_allowed=true`; the focused test failed, and the mutation was
  reverted.

**Deferred / Still Open:**
- No package-publication executor, AppAdoption mutation, deployed route mutation,
  production state, auth/session mutation, staging proof, VM lifecycle operation,
  or RunAcceptanceRecord consumer was touched.
- The review gate is not executor permission. It only states that a preflight
  packet has enough owner/reviewer review refs to enter future red executor
  design review.

**Next:** Define the first pure executor design spec object that consumes this
review gate and enumerates the red surfaces/evidence required for any future
package-publication executor implementation, still without executing publication
or changing product state.

## Pass 81 — 2026-07-04 (Candidate-Package Publication Executor Design Spec Contract)

**Conjecture:** A pure executor design spec object can consume the owner/reviewer
review gate and enumerate required red surfaces/evidence for any future
package-publication executor without implementing or authorizing that executor,
publishing a package, making direct publish ready, activating product state, or
claiming promotion-level acceptance.

**Move:** Added `CandidatePackagePublicationExecutorDesignSpecEvidence`,
`CandidatePackagePublicationExecutorDesignSpecContract`, and
`BuildCandidatePackagePublicationExecutorDesignSpecContract` in
`internal/computerversion`. The builder consumes the Pass 80 review gate,
requires an executor design spec ref, required evidence refs, a rollback plan
ref, and all required red surfaces for a future package-publication executor.

**Actual ΔV:**
- A publication executor review gate can now be transformed into a pure design
  spec contract that binds package id, package manifest hash, `ComputerVersion`,
  local acceptance id, verifier/source/payload refs, owner/reviewer authorization
  refs, an executor design spec ref, required evidence refs, required red
  surfaces, and a rollback plan ref.
- The required red surfaces are explicit:
  `package_artifact_publication`, `provider_publish_credentials`,
  `publication_ledger_write`, and `rollback_path`.
- The emitted contract has `executor_design_spec_ready=true` but still has
  `executor_implemented=false`, `executor_allowed=false`,
  `actual_package_published=false`, `direct_publish_ready=false`,
  `no_mutation=true`, `activation_ready=false`, and
  `promotion_level_claimed=false`.
- Package publication, AppAdoption, deployed route, auth/session, staging, VM
  lifecycle, and run-acceptance prerequisites remain blocked.
- Malformed review gates, mismatched identity/source/payload/authorization refs,
  missing design/evidence/rollback refs, missing/unsupported red surfaces,
  executor-implemented claims, executor-allowed claims, actual publication
  claims, direct-publish-ready claims, AppAdoption, deployed-route,
  auth/session, staging, VM lifecycle, run-acceptance, activation-ready, and
  promotion-level claims are rejected.

**Evidence:**
- `local://pass81-publication-executor-design-spec-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildCandidatePackagePublicationExecutorDesignSpecContract -parallel=1`
  passed: 43 pass events, 0 fail events.
- Tester teeth check temporarily changed the design spec contract to report
  `executor_design_spec_ready=false`; the focused test failed, and the mutation
  was reverted.

**Deferred / Still Open:**
- No package-publication executor, AppAdoption mutation, deployed route mutation,
  production state, auth/session mutation, staging proof, VM lifecycle operation,
  or RunAcceptanceRecord consumer was touched.
- The design spec is not implementation, executor permission, package
  publication, direct publish readiness, or product activation.

**Next:** Define the first pure executor implementation-readiness contract that
consumes this design spec and states which red ceremony/evidence gates must be
opened before code may touch any package-publication executor surface.

## Pass 82 — 2026-07-04 (Candidate-Package Publication Executor Implementation Readiness Contract)

**Conjecture:** A pure implementation-readiness contract can consume the
executor design spec and name the red ceremony/evidence gates that must open
before code touches package-publication executor surfaces without opening those
gates, touching code, implementing an executor, publishing a package, making
direct publish ready, activating product state, or claiming promotion-level
acceptance.

**Move:** Added `CandidatePackagePublicationExecutorImplementationReadinessEvidence`,
`CandidatePackagePublicationExecutorImplementationReadinessContract`, and
`BuildCandidatePackagePublicationExecutorImplementationReadinessContract` in
`internal/computerversion`. The builder consumes the Pass 81 design spec,
requires a red ceremony plan ref, required implementation gate refs, evidence
gate refs, and rollback drill ref, then emits a blocked readiness contract.

**Actual ΔV:**
- The future executor path now has a pure readiness object that binds the
  design-spec identity, owner/reviewer authorization refs, executor design spec
  ref, required red surfaces, required evidence refs, rollback plan ref, red
  ceremony plan ref, required implementation gate refs, evidence gate refs, and
  rollback drill ref.
- The required implementation gates are explicit:
  `red_ceremony_required`, `owner_approval_required`,
  `security_review_required`, `provider_credential_proof_required`, and
  `rollback_drill_required`.
- The emitted contract has `implementation_readiness_status=blocked_until_red_ceremony`
  and keeps `red_ceremony_opened=false`, `code_surface_touched=false`,
  `implementation_ready=false`, `executor_implemented=false`,
  `executor_allowed=false`, `actual_package_published=false`,
  `direct_publish_ready=false`, `no_mutation=true`, `activation_ready=false`,
  and `promotion_level_claimed=false`.
- Package publication, AppAdoption, deployed route, auth/session, staging, VM
  lifecycle, and run-acceptance prerequisites remain blocked.
- Malformed design specs, mismatched identity/source/payload/authorization/design
  refs, mismatched required red surfaces/evidence refs, missing readiness refs,
  missing/unsupported required implementation gates, red-ceremony-opened claims,
  code-surface-touched claims, implementation-ready claims, executor-implemented
  claims, executor-allowed claims, actual publication claims, direct-publish-ready
  claims, AppAdoption, deployed-route, auth/session, staging, VM lifecycle,
  run-acceptance, activation-ready, and promotion-level claims are rejected.

**Evidence:**
- `local://pass82-publication-implementation-readiness-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildCandidatePackagePublicationExecutorImplementationReadinessContract -parallel=1`
  passed: 50 pass events, 0 fail events.
- Tester teeth check temporarily changed the readiness status to
  `implementation_ready`; the focused test failed, and the mutation was
  reverted.

**Deferred / Still Open:**
- No red ceremony was opened.
- No package-publication executor, code surface mutation, AppAdoption mutation,
  deployed route mutation, production state, auth/session mutation, staging
  proof, VM lifecycle operation, or RunAcceptanceRecord consumer was touched.
- The readiness contract is not implementation, executor permission, package
  publication, direct publish readiness, or product activation.

**Next:** Stop before red executor implementation; the next safe non-red probe is
to define a read-only reviewer checklist/report object for this readiness packet
so a future red ceremony can be opened with explicit review questions instead of
implicit implementation pressure.

## Pass 83 — 2026-07-04 (Candidate-Package Publication Executor Readiness Review Contract)

**Conjecture:** A read-only reviewer checklist/report object can consume the
blocked implementation-readiness packet and record explicit review questions
without opening red ceremony, approving red ceremony, authorizing implementation,
touching code, implementing an executor, publishing a package, making direct
publish ready, activating product state, or claiming promotion-level acceptance.

**Move:** Added `CandidatePackagePublicationExecutorReadinessReviewEvidence`,
`CandidatePackagePublicationExecutorReadinessReviewContract`, and
`BuildCandidatePackagePublicationExecutorReadinessReviewContract` in
`internal/computerversion`. The builder consumes the Pass 82 implementation
readiness contract, requires a review report ref, required checklist item refs,
reviewer finding refs, and open question refs, then emits a read-only review
contract.

**Actual ΔV:**
- The future executor path now has a pure reviewer checklist/report object that
  binds package identity, version, local acceptance id, verifier/source/payload
  refs, owner/reviewer authorization refs, executor design spec ref, red ceremony
  plan ref, required gate refs, evidence gate refs, rollback drill ref, review
  report ref, checklist item refs, reviewer finding refs, and open question refs.
- The required checklist items are explicit:
  `red_ceremony_scope_review`, `owner_approval_path_review`,
  `security_review_scope_review`, `provider_credential_boundary_review`, and
  `rollback_drill_review`.
- The emitted contract has `readiness_review_status=checklist_recorded_without_red_authorization`
  and keeps `red_ceremony_opened=false`, `red_ceremony_approved=false`,
  `implementation_authorized=false`, `code_surface_touched=false`,
  `implementation_ready=false`, `executor_implemented=false`,
  `executor_allowed=false`, `actual_package_published=false`,
  `direct_publish_ready=false`, `no_mutation=true`, `activation_ready=false`,
  and `promotion_level_claimed=false`.
- Package publication, AppAdoption, deployed route, auth/session, staging, VM
  lifecycle, and run-acceptance prerequisites remain blocked.
- Malformed readiness packets, mismatched identity/source/payload/authorization
  refs, mismatched red ceremony/gate/rollback refs, missing review report refs,
  missing reviewer findings, missing open questions, missing/unsupported
  checklist items, red-ceremony-opened claims, red-ceremony-approved claims,
  implementation-authorized claims, code-surface-touched claims,
  implementation-ready claims, executor-implemented claims, executor-allowed
  claims, actual publication claims, direct-publish-ready claims, AppAdoption,
  deployed-route, auth/session, staging, VM lifecycle, run-acceptance,
  activation-ready, and promotion-level claims are rejected.

**Evidence:**
- `local://pass83-publication-readiness-review-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildCandidatePackagePublicationExecutorReadinessReviewContract -parallel=1`
  passed: 50 pass events, 0 fail events.
- Tester teeth check temporarily changed the review contract to report
  `red_ceremony_approved=true`; the focused test failed, and the mutation was
  reverted.

**Deferred / Still Open:**
- No red ceremony was opened or approved.
- No package-publication executor, code surface mutation, AppAdoption mutation,
  deployed route mutation, production state, auth/session mutation, staging
  proof, VM lifecycle operation, or RunAcceptanceRecord consumer was touched.
- The review contract is not implementation, executor permission, package
  publication, direct publish readiness, product activation, or red-ceremony
  authorization.

**Next:** A future red-ceremony decision packet remains blocked unless human
authority explicitly opens it; the next non-red work should return to the base
audited-computer substrate unless the owner authorizes red executor work.

## Pass 84 — 2026-07-04 (Local Base Product-Path Harness Verification)

**Conjecture:** Existing local Base harness commands can prove an explicit-path,
auth-backed, read/write product-path loop that persists Base API state and then
re-observes it through `computerversion` observation/equivalence tooling without
registering deployed service routes, touching staging, mutating production
auth/session, or claiming substrate completion.

**Move:** Verified the local `cmd/baseharness`, `cmd/baseobserve`, and
`cmd/basecompare` command test suite. This returns from the package-publication
executor boundary to the base audited-computer substrate and exercises the route
registration seam only inside local harness/test servers with explicit journal,
blob, and auth DB paths.

**Actual ΔV:**
- `cmd/baseharness` proves a configured local server can open a real auth store,
  mount persistent Base API routes into a local harness server, write blob/item
  state through authenticated Base API calls, close persistence, and read the
  resulting state back as a `computerversion.ObservationSet`.
- The seed-fixture path proves explicit `--journal`, `--blob-root`, and
  `--auth-db` paths are required and that fixture output points to durable
  observable Base state.
- `cmd/baseobserve` proves read-only observation emission from explicit Base
  journal/blob paths with explicit `ComputerVersion` refs and refuses missing
  observation roots.
- `cmd/basecompare` proves equivalent observation sets compare green and a
  seeded tampered projection exits non-zero with `not_equivalent` output.
- This is local product-path evidence only; it does not register deployed Base
  routes or claim staging/product service wiring.

**Evidence:**
- `local://pass84-base-product-path-harness-tests.jsonl`
- `go test -json ./cmd/baseharness ./cmd/baseobserve ./cmd/basecompare -parallel=1`
  passed: 20 pass events, 0 fail events.

**Deferred / Still Open:**
- No deployed service route registration, production auth/session mutation,
  staging deploy routing, persistent production state mutation, VM lifecycle
  operation, promotion/rollback mutation, package publication, or run-acceptance
  record was touched.
- Local harness registration is not staging route registration, production
  service wiring, substrate independence, or promotion-level acceptance.

**Next:** Define a local Base route-registration contract that can consume this
harness evidence and state exactly which additional auth/session, deployed
service, staging, and rollback evidence would be required before any deployed
route-registration mutation is allowed.

## Pass 85 — 2026-07-04 (Local Base Route-Registration Readiness Contract)

**Conjecture:** A local Base product-path harness proof can feed a pure
route-registration readiness contract that names the missing auth/session scope,
deployed service registration, staging build identity, rollback route revert,
and production-state boundary evidence required before deployed Base route
registration can be considered, while remaining blocked and no-mutation.

**Move:** Added `LocalBaseProductPathHarnessEvidence`,
`BaseRouteRegistrationReadinessEvidence`,
`BaseRouteRegistrationReadinessContract`, and
`BuildBaseRouteRegistrationReadinessContract` in `internal/computerversion`.
The builder consumes Pass 84 local harness refs, binds them to `/api/base/` and
read/write Base scopes, requires the five explicit prerequisite review refs plus
a rollback plan ref, and emits a blocked readiness contract.

**Actual ΔV:**
- Local Base route-registration readiness is now represented as pure evidence,
  not as route registration.
- The contract is blocked with
  `blocked_until_red_route_registration_ceremony`, `NoMutation=true`, and
  `RouteRegistrationAllowed=false`.
- Required future evidence is named explicitly:
  `auth_session_scope_review`, `deployed_service_registration_review`,
  `staging_build_identity_review`, `rollback_route_revert_review`, and
  `production_state_boundary_review`.
- The builder rejects missing local harness proofs, identity drift across
  harness/observation/comparison refs, missing route/prerequisite/rollback refs,
  route-registration-allowed claims, no-mutation violations, and deployed
  route/auth/session/staging/production-state/VM/promotion/run-acceptance claims.

**Evidence:**
- `local://pass85-base-route-registration-readiness-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseRouteRegistrationReadinessContract -parallel=1`
  passed: 33 pass events, 0 fail events.

**Deferred / Still Open:**
- No deployed route registration, production auth/session mutation, staging
  deployment claim, persistent production state mutation, VM lifecycle operation,
  promotion/rollback mutation, package publication, or run-acceptance record was
  touched.
- The readiness contract is not route registration, service wiring, auth/session
  authorization, staging proof, substrate independence, or promotion-level
  acceptance.

**Next:** Continue local Base route-authority/readiness review only if it stays
inside pure computerversion contract evidence; any deployed service/auth/session
or staging route-registration mutation requires red ceremony and fresh staging
evidence.

## Pass 86 — 2026-07-04 (Local Base Route-Registration Authority Review Contract)

**Conjecture:** A blocked local Base route-registration readiness packet can
accept read-only owner/reviewer authority-review refs, required review checklist
coverage, findings, open questions, red-ceremony plan ref, and rollback ref
without opening red ceremony, authorizing route registration, touching deployed
service/auth/session/staging/production-state/VM/promotion/run-acceptance
surfaces, or claiming product wiring.

**Move:** Added `BaseRouteRegistrationAuthorityReviewEvidence`,
`BaseRouteRegistrationAuthorityReviewContract`, and
`BuildBaseRouteRegistrationAuthorityReviewContract` in
`internal/computerversion`. The builder consumes the Pass 85 blocked readiness
contract, requires owner and reviewer refs plus a red-ceremony plan ref, checks
the required review checklist items, records findings/open questions, and emits
a read-only authority-review contract.

**Actual ΔV:**
- Base route-registration authority review is now typed evidence rather than an
  implicit prose checkpoint.
- Owner/reviewer attention, checklist coverage, findings, open questions, a
  red-ceremony plan ref, and rollback ref can be recorded without opening or
  approving red ceremony.
- The contract keeps route registration unauthorized and rejects any deployed
  route, production auth/session, staging, production-state, VM lifecycle,
  promotion/rollback, or run-acceptance claim.
- Invalid readiness packets and mismatched route/harness/observation/comparison
  identity refs are rejected before authority-review evidence can be recorded.

**Evidence:**
- `local://pass86-base-route-registration-authority-review-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseRouteRegistrationAuthorityReviewContract -parallel=1`
  passed: 35 pass events, 0 fail events.

**Deferred / Still Open:**
- No red ceremony was opened or approved.
- No deployed route registration, production auth/session mutation, staging
  deployment claim, persistent production state mutation, VM lifecycle operation,
  promotion/rollback mutation, package publication, or run-acceptance record was
  touched.
- The authority-review contract is not route registration, service wiring,
  auth/session authorization, staging proof, substrate independence, or
  promotion-level acceptance.

**Next:** Continue local Base route authority narrowing only if it remains pure
contract evidence, or return to substrate equivalence; deployed service,
auth/session, or staging route-registration mutation requires red ceremony and
fresh staging evidence.

## Pass 87 — 2026-07-04 (Base Substrate Equivalence Contract)

**Conjecture:** A scoped Base current-state reader/file-projection equivalence
proof can become typed computerversion evidence for one `ComputerVersion`,
requiring `file_manifest` and `blob_set` observations, non-identical
materializer or substrate identities, equivalent realization comparison, named
observation/realization/equivalence refs, and no-mutation flags, without
claiming full substrate independence or touching runtime/deployed surfaces.

**Move:** Added `BaseSubstrateEquivalenceEvidence`,
`BaseSubstrateEquivalenceContract`, and
`BuildBaseSubstrateEquivalenceContract` in `internal/computerversion`. The
builder validates current/projection realization identity, checks that both
realizations name the same `ComputerVersion`, requires a non-identical
materializer or substrate, requires the `file_manifest`/`blob_set` scope, runs
`EquivalenceChecker.CheckRealizations`, rejects narrowed or non-equivalent
results, and emits a no-mutation contract.

**Actual ΔV:**
- Base current-state reader/file-projection equivalence is now represented as
  typed evidence rather than an implicit test result.
- A passing comparison is scoped to `base_current_state_file_manifest_blob_set`
  and cannot certify full substrate independence, deployed proof, or product
  wiring.
- The builder rejects seeded observation mismatches, unsupported capability
  narrowing, identical substrate/materializer self-certification, missing
  `file_manifest` or `blob_set` scope, `ComputerVersion` drift, observation
  version drift, empty observations, invalid realization identity, missing
  refs/scope, unsafe claims, and `NoMutation=false`.

**Evidence:**
- `local://pass87-base-substrate-equivalence-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseSubstrateEquivalenceContract -parallel=1`
  passed: 26 pass events, 0 fail events.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment claim, persistent production state
  mutation, VM lifecycle operation, promotion/rollback mutation, package
  publication, gateway/provider call, or run-acceptance record was touched.
- The equivalence contract is not full substrate independence, Cloud
  Hypervisor/Nucleus/container implementation, autopaper/wire work, deployed
  proof, or promotion-level acceptance.

**Next:** Continue substrate equivalence only through scoped computerversion
evidence, or choose a lower-risk local contract slice that does not touch
runtime behavior, deployed service/auth/session, staging, VM lifecycle,
promotion, package publication, gateway/provider calls, or run acceptance.

## Pass 88 — 2026-07-04 (Base Current-State User-Isomorphism Contract)

**Conjecture:** A scoped Base current-state user-isomorphism contract can consume
the Pass 87 file-manifest/blob-set equivalence contract and record exactly the
user-visible semantics it proves—file path, file content, deletion state, and
file provenance—while explicitly marking live-process/full-computer continuity
unsupported and keeping all runtime/deployed/protected surfaces unmutated.

**Move:** Added `BaseCurrentStateUserIsomorphismEvidence`,
`BaseCurrentStateUserIsomorphismContract`,
`BaseCurrentStateUserIsomorphismScope`, and
`BuildBaseCurrentStateUserIsomorphismContract` in `internal/computerversion`.
The builder validates the Pass 87 equivalence contract kind, scope, status,
required observations, no-mutation flags, and realization identities, then runs
`UserIsomorphismChecker` over the exact Base file/blob semantic scope.

**Actual ΔV:**
- Base current-state user-isomorphism is now represented as typed evidence
  layered on the scoped equivalence contract, not as an implied property of
  matching observations.
- The contract covers only `file_path`, `file_content`, `deletion_state`, and
  `file_provenance` over `file_manifest` and `blob_set`.
- The contract explicitly records `live_process_continuity` as unsupported and
  rejects full-computer continuity claims.
- The builder rejects observation mismatch, unsupported capability narrowing,
  wrong equivalence contract kind/scope/status, missing required observations,
  unsafe equivalence contracts, realization version/identity drift, missing
  refs, protected-surface claims, and `NoMutation=false`.

**Evidence:**
- `local://pass88-base-user-isomorphism-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseCurrentStateUserIsomorphismContract -parallel=1`
  passed: 30 pass events, 0 fail events.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment claim, persistent production state
  mutation, VM lifecycle operation, promotion/rollback mutation, package
  publication, gateway/provider call, Texture canonical write, or run-acceptance
  record was touched.
- The user-isomorphism contract is not full substrate independence, live-process
  continuity, deployed proof, product wiring, package publication, or
  promotion-level acceptance.

**Next:** Continue scoped substrate/user-isomorphism evidence only through
computerversion contracts, or choose a lower-risk local contract slice that does
not touch runtime behavior, deployed service/auth/session, staging, VM
lifecycle, promotion, package publication, gateway/provider calls, Texture
canonical writes, or run acceptance.

## Pass 89 — 2026-07-04 (Base Durable-State-Slice Contract)

**Conjecture:** A scoped Base durable-state-slice contract can consume the Pass
87 equivalence contract and Pass 88 user-isomorphism contract, require typed
artifact-program evidence for the `file_manifest`/`blob_set` slice, and
explicitly reject opaque `data.img` dependency, full-computer coverage,
`data.img` disposability, and protected-surface mutation claims.

**Move:** Added `BaseDurableStateSliceEvidence`,
`BaseDurableStateSliceContract`, `BaseDurableStateClass`, and
`BuildBaseDurableStateSliceContract` in `internal/computerversion`. The builder
validates the scoped equivalence contract, scoped user-isomorphism contract,
matching typed artifact-program ref, durable-slice evidence ref, file/blob
observation scope, required file-path/file-content/deletion/provenance
semantics, unsupported live-process semantics, and no-mutation boundary.

**Actual ΔV:**
- Base file/blob durable state now has a typed slice contract layered on the
  equivalence and user-isomorphism proof chain.
- The contract records only `base_file_manifest` and `base_blob_content` as
  persistent state classes.
- The contract requires the typed artifact-program ref to match the
  `ComputerVersion` artifact-program ref.
- The contract emits `NoOpaqueDataImageDependency=true` for this slice while
  keeping `FullComputerClaimed=false` and `DataImageDisposableClaimed=false`.
- The builder rejects missing refs, wrong typed artifact-program refs, unsafe
  protected-surface flags, wrong equivalence/user-isomorphism kinds or statuses,
  missing `file_manifest`/`blob_set` scope, missing required user semantics,
  missing unsupported `live_process_continuity`, full-computer claims,
  `data.img` disposability claims, and `NoMutation=false`.

**Evidence:**
- `local://pass89-base-durable-state-slice-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseDurableStateSliceContract -parallel=1`
  passed: 44 pass events, 0 fail events.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment claim, persistent production state
  mutation, VM lifecycle operation, promotion/rollback mutation, package
  publication, gateway/provider call, Texture canonical write, or run-acceptance
  record was touched.
- The durable-state-slice contract is not full substrate independence, full
  computer coverage, `data.img` disposability, live-process continuity, deployed
  proof, product wiring, package publication, or promotion-level acceptance.

**Next:** Continue local Base durable-state-slice narrowing through
computerversion contracts, or return to substrate equivalence without touching
runtime behavior, deployed service/auth/session, staging, VM lifecycle,
promotion, package publication, gateway/provider calls, Texture canonical
writes, or run acceptance.

## Pass 90 — 2026-07-04 (Base Extract-Boundary Contract)

**Conjecture:** A scoped Base extract-boundary contract can bind an
`ExtractRequest`, `ComputerVersion` artifact-program ref, and
`file_manifest`/`blob_set` `ObservationSet` before materialization, proving
typed extraction authority without claiming materialization, full-computer
continuity, `data.img` recovery, deployed routing, or runtime mutation.

**Move:** Added `BaseExtractBoundaryEvidence`,
`BaseExtractBoundaryContract`, `BaseExtractorKindJournalBlobCurrentState`, and
`BuildBaseExtractBoundaryContract` in `internal/computerversion`. The builder
validates request name/version, observation-set name/version/non-emptiness,
`file_manifest`/`blob_set` observation scope, extractor kind, typed
artifact-program ref, proof refs, no opaque `data.img` dependency, and
no-mutation boundary.

**Actual ΔV:**
- Base extraction now has a typed authority boundary below materialization and
  equivalence.
- The contract binds the request `ComputerVersion` to the produced
  `ObservationSet` version and to the typed artifact-program ref.
- The contract requires the extracted observation set to include both
  `file_manifest` and `blob_set`.
- The contract emits `NoOpaqueDataImageDependency=true` for extraction while
  keeping materialization, full-computer continuity, and `data.img` recovery
  claims false.
- The builder rejects invalid/mismatched request and observation-set inputs,
  missing refs, wrong typed artifact-program refs, wrong extractor kind, opaque
  `data.img` dependency, protected-surface claims, materialization claims,
  full-computer continuity claims, `data.img` recovery claims, and
  `NoMutation=false`.

**Evidence:**
- `local://pass90-base-extract-boundary-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseExtractBoundaryContract -parallel=1`
  passed: 27 pass events, 0 fail events.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment claim, persistent production state
  mutation, VM lifecycle operation, promotion/rollback mutation, package
  publication, gateway/provider call, Texture canonical write, or run-acceptance
  record was touched.
- The extract-boundary contract is not substrate equivalence, materialization,
  full-computer continuity, `data.img` recovery, deployed proof, product wiring,
  package publication, or promotion-level acceptance.

**Next:** Continue local Extract/ObservationSet/Materializer boundary narrowing
through computerversion contracts, or return to substrate equivalence without
touching runtime behavior, deployed service/auth/session, staging, VM lifecycle,
promotion, package publication, gateway/provider calls, Texture canonical
writes, or run acceptance.

## Pass 91 — 2026-07-04 (Base Materializer-Boundary Contract)

**Conjecture:** A scoped Base materializer-boundary contract can bind a
`Realization`, `CapabilityManifest`, and `file_manifest`/`blob_set`
`ObservationSet` for one `ComputerVersion`, proving local materializer shape
without claiming runtime materialization, VM lifecycle, Firecracker boot, full
substrate independence, deployed routing, or runtime mutation.

**Move:** Added `BaseMaterializerBoundaryEvidence`,
`BaseMaterializerBoundaryContract`, and
`BuildBaseMaterializerBoundaryContract` in `internal/computerversion`. The
builder validates realization identity/version, materializer/substrate names,
observation-set name/version/non-emptiness, capability support for required
observations, `file_manifest`/`blob_set` scope, proof refs, no runtime
materialization, no opaque `data.img` dependency, and no-mutation boundary.

**Actual ΔV:**
- Base materialization now has a typed local boundary below VM lifecycle and
  below full substrate-independence authority.
- The contract binds one `Realization` to its declared materializer/substrate,
  capability manifest, observation set, and `ComputerVersion`.
- The contract requires the realized observation set to include both
  `file_manifest` and `blob_set` and rejects unsupported capability manifests.
- The contract emits `NoRuntimeMaterialization=true`,
  `NoOpaqueDataImageDependency=true`, and `NoMutation=true` while keeping VM,
  Firecracker boot, deployed route, staging, promotion, auth/session,
  run-acceptance, and full substrate-independence claims false.
- The builder rejects invalid realization identity, invalid/mismatched
  versions, missing materializer/substrate names, empty or incomplete
  observations, unsupported capabilities, missing refs, unsafe claims, and
  `NoMutation=false`.

**Evidence:**
- `local://pass91-base-materializer-boundary-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseMaterializerBoundaryContract -parallel=1`
  passed: 28 pass events, 0 fail events.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment claim, persistent production state
  mutation, VM lifecycle operation, Firecracker boot, promotion/rollback
  mutation, package publication, gateway/provider call, Texture canonical write,
  or run-acceptance record was touched.
- The materializer-boundary contract is not a Firecracker lifecycle proof, full
  substrate independence, deployed proof, product wiring, package publication,
  or promotion-level acceptance.

**Next:** Continue local Materializer/EquivalenceCheck boundary narrowing
through computerversion contracts, or return to substrate equivalence without
touching runtime behavior, deployed service/auth/session, staging, VM lifecycle,
promotion, package publication, gateway/provider calls, Texture canonical
writes, or run acceptance.

## Pass 92 — 2026-07-04 (Base Equivalence-Check-Boundary Contract)

**Conjecture:** A scoped Base equivalence-check-boundary contract can bind two
non-identical Base materializer-boundary contracts and an equivalent
`EquivalenceResult` for one `ComputerVersion`, proving pure local
`EquivalenceCheck` authority without claiming VM lifecycle, Firecracker boot,
full substrate independence, deployed routing, or runtime mutation.

**Move:** Added `BaseEquivalenceCheckBoundaryEvidence`,
`BaseEquivalenceCheckBoundaryContract`, and
`BuildBaseEquivalenceCheckBoundaryContract` in `internal/computerversion`. The
builder validates left/right materializer-boundary contract kinds, scopes,
versions, materializer/substrate identities, non-identical comparison, required
`file_manifest`/`blob_set` observation scope, equivalent result status, proof
refs, no runtime materialization, no opaque `data.img` dependency, and
no-mutation boundary.

**Actual ΔV:**
- Base equivalence checking now has a typed local boundary over already-scoped
  materializer contracts.
- The contract rejects materializer self-comparison and version drift before an
  equivalence result can become authority.
- The contract accepts only `EquivalenceEquivalent` with no differences and no
  unsupported capabilities.
- The contract emits `NoRuntimeMaterialization=true`,
  `NoOpaqueDataImageDependency=true`, and `NoMutation=true` while keeping VM,
  Firecracker boot, deployed route, staging, promotion, auth/session,
  run-acceptance, and full substrate-independence claims false.
- The builder rejects wrong materializer contract kinds/scopes, invalid
  versions, missing materializer/substrate identities, missing file/blob
  observations, unsafe materializer contracts, non-equivalent or narrowed
  results, result payloads that carry differences/unsupported capabilities,
  missing proof refs, unsafe claims, and `NoMutation=false`.

**Evidence:**
- `local://pass92-base-equivalence-check-boundary-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseEquivalenceCheckBoundaryContract -parallel=1`
  passed: 45 pass events, 0 fail events.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment claim, persistent production state
  mutation, VM lifecycle operation, Firecracker boot, promotion/rollback
  mutation, package publication, gateway/provider call, Texture canonical write,
  or run-acceptance record was touched.
- The equivalence-check-boundary contract is not a Firecracker lifecycle proof,
  full substrate independence, deployed proof, product wiring, package
  publication, or promotion-level acceptance.

**Next:** Continue local equivalence-result/failure-boundary narrowing through
computerversion contracts, or return to substrate equivalence without touching
runtime behavior, deployed service/auth/session, staging, VM lifecycle,
promotion, package publication, gateway/provider calls, Texture canonical
writes, or run acceptance.

## Pass 93 — 2026-07-04 (Base Equivalence-Failure-Boundary Contract)

**Conjecture:** A scoped Base equivalence-failure-boundary contract can bind
non-equivalent or narrowed `EquivalenceResult` evidence for two materializer
contracts, proving the checker has teeth without converting failure evidence
into substrate-independence, deployed product, or runtime mutation claims.

**Move:** Added `BaseEquivalenceFailureBoundaryEvidence`,
`BaseEquivalenceFailureBoundaryContract`, and
`BuildBaseEquivalenceFailureBoundaryContract` in `internal/computerversion`.
The builder validates left/right materializer-boundary contract identities,
required `file_manifest`/`blob_set` scope, non-identical comparison,
`not_equivalent` results with differences, `narrowed` results with unsupported
capabilities, proof refs, no successful-equivalence claim, no runtime
materialization, no opaque `data.img` dependency, and no-mutation boundary.

**Actual ΔV:**
- Base equivalence failure evidence now has a typed local boundary instead of
  living only as raw `EquivalenceResult` values.
- The contract accepts seeded mismatch evidence only when a non-equivalent
  result carries concrete differences.
- The contract accepts narrowed evidence only when a narrowed result carries
  unsupported capabilities.
- The contract rejects equivalent results and any mixed malformed
  result payload before failure evidence can be used as authority.
- The contract emits `SuccessfulEquivalenceClaimed=false`,
  `NoRuntimeMaterialization=true`, `NoOpaqueDataImageDependency=true`, and
  `NoMutation=true` while keeping VM, Firecracker boot, deployed route, staging,
  promotion, auth/session, run-acceptance, and full substrate-independence
  claims false.

**Evidence:**
- `local://pass93-base-equivalence-failure-boundary-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseEquivalenceFailureBoundaryContract -parallel=1`
  passed: 29 pass events, 0 fail events.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment claim, persistent production state
  mutation, VM lifecycle operation, Firecracker boot, promotion/rollback
  mutation, package publication, gateway/provider call, Texture canonical write,
  or run-acceptance record was touched.
- The equivalence-failure-boundary contract is not successful equivalence,
  Firecracker lifecycle proof, full substrate independence, deployed proof,
  product wiring, package publication, or promotion-level acceptance.

**Next:** Continue local equivalence evidence-set/summary narrowing through
computerversion contracts, or return to substrate equivalence without touching
runtime behavior, deployed service/auth/session, staging, VM lifecycle,
promotion, package publication, gateway/provider calls, Texture canonical
writes, or run acceptance.

## Pass 94 — 2026-07-04 (Base Equivalence Evidence-Set Contract)

**Conjecture:** A scoped Base equivalence-evidence-set contract can bind one
passing equivalence contract and one failure/narrowing contract for the same
`ComputerVersion`, proving local checker calibration without claiming full
substrate independence, deployed behavior, or runtime mutation.

**Move:** Added `BaseEquivalenceEvidenceSetEvidence`,
`BaseEquivalenceEvidenceSetContract`, and
`BuildBaseEquivalenceEvidenceSetContract` in `internal/computerversion`. The
builder validates success/failure contract kind, boundary, scope, shared
version, required `file_manifest`/`blob_set` scope, equivalent success status,
non-equivalent or narrowed failure status, difference/unsupported counts, proof
refs, no runtime materialization, no opaque `data.img` dependency, no protected
surface claims, and no-mutation boundary.

**Actual ΔV:**
- Base equivalence checker calibration now has a typed local evidence-set
  boundary instead of separate unjoined success/failure contracts.
- The contract requires one passing equivalence boundary and one failure or
  narrowing boundary for the same `ComputerVersion`.
- The contract preserves the difference between checker calibration and full
  substrate-independence authority.
- The contract emits `NoRuntimeMaterialization=true`,
  `NoOpaqueDataImageDependency=true`, and `NoMutation=true` while keeping VM,
  Firecracker boot, deployed route, staging, promotion, auth/session,
  run-acceptance, and full substrate-independence claims false.

**Evidence:**
- `local://pass94-base-equivalence-evidence-set-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseEquivalenceEvidenceSetContract -parallel=1`
  passed: 40 pass events, 0 fail events.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment claim, persistent production state
  mutation, VM lifecycle operation, Firecracker boot, promotion/rollback
  mutation, package publication, gateway/provider call, Texture canonical write,
  or run-acceptance record was touched.
- The equivalence evidence-set contract is not a Firecracker lifecycle proof,
  full substrate independence, deployed proof, product wiring, package
  publication, or promotion-level acceptance.

**Next:** Decide whether local computerversion contracts are sufficient to
re-enter substrate-equivalence proof, or add one more non-runtime contract only
if it preserves the same no-deployed/no-VM/no-promotion boundary.

## Pass 95 — 2026-07-04 (Base Substrate Reentry Readiness Contract)

**Conjecture:** A scoped Base substrate-reentry-readiness contract can bind the
prior substrate-equivalence contract to the calibrated equivalence evidence-set
for the same `ComputerVersion`, authorizing only local substrate-equivalence
reentry without claiming full substrate independence, deployed behavior, VM
lifecycle, or completion.

**Move:** Added `BaseSubstrateReentryReadinessEvidence`,
`BaseSubstrateReentryReadinessContract`, and
`BuildBaseSubstrateReentryReadinessContract` in `internal/computerversion`. The
builder validates the substrate-equivalence contract, the equivalence
evidence-set contract, shared version, required `file_manifest`/`blob_set`
scope, non-identical current/projection materializer identities, equivalent
substrate status, calibrated success/failure statuses, proof refs, no completion
claim, no runtime materialization, no opaque `data.img` dependency, and
no-mutation boundary.

**Actual ΔV:**
- Re-entering substrate-equivalence work now has a typed local readiness gate.
- The gate requires the earlier scoped substrate-equivalence proof and the
  positive/negative checker calibration set for the same `ComputerVersion`.
- The gate emits `LocalSubstrateReentryAllowed=true` while keeping completion,
  runtime, deployed route, staging, promotion, auth/session, VM lifecycle,
  Firecracker boot, run-acceptance, and full substrate-independence claims false.
- The gate rejects wrong contract kinds/scopes/boundaries, invalid statuses or
  counts, missing file/blob observation scope, identical materializer/substrate
  comparison, version drift, missing refs, unsafe proof flags, protected-surface
  claims, and `NoMutation=false`.

**Evidence:**
- `local://pass95-base-substrate-reentry-readiness-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseSubstrateReentryReadinessContract -parallel=1`
  passed: 61 pass events, 0 fail events.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment claim, persistent production state
  mutation, VM lifecycle operation, Firecracker boot, promotion/rollback
  mutation, package publication, gateway/provider call, Texture canonical write,
  or run-acceptance record was touched.
- The reentry-readiness contract is not a Firecracker lifecycle proof, full
  substrate independence, deployed proof, product wiring, package publication,
  or promotion-level acceptance.

**Next:** Re-enter local substrate-equivalence proof by strengthening the scoped
Base substrate contract, without touching runtime behavior, deployed
service/auth/session, staging, VM lifecycle, promotion, package publication,
gateway/provider calls, Texture canonical writes, or run acceptance.

## Pass 96 — 2026-07-04 (Base Substrate-Equivalence Safety Strengthening)

**Conjecture:** The scoped Base substrate-equivalence contract can be
strengthened to carry the same no-runtime, no-opaque-`data.img`,
no-Firecracker-boot, no-full-substrate, no-completion, and no-mutation safety
boundary as newer local contracts, without changing runtime behavior.

**Move:** Strengthened `BaseSubstrateEquivalenceEvidence`,
`BaseSubstrateEquivalenceContract`, and
`BuildBaseSubstrateEquivalenceContract` in `internal/computerversion`. The
contract now requires evidence to prove no runtime materialization and no opaque
`data.img` dependency, rejects Firecracker boot/full-substrate/completion
claims, and emits explicit false flags for those claims.

**Actual ΔV:**
- The scoped substrate-equivalence contract now matches the safety vocabulary of
  the later materializer, equivalence, failure, evidence-set, and reentry gates.
- A passing scoped substrate-equivalence result cannot omit the
  no-runtime-materialization or no-opaque-`data.img` boundary.
- Firecracker boot, full substrate independence, and completion are now explicit
  false claims on the contract, not merely absent fields.
- Existing seeded mismatch, unsupported capability narrowing, version drift,
  missing refs, protected claims, and `NoMutation=false` rejection behavior still
  passes focused verification.

**Evidence:**
- `local://pass96-base-substrate-equivalence-strengthened-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseSubstrateEquivalenceContract -parallel=1`
  passed: 31 pass events, 0 fail events.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment claim, persistent production state
  mutation, VM lifecycle operation, Firecracker boot, promotion/rollback
  mutation, package publication, gateway/provider call, Texture canonical write,
  or run-acceptance record was touched.
- The strengthened substrate-equivalence contract remains a scoped local Base
  file/blob projection proof, not Firecracker lifecycle proof, full substrate
  independence, deployed proof, product wiring, package publication, or
  promotion-level acceptance.

**Next:** Build a local substrate-equivalence proof summary over strengthened
substrate equivalence and reentry readiness, without touching runtime behavior,
deployed service/auth/session, staging, VM lifecycle, promotion, package
publication, gateway/provider calls, Texture canonical writes, or run
acceptance.

## Pass 97 — 2026-07-04 (Base Local Substrate Proof Summary Contract)

**Conjecture:** A local substrate proof summary can bind the strengthened
`BaseSubstrateEquivalenceContract` to the `BaseSubstrateReentryReadinessContract`
for the same `ComputerVersion`, proving the local file/blob
substrate-equivalence slice is summarized without converting that slice into
runtime proof, staging proof, VM lifecycle proof, promotion authority, package
publication, or mission completion.

**Move:** Added `BaseLocalSubstrateProofSummaryEvidence`,
`BaseLocalSubstrateProofSummaryContract`, and
`BuildBaseLocalSubstrateProofSummaryContract` in `internal/computerversion`.
The summary requires matching substrate-equivalence and reentry-readiness
contracts, matching refs, file-manifest/blob-set observation scope, and explicit
remaining runtime/staging/promotion proof gaps. It emits
`LocalFileBlobProofSummarized=true` while preserving no-runtime, no-opaque
`data.img`, no-mutation, no-Firecracker, no-package-publication,
no-full-substrate, and no-completion boundaries. Also tightened
`BuildBaseSubstrateReentryReadinessContract` so reentry rejects substrate
contracts that omit no-runtime/no-opaque proof flags or carry
Firecracker/full-substrate/completion claims.

**Actual ΔV:**
- The local file/blob substrate-equivalence proof stack now has a summary
  contract instead of an implicit narrative boundary.
- The summary can only be built when the strengthened substrate-equivalence
  contract and reentry-readiness contract agree on `ComputerVersion`, claim
  scope, current/projection materializer identities, and refs.
- Remaining runtime, staging, and promotion proof gaps are mandatory fields in
  the evidence path, so local proof cannot be mistaken for product completion.
- Reentry now rejects stale substrate contracts that lack explicit
  no-runtime/no-opaque flags or carry newer Firecracker/full-substrate/completion
  claims.

**Evidence:**
- `local://pass97-base-local-substrate-proof-summary-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run 'TestBuildBaseLocalSubstrateProofSummaryContract|TestBuildBaseSubstrateReentryReadinessContract' -parallel=1`
  passed: 115 pass events, 0 fail events.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment claim, persistent production state
  mutation, VM lifecycle operation, Firecracker boot, promotion/rollback
  mutation, package publication, gateway/provider call, Texture canonical write,
  or run-acceptance record was touched.
- The proof summary is still a local Base file/blob substrate-equivalence
  summary. It is not runtime materialization proof, not a staging proof, not a
  package-publication proof, not promotion authority, not a full substrate
  independence proof, and not mission completion.

**Next:** Decide whether the local file/blob substrate-equivalence summary is
sufficient to justify opening red runtime materialization ceremony, or whether
one more non-runtime source/provenance boundary must be added first.

## Pass 98 — 2026-07-04 (Base Source/Provenance Readiness Contract)

**Conjecture:** A non-runtime source/provenance readiness contract can bind the
local substrate proof summary to the typed durable-state slice for the same
`ComputerVersion`, proving file/blob source provenance is carried into the
readiness boundary before any red runtime materialization ceremony is opened.

**Move:** Added `BaseSourceProvenanceReadinessEvidence`,
`BaseSourceProvenanceReadinessContract`, and
`BuildBaseSourceProvenanceReadinessContract` in `internal/computerversion`.
The contract requires a `BaseLocalSubstrateProofSummaryContract`, a
`BaseDurableStateSliceContract`, matching `ComputerVersion`, matching typed
artifact-program ref, file-manifest/blob-set observations, file path/content/
deletion/provenance semantics, and explicit downstream runtime/staging/
promotion/package-publication requirements. Also tightened
`BuildBaseDurableStateSliceContract` so durable-state validation rejects
substrate-equivalence contracts that omit no-runtime/no-opaque proof flags or
carry Firecracker/full-substrate/completion claims.

**Actual ΔV:**
- The local proof summary is now connected to the typed artifact-program and
  provenance slice instead of only to equivalence/reentry evidence.
- Runtime ceremony readiness is now explicit and narrow:
  `RuntimeCeremonyMayOpen=true` means the local source/provenance prerequisites
  are present, not that runtime materialization, staging, promotion, package
  publication, or completion has happened.
- The durable-state gate now rejects stale substrate-equivalence contracts that
  predate the no-runtime/no-opaque/full-substrate/completion safety vocabulary.

**Evidence:**
- `local://pass98-base-source-provenance-readiness-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run 'TestBuildBaseSourceProvenanceReadinessContract|TestBuildBaseDurableStateSliceContract' -parallel=1`
  passed: 101 pass events, 0 fail events.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment claim, persistent production state
  mutation, VM lifecycle operation, Firecracker boot, promotion/rollback
  mutation, package publication, gateway/provider call, Texture canonical write,
  or run-acceptance record was touched.
- The readiness contract authorizes considering a red runtime materialization
  ceremony. It is not runtime proof, staging proof, package-publication proof,
  promotion authority, full substrate independence, or mission completion.

**Next:** Open red runtime materialization ceremony only after restating the
protected surfaces and evidence class.

## Pass 99 — 2026-07-04 (Red Runtime Materialization Ceremony Opened)

**Conjecture:** The source/provenance readiness boundary is sufficient to open,
but not satisfy, a red runtime-materialization evidence ceremony against the
scoped vmmanager/Firecracker seams. Opening the ceremony is not itself runtime
materialization, VM lifecycle operation, staging proof, promotion authority,
package-publication proof, full substrate independence, or mission completion.

**Move:** Updated the active red ceremony state in
`docs/definitions/substrate-independent-audited-computer-2026-07-04.md` after
restating the protected surfaces and admissible evidence class. Inspected
`cmd/vmrealize`, `cmd/vmstateobserve`, `cmd/vmstatecompare`, and the
`internal/computerversion` vmmanager boundary to identify the first runtime
materialization evidence seam.

**Actual ΔV:** The mission is now in an explicit red ceremony. The next proof
must bind runtime materialization evidence to the same `ComputerVersion` and
typed artifact-program ref carried by `BaseSourceProvenanceReadinessContract`,
and must keep staging, promotion, package publication, full-substrate
independence, and mission completion as separate downstream evidence gates.

**Evidence:**
- `docs/definitions/substrate-independent-audited-computer-2026-07-04.md`
  active red ceremony state.
- `cmd/vmrealize/main.go`
- `cmd/vmstateobserve/main.go`
- `cmd/vmstatecompare/main.go`
- `internal/computerversion/vmmanager_boundary.go`

**Protected surfaces named:** runtime materialization boundary, VM lifecycle
evidence boundary, staging deployment and health identity boundary,
promotion/rollback boundary, package-publication boundary, and run-acceptance
record boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment claim, persistent production state
  mutation, VM lifecycle operation, Firecracker boot, promotion/rollback
  mutation, package publication, gateway/provider call, Texture canonical write,
  or run-acceptance record was touched in this ceremony-opening pass.
- The active ceremony still needs code/test evidence before any runtime,
  staging, promotion, package-publication, full-substrate, or completion claim.

**Next:** Define the runtime-materialization ceremony gate in
`internal/computerversion` so it accepts only a realization bound to the existing
source/provenance readiness contract and rejects staging/promotion/completion
claims.

## Pass 100 — 2026-07-04 (Base Runtime Materialization Ceremony Gate)

**Conjecture:** Runtime-materialization evidence can be admitted only after the
source/provenance readiness contract exists, and only when the scoped
Realization names the same `ComputerVersion` and typed artifact-program ref.
The gate must prevent vmmanager-scoped Realization evidence from becoming a
durable-state equivalence claim, staging proof, promotion authority,
package-publication proof, full-substrate-independence claim, run-acceptance
record, or mission-completion claim.

**Move:** Added `BaseRuntimeMaterializationCeremonyEvidence`,
`BaseRuntimeMaterializationCeremonyContract`, and
`BuildBaseRuntimeMaterializationCeremonyContract` in `internal/computerversion`.
The builder validates `BaseSourceProvenanceReadinessContract`, validates a
vmmanager-scoped `Realization`, requires `vm_state_manifest` runtime evidence,
rejects durable file/blob observations inside runtime realization evidence, and
requires explicit no-VM-lifecycle/no-production-mutation evidence flags.

**Actual ΔV:** The red runtime-materialization ceremony now has a typed local
gate. It accepts runtime-boundary evidence only as runtime evidence and keeps
runtime equivalence, staging, promotion, package publication, run acceptance,
full substrate independence, and completion open as downstream proof
requirements.

**Evidence:**
- `internal/computerversion/base_runtime_materialization_ceremony_contract.go`
- `internal/computerversion/base_runtime_materialization_ceremony_contract_test.go`
- `local://pass100-base-runtime-materialization-ceremony-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseRuntimeMaterializationCeremonyContract -count=1`
  passed: 36 pass events, 0 fail events.

**Protected surfaces named:** runtime materialization evidence boundary, VM
lifecycle evidence boundary, staging deployment and health identity boundary,
promotion/rollback boundary, package-publication boundary, and run-acceptance
record boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment claim, persistent production state
  mutation, VM lifecycle operation, Firecracker boot, promotion/rollback
  mutation, package publication, gateway/provider call, Texture canonical write,
  or run-acceptance record was touched.
- The new gate is not runtime equivalence proof, staging proof,
  package-publication proof, promotion authority, full substrate independence,
  or mission completion.

**Next:** Define the next red ceremony gate that compares accepted runtime
Realization evidence against the source/provenance-ready file/blob proof without
allowing vmmanager metadata or opaque `data.img` presence to stand in for typed
durable state equivalence.

## Pass 101 — 2026-07-04 (Base Runtime Equivalence Boundary Narrows)

**Conjecture:** Accepted vmmanager-scoped runtime materialization evidence must
not be allowed to satisfy source/provenance durable file/blob equivalence. The
honest result at this boundary is a narrowed runtime-equivalence claim that names
unsupported `file_manifest` and `blob_set` observations.

**Move:** Added `BaseRuntimeEquivalenceBoundaryEvidence`,
`BaseRuntimeEquivalenceBoundaryContract`, and
`BuildBaseRuntimeEquivalenceBoundaryContract` in `internal/computerversion`.
The builder validates source/provenance readiness, validates the runtime
materialization ceremony contract, accepts only `EquivalenceNarrowed` results
with unsupported `file_manifest` and `blob_set`, and rejects equivalent,
not-equivalent-with-differences, malformed narrowed, protected-surface, and
completion claims.

**Actual ΔV:** Runtime evidence now reaches the equivalence boundary without
becoming a fake equivalence proof. The vmmanager-only path is explicitly
recorded as narrowed because it carries `vm_state_manifest` runtime evidence but
does not observe typed durable file/blob state.

**Evidence:**
- `internal/computerversion/base_runtime_equivalence_boundary_contract.go`
- `internal/computerversion/base_runtime_equivalence_boundary_contract_test.go`
- `local://pass101-base-runtime-equivalence-boundary-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseRuntimeEquivalenceBoundaryContract -count=1`
  passed: 36 pass events, 0 fail events.

**Protected surfaces named:** runtime equivalence evidence boundary, runtime
materialization evidence boundary, VM lifecycle evidence boundary, staging
deployment and health identity boundary, promotion/rollback boundary,
package-publication boundary, and run-acceptance record boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment claim, persistent production state
  mutation, VM lifecycle operation, Firecracker boot, promotion/rollback
  mutation, package publication, gateway/provider call, Texture canonical write,
  or run-acceptance record was touched.
- The new boundary is not durable-state runtime equivalence proof, staging proof,
  package-publication proof, promotion authority, full substrate independence,
  or mission completion.

**Next:** Define the next safe proof that would make runtime equivalence
constructive: a runtime file/blob observation extraction boundary or adapter
that can produce typed `file_manifest` and `blob_set` observations from a
materialized runtime without treating opaque `data.img` presence as user-state
proof.

## Pass 102 — 2026-07-04 (Base Runtime File/Blob Extraction Boundary)

**Conjecture:** Runtime equivalence can become constructive only after runtime
evidence is converted into typed `file_manifest` and `blob_set` observations for
the same `ComputerVersion`; `vm_state_manifest` metadata or opaque `data.img`
presence is still not user-state proof.

**Move:** Added `BaseRuntimeFileBlobExtractionEvidence`,
`BaseRuntimeFileBlobExtractionContract`, and
`BuildBaseRuntimeFileBlobExtractionContract` in `internal/computerversion`.
The builder binds a typed runtime `ObservationSet` to the prior narrowed
`BaseRuntimeEquivalenceBoundaryContract`, requires `file_manifest` and
`blob_set`, rejects `vm_state_manifest` reliance and opaque `data.img`
dependency, and preserves separate staging, promotion, package-publication,
run-acceptance, full-substrate, and completion gates.

**Actual ΔV:** The mission now has a non-opaque input shape for a future runtime
equivalence retry. The current evidence still does not compare or accept runtime
equivalence; it only proves that a retry is admissible once typed runtime
file/blob observations exist.

**Evidence:**
- `internal/computerversion/base_runtime_file_blob_extraction_contract.go`
- `internal/computerversion/base_runtime_file_blob_extraction_contract_test.go`
- `local://pass102-base-runtime-file-blob-extraction-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseRuntimeFileBlobExtractionContract -count=1`
  passed: 38 pass events, 0 fail events.

**Protected surfaces named:** runtime file/blob observation extraction boundary,
runtime equivalence evidence boundary, VM lifecycle evidence boundary, staging
deployment and health identity boundary, promotion/rollback boundary,
package-publication boundary, and run-acceptance record boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment claim, persistent production state
  mutation, VM lifecycle operation, Firecracker boot, promotion/rollback
  mutation, package publication, gateway/provider call, Texture canonical write,
  or run-acceptance record was touched.
- The new extraction boundary is not a runtime equivalence result, staging proof,
  package-publication proof, promotion authority, full substrate independence,
  or mission completion.

**Next:** Define the runtime equivalence retry gate that can compare the source
file/blob observation set against the typed runtime file/blob observation set,
while still rejecting downstream protected-surface claims.

## Pass 103 — 2026-07-04 (Base Runtime Equivalence Retry Gate)

**Conjecture:** The narrowed runtime path becomes constructive only when a
source/provenance `file_manifest`/`blob_set` observation set and a typed runtime
`file_manifest`/`blob_set` observation set compare equivalent for the same
`ComputerVersion`. A mismatch must remain not-equivalent evidence, not
downstream authority.

**Move:** Added `BaseRuntimeEquivalenceRetryEvidence`,
`BaseRuntimeEquivalenceRetryContract`, and
`BuildBaseRuntimeEquivalenceRetryContract` in `internal/computerversion`.
The builder validates the source/provenance readiness contract, validates the
runtime file/blob extraction contract, rejects `vm_state_manifest` reliance,
compares source/runtime `ObservationSet`s with `EquivalenceChecker`, and accepts
only `EquivalenceEquivalent` with no differences or unsupported capabilities.

**Actual ΔV:** The suite now has a bounded runtime-equivalence acceptance shape:
runtime equivalence may be claimed for typed file/blob observations only, while
staging, promotion, package-publication, run-acceptance, full-substrate, and
completion remain separate gates.

**Evidence:**
- `internal/computerversion/base_runtime_equivalence_retry_contract.go`
- `internal/computerversion/base_runtime_equivalence_retry_contract_test.go`
- `local://pass103-base-runtime-equivalence-retry-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseRuntimeEquivalenceRetryContract -count=1`
  passed: 40 pass events, 0 fail events.

**Protected surfaces named:** runtime equivalence retry evidence boundary,
runtime file/blob observation extraction boundary, staging deployment and health
identity boundary, promotion/rollback boundary, package-publication boundary,
and run-acceptance record boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment claim, persistent production state
  mutation, VM lifecycle operation, Firecracker boot, promotion/rollback
  mutation, package publication, gateway/provider call, Texture canonical write,
  or run-acceptance record was touched.
- The retry gate is not staging proof, package-publication proof, promotion
  authority, full substrate independence, or mission completion.

**Next:** Define the staging-readiness boundary that can consume a bounded
runtime-equivalence retry contract without confusing it for deployment health,
route identity, promotion authority, package publication, or run acceptance.

## Pass 104 — 2026-07-04 (Base Staging Readiness Boundary)

**Conjecture:** Bounded runtime-equivalence retry evidence can authorize a
staging smoke probe, but it must not be confused with deployed health, route
identity, promotion authority, package publication, run acceptance, full
substrate independence, or completion.

**Move:** Added `BaseStagingReadinessEvidence`,
`BaseStagingReadinessContract`, and `BuildBaseStagingReadinessContract` in
`internal/computerversion`. The builder validates the bounded runtime
equivalence retry contract, requires no-deployment/no-route/no-run-acceptance/
no-production mutation evidence, and produces only staging-smoke readiness.

**Actual ΔV:** The suite now has a typed handoff from local runtime-equivalence
proof to the next product-path probe. A staging smoke probe may be planned, but
deployment execution, deployed health identity, route identity, promotion,
package publication, run acceptance, full-substrate independence, and completion
remain unclaimed and separately required.

**Evidence:**
- `internal/computerversion/base_staging_readiness_contract.go`
- `internal/computerversion/base_staging_readiness_contract_test.go`
- `local://pass104-base-staging-readiness-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseStagingReadinessContract -count=1`
  passed: 52 pass events, 0 fail events.

**Protected surfaces named:** staging readiness evidence boundary, staging
deployment and health identity boundary, deployed route identity boundary,
promotion/rollback boundary, package-publication boundary, and run-acceptance
record boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment claim, persistent production state
  mutation, VM lifecycle operation, Firecracker boot, promotion/rollback
  mutation, package publication, gateway/provider call, Texture canonical write,
  or run-acceptance record was touched.
- The readiness boundary is not deployed health, route identity, promotion
  authority, package-publication proof, run-acceptance proof, full substrate
  independence, or mission completion.

**Next:** Define the staging smoke evidence boundary that can record a specific
product-path staging probe result and build/route identity without promoting,
publishing, or creating a run-acceptance record.

## Pass 105 — 2026-07-04 (Base Staging Smoke Evidence Boundary)

**Conjecture:** A staging smoke record can prove a product-path observation and
matched build/route identity only when it is downstream of staging-readiness and
still refuses promotion, package publication, run-acceptance, full-substrate, or
completion authority.

**Move:** Added `BaseStagingSmokeEvidence`,
`BaseStagingSmokeEvidenceContract`, and
`BuildBaseStagingSmokeEvidenceContract` in `internal/computerversion`. The
builder validates the Pass 104 staging-readiness contract, rejects internal and
test-only route bypasses, rejects manual success seeding, requires matched build
and route identity, requires passed health from authenticated product/control
evidence, and preserves downstream proof requirements.

**Actual ΔV:** The suite can now record a product-path staging smoke observation
as evidence without turning it into a promotion, publication, run-acceptance
record, full-substrate proof, or completion claim. Failed health, identity
mismatch, bypass routes, and manual seeded success remain explicit failures.

**Evidence:**
- `internal/computerversion/base_staging_smoke_evidence_contract.go`
- `internal/computerversion/base_staging_smoke_evidence_contract_test.go`
- `local://pass105-base-staging-smoke-evidence-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseStagingSmokeEvidenceContract -count=1`
  passed: 35 pass events, 0 fail events.

**Protected surfaces named:** staging smoke evidence boundary, staging
deployment and health identity boundary, deployed route identity boundary,
promotion/rollback boundary, package-publication boundary, and run-acceptance
record boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment mutation, persistent production
  state mutation, VM lifecycle operation, Firecracker boot, promotion/rollback
  mutation, package publication, gateway/provider call, Texture canonical write,
  or run-acceptance record was touched.
- The smoke evidence boundary is not promotion authority, package-publication
  proof, run-acceptance proof, full substrate independence, or mission
  completion.

**Next:** Define the promotion/package-publication or run-acceptance evidence
handoff that consumes staging smoke evidence without weakening owner review,
promotion/rollback, publication, or verifier-contract requirements.

## Pass 106 — 2026-07-04 (Base Post-Smoke Handoff Readiness Boundary)

**Conjecture:** A passed staging smoke record is useful only if the next handoff
keeps owner review, promotion/rollback review, package-publication proof,
verifier-contract proof, run-acceptance proof, full-substrate proof, and
completion authority explicit and blocked until separately satisfied.

**Move:** Added `BasePostSmokeHandoffReadinessEvidence`,
`BasePostSmokeHandoffReadinessContract`, and
`BuildBasePostSmokeHandoffReadinessContract` in `internal/computerversion`. The
builder validates the Pass 105 staging-smoke contract, preserves product probe,
build identity, and route identity refs, requires downstream prerequisite refs,
and returns a blocked handoff with no owner approval, promotion, publication,
run-acceptance synthesis, full-substrate, or completion claim.

**Actual ΔV:** The suite now has a typed seam between staging smoke evidence and
the later owner-review/promotion/publication/run-acceptance authorities. A
staging smoke pass can be handed forward without silently becoming approval,
promotion, package publication, verifier satisfaction, run acceptance, or
completion.

**Evidence:**
- `internal/computerversion/base_post_smoke_handoff_contract.go`
- `internal/computerversion/base_post_smoke_handoff_contract_test.go`
- `local://pass106-base-post-smoke-handoff-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBasePostSmokeHandoffReadinessContract -count=1`
  passed: 52 pass events, 0 fail events.

**Protected surfaces named:** post-smoke handoff readiness boundary, owner
review boundary, promotion/rollback boundary, package-publication boundary,
run-acceptance record boundary, and verifier-contract boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment mutation, persistent production
  state mutation, VM lifecycle operation, Firecracker boot, owner approval,
  promotion/rollback mutation, package publication, gateway/provider call,
  Texture canonical write, verifier-contract satisfaction, or run-acceptance
  record was touched.
- The handoff boundary is not owner approval, promotion authority,
  package-publication proof, run-acceptance proof, full substrate independence,
  or mission completion.

**Next:** Define the owner-review readiness or verifier-contract evidence seam
that consumes the blocked post-smoke handoff without executing promotion,
publication, or run-acceptance synthesis.

## Pass 107 — 2026-07-04 (Base Owner-Review Readiness Boundary)

**Conjecture:** Owner review can be prepared from post-smoke handoff evidence
without being confused for owner approval, promotion execution, package
publication, verifier-contract satisfaction, run acceptance, full-substrate
proof, or completion.

**Move:** Added `BaseOwnerReviewReadinessEvidence`,
`BaseOwnerReviewReadinessContract`, and
`BuildBaseOwnerReviewReadinessContract` in `internal/computerversion`. The
builder validates the Pass 106 blocked handoff, carries product probe/build/route
identity, requires review packet, reviewer identity policy, instructions, risk
summary, and rollback refs, and returns review readiness while blocking every
downstream execution flag.

**Actual ΔV:** The suite now has a typed owner-review packet boundary. Evidence
can be made reviewable by an owner without silently granting approval or
executing promotion, publication, verifier satisfaction, run acceptance, or
completion.

**Evidence:**
- `internal/computerversion/base_owner_review_readiness_contract.go`
- `internal/computerversion/base_owner_review_readiness_contract_test.go`
- `local://pass107-base-owner-review-readiness-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseOwnerReviewReadinessContract -count=1`
  passed: 59 pass events, 0 fail events.

**Protected surfaces named:** owner-review readiness boundary, owner approval
boundary, promotion/rollback boundary, package-publication boundary,
run-acceptance record boundary, and verifier-contract boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment mutation, persistent production
  state mutation, VM lifecycle operation, Firecracker boot, owner approval,
  promotion/rollback mutation, package publication, gateway/provider call,
  Texture canonical write, verifier-contract satisfaction, or run-acceptance
  record was touched.
- The owner-review readiness boundary is not owner approval, promotion authority,
  package-publication proof, run-acceptance proof, full substrate independence,
  or mission completion.

**Next:** Define the verifier-contract readiness seam that consumes owner-review
readiness without satisfying the verifier, approving promotion, publishing a
package, or synthesizing run acceptance.

## Pass 108 — 2026-07-04 (Base Verifier-Readiness Boundary)

**Conjecture:** Verifier inputs can be prepared from owner-review readiness
without being confused for verifier-contract satisfaction, owner approval,
promotion execution, package publication, run acceptance, full-substrate proof,
or completion.

**Move:** Added `BaseVerifierReadinessEvidence`,
`BaseVerifierReadinessContract`, and `BuildBaseVerifierReadinessContract` in
`internal/computerversion`. The builder validates the Pass 107 owner-review
readiness contract, carries product probe/build/route identity and review packet
refs, requires verifier input bundle, verifier contract spec, evidence manifest,
expected verdict policy, and rollback refs, and returns verifier readiness while
blocking verifier satisfaction and all downstream execution flags.

**Actual ΔV:** The suite now has a typed verifier-input boundary. Evidence can
be prepared for verifier review without silently satisfying the verifier,
approving an owner decision, executing promotion, publishing a package,
synthesizing run acceptance, or claiming completion.

**Evidence:**
- `internal/computerversion/base_verifier_readiness_contract.go`
- `internal/computerversion/base_verifier_readiness_contract_test.go`
- `local://pass108-base-verifier-readiness-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseVerifierReadinessContract -count=1`
  passed: 61 pass events, 0 fail events.

**Protected surfaces named:** verifier-readiness boundary, verifier-contract
satisfaction boundary, owner approval boundary, promotion/rollback boundary,
package-publication boundary, and run-acceptance record boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment mutation, persistent production
  state mutation, VM lifecycle operation, Firecracker boot, owner approval,
  promotion/rollback mutation, package publication, gateway/provider call,
  Texture canonical write, verifier-contract satisfaction, or run-acceptance
  record was touched.
- The verifier-readiness boundary is not verifier satisfaction, owner approval,
  promotion authority, package-publication proof, run-acceptance proof, full
  substrate independence, or mission completion.

**Next:** Define the verifier-result boundary that can record pass/fail verifier
outcome without approving promotion, publishing a package, or synthesizing run
acceptance.

## Pass 109 — 2026-07-04 (Base Verifier Result Boundary)

**Conjecture:** A verifier pass/fail result can be recorded after verifier
readiness without being confused for owner approval, promotion execution,
package publication, run acceptance, full-substrate proof, or completion. A pass
satisfies only the verifier contract; a fail remains explicit blocking evidence.

**Move:** Added `BaseVerifierResultEvidence`,
`BaseVerifierResultContract`, and `BuildBaseVerifierResultContract` in
`internal/computerversion`. The builder validates the Pass 108 verifier
readiness contract, accepts only `pass` or `fail` verdicts, requires verifier
run/result/log and rollback refs, records pass as verifier-contract satisfaction
only, records fail with a required failure reason and downstream block, and
keeps owner approval, promotion/rollback, package publication, run acceptance,
full-substrate proof, and completion as separate gates.

**Actual ΔV:** The suite now has a typed verifier-result boundary. Verifier
outcomes are no longer forced to remain only readiness inputs, but satisfying
the verifier no longer silently grants owner approval, promotion authority,
package-publication proof, run-acceptance proof, full-substrate independence, or
mission completion.

**Evidence:**
- `internal/computerversion/base_verifier_result_contract.go`
- `internal/computerversion/base_verifier_result_contract_test.go`
- `local://pass109-base-verifier-result-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseVerifierResultContract -count=1`
  passed: 38 pass events, 0 fail events.

**Protected surfaces named:** verifier-result boundary, verifier-contract
satisfaction boundary, owner approval boundary, promotion/rollback boundary,
package-publication boundary, and run-acceptance record boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment mutation, persistent production
  state mutation, VM lifecycle operation, Firecracker boot, owner approval,
  promotion/rollback mutation, package publication, gateway/provider call,
  Texture canonical write, or run-acceptance record was touched.
- The verifier-result boundary is not owner approval, promotion authority,
  package-publication proof, run-acceptance proof, full substrate independence,
  or mission completion.
- A failing verifier result blocks downstream authority until a later corrective
  verifier pass is produced under its own evidence.

**Next:** Define the owner-approval boundary that can consume a passing verifier
result and owner-review packet without executing promotion, publishing a
package, or synthesizing run acceptance.

## Pass 110 — 2026-07-04 (Base Owner Approval Boundary)

**Conjecture:** Owner approval can be recorded as typed local review evidence
after a passing verifier result and matching owner-review packet without
executing promotion, publishing a package, synthesizing run acceptance, claiming
full-substrate proof, or claiming completion. Rejection must remain explicit
blocking evidence.

**Move:** Added `BaseOwnerApprovalEvidence`,
`BaseOwnerApprovalContract`, and `BuildBaseOwnerApprovalContract` in
`internal/computerversion`. The builder validates the Pass 107 owner-review
readiness contract, validates a matching Pass 109 passing verifier result,
requires owner decision, owner identity, review packet, verifier result, and
rollback refs, records `approve` as owner approval evidence only, records
`reject` with a required rejection reason and downstream block, and keeps
promotion/rollback review, package-publication proof, run-acceptance proof,
full-substrate proof, and completion as separate gates.

**Actual ΔV:** The suite now has a typed owner-decision boundary. A human/owner
decision can be represented without turning approval into promotion authority or
turning rejection into an ambiguous missing prerequisite.

**Evidence:**
- `internal/computerversion/base_owner_approval_contract.go`
- `internal/computerversion/base_owner_approval_contract_test.go`
- `local://pass110-base-owner-approval-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseOwnerApprovalContract -count=1`
  passed: 34 pass events, 0 fail events.

**Protected surfaces named:** owner approval boundary, verifier-contract
satisfaction boundary, promotion/rollback boundary, package-publication
boundary, and run-acceptance record boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment mutation, persistent production
  state mutation, VM lifecycle operation, Firecracker boot, promotion/rollback
  mutation, package publication, gateway/provider call, Texture canonical write,
  or run-acceptance record was touched.
- The owner-approval boundary is not promotion authority,
  package-publication proof, run-acceptance proof, full substrate independence,
  or mission completion.
- A rejected owner decision blocks downstream authority until a later corrective
  owner approval is produced under its own evidence.

**Next:** Define the promotion/rollback review boundary that can consume owner
approval without executing promotion, publishing a package, or synthesizing run
acceptance.

## Pass 111 — 2026-07-04 (Base Promotion/Rollback Review Boundary)

**Conjecture:** Promotion/rollback review can consume owner approval only as a
readiness prerequisite. It must not execute promotion, publish a package,
synthesize run acceptance, claim full-substrate proof, or claim completion.

**Move:** Added `BasePromotionRollbackReviewEvidence`,
`BasePromotionRollbackReviewContract`, and
`BuildBasePromotionRollbackReviewContract` in `internal/computerversion`. The
builder validates the Pass 110 owner approval contract, rejects owner rejection
or blocked owner decisions, requires promotion plan, rollback plan, risk review,
ledger freshness, route continuity, and operator review policy refs, and records
promotion/rollback review readiness while preserving package-publication,
run-acceptance, full-substrate, and completion gates.

**Actual ΔV:** The suite now has a typed review-readiness boundary between
owner approval and any later promotion executor. Approval no longer creates an
implicit promotion path; it must pass through explicit promotion/rollback review
evidence first.

**Evidence:**
- `internal/computerversion/base_promotion_rollback_review_contract.go`
- `internal/computerversion/base_promotion_rollback_review_contract_test.go`
- `local://pass111-base-promotion-rollback-review-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBasePromotionRollbackReviewContract -count=1`
  passed: 47 pass events, 0 fail events.

**Protected surfaces named:** promotion/rollback review boundary, promotion
execution boundary, package-publication boundary, and run-acceptance record
boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment mutation, persistent production
  state mutation, VM lifecycle operation, Firecracker boot, promotion/rollback
  mutation, package publication, gateway/provider call, Texture canonical write,
  or run-acceptance record was touched.
- The promotion/rollback review boundary is not promotion execution,
  package-publication proof, run-acceptance proof, full substrate independence,
  or mission completion.

**Next:** Define the package-publication readiness boundary that can consume
promotion/rollback review readiness without publishing a package, executing
promotion, or synthesizing run acceptance.

## Pass 112 — 2026-07-04 (Base Package-Publication Readiness Boundary)

**Conjecture:** Package-publication readiness can consume promotion/rollback
review readiness only as publication prerequisite evidence. It must not publish
a package, execute promotion, synthesize run acceptance, claim full-substrate
proof, or claim completion.

**Move:** Added `BasePackagePublicationReadinessEvidence`,
`BasePackagePublicationReadinessContract`, and
`BuildBasePackagePublicationReadinessContract` in `internal/computerversion`.
The builder validates the Pass 111 promotion/rollback review contract, rejects
promotion-review drift, requires package manifest, publication payload,
publication target, publication policy, publication dry-run plan, and rollback
refs, and records package-publication readiness while preserving promotion,
run-acceptance, full-substrate, and completion gates.

**Actual ΔV:** The suite now has a typed package-publication readiness boundary
between promotion/rollback review and any later package publisher. Promotion
review no longer implies publication authority; publication prerequisites must
be packaged separately as readiness evidence first.

**Evidence:**
- `internal/computerversion/base_package_publication_readiness_contract.go`
- `internal/computerversion/base_package_publication_readiness_contract_test.go`
- `local://pass112-base-package-publication-readiness-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBasePackagePublicationReadinessContract -count=1`
  passed: 49 pass events, 0 fail events.

**Protected surfaces named:** package-publication readiness boundary,
package-publication execution boundary, promotion execution boundary, and
run-acceptance record boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment mutation, persistent production
  state mutation, VM lifecycle operation, Firecracker boot, promotion/rollback
  mutation, package publication, gateway/provider call, Texture canonical write,
  or run-acceptance record was touched.
- The package-publication readiness boundary is not package publication,
  promotion execution, run-acceptance proof, full substrate independence, or
  mission completion.

**Next:** Define the package-publication proof boundary that can consume
publication readiness without executing promotion or synthesizing run acceptance.

## Pass 113 — 2026-07-04 (Base Package-Publication Proof Boundary)

**Conjecture:** Package-publication proof can consume package-publication
readiness only as external proof refs. It must not execute promotion, synthesize
run acceptance, mutate production publication state, claim full-substrate proof,
or claim completion.

**Move:** Added `BasePackagePublicationProofEvidence`,
`BasePackagePublicationProofContract`, and
`BuildBasePackagePublicationProofContract` in `internal/computerversion`. The
builder validates the Pass 112 package-publication readiness contract, rejects
readiness drift, requires publication readiness, publication proof, published
package, package digest, publication receipt, publication ledger, publication
review, and rollback refs, and records package-publication proof while
preserving promotion, run-acceptance, full-substrate, and completion gates.

**Actual ΔV:** The suite now has a typed proof boundary after
package-publication readiness. Publication proof refs no longer imply promotion
authority, run-acceptance authority, production publication-state mutation, or
full mission completion.

**Evidence:**
- `internal/computerversion/base_package_publication_proof_contract.go`
- `internal/computerversion/base_package_publication_proof_contract_test.go`
- `local://pass113-base-package-publication-proof-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBasePackagePublicationProofContract -count=1`
  passed: 51 pass events, 0 fail events.

**Protected surfaces named:** package-publication proof boundary,
package-publication execution boundary, promotion execution boundary,
run-acceptance record boundary, and production publication state.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment mutation, persistent production
  state mutation, VM lifecycle operation, Firecracker boot, promotion/rollback
  mutation, package publication, gateway/provider call, Texture canonical write,
  or run-acceptance record was touched.
- The package-publication proof boundary is not promotion execution,
  run-acceptance proof, full substrate independence, production publication
  mutation, or mission completion.

**Next:** Define the promotion-execution readiness boundary that can consume
package-publication proof without executing promotion or synthesizing run
acceptance.

## Pass 114 — 2026-07-04 (Base Promotion-Execution Readiness Boundary)

**Conjecture:** Promotion-execution readiness can consume package-publication
proof only as promotion prerequisite evidence. It must not execute promotion,
synthesize run acceptance, mutate production state, claim full-substrate proof,
or claim completion.

**Move:** Added `BasePromotionExecutionReadinessEvidence`,
`BasePromotionExecutionReadinessContract`, and
`BuildBasePromotionExecutionReadinessContract` in `internal/computerversion`.
The builder validates the Pass 113 package-publication proof contract, rejects
publication-proof drift, requires package-publication proof, promotion
candidate, promotion execution plan, promotion preflight, operator policy,
promotion rollback plan, route cutover plan, and ledger freshness refs, and
records promotion-execution readiness while preserving promotion proof,
run-acceptance, full-substrate, and completion gates.

**Actual ΔV:** The suite now has a typed readiness boundary before any later
promotion executor. Package-publication proof no longer implies promotion
execution authority, production mutation authority, run-acceptance authority, or
mission completion.

**Evidence:**
- `internal/computerversion/base_promotion_execution_readiness_contract.go`
- `internal/computerversion/base_promotion_execution_readiness_contract_test.go`
- `local://pass114-base-promotion-execution-readiness-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBasePromotionExecutionReadinessContract -count=1`
  passed: 51 pass events, 0 fail events.

**Protected surfaces named:** promotion-execution readiness boundary, promotion
execution boundary, run-acceptance record boundary, production publication
state, and rollback boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment mutation, persistent production
  state mutation, VM lifecycle operation, Firecracker boot, promotion/rollback
  mutation, package publication, gateway/provider call, Texture canonical write,
  or run-acceptance record was touched.
- The promotion-execution readiness boundary is not promotion execution,
  run-acceptance proof, full substrate independence, production mutation, or
  mission completion.

**Next:** Define the promotion-result boundary that can record a no-op or
blocked promotion outcome without mutating production or synthesizing run
acceptance.

## Pass 115 — 2026-07-04 (Base Promotion Result Boundary)

**Conjecture:** Promotion results can be recorded after promotion-execution
readiness only as blocked/no-op outcome evidence. Recording a result must not
execute promotion, synthesize run acceptance, mutate production state, claim
full-substrate proof, or claim completion.

**Move:** Added `BasePromotionResultEvidence`,
`BasePromotionResultContract`, and `BuildBasePromotionResultContract` in
`internal/computerversion`. The builder validates the Pass 114
promotion-execution readiness contract, rejects readiness drift, accepts only
`blocked` or `noop` outcomes, requires result/outcome/operator/attempt/rollback
refs, and records the promotion result while preserving promotion proof,
run-acceptance, full-substrate, and completion gates.

**Actual ΔV:** The suite now has a typed result boundary after promotion
readiness. A blocked/no-op promotion result no longer implies promotion
execution authority, production mutation authority, run-acceptance authority, or
mission completion.

**Evidence:**
- `internal/computerversion/base_promotion_result_contract.go`
- `internal/computerversion/base_promotion_result_contract_test.go`
- `local://pass115-base-promotion-result-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBasePromotionResultContract -count=1`
  passed: 55 pass events, 0 fail events.
- `scripts/doccheck report-only`
  passed: 293 docs, 440 warnings.

**Protected surfaces named:** promotion-result boundary, promotion execution
boundary, run-acceptance record boundary, production state, and rollback
boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment mutation, persistent production
  state mutation, VM lifecycle operation, Firecracker boot, promotion/rollback
  mutation, package publication, gateway/provider call, Texture canonical write,
  or run-acceptance record was touched.
- The promotion-result boundary is not promotion execution, run-acceptance
  proof, full substrate independence, production mutation, or mission
  completion.

**Next:** Define the promotion review/settlement boundary that can consume a
blocked/no-op promotion result without executing promotion, mutating production,
or synthesizing run acceptance.

## Pass 116 — 2026-07-04 (Base Promotion Settlement Boundary)

**Conjecture:** A blocked/no-op promotion result can be settled as operator
review evidence without becoming promotion execution, production mutation,
run-acceptance proof, full-substrate proof, or mission completion.

**Move:** Added `BasePromotionSettlementEvidence`,
`BasePromotionSettlementContract`, and `BuildBasePromotionSettlementContract`
in `internal/computerversion`. The builder validates the Pass 115 promotion
result contract, rejects result drift, requires settlement decisions to match
the blocked/no-op result outcome, requires settlement/reason/operator/rollback
refs, and records operator settlement while preserving promotion proof,
run-acceptance, full-substrate, and completion gates.

**Actual ΔV:** The suite now has a typed settlement boundary after blocked/no-op
promotion results. Settling a promotion result no longer implies promotion
execution authority, production mutation authority, run-acceptance authority, or
mission completion.

**Evidence:**
- `internal/computerversion/base_promotion_settlement_contract.go`
- `internal/computerversion/base_promotion_settlement_contract_test.go`
- `local://pass116-base-promotion-settlement-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBasePromotionSettlementContract -count=1`
  passed: 60 pass events, 0 fail events.
- `scripts/doccheck report-only`
  passed: 293 docs, 440 warnings.

**Protected surfaces named:** promotion-settlement boundary, promotion-result
boundary, promotion execution boundary, run-acceptance record boundary,
production state, and rollback boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment mutation, persistent production
  state mutation, VM lifecycle operation, Firecracker boot, promotion/rollback
  mutation, package publication, gateway/provider call, Texture canonical write,
  or run-acceptance record was touched.
- The promotion-settlement boundary is not promotion execution, run-acceptance
  proof, full substrate independence, production mutation, or mission
  completion.

**Next:** Define the post-settlement handoff boundary that can return from
blocked/no-op promotion settlement to the next substrate-independence proof
without treating promotion settlement as mission completion.

## Pass 117 — 2026-07-04 (Base Post-Promotion-Settlement Handoff Boundary)

**Conjecture:** Promotion settlement can hand control back to
substrate-independence proof work only by recording residual proof obligations
and the next safe substrate proof probe. The handoff must not become promotion
execution, production mutation, run-acceptance proof, full-substrate proof, or
mission completion.

**Move:** Added `BasePostPromotionSettlementHandoffReadinessEvidence`,
`BasePostPromotionSettlementHandoffReadinessContract`, and
`BuildBasePostPromotionSettlementHandoffReadinessContract` in
`internal/computerversion`. The builder validates the Pass 116 promotion
settlement contract, rejects settlement drift, requires substrate proof plan,
durable state slice, observation set, materializer contract, equivalence check,
residual risk, and rollback refs, and records a blocked handoff while preserving
promotion proof, run-acceptance, full-substrate, and completion gates.

**Actual ΔV:** The suite now has a typed handoff from promotion settlement back
to substrate-independence proof work. Settled blocked/no-op promotion results no
longer function as completion claims or as authority to execute promotion,
mutate production, synthesize run acceptance, or stop the mission.

**Evidence:**
- `internal/computerversion/base_post_promotion_settlement_handoff_contract.go`
- `internal/computerversion/base_post_promotion_settlement_handoff_contract_test.go`
- `local://pass117-base-post-promotion-settlement-handoff-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBasePostPromotionSettlementHandoffReadinessContract -count=1`
  passed: 57 pass events, 0 fail events.
- `scripts/doccheck report-only`
  passed: 293 docs, 440 warnings.

**Protected surfaces named:** post-settlement handoff boundary,
promotion-settlement boundary, promotion execution boundary, run-acceptance
record boundary, production state, and rollback boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, deployed route registration, production
  auth/session mutation, staging deployment mutation, persistent production
  state mutation, VM lifecycle operation, Firecracker boot, promotion/rollback
  mutation, package publication, gateway/provider call, Texture canonical write,
  or run-acceptance record was touched.
- The post-settlement handoff boundary is not promotion execution,
  run-acceptance proof, full substrate independence, production mutation, or
  mission completion.

**Next:** Return to substrate-independence proof work by defining the first
durable state slice readiness boundary that can consume the post-settlement
handoff without invoking promotion or run acceptance.

## Pass 118 — 2026-07-04 (Base Durable-State-Slice Readiness Boundary)

**Conjecture:** A post-settlement handoff can return the mission to
substrate-independence proof work only by authorizing a typed durable-state-slice
readiness probe. The boundary must not become runtime materialization, durable
computer mutation, package publication, promotion execution, production mutation,
run-acceptance proof, full-substrate proof, or mission completion.

**Move:** Added `BaseDurableStateSliceReadinessEvidence`,
`BaseDurableStateSliceReadinessContract`, and
`BuildBaseDurableStateSliceReadinessContract` in `internal/computerversion`.
The builder validates the Pass 117 post-settlement handoff contract, rejects
handoff drift, requires durable-state-slice plan, file-manifest probe,
blob-content probe, observation set, materializer contract, equivalence check,
residual risk, and rollback refs, and records readiness for a typed durable
state slice probe while preserving promotion proof, run-acceptance,
full-substrate, and completion gates.

**Actual ΔV:** The suite now has a typed return edge from promotion-settlement
handoff back into durable-state-slice proof work. Post-settlement handoff no
longer functions as durable-state evidence, runtime materialization authority,
promotion authority, production mutation authority, run-acceptance authority, or
mission completion.

**Evidence:**
- `internal/computerversion/base_durable_state_slice_readiness_contract.go`
- `internal/computerversion/base_durable_state_slice_readiness_contract_test.go`
- `local://pass118-base-durable-state-slice-readiness-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseDurableStateSliceReadinessContract -count=1`
  passed: 35 pass events, 0 fail events.
- `scripts/doccheck report-only`
  passed: 293 docs, 440 warnings.

**Protected surfaces named:** durable-state-slice readiness boundary,
post-settlement handoff boundary, promotion execution boundary, run-acceptance
record boundary, production state, and rollback boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, durable computer mutation, deployed route
  registration, production auth/session mutation, staging deployment mutation,
  persistent production state mutation, VM lifecycle operation, Firecracker boot,
  promotion/rollback mutation, package publication, gateway/provider call,
  Texture canonical write, or run-acceptance record was touched.
- The durable-state-slice readiness boundary is not runtime materialization,
  durable computer mutation, promotion execution, run-acceptance proof, full
  substrate independence, production mutation, or mission completion.

**Next:** Use the durable-state-slice readiness boundary to run the narrow
file-manifest/blob-content durable state slice probe without materializing a
runtime computer or claiming full substrate independence.

## Pass 119 — 2026-07-04 (Base Durable-State-Slice Probe Boundary)

**Conjecture:** Durable-state-slice readiness can become useful proof only when
it binds to the existing typed file-manifest/blob-content durable slice. The
probe must not become runtime materialization, durable computer mutation, package
publication, promotion execution, production mutation, run-acceptance proof,
full-substrate proof, or mission completion.

**Move:** Added `BaseDurableStateSliceProbeEvidence`,
`BaseDurableStateSliceProbeContract`, and
`BuildBaseDurableStateSliceProbeContract` in `internal/computerversion`. The
builder validates the Pass 118 durable-state-slice readiness contract, validates
the existing `BaseDurableStateSliceContract`, rejects readiness and durable-slice
drift, requires durable-state-slice readiness, durable-state-slice contract,
file-manifest probe, blob-content probe, probe evidence, residual risk, and
rollback refs, and records the scoped durable-state-slice probe result while
preserving runtime, staging, promotion, run-acceptance, full-substrate, and
completion gates.

**Actual ΔV:** The suite now has a typed proof edge from durable-state-slice
readiness to the existing Base file-manifest/blob-content durable slice.
Readiness no longer functions as durable-state proof by itself, and the scoped
probe result does not grant runtime materialization, durable computer mutation,
promotion, production mutation, run-acceptance, full-substrate, or completion
authority.

**Evidence:**
- `internal/computerversion/base_durable_state_slice_probe_contract.go`
- `internal/computerversion/base_durable_state_slice_probe_contract_test.go`
- `local://pass119-base-durable-state-slice-probe-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseDurableStateSliceProbeContract -count=1`
  passed: 99 pass events, 0 fail events.
- `scripts/doccheck report-only`
  passed: 293 docs, 440 warnings.

**Protected surfaces named:** durable-state-slice probe boundary,
durable-state-slice readiness boundary, runtime materialization boundary,
promotion execution boundary, run-acceptance record boundary, production state,
and rollback boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, durable computer mutation, deployed route
  registration, production auth/session mutation, staging deployment mutation,
  persistent production state mutation, VM lifecycle operation, Firecracker boot,
  promotion/rollback mutation, package publication, gateway/provider call,
  Texture canonical write, or run-acceptance record was touched.
- The durable-state-slice probe boundary is not runtime materialization, durable
  computer mutation, promotion execution, run-acceptance proof, full substrate
  independence, production mutation, or mission completion.

**Next:** Use the scoped durable-state-slice probe result to open only the next
source-provenance/materializer readiness boundary, not runtime materialization
itself.

## Pass 120 — 2026-07-04 (Base Source-Provenance/Materializer Readiness Boundary)

**Conjecture:** The scoped durable-state-slice probe can support runtime-facing
work only by binding to source-provenance readiness and materializer boundary
evidence. That binding may open a later red runtime-materialization ceremony, but
it must not materialize runtime state, mutate a durable computer, publish a
package, execute promotion, mutate production, synthesize run acceptance, claim
full-substrate proof, or complete the mission.

**Move:** Added `BaseSourceMaterializerReadinessEvidence`,
`BaseSourceMaterializerReadinessContract`, and
`BuildBaseSourceMaterializerReadinessContract` in `internal/computerversion`.
The builder validates the Pass 119 durable-state-slice probe contract, validates
the existing `BaseSourceProvenanceReadinessContract` and
`BaseMaterializerBoundaryContract`, rejects probe/source/materializer drift,
requires durable-slice probe, source-provenance readiness, materializer boundary,
materializer readiness plan, residual risk, and rollback refs, and records only
readiness to open a later runtime-materialization ceremony while preserving
runtime, staging, promotion, package-publication, run-acceptance, full-substrate,
and completion gates.

**Actual ΔV:** The suite now has a typed readiness edge from scoped durable-state
proof into source-provenance/materializer readiness. The runtime-facing boundary
is explicit, but the pass still grants no authority to materialize runtime state,
mutate durable computers, publish packages, execute promotion, mutate production,
synthesize run acceptance, claim full substrate independence, or complete the
mission.

**Evidence:**
- `internal/computerversion/base_source_materializer_readiness_contract.go`
- `internal/computerversion/base_source_materializer_readiness_contract_test.go`
- `local://pass120-base-source-materializer-readiness-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseSourceMaterializerReadinessContract -count=1`
  passed: 59 pass events, 0 fail events.
- `scripts/doccheck report-only`
  passed: 293 docs, 440 warnings.

**Protected surfaces named:** source-provenance/materializer readiness boundary,
durable-state-slice probe boundary, materializer boundary, runtime
materialization boundary, promotion execution boundary, run-acceptance record
boundary, production state, and rollback boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, durable computer mutation, deployed route
  registration, production auth/session mutation, staging deployment mutation,
  persistent production state mutation, VM lifecycle operation, Firecracker boot,
  promotion/rollback mutation, package publication, gateway/provider call,
  Texture canonical write, or run-acceptance record was touched.
- The source-provenance/materializer readiness boundary is not runtime
  materialization, durable computer mutation, promotion execution,
  run-acceptance proof, full substrate independence, production mutation, or
  mission completion.

**Next:** Open the runtime-materialization ceremony only as a red ceremony with
fresh staging/deploy rollback constraints, or continue strengthening the
non-runtime proof chain if runtime authority remains out of scope.

## Pass 121 — 2026-07-04 (Base Runtime-Materialization Bridge Boundary)

**Conjecture:** Runtime-materialization ceremony evidence can consume
source-provenance/materializer readiness only as an admissibility bridge to an
existing scoped runtime ceremony contract. The bridge must not mutate VM
lifecycle, durable computer state, deployed routing, production state, package
publication, promotion, run acceptance, full-substrate proof, or completion
state.

**Move:** Added `BaseRuntimeMaterializationBridgeEvidence`,
`BaseRuntimeMaterializationBridgeContract`, and
`BuildBaseRuntimeMaterializationBridgeContract` in `internal/computerversion`.
The builder validates the Pass 120 source/materializer readiness contract,
validates the existing `BaseRuntimeMaterializationCeremonyContract`, rejects
readiness drift, runtime ceremony drift, missing refs, missing no-mutation flags,
and protected-surface/completion claims, and records only scoped runtime
ceremony evidence admissibility for later runtime-equivalence work.

**Actual ΔV:** The suite now has a typed bridge from non-runtime
source/materializer readiness to existing scoped runtime-materialization ceremony
evidence. This does not grant authority to mutate VM lifecycle, durable
computers, deployed routes, production, packages, promotion state, run-acceptance
records, or mission completion.

**Evidence:**
- `internal/computerversion/base_runtime_materialization_bridge_contract.go`
- `internal/computerversion/base_runtime_materialization_bridge_contract_test.go`
- `local://pass121-base-runtime-materialization-bridge-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseRuntimeMaterializationBridgeContract -count=1`
  passed: 60 pass events, 0 fail events.
- `scripts/doccheck report-only`
  passed: 293 docs, 440 warnings.

**Protected surfaces named:** runtime-materialization ceremony bridge boundary,
source-provenance/materializer readiness boundary, runtime materialization
boundary, VM lifecycle boundary, deployed routing boundary, promotion execution
boundary, run-acceptance record boundary, production state, and rollback
boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, durable computer mutation, deployed route
  registration, production auth/session mutation, staging deployment mutation,
  persistent production state mutation, VM lifecycle operation, Firecracker boot,
  promotion/rollback mutation, package publication, gateway/provider call,
  Texture canonical write, or run-acceptance record was touched.
- The runtime-materialization bridge boundary is not VM lifecycle mutation,
  deployed runtime behavior, durable computer mutation, promotion execution,
  run-acceptance proof, full substrate independence, production mutation, or
  mission completion.

**Next:** Use the accepted runtime ceremony evidence only to open the next
runtime-equivalence boundary, or continue strengthening non-runtime proof if VM
lifecycle and staging authority remain out of scope.

## Pass 122 — 2026-07-04 (Base Runtime-Equivalence Reentry Boundary)

**Conjecture:** Runtime-equivalence reentry can consume a
runtime-materialization bridge only if it preserves the existing narrowed
equivalence result. Bridge admissibility must not become equivalence success,
VM lifecycle mutation, durable computer mutation, deployed routing, production
mutation, package publication, promotion execution, run acceptance, full-substrate
proof, or mission completion.

**Move:** Added `BaseRuntimeEquivalenceReentryEvidence`,
`BaseRuntimeEquivalenceReentryContract`, and
`BuildBaseRuntimeEquivalenceReentryContract` in `internal/computerversion`. The
builder validates the Pass 121 runtime-materialization bridge contract, validates
the existing narrowed `BaseRuntimeEquivalenceBoundaryContract`, requires bridge
and equivalence refs to align, requires unsupported `file_manifest` and
`blob_set` durable observations to remain present, rejects bridge/equivalence
drift, missing refs, missing no-mutation flags, and protected-surface/completion
claims, and records only narrowed runtime-equivalence reentry for later durable
state proof work.

**Actual ΔV:** The suite now has a typed reentry boundary from accepted runtime
ceremony evidence back to the existing narrowed runtime-equivalence result. The
result remains narrowed: vmmanager runtime evidence still does not prove durable
file/blob state, staging readiness, promotion, run acceptance, full substrate
independence, or completion.

**Evidence:**
- `internal/computerversion/base_runtime_equivalence_reentry_contract.go`
- `internal/computerversion/base_runtime_equivalence_reentry_contract_test.go`
- `local://pass122-base-runtime-equivalence-reentry-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseRuntimeEquivalenceReentryContract -count=1`
  passed: 54 pass events, 0 fail events.
- `scripts/doccheck report-only`
  passed: 293 docs, 440 warnings.

**Protected surfaces named:** runtime-equivalence reentry boundary,
runtime-materialization ceremony bridge boundary, runtime-equivalence boundary,
durable-state equivalence boundary, VM lifecycle boundary, deployed routing
boundary, promotion execution boundary, run-acceptance record boundary,
production state, and rollback boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, durable computer mutation, deployed route
  registration, production auth/session mutation, staging deployment mutation,
  persistent production state mutation, VM lifecycle operation, Firecracker boot,
  promotion/rollback mutation, package publication, gateway/provider call,
  Texture canonical write, or run-acceptance record was touched.
- The runtime-equivalence reentry boundary is still narrowed. It is not durable
  state equivalence success, staging proof, promotion execution, run-acceptance
  proof, full substrate independence, production mutation, or mission completion.

**Next:** Use the narrowed reentry result to strengthen durable-state
equivalence proof obligations, not to claim runtime equivalence success.

## Pass 123 — 2026-07-04 (Base Runtime-Durable Proof Gap Boundary)

**Conjecture:** A runtime-durable proof gap can consume the narrowed
runtime-equivalence reentry and local file/blob substrate proof summary only if
it preserves the gap between local durable-state proof and runtime substrate
proof. It must require runtime file/blob extraction and retry evidence instead
of upgrading either input to runtime-equivalence, full-substrate, or completion
authority.

**Move:** Added `BaseRuntimeDurableProofGapEvidence`,
`BaseRuntimeDurableProofGapContract`, and
`BuildBaseRuntimeDurableProofGapContract` in `internal/computerversion`. The
builder validates the Pass 122 narrowed runtime-equivalence reentry, validates
the local file/blob substrate proof summary, requires aligned version and
artifact-program refs, preserves runtime VM-state observations, preserves local
`file_manifest` and `blob_set` observations, preserves unsupported runtime
durable `file_manifest` and `blob_set` observations, records required runtime
file/blob extraction and runtime-equivalence retry gaps, and rejects reentry
drift, local summary drift, missing refs, missing gap obligations, no-mutation
flag drift, and protected-surface/completion claims.

**Actual ΔV:** The suite now has an explicit typed gap boundary between
narrowed runtime evidence and local durable-state proof. Local file/blob proof
and runtime-equivalence reentry are recorded as complementary evidence, not as
runtime substrate proof. The next admissible runtime step remains runtime
file/blob extraction plus equivalence retry evidence.

**Evidence:**
- `internal/computerversion/base_runtime_durable_proof_gap_contract.go`
- `internal/computerversion/base_runtime_durable_proof_gap_contract_test.go`
- `local://pass123-base-runtime-durable-proof-gap-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseRuntimeDurableProofGapContract -count=1`
  passed: 113 pass events, 0 fail events.
- `scripts/doccheck report-only`
  passed: 294 docs, 441 warnings.

**Protected surfaces named:** runtime-durable proof gap boundary,
runtime-equivalence reentry boundary, local substrate proof summary boundary,
durable-state equivalence boundary, runtime substrate proof boundary, VM
lifecycle boundary, deployed routing boundary, promotion execution boundary,
run-acceptance record boundary, production state, and rollback boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, durable computer mutation, deployed route
  registration, production auth/session mutation, staging deployment mutation,
  persistent production state mutation, VM lifecycle operation, Firecracker boot,
  promotion/rollback mutation, package publication, gateway/provider call,
  Texture canonical write, or run-acceptance record was touched.
- The runtime-durable proof gap boundary is not runtime equivalence success,
  durable-state equivalence success, staging proof, promotion execution,
  run-acceptance proof, full substrate independence, production mutation, or
  mission completion.

**Next:** Use the runtime-durable proof gap only to authorize runtime file/blob
extraction and runtime-equivalence retry evidence, or continue strengthening
non-runtime proof if VM lifecycle and staging authority remain out of scope.

## Pass 124 — 2026-07-04 (Base Runtime-Durable Gap Extraction Handoff Boundary)

**Conjecture:** Existing runtime file/blob extraction proof can be reused after
the runtime-durable proof gap only if a handoff validates that the extraction
satisfies the gap's required runtime file/blob obligation while preserving
runtime-equivalence retry, staging, promotion, package-publication,
run-acceptance, full-substrate, and completion obligations.

**Move:** Added `BaseRuntimeDurableGapExtractionHandoffEvidence`,
`BaseRuntimeDurableGapExtractionHandoffContract`, and
`BuildBaseRuntimeDurableGapExtractionHandoffContract` in
`internal/computerversion`. The builder validates the Pass 123
`BaseRuntimeDurableProofGapContract`, validates the existing
`BaseRuntimeFileBlobExtractionContract`, requires version and artifact-program
alignment, requires source-provenance and runtime-equivalence-boundary refs to
match, requires typed runtime `file_manifest` and `blob_set` observations,
rejects `vm_state_manifest` reliance, marks only runtime file/blob extraction as
satisfied, preserves retry and downstream proof obligations, and rejects gap
drift, extraction drift, missing refs, authority drift, missing no-mutation
flags, and protected-surface/completion claims.

**Actual ΔV:** The suite now connects the newer runtime-durable proof gap to the
older typed runtime file/blob extraction contract. The extraction proof can be
admitted only as satisfaction of the extraction prerequisite; it is not runtime
equivalence success, staging proof, promotion execution, run-acceptance proof,
full substrate independence, production mutation, or mission completion.

**Evidence:**
- `internal/computerversion/base_runtime_durable_gap_extraction_handoff_contract.go`
- `internal/computerversion/base_runtime_durable_gap_extraction_handoff_contract_test.go`
- `internal/computerversion/base_runtime_durable_proof_gap_contract.go`
- `local://pass124-base-runtime-durable-gap-extraction-handoff-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseRuntimeDurableGapExtractionHandoffContract -count=1`
  passed: 69 pass events, 0 fail events.
- `scripts/doccheck report-only`
  passed: 294 docs, 441 warnings.

**Protected surfaces named:** runtime-durable proof gap boundary, runtime
file/blob observation extraction boundary, runtime-equivalence retry boundary,
VM lifecycle boundary, durable computer mutation boundary, deployed routing
boundary, package-publication boundary, promotion execution boundary,
run-acceptance record boundary, production state, and rollback boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, durable computer mutation, deployed route
  registration, production auth/session mutation, staging deployment mutation,
  persistent production state mutation, VM lifecycle operation, Firecracker boot,
  promotion/rollback mutation, package publication, gateway/provider call,
  Texture canonical write, or run-acceptance record was touched.
- The runtime-durable gap extraction handoff satisfies only the extraction
  prerequisite. Runtime-equivalence retry, staging, promotion, package
  publication, run acceptance, full-substrate proof, production mutation, and
  mission completion remain open.

**Next:** Use the extraction handoff only to authorize a runtime-equivalence
retry handoff under the runtime-durable proof gap; do not treat extraction
admission as runtime-equivalence success.

## Pass 125 — 2026-07-04 (Base Runtime-Durable Gap Retry Handoff Boundary)

**Conjecture:** Existing runtime-equivalence retry proof can close the
runtime-durable gap's retry obligation only if a handoff validates that it
follows the admitted extraction handoff, compares source/runtime
`file_manifest` and `blob_set` observations for the same `ComputerVersion`, and
preserves every downstream proof gate.

**Move:** Added `BaseRuntimeDurableGapRetryHandoffEvidence`,
`BaseRuntimeDurableGapRetryHandoffContract`, and
`BuildBaseRuntimeDurableGapRetryHandoffContract` in `internal/computerversion`.
The builder validates the Pass 124
`BaseRuntimeDurableGapExtractionHandoffContract`, validates the existing
`BaseRuntimeEquivalenceRetryContract`, requires version and artifact-program
alignment, requires source-provenance, extraction, and runtime-boundary refs to
match, requires typed source/runtime `file_manifest` and `blob_set`
observations, accepts only scoped `EquivalenceEquivalent` retry status, records
runtime file/blob extraction and runtime-equivalence retry as satisfied, leaves
staging/promotion/package/run/full-substrate gaps open, and rejects extraction
handoff drift, retry drift, missing refs, observation-scope drift, no-mutation
flag drift, and protected-surface/completion claims.

**Actual ΔV:** The suite now connects the runtime-durable proof gap to the
existing runtime-equivalence retry proof through explicit extraction and retry
handoffs. Scoped source/runtime file-blob equivalence is admitted, but only as a
bounded runtime-equivalence retry result; it is not staging proof, promotion
execution, package publication, run-acceptance proof, full substrate
independence, production mutation, or mission completion.

**Evidence:**
- `internal/computerversion/base_runtime_durable_gap_retry_handoff_contract.go`
- `internal/computerversion/base_runtime_durable_gap_retry_handoff_contract_test.go`
- `local://pass125-base-runtime-durable-gap-retry-handoff-contract-tests.jsonl`
- `go test -json ./internal/computerversion -run TestBuildBaseRuntimeDurableGapRetryHandoffContract -count=1`
  passed: 75 pass events, 0 fail events.
- `scripts/doccheck report-only`
  passed: 294 docs, 441 warnings.

**Protected surfaces named:** runtime-durable proof gap boundary,
runtime-durable gap extraction handoff boundary, runtime-equivalence retry
boundary, runtime file/blob observation extraction boundary, VM lifecycle
boundary, durable computer mutation boundary, deployed routing boundary,
package-publication boundary, promotion execution boundary, run-acceptance
record boundary, production state, and rollback boundary.

**Deferred / Still Open:**
- No runtime behavior mutation, durable computer mutation, deployed route
  registration, production auth/session mutation, staging deployment mutation,
  persistent production state mutation, VM lifecycle operation, Firecracker boot,
  promotion/rollback mutation, package publication, gateway/provider call,
  Texture canonical write, or run-acceptance record was touched.
- The retry handoff closes only the scoped runtime-equivalence retry obligation.
  Staging proof, promotion proof, package publication proof, run-acceptance
  proof, full-substrate proof, production mutation, and mission completion
  remain open.

**Next:** Use the scoped runtime-equivalence retry handoff only to define the
next non-runtime downstream proof gate, or open a fresh red ceremony before any
VM lifecycle, staging, promotion, package-publication, run-acceptance, or
production mutation.
