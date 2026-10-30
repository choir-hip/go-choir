# Automatic checkpoints, and an apply without a replay (2026-10-26)

Status: read-only investigation and design, for the owner. Green. No code is
proposed to land from this document until the owner rules. The lane's earlier
fix commits on `lane/apply-replay-bound` (7f3a8e1c2 tests, 131567413 fix) bound
the replay (scratch on the data volume, admission, in-flight watch, stale-root
removal at start, a named hold); they are kept as the guard for the manual
replay paths, not proposed as the answer.
Problem record: [apply-checkpoint-replay-fills-guest-tmp-2026-10-26](../problems/apply-checkpoint-replay-fills-guest-tmp-2026-10-26.md).

## The owner's questions, answered first

**"Why do we need to be replaying stuff to apply anyway?"** We do not. The
checkpoint stores the **live** store's content hash. The replay produces
nothing the checkpoint keeps. It only checks, at apply time, that the tape
would rebuild the same content. Restore already re-runs that check whenever it
is used, and refuses on a mismatch. So each apply copied the whole store into
the guest (and on b@c, crashed the computer) for an early warning that restore
enforces anyway (section 1).

**"How come b@c is running whole tape replay rather than from a checkpoint?
We set up checkpoints months ago, no?"** We did, and it does start from one,
but from the wrong kind, and that one was stale (section 2). The probe never
replays from genesis on a computer this size (it refuses chains over 10,000
events without a base). It starts from the newest **projection base**, the
automatic host-built snapshot checkpointd publishes every 2,500 events. It
installs that whole base (a full copy of the store, twice at peak) and replays
the events since. On b@c the base was at least 8k events behind (the slow-page
log shows tail pages from `after=36377`), against a cadence that should keep it
within about 2,500. So two things are wrong: the apply copies a whole snapshot
every time, and b@c's automatic snapshots had stopped advancing.

**"We shouldn't have to bind checkpoints manually."** Agreed; section 4 makes
the checkpoint automatic (every N events or T minutes), incremental from the
previous one, with the proof on the host. An apply then needs only a cheap
content-addressed bind at its own head.

## 1. What the apply's bind is for

### The chain and what it computes

`recordMaterializationApplied` (`self_development_materializer.go:586`) appends
the `materialization-applied` event (`:596-618`), signs the verifier
certificate, and calls `bindCheckpointAtHead`
(`checkpoint_restore_bindings.go:53`), which calls `openReplayProbe`
(`replay_completeness.go:252`). The probe:

1. holds appends and extracts the live Dolt state at head H1, every row of
   every table hashed (`computerversion.DoltStateExtractor`; `SELECT *` per
   table, `dolt_state_extractor.go:389`);
2. installs the newest advertised projection base into scratch and replays
   the tape from it to H1;
3. extracts the replayed state, checks `live == replay`, and computes
   eligibility plus run-memory and object-graph diagnostics.

The bind then holds appends again, advances the replay by the tail desks wrote
meanwhile, re-checks at the bound head H, pins the base the replay started from
under the checkpoint's reference (`:101`), and publishes.

The published witness is `WitnessFromObservationSets(live, replay, result)`
(`selfdevprotocol/restore.go:142-179`): it refuses unless the two sets are
equal, then **returns the live witness**. `WitnessFromObservationSet(live)`
(`restore.go:181`) returns the same value and the same `DerivabilityDigest`
with no replay.

### What the replay proves

"The VM-local state at H is derivable from the tape alone": every
behavior-bearing row is event- or receipt-derivable, so a restore to H can
rebuild it.

### Where it came from

- 2026-08-13, `6b28999e2` "bind checkpoint witness and refuse platform/cycle
  restore (red)", under the tape-recovery Definition
  ([archive/choir-tape-recovery-2026-08-13.md](../archive/choir-tape-recovery-2026-08-13.md)).
  Its action (line 40): "Refuse checkpoint creation while any behavior-bearing
  VM-local row is not event- or receipt-derivable"; proves (line 41): "A
  checkpoint that cannot be rebuilt from the tape is not recorded; the witness
  is the verification commitment, not itself the restore address". The same
  Definition says (line 141) "the restore reconstruction gate remains the
  enforcement that the witness is true", and leaves the guest-attested witness
  as an open trust split (line 150).
