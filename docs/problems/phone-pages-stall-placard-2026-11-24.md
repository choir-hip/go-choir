# Phone pages stall on a placard; the radio stops after a segment (2026-11-24)

Status: open. Diagnostic logging lands first (lane `native-op-log`); no fix yet.

## Problem

On the owner's phone (Autoradio 1, built from lane `exact-editor`), pages show
only their placard (kind and title, `pages.contract` Placard) and never their
content, and the radio stops after a segment: the console says
`AUTORADIO ran dry ... played=true` and nothing follows. It began after the
owner opened a Texture document.

## Evidence

- The server answers every request in under 0.4 s.
- The console has no `exact: request N failed` line.
- The page's reads (`pagesOpen`: load, save, noteRecent, mirror) never move
  `pg.top`, and the radio's next fetch never starts.
- Not yet seen: which native call, if any, is open, and whether shared code is
  still running turns. Nothing logs either today.

## Belief

Two hypotheses, neither confirmed.

1. **Head-of-line blocking in Exact's ordered request sequence.** Replies are
   delivered in the order asked; a read waits for earlier sends, storage steps
   and native continuations. One native `later` call that is slow or never
   replies would hold every later page read and the radio's next fetch, with no
   failure line. Leading hypothesis.
2. **The Exact runtime thread stopped running turns.**

## Remaining error

Everything above is inference from the console. The ledger of native calls does
not exist: a `reply` that is never called leaves no trace.

## The experiment

Permanent diagnostic logging, not a switch:

- Every native call at the two entry points (`AutoradioModule.later`, the app
  module's `ModelsModule.later`) logs, with the op name only (no arguments, no
  payloads, no user data):
  `AUTORADIO native begin op=<op> n=<seq>` and
  `AUTORADIO native end op=<op> n=<seq> ms=<elapsed>`.
- A watcher every 5 s logs `AUTORADIO native open op=<op> n=<seq> for_ms=<age>`
  for each call open longer than 5 s, and a call whose reply is called twice
  logs `AUTORADIO native twice op=<op> n=<seq>`.
- Shared code logs `AUTORADIO js heartbeat n=<count> t=<clock.ms>` every 5 s
  (a contract task over a resource). It is read through the same ordered
  sequence as page reads, so it stalls under either hypothesis; read it with
  the native lines.

Reading it, on the next stall:

| native `open` lines | heartbeat | meaning |
| --- | --- | --- |
| yes, same `n` growing | stopped | hypothesis 1: that op is the head of the line |
| none | stopped | hypothesis 2 (or a stall in shared code itself) |
| none | running | neither: the stall is above the sequence (a page's own read) |
| yes | running | a slow call that is not blocking the line |
