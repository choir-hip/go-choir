# Mission: Node B Storage Retention And VM State Prevention v0

## Summary

Node B is operating with high disk usage because two durable storage pools have
grown without an owner-visible budget:

- `/var/lib/go-choir/vm-state`: vmctl's durable computer state root, dominated
  by protected primary-computer `data.img` files and manual recovery snapshots.
- `/nix/store`: host build/deploy closure cache, preserved by system
  generations and explicit build-result GC roots.

The immediate owner-primary VM recovery incident is repaired, but the storage
conditions that made recovery fragile remain. This mission builds the retention
and reporting path that prevents recurrence without deleting real user state.

## Current Answers

### What is `vm-state`?

`/var/lib/go-choir/vm-state` is the host-side vmctl state root. It is broader
than any single guest disk. It contains:

- `ownerships.json`, the durable vmctl routing/ownership registry;
- one `vm-*` directory per computer;
- per-VM launch/lifecycle metadata such as `epoch`, `fc-config.json`, and
  `firecracker.pid`;
- each VM's mutable guest disk, usually `data.img`;
- ad hoc recovery artifacts such as `data.img.pre-prune-*` or
  `data.img.corrupted-*`.

`data.img` is the guest's persistent disk mounted inside the VM at
`/mnt/persistent`. It contains user-computer state such as VText/Dolt data,
uploaded files, caches, runtime material, and local work artifacts. In other
words: `vm-state` is the vmctl storage universe; `data.img` is one computer's
mutable disk inside that universe.

### What are manual recovery snapshots?

Manual recovery snapshots are operator-created copies of a VM's `data.img`
placed next to the live image before risky repair actions such as mounting the
guest ext4 filesystem, running `e2fsck`, pruning guest caches, expanding the
image, or attempting recovery from a suspected corrupt disk. They are rollback
refs for the incident, not product-managed backups.

The operator VM currently has three such files:

- `data.img.pre-prune-20260609T224644Z`: documented snapshot taken before
  pruning rebuildable guest caches during the 2026-06-09 recovery.
- `data.img.pre-prune-20260610T064824Z`: likely a second pre-prune/pre-repair
  snapshot from the 2026-06-10 follow-up recovery window; verify before
  deletion.
- `data.img.corrupted-20260610T065012Z`: likely a quarantined copy from a
  suspected-corruption or fsck recovery branch in the same 2026-06-10 window;
  verify before deletion.

The first is fully explained by the incident doc. The latter two have enough
filename/time evidence to classify as recovery artifacts, but not enough
evidence to delete blindly. A retention system must attach metadata at creation
time so future agents do not have to infer from filenames.

### Which fake users are allowed to be pruned?

The owner explicitly wants to keep real user `yusefnathanson@me.com` and also
uses `a@b.com` and `b@c.com` for testing. This mission should treat those as
protected test accounts unless the owner later says otherwise.

Disposable/fake-user cleanup must be identity-aware, not only domain-aware.
The current `example.com` classifier is too narrow, and a broad "unknown domain"
classifier would risk deleting owner-used test accounts.

### What should happen to `/nix/store`?

`/nix/store` should be pruned more frequently and against a budget, not only
when root free space falls below the current emergency floor. Today the daily
disk sweep deletes old system generations and preserves the warm Nix cache when
there is still more than about 40 GiB free. That lets disk usage sit around
85-86%.

The deploy workflow also leaves GC roots such as:

- `/tmp/go-choir-guest-image-new`
- `/tmp/guest-image-result`
- `/tmp/guest-image-playwright-result`
- `/tmp/go-choir-nixos-result`
- `/tmp/go-choir-service-*-result`
- `/opt/go-choir/result`

Some are useful current/rollback pointers; some may be stale build roots. The
mission must replace accidental roots with an explicit current/rollback root
policy before increasing GC aggressiveness.

### Why can storage-tooling edits rebuild images?

The deploy impact classifier currently treats `scripts/node-b-storage-report`,
`scripts/node-b-storage-proof`, and `scripts/node-b-storage-verify-report` as
unknown deployed paths. That is conservative, but it means a report-tooling-only
change requests host OS plus ordinary and Playwright guest image builds. This
is a repeatable source of slow deploys until those scripts are explicitly
classified as operator/report tooling or moved behind a deployed service path.

This is separate from docs-only commits. The workflow path filters already make
docs-only commits run Docs Truth Check only; commit
`25c4242bbbad89fe150a782f50b3e27a7501fe0c` proved that path.

