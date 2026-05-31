# Super Console Real Zot Cutover

Date: 2026-05-31

## Problem

Super Console currently presents itself as zot repair mode, but the deployed
runtime is still able to start the in-tree placeholder session instead of the
actual upstream `zot` program. The observed symptom is that arbitrary user input
such as `ls`, `hi`, or "how do I submit a bug" receives the same canned
diagnosis-report path. That is not zot; it is a narrow fallback loop.

The same surface is also unreadable under London Salmon because terminal
foreground color follows the light theme's dark text while terminal background
stays hard black. The resulting dark-red-on-black text blocks human or
computer-use inspection of Super Console.

## Evidence

- User screenshots on 2026-05-31 showed Super Console printing:
  `zot repair session zot-2`, `session log: .../.choir/zot/sessions/...`,
  then repeatedly `diagnosis report: .../diagnosis.md` for unrelated prompts.
- Local code inspection showed `cmd/sandbox zot-session` routes to
  `internal/zot.RunSession`, while `internal/zot/session.go` writes the
  repeated diagnosis report for every non-`!` line.
- `frontend/src/lib/SuperConsoleApp.svelte` mapped terminal background to
  `--choir-surface-media` while London Salmon's primary text color is dark.
- Upstream zot was identified as `https://github.com/patriceckhart/zot`, tag
  `v0.2.6`, commit `917da8c414e183118e68034e0e8c6f6b746f0132`.

## Belief State

Super Console must start a separate zot process inside each user computer. The
in-tree placeholder can remain only as a degraded fallback when no packaged or
PATH-provided zot binary exists; the normal product path must set
`CHOIR_ZOT_PATH` to a real binary. Zot state must live in the user computer's
persistent filesystem through `ZOT_HOME`, not in process-local host state.

The UI should not special-case London Salmon by name. It should choose a
terminal background that preserves contrast between the active theme's text and
the terminal surface.

## Build Probe

Before landing, a scratch tree containing only the Super Console/Zot changes was
copied to Node B at `/tmp/go-choir-zot-build` and built there:

- `nix build .#zot` produced `zot 0.2.6 (917da8c, 2026-05-30T17:33:08Z)`.
- Host NixOS closure built in 122 seconds:
  `/nix/store/hxii9n1az753iyhmap12ggi2na1z77xb-nixos-system-go-choir-b-26.05.20260409.4c1018d`.
- Ordinary guest image built in 35 seconds:
  `/nix/store/2shmf5gy5v1mdc4hwzks7ka1vpv59g3g-go-choir-guest-image`.
- Playwright guest image built in 44 seconds:
  `/nix/store/jsnjggag6blqd2w394d0d84b7n093xzk-go-choir-guest-image-playwright`.
- Nix eval showed both host and guest sandbox service configs setting
  `CHOIR_ZOT_PATH=/nix/store/vq83nfypy7a47i77pj7r38p544qbiiix-zot-0.2.6/bin/zot`.

## Remaining Error Field

This checkpoint documents the problem and build feasibility. It does not prove
staging behavior. The fix must still be committed separately, pushed, deployed,
and verified on `https://choir.news` through the product path.
