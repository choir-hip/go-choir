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

status: working

**mission conjecture:** if M3.2 adds an off-document VText decision record,
exposes those records in Trace/logs and the VText Sources panel, rewrites prompt
defaults in a direct reason-bearing register, and restores VText as Choir's
artifact control plane for ordinary prompt/source/article/mission ingress, then
M3 can resume without recreating either forced workflow choreography, direct
super ingress, or document-body agent work logs.

**deeper goal (G):** keep Choir's VText core agentic and artifact-centered:
canonical VText/artifact state coordinates agents, while execution authority and
agent decisions remain attached as evidence rather than replacing the artifact
control plane.

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
- Conductor materializes ordinary exogenous input as VText-owned artifact state:
  prompt-bar requests, sourcecycled/news ingestion, article creation, mission
  work, and most user prompts enter conductor -> VText before any super
  execution. `request_super_execution` remains available only as a VText
  affordance after VText inspects the artifact/request.

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
- I: VText is Choir's artifact control plane. Conductor may classify/open/create
  VText/context, but must not route ordinary prompt-bar, source/news, article,
  mission, or document/artifact work directly to super based on prompt text.
- I: Super before VText is a route invariant failure for ordinary VText-centered
  ingress. Super after VText is valid only when VText requested it through an
  explicit affordance such as `request_super_execution`.
- I: Downstream researcher/super work must attach back to the VText/artifact
  context as sources, findings, worker updates, decisions, or revisions.
- Q: direct prompt style beats passive policy prose. State the action and the
  reason: "Use researcher when..." / "This protects..." / "When you skip..."
- Q: record only audit-worthy decisions. Do not require a note for every minor
  prompt choice.
- D: start with schema/tool focused tests; grow to API/run-evidence tests; then
  frontend Sources-panel proof; then staging proof because VText tools, Trace,
  and UI visibility are product-path behavior.

**variant (ranking function) V:** current V=2:
1. prove on staging that prompt-bar VText materialization keeps
   instruction/control text, including explicit off-document decision
   rationale, out of canonical reader-facing document text while preserving the
   full request as VText instruction/context.
2. prove an ordinary execution-shaped prompt can still cause VText to request
   downstream super execution after VText owns the artifact context, or record
   the precise blocker if the deployed VText/model path cannot make that
   request within the acceptance budget.
Then rerun deployed acceptance so prompt-bar VText ingress starts with VText,
super before VText fails, super after VText is allowed only when VText
requested it, explicit owner-requested decision notes create `vtext_decisions`
rows plus Trace/log projections without leaking into canonical text, and
source/news/article product paths attach downstream work back to VText/artifact
context where product-path proof is available.

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
why not" prompt language and prompt-bar materialization that stores control
prompts as canonical VText text. Introduced: none accepted. Repaired:
off-document decision evidence replaces document-body process logging, prompt
register states reasons without forcing choreography, and deployed direct-super
ingress has been repaired for fresh prompt-bar VText submissions.

**position / live conjectures / open edges:**
- C0 active: VText is Choir's artifact control plane. Conductor routes
  exogenous input into VText-owned artifact state; super is downstream execution
  authority that VText may request later.
- C0a supported for prompt-bar route order: deployed browser-public proof on
  `3a5cbb41fd05b0eb3acf50c7ae930cbfc2108d1f` returned conductor as Trace
  entry and a VText `initial_loop_id` for fresh authenticated prompt-bar VText
  submissions. No super-before-VText run appeared in the proof.
- C0b active: prompt-bar VText materialization still creates the initial
  user-authored canonical revision from the full control prompt. The explicit
  off-document decision note row now exists, but the exact decision rationale
  appears in canonical document text because it was part of that initial
  revision.
- C0c active: deployed execution-shaped prompt proof started with VText but did
  not produce a downstream VText-requested super run within the acceptance
  window; determine whether this is prompt/tool pressure, provider latency, or a
  missing VText affordance path.
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
- Deployed first-turn decision guarantee failed: staging at
  `f244c5446f387ca0df9ef0ebed2188b75de38d17` still produced zero decision
  records and zero Trace decision moments for the explicit no-worker decision
  prompt, while the canonical reason leak remained fixed. The local metadata
  guarantee is not yet reaching the deployed product path.
- Local pre-activation decision repair complete: explicit no-worker VText
  decision metadata is now recorded synchronously before the root or child VText
  run activation starts, with the existing tool-loop hook left as an idempotent
  fallback. Prompt-bar route coverage now waits for the VText child run and
  asserts the durable decision row.
- Deployed pre-activation repair failed: staging at
  `916885ce5fde61a146a8317353ac6b2096cee4e6` still produced zero decision
  records and zero Trace decision moments for the explicit no-worker decision
  prompt. Trace again showed a `super` run before the VText run, while the
  canonical private-reason leak stayed repaired. The deployed route is still
  bypassing the deterministic decision-record path.
