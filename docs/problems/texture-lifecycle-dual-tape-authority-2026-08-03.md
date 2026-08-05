# Texture/Lifecycle Dual-Tape Authority

**Date:** 2026-08-03  
**Status:** observed; no runtime repair authorized by this record  
**Classification:** substrate — semantic event authority and projection  
**Mutation class of the exposing change:** none (read-only source reconciliation)  
**Mutation class of the planned repair:** red

## Problem

Choir's current source does not yet make Texture a projection of the canonical
per-computer audit log.

The trusted `ComputerEventAppender` is the sole sequencer for the canonical
`ComputerID` event chain, but it is called mainly by self-development capsule,
verification, decision, materialization, checkpoint, and rollback paths.
Texture and the generic durable-work kernel instead commit document revisions,
trajectory state, work items, update dispositions, command receipts, and a
separate stream of `LifecycleEvent` objects directly into the embedded object
graph.

Those embedded transactions are durable and reducer-owned, but they are not
derived from the canonical computer event chain. Choir therefore has two
overlapping causal tapes:

1. `ComputerEventAppender` + externally pinned events + corpusd head CAS; and
2. embedded-Dolt lifecycle events + directly committed Texture/lifecycle state.

The two paths currently describe different subsets of one computer's meaning.
That violates the vision's one-tape claim and prevents Texture from being the
faithful supervisory projection on which self-development can rest.

## Evidence

- `internal/computerevent/appender.go` implements pin, prepare, canonical head
  CAS, finalize, recovery, and reconstruction for the per-computer event chain.
- `internal/store/computer_events.go` stores only the embedded event index/head
  projection during `Prepare`/`Finalize`; it does not reduce Texture or durable
  lifecycle state from the event payload.
- `internal/store/lifecycle.go` constructs and commits Texture documents,
  revisions, trajectories, work, updates, `LifecycleEvent` objects, and command
  receipts directly in embedded Dolt.
- Production calls to `ComputerEventAppender.AppendNew*` are concentrated in
  `internal/agentcore` self-development/capsule paths; `internal/textureowner`
  calls `StartLifecycle` and `CommitLifecycleArtifactHead` directly.
- The completed durable-computer convergence Definition proved restart-safe
  lifecycle reducers with effects OFF. It explicitly did not prove
  self-development or one-tape Texture reconstruction.

Observed source identity: `794b99c9bf1526ee74a72fec8ba31e0c21df6d16`.

## Root-Cause Belief

The prior self-development mission built the protected computer-event spine
before the generic artifact/trajectory/work substrate converged. The convergence
mission then repaired durable work inside embedded Dolt while deliberately
leaving effects OFF. The two accepted pieces were never joined afterward.

This is a substrate problem, not a Texture UI symptom. Fixing individual Texture
flows without changing authority would make a more usable projection of the
wrong causal model.

## Existing Replacement Opportunity

The replacement already exists: the canonical `ComputerEventAppender`, its
content-addressed event artifacts, corpusd CAS, crash recovery, and reconstruction
path. The preferred repair is to route typed Texture/supervision transactions
through that spine and make the embedded Texture/lifecycle objects deterministic
reducer projections. It is not to add a third log or patch the independent
`LifecycleEvent` stream.

## Bounded Repair Contract

The proposed successor Definition is
[`choir-texture-tape-supervision-2026-08-03.md`](../definitions/choir-texture-tape-supervision-2026-08-03.md).
It must:

- preserve Texture's semantic authorship and single-writer authority while
  moving causal ordering/durability to the canonical computer tape;
- preserve the layered authority contract: Texture is the agentic owner-facing
  document author and human-side supervisor, Super is the operational
  supervisor, CoSupers are scoped workers, and Researchers provide sourced
  evidence;
- bind Texture's claims of work, evidence, decision, and completion to exact
  event references while keeping its main surface legible at human bandwidth;
- import existing state with an explicit, content-addressed migration receipt;
- reduce Texture, intent, work, evidence, decisions, and settlement from typed
  computer events;
- prove deterministic reconstruction and one material mid-trajectory intent
  revision through the public CLI and Texture product path;
- delete or derive the competing lifecycle-event authority; and
- keep self-development effects OFF.

No code mutation is authorized until that Definition is owner-ratified and all
three mission registries promote it.

## Belief State

- **Supported:** the two durable substrates exist and are not currently one
  reducer chain.
- **Supported:** the computer-event spine is the existing replacement to wire
  in, not a new store to invent.
