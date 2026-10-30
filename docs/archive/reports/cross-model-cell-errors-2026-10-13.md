# Which desk cell errors recur across models (Autopaper run, staging 18d01cc0)

This is a read-only analysis, written 2026-10-13. The data comes from one 25-minute Autopaper run with
a pool of four models: 828 cells, of which 165 failed (19.9%). It builds on
[reeng-desk-cell-errors-2026-10-13.md](reeng-desk-cell-errors-2026-10-13.md) and keeps that report's
mechanism names (M1 to M8). Mechanisms were checked by running cells through the real
`yaegikernel` session path, using `go test -overlay` with a scratch test, so nothing in the repo was
written. The kernel, prompt and frame sources at HEAD are identical to 18d01cc0.

Failure rate by model: deepseek-v4.1-flash 52/411 (12.7%), gpt-6-luna 20/115 (17.4%), cat-jam 47/179
(26.3%), longcat 45/113 (39.8%).

## Summary

| # | cluster | n | % | where it occurs (failed/cells for that desk and model) | class | fix |
|---|---|---|---|---|---|---|
| A | top-level define from a choir value: `constant definition loop` (M7); compile panic on `recs[0].Support...` (M2a); compile panic `nil type` on top-level `v, ok :=` | 40 | 24.2 | all 4; mgmt: catjam 11/41, longcat 20/55, deepseek 1/10, luna 1/11 | CROSS | KERNEL substrate; ERROR-MESSAGE now |
| B | kernel rewrote `x :=` to `x =` and the type changed (M3): `cannot use type A as B`, or runtime `reflect.Set ... not assignable` | 20 | 12.1 | all 4 | CROSS | KERNEL |
| C | `Read`/`ReadCapture` result used as a map or struct: `cannot convert "capture_id" to int`, `non-interface type yaegikernel.JSON`, `undefined method: CaptureID` | 18 | 10.9 | research: deepseek 5/279, catjam 7/47, longcat 6/57 | CROSS | KERNEL |
| D | `undefined method: ID` (12) / `Source` (3) / `Title` (1) on records or updates | 16 | 9.7 | texture: deepseek 8/122, catjam 7/91; longcat 1 | CROSS | KERNEL + ERROR-MESSAGE |
| E | `import "go-choir/choir"` (and the `github.com/...` and `choir.com/...` forms) refused | 9 | 5.5 | texture: deepseek 7/122, catjam 2/91 | CROSS among exposed* | KERNEL |
| F | `undefined method: Started` (5) / `Total` (2) on `Search` | 7 | 4.2 | deepseek, luna, catjam | CROSS | KERNEL |
| G | `undefined: min` | 5 | 3.0 | deepseek, catjam, longcat | CROSS | KERNEL |
| H | named `func f(){}` together with statements (M1b), plus 2 cells that failed because of it | 6 | 3.6 | deepseek, longcat | 2 models | KERNEL |
| I | syntax slip in a giant one-line literal (M6) | 7 | 4.2 | luna 5/96, longcat, catjam | CROSS (luna-heavy) | LEAVE |
| J | hallucinated helper or a name never declared (M8) | 8 | 4.8 | deepseek, longcat, catjam | CROSS | LEAVE |
| - | model-specific remainder | 29 | 17.6 | last section | SPEC | - |

\*Only deepseek and catjam ran Texture in volume. Luna had 8 Texture cells and longcat 1.

**Status of the earlier report's mechanisms:**

- M1 (`type/var/const`-led cells): 0. The Cut 1 B3 fix holds.
- M1b (named func plus statements): 4. Its step 2 is unlanded.
- M3: 20. Worse than before, because 14 of them now fail at runtime and kill the worker.
- M7 and M2a: one cause, together cluster A.
- M4 (undocumented result fields): now on `Records`, `Search` and `Read` (D, F, C).
- M5 (imports): 16.
- M6: 7.
- M8: 8, plus 5 `min`.
- M4b: 0.

**Damage:** 41 failures (25%) are panics, and each one poisons the worker and loses its heap
(`session.go:203`): A 21, B 14, `fmt := fmt` 2, string index 3, interface conversion 1.

## Cross-model clusters: root cause in the surface

**A (40).** The model writes `rec := recs[0]`, `sp := recs[0].Support`, `pairs :=
recs[0].Support.Pairs` or `v, ok := m["k"]` at cell top level. Reproduced:

- `rec := recs[0]` gives "constant definition loop". So does `var rec = recs[0]`, even when `recs` was
  bound in an earlier cell.
- `sp := recs[0].Support` gives a compile panic (nil pointer).
- Any top-level comma-ok gives a compile panic (nil type), even on a local `map[string]any`.
- The same code runs inside a `for` loop or `func(){...}()`.
- Exporting `InboxRecord`/`CommitmentSupport` as choir types does not help.

