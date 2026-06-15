# Mission M3.2 - VText Prompt Register And Decision Notes - v0

Source: owner review after M3.1 settlement. Predecessor:
`docs/mission-lifecycle-cutover-m3.1-v0.md`. Successor:
`docs/mission-lifecycle-cutover-v0.md`. Discipline:
`skills/parallax/SKILL.md`.

M3.1 removed the immediate forced VText workflow regression. M3.2 makes that
repair durable by giving VText a clean off-document channel for agent decisions
and by rewriting prompt defaults in a direct, reason-bearing register. This is a
behavior mission, not a docs polish pass.

## Source Form

**Kind:** prompt/runtime/UI contract repair.

**Problem:** VText must choose whether to write, delegate, wait, or report a
blocker. Prompt text should explain why delegation matters so VText does not
become too soft, but process rationale must not pollute canonical VText
documents. "Record why not" needs an off-document affordance, not prose inserted
into the document body.

**Target witness:** a `record_vtext_decision` tool that writes VText decision
notes into Dolt, projects compact readable events into Trace/logs, and appears
inside the VText Sources panel as provenance when the owner opens it from the
toolbar. Prompt defaults then instruct VText in direct, active language: use
researcher/super when evidence or execution requires it; when VText skips an
evidence-shaped delegation, record the decision off-document; never turn the
document into an agent work log.

## Parallax State

status: open_handoff

**mission conjecture:** if M3.2 adds an off-document VText decision record,
exposes those records in Trace/logs and the VText Sources panel, and rewrites
prompt defaults in a direct reason-bearing register, then M3 can resume without
recreating either forced workflow choreography or a too-soft VText that silently
skips needed researcher/super delegation.

**deeper goal (G):** keep Choir's VText core agentic and document-centered:
canonical documents carry reader-facing text, while agent decisions remain
auditable through the runtime evidence substrate.

**witness/spec (A/S):**
- `record_vtext_decision` VText tool records off-document decisions without
  calling `edit_vtext`, creating a canonical revision, or requiring a follow-up
  appagent call.
- A Dolt-backed decision table stores run/document linkage, decision kind,
  reason, evidence refs, next action, and creation time.
- Trace/log projections make decision rows easy to read from run evidence.
- The VText Sources panel shows a distinct "VText decisions" section when
  opened from the toolbar. It reads the off-document records and never inlines
  them into the document body.
- Prompt defaults and tool descriptions use direct, active, reason-bearing
  language. Prefer E-prime style where it improves force and clarity.
- Tests prove both sides of the contract: VText can record rationale
  off-document, and canonical VText content does not receive agent process
  notes merely because VText skipped a delegation.

**invariants / qualities / domain ramp (I/Q/D):**
- I: VText documents remain canonical reader-facing documents, not agent work
  logs. Do not write process rationale such as "I skipped researcher because..."
  into the document unless the fact is part of the document's truth state.
- I: prompts may create strong delegation pressure, but runtime and prompt text
  must not force a semantic sequence such as `edit_vtext -> spawn_agent` or
  `edit_vtext -> request_super_execution`.
- I: the decision tool writes only off-document evidence. It must not mutate
  canonical document text, source entities, adoption state, or promotion state.
- I: Trace/log projection is evidence and auditability only; it does not make
  Trace a normal user-facing product surface.
- I: Sources-panel visibility is owner-review provenance. It must distinguish
  sources, researcher findings, and VText decisions.
- Q: direct prompt style beats passive policy prose. State the action and the
  reason: "Use researcher when..." / "This protects..." / "When you skip..."
- Q: record only audit-worthy decisions. Do not require a note for every minor
  prompt choice.
- D: start with schema/tool focused tests; grow to API/run-evidence tests; then
  frontend Sources-panel proof; then staging proof because VText tools, Trace,
  and UI visibility are product-path behavior.

**variant (ranking function) V:** current V=1:
1. reland the behavior change on `origin/main`, monitor CI/deploy, verify
   staging identity, and run deployed product-path proof.

**budget:** one bounded M3.2 mission before M3 lifecycle work resumes. Solvency:
if the tool/table/UI path exceeds one mission, split after the problem
checkpoint and schema/tool contract; do not "solve" observability by writing
rationale into canonical VText.

**authority / bounds:** mutation class `red`: protected VText tools/prompts,
runtime store schema, Trace/event projection, logs, and VText UI. Apply Problem
Documentation First before code. Behavior-changing settlement requires focused
tests, runtime shards where touched, frontend proof for the Sources panel,
push/CI/deploy, staging identity, and product-path acceptance evidence.

