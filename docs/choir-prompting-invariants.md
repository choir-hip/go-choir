# Choir prompting invariants

## Status

Operating invariant for seeded agent system prompts and YAML prompt specs.
This document contains the maintained prompting contract; superseded protocol
proposals are available only through Git history.

## No false identity assignment

Choir prompts must **not** open with persona assignment such as:

- `You are a researcher.`
- `You are Choir super.`
- `You are one agent inside Choir…` (when that line assigns a self-image)

Those lines induce role theater: the model performs an identity instead of
advancing proof state under an authority envelope.

## Required opening frame

Each role-specific prompt body should open with a **descriptive frame**, not an
identity claim:

```text
This is the system prompt for the <role> agent in Choir.
```

`<role>` is the functional desk name (`texture`, `research`, `management`, or
`engineering`). `conductor` is deferred to system-one; `processor` and
`reconciler` are deferred to world-wire. `vsuper` is retired.

## Choir context

After the opening frame, explain Choir in third person or as system context:

- multiagent writing, research, and execution;
- one product, one runtime, one standard of truth;
- durable documents, agents, coordination channels, event and revision history;
- agents as participants in a workflow, not isolated chatbots.

The shared `core` prompt carries the cross-role Choir explanation. Role prompts
carry the opening frame plus role-specific theory and operational morphisms.

## Texture role contract

Texture scope is exactly two jobs: revise the owner-readable document and
communicate through semantic acts. It has no capsule, host filesystem,
provider-routing, event-chain, or promotion authority. Humans interface only
with Texture; Texture documents are optimized for human-language prose and
structure. `AuthorUser` is the owner writer and may immediately CAS the
canonical head; `AuthorAppAgent` is Texture and the sole agent writer. Research,
management, and engineering reports are inputs/evidence and never direct
document writes.

A Texture authoring turn that changes semantic state commits exactly one new
monotonic, self-contained snapshot. Prior versions are optional history, never
required context. Wait, block, control, rejected, pending, and no-change turns
may emit `texture_turn_committed` lifecycle events but do not create revisions.
A delivered semantic act may wake or supply a turn; it does not itself require a
revision. A successful patch test should assert a revision only when the patch
changes semantic state.

## Obligation over persona

Prefer obligation, authority envelope, and morphism class over persona:

- what evidence or artifact state must change;
- which tools/oracles are admissible;
- when to checkpoint, incorporate, delegate, or stop;
- stop when **marginal returns diminish**, not when a role “feels done.”

## Semantic-act delivery (2026-09-23)

For the four desks, `update_coagent` is deleted. The desks communicate through
in-cell yaegi `choir.*` functions, not a tool-call channel. `Report` carries
evidence; `Cast` is delegated admission into a per-assignment sub-RLM run;
`Ask`, `Precommit`, `Resolve`, `Cancel`, `Escalate`, and `Note` carry their
respective semantic authorities. Engineering effect verbs are `Complete`,
`Freeze`, and `Verify`. `processor` and `reconciler` retain packet machinery
until their deferred world-wire phase.

### Typed Report bodies, not inferred routing

- A delivered `Report` carries a typed body in the target desk's context with
  its act metadata, delivery phase (`activation_mailbox_turn`, `cold_activation`,
  `mid_activation`, `final_checkpoint`), and structured evidence records.
- Persistent desk RLMs receive pending semantic acts between yaegi-cell
  executions. A desk is a killable subprocess; a sub-RLM is a per-assignment
  run cast by a desk.
- Report identity is runtime-owned for Trace, delivery accounting, and
  debugging. Model-facing prompts and functions must not require an agent to
  invent a globally unique checkpoint key.
- Runtime must **not** traverse spawned-by / parent-run edges to decide the
  recipient. Provenance fields are audit-only.

### One Texture desk binding per article

- Each Texture document/article binds to its persistent Texture desk.
- Research desks report evidence to that binding. Management and engineering
  use explicit semantic recipients and delegated `choir.Cast` where admission
  is required.
- Spawn metadata may name a recipient for convenience; runtime does not infer
  the target from spawned-by lineage.

### Texture wake path

- The common desk reconcile path addresses the bound Texture desk.
- Texture integration runs when pending Reports exist and no conflicting pending
  mutation blocks; evidence arrives in Report bodies, not a separate channel
  prompt embed.
- Failed Texture integration must **not** advance Report delivery state or mark
  evidence resolved without a canonical revision.

### Required tests

- Research Report delivery uses the explicit bound Texture recipient;
- model-facing semantic acts need no model-invented global delivery id;
- retries dedupe by runtime-derived identity while distinct deliveries cannot
  collide because a model reused a local label;
- typed Report construction and persistent desk delivery;
- Texture integration after research evidence produces a revision when the
  patch changes semantic state; wait/no-change turns correctly produce none.

## Research cadence

Research desk search cadence is one source of Texture inputs, but Report count
and revision count are distinct. Texture revises whenever its semantic state
changes, at a cadence ranging from dozens to hundreds per session; it may also
wait, reject, or defer without a revision. Do not treat research Report volume,
`texture_turn_committed` volume, or a fixed lifecycle stage as a revision
guarantee. The selfdev join path's synthetic deterministic `TextureTurnWait` is
a separate defect: it bypasses genuine Texture authoring and must not be
repaired by forcing every worker milestone into a revision.

While research keeps searching and reporting evidence, Texture should keep
incorporating with `patch_texture` and, when helpful, address research with
semantic follow-up questions or delegated `choir.Cast` probes — until depth no
longer materially improves the artifact.

Research should prefer **parallel saturation**: in the same yaegi-cell work,
combine a `choir.Report` with the next `web_search`, `source_search`,
`fetch_url`, or import probe; repeat for multiple rounds in one run until
further searches mostly repeat prior findings and no longer add marginal
grounded material.

## Research delivery addressing

Each Texture article binds to its persistent Texture desk. Research reports to
that exact binding for the active assignment.

- Every research `choir.Report` identifies the bound Texture desk recipient.
- Assignment context may name the delivery target. The research desk copies
  that value into its semantic act; runtime does not infer a recipient from
  spawned-by lineage or channel alone.
- Report bodies preserve the typed evidence, warm delivery, and Texture-wake
  machinery previously carried by packets.

## Enforcement

- Seeded defaults live in `internal/promptstore/defaults/*.yaml` and
  `internal/textureprompts/texture.yaml`.
- Per-run runtime overlays live in `internal/runtimeprompts/overlays/*.yaml`
  (temporal grounding, desk routing, research saturation, management/
  engineering boundaries, worker repo bootstrap, run context).
- Runtime fallbacks in `systemPromptForRun` must use the same frame, not
  `You are Choir <role>.`
- Tests should assert the descriptive opening where they pin default prompt text.

> **Transition note — 2026-09-23:** Prompt YAMLs still carry old role names
> pending the R1 stratum-A sweep. They are transition residue, not current desk
> authority.

## Style-guide Textures (planned)

Style-guide Textures are planned, not implemented. In the planned shape, any
document may be designated as a styleguide that influences future Texture prose
and register; this is a writing input only and does not grant authority or alter
routing. The existing wire-publish style catalog is a precursor, not the
generalized arbitrary-document feature.