The cause is yaegi v0.16.1's global type analysis in statement mode, not a model misreading anything:
this is the natural way to write Go. The error teaches nothing, because `cellErrorHint`
(`surface.go:280`) only checks choir names. The `Judge` example (`surface.go:85`) and the management
prompt (`rlm_management_runtime.yaml:21,26`) use the loop form, which works, but nothing tells the
model the top-level form fails. Models repeat it: 40 cells across 17 runs, up to 6 in one run. 36 of
the 40 are in management, where every support_pairs and captures record is reached through
`r.Support`/`r.Captures`.

**B (20).** `degradeBoundDefines` (`cell_normalize.go:227`, rewrite at `:285`) turns `x := e` into
`x = e` whenever `x` was bound by an earlier cell. A new type then fails at compile time, or at
runtime through `reflect.Set`, which kills the worker. Both reproduced. Luna's 10 cells get here by
a model-specific route. `r := choir.Read(url)` assigns two results to one variable, and yaegi
accepts it silently. The repeat of that cell is rewritten and panics with "value of type error is
not assignable to type func(string) (JSON, error)" (reproduced), a message that misleads the model.

**C (18).** `Read`/`ReadCapture` return `JSON []byte` (`choir.go:526,603`). The surface summary
(`surface.go:75`) and the research prompts (`rlm_research_runtime.yaml:26`, `research.yaml:39-43`) say
it "returns capture_id, url, title, block_count and the first blocks". In Go, that describes a map or
a struct. Every other world verb returns a typed struct. What models tried:

- indexing it like a map: 7
- type-asserting it: 4
- selecting fields: 4
- passing it as a map or string: 2

Four more cells, in A, unmarshalled `fmt.Sprintf("%v", r)` into a map and then hit a top-level
comma-ok.

**D (16; 15 in Texture).** The fields are `InboxRecord.RecordID` (`intent.go:158`) and
`PendingUpdate.UpdateID` (`intent.go:194`), and models write `.ID`. What pushes them toward the guess:

- The Texture frame prints "(record <id>)" (`texturewrite/frame.go:130`).
- The prompts give JSON names only ("seq, kind, from, ...", `rlm_research_runtime.yaml:23`).
- `texture.yaml` names no fields at all.
- `choir.Help("Records")` lists the Go fields, but `DeskSurface` (`surface.go:266`) shows only
  signatures.
- "undefined method: ID" names neither the type nor the real field.

Nearly all of these are in the first "survey" cell of a Texture turn.

**E (9).** "not in activation allowlist" (`allowlist.go:93`). A bare `import "choir"` and a whole
`package main ... func main()` program both work (`allowlist.go:83`, tested); only the path variants
fail. `texture.yaml` lacks the "cells are fragments; never `import "choir"`" rule that the research
and management overlays have (`rlm_research_runtime.yaml:23`). 34 of the 47 Texture failures were
written as whole programs.

**F (7).** The `Searches` summary says "how many answered of how many started" (`surface.go:73`).
`Search` has `Answered` but no `Started` or `Total` (`search_store.go:75`), while `SearchCount` has
`Total` (`:97`). The summary's own wording taught the guess `s.Started`.

**G (5).** yaegi v0.16.1 lacks the Go 1.21 builtins. Every case was `s[:min(n, len(s))]`.

**Misleading hints.** `cellErrorHint` scans string literals (`surface.go:302`). Capture ids look like
`obj:choir.web_capture:...`, so 19 errors (11.5%) carry "choir has no web_capture. Closest:
choir.ReadCapture" plus the full verb list, attached to unrelated errors (reproduced). Two more carry
"choir has no com" from an import path.

## Ranked fixes (estimated share of 165)

1. **KERNEL, B:** restrict the `:=` to `=` rewrite to the case where the right-hand side reads the
   bound name (`cell_normalize.go:285`). 20 cells (12%), and 14 fewer worker kills. Cheapest, with a
   known size.
2. **KERNEL substrate, A:** fix yaegi's global type analysis (patch or fork), or compile cell bodies
   in function scope and hoist the defined names. Up to 40 cells (24%). Needs a design note first.
   **Now (ERROR-MESSAGE):**
   - Make a compile panic a preserved compile error (the earlier M2a fix, unlanded at
     `session.go:203`).
   - Add a hint on "constant definition loop" and on compile panics: "top-level `x := v[i]`,
     `x := v.F.G` or `a, ok := ...` does not compile here; use it inside `for _, r := range ...` or
     `func(){...}()`".

   That is about 10-12 cells (6-7%), from fewer repeats, and 21 fewer heap losses.
