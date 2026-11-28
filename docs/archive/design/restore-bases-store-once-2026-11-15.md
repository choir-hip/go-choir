# Restore bases that cost the change (2026-11-15)

Standing: superseded | kind: decided | settled-by: header line 5: set aside 2026-11-15 after three review rounds (docs/problems/restore-bases-design-three-review-rounds-structural-assessment-2026-11-15.md) | replaced-by: none | note: nothing was built from its landing list

> Set aside on 2026-11-15 after a third review round asked for repair. Nothing is built from the landing list below. See docs/problems/restore-bases-design-three-review-rounds-structural-assessment-2026-11-15.md.

Status: revised after review round 2. Round 1 had one seat (DeepSeek), which
returned REPAIR. Round 2 had two (DeepSeek and Sonnet), both REPAIR. Sections
8 and 9 list each finding, my verdict and what changed. Code references were
checked at origin/main 137f419dd. Between f6c8176e0 and 137f419dd only docs
changed, so the earlier line numbers still hold.

Class: green. This is a design record. It covers the "going forward" half of
the owner's ask of 2026-11-15 ("solve the paper restore issue in a storage
efficient way now and going forward"). The owner also said: "We don't need
to restore right now. Autopaper data isn't precious yet."

Labels:

- **[M]** measured, with its source;
- **[C]** read in code;
- **[W]** a primary web source;
- **[E]** an estimate, arithmetic shown;
- **[I]** inferred;
- **[?]** not known.

## 0. The answer in brief

1. **The paper is not replay-eligible today, and it binds no checkpoints**
   (section 2):
   - It has 150 `agent_evidence` rows. That table is written by direct SQL
     and is classed "empty until supported", so any replay-completeness
     judgement reads it as ineligible (`replay_eligibility.go:90`, `:198`,
     `:270`) [C, M].
   - The host runs the checkpoint cadence only for d@e and `class:test`
     (`nix/node-b.nix:1704`). No paper console log from 01:29Z to 06:38Z
     has a single cadence line [C, M].

   A staged restore of the paper can therefore succeed only to a checkpoint
   someone binds by hand, from a base whose `agent_evidence` rows equal that
   checkpoint's. Whether they still do is [?]. **The first landing makes the
   paper eligible.**
2. **The capture keeps its witness, but stops pinning the WAL for it.**
   Inside the read transaction, the capture copies the pages to a private
   scratch file. It then releases the transaction and computes the witness
   on the static copy. Then it uploads.
   - The pin falls from about 26 to 38 minutes to the length of the copy
     (3 to 16 minutes, [E]).
   - No earlier ruling is overruled: the witness stays.
   - Nothing comes to depend on the checkpoint cadence.
   - The price is about one store's size of guest scratch for the length of
     the witness.
3. **One static copy, three uses.** That copy is a consistent state at head
   N. It can serve as:
   - the snapshot base;
   - the place where a checkpoint at N is bound, so a bind no longer holds
     appends for a whole-store extract;
   - when compaction is due, the source of the compact copy.

   Bases and checkpoints then come in pairs at the same head.
4. **Page deltas, logical bases and "one restore path" (landing 7 of round
   1) are not recommended now.** Each is cut to one paragraph in section 3.
   At ten times the scale an incremental base is still needed, and the
   choice remains a measurement.
5. **There is no genesis fallback.** A cold boot whose chain has no usable
   base within 720,000 events refuses durably with `base_missing`
   (`projection_base.go:170-175`, `recovery_plan.go:22`, `:131`) [C]. No
   owner act authorizes a longer replay, and I do not propose one: on the
   paper it would be 11.8 M events, about 43 hours at 77 events/s [E].

## Owner decisions (with my defaults)

1. **The restore budget R** sets the base cadence, and so bytes a day.
   Default: **120 minutes** of replay at the slowest measured rate. That
   costs about 1.6 to 2.6 GB a day on today's store, near today's 2.2
   [E, section 3]. R = 60 would cost 3.2 to 5.8 GB a day.
