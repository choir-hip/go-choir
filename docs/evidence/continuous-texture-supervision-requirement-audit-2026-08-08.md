# Continuous Texture Supervision — Requirement Audit

**Date:** 2026-08-08
**Definition:** `choir-continuous-texture-supervision-2026-08-07`
**Runtime candidate:** `ac6dd16b1980a1a3faedd7d1d83fefa79395a1ee`
**Audit verdict:** **NOT COMPLETE — exact product path reached; protected provider availability blocked the supervision loop**
**Effects:** OFF

This is a requirement-by-requirement audit, not a completion receipt. It
separates source/CI proof, deployed product-path proof actually reached, and the
remaining evidence floor. A partial product receipt does not promote the
unexecuted Researcher/Super/capsule behavior.

## Identity and landing gates

| Gate | State | Evidence |
| --- | --- | --- |
| Frozen implementation candidate | PASS | `ac6dd16b1980a1a3faedd7d1d83fefa79395a1ee` |
| Independent lifecycle review | PASS | Earlier exact-control ACCEPT at `3ac0935b`; final document-scoped wake-serialization review ACCEPT at `ac6dd16b` |
| Independent capsule/security review | PASS at source | ACCEPT at `363bf393`; protected-delta ACCEPT through `99fc3e6b` |
| Applicable CI | PASS | GitHub Actions run `31261269488`, including build, vet, selected Race, SBOM, docs truth, and staging deployment |
| Host and product identity | PASS | Nonce-bound identity joined host, sandbox, route, deployment receipt, platform attestation, computer scope `vm-bbdbbd01c4390b7036067aaa12afeb68`, guest `computer-42850e9734d9442386c5dd8bf3afbf19`, and VM epoch 8247 to exact `ac6dd16b` |
| Product lifecycle route | PASS | Authenticated create, tell, model-policy file/resolve, trajectory inspection, and conditional public cancel all executed on the exact candidate |
| Initial committed-work projection | PASS deployed | No-SSH deployment restart created run `5ee276b3-d25c-41ac-afaa-5879a6ea5ecf` for the previously stranded initial work |
| Provider-dependent acceptance | **BLOCKED — RESTORATION AUTHORIZED/PENDING** | Host ChatGPT returned HTTP 401 `refresh_token_reused` on both original and fresh trajectories; Z.AI returned HTTP 429 and later an open unhealthy-provider circuit; configured fallback chains ended at stale ChatGPT. Sanitized local reproduction after staging found DeepSeek/Xiaomi HTTP 402 balance failures, Fireworks HTTP 412 `PRECONDITION_FAILED`, Z.AI HTTP 429 code `1113`, and direct AWS Bedrock bearer invokes in `us-east-1` returned HTTP 403 for both gateway seed `us.anthropic.claude-haiku-4-5-20251001-v1:0` and exact owner-policy-selected `us.anthropic.claude-sonnet-4-6`; these local calls diagnose availability only and are not acceptance evidence or host-credential proof. No Researcher/update/revision/Super/capsule was produced. Exact-F recurrence trajectory `e5f85464-560b-5383-b199-cf4c62c12145` / run `74a5a20f-24a3-4b25-b11c-1072f881f8a9` again failed iteration zero on ChatGPT 401 and was publicly cancelled at version 3. At `2026-08-08T23:24Z` the operator explicitly authorized the local-token/SSH Node B restoration; sanitized preflight selected a ChatGPT-only atomic auth-file replacement because full helper execution would also change Bedrock. |
| Acceptance-artifact cleanup | PASS | Original failed trajectory `8f3b6ac6-dbdf-5bfe-99f0-661961c64f3d` was publicly cancelled at lifecycle version 9; fresh recurrence trajectories `41cec88f-510f-53cc-a5e7-84c372b5421b` and `aca3504c-2ae0-5a4e-bab5-b22541e90585` were publicly cancelled at lifecycle version 3; original model-policy bytes were restored at SHA-256 `7192b8b1600561a331fda32f27628296c3f5b9bd1ba30dd5fb82681985c45e2a`; all temporary API keys were revoked and rejected on subsequent use. Exact-F recurrence was likewise retained/cancelled at version 3; the first setup key lacked `write:texture`, was refused HTTP 403 with no mutation, and both new setup keys were revoked/post-401. Empty generic document `457320df-e047-405c-b2a1-a0263b4cb5dc` from an operator route mistake is retained with current_version_number/revision_count 0 and no trajectory/run and is not acceptance. |
| Source rollback | **DEPLOYED BOUNDED REHEARSAL + RECOVERY PASS / STRICT MIDPOINT OBSERVABILITY FAIL** | Canonical R `10d48659` made the exact 99 non-document paths cdaa-equivalent; CI/rolling/deploy `31267448310` passed and exact R ran at epoch 8248. R preserved run/lifecycle/selfdev/policy/inventory summaries, but its legacy response omitted six stored owner-instruction `request_id` fields, so the full v9 response digest differed and the identity-ambiguity gate fired. Canonical F `67a61358` immediately restored the exact pre-R whole tree; CI/rolling/deploy `31268477380` passed, exact F ran at epoch 8249, and all captured frozen comparator response/state digests equalled baseline. Exposure was 1,909 seconds; key revoked/post-401. Canonical deployed rollback/recovery is rehearsed; strict old-runtime forward-observability and complete checkpoint/capsule acceptance remain open. |
| Registry terminal closure | OPEN | ACTIVE, mission graph, and authority manifest remain active/working; effects remain OFF |

