# VText Regression Review - 2026-05-31

Status: report only. No code changes were made for this review.

Scope: git history and current source review from the first e-text/VText
implementation on 2026-04-11 through current `main` at `ba49f0f`. This review
does not claim staging reproduction of the current owner-visible failure. It
uses commit history, mission docs, and current code to identify why VText keeps
regressing and where "versions are not advancing" is most likely to recur.

## Executive Read

VText is not failing because of one missing fix. The last seven weeks show an
engine-like subsystem accumulating state-machine patches around the same chain:

```text
prompt bar -> conductor route -> user v0 -> appagent/VText revision ->
worker request -> worker evidence -> VText wake -> later canonical version
```

At least 90 commits since 2026-04-11 mention VText/vtext, and 79 commits touch
the core VText files reviewed here. The repeated regression pattern is that the
system patches one edge of this chain without collapsing the chain into a
single explicit contract. Versions can stop advancing when any of these edges
silently becomes "pending", "deferred", "waiting for worker evidence",
"stale-head", or "new version available" without owner-visible recovery.

The highest-leverage conclusion is via negativa: delete the VText workflow
state machine instead of patching it again. VText should be a single-writer
revision loop driven by durable co-agent messages. Conductor routes and opens;
VText writes the first canonical appagent version; workers return durable
updates/evidence; VText wakes and writes later versions. The current code still
contains multiple control surfaces that encode workflow state indirectly:
conductor-created initial appagent revisions, prompt keyword classifiers,
`requires_worker_grounding`, required-next-tool outputs, pending mutation
idempotency, worker-message checkpoints, frontend autosave revisions, and
dirty-editor head-follow logic.

The next repair should remove these surfaces unless evidence proves one is
strictly necessary. The only complexity that should remain is the lifeblood of
the system: durable agent-to-agent communication, VText's single-writer
authority, and clear evidence of what messages each revision consumed.

## Reviewed Evidence

Primary current source paths:

- `internal/runtime/runtime.go`
- `internal/runtime/vtext.go`
- `internal/runtime/tools_vtext.go`
- `internal/runtime/vtext_controller.go`
- `internal/store/vtext.go`
- `internal/runtime/prompt_defaults/vtext.md`
- `frontend/src/lib/VTextEditor.svelte`
- `frontend/src/lib/vtext.js`

Historical docs and checkpoint artifacts:

- `docs/api-surface-and-vtext-workflow-review-2026-05-01.md`
- `docs/api-vtext-hard-cutover-checklist-2026-05-01.md`
- `docs/vtext-next-planning-checklist-2026-05-09.md`
- `docs/mission-vtext-live-cadence-repair-v3.md`
- historical, pruned docs read from git: `mission-vtext-durable-draft-version-graph-v0.md` and `review-search-vtext-context-2026-05-26.md`

Representative commits:

- `55ad169` - hard cut over e-text to VText with embedded Dolt.
- `f4b65ea` - harden VText workflow and runtime API.
- `d73afac` - make conductor own initial VText revision.
- `b2252fe` - rebase stale VText user drafts.
- `078aec5` - force VText worker continuations after early drafts.
- `8030807` - fix VText worker wake reconciliation race.
- `4cd61b2` - skip duplicate VText researcher spawns.
- `86489f1` - requeue blocked VText worker wakes.
- `408884a` - bound researcher checkpoint recovery.
- `4199176` - make VText publish links usable.
- `3547bef` - stabilize desktop and VText reload state.
- `0e4fbaa` - register media sources in VText revise.

## Regression Clusters

### 1. Canonical Writer Boundary Keeps Moving

The architecture says VText owns canonical document versions, but the current
runtime still creates a conductor-authored appagent seed when
`create_initial_version` is true. In `runtime.go`, conductor route materializer
creates user `v0`, then can create an appagent `v1` with
`source=initial_vtext_seed` and `AuthorLabel=conductor`.

Evidence:

- `internal/runtime/runtime.go:1528` computes `createInitialVersion`.
- `internal/runtime/runtime.go:1546` creates user `v0`.
- `internal/runtime/runtime.go:1573` creates the conductor appagent seed.
- `internal/runtime/runtime.go:1587` marks that seed as appagent-authored.

