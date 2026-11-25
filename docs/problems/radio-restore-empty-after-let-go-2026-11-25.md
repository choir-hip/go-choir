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
post Exact refused is not a failed post (no backoff), and the store arms no
retry timer: the pulse says `write` while ops are pending and the store is
not backing off, and `radioWrite` (a mutation) posts.

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
