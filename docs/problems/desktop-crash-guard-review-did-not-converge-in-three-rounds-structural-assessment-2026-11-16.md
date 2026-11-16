# The desktop crash guard did not converge in three review rounds: structural assessment

Recorded 2026-11-16 15:00Z. The change is `lane/crash-guard` (tip 7aa31163e at
round 3), the fix for
[the desktop reopening the window that crashed the page](desktop-restore-reopens-the-window-that-crashed-the-page-crash-loop-2026-11-16.md).

## What happened

- Round 1 (one seat): REPAIR. SDK gate failing; guard absent on a failed fetch;
  quiet period could end during mount work; one marker shared by all tabs.
- Round 2 (two seats): REPAIR, both. Those four fixed; a hold lasted one load;
  the liveness check could hang; the automatic-open hold cleared on any tap.
- Round 3 (two seats): split. Sonnet ACCEPT; DeepSeek REPAIR with three
  findings, two replayed in a browser: closing a second tab inside the quiet
  period holds every window of a healthy tab; an automatic open's hold is lost
  when the server restores the same window; the heartbeat fallback (browsers
  without Web Locks) lets a fast loop through.

The core claim has held since round 2: both seats replayed death, held, held,
reopen, death, held, reopen, proof, clean. What does not converge is the edge
of the state machine.

## Where the findings cluster

Nine of the twelve findings across three rounds have one root: **the guard
treats a page that left as if it might have died, and then needs machinery to
tell the two apart.** That one choice (mine, in the first brief: "a pagehide
must not clear a pending marker") produced the tab identity, the liveness
check and its timeout, the heartbeat fallback, the planned-reload marker, and
the false holds on a quick reload, a closed second tab and a backgrounded
phone tab. Each round fixed one of these and exposed the next.

The premise was wrong. A page whose `pagehide` or `visibilitychange` handler
runs has a live main thread. It is not the crash case. A page that is hung
cannot run the handler, and a page that is killed fires nothing. So a leaving
page can say truthfully "I am leaving alive" and take its own markers with it.

## Change of approach

Delete, not patch:

1. A page that leaves alive clears its own pending markers at `pagehide`
   (and when it goes hidden; it marks its unproven windows again when it is
   shown again). One exception stays conservative: a window with a foreground
   step in flight keeps its marker, because "the user gave up on a slow open"
   is the one leaving-alive case worth a held window.
2. With (1), the planned-reload marker has nothing left to do: remove it.
3. Remove the heartbeat fallback. Without Web Locks, every other page's marker
   counts as dead. That errs toward a hold on browsers we do not target.
4. Keep, unchanged: the durable `held` map, the single mount gate, proof by
   quiet period after the last foreground step, the armed-before-fetch gate,
   Web Locks liveness with its timeout.
5. One addition both seats asked for: a dead automatic open also holds the
   window id it created, so the server restoring that window cannot mount it.

## Remaining error

- Web Locks releasing on page death is assumed from the specification, not
  observed on iOS Safari. The first real observation will be on staging.
- A crash later than the quiet period after an app's last foreground step is
  not held. Its steps are still in the device history.
- If this approach also fails review, the next step is the smaller mechanism:
  one page-level marker and "the last load did not finish: open everything
  held", with no per-window attribution at all.