- 2026-10-10, c374be0f9: a checkpoint pins the base its replay started from, so
  a restore to an older checkpoint starts from a snapshot (owner: "restore from
  pinned snapshots — that's their purpose").
- 2026-10-14, `59530c2f2`, the first commit of `bindCheckpointAtHead` and
  `openReplayProbe`: bind the head and the witness under one append hold,
  because a busy computer's checkpoint named one head and witnessed another
  ([selfdev-go-drill-jobs-0101-2026-10-15](../problems/selfdev-go-drill-jobs-0101-2026-10-15.md),
  item 1).

Provenance (standing question 1): derivability-at-bind is an orchestrator
Definition action, not owner doctrine. Doctrine `I25`
(`choir-doctrine.md:385-390`) requires checkpoint creation to fail closed only
when the **served UI identity** is underivable. The ontology defines a
checkpoint as "immutable reconstruction inputs at an event head", and a
semantic snapshot as "event head, release, content witness"
(`computer-ontology.md:181,402,418-420`): an address plus a commitment, not a
proof.

### Who reads a bound checkpoint

| Reader | What it uses | Needs the bind-time replay? |
|---|---|---|
| vmctl route projection (promote after an apply) | checkpoint digest and receipt joins (`recordMaterializationApplied`, after `RouteSlotID`) | No |
| self-development rollback (`startSelfDevelopmentRollback`, `api_self_development.go:824`) | `PriorCheckpoint` ref and prior materialization; it reinstalls a prior **release** | No |
| restore / rematerialize (`RematerializeFromTape`) | replays to the checkpoint head itself; `WitnessContentMatches` (`rematerialize.go:197`, `selfdevprotocol/rematerialize.go:66`) | No: its own replay, refuses a mismatch |
| restore start point (`newestPinnedBaseAtOrBelow`, `restore_base.go:129`) | the pinned base ref | No: the advertised base can be pinned without a replay |
| corpusd checkpoint authority (`CheckpointFromRequest`, `selfdevprotocol/control.go:244`) | witness **shape** (`Validate`) only; nothing on the platform compares the witness | No |
| acceptance-fenced restore | restore's own reconstruction gate | No |

### What breaks if the apply's bind did no replay

- **Lost: the early derivability gate.** A computer with a non-derivable row
  still gets a checkpoint; a restore to it would refuse. Nothing is restored
  wrongly, but the restore point is found dead late. Section 4 moves the proof
  to the host, so it is not lost, only moved.
- **Lost from the apply path: a diagnostic.** The probe's run-memory and
  object-graph comparisons found real replay heresies. They stay in the
  explicit probe (`choir computer replay-completeness`) and the operator stamp
  heal.
- **Removed:** the whole-store copy and replay per apply; the /tmp exhaustion
  and its Dolt Fatalf; the crash-settlement loop on b@c (problem doc, addendum
  12:3x); the second append hold; minutes in `materializing` (planned-boot
  work waits on it, `actorruntime/handler.go:61-81`). Also removed: the
  apply-time eligibility refusal that ordinary stock-app use triggers
  ([stock-ui-writes-make-a-computer-checkpoint-ineligible-2026-10-24](../problems/stock-ui-writes-make-a-computer-checkpoint-ineligible-2026-10-24.md)):
  an underived table would now mark the checkpoint `not_derivable`, visible,
  instead of blocking every apply.
- **Unchanged:** the witness value and digest, route projection, rollback,
  restore's verification, the head/witness race fix (the bind stays under the
  append hold).

## 2. The checkpoint machinery that exists, and why b@c used it badly

There are three objects, often all called "checkpoint":

| Object | Made by | When | Content | Replay start? |
|---|---|---|---|---|
| **Projection base** (`projectionbase.Descriptor`) | host checkpointd, from the tape alone | automatically: a job is due when the tail since the watermark is at least 2,500 events, or at least 1,000 and 24 h old (`checkpointDue`, `platform/projection_jobs.go:90-96`); timer every minute (`nix/node-b.nix:1341-1348`) | a full Dolt store blob plus the **replay** witness at its sequence (`rebuilder.go:437`); built incrementally from the newest compatible base (`seedScratch`/`adoptSeed`, `rebuilder.go:465-553`) | yes: what the probe and restore install |
| **ComputerVersion checkpoint** (`selfdevprotocol.Checkpoint`) | the guest, published to corpusd | on an apply, a platform-update tail, or `choir computer checkpoint` (`cmd/choir/commands.go:435`) | head, CodeRef, ArtifactProgramRef, release, frontend, **live** witness | no; it pins the base its replay used, and restore starts from that pin |
| **Residue snapshot** (`choir computer import-residue-snapshot`, `commands.go:436`; `residue_import.go`) | the guest, owner act | manual | current desktop and object-graph residue appended to the tape as one event | no; it is tape content, a migration aid |

`ReconstructThroughTarget` starts from whatever head the store it is given
already has: an empty store (genesis), an installed advertised base, or an
installed pinned base. A ComputerVersion checkpoint is not a start point by
itself; only the base it pinned is.