- Public Trace diagnostic identified the route gap: deployed diagnostic artifact
  `/tmp/vtext-route-diagnostic-1781489784153.json` showed
  `initial_loop_id=0e66ef35-accd-4113-874f-3d3451d8fb47` was the `super` run,
  and the VText run was spawned from that super run. The failure is initial
  persistent-super preemption, not a later VText recording failure.
- Local prompt-bar no-worker route repair complete: prompt-bar now stamps an
  explicit no-worker decision route flag for prompts containing
  `no_worker_needed` or "no research or execution worker", and conductor VText
  materialization honors that flag before persistent-super preemption. Negative
  coverage still proves operational proof prompts use persistent super.
- Deployed prompt-bar no-worker route repair still failed: staging at
  `6be05f87043553e07cebd56940c3d004deaeaebd` produced zero decision records
  and zero Trace decision moments, with no private-reason canonical leak and no
  forbidden routes. Unlike the prior proof, VText had two runs and the document
  had three revisions, so the route likely reaches initial VText but still
  misses the deterministic decision-record metadata on at least one VText run.
- Follow-up public Trace diagnostic still showed initial persistent-super
  preemption: `/tmp/vtext-route-diagnostic-1781490432255.json` on deployed
  `6be05f87043553e07cebd56940c3d004deaeaebd` showed
  `initial_loop_id=a5028aa1-9cfb-46db-88ed-f9d6f2b9e9f9` was the `super` run,
  with VText spawned from that run. The `execution worker` phrase must be
  excluded inside the super-execution detector itself for explicit no-worker
  decision prompts.
- Local detector-level no-worker repair complete: `vtextPromptNeedsSuperExecution`
  now returns false for explicit prompt-bar no-worker decision routes before
  scanning super-execution markers, while operational proof prompts still route
  to persistent super.
- Deployed detector-level no-worker repair failed: staging at
  `9c11fab05c5d5f24e9d869a721a25a1455ce63b5` still produced zero decision
  records and zero Trace decision moments for the explicit no-worker decision
  prompt. The canonical private-reason leak stayed repaired, but public Trace
  agent summary still showed `super` before VText. The remaining gap is still a
  deployed super-first route path, not document-body pollution.
- Public Trace route diagnostic on deployed `9c11fab05c5d5f24e9d869a721a25a1455ce63b5`
  clarified the path: conductor creates the VText document, then sends the
  initial assignment to `super`; `super` later wakes VText, and VText edits the
  document without any decision row. The bypass happens before VText owns the
  initial route, not inside VText's final edit loop.
- Local route-carrier repair complete: conductor VText routing now derives the
  no-worker decision route from every durable prompt carrier available at the
  route boundary, stamps the no-worker route flag before any super branch, and
  preserves that prompt as the seed for deterministic VText decision metadata.
  Focused route/tool-loop/decision tests pass.
- Deployed route-carrier repair failed: staging at
  `081a411e88a8d81fb35f62f59c6eecae2baf22e6` still returned a `super`
  initial loop for the explicit no-worker prompt. The proof saw one Trace
  moment that mentioned `no_worker_needed`, but that moment was a
  `channel.message` from `super` to VText, not a durable
  `vtext.decision.recorded` row. VText diagnosis still returned zero decision
  rows, while the canonical private-reason leak stayed repaired.
- Local super-request choke-point repair complete: if a prompt-bar conductor
  no-worker route still reaches `requestPersistentSuperExecution`, the runtime
  now redirects it to a VText initial revision run instead of dispatching a
  persistent-super update. The redirected VText run records the deterministic
  `no_worker_needed` decision before provider execution.
- Deployed super-request choke-point repair failed: staging at
  `80883c5f34add2de0a77e1e5a193e314a6ca602d` still returned a `super` initial
  loop for the explicit no-worker prompt. VText diagnosis returned zero
  decision rows and Trace returned zero decision moments. The canonical
  private-reason leak stayed repaired, so the remaining failure is still
  route/decision persistence rather than document pollution.
- Public Trace diagnostic on deployed `80883c5f34add2de0a77e1e5a193e314a6ca602d`
  showed the first super assignment as a `channel.message` from the conductor
  loop to `super`, labeled with VText requester identity. The redirect hook
  exists on the right function, but its stored-conductor metadata guard is too
  strict for the deployed prompt-bar run shape.
- Local redirect-predicate repair complete: the super-request redirect now
  relies on owner, conductor profile, existing VText document channel, and
  durable no-worker prompt text. It no longer requires stored prompt-bar app
  metadata fields that the deployed conductor route may not carry.
- Deployed redirect-predicate repair still failed M3.2: staging at
  `025fe3020f597637a302c272004b0c8719c7f7a2` produced zero decision rows and
  zero Trace decision moments. Follow-up public diagnosis artifact
  `/tmp/vtext-decision-full-diagnostic-1781493917187.json` showed the stored
  conductor route still had `initial_handoff=persistent_super`; the later
  VText run was a scheduled worker-integration turn with no deterministic
  decision metadata. The private no-worker reason appeared in the user prompt
  revision and was absent from the final appagent revision, but the off-document
  decision row never existed.
