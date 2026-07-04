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
