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

## Settlements

None yet.

## Learning ledger

Empty. Entries record: the record id, the discrepancy, the assumption that
failed, what changes, and the later record that checks the change.
