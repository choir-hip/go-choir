# Claude delegate resumes on Sonnet (2026-10-20)

Owner, 2026-10-20 (owner notes §28): "we should resume using Claude
(sonnet) within Eng desk (shelling out). We set that up then stopped it.
Let's resume that". This is the first step toward rewriting the
less-important apps through self-development, which waits on app packages.

## State found

- The delegate is built and in the tree, switched off by an empty
  allowlist (431dc5d3b, 2026-10-15). The reason was quota, not safety:
  "I'm at 87% of my token limit" (owner notes §21). Design:
  `docs/design/claude-delegate-slice1-2026-10-14.md`; research and terms:
  `docs/reports/claude-agent-sdk-in-choir-2026-10-14.md`.
- Shape: the engineering desk's implementation capsule runs unmodified
  `claude -p` through `choir-delegate` and `capsule-broker`, with egress only
  to `api.anthropic.com:443` and the Go module proxy. The token reaches the
  `claude` process only as `CLAUDE_CODE_OAUTH_TOKEN`; engineering re-runs the
  tests itself.
- `internal/claudedelegate/spec.go` pins `Model = "claude-opus-5-5"`.
- `nix/node-b.nix`: `CHOIR_CLAUDE_DELEGATE_COMPUTERS=` (empty) and
  `CHOIR_CLAUDE_DELEGATE_EXPIRES=2026-10-22T00:00:00Z`, two days out.
- The token file `/var/lib/go-choir/secrets/claude-oauth-token` is still
  installed (root, 0400, 108 bytes, written 2026-10-15 01:54 UTC; checked by
  `stat`, never read).
- The allowlisted test computer `computer-33594053a05f46e93d80e7e15ce8a0d2`
  is not running (absent from the vmctl list) and was not removed by the
  2026-10-20 tape sweep (no archive), so it boots on demand.

## Change (red)

1. `Model = "claude-sonnet-5-5"`.
2. Re-add the test computer to `CHOIR_CLAUDE_DELEGATE_COMPUTERS`.
3. Move the hard expiry to 2026-11-03T00:00:00Z: two weeks, renewed by
   an explicit edit, so a forgotten grant still lapses.

- **Conjecture delta.** The delegate on Sonnet does useful engineering work
  inside the capsule at a quota cost the owner accepts. Refuted if a run
  fails on the model name, the capsule kills `claude` (seccomp), or one
  run spends a visible share of the subscription.
- **Protected surfaces.** Provider calls (Anthropic, outside the gateway),
  the credential disk path in vmctl, capsule network policy. No change to
  Texture writes, Trace, checkpoints, auth or routing.
- **Admissible evidence.** One staging run on the test computer:
  `choir-delegate` status reports `available: true` and model
  `claude-sonnet-5-5`; one small delegated task finishes with
  `subtype: success`, and engineering's own test re-run passes.
- **Rollback.** Empty the allowlist and deploy (the grant lapses at the next
  boot, and in any case at the expiry); revoke the token if exposed.
- **Heresy delta.** Discovered: none. Introduced: none (the exception is the
  owner-ratified option T, §14/§19). Repaired: none.

## Open risks carried from the slice

- The seccomp fix for the Bun timer (fea6d7c5b,
  `docs/problems/claude-delegate-bun-timer-seccomp-2026-10-15.md`) was
  verified on aarch64 in Docker only; this is its first staging run on
  x86_64.
- Claude's own Bash tool fails inside bwrap without `CAP_SYS_ADMIN` (design
  §6); delegated tasks rely on file edits, and engineering runs the tests.
- Landlock isolation of `/proc/<pid>/environ` (F13) is unverified on the
  staging kernel.
- Terms: the token stays in the unmodified binary's environment, for the
  owner's own computer, never relayed or shared. Re-read the terms wording
  before any second computer is allowlisted.
- Quota: the subscription that ran out on 2026-10-15 is the same one.
