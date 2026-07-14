# Mission M3.4 - Prompt-Bar VText First-Draft Regression v0

## Summary

Prompt-bar ingress to VText is currently not reliable enough to resume M3 or
source/news work. On 2026-06-15 the owner submitted "What's new with Iran war"
through the prompt bar. The desktop opened a VText window and showed `V0` plus
`Writing first draft...`, but the document stayed empty for minutes and did not
advance to a useful appagent revision.

This is a core Choir regression. The ordinary product path should be:

```text
prompt bar -> conductor -> seeded VText artifact -> VText writes V1
  -> VText may request researcher/super -> findings/updates attach back
  -> VText writes later revisions
```

If that path is broken, M3 lifecycle work and sourcecycled/news work are both
untrustworthy, because their evidence can be dominated by broken artifact
control rather than the mission under test.

## Initial Evidence

- Owner screenshot at about 2026-06-15 08:17 America/New_York showed VText
  titled "What's new with Iran war", version `V0`, `Writing first draft...`,
  empty body text, and visible run id prefix
  `386f6c28-5...7be3ad`.
- Node B logs identify that VText activation as
  `386f6c28-5594-4605-ba02-5c90387be3ad`, started at
  `2026-06-15T12:15:16Z` from prompt-bar/conductor run
  `7855146d-59f0-419a-ab99-3ebb0e28481f` for owner
  `5bd6de97-3b58-408c-bf89-c42c81b083de`.
- Gateway logs show VText first tried configured provider fallbacks
  `xiaomi/mimo-v2.5` and `deepseek/deepseek-v4-flash`; both returned
  `402 Payment Required`. ChatGPT fallback then succeeded repeatedly.
- For the owner VM, gateway logs show repeated ChatGPT tool-use responses for
  the VText run with `tool_choice=function:edit_vtext`, but no observed
  terminal VText revision before the VM was killed.
- A deployed proof after runtime commit
  `60bd2f47c380432a3e55db5f766db6b6f9209bb9` reproduced the loop on a fresh
  staging prompt. It created prompt-bar/conductor run
  `e8bb34ab-8f47-4848-840f-f1b505487f0b` and VText activation
  `793f1e07-27e9-4c96-a33e-96c23ed0ea2d`; gateway logs again showed repeated
  exact `edit_vtext` tool calls until the owner VM restarted and passivated
  the run.
- vmctl logs show `vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19` Firecracker exits with
  `signal: killed` at `2026-06-15T12:17:22Z`. The restarted guest runtime logs
  `runtime: passivated run 386f6c28-5594-4605-ba02-5c90387be3ad (was running)
  after restart` at `2026-06-15T12:18:00Z`.
- Later direct probes found active guest `/health` ready on
  `http://10.200.64.2:8085`, but authenticated data routes such as
  `/api/prompt-bar/submissions/...`, `/api/trace/trajectories/...`, and
  `/api/vtext/documents` timed out in the same probe window.
- Current code intentionally creates an empty prompt-bar `V0` revision and
  stores the prompt in metadata plus the initial VText run prompt. Therefore
  an empty visible V0 is not alone the bug. The bug is failure to produce V1,
  clear pending VText state, or show an actionable recovery/blocker.

## Not This Mission

- Do not resume M3 until this prompt-bar-to-VText first-draft path has a
  deployed product proof.
- Do not treat "reload eventually helped" as success. Reload may only prove
  recovery from a stale UI or VM route, not VText artifact correctness.
- Do not weaken the VText doctrine by forcing a fixed researcher or super
  sequence. VText must keep agency; the path must prove VText can write and
  delegate when needed, not that runtime choreography scripts the roles.
- Do not absorb sourcecycled/news publication. Sourcecycled may add pressure,
  but the acceptance case starts with a plain owner prompt-bar submission.

## Parallax State

status: settled

mission conjecture: if prompt-bar/conductor/VText first-draft ingress is
repaired and proven through the deployed product path, then M3 can resume
without confusing lifecycle evidence with a broken artifact control plane.

deeper goal (G): restore VText as Choir's versioned artifact control plane for
ordinary owner prompts, so document work can create versions, collect
research/super evidence when VText chooses, and incorporate that evidence into
later revisions.

witness/spec (A/S): a narrow problem-first investigation and repair over the
prompt-bar -> conductor -> VText initial revision path. The final witness must
show a fresh deployed prompt-bar prompt creating a VText document, creating a
non-empty V1 appagent revision within a bounded window, clearing stale pending
state, exposing trace/diagnosis evidence, and preserving VText-first ingress.

