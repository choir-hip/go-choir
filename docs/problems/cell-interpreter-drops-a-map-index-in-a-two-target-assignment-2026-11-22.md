# The cell interpreter drops a map-index target in a two-target assignment

Recorded 2026-11-22, found when the research overlay's worked cell reported "0 from the
web" for pages it had read (my own failing test of the cell, and seat 2's reproduction in the
real session on review round 3).

## Problem

In a Go cell, a two-target assignment whose second target is a map index and whose first
right-hand value is a function call leaves the map unchanged and raises no error:

```go
x, m := 1, map[string]bool{}
var y int
f := func(i int) int { return i + 1 }
y, m["k"] = f(x), true
fmt.Println(y, len(m)) // prints "2 0": y is assigned, m["k"] is not
```

The overlay's worked cell wrote `pages, fromWeb[p.URL] = append(pages, p), true` and
silently produced a notes line that said no page came from the web.

## Evidence

Minimal reproductions, run on the build VM with the interpreter alone (no Choir code in the
path: `interp.New`, `stdlib.Symbols`, `Eval` of a `package main` source), in a function body
and at `init`:

| right-hand side of `y, m["k"] = ...` | `len(m)` afterwards |
| --- | --- |
| `1, true` | 1 (assigned) |
| `x+1, true` | 1 (assigned) |
| `f(x), true` | **0 (dropped)** |
| `x, g()` (call second) | 1 (assigned) |

and `a, m["k"] = append(a, 1), true` leaves `a` appended and `m` empty, in a plain loop body
too. The table is from the Choir fork (`choir-hip/yaegi`, upstream plus two race fixes). The
`append` forms (top level and in a `range` loop) were also run against **upstream
`github.com/traefik/yaegi` v0.16.3** in a scratch module with no `replace`, and give the same
result (the map stays empty). The Choir wrapper
(`internal/yaegikernel/eval.go`, the cell normalizer) is therefore not the source: the
defect is in yaegi itself, in the compile of a multi-value assignment whose first value is a
call when a later target is a map index. Seat 2's session probe and the failing
`TestOverlayWorkedCellCountsOnlyPagesReadWithoutError` show the same in the cell.

## Belief

Upstream yaegi v0.16.3 treats `T, m[k] = f(...), v` as an assignment from a call's
multiple results and loses the map-index store; the same shape with no call on the
right-hand side is compiled correctly. I did not read yaegi's compiler to name the exact
function; the table above is the evidence, and "upstream" is established by running upstream.

## Remaining error

Any cell a model writes in that shape (a call first on the right, a map index among the
targets) computes wrongly with no error, in every desk's cells. Nothing detects it. The
worked cell now splits the assignment into two statements. The interpreter is **not** fixed
here (owner's rule: not in this change); the options are a normalizer rewrite in
`cell_normalize.go` that splits such assignments (the wrapper already normalizes cells), or
a yaegi fix upstreamed and pinned; neither is done, and no count of how often models write
this shape exists.