Why this regresses: downstream code and tests must remember whether the current
head is a user revision, conductor seed, or VText-authored revision. Several
continuation rules depend on `baseRevision.AuthorKind == user`; a conductor
seed changes that branch. A document can appear to have advanced to `v1` while
the real VText agent has not yet synthesized anything.

Deletion direction: remove conductor-authored appagent document text. Conductor
may create the document shell and user seed, but VText must write `v1` through
the same edit path used for later revisions.

### 2. Prompt Taxonomy Is Still Control Flow

The current system still classifies prompts with keyword lists and uses those
classifications to force tool order.

Evidence:

- `internal/runtime/vtext.go:1694` computes `requiresWorkerGrounding`.
- `internal/runtime/vtext.go:1719` persists it into run metadata.
- `internal/runtime/runtime.go:1690` selects initial VText tool choice.
- `internal/runtime/tools_vtext.go:205` computes research/super needs.
- `internal/runtime/tools_vtext.go:263` begins the marker list for current
  events, sports, citations, etc.

Why this regresses: every new product path adds another exception: creative
drafts, current sports, terminal commands, media sources, email drafts, source
ledgers, app/candidate work. Those exceptions landed as patches across May 24
through May 30. The system now behaves like a decision engine, but its state is
distributed across prompts, tool descriptions, metadata, and string markers.

Deletion direction: remove prompt taxonomy as control flow. VText can ask
co-agents for help in natural language over durable channels; runtime should not
need keyword classifiers to decide whether a document deserves a first version
or a worker.

### 3. Required Tool Continuations Are A Partial State Machine

VText tools can return `next_required_tool`, and the tool loop treats some VText
tools as terminal unless a required next tool is declared.

Evidence:

- `internal/runtime/runtime.go:1046` adds VText-specific tool loop behavior.
- `internal/runtime/runtime.go:1048` marks `edit_vtext`, `spawn_agent`,
  `request_super_execution`, and `request_email_draft` as terminal tools.
- `internal/runtime/tools_vtext.go:100` emits required continuations after an
  edit.
- `internal/runtime/tools_vtext.go:110` returns `next_required_tool`.

Why this regresses: this is safer than relying on final model text, but it is
still tool-result choreography rather than durable transition state. A model,
provider adapter, or tool-loop edge can satisfy the local rule while the durable
product chain remains incomplete. The May 25-27 fixes repeatedly patched this
boundary: terminal tool success, forced worker continuations, researcher
checkpoint recovery, duplicate spawns, blocked wake requeue.

Deletion direction: prefer ordinary durable wakeups from addressed co-agent
messages over hidden `next_required_tool` obligations. The product should
advance because messages exist and VText owns a revision loop, not because a
tool result smuggled the next state transition.

### 4. Pending Mutation Rows Can Become A Gate

The public revise handler refuses to start a new VText run if a pending mutation
exists for the document. It returns the existing run instead.

Evidence:

- `internal/runtime/vtext.go:1310` checks pending mutation.
- `internal/runtime/vtext.go:1313` returns the existing pending run.
- `internal/runtime/vtext.go:1395` only clears pending rows when the associated
  run can be loaded and is terminal.
- `internal/runtime/vtext.go:1400` returns the pending mutation unchanged if
  the run lookup fails.

Why this regresses: this is necessary idempotency, but it can also make "Revise"
look alive while no new version can be created. If the run is non-terminal but
stuck, or if the run record cannot be loaded, the document remains gated by the
pending mutation. The controller has the same gate: if an active VText run or
pending mutation exists, it reschedules rather than starting integration.

### 5. Worker Wake Depends On Exact Addressing And Checkpoints

Worker evidence only advances the document if a worker message is addressed to
`vtext:<docID>`, comes from an eligible role, and is newer than the integrated
checkpoint.

Evidence:

- `internal/runtime/vtext_controller.go:75` documents the invariant.
- `internal/runtime/vtext_controller.go:99` finds latest eligible worker
  message after checkpoint.
- `internal/runtime/vtext_controller.go:107` reschedules if a VText run is
  active.
- `internal/runtime/vtext_controller.go:113` reschedules if a pending mutation
  exists.
- `internal/runtime/tools_vtext.go:551` updates the integrated checkpoint only
  after a scheduled-message VText edit.

Why this regresses: a worker can do useful work but fail to create the exact
addressed channel message the controller recognizes. Or VText can consume one
message and checkpoint past adjacent work. Or a pending mutation can suppress
the wake. The May 20 and May 26 wake fixes show this edge has already failed in
multiple forms.

