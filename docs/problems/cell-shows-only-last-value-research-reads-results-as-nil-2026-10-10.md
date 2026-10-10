# A cell shows only its last value, so research read live search results as nil (2026-10-10)

Status: documented, not fixed. Mutation class of the fix: `orange`
(desk kernel result display).

## Evidence

Research-only demo on staging at 37498792 (test computer
demo-arxiv-2026-10-10c, document a4a9b020…, prompt "Research the news
on and around AI for the last week."; traces in the session scratchpad
under traces/ainews1).

- Research run 9f55c0f5, 22:47:34: the cell `r,err :=
  choir.WebSearch("AI news October 3 2026 October 10 2026", 10); r; err`
  ran for 2011 ms and printed `=> <nil>`. Run 8807ef50, 22:46:08: the same
  shape with 20 results ran for 13991 ms and printed `=> <nil>`.
- 22:47:41 and 22:47:46: `u1,e1 := choir.FetchURL("https://openai.com/news/"); u1; e1`
  and a two-URL variant printed `=> <nil>`.
- Research reported "WebSearch and direct FetchURL calls returned nil
  payloads without an error" and a blocker, then ended at 22:48:17.
- Texture run 1d9c40b0 accepted that as fact. It staged a `blocker` decide
  turn and escalated to management at 22:48:57: "All source-retrieval
  paths are dead in this environment". Management run 110a746a started
  on that escalation. The document never got past its scaffold revision.

## Cause (from source)

`describeCellValue` (`internal/yaegikernel/session_loop.go`) displays
only the value of a cell's final bare expression. In `r; err` the shown
value is `err`, which is nil on success; `r` is never displayed. The
search ran (two to fourteen seconds each), so its payload was almost
certainly there; the model had no way to see it. A second trap sits
behind the first: the payload is a `json.RawMessage`, so even as the final
expression it would display as a list of byte values, not as text.

The Texture prompt documents the rule ("A cell's final bare expression is
shown in its result"); the research prompt does not. A Jupyter-style
"last value only" rule is a reasonable REPL convention, but these desks
write `a; b; c` expecting each value to be shown, and a nil shown for an
error variable reads as "the call returned nothing".

## Belief state

Before: research's web tools failed on the new build (the desks' own
claim). After: the tools ran; the display hid their results. This is the
second time today a kernel surface made a working call look like a broken
one (the first was the bare "context deadline exceeded", R2). Remaining
error field: whether the payloads were non-empty. The fix's own staging
run settles it.

## Fix direction

Show every bare name a cell lists, not only the last: after a successful
cell, each earlier top-level bare identifier is read again (reading a
variable has no side effects) and shown as `=> name: value`. Bytes that
are valid UTF-8 text, such as a `json.RawMessage`, display as text.
