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

status: open_handoff

mission conjecture: if Node B Nix retention is converted from emergency-only
cleanup to a routine generation/root/GC/optimise policy with typed rollback
exceptions, then launch operations can keep enough rollback/cache value while
preventing recurring Nix-store growth from threatening deploys or VM recovery.

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

variant (ranking function) V: 7 open obligations:
1. root inventory classifies ad hoc roots as keep/delete/formalize;
2. generation retention target selected with rollback rationale;
3. GC cadence and `min-free` / `max-free` thresholds selected;
4. store optimisation cadence selected or explicitly rejected;
5. host config implemented and parsed;
6. CI/deploy and staging identity captured;
7. post-cleanup proof shows protected surfaces intact and routine policy
   active. Current V=3. Completed: 1-4. Remaining: 5-7.

budget: one focused implementation mission after this paradoc. Solvency:
fits if the first pass is read-only inventory plus a small host-config patch;
does not fit if it tries to redesign guest-image build distribution at the
same time.

authority / bounds: this document authorizes planning only. Later execution
requires explicit mutation-class declaration and owner approval before
deleting roots manually or changing rollback retention. Declarative scheduled
GC policy changes are orange platform behavior. Manual root deletion or Nix GC
outside the declared service path is red and must name rollback/refusal
evidence.

mutation class / protected surfaces: current doc is green. Future host policy
change is orange. Future live cleanup is red. Protected surfaces are
`/nix/store`, `/nix/var/nix/profiles/system-*`,
`/nix/var/nix/gcroots`, `/tmp/go-choir-*result`, guest image closures,
vmctl guest image paths, and Node B deployment rollback.

evidence packet: pre-change `df`, `/nix/store` size, dead-path count/sample,
root inventory, generation list, current vmctl guest image paths, Nix config,
CI/deploy run, post-change `systemctl list-timers`, service journal, dry-run or
actual GC output, post-cleanup `df`, and proof that current/rollback closures
and vmctl guest paths still resolve. Current root classification evidence:
`docs/evidence/node-b-nix-root-classification-2026-06-14.md`.

heresy delta: discovered existing Nix dead-path/root pressure and ambiguous
ad hoc roots; introduced none in planning; repaired only after a later deployed
policy proves routine cleanup without losing rollback capability.

position / live conjectures / open edges: read-only root classification on
2026-06-14 found root available `122.16 GiB`, root used `350.96 GiB`,
`/nix/store` apparent `243G`, 9,423 dead store paths, and no unknown deploy
roots after classifying `/root/.cache/nix/flake-registry.json` as a small Nix
cache root. Active roots to keep: `/run/current-system`, `/run/booted-system`
until reboot convergence is understood, running-process roots, active service
runtime pointers, active guest image files, and the four newest system
generations 493-496. Stale rollback candidates: generations 489-492. Ad hoc
roots to formalize/delete after identity proof: `/tmp/go-choir-*result`,
`/tmp/guest-image-*`, `/tmp/go-choir-guest-image-new`, and invalid
`/opt/go-choir/result`. Exact policy selected for implementation:
four-generation retention, daily off-peak retention sweep targeting 180 GiB
free with a 120 GiB emergency floor, Nix daemon `min-free=120 GiB` and
`max-free=180 GiB`, weekly off-peak `nix-store --optimise`, and typed TTL/owner
rules for ad hoc result roots. Open edge: do not manually delete roots or run
ad hoc GC from the report alone; root cleanup must happen through the reviewed
service path or a separate owner-approved manual action.

next move: implement the selected routine retention policy in `nix/node-b.nix`
and deployment preflight without manual live GC/root deletion; keep operator
report tooling ignored by deploy-impact; then run focused tests and deploy.

ledger file: docs/mission-node-b-nix-store-retention-v0.ledger.md

version / lineage: v0 follows
`docs/mission-node-b-storage-retention-v0.md` after Codex-domain VM-state
cleanup settled. This mission must not re-open VM-state deletion except as
read-only context.

learning state: retain Nix-store retention findings here until a deployed
policy is proven, then promote the stable operating rule into the repo
operating contract or Node B runbook.

settlement: settled only when Node B has a deployed routine Nix retention
policy, the policy has run once successfully, docs-only commits still avoid
host deploy, guest-image builds are not unexpectedly triggered, current and
rollback closures still resolve, vmctl guest image paths are intact, and disk
headroom remains above the selected target after cleanup.

## Suggested Goal String

```text
/goal Use Parallax on docs/mission-node-b-nix-store-retention-v0.md. Treat it as the next source program after Node B VM-state cleanup. Current status is open_handoff: live read-only evidence shows /nix/store at 243G, root filesystem 357G used / 117G free / 76%, system generations 489-496 retained, 9409 absolute dead /nix/store paths, largest dead paths dominated by old guest-image/playwright-image/runtime-deps/closure-info outputs, and ambiguous ad hoc GC roots under /tmp/go-choir-*result, /tmp/guest-image-*, and /opt/go-choir/result. Do not run live Nix GC, delete roots, delete guest images, or change rollback generation retention without typed root classification, rollback/refusal evidence, and explicit authorization. First next move: build a read-only root classification report grouping roots into current/booted systems, retained system generations, active service/guest-image roots, temporary build-result roots, invalid roots, and unknown roots; then propose exact keep/delete/formalize policy before editing nix/node-b.nix. Target policy to test: keep 3-4 generations, scheduled GC routinely rather than below-100G only, Nix daemon min-free/max-free backstop around 120G/180G, weekly off-peak store optimisation, and formal TTL/owner rules for ad hoc result roots. Ledger: docs/mission-node-b-nix-store-retention-v0.ledger.md. Settlement requires deployed routine Nix retention, one successful service run, CI/deploy evidence, current/rollback and vmctl guest paths intact, docs-only commits still avoiding host deploy, and disk headroom above target.
```
