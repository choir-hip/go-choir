# Autoradio: the app's logic is a Swift package, not Exact, so nothing is shared with Android (2026-11-18)

Status: open. The rule is now in the contract (CLAUDE.md, Product: "The phone
app is Exact; only voice is native"). The move has not started; a design is
commissioned. Class of the move: orange per page, red where it carries
sign-in, the mail store or the run sheet's posted ops.

## Problem

The owner's intent is one codebase in Exact for iPhone and Android, with only
voice native. What was built is Exact pages over a Swift application. An
Android build today would draw the pages with nothing behind them.

## Evidence (measured on `lane/autoradio-integ20`, 2026-11-18)

| Part | Size | Language |
| --- | --- | --- |
| Pages | 50 contract files, 7,742 lines | Exact contract |
| App glue | 8 files, 4,884 lines | TypeScript in the Exact app |
| Logic behind the pages (`mobile/AutoradioKit/Sources`) | 157 files, 38,149 lines | Swift |
| Binding (`mobile/exact-package/apple`) | 22 files, 3,136 lines | Swift |
| Interface (`mobile/exact-package/exact.json`) | 181 names, 62 with a declared substitute | JSON |

The Swift in `AutoradioKit` by what it does (by file name, so approximate):

| Lines | Share | What |
| --- | --- | --- |
| 10,086 | 28% | page models, onboarding, settings, the bar, demo data, the rest |
| 8,964 | 25% | voice, audio, speech, models |
| 4,652 | 13% | server client, sign-in, sync, content |
| 3,220 | 9% | run sheet, queue, player logic |
| 2,959 | 8% | files, reader, books |
| 2,061 | 6% | podcasts, clips |
| 1,646 | 5% | search, embedding, retrieval |
| 1,337 | 4% | takes, notebook, learning |
| 1,167 | 3% | mail |

The package has an Apple implementation only. On Android an Exact package is
a Kotlin class (`host/android/README.md:796-806` in the pinned source); ours
has none.

## Belief

The native package is the only way Exact gives an app the audio session
(`docs/reference/exact/native-modules.md`, "Two ways in; only one reaches the
audio session"). Voice went there for that reason. Each later feature was
added where the code already was, with builders briefed in Swift and tested
by a Swift suite, and no rule said otherwise. The lead did not flag it.

What it has cost already: a second copy of the run sheet reducer (Go and
Swift) kept in step by fixtures; an interface of 181 calls in which a field
the declaration does not know empties a page; app-target Swift that no suite
compiles; long builds.

## Not known

- What Exact's shared layer can carry: network requests with our headers, a
  server-sent stream, durable storage, a store encrypted at rest (the mail
  rule), background work, crypto for the sign-in key. The move depends on
  each; none has been read from the source for this purpose.
- What Android does with an interface name that has no implementation and no
  substitute (119 of the 181): refuse the build, refuse the call, or answer
  empty.
- Which of the retrieval and embedding code is "voice" (it feeds the
  on-device model) and which is app logic.

## Next

1. A design for the move: the 181 names sorted into native (voice) and
   shared, what the shared layer offers, and an order by page with the
   existing fixtures as the specification.
2. The Android toolchain proved on Exact's own sample app, then the app with
   Android enabled, to see what the pages do with no native side.
3. Until a page is moved, repairs to its Swift are allowed; new features in
   Swift are not.
