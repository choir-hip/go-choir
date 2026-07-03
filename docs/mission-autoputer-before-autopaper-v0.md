# Mission: Autoputer before Autopaper

**Status:** scoping / planning  
**Date:** 2026-07-03  
**Predecessor:** `docs/mission-universal-wire-stabilization-v1.md` (preamble: CI green, test infrastructure fixed, D3 scoped)  
**Successor:** `docs/mission-universal-wire-stabilization-v2.md` (autopaper publishing, after autoputer is healthy)  
**Harness:** Parallax + Mission Gradient (15-25 passes)  
**Mutation class:** Red/Black — touches VM lifecycle, runtime substrate, protected surfaces (Texture, promotion, VM lifecycle)  
**Protected surfaces:** Texture canonical writes, corpusd sync contract, source entity graph, promotion/rollback, VM lifecycle, auth/session renewal, gateway/provider calls

---

## Executive Summary

The micro-level work of the last three passes stabilized CI and surfaced the real macro problem: the **autoputer** (persistent user computer, currently called `sandbox`) is not healthy. The VM boot loop on staging — sandbox runtime starts but never binds to port 8085 — is not a config bug to patch. It is the substrate saying that the current computer implementation is the wrong shape for the system Choir is becoming.

The choir sequence is:

> **autoputer before autopaper**

The automatic newspaper (Universal Wire) cannot run on a broken computer. The computer must be releveled first: the durable actor runtime must be wired in, the old borked concurrency core must be deleted, the service name must be renamed from `sandbox` to `autoputer`, and the Nucleus capsule layer must be installed so that supers, vsupers, and cosupers can delegate risky effects to bounded capsules while living inside the persistent computer.

This mission is the macro pivot. The preceding Wire stabilization mission is **settled as preamble**: it produced a green CI, a fixed race-detector test model, and a documentation critique. Those were necessary preconditions. But the actual next step is not "fix the VM boot loop and resume Wire publishing." The next step is the autoputer releveling.

---

## Predecessor Settlement

`docs/mission-universal-wire-stabilization-v1.md` is settled-preamble with these conjectures decided:

- **C1 SUPPORTED** — the flaky test was a test bug (event polling race), not a production data race.
- **C2 SUPPORTED** — fixing the event polling made CI fully green (21/21 jobs passed, race detector shard 2 passed).
- **D3 SCOPED** — canonical doctrine docs are current; stale vocabulary in historical mission docs is appropriate in context.
- **C3 UNDECIDED → deferred** — staging VMs did not recover because the guest runtime never binds to port 8085; the VM recovery failure is a symptom of the autoputer substrate failure.
- **C4/C5 UNDECIDED → deferred** — sourcecycled dispatch and Wire publishing cannot be verified until the autoputer is healthy.

The test-fix work was not wasted. It proved that the CI/test substrate was stale, and it removed the immediate CI blocker. But the work revealed that the runtime/VM substrate is also stale, and that is the actual priority.

---

## Situation Hypotheses (Releveled to Definitions)

These are not uncertain conjectures. They are definitions established by the current architecture and the evidence gathered.

### D1: The current `sandbox` implementation is the autoputer in the wrong shape

The `cmd/sandbox` binary is the persistent computer runtime. It already imports `internal/actorruntime` and wraps the durable actor runtime (`internal/actor`). It already runs the tool profiles, the file handlers, the runtime API, and the wire-pipeline logic. But it is still entangled with the old `internal/runtime` concurrency core (3797 lines, 10+ mutexes), and it is still named `sandbox` — an implementation service name that leaked into the product ontology.

**Implication:** The autoputer is partially built. The work is to finish the actor runtime migration, delete the old concurrency core, and rename the service to match the ontology.

### D2: The VM boot loop is a symptom of the substrate, not a configuration bug

The VM boots, systemd reports `Started go-choir Sandbox Runtime (VM guest)`, but the binary never binds to port 8085. The health check times out after 3 minutes and the recovery loop restarts the VM. This is not a missing env var or a wrong port. It is the sandbox runtime failing to start on the current persistent state / Dolt store / actor runtime wiring.

**Implication:** Patching the VM boot loop is the wrong strategy. The strategy is to replace the substrate so the autoputer starts cleanly.

### D3: The actor runtime is the correct substrate and is already built but not fully wired

`internal/actor` was built on 2026-06-11 with a TLA+ spec. `internal/actorruntime/adapter.go` wraps it. `cmd/sandbox/main.go` already uses `actorruntime.New(...)`. The migration plan in `docs/actor-runtime-migration-and-agents-md-revision-2026-06-27.md` describes the remaining 8-state state machine to extract interfaces, rewire providers, extract tool registry/API handlers, build the adapter (already done), rewire `cmd/sandbox`, migrate the wire pipeline, delete old concurrency code, and verify end-to-end.

**Implication:** The fix is not to build a new runtime. The fix is to execute the already-designed migration and deletion.

### D4: The name `sandbox` must be retired from the product ontology

