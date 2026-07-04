# Review: Substrate-Independent Audited Computer Changeset

**Date:** 2026-07-04 (updated for Pass 125)
**Reviewers:** Devin (5 parallel subagents + 2 follow-up subagents), Claude Code 2.1.201, Codex CLI 0.139.0 (gpt-5.5)
**Scope:** 36 changed entries — 17 modified tracked files + 19 untracked paths/directories
**Scale:** ~40,000 LOC (34,601 in `internal/computerversion` at review time, 13,489 in `cmd/`, ~5,000 in tracked diffs, 5,490 in ledger)
**Mode:** Read-only review. No modifications made.
**Note:** Initial review covered Passes 0-103. Follow-up review covered Passes 104-125. All three landing-blocker bugs (1.1, 1.2, 1.7) remained unfixed as of Pass 125 and were fixed locally in the landing pass described in §6.1. TLA+ invariants (1.5) remain vacuous. The changeset grew from 114 to 116 Go files and from 37 to 39 contract files. The ledger grew from 5,429 to 5,490 lines. Passes 104-125 added 22 boundary contract files in the same template-duplicated pattern (staging -> smoke -> handoff -> owner review -> verifier -> approval -> promotion -> publication -> settlement -> substrate return -> runtime reentry). The pattern was identified as "boundary inflation" — growing, not converging.

---

## 1. Issues by Severity

### HIGH

#### 1.1 Cross-owner intake takeover via upsert

**Found by:** Claude Code
**Location:** `internal/store/store.go:233`, `internal/store/candidate_package_intake.go:83-110`

The `candidate_package_intakes` table primary key is `intake_id` alone. `UpsertCandidatePackageIntake` performs `ON DUPLICATE KEY UPDATE owner_id = VALUES(owner_id)` with a caller-suppliable `intake_id`. Owner B can POST with owner A's `intake_id` and silently overwrite the entire row, including ownership transfer.

Reads are owner-scoped so it's a hijack/denial rather than a cross-owner leak, and the write routes aren't deployed (see 1.4), but the store primitive is unsafe.

**Fix:** Uniqueness on `(owner_id, intake_id)` or an ownership check before upsert.

#### 1.2 TOCTOU on intake state transitions

**Found by:** Claude Code, Codex (intermediate observation)
**Location:** `internal/runtime/candidate_package_intake.go` (all write methods)

Several write methods load, validate, mutate, and upsert records without compare-and-set guards. Concurrent review/adoption requests can race terminal transitions (double-approve, conflicting switch/rollback).

The lineage-drift checks only cover the switch/rollback paths, not the review/decision paths. There is no `updated_at` or version column used for optimistic concurrency.

**Fix:** Add an optimistic-concurrency guard (`updated_at` or version column) to intake state transitions. Use atomic `WHERE old_status = ? UPDATE SET new_status = ?` patterns.

#### 1.3 Changeset violates AGENTS.md operating contract

**Found by:** Claude Code
**Location:** Entire changeset

Three AGENTS.md clauses are violated by the uncommitted state:

- **Landing loop (AGENTS.md:175):** ~40k LOC including orange/red surfaces (`internal/server`, `internal/store`, `internal/types`, TLA+ spec) sits uncommitted. Every ledger pass cites only local `go test` + `local://passNN-*.json` — no pushed SHA, no CI run, no staging proof.
- **Problem-documentation-first (AGENTS.md:150-153):** With zero commits, the mandated doc-commit-before-fix-commit ordering can't exist in git. Ledger and fixes will land as one blob unless deliberately sequenced.
- **Worktree hygiene (AGENTS.md:185-195):** At least three distinguishable missions (TLA refinement, Base persistence/API, candidate-package pipeline + Svelte UI) are mixed on main with no branch/stash recovery handle.

**Mitigating:** Red ceremony is genuinely observed *in the ledger* (Pass 39 route-registration red assessment, Pass 99 red materialization ceremony), and the code is conservative about mutation. But the ledger is *substituting* for commits rather than preceding them. "Mutation class" appears 0 times in 122 passes — the per-pass schema doesn't record the mutation class / rollback / heresy-delta fields that red ceremony requires.

