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