# Texture Agentic Invariants - 2026-06-13

## Status

Doctrine and guardrail document. This records the working Texture semantics after
the M3 regression review, where an acceptance probe accidentally turned Texture
research-desk delegation into a forced runtime workflow.

This document inherits [choir-doctrine.md](choir-doctrine.md). Choir Doctrine
is the apex; this file specializes it for Texture. If this document is used to
justify hidden workflow forcing, first update the doctrine-level conjecture and
protected-surface evidence packet.

Texture is currently the most fragile core of Choir. It is also the key dependency
for almost every higher-level product path: owner-readable mission state,
canonical documents, appagent coordination, source-to-story work, publication,
promotion review, and long-running self-development all rely on Texture behaving
as the versioned artifact control plane rather than a workflow runner.

Read this before changing Texture tools, prompts, routing, revision creation,
coagent wake behavior, trace/Texture projection, run acceptance, or any mission
that uses Texture as its owner-readable narrative.

## Core Invariant

> Texture owns canonical document and artifact state inside a multi-agent system
> and is the delegated controller that keeps its trajectory aligned with the
> owner's intent. It is not a workflow engine, route script, or role-sequence
> executor.

Texture may revise, wait, ask the research desk, ask the management desk, ask
both, ask neither, request clarification, or report a blocker. The correct
choice is part of the Texture agent's obligation and authority envelope. Runtime
may expose tools, durable evidence, pending work, and policy constraints; runtime
must not force a semantic delegation merely because text or metadata mentions
another desk.

Deferred world-wire ingress routes exogenous input into Texture-owned artifact
state. Prompt-bar requests, sourcecycled/news ingestion, article creation,
mission work, and most user prompts should open or create Texture/context first.
Texture then decides whether to write/revise, attach or transclude sources, ask
research, send a semantic act to management, wait, or record an off-document
decision/blocker. Management admits engineering through delegated
`choir.Cast`; it is downstream coherence and admission authority, not the
ordinary ingress target for user or source prompts.

The owner is not the clock of this loop. One Texture actor may write many
versions and redirect downstream agents many times between owner reads. The
owner samples and corrects the current head asynchronously; Texture carries the
delegated responsibility for keeping the trajectory on track between those
interventions.

## Non-Negotiable Rules

1. **Canonical text is Texture-owned.** User revisions and Texture appagent
   revisions are canonical document versions. Research reports, management and
   engineering reports, trace moments, search results, and worker evidence are
   inputs to Texture, not canonical text until Texture incorporates them into a
   revision.

   Canonical text carries semantic control state: current understanding,
   changed beliefs, evidence, uncertainty, intent, and idea-level futures.
   Commands, worker choreography, retries, and action checklists remain in
   addressed messages, work items, artifacts, and Trace. A new revision records
   a semantic state change; it is not an owner notification or approval gate.

   Prompt-bar creation is not an exception. The owner's submitted prompt is the
   canonical `V0` Texture revision. It must not be moved into hidden metadata,
   a separate prompt band, or any other product chrome while `V0` remains blank.
   Metadata such as `seed_prompt` may preserve provenance, but it must not be
   the product display mechanism or the agent's substitute for the canonical
   starting version.

   `V1` is Texture's first response to prompt-bar `V0`. It may be a draft, a
   seed, an acknowledgement, or a work-state revision, depending on what the
   prompt requires. The deferred world-wire/system-one conductor may create or
   open the Texture shell and preserve the prompt, but it must not author the
   first appagent document body.

2. **Texture is the control plane for document and artifact work.** Deferred
   world-wire ingress may classify exogenous input and create or open the target
   Texture/context, but it must not send ordinary prompt-bar, sourcecycled/news,
   article, mission, or document/artifact work directly to management based on
   prompt text. Product-path proof should show ingress, then Texture artifact
   materialization. Management before Texture is a route invariant failure;
   management after Texture is valid only when Texture requested it by semantic
   act.

3. **Delegation is agentic.** Texture decides whether to revise, address
   research, send a semantic act to the persistent management desk, do several
   of those, or do none. A prompt naming research or execution is evidence about
   owner intent; it is not a hard runtime command.

4. **No semantic forced continuations from Texture writes.** `patch_texture` or
   `rewrite_texture` stores a document revision. It must not require a subsequent
   research, management, verifier, or other semantic appagent call.
   Deterministic app protocol handoffs, such as persisting an email draft for
   owner approval, must be explicit, narrow, and documented separately.

   Texture write tools must also not become premature run terminators. A
   successful Texture write stores a canonical revision; the same logical
   `texture:<doc_id>` actor may store later canonical revisions in the same
   physical run as new evidence or owner direction arrives. The verifier and
   Trace evidence must therefore support N:1 loop-to-revision causality instead
   of assuming one run equals one write. A write should not prevent the same
   Texture run from making the next legitimate desk decision, such as opening
   research work, reporting or asking management, recording an off-document
   decision, requesting an email handoff, parking for later reports, or ending
   intentionally.

   Semantic acts are not terminal shortcuts for a parked Texture actor. If
   Texture writes an owner-visible work-state revision and then opens research,
   management, or email handoff work, the actor should reach its normal
   park/passivation path so later addressed `choir.Report` material enters the
   same document thread.

