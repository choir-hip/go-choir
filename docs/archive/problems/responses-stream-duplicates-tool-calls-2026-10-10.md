# Every OpenAI-routed desk cell runs twice: the Responses stream parser appends each function call twice (2026-10-10)

## Evidence

Minesweeper demo rerun on test computer computer-abd38983b70a701109c3b04c1ee96966
(all-agents trace, 18:05Z). Both management runs (model `gpt-5.6-luna`,
ChatGPT Responses API) invoked **every** tool call id twice, back to back,
inside one model turn:

- run 376149b4: 7 call ids, 14 invocations; run e78934df: 19 ids, 38
  invocations. Texture runs on other providers: each id exactly once.
- Example: call `call_8azHXQyl4SJtGI1rzyjtvoWD`, seq 18 invoked 18:01:56.134,
  seq 20 invoked again 18:01:56.306 with the same id.
- Side effects ran twice. At 18:01:44 a cell committed a cast and a report
  (`rlm:cast:6, rlm:report:27`); its duplicate at 18:01:49 failed
  (`persist tray-2: channel cast: "texture" is not reachable by channel
  mail`). The model then read the failure and spent its next turns repairing
  a report that had already committed.

## Cause

`parseOpenAIStream` (internal/provider/provider.go) appends a tool call on
`response.function_call_arguments.done` and again on
`response.output_item.done`. The Responses stream emits both for every
function call. Kernel reproduction:
`TestResponsesStreamReturnsEachFunctionCallOnce` (call_A returned twice).

## Consequences

- Every cell of every desk routed to the ChatGPT Responses provider executes
  twice: double casts, double reports, double Texture applies where those
  desks run on it, and doubled tool-loop latency.
- Part of the "management fights the desk API" behavior in the Minesweeper
  trace is this bug: the model sees the duplicate's failure, not the
  original's success.
- `TestSelfDev*` and kernel tests never cross the provider parser, so no
  test caught it.

## Fix (red: provider surface)

Append each function call once, keyed by call id: the arguments-done event
records the call; output_item.done adds it only when no event for that call
id was recorded (some streams send only output_item.done).
