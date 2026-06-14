# Node B Nix Store Retention Mission v0

This paradoc is the next mission after
`docs/mission-node-b-storage-retention-v0.md`. The prior mission repaired the
runaway Codex-agent VM-state class and left Node B at roughly 76-77% disk use.
The remaining storage risk is now mostly Nix-store and root retention policy:
`/nix/store` is large, dead paths exist, and guest-image build products can
accumulate faster than launch operations can safely reason about them.

The problem is not novel. Nix storage is managed by a known sequence:

1. delete old profile generations so stale closures stop being GC roots;
2. run Nix garbage collection to remove paths unreachable from roots;
3. configure `min-free` / `max-free` as a build-time backstop;
4. run scheduled GC routinely instead of waiting for emergency pressure;
5. run store optimisation separately when the IO budget allows it;
6. make ad hoc result roots either intentional or disposable.

References:

- Nix garbage collection manual:
  <https://nix.dev/manual/nix/2.24/package-management/garbage-collection>
- Nix `nix.conf` `min-free` / `max-free` settings:
  <https://nix.dev/manual/nix/2.34/command-ref/conf-file>
- Nix store optimisation:
  <https://nix.dev/manual/nix/2.24/command-ref/nix-store/optimise>
- NixOS `nix.gc.automatic` option:
  <https://mynixos.com/nixpkgs/option/nix.gc.automatic>

## Live Starting Evidence

Read-only Node B probes on 2026-06-14 showed:

- root filesystem: 476G total, 357G used, 117G available, 76% used;
- `/nix/store`: 243G apparent usage;
- `/var/lib/go-choir/vm-state`: 127G apparent usage after Codex-domain VM
  cleanup;
- current system generations retained: 489 through 496;
- `nix-store --gc --print-dead` produced 9,409 absolute `/nix/store` dead
  paths;
- the largest dead paths include old `go-choir-guest-image-playwright`,
  `go-choir-guest-image`, `runtime-deps`, `closure-info`, and historical
  system-unit/activation outputs;
- a naive `du` sum over dead paths was 460.99 GiB, which overstates precise
  reclaimability because hardlinks/shared store file accounting can double
  count; it is still strong evidence of material collectible garbage;
- `nix-store --gc --print-roots` reported stale temporary roots and one invalid
  `/opt/go-choir/result` root pointing to
  `/nix/store/srzb724qbl2s871jjkjja61zdfqwzv2j-prefetch-npm-deps-0.1.0`;
- live automatic roots include ad hoc `/tmp/go-choir-*result`,
  `/tmp/guest-image-result`, `/tmp/guest-image-playwright-result`, and current
  system generation roots.

The immediate operational question is not "can Nix delete something?" It can.
The real question is what retention boundary gives Choir enough rollback and
build-cache value without allowing old guest-image closures and temporary roots
to grow unbounded before launch.

## Recommended Target Policy

The mission should decide, implement, and prove a policy close to:

- Keep 3 or 4 system generations, not 8, unless an incident explicitly pins an
  older rollback generation with a named expiry.
- Run generation pruning and `nix store gc` routinely, not only below 100 GiB
  free.
- Configure Nix daemon `min-free` / `max-free` as a backstop, e.g.
  `min-free = 120 GiB`, `max-free = 180 GiB`, so large builds self-correct
  before deployment enters a low-headroom state.
- Run `nix-store --optimise` weekly or on another low-traffic cadence, separate
  from deploy, because it is IO-heavy.
- Remove or formalize ad hoc build roots under `/tmp/go-choir-*result`,
  `/tmp/guest-image-*`, and `/opt/go-choir/result`. A root is either a named
  rollback/cache artifact with an owner and TTL, or it should not survive as a
  GC root.
- Prefer building guest images away from the long-lived launch host, or push
  only the current required guest closures to Node B, if repeated image builds
  remain a major storage source.

## Parallax State

status: settled

mission conjecture: if Node B Nix retention is converted from emergency-only
cleanup to a routine generation/root/GC/optimise policy with typed rollback
exceptions, then launch operations can keep enough rollback/cache value while
preventing recurring Nix-store growth from threatening deploys or VM recovery.
Status: supported for the current Node B launch host by deployed policy and
one successful declared service run.

deeper goal (G): launch next week with Node B storage behavior boring enough
that alpha-user growth, docs-only changes, host deploys, and occasional guest
image builds do not require emergency operator intervention or opaque manual
cleanup.

witness/spec (A/S): a reviewed and deployed Nix retention policy for Node B
that includes generation retention, scheduled GC, Nix daemon free-space
backstop, root hygiene, and optional store optimisation; plus a report command
that proves current roots, dead-path pressure, and rollback generations before
and after cleanup.

invariants / qualities / domain ramp (I/Q/D): preserve current and recent
rollback system generations; do not delete live guest images referenced by
vmctl; do not delete real-user VM state; do not treat large reclaim estimates
as exact when hardlinks can double count; prefer declarative Nix/systemd policy
over manual root deletion; prove on Node B staging before claiming settlement.
Ramp from read-only root/dead-path inventory, to dry-run plan, to host-config
change, to one explicit scheduled-service run, to post-cleanup proof.

variant (ranking function) V: original 7 open obligations:
1. root inventory classifies ad hoc roots as keep/delete/formalize;
2. generation retention target selected with rollback rationale;
3. GC cadence and `min-free` / `max-free` thresholds selected;
4. store optimisation cadence selected or explicitly rejected;
5. host config implemented and parsed;
6. CI/deploy and staging identity captured;
7. post-cleanup proof shows protected surfaces intact and routine policy
   active. Current V=0; last delta closed CI/deploy and post-cleanup proof.

