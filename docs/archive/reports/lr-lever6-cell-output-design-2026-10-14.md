# LR Lever 6: Keep Bulk Cell Output Out Of The Transcript

Date: 2026-10-14. Revised after panel round 1 (claude and glm: send
back on the cut point, handle identity and poisoned cells). Station:
[LR](../definitions/choir-ledger-lr-resource-2026-10-13.md), lever 6 of
the [assessment](resource-efficiency-memory-assessment-2026-10-13.md)
(root R5). Mutation class of the change: red-adjacent. It changes every
desk's contract with its cells (runtime and prompts), but touches no
canonical write, run acceptance or provider routing. This note is green.

## The problem, measured

A desk cell's whole stdout goes into the model's transcript as the tool
result (`internal/agentcore/tools_desk.go:504-523`, `"stdout":
result.Stdout`). Two bounds apply today:
- 2 MiB per cell in the worker (`internal/yaegikernel/eval.go:134`);
- 100 KiB on every tool result the model sees
  (`internal/toolregistry/batch_executor.go:153-159`). This cut is by
  bytes, so it can split a UTF-8 sequence, and its marker offers no way
  to get the rest back.

Everything in the transcript is re-sent on every later call of the run,
so a large print is paid for again on each call until the run compacts.

On the paper computer's recent completed runs (12 per desk, read
2026-10-14 01:20 UTC through `GET /api/runs` and `/api/runs/{id}/events`):

| desk | cells | median stdout | p90 | p99 | max | cells over 8,000 | share of all stdout beyond 8,000 |
|---|---|---|---|---|---|---|---|
| research | 243 | 1,600 | 27,042 | 102,059 | 102,273 | 56 | 67% (1.63 M of 2.43 M chars) |
| management | 230 | 415 | 11,484 | 36,473 | 67,711 | 29 | 48% (0.44 M of 0.92 M) |
| texture | 61 | 420 | 6,617 | 47,306 | 47,306 | 6 | 49% (0.10 M of 0.20 M) |

The maximum, 102,273, is the 100 KiB cap at work, so the top of the
distribution is clipped. The p90 may also include value views (below),
which this change exempts.

The longest research run shows the cost. One cell printed six whole
search records (103,782 characters). The next call's input went from
7,211 to 30,221 tokens and stayed there until compaction (assessment,
R5).

The RLM design point is that the model sees references and small views
while the bulk stays in the cell's environment. The cells already keep
state between runs ("Keep findings in named variables", research
prompt). Only the print path ignores it.

## Decision

**L6-1. A bounded view in the transcript, with the whole output kept in
the session.** When a cell's printed output exceeds the bound, the tool
result carries:
- the first 6,000 characters;
- a marker line;
- the last 1,500 characters.

The marker gives the full length and the handle that returns the whole
text:

    … stdout truncated: 103,782 chars; 96,282 not shown. The whole
    output is choir.CellOut("k3f9:17") (a string); slice it or search it
    in your next cell.

The bound is 8,000 characters, the measured threshold above, and a
single constant.

**Where the cut runs (round 1, B3).** In the worker's `serveCell`
(`internal/yaegikernel/session_loop.go`), on the cell's printed output
(`res.Stdout`) only, before the value views are appended. So the
session stores the full string, and the frame transport, `tools_desk.go`,
the `EventToolResult` record and the transcript all carry the same cut
view. A cut in `tools_desk.go` would leave the full text in the runtime
process, where no cell can reach it.

**Value views are exempt (round 1).** `serveCell` appends two views that
are not prints: listed names (`showListedNames`) and a bare final
expression (`describeCellValue`). Both keep their existing 48,000-
character bound. That bound was raised from 4,000 on purpose, because a
research parent could not read its children's findings
(`problems/research-parent-cannot-read-its-children-2026-10-11.md`). A
bare final expression has no session copy, so cutting it would lose it.
The cost: a cell's value views can still reach 48,000 characters each.
That is a deliberate, recorded bound, and the after-measurement counts
results above 8,000 by kind (print or view).