**Fix:** Split and land the changeset per AGENTS.md ordering: ledger/doc commits first, then fixes, pushed through the landing loop with CI + staging evidence. Suggested sequence: docs → store/types → computerversion → cmds → runtime → frontend.

#### 1.4 3,648-line ledger uncheckpointed

**Found by:** Devin subagent (ledger review)
**Location:** `docs/mission-suite-autoputer-autopaper-spec-first-v0.ledger.md`

103 passes over 2 days with zero intermediate commits. Natural checkpoint points exist (Pass 7, 23, 25, 57, 83, 97, 103). High risk of data loss.

**Fix:** Commit the ledger in logical phase chunks (Phase 1: Passes 0-7, Phase 2: Passes 8-23, etc.).

### MEDIUM

#### 1.5 TLA+ invariants are vacuous

**Found by:** Claude Code
**Location:** `specs/promotion_protocol.tla:66-76, 382-390`

`ComputerVersionOfBase(n) == [codeRef |-> n, artifactProgramRef |-> n]` maps every bounded counter into `ComputerVersions` by construction. `RouteNamesComputerVersion` and `PromotionNamesComputerVersion` can never fail — `TypeOK` already bounds the inputs. They add CI cost and a false sense of "refinement proven" while proving nothing.

Worse, the mapping sets `codeRef = artifactProgramRef =` the same counter, so the model structurally cannot express "code changed, durable state didn't" — the exact distinction the mission's `ComputerVersion = (CodeRef, ArtifactProgramRef)` pair exists to make.

Also cosmetic: the two new lines in `promotion_protocol.cfg` (lines 20-21) aren't indented like their siblings (lines 11-19).

**Fix:** Either strengthen to a real refinement (separate spec with `INSTANCE`-based mapping, independent CodeRef/ArtifactProgramRef counters) or drop the vacuous invariants. Don't ship them as "proof."

#### 1.6 Purity claim is false in computerversion package

**Found by:** Devin subagent, Claude Code
**Location:** `internal/computerversion/types.go` (package doc), `base_current_state_loader.go:36-44`, `base_blob.go:44-50`, `base_journal.go:33`, `base_tree.go:7,88`

The package doc claims "no filesystem, network, database, hypervisor, clock, or random operations" but the extraction layer performs real I/O (SQLite opens, blob store reads, journal verification) and `base_tree.go` imports `time` for serialization formatting.

The contract types are pure; the observation extraction layer is not.

**Fix:** Scope the claim to the contract files, or move the loader/blob files to a sub-package.

#### 1.7 Deployment footgun: write-route registration not guarded

**Found by:** Devin subagent, Claude Code, Codex
**Location:** `internal/runtime/api_candidate_package_intake.go:22`

`RegisterCandidatePackageIntakeRoutes` (full write routes) must never be called in deployed runtime. Currently enforced only by comments. Codex verified that `internal/apihandler.RegisterRoutes` delegates to `runtime.RegisterRoutes`, so the deployed sandbox binary inherits only the read-only registrar — this is the right direction. But the guard is convention, not mechanism.

**Fix:** Add a build tag (`//go:build !production`) or runtime guard (env var check, panic on deployed profile) to prevent accidental registration of write routes in deployed builds.

#### 1.8 ActiveSourceRef mutation is a live pointer

**Found by:** Claude Code
**Location:** `internal/runtime/candidate_package_intake.go:648-657`

`SwitchCandidatePackageIntakeAdoptionReview` mutates `ComputerSourceLineage.ActiveSourceRef` — a live pointer, not evidence. If the harness registrar is ever mounted for real, this could drive a deployed route.

**Open question:** Does anything consume `ComputerSourceLineage.ActiveSourceRef` to serve a deployed route? If yes, the "source_lineage_only, non-deployed" switch claim is wrong and that path is red.

#### 1.9 Contract explosion is not maintainable

**Found by:** Devin subagent, Claude Code
**Location:** 37 `*_contract.go` files at ~10.2k LOC in `internal/computerversion/`