budget: one focused implementation mission after this paradoc, spent. Solvency
result: fit because the mission stayed to read-only inventory, a small
host-config patch, normal CI/deploy, and one declared service run. Guest-image
build distribution remains a separate future optimisation, not a settlement
dependency for this mission.

authority / bounds: declarative scheduled GC policy changes were treated as
orange platform behavior and landed through git/CI/deploy. The live cleanup ran
only through the reviewed `go-choir-disk-gc.service` path after explicit owner
authorization. Manual root deletion or Nix GC outside the declared service path
remains forbidden without a separate typed approval and rollback/refusal
evidence.

mutation class / protected surfaces: mission docs/report tooling were green;
host policy was orange; declared service cleanup was red live infrastructure
mutation bounded by the reviewed service path. Protected surfaces were
`/nix/store`, `/nix/var/nix/profiles/system-*`,
`/nix/var/nix/gcroots`, `/tmp/go-choir-*result`, guest image closures,
vmctl guest image paths, and Node B deployment rollback.

evidence packet: pre-change `df`, `/nix/store` size, dead-path count/sample,
root inventory, generation list, current vmctl guest image paths, Nix config,
CI/deploy run, post-change `systemctl list-timers`, service journal, dry-run or
actual GC output, post-cleanup `df`, and proof that current/rollback closures
and vmctl guest paths still resolve. Current root classification evidence:
`docs/evidence/node-b-nix-root-classification-2026-06-14.md`. Behavior commits:
`5505ff9a` added the root classifier/report and `e4bfae61` tightened Node B
retention policy. CI run `27510888401` passed; deploy job `81310709649`
succeeded. Deploy log selected host OS only, skipped frontend install, skipped
ordinary and Playwright guest image build/install, restarted vmctl, and skipped
active computer refresh. Staging health reported deployed commit
`e4bfae6106ad33c9c8f021819b335041348f4078`.

heresy delta: discovered existing Nix dead-path/root pressure and ambiguous
ad hoc roots; introduced no untyped manual cleanup path; repaired for v0 by
deployed routine policy, zero remaining dead paths after service GC, and
preserved rollback/guest image evidence.

position / live conjectures / open edges: read-only root classification on
2026-06-14 found root available `122.16 GiB`, root used `350.96 GiB`,
`/nix/store` apparent `243G`, 9,423 dead store paths, and no unknown deploy
roots after classifying `/root/.cache/nix/flake-registry.json` as a small Nix
cache root. The implemented policy keeps four generations, sets the retention
sweep floor/target to 120/180 GiB, sets Nix daemon `min-free=128849018880` and
`max-free=193273528320`, disables per-build `auto-optimise-store`, enables
weekly off-peak `nix-store --optimise`, and leaves ad hoc manual root deletion
outside the service path forbidden. Post-deploy Node B evidence at
`2026-06-14T20:31:40Z`: `/` and `/nix/store` had `299G` free and `37%` use;
`/nix/store` apparent usage was `31G`; `/var/lib/go-choir/vm-state` was
`122G`; `nix-store --gc --print-dead` reported `0` dead store paths. The
declared service ran from `20:26:51Z` to `20:27:30Z` with `Result=success`,
deleted 9,624 store paths, and freed `220991.43 MiB`. System generations are
494, 495, 496, and 497 current. `/run/current-system` resolves to generation
497 and `/run/booted-system` still resolves as the booted rollback surface.
Ordinary and Playwright vmctl guest image files all still exist.

next move: no required v0 work remains. Future work should be a successor
mission for explicit TTL/owner cleanup of ad hoc result roots, reboot
convergence if desired, and moving repeated guest-image builds away from the
long-lived launch host.

ledger file: docs/mission-node-b-nix-store-retention-v0.ledger.md

version / lineage: v0 follows
`docs/mission-node-b-storage-retention-v0.md` after Codex-domain VM-state
cleanup settled. This mission did not re-open VM-state deletion except as
read-only context.

learning state: retain Nix-store retention findings here until a deployed
policy is proven, then promote the stable operating rule into the repo
operating contract or Node B runbook if the policy remains correct under
launch load.

settlement: settled only when Node B has a deployed routine Nix retention
policy, the policy has run once successfully, docs-only commits still avoid
host deploy, guest-image builds are not unexpectedly triggered, current and
rollback closures still resolve, vmctl guest image paths are intact, and disk
headroom remains above the selected target after cleanup. Status: satisfied for
v0. A docs-only deploy-impact probe for this mission/Pulse docs returned
`deploy_needed=false` and all deploy classes false; the pushed behavior deploy
skipped guest image build/install and active computer refresh; post-cleanup
headroom `299G` is above the 180 GiB target.

## Suggested Goal String

```text
/goal Use Parallax on docs/mission-node-b-nix-store-retention-v0.md. Treat it as the settled v0 Node B Nix-store retention mission after Node B VM-state cleanup. Current status is settled: root classification report exists, four-generation retention and 120/180 GiB floor/target policy deployed, weekly off-peak store optimisation enabled, CI/deploy evidence captured, declared retention service ran once successfully, dead store path count reached zero, `/nix/store` dropped to 31G apparent, root filesystem headroom rose to 299G free, current/booted rollback surfaces resolve, vmctl ordinary and Playwright guest paths are intact, and docs-only deploy-impact still selects no host deploy. Do not reopen v0 except to audit the evidence packet or correct factual drift. For new work, open a successor mission for explicit TTL/owner cleanup of ad hoc result roots, reboot convergence, or moving repeated guest-image builds away from the launch host. Ledger: docs/mission-node-b-nix-store-retention-v0.ledger.md.
```
