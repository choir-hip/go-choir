# Precommitment ledger: data architecture program (opened 2026-10-28 14:35 UTC)

Companion to docs/definitions/choir-data-architecture-selection-2026-10-28.md.
It is the ledger for that goal and, as they are authored, for the goals the
program splits off (byte deletions, tape schema v2, the hypervisor bake-off,
progressive delivery, the optimization pass); each of those adds its own
records before its evidence exists. Round 0 covers the selection goal and a
first frame for the hypervisor bake-off. (Preamble scope clarified with
round 0b; no record was edited.) The mechanism and scoring rules are in
docs/precommitment-records.md, sections 2 and 5. This file applies them by
hand until desk records carry the work.

Owner ruling (2026-10-28 ~14:40): "the point is not to have performance
targets or a budget, but to have and state our expectations of what sort of
performance we can achieve, and then we build it, then we optimize... at
each step of the way, we are scoring our predictions, and as the more we
learn, we then make new predictions, and we keep calibrating... It's not
enough merely to make the system work. We also want to extract all of our
learnings."

## Rules

1. **A record is committed before the evidence that resolves it exists**,
   and committed text is never edited. A revision is a new record that
   cites the one it revises. Commit times come from `date -u`. The git
   commit that adds a record is its timestamp witness.
2. **Every record carries:**
   - a hypothesis and its assumptions;
   - a probability, or a point estimate with an 80% interval;
   - a falsifier;
   - the resolving observation;
   - what will be hard to measure, and how we mitigate that.
3. **The committer does not settle its own records.** FP0 commits. The
   settler is a rule (a measured number read from an artifact), Devin, or
   the owner, and the settlement cites the artifact. FP0 may self-score,
   and that score is recorded beside the independent one; self-scores never
   gate.
4. **Scoring:**
   - probabilities get a Brier score and a log score, with skill against a
     stated base rate where one exists;
   - 80% intervals get coverage (about 80% of them should contain the
     truth) and the interval score;
   - point estimates get the log of the ratio (observed over predicted).

   Calibration is reported with sharpness, because vaguer predictions can
   look better calibrated.
5. **Resolution classes:** confirmed, qualified, contradicted, unresolved,
   or interrupted. Interrupted means a crash or a platform failure; it is
   excluded from calibration.
6. **A discrepancy names the assumption that failed.** The learning entry
   says what will be done differently, and a later record checks that the
   behaviour changed.
7. **Re-predict at every phase boundary:** after the census, after the
   witness spike, after the d@e window, after the paper window, and after
   the differential. Each new prediction cites what was learned.

## Records (round 0, committed 2026-10-28 ~14:35 UTC, before the census)

Notation: "p" is a probability; "est" is a point estimate with an 80%
interval [lo, hi]. Every record below starts as unresolved.

### Census

**R0-01.** In the paper's captured base 5a241cb0, `computer_event_index`
(all columns) is the largest table by logical bytes.
- Estimate: 55% of all logical bytes in the base, 80% interval [40%, 70%].
- Grounds: the 10-19 census gave 3.84 of about 6.8 GiB raw.
- Falsifier: under 40%.
- Hard to measure: Dolt's on-disk bytes (chunks, compression, sharing)
  differ from logical bytes. Mitigation: record both, logical as
  `SUM(LENGTH(col))` per column and on-disk per table file where Dolt
  allows it. The prediction is about logical bytes.

**R0-02.** The paper's `event_json` alone is about 12 GiB logical.
- Estimate: 12 GiB, 80% interval [8, 17].
- Grounds: scaling the 10-19 ratio to 4.5M events.

**R0-03.** Authority state (tables and og kinds keyed by external key, not
by content hash, excluding the tape index, run memory and content-hash
kinds) is small.
- p(< 1 GiB logical on the paper) = 0.70.
- p(< 200 MiB logical on d@e) = 0.85.
- Falsifier: over 1 GiB on the paper. That is also a consensus kill
  criterion, which would mean redesigning the authority model.
- Hard to measure: some tables mix both kinds. Mitigation: classify per
  og kind through `IdentityContentHash` versus `IdentityExternalKey`, and
  list every table that has no classification as unknown, not as
  authority.

**R0-04.** Content-hash og kinds hold most of `og_objects`' bytes on the
paper: p(at least 70%) = 0.65.

**R0-05.** Memory shape at census time on the paper, with no reclaim:
- guest page cache share of host RSS: est 55%, 80% interval [40%, 70%];
- store files' share of that page cache: est 85%, [70%, 95%];
- daemon anonymous memory: est 1.6 GiB, [0.7, 2.8].

Hard to measure: guest page cache is not visible from host smaps.
Mitigation: read guest /proc/meminfo and per-file residency (fincore or
mincore) through the debug path. Where the guest cannot be read, record
the number as unmeasured and do not infer it.

**R0-06.** At least one more kind of authority state that cannot be rebuilt
from the tape turns up, beyond `agent_evidence` and `co_super_slots`, by the
end of the differential: p = 0.70.

### Mitigations already built (parallel track)

**R0-07.** The kill -9 drill for `store.debounced_checkpoint` passes on its
first run, with no acknowledged write lost: p = 0.60.
- Hard to measure: what counts as "acknowledged" for the drill.
  Mitigation: the drill's definition is the fsynced tape append (the
  amended durability gate), written down before the run.