**Handles name their session (round 1, B2).** Each worker `Session` gets
a random instance id at start and owns a cell counter. A handle is
`"<instance>:<n>"`. `choir.CellOut` refuses a handle from another
instance with an error that names both instances ("handle k3f9:17 is
from a session that ended; this session is m2a1; re-run the cell"). A
handle from this instance that was evicted gets an error naming the
eviction. A fresh session therefore never returns another cell's bytes.

**A cell that poisons its session is not cut (round 1, B1).** A runtime
error, panic, timeout or overflow kills the worker
(`desk_session_worker.go:356-362`), so no handle could be read. When
`Reuse` is not `preserve`, stdout passes whole up to the existing caps,
as today.

**L6-2. Where the full text lives.** In the desk's yaegi session, beside
the variables the cell already keeps. `choir.CellOut(handle)` returns it.
Retention is bounded: the last 8 truncated outputs, each no more than
the existing 2 MiB cap. That is at most 16 MiB per session in the worst
case and about 0.8 MiB at the measured p99. With up to 12 idle sessions
per computer, the worst case is about 192 MiB, inside each worker's
8 GiB address limit. One REPL serves every run of a desk
(`tools_desk.go:200-212`), so one run's cells can evict another's
handles; the eviction error says so. A session reset loses them,
and a later read of an old handle is refused by instance, as above. Nothing
is written to the store, because this is a view, not evidence. Evidence
stays on captures and reports.

**L6-3. The cut is honest.**
- Cuts fall on rune boundaries, never inside a UTF-8 sequence. Lengths
  are counted in characters (runes).
- The 100 KiB tool-result cap stays as the backstop and is changed to
  cut on a rune boundary as well. Today a stderr over 100 KiB pushes
  stdout (the last key) past the cap; the backstop's cut is unchanged in
  that respect, and stderr is rarely that large.
- The guarantee is any byte of the last 8 cut outputs of a live session,
  not any byte ever. Slicing a handle into views under 8,000 characters
  never consumes a slot.
- `error` and `stderr` are never cut by this rule. An error the model
  must see stays whole up to the existing cap.
- The marker always states the total length, so the model never
  mistakes a view for the whole.

**L6-4. The prompts say it once.** Each desk prompt's cell section gains
one rule: print summaries, such as counts, keys, the first few items or a
search hit, and keep whole records in variables. Large output is shown
cut, with its handle; when a result shows the cut marker, read what you
need with `choir.CellOut` before concluding. `CellOut` joins the
universal exports beside `Help`. This repeats the research prompt's existing "keep
findings in named variables" and does not add a budget. No desk is told
to print less than it needs: the handle gives back everything.

## The existing projection, and why not here

`__choir_tool_projection` (`batch_executor.go:92-115`) already splits
what the model sees from a durable `full_output` (512 KB cap plus
sha256). It is not used here because it writes every large print to the
store, and a desk's prints are a working view, not evidence. Evidence
stays on captures, search records and reports. The cost is stated: after
this change, the run's events hold only the cut view of a large print,
where today they hold up to 100 KiB.

## What this does not add

- No budget and no cap on model work. The model can still read every
  byte, through the handle, in a cell.
- No new record kind or store object.
- No summarizer model in the loop.

## Ways this could fail (written before the code)

1. The model needs the cut part and does not know it exists. Answer: the
   marker states the length and the handle. Tested: a cell that prints
   20,000 characters shows the marker, and `choir.CellOut(handle)`
   returns all of them.
2. Printed JSON cut mid-structure, then parsed by the model as whole.
   Answer: the marker line is outside the head, and the head is marked
   cut. Tested with a JSON print.
3. A cut inside a multi-byte rune. Tested with CJK and Arabic text at the
   boundary.
4. An error hidden by a cut. `error` and `stderr` are untouched. Tested.
5. Session memory grows. Retention is bounded at 8 outputs. Tested: the
   ninth evicts the first, which then answers with an error naming its
   eviction.
6. A session reset between cells. The handle names the old instance, so
   `choir.CellOut` in the new session refuses it and says why. Tested:
   kill the worker, then read an old handle.
10. A cell prints 20,000 characters and then panics. The session dies,
    so the output is not cut. Tested.
11. A research parent's 48,000-character findings view. It is a value
    view, so it is not cut. Tested with a bare `choir.Research` result.
7. Models re-print the whole output in the next cell to "see" it. The
   prompt rule and the cut together make that self-defeating, since the
   re-print is cut too. Measured in the after run: cells over the bound
   per research run.
8. Desks read search results only through printing. `choir.Searches.Get`
   already returns values that a cell can index. No change is needed;
   the prompt rule points at it.
9. A quality regression: stories lose evidence because the desks saw
   less. Measured: the paper's published versions over a window with
   the same method (scripts/l2a_paper_window.py), outlets per version,
   and the support-judge unsupported rate.

## Measurement

Before and after, on the paper computer, with the assessment's method
(`GET /api/costs?detail=1`, per-run input growth) and this note's stdout
sample:
- research's last-call input, median and maximum;
- input tokens per published story version;
- cells over the bound per run;
- the quality measures in failure mode 9.

Expected direction, an estimate: research input falls substantially,
because 67% of its stdout sits beyond the bound and is re-sent on every
later call. The exemption for value views makes the size of the fall
uncertain until measured.

## Order

Deploy lane position 3, after L2a and C1e B1 (one shared-runtime cutover
at a time). It is designed and reviewed now, built in its own worktree,
and deployed after B1 is verified.

## Round 2 adjudication (2026-10-14)

Accepted: claude, glm and deepseek in round 2, and gemini on the revised
note. No blocker remains. Build notes adopted:

1. **Wiring.** The store of cut outputs and the instance id live on
   `ChoirScope`, which `choir.CellOut` reads. `serveCell` hands the full
   print to a new `CellHooks` member (`KeepOutput(stdout) string`, beside
   `Refusals` and `SearchCounts`), which stores it and returns the view.
2. **The cut needs the hook.** With nil hooks, or no `KeepOutput`, there
   is no cut, because no store could serve the handle. The capsule path
   (`cmd/capsule-broker`, `capsule_go_eval`) sets no `KeepOutput`. Its
   output and its receipt's `StdoutDigest` stay uncut, so no evidence
   surface changes.
3. **Signature.** `choir.CellOut(handle string) (string, error)`. It
   never panics, because a panic would poison the session. The marker
   shows the two-value form: `out, err := choir.CellOut("…")`. It is
   registered in `surfaceVerbs` with Help text.
4. **Instance id.** 8 hex characters, so a collision is effectively
   impossible.
5. **Synchronization.** The store is guarded by its own mutex, since
   cell code can start goroutines.
6. **Rune-safe value views.** `cellValueText`'s 48,000 bound is in bytes
   and can split a rune. It is moved to a rune boundary in the same pass.
   The bound is stated as bytes.
7. **Honest wording.** "The marker always states the total length" holds
   unless the 100 KiB backstop drops stdout (a stderr over 100 KiB). On a
   context timeout, the host ships no stdout at all, as today.
8. **Refusal dedup** now checks the cut view, so a refusal printed in the
   hidden middle is appended again. That is acceptable.
