# The Eleven Missions: From Restore to Self-Direction

Restore-zero through native goals: what each mission is, why it sits where it sits, and what done
means. Awaiting owner approval before missions 1-4 are drafted.

Provenance: Markdown rendering of `choir-rlm-missions-overview-2026-09-09.pdf` (iCloud Choir
Reports, 2026-09-09). Research record, not mission authority. Mission state lives in
[`ACTIVE.md`](../ACTIVE.md) and [`mission-graph.yaml`](../mission-graph.yaml); this document
supplies the mission stack's scope and ordering rationale.

## How we got here

Choir's autonomous execution harness is mid-transition from legacy JSON tool calling, where the
model choreographs every file read and command as a separate network round trip, to orchestration
as code, where the model writes Go inside a persistent interpreter in an isolated capsule and the
supervisor side reduces staged intents into durable truth. The target-architecture cutover proved
execution on staging hardware: a sealed agent ran a Go cell to exit zero, wrote its proof file, and
earned a signed receipt. But the assignment never closed its own fate. A retried terminal report
arrived under a fresh provider call identifier, the store saw one command identity with two
digests, and the run's acceptance was explicitly withheld. That single conflict named the entire
mission stack below: the settlement path, the restore path it leans on, the vocabulary that must
survive both, the desks that must cross to the new carrier one by one, and the goal machinery that
can only begin once all of it holds.

The order that follows was settled by the owner, pressure-tested across three consensus rounds
with a recovered adversarial review, and recorded with dissent in the mission-state report. Its
logic is narrow. Foundations first, because everything above inherits their depth. The rename
before any desk writes new vocabulary, because migration after the fact is rework. One desk at a
time across the carrier, because shared authority surfaces cannot be debugged in parallel.
Latency's diet before its contract change, because hidden waste must never be disguised as
architecture. Evaluation in shadow before anything it measures becomes promotion authority. And
goals last of all, because a selector is only as trustworthy as the settlement, restore, and
verification it stands on.

## Preparation: model access before hill-climbing

Before any mission that measures rather than builds, the additional model access gets wired so
that cross-model comparison is never gated on credential plumbing later. This is deliberately the
only preparatory act: no evaluation runs, no promotion weight, no substrate changes. The
sequential trial-and-error posture with current models remains the default through the substrate
missions; breadth arrives exactly when the shadow evaluations need it, and not before.

## Mission 0 — Restore-zero: the recovery object, redefined

Every mission below assumes a computer can be brought back, and today that assumption is weaker
than it looks. Restore currently opens an empty store and replays the entire event tape from
genesis, consulting no projection base along the way; the only base consumer is boot-time
materialization, which fails silently back into full replay. Wall time therefore scales with the
age of the computer rather than the size of what changed, and any vocabulary migration would have
to survive exactly that full-depth reconstruction.

Restore-zero replaces the object itself: a verified snapshot or projection base at a watermark,
plus the immutable tail of events after it, with the canonical event head as the restore address
and a materialized witness as independent proof. The base is a verified accelerator, never a
parallel authority; anything missing, foreign, corrupt, or incompatible fails loudly instead of
falling back to genesis; the final head and witness verify before anything is published;
interruption resumes from durable progress rather than restarting at the beginning. The
current-head-only recovery path stays non-rewinding throughout, historic tape stays decodable
under frozen versioned rules, and the base contract carries a vocabulary version from the start so
that bases emitted before the rename stay interpretable after it.

The mission's governing aesthetic, set at chartering, is simplification over addition: existing
restore surfaces are extended or repaired and the flawed patterns deleted, with new modules
admitted only where no living surface can carry the contract. The end state should read as how the
substrate should have been all along.

## Mission 1 — The settlement gate: making finishes true

This is the mission that retires the withheld acceptance. Its work divides into five interlocking
repairs that share one theme: a finish must be a single durable truth, not a race between
transports.

