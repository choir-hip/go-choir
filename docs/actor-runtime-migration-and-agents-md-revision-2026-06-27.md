# Actor Runtime Migration and AGENTS.md Revision

**Status:** planning
**Date:** 2026-06-27
**Scope:** Wire the durable actor runtime into production, extract and delete the old runtime, and revise AGENTS.md to prevent the failure class that caused two weeks of misdirected debugging

## Context: What happened

On 2026-06-11, the durable actor runtime (`internal/actor/actor.go`) was built with a TLA+ spec to replace the borked concurrency model in `internal/runtime/` that was produced by the automated port from choiros-rs. The new runtime was correct, verified, and committed.

It was never wired in.

For the next two weeks, agents debugged the universal wire / sourcecycled / news pipeline. The wire kept failing: lost messages, wedged VMs, runs that disappeared, backpressure that didn't exist. Each bug was documented and patched per the Problem Documentation First rule. The patches were correct. The documentation was thorough. The direction was wrong.

The wire bugs were symptoms of the borked concurrency substrate. The fix already existed in `internal/actor/`. No agent connected them because:

1. No rule said "check for existing fixes before debugging"
2. No rule said "stop patching after N bugs in the same subsystem"
3. No rule said "escalate when you're not converging"
4. The AGENTS.md optimized for careful incremental work, not for recognizing when incremental work is the wrong strategy

The root cause doc from 2026-06-10 (`universal-wire-empty-front-page-root-cause-2026-06-10.md`) already identified the substrate-level failures: lost wakes, no backpressure, no completion feedback, check-then-act races. These are the exact bugs the actor runtime was built to fix. The connection was never made.

## Cognitive transform analysis

Five transforms were applied to escape the local minimum that produced this failure:

### 1. Substrate transform

The wire bugs are not wire bugs. They are substrate bugs — failures of the concurrency model underneath the wire. Similarly, the AGENTS.md rules are a substrate that shaped agent behavior. The two-week failure was caused by two substrates working together: a broken code substrate (old runtime) and a rule substrate (AGENTS.md) that optimized for patching symptoms instead of replacing substrates.

**Route-changing insight:** Fixing the code substrate without fixing the rule substrate means the next substrate-level failure will produce the same two-week patch loop. Both must change.

### 2. Deletion-first

The migration is fundamentally a deletion problem. The new runtime exists. The work is: extract the non-concurrency machinery that's entangled with the old runtime, rewire callers to the new runtime, then delete the old concurrency code. The AGENTS.md has no deletion-first rule. It has rules for adding documentation, adding ceremony, adding review gates. It has no rule that says "before adding code, check what can be removed" or "before patching, check if the code being patched should be deleted."

**Route-changing insight:** The migration plan should be ordered by deletion safety, not by feature priority. And the AGENTS.md should include a deletion-first heuristic so agents consider removal before addition.

### 3. Local minimum

The patch loop was a local minimum. Each patch made the wire slightly better. Each documentation step was correct. The gradient was downhill — toward more patches, more docs, more careful work. But the global minimum was elsewhere: wire up the actor runtime. No rule provided an escape gradient.

**Route-changing insight:** The AGENTS.md needs escape rules that trigger on non-convergence, not just descent rules that optimize for careful steps. Root cause clustering, dead-end escalation, and check-for-existing-fixes are escape rules.

### 4. State machine

The migration has a dependency graph that determines safe deletion order. Representing it as a state machine makes the sequence verifiable and prevents the "delete something that 18 files depend on" failure mode that makes models bad at refactoring.

**Route-changing insight:** The migration plan below is structured as a state machine with explicit preconditions and postconditions for each state, not as a task list. Each state is safe to enter only when the previous state's postcondition is verified.

### 5. Loss function

What did the AGENTS.md actually optimize for? Careful, documented, incremental, reversible changes. What should it optimize for? Correct substrate-level decisions, with careful incremental work as the execution strategy *after* the substrate is confirmed right.

The loss function was: minimize risk per change. It should be: minimize time-to-correct-substrate, then minimize risk per change. The current loss function rewards spending two weeks patching symptoms because each patch is low-risk. The revised loss function should reward recognizing that the substrate is broken and connecting an existing fix.

**Route-changing insight:** The AGENTS.md revision should make substrate-vs-symptom classification explicit, and should make "connect existing fix" cheaper than "patch symptom" in the rule structure.

---

## Part 1: AGENTS.md Revision

### 1.1 Split the file

The current AGENTS.md is 417 lines mixing three concerns. Split into:

**`AGENTS.md` (~150 lines)** — agent operating rules, loaded every session:
- Default environment (condensed)
- Mutation classes (simplified ceremony — see 1.3)
- Problem documentation first + root cause clustering (see 1.2)
- Check for existing fixes (see 1.2)
- Dead-end escalation (see 1.2)
- Substrate-vs-symptom classification (see 1.2)
- Landing loop (condensed)
- Worktree hygiene
- Safety
- Git and staging

