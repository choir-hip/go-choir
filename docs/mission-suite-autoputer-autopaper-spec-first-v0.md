# Mission Suite: Autoputer / Autopaper — Spec-First Substrate Redesign

**Status:** scoping / paradoc  
**Date:** 2026-07-03  
**Predecessor:** `docs/mission-autoputer-before-autopaper-v0.md`  
**Harness:** Parallax + Mission Gradient, orchestrated via subagent suite  
**Budget:** 15-25 passes across the suite, tonight  
**Mutation class:** Red/Black — touches runtime substrate, VM lifecycle, TLA+ specs, protected surfaces (Texture, promotion, wire, VM lifecycle)  
**Protected surfaces:** Texture canonical writes, corpusd sync contract, source entity graph, promotion/rollback, VM lifecycle, auth/session, gateway/provider calls

---

## Executive Thesis

The current system is a stack of migrations that each outran their specification:

- The **actor runtime** was built and partially wired, but the old `internal/runtime` package still lives as a zombie business-logic library.
- The **object graph** migration changed the data model, but the specs still speak in old vocabulary.
- The **Universal Wire** was patched on top of a borked substrate, while a formal redesign of the wire pipeline already exists in `specs/wire_pipeline.tla`.
- The **autoputer** (persistent computer) is failing to boot because the service still called `sandbox` tries to start the old runtime.

The missing step is not more code. The missing step is a **redefinition of the TLA+ specification** so that it describes the system as it is now — actor runtime + object graph + autoputer + capsules + wire — and then driving the code to match the spec.

> **TLA+ is the spec. The compiled, verified spec. We center it.**

This mission suite redesigns the specs first, then executes the code migrations that make the code conform to the new specs. The specs are not a verification afterthought; they are the source of truth for what the system does.

---

## Central Conjecture

```text
If we rewrite the TLA+ specs to model the current architecture
(actor runtime + object graph + autoputer + capsules + wire),
and then refactor the code so that the Go implementation is a
mechanical refinement of those specs, then the autoputer will boot
 cleanly, the wire pipeline will publish end-to-end, and the system
will be ready for scale-up.
```

The deeper goal (G): a self-describing, model-checked automatic computer that runs an automatic newspaper. The spec is the contract; the code is the implementation.

---

## Current Spec Inventory (and Staleness)

| Spec | File | Status | Notes |
|------|------|--------|-------|
| **actor_protocol.tla** | `specs/actor_protocol.tla` | **Delete** | Single-process mailbox model. We rewrite it for object-graph durable state. |
| **actor_protocol_xvm.tla** | `specs/actor_protocol_xvm.tla` | **Delete** | Active/candidate computer split is correct but lacks Nucleus capsule and autoputer lifecycle boundaries. |
| **wire_pipeline.tla** | `specs/wire_pipeline.tla` | **Delete** | Publication-trajectory idea is sound but the vocabulary predates the object graph. We rewrite it as the driver for the wire redesign. |
| **promotion_protocol.tla** | `specs/promotion_protocol.tla` | **Delete** | The two violations it flagged (`NoStaleCommit`, `ApprovalGate`) have been fixed in the Go code. The spec now describes a prior architecture, not the autoputer. |

**Default decision: delete old specs, write new ones in place.** We are pre-launch. Only good code. The old `.tla` files are removed and replaced with new specs that model the current architecture.

**The promotion protocol is the gate.** A persistent computer is not an autoputer until candidate promotion is model-checked, verified, and encoded. The autoputer must be able to fork itself, verify the fork, approve the promotion, atomically flip the route, and roll back if the health window fails. That is the promotion protocol, and it is the first spec we rewrite.

---

## Mission Suite Architecture

The suite has one **central spec mission** and three **implementation missions**. The spec mission writes the truth; the implementation missions refine the code to match it.

### Mission S: TLA+ Spec Redesign (the center)

**Goal:** Rewrite the TLA+ specifications to describe the system as it is now and as it will be after the migration. Old specs are deleted and replaced.