First, terminal semantic identity is scrubbed of everything transient. Provider call identifiers,
retry ordinals, batch positions, and delivery receipts leave the report identifier and fingerprint,
so a retry under a new call is recognized as the same semantic command and handed its original
receipt, while genuinely changed content conflicts deterministically. Second, the failure taxonomy
splits in two: compile rejections, proven pre-execution, return structured diagnostics while
preserving the heap and imports; panics, timeouts, overflows, and worker deaths poison and respawn
over the same durable snapshot. The cut travels as an explicit error class from a genuinely
non-executing checker through every layer that currently treats all errors as fatal, and string
matching is disqualified as its carrier. Third, the one-shot execution path is unified into the
persistent session with disposal as its single-cell case, deleting the fallback that buries exact
diagnostics under generic wait errors. Fourth, capsule fate becomes a fenced, crash-resumable
intent-and-ack sequence rather than an assumed atomic transaction, with same-semantic replay
idempotent, changed content conflicting, and cancellation authoritative. Fifth, the narrow
singleton terminal detector that caused the original failure is replaced by serialize-or-reject
handling of mixed consequential batches, where any accepted terminal disposition closes the loop
and rejections never count as settlement — and the researcher fallback's write port is closed so
it explains and repackages rather than commits.

Mission one may rehearse beside mission zero, but their shared files, deployment, route, and
registry mutations serialize, and the rename waits on both missions' deployed acceptance.

## Mission 2 — The versioned rename: new words, old evidence

With restore bounded and settlement true, the desks receive their working names: engineering,
management, research. These are protocol values, not labels — they live in profiles, capabilities,
verb sets, session selection, reducer aliases, prompts, persisted runs, lifecycle records,
assignments, mailbox destinations, digest domains, signed grants, and evidence identifiers — so the
migration is a protocol cutover with a replay decoder, never a find-and-replace. Writers from the
cutover emit only the new vocabulary while historic tape keeps decoding under frozen original rules
with original bytes preserved; unknown or unversioned values are rejected at activation, with
special attention to the strict-equality check that today grants broad authority to anything that
is not exactly the old researcher string. Display names for computers travel alongside this mission
as a parallel UI field while canonical identifiers stay untouched in every route, signature, and
replay contract. Landing early, right after the two gates, puts the rename at the narrowest point
where a replay regression can be identified without also debugging a carrier rewrite.

## Mission 3 — Engineering crosses first, Texture designs beside it

Engineering is the first desk to live entirely on the new carrier because it owns the shortest path
to retiring the residual terminal machinery: the evaluation primitive becomes the desk's sole JSON
envelope while every other affordance — files, execution, messaging, spawning, completion,
verification — becomes a typed in-cell function staging intents for the one reducer, and the old
paths are deleted rather than hidden. Each retired operation earns its deletion by proving replay
returns the original receipt through the new path. Running in parallel but landing after, Texture's
packet and reducer design translates documents, patches, diffs, source graphs, controls, and
dispositions into the same in-cell discipline, specified against the canonical writer's
non-negotiable invariants rather than around them. Read-only observations may remain synchronous
typed functions; nothing forces them through a needless two-turn protocol.

## Mission 4 — Texture lands on its own invariants

Texture's landing is gated on proof, not on schedule: the compiled reducer must preserve
single-writer discipline, stale-base comparison, atomic revision-graph-identity commits,
retry-preserved pending mutations, versioned compare-and-swap, fresh-but-not-replay wakes,
per-document locking, and atomic researcher opening — every one of them, under the new carrier,
before the old JSON writing tools disappear. This is deliberately the most conservative landing in
the stack, because the canonical document is the artifact the user actually sees and a
serialization regression here corrupts truth rather than merely delaying it.

## Missions 5 and 6 — Research, then Management, with the routing invariant

Research follows onto the packet discipline once Texture has landed on it: maximally restricted
authority to begin, a no-search first revision that usefully proves the search gate stayed shut, and
the search-to-checkpoint cadence as durable contract once search exists. A researcher that
orchestrates its own retrieval in code can then do what no per-turn cadence can express — fan out,
assess coverage, deepen where evidence runs thin, stop where returns diminish — and checkpoint once,
at cell success, with the whole expedition behind the packet.