**R0-08.** The Store A squash's peak memory falls mostly in one phase
(at least 80% of the peak in one named phase): p = 0.70. That peak is at
least 12 GiB: p = 0.65.

### Witness spike

**R0-09.** A trigger-maintained LtHash32 per-table root passes the four
named pitfall tests (REPLACE, VACUUM, ALTER ADD COLUMN DEFAULT, UPSERT) once
the mitigations are applied: p = 0.85.

**R0-10.** That trigger maintenance slows writes on a frozen d@e workload.
- Estimate: 15% slower, 80% interval [5%, 40%].
- Falsifier: over 40%, which moves the root to the single-writer path.
- Hard to measure: noise in write throughput. Mitigation: 5 runs, report
  the median and the spread, with the same tape window and a warm cache.

### Bake-off (Firecracker, d@e window, then a paper window)

**R0-11.** After the refs+CAS data model, the paper's store shrinks by at
least 50% in logical bytes on either engine: p = 0.75.

**R0-12.** Attribution in the 2x2 (peak RSS across the named operations):
- the data model accounts for at least 60% of the improvement and the
  engine for at most 40%: p = 0.60;
- the engine's share is under 20%, which is the "defer the swap" kill
  criterion: p = 0.30.

**R0-13.** Steady host RSS of a d@e-class computer on the refs+CAS model,
Firecracker, no reclaim:
- SQLite: est 600 MiB, [350, 1100];
- Dolt-small: est 900 MiB, [550, 1600].

Hard to measure: Go GC timing and page-cache warmth. Mitigation: a fixed
GOMEMLIMIT, a 30-minute settle, then 3 samples. Report the median and the
max.

**R0-14.** The worst peak RSS of any single operation (witness, checkpoint,
compaction, restore, or adversarial inputs) on the best candidate:
- est 300 MiB, [120, 900];
- p(it stays bounded, flat as input grows) = 0.80.

Hard to measure: finding the worst operation. Mitigation: the adversarial
input set named in plan step 4, and every maintenance operation run once
under a memory-capped cgroup.

**R0-15.** Harness replay throughput into SQLite on the d@e window:
- est 2,500 events per second, [400, 8,000];
- p(two runs give the same witness root on the first try) = 0.70.

**R0-16.** DoltLite survives the paper cut to three candidates: p = 0.35.

**R0-17.** Binding ratification (after the differential), mutually
exclusive:
- SQLite + refs/CAS: p = 0.60;
- Dolt + refs/CAS: p = 0.30;
- DoltLite: p = 0.07;
- other: p = 0.03.

**R0-18.** The d@e shadow differential is clean on its first full run:
p = 0.40. "Clean" means the same answers and witness over the frozen window.

### Calendar (planning-fallacy check)

**R0-19.** Time from the start of execution to the d@e differential:
est 3 weeks, [1.5, 7].
- Hard to measure: interruptions from other work. Mitigation: record the
  elapsed time and the active days separately, and score the active days.

### Hypervisor bake-off (next goal; committed now so the frame is on record)

**R0-20.** Cloud Hypervisor's idle per-VM host RSS, in the same guest
configuration, is within 15% of Firecracker's: p = 0.60.

**R0-21.** Cloud Hypervisor's virtio-mem plugs and unplugs memory on Node B,
where Firecracker's fails (issue 6255): p = 0.70.

**R0-22.** Some hypervisor configuration returns at least 1 GiB of guest
page cache to the host within 5 minutes on a busy computer, through
virtio-mem unplug or balloon inflation: p = 0.55.

**R0-23.** Porting the recovered vmbackend package to current main takes
less effort than writing a new backend; "less" means the port reuses at
least 60% of its lines: p = 0.75.

## Records (round 0b, committed 2026-10-28 ~14:55 UTC, still before any evidence)

Revisions from Devin plan review 2. Each cites the record it revises; the
round-0 text stays unedited and is scored as committed, and the revision is
scored too.

Scoring conventions added: an estimate resolves by its interval on both
sides (a value outside [lo, hi] either way counts against coverage), and the
"falsifier" lines name the decision-relevant side. Settlers by section:
census, memory shape and replay numbers are settled by rule from the
artifact (the number read by a script, cited); witness tests by rule from
the test run; attribution, ratification and the hypervisor records by
Devin from the scorecard; calendar by the owner.

**R0b-01 (revises R0-03).** Falsifier, two-pronged per the consensus kill
criterion: authority state over 1 GiB logical on the paper, or over 20% of
logical bytes on either computer. The d@e leg resolves at both 200 MiB and
20% of d@e's logical bytes: p(under both on d@e) = 0.75; p(under 20% on
the paper) = 0.80.

**R0b-02 (revises R0-12).** Attribution formula, pre-registered: on the
frozen scorecard metric (peak RSS across the named operations), model
share = mean improvement from changing the model, averaged over both
engines; engine share = mean improvement from changing the engine,
averaged over both models; each divided by the total delta from the
incumbent cell (Dolt, incumbent model) to the best cell; the interaction
term is reported separately and not assigned. Hard to measure: interaction
can be large, making shares sum far from 1. Mitigation: report all three
terms; R0-12 resolves on the two main-effect shares as defined. Same
probabilities as R0-12.