invariants / qualities / domain ramp (I/Q/D):
- I: VText remains the canonical artifact writer. Conductor may create/open the
  artifact and seed intent, but must not become the first-draft author.
- I: ordinary prompt-bar ingress must not route directly to super before VText.
  Super after VText is valid only through an explicit VText request.
- I: VText may delegate, wait, write, or report a blocker; the repair must not
  turn `edit_vtext` into a semantic role-sequence gate.
- I: interrupted VText runs must not leave an owner-visible document in
  indefinite `Writing first draft...` state.
- Q: failures must become inspectable product/control evidence: run state,
  VText mutation state, tool result errors, provider fallback, VM restart, and
  any passivation/stale-mutation reconciliation.
- D ramp: start with the observed staging trace and focused local tests; then
  deploy and prove with browser-driven QA on `https://choir.news`.

variant (ranking function) V: current V=0:
1. completed: record the problem and initial conjectures before code changes;
2. completed: extract enough failed transition evidence for run
   `386f6c28-5594-4605-ba02-5c90387be3ad`: conductor decision, document id,
   revisions, VText mutation row, run state, tool result errors, and trace;
3. completed: identify repeated `edit_vtext` continuation as a tool-loop
   termination mismatch rather than prompt seed loss;
4. completed/falsified on staging: implement the narrow repair with focused tests, including
   terminal `edit_vtext` success and defaulting underspecified edit payloads
   from the pending VText activation;
5. completed: extract the failed deployed `edit_vtext` tool result/arguments
   from VText activation `20f1b17d-c8b5-4bfe-b17e-2ac546e77f5f`;
6. completed locally: implement the third repair against that failed transition;
7. completed: verify deployed product path with browser/computer-use evidence;
8. completed for this mission: update M3 readiness only after deployed proof shows prompt-bar VText V1
   creation and no indefinite pending state.

budget: one urgent red-surface repair pass before M3. If root cause crosses VM
duplicate-kill policy, provider fallback policy, and VText mutation recovery,
split after the first causal proof rather than shipping a broad blind patch.

authority / bounds: mutation class `red`. Protected surfaces: prompt-bar API,
conductor route materialization, VText document/revision/mutation state,
provider tool-loop handling, Trace/VText projection, vmctl restart/passivation
recovery, and staging deploy routing. Apply Problem Documentation First.

evidence packet: exact failed-run report, focused regression tests for the
identified transition, `nix develop -c scripts/go-test-runtime-shards` or a
justified narrower equivalent while shaping, pushed runtime commit, CI, Node B
deploy, staging health identity, browser-use or computer-use proof submitting a
fresh prompt through the real prompt bar, VText V1 content observed in UI/API,
Trace showing conductor then VText, no super before VText, pending mutation
cleared or honestly failed, and residual risks.

heresy delta: discovered: the product could open a VText artifact and then
leave the owner in an indefinite first-draft pending state after repeated
tool-use responses and VM restart/passivation. Repaired: exact initial
tool-choice duplicate handling now allows VText to execute the first
`edit_vtext` and create V1. Discovered but not repaired here: H001
parent/child ontology still appears in runtime logs, APIs, tests, and mission
docs. Introduced: none accepted.

position / live conjectures / open edges:
- C1 supported: prompt-bar seed is not necessarily lost. Code intentionally stores
  prompt-bar input in metadata and starts VText with that prompt while keeping
  visible V0 empty. The real failure is V1 non-creation or pending-state
  recovery.
- C2 supported and partially repaired: VText agent-revision runs made
  `edit_vtext` the exact initial tool, but did not treat successful
  `edit_vtext` as terminal. The loop could ask the provider for another turn
  after the canonical document write instead of completing the run.
- C2b weakened: deployed proof after the first repair still looped, so the
  first repair did not create a successful terminal edit. Runtime now derives
  omitted `doc_id`, `base_revision_id`, and operation from the single pending
  VText activation, but staging proof after
  `3b7e4c2b1571ca055be4826b686c782292a7a884` still produced only the
  user-authored V0 and repeated exact `edit_vtext` provider calls. Defaulting
  omitted context was necessary at most, not sufficient.
- C3 active: configured VText provider policy starts with providers returning
  `402 Payment Required`, then falls back to ChatGPT. This adds latency and
  noise but is not yet proven to be the root cause because ChatGPT returned
  tool-use responses.