**`docs/agent-product-doctrine.md`** — product architecture rules, loaded on demand:
- Authority boundaries (conductor, texture, super, vsuper, cosuper, verifier)
- Harness minimalism
- Prompt control-flow antipattern
- Texture as artifact control plane
- Runtime configuration
- Product-path verification
- Run acceptance records

**`docs/agent-parallax-rules.md`** — long-running mission rules, loaded when doing Parallax work:
- Parallax section
- Texture narrative rules
- Independent review threads

The split reduces context window tax by ~60% for most sessions and ensures agents aren't loading Texture delegation semantics when they're debugging a concurrency bug.

### 1.2 Add four new rules

Add these to the AGENTS.md operating rules section:

```markdown
## Check for Existing Fixes

Before debugging a bug in subsystem X, search for replacement or
alternative implementations of X in the codebase. If one exists:
- Is it wired in?
- If not, is the bug you're debugging a symptom of the old implementation
  that the new one would fix?
- Is connecting the existing fix cheaper than patching the old code?

If a replacement exists and is not wired in, document the connection
opportunity before patching the old code. Connecting an existing fix is
preferred over patching code that is already superseded.
```

```markdown
## Root Cause Clustering

When you document 3+ bugs in the same subsystem within one week, stop
patching. Write a root cause clustering assessment before the next fix:
- Do these bugs share a common cause?
- Is there existing code that addresses the root cause but isn't wired in?
- Is the substrate itself broken, and are you patching symptoms on top of it?

Apply the substrate-vs-symptom classification (below) to each bug in the
cluster. If 3+ symptoms trace to the same substrate, the next action is
substrate-level, not symptom-level.
```

```markdown
## Substrate vs Symptom Classification

When documenting a problem, classify it:
- **Substrate:** the bug is in a foundational layer (concurrency model,
  message delivery, data persistence, runtime engine, provider interface,
  VM lifecycle, event bus).
- **Symptom:** the bug is in code that runs on top of a substrate.

If you document 3+ symptom bugs traced to the same substrate, apply
Root Cause Clustering before patching the next symptom. The substrate
fix may already exist and just need wiring.
```

```markdown
## Dead-End Escalation

If you've been working on the same problem for 3+ iterations or 2+ days
without convergence, stop patching. Write a structural assessment:
- What's the dependency graph around the problem?
- Is there a substrate-level fix that would eliminate the problem class?
- Are you debugging symptoms because the substrate is broken?
- Does a replacement implementation exist that isn't wired in?

Escalate to the human with the assessment. Do not attempt another
incremental patch without explicit direction. Continuing to patch after
non-convergence is a known failure mode, not persistence.
```

### 1.3 Simplify mutation class ceremony

Current: every orange/red change requires conjecture delta, protected surfaces, admissible evidence class, rollback path, and heresy delta.

Revised:
- **Green/yellow:** name the class, proceed
- **Orange:** name the class and the rollback path. Full ceremony optional unless touching provider routing or VM lifecycle
- **Red/black:** full ceremony required (conjecture delta, heresy delta, protected surfaces, admissible evidence class, rollback path)

The ceremony is most valuable at the boundary where changes are hard to reverse. For most orange changes, naming the rollback path is sufficient. The full ceremony is a token tax that doesn't prevent the failure mode (wrong direction) and does reduce the tokens available for reasoning about direction.

### 1.4 Add deletion-first heuristic

```markdown
## Deletion-First Heuristic

Before adding code to fix a bug, ask:
- Is the code being patched already superseded by a replacement?
- Would deleting the code being patched and connecting the replacement
  be safer than patching?
- What can be removed instead of added?

Prefer connecting an existing replacement over patching superseded code.
Prefer deletion over addition when both resolve the bug. Patching
superseded code extends the life of code that should be removed.
```

---

## Part 2: Runtime Extraction and Actor Migration

### 2.1 The dependency graph

```
cmd/sandbox/main.go
  └── runtime.New()              ← production entry point
       ├── runtime.Config         ← struct, no concurrency
       ├── runtime.Provider       ← interface, no concurrency
       ├── runtime.ToolLoopProvider  ← interface, no concurrency
       ├── runtime.ToolRegistry   ← struct, no concurrency
       ├── runtime.AgentProfile*  ← constants, no concurrency
       ├── runtime.NewAPIHandler  ← HTTP handlers, no concurrency
       ├── runtime.RegisterRoutes ← HTTP routing, no concurrency
       └── [concurrency core]     ← THE BORKED PART
            ├── runtime.Runtime   ← 3797 lines, 10+ mutexes
            ├── channels.go       ← lost wakes
            ├── tools_coagent.go  ← check-then-act races
            └── wire_*.go         ← wire pipeline on borked substrate

internal/provider/bridge.go
  └── runtime.Provider            ← interface dependency
  └── runtime.ToolLoopProvider    ← interface dependency
  └── runtime.ProviderPolicy      ← struct dependency
  └── runtime.ToolLoopRequest/Response ← struct dependencies

internal/gatewayruntime/provider.go
  └── same interface dependencies as bridge.go

cmd/sourcecycled/main.go
  └── (does not import internal/runtime directly for concurrency)
  └── submits work via HTTP to sandbox, which uses old runtime
```

