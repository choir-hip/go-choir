# One let-go answer fails every shared-code request until the app restarts

Recorded 2026-11-25 by a builder in lane `settle-once`, from a read-only
diagnosis and the owner's phone log. Fix in the same lane (orange).

## Problem

Every source the phone's shared code answers goes through `answer`
(`mobile/AutoradioLab/app.ts:272-281`), whose first step is
`await settleAccount(storage, store)`. `settleAccount`
(`mobile/AutoradioLab/account.ts:402-409`) kept ONE promise for the life of
the process in the module variable `settled`:

```ts
settled = (async () => { await forgetOldCopy(storage); const r = await reconcile(...);
  await Promise.race([retryPendingRevoke(...).catch(() => undefined), timeout(3000)]); return r; })();
```

`timeout` (`account.ts:119`) is `new Promise(r => setTimeout(...))`.

Exact refuses `setTimeout` when the answer running is let go (asked again
with new arguments), has replied, is background work, or is a mapper
(pinned source `$EXACT2/js/src/prelude.js:1352-1366`, `timerOwner`). The
bundle's async helper resumes every step of an async function under the
answer that called it (`prelude.js:230-241`, `__exact_async_context`), so
every step of the shared chain runs as the FIRST answer that called
`settleAccount`. When that answer is let go before `reconcile` finishes:

1. its native calls are refused (`prelude.js:997-1002`, `__exact_forget`);
   on the phone the key itself is read from Exact's secret store, so the
   reconcile may well finish;
2. `timeout(3000)` then throws "setTimeout() is unavailable here ...";
3. `settled` holds that rejected promise for the life of the process, and
   every later answer awaits it and fails with the same message.

A failed resource keeps its last value (`docs/reference.md:3156` of the
pinned Exact), so pages stay on their placard: `pgs` (`pagesView`) fails,
`pg = pgs` never gets a top page, and a page mounts only when its address is
`pg.top` (`spine.contract:20-21`, `pages.contract`).

The same shape is in `host.ts` (`inflight`, lines 9-45): a shared probe
loop whose `setTimeout`s (`:16`, `:33`, `:37`) belong to the answer that
started it. If that answer is let go, the loop's next timer throws, the
promise rejects, and every answer waiting on it fails; if the answer is let
go while the loop sleeps, its timers are dropped and the promise never
settles, so every later answer fails "pending on nothing" (seat 2, round 1
review, A1). `demo-leftovers.ts:66-71` shares
`attempt` the same way but arms no timer, does no fetch and catches every
error, so it cannot reject.

Exact's own guidance (`prelude.js:742-748`): an answer that awaits a promise
another answer started waits on work it does not own; share the resolved
value, not the promise.

## Evidence

- Owner's phone, 2026-11-24, `phone-console-5.log` (session scratchpad
  `569bd20c-…/scratchpad/phone-console-5.log`): 116 lines
  `exact: request N (<src>) failed ... setTimeout() is unavailable here`,
  the first at line 39, right after the launch's first native ops, for `hc`,
  `preqd`, `pgs`, `th`, then `bi`, `rp`, `learnt`, `hb` and the rest, and
  still at lines 223-225 (`rpulse`, `acs`, `micperm`) just before the app
  exits. No shared source succeeds after the first failure in that process.
- Owner, 2026-11-25: pages sometimes show only the placard; they "did load
  after some time... it seems random"; a relaunch also fixes it.
- `acs` and `hc` take arguments that change several times in the first
  second (`account.contract:47`: visibility, focus, sign-in phase, lock
  tick; `spine.contract:13`), so the first answer being let go is the
  ordinary case at launch, decided by timing.

## Belief

The first answer to call `settleAccount` after a launch (or after a sign-in,
which resets it through `adoptKey`) is let go before `reconcile` finishes in
some launches, depending on how fast the native `keystate` answers against
how fast the contract re-asks that resource. In those launches every shared
source fails for the rest of the process. That is "random".

The "after some time" recovery: nothing inside a process clears a rejected
`settled` except `resetSettled`, called only by `adoptKey` (a sign-in). So
the page loading later is a new JS runtime: iOS ending the backgrounded
process and the next open being a cold launch, a runtime Exact replaced
(heap exhaustion destroys the module's runtime and the next request loads a
fresh one, pinned `docs/reference.md`, "64 MiB heap"), or a sign-in. A
stalled (not rejected) chain is not the mechanism: once let go, the chain's
fetches are refused at once and its remaining step throws, so it rejects
rather than waits. The log shows no recovery within its process.