- Local structured route-predicate repair complete: conductor VText routing
  and the super-request redirect now also derive the no-worker route from the
  structured explicit decision parser used for deterministic decision metadata.
  Stored-conductor route coverage now waits for completion and asserts the
  durable `no_worker_needed` decision row, not only the initial loop profile.
- Deployed structured route-predicate repair failed: staging at
  `3dfee389c5f4105466742b8d9f0576662d55c2ae` still persisted
  `initial_handoff=persistent_super` for the explicit no-worker prompt.
  Public diagnosis artifact
  `/tmp/vtext-decision-full-diagnostic-1781494768657.json` showed the VText
  run remained a scheduled `integrate_worker_findings` turn with
  `scheduled_message_seq=2` and no deterministic decision metadata.
- Local prompt-bar boundary stamping repair complete: completed prompt-bar
  conductor runs now stamp `prompt_bar_no_worker_decision_route` when the
  prompt carries the structured `decision_kind no_worker...` marker, even if
  the caller did not pre-stamp metadata. Stored-route coverage now asserts the
  flag exists before materialization and still proves the decision row after
  VText completion.
- Deployed prompt-bar boundary stamping repair failed: staging at
  `97852b155b7896f4af101cf3103dead3fb78c9a1` still showed no
  `prompt_bar_no_worker_decision_route` in public conductor metadata and still
  persisted `initial_handoff=persistent_super`. This suggests the live
  `/api/prompt-bar` path may not be exercising the sandbox runtime boundary
  modified by the local repairs, or another route layer rewrites the completed
  conductor metadata before materialization.
- Owner clarification supersedes the no-worker route-repair frame: the failure
  is not a missing no-worker predicate. The violated core invariant is that
  Choir is VText-centered artifact state, not prompt routing to agents.
  Conductor must route ordinary prompt/source/article/mission ingress into
  VText-owned artifact context first. Super is never the direct ingress target
  for ordinary user/source prompts; VText may call `request_super_execution`
  later when the artifact needs execution authority.
- The prior `prompt_bar_no_worker_decision_route` work is retained only as
  staging evidence about how overfit predicate patches fail. It must be removed
  or quarantined before settlement and must not become architecture.
- Local VText control-plane repair complete: prompt-bar and completed-conductor
  materialization no longer stamp no-worker route flags; conductor VText routing
  no longer has a persistent-super preemption branch; `request_super_execution`
  no longer redirects conductor no-worker requests back to VText; Universal Wire
  source/article handoff no longer eagerly persists super. Focused tests prove
  prompt-bar execution-shaped prompts and processor article routes start with
  VText, and VText can still request persistent super afterward.
- Deployed VText route-order repair complete: after the vmctl runtime package
  pointer repair, staging at
  `3a5cbb41fd05b0eb3acf50c7ae930cbfc2108d1f` shows prompt-bar VText
  submissions starting with VText, not super.
- Deployed canonical-control-text problem discovered: the same proof created a
  durable `no_worker_needed` decision row and Trace/log evidence, but the exact
  decision rationale still appeared in canonical VText text because the
  prompt-bar seed revision used the full prompt as document content.
- Deployed downstream-super proof remains open: an execution-shaped prompt
  started with VText and no super-before-VText violation, but the VText run did
  not call `request_super_execution` within the 240 second proof window.
- Local prompt-bar intake repair complete: prompt-bar conductor materialization
  now creates an intentionally blank input revision, stores the full user
  request in `seed_prompt` metadata, marks the revision as a prompt-bar
  instruction revision, and tells VText to use the original request as
  instruction/context rather than existing canonical prose. Focused tests prove
  the explicit decision row still exists while the seed revision and canonical
  head do not contain the private decision reason.
- Deployed prompt-bar route/canonical/decision acceptance supported:
  `39273a164ce08d6567bc5e05a04099a1167acdca` passed CI/deploy, `/health`
  reported that SHA for proxy and sandbox, and deployed proof artifact
  `/tmp/vtext-control-plane-staging-proof-1781499079736.json` showed conductor
  entry, VText initial loop, decision row
  `37589f30-d126-4f14-99e5-bfa1485b10ac`, no super runs, no forbidden
  browser-public internal routes, and no canonical decision-rationale leak.
- Deployed downstream-super leg still failed: the execution-shaped proof
  submission started conductor -> VText with initial VText loop
  `227288c3-d159-466f-8288-3040b934661b`, but the VText run remained `running`
  and did not call `request_super_execution` within the 240 second proof window.

**next move:** commit this deployed partial-acceptance checkpoint, then
discriminate the downstream-super miss with an extended proof window or focused
VText prompt/tool evidence. Do not add conductor-level prompt heuristics or
direct super ingress; any repair must keep super downstream of a VText
`request_super_execution` decision.

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
proof are satisfied for the decision-note artifact, but the route repair frame
was too narrow. Settle only after landing and deployed product proof show M3 can
resume with all three hazards covered: no forced semantic delegation, no
document-body agent work logs, and no direct-super ingress that bypasses
VText-owned artifact state.

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

