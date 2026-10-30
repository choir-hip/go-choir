# Upstream issue draft: yaegi never ends `for init; ; {` loops (2026-10-20)

Status: **draft for the owner to post.** Nothing has been posted anywhere.
Owner ruling: owner notes §27 item 5 ("An agent drafts the upstream yaegi
loop bug report; the owner posts it"). Origin:
[desk-cell-init-only-for-loop-never-ends-guest-oom-2026-10-16.md](../archive/problems/desk-cell-init-only-for-loop-never-ends-guest-oom-2026-10-16.md).
Choir already works around this in `normalizeCellSource` (rewrites
`for init; ; {` to `for init; true; {`), so the report is a courtesy to
upstream, not a blocker for us.

Target: <https://github.com/traefik/yaegi/issues/new> (bug report template).
Check first for an existing issue: search "for init; ; infinite loop" and
"forStmt1".

---

## Title

`for init; ; {` (init, no condition, no post) re-runs the init statement on every iteration

## Body

### Description

A `for` loop with an init statement but no condition and no post statement
never terminates in yaegi when the body advances the loop variable, because
the init statement is executed again on every iteration. In Go the init runs
once.

```go
for i := 0; ; {   // init only: no condition, no post
	...
	i = i + 1
}
```

In yaegi `i` is reset to its init value at the start of every pass, so a
`break` that depends on `i` is never reached.

### Reproducer

Standalone program (`go.mod` requires `github.com/traefik/yaegi v0.16.1`):

```go
package main

import (
	"fmt"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// Go prints "iterations: 4". yaegi prints "iterations: 11": i is reset to
// 0 on every pass, so only the n > 10 guard ends the loop.
const src = `
package main

import "fmt"

func main() {
	n := 0
	for i := 0; ; {
		n++
		if i >= 3 || n > 10 {
			break
		}
		i = i + 1
	}
	fmt.Println("iterations:", n)
}
`

func main() {
	i := interp.New(interp.Options{})
	if err := i.Use(stdlib.Symbols); err != nil {
		panic(err)
	}
	if _, err := i.Eval(src); err != nil {
		panic(err)
	}
}
```

The `n > 10` guard is only there so the demonstration terminates. Without
it (`if i >= 3 { break }`) the program never ends, and if the body also
appends to a slice, memory grows until the process is killed.

### Expected

`iterations: 4` (what `go run` prints for the same loop body). The init
statement runs once, before the first iteration.

### Actual

`iterations: 11`; without the guard, an infinite loop.

### Which shapes are affected

Found by running variants through an interpreter session:

| Shape | Result in yaegi v0.16.1 |
|---|---|
| `for i := 0; ; { ...; i = i + 1 }` | wrong: init re-runs, `i` stays 0 |
| `var i int; for i = 0; ; { i++ ... }` | wrong: the assignment init re-runs |
| `for i, k := 0, 0; ; { ... }` | wrong |
| `for i := 0; true; { ... }` | correct |
| `for i := 0; i < 3; { i++ }` | correct |
| `for i := 0; ; i++ { ... }` | correct |
| `for ; ; { ... }`, `i := 0; for { ... }` | correct |

So only the "init, no condition, no post" form (`forStmt1`) is affected.

### Cause

In `interp/cfg.go` (v0.16.1, around line 1424) the `forStmt1` case wires the
end of the body back to the start of the whole statement, which is the init:

```go
case forStmt1: // for init; ; {}
	init, body := n.child[0], n.child[1]
	n.start = init.start
	init.tnext = body.start
	body.tnext = n.start      // <- jumps back to init
	sc = sc.pop()
```

Compare `forStmt0` (`for {}`), where `n.start` is the body and looping back to
it is correct. For `forStmt1` the loop-back target should be `body.start`.

### Possible fix (tested only against the reproducer)

```diff
 		case forStmt1: // for init; ; {}
 			init, body := n.child[0], n.child[1]
 			n.start = init.start
 			init.tnext = body.start
-			body.tnext = n.start
+			body.tnext = body.start
 			sc = sc.pop()
```

With this one-line change the reproducer prints `iterations: 4` and the
unguarded variant prints `len: 4` and exits. We have not audited the
`continue` path (`sc.loopRestart` is set to the last child, which for this
shape is the body) or added a regression file under `_test/`; a maintainer
may want a `_test/for*.go` case for the shape.

### Environment

- yaegi: v0.16.1 (the latest release at the time of writing)
- Go: go1.26.9 darwin/arm64 (also seen in production in a Linux guest; the
  wiring is platform independent)
- Embedding: `interp.New(interp.Options{})` with `stdlib.Symbols`

### Impact (optional context for maintainers)

We run LLM-written snippets in yaegi sessions. This is a common idiom for
scanning a string (`for i := 0; ; { j := strings.Index(s[i:], kw); if j < 0
{ break }; ...; i = i + j + len(kw) }`) and, being valid Go that behaves
differently, it produced a runaway interpreter that exhausted a VM's memory
three times in one day. We now rewrite the loop header before evaluation.

---

## Local verification (2026-10-20)

Run on this machine (darwin/arm64, go1.26.9) from a scratch module outside
the repo requiring `github.com/traefik/yaegi v0.16.1` (the version pinned in
this repo's `go.mod`). `ulimit -v` is rejected on this macOS
(`setrlimit failed: invalid argument`), so the run used
`GOMEMLIMIT=512MiB`, a hard `timeout 10`, and (for the unguarded variant
only) an in-program watchdog that exits when the heap passes 256 MiB.

| Run | Observed |
|---|---|
| native Go, guarded loop | `iterations: 4` |
| yaegi v0.16.1, guarded loop (`./repro`) | `iterations: 11`, exit 0 (the bug: init re-ran each pass) |
| yaegi v0.16.1, unguarded loop (`./repro unbounded`) | no termination; heap passed 256 MiB in about 1 s and the watchdog aborted it, exit 3 (nowhere near the 10 s timeout, because the append grows fast) |
| yaegi with the one-line patch, guarded | `iterations: 4` |
| yaegi with the one-line patch, unguarded | `len: 4`, exit 0 |

The loop reproduces as described: in yaegi the unguarded loop does not end,
it only stops because a memory watchdog and `timeout` were in place.

## Before posting (owner)

1. Search upstream issues for an existing report (above).
2. Decide whether to include the "Impact" paragraph (it mentions our use).
3. Optionally offer the one-line patch as a PR; the yaegi `_test/` directory
   takes a Go file with an `// Output:` comment as the regression test.
