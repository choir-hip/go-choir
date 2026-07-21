# Choir Agent Product Doctrine

This file carries product architecture rules for agents working on Choir. It is
loaded on demand when a mission touches authority boundaries, harness behavior,
Texture, runtime configuration, product-path verification, or run acceptance.
Long-running missions now execute as Definition documents (`/goal <doc>.md`);
see [skills/definition/SKILL.md](../skills/definition/SKILL.md). The active
[durable-computer convergence Definition](definitions/choir-coherent-computer-convergence-2026-07-21.md)
is the sole executable top-level product authority. The superseded self-development
Definition and completed audited-construction Definition are historical evidence.
It inherits [Choir Doctrine](choir-doctrine.md) and must not become a competing
doctrine source.

`AGENTS.md` is the operating contract loaded every session; this file is the
deeper product-architecture reference. When they conflict, follow Choir Doctrine
unless `AGENTS.md` is carrying a newer explicitly promoted operating update.

## Authority Boundaries

- `conductor` routes exogenous user/app/connector input into Texture/artifact state. It has no self-development mutation authority.
- Appagents own durable app artifacts. `texture` owns canonical document versions.
- `researcher` reads/researches and may submit only the typed `update_coagent` source-packet mutation through the canonical event appender. It has no bash, raw Dolt, writable files, capsule commit, acceptance, route, or host authority.
- `super` is the foreground orchestration root. It may orchestrate capsules, delegation, inspection, verification requests, and decision proposals; it has no bash, writable/coding, shipper, worker-VM, route, or host tools.
- `co-super` remains an agent loop, but every shell, filesystem, and build effect is a capability-bound guest-local capsule broker verb.
- `vsuper`, candidate-super, and aliases are retired from production profiles for self-development and fail closed.
- Verification is a read-only contract over evidence. It cannot append, accept, materialize, checkpoint, or route an event.

One stable `ComputerID` plus its canonical event chain is the evolving
computer. A frozen capsule effect bundle is speculative and inert. Canonical
desired state changes only by an authorized acceptance event; effective state
changes only after verified guest materialization.

## Current Invariants (2026-07-08)

- **Stable computer identity:** `ComputerID` plus its canonical event chain is
  durable identity. `RealizationID` is replaceable machine state.
- **One event authority:** exactly one trusted guest `ComputerEventAppender`
  validates and sequences semantic events. corpusd mechanically performs typed
  head CAS; Trace, trajectories, embedded state, vmctl, route tables, status,
  checkpoints, and reducers are projections or actuators.
- **Two Dolt stores:** narrow event-head/idempotency and platform-control rows
  live on the existing corpusd world-wire sql-server; the VM-local embedded
  Dolt indexes the chain and materializes effective state. Neither creates a
  third semantic store or alternate head.
- **Acceptance before effect:** an effect bundle is inert until acceptance.
  Guest materialization, checkpoint publication, and route CAS cannot
  acknowledge or substitute for the event.
- **ComputerVersion checkpoint:** `(CodeRef, ArtifactProgramRef)` is an
  immutable reconstruction checkpoint at an event head, not computer identity
  or promotion authority.
- **vmctl projection:** vmctl remains the sole route-slot CAS actuator and
  verifies exact accepted-event, checkpoint, materialization, verifier, and
  route-certificate joins. Post-genesis legacy route authority refuses.
- **No embedded-store promotion:** the obsolete tag/commit/reset
  `DoltPromotionAdapter` is deleted; accepted-event materialization is the only
  self-development path.
- **Timeout hardening landed:** `vmctl.Client` defaults to 60 seconds and the
  server has bounded read/write timeouts (120-second defaults). Staging proved
  the induced resolve-failure path returns a bounded 504; re-prove after a
  routing or timeout change rather than reopening the old 180-second diagnosis.