## Remaining error

- Which of the three restarts accounts for the owner's "after some time" is
  not observed; the log covers one process.
- The let-go timing is inferred from the contract and the prelude; no log
  line names which answer started the chain.
- The residual sites at the end of this record.

## Round 1 and why it was sent back (2026-11-25)

The first fix (`74f41b48b`) kept the settle as a value but still let the
answer that asked first own the run, and kept `host.ts`'s shared probe.
Seat 2 sent it back (review in the lane's session): (A1) a shared probe
whose answer is let go while it sleeps has its timers DROPPED
(`timers.drop`, `prelude.js:1041`, `time.js:234-238`): nothing settles
it, and every answer awaiting it fails "the answer is pending on nothing"
(`prelude.js:719-752`); (A2) a run whose owner is let go during the native
wipe turns the refused call into `false` and was kept as a finished run
with `failed:['native']`, the same poisoning carried by a value; the
comment "the run arms no timer of its own" was false (`nativeOf`,
`announceGeneration`, the picks mirror's delay and the run sheet's
`settlePending` all arm timers in the wipe path); the Bun model lacked the
dropped timer and "pending on nothing", so the tests could not see A1 or
A2. The lead's call: this is a class, work shared between answers by
sharing a pending promise; fix the class.

## The class

Exact runs each step of an async function as the answer that called it
(`prelude.js:230-241`). Once that answer is let go (asked again with new
arguments, a mutation sent again, a `refresh`) its fetches and native calls
are refused or rejected (`prelude.js:997-1034`, `fetch.js:207-212`), new
timers throw (`prelude.js:1352-1366`) and its armed timers are dropped
(`prelude.js:1041`). So a promise one answer started and others await can
reject for all of them, or never settle; an answer whose only wait is such
a promise fails "pending on nothing" (`prelude.js:741-752`), unless it has
a timer of its own armed (then the runner asks it again at that timer,
`prelude.js:719-726`). Exact's own guidance at that line: share the
resolved value, not the promise.

## Fix (round 2)

The mechanism, chosen from the pinned Exact:

- **One answer owns once-per-launch work, and nothing lets it go.** The
  account settle is the `accountSettle` resource (`account.contract`
  `acctSettle`), with constant arguments, so the runner never asks it again
  with new ones; it is `transient` (each launch settles). It is asked again
  only when it failed (`task settleAgain when failed(acctSettle)`, a second
  later) and after a sign-in (`mutation signedRun ... refreshes
  acctSettle`; `adoptKey` resets the value). The demo-leftovers pass runs
  inside it, before the value is published.
- **Every other answer waits for the VALUE with a timer of its own.**
  `app.ts` `answer` calls `awaitSettled()` (`account.ts`): it reads the
  published value; if there is none it races a resolve-only wake against a
  timer it arms itself (250 ms), and looks again. It never depends on
  another answer's promise, and Exact always has its own timer to wake it
  by. (A contract `when` on every resource would also work, `resource r =
  s(args) when cond`, "idle while cond is false, asked again, forced, when
  it turns true", `contract-for-agents.md:1084`; it needs an edit to some
  sixty resources and does not reach mutations or resources inside
  components, so it is not used.)
- **A run publishes only if it was neither refused nor superseded.** A
  native call refused because the run's answer was let go marks the run
  (`api.ts` `isRefusal`), and each run has a number: an older run that
  finishes after a newer one started does not publish.
- **`host.ts`**: no shared probe; each answer probes for itself with its
  own timers and native calls, and only the decision is shared.
- **`notebook-run.ts`**: the read is kept as a value per key; an answer
  with no value reads for itself; a read with a refused request is not
  kept.
- **Reads shared by several answers** (`paper.ts` front list,
  `reader.ts` story fetch, `api.ts` computer discovery): a refusal is
  rethrown, not turned into an error state, and an answer that joined a
  read another answer started reads for itself when that read was refused
  (`api.ts` `joinOrRedo`). Reads only: a write is never repeated.
- **A refusal is not a fact**: `mail-seal.ts` no longer keeps "no cipher"
  for the session from a probe that threw or was refused; `api.ts` sends
  the boot announcement again when it was refused; `clips.ts` asks the clip
  set again; `consent.ts` pushes the agreement again; the picks mirror
  reports a refused push as not sent, so it is pushed at the next open.
- **The gate**: `mobile/AutoradioLab/shared-promise-gate.test.ts` fails
  for any module-level binding, class field or type member of the shared
  modules that holds a Promise (or a Map or Set of them) and is not on its
  allow-list, by file, line and name; each entry carries its kind
  (`storage`, `unawaited`, `value`, `redo`, `residual`) and a one-line
  reason, and an entry whose site is gone fails too.

## Tests (`settle-once.test.ts`, model `testkit/exact-answers.ts`)

The model now has the two missing rules: a let-go answer's timers are
dropped and what awaits them stays pending, and an answer awaiting work it
does not own fails "pending on nothing" when it has no timer of its own and
nothing is in flight (another answer's timer does not count,
`prelude.js:719-752`). A native call held back by the test and then let go
is withdrawn (never reaches native, `prelude.js:1017`).

Before round 2 (the round-1 code, Bun 1.4.2): 1 pass, 9 fail. Behaviour:
W1 (A2: the waiting answer answered on the run whose native wipe was
refused), A1 (the waiter failed "the answer is pending on nothing"), N1
(the next answer got the let-go read: `offline`), M1 (the probe's refusal
was kept as "no cipher"), B1 (the refused boot announcement was never sent
again). F1, F2, F4 and C1 failed for want of the `accountSettle` source.
F3 passed. After: 10 pass. The gate on the round-1 code listed
`account.ts settling`, `demo-leftovers.ts attempt`, `notebook-run.ts`
(the memo's `read`) and `host.ts inflight`.

## Residuals (on the gate's allow-list as `residual` or `radio`; not proven safe)

For each non-radio entry, what an answer that joined the work sees was
checked by reading the code, not by a test. The radio entries were not
examined (lane move-a replaces the radio).

- `content-run.ts` `running`: the content sync; an answer that joined a
  run refused by its let-go owner sees "offline" for that key once.
- `files.ts` `resolved`: opening a file as Texture is a POST; it is not
  repeated for the joining answer, which gets one failure.
- `podcast.ts` `inflight`: a feed load; the joining answer sees the feed it
  had. (`attempted` also keeps the feed from being asked again this
  session after a refused load: not fixed.)
- `retrieval-run.ts` `answered`: an ask answered once; a joining answer
  gets `ok:false` once. Separately (not the class): the indexing loop's
  sleeps are armed under the answer that kicked it, so once that answer
  replies the loop's next sleep is refused and the loop ends.
- `steer.ts` `flushing`: the steer POSTs; a joining answer sees "offline"
  once; the next flush retries.
- `clips.ts` `asking`: a joining answer sees no clips once.
- `reader.ts` `decoding`: one decode at a time; a link run as an answer
  that was let go fails that decode once.
- The radio (`radio-bridge.ts` `restored`, `radio-runtime.ts` `chain`,
  `radio-writer.ts` `task`, `runsheet-store.ts` `flushTask`): governed by
  `radio-pulse-rule.test.ts` (pulses read, mutations work); not changed
  here.
- `edition-writer.ts`: a let-go `obm` answer could leave `status
  'writing'` only with a writer that waits on the network; the production
  writer (`localWriter`) answers within the turn. Not changed.
- Concurrent settle runs: an older run whose answer was let go still runs
  its storage steps (the wipes) beside the newer one; it never publishes,
  and the stamp and marker are written only when its native step
  succeeded, which a refused one does not.

## Round 2 review and what changed (2026-11-25)

Seat 2 (round 2) found the round-1 defects fixed, and one regression:
a key read that THREW (`api.ts` `keyState` answers `{state:'unreadable',
error:true}`) went through `decide` as "nothing" and was published for the
process, so the one-second retry never ran. Now a settle whose key read
threw does not publish (it fails; the contract asks again a second later),
and only after `KEY_ERROR_RUNS` (3) such settles in a row is "nothing"
published, logged (tests K1, K2: both failed on the round-2 code, pass
now). Also: `awaitSettled` has a deadline (10 s of its own timers; then it
answers "nothing", which never wipes or adopts, and logs; test D1 timed out
before, passes now); a test (D2) fails if the contract stops asking the
settle; the gate's header and the reference page now say exactly what the
scan sees; the radio entries are marked `radio`, not examined; `isRefusal`
is pinned to the runtime's refusal messages (R1, R2); a refused boot
announcement clears `bootSent` only if it is still its own.
