# Yaegi RLM Dress Rehearsal on DeepSeek Harness — Progress Report

**Date:** 2026-08-18
**Author:** go-choir agent (deepseek-v4-pro)
**Scope:** A dress rehearsal, on a freshly cloned DeepSeek Harness monorepo
(`~/deepseek-harness`, `master@47f943859b`), of Choir's own Yaegi RLM plans
(`docs/definitions/choir-private-go-actor-kernel-2026-08-12.md` and the
architecture memo it grew from). The goal was **not** to ship a feature in
DeepSeek Harness; it was to prove — on a foreign substrate, where failure is
cheap — the three load-bearing unknowns Choir's private-Go actor kernel must
de-risk: the lossless-JSON binding round-trip, process-layer containment of
arbitrary model-authored Go, and the seam extension that adds a Go language to
an existing code-execution runtime.

## Verdict

**Complete.** A real model, through the fully-built DeepSeek Harness app, wrote
one Go program composing two tool calls (`tools.Bash` → `ls -la`, `tools.Read`
→ the top of `AGENTS.md`) and returned a single JSON object with both
structured results, exit code 0. Every phase of the confirmed Definition
(`dsh-go-yaegi-code-mode-rlm-rehearsal-2026-08-17`) is done and verified.

## What was built

| Phase | Deliverable | Status |
|---|---|---|
| 0 | Binding-crux spike (compiled `func(map[string]any)(any,error)` round-trip through Yaegi v0.16.1; `ToolCallError` via `errors.As`; cooperative vs. non-cooperative cancellation; SIGKILL reclamation) | ✅ verified |
| 1 | Go sidecar binary (versioned length-delimited stdio protocol, request ids, `UseNumber` JSON, per-request fresh interpreters) + Node bridge (`YaegiCodeRuntime`, `language:'go'`, `isolation:'process'`, confine-then-spawn, process-per-run) | ✅ verified |
| 2 | Pruned stdlib allowlist (os/os-exec/net/net-http/syscall/unsafe/reflect refuse; fmt/errors/encoding/json permit; empty `SourcecodeFilesystem`) | ✅ verified |
| 4 | Four-edit language extension in `dsh-tools` (`CodeSdkLanguage 'go'`, `SDK_RENDERERS.go`, `RUN_CODE_FLAVORS.go`, Go SDK renderer, `PORTABLE_RESERVED_WORDS` widened) | ✅ verified |
| 5 | `code-go` preset + rehearsal-only `--patch` overlay + full-app boot + live-model 2-tool composition | ✅ verified live |

The artifacts live in `~/deepseek-harness` (the disposable rehearsal clone) and
`~/yaegi-binding-spike` (the standalone Go module): the `code-go` preset under
`apps/cli/config/agent-presets/code-go/`, the `@deepseek-ai/dsh-code-runtime-yaegi`
bridge package, `go-types.ts` (the Go SDK renderer), the allowlist, and the
rehearsal overlay at `/tmp/yaegi-rehearsal/code-go-overlay.yml`.

## What the rehearsal transferred back to Choir

These are the findings that matter for the real mission, not the artifact itself.

1. **Process-per-run is the containment posture, and it is mandatory.** Yaegi's
   `EvalWithContext` cancellation is cooperative — a compiled binding blocked on
   a raw channel never notices `stop()`. The caller unblocks in ~15µs with
   `context.Canceled`, but the eval goroutine leaks. Only process exit (SIGKILL)
   reclaims a stuck call. Choir's disposable-activation invariant is therefore
   not optional: every activation must own a process it can kill, and the
   bridge must SIGKILL the tree on abort/timeout, with cooperative cancel as the
   fast path.

2. **The allowlist is vocabulary, not containment.** Dropping `os`/`net`/
   `reflect`/`syscall`/`unsafe` from the symbol table makes a hostile import
   fail at eval time, but the process/sandbox layer is the real boundary. A
   program can still reach the OS through the allowed `tools` binding (which
   dispatches host-side). This is Choir's promoted invariant #2, rehearsed.

3. **Lossless JSON is a two-sided contract.** Go's `UseNumber` preserves a
   `9007199254740993` integer bit-exact across the boundary; JS `JSON.parse`
   silently rounds `2^53+1`. Choir's typed-module broker must own number-safe
   encoding end to end, not per side.