**Why the probe used a stale base on b@c.** The probe takes the advertised
watermark (`resolveRecoveryTarget`, `restore_base.go:54`) and replays H − W.
The cadence should keep H − W near 2,500 plus what lands during a rebuild. On
b@c it was at least 8,192 at 11:20 (eight slow pages from `after=36377` to
`after=44569`; the log prints only pages over 2 s, so the true start could be
lower), and the 11:58 replay ran toward a head near 57k. So b@c's projection
job has not published for a long time. **Hypothesis, not checked** (this lane
has no platform access): the job is failing or stuck. The check is
`GET /internal/computers/projection-base/jobs?computer_id=<b@c>` (status,
failures, error_text, watermark; `platform/projection_jobs_http.go`).

**Would replaying only the events since the last checkpoint prove the same
claim?** Yes, if the start point is itself proven. Derivability at H follows
from "base at W is the tape's projection at W" plus "replaying (W, H] onto it
equals live at H". checkpointd's bases are built from the tape alone, so W is
proven; the tail replay is the rest. That is what the probe already does. What
makes it expensive is not the length of the tail but that the guest **installs
a full copy of the base every time** to have something to replay onto.

**Cost on b@c (about 57k events).** Genesis replay: refused (over 10,000
events with no base; and at 0.14 to 0.31 GiB of journal per 1k events it would
write 8 to 18 GiB). From a fresh base (W within 2,500): the base install
(2 × blob; projection bases are about 1.7 GB, the owner computer's are about
8 GiB, `platform-artifact-retention-2026-10-22.md:20`) plus at most a few
thousand tail events, about 0.4 to 0.8 GiB; minutes, dominated by the copy.
From b@c's stale base: the copy plus about 20k events, 2.8 to 6 GiB of
journal; that is what filled 4 GiB of tmpfs. The incremental part is cheap;
the copy is the cost, and it is avoidable only by not doing the proof in the
guest.

## 3. Alternatives

| Option | Cost per apply | Proves | Risk |
|---|---|---|---|
| A. **Content-addressed checkpoint, no proof at bind.** Head H + CodeRef + ArtifactProgramRef + release + frontend + live witness at H under the append hold + Dolt HEAD as audit; pin the advertised base. | One live extract under the hold: O(store) reads, no writes, no scratch. | Identity of the state at H. | Derivability found late unless proved elsewhere. |
| B. **Incremental proof in the guest**, a retained replay store advanced by the tail. | O(tail), plus a permanent second store copy on the data volume (1.7 to 8 GiB per computer). | Derivability at each checkpoint. | Dolt still runs in the runtime: a full volume still Fatalfs it. New persistent state. |
| C. **Proof off the apply path, in a guest background job** (subprocess). | Same copy and replay, later. | Derivability, eventually. | Guest disk, CPU, RAM; needs a subprocess to survive a Fatalf. |
| D. **Proof on the host.** corpusd enqueues a projection job at H; checkpointd's incremental rebuilder builds the base at H; corpusd marks the checkpoint verified when the base witness equals the checkpoint's. | About 45 to 60 s of host CPU (rebuilds measured 43 to 56 s, `nix/node-b.nix:1306`), under checkpointd's caps (MemoryMax 6G, OOM-first, 10 min timeout, `node-b.nix:1309-1317`). | Derivability from the tape alone, **checked by the platform**, closing tape-recovery's trust split; and restore to H then needs no tail. | Host load per checkpoint; the `base_upcast` deposit-mode residual can make a host witness differ from a retained live store. |
| E. **Deletion** of derivability checking. | As A. | As A. | Gives up the warning permanently. |

## 4. Recommendation: automatic checkpoints (A + D), the apply binds cheaply

One mechanism replaces the apply-time probe and `choir computer checkpoint`:
the runtime binds a cheap content-addressed checkpoint (A) automatically, and
the host proves each one incrementally from the previous (D). The guest never
copies its store to make or prove a checkpoint. This removes the problem
class: no guest code path replays to checkpoint, so none can fill a guest
volume or kill the runtime.

Moving derivability from "refused at bind" to "verified on the host after
bind" withdraws an orchestrator Definition action (section 1): an **owner
decision**.

### 4.1 The automatic bind

- **Trigger.** The runtime binds when either N events have committed since the
  last bound checkpoint, or T has passed since it and at least one event has
  committed (no empty checkpoints). An apply, a rollback and a platform-update
  tail each force a bind at their own head and reset the counters; they are
  the same function with a different trigger.
