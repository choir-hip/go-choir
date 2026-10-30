# Continuous Texture Supervision — Joined Runtime Review

**Date:** 2026-08-08  
**Reviewed runtime candidate:** `363bf39398128fa0e1a85a19ae9a7762f92ba3dc`  
**Final deployed source candidate:** `ac6dd16b1980a1a3faedd7d1d83fefa79395a1ee`
**Base deployed source:** `cdaa787bf2d006a1d4e59c1650a232f2083d8f9d`  
**Mutation class:** red  
**Effects:** OFF pending exact Linux staging acceptance and terminal registry closure

## Joined result

The local candidate now uses the standard lifecycle object graph, event tape,
conditional transitions, and `choir.lifecycle_command` receipts for continuous
Texture control. It does not add a second queue, callback authority, poller,
public supervision API, generic capsule route, or test-only product route.

Independent lifecycle red-team review returned **ACCEPT** on `3ac0935b` after
repairs for terminal Super evidence, complete ordered delivery, authenticated
memory-derived settlement, same-run append recovery, owner-head rebase, one
public version per turn, cancellation/report races, and unbound assignment fate.
The later runtime candidate preserves those surfaces and adds resident drain and
bounded delayed-report closure.

Independent capsule/security red-team review returned **ACCEPT** on the exact
runtime candidate `363bf393`. It found no remaining critical/high source defect
in assignment cancellation or late-receipt authority. The review confirmed:

- durable cancellation intent wins every report/freeze/candidate race;
- bounded retry retains exact delayed receipts across fate/version movement;
- terminal and unbound capsules require revoke intent, executor cleanup and a
  structured durable acknowledgement;
- restarted Linux executors must kill/delete exact orphan cgroups, detach exact
  overlays, remove state, and independently prove residue absent before receipt;
- receipt-bearing late evidence requires exact stored execution-handle identity;
- durable cancellation drains the intersection of exact trajectory runs and
  process-local resident activations without starting historical actors; and
- only one sole runtime-selected terminal assignment report receives a bounded
  detached closure after provider cancellation. Partial reports, mixed batches,
  capsule effects, injections, and later calls remain cancelled.

## Verification performed

All commands below passed on the joined source (with repeated focused runs during
repair):

```text
go test ./internal/store -count=1
go test ./internal/agentcore -run '^(Test[A-F]|Example[A-F])' -count=1
go test ./internal/agentcore -run '^(Test[G-L]|Example[G-L])' -count=1
go test ./internal/agentcore -run '^(Test[M-R]|Example[M-R])' -count=1
go test ./internal/agentcore -run '^(Test[S-Z]|Example[S-Z])' -count=1
go test ./internal/textureowner ./internal/toolregistry ./internal/types ./cmd/choir -count=1
go test -race ./internal/store ./internal/agentcore ./internal/textureowner <focused joined patterns> -count=1
go test -race ./internal/toolregistry ./internal/agentcore   -run 'Test(RunToolLoopDetachesOnlySoleSelectedTerminalEvidenceAfterProviderCancellation|DetachedAssignmentReportClosureSelectsOnlySoleTerminalResult|LateAssignmentExecutionReceiptsAuthenticateExactDetachedAuthority)$' -count=1
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test ./internal/capsule -c   -o /tmp/choir-capsule-final-linux.test
```

The Linux artifact was an x86-64 statically linked ELF. It was not executable on
the Darwin development host. This is compile evidence, not Linux behavior proof.

The Definition receipt linter passed with zero errors before this receipt was
added. The Definition dashboard remained live at `http://127.0.0.1:8787/`.

## Red-mutation receipt

- **Conjecture delta:** the joined source makes C6/C7 testable through one durable
  feedback substrate; they remain conjectures until deployed product evidence.
- **Protected surfaces:** Texture canonical writes/observation; lifecycle CAS,
  event and receipt projection; actor delivery and cancellation; authenticated
  run memory; persistent Super identity; capsule capability, cgroup, overlay,
  receipt and late-result authority; API/CLI; run acceptance and deployment.
