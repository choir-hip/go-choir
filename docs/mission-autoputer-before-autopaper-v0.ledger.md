# Mission Ledger: Autoputer before Autopaper

## Pass 0 — 2026-07-03 05:00 EDT

**Conjecture:** The micro-level evidence from the Wire stabilization mission points to a macro-level substrate problem: the autoputer (persistent computer) is not healthy. The next mission should be autoputer releveling, not Wire patching.

**Move:** construct (create scoping document, document hypotheses, scope dead code, settle predecessor as preamble)

**Expected ΔV:** 5 new conjectures to test (C1-C5) plus 6 definitions (D1-D6)

**Actual ΔV:** 5/5 conjectures proposed, 6/6 definitions recorded

**Definitions recorded:**
- D1: The current `sandbox` is the autoputer in the wrong shape.
- D2: The VM boot loop is a substrate symptom, not a config bug.
- D3: The actor runtime is the correct substrate and is already built.
- D4: The `sandbox` name must be retired from product ontology.
- D5: Nucleus capsules are the future effect chamber but not yet integrated.
- D6: Dead code must be removed aggressively.

**Conjectures proposed:**
- C1: Actor runtime migration makes the autoputer start and bind to port 8085. (UNDECIDED)
- C2: Wire pipeline works on actor runtime without substrate bugs. (UNDECIDED)
- C3: Sandbox → autoputer rename can be done without breaking CI/deploy. (UNDECIDED)
- C4: Nucleus can be installed in the autoputer VM and launch strict-agent capsules. (UNDECIDED)
- C5: Autoputer health enables autopaper (Universal Wire) end-to-end. (UNDECIDED)

**Evidence:**
- Staging VM `vm-universal-wire-platform` is in `failed` state, `stopped_by: recovery_failed`
- VM boots to "Started go-choir Sandbox Runtime" but never binds to port 8085
- Health check fails after 3 minutes: `guest did not become healthy at http://10.200.X.2:8085 within 3m0s`
- Both user VM and universal-wire platform VM show the same failure pattern
- CI is fully green after the event-polling fix (21/21 jobs passed)
- `cmd/sandbox/main.go` already imports `internal/actorruntime` and uses `actorruntime.New(...)`
- Migration plan `docs/actor-runtime-migration-and-agents-md-revision-2026-06-27.md` exists with 8-state state machine
- Naming rectification plan `docs/naming-rectification-2026-06-27.md` exists mapping sandbox → autoputer/computer
- Capsule architecture handoff `docs/archive/handoff-hybrid-computer-capsule-architecture-2026-06-10.md` exists
- No Go code in the repo references `nucleus` or `capsule` — integration is not yet started

**Dead code scoped (high-confidence):**
- `internal/runtime/runtime.go` (3797 lines, old concurrency core)
- `internal/runtime/tools_coagent.go` (check-then-act races)
- `internal/runtime/channel_store.go` (rename/delete, channel→mailbox)
- `internal/sourcegraph/` (inline into `internal/cycle`, delete)
- `internal/sandbox/` → fold into `internal/autoputer`
- `cmd/sandbox/` → `cmd/autoputer/`
- `nix/sandbox-vm.nix` → `nix/autoputer-vm.nix`
- Legacy env vars: `SANDBOX_PORT`, `SANDBOX_ID`, `SANDBOX_FILES_ROOT`

**Open decisions needing owner input:**
1. Confirm exact naming: `cmd/autoputer`, `internal/autoputer`, `nix/autoputer-vm.nix`?
2. Env var naming: `AUTOPUTER_*` vs `COMPUTER_*`?
3. Nucleus integration timing: parallel with rename or sequential?
4. Nucleus source: flake input or vendored/pinned?
5. Staging persistent state: reset or migrate? (destructive)
6. Wire migration scope: include in this mission or hand off separately?

**Next:** Commit this scoping document, update predecessor mission to settled-preamble, get owner decisions on open questions, then begin Phase 1 or Phase 5.