- **Defaults: N = 2,500 events, T = 60 minutes.** N is today's checkpointd
  cadence (`checkpointDue`), whose rebuild cost is measured (43 to 56 s), and
  it keeps any restore tail under a quarter of `MaxRecoveryTailEvents` (10,000).
  T bounds how old the newest restore point on a quiet computer can be; the
  current 24 h fallback is too coarse for "put my computer back to this
  morning". At the paper's 7 to 12 events/s, N fires every 3.5 to 6 minutes:
  the same host rebuild rate as today, since today's cadence is already 2,500.
- **Configuration.** Compiled defaults in the runtime, overridable per guest
  by `CHOIR_CHECKPOINT_EVERY_EVENTS` and `CHOIR_CHECKPOINT_EVERY` in
  `/run/go-choir-autoputer.env` (already the unit's `EnvironmentFile`,
  `autoputer-vm.nix:1098`), written by vmctl from host config, so the host
  can tune the fleet without a guest release. A per-computer owner setting can
  come later; not in the first slice.
- **What one bind does.** Under one append hold: read H, extract the live
  witness at H, read the effective release and frontend identity, pin the
  currently advertised base. After the hold: publish the checkpoint to corpusd
  (the request is built from the held values, as today). No scratch, no
  replay, no Dolt writes.
- **Cost, and why it cannot fill /tmp or stall a busy computer.** It writes
  nothing locally, so it cannot fill any volume. Its cost is the live extract
  under the hold: O(store) reads while appends wait. That is the one stall
  left, and it must be measured on the paper before enabling there (residual
  R2). If it is too long, the extract moves out of the hold: commit the Dolt
  working set at H under the hold (cheap), then extract `AS OF` that commit
  outside it; the extractor does not support a ref today
  (`dolt_state_extractor.go`), so that is a follow-up, not slice 1.