**R0b-03 (revises R0-14).** The quantity is process max RSS during the
operation minus the pre-operation baseline (operation-attributed
footprint), with process max RSS reported beside it. Same estimate and
probability.

**R0b-04 (revises R0-11).** "Either engine" means both: p(at least 50%
logical-byte reduction on both engines) = 0.70; on at least one = 0.78.

**R0b-05 (revises R0-17).** R0-17 is conditional on a binding
ratification. p(no candidate ratified binding by the end of this goal,
because of a hard-gate failure or an unclean differential that is not
repaired) = 0.15.

**R0b-06 (extends R0-13).** SQLite + refs/CAS steady host RSS is lower
than Dolt + refs/CAS on the same d@e computer: p = 0.75.

**R0b-07 (revises R0-21).** Ground: Firecracker issue 6255, open, verified
by the GitHub API on 2026-10-28, and the repo's record
docs/problems/memory-hotplug-cannot-work-on-node-b-39-bit-cpu-2026-10-22.md.
Same probability.

**R0b-08 (revises R0-22).** The tested configuration set is fixed by the
hypervisor bake-off's goal file before its runs; this record resolves on
that set, and "busy" means the census-time paper workload replayed on a
test computer, never the paper itself. Same probability.

New expectations (kill criteria and better_means items without a record):

**R0b-09.** After the byte deletions, a d@e-class computer's guest-plus-
agent floor (host RSS at idle, no store activity) is under 2 GiB on
Firecracker: p = 0.85. Falsifier: at or over 2 GiB, which means thinning
the guest, not the storage.

**R0b-10.** Restore of a d@e-class computer from its newest semantic
checkpoint needs a tape tail of under 1 GiB and under 5 s of replay:
p = 0.75. Hard to measure: the tail depends on checkpoint cadence, which is
itself a design choice. Mitigation: measure at the cadence the candidate
proposes and state it.

**R0b-11.** Write amplification (bytes written to disk per byte of event
appended, over the d@e window) on the best candidate: est 3x, [1.5x, 8x];
on the incumbent: est 25x, [10x, 80x]. Hard to measure: the guest's
filesystem and Dolt's journal both write. Mitigation: read the block
device's write counter for the window and divide by tape bytes appended.

**R0b-12.** Checkpoint cost on the best candidate (bytes written per
checkpoint of a d@e-class computer): est 20 MiB, [2, 200]; wall time est
3 s, [0.5, 30].

**R0b-13.** A kill -9 drill on each surviving candidate loses no
acknowledged write on its first run: p = 0.70 per candidate.

**R0b-14.** The census's write-rate measure shows the tape index and
content-hash kinds carry most write load too, not only bytes: p(at least
60% of rows written per hour) = 0.70.

The tape-ack latency kill criterion (p99 over 100 ms) belongs to step 6
(relay receipts) and gets its record in that goal.

Timestamp correction (2026-10-28 14:42 UTC): the round headers' approximate
times are off. By the git commits that witness them (rule 1), round 0 was
committed at 14:34 UTC (e378c3a1d) and round 0b at 14:40 UTC (f8e65155e),
not ~14:35 and ~14:55.

## Authorship and the owner's records (added 2026-10-28 14:43 UTC, commit bcbd1c096; first written as ~14:50, corrected)

Rounds 0 and 0b are FP0's predictions (the orchestrator, Claude Opus 5.5);
they are scored as FP0's track record. Owner, 2026-10-28: "keep these
predictions, note that they are yours."

**O-01 (owner).** Implementation through the d@e differential takes 1 to 3
days of agentic time ("agentic time is really fast"). Scored as an
interval against the same observation as R0-19 (active days, with elapsed
days reported beside it); R0-19's FP0 estimate was 3 weeks [1.5, 7], so
the two forecasts barely overlap and the outcome will discriminate.

**O-02 (owner, directional).** "I'm more optimistic about all the other
numbers too, but less specifically." Scored as a sign test: across FP0's
resolved point estimates and probabilities in rounds 0 and 0b, the share
whose outcome falls on the favourable side of FP0's point (smaller cost,
higher pass rate, sooner). The owner's direction is supported if that
share is above one half.

## Records (round 0c, committed 2026-10-28 about 14:54 UTC; the commit is the witness)

**R0c-01 (revises R0b-10, after DeepSeek found it inconsistent with R0-15).**
R0b-10 (tail under 1 GiB and under 5 s of replay) and R0-15 (harness replay
about 2,500 events per second) cannot both hold unless the tail is about
12,000 events or the restore path is a bulk loader faster than the harness.
FP0 prediction: the design that satisfies both sets the checkpoint cadence
so the tail stays at or under 20,000 events and restores through a bulk
loader; p(a d@e-class restore replays its tail in under 5 s at the cadence
the winning candidate proposes) = 0.65 (down from 0.75). Settler: rule,
from the restore receipt.

**R0c-02 (portability, FP0).** Source-dead restore on a second NixOS host
(leg N2) needs zero bytes from the source host on its first attempt after
the blockers named in docs/evidence/portability-inventory-and-verification-2026-10-28.md
are fixed: p = 0.55. Falsifier: any byte from the source host, which is
re-architecture trigger A4. Settler: rule, from edge byte accounting.