## Staging Metadata Guarantee Checkpoint - 2026-06-15

Reliable evidence: commit
`f244c5446f387ca0df9ef0ebed2188b75de38d17` passed CI run `27519472366`, Docs
Truth Check `27519472379`, and FlakeHub publish `27519472396`, then deployed to
Node B. Public `https://choir.news/health` reported both proxy and upstream
sandbox `deployed_commit` equal to that SHA. A deployed product-path proof
submitted through `/api/prompt-bar` and observed through
`/api/vtext/*/diagnosis` and `/api/trace/*`, using no forbidden browser-public
internal routes. Proof artifact
`/tmp/vtext-decision-staging-proof-1781488672918.json` recorded submission
`72ef2f03-b3d5-4157-9166-52b378443e80`, document
`f0740135-8059-403e-a6b3-6c9c4c003883`, and initial loop
`2c399a26-844b-4207-8d82-2b765c2fe401`. The proof ended with diagnosis
decisions `0`, Trace decision moments `0`, `canonical_contains_reason=false`,
revision count `2`, and forbidden internal routes `[]`.

Conjecture delta: deterministic local metadata recording did not reach the
deployed prompt-bar-to-VText execution path. The likely fault is now at the
route metadata boundary, child-run metadata persistence, or a mismatch between
the deployed proof prompt and the parser that marks an initial no-worker
decision as required.

Protected surfaces: prompt-bar materialization, VText child-run constraints and
metadata, VText first-turn decision recording, Trace decision projection, and
canonical VText writes.

Admissible evidence class: focused runtime tests that exercise the same
prompt-bar/child-run boundary as staging and prove the explicit no-worker
decision metadata reaches `executeWithToolLoop` before provider execution;
deployed product-path proof showing diagnosis and Trace decision evidence with
the private reason absent from canonical text.

Rollback path: revert the next route/metadata repair if it records false
decision notes, records ordinary VText edits too aggressively, or weakens
super routing for real execution work. Keep this checkpoint as evidence that
canonical leak prevention alone still does not settle M3.2.

Heresy delta: discovered: local first-turn guarantees can be bypassed by a
deployed route/metadata boundary even when code-level VText tests pass.
introduced: none accepted. repaired: pending route/metadata repair.

## Staging Pre-Activation Repair Checkpoint - 2026-06-15

Reliable evidence: commit
`916885ce5fde61a146a8317353ac6b2096cee4e6` passed CI run `27519880134`, whose
Node B staging deploy job completed successfully. Public
`https://choir.news/health` reported both proxy and upstream sandbox
`deployed_commit` equal to that SHA. A deployed product-path proof submitted
through `/api/prompt-bar` and observed through `/api/vtext/*/diagnosis` and
`/api/trace/*`, using no forbidden browser-public internal routes. Proof
artifact `/tmp/vtext-decision-staging-proof-1781489484677.json` recorded
submission `68ff0f37-9b14-47ad-a711-ad5ebf0be660`, document
`f6caec7a-a975-4b38-8a17-6b4804e8a9ec`, and initial loop
`efec393f-bc03-4cc9-871a-2b5caa14d3c9`. The proof ended with diagnosis
decisions `0`, Trace decision moments `0`, `canonical_contains_reason=false`,
revision count `2`, and forbidden internal routes `[]`. Trace agents again
included conductor, `super`, and VText, with `super` observed before VText.

Conjecture delta: moving deterministic decision recording before VText
activation did not repair the deployed route because the route still appears to
preempt or branch through `super` before the VText no-worker decision record
exists. The next repair must distinguish whether `initial_loop_id` is still a
super handoff, whether the initial VText run asks super before recording, or
whether the public proof is following a later VText run while the true initial
route lives elsewhere.

Protected surfaces: prompt-bar initial handoff selection, persistent-super
preemption predicates, VText no-worker decision detection, public Trace
projection, and canonical VText writes.

Admissible evidence class: product-route coverage or deployed public Trace
evidence showing which run id corresponds to the initial loop and why `super`
appears before VText; focused runtime tests for that route; deployed
product-path proof showing a decision row, Trace decision moment, no forbidden
routes, and no private reason in canonical text.

Rollback path: revert the next route repair if it suppresses required super
handoffs for execution, verification, code, or artifact work. Keep this
checkpoint as evidence that moving the record earlier is still insufficient
while super preemption remains possible.

Heresy delta: discovered: the deployed route can still pass through `super`
before off-document VText accountability exists, despite local pre-activation
recording coverage. introduced: none accepted. repaired: pending route
preemption repair.

## Staging Prompt-Bar Route Repair Checkpoint - 2026-06-15