- **Supported:** general Texture polish is downstream of the authority repair;
  only projection UX needed to prove supervision belongs in this mission.
- **Pending:** the exact closed transaction schema, migration boundary, and
  compatibility-floor rollback release require a frozen candidate and review.

## Remaining Error Field

This record does not establish that every Texture caller bypasses the canonical
tape, that all current event kinds are sufficient, or that the existing
computer-event reducer can atomically materialize the full supervision graph.
The Definition's first execution slice must inventory every production caller
and formalize the refinement from event payload to embedded projection before
implementation.

## Rollback

Not applicable to this evidence-only record. The eventual behavior change must
use an additive event schema, a frozen compatibility-floor release that can
read/reconstruct every emitted event, write disablement before rollback, and
nondestructive projection rebuild. Events are never deleted or rewritten.

## Heresy Delta

- `discovered`: H032, dual semantic tape for Texture and lifecycle state.
- `introduced`: none by this evidence record.
- `repaired`: none; discovery and Definition authoring are not repair.

## Frozen implementation review blockers — 2026-08-04

The first runtime candidate was frozen at base
`ecb43cfab6d5c206b62d4dada38843c2c46216bd` with staged binary-diff digest
`sha256:715854040d022f7a007b66ca9a21217ad33786eb8e3f18a23b27fb128c051804`.
An independent read-only review returned `REPAIR` with high confidence. The
successful reviewer was `openai-codex/gpt-5.6-sol`; three other panel routes
timed out after ten minutes and therefore supplied no verdict.

The candidate is not executable authority for these observed reasons:

1. Caller-side evidence/import payloads are randomly encrypted and pinned before
   the canonical command reservation. A crash between pin and reservation makes
   an exact retry generate a second ciphertext/ref and leaves the first pin
   orphaned. The appender's frozen transaction-envelope retry does not cover
   those referenced payloads.
2. `trajectory_settled` accepts Texture authority and requires only Super's
   proposal. The reducer has no fresh owner acceptance bound to the exact Super
   proposal, current semantic heads, and snapshot digest.
3. `revise_artifact` permits nullable semantic expectations, and the reducer
   does not require the submitted parent revision and fulfilled intent to equal
   the current heads. A stale owner command can therefore become canonical.
4. The configured twelve-event model cannot reach a three-assignment closed
   settlement trace; candidate verification is Super-authored rather than
   independently verified; and no reachability coverage proves the claimed
   settlement/promotion states were exercised.
5. The runtime exposes only assignment opening, attempt start/result, and
   message projection to Super. No production tool can author cancellation,
   disposition, belief, finding, dissent, reconciliation, decision proposal,
   or settlement proposal, so the promised product-path trajectory cannot reach
   the reducer's closed states even if the reducer is correct.
6. The global compatibility-floor write switch is process-environment-only and
   enabled by default. The current deployment path cannot first deploy a
   forward-readable disabled floor, rehearse refusal/rebuild, and then
   deliberately enable the first new supervision event.

These are substrate findings, not six independent symptoms: the candidate split
one supervision command across caller-owned pinning, appender reservation, and
an incomplete model-facing tool surface. Repair must move the complete logical
command—including referenced payload descriptors—behind one reservation/freeze
boundary, require dual fresh settlement authority, close semantic head checks,
make the full typed transition surface reachable, and land in disabled-floor
then enabled-release order.

Belief-state update: H032 remains discovered but unrepaired. The one-tape
appender/projection direction is still supported; the first implementation
candidate is rejected until the six blockers above have focused receipts and a
new frozen digest. Heresy delta remains `discovered=[H032]`,
`introduced=[]`, `repaired=[]`.


## Second frozen implementation review blockers — 2026-08-04

The repaired candidate was frozen against `d163a4aaa732e54ad56cbb7fc8a08d3aa8722268`
with tracked binary-diff digest
`sha256:a3b330278e0a4c63e4c652e4f8a9dd81f0a2a0d643e868dd6b01e8b61e3e6e78`.
Three independent read-only reviewers rejected it. Their findings are reliable
source evidence and precede any further repair:

1. Global write disablement still permits a first legacy trajectory write, so
   the compatibility-floor release is not actually quiescent.
2. The frozen transaction schema omits `referenced_artifacts` and declares the
   wrong digest recipe; the projection-import schema does not describe the DTOs
   emitted by the importer.
3. The formal rebuild copies the live fingerprint rather than folding tape
   history, and the branching safety bound cannot reach the protected states
   whose invariants it claims.
