# Why 16-25% of desk REPL cells end in an error (C0 tracer bundles)

Read-only analysis of the three Cut 0 trace bundles, 2026-10-13 (a Sonnet
agent for the orchestrator; agentic-consensus plan review candidate C5).
Experiments ran through the real `yaegikernel.Session` without changing
the repository. Mechanism M1 is fixed by the commit that adds this report
(Cut 1 breakage B3); the rest are open.

## Data

| bundle | build | cells | errored | rate |
|---|---|---|---|---|
| c0-tracer-dev-8 | e6995e08 | 126 | 21 | 16.7% |
| c0-accept-1 | b9f195b1 | 128 | 30 | 23.4% |
| c0-fast-1 | 287734c5 | 54 | 16 | 29.6% |
| total | | 308 | 67 | 21.8% |

Join: per `loop_id`, ordered by `seq`: `tool.invoked(desk_go_eval)` -> `desk.cell` -> `tool.result`. Tokens are
`input_tokens`/`output_tokens` of the `loop.progress` response event that produced the cell. Input tokens are
whole-context per call (mostly prefix), so rank by cells and turns; tokens are shown for scale.
Caveat: in dev-8, 12 `desk.cell` events (2 errors) are `choir.Research` child cells with no
`tool.invoked` row, so no source. `worker_pid` is recycled inside the guest (pids 754/761 alternate), so it
is not a unique worker id; `desk.cell` has no spawn generation field. Worker replacement is inferred only
from `reuse=unsafe_to_reuse` and the `session: new session` notice.

## Kernel semantics (verified by code and by experiment)

* The first token of a cell decides yaegi's parse mode. yaegi v0.16.1 `interp/ast.go:381` (`parse`): if the
  first token is `const|func|import|type|var`, the whole cell is parsed as a Go file (`"package main;"+src`).
  Anything else is wrapped as a function body (statement mode). So `type T ...` followed by `x := ...` is
  "expected declaration, found x", and `x := 1` followed later by `func f(){}` is "expected '(', found f".
* The normalizer has no answer for that shape. `cell_normalize.go:53-83`: parse as file, parse as decl
  fragment, `parseWrappedStatementMix` (requires leading imports), else
  `guardLeadingFuncStatement(degradeBoundDefines(...))`. `guardLeadingFuncStatement` (`:158`) prepends `;`
  only when the first token is `FUNC`. A `type|var|const`-led mix reaches yaegi unchanged.
  Experiment: with a leading `;\n` guard, `type sr struct{}` + `qs := ...`, `var fv any` + `fv = 3`,
  `const base=...` + `p := func...`, and `var x = ... ; func(){...}()` all compile and their names persist into
  later cells. Without it they fail. A guard cannot fix a named `func f(){}` inside statement mode
  (`expected '('`); that shape needs splitting.
* Failure policy (`session.go`):
  - import preflight refusal -> `ReusePreserve` (`:163`); the cell never ran.
  - Compile error (`:208`) -> `ReusePreserve`; the worker stays.
  - Compile panic (`:198-204`) -> `s.poisoned`, `ReuseUnsafeToReuse`/`DiagPanic`.
  - Any Execute error, including a user-level runtime panic (slice out of range, nil map), poisons (`:263`,
    `classifyExecuteError` `:299`).
  - `desk_session_worker.go:358`: a result with `Reuse != preserve` marks the worker dead.
  - `agentcore/tools_desk.go:174` respawns a dead worker; its first cell gets
    "new session: nothing ... is defined" (`:194`). Confirmed in the data (accept-1 research cell 50).
  - So the whole heap is lost on: compile panic, any runtime error or panic, timeout, output overflow,
    transport error, idle > retention, and restarts.
* A failed compile does NOT leave the heap "exactly intact", contrary to `eval.go:34` and the `session.go:195`
  comment. Experiment: `a := 1\nb := "s"+1\n...` fails at compile, and the next cell `fmt.Println(a)` prints
  `0`. Names defined earlier in the failing cell are installed as zero values (yaegi registers statement-mode
  defines in global scope before type-checking the cell). Parse errors ("expected declaration", syntax slips)
  install nothing. So an `x := ...` cell that fails late leaves a silent zero-valued ghost `x`.
  Not shown in the traces as a cause of any error.