3. **KERNEL, C:** give `Read`/`ReadCapture` a typed `Capture` result (`CaptureID, URL, Title,
   BlockCount, Blocks{ID, Kind, Text, Page}, NextBlock, Headings`, with a JSON `String()`), so the
   generated return fields name it. 14-18 cells (8-11%). Fallback (ERROR-MESSAGE): "Read returns JSON
   bytes: `json.Unmarshal(r, &m)`".
4. **KERNEL + ERROR-MESSAGE, D:** add `ID` (`json:"-"`) to `InboxRecord` and `PendingUpdate`.
   Everyone guesses it, so it is the natural reading. For any other "undefined method/selector: F",
   hint the closest choir result type and its fields. Render `returnFields` in `DeskSurface` for the
   observation verbs. 12-16 cells (7-10%).
5. **KERNEL, E:** treat any import path whose last element is `choir` as the predeclared package, and
   add the fragment rule to `texture.yaml`. 9 cells (5.5%).
6. **KERNEL, F:** add `Search.Total` and reword the summary to "Answered of Total". 7 cells (4%).
7. **KERNEL, G:** predeclare int `min`/`max` in the session prelude. 5 cells (3%).
8. **KERNEL, H:** split named funcs out of mixed cells (the earlier M1 step 2). 4-6 cells (2-4%).
9. **ERROR-MESSAGE:** skip string literals and comments in `cellErrorHint`. This removes 0 failures
   directly but stops 21 wrong hints.

Together, fixes 1 and 3-8 plus the mitigation in 2 come to about 81-97 cells (49-59%). With the
substrate fix in 2, about 106-121 (64-73%). The clusters do not overlap.

**LEAVE:** I (7), J (8), and the banned imports `os`/`reflect`/`io/fs`/`net/http` (7), which are
correct refusals. The `os` message could name `choir.ReadFile`/`ListDir`.

**Aside:** `rlm_management_runtime.yaml:37` still says "a failed cell poisons the session". Research
fixed this at `rlm_research_runtime.yaml:29`.

## Model-specific clusters

- **luna:** `r := choir.Read(url)` (10 cells, 8.7% of luna's cells, 7 in one run, every one killing
  the worker). Giant one-line literals with unescaped quotes (5). `Write` then `Wait` (1). A `map`
  read as a struct (1).
- **longcat:** `r.Support` as a map (3). A phantom error result from `Records()`/`Searches.List()`
  (2). A `SearchStub` passed as the id (2). `Providers` treated as arrivals (1). Highest rate in A
  (20/55 management cells).
- **deepseek:** `fmt := fmt` compile panic (2). `import "reflect"` (2). 7 of the 9 cells in E.
- **catjam:** Texture programs that walk the filesystem with `os`/`io/fs`/`net/http` (5).
  `IsNil on string` panics (2).

## Method and limits

- **Normalizing:** each error had its `line:col`, `| source` excerpt and `hint:` tail stripped, then
  was matched against signature rules.
- **Hand assignments:** six cells were placed by reading their source (two failed because of an
  earlier M7 failure, plus one each of M1b, M8, `Source.ID`, and a map read as a struct).
- **CROSS:** at least 3 models, or deepseek and luna plus a free model.
- **Rates:** failures per cell for each (desk, model), because volume is uneven (deepseek research
  279 cells, longcat texture 1).
- **Limits:**
  - One run.
  - The model is the last provider call before the cell.
  - Only failed cells carry source, so cascades are inferred from those cells alone.
  - 818 of the 828 cells have a desk/model count.
  - Fix shares are estimates for the first attempt.

## Fixes landed and first measurement (2026-10-13)

Landed on staging at d2324a55:

- B: 0e270be6.
- A, the stopgap: 65060843. A compile panic now preserves the session, gets a loop-form hint, and the hint scanner skips literals.
- E, F and G: c21dee79.
- D: 67162152.
- Prompts: d2324a55. Texture gets the fragment rule, and management's recovery rule was corrected.

Two fixes followed: C's decode hint (cdffb40d) and the field hint that names every type or the exact case (a17ea6c3).

The paper run on d2324a55 (docs/evidence/free-models-paper-3-2026-10-13.json):

- 37 of 283 cells failed, 13.1% (19.9% before).
- Clusters A to G account for 6 failed cells (about 105 before).
- No session was poisoned by a compile panic.

By model:

| model | failure rate |
|---|---|
| deepseek | 13.6% |
| luna | 10.0% |
| cat-jam | 17.3% |
| longcat | 5.6% (18 cells) |

This is one shorter run on different news, so read it as a direction, not a size. Residuals:

- the typed Read result (C);
- yaegi's global type analysis (A);
- named funcs mixed with statements (H; 2 cells in the new run);
- Texture's whole-program cells that walk the filesystem (cat-jam).