4. Updater admission trusts manifest literals and a shallow health identity
   instead of requiring the staged/restored reader to replay the signed private
   tape and attest semantic equivalence.
5. Ordinary boot reconstruction cannot hydrate private supervision
   transactions, so a replacement realization with an empty or behind
   projection cannot boot after the first supervision event.
6. Projection import retries do not recover an already accepted command and
   regenerate time-bearing evidence after an incomplete reservation, causing
   conflicts instead of returning the original result.
7. Store preparation inserts the event and binds its command reservation in
   separate commits, leaving a crash/error state that can advance corpusd while
   never finalizing locally.
8. The reducer conflates a trajectory-local cursor with the global computer
   head, so unrelated interleaved events make the next command stale and make
   replay fail.
9. The Super transition tool and owner command endpoint cannot reserve and pin
   the private artifacts required by their own mutation vocabulary. Fresh
   cancellation, disposition, decision, acceptance, settlement, and archive
   commands are therefore unreachable or violate reservation-before-pinning.
10. Runtime CoSuper dispatch always emits an initial ordinal-one attempt and
    authorizes against a bounded eight-item owner view; retries and assignments
    beyond the display bound cannot execute.
11. Operational status and reconciliation disposition share one state field,
    so an open assignment or attempt can be made settlement-ready without
    terminating. A late result also leaves its attempt's older disposition
    current.
12. Material rebase does not validate its state digest or invalidate affected
    target dispositions, allowing pre-revision reconciliation to settle a
    post-revision trajectory.
13. Assignment opening does not resolve its parent Super decision; settlement
    evidence refs need not resolve to retained evidence; and owner-reserved
    decisions do not create owner-attention obligations.
14. Initial assignments are hidden from the owner obligation projection even
    though they block settlement.
15. Archive projection overwrites canonical document fields, while the ordinary
    owner DELETE route cannot reach canonical archive authority.

These are one authority-closure cluster, not isolated endpoint bugs. The next
candidate must make reservation, private pinning, append, global sequencing,
trajectory reduction, operational closure, semantic rebase, owner attention,
settlement evidence, replay, updater admission, and UI/API reachability one
coherent contract. H032 remains `discovered` and unrepaired; no reviewer finding
is counted as repair.

## Third frozen implementation review blockers — 2026-08-04

The next repaired candidate was frozen against
`6dd0072fb3daf85a077c97fea2114f9dcf515147` with complete tracked-and-untracked
content digest
`sha256:289580c58dca44ef348adf1c20345d7dc9f8101e993b963365568a9d1c66ebb1`.
Three independent read-only reviewers rejected it. A separate four-model panel
timed out before returning verdicts and is reviewer-health evidence only.

The candidate remains non-executable for these observed reasons:

1. Empty-tape boot reconstruction passes a nil canonical head to the store
   rebuild, while the rebuild validator requires an explicit sequence-zero
   head. A fresh computer therefore fails its unconditional startup replay.
2. Projection import reserves the command before durably freezing the complete
   import inputs. A crash in that interval leaves a reserved command with no
   recoverable frozen plan, and retry cannot regenerate the time-bearing input
   after write-disable state changes.
3. The supervision snapshot still exposes a trajectory-local canonical head as
   the next global tape expectation. An unrelated event for trajectory B makes
   the next valid command for trajectory A stale.
4. Material intent rebase validates affected targets only through status-backed
   entities. Artifact-premise and belief targets required by the contract have
   no accepted state-digest path and are rejected as unknown.
5. Settlement evidence validates only artifact-reference syntax. A fabricated
   digest can support settlement without resolving to retained or pinned
   evidence on the canonical tape.
6. The Super-only `product_api_request` tool broadly allows `/api/texture/*`
   and injects the run owner's authenticated-user header. It can therefore call
   the owner command endpoint and synthesize owner-authored decision or
   settlement authority without owner presence.

These findings remain the same authority-closure cluster: global tape position
must be distinct from trajectory semantic base; every entropy-bearing command
must freeze before entropy or become exactly recoverable; replay must define
the empty state; rebase and settlement references must resolve against
canonical retained state; and owner authority must be non-delegable through
agent product tools. H032 remains `discovered` and unrepaired. No runtime repair
may be counted until a new frozen candidate clears these exact sequences.

## Accepted local implementation candidate — 2026-08-04