4. **`errors.As` on an exported pointer type is the Go typed-rejection idiom.**
   Cleaner than exceptions, and directly transferable to Choir's typed modules.

5. **The four-edit language axis is closed and compiler-checked.** Adding `go`
   was exactly the documented parallel edits (union + flavor + renderer +
   reserved words), and `satisfies Record<CodeSdkLanguage, …>` forces the tables
   to stay in step. The fail-loud unknown-language guard widened its known-set
   correctly.

6. **Confine-then-spawn, never spawn-then-confine.** The sandbox seam returns
   *argv*; the caller must spawn the returned argv. A raw `subprocess.spawn` is
   unconfined — the exact bug class the consensus panel flagged, now an encoded
   invariant.

7. **A `complete: true` persona suppresses the Code Mode SDK.** `minimal`'s
   fixed persona would swallow the generated SDK section; `code-go` declares
   `complete: false` and a test pins it.

## Two real bugs caught live (the point of a dress rehearsal)

The seam-level tests were green, but the live run surfaced two defects that only
the full-app path exposed:

1. **Missing `inject` on the bridge.** `YaegiCodeRuntime` read
   `ctx.sandbox`/`ctx.subprocess` without declaring them, so every `run_code`
   failed with `cannot get property "sandbox" without inject`. Fixed with
   `static inject = ['subprocess', 'sandbox']`.

2. **Camelization mismatch.** The Go SDK renderer declared `tools.Bash`
   (PascalCase) but the sidecar exported raw tool names (`bash`); the model
   worked around it with `tools.bash`. Fixed by making the sidecar camelize to
   exported identifiers (`sidecar/camelize.go`), matching the renderer, with a
   parity test (`bash→Bash`, `str_replace_editor→StrReplaceEditor`, …).

Both are the kind of implementation bug Choir should expect to hit in its own
Yaegi broker, and both are now recorded with their fixes.

## Verification snapshot

- `tsc -p tsconfig.host.json --noEmit`: **0 errors** (bridge integrated into the root project graph)
- `pnpm run build`: full 239-package lib build succeeded
- `go test ./...`: sidecar + spike + camelize parity green
- built-lib e2e: plain-Node runs a Go program through `lib/index.js`
- `code-go-mount.spec.ts`: 6 tests (shape + Loader round-trip over built lib)
- full source suite: **418 tests green**
- live run: `dsh --profile headless --patch /tmp/yaegi-rehearsal/code-go-overlay.yml` → exit 0

## Honest non-completions

- The live-model proof was captured as a verified run transcript, **not** yet
  converted into the repo's `DSH_SNAPSHOT=record` keyless-replay fixture (a
  `.cordis.snapshot.yml` + fixture capture under the snapshot harness). That is
  a mechanical follow-up, not a blocking gap.
- The full *shipped* `code-go` preset's literal Loader mount requires the
  terminal/fs/subprocess/sandboxPolicy providers only the complete app host
  composition supplies; the reduced-composition Loader round-trip over built lib
  is proven, and the runtime-level path is proven by the live headless run.
- The OS-level containment gate (netns/seccomp/rlimit) remains Linux-only and
  was deliberately not claimed — the rehearsal proves process-per-run, not OS
  isolation of a hostile Go program.

## Documents relocated

As directed, the two mission documents now live in the new repository
(`~/deepseek-harness/docs/`):

- `dsh-go-yaegi-code-mode-rlm-rehearsal-2026-08-17.md` — the confirmed, executed Definition
- `dsh-go-yaegi-code-preset-architecture.md` — the prior research synthesis

## Residual risks and the next realism axis

- The rehearsal proves the seam + round-trip + process containment. What it
  does **not** rehearse — correctly excluded — is Choir's durable-actor
  continuity (mailbox, wake, trajectory settlement), assignment-scoped module
  authority, and the supervised multi-actor protocol. Those remain the real
  mission's center of gravity, untouched here.
- The next realism axis for Choir is to replay the **authority** layer: prove
  that a Go activation cannot reach host authority it was not handed, under a
  real OS confinement, with a durable continuation across forced activation
  death — the two acceptance items this rehearsal scoped out.
