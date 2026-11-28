---
date: 2026-11-28
title: a URL typed into the bar is decided in shared code, so it opens on Android too
class: orange
---
What: station E5. `BarURL.detect` (Swift, `PlaybackSupport.swift`), the app module's `barWebURL` setter and the early branch in its `choirAsk` are gone. The rule is `barUrlOf`, a pure `fn` of the field in `AutoradioLab/spine.contract`, derived as `askUrl`. `sendAsk` decides in the same step as the send (Exact `fn` and `derive`: contract-grammar.md, "Standard functions and intrinsics" and the `derive` rule that an action reads state as it was at its start): an address goes to the radio's `queueAct("openWeb", askUrl)`, anything else to `choirAsk(text=ask, ...)` exactly as on main, and `ask` is cleared after the chosen send. No source, no round trip: nothing can be pending, overwritten by a second Send, or answer nothing and lose the question. The radio's `openWeb` and `closeWeb` verbs from native are deleted from `radio-bridge.ts` (the bar was their one sender). An earlier version of this change asked a TypeScript op first; a reviewer found that a pending or failed answer dropped the question, and the op is gone. Native-boundary gate: 3,066 to 3,036 lines, 15 names.

Why: whether a typed string is an address is page logic; Android's module answers `choirAsk` "not supported", so a URL typed there did nothing. Now one rule serves both and the page opens in the bar's web layer, which Android has.

Differences from the Swift rule, each decided: ports are not range-checked (accepted; the page's own load fails on a bad one); `http(s)` addresses and bare words containing `^ { } | \` are refused, as Foundation refused them (matched); a bare internationalised host such as an accented `.de` is accepted, as before, and is not punycoded here (the web layer does it); a top level of non-Latin letters with no case is accepted (Swift refused: accepted); U+0085 counts as white space (matched); an upper-case ASCII top level is refused as before.

Proof: `bar-url.test.contract` R1 to R8 (the rule, through the derive, `bun exact.mjs test web`); `bar-url.test.ts` S1 to S5 (the shape of `sendAsk`) failed on the first version of this change (S1 to S3) and pass now. Send itself cannot run headless: the web host holds no key, so signed in cannot be made, and `answer source` is skipped on web; so routing is held by reading `sendAsk`, and a device must still show: typing `choir.news` in the bar opens the page in place, a sentence asks, two quick Sends both act. The Swift edits in `exact-package/apple/AutoradioModule.swift` and `AutoradioModule+Spine.swift` are compiled in the app build, not by `swift test`.

Replaced: `BarURL`, `barWebURL`, the `openWeb` and `closeWeb` verbs of `RadioBridge`.

Undo: revert the commit.