## Finish acceptance actions

| # | Required action | Source/contract state | Deployed state |
| --- | --- | --- | --- |
| 1 | Focused registry, authorization, reducer, atomic turn, projection, transclusion, replay, cancellation, capsule, API/CLI tests | **PASS.** Joined full/focused/Race validation and selected CI passed; independent reviews found no remaining critical/high source defect. | Local/CI is the admissible class for this action. |
| 2 | Reconstruct pending controls; replay identities and pre-cutover fixtures | **PASS locally.** Exact-run Super reconstruction, same-run Researcher append recovery, authenticated-memory replay, restarted observation, create/instruction replay, CLI cursor replay, and historical show/source contracts passed. | **PARTIAL PASS.** The deployment restart recovered the exact initial work into run `5ee276b3-d25c-41ac-afaa-5879a6ea5ecf`; restart with pending Researcher/Super controls remains unexecuted because no provider completed the first Texture turn. |
| 3 | Real Texture→Researcher/Super→Texture repeated loop with parallel isolated CoSupers | Runtime path exists and is source-reviewed. | **BLOCKED.** Texture reached its real provider boundary, but no provider completed iteration zero/its fallback chain; no downstream actors, assignments, or capsules exist. |
| 4 | Public API/CLI create, tell/correct, watch/resume, show, open-source | Local API/CLI contracts passed. | **PARTIAL PASS.** Public create, durable tells, trajectory show, model-policy rollback, and lifecycle cancel executed. Deployed CLI `read`, `history`, `revisions`, and exact `show` returned the retained document/v0; separate `watch --once --limit 2` processes resumed cursors `0→2→4→6→8→9`, then returned empty after 9. Cancelled `tell`/`correct` failed HTTP 409 and cross-document revision `show` was rejected. Positive correction against a live Texture-authored head, historical v1+, and positive source-open remain blocked on provider output. |
| 5 | Continuous-prose and differently styled report cases | Generic schema/prompt source contract exists. | **BLOCKED.** The continuous-prose case was durably created, but provider failure prevented its first generated revision; the second style case was not started. |
| 6 | Pending controls, passivation, approved no-SSH same-build restart, same identities | Deterministic local recovery contracts passed. | **PARTIAL PASS.** Ordinary deployment restart recovered initial owner work on exact identities. Pending Researcher/Super controls and same-run append recovery remain unproved on staging. |
| 7 | Direct owner revision plus natural-language correction | Atomic owner-head rebase and lifecycle instruction source tests passed. | **PARTIAL PASS.** Natural-language owner instructions queued durably and woke exact Texture runs. No Texture-authored revision existed to correct or rebase. |
| 8 | In-flight cancellation, delayed authenticated result, exact retry | Deterministic cancellation and bounded evidence-only closure passed source review and Race tests. | **PARTIAL PASS.** Public conditional cancellation cancelled the failed trajectory and resident activation. No in-flight capsule effect or delayed authenticated result existed, so late-evidence semantics remain unproved. |
| 9 | Compare event heads, self-development state, checkpoint, route, host projections before/after | Source gates refuse out-of-capsule and late promotion effects. | **DEPLOYED BOUNDED R/F PASS / ACCEPTANCE PARTIAL.** Before/R/F comparison kept the run digest exact (12 terminal/0 active), all lifecycle summaries/heads/work states exact, self-development `off` generation 0, policy SHA-256 `7192b8b1…`, the stable VM/guest/route digest, and one-computer inventory; only deployment/build receipts and epochs 8247→8248→8249 changed. R omitted six stored request IDs from its legacy response, so strict midpoint observation failed and triggered immediate F; final F full trajectory digests exactly returned to baseline. The key was revoked/post-401. No real capsule effect or canonical checkpoint inventory exists, so the complete provider-dependent no-effect acceptance remains open. |

