# Choir Agent Product Doctrine

This file carries product architecture rules for agents working on Choir. It is
loaded on demand when a mission touches authority boundaries, harness behavior,
Texture, runtime configuration, product-path verification, or run acceptance.
Long-running missions now execute as Definition documents (`/goal <doc>.md`);
see [skills/definition/SKILL.md](../skills/definition/SKILL.md) and
[docs/definitions/choir-autoputer-completion-suite-2026-07-11.md](definitions/choir-autoputer-completion-suite-2026-07-11.md)
for the current umbrella mission.
It inherits [Choir Doctrine](choir-doctrine.md) and must not become a competing
doctrine source.

`AGENTS.md` is the operating contract loaded every session; this file is the
deeper product-architecture reference. When they conflict, follow Choir Doctrine
unless `AGENTS.md` is carrying a newer explicitly promoted operating update.

## Authority Boundaries

- `conductor` routes exogenous user/app/connector input into Texture/artifact state. It is not the semantic babysitter and not a direct-super router for ordinary prompts.
- Appagents own durable app artifacts. `texture` owns canonical document versions.
- `researcher` writes structured findings/evidence, not canonical text or code.
- `super` is the foreground orchestration root. It can request workers and candidate worlds.
- `vsuper` owns a background/candidate computer or candidate world.
- `cosuper` is subordinate to the super/vsuper that requested or assigned it.
- Verification is a contract over evidence, not a separate privileged caste.

Foreground/canonical state stays stable. Background/candidate computers mutate. Canonical state changes only by promotion.

## Current Invariants (2026-07-08)

- **Route-over-ComputerVersion:** no product route resolves to a VM or desktop
  identity; routes point at `ComputerVersion = (CodeRef, ArtifactProgramRef)`
  records. This is currently violated by the hard-coded platform computer
  fallback in `internal/proxy/route_resolver.go` and `internal/proxy/lineage_route_resolver.go`
  (H031; see `docs/definitions/og-dolt-heresy-completion-2026-07-08.md` I1).
- **Two Dolt stores:** the world-wire store (`internal/platform/objectgraph_store.go`,
  moving to sql-server) and the VM-local embedded store (`internal/objectgraph/dolt_store.go`)
  are distinct substrates. Promotion is an operation on the embedded store, not a
  property of the world-wire store (see D-STORES in the umbrella mission).
- **World Wire (formerly Universal Wire):** the public feed surface for the
  Community Cloud. Documents still using "Universal Wire" are historical or
  pending the Phase E rename.
- **D-PROMO interim:** branch isolation on the VM-local embedded store is
  settled for pinned single-writer connections; the current
  `DoltPromotionAdapter` is tag-only interim and must not be enabled for
  production promotion until the Phase D branch adapter and conformance binding land.
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

Prefer asynchronous supervision. A delegation, worker VM run, candidate preview,
or verification job should leave durable status/evidence and return a handle
rather than blocking the foreground supervisor until completion. If a required
tool is blocking, treat that as runtime debt to repair or document precisely.

Avoid skip-level authority confusion. If `super` needs to address a `cosuper`,
the owning `vsuper` must receive the same instruction or remain the forwarding
authority. A subordinate should not have to reconcile competing directives from
two supervisors.

Verifier agents may be read-only with respect to product/canonical state, but
they are not necessarily computation-only observers. They may run commands,
write temporary scripts, or create tests inside an authorized scratch or
candidate environment when that is required to verify behavior.

## Harness Minimalism

Keep the agent harness small and programmatically uniform across roles by
default. The core tool loop, provider call semantics, run-memory plumbing,
event emission, cancellation, retry, compaction, and continuation mechanics
should behave identically for conductor, Texture, researcher, super, vsuper,
co-super, verifier, and future agent roles unless there is a proven invariant
that requires divergence.

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
serve any agent role when its declared capabilities match the current turn:
conductor, Texture, researcher, super, vsuper, co-super, verifier, or future
roles. Text-only models are valid for orchestration, research, coding, writing,
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
- `/api/app-change-packages/*`
- `/api/computers/*/source-lineage`
- `/api/computers/*/adoptions`
- `/api/adoptions/*`
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
what counts as settled. Synthesize a durable `RunAcceptanceRecord` from that
ledger when a mission reaches `complete` or a clean handoff state. For
self-development proof outside a Definition run, synthesize a record from
existing evidence:

```text
POST /api/run-acceptances/synthesize
```

Required evidence should include trajectory/run ids, authority profile,
build/deploy identity, worker/candidate handle evidence, AppChangePackage/
adoption evidence or a precise blocker, verifier contracts, rollback refs,
heresy delta, conjecture delta, and residual risks. Existing records may still
carry legacy lease vocabulary; treat that as transitional H019 residue, not the
target actor model. Use explicit levels: `docs-level`, `staging-smoke-level`,
`export-level`, `promotion-level`, retired `continuation-level`.

Do not claim `promotion-level` without AppChangePackage adoption verifier contract evidence plus owner review and promote/rollback evidence. Do not claim retired `continuation-level` without run-memory/compaction and continuation evidence.

`continuation-level` is transitional H008/H014 residue: the durable-actor contract re-points this
acceptance level at trajectory/work-item settlement evidence. No deleted
portfolio mission remains executable authority.
Until that cutover lands, retired `continuation-level` keeps its current meaning and
evidence requirement above — do not weaken it and do not claim trajectory
settlement evidence in its place before the level is formally re-pointed.
Do not introduce new retired `continuation-level` claims or APIs as doctrine; M4 must
delete or explicitly shim the old surface.