60-70% field-for-field template duplication: same Kind/Boundary/Scope constants, same `NoPromotionMutation`-style boolean blocks (copy-edited across 19+ files, `CompletionClaimed` in 57), same builder + validate trio per file.

Two deeper problems:
1. The booleans are set unconditionally by the builders (`NoPromotionMutation: true`) — they're self-attestation any caller could hand-construct, not proof.
2. Names encode pipeline position (`BasePostSmokeHandoffReadinessContract`), so reordering the lifecycle renames types package-wide.

`candidate_package_activation_contract.go` crams 10 contracts into 2,131 lines.

**Fix:** Extract a shared `ContractHeader` + `NegativeClaims` struct and a generic upstream-validation helper, or represent the lifecycle as data (a step graph).

#### 1.10 Review surface wording ambiguity

**Found by:** Codex (intermediate observation)
**Location:** `internal/runtime/candidate_package_intake.go`, `CandidateReviewApp.svelte`

The review surface says it creates no `AppChangePackage`/`AppAdoption` writes, but acceptance generation requires a prior draft package and adoption review created through the write API. The product wording needs to distinguish "read-only deployed surface" from "read-only after prior mutation pipeline."

### LOW

#### 1.11 cmd binary duplication

**Found by:** Devin subagent, Claude Code
**Location:** `cmd/evidenceroot`, `cmd/baseharness`, `cmd/basecompare`, `cmd/vmstatecompare`, `cmd/vmrealize`, `cmd/vmstateobserve`

~120 near-verbatim LOC of fixture-server code shared between `evidenceroot` and `baseharness`. Byte-identical observation-set loaders in `basecompare`/`vmstatecompare`. Identical 14-flag config scaffolding in `vmrealize`/`vmstateobserve`.

**Fix:** One shared internal fixture/loader package would fix a bug once instead of 2-4 times.

#### 1.12 JSON column validation too loose

**Found by:** Claude Code
**Location:** `internal/store/candidate_package_intake.go`

Store blob-JSON columns validate only `json.Valid` — `adoption_blockers_json: "not-an-array"` persists and fails later at review time instead of at the write boundary.

#### 1.13 No idempotency key on intake creation

**Found by:** Claude Code
**Location:** `internal/runtime/candidate_package_intake.go`

Retried POSTs with server-generated IDs create duplicate rows, while draft/adoption creation has ad-hoc existence-check idempotency — inconsistent.

#### 1.14 SQLite DSN fragility

**Found by:** Claude Code
**Location:** `internal/base/journal/sqlite.go:88`

`url.URL{Scheme:"file", Path:path}` with a relative path emits an opaque `file:foo.db?mode=ro` form — works with modernc sqlite but is fragile. Consider requiring/normalizing to absolute path.

#### 1.15 Large files should be split

**Found by:** Devin subagent, Claude Code
**Location:** `internal/computerversion/candidate_package_activation_contract.go` (2,131 lines), `internal/runtime/candidate_package_intake.go` (1,688 lines), `internal/runtime/candidate_package_intake_test.go` (3,314 lines)

**Fix:** Split `candidate_package_activation_contract.go` into 5 files by contract type. Split `candidate_package_intake.go` by concern (creation, review, adoption, promotion).

---

## 2. Learnings

### 2.1 The ledger-as-commit-substitute pattern

The mission ledger is practicing genuine problem-first discipline (Pass 19 failure-identified → Pass 20 fix-prepared), but it's substituting for git commits rather than preceding them. This creates a review bottleneck: 3,648 lines in one uncommitted change is too large for meaningful review. The ledger's per-pass schema also doesn't record AGENTS.md-required fields (mutation class, rollback path, heresy delta).

**Learning:** A ledger is not a commit history. It records reasoning; git records state. Both are needed, and they should be complementary, not substitutive.

### 2.2 Self-attested safety flags are not proof

Every contract type has 10+ boolean flags for unsafe operations (`NoPromotionMutation`, `NoRuntimeMaterialization`, etc.), all set unconditionally by builders. These are self-attestation — any caller could hand-construct a contract with `NoMutation: false`. The type system doesn't prevent this.