### 2.2 Migration state machine

The migration is a state machine. Each state has preconditions and postconditions. Do not enter a state until the previous state's postcondition is verified.

```
State 0: Baseline
  Precondition: production runs on old runtime, actor runtime exists but unused
  Action: verify current state, snapshot test coverage

State 1: Extract interfaces
  Precondition: State 0
  Action: move runtime.Provider, runtime.ToolLoopProvider, runtime.ProviderPolicy,
          runtime.ToolLoopRequest, runtime.ToolLoopResponse, runtime.ToolDefinition,
          runtime.TokenUsage, runtime.EventEmitFunc, runtime.AgentProfile*,
          runtime.Config into new packages (internal/provideriface, internal/agentprofile, etc.)
  Postcondition: internal/runtime/ still compiles, but re-exports from new packages
  Verification: go build ./... passes, no behavior change

State 2: Rewire providers
  Precondition: State 1 postcondition verified
  Action: update internal/provider/bridge.go and internal/gatewayruntime/provider.go
          to import from new packages instead of internal/runtime
  Postcondition: providers no longer depend on internal/runtime for interfaces
  Verification: go build ./... passes, go test ./internal/provider/... passes

State 3: Extract tool registry and API handlers
  Precondition: State 2 postcondition verified
  Action: move runtime.ToolRegistry, runtime.NewAPIHandler, runtime.RegisterRoutes
          into internal/toolregistry and internal/apihandler (or similar)
  Postcondition: HTTP handlers and tool registry no longer in internal/runtime
  Verification: go build ./... passes, API tests pass

State 4: Build actor-based runtime adapter
  Precondition: State 3 postcondition verified
  Action: create internal/actorruntime package that adapts internal/actor.Runtime
          to the same surface that cmd/sandbox/main.go expects (New, LoadConfig,
          RuntimeOption, etc.). The adapter wraps actor.Runtime and provides
          the provider/tool/API integration points.
  Postcondition: actorruntime compiles, provides same surface as old runtime.New
  Verification: unit tests for actorruntime, go build ./cmd/sandbox passes with actorruntime

State 5: Rewire cmd/sandbox/main.go
  Precondition: State 4 postcondition verified
  Action: replace runtime.New() with actorruntime.New() in cmd/sandbox/main.go
          keep old runtime import temporarily for config loading if needed
  Postcondition: sandbox binary uses actor runtime for concurrency
  Verification: go build ./cmd/sandbox passes, sandbox starts, health endpoint responds,
                basic run submission works

State 6: Migrate wire pipeline
  Precondition: State 5 postcondition verified
  Action: move wire_*.go files (universal_wire.go, wire_publication.go, wire_synthesis.go,
          wire_reconciler_debounce.go, wire_platform_publish.go, tools_wire_processor.go,
          tools_coagent.go) to run on top of actor runtime instead of old runtime.
          These files contain wire logic (not concurrency logic) but are entangled
          with old runtime types. Extract the logic, adapt to actor runtime's
          Handler interface and Update message type.
  Postcondition: wire pipeline runs through actor runtime
  Verification: sourcecycled → processor → Texture → publish → edition → /api/universal-wire/stories
                returns non-empty after a cycle

State 7: Delete old concurrency code
  Precondition: State 6 postcondition verified, no imports of old runtime concurrency
  Action: delete runtime.go, channels.go, tools_coagent.go, and remaining concurrency
          code from internal/runtime/. Keep extracted interfaces/types in their new
          packages. Delete internal/runtime/ entirely if nothing remains.
  Postcondition: internal/runtime/ does not exist or contains only non-concurrency
                 re-exports that will be removed next
  Verification: go build ./... passes, go test -race ./... passes, full test suite passes

State 8: Verify wire works end-to-end
  Precondition: State 7 postcondition verified
  Action: run the deployed acceptance test from the root cause doc:
          - sourcecycled dispatches
          - platform sandbox accepts runs via actor runtime
          - processor creates Texture article revisions
          - autonomous publish to corpusd
          - edition transclusion
          - /api/universal-wire/stories returns non-empty
          - Universal Wire app renders article cards
  Postcondition: wire works end-to-end on actor runtime
  Verification: staging acceptance proof with article cards visible
```