5. **Current work state is canonical without becoming a status dashboard.**
   Texture should not wait silently when an owner-triggered request requires
   research, execution, verification, long reasoning, or delegation. A prompt
   semantic revision may preserve the live objective, uncertainty, and ideas
   being explored. It should not transcribe assignments or tool actions merely
   to show activity. What is forbidden is hiding material background learning
   only in Trace or replacing the owner's intent with a trivial patch.

6. **Required tool choice is not policy.** Exact next-tool enforcement is
   allowed only for mechanical tool protocols whose second call is part of the
   same protocol state, for example a worker allocation followed by a
   worker-start handshake. It must not be used to steer Texture's semantic
   choices.

7. **Prompts and metadata do not replace agency.** Prompt flags, revision
   metadata, and route hints may inform Texture. They must not become hidden
   workflow edges. If the product needs explicit commands, create a visible
   command grammar and still record the resulting obligation as state Texture can
   settle honestly.

   Durable metadata forcing is an explicit violation: persisted flags such as
   the retired `explicit_research_request`, base-revision content scans, or
   carried request-intent fields must not re-derive a required research/management
   delegation across turns. Prompt-pipeline forcing is also a violation:
   prompts and revision builders may describe obligations and affordances, but
   must not mandate the retired "call spawn_agent now" or similar semantic desk
   sequences.

8. **Trace and Texture have different jobs.** Trace is the causal ledger for tool
   calls, LLM content, events, and agent messages. Texture is the owner-readable
   narrative and canonical document surface. Do not turn Texture into a Trace-like
   topology/status dump, and do not use Trace role sequences as a substitute for
   Texture semantics.

9. **Acceptance verifies outcomes, not desk choreography.** A test may require
   research participation only when the product behavior under test is research
   participation. Lifecycle missions must verify lifecycle evidence: open
   obligations, passivation, rewarm, delivered reports, settlement, and no
   stranded work. They must not force a particular Texture delegation sequence as
   a proxy.

10. **Harness minimalism protects Texture.** Do not add Texture-specific branches to
   the core tool loop, provider loop, continuation machinery, or run acceptance
   unless there is a documented invariant, a simpler prompt/policy/tool
   alternative has been rejected, focused regression tests exist, and a human has
   explicitly approved the divergence.

11. **Structured edits are the default for long documents.** Whole-document
   rewrites are exceptional and require rationale. Texture must preserve document
   structure, provenance, revision history, and source/citation semantics.

12. **Texture regressions are architecture regressions.** Treat unexplained Texture
    failures as mission-level blockers. Do not patch around them with one-off
    workflow enforcement unless the invariant being protected is explicitly
    named and reviewed.

## Allowed Runtime Help

Runtime may:

- expose canonical Texture writes, research affordances, semantic acts to
  management, source tools, and other capability-bounded affordances to Texture;
- preserve owner intent, source refs, revision metadata, and trajectory/work
  evidence durably;
- wake Texture from pending `choir.Report` material or assigned work items;
- debounce/coalesce reports before waking Texture;
- mint or derive semantic-act delivery identities before persistence;
- surface pending obligations and missing evidence in prompts;
- prevent duplicate revision writes and protect owner approval boundaries;
- reject invalid edits or unsafe operations.

Runtime may not:

- convert a desk mention into a forced next tool;
- route ordinary prompt-bar, source/news, article, mission, or artifact work
  directly to management before Texture has created or opened the controlling
  artifact context;
- require Texture to ask research/management/verifier after storing a revision;
- terminate Texture merely because a `patch_texture`/`rewrite_texture` call
  succeeded when unresolved report, decision, or handoff obligations remain;
- silently satisfy Texture obligations through another desk's route;
- mark exact internal desk sequence as acceptance unless that sequence is the
  product requirement;
- hide desk-specific control policy in generic tool-loop continuation code.

Texture tool inventory should match Texture authority. Research-owned evidence
gathering and provider/model diagnostics should not be bundled into Texture
simply because they share an implementation registry. Split memory, evidence,
and diagnostic affordances when needed instead of giving Texture a large generic
tool bag.

## Semantic-act delivery (2026-06-17)

The four desks communicate by in-cell yaegi `choir.*` functions, not a
tool-call channel. `choir.Report`, `Cast`, `Ask`, `Precommit`, `Resolve`,
`Cancel`, `Escalate`, and `Note` are the agent-to-agent surface. The retired
`update_coagent` interface is deleted for Texture, management, engineering, and
research; its packet machinery survives as `Report`'s body and for deferred
processor/reconciler/conductor roles until their world-wire/system-one phase.