**Learning:** Safety flags should be set by unexported constructors that enforce invariants, not by struct literals. Consider making the flags unexported with accessor methods.

### 2.3 Vacuous invariants give false confidence

The two new TLA+ invariants can never fail by construction. They add CI cost and create the impression that the ComputerVersion refinement has been formally verified, when it hasn't. The model can't even express the key distinction (code changed, state didn't) because both refs map to the same counter.

**Learning:** An invariant that can't fail is documentation, not verification. Label it as such, or make it real.

### 2.4 Read-only surface depends on prior mutation pipeline

The deployed review surface is genuinely read-only, but the acceptance evidence it displays requires prior writes through the harness-only API. This is a staged pipeline, not a pure read-only surface.

**Learning:** "Read-only" needs qualification: read-only *deployed* vs read-only *after prior mutation pipeline*. Product wording should make this distinction.

### 2.5 Multi-agent review convergence

All three reviewers (Devin subagents, Claude Code, Codex) independently identified:
- State-machine TOCTOU risk on intake transitions
- Purity claim violation in computerversion
- Deployment guard concern on write routes
- Large file / contract duplication maintainability issue

Claude Code uniquely found the cross-owner upsert takeover bug (1.1) and the vacuous TLA+ invariants (1.5). Codex uniquely found the review-surface wording ambiguity (1.10) and verified the deployed routing path. Devin subagents uniquely quantified the test coverage (1:1 ratio, 60% test LOC) and the ledger checkpoint concern (1.4).

**Learning:** Multi-agent review with different tooling surfaces different issues. Convergence on an issue increases confidence; unique findings from a single reviewer are still valuable and should be verified.

---

## 3. Suggested Improvements

### 3.1 Immediate (before any of this lands)

1. **Upsert ownership bug** (1.1) — fixed locally with an owner check before upsert.
2. **Optimistic-concurrency guard** (1.2) — fixed locally with updated-at compare-and-set helpers for intake/adoption transitions.
3. **Deployment guard** (1.7) — fixed locally with a deployed-env guard on write-route registration.
4. **Split and land the changeset** (1.3) — sequence: docs → store/types → computerversion → cmds → runtime → frontend, pushed through the landing loop with CI + staging.
5. **Checkpoint the ledger** (1.4) — commit in phase chunks at natural boundaries.

### 3.2 Short-term

6. **Strengthen or drop TLA+ invariants** (1.5) — don't ship vacuous invariants as "proof"
7. **Fix purity claim** (1.6) — scope to contract files or extract sub-package
8. **Trace ActiveSourceRef consumers** (1.8) — confirm no deployed route consumes it
9. **Extract shared contract header** (1.9) — reduce O(n) fan-out for adding new claims
10. **Clarify review-surface wording** (1.10) — "read-only deployed surface after prior harness mutation pipeline"

### 3.3 Medium-term

11. **Extract shared cmd helpers** (1.11) — fixture server, observation-set loader, config scaffolding
12. **Strengthen JSON column validation** (1.12) — validate array/object shape at write boundary
13. **Add idempotency key** (1.13) — consistent idempotency across intake creation
14. **Normalize SQLite DSN** (1.14) — require absolute paths
15. **Split large files** (1.15) — by contract type / concern

### 3.4 Test gaps

16. **Add concurrent transition tests** — no tests currently exercise concurrent review/adoption/switch paths
17. **Add deployment-route negative tests** — verify write routes cannot be reached in deployed configuration (Codex observation)
18. **Add integration test for read-only surface** — verify the deployed read-only surface cannot call write endpoints

---

## 4. Emergent Questions

1. **Does anything consume `ComputerSourceLineage.ActiveSourceRef` to serve a deployed route?** If yes, the "source_lineage_only, non-deployed" switch claim is wrong and that path is red.

2. **Is the contract-per-micro-step pattern intended as scaffolding to be collapsed later, or the permanent shape?** If permanent, decide the generic-header refactor now — the O(n) fan-out (add one claim, touch 19+ files) is already visible.

3. **When does the TLA+ "refinement seam" become an actual refinement?** A separate spec with `INSTANCE`-based mapping and independent CodeRef/ArtifactProgramRef counters would be a real refinement. As written it's documentation wearing an invariant's clothes.

4. **What is the landing plan for 97 uncommitted ledger passes?** One mega-commit will violate problem-doc-first ordering. The ledger suggests a natural commit sequence — is that the plan?

5. **Should the computerversion package be split into `computerversion/contracts` (pure) and `computerversion/observing` (I/O)?** This would make the purity claim true without changing any code.

6. **Are the 8 new cmd binaries intended to ship as standalone tools, or are they harness-only?** If harness-only, should they live under `cmd/` or `internal/harness/`?

7. **What happens when two concurrent requests approve the same intake?** The current code will upsert both, last-write-wins. Is this acceptable for the harness-only phase?

8. **How does the candidate_package_intake flow relate to the existing AppChangePackage/AppAdoption promotion path?** The bridge contract exists but is blocked. What's the unblocking ceremony?

---

## 5. What's Good

- **Docs are coherent and honest:** The suite-definitions edit correctly reframes completion as an "autoputer-first interval" and marks autopaper deferred-not-deleted. The mission doc's authority ordering and non-purpose list are sharp.
- **Ledger practices problem-first discipline:** Pass 19 failure-identified → Pass 20 fix-prepared is the right pattern.
- **Intake status machine is well-built:** Rejected→approved is blocked, adoption_ready requires approved + zero blockers, no path skips owner review.
- **Deployed surface is genuinely read-only:** `RegisterRoutes` mounts only the review-surface GET; the write registrar has no non-test caller (verified by Codex).
- **Test coverage is extensive:** 1:1 test-to-impl ratio in computerversion (~13,294 LOC tests). Tests verify no promotion side effects. Negative-case coverage exists.
- **No SQL injection:** All database operations use parameterized queries through the store layer.
- **Auth required on all handlers:** All API handlers call `authenticateUser()` and require `X-Authenticated-User` header.
- **Owner scoping enforced:** All queries require `owner_id`, preventing cross-owner data leaks (modulo the upsert bug in 1.1).
- **Svelte CandidateReviewApp is genuinely read-only:** The "activation decision" is client-side display only. No mutation buttons.
- **cmd binaries are clean:** No hardcoded paths, proper exit codes, no security concerns, consistent structure.
- **No TODO/FIXME/dead code in computerversion:** All functions have implementations.

---

## 6. Reviewer Attribution

| Finding | Devin subagents | Claude Code | Codex |
|---------|:---:|:---:|:---:|
| 1.1 Cross-owner upsert takeover | | ✓ | |
| 1.2 TOCTOU on state transitions | | ✓ | ✓ |
| 1.3 AGENTS.md violations | | ✓ | |
| 1.4 Ledger uncheckpointed | ✓ | | |
| 1.5 Vacuous TLA+ invariants | | ✓ | |
| 1.6 False purity claim | ✓ | ✓ | |
| 1.7 Deployment guard needed | ✓ | ✓ | ✓ |
| 1.8 ActiveSourceRef is live pointer | | ✓ | |
| 1.9 Contract explosion | ✓ | ✓ | |
| 1.10 Review-surface wording | | | ✓ |
| 1.11 cmd binary duplication | ✓ | ✓ | |
| 1.12 JSON validation too loose | | ✓ | |
| 1.13 No idempotency key | | ✓ | |
| 1.14 SQLite DSN fragility | | ✓ | |
| 1.15 Large files | ✓ | ✓ | |
| 1.16 Boundary inflation (104-125) | ✓ | | |

---

## 6.1 Follow-Up: Passes 104-125

After the initial review (Passes 0-103), the agent continued to Pass 125. A
follow-up subagent review found:

**22 new boundary contract files** added in passes 104-125, building a typed
chain from staging through promotion settlement to runtime reentry:
- Staging path (104-106): staging readiness, smoke evidence, post-smoke handoff
- Owner/verifier path (107-110): owner-review readiness, verifier readiness,
  verifier result, owner approval
- Promotion path (111-116): promotion/rollback review, package-publication
  readiness, package-publication proof, promotion-execution readiness,
  promotion result, promotion settlement
- Substrate return (117-119): post-settlement handoff, durable-state-slice
  readiness, durable-state-slice probe
- Runtime reentry (120-125): source/materializer readiness,
  runtime-materialization bridge, runtime-equivalence reentry,
  runtime-durable proof gap, extraction handoff, retry handoff

**New issue 1.16: Boundary inflation.** The changeset is growing, not
converging. Each pass adds a new typed boundary between proof states without
executing any actual operations. The agent has not executed any runtime
behavior, touched production state, performed actual promotion/deployment, or
closed any major proof gate. This is the AGENTS.md dead-end escalation
pattern: 125 passes, 2+ days, zero commits, no convergence on a landing loop.

**Pass 125 bug status:** All three landing-blocker bugs (1.1 upsert ownership,
1.2 TOCTOU, 1.7 deployment guard) were still unfixed at the Pass 125 stop.
TLA+ invariants (1.5) remain vacuous.

**Landing pass update:** Issues 1.1, 1.2, and 1.7 were fixed locally after
the Pass 125 stop. Fixes added an owner check before candidate-package intake
upsert, updated-at compare-and-set helpers for intake/adoption transitions, and
a deployed-env guard around the opt-in write-route registrar. Focused
regressions:
`TestCandidatePackageIntakeRejectsCrossOwnerUpsertTakeover`,
`TestUpdateCandidatePackageIntakeIfCurrentRejectsStaleUpdatedAt`,
`TestUpdateAppAdoptionIfCurrentRejectsStaleUpdatedAt`,
`TestRegisterCandidatePackageIntakeRoutesPanicsWhenDeployedEnvSet`, and
`TestRegisterCandidatePackageReviewSurfaceRoutesRegistersWhenDeployedEnvSet`.

**Notable observation:** The 22-step boundary chain maps closely to a PlusCal
algorithm structure (staging -> verification -> approval -> promotion ->
settlement). This may be unintentional preparation for MPCal translation, or
just the natural shape of the proof chain. This question is posed to the
landing agent in the landing brief.

---

## 7. PGo Impact on Refactor/Hardening Strategy

A PGo evaluation mission is staged in the `/private/tmp/pgo-evaluation` worktree
(`docs/missions/pgo-evaluation-v0.md`). PGo compiles Modular PlusCal (MPCal) specs
directly to Go, potentially making the spec-to-code refinement mechanical rather
than hand-written. The evaluation has not yet been executed (PGo not built, MPCal
translation not attempted).

### 7.1 How PGo Changes the Refactoring Calculus

The current changeset is ~40k LOC of **hand-written** Go that implements contracts,
equivalence checkers, observation extractors, and a candidate package intake state
machine. The TLA+ spec has vacuous invariants (issue 1.5) that don't actually prove
the refinement. If PGo works, much of this hand-written code could be **generated
from the spec** instead.

| Refactor item | If PGo is GO | If PGo is NO-GO |
|---|---|---|
| Contract explosion (1.9) | **Moot** — contracts generated from spec | **Must refactor** — extract shared header |
| Large file splitting (1.15) | **Different** — split generated code by archetype | **Must refactor** — split by concern |
| Vacuous TLA+ invariants (1.5) | **Replaced** — real MPCal refinement by construction | **Must fix manually** — independent counters |
| State machine TOCTOU (1.2) | **Still needed** — generated code still needs CAS guards | **Still needed** |
| Upsert ownership bug (1.1) | **Still needed** — store layer is hand-written regardless | **Still needed** |
| Deployment guard (1.7) | **Still needed** — deployment wiring is hand-written | **Still needed** |
| Purity claim (1.6) | **Reshaped** — extraction layer may be generated | **Must fix** — scope or split package |
| cmd binary duplication (1.11) | **Unaffected** — cmd tools are harness, not spec | **Must refactor** |
| Shared contract header | **Skip** — contracts will be regenerated | **Must do** |

### 7.2 Spec Shape Gap

Our specs are raw TLA+ using `CHOOSE`, `EXCEPT`, function construction, and record
updates — no PlusCal/MPCal annotations. PGo requires MPCal (archetypes, mapping
macros, process instantiation). The `promotion_protocol.tla` spec uses:
- `CHOOSE` for initialization (lines 105-106)
- `EXCEPT` for state updates (throughout)
- Function construction `[c \in CandidateComps |-> ...]`
- Bounded model constants

These are standard TLA+ features. MPCal compiles to TLA+, so the expressiveness
should be sufficient, but the translation requires restructuring the spec from
declarative (VARIABLES/Init/Next) to procedural (algorithm/process archetypes).
This is non-trivial but bounded for `promotion_protocol.tla` (it has a clear
state machine: Prepare → Verify → Approve → Commit/Abort).

### 7.3 Recommended Sequencing

**Phase 0 (now): Land the security/correctness fixes and checkpoint docs**
- Upsert ownership bug (1.1): fixed locally
- Optimistic-concurrency guards (1.2): fixed locally
- Write-route deployment guard (1.7): fixed locally
- Checkpoint the ledger (1.4)

**Phase 1 (parallel with Phase 0): Run the PGo evaluation**
- Build PGo, attempt MPCal translation of `promotion_protocol.tla`, generate Go
- This is a few hours of work, not days
- The result determines the refactoring strategy

**Phase 2 (after PGo decision): Refactor/harden**
- **If PGo is GO:** Focus refactoring on the integration boundary (how generated
  code connects to our actor runtime, store layer, and API handlers). Don't
  refactor the hand-written contracts for maintainability — they'll be replaced.
  Rewrite the spec in MPCal, compile, and use generated code as the foundation.
- **If PGo is NO-GO:** Execute the full maintainability refactor — extract shared
  contract header, split large files, fix purity claim, strengthen TLA+ invariants
  manually, consolidate cmd helpers.

### 7.4 Why Not Refactor First?

Refactoring the contract types (1.9) and splitting large files (1.15) is ~60% of
the maintainability work. If PGo is a GO, that work is wasted — the contracts will
be regenerated and the file structure will follow the MPCal archetype structure
instead of the current hand-written organization.

The security/correctness fixes (1.1, 1.2, 1.7) are needed in both branches, so
they should proceed immediately. The PGo evaluation is cheap (hours) relative to
the refactor (days), so running it first is a small investment that prevents
potentially wasted refactoring effort.

### 7.5 PGo Risks That Affect the Decision

- **JDK compatibility:** PGo was tested on JDK 1.11-1.16; we have JDK 25. May need
  JDK 17 via Nix/Homebrew.
- **Research-grade:** No stable releases. May break on modern sbt.
- **MPCal rewrite effort:** Our specs must be restructured from declarative TLA+
  to procedural MPCal. This is spec work, not code work, but it's non-trivial.
- **Generated code integration:** PGo generates Go with a `distsys/` runtime
  library. Integration with our actor runtime, store layer, and API handlers is
  unknown until attempted.
- **distsys dependency:** Generated code depends on PGo's runtime library — an
  external dependency from a research project.

### 7.6 Emergent Question

If PGo works for `promotion_protocol.tla`, should we also express the
`candidate_package_intake` state machine in MPCal and compile it? The intake
flow's 11 operations and state transitions (owner_review_pending → owner_approved
→ adoption_ready) are a natural fit for a PlusCal algorithm. This would replace
~1,688 lines of hand-written state machine code with generated code, and the
TOCTOU concern (1.2) would be addressed at the spec level (atomic actions) rather
than the code level (optimistic concurrency guards).

---

## 8. Suggested Order of Fixes

1. Split and land the changeset per AGENTS.md ordering (1.3)
2. Checkpoint the ledger in phase chunks (1.4)
3. Run PGo evaluation (Phase 1) — determines strategy for items 4-8
4. Either strengthen or drop the two TLA+ invariants (1.5) — or replace with MPCal refinement if PGo is GO
5. Scope the computerversion purity claim (1.6) — or reshape if PGo generates extraction layer
6. Trace ActiveSourceRef consumers (1.8)
7. Extract shared contract header and cmd helpers (1.9, 1.11) — skip if PGo is GO
8. Add broader concurrent transition and deployment-route integration tests (3.4)