Reliable evidence: commit
`6be05f87043553e07cebd56940c3d004deaeaebd` passed CI run `27520207638`, Docs
Truth Check `27520207623`, and FlakeHub publish `27520207634`, then deployed to
Node B. Public `https://choir.news/health` reported both proxy and upstream
sandbox `deployed_commit` equal to that SHA. A deployed product-path proof
submitted through `/api/prompt-bar` and observed through
`/api/vtext/*/diagnosis` and `/api/trace/*`, using no forbidden browser-public
internal routes. Proof artifact
`/tmp/vtext-decision-staging-proof-1781490150274.json` recorded submission
`0f1b0472-a833-4370-9862-b268d93b6fd9`, document
`5bbaccb7-fca4-45b9-94e7-f67379dee590`, and initial loop
`2cdcc3f6-00d4-4da0-9574-3c2f2e21f4aa`. The proof ended with diagnosis
decisions `0`, Trace decision moments `0`, `canonical_contains_reason=false`,
revision count `3`, and forbidden internal routes `[]`. Trace agents included
conductor, `super`, and VText; VText had run count `2`.

Conjecture delta: prompt-bar no-worker route flagging appears to have changed
the route shape but did not create the required off-document decision row. The
next repair must inspect the two VText runs and identify whether the initial
VText run lacks explicit decision metadata, records against the wrong document
or trajectory, or is bypassed by a later super-spawned VText run.

Protected surfaces: prompt-bar route metadata, VText run metadata propagation,
pre-activation decision recording, Trace decision projection, and canonical
VText writes.

Admissible evidence class: public Trace evidence that identifies both VText
runs and the initial-loop agent; focused runtime tests for the selected
metadata/recording repair; deployed product-path proof showing a decision row,
Trace decision moment, no forbidden routes, and no private reason in canonical
text.

Rollback path: revert the next metadata or recording repair if it records false
decision notes, duplicates decision records for ordinary VText runs, or weakens
required super handoffs. Keep this checkpoint as evidence that route bypass
alone did not settle M3.2.

Heresy delta: discovered: bypassing initial super preemption is not sufficient
if VText run metadata still does not drive a durable decision row. introduced:
none accepted. repaired: pending metadata/recording repair.

## Staging Redirect-Predicate Repair Checkpoint - 2026-06-15

Reliable evidence: commit
`025fe3020f597637a302c272004b0c8719c7f7a2` passed CI run `27521818228`, Docs
Truth Check `27521818222`, and FlakeHub publish `27521818242`, including Node
B staging deploy. Public `https://choir.news/health` reported both proxy and
upstream sandbox `deployed_commit` equal to that SHA. A deployed product-path
proof submitted through `/api/prompt-bar` and observed through
`/api/vtext/*/diagnosis` and `/api/trace/*`, using no forbidden
browser-public internal routes. Proof artifact
`/tmp/vtext-decision-staging-proof-1781493489068.json` and screenshot
`/tmp/vtext-decision-staging-proof-1781493489068.png` recorded submission
`f6dcce66-40dc-44d7-9e5a-4392cb2f3967`, document
`b44d2c31-8348-410c-bd99-517a52bbc933`, and initial loop
`3e411e52-cdc3-4ce8-b992-10cc9b054e2a`. Trace agents were conductor, `super`,
and VText. The proof ended with diagnosis decisions `0`, Trace decision moments `0`,
`canonical_contains_reason=false`, revision count `2`, and forbidden internal
routes `[]`. Evidence samples showed `canonical_contains_reason=true` for the
first revision and `false` after the final revision, so the private reason was
transiently written before being removed.

Follow-up public diagnosis artifact
`/tmp/vtext-decision-full-diagnostic-1781493917187.json` on the same deployed
commit recorded submission `cf29eb8b-f9f5-45a4-afef-f2abb4ad71bd`, document
`bc7479ab-2094-4b22-8057-f8f1fa178fc2`, and initial loop
`fbb2876b-9b01-4a01-9055-a8a58094179d`. Public run metadata showed the
conductor route persisted `initial_handoff=persistent_super`; the VText run had
`parent_id=fbb2876b-9b01-4a01-9055-a8a58094179d`, `scheduled_message_seq=2`,
`request_intent=integrate_worker_findings`, and no
`vtext_initial_decision_required` metadata. This corrects the initial reading:
the deployed failure remains a persistent-super route bypass, not a
VText-initial decision-persistence failure.

Conjecture delta: relaxing the redirect predicate did not fix the deployed
super-first route. The next repair must identify why the no-worker route flag
or prompt-derived predicate is absent at the conductor route branch even though
the stored seed prompt contains `decision_kind no_worker_needed` and "no
research or execution worker."

Protected surfaces: prompt-bar route metadata, conductor VText handoff
selection, persistent-super fallback routing, pre-activation decision
recording, Trace decision projection, diagnosis decision exposure, and
canonical VText revision creation.

Admissible evidence class: focused runtime tests reproducing the deployed
stored-conductor route shape and proving explicit no-worker prompts bypass
persistent super and create a decision row before any appagent edit; deployed
product-path proof showing one matching diagnosis decision, one matching Trace
decision moment, no forbidden routes, and no private reason in the final
canonical revision.

Rollback path: revert the next decision-persistence repair if it records false
or duplicate decision notes, blocks ordinary VText edits, or suppresses required
super routing for real execution work. Keep this checkpoint as evidence that
final canonical cleanliness does not settle M3.2 when transient pre-decision
pollution and missing durable decision rows remain.