### Typed acts, not inferred routing

- Every delivered act has a typed envelope, recipient, causal provenance, and
  runtime-owned idempotency identity. A model never invents that identity.
- **Persistent root desks** run yaegi Go cells in killable subprocesses. Warm,
  parked, and rewarmed activations receive their addressed acts through the
  desk's durable inbox; the delivery mechanism must not impose a semantic next
  step.
- `choir.Report` carries evidence assertions. Reports resolve commitments on the
  tape; commitment objects are OG objects, not a separate store, and scores stay
  outside the acting desk's context.
- Runtime must **not** traverse spawned-by / parent-run edges to decide a
  recipient. Provenance fields are audit-only.
- Deleting a Texture document cancels the addressed `texture:<doc_id>` actor and
  any pending Texture revision trajectory before removing canonical document
  rows. Deletion must not leave a parked actor or pending mutation able to write
  a deleted document.

### One Texture desk per article

- Each Texture document/article has a durable Texture desk id:
  `texture:<doc_id>`.
- Research and engineering acts about that article address that exact id through
  their semantic-act recipient. Runtime does not infer a recipient from a
  caller's ancestry.
- Management is the only desk that admits engineering work, through delegated
  `choir.Cast`; it escalates unresolved owner decisions.

### Texture wake path

- A Texture integrate run starts when pending reports exist and no conflicting
  pending mutation blocks. Evidence arrives through the addressed semantic-act
  inbox, not a separate channel-only prompt embed.
- A failed Texture integrate run must **not** resolve its commitments or mark
  report material incorporated without a canonical revision.

### Owner-triggered revision cadence

- For prompt-bar creation, `V0` is the owner prompt and `V1` is Texture's first
  response to that prompt.
- For direct Texture editing, the user-authored current revision is already
  canonical artifact state. A later revise request should not be interpreted as a
  requirement to clean up the document before Texture can think, delegate, or
  wait.
- Every owner-triggered request should produce owner-visible artifact state
  promptly when it cannot complete immediately. That state may be an
  acknowledgement, an active-work note, a short plan, or a precise blocker.
- Background work must be represented in Texture as owner-readable state, not
  only in Trace or Chyron. Later revisions should consume durable reports and
  source entities when they arrive.

### Required tests for this contract

- semantic-act delivery rejects a missing or invalid Texture recipient;
- model-facing acts do not require a model-invented delivery identity;
- retries of the same delivery remain idempotent while distinct payloads cannot
  collide on a human checkpoint label;
- desk inbox delivery across warm, cold, parked, and rewarmed activations;
- Texture wake after a research report produces a revision when the model
  patches;
- owner-triggered Texture work that delegates or waits writes an honest
  work-state revision instead of a trivial instruction-removal patch;
- document deletion cancels the pending or parked Texture actor before deleting
  the document;
- workflow verification accepts many appagent revisions from one Texture loop
  when each revision has write-tool evidence and valid parent causality.

## Problem: Texture integrate wake was blind on turn 1 (2026-06-17)

Observed on staging `5e17138f` during the deployed live-search eval: a research
result reached `texture:<doc_id>` through the then-current legacy packet path,
but the Texture document stayed at v0 with "Texture run completed without storing
a Texture revision" / "Revision failed". The activity log showed the Texture
integrate run hitting repeated tool errors and ending without a canonical
revision.

Root cause (logic, model-independent):

1. **First-turn blindness.** The integrate wake did not seed the pending finding
   for its first inference turn. Its cold-prepend guard was false, and delivery
   had moved from prompt embedding to injection. The model therefore had the
   document and diff but none of the grounded finding until after the first tool
   round or at the end-turn checkpoint. A model that ended the first turn with
   prose could complete the run before the evidence entered context.
2. **Historical detector finding: no "must act" constraint on integrate.**
   Grounded integrate wakes had no initial durable-action constraint, so they
   could legally end with prose and produce no durable artifact, surfacing as
   "Revision failed".

This was distinct from delivery accounting: a no-write integrate left evidence
pending for re-wake rather than silently dropping it.

### Intended invariant

- A Texture integrate wake must place pending report evidence in the model's
  context on its **first** inference turn, matching warm-inbox delivery.
- A grounded integrate turn must take a **durable action**: write
  (`patch_texture`/`rewrite_texture`), send a capability-bounded semantic act or
  follow-up, or record an explicit Texture decision
  (`record_texture_decision`). It must not silently end with prose. This keeps
  Texture agentic (it chooses which durable action) while banning the silent
  no-op that presents as "Revision failed".

### Required tests for this fix