The fourth candidate was frozen against
`a35236a46b99ba955b6d7e4b71311ea02cf210e1` with complete content digest
`sha256:3f1a3bdd4ed9e4ac00e61b45014d61fc1c134bd937637468dd13a1206fd979cf`
and committed locally as `f5c4c43e17e3e9b2e6de71170049695361c224bb`.
AppenderPrivacyReview, ReducerAuthorityReview, and CompatibilityProofReview each
accepted that exact manifest after focused rechecks.

The candidate repairs the six third-round sequences and two additional
crash/stale-base sequences found during the final recheck:

1. explicit sequence-zero tape replay boots without inventing a genesis event;
2. entropy-bearing private inputs and import inputs are frozen atomically with
   command reservation before any pin or mutable barrier observation;
3. the global canonical tape head is distinct from each trajectory's observed
   semantic base;
4. belief and artifact-premise rebase targets resolve exact current-state
   digests, and each target must bind the current prior intent before mutation;
5. settlement evidence must resolve to the retained referenced-artifact
   registry populated from verified transaction bindings;
6. Super product API tooling refuses every normalized owner supervision command,
   import, and rebuild route before authenticated-owner header injection;
7. rebuild rewinds a locally prepared but canonically absent supervision command
   to its pinned frozen plan so restart can retry the exact event; and
8. replay, retry, cancellation/late delivery, rebase, settlement, promotion,
   startup attestation, disabled-writer, and owner-authority focused checks pass.

This is local repair evidence, not deployed acceptance. HTTPS push failed
because the configured GitHub token is invalid; SSH authenticated as
`yusefmosiah` but `choir-hip/go-choir` denied write permission. H032 therefore
remains `discovered`; its implementation is accepted locally, but `repaired`
remains pending origin/main, CI, exact staging identity, deployed product-path
acceptance, rollback, and terminal registry closure.

## Deployed product-path blockers — 2026-08-04

The accepted implementation reached `origin/main` as
`248e4692595534df9843dff37a00a4146f3d570f`; CI run `30947754942`
completed successfully, and `choir.news` reported that exact proxy, sandbox,
and vmctl identity. A fresh passkey owner
(`4f9662ea-51d5-48d2-90de-71b734d40e5b`) then exercised the prompt/Texture
product path on computer `vm-58e28a39cda64651f8bca7e9ac2efc52`.

That deployed proof exposed two new blockers before settlement:

1. Texture run `4f1014e5-3fc3-494b-a2db-8b3a7c1f7578` failed its first
   provider call because the staging ChatGPT refresh token had already been
   consumed. The owner changed only this disposable computer's
   `System/model-policy.toml` Texture and Super roles to the already configured
   `deepseek/deepseek-v4-flash`, preserving the exact original file at
   `System/model-policy.texture-tape-acceptance-backup.toml`. Texture run
   `03de15ce-77ad-4eb3-b633-468f4d43e8ee` resolved and recorded DeepSeek in its
   run metadata, but its post-tool iteration still called the ChatGPT gateway
   and failed with the same `refresh_token_reused` response. Model selection
   therefore does not remain authoritative across a multi-turn tool loop.
2. Source inspection of the exact deployed candidate found a prior authority
   guard in `update_coagent`: once the caller has a durable lifecycle run,
   `newUpdateCoagentTool` returns `ErrSupervisionAuthorityRequired` before it
   evaluates the supervised-trajectory branch that appends the closed
   transaction. Every prompt-bar Texture trajectory has that lifecycle
   binding. Texture therefore cannot hand an `execution_request` to persistent
   Super through the documented product tool, so no deployed three-way
   assignment fan-out can start even with a healthy provider.

These are product-path failures, not a weaker test gap. The next repair must
first make closed supervised `update_coagent` handling precede legacy-lifecycle
refusal while keeping unsupervised lifecycle writes refused, then prove the
model-policy provider remains fixed across every tool-loop iteration. The
admissible evidence class remains a fresh deployed prompt/Texture/Super run on
the exact staging commit, followed by owner settlement; local tests alone
cannot close either blocker.

Mutation class is `red`. Protected surfaces are the canonical supervision
appender, persistent Super handoff, provider/model routing, and run acceptance.
Rollback is source revert plus restoration of the disposable owner's exact
model-policy backup. Conjecture delta: “the accepted local candidate is
product-path complete” is falsified; the one-tape reducer/appender conjecture
remains supported by its accepted local evidence. Heresy delta:
`discovered` = supervised handoff guard precedence and tool-loop provider
authority loss; `introduced` = none by this observation; `repaired` = none.

## Deployed handoff root-cause cluster — 2026-08-04

The first problem checkpoint identified the observed refusal but understated
the substrate gap. Reinspection of the exact deployed source and the failed
Texture run establishes one connected authority/delivery cluster:

