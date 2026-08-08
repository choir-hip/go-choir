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
`2026-08-08T13:12:35Z` names exact commit `99fc3e6b` for sandbox, active
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
`computer-42850e9734d9442386c5dd8bf3afbf19`, VM epoch 8247, sandbox and host
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
pending controls, late evidence, no-effect comparison, rollback rehearsal, run
acceptance, and registry closure remain open.


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
`insufficient_balance`, Fireworks HTTP 412 `PRECONDITION_FAILED`, and Z.AI HTTP
429 code `1113` with a balance/rate classification. This shows every configured provider/model route failed before tool semantics;
account/balance attribution applies to DeepSeek/Xiaomi and the qualified Z.AI
classification, not the Fireworks precondition failure. Local Codex token
metadata reports an unexpired expiry, which does not prove usable auth, while
the host gateway retains stale auth.

GitHub authority inspection found only the Node B SSH host/key Actions secrets,
no provider secret, no repository environment, and no repository variable.
Repository inspection found no scoped product API or `choir` CLI provider-auth
renewal. The existing credential deployment and recovery workflows are
SSH-shaped break-glass paths forbidden by this acceptance. No credential or
authority was changed. The remaining transition is provider account restoration
or a separately ratified scoped no-SSH host renewal authority.


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