Heresy delta: discovered: the deployed prompt-bar route can still persist
`initial_handoff=persistent_super` for explicit no-worker VText prompts even
after local route and redirect predicates pass. introduced: none accepted.
repaired: pending route-predicate repair.

## Staging Structured Route-Predicate Repair Checkpoint - 2026-06-15

Reliable evidence: commit
`3dfee389c5f4105466742b8d9f0576662d55c2ae` passed CI run `27522338867`, Docs
Truth Check `27522338874`, and FlakeHub publish `27522338863`, including Node
B staging deploy. Public `https://choir.news/health` reported both proxy and
upstream sandbox `deployed_commit` equal to that SHA after a short post-deploy
settling window. A deployed product-path proof submitted through
`/api/prompt-bar` and observed through `/api/vtext/*/diagnosis` and
`/api/trace/*`, using no forbidden browser-public internal routes. Proof
artifact `/tmp/vtext-decision-staging-proof-1781494549750.json` and screenshot
`/tmp/vtext-decision-staging-proof-1781494549750.png` recorded submission
`9cfeef9a-221f-4b05-8b19-dbac1fd3b6ce`, document
`1a8edec4-2ecd-4c71-acf3-bd77b59605f6`, and initial loop
`e02d066c-80a9-41ce-9aa8-cdc2848f55de`. The proof ended with diagnosis
decisions `0`, Trace decision moments `0`, `canonical_contains_reason=false`,
revision count `2`, and forbidden internal routes `[]`.

Follow-up public diagnosis artifact
`/tmp/vtext-decision-full-diagnostic-1781494768657.json` recorded submission
`23bd398c-ed82-4e41-a193-928ac64de512`, document
`455c0b47-c47d-4cb2-aaf6-3ffa34c6e793`, and initial loop
`ca8f79b8-48a4-4f4d-b71f-5cb56be8792f`. Public run metadata still showed
conductor `initial_handoff=persistent_super`; the VText run had
`parent_id=ca8f79b8-48a4-4f4d-b71f-5cb56be8792f`,
`scheduled_message_seq=2`, `request_intent=integrate_worker_findings`, and no
`vtext_initial_decision_required` metadata.

Conjecture delta: deriving the no-worker route from the structured parser at
the conductor and redirect branch did not affect the live prompt-bar route.
The next repair must stamp or persist the no-worker route at the prompt-bar API
boundary before completed conductor materialization, using the deployed prompt
shape itself as the source of truth.

Protected surfaces: prompt-bar API run metadata, conductor route
materialization, persistent-super fallback routing, VText decision recording,
Trace decision projection, diagnosis decision exposure, and canonical VText
revision creation.

Admissible evidence class: focused API route tests proving a prompt-bar
submission with `decision_kind no_worker_needed` stores the no-worker route
flag before conductor materialization and creates a durable decision row;
deployed product-path proof showing one matching diagnosis decision, one
matching Trace decision moment, no forbidden routes, and no private reason in
the final canonical revision.

Rollback path: revert the next prompt-bar stamping repair if it sends ordinary
execution or operational proof requests away from persistent super. Keep this
checkpoint as evidence that local downstream route predicates alone did not
change the live prompt-bar boundary.

Heresy delta: discovered: the deployed prompt-bar API boundary can still
materialize a completed conductor route without the no-worker route flag even
when downstream conductor and redirect predicates parse the same prompt shape.
introduced: none accepted. repaired: pending API-boundary stamping repair.

## Staging Prompt-Bar Boundary Stamping Checkpoint - 2026-06-15

Reliable evidence: commit
`97852b155b7896f4af101cf3103dead3fb78c9a1` passed CI run `27522658503`, Docs
Truth Check `27522658505`, and FlakeHub publish `27522658518`, including Node
B staging deploy. Public `https://choir.news/health` reported both proxy and
upstream sandbox `deployed_commit` equal to that SHA. A deployed product-path
proof submitted through `/api/prompt-bar` and observed through
`/api/vtext/*/diagnosis` and `/api/trace/*`, using no forbidden
browser-public internal routes. Proof artifact
`/tmp/vtext-decision-staging-proof-1781495174835.json` and screenshot
`/tmp/vtext-decision-staging-proof-1781495174835.png` recorded submission
`f5719caa-246d-498d-a717-0e1667030fae`, document
`a7480eed-574c-482a-af85-f306778e5ccd`, and initial loop
`4fbf3dde-240b-40b9-984b-bd8220472bee`. The proof ended with diagnosis
decisions `0`, Trace decision moments `0`, `canonical_contains_reason=false`,
revision count `2`, and forbidden internal routes `[]`.

Follow-up public diagnosis artifact
`/tmp/vtext-decision-full-diagnostic-1781495392699.json` recorded submission
`44d86ec7-ab18-4b03-90ab-24de08d86234`, document
`9de6a1d0-5233-4c36-971d-2054fb8f2dcf`, and initial loop
`8a58455f-fef5-4a2c-82e5-74ccaf0637e6`. Public conductor metadata still lacked
`prompt_bar_no_worker_decision_route` and still persisted
`initial_handoff=persistent_super`; the VText run had
`parent_id=8a58455f-fef5-4a2c-82e5-74ccaf0637e6`,
`scheduled_message_seq=2`, `request_intent=integrate_worker_findings`, and no
`vtext_initial_decision_required` metadata.