- **The proof.** On publish, corpusd enqueues a projection job targeted at H
  (`EnqueueProjectionJob`, `projection_jobs.go:166`, reason `checkpoint`). The
  rebuilder seeds from the previous checkpoint's base and replays (W, H] on
  the host; never from genesis unless no base exists and the chain is within
  10,000 events, as today. corpusd records `verified` (base witness equals the
  checkpoint's) or `not_derivable` (with the differing tables) on the
  checkpoint in Store A, and advertises the new base as the watermark. The
  guest-built checkpoint and the host-built base become one restore point per
  head. checkpointd's own `checkpointDue` cadence stays only as the fallback
  for a computer whose runtime is not binding (old releases, a stuck runtime).

### 4.2 Restart rules

The cadence is a standing intent; each bind is an occurrence (the schedule rule
in CLAUDE.md). A bind is background work: it never blocks boot, a request or a
desk. A crash closes the bind in flight (an unpublished bind leaves nothing;
a published one is complete); nothing resumes it at boot; the next tick after
boot is new work, counted from the last **published** checkpoint. A tick that
crashes the computer twice turns the cadence off with that cause until an owner
act, as a schedule is closed. An apply's own bind follows the apply's rules
(4.5).

### 4.3 Retention and pruning

- Checkpoint records (a few KiB each, Store A) are kept.
- Bases (blobs, 1.7 to 8 GiB each, host disk) are pruned by the existing
  retention: keep the newest two verified per computer (today's "latest two"
  floor, `platform-artifact-retention-2026-10-22.md:20`), plus one per day for
  7 days, plus every base pinned by an apply (the apply's checkpoint and the
  one before it, which its rollback needs) until two later applies have
  succeeded; then the pin is released. Pins have no release path today (same
  doc, line 300); this adds one. An unverified or `not_derivable` base is
  removed once a newer verified one exists.

### 4.4 How restore and rollback pick

- **Restore to an event head X:** the newest **verified** checkpoint at or
  below X; install its base, replay (W, X] (at most N plus one rebuild's worth
  of events). A checkpoint exactly at X installs with no tail. No verified
  checkpoint within `MaxRecoveryTailEvents` of X refuses, as today.
- **Rollback of a self-development apply:** unchanged in kind. It reinstalls
  the prior release and names the prior checkpoint, which is the checkpoint
  the previous apply bound (pinned, 4.3). It does not restore data.
- `choir computer checkpoint` is **deleted** as a bind: the automatic bind
  replaces it, and a second, manual publication path is a second authority for
  the same object (standing question 5). Its read half survives as
  `choir computer checkpoints`, listing the computer's checkpoints with their
  verification state. `choir computer replay-completeness` stays as the
  diagnostic replay, behind the lane's bound.

### 4.5 The apply

The apply's bind is one automatic bind forced at the applied head (4.1):
seconds of live extract under the hold, no replay. Route projection, rollback
and restore read it as they read today's. Its proof arrives from the host
later and does not gate the route; a `not_derivable` result is shown on the
operation and blocks only restore to that head. Two more rules close the
crash loop:

- **The crash settlement never repeats an unbounded step.** With the bind
  cheap, the journal-only settlement (`settleSelfDevelopmentOperationAfterCrash`,
  `self_development_materializer.go:164`) records the apply with bounded cost.
- **A settlement that crashes the computer twice closes.** It spends a durable
  attempt before recording (reuse `spendGuestApplyAttempt`,
  `planned_restart.go:256`, reason `self_development_settlement`, limit 2).
  With both spent and no terminal record, the operation closes `degraded` with
  the cause "the settlement crashed the computer twice"; the applied release
  and the `materialization-applied` event stay; the route is not promoted; an
  owner act (rollback, or a manual re-bind) decides.

### 4.6 Slices

1. **Guest, red, tier 2.** `bindCheckpointAtHead` stops calling
   `openReplayProbe` and binds as in 4.1 (all three callers change with it);
   the settlement attempt bound (4.5); the lane's scratch guard stays for the
   diagnostic probe and the stamp heal. No corpusd, schema or protocol change:
   corpusd validates shape only and restore compares the same digest. Tests
   first: the bind creates no scratch and installs no base; its witness equals
   the replay path's on the same fixture; a restore to it succeeds; a restore
   to one whose live store has a non-derivable row refuses at restore; a
   settlement that crashed twice closes `degraded`.
2. **Host, red.** Projection job at the checkpoint head; `verified` /
   `not_derivable` on the checkpoint; the restore picker prefers verified.
3. **Cadence, orange.** The N/T trigger in the runtime, env overrides, the
   crash-twice cadence rule; delete `choir computer checkpoint`'s bind, add
   `choir computer checkpoints`.
4. **Retention.** Pin release and the keep rules of 4.3.

### 4.7 b@c's selfdev-83fbdfa9 on its next boot

Its updater journal is terminal `applied` and its `materialization-applied`
event is on the tape; only the checkpoint, the route promotion and the
operation's `applied` row are missing. With slice 1 deployed, the next boot is
a crash boot (no marker), and the journal-only settlement records it: one live
extract under the hold, publish, promote, `applied`. No scratch and no replay.
tmpfs is empty after the VM stop. The settlement spends one of its two
attempts first, so if any other step crashes it, the second boot closes the
operation `degraded` instead of looping. Before booting b@c, read its
projection job (section 2): if checkpointd is failing for b@c, slice 2's proof
will fail too, but slice 1 does not depend on it.

If the owner prefers to land the lane's bound first instead: the probe moves to
the 32 GiB data volume and is admitted against its free space, so that boot
would most likely settle too, but by copying the store and replaying about 20k
events, minutes under the hold. Slice 1 is the safer boot.

## Residuals

- R1: deposit-mode differences between a host rebuild and a retained live
  store (`base_upcast`, `replay_completeness.go:404` on main) can make slice 2
  report `not_derivable` for a store that restores correctly; slice 2 must
  name that case.
- R2: the live extract's time under the hold is unmeasured on the paper; it
  sets whether 4.1 needs the `AS OF` follow-up before the cadence runs there.
- R3: why b@c's projection base stopped advancing (section 2) is unexplained;
  read its projection job.
- R4: the lane's broad `./internal/agentcore/...` run on choir-agents hit the
  10-minute `go test` timeout (the package is sharded in CI); the focused
  replay, restore, checkpoint, self-development and crash-settlement tests
  pass. Run `scripts/go-test-runtime-shards` before any landing.

## Build findings (2026-10-26, lane x1-slices23: slices 2 and 3)

Slices 2 and 3 are built on `lane/apply-replay-bound` (slice 1b). Slice 4 is
not built (F10). Where the design and the code disagreed, the code's reading
and the choice made are recorded here; each is a one-line reversal.

**F1. There is no restore picker to make prefer verified.** Restore takes the
checkpoint as its operand (`RematerializeFromTape`); the only choice it makes
is which base to install, and a base is derived from the tape alone, so it is
neither verified nor unverified. What 4.4 asks for is built another way:
restore asks the host's verdict first and refuses `not_derivable` before it
installs anything (409, `ErrCheckpointNotDerivable`); and the host pins the
judged base under an apply checkpoint's reference, so a restore to it starts
at the exact head with no tail (`newestPinnedBaseAtOrBelow` already picks the
newest pin at or below the target).

**F2. Four verdict states, not two.** `pending` (published, not judged),
`verified`, `not_derivable`, and `unverified` (the host did not or could not
judge: no base at or below the head and genesis beyond the replay bound, a job
the host refused or failed three times at that head, a base at another head,
an invalid witness, or R1). Two outcomes cannot say "not yet" or "cannot tell",
and calling "cannot tell" `not_derivable` would make restore refuse good
restore points. Restore refuses only `not_derivable`.

**F3. R1 is named and is `unverified / base_upcast`, never `not_derivable`.**
The host rebuild always upcasts deposits; a live store without an upcast ledger
kept its recorded bytes (`matchLiveDepositMode`, `replay_completeness.go`).
The bind now records the live store's deposit mode in the checkpoint request
(`deposit_mode`, additive, omitted when empty, so every pre-field digest is
unchanged). A mismatch is `not_derivable` only when the checkpoint says
`upcast` (both sides built in one mode); against `retained`, or a checkpoint
that does not say, it is `unverified / base_upcast` with the differing tables
named, and restore's own reconstruction (in the live store's mode) decides.
Equal witnesses are `verified` in every mode. Consequence: a computer whose
store is retained gets no `not_derivable` verdict until it is upcast; it keeps
today's restore-time enforcement.