## Measures

| Measure | Audit state |
| --- | --- |
| `target_authority` | **PASS at source/CI; partial deployed.** Exact document/work/head-bound initial wake and owner instruction ran; downstream actor target bindings were not exercised. |
| `single_delivery_authority` | **PASS at source/CI.** New traffic uses lifecycle controls/bindings/receipts and did not enter the legacy mailbox. No deployed downward packet was produced. |
| `revision_control_causality` | **BLOCKED deployed.** No accepted trajectory with two downward cycles and later revisions. |
| `authority_negative_matrix` | **PASS at source/CI; Linux residual open.** Registries/runtime refuse generic host/effect tools; real capsule kernel isolation awaits a provider-completed Super assignment. |
| `owner_read_amplification` | Telemetry only; no generated owner-visible version exists. |
| `progressive_owner_visibility` | **BLOCKED deployed.** No informative revision while work remained open. |
| `generic_grounded_writing` | Schema/source contract passes locally; exact deployed research/execution transclusions in two writing forms are blocked. |
| `automatable_texture_surface` | **PARTIAL PASS deployed.** API create/tell/show/cancel and CLI read/history/revisions/exact-show plus disconnected durable watch resume through terminal cursor 9 passed; caught-up resume returned empty and cancelled/mismatched authority failed closed. Positive live correction, generated historical v1+, and source-open remain unproved. |

## `not_done_when` audit

The Definition is not complete because multiple stopping prohibitions remain
triggered or cannot yet be excluded by the required evidence class.

| # | Current disposition |
| --- | --- |
| 1 | **EXCLUDED:** owner ratification and executable authority remain present in all registries; terminal registry closure is separate and still open. |
| 2 | **TRIGGERED:** source, review, CI, exact deployment, and partial product receipts exist, but the repeated authenticated supervision loop does not. |
| 3 | **UNPROVED:** Texture was product-started and attempted the provider call, but no successful run addressed Researcher, Super, or CoSuper. |
| 4 | **TRIGGERED:** no worker round trip or progressive revision exists. |
| 5 | Excluded by source tests/review: lifecycle authority is single-tape and new traffic does not enter the legacy worker inbox. |
| 6 | Excluded by source tests/review; real Linux capsule isolation remains required. |
| 7 | Excluded by atomic-turn/durable-disposition source tests; deployed turn behavior remains unobserved. |
| 8 | **PARTIALLY EXCLUDED:** authenticated readback kept self-development OFF generation 0, policy bytes exact, route digest/epoch/build identity unchanged, and cancelled lifecycle head stable; no real capsule/checkpoint/event-head comparison exists, so full no-effect proof remains open. |
| 9 | Excluded by fail-closed source tests. |
| 10 | Excluded by direction-specific producer/target work identities and reducer validation. |
| 11 | Excluded by atomic Texture-turn conditional commit tests. |
| 12 | Excluded by direct owner atomic head/rebase/wake source tests; no deployed Texture-authored head existed to rebase. |
| 13 | Excluded by assigned-CoSuper empty-set registry and Store/runtime authority checks; no deployed assignment ran. |
| 14 | **UNPROVED on staging Linux:** cross-compile and source review cannot prove namespace, cgroup, seccomp, Landlock, overlay, network, and cleanup behavior. |
| 15 | **TRIGGERED:** no deployed informative revision while work remained open. |
| 16 | **TRIGGERED:** no deployed openable research/execution transclusions exist. |
| 17 | **TRIGGERED/PARTIAL:** the exact API/CLI read/show/watch-resume surface is deployed and automatable through terminal cursor 9, including fail-closed negatives, but positive correction, generated historical version, and source-open acceptance are absent. |
| 18 | **UNPROVED deployed:** neither requested generated writing form completed a first revision. |
| 19 | Multiple isolated assignment support exists in source, but parallel writable-capsule product behavior is **unproved**. |