### 6. Stale Head Protection Is Correct But Not Product-Complete

The store rejects stale parent revisions, and `edit_vtext` also checks that
`base_revision_id` equals the current document head.

Evidence:

- `internal/store/vtext.go:539` reads the current head in a transaction.
- `internal/store/vtext.go:548` rejects a parent/head mismatch.
- `internal/runtime/tools_vtext.go:507` rejects stale `base_revision_id`.
- `internal/runtime/tools_vtext.go:542` fails the mutation if revision creation
  fails.

Why this regresses: stale-write rejection prevents data loss, but it does not
automatically advance the version graph. If the user autosaves or rebases while
a VText run is in flight, the VText edit can fail safely and still leave the
owner with "versions are not advancing." The May 25 durable-draft mission called
this out: stale writes need fail-safe retry, re-run, or explicit merge/rebase
behavior, not only a 409-style stop.

### 7. Frontend Autosave Creates Canonical Versions

The editor autosaves user drafts by creating real user revisions with
`allowRebase: true`.

Evidence:

- `frontend/src/lib/VTextEditor.svelte:785` starts autosave.
- `frontend/src/lib/VTextEditor.svelte:799` creates a user revision.
- `frontend/src/lib/VTextEditor.svelte:807` uses the current revision as parent.
- `frontend/src/lib/VTextEditor.svelte:850` only auto-advances to a new head
  when the editor is not dirty.
- `frontend/src/lib/VTextEditor.svelte:1182` saves the user version before
  calling revise.

Why this regresses: autosave as canonical revision is a pragmatic safety patch,
but it increases version churn and head movement. VText runs now race ordinary
typing more often. The UI also has a legitimate "New version available" state
that can make the backend version advance while the visible editor remains on
an older dirty draft.

### 8. Observability Still Mixes Product State And Runtime Internals

Several fixes improved Trace and stream behavior, but owner-visible state is
still assembled from run IDs, mutation state, SSE events, and editor state.
There is no single transition ledger that says: "revision N is blocked because
edge X failed."

Evidence:

- `frontend/src/lib/VTextEditor.svelte:1188` reads `response?.run_id`, while
  the backend response JSON field is `loop_id`; stream events later repair most
  visibility, but the direct response shape is inconsistent.
- `internal/runtime/vtext.go:615` sends stream snapshots with pending mutation.
- `frontend/src/lib/VTextEditor.svelte:892`/`head_changed` logic updates local
  state only after event projection.

Why this regresses: UI polish can make the surface look steady while the actual
transition is stuck at mutation, worker, checkpoint, stale head, or dirty-head
follow. This is why prior reports repeatedly required Trace, controller state,
worker messages, and revision metadata to classify failures.

## Timeline Pattern

April 11-15: document storage and UI were built, then e-text was renamed/hard-cut
to VText with embedded Dolt.

April 16-22: runtime loops, tool profiles, prompt routing, inbox delivery, and
controller reconciliation were repeatedly changed. Early failures were mostly
about making VText route through the runtime and browser harness at all.

May 1-4: API hardening moved toward prompt-bar product paths and event-log
verification, but also added a lot of verifier/tool scaffolding. The system
became more correct at the boundary while keeping several internal workflow
assumptions.

May 9-18: UX, autosave, Trace coexistence, publication flow, persistent super,
and live sync work expanded VText from an editor into an app/runtime substrate.
This introduced more concurrent state: browser drafts, canonical versions,
workers, publications, and desktop reload.

May 24-27: the code entered a heavy patch cycle around cadence: required tool
choice, terminal tool success, first-pass research, worker continuations,
temporal grounding, hard constraints, duplicate spawns, blocked wakes, and
researcher checkpoint recovery. This is the "v12 engine" phase: many patches
were locally reasonable, but the number of special cases is now itself a risk.

May 28-30: email and media-source integration added more VText obligations:
email draft handoff, approval hashes, source provenance, and media transcript
research. These changes increase the number of reasons a first revision must
trigger another actor before later versions can appear.

## Most Likely Current Causes Of "Versions Are Not Advancing"

1. A pending mutation is blocking new revise attempts. Check
   `vtext_agent_mutations` for the document: if state is `pending`, verify
   whether the run exists and is truly active. The current cleanup only marks it
   stale when the run loads and is terminal.