## First Read-Only Report Evidence

`scripts/node-b-storage-report --host node-b --top 12` now emits a report-only
storage classifier. The 2026-06-14 run completed in 9.155 seconds over SSH and
performed no deletion, Nix GC, service restart, or VM mutation.

Current report findings:

- root filesystem: 476G total, 393G used, 81G available, 84% used;
- `/var/lib/go-choir/vm-state`: 163.49 GiB allocated;
- `/nix/store`: 236.64 GiB allocated;
- vmctl active retention candidates: 0 bytes, because no VM state matches the
  current retention prune policy;
- manual recovery snapshots: 4 files, 23.82 GiB, review-only;
- known protected user ID `5bd6de97-3b58-408c-bf89-c42c81b083de`: 31.61 GiB,
  refusal;
- platform VMs/state: 16.77 GiB, refusal;
- synthetic non-UUID users such as `diagnostic-*`, `obscura-proof-*`, and
  `sourcemaxx-*`: 13 ownerships, 7.62 GiB, owner-review only;
- UUID primary VMs without email mapping: 107.50 GiB, refusal until auth email
  identity is exposed to the report;
- known deploy Nix roots: review-only; one `/opt/go-choir/result` root points at
  a missing store path.

The first report deliberately listed `yusefnathanson@me.com`, `a@b.com`, and
`b@c.com` as protected email policy while refusing UUID primary VMs because
`ownerships.json` stores user IDs rather than emails. The follow-up
identity-mapped report below resolves that missing oracle for the protected
accounts.

## Identity-Mapped Report Evidence

The report now locates an existing `sqlite3` binary in `/nix/store` and opens
`/var/lib/go-choir/auth/auth.db` read-only. It does not install packages, write
to the auth DB, call Nix GC, delete files, restart services, or mutate VM state.

`scripts/node-b-storage-report --host node-b --top 12` completed in 6.719
seconds after consolidating duplicate `du` scans and filtering Nix profile lock
noise. The report proves the
protected accounts as:

- `yusefnathanson@me.com`:
  `5bd6de97-3b58-408c-bf89-c42c81b083de`, 31.61 GiB, refusal;
- `a@b.com`: `0e5c45ab-44de-49cd-b07d-e58973b21ad5`, 479.55 MiB,
  refusal;
- `b@c.com`: `5885aafc-eb85-4255-9818-d521020bdce2`, 619.13 MiB,
  refusal.

The auth DB currently has 381 users by email domain: 338 `example.com`, 38
`example.test`, and one each for `b.com`, `c.com`, `choir-ip.com`, `gmail.com`,
and `me.com`. The report classifies `example.com` and `example.test` as
fake-domain dry-run candidates only: 54 VM ownerships, 40.81 GiB. It leaves 13
synthetic non-UUID ownerships, 7.62 GiB, as owner-review candidates. It refuses
64.96 GiB of UUID primary ownerships that have no auth user record; those need
a separate retention rule for missing-auth-user VM ownerships.

## Baseline Cleanup Plan Evidence

The report now includes a `Baseline Cleanup Plan (Report-Only)` section. It is
still not an active cleanup command. It states:

- `active_delete_authorized: false`;
- required delete gate: typed candidate, owner-reviewed policy,
  rollback/refusal record, and staged behavior proof;
- protected-account gate: auth DB mapping proves protected identities remain
  refusal classes.

The current baseline plan separates class matches from policy-eligible
candidates. Policy eligibility currently means the current vmctl
ephemeral-primary guard plus owner review: interactive kind, `desktop_id:
primary`, `published: true`, terminal lifecycle state (`hibernated`, `stopped`,
or `failed`), and at least 24 hours since last active.

- fake-domain VMs: 54 ownerships / 40.81 GiB total, of which 37 ownerships /
  24.37 GiB are policy-eligible;
- synthetic non-UUID VMs: 13 ownerships / 7.62 GiB total, currently all match
  the vmctl ephemeral-primary policy gate but still require owner approval to
  treat the synthetic owner IDs as disposable;
- manual snapshots: 4 files / 23.82 GiB, classified from filename-only
  metadata as two pre-prune rollback copies, one corrupt-disk quarantine, and
  one platform migration artifact; all remain preserve/refusal rows until typed
  snapshot metadata, recovery settlement, rollback proof, and owner approval
  exist;
- missing-auth-user UUID VMs: 64.96 GiB, refusal until a policy exists for
  historical owner IDs without auth rows;
