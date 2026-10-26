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
