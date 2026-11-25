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

1. its native `keystate` call (a fetch to `exact-native:`) is refused
   (`prelude.js:997-1002`, `__exact_forget`), `keyState` answers
   `unreadable` with `error`, and `reconcile` answers "nothing";
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
promise rejects, and every answer waiting on it fails (the rejection is not
kept: `.finally` clears `inflight`). `demo-leftovers.ts:66-71` shares
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
- After the fix, a later answer still waits on a run another answer started
  while it is in flight (Exact supports waiting on another answer's work,
  `prelude.js:720-726`); if that run ever waits on nothing but its owner's
  timer, the waiter would fail "pending on nothing" for that one answer. No
  step of the new run waits on a timer alone.

## Failure list (tests in `mobile/AutoradioLab/settle-once.test.ts`)

- F1 the answer that starts the launch settle is let go before `reconcile`
  answers: a later answer (`answerDismissShared`, `pagesView`) still
  answers.
- F2 after F1 the launch's account work still happens: a sign-out the last
  launch left unfinished is finished by a live answer, not skipped because
  the let-go run's `keystate` was refused.
- F3 a settle that rejects is not kept: the next call reconciles again and
  answers.
- F4 no timer is armed in the settle chain at launch (the pending-revoke
  retry is bounded by each request's own `exactTimeout`).
- F5 `hostMode`: the answer that starts the probe is let go; another answer
  waiting on it still gets "native" or "none", not the let-go answer's
  error.

The tests model Exact's ownership in Bun: `AsyncLocalStorage` stands for
the answer each async step resumes under, `setTimeout` throws for a let-go
answer, and letting an answer go clears its timers and rejects its native
calls. Run on the code before the fix (`bun test settle-once.test.ts`,
Bun 1.4.2): F1 to F5 failed, F1 and F2 with the device's message
("setTimeout() is unavailable here ...", thrown at `account.ts:119` from the
settle chain at `:405`), F3 with the first call's error returned again to
the second, F4 with one timer armed by the answer, F5 with the let-go
answer's setTimeout error handed to the waiting answer. The control (two
live answers share one reconcile, the native sign-out runs once) passed.

## Fix (same lane)

- `account.ts`: the settle is shared as a value (`settledValue`). A run in
  flight is waited on by later answers, but a run that rejects is dropped
  (the next answer reconciles again), and a run whose key read threw is not
  taken by another answer as the launch's answer (after three such runs it
  is, as an unreadable key is today). The pending-revoke retry is no longer
  raced against `timeout(3000)`: its requests share a 3 s budget as their
  own `exactTimeout` (`api.ts` `Authed.timeoutMs`).
- `host.ts`: an answer that waited on a probe another answer started and saw
  it fail probes again as itself; only the decision is shared.
- `demo-leftovers.ts` is left as it is: its shared run arms no timer, makes
  no fetch or native call, and catches every error, so it cannot reject.