- Nix roots: 9 known roots / 9.35 GiB direct target allocation, review-only
  under a current/rollback/stale root budget; no Nix GC authorized.

The top-candidate rows include action, gate, age, owner identity, state, and
allocated bytes. Rows that fail any guard explicitly say whether they are not
current vmctl retention kind, not primary desktop, unpublished, non-terminal,
or under the 24-hour age gate.

## Dry-Run Retention Test Evidence

The vmctl retention-plan test now exercises the report's next policy shape
without changing Node B configuration or enabling live deletion:

- disposable email domains include both `example.com` and `example.test`;
- disposable synthetic owners may be represented with explicit prefixes such
  as `diagnostic-` and `sourcemaxx-proof-`;
- protected owner/test accounts `yusefnathanson@me.com`, `a@b.com`, and
  `b@c.com` remain excluded;
- active fake-domain primaries remain excluded;
- unpublished non-primary desktops remain excluded;
- old orphan state dirs remain eligible under the existing orphan policy.

Evidence: `nix develop -c go test ./internal/vmctl` passed locally after this
test-only change. This is not staging proof and does not authorize active
cleanup.

## Shadow Dry-Run Retention Evidence

vmctl now has a separate retention shadow plan for observation-only policy
proof. The active retention policy remains the only policy consumed by
`PruneRetention`, `reclaim`, and idle sweeps. The shadow policy is exposed by:

- `GET /internal/vmctl/retention-shadow-plan`;
- `VMCTL_RETENTION_SHADOW_*` environment variables;
- Node B Nix configuration that sets the shadow policy to dry-run for
  `example.com`, `example.test`, `diagnostic-*`, and `sourcemaxx-proof-*`.

The setter force-normalizes any non-off shadow mode to `dry-run`. The new
regression test proves the shadow plan can see `example.test` and synthetic
prefix candidates while active pruning deletes only the currently active
`example.com` candidate and leaves the shadow/protected VMs untouched.

Evidence:

- `nix develop -c go test ./internal/vmctl -run
  'TestOwnershipRegistry_RetentionPlanTargetsOnlyOrphansAndEphemeralPrimaries|TestOwnershipRegistry_RetentionShadowPlanDoesNotExpandActivePrune|TestOwnershipRegistry_PruneRetentionRemovesEphemeralPrimaryOwnership|TestOwnershipRegistry_RetentionPlanPrefersLargeSafeCandidates|TestEndpointHelpers|TestHandlerEndpointsExist'`
  passed;
- `nix develop -c go test ./internal/vmctl` passed;
- current Node B report proof completed in 7.072 seconds before deployment and
  shows `retention_shadow_plan: null`, which means the new endpoint/config has
  not yet been deployed on Node B.

This is an orange runtime/config change in the worktree. It is not settled
until committed, pushed, CI passes, Node B deploy identity is verified, and the
deployed report proves the shadow plan is `dry-run` while active projected
delete remains bounded and reviewed.

## Manual Snapshot Classifier Evidence

The report now gives manual `data.img.*` snapshots a typed report-only class,
inferred purpose, TTL policy, gate, age, owner ID, allocation, and
`metadata_status: inferred_from_filename_only`.

The 2026-06-14 Node B report found:

- 4 manual snapshots / 23.82 GiB;
- 2 `pre_prune_rollback_review` snapshots for the owner VM;
- 1 `corrupt_disk_quarantine_review` snapshot for the owner VM;
- 1 `platform_migration_snapshot_review` artifact;
- 4 snapshots still missing typed metadata.

No snapshot deletion is authorized. The delete gate is typed snapshot metadata,
recovery settlement, replacement/rollback proof, and owner approval.

## Typed Snapshot Metadata Path Evidence

Manual `data.img.*` snapshots now have a typed sidecar path:
`scripts/node-b-data-img-snapshot` can create or annotate a snapshot with
`data.img.*.metadata.json` using schema
`choir.manual-data-img-snapshot.v1`. The helper defaults to dry-run, never
deletes VM state or snapshots, and refuses to copy a live VM disk while
`firecracker.pid` is active unless the operator passes `--allow-running`.

`scripts/node-b-storage-report` now consumes valid sidecars, excludes
`.metadata.json` files from the snapshot list, and reports
`metadata_present_count`, `metadata_missing_count`, and
`metadata_invalid_count`. Invalid sidecars are preserve/refusal evidence, and
the verifier fails closed when invalid metadata is present.

Evidence:

- local temp-fixture `scripts/node-b-data-img-snapshot --apply` created a
  sparse `data.img.pre-prune-*` copy and valid metadata sidecar;