Texture delegation is agentic. Texture may write, ask researcher, ask super, ask
both, ask neither, wait for more evidence, or report a blocker within its
authority envelope. `edit_texture` stores a canonical revision; it must not become
a semantic workflow gate that requires a subsequent researcher/super/verifier
tool call. Exact required-tool continuation is reserved for narrow mechanical
tool protocols, not appagent policy.

Prompt bar, source ingestion, and article/news creation should show conductor
entry followed by Texture artifact materialization. `super` before Texture is a
route invariant failure. `super` after Texture is valid only when Texture requested
execution through an explicit affordance such as `request_super_execution`.

Prefer asynchronous supervision. Capsule work, verification, and durable
operations return handles and event-derived status. Worker-VM and candidate-VM
delegation are obsolete and deleted, not classified for retention. Generic
delegated agents use durable runs/trajectories and capsules.

Super addresses a CoSuper through the durable run/trajectory and its
capsule-bound operation handle. No VSuper forwarding authority exists for
self-development. A subordinate must not reconcile competing supervisors or
receive a capability from model-visible text.

Verifier agents are read-only with respect to canonical product state. They may
execute only in an independently provisioned read-only capsule whose
capabilities cannot commit, accept, materialize, checkpoint, or route effects.

## Harness Minimalism

Keep the agent loop programmatically uniform where authority permits: provider
call semantics, cancellation, retry, compaction, and durable trajectory
projection should not fork by persona. Capability resolution, privacy-safe
canonical event append, capsule isolation, and typed role policy are deliberate
security boundaries. Production roles are conductor, Texture, Researcher,
Super, CoSuper, and explicitly bounded appagents; VSuper aliases refuse.

Prefer prompts, tool descriptions, capability policy, and product-visible
state over role-specific harness branches. (Prompt content itself is moving
from persona framing toward obligation/authority-envelope framing, but the structural point
here, prompt/policy over code branches, holds either way.) If a proposed fix
requires programmatic divergence in the core loop for one role, document the
evidence, the invariant being protected, the simpler alternatives rejected,
and obtain explicit human approval before landing it. Divergence is acceptable
only when it protects correctness, security, authority boundaries, or
resource isolation in a way that cannot be represented cleanly as policy or
prompt contract.

## Prompt Control-Flow Antipattern

Prompts provide data and invariants, not boolean branches that switch behavior.
A prompt should name the style texture, the available sources, the run context,
and the invariants (cite sources, no model priors as grounded, canonical
revisions via tools). It must not branch on runtime metadata to switch behavior
(`{{if .WireTexture}}`, first-owner-prompt special cases, worker-finding gates).
Unconditional invariant text is not control flow. Decisions that used to live in
prompt branches belong in the style texture, the run context, or tool
plan and `choir-doctrine.md` invariant I16.

Source citation is tri-state and citation shape is a display mode, not a
separate node type (Choir Doctrine I15). Every source entity is cited
(`source_ref` in the body), toolbar-only (a Style.texture style source), or
marked-unused (`mark_source_unused` with a rationale). The former
`source_embed` block node is removed; all citations are `source_ref` with
`display_mode` (`numbered_ref` | `expanded_ref`).

## Texture as Artifact Control Plane

Texture is also Choir's artifact control plane. Conductor routes exogenous
user/app/source input into Texture-owned artifact state: prompt-bar requests,
sourcecycled/news ingestion, article creation, mission work, and most user
prompts should open or create Texture/context first. Super is not the direct
ingress target for ordinary user or source prompts. Texture may later call
`request_super_execution` when the Texture-controlled artifact needs execution,
coding-agent trees, generated artifacts, verification, candidate work, or other
privileged action, and downstream researcher/super evidence must attach back to
the Texture/artifact context.

Read `texture-agentic-invariants-2026-06-13.md` before changing Texture tools,
prompts, routing, revision creation, coagent wake behavior, Trace/Texture
projection, run acceptance involving Texture, or missions that use Texture as
their owner-readable narrative. Texture is the canonical document/versioning
core and must remain an agentic participant in a multi-agent system, not a
workflow runner. Runtime may expose affordances and durable obligations, but it
must not force Texture to call researcher, super, verifier, or any semantic
appagent merely because prompt text, revision metadata, or an acceptance probe
mentions that role.

