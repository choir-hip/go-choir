# TRACE-RUN: `choir trace run` and `choir trace runs`

Read-only. One run, step by step, with the reasons its record holds. Built on
the client from routes that already exist; no server change, no write, no
model call. Code: `cmd/choir/trace_run*.go`, `trace_runs.go`; tests
`trace_run_test.go` (fixtures use the real stored event shapes).

## What it shows

- Header: agent, desk, story, state, start, end, duration, parent and child
  runs, models, who woke it (a label plus an inferred candidate record, marked
  inferred), the prompt and result heads, totals.
- Steps in time order with offsets from the run's start and durations: each
  model call (model, tokens, latency, what the model said), the cells it
  issued (source, output head, errors), the module-surface calls inside a cell
  (verb, time, error, search counts and end), restarts and resumes, retries,
  turn commits, records sent and received.
- `why:` appears on a step only when the record holds a reason: the turn
  commit's Reason, or the model's own text just before the cell.
- A closing list, "What this run's record does not contain", says once what is
  absent in this run's data. Steps never repeat it.
- Model calls come from the `loop.progress` rows of phase `tool_loop`
  (phase is read from the payload; the record's own phase field is empty for
  run events). Where those rows are absent the total falls back to the kept
  `model.served` summary and says so; a research child's calls appear only
  there, as "children". With neither it prints `model calls not recorded`, or
  `aged out` when the run is older than the 14-day window. It never prints 0
  for unknown.

## Flags

`choir trace run [flags] <run_id>`: `--json` (complete, stable names),
`--full` (no clipping), `--host`, `--computer`, `--api-key-file`,
`--timeout`, `--max-events` (default 50000; beyond it the run prints as
PARTIAL), `--inbox-window` (default 500), `--no-records` (skip inboxes).
`choir trace runs` lists runs to choose from (see `--help`).

## A key for the drill account

The key is an owner API key with scope `read:runtime` for the computer's
owner. Sign in as the drill account (`choir login`), mint one with
`choir api-key create --scopes read:runtime`, write it to a mode-0600 file
outside the repo, and pass `--api-key-file`. Never print it, never commit it.
Staging runs are called by the lead only. Only the owner of a computer can
read its runs: the paper's runs need the paper owner's key.

## What the record does not hold

The reason for each call (only turn commits and the model's text), the prompt
each model saw (request-side rows are live-only), the record that woke the
run, the digest's capture ids, tool call ids on model rows, search queries
(unless a literal in the cell source), reasoning text, and children's own
events. Retention: `tool.result` 7 days, `loop.progress` and `desk.cell` 14.

## Example (a test fixture)

```
models       devin/swe-2-flash x1
totals       model calls 1 | tokens in 9000 out 300 cached 8000 | cells 1 | errors 0
+0.200s   MODEL CALL  devin/swe-2-flash  1.0s  in 9000 out 300 cached 8000  stop=tool_use  calls=1 desk_go_eval
          said:
            | Reading the inbox.
  +1.300s   CELL  100ms
            source: (23 chars)
              | recs := choir.Records()
            output (14 bytes):
              | {"stdout":"2"}
            why: the model said before this cell: "Reading the inbox."
```

An aged-out run prints `totals       model calls aged out, tokens aged out | cells 1`.