**R0c-03 (portability, FP0).** The same tape and release revision give
identical per-table roots on arm64 (Mac) and x86 on the first comparison:
p = 0.60. Falsifier: any differing root (trigger A5). Hard to measure: the
Mac leg can fail for unrelated reasons (signing, entitlements). Mitigation:
compare roots from a headless replay on arm64 Linux first.

**R0c-04 (portability, FP0).** d@e-class T_usable on leg N2: est 6 s,
80% interval [2, 30]; on the Mac: est 20 s, [5, 90].

## Records (round 0d, FP0, committed 2026-10-28 about 15:05 UTC; the commit is the witness)

Agent density (owner note of 15:05 in the goal file). Measured on a
d@e-class computer, Firecracker, 1 vCPU unless stated, replaying recorded
desk turns with model calls stubbed at recorded latency.

**R0d-01.** On today's incumbent, raising dispatch slots from 8 to 24 does
not raise completed turns per minute by more than 20%: p = 0.75 (the store
mutex and CPU saturate first). Settler: rule.

**R0d-02.** On the leading refs+CAS candidate, the knee of the curve
(where p99 turn latency doubles over its 4-desk value) is at est 24
concurrent desks, 80% interval [10, 64], at 1 vCPU.

**R0d-03.** At the knee, the binding limiter is CPU (not store wait, not
memory): p = 0.60; store wait: p = 0.25; memory: p = 0.10; other: p = 0.05.

**R0d-04.** Steady host RSS grows by est 15 MiB per added idle-waiting
desk, [5, 50].

Hard to measure: real model latency and provider limits dominate live
throughput. Mitigation: stub model calls with recorded latency
distributions, and report provider-bound throughput separately.

## Records (vault, DeepSeek, copied 2026-10-28 about 15:27 UTC from the accepted design)

Authored by the DeepSeek seat in docs/design/secrets-vault-design-2026-10-28.md
(revision 2, accepted by Devin round 2 at 208df8855). Copied verbatim; the
design's table is the original. Settlers: the vault build goals.

| ID | Claim | Estimate | Falsifier | Hard to measure / mitigation |
|---|---|---|---|---|
| V0-01 | `secretd` steady RSS on Node B | 45 MiB [30, 90] | > 128 MiB | Page cache vs anon; smaps_rollup, 24 h |
| V0-02 | p99 lease issue (local socket, batched audit) | 7 ms [3, 20] | > 50 ms | audit fsync; client histograms |
| V0-03 | broker-added p99 latency | 3 ms [1, 12] | > 30 ms | gateway queueing; eBPF/client histograms |
| V0-04 | portable vault slice per 1,000 secrets (1 KiB median values, 64 KiB cap) | 3 MiB [1.5, 8] | > 12 MiB | cap-heavy fixture reaches ~64 MiB; measure on a fixture, report both |
| V0-05 | 1,000 secrets restored on a second host (vault portion) | 25 s [10, 90] | > 180 s | tape/CAS fetch dominates; instrument phases |
| V0-06 | No secret values in tape segments over 7 days | p = 0.85 | one non-fixture match | detector false negatives; known-value hash scan + manual review |
| V0-07 | Crypto-erase: reads fail < 1 s locally | p = 0.95 | any success | caches; restart + cross-process check |
| V0-08 | TPM-sealed host root survives 10 NixOS updates without re-enrollment | p = 0.60 | < 5/10 | PCR changes; per-update ledger |
| V0-09 | ≥90% of gateway provider calls brokered with no key in guest by end of Q1 | p = 0.60 | < 70% | per-provider API shapes; per-provider counters |
| V0-10 | Guest-root red team cannot obtain a broker-only secret without a grant | p = 0.75 | any success | deny-by-default; red-team receipt |
| V0-11 | Owner-mediated rewrap to a Mac | 12 s [5, 45] | > 120 s | WebAuthn prompts, network; timed drill |
| V0-12 | Vault adds < 5% host RSS and < 1% CPU at 1k leases/min | p = 0.70 | > 15% / 5% | noisy host; cgroup accounting |
| V0-13 | Detector catches ≥95% of injected known values in tape scan | p = 0.50 | < 80% | regex limits; inject a corpus, measure |
| V0-14 | No secret remains in env files after cutover | p = 0.80 | any `EnvironmentFile` secret | migration discipline; grep the Nix tree |
| V0-15 | After provider/escrow migration, 100% of existing custodian wraps and privacy keys unwrap to identical key digests through the vault path | p = 0.90 | any digest mismatch or missing wrap | legacy wrap formats; dry-run replay of every Store A row before cutover, not a sample |
| V0-16 | A recovery-kit-only ceremony (no host key files, no custodian wrap) restores a test computer's private tape artifacts | p = 0.70 | any byte unrecoverable | drill UX; precommit and receipt per drill |

## Settlements

**R0-06, settled confirmed by Devin's inventory (2026-10-28, independent of
FP0; evidence docs/evidence/portability-inventory-and-verification-2026-10-28.md).**
Off-tape authority beyond agent_evidence and co_super_slots was found:
state.apply-attempts (a crash-loop bound), actor.db snapshots and poison
counters (volatile), the files tree's model policy and prompt overrides,
image blobs, and the privacy key's single copy. FP0's p = 0.70; Brier
0.09. FP0 self-score: confirmed. Learning entry below.

## Learning ledger

Entries record: the record id, the discrepancy, the assumption that failed,
what changes, and the later record that checks the change.

