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
- Node B logs identify that run as
  `386f6c28-5594-4605-ba02-5c90387be3ad`, started at
  `2026-06-15T12:15:16Z` as a child of prompt-bar/conductor run
  `7855146d-59f0-419a-ab99-3ebb0e28481f` for owner
  `5bd6de97-3b58-408c-bf89-c42c81b083de`.
- Gateway logs show VText first tried configured provider fallbacks
  `xiaomi/mimo-v2.5` and `deepseek/deepseek-v4-flash`; both returned
  `402 Payment Required`. ChatGPT fallback then succeeded repeatedly.
- For the owner VM, gateway logs show repeated ChatGPT tool-use responses for
  the VText run with `tool_choice=function:edit_vtext`, but no observed
  terminal VText revision before the VM was killed.
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

status: working

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

variant (ranking function) V: current V=3:
1. completed: record the problem and initial conjectures before code changes;
2. completed: extract enough failed transition evidence for run
   `386f6c28-5594-4605-ba02-5c90387be3ad`: conductor decision, document id,
   revisions, VText mutation row, run state, tool result errors, and trace;
3. completed: identify repeated `edit_vtext` continuation as a tool-loop
   termination mismatch rather than prompt seed loss;
4. completed locally: implement the narrow repair with focused tests;
5. remaining: verify deployed product path with browser/computer-use evidence;
6. remaining: update M3 goalstring only after deployed proof shows prompt-bar VText V1
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

heresy delta: discovered: the product can open a VText artifact and then leave
the owner in an indefinite first-draft pending state after repeated tool-use
responses and VM restart/passivation. Introduced: none accepted. Repaired:
local repair candidate only; not accepted until deployed browser proof passes.

position / live conjectures / open edges:
- C1 supported: prompt-bar seed is not necessarily lost. Code intentionally stores
  prompt-bar input in metadata and starts VText with that prompt while keeping
  visible V0 empty. The real failure is V1 non-creation or pending-state
  recovery.
- C2 supported and repaired locally: VText agent-revision runs made
  `edit_vtext` the exact initial tool, but did not treat successful
  `edit_vtext` as terminal. The loop could ask the provider for another turn
  after the canonical document write instead of completing the run.
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

next move: commit and push the local repair, monitor CI and Node B deploy, then
run browser/computer-use product proof against `https://choir.news` with a fresh
prompt-bar submission. Acceptance requires non-empty V1, cleared pending state
or precise blocker, and trace evidence showing conductor -> VText before any
super.

ledger file: `docs/mission-vtext-first-draft-regression-m3.4-v0.ledger.md`

version / lineage: spawned from M3.3/M3 readiness review after owner-reported
manual QA regression on 2026-06-15. Blocks M3 until settled or explicitly
superseded by a narrower root-cause mission.

learning state: early evidence suggests the acceptance portfolio over-weighted
API/trace route proofs and under-weighted browser-driven manual QA of the core
artifact loop. Promote a durable acceptance once the repair lands.

settlement: settled only after deployed browser/product proof shows a fresh
prompt-bar submission creates a non-empty appagent V1 within a bounded window,
VText pending state clears or turns into a precise blocker, trace shows
VText-first ingress, and no runtime/code change remains unverified by CI and
staging.

## Suggested Goal String

```text
/goal Run docs/mission-vtext-first-draft-regression-m3.4-v0.md with Parallax. Treat this as a red protected-surface repair before M3. Start from the Parallax State, append moves to docs/mission-vtext-first-draft-regression-m3.4-v0.ledger.md, and do not change runtime code until the problem checkpoint is committed or preserved. Investigate the failed owner prompt-bar run 386f6c28-5594-4605-ba02-5c90387be3ad and parent 7855146d-59f0-419a-ab99-3ebb0e28481f: conductor decision, VText doc/revisions, mutation state, tool result errors, provider fallback, vmctl duplicate-kill/passivation, and data-route timeouts. Preserve invariants: VText is the artifact control plane; no direct-super ingress for ordinary prompts; no forced researcher/super sequence; no indefinite Writing first draft state. Implement only the narrow root-cause repair, then verify with focused tests, CI, Node B deploy, staging health identity, and browser/computer-use proof that a fresh prompt-bar prompt creates a non-empty V1, clears pending state or records a precise blocker, and shows conductor -> VText before any super.
```