Conjecture delta: stamping inside `completePromptBarDecisionRun` did not
change live staging behavior. The failure is now likely outside the local
boundary assumed by the repair: either `/api/prompt-bar` reaches a different
implementation path than the sandbox runtime function just changed, or a later
route/materialization layer rewrites the conductor metadata/result before the
VText route branch observes the flag.

Protected surfaces: prompt-bar proxy/upstream routing, sandbox prompt-bar API
handler, completed conductor metadata persistence, conductor materialization,
persistent-super fallback routing, VText decision persistence, Trace decision
projection, and canonical VText revision creation.

Admissible evidence class: route-boundary evidence showing which process and
handler materializes `/api/prompt-bar` on staging; focused tests for that
actual boundary; deployed product-path proof showing one matching diagnosis
decision, one matching Trace decision moment, no forbidden routes, and no
private reason in the final canonical revision.

Rollback path: revert the next boundary repair if it reroutes ordinary
execution/operational proof prompts away from persistent super or records false
decision notes. Keep this checkpoint as evidence that sandbox-local
`completePromptBarDecisionRun` stamping alone did not affect staging.

Heresy delta: discovered: staging can report the expected sandbox commit while
the observable prompt-bar route still omits metadata written by the patched
sandbox function. introduced: none accepted. repaired: pending live-boundary
repair.

## VText Control-Plane Ingress Checkpoint - 2026-06-15

Reliable evidence: repeated deployed product-path proofs from
`890dbe6fafc413f7d301828c83a51cbe10705ad4` through
`97852b155b7896f4af101cf3103dead3fb78c9a1` showed prompt-bar VText requests
could route conductor -> super -> VText before any durable VText decision row
existed. The repairs then narrowed around `no_worker_needed` predicates and
`prompt_bar_no_worker_decision_route` metadata, but staging continued to show
`initial_handoff=persistent_super`. Owner clarification on 2026-06-15 states
that this whole frame was too narrow: the problem is not a missing no-worker
predicate, it is a violation of the VText-centered Choir paradigm.

Core invariant: VText is Choir's versioned artifact control plane. Conductor
routes exogenous user/app/source input into VText-owned artifact state.
Prompt-bar requests, sourcecycled/news ingestion, article creation, mission
work, and most user prompts should open or create VText/context first. VText
owns the canonical artifact and then decides whether to write/revise, attach or
transclude sources, ask researcher, request super execution, coordinate
coding-agent trees through super, wait, or record an off-document
decision/blocker. Super is downstream execution authority invoked by VText; it
is not the ordinary direct ingress target for user/source prompts.

Conjecture delta: M3.2 must stop optimizing around no-worker special cases and
repair conductor-level ingress. The acceptance route is prompt bar -> conductor
-> VText for normal VText-centered submissions. Super before VText is failure.
Super after VText is valid only when VText requested it through
`request_super_execution`. Sourcecycled/news ingestion and article creation must
also show source/article artifacts becoming VText-owned, with downstream
researcher/super work attached back to VText/artifact context.

Protected surfaces: Choir Doctrine, AGENTS operating contract, VText invariant
doctrine, prompt-bar route materialization, source/article ingestion routes,
persistent-super preemption, VText `request_super_execution`, VText decision
persistence, Trace/log projection, and canonical VText writes.

Admissible evidence class: doctrine/docs checkpoint before code; focused local
route tests proving prompt-bar and source/article routes start with VText, not
super; tests proving `request_super_execution` remains available only as a VText
affordance; deployed product-path proof showing initial loop is VText for
prompt-bar VText submissions, no durable decision rationale in canonical text,
explicit owner-requested decision notes create `vtext_decisions` rows plus
Trace/log projection, and ordinary execution-shaped prompts can still lead
VText to request super after VText sees the artifact/request.

Rollback path: revert the route-invariant repair commit if it blocks VText from
requesting needed execution authority or breaks source/article materialization.
Do not restore conductor direct-super ingress as an untyped prompt heuristic;
instead document the narrower exception and prove why it is not ordinary
VText-centered ingress.

Heresy delta: discovered: no-worker route predicates were an overfit staging
repair that preserved the deeper heresy of super as direct ingress for
VText-centered work. introduced: none accepted. repaired: local route invariant
implementation complete, deployed proof pending.

## Staging Control-Text Checkpoint - 2026-06-15

