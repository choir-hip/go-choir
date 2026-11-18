# Autoradio: asking the radio could go quiet, lose the answer, or show nothing (2026-11-15)

Owner QA round 1: "I tried prompting in prompt bar and using voice and it didn't seem to work right. Hard to describe exactly."

## What the trace of the code found (read, not yet confirmed on a phone ledger)

The ask path: the bar's field (`spine.contract` `sendAsk`) sends `choirAsk` (`AutoradioModule+Spine.swift`), which calls `LabRuntime.ask`
then `TurnEngine.typedTurn`; a spoken turn arrives through `HandsFree.swift` `hfCommit`. Both call `runTurn`, then `runPipelineBody`
(`TurnEngine.swift`), which streams `POST /api/autoradio/turns` (`AutoradioDesk.swift`) and speaks each sentence. The app does not use
`/api/prompt-bar` or `/api/autoradio/programme` for asking.

Where it could go quiet or lose the answer, before this change:

1. The answer was only spoken. No place held the text: the bar showed "Revised · Show" for six seconds and "Show" opened the player, which
   shows the programme, not the answer. A reader who looked away heard nothing again and could not read it.
2. A desk failure that threw (no network after three tries, no sign-in, a 401) logged an error and spoke nothing; the bar showed one line for
   four seconds. The reader got silence.
3. The bar cleared the typed text at once (`sendAsk`); if the engine refused (a turn open, the engine not running) the question was lost.
4. A typed question while hands-free had a half-heard session open left that session to commit a second question on top.
5. Two questions in quick succession could run two pipelines at once (the interrupt only fired in some phases).
6. A turn System 1 judged "nothing asked" cancelled the desk and left no trace of why.
7. Nothing said what the radio had understood: the bar's holding line was the local model's thinking-aloud line or "On it…".

## Belief and remaining error

The first four are certain from the code. Which of them the owner hit is not known: no ledger of the owner's phone was read.
The phone's first answer from saved stories, and the failure line, are new behaviour whose accuracy on a phone is unmeasured.
