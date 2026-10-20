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

## Finding after deploy (2026-10-20 10:00 UTC): the allowlisted computer is gone

4c2d66c38 deployed and vmctl carries the allowlist and the new expiry. The
cold boot then failed: the test computer's key gets `403 api key computer
ownership required`, and `GET /api/computers/computer-33594053…` answers
404. The computer's ownership record is gone; its tape is still on disk
because it has not yet been quiet for the tape sweep's 7 days, which is why
the 2026-10-20 sweep did not list it. The test computer
`v3-events-0146` shows the same pattern (403, not in the vmctl list).
Cause of the removal not traced here.

Replacement: `scripts/qa_test_computer.mjs --label
claude-delegate-sonnet-20261020` minted `computer-ad1b46d4c20129e84b9e239cf4720fb9`
(guest build 4c2d66c38, key expiry 2026-10-27) at 10:01 UTC. It replaces the
dead id on the allowlist; the proof runs on it after a cold boot.

## Staging proof (2026-10-20 10:11–10:12 UTC, on 09e98e9b2): delegate runs, its edit does not reach engineering

Test computer `computer-ad1b46d4…` cold-booted through the lifecycle API
(stop 10:09:45, start 10:10:04). `scripts/claude_delegate_probe.py`:

- `--status-only`: pass. Guest status `available: true`, model
  `claude-sonnet-5-5`, expiry 2026-11-03, guest build 09e98e9b2.
- End to end: **fail**. Trajectory `366115a4-…`, engineering run
  `run:assignment-c64a301390f01c2d40e7f15e`, capsule network mode `claude`.
  - Engineering (gpt-6-luna) precommitted, called `choir-delegate start` and
    `wait`. Delegate run `d20261020T101200-73eeba02` ended `state: done`,
    `subtype: success`, 5 turns, 13 s, $0.039, last tools `Write, Bash,
    Bash, Read`. So the Bun seccomp fix holds on x86_64 staging.
  - stderr tail: "Permission mode forced to default —
    CLAUDE_CODE_SUBPROCESS_ENV_SCRUB is set (allowed_non_write_users
    hardening). Declare allowedTools explicitly, or set
    CLAUDE_CODE_SUBPROCESS_ENV_SCRUB=0 to opt out." `Args` passes
    `--permission-mode dontAsk` (spec.go:163) and the env sets the scrub
    (spec.go:231); the CLI overrides the mode.
  - Engineering then ran `cat docs/probes/claude-delegate-20261020T101114.md`
    in its own cell: **No such file or directory**. It completed the
    assignment `blocked`, citing the delegate's report that its own `cat`
    checks failed because bwrap was unavailable (design §6, known).
- **Not yet known (hypotheses, the delegate's event log in the capsule's
  run dir would decide):**
  1. The forced `default` permission mode denied the Write in `-p` mode
     (look for `permission_denials` in the result).
  2. The delegate stage writes to a different view of `/workspace/platform`
     than the engineering cells see (broker namespace vs cell overlay), so
     a successful Write is invisible to engineering.
- Next probe: read the delegate's `events.jsonl` / result
  `permission_denials` through `choir-delegate status --id …` from an
  engineering cell, before any change. A fix for (1) is to drop the scrub
  variable or declare the tools as the message says, weighed against why
  the scrub was set (token isolation from Claude's subprocesses, F13).

### Hypothesis 1 refuted locally (2026-10-20 ~10:20 UTC)

Local Claude Code 2.1.299 with the delegate's exact flags (`-p`, Sonnet,
`--tools`/`--allowedTools`/`--disallowedTools` as in spec.go, empty MCP,
`--permission-mode dontAsk`) and `CLAUDE_CODE_SUBPROCESS_ENV_SCRUB=1` prints
the same "Permission mode forced to default" warning, then writes the file:
`subtype: success`, `permission_denials: []`. Write is on the explicit allow
list, so the forced mode does not refuse it. The guest's Claude Code version
may differ; the evidence moves to hypothesis 2 (the delegate and the cells see
different views of `/workspace/platform`), under investigation.