2. **The checkpoint cadence on every computer**, deleting the host
   allowlist. That allowlist is a switch, against the delivery rule.
   Default: **yes, in landing 3**, once landing 2 has made a bind's append
   hold about the length of a page copy. Each hold is measured and reported.
3. **`evidence.graph` on everywhere.** The owner held it on 2026-11-03 until
   the first-write import was measured on a copy of the paper's store
   (`computer_setting.go:77-85`). Default: **proceed**. Landing 1 measures
   the import on d@e, and the paper's import is 150 rows.

## 1. The frame

- **A base is a cache of replay.** It adds no information; it bounds rebuild
  time.
- **The capture's cost is the whole-store witness inside the live read
  transaction, not the bytes.** On 11-12 the witness took 22 min 37 s and
  the stream 3 min 02 s [M]. Moving the witness off the pin keeps its
  detection and drops its main cost. This answer came from round 2 (Sonnet).
  It beats dropping the witness, which made every install depend on a
  checkpoint cadence that is off for the paper.
- **One mechanism, several jobs.** A static, consistent copy at head N is
  what the snapshot needs (pages plus witness), what a checkpoint bind needs
  (a canonical witness at a fixed head) and what compaction needs (a source
  to rewrite). Today each of the three makes or pins its own read of the
  live store.
- **The limit case (ten times the scale)** still fails every whole-store
  capture on bytes (section 3). The design there is a later, measured
  choice.

## 2. Facts, with sources

### The paper

- **Store.** 39.6 GB: `page_count` 9,669,301 × 4,096 bytes, observability at
  2026-11-15T05:49:12Z [M]. Volume available: 85,901,574,144 bytes (85.9 GB)
  [M, `data_volume`].
- **Compaction** (console): VACUUM INTO 17 min 17 s for 42 GB live; staged
  after 45 min 13 s, which includes `quick_check` (`compact.go:482`). Rebased
  at 03:43:13Z with 13,782 events to replay [M].
- **Replay**, measured once, with a cold cache after that rebase (console
  03:46:13Z): 77 events/s.
  - Per event: finalize 7.7 ms, fetch 3.3 ms, verify 0.17 ms [M].
  - The producer takes the slower of 113 and the measured rate
    (`snapshot_producer.go:379-381`). But the rate lives in process memory,
    so the block at 05:46:26Z reads `assumed` [C, M].
- **Event rate.** 315,882 events in 31.2 hours, from the newest snapshot
  (11,537,493 at 2026-11-13T22:36:18Z) to head 11,853,375: about 10,100 an
  hour [M, E].
- **Last captures.**
  - 11-13: 46.7 GB in 37 min 40 s, 4.8 GB sealed added [M].
  - 11-12: witness 22 min 37 s, stream 3 min 02 s, read transaction
    25 min 51 s, 5,858 of 10,524 chunks uploaded [M].
- **Checkpoints.**
  - The cadence compiles off (`checkpoint_cadence.go:27-34`). The host names
    the computers that run it: `VM_CHECKPOINT_CADENCE_ONLY=computer-59c902c5…,class:test`
    (`nix/node-b.nix:1704`). The paper is `computer-1355b83b…`
    (`nix/node-b.nix:342`). Where the host means the paper alongside
    `class:test`, it names it explicitly (`:1572`) [C].
  - `grep -c "checkpoint cadence"` over the paper's five console logs
    (01:29Z to 06:38Z) gives 0 [M].
  - A bind holds appends while it extracts the live store
    (`checkpoint_cadence.go:43-46`). This is why the cadence is held back
    for the paper [C].
- **Eligibility.**
  - `agent_evidence` and `co_super_slots` are `ReplayEmptyUntilSupported`
    (`replay_eligibility.go:69`, `:90`). A non-empty one makes a probe
    ineligible (`:198`, `:270`) [C].
  - On the paper, `agent_evidence` has 150 rows and `co_super_slots` 0
    [M, `store_tables`].
  - The restore gate compares the reconstructed witness with the
    checkpoint's, which includes these tables (only `DerivedKeysTable` is
    ignored, `restore_snapshot.go:218-232`) [C].
  - So a restore passes only if the base already holds the same rows as the
    checkpoint: replay never writes them [I].