1. `newUpdateCoagentTool` discovers the lifecycle-bound run and returns
   `ErrSupervisionAuthorityRequired` before checking whether that same
   trajectory has the canonical supervision projection. The closed append
   branch is therefore unreachable for every supervised Texture run.
2. `resolveCoagentFindingsTarget` resolves a run's Texture channel before an
   explicit target actor. A Texture `execution_request` that names persistent
   Super is consequently rewritten to the current Texture actor.
3. `appendSupervisedUpdate` records `actor_message_recorded` on the canonical
   tape, but persistent-Super cold wake and tool-loop injection still read the
   legacy worker-update mailbox. No consumer projects the canonical private
   packet back into an addressed actor turn, and no restart sweep can recover
   the post-append/pre-dispatch gap.

These are not three independent edge cases. The replacement one-tape writer
exists, but the product handoff still enters and exits through the superseded
mailbox authority. Repair must connect the closed transaction to an addressed,
idempotent, restart-recoverable actor delivery projection derived from the
canonical message/result/research artifact. It must not restore a lifecycle
writer or treat actor delivery state as semantic authority.

The deployed provider receipt remains a separate continuity blocker: the
DeepSeek-selected Texture completed its first tool turn, then the fallback
chain surfaced the invalid ChatGPT credential on the next provider turn.
The next proof must expose the selected and fallback provider attempts rather
than infer the failed primary from the terminal fallback error.

Mutation class for the repair remains `red`. Protected surfaces are closed
supervision append, private-artifact recovery, actor delivery, persistent Super
wake, model/provider routing, and run acceptance. Admissible evidence is a
focused restart regression plus a fresh exact-commit deployed
Texture-to-Super-to-three-CoSuper trajectory and owner settlement. Rollback is
source revert and restoration of the disposable staging model-policy backup.
Heresy delta: `discovered` adds the explicit-target rewrite and missing
canonical delivery consumer; `introduced` remains none by observation;
`repaired` remains none.

## Fourth frozen implementation review blockers — 2026-08-04

The delivery/privacy repair candidate was frozen against
`215775d467d5ae876ab54e38f9bea790ad347a89` with tracked binary-diff digest
`sha256:17a35ec073e22df58f8f9b1b5ea8ab5c635d554fee481ce2d9b4101909b4fc77`.
Four independent read-only reviewers rejected it before landing. Their observed
failures precede any further repair:

1. Generic cold and warm compatibility paths can use legacy mailbox rows when
   canonical appender/private-artifact authority is unavailable, rather than
   proving the canonical delivery set is empty.
2. Super and generic cold compatibility wakes persist packet plaintext in the
   run prompt and mark it pre-injected, bypassing private-envelope taint and
   exposing literals to provider-call Trace and terminal run surfaces.
3. Canonical-first selection still reads compatibility storage eagerly, so an
   irrelevant legacy read/decode failure can block an available canonical
   Texture or generic delivery.
4. A post-commit actor dispatch failure has no live-process retry, and restart
   recovery can strand a persisted initial activation by redispatching only a
   coagent wake rather than the stable initial activation.
5. Role-addressed CoSuper cold recovery creates a run without assignment and
   attempt authority, so its supervised result cannot be returned.
6. Completed derived runs treat every injected delivery ID as consumed without
   a canonical acknowledgement/disposition mutation, allowing one undisposed
   result to disappear from delivery while still blocking settlement.
7. Canonical Texture wakes mark every legacy mutation stale but the edit path
   still requires that legacy mutation, so the owner-visible revision cannot be
   committed.
8. Texture source extraction is not scoped to the activated trajectory and can
   cite a pending delivery from another trajectory for the same actor.
9. Private tool-result Trace records only the provider-visible capped output
   digest/length for ordinary tools, not the full redacted output digest/length.

This is one remaining authority-and-privacy cutover cluster. The next candidate
must make canonical emptiness a prerequisite for every compatibility read;
route all packets through one private-tainted injection path; bind dispatch,
activation, acknowledgement, Texture edits, source projection, and CoSuper
assignment authority to the same canonical delivery; and make every
post-commit failure restart- and live-recoverable. Mutation class remains
`red`. Protected surfaces remain canonical supervision append/reduction,
private artifacts and Trace, actor delivery/restart, Texture revision authority,
and settlement. Rollback is source revert before deployment. H032 remains
`discovered`; `introduced=[]`, `repaired=[]`.