## Runtime Configuration

Provider secrets and platform model catalogs are platform-owned. Per-computer
model policy is computer-owned durable state and should be editable through the
product path, including by `super` in response to an owner prompt. Do not patch
Node B environment variables or tracked server files as a substitute for a
runtime policy path unless the mission is explicitly a platform config deploy.

Role defaults are policy defaults, not architecture. Any configured model may
serve any production agent role when its declared capabilities match the current
turn: conductor, Texture, researcher, super, co-super, verifier, or a future
bounded role. Text-only models are valid for orchestration, research, coding,
and verification that does not need media input. Multimodal models are required
only when the turn needs screenshots, images, video frames, files, or other
media inputs. If a current policy maps a role to ChatGPT or Fireworks, treat
that as the active computer's effective policy, not a hard-coded role boundary.
Capability is evaluated for the next turn, not permanently for the role.
Do not add new role-specific provider assumptions such as "conductor must be
ChatGPT", "super must be ChatGPT", "Texture must be Fireworks", or "verifier must
be multimodal" unless the current turn's capability requirements actually imply
that. The long-term target is dynamic, agentically editable per-computer model
policy: an owner prompt may ask `super` to edit the computer's model policy,
and subsequent runs should use that policy without a platform deploy or Node B
environment edit. The platform catalog records model capabilities and provider
request semantics; per-computer policy selects among those capabilities.

Provider request schemas must preserve modality. If a task needs screenshots,
videos, files, or other media evidence, route through a model/provider path that
declares that modality and record the blocker precisely when the adapter cannot
resolve the artifact.

## Product-Path Verification

Browser or Playwright acceptance may use public authenticated product APIs such as:

- `/api/prompt-bar`
- `/api/prompt-bar/submissions/{id}`
- `/api/texture/*`
- `/api/trace/*`
- `/api/current-computer`
- `/api/computers/*/self-development/*`
- `/api/continuations/*` (transitional H007/H008 residue; prefer
  trajectory/work-item product evidence when available and do not add new
  continuation-shaped acceptance)
- `/api/run-acceptances/*`

Do not use browser-public internal or test-only routes to bypass the product path:

- `/api/agent/*`
- `/api/prompts`
- `/api/test/*`
- `/internal/*`
- raw event mutation endpoints

The verifier must observe product/control evidence. It must not manually seed success records.

## Run Acceptance Records

For long-running Definition mission proof (`/goal <doc>.md`), the mission's own
evidence ledger and completion semantics in `skills/definition/SKILL.md` govern
what counts as settled. `RunAcceptanceRecord` is a historical evidence
projection for older runs; it is not self-development authority.

The active convergence Definition requires exact source/deploy/host/guest
identity; artifact, subject/activation, obligation, update-disposition, and work
settlement refs; restart/reconstruction and cancellation traces; UI/headless
protocol conformance; authority refusals; rollback; mutation and heresy deltas;
and residual risks. Self-development effects remain OFF.

Do not claim deployed self-development from the superseded Definition, rejected
Round 72 candidate, AppChangePackage/AppAdoption, RunAcceptance, worker/candidate
VM, local tests, a verifier statement, checkpoint publication, or route
transition. A future separately owner-ratified Definition must re-authorize and
prove that product path over the accepted generic kernel.

`continuation-level` is transitional H008/H014 residue: the durable-actor contract re-points this
acceptance level at trajectory/work-item settlement evidence. No deleted
portfolio mission remains executable authority.
Until that cutover lands, retired `continuation-level` keeps its current meaning and
evidence requirement above — do not weaken it and do not claim trajectory
settlement evidence in its place before the level is formally re-pointed.
Do not introduce new retired `continuation-level` claims or APIs as doctrine; M4 must
delete or explicitly shim the old surface.
