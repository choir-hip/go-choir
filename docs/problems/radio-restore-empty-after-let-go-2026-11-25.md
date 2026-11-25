# The radio starts empty after a quick relaunch, and its writer waits 30 s at every launch

Recorded 2026-11-25 by a builder in lane `move-a`, from the native-boundary
study (Move A) and the owner's phone run of 2026-11-24 (8 launches, the
phone build `f7e16f502`: main plus the `voice-models`, `native-op-log`,
`exact-editor` and `settle-once` lanes). Fix in the same lane (orange).

## Problem

The run sheet is restored from its file (`app:/data/runsheet.json`) once per
process. `getRadio` (`mobile/AutoradioLab/radio-bridge.ts:191-196`) kept that
restore as one module-level promise, `restored = r.rt.restore()`, started by
the first answer that asked for the radio, whichever it was: the pulse
(`radioPulse`, a resource asked again every second and on every native
"ask again"), `queueView`, `barRun` or a mutation.

Exact runs every step of an async function as the answer that called it, and
refuses that answer's fetches, native calls and storage once it is let go
(asked again with new arguments) or has replied
(`$EXACT2/js/src/prelude.js:997-1040` `__exact_forget`;
`js/src/storage.js:127-137`: "storage.readFile() called outside an answer:
it was never run"). So when the answer that started the restore was a pulse,
and the pulse was asked again (native touches `radioNote` and `deckState`
during launch: `rpulse` is keyed on `ds.seq, rnote.tick, radioTick`):

1. `RadioRuntime.restore` began with a native call, `await this.deck.state()`
   (`radio-runtime.ts:391`): refused, "the answer was let go before this
   fetch"; it was caught and the restore went on without the deck;
2. `this.store.restore()` read the sheet file: refused;
   `runStorage.read` (`radio-bridge.ts:65`) turned EVERY read failure into
   `undefined`, and `RunSheetStore.restore` returned at `if (!text) return;`
   (`runsheet-store.ts:413-414`) with the empty sheet;
3. `restored` was now a resolved promise: no later answer restored again, and
   the radio said "Nothing is on air" with `ahead=0` until the app was killed.

A second, smaller defect rides the same chain. `RunSheetStore.restore` ends
with `this.flush()` (`runsheet-store.ts:426`). That post, running as the
let-go answer, was refused; the refusal looked like "offline", so
`failedPost` held all posts for 30 s (`POST_BACKOFF_BASE_MS`), and then
`scheduleRetry` armed a `setTimeout` as that answer, which Exact refuses
("setTimeout() is unavailable here"), an uncaught rejection. The desk's
writer then waited half a minute at every launch.

The pulse also read the deck through a second native call (`peek(true)` →
`deck.state()`, `radio-bridge.ts:248`) although the contract had just read
the same state through the package source `deckState` (`ds`).

## Evidence

The phone run, `unified-app.log` (shared-code console) and `timeline.txt`.
Launches 2→3→4 and 6→7 are quick relaunches; 4 and 7 are the bad ones:

```
20:45:01 launch 4
20:45:05.209 deck: deckState failed: the answer was let go before this fetch   (five lines, one millisecond)
20:45:05.209 radio: pulse ready=false started=false onAir=no write=false hold=false ahead=0 inFlight=false lost=0
20:48:04 launch 7
20:48:08.641 deck: deckState failed: the answer was let go before this fetch   (five lines, one millisecond)
20:48:08.641 radio: pulse ready=false started=false onAir=no write=false hold=false ahead=0 inFlight=false lost=0
```

No later pulse line in either launch (pulse lines are logged on change only),
so `ahead=0` held for the whole of each (90 s and 91 s). Launch 5, the next
plain launch, logged `ahead=6`: the file was intact.

The writer's delay and the timer, launch 1:

```
20:43:28.462 E Uncaught (in promise) Error: setTimeout() is unavailable here: a timer belongs to the answer that arms it ...
20:43:28.462 E     at scheduleRetry (app.js:20819:27)
20:43:28.518 radio: pulse ... write=false hold=false ahead=6 ...
20:43:58.803 radio: pulse ... write=true hold=false ahead=6 ...      (30.3 s later)
```

## Belief

The restore belongs to a mutation (`radioHold`, which nothing lets go while
it runs: one at a time, `holdFlag`), and its outcome is kept as a value on the
radio (restored or not), never as a promise another answer awaits. A read
that is refused, or that fails for any reason but "no such file", leaves the
radio not restored, and the next hold tries again; only an absent file is an
empty sheet. Until it is restored the pulse says `hold` and never `write`
(the desk is not asked to write onto a sheet not yet read), and a mutation
that changes the sheet restores first. The pulse reads the deck from the
contract's `ds` (passed as its argument) and makes no deck call of its own. A
post Exact refused (before it was sent, or in flight: "the answer was let go
before this reply") is not a failed post (no backoff), and the store arms no
retry timer: the pulse says `write` while ops are pending and the store is
not backing off, and `radioWrite` (a mutation) posts. The restore starts no
post of its own, but an op it applies (the entry left on air closed as
interrupted) posts as every applied op does, inside the hold. A read that
keeps failing as the disk's failure is not retried for ever: after a bound
the file is moved aside and the radio starts empty. One timer remains in the
store, `settlePending`'s poll at sign-out, which is not safe if its answer is
let go (the sign-out then rejects before the wipe); it is recorded, not fixed
here.

## Remaining error

- The mechanism is reconstructed from code and the log's shape; no log line
  names which answer ran the restore. The fix adds a line when a restore is
  refused (`radio: restore ... not yet`), so a device run can confirm it.
- The `ds` resource is now read once per `radioTick` (it carries the tick), so
  the deck's `sounding` line and the position still come once a second; the
  pulse is asked a second time in a second where `ds` changed. That pulse
  only reads, so being let go costs nothing but its `radioFacts` read.
  Unproved on a device.
- Not proved on a device until the lead's 8-launch script (`run.sh`) shows
  `ahead>0` in every launch and `write=true` within seconds of launch.

## Follow-up: restore the listening place and start the Desk independently

The owner reports that Autoradio opens with nothing to play, then reads the
paper for five minutes or more before a Desk segment airs. The release source
is `16f04df4d` (App Store build `473021`), which includes Move A. The phone was
unavailable during this follow-up, so the report has not been matched to a
device trace or a particular installed build. Do not attribute this instance
to the old let-go restore defect just because the empty screen looks similar.

**What the release code establishes.** The restore reads the saved queue and
reconciles the deck journal, but marks an unfinished on-air entry as
interrupted instead of resuming it (`runsheet-store.ts:406–427`). `radioHold`
awaits the public front-page fetch before starting even a restored queue;
its two-second check is after that await, not a timeout. The public fetch
allows 15 seconds, and `openRadioOf` can try again if there are still no
headlines (`radio-bridge.ts:337–349,448–454`). The opening can play before
the saved queue. Front-list fallback entries have references, not necessarily
local words; the fully cached story path is wired to writer failure rather
than consulted directly at launch.

The Desk writer is armed by the playback opener and shares a mutation with
sheet posting. Its refill gate counts the whole queue: at least two playable
entries and sufficient estimated time (normally three minutes or more) can
defer the first request. Even when the writer does run, it fetches the front
list again, awaits takes and optional story text, buffers the whole programme
response, and places fresh segments behind entries already ahead. Those are
distinct waits; none has yet been measured for the reported launch. The
proxy already wakes the selected computer on an eligible authenticated
guest request, so the client can exercise the existing wake path earlier.

**Read-only evidence on 2026-11-25.** Ten completed phone programme requests
in the live guest logs took 14.6–44.5 s to the first segment (median 26.2 s)
and 20.4–49.4 s to finish (median 32.9 s), from the route's own clocks.
These are successful samples, not a measure of failed attempts or of how
long a segment waited in the phone queue. Stock Bun suites `radio-restore`,
`radio-start`, `radio-never-waits`, `programme`, and `radio-listening` passed
58 tests against the release SHA. They cover the old startup policy: one
explicitly expects the opening to precede a saved queue.

**Proposed first slice: continuity in Exact.** Treat playback and programme
replenishment as independent clocks. On launch, settle the deck journal,
then use an account-scoped listening checkpoint (entry ID, last confirmed
sentence or audio offset, and pause intent) to resume the unfinished item.
If it is not playable, take the next playable saved entry in order. Use the
already-held story bodies before doing any network work. Keep the small
fallback supply ready for the native deck while Exact is suspended. Preserve
an explicit pause, invalidate the checkpoint on account change, and never
report an unheard entry as heard. Reuse the existing entry and forecast on
same-entry resume; if the ledger cannot reopen it, keep an honest interrupted
result and resume at the saved place as a new local continuation, without
scoring the same forecast twice. This is restoring a listener's place, not
resuming crashed computer work. For a clean install with no cache and no
network, a bundled, dated sample or station intro is the only local audio;
it must be presented as such.

**Proposed second slice: early Desk request and clean handoff.** With valid
auth and Desk consent, start the initial replenish at launch or when the
computer comes ready, in a durable Exact mutation independent of playback and
sheet posting. Use eligible cached story candidates when present; otherwise
fetch the front once. Give optional takes/text a shared deadline so they
cannot hold the first request; keep listening-only entirely local to the
phone and public paper. Use a distinct initial decision from steady-state
refill, with the amount of actually ready Desk material as the signal, not
the length of emergency paper fill. On availability changes, distinguish
computer waking from a provider refusal and honor server retry guidance.
Play the first accepted Desk item at the next natural boundary after any
explicit listener queue and playable saved programme, ahead of speculative
paper fill. Do not cut the current story or lose a user removal. The client
currently cannot consume the POST stream incrementally (Exact gaps item 36);
if measured arrival time still warrants it, try a ticketed Exact GET stream
from the same generation, after the launch and handoff fixes.

**Remaining error and proof.** There are three separate times to measure:
launch to first audible sound, launch/computer-ready to first Desk request,
and accepted Desk entry to audible playback. Record those alongside restore,
request ID, route first segment and completion, client enqueue, current item
source and fallback seconds ahead. A slow request, an unsent request and a
ready item waiting in the queue must be distinguishable. Repeat the eight
rapid relaunches, then test a real iPhone with a saved mid-sentence item, a
paused item, airplane mode, and a slow/waking Desk on the drill computer.
First-slice acceptance: resume or start the saved queue before any network
response; no empty state while playable local material exists; no replay of
finished items or migration of private content across sign-out. With a warm
voice and playable local material aim for sound within two seconds. For a
signed-in warm computer with cached candidates and no ready Desk buffer, aim
to send the initial request within two seconds of readiness and air the new
segment at the first eligible boundary. Prove lock-screen continuation as
well as the transition from fallback to Desk.