**Deliverables (in priority order):**
1. **`specs/promotion_protocol.tla`** — candidate promotion with object-graph atomic flip and rollback. **This is the gate.** It models the computer ontology (active, candidate, route identity), the ledger split, the promotion certificate, and the health window. Written first because nothing else can be called an autoputer until promotion is model-checked.
2. `specs/actor_protocol.tla` — actor runtime with object-graph state (Dolt-backed objects and edges as the durable log).
3. `specs/actor_protocol_xvm.tla` — cross-VM + cross-capsule message protocol. Agents live in the autoputer; capsules are ephemeral effect chambers.
4. `specs/wire_pipeline.tla` — wire pipeline on object-graph trajectories and editions. The spec drives the wire redesign; the Go code is a refinement.
5. `specs/autoputer_lifecycle.tla` — autoputer VM boot, health, recovery, and hibernation. Explains why the current VM fails to bind to 8085.
6. Updated `specs/README.md` — plain-language guide to the new specs and their layering.

**Invariants:**
- Every spec must be model-checked by TLC in CI.
- Every spec change must be written before the corresponding code change.
- No spec may be weakened to make the current code pass.
- If the code already matches an intended invariant, the spec must encode that invariant and model-check green (not just flag old violations).

**Conjectures to test:**
- C-S1: `actor_protocol.tla` holds under object-graph semantics. (**SUPPORTED** — main CI run `28684139979`)
- C-S2: `wire_pipeline.tla` captures the sourcecycled → processor → Texture → publish → edition flow. (UNDECIDED)
- C-S3: `autoputer_lifecycle.tla` model-checks the boot/recovery safety model. (**SUPPORTED** — main CI run `28684139979`; refreshed guest health remains an implementation gap under Mission C)
- **C-S4 (gate):** `promotion_protocol.tla` models active/candidate/route/rollback/health-window and checks green against the intended autoputer protocol. (**SUPPORTED** — CI run `28648508586`, 826 states, no errors)
- **C-S5 (gate):** `promotion_protocol.tla` encodes `NoStaleCommit`, `ApprovalGate`, `NoTornOutcome`, `RouteConsistency`, `CandidateIsolation`, `HealthWindowReversible`, `ConfirmedLedgersApplied`, `AbortedLedgersRolledBack`, `CertificateCompleteness`, and liveness `EveryCommittedPromotionSettles` / `SystemProgress`. (**SUPPORTED**)

---

### Mission A: Actor Runtime Defactoring (refines actor_protocol.tla)

**Goal:** Delete the old `internal/runtime/runtime.go` and make the actor runtime the sole business + concurrency substrate.

**Depends on:** Mission S's `actor_protocol.tla` to define the exact state/transition model.

**Deliverables:**
- Move `ResolvedLLMConfigFromMetadata` and `MaxInteractiveOutputTokensForSelection` from `internal/runtime` to `internal/provideriface` (completed in Pass 2).
- Move `NewAPIHandler` and `RegisterRoutes` behind `internal/apihandler` and wire `cmd/sandbox` through that package (completed in Pass 2).
- Move stub provider and config defaults to `internal/provideriface`.
- Refactor `internal/actorruntime` so it does not embed `*runtime.Runtime` for business logic.
- Extract remaining business logic from `internal/runtime/runtime.go` into `internal/runengine` or directly into `internal/actorruntime`.
- Delete `internal/runtime/runtime.go`, `tools_coagent.go`, `channel_store.go`.
- `cmd/sandbox/main.go` imports no `internal/runtime`.

