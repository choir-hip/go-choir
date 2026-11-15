# Receipts step 2: three rounds without convergence, structural assessment

2026-11-14. Recorded 08:03Z. Contract rule: three rounds without convergence,
stop patching, write a structural assessment, change the approach.

## What happened

Step 2 of the receipts change has been to two seats three times and got
REPAIR from both each time. Step 1 (readers only) was accepted by both in
round 2 and is landing.

| Round | Candidate | Blocking findings |
| --- | --- | --- |
| 1 | one-step change | not revert-safe; split into reader first, writer second |
| 2 | 40a3a1fd6 | append-time gate dead on first boot; index build unguarded; baseline not per computer; revert hole in the walk's progress |
| 3 | 46797e8c6 | the wait can never finish (burst baseline, stale baseline computer); a cancelled build re-baselines on the slow window and ratchets; the gate's only signal has no test through the append path; the revert hole is narrower but open (expand, then an older active pass) |

Reviews: scratchpad rev-receipts2 and rev-receipts3 of session
015KcU32mtuz192PQvhm33oV; the findings are restated above so this record
stands without them.

## What is the same in every round

Step 2 carries three things. Two of them converge and one does not.

1. The format-2 writer and the backfill walk. Findings shrink each round:
   round 3 has one (the legacy-row restart rule) plus wording.
2. The reader side and the index definitions. No finding since round 1.
3. An automatic controller inside corpusd that builds and drops indexes at
   boot while watching append times: a baseline, a wait, a watcher, a
   cancel, a retry. Every blocking finding of rounds 2 and 3 except the
   walk's is in this controller, and each repair added a state (baseline
   window, per-computer key, rate arm, cancel) that the next round broke.
   It is a feedback controller written in a night, with no model of its
   signal, run once in its life.

## The frame was wrong

The owner's ruling N3 says the indexes "may be rebuilt or dropped on the
live table overnight, after review, one at a time, concurrently, stopping
if the paper's append times rise". I read that as a requirement on the
binary and built a controller. It is a permission for a supervised act.
A thing done once, on one table, on one host, does not need to be
automatic; it needs to be reviewed, one statement at a time, with someone
watching the number and able to cancel.

A fresh database has no rows, so it can simply be created with the right
index. Only the live table needs the swap.

## Changed approach

- **Step 2a, code (red, two seats):** the format-2 writer and the walk
  with its own progress table. The restart rule becomes the conservative
  one from round 3: any legacy progress row newer than the walk's own
  restarts the walk from zero. Bootstrap creates the partial index when
  the receipts table is empty and never otherwise. corpusd logs one line
  a minute with the busiest computer's append count and p95, tested
  through the append path. No migration, no baseline, no wait, no cancel
  in the binary. The walk does not depend on the index state (readers
  work with either set since step 1); it waits only for free space, and
  says in its log which volume it measured.
- **Step 2b, a supervised act (reviewed script and procedure, two
  seats):** `scripts/receipts-index-swap` with one subcommand per
  statement: status, build, drop of each old index, cancel. Each runs one
  CONCURRENTLY statement with a lock timeout and refuses unless its
  precondition reads true (the partial index valid before any drop). The
  operator runs it on Node B, watches the append line, and cancels on the
  rule: p95 over the larger of twice and 250 ms above the pre-build
  reading, on two readings a minute apart. Run under N3, on a night with
  no deploy in flight.

Rollback is unchanged: step 2a reverts to step 1; the swap is reversed by
recreating the old indexes concurrently, which no reader needs.

## Remaining error

The supervised act depends on the operator's attention for three to ten
minutes of build. That is the point. If round 1 of 2a or 2b comes back
with a blocking finding of the same class as before, the class is not the
controller and this assessment is wrong.
