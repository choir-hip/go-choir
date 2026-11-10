# A failed health check rolls back the guest ring walk, but nothing rolls back a host deploy (2026-11-10)

Date: 2026-11-10. Found by: the X5 code read of the overnight Autopaper
mission. Status: open, problem record before any fix. Mutation class of a fix:
red (deploy routing). Read only; nothing was run on staging.

Question asked: does a failed health check during a ring rollout roll the ring
back automatically? The design promises it
(`docs/design/progressive-delivery-2026-10-26.md:791-797`: "automatic code
rollback on a broken health check (slice S6)"; `docs/design/ring-ordered-deploys-2026-11-03.md`
sections 7 and 8). The answer is split: yes for the guest release, no for the
host.

## The chain for a guest release (present and wired)

1. Detection. `TickRingWalk` (`internal/vmctl/ring_walk_handler.go:233`) runs
   from the retry loop (`internal/vmctl/deploy_refresh_retry.go:565`). It reads
   each open-ring member's `/health` (`httpRingHealth`,
   `ring_walk_handler.go:209`) and evaluates it in `ringMember.evaluate`
   (`internal/vmctl/ring_walk.go:820`): not ready for 3 strikes after a 5 minute
   first-boot delay (`ringReadinessStrikes`, `ringReadinessInitialDelay`,
   `ring_walk.go:109-110`), an appender wedge for 2 samples
   (`ringWedgeStrikes`, `:111`), a boot-failed crash record, or any crash
   record after the member was first seen on the release. Held and divergent
   members are skipped (`:826`).
2. Trigger. The breach is acted on in the same tick: `step` calls
   `w.rollback` (`ring_walk.go:566`), and `stepExpedited` calls it too
   (`:724`). No CI step or human is involved. The walk is installed at boot:
   `cmd/vmctl/main.go:204-206` (`SetReleasePinner`, `PrimeRingWalk`).
3. What is rolled back (`ring_walk.go:933`). The guest release only. Every
   open ring's members running the target get a planned, marker-backed return
   refresh to the last good release (`stepReturn`, `:971`); every other ring
   closes `stopped_by_rollback`; every computer is pinned to the last good
   release (`pinFor`, `:487`) so a cold boot launches it. State and tape are
   not touched. The bad release is not retried: while the walk is
   `rolled_back` the same release id returns early (`observeRelease`, `:433`).
4. Under Ring-Expedite. CI finds the trailer
   (`.github/scripts/node-b-ci-deploy:635`) and posts
   `/internal/vmctl/ring-walk/expedite` (`:662`); a failed post only warns and
   the release walks at the ordinary pace (`:666`). `stepExpedited`
   (`ring_walk.go:713`) opens every ring at once, keeps all guards running over
   every unclosed ring, and a breach in any ring rolls back every computer
   that took the release. R0 and R5 (d@e, the paper) are watched and hold
   until seen healthy.
5. Crash-loop hold. `evaluate` skips a held member (`ring_walk.go:826`), so a
   held computer neither breaches nor gets a return; its next owner start
   launches the pinned release. Crashes 1 and 2 before the hold count as
   breach evidence.

Tests present: `TestRingGuardsKnownBadFixtures`, `TestRingTickBreachEnrollsReturnAndHoldsTheRest`
(handler level, end to end), `TestRingRollbackAtR0StopsWalk`,
`TestRingWalkSupersedeAndReturn`, `TestRingWalkContainmentPinAndGate`,
`TestRingExpediteOpensEveryRingAndGuards`, `TestRingExpeditedRollbackClosesOtherRings`,
`TestRingExpediteHoldsProofAndPaper`, `TestRingExpediteEndpoint`.

## The missing links

- **No automatic rollback of a host deploy.** Slice S6's first half, "a
  post-deploy health probe that, on failure, selects the previous NixOS
  generation" (`progressive-delivery-2026-10-26.md:575, 644`), is only
  documented. `.github/scripts/node-b-ci-deploy` switches the system at `:336`
  (`switch-to-configuration switch`), then curls the six service health ports
  (`:893-900`); a failure there fails the deploy. The EXIT trap
  (`:24-56`) writes an "incomplete" receipt that names `previous_receipt`,
  `frontend-previous`, `<service>-previous` and the NixOS profile as the
  manual rollback, and does nothing with them. Nothing in `nix/` or the
  workflows calls a rollback (grep for `rollback` finds only that receipt
  field). A bad host service, frontend or vmctl build stays live until a human
  reverts. The guest-release walk does not help here: vmctl is itself a host
  service, and a vmctl that fails to start runs no walk.
- **No test of the CI trailer path.** The `Ring-Expedite` parsing and the post
  to the expedite endpoint (`node-b-ci-deploy:622-668`) have no test in
  `.github/scripts` or `scripts`; the endpoint itself is tested
  (`TestRingExpediteEndpoint`).
- **No test combines a crash-loop hold with a rollback.** Each is covered
  alone (`TestCrashLoopHoldSurvivesAVMCTLRestart`, `TestRingBootGuardScoped`).

## Not decided here

Whether a host rollback is worth building on one host is the design's own open
question (`progressive-delivery-2026-10-26.md:644`: "the one-host constraint
limits its value"). The cheap shape, if wanted: after the smoke test fails, run
`nix-env --rollback -p /nix/var/nix/profiles/system` and
`switch-to-configuration switch`, then mark the deploy failed. That changes
deploy routing and needs the full red ceremony; it is a separate mission.

Heresy delta: discovered 2 (host rollback is documented and absent; the CI
expedite path is untested), introduced 0, repaired 0.