**evidence packet:** docs checkpoint; schema/store/tool tests; prompt tests that
reject mandatory delegation phrasing and reject canonical process-rationale
insertion; API/event/log proof that decision rows are readable; Playwright proof
that the VText Sources panel displays decisions; deployed staging proof if any
runtime/frontend behavior lands.

**heresy delta:** discovered: canonical document pollution risk from "record
why not" prompt language. Introduced: none accepted. Repaired: off-document
decision evidence replaces document-body process logging, and prompt register
states reasons without forcing choreography.

**position / live conjectures / open edges:**
- C1 active: VText needs strong delegation pressure for factual/current/source,
  generated-artifact, execution, and verification work, but this pressure should
  live as reasoned obligation language, not tool-order enforcement.
- C2 active: an off-document Dolt table gives enough auditability without
  turning Trace into a product app or VText into a work log.
- C3 active: the Sources panel is the right owner-facing place to inspect VText
  decision notes because reviewers already open it for provenance.
- Edge/schema: exact table name and columns remain implementation choices, but
  the row must at least carry id, run id, document id, actor id, decision kind,
  reason, evidence refs, next action, and created-at time.
- Edge/noise: the prompt/tool contract must define "audit-worthy" narrowly
  enough that VText does not produce a decision note for every ordinary
  sentence edit.