### 2.3 What makes this hard for models

The migration requires holding the entire dependency graph in working memory while making changes that span multiple files. Models lose the thread on this because:

1. **Each file change breaks other files** in ways the model doesn't trace back to its own change
2. **The extraction order matters** — extracting interfaces before rewiring providers is safe; doing it in reverse breaks the build
3. **The wire logic is entangled with old runtime types** — moving wire_*.go requires understanding which types are concurrency types (to be replaced) and which are domain types (to be preserved)
4. **Deletion is harder than addition** — the model must verify nothing depends on the deleted code before deleting, which requires graph reasoning

The state machine structure above mitigates this by making the order explicit and the verification gates explicit. An agent executing state N only needs to understand the dependency graph for that state, not the entire migration.

### 2.4 Point estimates

| State | Description | Est | Risk |
|-------|-------------|-----|------|
| 1 | Extract interfaces | 3 pts | Low — mechanical move, re-export |
| 2 | Rewire providers | 2 pts | Low — import path changes |
| 3 | Extract tool registry + API handlers | 3 pts | Medium — larger surface, more callers |
| 4 | Build actor runtime adapter | 5 pts | Medium — new code, must match old surface |
| 5 | Rewire cmd/sandbox/main.go | 2 pts | Low — single file, but high blast radius |
| 6 | Migrate wire pipeline | 8 pts | High — entangled logic, the actual wire bugs live here |
| 7 | Delete old concurrency code | 3 pts | Medium — must verify no remaining imports |
| 8 | End-to-end wire verification | 3 pts | Medium — staging acceptance proof |
| **Total** | | **29 pts** | |

State 6 is the highest-risk, highest-value state. It's where the wire bugs get fixed (by running on the correct substrate) and where the entanglement is deepest. States 1-5 are preparation. State 7-8 are cleanup and verification.

### 2.5 What to do if State 6 reveals wire logic bugs (not substrate bugs)

After the substrate is replaced, some wire bugs may persist — these would be genuine logic bugs in the wire pipeline, not substrate symptoms. The root cause doc listed candidates:
- Texture edits created but ineligible for autonomous publish (metadata gates)
- Edition transclusion failures
- Corpusd sync failures

These are real bugs that were invisible while the substrate was broken. After State 6, debug them as individual wire logic bugs, not as substrate symptoms. The substrate-vs-symptom classification should be re-applied: if 3+ wire logic bugs cluster in the same wire component, apply root cause clustering again.

---

## Part 3: Execution recommendations

### 3.1 Model selection

This migration is the kind of work GPT-5.5 (and most current frontier models) are structurally bad at: multi-file architectural surgery with dependency graph reasoning. Options:

1. **Human-driven with agent assistance** — you drive the state machine, agent executes individual states. Best for States 4 and 6 (high reasoning, high entanglement)
2. **Agent-driven with human gates** — agent executes states, you verify postconditions at each gate. Best for States 1-3 and 7 (mechanical, verifiable)
3. **Hybrid** — agent does States 1-3, you do State 4 together, agent does State 5, you do State 6 together, agent does States 7-8

The hybrid matches the risk profile: agent handles mechanical extraction, you handle the adapter and wire migration, agent handles cleanup.

### 3.2 Verification discipline

Each state's postcondition must be verified before proceeding:
- `go build ./...` after every state
- `go test -race ./internal/actor/...` after States 4-7
- `go test ./internal/runtime/...` (sharded) after States 1-5 to catch regressions
- Staging acceptance proof after State 8

Do not skip gates. The state machine's safety depends on postcondition verification. Skipping a gate is how you get a half-migrated system where both runtimes are partially wired and neither works.

### 3.3 Rollback

Each state is a commit. If a state fails verification, revert to the previous state's commit. The migration is designed so that States 1-3 are pure refactors (no behavior change) and can be reverted independently. States 4-6 change behavior and should be reverted as a group if State 6 fails.

### 3.4 AGENTS.md changes should land first

The AGENTS.md revision (Part 1) should land before the migration (Part 2). The new rules (check for existing fixes, root cause clustering, dead-end escalation, deletion-first) are what prevent the next two-week patch loop. Landing them first means any agent working on the migration operates under the revised rules.

---

## Lineage

- Predecessor: `docs/choir-rearchitecture-durable-actors-2026-06-11.md` (actor runtime design)
- Predecessor: `docs/universal-wire-empty-front-page-root-cause-2026-06-10.md` (wire failure root cause)
- Predecessor: `docs/production-readiness-checklist.md` (actor model checklist items)
- This document: connects the actor runtime, the wire failures, and the AGENTS.md revision into one coherent migration plan
- Successor: the migration execution (States 1-8) and the AGENTS.md revision commit
