# Settlement Gate Item 1 — Code-Free Define Receipt (2026-09-09)

Mutation class: green (docs only). No source changed in this receipt.
Mission: `docs/definitions/choir-rlm-settlement-gate-2026-09-09.md`, acceptance item 1.
Charter HEAD observed: `main@9157d2f1`. Worktree: 4 untracked unrelated-WIP paths preserved
(`docs/reports/choir-rlm-restore-zero-completion-report-2026-09-09.md`,
`scripts/__pycache__/`, `scripts/generate_restore_zero_completion_pdf_2026_09_09.py`, `tmp/`).

## Problem (observed, not inferred)

`Session.Eval` (`internal/yaegikernel/session.go:75-157`) poisons the live interpreter on
**every** `EvalWithContext` error (`finish`, lines 131-140: `if err != nil { s.poisoned = err }`).
Both session loops exit the worker on any evaluation error
(`session_loop.go:82-84`, `:161-163`), and the broker drops the session on any non-empty
error string (`cmd/capsule-broker/session_worker.go:403-407`) and on any transport error
(`:390-394`). The host import preflight (`checkImports`, `session.go:90-93`) returns
before `finish` and preserves the heap — the only preserving class today.

The defect: pure compile-phase rejections (syntax, type, declaration, undefined symbol)
that never executed anything discard the working heap through loop-level worker exit.
There is no typed reuse disposition anywhere on the path: `SessionResult`
(`session_loop.go:29-37`), `capsule.GoEvalResult` (`internal/capsule/types.go:119-129`),
and the broker drop decision all carry only an opaque `Error` string. String matching
is the only available classifier, which the acceptance contract forbids.

## Isolation matrix (thrown-away probes, run 2026-09-09 at charter HEAD, files removed)

Exact `NewSession(nil, nil)` construction; heap marker `base := 21` + `import "strings"`;
failing cell; successor reads `base * 2` and `strings.ToUpper("rlm")`.

| failing cell class | `Eval` error | successor heap | poisoned |
|---|---|---|---|
| host-preflight disallowed import | allowlist refusal | intact (`42`, `RLM`) | no |
| body syntax `func broken(((` | `expected type, found 'EOF'` | refused (`poisoned`) | yes |
| type mismatch `"a" + 1` | `mismatched types` | refused | yes |
| undefined symbol | `undefined:` | refused | yes |
| decl `x := 1 / x := 2` | `no new variables` | refused | yes |
| `panic("boom")` | `boom` | refused | yes |
| index out of range | `slice index out of range` | refused | yes |
| timeout `for {}` @200ms | `evaluation timed out` | refused | yes |

Compile-only boundary on the same interpreter (`interp.Compile` without `Execute`):

| probe | result |
|---|---|
| failed `Compile` (syntax / type / undefined) then `Eval` successor | heap intact (`cbase + 1` ok) — failed compile does not mutate |
| successful `Compile(q := 7)` without `Execute`, then `Eval(qcheck)` | resolves to zero value `0`, no error (control `neverdefinedname` errors) — **successful Compile mutates scope** |
| `Compile` + `ExecuteWithContext` of `w := m2 * 2` | `w` visible (`40`) — split preserves definition semantics |
| raw `ExecuteWithContext` of `h4 := 99; panic` then read `h4` | `h4 = 99` visible — failed execute partially mutates |

Yaegi `v0.16.1` mechanism: `eval = compileSrc + Execute` (`interp.go:eval`); `CompileAST`
returns parse/type errors before the scope-installing mutex block (`program.go:CompileAST`),
while successful compile installs package symbols even before `Execute`. `Execute` runs
user code, so any execute-phase failure may have partially run.

## Proven classification (matrix conclusion)

- **Proven non-mutating, may preserve:** host import preflight rejection + **compile-phase
  rejection** (a `Compile` error on the exact session interpreter, no `Execute` attempted).
  Valid successor observes exactly the prior heap.
- **Unsafe-to-reuse, must poison + respawn from durable snapshot:** every `Execute`-phase
  failure (runtime error, panic, timeout, overflow), worker death, transport failure,
  cancellation, and any doubt. Failed execute demonstrably leaves partial mutation.
- Compile **success** is not a preserving class: it installs zero-valued symbols.

## Authorized repair boundary (item 1 only; red, next commit)

1. Split `Session.Eval` into `Compile` (non-executing gate) then `ExecuteWithContext`;
   compile rejection returns a typed preserving error without poisoning; any
   execute-phase error poisons. No string matching: introduce a typed reuse disposition
   (`preserve` vs `unsafe-to-reuse`) carried separately from the diagnostic kind
   (`import-preflight | syntax | type | runtime | timeout | panic | overflow | worker |
   transport`, original message preserved verbatim).
2. Carry the typed class through `serveCell`, both loop variants
   (`RunSessionLoopWithDrainAndHooks`, `RunSessionLoopFramedWithHooks`), the sidecar
   boundary, and the broker drop decision: preserving class ships the error result
   **without** worker exit/drop; unsafe class keeps exactly today's poison/exit/drop.
3. Surface the typed disposition + kind on the `SessionResult` / `GoEvalResult` seam
   (additive fields; no existing field redefined).
4. Tests (local_test): preserving-class heap identity (vars + imports intact across a
   compile rejection, worker alive across frames); every execute-phase class poisons;
   timeout poisons; diagnostic kind preserved verbatim; no string-match classifier.

## Explicitly not in this boundary

Fallback removal (item 2), identity/canonicalization (item 3), admission grammar (item 4),
fate saga (item 5), orphan close (item 6), staging proof (item 7) — untouched. One-shot
worker untouched. No new execution path: the split reuses the same interpreter and the
same `Execute` the current `Eval` already calls.

## Evidence floor / rollback

Floor: focused `go test ./internal/yaegikernel` contracts named above. Rollback: revert
the repair commit(s); immutable events/reports unaffected. If the repair cannot carry
the typed class without worker disposal, it stops: all failures stay unsafe-to-reuse
(host-preflight class alone preserves) and this receipt records the refusal.

## Standing-questions answers for this boundary

1. Settled by: owner-chartered definition item 1 (this receipt executes, settles nothing).
2. Topology: conforms to charter entrypoints (`session.go`, `session_loop.go`,
   sidecar seam, broker worker); no other mission owns these paths.
3. Deletion citers: no deletion in this boundary (additive typed fields; obsolete-path
   removal belongs to items 2/4).
4. Production consumers: session worker is the live RLM eval path; one-shot preserved.
5. Single authority for heap truth: the live interpreter only for proven classes;
   otherwise the durable snapshot via poison+respawn (broker `dropSession` + recreate).
6. Artifact: local_test matrix + contract tests (this receipt is Define, not proof).
7. Fate-sharing: preserving path needs only the live worker; poison path needs broker
   respawn — same as today.
8. Restart: worker death already poisons/respawns; preserving class never outlives the
   worker (loop exit still ends it; broker drop still forgets it).
9. No SSH: all verification is `go test` local.
