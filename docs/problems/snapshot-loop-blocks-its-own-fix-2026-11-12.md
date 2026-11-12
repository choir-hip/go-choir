# The paper cannot take the snapshot loop fix, because the loop refuses every planned restart

Date: 2026-11-12, 22:35Z. Status: the paper is unblocked (host stop at 22:52Z on the owner's word, booted on 0d2f0bd46, snapshot recorded 23:28Z); the fix direction below is still open.

## Problem

The snapshot loop fix (`0d2f0bd46`) is deployed to staging (receipt 22:31Z)
with an expedite. The paper still runs `4faab153`. Every deploy refresh of the
paper since 21:34Z has been deferred, 55 times at one a minute, with

    busy ... "snapshot_in_flight":true, "holds_new_work":true

A planned restart refuses while a snapshot is streaming
(`internal/agentcore/planned_restart.go:571`, `unsafe()`), whatever the host
asks, `allow_busy` included. On the old code a failed attempt is followed at
once by the next one (the ticker has a buffered tick), so a snapshot is in
flight all the time. The defect being fixed is what stops its fix arriving.

The paper also refused the three releases before this one for the same
reason. Its last refresh was 19:46Z, in the three minutes after a boot before
the first attempt starts.

## Evidence

- vmctl journal, 21:30Z to 22:33Z: 55 `outcome=deferred` lines for
  vm-1669020a, none refreshed; the reason quoted above.
- The paper's observability at 22:31Z: `build.commit 4faab153`.
- The paper keeps publishing (174 successful gateway lines in 15 minutes); it
  holds new engineering only.

## Belief

There is no planned route through. The operator restart, the owner refresh
and the deploy refresh all end at the same guest check. What remains is a
stop of the VM by the host, which the guest sees as a crash: open obligations
are closed as interrupted, standing schedules survive, and the boot takes the
new release.

## Remaining error

Not checked: whether a gap of a few seconds between attempts exists that a
faster retry could hit. Fifty-five misses at one a minute say it is small.

## Fix direction

A streaming snapshot is not work a restart destroys: nothing is recorded
until it finishes, and it can be taken again. A planned restart should cancel
an in-flight capture and proceed, not wait for it (the 90-minute budget makes
the wait worse). That change is guest-side, so it also cannot reach the paper
until the paper is restarted once.
