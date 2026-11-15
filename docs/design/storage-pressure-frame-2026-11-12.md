# Storage pressure: one computer stored many times (2026-11-12)

Class: green (design frame; nothing changed by this file). Inputs:
[clustering assessment 2026-11-11](../problems/node-b-storage-clustering-assessment-2026-11-11.md),
[storage review 2026-11-12](../evidence/node-b-storage-review-2026-11-12.md),
[chunk drain](../problems/paper-chunk-writes-drain-node-b-disk-2026-11-12.md).
Owner, 2026-11-12: deletions help now; the point is to reduce the pressure
going forward, and that work is architecture. Tags: [M] measured today,
[E] estimate, [?] not known.

## The frame

The earlier records list holders and propose a release per holder. That
frame keeps finding the next grower. A different question: what is Node B
storing?

It is storing the paper, about nine times over. The paper's live store is
21 GiB [M, guest split summary]. Node B holds for it [M]:

| Copy | GiB | What it is |
|---|---|---|
| guest image | 49.2 | the working store, plus 28 GiB not accounted for [?] |
| tape segments | 69.4 | the write log of that store, one sealed record per write |
| append receipts | 24 | the host's signed acceptance of each record |
| snapshot chunks | 53 and rising | sealed copies of the store |
| total | about 196 | 52% of the 380 GiB in use |

Node A's nightly backup then copies the tape, the receipts and the chunks
again (34 to 41 GB added a night [M]).

By doctrine the store, the snapshots and the receipts are projections or
attestations of the tape. Only the tape and the keys are irreducible. So
the bound to aim at is the tape's information content plus one working
copy and one or two snapshots, not 196 GiB.

## Three measurements that change the plan

1. **The tape appender is saturated.** 09:00 to 15:00Z today: 129,868
   events, 99.4% `projection_batch_recorded`; 126,143 (97%) landed under
   300 ms after the previous one, in 3,725 runs, 147 ms apart on average
   inside a run; 273 gaps over 2 s [M, host receipts table]. One
   serialized CAS round trip per store write keeps the appender busy
   about 88% of wall time. The guest's single write connection has waited
   8.4 million ms across 699,568 waits since boot [M, guest dump]. The
   unit of durability is one small write.
2. **Each event is expensive.** 69.4 GiB over 10.9 million events is
   6.4 KB of tape per event, and the receipts table is 2.2 KB per event
   with its indexes [M]. Many store writes are a few hundred bytes
   [M, mean payload by kind in the guest dump: loop.progress 663 B,
   desk.cell 907 B]; for those the envelope and receipt outweigh the
   content.
3. **Snapshots cost the store's size, not its change.** A snapshot about
   every 23 minutes writes about 2.3 GiB of chunks the host has not seen,
   6 GiB an hour, including while the store was wedged and unchanged
   [M]. The cause is being traced.

Also [M]: the paper's `data.img` carries the no-compress attribute, so the
49 GiB image gets nothing from the volume's zstd.

## Mechanisms, each doing several jobs

**M1. Snapshots cost the change; keep two.** Chunk the store at stable
boundaries and make a chunk's sealed bytes a function of its content and
the computer's key, so an unchanged chunk is the same file and is never
written twice. Keep the latest two verified snapshots; a superseded
snapshot's chunks are derived data and go at once, not after a 24 hour and
7 day window. Trigger a snapshot by what a replay of the tail would cost,
not every 10,000 events. Effect: about 50 GiB now, the 6 GiB an hour drain,
and most of the nightly backup volume [E]. Class red (snapshot and restore
path).

**M2. Group commit on the tape.** One event carries every store write
that queued while the previous CAS was in flight. No timer and no added
wait: it batches what is already waiting. Effect [E, to be measured]:
several times fewer events, receipts and CAS calls; better compression
(larger frames); faster replay; a faster paper, because writers stop
queueing behind 147 ms round trips; and fewer chances for the lost-reply
wedge of 2026-11-12. Class red (append path), after the appender wedge fix
lands.

**M3. Keep ephemeral state off the tape.** A crash restart never resumes
work (CLAUDE.md, Restarts). State that exists only to run the current
occurrence (in-flight marks, leases, step bookkeeping, telemetry) does not
need a canonical, receipted, sealed record. A census of which writers
produce the 400,000 batches a day is running; the share is [?]. Class red
(what the tape holds), design first.

**M4. Receipts fold to a segment root.** Past the last verified
checkpoint, one signed root per sealed segment (50,000 events) proves what
50,000 receipt rows prove. Effect: 24 GB to under 1 GB, and the nightly
19 GB database dump shrinks with it [E]. With M2 the growth also falls at
the source. Class red (chain of acceptance), two seats.

**M5. Cold tape leaves the host.** Sealed segments older than the last two
verified snapshots plus a margin move to an archive that is never pruned,
are verified there, and are read through on a miss. Effect: the paper's
local tape from 69 GiB to about 10, and local tape growth stops [E]. Open:
today the tape has two copies (Node B and Node A's backup). After tiering
the archive would be the only copy unless a second archive exists. That
needs a second location before any segment is dropped. Class red; owner
decision on the second location.

**M6. A smaller store.** Compress the large text and JSON columns in the
store with a trained zstd dictionary. A smaller store shrinks the image,
every snapshot, every compact copy (the +33 GiB transient that sets the
60 GiB floor) and replay memory. Ratio [?]: needs a table census inside
the guest first. If the store falls to a quarter, the floor can fall by
about 25 GiB, which is usable space with no deletion [E]. Class red (store
schema).

**M7. The collector reaps the dead by itself.** The destroyed-computer
sweep exists and runs only on demand. Run it from the hourly collector
with the same proof, so dead test computers never accumulate. A dry run is
in progress today under the owner's go. Class red mechanism.

**M8. Forecast and meter.** B1 and B2 of the assessment, plus chunk,
capture and update trees in the growth meter. A lane is building them.

## Order

1. Stop the chunk drain (cause first), and release dead state (today).
2. M8 and the appender wedge fix (lanes in progress).
3. Measure: the writer census (M3), the group-commit factor (M2), the
   store table census (M6), the snapshot dedupe rate (M1).
4. Build M1, then M2 with M3, then M4.
5. Design M5 and M6 with the owner's decisions.

If M1 to M5 hold, the paper's footprint on Node B falls from about 196 GiB
to about 45 [E]: image 20 to 49, tape 10, receipts 1, snapshots 5 to 10.

## Not known

- What fills the other 28 GiB of the paper's image.
- Whether snapshot chunks fail to dedupe by design or by a defect.
- The share of projection batches that are ephemeral.
- The compression ratio of the store and of batched tape frames.
- Whether capture bodies repeat across polls (21.6 GiB, rising 2 GiB a
  day by the assessment).
