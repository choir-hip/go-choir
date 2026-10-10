# Self-development apply: the checkpoint never sees a quiet computer (2026-10-10)

Found by the seventh Gate 2 reality rerun (M11 probe) on staging, build
2a16a6db, disposable `computer-520c29ba…` (VM `vm-1a9c51e0…`), operation
`selfdev-72caae58…`. Written 01:19Z, while the probe is still waiting.
Problem first; no fix in this commit. Mutation class of a fix: red
(checkpoint, run lifecycle after a planned restart).

## How far it got

For the first time since S2: implementation froze a candidate
(01:07:18Z), the independent verifier recorded a **pass** (the 3e69567b
mirror fix held), the operation reached `awaiting_approval`, the probe
approved it, and materialization began with a planned restart
(`restart kind=planned reason=self_development_apply`, 01:13:51Z).

## What happens next (guest console)

- The verifier run (`run:assignment-f59a636d…`) was still iterating
  after recording its verdict (as in rerun 5). The apply restart cut it;
  boot passivated it and its assignment was cancelled; the cancel report
  (`blocker`, 01:13:54Z) reached Texture (the 028446a5 binding fix lets
  it through) and a Texture supervision run (`280c9590…`) started and
  was still iterating at 01:18Z (iteration 34+).
- Every ~47 s from 01:14:41Z the materializer retries and fails:
  `self-development checkpoint: replay completeness: reconstruct event
  chain: computer event projection repair required`, with the replayed
  head 10–24 events behind the platform head (local seq 1368 → 1541,
  platform 1392 → 1551 across retries).

## Cause (code reading)

`ReplayCompleteness` (`agentcore/replay_completeness.go`) replays the
chain into a disposable workspace and then requires the platform head to
equal the replayed head (`computerevent/appender.go`
`ReconstructInto`), and it separately refuses if the live head moves
during the probe. It is designed for a quiet computer. After a planned
restart, pre-boot work resumes by the owner's rule ("if we're rebooting
to do an update … we can resume work"), and any desk that appends events
during the checkpoint starves it.

Two contributing facts: the verifier keeps working after its verdict
(so the apply restart always cuts a live run), and a cancel report now
wakes a Texture supervision turn.

## Fix directions (to decide; red)

- A: the materializer holds new desk activations (the actor dispatcher)
  from the planned-restart boot until the checkpoint and route
  projection finish, then releases them. Work still resumes after the
  update, as the owner's rule says; it just waits for the checkpoint.
- B: the checkpoint replays up to a head captured at its start and
  compares the live state at that head, tolerating later appends. Larger
  change to a protected verifier.
- Also: the verifier's run ends once it has recorded its verdict, so the
  apply restart does not cut a live verifier.

Leaning A (smallest, keeps the checkpoint's quiet-computer contract).