- C4 active: vmctl duplicate Firecracker kills and guest restarts can interrupt
  VText mid-tool-loop. Boot passivation should mark pending VText mutations
  stale, but the owner experience still showed indefinite pending, so UI/SSE,
  mutation reconciliation, or route recovery may still be wrong.
- C5 active: active guest `/health` can be ready while authenticated data routes
  time out. Health cannot be the sole recovery oracle for VText product
  readiness.
- C6 active: the runtime and tests still carry H001 parent/child terminology
  and APIs (`StartChildRun`, `ParentRunID`, child-run budgets/result channels).
  That is a doctrine violation already named in Choir Doctrine and blocks M3
  lifecycle settlement, but it is not the root cause of the first-draft loop
  repair unless deployed proof shows parent/child control still drives this
  prompt-bar VText path.
- C7 active: the current missing oracle is the actual `edit_vtext` tool
  execution result and arguments for activation
  `20f1b17d-c8b5-4bfe-b17e-2ac546e77f5f`. Gateway logs prove repeated
  provider tool-use attempts but not whether runtime rejects arguments,
  duplicate-call ordering skips the valid edit, rationale/operation validation
  rejects the write, or trace/tool-result persistence is hidden behind a data
  route timeout.
- C8 supported and repaired locally: product diagnosis showed the provider
  returned two `edit_vtext` calls, but the tool loop emitted
  `model_called_different_initial_tool` and retried before executing any tool.
  The exact initial-tool guard required exactly one call instead of accepting
  one or more calls whose names all match the required tool. The existing VText
  duplicate policy would have executed the first edit and skipped the second,
  but the guard sat before that policy. Local repair makes the guard accept
  same-tool duplicate calls and adds a regression test proving one canonical
  edit executes, the duplicate notice is non-error, and terminal success ends
  the VText turn.
- C9 supported on staging: deployed commit
  `bf4f5158f26581e35534b7256043aaced009daa4` passed CI/deploy and fresh-auth
  browser product proof. The prompt-bar trajectory created user V0 and
  appagent V1 with marker `M34_DUP_EDIT_FIX_1781529413860`; Trace showed
  conductor -> VText, no super before VText, and no
  `model_called_different_initial_tool` retry. Provider fallback `402`
  noise remains, but it no longer prevents V1 creation.

next move: do not reopen M3.4 unless prompt-bar first-draft proof regresses.
Before M3 settlement, either repair or explicitly bound H001 parent/child
runtime residue in the M3 lifecycle paradoc. H001 is not accepted architecture.

ledger file: `docs/mission-vtext-first-draft-regression-m3.4-v0.ledger.md`

version / lineage: spawned from M3.3/M3 readiness review after owner-reported
manual QA regression on 2026-06-15. Blocks M3 until settled or explicitly
superseded by a narrower root-cause mission.

learning state: promote a durable acceptance for prompt-bar -> conductor ->
VText V1 creation with same-tool duplicate `edit_vtext` calls. The acceptance
portfolio over-weighted route proofs and under-weighted browser-driven/manual
QA of the core artifact loop.

settlement: settled on 2026-06-15 by deployed browser/product proof against
`https://choir.news` at `bf4f5158f26581e35534b7256043aaced009daa4`.
Fresh passkey user `m34-proof-1781529413860@example.com` submitted prompt-bar
run `1438bb6f-93fe-4e0a-99ac-212a68653391`, which opened VText activation
`6ab801c5-276c-4ebd-a028-8a578629bd50`, document
`75c85eba-b07b-41cc-811c-57528ba6f84c`, and appagent V1
`12c2df99-4370-4d98-babf-b680ea36021f` containing marker
`M34_DUP_EDIT_FIX_1781529413860`. Trace had first VText at stream seq 5,
no super, and no `model_called_different_initial_tool` retry. Artifact:
`/tmp/m34-vtext-proof-1781529413860/proof.json`.

## Suggested Goal String

```text
/goal M3.4 is settled in docs/mission-vtext-first-draft-regression-m3.4-v0.md. Do not reopen it unless prompt-bar first-draft proof regresses. Settlement commit bf4f5158f26581e35534b7256043aaced009daa4 repaired exact initial tool-choice handling so same-tool duplicate edit_vtext responses execute one canonical VText edit instead of retrying forever. Before M3 settlement, carry forward the residual H001 obligation: parent/child runtime vocabulary and control residue remain a discovered heresy, not accepted architecture.
```