## Compatibility-floor deployment and control gap — 2026-08-05

Forced staging run `30971525939` built source
`8cab71b2c4ee3fc0484675830a3a46767a678039` after every selected CI gate
passed. The deploy reached the host: public proxy `/health` reported that exact
`build.commit`. It did not reach one accepted host/guest/computer identity.
Refresh of active mutable computer
`vm-58e28a39cda64651f8bca7e9ac2efc52` timed out after 300 seconds. The
workflow projected that ownership `failed`, observed the acceptance computer
still serving sandbox commit `794b99c9bf1526ee74a72fec8ba31e0c21df6d16`,
refused the activation receipt, and retained
`/var/lib/go-choir/deploy-failures/30971525939-1.json`.

The failed ownership is no longer in the active-only refresh set. A retry is
therefore the smallest safe probe; mixed host/guest commits remain rejected
until the workflow publishes one exact activation receipt.
Deployment diagnostics made the immediate boot refusal explicit: the refreshed
guest repeatedly opened its persistent store and then exited because
`/mnt/persistent/choir-updater/current` did not exist. Existing ordinary
computers predate self-development baseline import, so requiring that mutable
updater pointer before the sandbox can boot creates a circular prerequisite.
The compatibility-floor repair must prefer an authenticated current updater
release when present, but otherwise bind startup replay to the immutable guest
image manifest already in the booted Nix closure. It must not fabricate or
import a mutable baseline merely to make startup succeed.


The attempt also exposed a separate cutover-control gap. The compatibility
floor hard-sets `CHOIR_SUPERVISION_WRITES_DISABLED=1` in the immutable guest
systemd service. The required global switch exists and fails closed, but the
same exact release has no tracked platform configuration path that can boot
guests disabled for the floor, enabled for cutover, disabled for rollback
rehearsal, and enabled again while binding each state to deployment and guest
health receipts. Removing the hard-set environment line would enable writes but
would not supply the required exact-release rollback control.

Mutation class remains `red`. Protected surfaces are deployment routing, active
VM refresh, guest boot configuration, and the canonical supervision write
gate. Admissible repair evidence is a successful exact-identity floor retry,
then same-release disabled/enabled/disabled/enabled deploy receipts whose
refreshed guest health reports the expected switch state. Rollback is the
fail-closed default plus the prior activation receipt; no canonical event may be
deleted or rewritten. Conjecture delta: a platform-owned tri-state deployment
input (`preserve`, `disabled`, `enabled`) can control a boot-time guest override
without changing the release artifact. Heresy delta: `discovered` adds the
mixed-identity refresh failure and absent tracked same-release switch control;
`introduced=[]`; `repaired=[]`.

## Explicit enable rejected without mutable authority — 2026-08-05

Source `d69e1a6f7e89e71bb7457f5a641e96c1e9c34e80` landed through successful
CI run `30975427662` and deploy job `92210016957`. Its preserve-mode activation
receipt bound the host, installed sandbox package, and all selected host
services to that exact commit while recording
`supervision_writes_disabled=true`, `guest_health_verified=false`, and
`active_computers.status=empty`.

The existing public lifecycle route then reported legacy
`computer-03335285269bdba4f94377e56879f9e6` active, accepted a signed stop/start
sequence, and advanced its realization epoch from 129 to 130. That did not make
the computer part of vmctl's active mutable ownership authority: forced
same-release enable run `30976735765` preserved the one active constructed
ComputerVersion, found no mutable active computer, and rejected activation with
`Explicit supervision write mode lacks a refreshed mutable guest health proof`.
Its EXIT compensation forced disabled mode, restarted vmctl, found no mutable
writer requiring refresh or stop, and retained
`/var/lib/go-choir/deploy-failures/30976735765-1.json`.

This is now a distinct authority mismatch, not a write-gate failure. A signed
public lifecycle receipt can describe a legacy computer as active while the
vmctl ownership registry has no corresponding active mutable realization.
Neither that receipt nor a constructed ComputerVersion may be relabeled as the
mutable guest proof. The next probe must use an existing no-SSH product/control
path to create or reactivate one disposable mutable vmctl ownership, or add that
missing operator path only after separate source convergence and review. The
write gate remains fail-closed and no supervision event was emitted.

Mutation class remains `red`; protected surfaces are lifecycle authority,
vmctl ownership, deployment mode, and guest boot identity. Rollback succeeded
to disabled mode. Heresy delta: `discovered` adds contradictory public
lifecycle versus vmctl-active authority; `introduced=[]`; `repaired=[]`.