- a Node B `/tmp` fixture proved the report recognizes a valid typed sidecar as
  `typed_sidecar_valid` with one present metadata row, zero missing rows, and
  zero invalid rows;
- live Node B proof still reports 4 manual snapshots, 4 missing metadata rows,
  0 typed sidecars, 0 invalid sidecars, and no snapshot deletion authorization.

This creates the typed creation/annotation path, but it does not authorize
deletion and does not yet enforce snapshot cleanup.

## Structured Report Evidence

`scripts/node-b-storage-report` now supports `--format json` as an alternate
read-only output over the same classifier. This lets CI or a staging verifier
assert refusal/protected-account evidence without scraping Markdown.

The 2026-06-14 Node B JSON report completed in 6.755 seconds and proved:

- `yusefnathanson@me.com`,
  `5bd6de97-3b58-408c-bf89-c42c81b083de`, `refuse_delete`;
- `a@b.com`, `0e5c45ab-44de-49cd-b07d-e58973b21ad5`, `refuse_delete`;
- `b@c.com`, `5885aafc-eb85-4255-9818-d521020bdce2`, `refuse_delete`;
- `manual_recovery_snapshots.policy_status` is
  `report-only; no snapshot deletion authorized`;
- `nix_roots.policy_status` is
  `report-only; no root deletion or nix-store GC authorized`;
- `baseline_cleanup_plan.active_delete_authorized` is `0`.

This is a report-verifier oracle, not a cleanup authorization or staging deploy
proof.

`scripts/node-b-storage-verify-report` now verifies the JSON report contract.
It fails closed unless:

- report mode is read-only;
- `active_delete_authorized` is `0`;
- the three protected emails are present and `refuse_delete`;
- snapshot deletion and Nix root deletion/GC remain unauthorized;
- protected identity refusal bytes are nonzero;
- manual snapshots remain metadata-missing rows;
- current vmctl active retention projects `0` bytes.

Evidence: the verifier passed against `/tmp/node-b-storage-report.json`;
negative smoke reports with `baseline_cleanup_plan.active_delete_authorized =
1` and `policies.active_delete_authorized = 1` failed as expected. This still
is not a staging/deploy proof until the verifier runs in the deployed/reporting
environment.

`scripts/node-b-storage-proof` now packages the report and verifier into a
single read-only proof command. It writes Markdown and JSON artifacts to an
operator-selected output directory, runs both report formats in parallel, and
then verifies the JSON protected-identity/no-delete contract.

Evidence: `scripts/node-b-storage-proof --host node-b --top 10 --out-dir
/tmp/node-b-storage-proof-20260614T154633Z` completed in 7.739 seconds. It
produced:

- `/tmp/node-b-storage-proof-20260614T154633Z/node-b-storage-report.md`;
- `/tmp/node-b-storage-proof-20260614T154633Z/node-b-storage-report.json`.

The verifier passed on that JSON report. This is still report-only proof: it
does not delete VM state, delete snapshots, run Nix GC, restart services, or
mutate VMs.

## Nix Root Budget Evidence

The report now classifies known deploy/build roots under a report-only budget:
preserve the current deploy root, current system generation, one explicit
rollback generation, latest proven guest image root, and only explicitly
required specialized worker image roots. Deletion requires deployed identity,
a rollback manifest, and an owner-reviewed stale-root decision.

The 2026-06-14 Node B report found:

- 9 known roots;
- 9.35 GiB direct target allocation; this is target-path allocation, not full
  closure size;
- one broken current deploy pointer at `/opt/go-choir/result`;
- four service build roots, two guest-image candidate roots, one browser-worker
  guest-image root, and one host-system build root.

This satisfies the report-only budget shape. It does not authorize root
deletion or `nix-store --gc`; active pruning still needs deploy identity,
rollback refs, and staging/deploy evidence.

## Nix GC Plan Evidence

The report now includes a structured `nix_roots.gc_plan` section. It remains
report-only and explicitly forbids generation deletion, root deletion, and
`nix store gc`. The plan records:

- current root free space versus the active daily sweep floor and a proposed
  100 GiB target headroom;
- the current NixOS generation and one rollback generation to preserve;
- older system generations that would be review-only prune candidates;
- known deploy roots and their preserve/repair/review action;
- delete and GC gates requiring deployed identity, rollback manifest,
  owner-reviewed stale generation/root decision, and a dry-run report.

