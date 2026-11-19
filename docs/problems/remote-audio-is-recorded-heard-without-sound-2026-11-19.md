# Remote audio that reaches its end is recorded as heard, sound or not (2026-11-19)

## Problem

The phone reports each run-sheet entry's fate to the listener's computer as
an `aired` op, and "heard, finished" settles the forecast made for that
segment (the learning loop's outcome stroke). For remote audio (a podcast
episode, a clip) the Swift player reports "heard, finished" whenever the
end of the audio is reached: `RunSheetPlayer.remoteEnded` calls
`finish(id, played: true, …)` unconditionally
(`mobile/AutoradioKit/Sources/AutoradioKit/RunSheetPlayer.swift`, on main).
Reaching the end is not hearing: a seek past the end, or an end reached on
a silent route or at zero volume, is recorded as heard through.

Speech entries are different: their verdict comes from a played flag the
speech engine sets from rendered audio.

## Evidence

- Found by review seat 1 of the deck change on 2026-11-19, reading the
  frozen integration commit `e60724280`; confirmed by the lead on `main`
  (`remoteEnded`, the `played: true` argument).
- The same review found that the check "is sound being rendered" for
  remote audio (`rendering()` in `ChoirPlayback.swift`) is a test of the
  playhead's position, not of output, so a clip that starts mid-file counts
  as rendering before any sound.
- Not measured: how many `aired` ops on the owner's computer came from
  remote audio, and how many of those ended by a seek.

## Belief

Real, and small in count today: most of what the radio plays is speech,
and a listener rarely seeks to the end. It matters because it is a false
"heard": the one kind of error the learning loop cannot recover from by
itself, since the forecast is settled as correct.

## What would fix it, and the remaining error

For remote audio, "heard" needs observed progress through the audio at
normal rate (played time, not position) on a route that is not silent; an
end reached by a seek is "skipped" with the position. The deck's journal
(the Exact cutover) already carries a `rendered` flag for this, but at
`e60724280` the Swift player, not the journal, decides what is posted, so
the rule there does not yet hold. No fix is built. Whether zero volume can
be observed at all on iPhone (the system does not report the hardware
volume to a background app reliably) is not established; a muted route
may have to stay a named residual.
