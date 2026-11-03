# Delegate capsules see no computer state, so delegate rounds are judged on mocks (2026-11-03)

Date: 2026-11-03. Problem 26. Mutation class of the fix: red (capsule
contents beside model egress; guest capsule executor). Problem first; no
fix in this commit.

Owner rulings: "capsules should be an overlay over the exact computer
state" and "there shouldn't be credentials on computers anyway".

## The problem in one sentence

A self-development engineering capsule in `claude` network mode (the
Claude Sonnet delegate, reached through the gateway's Anthropic relay)
gets no computer-state overlay: `/computer` holds only `receipt.json`,
which says `"withheld: delegate capsule (credential present)"`, so the
delegate builds against mocks.

## Evidence

- Code: `internal/capsule/computer_state.go` `computerStateWithheld`
  withholds whenever `spec.ClaudeDelegate` is set, a `delegateToken` is
  about to reach the broker, or `CHOIR_CLAUDE_DELEGATE_TOKEN`
  (`ClaudeDelegateBrokerTokenEnv`) is in the spawn's environment. Its own
  comment says the rule holds "until the delegate call moves behind the
  gateway".
- The delegate call has moved. 55420cab4 took the owner's Claude token off
  every computer: it lives only on the host, in the gateway's runtime
  directory, and vmctl writes no token to any credential disk. Problem 25
  (7800706b5, deployed 17:05Z) made the relay grant work.
- What a claude-mode capsule holds today (traced 2026-11-03 on
  `origin/main` 0ab560c90):
  - `executor.Spawn` mints a per-capsule relay secret
    (`NewClaudeRelaySecret`, `choir-relay-` plus 32 random bytes) and
    passes it as `delegateToken` to `prepareComputerState` and as
    `CHOIR_CLAUDE_DELEGATE_TOKEN` to the broker.
  - The broker (`cmd/capsule-broker/delegate.go`) removes it from its
    environment and hands it only to the claude process as
    `ANTHROPIC_AUTH_TOKEN`; `claudedelegate.Env` strips every other
    `ANTHROPIC_*`, `CLAUDE_*`, `*TOKEN*`, `*API_KEY*` and `*SECRET*` key.
  - The secret is good only at the capsule's own egress proxy, which swaps
    it for the computer's gateway bearer on the `anthropic.choir.internal`
    route to the relay. A claude-mode capsule has no CONNECT host at all.
  - No path left on the guest sets the owner's raw token in a broker or a
    capsule environment. The withholding rule now fires on the relay
    secret alone.
- Effect on the luna arm, 2026-11-03: the first round where the delegate
  did the work end to end (operation
  `selfdev-3d4b74c01f47ebb22a70e8ef773c3691`) was rejected by the judge for
  "mocked slides, routes, and fixtures rather than authentic computer
  content"; its capsule had no `/computer/state.db`. The claude arm's
  claude-mode rounds have the same gap.

## Why it matters

The delegate is the engineering arm that does the most work per round, and
it is the one arm that cannot see the computer it is building for. Rounds
it runs are judged on fixtures, and the comparison between arms measures
the overlay, not the agents.

## Proposed fix

Withhold the overlay only when the owner's raw Anthropic credential is
present (the original reason), not for the gateway-scoped relay secret: a
`delegateToken` or `CHOIR_CLAUDE_DELEGATE_TOKEN` that is not a relay
secret, an `ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN` or
`CLAUDE_CODE_OAUTH_TOKEN` in the spawn environment, or any value shaped
like an Anthropic key. The leave-outs stay exactly as for other
implementation capsules. Verification capsules still get no `/computer`.

## Security effect of the fix (not none)

A claude-mode capsule will hold the computer's files snapshot and store
copy beside an egress path to the model API: through the relay only,
Anthropic only, the delegate's one model, rate-limited, with the gateway's
per-computer daily token cap. Content in `/computer` can reach Anthropic
as model context. That is the delegate's purpose, and the owner authorized
delegating on these computers.

What the overlay already leaves out, unchanged by the fix: `mail/` (the
owner's emails), `Source/platform`, files without `o+r`, symlinks and
special files, helper temporaries; `/mnt/persistent` and `/run` (identity
keys, gateway token) stay masked; the store copy's secret-shape scan
blanks credential-shaped values, and the guest store has no mail tables
(they live in the host's maild store, `internal/maild`).
No narrower guard is proposed now. The ones worth naming as residuals: a
per-capsule upload bound on relay request bodies below the 512 MiB claude
policy, and the relay logging the bytes of context it forwards per
capsule.