- **The graph homes.** `engineering.slots_graph` is on by default and
  `evidence.graph` is off (`computer_setting.go:77-92`, `:211-212`) [C].
  - The import moves rows into the graph through the graph writer, then
    deletes each row by direct SQL (`graph_home.go:433-500`) [C]. The graph
    writer reaches the tape [I, from the setting's own comment "on the
    tape"].
- **Bounds.** `MaxRecoveryTailEvents` is 720,000 (`recovery_plan.go:22`), and
  a longer tail refuses (`:131`). The cold path with no base inside the bound
  refuses `base_missing` (`projection_base.go:170-175`) [C].
  - The paper's newest base is 316,000 events behind head. At 10,100 an
    hour it leaves the bound in about 40 hours, unless the 540,000-event
    ceiling capture fires first, in about 22 hours [E].
- **The compact swap** copies `co_super_slots` and `agent_evidence` from the
  original into the copy (`carryDirectTables`, `compact.go:700-720`) [C].
- **Host.** 195 GiB free. The paper's `data.img` is 128 GiB apparent and 42
  GiB allocated, down from 88.5 GiB at 01:35Z [M]. So guest discards reach
  the host now [I].

### Primary sources

- **VACUUM INTO** output is "a consistent snapshot of the original
  database", and the page says nothing about layout
  (https://www.sqlite.org/lang_vacuum.html) [W].
- **Page format** (https://www.sqlite.org/fileformat.html) [W]: interior,
  overflow and ptrmap pages carry page numbers, and a WAL frame header
  carries the page number.
- **`sqlite_dbpage`** reads pages through the pager inside the transaction
  (https://www.sqlite.org/dbpage.html, cited by round 2; not re-fetched by
  me).
- **Session extension** (https://www.sqlite.org/sessionintro.html) [W].
- **`sqlite3_snapshot_open`**
  (https://www.sqlite.org/c3ref/snapshot_open.html) [W].
- **Litestream** (https://litestream.io/how-it-works/) [W].
- **FastCDC**
  (https://www.usenix.org/conference/atc16/technical-sessions/presentation/xia)
  [W].

## 3. The options, with numbers

### The cost model

Inputs:

- R is minutes of replay at 77/s.
- Base interval I = 0.457 × R hours. Bases a day n = 24 / I.
- Full sealed F ≈ S / 6 = 6.6 GB [E]. A sealed ratio of about 10.9 on the
  whole store would put F near 3.6 GB.
- A fixed-chunk full writes p_1024(I) × F.

| R | I | n a day | Fixed-chunk fulls, sealed a day (p_1024 0.5 to 1) |
| --- | --- | --- | --- |
| 4 min | 1.83 h | 13.1 | 43 to 86 GB [E] |
| 60 min | 27.4 h | 0.88 | 2.9 to 5.8 GB [E] |
| 120 min | 54.9 h | 0.44 | 1.5 to 2.9 GB [E] |

For comparison, today's floor-limited producer at the 540,000-event ceiling
is about 2.2 GB a day [E: 4.8 GB per 53 hours]. That cadence implies a
restore of up to about 117 minutes of replay. The 0.5 is the compaction
bench's 24-hour figure on a compacted file [M, synthetic]. A grown file is
near 1.

### The recommended option: the static copy (scratch capture)

**Inside the read transaction** (head first, `snapshot.go:547`), stream
`sqlite_dbpage` into `state.db.snapshot-scratch`, a private file. Then
release the transaction.

**Outside it, on the static file:**

1. the witness (v3, as today);
2. optionally the canonical checkpoint witness (landing 3);
3. chunk, seal and upload as today;
4. delete the file.

**Pin.** The read is at most the 11-12 stream rate (44.1 GB in 3 min 02 s,
with sealing and upload included) [M]. The write runs at the guest disk's
rate, which is [?]. VACUUM INTO wrote 42 GB in 17 minutes while also
rebuilding b-trees, so it is a lower bound [M]. The pin is therefore about
3 to 16 minutes, against 26 to 38 today [E].

**Unpinned work.** The witness: 22 to 35 minutes of CPU and IO on a file no
writer touches [M, from the in-transaction figures].

**Disk.** S of scratch (39.6 GB) against 85.9 GB available, leaving about
46 GB [M, E]. This cannot run beside a staged compact copy: two copies
would be 79 GB against 85.9 GB less the 2 GiB floor. While a staged copy
exists, the capture either uses it (landing 5) or waits.

**Host.** Each capture writes about 40 GB of guest blocks. At R = 120 that is
about 18 GB a day of host writes [E]. Guest discards appear to reach the
host now [I], so allocation should not ratchet up. If they do not, growth is
bounded by the 128 GiB image.

**What it blocks.**

- During the copy: the WAL pin, as today but shorter.
- A planned restart waits only for the copy (the existing `inFlight`,
  `snapshot_producer.go:218-222`). A restart during the witness abandons the
  scratch file. Nothing resumes it, and the next tick starts over.

**Deletes.**

- The 90-minute in-transaction budget (`snapshot_producer.go:80`) becomes a
  copy budget (20 minutes proposed) and a separate, unpinned witness budget.
- The WAL headroom rule (`:124`) is computed from the copy's duration, not
  the capture's.

**Bytes.** Unchanged: the pages are the live file's, so fixed-chunk dedupe is
as today.

**At ten times the scale.** At 396 GB the copy would be about 27 minutes or
more of pin, and 396 GB of scratch would not fit a volume like the paper's
[E]. A bounded store (storage mission T3) is a precondition there.

### Not recommended now (one paragraph each)

- **Page-delta bases (round 1's choice).** They beat fixed-chunk fulls only
  if 2 × p_1 < p_1024 at the base interval. The paper's 19 `og_objects`
  indexes and the daily `incremental_vacuum` of up to 250,000 pages
  (`telemetry_retire.go:333`, `compact.go:105`) push p_1 up. A chain also
  needs, all of them unbuilt:
  - a base sequence and content dating (`restore_snapshot.go:108-136`,
    `snapshot.go:945`);
  - chain liveness on the host (`snapshot_roots.go:43-74`);
  - a page root computed from the pages read;
  - truncation;
  - a tree of chunk lists for ten-times caps.

  Revisit at scale, after the store is bounded. Landing 0 (below) can only
  falsify them.
- **Logical (row) bases.** They ship no index pages and are immune to page
  moves. Restore costs an import plus an index build (the split rebuilt
  22.4 GB in 579 s [M]). Deltas need per-row change tracking on about 15
  tables. Revisit together with page deltas at scale.
- **One restore path, and dropping the capture witness (round 1's landing
  7).** Withdrawn. Every install would have depended on checkpoints the
  paper does not bind. The cold path would have lost its only logical check,
  and a systematic capture fault would have failed every base, to a durable
  refusal. The scratch copy keeps capture-time detection and still removes
  the pin.
- **The compact copy as the only base.** VACUUM INTO renumbers pages, so a
  copy-based full never dedupes. Its cadence (freelist, or 7 days) leaves the
  tail past the bound. As a source of the static copy it is landing 5.
- **CDC over whole captures.** The live file never shifts. Vacuumed copies
  shift and also rewrite every pointer page. Rejected.
- **Faster replay.** Overlapping the fetch reaches at most about 104
  events/s [E], and 2,000 events/s is out of reach. Worth doing for itself,
  but not a base answer.

## 4. Why not "make replay fast"

Without a base, the paper replays 11.8 M events: 1.6 hours even at 2,000/s,
and 43 hours at 77/s [E]. The bound refuses it anyway. Fast replay shortens
the tail. It cannot replace the base.

## 5. Recommendation: ordered landings

Five landings. Landings 1 and 2 need no owner decision. Each is red and
needs two seats, unless marked otherwise. Proofs run on d@e. The paper is
read-only.

### Landing 1: both direct-SQL tables become tape-derived, and empty on every computer

**What it does.**

- Turn `evidence.graph`'s behaviour on permanently, and delete both settings
  (`computer_setting.go:77-92`).
- At boot, run the existing imports (`ImportEngineeringSlotsToGraph`, and the
  evidence import of the same shape). They move each row into the graph
  through the graph writer, onto the tape, then delete it.
- Delete the direct-SQL writers, `carryDirectTables` and
  `compactCarriedTables` (`compact.go:700-720`).
- End state: both tables empty on every computer, then dropped from the
  schema in the same landing or the next. The eligibility manifest entries
  become `ReplayRetiredAbsent`.

**What is lost.** Nothing is dropped: rows move into the graph.

**What becomes unrestorable.** Checkpoints bound before the import whose
witness included rows. A replay cannot recreate those rows, so such a
checkpoint restores only from a base captured with the same rows. That is
true today as well.

**Proof on d@e.**

1. Write an evidence row.
2. Deploy.
3. `store_tables` shows both tables empty.
4. A replay-completeness report reads eligible, or names a reason other than
   `unsupported_tables`.
5. A restore to a checkpoint bound after the import answers
   `witness_matched: true`.

**Measured.** The import's time and hold on d@e, answering the owner's
11-03 request.

**Rollback.** Revert. Rows already in the graph stay readable only through a
graph path. The reverted binary's `evidence.graph` default is off, so new
evidence returns to the table and moved rows are not read until it is set
on. Say so in the commit.

**Prediction** (claude ledger, settle 24 hours after the deploy):
`store_tables` on the paper shows `agent_evidence` with 0 rows, and on d@e a
replay-completeness report does not name `agent_evidence` or
`co_super_slots`. Probability 0.6.

### Landing 2: capture through a static scratch copy

**What it does.** As described in section 3, "the recommended option".

- A copy budget replaces the 90-minute in-transaction budget, and the
  witness runs unpinned on the file.
- The producer refuses, with its reason, when free space is below S plus the
  2 GiB floor, or while a staged compact copy exists. d@e's store is about
  1.1 GB, so it needs about 3.1 GB free.
- Record, restore and `VerifySnapshotFile` are unchanged.
- No page root is added: chunk seals and the manifest already authenticate
  the bytes (`snapshot.go:1008`, `:1166-1210`).
- The capture record gains the copy's and the witness's durations. The
  floor (`snapshotCaptureFloor`) is computed from the pinned time, not the
  whole.

**Protected surfaces.** Snapshot capture; the guest data volume.

**Conjecture delta.** A page copy made inside one read transaction, with the
head read first, is the head's state. The witness on the copy checks that
claim as the in-transaction witness did.

**Proof on d@e.** Two captures whose logs show copy time, witness time and a
read transaction equal to the copy time. Then a `choir computer restore` to
a checkpoint above the second answers `witness_matched: true`.

**Rollback.** Revert. Records are unchanged, so old binaries restore the new
bases.

**Prediction for the paper** (its next capture after the deploy; read-only
from the console): the read transaction is at most 16 minutes, the witness
completes on the copy, and the snapshot is recorded. Probability 0.55.

### Landing 3: checkpoints bound from the capture's static copy; the cadence on every computer; the allowlist deleted

Owner decision 2; my default is yes.

**What it does.**

- The bind's canonical witness is computed on the scratch copy at the
  capture's head, so the bind's append hold becomes the copy's length, not a
  whole-store extract.
- Each base gets a checkpoint at its own head.
- The cadence runs on every computer.
- `VM_CHECKPOINT_CADENCE_ONLY` (`nix/node-b.nix:1704`,
  `vmmanager/checkpoint_cadence.go:39`) is deleted, and `last_bind_millis` is
  reported in observability.

**Proof on d@e.** Each capture is followed by a published checkpoint at the
same head. A restore to it replays zero events and passes the gate.

**Rollback.** Revert, which restores the allowlist.

**Prediction for the paper:** within 72 hours it publishes at least one
checkpoint, with a held-appends time of at most 16 minutes. Probability
0.45.

### Landing 4 (orange; rollback: revert): the base cadence by restore budget R, with the measured rate persisted

Owner decision 1; default R = 120 minutes.

**What it does.**

- `snapshotReplayBound` (`snapshot_producer.go:101`) becomes R.
- The slowest replay rate measured at boot or restore is persisted beside
  the store (the slowest of the last several, so one fast run cannot relax
  it).
- The 540,000-event ceiling stays as a backstop.

**Proof on d@e.** The block shows `replay_rate_source: measured` after a
restart.

**Prediction for the paper** (over 7 days after landings 2 to 4): sealed
bytes added by its snapshots average at most 3 GB a day, with at most 0.6
captures a day. Probability 0.5.

### Landing 5 (optional): a staged compact copy is the capture's static copy

**What it does.** When compaction has staged a copy and a capture is due,
the producer:

- witnesses that copy (v3, plus the checkpoint witness after landing 3);
- uploads it as the base at the copy's sequence;
- publishes it before the planned swap.

This removes the second store-sized write of that cycle. The base is a true
state at its sequence, because it is published before the carry runs. After
landing 1 there is no carry at all.

**Bytes.** A copy-based base never dedupes against the previous live-file
base, so it costs a full F (about 6.6 GB). Take it only when a capture is
due anyway.

**Best effort.** A planned restart that comes first discards the copy before
publication, and the next capture uses scratch.

**Prediction:** at the paper's next compaction, a base is recorded at the
copy's sequence before the swap. Probability 0.4.

### Landing 0 (yellow, offline): a first reading of p before any production change

This is not one of the five. It is the C97 bench, already entered in the
ledger, and it may run on the build VM: page hashes on the compaction
bench's synthetic store at 0, 1.8, 6 and 24 hours.

Its biases relative to the paper:

- about a sixth of the size at the full hourly volume biases p high;
- two tables with bodies inline, almost no in-place updates and no deletes
  (so no incremental vacuum) bias absolute changed pages low.

It can falsify page deltas but cannot confirm them. It decides nothing in
landings 1 to 5.

## 6. Measure first

1. **The guest disk's sequential write rate**, which sets landing 2's pin.
   Measure it in landing 2's d@e proof, as copy time per GB. d@e is on the
   same host.
2. **Whether the paper's base still matches its live `agent_evidence`.** It
   matters only for a restore before landing 1. Nothing read-only answers
   it without reading the base.
3. **Restore install time for a 40 GB base** remains [?]. Time
   `snapshot.Install` against a synthetic root on the build VM.

## 7. Residual risks

- **Scratch space.** A computer with less than S plus 2 GiB free takes no
  snapshots, and says why. The data-volume bound (T7) is the guard.
- **Bind hold on the paper** after landing 3 is unmeasured until it runs.
- **The checkpoint witness's blind spots** (derived tables, ignored columns,
  index and freelist pages) are unchanged. Nothing here widens them.
- **The cold path's replay past the base is unchecked**, as today.
- **A failed cold install** leaves a non-empty state file that a retry
  refuses (`snapshot.go:878`). The operator step is to remove it.
- **Ten times the scale** needs a bounded store and an incremental base,
  neither designed here.
- **Unverified:** the `dbpage.html` citation (round 2's); whether the graph
  writer always reaches the tape for the imports; the cause of the
  `data.img` shrink; the brief's 11.5 GiB.

## 8. Review round 1 and what changed

DeepSeek, REPAIR.

| Finding | Status now |
| --- | --- |
| B1: the page root comes from pages read | Accepted. Moot now that page deltas are not recommended; kept as a requirement in section 3. |
| B2: chain base and dating | Accepted. Moot, as B1. |
| B3: the carried tables | Accepted, and wider. Answered by landing 1. |
| B4: the meter and the gate | Accepted. The cost model in section 3 is what demoted page deltas. |
| Should-fix list | Accepted. Most of it is moot with page deltas cut. |

## 9. Review round 2 and what changed

DeepSeek and Sonnet, both REPAIR. Each claim was checked in the code.

**Blocking findings:**

- **Landing 7 has no checkpoint on most computers** (both seats). Accepted
  (`checkpoint_cadence.go:27-34`, `nix/node-b.nix:1704`, 0 cadence lines on
  the paper). Landing 7 is withdrawn, the scratch copy keeps the witness,
  and the cadence becomes landing 3.
- **"Falls back to genesis replay" is false** (DeepSeek B2). Accepted
  (`recovery_plan.go:131`, `projection_base.go:170-175`). The real behaviour,
  a durable `base_missing`, is stated in section 0. No owner act exists, and
  none is proposed.
- **The page root on fulls adds nothing over seals** (DeepSeek B3, Sonnet
  1). Accepted (`snapshot.go:1008`). Dropped.
- **Landing 6 did not make existing rows replayable** (DeepSeek B4). Accepted
  (`graph_home.go:433-500`). Landing 1 moves rows into the graph and empties
  the tables.
  - **Correction:** `engineering.slots_graph` is on by default
    (`computer_setting.go:211-212`); my "both off" was wrong.
- **The paper may be ineligible** (Sonnet 2). Accepted and established
  (section 2). Landing 1 comes first.
- **A failed cold gate is unrecoverable** (Sonnet 3). Moot: landing 7 is
  withdrawn. Section 0 states the durable refusal.
- **"Restores across a write already fail" was unproven** (Sonnet 4).
  Accepted as [I], and labelled so. Landing 1's proof binds after a write,
  then restores.
- **`PlanRecovery` from the chain base double-replays** (DeepSeek B5).
  Accepted as a correction to round 1's landing 2 text. Moot now.
- **Chain liveness on the host** (DeepSeek B6). Accepted. Moot now; listed
  in section 3.

**Should-fix items:**

- §0 "12 GB, confirmed" contradicted §3: removed.
- "3 to 5 GB" was R = 60 only: the table now shows every R.
- p_1024 capped at 0.9: the range is now 0.5 to 1.
- The cold replay bound is the chain gap, not R: moot without landing 7.
- The landing 1 ring was too heavy: cut. Landing 0 is offline only.
- The gate was on an estimate: moot.
- Hash grinding is out of the threat model: moot.
- The chain link was undefined: moot.
- Landing 3 after landing 7 had no witness path: now landing 5, which keeps
  the witness.
- The compact copy is best effort, and is published before the swap: landing
  5 says so.
- Deposit mode on the cold path: moot without landing 7.
- `snapshotDefaultMaxDuration`: landing 2 splits it.
- The root cap is 256 GiB, not 396 GB: moot now.
- `CandidateBudget` replays: moot.
- The base revision: noted in the status line.
- Measured versus estimated labels: corrected throughout.
- Order (land 6 first, measurements later): adopted.
- Eligibility as evidence: section 2.
- v2 records: none are introduced.
- Split landing 7 into 7a and 7b: superseded by the scratch copy.
- The checkpoint witness does not cover the tail: section 7.
- 77 events/s is one run: landing 4 keeps the slowest of several.
- The 8-byte collision arithmetic: moot.
- The mid-chain prune: moot.
- The owner's measurement hold on `evidence.graph`: owner decision 3 and
  landing 1.

**Rejected:** nothing.

**Narrowed.** Sonnet's scratch option is adopted, but `quick_check` is not
added to it. The copy is the live file's own pages, and the witness reads
every table row. A structural fault in an index is invisible to both today.
