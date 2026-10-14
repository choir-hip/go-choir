# LR Lever 6: Keep Bulk Cell Output Out Of The Transcript

Date: 2026-10-14. Station:
[LR](../definitions/choir-ledger-lr-resource-2026-10-13.md), lever 6 of
the [assessment](resource-efficiency-memory-assessment-2026-10-13.md)
(root R5). Mutation class of the change: red-adjacent. It changes every
desk's contract with its cells (runtime and prompts), but touches no
canonical write, run acceptance or provider routing. This note is green.

## The problem, measured

A desk cell's whole stdout goes into the model's transcript as the tool
result (`internal/agentcore/tools_desk.go:504-523`, `"stdout":
result.Stdout`). The only bound is 2 MiB per cell
(`internal/yaegikernel/eval.go:134`). Everything in the transcript is
re-sent on every later call of the run, so a large print is paid for
again on each call until the run compacts.

On the paper computer's recent completed runs (12 per desk, read
2026-10-14 01:20 UTC through `GET /api/runs` and `/api/runs/{id}/events`):

| desk | cells | median stdout | p90 | p99 | max | cells over 8,000 | share of all stdout beyond 8,000 |
|---|---|---|---|---|---|---|---|
| research | 243 | 1,600 | 27,042 | 102,059 | 102,273 | 56 | 67% (1.63 M of 2.43 M chars) |
| management | 230 | 415 | 11,484 | 36,473 | 67,711 | 29 | 48% (0.44 M of 0.92 M) |
| texture | 61 | 420 | 6,617 | 47,306 | 47,306 | 6 | 49% (0.10 M of 0.20 M) |

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
the session.** When a cell's stdout exceeds the bound, the tool result
carries:
- the first 6,000 characters;
- a marker line;
- the last 1,500 characters.

The marker gives the full length and the handle that returns the whole
text:

    … stdout truncated: 103,782 chars; 96,282 not shown. The whole
    output is choir.CellOut(17) (a string); slice it or search it in your
    next cell.

The session keeps each truncated output under its cell number. The
bound is 8,000 characters, the measured threshold above, and a single
constant.

**L6-2. Where the full text lives.** In the desk's yaegi session, beside
the variables the cell already keeps. `choir.CellOut(n)` returns it.
Retention is bounded: the last 8 truncated outputs, each no more than
the existing 2 MiB cap. That is at most 16 MiB per session in the worst
case and about 0.8 MiB at the measured p99. A session reset loses them,
and `choir.CellOut(n)` then returns an error naming the reset. Nothing
is written to the store, because this is a view, not evidence. Evidence
stays on captures and reports.

**L6-3. The cut is honest.**
- Cuts fall on rune boundaries, never inside a UTF-8 sequence.
- `error` and `stderr` are never cut by this rule. An error the model
  must see stays whole up to the existing cap.
- The marker always states the total length, so the model never
  mistakes a view for the whole.

**L6-4. The prompts say it once.** Each desk prompt's cell section gains
one rule: print summaries, such as counts, keys, the first few items or a
search hit, and keep whole records in variables. Large output is shown
cut, with its handle. This repeats the research prompt's existing "keep
findings in named variables" and does not add a budget. No desk is told
to print less than it needs: the handle gives back everything.

## What this does not add

- No budget and no cap on model work. The model can still read every
  byte, through the handle, in a cell.
- No new record kind or store object.
- No summarizer model in the loop.

## Ways this could fail (written before the code)

1. The model needs the cut part and does not know it exists. Answer: the
   marker states the length and the handle. Tested: a cell that prints
   20,000 characters shows the marker, and `choir.CellOut(n)` returns all
   of them.
2. Printed JSON cut mid-structure, then parsed by the model as whole.
   Answer: the marker line is outside the head, and the head is marked
   cut. Tested with a JSON print.
3. A cut inside a multi-byte rune. Tested with CJK and Arabic text at the
   boundary.
4. An error hidden by a cut. `error` and `stderr` are untouched. Tested.
5. Session memory grows. Retention is bounded at 8 outputs. Tested: the
   ninth evicts the first, which then answers with an error naming its
   eviction.
6. A session reset between cells. `choir.CellOut` answers with an error
   naming the reset. Tested.
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

Expected direction, an estimate: research input falls by roughly half,
because 67% of its stdout sits beyond the bound and is re-sent on every
later call.

## Order

Deploy lane position 3, after L2a and C1e B1 (one shared-runtime cutover
at a time). It is designed and reviewed now, built in its own worktree,
and deployed after B1 is verified.