* `degradeBoundDefines` (`cell_normalize.go:277-278`) rewrites `x := e` to `x = e` when `x` is in
  `declaredNames` (a name defined by an earlier SUCCESSFUL cell). Raw yaegi allows redefining a global with a
  different type (`b := []byte("x")` then `b := "hello"`, `ch := make(chan int)` then `chan string`, all fine).
  The rewrite makes those "cannot use type X as type Y in assignment".
* yaegi gta limitation (reproduced): a top-level `x := <binary call>[i]` ("constant definition loop"), a
  `y := f(); z := y[0]` with `f` a binary func returning a slice (same), `x := choir.Updates()[0].UpdateID`
  (compile panic, nil pointer), `u.Packet.(map[string]any)` where `Packet` is a struct (compile panic "nil
  type"), and `fv := any(nil)` (compile panic "nil type"). Indexing inside a `for`, a func literal, or
  indexing a map or a local slice literal is fine. The same defect family yields a preserved error or a
  worker-killing panic depending on the statement shape.
* yaegi v0.16.1 lacks the Go 1.21 builtins: `min`, `max`, `clear` are "undefined" (experiment).

## Classification of the 67 errors (one primary mechanism each)

| id | mechanism | cells | % of errs | in_tok | out_tok | retry latency (s) |
|---|---|---|---|---|---|---|
| M1 | declaration and statements mixed in one cell (first-token rule) | 22 | 32.8 | 576k | 33k | 160 |
| M4 | undocumented fields on `choir.Updates()` / `choir.Inbox()` records (+1 `Outline` is a string) | 14 | 20.9 | 254k | 13k | 32 |
| M8a | `x = ...` on a name never declared or lost with the heap | 6 | 9.0 | 122k | 5k | 230 |
| M6 | syntax slip in a giant one-line literal | 5 | 7.5 | 114k | 8k | 35 |
| M3 | kernel rewrote `x :=` to `x =`, type changed | 5 | 7.5 | 262k | 12k | 31 |
| M7 | yaegi gta `x := call()[i]` "constant definition loop" | 3 | 4.5 | 66k | 3k | 147 |
| M5 | import refused (`sync` x2, `net/url`) | 3 | 4.5 | 130k | 4k | 18 |
| M2a | yaegi compile panic -> worker killed | 3 | 4.5 | 72k | 2k | 37 |
| M8b | hallucinated helper or Go 1.21 builtin (`jsonMarshal`, `min`, `minInt`) | 3 | 4.5 | 121k | 2k | 12 |
| M2b | user-code runtime panic -> worker killed | 2 | 3.0 | 113k | 1k | 6 |
| M4b | verb absent from this scope (`choir.ReportPacket` in a research child) | 1 | 1.5 | - | - | - |

By the task's lettering:
(a) previously declared name undefined: 2 cells. `fout` (accept-1 research 51) after the worker was killed by
the compile panic in cell 49; `raws` (accept-1 research 56) was declared only in cell 55, which an import
preflight refused, so it never ran and the model assumed it had.
(b) never declared: 7 (`qsA`, `sweepA`, `sweepBRaw`, `qparsed`, plus `jsonMarshal`, `min`, `minInt`).
(c) parse errors: 22 declaration/statement mix + 5 syntax slips.
(d) refused imports: 3.
(e) compile panics: 3, all of them kill the worker; runtime panics: 2 more (dev-8: `s[11000:]` on a shorter
string, and a `reflect.Set ... yaegikernel.JSON` panic from a one-line `r,e:=choir.WebSearch(...); fmt.Printf("%T %v"...)`;
that second one is not isolated).
(f) stale or undocumented fields: 14 plus the `ReportPacket` miss.
(g) other: 6 (M6), the type-confusion 5 (M3), 3 (M7).

Per desk: accept-1 research 25%, texture 21%; fast-1 research 28%, texture 36%; dev-8 research 15%, texture 19%.
The first cell of a run errors 38% (10/26), mostly M4 (the model probing `Updates()`), against 18% for cell 4+.

## Q2: class (a), did the declaring cell fail, what persists?

Only 2 of 9 "undefined name" errors had an earlier declaring cell in the same desk_session, and neither is the
"compile failure dropped declarations" case. One is a poisoned-worker loss (fout), one is a preflight-refused
cell (raws). The other 7 are `name = ...` on names no cell ever declared, plus three hallucinated helpers.
The cells that issued `qsA =`, `sweepA =`, `sweepBRaw =`, `qparsed =` followed or accompanied M1 failures
(accept-1 research cells 2,3 then 5,6). Hypothesis (not proven; reasoning text is not in the bundle,
`response_text` is empty on tool-call turns): the "expected declaration, found qs" message at a `qs :=` line
taught the model that top-level `:=` is rejected, so it switched to `=`.

## Q3: cascade

48 error episodes over 67 error cells. 34 episodes last 1 turn, 10 last 2, 3 last 3, 1 lasts 4. 19 error cells
(28%) are repeat failures directly after a prior failure. 7 of those 19 are M1 -> M1 (the model resubmits the
same shape; fix usually arrives as "split the type decl into its own cell"). Mean error turns per M1 episode:
1.54. Panics are rare but costly per event: each kills the entire heap. Measured loss at the kill: dev-8 research
17 prior cells and 46 host egress calls (25 web_search); accept-1 research 49 cells (36 ok) and 100 egress calls
(56 web_search); accept-1 texture 8 and 12 cells; dev-8 texture 7. Whether the model re-fetched those results was
not measured.

## Q4: stale or undocumented affordances (class f)

Real shapes: `PendingUpdate{UpdateID, FromAgentID, FromRole, ChannelID, MessageSeq, WorkItemID, Packet
types.CoagentSourcePacketPayload (a struct), HumanProjection}` (`yaegikernel/intent.go:108-117`);
`IncomingMessage{ID, FromDesk, ToDesk, Kind, CreatedAt, EvidenceRefs, Body}` (`intent.go:94-102`).

| tried by model | cells | reality |
|---|---|---|
| `u.Kind` on an update | 6 | lives at `u.Packet.Kind` |
| `m.FromAgentID` on an inbox message | 3 | `FromDesk` (FromAgentID belongs to PendingUpdate) |
| `u.Seq`, `u.ID`, `u.From`, `u.Ref`, `u.ProducerAgentID` | 1 each | `MessageSeq`, `UpdateID`, `FromAgentID`/`FromRole`; no Ref |
| `u.Packet.(map[string]any)`, `p.Kind` after `p := ups[0].Packet` | 2 panics | `Packet` is a struct |
| `for _, l := range doc.Outline` | 1 | `Outline` is a string; the loop yields runes |
| `choir.ReportPacket` in a research child | 1 | not in that scope |

Where it is taught, and the gap:
* `runtimeprompts/overlays/rlm_research_runtime.yaml:22`, `rlm_management_runtime.yaml:25`,
  `rlm_engineering_runtime.yaml:50`: "`choir.Updates()` returns the cell-start snapshot of pending coagent
  update records". No field list. Inbox is "unread mail" (`surface.go:34`), no fields.
* `surface.go:35` (`choir.Help("Updates")`) shows only `u.UpdateID, u.HumanProjection`.
* `cellErrorHint` (`surface.go:225`) only fires for `choir.X` selector and arg-count errors; "undefined method:
  Kind" on a returned value gets no hint, even though the receiver type's fields are known.
* The ReportPacket map literal contract (`rlm_research_runtime.yaml` "Reporting & commitment", and Texture's
  "Packet field contract ... typed object") invites the model to treat a received packet as a map. Hypothesis.
* Stale doctrine, in three prompts: `rlm_research_runtime.yaml:29`, `rlm_management_runtime.yaml:35`,
  `rlm_engineering_runtime.yaml:44`: "A failed, timed-out, or panicking cell poisons the session; the next
  cell runs in a fresh subprocess. Do not assume heap state survived ... re-declare variables". Also the
  `desk_go_eval` description (`tools_desk.go:316`) and the new-session notice (`:194`): "a failed or
  timed-out cell" ends a session. For compile errors (the large majority of failures: 59 of 67 here) and
  import refusals the session is preserved (`session.go:163,208`). The doctrine tells the model to re-declare
  names that still exist, which feeds M3 (the `:=` -> `=` rewrite then mismatches types). The 5 M3 cells all
  carry the original `x :=` in the raw source (verified).

## Q5: ranking and first fix

Ranked by cells (all three give the same order for the top): M1 22, M4 14, M8a 6, M6 5, M3 5. By damage per
event, M2 (worker kill: whole heap and any in-heap research results). Kernel-induced and fixable without touching
a prompt: M1 + M3 + M7 + M2 = 35 of 67 (52%).

Fix first: M1. Reasons: largest (7.1% of all cells; 22% -> ~14.6% overall error rate if eliminated), it is the
only mechanism that repeats itself (7 of 19 repeat failures), it is deterministic, and the cause is one hole
in `cell_normalize.go`. Of the 22: 15 are `type|var|const`-led (guard `;` fixes them), 4 are `func`-led, 3 have
`func name(...)` after statements (those need a split, see below).

Test-first (no tests existed that cover it; `cell_ergonomics_test.go` is the home):
1. Failing test A: cell `type pair struct{ a int }\nps := []pair{{1}}\nfmt.Println(len(ps))` must succeed, and the next
   cell `fmt.Println(ps[0].a, pair{2})` must see both. Same for `var fv any\nfv = 3`, `const base = "b"\np := func(...)...`,
   and `var fqs = []...{...}\nfunc(){...}()`.
2. Smallest change: at the fall-through return of `normalizeCellSource` (`cell_normalize.go:83`) extend
   `guardLeadingFuncStatement` (`:158`) to prefix `;\n` when the first token is `TYPE|VAR|CONST` and the source did not
   parse as a file or decl fragment (a pure declaration cell must stay in file mode). Verified by experiment.
3. Second step for the remaining 7: failing test B: `func uris(s string) []string {...}\nmsgs := choir.Inbox()` and
   `x := 1\nfunc f() int {...}\nfmt.Println(f()+x)`. Split top-level named `func` decls (scan by brace depth) into their own
   file-mode compile, then run the remaining statements, mirroring `splitNewLeadingImports` (`session.go:167`).

Next, in order of cells recovered per unit of work:
* M3 (5 cells, pure kernel fault): failing test `b := []byte("x")` then `b := "hello"; fmt.Println(b)`; restrict the
  `:=` -> `=` rewrite in `degradeBoundDefines` to the case it was added for (the RHS reads the bound name).
* M2a/M7: treat a `Compile` panic as a compile error (`ReusePreserve`), the same disposition already used for the
  identical defect family when it surfaces as "constant definition loop"; the ghost-zero-value exposure is the same
  as for ordinary failed compiles today. Separately, correct `eval.go:34` / `session.go:195` (failed compile
  does leave zero-valued ghosts).
* M4/stale doctrine ("teach X not Y"): add the field lists to `choir.Help("Updates")` / `Help("Inbox")` (surface.go),
  give `cellErrorHint` a field-hint for "undefined method: F" on `PendingUpdate`/`IncomingMessage`, and delete the
  "failed cell poisons the session ... re-declare" paragraph from the three runtime overlays and the tool/notice text,
  replacing it with the real rule (compile errors and import refusals keep the heap; panics, runtime errors,
  timeouts and restarts end it).
* Add a worker generation/spawn id to `desk.cell` so worker replacement is observable (currently inferred).

## Hypotheses explicitly not proven
* That M8a `x =` came from the M1 error text (model reasoning not in the bundle).
* That `Packet` is treated as a map because of the ReportPacket map-literal contract.
* The cause of the `reflect.Set ... yaegikernel.JSON` runtime panic in dev-8 (engineering test scope has no
  WebSearch, so not reproduced).
* Whether results lost in a killed heap were re-fetched.