- **R0-06 (confirmed, not a surprise in direction, a surprise in kind).**
  FP0 expected another table; the larger findings were not tables at all:
  a key with a single copy, files and config in a second home, and counters
  that bound crash loops. Assumption weakened: "off-tape state lives in the
  store". Change: inventories search files, keys and counters, not only
  tables. Checked by: the portable-unit export set (blocker 3) must list
  every non-table item, and leg N2's edge byte accounting (R0c-02).
- **R0b-10 vs R0-15 (inconsistent pair, caught by DeepSeek before any
  evidence).** Assumption failed: FP0 predicted restore speed and replay
  throughput independently. Change: predictions that share a mechanism are
  checked against each other at commit. Revised by R0c-01.

## Records (round 0e, FP0, committed 2026-10-29 about 21:00 UTC; the commit is the witness)

From the host storage audit (docs/evidence/node-b-storage-inventory-2026-10-29.md),
before the census and before any retention change.

**R0e-01.** In the census, `event_json` plus the rest of
`computer_event_index` accounts for at least 40% of the logical bytes of the
paper's newest projection base: p = 0.60. (Consistent with R0-01's 55%
[40, 70], restated for the base, which is what multiplies on disk.)

**R0e-02.** A projection-base retention rule of the latest base per computer
plus pinned restore points frees at least 35 GB of `sha256/projection-base`
once applied: p = 0.65. Hard to measure: which bases are pinned by a
rollback or restore point. Mitigation: read the GC dry-run's live/eligible
list before the change and record it.

**R0e-03.** The paper's `data.img` exclusive bytes exceed its store's
logical bytes by at least 2x at census time: p = 0.70 (Dolt journal, oldgen
and unreclaimed space rather than live rows).

**R0e-04.** The legacy pin directories are still being written by a live
path, not only touched by metadata: p = 0.55. Settler: a count of files
newer than a marker over 24 h.

## Records (round 0f, FP0, storage needs of the new engines; committed 2026-10-29 about 22:55 UTC; the commit is the witness)

Owner, 2026-10-29: "before we move to new engines, ... estimate the real
storage needs of our new engines. thats a key precommitment." Committed
before the per-table census and before the Store A pre-split liveness
investigation reports. Target model: PostgreSQL on the host (control and
corpus as separate instances), SQLite per guest, one object store for blobs;
no engine history; pre-split leftovers dropped; the keep set only (captures,
the object graph, produced Textures, the owner's mail, plus the control state
that running computers need). Sizes are on-disk bytes including indexes and
the engine's own overhead at steady state after a vacuum/checkpoint, not
logical bytes. Inputs known at commit: Store A 6.05 M append receipts and
about 30 K other control rows; Store B 14.7 GiB Dolt with history; capture
bodies 9.4 GB; the paper's store 26.5 GB after the 10-28 compaction; the
paper's head went from 4,642,559 (10-28 21:18) to 4,940,220 (10-29 15:38),
about 400 K events a day.

**R0f-01.** The Store A pre-split tables (og_objects, og_edges,
ingestion_events, fetches, items, cycle_events, processor_requests, cycles)
have no production writer on main: p = 0.85. Settler: the liveness
investigation's code read plus last-write times.