Management comes after as a host-side activation, never an effects-capsule copy of engineering, and
it carries the routing invariant the whole topology depends on: Texture faces the user and never
reaches past Management for machinery, speaking document-and-obligation downhill while Management
speaks ticket-and-attempt uphill to Engineering. Texture observes unanswered requests through the
reducer and re-issues identical semantic identities or hands obligations to Management; it never
learns what a capsule is. This is the pattern that survives scale, and the World Wire will need it
far more than it needs fast lanes between every pair of desks.

## Mission 7 — The prompt-bar diet, then the contract

The twenty-second prompt-bar path is profiled as a waste bug before anything about it is redesigned:
per-step roundtrips, redundant durable writes across trajectory, run, audit, revision, submitted,
and started events, snapshot and budget fan-out, object-graph commits, bytes and commits per input
byte, and the synchronous media fetch hiding in run preparation. Admission collapses to a bounded
idempotent seed command and the redundant materialization goes; only then may genuinely
nonessential work move behind the response, with the response narrowed to a submission identity
carried by a durable state machine that the command line and the browser consume identically. The
stale stream banner, the competing test contracts, and the media-import deferral all travel with
this repair. Asynchrony that hides unchanged appetite is explicitly disqualified as an outcome.

## Mission 8 — The continuation census: every write needs a reader

Before anything claims the old continuation tables as goal machinery, a bidirectional audit traces
every record from production routes, tools, and profiles through writers, records, readers, and
resulting effects, and from every tape and schema object through replay, projection, acceptance,
and API consumers. Each occurrence sorts into live consumer, versioned replay input, or deletion —
the decision and runtime layers are already gone, and the residue of stores, schemas, graph
registrations, and allowlists is migration input or removal, never autonomous authority by name.
The read-only audit may start early; the deletions resolve before anything builds atop the tables.

## Mission 9 — Shadow evaluations: measurement without authority

With the carrier proven and restore bounded, evaluation begins in a form that cannot promote
anything: content-addressed evidence packets carrying candidate, baseline, verifier, policy, model,
cost, and timeout identities, with stored outcomes replayable without calling any provider,
disagreement and unavailable-provider states as first-class results, and live-search proof that
distinguishes retrieved evidence from model assertion. Cross-model access arrives here, in shadow
mode against the same stored packets. Nothing measured becomes promotion authority until replay can
consume stored evaluator outcomes without provider calls.

## Mission 10 — Native goals: the selector, at last

The final mission restores what was removed years ago in modern form: a goal as observed start,
target, done-state, verifier contract, authority boundary, rollback condition, and compact proof,
expressed in the vocabulary of missions, campaigns, sweeps, and cycles. Its smallest proof is
deliberately one lineage — a rejected evidence-deficient packet, researcher retrieval with query and
digest and span receipts, corrected settlement, stored verifier and freeze state with a bearing
revision, gradient comparison across attempts, one bounded re-entry, provider-free replay
reproducing every receipt and decision, and a cancellation branch proving revoke wins.
Natural-language evidence enters texture revisions as bearing revisions rather than telemetry
exhaust; rematerialization makes bad changes recoverable without measuring anything. Styleguide
control, itself another texture document, and any report-formatting skill wait until the revision
pipeline they would govern exists.

## What done looks like

Done is not eleven checkmarks but a single system that reads as though it had always been this way:
restores bounded in the tail, finishes that are each one durable truth, desks speaking one carrier
apiece under versioned names, a user surface that never leaks machinery, a prompt bar that answers
instantly, no dead tables masquerading as machinery, evaluations that measure without ruling, and a
selector that can finally choose the next objective on evidence. The missions are ordered so that
each of those properties is proven before the next one leans on it — and the first proof begins
with restore-zero, whose charter awaits approval.
