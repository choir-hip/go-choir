# The Texture root digest had two authorities for one fact (2026-11-20)

## Problem

The root desk's digest decided what the master still had to read from a pending set and an armed nonce held in
process memory, while the digest itself was a durable actor-log message that survives restarts, retries and
redelivery. Two authorities (memory and the log) answered "what is pending, and is this the armed digest" and
disagreed whenever the process, the activation or the delivery did not go the straight way. Four rounds of review of
the change that removed `texture.root_desk` each found a new disagreement; each patch added a flag (`full`,
`fullRefreshed`, a boot id, a nonce) to reconcile them, and the next round found the seam between the flags.

## Evidence

| Round | Finding | Which two disagreed |
| --- | --- | --- |
| 1 | The digest carried a document snapshot; a second digest re-ran documents already digested | message vs pending set |
| 2 | Resumed digest always ran a full refresh; two orphan digests made two runs; the armed digest buffered in a dispatcher activation was discarded when the handler deferred, leaving the nonce armed for 70 minutes | in-memory arm vs the durable send |
| 3 | `fullRefreshed` was set when the run was created, not when it succeeded; K planned restarts made K full refreshes; an orphan digest started a run for a newly pending document without waiting the hour | in-memory flag vs the run's outcome and the boot |
| 4 | The handler cleared `armedNonce` before fallible calls whose errors the actor handler turns into a deferral, so the dispatcher's own retry was dropped by the nonce check | nonce vs the dispatcher's retry |

Rounds 2 to 4: review of `62e9ac65c`, `6212fc876` and the commit before it (Gemini and DeepSeek seats; findings quoted
in the lead's briefs). A crash also lost the pending set outright (documented as known, not fixed).

## Belief

The fact "what has the master digested" is one thing and belongs in the store, written when the root run commits its
revision. Then the digest need carry nothing, and its handler is a pure function of the store: every duplicate, late,
retried, resumed or orphaned digest recomputes the same answer, and "nothing changed" ends with no run. What remains
in memory is one value, the time a digest is armed, and its loss or staleness can only cause one spare digest or one
late one, never a lost document. This is the substrate fix (contract rule 2) and the change of approach after three
rounds without convergence (rule 7).

## Round five and the decisions of 2026-11-20

Both seats accepted the one-authority structure and sent back its boundary conditions: the watermark was the run's creation time, taken after the
reads (a change in between was in neither the ledger nor pending); a read that failed moved the watermark past an unread document; more than
50 changed documents were dropped silently, and stories took the slots; K planned restarts stacked root runs because the hour bound lived in
memory; every ordinary run listed and decoded every document object (about 35,000 on the paper) to order 4 KiB; a root run that committed nothing
cost a model call an hour; a malformed watermark wedged every digest. The lead's decisions, all built: the cut-off is taken before the first read
and carried in the run; the revision records the documents it read at a version mark, and the next digest looks one minute behind the cut-off;
the root desk **does not digest stories** (a decision for the owner to confirm: they have their own index and publication path); the oldest
changes are read first and the watermark never passes one left; the hour bound is read from the store with a doubling wait after runs that move
nothing; the excerpt takes its order from the master's own recorded documents.

## Remaining error

- A root run that commits no revision moves no watermark; the doubling wait (1 h to 24 h) bounds the model calls on a busy computer and the
  repeat on an idle one needs a note.
- A document that cannot be read holds the watermark behind it, so each digest scans back to it (never past 14 days) and re-reads what was
  not recorded (the marks carried are capped at 200).
- With no watermark (a fresh master, or 32 owner edits on it) the first digests read the oldest changed documents first, 50 a run, an hour apart,
  until all of the last 14 days are read.
- The version mark is the head revision and the trajectory's version and reducer sequence; a change that moves neither is not seen.
- A digest sent for an activation that later rolls back is a spare digest, ended by the watermark.
- The rebase notice still wakes the master after a revert (`rebaseRoot` does not read a setting): close the master's standing item as part of a rollback.
- Not tested: a master missing after a replayed create (no route in this code deletes a document).