Reliable evidence: commit
`3a5cbb41fd05b0eb3acf50c7ae930cbfc2108d1f` passed CI run `27524029167`; the
Deploy to Staging (Node B) job `81347742696` passed; public
`https://choir.news/health` reported both proxy and upstream sandbox
`deployed_commit` equal to that SHA. A deployed product-path proof submitted
fresh authenticated prompt-bar VText requests through `/api/prompt-bar` and
observed them through `/api/vtext/*/diagnosis` and `/api/trace/*`, with no
forbidden browser-public internal routes. Proof artifact
`/tmp/vtext-control-plane-staging-proof-1781498017452.json` recorded submission
`f26cc5ee-039c-4e35-82f9-24f6720efb4c`, document
`37ecfd95-5f79-4a14-a672-f828937c5e81`, and initial loop
`908aad8c-03aa-4e42-8f14-3501a4975145`. Diagnosis resolved that initial loop
as profile `vtext`, Trace entry was conductor, and no super-before-VText run
appeared. The same proof also showed decision row
`4b4ca8ed-c7ca-4fd5-a528-128d91d5e2e2` with
`decision_kind=no_worker_needed` and the expected reason/evidence refs, but
`canonicalContainsReason=true` because the prompt-bar initial user revision
stored the full control prompt as canonical VText text. A second deployed
execution-shaped prompt started with VText
(`initial_loop_id=6c245a00-feb6-4d80-b490-11e5994418c0`) but did not produce a
downstream VText-requested super run within the 240 second proof window.

Conjecture delta: the VText-centered ingress route repair and vmctl package
pointer repair fixed the super-first staging failure. M3.2 is now failing on a
different invariant: prompt-bar control text should enter VText as
instruction-bearing artifact context, not as durable reader-facing canonical
content. The off-document decision table can now record the explicit decision,
but acceptance still fails while the same rationale remains in canonical text.
The downstream-super acceptance also needs proof that VText can choose
`request_super_execution` after initial artifact materialization on the
deployed product path.

Protected surfaces: prompt-bar VText materialization, initial VText revision
creation, canonical VText writes, VText decision persistence, Trace/log
projection, `request_super_execution`, deployed prompt-bar route evidence, and
source/news/article VText ownership proof.

Admissible evidence class: focused local tests proving prompt-bar VText
materialization does not seed canonical document text with private control
rationale; tests proving explicit owner-requested decision notes still create
`vtext_decisions` rows and Trace/log projection; tests proving ordinary
execution-shaped prompts start with VText and can still lead VText to request
super; deployed product-path proof for the same route, canonical-text, decision
row, Trace/log, and downstream-super acceptance.

Rollback path: revert the prompt-bar materialization repair if it loses
owner-supplied document content or prevents VText from seeing the full request.
Do not restore direct super ingress or no-worker route predicates; repair must
preserve conductor -> VText artifact ownership.

Heresy delta: discovered: prompt-bar VText materialization can treat a control
prompt as canonical document body, leaking off-document rationale before VText
has authored a reader-facing revision. introduced: none accepted. repaired:
deployed prompt-bar super-first ingress.

## Suggested Goal String

```text
Use Parallax on docs/mission-vtext-prompt-decision-notes-m3.2-v0.md. Treat it
as the M3.2 gate between the settled M3.1 emergency repair and M3 lifecycle
cutover. Current status is working with V=2. Preserve Choir Doctrine and
docs/vtext-agentic-invariants-2026-06-13.md: VText is Choir's versioned artifact
control plane. Conductor routes exogenous prompt/source/article/mission input
into VText-owned artifact state; super is downstream execution authority that
VText may request later through request_super_execution. The no-worker route
predicate work is superseded as overfit staging evidence. Current pushed repair
is `3a5cbb41fd05b0eb3acf50c7ae930cbfc2108d1f`; CI run `27524029167` and Node B
deploy job `81347742696` passed, and `/health` reports that SHA for proxy and
sandbox. Deployed proof now shows prompt-bar -> conductor -> VText route order:
initial_loop_id was VText and no super-before-VText appeared. It still failed
because prompt-bar materialization stored the full control prompt, including the
off-document decision rationale, as canonical VText text; the explicit
`no_worker_needed` decision row and Trace/log evidence did exist. A second
execution-shaped prompt also started with VText but did not request super within
the proof window. A local prompt-bar intake repair now keeps the full request in
VText metadata/context while creating an intentionally blank canonical intake
revision, and focused route/prompt/source-article tests pass. Keep
record_vtext_decision backed by Dolt, readable from Trace/logs, and visible in
the VText Sources panel; keep request_super_execution only as a VText
affordance. Deployed prompt-bar route/canonical/decision acceptance now passes
on `39273a164ce08d6567bc5e05a04099a1167acdca`, but the execution-shaped leg
still failed because VText stayed running and did not request super inside the
240 second proof window. Next move: after committing the partial-acceptance
checkpoint, discriminate whether the downstream-super miss is proof-window/model
latency or insufficient VText prompt/tool pressure, then rerun deployed
acceptance covering prompt-bar plus source/news/article product paths where
feasible.
Settlement still requires focused schema/tool/prompt tests, route tests proving
VText before super, API/event/log readability proof, Sources-panel Playwright
proof, runtime/frontend checks for touched surfaces, push/CI/deploy, staging
identity, and deployed product-path proof; no claim outruns its evidence class.
```