## Product receipts retained

- document `11902866-d32e-55c4-9483-d9bd47c91a6c`;
- initial revision/head `d1a831ba-6af5-5206-aa03-49caf4b047dc`;
- cancelled trajectory `8f3b6ac6-dbdf-5bfe-99f0-661961c64f3d`;
- cancelled initial work `74fa5e0f-92ee-5e3a-ac8f-0c4b8f044e4c`;
- recovered exact initial-wake run `5ee276b3-d25c-41ac-afaa-5879a6ea5ecf`;
- later provider-probe runs recorded in the problem inventory;
- public cancellation command
  `public-cancel:cts-failed-acceptance-cancel-ac6dd16b-v7`, terminal lifecycle
  version/reducer sequence 9;
- post-escalation ChatGPT trajectory
  `41cec88f-510f-53cc-a5e7-84c372b5421b` / run
  `dcf2088c-836e-47db-8173-80f0adb7dcf3`, cancelled at lifecycle version 3;
- cooldown-aware Z.AI trajectory
  `aca3504c-2ae0-5a4e-bab5-b22541e90585` / run
  `f0d0e6ea-f98b-484c-9630-b6c849279118`, cancelled at lifecycle version 3.
- exact-F default ChatGPT recurrence: document
  `39eafb8c-11c6-5ecc-a8c9-aec323eaa67d`, v0
  `79dc0bed-d71b-5a31-97d5-371c3c06d916`, trajectory
  `e5f85464-560b-5383-b199-cf4c62c12145`, work
  `8d62ca55-cae2-5f9a-95e5-83c0245b3fb1`, run
  `74a5a20f-24a3-4b25-b11c-1072f881f8a9`; ChatGPT 401 at iteration zero;
  publicly cancelled at version 3; two keys revoked/post-401.
- deployed CLI partial acceptance on retained document
  `11902866-d32e-55c4-9483-d9bd47c91a6c`: exact v0
  `d1a831ba-6af5-5206-aa03-49caf4b047dc`; watch resume cursors
  `0→2→4→6→8→9`, empty after 9; cancelled tell/correct HTTP 409; cross-document
  revision show rejected.
- authenticated partial no-effect readback: exact acceptance computer
  self-development `off`, generation 0 before/after; model-policy SHA-256
  `7192b8b1…`; identical route digest, epoch 8247, and host/guest `ac6dd16b`
  identities across post-deploy probes; cancelled head/watermark stable; readback
  key revoked/post-401.
- deployment-rollback preflight/refusal: historical run `31030833230` and
  exact-ac6 recovery run `31261269488` inspected; one active interactive
  computer, 12 terminal/0 active runs, self-development OFF generation 0,
  policy/head stable; rerun-all rejected before effect; key revoked/post-401.
- representative cross-version rollback compatibility: ac6-created terminal
  graph with new owner-instruction, Texture-turn/revisions, exact-run control,
  cancellation-intent, run/work/event/receipt kinds; old `460c1423`
  Store/Runtime start on the same Dolt HEAD dispatched zero actors and left
  HEAD/status unchanged; ac6 reopen retained all new-only kinds.
