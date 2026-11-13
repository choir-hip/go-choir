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

## Structural assessment after five review rounds (2026-11-13)

The repair lane (`lane/restart-cancels-capture`, tip `0baebad90`, not landed)
makes a planned restart or a hibernate cancel an in-flight capture. Its capture
half was accepted in round 1 and has not been questioned since. Every later
finding is about one file, the planned-restart marker, and the protections tied
to it (refresh hold, the producer's restart block, the telemetry hold, paused
capsules).

- Rounds 1 to 3 each found a new interleaving of the host's request, the host's
  withdrawal and the guest's own restarts. The marker had several writers and
  no owner.
- Round 4 gave the marker one owner (one file, one mutex, a source scan). Both
  seats then found that ownership of access was not ownership of meaning: a
  guest cleanup deleted the host's marker, and two refusal branches released
  protections under a marker.
- Round 5 made clears reason-scoped, refused the host's write over a guest
  marker (409 `guest_restart_pending`) and put every release in one function.
  One seat accepted. The other found two cases left: a guest write may still
  replace the host's marker, and if that guest apply then fails, its cleanup
  removes the only marker before the host's kill lands (a crash boot of an
  announced restart); and the release function checks for a marker before it
  takes the telemetry mutex, so a host request can land in between and lose its
  telemetry hold.

Belief: the marker is a shared register with last-writer-wins meaning, and
each round has converted one more pair of writers to an explicit rule. The
remaining rule is the symmetric one (a guest write is refused while a fresh
host marker is pending, as the host's is under a guest's), plus taking the
telemetry mutex before the marker check. Both are small. The lane is parked
rather than patched a sixth time on the night before the live station moves:
the defect it removes is now rare, because the snapshot fix (0d2f0bd46) ended
the capture loop and a capture is in flight only when the tape tail passes its
distance threshold.

Remaining error: while a capture is in flight (up to 90 minutes), a deploy's
planned refresh of that computer is still refused and retried each minute, and
a hibernate waits. The next step is a sixth round with the two rules above and
a two-seat review, as a background lane. The better frame, if it fails again,
is to stop sharing the file: a host marker and a guest marker as two files,
the boot reading either.
