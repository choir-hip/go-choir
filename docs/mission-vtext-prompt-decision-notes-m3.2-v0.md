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

**variant (ranking function) V:** current V=6:
1. problem checkpoint names the document/work-log hazard before code;
2. Dolt schema and store APIs for VText decision rows;
3. `record_vtext_decision` tool registered for VText with validation and tests;
4. Trace/log projection and API readability for the decision event;
5. VText Sources panel shows decisions separately from sources/researcher
   findings;
6. prompt defaults/tool descriptions rewritten and tested for direct,
   reason-bearing, non-forcing register.

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

**next move:** create the problem checkpoint commit, then implement the
schema/tool/API/UI/prompt batch with tests. Start by inspecting current VText
tool registration, runtime store migrations, event emission, `VTextSourcePanel`
data flow, and prompt default tests.

**ledger file:** `docs/mission-vtext-prompt-decision-notes-m3.2-v0.ledger.md`.

**version / lineage:** M3.2 follows M3.1 and gates M3. It does not reopen M3.1's
settlement; it accepts M3.1 as the emergency repair and carries the durable
prompt/decision observability follow-up.

**learning state:** owner decision: "record why not" means off-document
accountability, not canonical VText prose. Owner decision: prompt register
should be direct, active, and reason-bearing, with less descriptive/passive
voice and more E-prime style where useful. Owner decision: VText decisions
should be visible in the VText UI when the Sources panel opens from the toolbar.

**settlement:** not settled. Settle only when the witness exists with product
proof and M3 can resume with both hazards covered: no forced semantic
delegation and no document-body agent work logs.

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