Live Node B evidence from
`/tmp/node-b-storage-proof-20260614T163005Z/node-b-storage-report.json`:

- `policy_status`: report-only; no generation deletion, root deletion, or
  `nix-store` GC authorized;
- current available space: 78,564,284 KiB;
- active sweep emergency floor: 41,943,040 KiB;
- proposed target headroom: 104,857,600 KiB;
- pressure: `below_target_headroom`;
- current generation: 494;
- rollback generation: 493;
- stale generation review candidates: 8;
- broken roots: 1.

This gives the future active GC change a typed budget and rollback oracle. It
does not itself change the Node B timer, delete generations, remove roots, run
GC, or restart services.

## Suggested Goal String

```text
/goal Use Parallax on docs/mission-node-b-storage-retention-v0.md. Treat it as the source program for preventing Node B storage recurrence after the 2026-06-14 VM recovery incident. Current status is open_handoff: read-only classifier, JSON verifier, single-command proof runner, deployed vmctl shadow dry-run retention reporting, typed manual snapshot sidecar reporting, a dry-run-by-default snapshot metadata helper, and a report-only Nix GC/current+rollback plan exist. CI/deploy passed for 32e754208e2a332165f3bce13ecbdf2ab17c5d97, and deployed Node B proof completed in 7.160 seconds with active retention deleting 0 bytes while shadow dry-run reports 46 candidates / 30.89 GiB. Commit ce52c115cd03bc07bcf40a3a95a2f31ccd8a7cc8 proved storage-tooling edits now run CI without host/guest deploy. The latest report-only proof `/tmp/node-b-storage-proof-20260614T163005Z` completed in 7.533 seconds and shows Nix pressure `below_target_headroom`, current generation 494, rollback generation 493, 8 stale generation review candidates, and 1 broken root, with no generation/root deletion or GC authorized. Mutation class is red for any live cleanup and orange for future vmctl/Nix retention behavior; do not run live deletion, Nix GC, manual snapshot cleanup, service restarts, or active prune expansion without explicit approval. Preserve real user yusefnathanson@me.com and protected test accounts a@b.com and b@c.com. Do not delete VM state, data.img snapshots, Nix roots, or guest images without a typed retention candidate, rollback/refusal reason, owner-visible report, and explicit approval. First next move: implement either snapshot cleanup gates over typed sidecars or convert the report-only Nix plan into an owner-approved active timer policy. Ledger: docs/mission-node-b-storage-retention-v0.ledger.md. Settlement requires staging-proven retention/reporting, CI/deploy evidence for behavior changes, a reviewed baseline cleanup plan, and explicit evidence that owner/test real accounts remain protected.
```

## Parallax State

status: open_handoff

mission conjecture: if Node B has an identity-aware storage classifier,
owner-visible retention report, and bounded cleanup policy for VM state, manual
snapshots, fake users, and Nix roots under Choir invariants, then storage
incidents become predictable maintenance instead of recovery-time surprises.

deeper goal (G): keep Choir's persistent-computer substrate reliable enough for
staging/self-development while preserving real user data and rollback evidence.

witness/spec (A/S): a read-only storage report first, then a bounded retention
implementation that classifies and optionally prunes: disposable fake-user
computers, orphan VM dirs, stale manual recovery snapshots, guest caches,
oversized protected primary disks, stale Nix build roots, and old store
closures.

invariants / qualities / domain ramp (I/Q/D): Choir Doctrine is apex; real
user computers and owner-used test accounts are protected; deletion requires a
typed candidate with reason, age, owner identity class, size, rollback/refusal
record, and dry-run report; no manual deletion of unknown `data.img` artifacts;
no weakening docs-only CI filters; start read-only on Node B, then dry-run
staging report, then active bounded cleanup only after explicit approval.

variant (ranking function) V: active VM retention implementation `1` +
snapshot cleanup enforcement over typed sidecars `1` + active Nix GC/rollback
enforcement `1` = `3`; last ΔV: report-only Nix GC/current+rollback plan `0`
because it creates the oracle but does not change the active timer or run GC.

budget: initial planning budget one Codex turn; execution budget extended
through report/shadow implementation and deployed proof. Solvency: prevention
work remains solvent; live cleanup is not authorized.

authority / bounds: documentation and read-only investigation are authorized.
The worktree now includes an orange vmctl/Nix config change that is
observation-only by construction. Live deletion, Nix GC, service restarts, and
any expansion of active prune/reclaim deletion require explicit approval and
staging evidence.