- **Admissible evidence reached:** local focused/full/race, exact independent
  source review, and Linux cross-compile. Deployed Linux behavior is outstanding.
- **Rollback:** revert the unpublished range to
  `cdaa787bf2d006a1d4e59c1650a232f2083d8f9d`; retain additive packets/receipts as
  pending or late; keep all effects OFF.
- **Heresy delta:** `discovered` — cancellation/report, restart residue,
  execution-handle compatibility, resident drain, and cancelled-provider closure
  gaps documented before their repairs. `introduced` — none known after review.
  `repaired` — source-level instances above; no global or deployed claim yet.

## Protected CI-repair follow-up and landing result

The first pushed documentation-bearing candidate failed CI as recorded in the
problem inventory. Candidate `99fc3e6b7bf151ddad1f0927ca18a24ba5275d10`
then repaired only the observed landing failures: mechanically scoped detached
closure cancellation, mountinfo-aware exact-root cleanup, exact Texture
activation fixture selection, and a bounded 25-minute timeout for the unchanged
isolated Store Race suite.

Independent capsule/security follow-up review returned **ACCEPT** on the exact
`363bf393..99fc3e6b` protected delta with no critical/high finding. Local vet,
focused Race, and linux/amd64 capsule cross-compilation passed. Full selected CI
run [`31257971088`](https://github.com/choir-hip/go-choir/actions/runs/31257971088)
completed successfully. Node B's activation receipt at
`2026-08-08T13:12:35Z` names exact commit `99fc3e6b` for autoputer, active
computers, host services, and the canonical checkout.

Authenticated product acceptance did **not** pass at this intermediate landing point. The then-current owner-scoped
computer `computer-03335285269bdba4f94377e56879f9e6`, epoch 130, remains joined
to an immutable constructed ComputerVersion whose code commit is
`7122f2799be4458f4b925be11990321c7e70ffc4`. The deploy log explicitly preserved
that `candidate-fleet` realization while refreshing a different user's active
computer. On the preserved computer,
`POST /api/texture/lifecycle-documents` returns HTTP 404
`texture endpoint not found`; it therefore cannot produce any of this
Definition's required actor, lifecycle, Texture, source, capsule, cancellation,
or restart artifacts.

The mismatch was documented before mitigation. The Definition did not authorize
SSH, a new candidate/worker computer, deletion of the unclaimed
constructed-computer residue, a user-computer route mutation, or effects/promotion
before acceptance. At that intermediate gate, CI and host deployment were green
while product-path and real-Linux gates remained open. The authorized environment
recovery and later exact-candidate product probe below supersede this route
blocker as current mission state; this section remains historical failure
evidence.


## Exact lifecycle-wake repair, deployment, and product probe

The authorized exact-candidate environment was recovered without changing the
preserved constructed computer or creating a candidate/worker computer. The
first authenticated create at deployed `99fc3e6b` exposed the initial-work wake
defect documented in the problem inventory. After two independent high findings
were documented and repaired, the final source
`ac6dd16b1980a1a3faedd7d1d83fefa79395a1ee` serialized reconciliation only for
the canonical `{owner_id, computer_id, doc_id}` scope, revalidated durable
document binding after waiting, and retained one authoritative winner without
blocking unrelated documents. The final lifecycle red-team returned **ACCEPT**.
Focused repeated Race, full `internal/textureowner`, vet, and joined validation
passed.

GitHub Actions run
[`31261269488`](https://github.com/choir-hip/go-choir/actions/runs/31261269488)
passed, including build, vet, Race, SBOM, and staging deployment jobs. A fresh
nonce-bound execution-identity request joined the authorized computer scope
`vm-bbdbbd01c4390b7036067aaa12afeb68`, guest identity
`computer-42850e9734d9442386c5dd8bf3afbf19`, VM epoch 8247, autoputer and host
builds, route digest, deployment receipt, executable/image/kernel digests, and
platform attestation to exact `ac6dd16b`. Boot recovery then created Texture run
`5ee276b3-d25c-41ac-afaa-5879a6ea5ecf` for the previously stranded initial
work. This is deployed proof that the committed-start projection repair ran
after the ordinary no-SSH deployment restart.

The provider-dependent acceptance did not pass. That recovered run failed before
iteration zero on server-owned ChatGPT authentication with HTTP 401
`refresh_token_reused`. Authenticated owner-visible model-policy probes then
exercised configured DeepSeek, Fireworks, Z.AI, and Bedrock selections without
exposing or injecting credentials. DeepSeek and Fireworks paths exhausted their
provider-precondition fallback chains into the same terminal ChatGPT failure;
Z.AI returned HTTP 429; Bedrock was unsupported in the deployed gateway. No
probe produced a Researcher, update, revision, source, Super control, capsule,
or effect. The exact original model-policy bytes were restored at SHA-256
`7192b8b1600561a331fda32f27628296c3f5b9bd1ba30dd5fb82681985c45e2a`.

The retained failed trajectory
`8f3b6ac6-dbdf-5bfe-99f0-661961c64f3d` was cancelled through the public
conditional lifecycle endpoint using expected version 7 and exact head
`d1a831ba-6af5-5206-aa03-49caf4b047dc`. The returned snapshot has trajectory and
initial work cancelled, terminal head unchanged, lifecycle version/reducer
sequence 9, and resident activation cancelled. The temporary acceptance API key
was revoked and a subsequent use returned HTTP 401. These are admissible product
receipts for exact identity, initial wake, owner instruction, byte-exact policy
rollback, and lifecycle cancellation—not for the required repeated supervision
or Linux capsule loop.

The remaining blocker belongs to protected canonical gateway credential/provider
availability. This mission does not authorize SSH, guest token injection, auth
weakening, route changes, or a silent model fallback. Effects remain OFF; real
Researcher/Super/CoSuper progression, capsule isolation/cleanup, restart with
pending controls, late evidence, complete checkpoint/capsule no-effect
comparison, run acceptance, and registry closure remain open. The later
canonical R/F section closes the deployed rollback-rehearsal gap with its
qualified bounded-pass/strict-observability-fail verdict.


### Post-escalation provider recurrence

A later continuation reverified exact `ac6dd16b` product identity and did not
assume the external blocker persisted without proof. A fresh ChatGPT trajectory
`41cec88f-510f-53cc-a5e7-84c372b5421b` again failed at iteration zero with
`refresh_token_reused`. After sufficient cooldown, fresh Z.AI trajectory
`aca3504c-2ae0-5a4e-bab5-b22541e90585` failed at iteration zero because the
provider circuit was open as upstream unhealthy. Both trajectories were
conditionally cancelled through public lifecycle authority at version 3; the
original policy bytes were restored at the same SHA-256, and both temporary keys
were revoked.

Structural inspection found no scoped product API or `choir` CLI for host-side
provider credential renewal. The repository's credential deployment and
one-time recovery paths are SSH-shaped break-glass operations, not admissible
product acceptance. This confirms the remaining dependency is external
protected gateway auth/provider health rather than lifecycle wake, route
identity, or another model-policy permutation.


### Sanitized account and operator-authority diagnosis

Local reproduction was performed only after exact staging provider failures and
is not counted as product acceptance. Using gitignored operator credentials
without printing secrets or response bodies, minimal requests returned DeepSeek
HTTP 402 with a balance-related error classification, Xiaomi HTTP 402
`insufficient_balance`, Fireworks HTTP 412 `PRECONDITION_FAILED`, Z.AI HTTP
429 code `1113` with a balance/rate classification, and direct AWS Bedrock
bearer invokes in `us-east-1` returned HTTP 403 for gateway seed
`us.anthropic.claude-haiku-4-5-20251001-v1:0` and exact owner-policy-selected
`us.anthropic.claude-sonnet-4-6`. This shows every configured provider/model route failed before tool
semantics; account/balance attribution applies to DeepSeek/Xiaomi and the
qualified Z.AI classification, not the Fireworks precondition failure or
Bedrock forbidden response. The Bedrock results are local bearer/model/region
availability only and do not establish host credential state. Local Codex token
metadata reports an unexpired expiry, which does not prove usable auth, while
the host gateway retains stale auth.

GitHub authority inspection found only the Node B SSH host/key Actions secrets,
no provider secret, no repository environment, and no repository variable.
Repository inspection found no scoped product API or `choir` CLI provider-auth
renewal. The existing credential deployment and recovery workflows are
SSH-shaped break-glass paths forbidden by this acceptance. No credential or
authority was changed. The remaining transition is provider account restoration
or a separately ratified scoped no-SSH host renewal authority.


### Exact-F default ChatGPT recurrence

A fresh normal owner session minted a short-lived computer-scoped key on exact
deployed F `67a61358`, epoch 8249. The first key omitted `write:texture`; its
attempted generic document create was refused HTTP 403 `missing required scope:
write:texture` with no mutation, and the key was revoked/post-401. A replacement correctly
scoped key first made the mistaken generic create described below; the correct
lifecycle route then started the unchanged continuous-prose objective. Lifecycle
identities were document `39eafb8c-11c6-5ecc-a8c9-aec323eaa67d`, v0
`79dc0bed-d71b-5a31-97d5-371c3c06d916`, trajectory
`e5f85464-560b-5383-b199-cf4c62c12145`, work
`8d62ca55-cae2-5f9a-95e5-83c0245b3fb1`, and run
`74a5a20f-24a3-4b25-b11c-1072f881f8a9`. The default policy resolved
`chatgpt/gpt-5.5`; iteration zero returned HTTP 401 `refresh_token_reused`
before any tool action.

Public conditional cancellation retained the trajectory at lifecycle/reducer/
watermark 3 with exact v0 terminal head and cancelled work/run. Final inventory
was thirteen terminal runs (five cancelled, five failed, three completed), zero
active; self-development remained OFF generation 0 and policy SHA-256 remained
`7192b8b1600561a331fda32f27628296c3f5b9bd1ba30dd5fb82681985c45e2a`.
Both setup keys were revoked and returned HTTP 401. A mistaken generic route call
also left empty non-lifecycle document `457320df-e047-405c-b2a1-a0263b4cb5dc`
with `current_version_number: 0`, `revision_count: 0`, and no trajectory/run; it
is retained and is not acceptance or a platform defect. The recurrence proves the
F/restart did not itself restore host ChatGPT auth; it does not justify policy permutation or credential bypass.


### Operator-ratified ChatGPT restoration

After docs-first ceremony `77be0419`/CI `31284157112`, a first auth-only install
was rolled back exactly because its bounded probe used a different owner's
legacy guest before provider. Receipt `8f55bb96`/CI `31284504546` landed that
failure before retry. The second attempt minted and nonce-verified the correct
acceptance-owner key to stable VM `vm-bbdbbd01c4390b7036067aaa12afeb68`, guest
`computer-42850e9734d9442386c5dd8bf3afbf19`, route `648d6071…`, epoch 8249,
and exact F.

The accepted root-only atomic transaction changed only Node B Codex auth
`eb1b7317…→cc744524…`, kept gateway env `7c5cc6e…`, and restarted only the
gateway `3806702→3807256`. Exact-F product probe document `c0956f24…`,
trajectory `2cbf6c95…`, work `f2e18cb7…`, and run `cb19cfff…` used real
`chatgpt/gpt-5.5` to create exact marker v1 `3de79895…` with completed work and
no error. The retained probe was publicly cancelled at lifecycle 4/reducer 5.
Final readback was 14 terminal/0 active runs, self-development OFF generation 0,
exact policy, exact env, and exact-F health. This repairs provider availability,
not the still-required full supervision acceptance. Root-only rollback remains
retained and the temporary key remains live only through that acceptance.

### First full restored-provider attempt — control refusal

After restoration receipt `86559e3d`/CI `31284908588`, exact-F preflight was
clean and the deployed CLI created document `82693dd5…`, v0 `dcbbcd84…`,
trajectory `14c99be0…`, work `22cc125b…`; correctly ordered CLI watch returned
exact durable start cursor 1. Real ChatGPT run `2ce06146…` then passivated with
no revision or child. Its public result says the attempted atomic Researcher
control was rejected by runtime schema and made the activation non-writable.
The run prompt separately says capsule execution is unavailable while effects
are OFF, contradicting the accepted guest-local capsule/no-host-effect boundary.
The trajectory remains live at lifecycle/reducer/watermark 1 with v0 and open
work. This problem receipt precedes any retry or fix.

### Control-refusal structural convergence

Problem receipt `76eed5bf`/CI `31285400390` preceded read-only source diagnosis.
Atomic controls advertise an opaque packet where runtime is strict; pre-commit
semantic error prematurely fails mutation despite bounded retry; and prompt
overlays forbid the already implemented atomic persistent-Super→assignment-
capsule path. Exact rejected args are not publicly projected. The red repair is
bound to shared explicit schema, pending mutation before commit, and precise
capsule-vs-host effects prompt authority, with strict validators and all
protected effects retained. No source mutation precedes ceremony landing.

### Deployed public CLI partial acceptance

A fresh scoped key exercised the checked-in `choir` CLI against retained
cancelled document `11902866-d32e-55c4-9483-d9bd47c91a6c` on exact deployed
`ac6dd16b`. `texture read`, `history`, `revisions`, and exact `show --revision
d1a831ba-6af5-5206-aa03-49caf4b047dc` returned the same document, trajectory,
and current v0 identity. Separate `watch --once --limit 2` processes proved
durable disconnect/resume across cursor pages `0→2→4→6→8→9`; resuming after
terminal cursor 9 returned an empty successful page. The sequence contained the
start, six owner-instruction events, cancellation request, and cancellation.

Authority negatives also used the public CLI: `tell` and `correct` against the
cancelled exact head both returned HTTP 409; a follow-up product snapshot remained cancelled at
watermark/lifecycle version 9 with the exact v0 head unchanged. Asking the
original document to show a revision belonging to another retained document was
rejected as not an exact version. The temporary key was revoked and then
returned HTTP 401. This is positive product proof for read/history/revisions,
exact current show, and resumable durable observation plus negative authority.
It is not positive correction, a generated historical v1+, or source-open proof;
those remain provider-blocked.


### Local source-only rollback rehearsal

A disposable detached worktree at exact runtime candidate `ac6dd16b` generated
and reverse-applied the binary diff for every non-document path changed since
`cdaa787b`. Mission documentation was intentionally retained. The scope
contained 99 source/test/script paths with ordered path-list SHA-256
`0b7eb4241e3dc5a70705ce596f436a372b5213593457c0c9b831c8c9296b22f3`; the
1,289,831-byte reverse patch has SHA-256
`64a61e5db159cf7d839532bad9a2a9d320e11a430e100a0ad6de2998b77530a8`.
After application, `git diff --name-only cdaa787b -- <scope>` returned zero
paths.

Focused `textureowner`, `toolregistry`, `types`, CLI, and capsule packages
passed, as did affected-package vet and `git diff --check`. A first parallel
runtime-shard run exposed one `context deadline exceeded` in
`TestCancelRunTrajectoryDrainsMoreThanOneActivePage`; the exact test passed
standalone, and the complete four runtime shards then passed sequentially. The
parallel failure is retained as resource-contention diagnostic evidence and is
not hidden as a pass.

This proves that the documented source rollback can be reconstructed cleanly and
returns the complete non-document candidate scope to the previously CI-passed
`cdaa787b` source while retaining evidence. It is local source-only rehearsal,
not a staging deployment, route rollback, or product rollback receipt. The
disposable worktree was removed; the patch and structured manifest remain under
`/tmp` for this session only and are reproducible from the two commit identities.


### Authenticated partial no-effect readback

The exact acceptance computer's self-development mode was `off`, generation 0,
before the first trajectory and remained `off`, generation 0, after provider,
policy, CLI, cancellation, and local rollback-rehearsal work. The computer-owned
model-policy bytes remained exactly SHA-256
`7192b8b1600561a331fda32f27628296c3f5b9bd1ba30dd5fb82681985c45e2a`.
Nonce-bound identities captured before and after the later probes retained the
same route digest, VM epoch 8247, and exact host/guest build
`ac6dd16b1980a1a3faedd7d1d83fefa79395a1ee`. The original cancelled trajectory
also remained at lifecycle version/watermark 9 with exact v0 head after the CLI
authority negatives. The scoped readback key was revoked and subsequently
returned HTTP 401.

This is admissible partial no-effect evidence for self-development mode,
computer policy bytes, route/build identity, and the retained lifecycle head. It
is not a complete protected-state comparison: no real capsule effect occurred,
and no canonical product readback for checkpoint state or the complete protected
event-head set was obtained. Those gates remain open.


### Deployed rollback preflight and refusal

A red-class preflight considered re-running historical successful deployment
run `31030833230` at `460c1423`, whose deployed runtime tree underlies
`cdaa787b`; exact-ac6 run `31261269488` was the proposed recovery. Authenticated
inventory found only one mutable active interactive computer (`primary`, epoch
8247), all twelve durable runs terminal, self-development OFF generation 0,
exact policy bytes, and the retained lifecycle snapshot unchanged at cancelled
watermark 9/v0. The preflight key was revoked and returned HTTP 401.

Independent deployment red-team nevertheless returned **DO NOT PROCEED**. A
rerun-all preserves the historical push event and exact checkout but also invokes
the sibling rolling-Flake publisher, produces a historical `main@460c` event-ref
receipt while current main is newer, rebuilds/restarts all Node B host services,
and refreshes every mutable active interactive computer. Exact-ac6 recovery has
normal CI/build latency. A job-scoped deploy rerun avoids the independent Flake
publisher but GitHub reruns the job with its dependencies, so it does not provide
a bounded deploy-only recovery window. Most importantly, the older runtime has
no restart/read receipt against the current persisted object graph.

No workflow was rerun and no deployment, route, checkpoint, computer, or product
state changed at this preflight checkpoint. Deployed rollback was then open; the
later canonical R/F receipt below supersedes that status. A safe rehearsal required either
a canonical current-main revert followed by a newly reviewed forward candidate,
or purpose-built bounded deploy-only authority with an exclusive change freeze,
complete affected-computer/protected-state inventory, and old-code proof against
a reconstructable current graph snapshot.


### Representative cross-version rollback compatibility

A disposable `ac6dd16b` Store fixture committed a terminal graph containing a
cancelled trajectory, two cancelled work items and run projections, a consumed
owner instruction, a Texture turn and two revisions, one exact-run-bound control
later cancelled with the trajectory, a durable cancellation intent, eight
lifecycle events, and seven command receipts. Its Dolt HEAD was
`lvtb74ss94q6u8jpmtd32707oefj2pu5` and `dolt_status` was empty.

Exact old runtime `460c142394e12b6e307949d0180da08d1b058745`, whose runtime
tree underlies `cdaa787b`, opened the same closed marker/workspace. Old
lifecycle/scoped snapshot, document, revision, update, work, and cancelled-run
reads passed. `Runtime.Start` used a counting dispatch hook and produced zero
actor dispatches. Dolt HEAD and clean status were identical before/after. A final
ac6 reopen verified the owner instruction, cancellation intent, exact control
binding/disposition, revisions, and every new-only object kind remained intact.
The old runtime's best-effort localhost Qdrant ensure failed closed without Store
mutation. Temporary probes/worktree were removed; session evidence remains at
`/tmp/choir-rollback-proof/`.

This proves representative additive-schema/unknown-object terminal startup and
non-wake behavior, including newer Texture-turn/control object kinds absent from
the blocked staging path; it is not a superset of the production graph or DB. It is not a production DB
copy, full service/VM proof, or deployed rollback receipt. There is no sanctioned
production Store export. A deployment rehearsal still requires fresh terminal-
only product preflight, current-main provenance, bounded forward recovery, exact
nonce identities, and before/mid/after protected-state comparison.


### Canonical two-leg deployment rollback ceremony

**Mutation class:** red. **Integration/deployment owner:** the current goal
session, exclusively, across both legs. **Conjecture delta:** a terminal ac6
persistent graph can restart under cdaa-equivalent source without semantic
mutation, then under restored ac6-equivalent source, while only deployment
identity, route certificate, and monotonic VM epoch change. The representative
cross-version fixture supports but does not prove this product claim.

**Protected surfaces:** current `origin/main`, CI and canonical rolling-Flake
publication, Node B service identities, vmctl and the stable guest lifecycle,
execution route/attestation, Texture lifecycle/events/revisions/controls,
self-development/materialization/checkpoint state, policy bytes, run acceptance,
and capsule/effect absence. **Admissible evidence:** exact 99-path tree equality
and digests; normal applicable CI/rolling/deploy receipts for new rollback commit
R and forward commit F; nonce-bound host/guest/route/platform identity at each
leg; and authenticated before/R/F protected-state comparisons. R is deployed as
its new commit identity, not mislabelled `cdaa`; F is likewise its new identity,
with ac6 source equivalence proved separately.

**Rollback/recovery:** freeze and review the forward inverse before R can deploy.
Any R CI, rolling, deploy, identity, or midpoint failure still restores canonical
main by pushing F through the normal landing loop; no historical rerun or old-
runtime mutation probe is allowed. Old-code exposure is bounded to 45 minutes
from R activation to F activation, after which failure to restore is immediately
escalated. The allowed midpoint deltas are deployment/build receipts, route
certificate/digest, service restart telemetry, and monotonic VM epoch only.
**Heresy delta:** `discovered: none; introduced: none expected; repaired: none`;
the operation closes an evidence gap, not a heresy. Effects remain OFF and
registries remain open.


### Canonical R/F execution receipt

The frozen reverse patch landed as new current-main rollback commit
`10d4865958b7d8deaab5665f74b37dd1b5005070` (R). Its index matched the exact
99-path ordered scope, every scoped mode/blob matched `cdaa787b`, no document
path changed, and independent review returned **PROCEED**. CI run [`31267448310`](https://github.com/choir-hip/go-choir/actions/runs/31267448310)
passed selected Race/build/SBOM gates, published the canonical rolling Flake, and
deployed exact R. Nonce-bound identity joined all host services, guest build,
route, deployment receipt, and platform attestation to R on the same stable VM
and guest at epoch 8248.

At the read-only midpoint the durable-run digest remained
`eb235251327268ec03909cd3c28d60b31a5edcca5f90b7b06a8cd074e8f84217`
with twelve terminal and zero active runs. All lifecycle summaries remained
cancelled at versions 9/3/3 with the same heads/work state; self-development
remained OFF generation 0; policy SHA-256 remained
`7192b8b1600561a331fda32f27628296c3f5b9bd1ba30dd5fb82681985c45e2a`;
and route, VM/guest, and single-computer identities were stable apart from the
allowed epoch. The full original-trajectory response digest differed because
R's legacy response omitted exactly six stored owner-instruction `request_id`
fields. No underlying event/head/work/run value changed, but strict forward
observability failed and the frozen identity-ambiguity abort rule fired. The
original trajectory full-response digest moved from
`2a4c429fa63b7dd33722e034f791bfe754a1270a732bfaada357e37fbc09e2e8` to
`dd5e25500a81dada79bd3400cce2064869aed641692a3c007d1f3e14ace260a9`;
the two v3 digests remained exact. F was initiated immediately, without
additional old-runtime probing.

Forward commit `67a61358ceda55c30e9853907f85648bb8531bb8` applied the identical
1,289,831-byte patch forward. Its whole tree
`d1a03e3e03f25d0ff201fd8d424b38549ccdb552` is exactly the pre-R
`2f8d912e` tree, independently proving restored ac6-equivalent runtime bytes and
retained current docs. CI run [`31268477380`](https://github.com/choir-hip/go-choir/actions/runs/31268477380) passed, published the rolling Flake,
and deployed exact F. R and F deployment jobs completed at `17:02:43Z` and
`17:34:32Z`, respectively: 1,909 seconds (31m49s), inside the 45-minute bound.
Final nonce-bound identity joined exact F on the same VM/guest at epoch 8249.

The sanitized comparator was:

| Comparator | Before | R | F |
| --- | --- | --- | --- |
| durable runs | `eb235251327268ec03909cd3c28d60b31a5edcca5f90b7b06a8cd074e8f84217` | same | same |
| retained v9 response | `2a4c429fa63b7dd33722e034f791bfe754a1270a732bfaada357e37fbc09e2e8` | `dd5e25500a81dada79bd3400cce2064869aed641692a3c007d1f3e14ace260a9` | baseline |
| ChatGPT v3 response | `196bbddebe3bd53d73f2a884446029ee42d94b04bef83c04bc45737866395172` | same | same |
| Z.AI v3 response | `292a5fe35086ef253a60eee86ca888ea3aaaf28520e87bf44435dacb6680f362` | same | same |
| policy | `7192b8b1600561a331fda32f27628296c3f5b9bd1ba30dd5fb82681985c45e2a` | same | same |
| route | `sha256:648d6071215206b190376ff6c24f3c93c08483b09bfb2ffc4790c00f3dd66489` | same | same |

Final full response digests for all three retained trajectories, the complete
run digest/counts, lifecycle summaries, policy bytes, self-development mode and
generation, route digest, and single-computer inventory exactly matched the
pre-R baseline; there were still zero active runs. Only new deployment/build receipts and monotonic epochs
8247→8248→8249 changed. Trees were R
`c87560fdf21c76c2b0840ec825c91459282d4c77` and F/pre-R
`d1a03e3e03f25d0ff201fd8d424b38549ccdb552`. The scoped acceptance key was revoked and
then returned HTTP 401. The qualified verdict is **bounded deployed rollback-and-recovery PASS; strict
midpoint forward-observability FAIL (safely recovered)**. This closes the prior
canonical deployed-rollback gap and strengthens the rollback/restore and Store-
preservation conjecture, but refutes strict backward observational equivalence.
Heresy delta: `discovered` — the legacy event projection omits forward-added
`request_id`; `introduced` — none durably; `repaired` — none. The compatibility
projection heresy remains unrepaired; F deployment/source restoration is the
recovery outcome, not heresy repair. It does not
supply the provider-blocked repeated Texture loop, real capsule/late-result,
positive correction/source-open, canonical checkpoint inventory, or run
acceptance.

Problem-documentation-first safety exception: the binding abort gate required F
to begin immediately when identity ambiguity appeared; pausing on old code to
commit a problem receipt would have prolonged protected exposure. F therefore
necessarily preceded the durable problem section, which is the first subsequent
commit and precedes any compatibility fix. No such fix is proposed here.
