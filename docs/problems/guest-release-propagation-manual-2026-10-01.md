# Problem: Guest Release Propagation Is Manual — Owner Directive to Design It

Date: 2026-10-01
Status: **open — design problem; owner-flagged as plausibly more urgent than
World Wire and a prerequisite to it.**
Owner statement (2026-10-01, verbatim): "we need to be sure to update the
yusefnathanson@me.com computer. Indeed, we need to improve our updating system
so we push out updates to guests. Well, it's not so simple since there may be
some compatibility issues since the vms diverge gotta design think this. That
may be even more urgent than the world wire actually, a good prerequisite to
it. But for now we just need to manually keep it up to date."

## What is true today

- A platform deploy updates the proxy/frontend/vmctl services and the release
  artifact pins on Node B, but **running guests do not move**. A guest
  realizes a new build only when something refreshes it:
  `choir computer refresh --computer <id> --idempotency-key <key>` (issued
  manually today) or lifecycle restart events.
- Realized evidence this week: latency cut-1/2 landed at `5439733b`; the
  owner guest sat on it until a manual refresh moved it to `cf0ef207`; a
  second manual refresh was then needed for `8f47a232`. Each miss means the
  owner-facing product (and its measured behavior) diverges from `main`.
- `vmctl`'s refresh record (`/tmp/go-choir-vm-refresh-vm-*.json`,
  `ownerships.json` on Node B) is the effective registry of who runs what
  build, but nothing closes the loop: no deploy → refresh fan-out, no
  staleness report, no auto-retry.
- Ontology already names the mechanism: the **root-owned guest updater**
  (`docs/computer-ontology.md` ~L100) stages and health-checks the immutable
  release before an applied event advances effective state. What is missing
  is the *triggering and ordering discipline*: who decides a guest should
  take a new release, in what order, and with what compatibility checks.

## The compatibility surface that makes this non-trivial

1. **Event schema / reducer version skew.** The guest replays its own event
   tape at boot against the new binary's reducer. If `main` has schema or
   reducer changes the old guest never saw, refresh is a replay-acceptance
   event — not just a binary swap. Guests that diverged (long-lived tapes,
   pinned artifacts, missed intermediate versions) may need staged replay
   rather than a single hop.
2. **Artifact program divergence.** `ComputerVersion = (CodeRef,
   ArtifactProgramRef)`; guests can sit on different artifact programs
   (model manifests, detector manifests, skill bundles). A release pin that
   assumes artifact N breaks a guest pinned at artifact M. The updater must
   carry artifact compatibility, not just commit SHA.
3. **In-flight run semantics.** Refresh mid-drain (as happened today:
   desk_pending 33 → killed runs, guest respawned on a new TAP IP) loses
   in-flight work. The updater needs quiescence rules or an ordered
   drain-then-refresh, not a blind cutover.
4. **Fleet ordering.** `premium_always_on` owner computers vs hibernated
   candidates vs CI-class VMs need different propagation classes. A
   fleet-wide naive fan-out on every deploy would churn every hibernated VM.
5. **Rollback coupling.** If a bad release reaches guests before staging
   acceptance completes, the only recovery is another refresh forward or a
   product restore; there is no per-guest release pin preventing uptake.

## Design questions the updater must answer

- Who is the authority for "this guest should now run release R": deploy
  pipeline receipt, per-computer desired-state event, or owner-visible pin?
- Ordering: can refresh respect `desk_pending_mutations`/running runs and
  schedule at a drain boundary rather than mid-write?
- Compatibility gate: replay-acceptance preflight (does the tape replay
  under the new reducer? does the artifact pin still resolve?) before the
  route slot CAS.
- Staleness observability: `build.commit` is already on guest `/health` —
  a fleet view of "guests not on deployed commit" is a query, not a
  subsystem; nobody runs it today.
- Failure policy: hibernated guests probably should not be woken for a
  deploy; the refresh must fire at next realize instead (lazy propagation
  for cold, eager for premium_always_on).

## Relationship to the mission stack

Not in M2 scope. This is platform plumbing that belongs beside the
capacity-stabilization mission (platform-dolt OOM fix already landed live
2026-10-01 as the substrate unblock). The owner has ranked it ahead of
World Wire work and called it a prerequisite — treat as a candidate for
the next blocking mission once the current latency cut-3 measurement lands.

## Interim discipline (in force until designed)

- Every behavior-changing deploy must be followed by `choir computer
  refresh --computer computer-03335285269bdba4f94377e56879f9e6` (owner,
  `premium_always_on`) when guest behavior matters to acceptance.
- Landing Loop reports must name both `deployed_commit` (proxy) and guest
  `build.commit` — divergence is now normal by design, not a deploy bug.