mutation class / protected surfaces: green for this paradoc; future orange for
retention code and CI/deploy changes; future red for live Node B VM/Nix cleanup.
Protected surfaces: vmctl ownerships, persistent user computers, `data.img`,
manual recovery snapshots, Nix GC roots/store, guest images, deployment
rollback refs.

evidence packet: Node B disk inventory; `scripts/node-b-storage-report` output;
`scripts/node-b-storage-proof` artifacts; read-only auth DB identity mapping;
vmctl health/list/retention-plan/retention-shadow-plan; Nix root inventory and
`nix_roots.gc_plan`; incident docs; code references in `internal/vmctl`,
`nix/node-b.nix`, and `.github/workflows/ci.yml`; deploy evidence from GitHub
Actions run `27504321847`; future active cleanup proof must include dry-run
report, focused tests, CI/deploy identity when behavior changes, and
post-cleanup health.

heresy delta: discovered `1` policy mismatch; introduced `0`; repaired `1`
for staging-proven report-only prevention visibility; active cleanup prevention
remains open.

position / live conjectures / open edges: Current evidence supports a policy
mismatch, not a single leak. The report has an identity-backed baseline cleanup
plan with independent review: `example.com` and `example.test` VMs are dry-run
candidates only when they match the vmctl ephemeral-primary guard plus owner
review, synthetic non-UUID VMs are owner-review candidates under the same guard,
protected owner/test accounts are explicit refusals, missing-auth-user UUID VMs
are refusals, manual snapshots carry filename-inferred classes and TTL gates
while remaining metadata-missing preserve/refusal rows, and Nix roots carry a
report-only current/rollback/stale budget with one broken `/opt/go-choir/result`
pointer identified. The report emits Markdown and JSON; the verifier turns the
protected-account/no-delete assertions into a reusable fail-closed contract.
`scripts/node-b-storage-proof` runs the two report formats plus verification in
one command. Commit `32e754208e2a332165f3bce13ecbdf2ab17c5d97` passed GitHub
Actions run `27504321847`, deployed to Node B, and the deployed proof completed
in 7.160 seconds. Deployed Node B evidence now shows active retention mode
`active` with projected delete count/bytes `0`, while retention shadow mode is
`dry-run` and reports 46 disposable candidates / 30.89 GiB for `example.com`,
`example.test`, `diagnostic-*`, and `sourcemaxx-proof-*`. Deploy slowness was
explained by Nix rebuilding selected guest images: the deploy spent 257 seconds
in Nix build, including ordinary and Playwright guest image roots. A fresh
classifier probe shows the storage report scripts fall through as unknown
deployed paths, so future report-tooling changes would repeat that conservative
image-build request unless the classifier explicitly treats them as
operator/report tooling. Commit `ce52c115cd03bc07bcf40a3a95a2f31ccd8a7cc8`
proved that fix in CI: run `27504868005` passed, Detect Staging Deploy Impact
passed, Build Frontend was skipped, and `Deploy to Staging (Node B)` was
skipped. Existing disk GC still preserves Nix cache above the 40 GiB free-space
floor. The report now carries a structured report-only Nix GC plan: latest live
proof completed in 7.533 seconds, identified pressure `below_target_headroom`,
current generation 494, rollback generation 493, eight stale generation review
candidates, one broken root, and explicit no-GC/no-delete gates. Open edges:
missing-auth-user policy, active fake-user cleanup approval/enforcement, active
Nix GC/rollback enforcement, snapshot cleanup enforcement after typed metadata
exists, whether live `data.img` sparsification/discard should be part of
hibernate/recovery, and how to expose reports to operators.

next move: implement either snapshot cleanup gates over typed sidecars or
convert the report-only Nix plan into an owner-approved active timer policy. A
separate owner-approved active fake-user cleanup can follow from the deployed
shadow proof, but only after retaining the protected account gate and
rollback/refusal record.

ledger file: docs/mission-node-b-storage-retention-v0.ledger.md

version / lineage: v0 created after the 2026-06-14 owner-primary recovery and
iptables cleanup. Related docs:
`docs/incident-vm-bootstrap-stale-route-2026-06-09.md` and
`docs/deferred-reliability-migrations-2026-05-14.md`.

learning state: retain storage taxonomy and prevention plan here; promote to
operating contract only after a proven report/retention implementation exists.

settlement: settled only when read-only reporting, dry-run classification,
protected-account policy, Nix root budget, and bounded cleanup behavior are
implemented/proven on staging, with reviewed baseline cleanup and no loss of
protected owner/test computers.