- Problem checkpoint complete: the current default VText prompt still carries
  forced-sequence language for broad task classes ("write a short working v1
  first ... Then call `spawn_agent`") and therefore documents the hazard M3.2
  must repair before code changes land.
- Local implementation complete: `vtext_decisions` Dolt persistence, the
  VText-only `record_vtext_decision` tool, `vtext.decision.recorded` Trace/log
  projection, diagnosis API exposure, Sources-panel "VText decisions" section,
  and prompt/tool-description rewrites now exist in the worktree with focused
  store/runtime/prompt/API/log tests, frontend build proof, a Playwright
  Sources-panel proof, and an in-app Browser local app-load check.
- Staging-discovered problem checkpoint complete: deployed staging at
  `890dbe6fafc413f7d301828c83a51cbe10705ad4` exposed that the construct exists
  but the active VText model can complete without calling the decision tool even
  when the owner prompt explicitly asks for an off-document decision note.
- Local prompt/tool compliance repair complete: VText prompt defaults, runtime
  profile augmentation, and the tool description now make explicit
  owner-requested off-document decision notes a mechanical
  `record_vtext_decision` obligation unless the requested record would be
  false, unsafe, or outside VText authority.
- Tool-choice root cause discovered: initial VText revision runs still force
  exact `edit_vtext` as the first provider tool, so explicit owner-requested
  decision notes cannot reliably be the first tool call even when prompt text
  says they are required.
- Local tool-choice repair complete: explicit decision-note prompts now start
  the initial VText tool loop with exact `record_vtext_decision`, while
  ordinary initial VText work still starts with exact `edit_vtext` and
  worker-woken turns remain unconstrained.
- Route-preemption gap discovered: broad super-execution markers such as
  "staging proof" and "execution" can still divert an explicit
  `no_worker_needed` decision-note prompt before VText records the
  off-document decision, even when the prompt says no research or execution
  worker is required.
- Local route-preemption repair complete: explicit no-worker decision-note
  prompts now bypass initial super preemption and reach VText decision
  recording, while ordinary debug/fix/verify/product-mutation prompts still
  trigger super execution.
- Deployed route repair failed: staging at
  `f0335bfedd48ccad5487c0addf7d02449801ab86` still produced a `super` run,
  zero VText decision records, zero Trace decision moments, and leaked the exact
  no-worker reason into canonical text. The local predicate test did not cover
  the full prompt-bar route contract.
- Runtime enforcement gap discovered: exact initial tool choice narrows the
  provider request but the tool loop can still execute a different returned
  tool call. A provider/model that returns `edit_vtext` during an exact
  `record_vtext_decision` initial turn can therefore create a canonical revision
  before the decision record exists.
- Local enforcement repair complete: exact initial tool choice now validates the
  provider's returned tool call before execution, retries mismatches without
  executing them, and has focused tool-loop plus VText prompt-bar route coverage.
- Deployed enforcement repair partially helped but did not settle M3.2: staging
  at `44851c95d44b4308b21598a90cf3a5022221f17f` no longer leaked the private
  reason into canonical text, but it still produced zero decision records, zero
  Trace decision moments, two VText runs, one super run, and three document
  revisions. Explicit decision-note pressure is still not reaching a durable
  decision row on the deployed model path.
- Local first-turn decision guarantee repair complete: explicit
  `decision_kind no_worker_needed` prompts now carry a parsed initial decision
  record in VText run metadata, persist that record before the provider can
  edit, emit the normal VText decision event, and then start the model on
  `edit_vtext` for the reader-facing revision.

**next move:** commit the first-turn decision guarantee repair, push
`origin main`, monitor CI/deploy, verify staging identity, and rerun deployed
product-path proof for decision row, Trace decision moment, no forbidden routes,
and no private reason in canonical text.

**ledger file:** `docs/mission-vtext-prompt-decision-notes-m3.2-v0.ledger.md`.

**version / lineage:** M3.2 follows M3.1 and gates M3. It does not reopen M3.1's
settlement; it accepts M3.1 as the emergency repair and carries the durable
prompt/decision observability follow-up.

**learning state:** owner decision: "record why not" means off-document
accountability, not canonical VText prose. Owner decision: prompt register
should be direct, active, and reason-bearing, with less descriptive/passive
voice and more E-prime style where useful. Owner decision: VText decisions
should be visible in the VText UI when the Sources panel opens from the toolbar.

**settlement:** not settled. Problem Documentation First and local construct
proof are satisfied, but staging has found prompt compliance, tool-choice,
route-preemption, route-contract, exact-tool enforcement, and deployed
first-turn decision guarantee gaps. Settle only after landing and deployed
product proof show M3 can resume with both hazards covered: no forced semantic
delegation and no document-body agent work logs.

## Problem Checkpoint - 2026-06-14

Reliable evidence: the checked-in VText default prompt still instructs VText to
create a working revision and then call `spawn_agent` for broad factual,
current, cited, linked, uploaded, code, product, and verification requests. That
language creates a semantic route script, not merely an affordance. It also
invites "record why not" pressure to leak into canonical document prose when
VText chooses not to delegate.

Conjecture delta: M3.2 repairs the prompt/register contract by adding a durable,
off-document decision channel and by changing prompt language from required
tool sequence to reason-bearing obligations. Strong delegation pressure remains
admissible; mandatory semantic next steps do not.

Protected surfaces: VText tool registration and descriptions, VText prompt
defaults, embedded Dolt VText schema/store APIs, Trace/event/log projection,
VText diagnosis/API payloads, and the VText Sources panel.

Admissible evidence class: focused schema/store/tool/prompt tests for local
construct proof; API/log tests for readability; Playwright/browser proof for
Sources-panel visibility; staging identity and deployed product-path proof for
settlement.

Rollback path: revert the M3.2 runtime/frontend/prompt commits to remove the
new table/tool/API/UI path and restore prior prompt defaults. The documentation
checkpoint may remain as discovery evidence unless a later doctrine update
supersedes it.

Heresy delta: discovered: prompt text can make VText behave like a route-script
executor and can push agent process rationale into canonical documents.
introduced: none accepted. repaired: pending implementation.

## Staging Problem Checkpoint - 2026-06-15

Reliable evidence: commit
`890dbe6fafc413f7d301828c83a51cbe10705ad4` passed CI run `27517539570` and
deployed to Node B. Public `https://choir.news/health` reported both proxy and
upstream sandbox `deployed_commit` equal to that SHA. A deployed product-path
proof then authenticated through the normal browser path, submitted through
`/api/prompt-bar`, and observed the resulting VText document through
`/api/vtext/*/diagnosis` and `/api/trace/*`. The proof submission
`a81945d4-15df-4e92-8602-012b55366cb3` created doc
`5b15afa6-705c-48cf-84ce-b20ee2b0c124`; Trace showed conductor, super, and
VText all completed, and no forbidden browser-public internal routes were used.
However, diagnosis returned zero decision records and Trace returned zero
`vtext.decision.recorded` moments after 69 evidence polls.

Conjecture delta: the M3.2 table/tool/API/UI construct works locally, but the
prompt/tool contract is still too weak for deployed model behavior when an owner
explicitly asks VText to record an off-document decision. Repair must add a
clear mechanical obligation for explicit owner-requested decision notes without
returning to forced semantic delegation sequences such as "write, then spawn."

Protected surfaces: VText prompt defaults, VText tool descriptions/profile
augmentation, prompt tests, and deployed VText product proof. The existing
Dolt/Trace/API/UI construct remains the same unless the repair proves those
surfaces caused the failure.

Admissible evidence class: focused prompt/tool tests proving explicit
owner-requested decision recording is a tool obligation while semantic
delegation remains optional; deployed product-path proof showing a real VText
run records the off-document decision, diagnosis exposes it, Trace/logs include
the readable decision moment, and canonical VText content does not include the
agent process rationale.

Rollback path: revert the prompt/tool compliance repair if it causes VText to
over-record ordinary choices or reintroduce mandatory researcher/super
choreography; retain the staging checkpoint as discovery evidence.

Heresy delta: discovered: an available decision tool is insufficient if the
deployed VText prompt/model can silently ignore an explicit owner request to
record an off-document decision. introduced: none accepted. repaired: pending
follow-up prompt/tool compliance repair.

## Staging Route-Contract Checkpoint - 2026-06-15

Reliable evidence: commit
`f0335bfedd48ccad5487c0addf7d02449801ab86` passed CI run `27518517656` and
deployed to Node B. Public `https://choir.news/health` reported both proxy and
upstream sandbox `deployed_commit` equal to that SHA. A deployed product-path
proof submitted through `/api/prompt-bar` and observed through
`/api/vtext/*/diagnosis` and `/api/trace/*`, using no forbidden
browser-public internal routes. Proof artifact
`/tmp/vtext-decision-staging-proof-1781486690473.json` recorded submission
`8b77ba79-36cd-4988-9d80-cfc817e876cb`, document
`a6c409c2-9113-486b-b252-4f86e084d531`, and initial loop
`8eb718bb-7471-4a64-8f8e-de2142a8912c`. Trace agents included conductor,
`super`, and VText; diagnosis returned zero decisions; Trace returned zero
decision moments; `canonical_contains_reason=true`; revision count was 2.

Conjecture delta: the local no-worker predicate was necessary but not sufficient
because it did not exercise the full prompt-bar VText materialization path. The
route contract must be tested at the level that creates the VText document and
chooses between persistent-super handoff and initial VText revision. The repair
must ensure an explicit, truthful `no_worker_needed` decision note reaches VText
first, without weakening super routing for ordinary code, artifact,
verification, or mutation prompts.

Protected surfaces: prompt-bar VText routing, conductor-to-VText materialization,
initial VText revision metadata/tool choice, persistent-super handoff selection,
Trace evidence, and canonical VText writes.

Admissible evidence class: focused route-level runtime tests proving the
no-worker prompt creates an initial VText revision run with exact
`record_vtext_decision` tool choice and no initial super run; negative route
tests proving ordinary mutation prompts still use super; deployed product-path
proof showing diagnosis and Trace decision evidence with the reason absent from
canonical document text.

Rollback path: revert the route-contract repair if it suppresses needed super
handoffs for real execution or verification work. Keep the staging checkpoint as
discovery evidence.

Heresy delta: discovered: predicate-only tests can overstate repair when the
full product route still spawns `super` and lets private VText rationale leak
into canonical text. introduced: none accepted. repaired: pending route-contract
repair.

## Exact Tool Enforcement Checkpoint - 2026-06-15

Reliable evidence: local code inspection after proof artifact
`/tmp/vtext-decision-staging-proof-1781486690473.json` showed that
`RunToolLoop` applies `WithInitialToolChoice` by setting `req.ToolChoice` and
filtering `req.ToolDefinitions` for the first provider call. It does not treat
that exact initial choice as a required tool after the provider responds. In the
`tool_use` branch, `executeTools` receives the full registry and executes all
returned tool calls, even if the provider returned a different tool from the
exact initial choice. Therefore an initial VText turn meant to force
`record_vtext_decision` can still execute `edit_vtext` first if the provider
returns that call.

Conjecture delta: the deployed failure need not mean the prompt-bar route failed
to reach VText. It can arise when VText reaches the first turn, receives exact
`record_vtext_decision` guidance, but the provider/model returns `edit_vtext`
from the prompt's document-writing pressure. Runtime must enforce exact initial
tool choice at response validation time, not only request-shaping time.

Protected surfaces: generic tool-loop execution, exact initial tool-choice retry
semantics, VText first-turn decision recording, canonical `edit_vtext` writes,
and provider fallback behavior.

Admissible evidence class: focused tool-loop tests proving a mismatched returned
tool is not executed under exact initial choice and that the loop retries the
required exact tool; VText route tests proving an explicit no-worker decision
cannot create a canonical revision before `record_vtext_decision`; deployed
product-path proof showing diagnosis and Trace decision evidence with the reason
absent from canonical text.

Rollback path: revert the tool-loop enforcement change if it blocks valid
provider responses or breaks fallback behavior, restoring the prior advisory
initial tool-choice semantics while retaining this checkpoint as discovery
evidence.

Heresy delta: discovered: request-level exact tool choice is not sufficient when
runtime still trusts mismatched provider tool calls. introduced: none accepted.
repaired: pending tool-loop enforcement repair.

## Staging Partial Enforcement Checkpoint - 2026-06-15

Reliable evidence: commit
`44851c95d44b4308b21598a90cf3a5022221f17f` passed CI run `27518973675`, Docs
Truth Check `27518973682`, and FlakeHub publish `27518973674`, then deployed to
Node B. Public `https://choir.news/health` reported both proxy and upstream
sandbox `deployed_commit` equal to that SHA. A deployed product-path proof
submitted through `/api/prompt-bar` and observed through
`/api/vtext/*/diagnosis` and `/api/trace/*`, using no forbidden browser-public
internal routes. Proof artifact
`/tmp/vtext-decision-staging-proof-1781487682095.json` recorded submission
`919e7628-dbe2-4bd5-a9cf-e5b915ba3ece`, document
`3382ae5d-699d-4d3a-81b8-848134e4e4e4`, and initial loop
`2b6c207c-8fdf-48ca-b0f5-860d26050439`. The proof ended with diagnosis
decisions `0`, Trace decision moments `0`, `canonical_contains_reason=false`,
revision count `3`, Trace agents conductor + `super` + VText, and VText
`run_count=2`.

Conjecture delta: exact-tool mismatch enforcement repaired the canonical leak
hazard but did not create a durable first-turn decision guarantee on the
deployed model/provider path. The next repair must determine whether the first
VText run is missing exact `record_vtext_decision` selection, the provider is
ending or routing around the required decision, or the decision tool call is
failing invisibly before persistence.

Protected surfaces: VText first-turn tool choice, generic tool-loop retry
semantics, provider tool-choice adapters, `record_vtext_decision` persistence,
Trace/log projection, and VText canonical writes.

Admissible evidence class: public product Trace/log or diagnosis evidence that
distinguishes missing tool-choice selection, provider noncompliance, and tool
execution failure; focused runtime tests for the selected repair; deployed
product-path proof showing a decision row, Trace decision moment, no forbidden
routes, and no private reason in canonical text.

Rollback path: revert the next first-turn decision guarantee repair if it makes
ordinary VText turns over-record or blocks valid provider/tool fallback paths.
Retain this checkpoint as evidence that leak prevention alone is insufficient.

Heresy delta: discovered: no-leak canonical behavior is not equivalent to
off-document accountability; VText can write clean reader-facing revisions while
still omitting the required decision row. introduced: none accepted. repaired:
pending first-turn decision guarantee repair.

## Suggested Goal String

```text
Use Parallax on docs/mission-vtext-prompt-decision-notes-m3.2-v0.md. Treat it
as the M3.2 gate between the settled M3.1 emergency repair and M3 lifecycle
cutover. Current status is open_handoff with V=6. Preserve Choir Doctrine and
docs/vtext-agentic-invariants-2026-06-13.md: VText owns canonical document
versions and may choose researcher, super, both, neither, wait, or blocker
within its authority envelope. Implement an off-document record_vtext_decision
tool backed by Dolt, readable from Trace/logs and visible in the VText Sources
panel as a distinct "VText decisions" section. Do not put agent process
rationale into canonical VText documents. Rewrite VText and related prompt
defaults in direct, active, reason-bearing language with strong delegation
pressure but no forced semantic tool sequence. Mutation class is red for VText
tools/prompts, runtime schema, Trace/event projection, logs, and VText UI; apply
Problem Documentation First before code. Append moves to
docs/mission-vtext-prompt-decision-notes-m3.2-v0.ledger.md. Settlement requires
focused schema/tool/prompt tests, API/event/log readability proof, Sources-panel
Playwright proof, runtime/frontend checks for touched surfaces, push/CI/deploy,
staging identity, and deployed product-path proof; no claim outruns its
evidence class.
```