- canonical deployed R/F: R `10d48659`, CI/rolling/deploy `31267448310`,
  epoch8248; strict midpoint v9 digest `2a4c…→dd5e…` from six legacy-response
  `request_id` omissions triggered immediate F; F `67a61358`, exact pre-R tree,
  CI/rolling/deploy `31268477380`, epoch8249; all captured frozen comparator response/state digests exact,
  exposure 1,909 seconds, key revoked/post-401. Qualified verdict: bounded
  rollback and recovery pass; strict midpoint forward-observability fail, safely
  recovered.

These are failure/partial-path evidence. They do not count as an accepted
Researcher, Super, CoSuper, capsule, transclusion, or run-acceptance graph.

## Remaining evidence floor

Still missing: successful usability proof for the operator-ratified scoped ChatGPT restoration;
authenticated repeated-cycle trajectory; three progressive revisions; two
downward control cycles; owner correction against a Texture-authored head;
openable research and execution sources plus positive correction/CLI parity;
parallel implementation and independent writable verification capsules; real
Linux isolation and cleanup; restart with pending controls; actual delayed
receipt/replay evidence; complete protected-state before/after comparison;
owner-visible inspection; canonical checkpoint inventory and strict legacy forward-observability (if separately required); run acceptance;
terminal registry closure; and the full accepted identity graph: owner/computer,
document/revision, immutable source/version/hash, trajectory/run,
producer/target work, actor, request/command/update/occurrence/digest,
assignment/capsule/execution handle, cancellation/late evidence, and acceptance
identifiers.

## Independent audit status

The earlier independent audit correction at exact docs candidate `c6af39f3`
remains incorporated: rollback is not overstated, action 2 names the complete
local recovery contracts, the missing identity graph is explicit, and owner
ratification is distinguished from terminal registry closure. New deployed
receipts narrow the blocker from route identity to protected provider
availability but do not change the **NOT COMPLETE** verdict. A final independent
doc-truth review returned **ACCEPT** after repairing the stale ACTIVE invocation,
terminal-registry wording, and prohibition 17 classification; it confirmed the
9/8/19 coverage and three-registry consistency.

The operator explicitly ratified the previously missing transition at
`2026-08-08T23:24Z`: use the existing local ChatGPT token and SSH Node B for a
scoped gateway-auth restoration. Sanitized preflight selected only the tracked
helper's atomic ChatGPT auth-file copy/restart subset because full helper
execution would also change the unequal local Bedrock credential and exceed the
ratified scope. Before the explicit ratification, repository and GitHub authority inspection
found no scoped product API or `choir` CLI renewal path, no provider Actions
secret, and no repository environment authority; only the Node B SSH host/key
secrets existed, so the tracked SSH-shaped break-glass path was then outside
acceptance authority. The `2026-08-08T23:24Z` exception now authorizes only the
exact Node B ChatGPT auth-file transfer and gateway restart described above; it
does not authorize general SSH mutation, guest injection, gateway env/Bedrock,
policy, route, or auth weakening. Sanitized local diagnostics show every
configured provider/model route failed before tool semantics; balance attribution applies only to DeepSeek/Xiaomi and
the qualified Z.AI classification, not Fireworks. Local ChatGPT token metadata reports an unexpired expiry, which does not prove
usable auth by itself; the ratified scoped transfer plus a successful sanitized
canonical product ChatGPT probe is now the admissible proof path.

A post-recurrence independent blocked-slice audit returned **NONE**: every
remaining acceptance gap is downstream of the first successful real Texture
provider turn; further local/source proof would duplicate already-passing
contracts, manually created actors/evidence would be synthetic acceptance, and
retained failures or active registries must not be deleted. After the ratified restoration transition proves usable auth, exact deployed F
`67a61358ceda55c30e9853907f85648bb8531bb8` identity must be reverified and a
fresh trajectory must run the complete acceptance. This Definition authorizes only the exact scoped Node B ChatGPT transfer/restart
exception. It still forbids general SSH, guest credential injection, gateway
env/Bedrock mutation, auth weakening, route mutation, or an unreviewed silent
fallback.