- integrate wake has first-turn report evidence;
- grounded integrate requires a durable action;
- a no-write integrate leaves report evidence pending for re-wake.

## Source entity tri-state and citation display mode (2026-06-23)

Source citation is tri-state. Every source entity on a revision is in exactly
one of three states:

- **cited**: a `source_ref` node in the body references the entity;
- **toolbar-only**: a Style.texture style source that shapes the document but is
  intentionally not cited in the body (it is a control input, not a content
  source);
- **marked-unused**: the model called `mark_source_unused` with the
  `source_entity_id` and a rationale, recorded in revision metadata under
  `unused_source_entity_ids`.

No source is silently ignored. The schema validator requires every material
source entity to be referenced by a `source_ref` node, with the marked-unused
list as the explicit audit exception.

Citation shape is a display mode, not a separate node type. The former
`source_embed` block node is removed. All citations are `source_ref` nodes:

- `display_mode: numbered_ref` — collapsed inline citation point that expands to
  reveal the source title and excerpt (default).
- `display_mode: expanded_ref` — expanded block showing the title and excerpt.

The reader can toggle any `source_ref` between expanded and collapsed. The
toggle is local reader UI state and does not mutate the canonical document.

There is no `WireTexture` prompt control-flow branch. Article-format and
citation guidance is unconditional for every Texture run, driven by the default
Style.texture (`styles/default.style.texture`). See Choir Doctrine I15 and I16.

### Required tests for this contract

- schema rejects `source_embed` nodes and `display_mode` values outside
  `numbered_ref`/`expanded_ref`;
- `mark_source_unused` allows an uncited source entity when its id is in the
  unused list, and rejects it otherwise;
- `patch_texture` rejects `insert_source_embed` and accepts `mark_source_unused`
  only with a rationale;
- `expanded_ref` `source_ref` nodes project as an expanded block marker and
  render as a block in the frontend;
- prompt generation contains no `insert_source_embed` string and no
  `{{if .WireTexture}}` branch.

## Regression From M3

During M3, the deployed restart proof required Trace to show deferred
world-wire ingress, Texture, research, and management before vmctl refresh. When
research did not appear, the mission drifted from durable-actor lifecycle proof
into trying to force Texture to open research work. The final shape returned the
retired `next_required_tool=spawn_agent` from `edit_texture` and relied on the
generic tool loop to enforce exact `spawn_agent`.

That was a regression. It made a probe precondition the runtime semantics.

Correct recovery:

- remove hard research continuation from Texture;
- document this invariant in worker-facing docs;
- test that Texture is not forced by desk mentions;
- redesign M3 acceptance around lifecycle evidence;
- keep research participation as a possible Texture choice, not a runtime
  workflow step.

## Required Tests For Future Changes

Any behavior-changing Texture coordination change should include tests proving:

- prompt-bar and source/article ingress enters Texture-owned artifact state
  before any management admission;
- prompt-bar `V0` preserves the owner prompt and `V1` is Texture's first response
  to that prompt;
- direct user-authored Texture documents can receive work-state revisions without
  a forced trivial cleanup patch;
- `edit_texture` does not emit semantic `next_required_tool` values (this retired detector term must not reappear);
- prompts mentioning research or management do not force a delegation;
- Texture still has access to research and management semantic-act affordances
  and can choose them;
- owner-visible Texture state names active background work when delegation,
  research, execution, or verification is underway;
- research findings remain non-canonical until Texture incorporates them;
- public/product acceptance observes outcomes and obligations, not hidden desk
  sequence;
- long-document revisions preserve structured-edit defaults and operation
  evidence.

Tests to invert or delete when M3.1 repairs H010/H024/H026:

- tests that expect `edit_texture` to emit the retired `next_required_tool=spawn_agent`;
- tests that preserve research intent through durable revision metadata as a
  forced follow-up;
- tests that treat base-revision content mentioning research as a required
  delegation oracle;
- tests that require Texture's first tool to issue a retired privileged-execution
  request because a prompt matched management keywords;
- tests that require Texture's first tool to be `patch_texture` for every
  owner-triggered revision regardless of request origin and work state;
- prompt-default assertions that encode a fixed Texture -> research -> management
  sequence instead of obligations and evidence.

## Protected Surface Rule

Texture canonical writes, revision metadata, prompt routing, desk inbox wake
behavior, Trace/Texture projection, and acceptance involving Texture are protected
surfaces under Choir Doctrine. Before changing them, name the mutation class,
conjecture delta, evidence class, rollback path, protected surface touched, and
heresy delta (`discovered`, `introduced`, `repaired`).

## Short Rule For Agents

If a proposed Texture change makes the sentence "Texture is required to call X next" true for
a semantic agent role, stop. You are probably turning the multi-agent system
into a workflow. Document the problem, shift the observer, and protect Texture's
agency before writing code.