**Conjectures:**
- C-A1: `internal/runtime` package can be deleted. (UNDECIDED)
- C-A2: `cmd/sandbox/main.go` builds using actor runtime + extracted helpers. (**SUPPORTED** — PR #42 merged; main CI run `28684139979`; sandbox Nix package fixed in `02fa2ea6`)
- C-A3: Actor runtime tests pass under `-race`. (UNDECIDED)

---

### Mission B: Wire Pipeline Redesign (refines wire_pipeline.tla)

**Goal:** Move the Universal Wire pipeline from the old in-run processor model to the object-graph trajectory model defined in the spec.

**Depends on:** Mission A (actor runtime is the sole substrate) and Mission S's `wire_pipeline.tla`.

**Deliverables:**
- Create `internal/wire/` package from `wire_*.go` files in `internal/runtime`.
- Decouple wire logic from old runtime types; use actor runtime `Update` messages and object-graph operations.
- Implement processor → Texture → publish → edition as durable trajectory state. (Default decision: leave `sourcecycled` alone for now; focus on the processor-to-publish core.)
- Wire the processor agent to spawn Texture agents via actor runtime, not via in-run goroutines.
- Verify with fake providers and fake clock; then verify on staging.

**Conjectures:**
- C-B1: Wire pipeline compiles and unit tests pass with fake providers. (UNDECIDED)
- C-B2: Wire pipeline produces a real article on staging. (UNDECIDED)
- C-B3: Every fetched item's story eventually settles (matches spec liveness). (UNDECIDED)

---

### Mission C: Autoputer Solidification (refines actor_protocol_xvm.tla + autoputer_lifecycle.tla + promotion_protocol.tla)

**Goal:** Rename the service from `sandbox` to `autoputer`, make the VM boot cleanly, install the Nucleus capsule foundation, and make promotion the operational gate for any computer mutation.

**Depends on:** Mission A (the binary no longer depends on old runtime), but rename/packaging can start in parallel. Mission C's promotion logic depends on Mission S's `promotion_protocol.tla`.

**Deliverables:**
- Rename `cmd/sandbox` → `cmd/autoputer`, `internal/sandbox` → `internal/autoputer`, `nix/sandbox-vm.nix` → `nix/autoputer-vm.nix`.
- Update env vars and flake.nix.
- Add Nucleus flake input and include Nucleus binary in the autoputer VM image. (Default decision: pin a specific `github:sig-id/nucleus` rev.)
- Define `internal/capsule/` package with `CapsuleRunner` interface and Nucleus backend.
- Add `/internal/capsule/*` endpoints to the autoputer runtime.
- Add `bash_in_capsule` tool behind opt-in flag.
- Move promotion logic from `internal/runtime` to `internal/autoputer/promotion` (or `internal/promotion`), aligned with the new `promotion_protocol.tla`.
- Deploy autoputer VM to staging; verify health on port 8085. Active refreshed guest health currently fails readiness during deploy and is the next Mission C boot-realism gap.

**Conjectures:**
- C-C1: Autoputer VM image builds with Nucleus included. (UNDECIDED)
- C-C2: Autoputer VM boots and binds to port 8085 on staging. (UNDECIDED)
- C-C3: Nucleus can launch a strict-agent capsule inside the autoputer VM. (UNDECIDED)
- C-C4: Promotion protocol end-to-end works on staging: candidate → verify → approve → promote → health window → confirm. (UNDECIDED)

---

### Mission D: CI / Verification Guard (runs continuously)

**Goal:** Keep CI green on every commit from missions A/B/C/S and verify the TLA+ specs remain model-checked.

**Deliverables:**
- Update CI to run TLC on all specs.
- Ensure race detector tests pass after every substrate change.
- Monitor staging deploys and health.
- Document TLA+ review findings.

**Conjectures:**
- C-D1: CI passes after each mission commit. (**SUPPORTED**)
- C-D2: TLA+ specs model-check in CI. (**SUPPORTED**)
- C-D3: Race detector model is correctly scoped. (SUPPORTED from predecessor)

---

## Dependency Graph

```
Mission S (Spec Redesign)
    ├──► Mission A (Actor Defactoring) ──► Mission B (Wire Redesign)
    │                                          │
    ├──► Mission C (Autoputer Solidification) ◄┘
    │       (rename/packaging parallel; full verify after B)
    │
    └──► Mission D (CI/Verification Guard) — runs across all
```

Mission S is the center. It produces the specs. Within Mission S, **`promotion_protocol.tla` is the gate** — it is written first, and Mission C's promotion work is blocked until it model-checks green. Missions A, B, and C refine the code to match. Mission D verifies both specs and code continuously.

---

## Spec-First Workflow

Every implementation change must follow this sequence:

1. **Spec change first** — update the TLA+ module to reflect the new behavior.
2. **Model check** — run TLC; confirm the spec still holds or deliberately accepts a new counterexample as a known limitation.
3. **Code change second** — refactor the Go code to match the spec.
4. **Build/test** — `go build ./...`, `go test -race ./...`, Nix build.
5. **Staging proof** — deploy and verify the behavior on `choir.news`.
6. **Ledger update** — record the spec change, the code change, and the evidence.

This is the spec-centered version of the Landing Loop.

---

## Invariants / Qualities / Domain Ramp

- **I:** Do not change a spec to make the current code pass. If the code violates the spec, either fix the code or explicitly weaken the spec with a documented rationale.
- **I:** Do not delete TLA+ specs without replacing them with a more precise spec.
- **I:** Do not touch Texture core (O1-O3 canonical writes), corpusd sync contract, or source entity graph structure without a spec update.
- **Q:** Every spec must be model-checked in CI.
- **Q:** Every mission commit must keep CI green.
- **D:** Spec → model check → code refactor → build/test → staging proof → scale-up ready.

---

## Variant (Conjecture Descent)

V = count of undecided conjectures across the suite.

Initial: 5 (S) + 3 (A) + 3 (B) + 4 (C) + 3 (D) = **18 conjectures**.
Current: 1 (S) + 2 (A) + 3 (B) + 4 (C) + 1 (D) = **11 conjectures remaining**. (C-S1, C-S3, C-S4, C-S5, C-A2, C-D1, C-D2 decided.)
Target: 0.

Each pass must decide at least one conjecture or discover a new conjecture with evidence. A pass that only adds code without changing the spec or deciding a conjecture is not descent.

---

## Default Decisions Made (Owner Can Override)

| # | Question | Decision | Rationale |
|---|----------|----------|-----------|
| 1 | Spec naming | **Delete old specs, write new ones in place.** | Pre-launch; only good code. Old specs describe prior architectures. |
| 2 | TLA+ CI tooling | Keep existing CI command; add per-spec `.cfg` files as needed during Mission S. | The command is correct; new specs may need individual configs. |
| 3 | Nucleus version | **Pin a specific rev.** | Reproducible builds. Mission C picks the rev. |
| 4 | Staging persistent state | **Reset persistent Dolt state on staging.** | Pre-launch; only good state. Migration is not worth the risk before launch. |
| 5 | Wire redesign scope | **Processor → Texture → publish first.** | Leave `sourcecycled` alone; focus on the core trajectory model. |
| 6 | Autoputer rename timing | **Rename in parallel with Mission A**, keep binary working. | Final verify waits for Mission A. |
| 7 | Promotion protocol | **Redefine spec first, then encode it.** | The old spec is stale; current code already fixed the two violations. The autoputer needs a new, comprehensive promotion protocol spec. |

---

## Orchestration Plan (Tonight)

I act as orchestrator, spawning subagents for each mission track.

### Pass 1 (complete)
- **Mission S subagent (promotion gate):** `specs/promotion_protocol.tla` rewritten and model-checked green in CI run `28648508586`. The gate is established.
- Default decisions recorded and old specs deleted.

### Pass 2 (complete)
- Mission S: `actor_protocol.tla` and `autoputer_lifecycle.tla` landed and model-check green in main CI.
- Mission A: model-policy helpers moved to `internal/provideriface`; API handler wrapper package `internal/apihandler` landed; sandbox package source filter repaired.
- Mission D: Codex review artifact landed.
- Evidence: PR #42 merged as `a6f11b7dbb64c07677a767c19c00e47cf87fdd54`; main CI run `28684139979` green.
- Known limitation: deploy-time active computer refresh still fails guest health on `:8085`; host services were healthy. This is Mission C boot-readiness work, not Pass 2 extraction work.

### Pass 3+
- Open Mission C active-refresh/autoputer boot readiness definition before promotion encoding.
- Continue remaining Mission A extraction/deletion only after preserving the boot-readiness boundary.
- Rewrite/model-check wire pipeline in Mission B once actor/runtime deletion risk is bounded.
- Address Codex reservations before moving promotion logic from `internal/runtime` to `internal/autoputer/promotion`.
- Keep Mission D running as guard.

Every subagent reports conjecture status, evidence, and proposed next move. I coordinate handoffs and verify builds. The promotion spec is the gate: Mission C promotion work does not proceed until `promotion_protocol.tla` checks green.

---

## Ledger

Ledger file: `docs/mission-suite-autoputer-autopaper-spec-first-v0.ledger.md`

## Version / Lineage

- Predecessor: `docs/mission-autoputer-before-autopaper-v0.md`
- Sibling: `docs/actor-runtime-migration-and-agents-md-revision-2026-06-27.md`
- Sibling: `docs/naming-rectification-2026-06-27.md`
- Sibling: `docs/archive/handoff-hybrid-computer-capsule-architecture-2026-06-10.md`
- Sibling: `specs/README.md` (to be updated by Mission S)
- Successor: `docs/mission-universal-wire-stabilization-v2.md` (autopaper publishing)