**R0f-02.** PostgreSQL control instance, all retained control tables with
today's receipts: est 9 GB, 80% interval [3, 20]. Assumes about 1-1.5 KB per
receipt row with its indexes. Falsifier: over 20 GB, which reopens receipt
retention and granularity (the closeout's first open item).

**R0f-03.** PostgreSQL corpus instance, the keep set of Store B (objects,
edges, captures' metadata, publications, capture index): est 6 GB [2, 15].
Hard to measure: Store B's live versus history bytes are unknown.
Mitigation: the census reports both, and the estimate is scored against a
real load of the keep set, not a model.

**R0f-04.** The paper's guest SQLite after `event_json` leaves the index and
content-hash kinds move to the object store: est 3 GB [1, 10]. A d@e-class
user computer: est 150 MiB [30, 800].

**R0f-05.** The object store holding the keep set's blobs today (capture
bodies plus referenced image blobs, published blocks and file contents,
deduplicated within an owner and for public objects across owners, without
projection bases): est 12 GB [9.5, 30].

**R0f-06.** Growth at the current Autopaper rate, 30-day mean per day: control
est 0.5 GB/day [0.15, 1.5] (dominated by receipts at about 400 K paper events
a day; falls by an order of magnitude if the paper leaves the computer
tape); corpus est 0.15 GB/day [0.05, 0.6]; blobs est 0.65 GB/day [0.4, 1.2].

**R0f-07.** Total host bytes of the keep set on the new engines (both
PostgreSQL instances, the paper's SQLite, the object store; excluding VM
images, Nix, swap, backups and projection bases): est 35 GB [15, 80],
against about 330 GB of data paths on Node B today.

Hard to measure, all of round 0f: the new engines do not exist yet.
Mitigation: settle each record by loading the census-frozen keep set into
each engine on a scratch host directory (offline, memory-capped), measuring
`pg_total_relation_size` per table and the SQLite file after `VACUUM`, and
growth from a replayed day of the paper's write mix. FP0 self-scores; the
settler is the rule (numbers read from those artifacts).

Timestamp correction: round 0f was committed at 22:46 UTC (c41146911), not about 22:55.

## Records (round 0g, object storage; drafted by the Opus design seat in docs/design/object-storage-2026-10-30.md section 8, committed by FP0 2026-10-29 about 23:37 UTC before any of it is built; the commit is the witness)
**R0g-01.** Stored bytes of steps 2-4 in the local pack store (headers,
frames, pack indexes included): est 9.95 GB [9.75, 10.6]. Assumes ≤ 2%
format overhead on 9.85 GB of referenced bodies plus ~0.25 GB for the
private copies. Falsifier: over 11 GB. Artifact: `du` of the store plus the
catalog's byte sum.

**R0g-02.** Files in the store after steps 2-3: est 165 [150, 260] packs and
indexes, against 176,511 files today. Falsifier: over 1,000.

**R0g-03.** Wall time of the step 2-3 import on Node B under `nice`/`ionice`,
one writer, every body hashed on read and on write: est 15 min [5, 45].
Hard to measure: page-cache state; mitigation: drop caches before the run
and record it.

**R0g-04.** blobd on Node B, 60 KB capture-body reads by a bound guest,
24 h of production traffic: p99 4 ms [1.5, 15]; group-committed puts p99
12 ms [4, 40]. Falsifier: read p99 over 25 ms or put p99 over 80 ms.

**R0g-05.** Peak RSS of one full mark-and-sweep over 1 M catalog entries
and three root providers: est 60 MiB [25, 150]. Falsifier: over 256 MiB,
which would mean the merge is not streaming.

**R0g-06.** Object-store growth, 30-day mean after cutover at the
then-current Autopaper rate: est 0.6 GB/day [0.35, 1.1], 95% of it public
captures. Falsifier: over 1.5 GB/day. Consistent with R0f-06.

**R0g-07.** A red-team pass with a second owner's bound computer, given
every hash in the capture and og tables, reads zero private or internal
bytes and gets identical responses for absent and present private refs:
p = 0.90. Falsifier: any byte read or any distinguishable response.


## Records (round 0h, corrections to round 0g; drafted by the design seat in docs/design/object-storage-2026-10-30.md section 8, revision 2, after the tier-2 review in .agentic-consensus/object-storage-r1/; written 2026-10-29 about 23:59 UTC, committed by FP0 on 2026-10-29/30 before any of it is built; the commit is the witness)

No round 0g record is edited. Each record below names the record it
corrects and why; the corrected record stays and is scored as committed.

**R0g-08** (corrects R0g-01: its model double-counted the private copies,
since the 9.85 GB of referenced bodies already includes the other owners'
~3.8 K bodies, and the paper's captures are now platform-tier and
encrypted). Stored bytes of steps 2-4 in the local pack store (headers,
frames, pack indexes, encryption overhead included): est 9.95 GB [9.85,
10.3]. Assumes 9.845 GB referenced (9.369 capture, 0.475 og, 0.001 mail)
plus ≤ 1% overhead; the 604 shared bodies cost ~0.04 GB more if the owner
declines the Q5 default, inside the interval. Falsifier: over 10.6 GB.
Artifact: `du` of the store plus the catalog's byte sum.

**R0g-09** (corrects R0g-02: per-scope packs plus one index per pack cannot
fit [150, 260]). Files in the store after steps 2-3, packs plus pack
indexes plus catalog files: est 360 [300, 460], against 176,511 files
today. Assumes ~135 platform capture packs, ~8 og packs, ~30 private
per-owner packs, one index each. Falsifier: over 1,000.

**R0g-10** (corrects R0g-05: the provider set is now six providers, closed
in the design's section 4, and the sweep runs in a separate driver
process). Peak RSS of the GC driver for one full mark, condemn and
purge-time re-mark over 1 M catalog entries and the six providers: est
60 MiB [25, 150]. Falsifier: over 256 MiB.

**R0g-11** (corrects R0g-06: it named no class scope although step 6 adds
internal extents, and "95% public captures" encodes the revision-1 tier,
now platform). Growth of the keep-set classes (capture, og, mail) in the
store, 30-day mean after cutover at the then-current Autopaper rate: est
0.6 GB/day [0.35, 1.1], over 90% of it platform-tier captures. Falsifier:
over 1.5 GB/day. Artifact: daily catalog byte sums per class.

**R0g-12** (new scope split out of R0g-06). Growth of the whole store, all
classes, 30-day mean after step 6, with the paper off the tape: est 1.2
GB/day [0.6, 3.0]. Falsifier: over 5 GB/day. Hard to measure: base
rebuild bursts dominate single days; mitigation: report the 30-day mean
and the max day separately. If the paper's tape is still written into the
store, this record is void as stated and a new one is filed before cutover.

**R0g-13** (corrects R0g-07: platform-tier hashes are the realistic oracle,
and "identical responses" needs a measurable definition). A red-team pass
with a second owner's bound computer, given every hash in the capture and
og tables, reads zero platform, private or internal bytes, and over 1,000
probes each of absent, present-forbidden and malformed refs gets the same
status and body, with median latencies within 1 ms of each other: p = 0.85.
Falsifier: any byte read or any distinguishable response.

## Records (round 0i, after the hold model; drafted by the design seat in docs/design/object-storage-2026-10-30.md section 8, revision 5, after the round-4 review in .agentic-consensus/object-storage-r4/; to be committed by FP0 before any of it is built; the commit is the witness)

No round 0g or 0h record is edited. Revision 5 replaces condemn with an
audit of holder-scoped holds, so R0g-10's "mark, condemn and purge-time
re-mark" no longer names the mechanism. R0g-10 stays and is scored as
committed.

**R0g-14** (corrects R0g-10: the GC driver now merge-joins (ref, holder)
streams against the `holds` table, with no condemn phase). Peak RSS of the
GC driver for one full audit and one purge-time re-audit over 1 M objects,
their holds and the six providers: est 60 MiB [25, 150]. Falsifier: over
256 MiB, which would mean the merge is not streaming.

**R0g-15** (new). Hold rows after steps 2-3, written by the import from the
provider streams inside the drain: est 220 K [175 K, 320 K] (about three
blob fields per capture row over ~65.7 K capture rows, less shared
fields, plus one per og row). And the first audit after the import repairs
zero holds and pages zero: p = 0.85. Falsifier: hold rows under 150 K or
over 400 K, or any repair or page on that first audit. Artifact: the
catalog's `holds` count and the audit report.

## Records (round 0j, after revision 12; drafted by the design seat in docs/design/object-storage-2026-10-30.md section 8, revision 12, after the Codex review in .agentic-consensus/object-storage-codex/; to be committed by FP0 before any of it is built; the commit is the witness)

No round 0g, 0h or 0i record is edited. Revision 12 drops cross-owner
dedup (the owner answered Q5: dropped), so the 604 bodies that equal a
platform body get private copies. R0g-08 named that cost but estimated
under the Q5 default; it stays and is scored as committed.

**R0g-16** (corrects R0g-08: no cross-owner dedup; every non-paper owner's
capture body is its own private object). Stored bytes of the capture, og
and mail parts of the cutover in the local pack store (headers, frames,
pack indexes, encryption overhead included): est 9.99 GB [9.89, 10.35].
Assumes 9.845 GB referenced plus ~0.04 GB of private copies of the 604
formerly shared bodies, plus ≤ 1% overhead. Falsifier: over 10.6 GB.
Artifact: `du` of the store plus the catalog's byte sum.

## Records (round 0k, history, restore and audit; drafted by the design seat in docs/design/computer-history-restore-audit-2026-10-30.md section 11, revision 1; written 2026-10-30 04:21 UTC, to be committed by FP0 before any of it is built; the commit is the witness)

No earlier record is edited. R0g-11 and R0g-12 assume object storage step 6
writes only user computers' tape ("the paper leaves the tape"); the
2026-10-30 direction keeps a 7-day paper tape, so they need correcting
records in the object-storage design's next round. They stay and are scored
as committed. Every byte figure below is on Node B, in the store's own
accounting (catalog sums or `du`), unless stated.

**R0k-01** (gap G1, before any fix). Hypothesis: today's restore leaves the
embedded head at the target T while the canonical head is the
`restore_requested` event, so the next append fails and the next boot
replays past T. Assumes d@e on staging, cadence on, at least one checkpoint
behind the head. Probe: restore to an earlier checkpoint, make one owner
edit, restart, read the projection and canonical heads. Estimate: the edit
fails with `ErrNeedsProjectionRepair` and after the restart the projection
head equals the canonical head (the restore undone): p = 0.8. Falsifier: the
edit commits and the restored state survives the restart. Artifact: the
API responses, the guest log lines and the two heads, in a problem record.
Hard to measure: a restart may be deferred by a busy computer; mitigate by
probing while idle.

**R0k-02** (sealing). After the cut, a sample of 1,000 post-cutover payload
records across 20 user computers' tape segments contains no record that
parses as plaintext outside the two control kinds (lifecycle receipts and
verification decisions): p = 0.95. Falsifier: any non-control payload that
is not an envelope v2. Artifact: the sampling script's report, which prints
kinds and media types only, never content. Hard to measure: a rare kind may
miss the sample; mitigate with a full scan by media type over one day.

**R0k-03** (user tape cost). Segment bytes per event for user computers,
days 8-30 after the cut (sealed payloads, envelopes, pin receipts, segment
framing; PostgreSQL receipt rows excluded): est 2.0 KB [1.3, 3.2]. Assumes
the payload mix of tape window sizing (mean 4,657 B, about 5x zstd before
sealing). Falsifier: over 4 KB, which would mean sealing defeated
compression. Artifact: segment byte sums over event counts per computer.
Hard to measure: a few bursty computers dominate; report the median and the
fleet total.

**R0k-04** (user snapshots). Snapshot bytes held per active user computer
(active in the window) 30 days after the cut, with the section 4 cadence and
thinning: median est 45 MB [15, 150]; total across non-paper computers est
9 GB [4, 30]. Falsifier: median over 250 MB or total over 50 GB. Artifact:
the catalog's byte sum by class `snapshot-chunk` and scope. Hard to measure:
the engine's layout may change; record its version with the figure.

**R0k-05** (paper snapshots). Snapshot bytes held for the paper's 7-day
window at daily cadence, 14 days after the cut: est 20 GB [8, 60]. Assumes
a store of 11.9-20.8 GB that compresses about 3x. Falsifier: over 80 GB,
which would argue for a different chunking or a longer interval. Artifact:
catalog sum by scope. Hard to measure: chunk reuse is unknown (R0k-06).

**R0k-06** (chunk reuse). Fraction of a snapshot's chunk bytes already held
from the previous snapshot: median user computer est 0.6 [0.25, 0.9]; the
paper's daily snapshot est 0.5 [0.15, 0.85]. Falsifier: under 0.1 for
either, which would mean page churn defeats fixed-offset chunks. Artifact:
the producer's `IfAbsent` hit bytes over total bytes. Hard to measure: the
first snapshot after the cut has no predecessor; start from the second.

**R0k-07** (replay rate). In-guest replay through the appender into
SQLite: median user computer est 1,200 events/s [400, 4,000]; the paper est
600 events/s [150, 2,500]. Baseline: about 50 events/s for today's host Dolt
rebuild (2,500 events in 43 to 56 s). Falsifier: under 200 events/s for
users or under 100 for the paper. Artifact: drill logs (events replayed,
wall time). Hard to measure: the guest's load varies; take the drill
median.

**R0k-08** (restore time, request to serving). Owner rewind of a user
computer to an arbitrary head inside the window: median est 12 s [4, 45].
Broken-VM restore onto a new realization: est 100 s [45, 300]. Paper restore
to an arbitrary head inside its window: est 12 min [4, 40]. Falsifier: user
median over 2 min, broken VM over 10 min, paper over 60 min. Artifact: the
audit entries' request and `restore_applied` times, and the drill reports.
Hard to measure: few real restores; drills supply the sample.

**R0k-09** (drills). Fraction of weekly in-guest snapshot-chain drills that
pass in the first 4 weeks after the cut: est 0.98 [0.92, 1.0], and every
failure has a named cause within 48 h: p = 0.8. Falsifier: pass fraction
under 0.9, or a failure with no cause found in a week. Artifact: drill
records and their resolutions. Hard to measure: drills skipped for free
space are not passes; report them separately.

**R0k-10** (audit volume). Audit entries per day over days 1-30 after the
cut: est 300 [80, 1,500]; bytes est 0.5 MB/day [0.1, 3]. Falsifier: over
5,000/day, which would mean an act is logged per object instead of per run.
Artifact: the audit table's count and size by day. Hard to measure: the
cut itself writes many entries; exclude day 0.

**R0k-11** (anchors). Fraction of hours in days 1-30 with a complete anchor
(both RFC 3161 tokens and the git commit) within 15 min of the hour: est
0.97 [0.90, 0.995]; no gap over 6 h: p = 0.9. Falsifier: under 0.85, or any
gap over 24 h. Artifact: the anchor entries and the public repository's
log. Hard to measure: an authority's outage is outside our control; count
it, do not exclude it.

**R0k-12** (control database). Growth of the control database over days
8-30 after the cut, with paper receipts windowed: est 0.1 GB/day
[0.04, 0.3], against about 1.07 GB/day today. Falsifier: over 0.5 GB/day.
Artifact: `pg_database_size` daily. Hard to measure: other tables grow too;
report the receipt and audit tables separately.

**R0k-13** (paper archive). Growth of the paper's archived sealed segments
over days 8-30 after the cut: est 0.45 GB/day [0.25, 0.8]. Assumes today's
volume (about 310-330 K events/day) and compression close to today's 5.4x on
payloads. Falsifier: over 1.2 GB/day. Artifact: catalog sum of the paper's
archive class. Hard to measure: volume follows the news; report events/day
with it.

**R0k-14** (key delivery at every boot). Added boot latency from delivery,
p50: est 0.3 s [0.05, 2]. Boots that fail because of delivery in days 1-30,
excluding intended refusals (sovereign absent, missing escrow): est 1
[0, 8]. Falsifier: p50 over 5 s, or more than 15 such failures. Artifact:
the boot timeline and `key_delivery` audit entries. Hard to measure: a host
restart produces a burst; report it separately.

**R0k-15** (carried records). In the first d@e restore drill with records
committed after the restore point, every such record is present after the
restore with its original `commit_hash` and first receipt: p = 0.85 (given
the owner accepts open question 3). Falsifier: any record missing or with a
changed hash. Artifact: the record list before and after, with hashes.

**R0k-16** (paper record witnesses before the cut). Pre-cutover batches
that carry a commitment record, resolution or score: est 120 K
[60 K, 200 K] (127,901 records, tape window sizing, some sharing a batch,
plus resolutions and scores); bytes after re-sealing: est 0.15 GB
[0.06, 0.4]. Falsifier: over 260 K batches or over 0.8 GB.
Artifact: the extraction report (counts, bytes, digests verified). Hard to
measure: records committed in shared batches; count batches, not records.

**R0k-17** (export). `choir verify-export` on the first export of the
owner's computer after the cut passes every check (chain links, receipt
signatures, anchors, key commitments, a sample of decryptions) on its first
run: p = 0.8; wall time est 20 s [5, 120] for at most 20 K events.
Falsifier: a failed check traced to the exporter or verifier, not to the
data. Artifact: the verifier's report.

**R0k-18** (owner-act enforcement). Observe-mode owner-act receipts over
the 7 days before the cut contain no would-refuse verdict on an act the
owner intended: p = 0.6. Falsifier: one or more such verdicts, which must be
fixed before `enforce`. Artifact: the receipts query and the owner's
confirmation of each act it lists. Hard to measure: intent; ask the owner
per listed act.
