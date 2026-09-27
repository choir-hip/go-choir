# M11 blocker: release secret gate refuses the genuine autoputer binary

**Date:** 2026-09-27
**Class:** red (self-development transition authority — the release staging
gate decides which files the frozen bundle may bind)
**Status:** documented; fix pending.
**First observed:** probe `M11_SELFDEV_EPISODE_1790540781081`, staging
`89970ebd`, computer `computer-e138a1e0d26ece87f551ca628dc5fc53`, operation
`selfdev-97e5f7d2db3a1b67835c887c8002630c`, desk run
`run:assignment-27d91e7e-7463-557b-94b0-ab3ecee01a67`.

## Evidence

The desk executed the full workflow end to end and landed `blocked`
(terminal, honest) at iteration 81:

- The desk authored its change (`var/lib/artifact/evidence/m11-selfdev-
  episode-1790540781081.md`, 2738 B) inside the capsule.
- It staged a genuine release payload at the canonical rootfs-relative
  prefix the freeze reads, `/var/lib/artifact/release`: 52 files,
  159467496 bytes — `bin/autoputer` (mode 0755) plus `frontend/**`
  byte-identical to the nix-built artifact (`diff -rq` clean).
- `choir.Freeze` was staged in clean cells and refused at reduction:
  `capsule release refuses secret content in
  var/lib/artifact/release/bin/autoputer`
  (`internal/capsule/executor.go:1425`).
- The desk reproduced each false positive: `credential_assignment`
  matches the string-blob adjacency `token: %v` immediately followed by
  `WebIdentityCredentials` inside the linked binary; `openai_key`
  matches Go symbol names `sk-session…` and `sk-Idx-oss-worm-id`.
  `password`/`api_key`/`secret`/`private-key` patterns are clean. No
  test or doc covers the refusal path.
- Because no frozen bundle exists, no verifier activation and no
  approval were requested; the approval gate was never crossed.

## Root cause

`StageGrantedRelease` (`internal/capsule/executor.go`) runs
`computerevent.DetectPrivateSecrets` line-by-line over every release
file. That detector is a **redaction-grade** pattern set built for
event-payload redaction, where over-matching only costs extra
encryption. Reused as a **refusal predicate** on binary payloads it is
structurally wrong: its broad keyword regexes
(`credential_assignment`, `openai_key`) match benign adjacent bytes in
linker string blobs and Go symbol tables. An honest release payload —
the product's own compiled binary — cannot pass, so the freeze leg is
structurally unsatisfiable.

## Supporting findings (cell-proven, same leg)

1. The release prefix in the diff is rootfs-relative
   `var/lib/artifact/release/`; staging under the subject path yields
   `capsule release contains no frozen runtime artifacts`.
2. Invoking the Go toolchain writes `$HOME/.config/go/telemetry/**`
   (capsule `HOME=/root`), which the commit classifier rejects as
   unknown paths (`unknown paths rejected at commit time`) — recurring
   noise for any Go-agent cell.
3. Capsule artifact paths are mode-stripped; payloads must be written
   with explicit numeric modes or via the broker.
4. An in-capsule `go build` cannot work (networkless: proxy.golang.org
   DNS denied; empty module cache), so a prebuilt artifact staged into
   the capsule is the only available payload source — which the secret
   gate then refuses.

## Fix direction

Give the release gate a binary-aware scan contract. Keep the full
pattern set for text content; for binary content (initial-chunk NUL
heuristic) apply only the structural high-precision patterns
(private-key blocks, bearer tokens, `github_token`, `google_api_key`)
whose byte-level specificity survives string-blob adjacency. Add
`/root/.config/go/telemetry` to the classifier ignore set for finding
(2).