`docs/computer-ontology.md` and `docs/current-architecture.md` already define the product object as `computer`. The `sandbox` name is implementation-only. The naming rectification plan (`docs/naming-rectification-2026-06-27.md`) already maps `sandbox` → `autoputer` (service/binary name) and `sandbox` → `computer` (product ontology). The rename is planned but not executed.

**Implication:** The autoputer mission includes the rename, because the service name is the handoff artifact. Running an autopaper on a `sandbox` is the wrong ontology.

### D5: Nucleus capsules are the future effect chamber but are not yet integrated

The capsule architecture handoff (`docs/archive/handoff-hybrid-computer-capsule-architecture-2026-06-10.md`) defines the three-layer model: host substrate → persistent computer (autoputer) → ephemeral capsule. No Go code in the repo references `nucleus` or `capsule`. The integration is a new, separable work stream.

**Implication:** Nucleus integration is not a prerequisite for the autoputer to become healthy. It is a parallel/enabled work stream that becomes possible once the autoputer is the correct substrate. But the name `autoputer` and the capsule concept are tied together: the autoputer is the computer that runs supers and delegates to capsules.

### D6: Dead code is extensive and must be removed aggressively

The old runtime concurrency core (`internal/runtime/runtime.go`, `tools_coagent.go`, `channel_store.go`), the `internal/sourcegraph` package, and the legacy `SANDBOX_*` env wiring are all superseded or stale. The migration plan explicitly deletes them in State 7. The deletion-first heuristic (from AGENTS.md) says: prefer deletion over addition when both resolve the bug.

**Implication:** This mission will be measured partly by how much code is removed, not just how much is added. The default move is delete-and-connect, not patch-and-extend.

---

## Dead Code Candidates (Aggressive Removal)

### High-confidence deletions (after actor runtime migration)

1. **`internal/runtime/runtime.go`** — 3797 lines, the borked concurrency core. Replaced by `internal/actor` + `internal/actorruntime`.
2. **`internal/runtime/tools_coagent.go`** — check-then-act races, coagent substrate superseded by actor runtime update messages.
3. **`internal/runtime/channel_store.go`** — rename to `mailbox_store.go` or delete; the `channel` concept is retired, the actor runtime uses `mailbox`.
4. **`internal/sourcegraph/`** — single-file package that projects sourcecycled items into web captures. Inline into `internal/cycle` and delete the package.
5. **`internal/sandbox/` package** — small config/handlers package. Fold into `internal/autoputer` or `internal/computer` as part of the rename.
6. **`cmd/sandbox/`** → **`cmd/autoputer/`** — the service binary. The old name is dead.
7. **`nix/sandbox-vm.nix`** → **`nix/autoputer-vm.nix`** — the VM image.
8. **Legacy env vars:** `SANDBOX_PORT`, `SANDBOX_ID`, `SANDBOX_FILES_ROOT` → `AUTOPUTER_PORT`, `AUTOPUTER_ID`, `COMPUTER_FILES_ROOT` (or similar; needs owner decision).

### Medium-confidence deletions

9. **`internal/runtime/wire_*.go` entanglement with old runtime** — not deleted, but extracted and adapted to actor runtime. The wire logic is domain logic; the old runtime types are substrate.
10. **`internal/proxy/handlers.go` fallback to `PROXY_SANDBOX_URL`** — once all platform computers are real VMs, remove host-sandbox fallback.
11. **Old test fixtures referencing `127.0.0.1:8085` host sandbox** — many tests in `internal/vmctl/vmctl_test.go` and `internal/proxy/handlers_test.go` assume a host process on port 8085. These may need VM-aware mocks or deletions.
12. **Docs in `docs/archive/`** that are historical and not referenced — some can be archived, but not deleted unless confirmed irrelevant.

### Deletion-first questions to answer before each removal

- What depends on this symbol/file?
- Is the replacement already wired in?
- Is deleting + connecting cheaper than patching?
- What is the rollback path if the deletion is wrong?

---

## Sequencing: The Choir Sequence

The mission is ordered by dependency, not by value. The newspaper cannot run before the computer.

### Phase 0: Documentation and rule update (1-2 passes)

- Commit this scoping document.
- Update AGENTS.md with the deletion-first, root-cause-clustering, and dead-end escalation rules (already planned in the actor runtime migration doc).
- Settle the naming decision: `autoputer` for service/binary, `computer` for product ontology.

### Phase 1: Actor runtime migration States 1-3 (interface/tool/API extraction) (3-6 passes)

- State 1: Extract interfaces into `internal/provideriface`, `internal/agentprofile`, etc.
- State 2: Rewire `internal/provider/bridge.go` and `internal/gatewayruntime/provider.go` to new packages.
- State 3: Extract tool registry and API handlers into `internal/toolregistry`, `internal/apihandler`.

### Phase 2: Actor runtime migration States 4-5 (adapter verification and cmd/sandbox rewire) (2-4 passes)

