# M11 freeze leg: secret-screen false positive + execution-ref prefix mismatch

**Date:** 2026-09-28
**Class:** red (self-development transition authority — the freeze leg decides
what the computer's canonical event chain binds)
**Status:** open — documented first per the problem-documentation-first
invariant; fix commits follow this receipt.
**Cluster note:** this is the third distinct defect in the freeze leg within
two days (base-head equality repaired 2026-09-27 commit 3812ec3e; no-runtime-
artifacts + never-thaw repaired same day). Per the convergence rule this is a
substrate-level trail, not three unrelated bugs — see "clustering" below.

## Evidence

Probe `M11_SELFDEV_EPISODE_1790581274029`, deployed `d3dc1445` (post
turn-consumption + actuator=tools deletion), computer
`computer-aee82d4e1ffd55a0bd675351f3500e28`, operation
`selfdev-d96350bce547878ed626479be6096358`, desk run
`run:assignment-28abe1fc-4120-57b8-96c2-09f2bde46a4a` (attempt 2 after a VM
hibernation passivated attempt 1 — the restart-recast path worked).

- The desk authored the evidence doc
  (`docs/evidence/m11-selfdev-episode-1790581274029.md`, 13,274 bytes), built a
  52-file release tree from the nix-store autoputer, and verified in-capsule.
- **`choir.Freeze` call A (cell seq 477)** failed:
  `reduce: persist tray-1: capsule release refuses secret content in
  "var/lib/artifact/release/frontend/assets/pdf.worker-B1D2UnXD.mjs"`.
- The desk deleted the vendored file, rebuilt the manifest, and retried.
- **`choir.Freeze` call B (cell seq 491)** failed differently:
  `reduce: persist tray-1: complete capsule effect bundle draft unavailable`.
- Probe timed out at the 60-minute `awaiting_approval` wait; the operation is
  pinned `executing`; three episode runs total have now died at this leg.

## Defect 1 — secret refusal false-positive on generated assets

`StageGrantedRelease` (`internal/capsule/executor.go:1420-1480`) scans staged
release files: binary payloads get `DetectBinarySecrets` (structural subset),
text payloads get `DetectPrivateSecrets` — the **redaction-grade** pattern set.

`pdf.worker-B1D2UnXD.mjs` (vendored pdf.js worker, 2.1 MB, shipped byte-identical
by the deployed product) trips `credential_assignment` on pdf.js internals:

```text
PASSWORD: 0x0002000        ← bitmask enum member (PasswordResponses)
token = this.getToken()    ← 10+ member-access hits
password = this.#decodeUserPassword(passwordBytes)
password: this.data.password / password = password / password = utf8StringToString(password)
```

27 matches, all program structure, zero credentials. The file's own desk-side
`grep -E` screening of the same candidate patterns found no hits — the guest
screener is strictly more permissive than the release gate, which is the
inversion: a vendored, already-deployed artifact can never be re-staged.

**Root cause:** `DetectPrivateSecrets` is documented as redaction-grade —
"over-matching only costs extra encryption" (`redaction.go:34`). Reusing it as
the refusal predicate on text payloads repeats the binary-payload mistake the
`TestStageGrantedReleaseAdmitsBinaryStringBlob` regression test already
documents for `bin/autoputer` (linker string-blob adjacency + symbol-name
`sk-` prefixes). Minified/vendored JS is the same class of content: machine
produced, densely packed with credential-shaped tokens that are identifiers,
not secrets.

## Defect 2 — `validExecutionRef` rejects the prompted receipt prefix

`internal/capsule/transaction/builder.go:125-128`:

```go
return strings.HasPrefix(ref, "capsule-exec:sha256:") &&
    computerevent.IsSHA256(strings.TrimPrefix(ref, "capsule-exec:sha256:"))
```

But the engineering overlay prompt
(`internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml:55`) instructs
the desk to pass `capsule-go-eval:sha256:` receipt refs to `choir.Freeze` —
which is also what `Executor.go:722` actually mints for `capsule_go_eval`
cells and what `ResolveGrantedExecutionReceipts` accepts
(`executor.go:879-880`). The desk passed correct `capsule-go-eval:` refs on
both freeze calls; resolution succeeded, then `Validate(false)` refused the
bundle because `BuildRecipeRef`, `TestReceipts`, and
`DependencyToolchainRefs` all carry the `capsule-go-eval:` prefix.

Freeze can **never** complete on the documented contract — the resolution path
admits the refs, then validation rejects them. Freeze A masked this (it died
earlier at release staging); freeze B exposed it.

## Scope of the fix

1. `internal/computerevent/redaction.go`: add `DetectRefusalSecrets(payload)` —
   private-key + provider-prefix patterns plus `credential_assignment` filtered
   to non-code values: refuse quoted string literals and unquoted values that
   are not hex literals (`0x…`), not `this.`/member/call/index expressions
   (contain `.`, `(`, `[`), not camelCase (contain uppercase), and not equal
   to the key. Keeps `api_key=abcdefghijklmnop` refusing.
2. `internal/capsule/executor.go`: text path calls `DetectRefusalSecrets`
   instead of `DetectPrivateSecrets`; binary path unchanged.
3. `internal/capsule/transaction/builder.go`: `validExecutionRef` accepts both
   `capsule-exec:sha256:` and `capsule-go-eval:sha256:` — matching the
   resolution contract at `executor.go:879`.
4. Tests: fixture case for the pdf.js patterns (PASSWORD hex enum, member
   `token = this.getToken()`) admitted; `api_key=abcdefghijklmnop` still
   refused; `capsule-go-eval:` refs validate in `Validate(false)`.

## Clustering assessment

Freeze leg defects so far, all same substrate (freeze intent → bundle build →
release stage → validate):

- 2026-09-27 `m11-freeze-base-head-equality-unsatisfiable` — pin equality vs
  projection stream (repaired).
- 2026-09-27 second defect same doc — no runtime artifacts + never-thaw +
  `%!w(<nil>)` (repaired).
- 2026-09-28 this doc — redaction-grade detector reused as refusal predicate
  on text + ref-prefix contract mismatch (open).

Common shape: each freeze precondition was authored against a stricter
assumption than the environment that exists (canonical head is not a stable
pin; text payloads are not human-authored; ref prefixes split across two
spelling conventions). There is no replacement implementation to wire in —
the fixes are small precondition corrections. If a fourth distinct freeze-leg
defect appears, stop patching and do the structural assessment of the whole
`freezeCapsuleEffectBundle` precondition chain.

## What proves closure

`scripts/m11_selfdev_episode_probe.mjs` on staging reaches `awaiting_approval`:
the desk's `choir.Freeze` commits the op to `frozen`, verifier records pass,
op reaches `awaiting_approval` — then approval/apply/restore legs complete.
