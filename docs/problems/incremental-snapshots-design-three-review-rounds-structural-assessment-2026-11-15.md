# Incremental snapshots design: three review rounds without convergence (2026-11-15)

Recorded 2026-11-15, about 20:35Z. The design is
`docs/design/incremental-snapshots-2026-11-15.md` on
`lane/incremental-snapshots-design` (tip 3703d6c28). It went three review
rounds with two seats (DeepSeek v4.1 flash and Sonnet) and every round ended
in REPAIR from both. Under the operating contract I stop repairing it and
change the approach.

## What happened

| Round | Trigger rule under review | Blocking findings |
| --- | --- | --- |
| 1 | `(tail + λ·d)/μ >= R`, floor deleted, capture before a planned restart | burst and zero-division in λ; μ not a bound; the pre-restart capture can never run and delays every deploy; group size ignores page size; the size gate not decidable |
| 2 | `tail >= min(R·μ_min, 540000)`, no spacing | nothing spaces two successful captures, so a low μ_min loops at full duty; the μ_min file unvalidated |
| 3 | the same, with today's floor restored | the 540,000 ceiling is now behind the floor, so a long capture can let the tail pass the 720,000 boot refusal; μ_min is written before it is loaded, so the ratchet can be raised or zeroed |

Round 2's blocker came from my own round 1 adjudication (delete the floor).
Round 3's first blocker came from the round 2 repair (the ceiling folded into
the threshold).

## What converged and what did not

Converged, in every round, on both seats:

- Smaller page groups inside the existing chunk store need no new restore
  path: restore reads the cut from the record. With the group capped at
  `ChunkSize/pageSize`, no reviewer found a defect in it after round 1.
- The page-churn meter is safe alone.
- The capture before a planned restart is wrong and is deleted.

Did not converge: the trigger. Each round reviewed a newly written rule, and
each new rule dropped an invariant the present code holds:

1. the floor spaces successful captures;
2. the ceiling fires regardless of the floor, because a boot refuses a tail
   past 720,000 events;
3. the replay rate only ever lowers and is read before it is compared.

## Belief

The trigger was being designed in prose as a replacement for
`snapshotDue`, when the working rule is already in the code and the wanted
change is small: the owner's recovery objective in place of the fixed 240 s
bound, the weekly interval removed, and the slowest replay rate kept across
restarts. Rewriting the rule each round is the cause of the non-convergence,
not the reviewers and not the idea. The smaller-groups slice was held back by
sharing a document with it.

## Change of approach

1. **The trigger leaves this design.** No new trigger rule is written.
   `snapshotDue` keeps its present case order (ceiling first, then the
   interval, then the replay-time case with its floor), as it stands after
   `lane/capture-trigger`.
2. **Each trigger change is its own lane, reviewed as a code diff against
   `snapshotDue`, not as prose**, with the three invariants above as the first
   three entries of its failure list:
   - the bound becomes the stated objective (one constant, 240 s to the
     owner's 30 minutes): a number change, to be weighed against what it
     uploads;
   - the weekly interval is removed, if still wanted after the paper's first
     captures are read;
   - the slowest replay rate is persisted, loaded before it is compared,
     written only when a real measurement is lower.
   None of these blocks the others or the smaller groups.
3. **The incremental design is cut down to the meter and the smaller page
   groups** and goes to review once more as a new, shorter document. The
   three-round count restarts for it because the object under review changes.
4. **The paper's first captures come first.** The static-copy capture has
   never run anywhere (C99, C100, C103 open). Its measured duration, pin time
   and upload are inputs to every number above, and are not known yet.

## Remaining error

The churn on the paper at any group size is unmeasured. Whether a 10-minute
objective is reachable is unknown until the meter and the first captures
report. The upload at a 30-minute objective with today's chunks is estimated
at about 8 GB a day at the present rate and about 27 GB at the busy end,
from the design's own figures, not from a measurement.