**F4. The verification job is frozen at the checkpoint's head, not the
computer's.** `EnqueueProjectionJob` freezes the head at enqueue time; by then
a working computer has moved on, and the rebuilt witness would be another
state's. `EnqueueCheckpointVerificationJob` targets the checkpoint's own
sequence and head and seeds from the advertised base when it is at or below
the checkpoint, else from the newest recorded base that is (a base published
below the advertised one is recorded in `computer_projection_bases` so it can
be found by sequence). With no base in reach and the checkpoint beyond the
10,000-event bound it refuses by name (`unverified / no_base`) instead of
queuing a job that can only block the computer's cadence. The job row is one
per computer, so verification and checkpointd's own cadence job take turns;
the oldest unjudged checkpoint per computer goes first, and each rebuild seeds
the next. A cadence base that lands on a checkpoint's head judges it too. The
verdict pass runs on checkpointd's minute timer and is best effort (a verdict
problem is recorded on the heartbeat and never stops cadence jobs).
Deploy order: corpusd before checkpointd (corpusd creates
`computer_checkpoint_verdicts`; checkpointd applies no DDL), and corpusd before
the guests (a guest on this build sends `deposit_mode`, which an older corpusd's
strict decoder refuses as an invalid checkpoint request, apply binds included).

**F5. Pins are for apply checkpoints, not the cadence (a deviation from the
wording of 4.1).** 4.1 says every bind pins the advertised base; 4.3 releases
only apply pins ("until two later applies have succeeded"). With a cadence
bind every 3.5 to 6 minutes on the paper, pinning each would hold one base
(1.7 to 8 GiB) per bind with no release path until slice 4. So the
owner-recovery bind (the cadence's) and the host's judged-base pin skip the
pin; the apply and the platform update keep theirs. The cost, stated: a
cadence checkpoint restores with no tail only while its base is among the
newest two (the existing retention); older ones fall back to the nearest pin or
the advertised base within the 10,000-event bound, or refuse. "Put my computer
back to this morning" is not delivered until slice 4's one-per-day keep.
Reversal: the `pinBase` argument of `bindCheckpoint` and the
`OwnerRecovery` test in `judgeAgainstBase`.

**F6. The cadence is built; its measurement is not (superseded by "Review 329"
below: it now compiles OFF and the host selects computers).**
Defaults were compiled in (2,500 events, 60 minutes); `CHOIR_CHECKPOINT_EVERY_EVENTS`
and `CHOIR_CHECKPOINT_EVERY` are read from the process environment (`0` turns a
trigger off; a typo keeps the default). The path that writes them into the
guest (vmctl, kernel cmdline, `go-choir-extract-cmdline` into
`/run/go-choir-autoputer.env`) is not built, so the fleet cannot be tuned
without a guest release yet. R2 is still unmeasured: the live extract's time
under the append hold on the paper. Until it is, the cadence limits itself:
binds are at least a minute apart and at least twenty times the last hold apart
(at most about 5% of the time held, whatever the store's size), a failing bind
backs off 5, 10, 20 up to 60 minutes, and the hold's length is published as
`last_bind_millis` in the listing, which is the measurement R2 asks for. Whoever
deploys this should read that number on the paper first and have the plumbing
ready; as built, the compiled defaults go live on every computer at deploy.

**F7. The owner act that reopens a closed cadence is `choir computer
resume-checkpoints`.** 4.2 says a tick that crashes the computer twice turns the
cadence off "until an owner act" without naming one. It is a CLI-only owner act
(`POST /api/computers/{id}/lifecycle/resume-checkpoints`, needs the owner's
session or an `owner:acts` key), and it only forgets the counted starts and the
backoff. The two-crash count is the existing durable attempt file
(`spendSettlementAttempt`, reason `checkpoint_cadence`, limit 2); a tick that
returns, in success or error, is forgotten, so only a bind that never returned
stays counted. A runtime with no durable attempt count closes the cadence.

**F8. What was removed and added.** Removed: `POST /lifecycle/checkpoint` (the
guest handler) and `choir computer checkpoint`; the proxy's special case and
long budget for that path. Added: `GET /api/computers/{id}/lifecycle/checkpoints`
(the computer's checkpoints with the host's verdict on each and the cadence's
state; `?checkpoint_digest=` returns one checkpoint with its receipt, which is
a `--checkpoint-file` operand for restore), `choir computer checkpoints
[--digest D]`, corpusd's `GET /internal/computers/checkpoints/list` and
`/checkpoints/verdict`, and the guest credential's `ListCheckpoints` and
`FetchCheckpointByDigest` (the receipt is verified against the platform key).
The cadence counts from the platform's newest checkpoint, the one authority
for "since the last checkpoint", and asks the platform only when a checkpoint
could be due. An apply's, a rollback's and a platform update's checkpoint reset
the count by being the newest.

**F9. Tests changed.** Four tests bound a checkpoint through the deleted route;
they call `BindCheckpointRestoreSet` directly. The proxy's long-budget test
moves from the deleted route to restore. Three assertions that the owner-
recovery bind pins a base now assert it does not, and a new test keeps the
apply bind's pin covered.

**F10. Slice 4 is not built: 4.3 does not fully specify it.** (a) "Pin release
after two later applies have succeeded" needs a count of successful applies and
an owner for the release; the release route needs `computer:lifecycle`, which
the guest's capability does not carry, and the host has no apply-success
signal. (b) "One per day for 7 days" is a new selection rule inside artifact GC
(`projectionBasePins`), which today keeps the newest two bases per computer,
every pin and in-flight seeds; it needs day buckets and a join to the verdict
table, in a protected surface with its own history
([platform-artifact-retention-2026-10-22](../problems/platform-artifact-retention-2026-10-22.md)).
(c) "An unverified or `not_derivable` base is removed once a newer verified one
exists" confuses the base (always tape-derived, a valid replay start) with the
checkpoint's live witness (what the verdict judges); it needs a ruling on what
is deleted. (d) F5 shows retention is not optional for the cadence's value: it
is what turns "newest two" into "this morning". It wants its own design pass.

**F11. Host load, to watch after deploy.** Each checkpoint costs one host
rebuild (43 to 56 s, checkpointd's caps unchanged). When the guest cadence is
every 2,500 events it replaces checkpointd's own cadence rebuild (the base lands
at the checkpoint's head and the watermark is current), so the rate is the same
as today's. The extra rebuilds are the time-triggered ones (a quiet computer,
every 60 minutes with at least one event) and each apply's. The queue is one
job per computer, so a burst on one computer cannot starve another.

## Review 329 (2026-10-26): what was adopted and what was deferred

Reviewed by Devin and DeepSeek v4.1 flash (tier 2); both CHANGES REQUIRED. The
lane is rebased onto `origin/main` at train 20 (`dd2725016`); the settlement
code (`recordPlatformUpdateApplied`, `recordSettlementApplied`, run before any
spend) is main's, untouched, and the density, base-join and live-capsule
reverts were the stale base, gone with the rebase.

Adopted:

- **Cadence compiles OFF, host-selected per computer (F6 superseded).** The
  guest reads `CHOIR_CHECKPOINT_EVERY_EVENTS` / `CHOIR_CHECKPOINT_EVERY`; unset,
  zero or bad is off. vmctl's `VM_CHECKPOINT_CADENCE_ONLY` (computer ids; the
  same selector family as the tunnel enforce and the DAMON reclaim) names the
  computers, with `VM_CHECKPOINT_CADENCE_EVERY_EVENTS` (2500) and
  `VM_CHECKPOINT_CADENCE_EVERY` (60m), as two kernel boot arguments; the guest
  boot script validates them (digits; digits and one unit) before it writes the
  environment file. `nix/node-b.nix` ships the list empty. Procedure: name d@e,
  redeploy (it cold boots once), read `last_bind_millis` in `choir computer
  checkpoints`, widen one computer at a time; the paper stays off until measured.
  Rollback: empty the list and redeploy.
- **Proof scripts moved off the deleted bind.** `scripts/reeng_tracer.mjs` and
  `scripts/m11_selfdev_episode_probe.mjs` read `GET .../lifecycle/checkpoints`,
  pin the newest checkpoint restore would not refuse, fetch it by digest, and
  restore to it (`scripts/checkpoint_pick.mjs`, tested). The restore leg no
  longer demands the pinned checkpoint's head equal the pre-apply head (a
  cadence checkpoint is older by up to a cadence): it demands the restore
  matched, restored that checkpoint, and that the apply had moved the head past
  it. On a computer the host did not select there is no checkpoint to pin: the
  run records the gap. A drill that needs a restore target must run on a
  selected computer.