- State 4: Verify `internal/actorruntime` adapter surface matches what `cmd/sandbox/main.go` expects.
- State 5: Rewire `cmd/sandbox/main.go` to use only the actor runtime; remove old runtime dependency.

### Phase 3: Actor runtime migration State 6 (wire pipeline migration) (4-8 passes)

- State 6: Move `wire_*.go` logic to run on actor runtime. This is the high-risk, high-value state. The wire bugs are expected to disappear when the substrate is replaced.

### Phase 4: Actor runtime migration State 7 (deletion of old concurrency code) (2-4 passes)

- State 7: Delete `runtime.go`, `tools_coagent.go`, `channel_store.go`, and remaining old concurrency code.

### Phase 5: Naming rectification (sandbox → autoputer) (2-4 passes)

- Rename `cmd/sandbox` → `cmd/autoputer`.
- Rename `internal/sandbox` → `internal/autoputer`.
- Rename `nix/sandbox-vm.nix` → `nix/autoputer-vm.nix`.
- Update env vars and flake.nix.
- Update docs and comments.

### Phase 6: Nucleus capsule foundation (parallel with Phase 5 if possible) (4-8 passes)

- Add Nucleus to flake.nix.
- Add Nucleus binary to autoputer VM image.
- Define `internal/capsule/` package with `CapsuleRunner` interface and Nucleus backend.
- Add `bash_in_capsule` / `build_in_capsule` tools behind an opt-in flag.
- Add `/internal/capsule/*` endpoints to the autoputer runtime.

### Phase 7: Verification and handoff to autopaper (2-4 passes)

- Deploy autoputer VM to staging.
- Verify guest health on port 8085.
- Verify sourcecycled → processor → Texture → publish.
- Create `mission-universal-wire-stabilization-v2.md` to resume the newspaper work.

---

## Conjectures and Definitions

**Definitions (established, not counted):**
- D1: The current `sandbox` is the autoputer in the wrong shape.
- D2: The VM boot loop is a substrate symptom, not a config bug.
- D3: The actor runtime is the correct substrate and is already built.
- D4: The `sandbox` name must be retired.
- D5: Nucleus capsules are the future effect chamber but not yet integrated.
- D6: Dead code must be removed aggressively.

**Conjectures to test (variant V):**
- C1: Executing the actor runtime migration state machine makes the autoputer start and bind to port 8085. (UNDECIDED)
- C2: The wire pipeline works on the actor runtime without the lost-message/wedged-VM bugs. (UNDECIDED)
- C3: The sandbox → autoputer rename can be done without breaking CI/deploy. (UNDECIDED)
- C4: Nucleus can be installed in the autoputer VM and launch strict-agent capsules. (UNDECIDED)
- C5: The autoputer health enables the autopaper (Universal Wire) to publish end-to-end. (UNDECIDED)

Target V: 0 (all supported or falsified).

---

## Open Decisions (Need Owner Input)

1. **Naming exact:** `autoputer` vs `computer` for service/binary. Doctrine says product is `computer`. `autoputer` is your coinage for the service. Confirm: `cmd/autoputer`, `internal/autoputer`, `nix/autoputer-vm.nix`?
2. **Env var naming:** `AUTOPUTER_*` or `COMPUTER_*` for service env vars? Suggest `AUTOPUTER_PORT`, `AUTOPUTER_ID`, `COMPUTER_FILES_ROOT` to distinguish service from product.
3. **Nucleus integration timing:** Do Phase 5 and Phase 6 run sequentially or in parallel? Nucleus is not strictly required for the autoputer to start, but it is the reason for the name.
4. **Nucleus source:** Use `github:sig-id/nucleus` as a flake input, or vendor/pin a specific rev?
5. **VM persistent state:** The VM is failing to start possibly because the Dolt state on the persistent volume is incompatible with the current runtime. Should we reset staging persistent state, or migrate it? This is a destructive decision.
6. **Scope of State 6 (wire migration):** Do you want to execute the full wire migration as part of this mission, or stop at a healthy autoputer and hand off wire to a separate mission?

---

## Next Move

1. Commit this scoping document as a "problem documentation first" checkpoint.
2. Update the Parallax State of the predecessor mission to "settled-preamble".
3. Get owner decisions on the open questions above.
4. Begin Phase 1: State 1 of the actor runtime migration (extract interfaces), or begin Phase 5 naming if you want to rename first.

The default direction, per the rules and the situation, is: **delete and connect, not patch and extend.**

---

## Ledger

Ledger file: `docs/mission-autoputer-before-autopaper-v0.ledger.md`

## Version / Lineage

- Predecessor: `docs/mission-universal-wire-stabilization-v1.md` (preamble)
- Sibling: `docs/actor-runtime-migration-and-agents-md-revision-2026-06-27.md` (the migration plan)
- Sibling: `docs/naming-rectification-2026-06-27.md` (the naming plan)
- Sibling: `docs/archive/handoff-hybrid-computer-capsule-architecture-2026-06-10.md` (capsule architecture)
- Successor: `docs/mission-universal-wire-stabilization-v2.md` (autopaper publishing)