2. A VText run requested a worker and then deferred without a later eligible
   worker message. The run can be "successful" as delegation while the document
   remains at the same head. Check for addressed channel messages to
   `vtext:<docID>`.

3. Worker evidence exists but is not checkpoint-eligible. Wrong channel, wrong
   `ToAgentID`, non-eligible source profile, or checkpoint sequencing can all
   prevent the controller from starting the next VText integration run.

4. VText attempted `edit_vtext` against a stale base revision after user
   autosave or cross-device head movement. That is a safe failure, but there is
   no product-complete automatic rebase/retry path for appagent synthesis.

5. The visible editor is dirty and therefore refuses to auto-follow a new head.
   In that case the version may have advanced in backend history, but the owner
   sees "New version available" instead of the latest document content.

6. Prompt taxonomy routed the first move to worker opening rather than a visible
   draft. For factual/code/media/email prompts, this can be intended, but it
   makes owner-visible version advancement depend on the worker/control plane.

## Recommendation

Do not add another prompt rule as the primary fix. The next repair should first
produce a transition ledger for one failing document:

```text
doc_id
current_revision_id
latest user revision
latest appagent revision
pending/failed/deferred mutation rows
run state for each mutation
worker messages addressed to vtext:<docID>
controller checkpoint seq
latest eligible worker message seq
frontend dirty/new-version state if available
```

Then choose the smallest invariant-level repair. The likely durable direction is
to make VText transitions explicit records with typed states such as:

```text
draft_saved
revise_requested
vtext_started
worker_requested
waiting_for_worker
worker_update_received
integration_scheduled
edit_attempted
stale_head_retry_required
revision_created
visible_head_available
blocked
```

That would let the product answer "why did versions stop advancing?" without
reverse-engineering pending mutations, run state, channel messages, and editor
state after the fact.

## Concrete Review Checklist For The Next Debug Session

For the currently broken owner document, collect:

- document row and revision history;
- all `vtext_agent_mutations` rows for the document;
- run records for those mutation run IDs;
- channel messages for the document channel, especially messages addressed to
  `vtext:<docID>`;
- controller checkpoint row;
- Trace events for `vtext.agent_revision.started`, `progress`, `completed`,
  `failed`, `tool.result`, and worker `submit_coagent_update`;
- whether the editor is dirty or showing `New version available`.

Classify the failure as exactly one edge first:

- revise request did not create/run VText;
- VText did not call the required tool;
- worker did not produce eligible evidence;
- controller did not schedule integration;
- VText integration failed stale-head;
- revision exists but UI did not advance;
- live/deploy identity is stale relative to the expected fix.

Only after that classification should code be changed. The default fix posture
should be deletion: remove the old state-machine/control-flow surface first,
then add only the smallest durable message or VText revision invariant required
by the reproduced evidence.

## Residual Risk

This report is based on local source and git history, not a live staging
reproduction. It should be treated as a regression map and hypothesis register.
The immediate current failure still needs product-path evidence before a fix
commit, consistent with the problem-documentation-first contract.

## Deployed Mission Outcome, 2026-05-31

Follow-up mission `docs/mission-agentic-debugging-vtext-stability-v0.md`
reproduced the product path on staging before code changes and did not observe a
total backend version-advancement failure: the sampled documents reached v0 plus
two appagent `edit_vtext` revisions. The repair still proceeded because the
source retained the regression surface this review identified.

The accepted staging commit is `84c8c4f005db913cf47f5bc66e1bf55c10bfb224`.
CI run `26706511492` passed and Node B health reported both proxy and sandbox at
that commit. The deployed acceptance prompt created document
`1f74f922-106f-44da-a118-2528f56d48a2`; conductor did not create an appagent
first draft, VText wrote the first appagent revision, super returned durable
command evidence, and VText woke to write the next revision.

The repair direction was deletion-led: conductor-authored first drafts,
worker-grounding/classifier scaffolding, and required-tool choreography were
removed from the VText path. Trace is no longer a desktop app, but
machine-readable trace/evidence APIs remain. Raw Terminal is replaced by a
singleton Super Console backed by out-of-process zot. A staging proof showed
`/api/terminal/ws` returning 410, no Trace or Terminal launcher, and zot writing
`.choir/zot/sessions/zot-1/session.jsonl` plus `diagnosis.md` in the user
computer filesystem.