- **Verification starvation (Devin 6, DeepSeek 4).** checkpointd queues the
  checkpoint rebuilds before its own cadence jobs, so a busy computer's job row
  is not retaken every minute; and a refusal or a backoff of the computer's job
  at another target no longer blocks a checkpoint's rebuild (a refusal at the
  checkpoint's own target still stands).
- **Version-skew verdicts (DeepSeek 6, Devin 7).** A mismatch across reducer
  versions (host rebuild vs the checkpoint's) is `unverified / version_skew`,
  never `not_derivable`; equal content is verified whatever the versions. The
  base is not pinned for a checkpoint judged `not_derivable`.
- **Deposit mode outside the digest (DeepSeek 5).** The digest is of the request
  without `deposit_mode`, so a guest rolled back below the field recomputes the
  same digest and every checkpoint stays restorable. The mode stays in the stored
  request for the host. A rolled-back guest's strict `FetchCheckpoint` decode
  still refuses a checkpoint carrying the field, which affects only a platform
  update tail resuming across the rollback.
- **Restore refuses before it writes (DeepSeek 7):** the not_derivable refusal
  precedes the restore intent on the restore route.
- **Unreadable attempt file (DeepSeek 8):** the cadence closes with that cause
  instead of reading `on` while skipping.
- Stale docs (the CLI parity row, the drill problem doc) now name the deleted
  route.

Deferred, with reasons:

- **Owner rejudge of a verdict.** `not_derivable` now needs the same reducer, a
  same-mode mismatch and a base at the exact head; a rejudge is a way to rerun a
  proof that has no demonstrated need. Add it when a good point is refused.
- **Repairing a corrupt attempt file on `resume-checkpoints`.** The file also
  bounds apply and settlement crash loops; deleting it is an operator act, not
  a side effect of an owner CLI verb.
- **A crash after the bind's work but before the start is forgotten.** A start
  is counted before the bind and forgotten when `bindFn` returns, so a bind
  that publishes and then crashes the guest before returning has never
  returned: it stays counted, and two of those close the cadence (owner-
  reopenable). The bound is fail-closed. What it does not count is a crash
  after the start is forgotten (the next tick's own work), which is the next
  tick's start. (Review 336 corrected this paragraph; it said the opposite.)
- **A vocabulary or schema version in the skew check.** The checkpoint carries
  a reducer version only; `deposit_mode: upcast` already implies the V2
  vocabulary. Adding versions to the checkpoint repeats F3's cost. The coupling
  is therefore explicit (review 336): **any vocabulary or schema bump that can
  change a rebuilt witness must bump `ReducerVersion`**, or a newer host will
  record a permanent `not_derivable` for a good point. Until a rejudge exists,
  that bump is the safety.
- **`last_bound_*` in the status is memory-only** (zero after each boot); the
  listing's checkpoints are the platform's and survive.

## Review 336 (2026-10-26): adjudication

Devin approved; DeepSeek asked for one blocking fix and recorded three lesser
points.

- **Superseded by the S3 owner decision (2026-10-28, see paper-never-checkpointed-2026-10-27.md): the code refusal below is removed; rings decide reach.**
- **Blocking, adopted: the selector refuses wildcards, classes and protected
  computers in code.** `VM_CHECKPOINT_CADENCE_ONLY` takes computer ids only:
  the config load drops `*` and every `class:` token (logging each), and the
  launch refuses them again at the point of use, and refuses any computer whose
  launch carries `critical_protected` or `public_platform`. The class is a
  second line, not the first: the platform computer's boot and resume config do
  not set `WarmnessClass`, so the paper is kept off by the list naming ids only
  and by the nix list shipping empty. Tested in both places.
- **Low 2, adopted:** the crash-bound sentence in "Review 329" said the opposite
  of the code; corrected above.
- **Low 3, accepted as designed:** the proof scripts need a checkpoint they
  cannot mint. That is the cost of deleting the manual bind (4.4, F9), recorded
  in "Review 329"; a drill that needs a restore target runs on a computer the
  host selected, or after an apply that bound one.
- **Residual 4, adopted as a stated coupling:** a vocabulary or schema bump must
  bump `ReducerVersion` (above); recording the vocabulary in the checkpoint is
  deferred with the rejudge.
