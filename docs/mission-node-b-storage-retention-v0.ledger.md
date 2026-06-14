# Ledger: Node B Storage Retention And VM State Prevention v0

## 2026-06-14 — initial paradoc compile

- Claim: Node B storage recurrence is a policy mismatch across `vm-state`,
  manual recovery snapshots, fake-user ownerships, and `/nix/store`, not a
  single leak.
- Move: compile a new paradoc from read-only investigation and owner guidance.
- Expected ΔV: 0; the move creates the mission control surface, not the
  prevention witness.
- Actual ΔV: 0.
- Receipt: `docs/mission-node-b-storage-retention-v0.md`.
- Open edge: next pass must construct a read-only classifier/report before any
  live deletion or retention behavior change.

## 2026-06-14 — read-only storage report witness

- Claim: a report-only classifier can separate immediate refusal classes from
  owner-review cleanup opportunities without mutating Node B.
- Move: added `scripts/node-b-storage-report`, a read-only SSH/local Markdown
  report over vm-state allocation, `data.img` snapshots, synthetic user IDs,
  vmctl projected reclaim, Nix roots, and refusal reasons.
- Evidence: `scripts/node-b-storage-report --host node-b --top 12` completed in
  9.155 seconds and wrote `/tmp/node-b-storage-report.md`.
- Findings: vmctl active retention would reclaim 0 bytes; manual snapshots
  total 23.82 GiB; synthetic non-UUID user VMs total 7.62 GiB review-only; UUID
  primary VMs total 107.50 GiB and are refused until auth email mapping exists;
  known protected user ID state totals 31.61 GiB; platform state totals
  16.77 GiB; `/opt/go-choir/result` points at a missing store path.
- Learning: `nix path-info -Sh` is too eager for this report because it may try
  to realize/fetch an invalid root; the script now uses filesystem allocation
  (`du`) for known root targets instead.
- Expected ΔV: -5 for replacing missing report/unclassified storage with typed
  report/refusal classes.
- Actual ΔV: -5.
- Receipt: `scripts/node-b-storage-report`; report sample
  `/tmp/node-b-storage-report.md`.
- Open edge: email-to-user-ID mapping is still missing, so broad fake-user
  pruning remains unsafe. Next pass should expose a read-only auth identity
  oracle and then produce dry-run candidates.

## 2026-06-14 — auth identity mapping and runtime consolidation

- Claim: the report can prove protected owner/test accounts by email without
  changing runtime services or copying the auth DB off host.
- Move: extended `scripts/node-b-storage-report` to locate an existing
  `/nix/store/*sqlite*/bin/sqlite3`, open `/var/lib/go-choir/auth/auth.db`
  read-only, resolve protected emails to vmctl user IDs, classify
  `example.com` and `example.test` VMs as fake-domain dry-run candidates, and
  consolidate duplicate `du` scans.
- Evidence: `scripts/node-b-storage-report --host node-b --top 12` completed in
  7.136 seconds and wrote `/tmp/node-b-storage-report.md`.
- Findings: `yusefnathanson@me.com` maps to
  `5bd6de97-3b58-408c-bf89-c42c81b083de` with 31.61 GiB protected;
  `a@b.com` maps to `0e5c45ab-44de-49cd-b07d-e58973b21ad5` with
  479.55 MiB protected; `b@c.com` maps to
  `5885aafc-eb85-4255-9818-d521020bdce2` with 619.13 MiB protected.
  Fake-domain VMs are now 54 ownerships / 40.81 GiB dry-run candidates; 13
  synthetic non-UUID ownerships remain owner-review candidates; 64.96 GiB of
  UUID primary ownerships have no auth user record and remain refused.
- Expected ΔV: -3 for resolving the protected-account oracle, classifying fake
  email domains, and restoring under-10-second report runtime.
- Actual ΔV: -3.
- Receipt: `scripts/node-b-storage-report`; report sample
  `/tmp/node-b-storage-report.md`.
- Open edge: next pass must turn fake-domain, synthetic, missing-auth-user,
  snapshot, and Nix-root report classes into reviewed retention policy
  candidates without active deletion.

## 2026-06-14 — independent report safety review

- Claim: the read-only report remains safe for default and custom invocations.
- Move: delegated a fresh-context review of untracked report/paradoc artifacts,
  then repaired the returned finding.
- Evidence: reviewer found no default-path mutation calls (`rm`, GC, restart,
  repair, mount, truncate, POST) and confirmed the default report path reads
  only; reviewer flagged one P2 issue where custom CLI/env values could break
  the remote `sudo env ... bash` quoting.
- Repair: changed the SSH wrapper to shell-quote every remote env value with
  Bash `%q`; smoke-tested `--protected-email "quote'probe@example.com"` over
  SSH successfully.
- Follow-up repair: filtered and timed out `nix-env --list-generations` so Nix
  profile lock messages do not pollute report output.
- Evidence after repair: `scripts/node-b-storage-report --host node-b --top 12`
  completed in 6.719 seconds with protected IDs proven and fake-domain dry-run
  reclaim still 40.81 GiB; `scripts/doccheck` passed.
- Expected ΔV: 0; this was verifier/consolidation work rather than a new
  retention-policy class.
- Actual ΔV: 0.
- Receipt: `scripts/node-b-storage-report`; report sample
  `/tmp/node-b-storage-report.md`.
- Open edge: active cleanup remains unauthorized until report classes become a
  reviewed policy with rollback/refusal records.

## 2026-06-14 — baseline cleanup plan in report

- Claim: the report can carry a reviewed cleanup plan shape, not just storage
  inventory, while staying read-only.
- Move: extended `scripts/node-b-storage-report` with `Baseline Cleanup Plan
  (Report-Only)`, top VM cleanup candidates, top VM refusals, per-row action,
  gate, age, identity, and allocated bytes.
- Evidence: `scripts/node-b-storage-report --host node-b --top 10` completed in
  7.102 seconds and wrote `/tmp/node-b-storage-report.md`.
- Findings: fake-domain VMs total 54 ownerships / 40.81 GiB, with
  37 ownerships / 24.37 GiB policy-eligible under terminal-state plus 24-hour
  gates; synthetic non-UUID VMs total 13 ownerships / 7.62 GiB and are all
  policy-eligible under those gates;
  manual snapshots remain 4 files / 23.82 GiB review-only; missing-auth-user
  UUID VM ownerships remain 64.96 GiB refusal; Nix roots remain review-only.
- Expected ΔV: -2 for turning broad report classes into typed cleanup
  candidate/refusal rows with gates.
- Actual ΔV: -2.
- Receipt: `scripts/node-b-storage-report`; report sample
  `/tmp/node-b-storage-report.md`.
- Open edge: this baseline plan still needs independent review, then a
  dry-run/staging retention implementation before any active deletion.

## 2026-06-14 — baseline plan review repair

- Claim: the report's cleanup candidates should not accidentally shape a
  future policy that deletes active/running computers.
- Move: sent the latest baseline plan to an independent reviewer; repaired the
  returned findings.
- Evidence: reviewer found no mutation path, but reported P1/P2/P3 issues:
  candidate rows needed lifecycle-state gates, "age-eligible" overclaimed
  eligibility, and synthetic counts used distinct owner IDs instead of
  ownership count.
- Repair: added terminal lifecycle gating (`hibernated`, `stopped`, or
  `failed`) before candidate rows are policy-eligible; renamed output to
  `policy_eligible_24h_terminal`; added `synthetic_ownerships`; updated docs
  from age eligibility to policy eligibility.
- Evidence after repair: `scripts/node-b-storage-report --host node-b --top 10`
  completed in 6.763 seconds; embedded Perl compiled; `scripts/doccheck`
  passed.
- Expected ΔV: -1 for independent baseline review and repair.
- Actual ΔV: -1.
- Receipt: `scripts/node-b-storage-report`; `docs/mission-node-b-storage-retention-v0.md`.
- Open edge: implement dry-run/staging retention behavior with tests before
  any active deletion.

## 2026-06-14 — vmctl guard alignment repair

- Claim: "policy-eligible" report rows must match the current vmctl
  ephemeral-primary deletion guard, not only age and lifecycle state.
- Move: re-reviewed the repaired baseline plan; resolved the remaining
  ambiguity by aligning report eligibility to interactive kind, `desktop_id:
  primary`, `published: true`, terminal state, and 24-hour age.
- Evidence: re-review found P3 fixed but reported P1/P2 still partially open
  because primary/published guard semantics were absent from the report and
  docs. `scripts/node-b-storage-report --host node-b --top 10` completed in
  6.941 seconds after repair; embedded Perl compiled.
- Independent confirmation: a follow-up re-review confirmed P1/P2 fixed for
  the requested surface and `bash -n scripts/node-b-storage-report` passed.
- Findings after repair: fake-domain VMs remain 54 ownerships / 40.81 GiB
  total, with 37 ownerships / 24.37 GiB matching the vmctl
  ephemeral-primary policy gate; synthetic non-UUID VMs remain 13 ownerships /
  7.62 GiB and currently match the same gate, but still require owner approval
  to treat the synthetic owner IDs as disposable.
- Expected ΔV: 0; this is a correctness repair to the report oracle, not a new
  retention implementation.
- Actual ΔV: 0.
- Receipt: `scripts/node-b-storage-report`; report sample
  `/tmp/node-b-storage-report.md`.
- Open edge: implement dry-run/staging retention behavior with tests before
  any active deletion.

## 2026-06-14 — dry-run retention policy test oracle

- Claim: the widened fake-domain/synthetic cleanup policy can be represented
  in vmctl dry-run planning without selecting protected owner/test accounts or
  non-disposable computers.
- Move: extended the existing retention-plan test to cover `example.test`,
  synthetic user ID prefixes (`diagnostic-`, `sourcemaxx-proof-`), protected
  owner/test emails (`yusefnathanson@me.com`, `a@b.com`, `b@c.com`), active
  fake-domain primaries, unpublished non-primary desktops, and old orphan state
  dirs.
- Evidence: `nix develop -c go test ./internal/vmctl -run
  'TestOwnershipRegistry_RetentionPlanTargetsOnlyOrphansAndEphemeralPrimaries|TestOwnershipRegistry_PruneRetentionRemovesEphemeralPrimaryOwnership|TestOwnershipRegistry_RetentionPlanPrefersLargeSafeCandidates'`
  passed; `nix develop -c go test ./internal/vmctl` passed.
- Independent review: reviewer found an initial overclaim because only
  `diagnostic-*` was exercised despite naming `sourcemaxx-proof-*`; repaired
  by adding a `sourcemaxx-proof-85751dc5` ownership/candidate assertion.
  Follow-up review confirmed both synthetic prefixes are now exercised and the
  docs/ledger claim is supported.
- Expected ΔV: 0; this adds a local oracle for the next implementation move
  but does not itself enable dry-run staging config, active policy, Nix budget,
  snapshot TTL, or deploy proof.
- Actual ΔV: 0.
- Receipt: `internal/vmctl/vmctl_test.go`.
- Open edge: expose the widened policy in dry-run/staging only and prove Node B
  active deletion remains unchanged until explicitly authorized.

## 2026-06-14 — Nix root budget report classifier

- Claim: known deploy/build Nix roots can be classified under a report-only
  current/rollback/stale budget without deleting roots or running GC.
- Move: extended `scripts/node-b-storage-report` to classify known roots by
  policy class, target existence, direct target allocation, root mtime, action,
  and gate. The encoded budget preserves current deploy root, current system
  generation, one explicit rollback generation, latest proven guest image root,
  and only explicitly required specialized worker image roots.
- Evidence: `scripts/node-b-storage-report --host node-b --top 10` completed
  in 7.122 seconds and wrote `/tmp/node-b-storage-report.md`.
- Findings: 9 known roots, 9.35 GiB direct target allocation, one broken
  current deploy root pointer at `/opt/go-choir/result`, four service build
  roots, two guest-image candidate roots, one browser-worker guest-image root,
  and one host-system build root.
- Independent review: reviewer found no blocking issues; confirmed the
  classifier is read-only, direct target allocation is not overclaimed as
  closure size/reclaimable space, and deletion gates require deployed identity,
  rollback manifest, and owner-reviewed stale-root decision. Follow-up wording
  repair renamed mixed report table headers from `projected_reclaim` to
  `size_or_projected_reclaim`.
- Expected ΔV: -1 for constructing the report-only Nix root budget.
- Actual ΔV: -1 for the report oracle; active Nix GC/rollback enforcement
  remains open.
- Receipt: `scripts/node-b-storage-report`; report sample
  `/tmp/node-b-storage-report.md`.
- Open edge: prove the root budget against deployed identity and rollback
  manifest before any root deletion or Nix GC.

## 2026-06-14 — manual snapshot report classifier

- Claim: manual `data.img.*` snapshots can be classified into typed
  preserve/refusal rows from current evidence without deleting snapshots or
  pretending filename inference is durable metadata.
- Move: extended `scripts/node-b-storage-report` with a read-only snapshot
  classifier that records class, inferred purpose, TTL policy, deletion gate,
  age, owner ID, allocation, and `metadata_status:
  inferred_from_filename_only`.
- Evidence: `scripts/node-b-storage-report --host node-b --top 10` completed
  in 6.750 seconds and wrote `/tmp/node-b-storage-report.md`.
- Findings: 4 manual snapshots / 23.82 GiB; 2
  `pre_prune_rollback_review` owner-VM rollback copies; 1
  `corrupt_disk_quarantine_review` owner-VM quarantine copy; 1
  `platform_migration_snapshot_review` artifact; all 4 still lack typed
  metadata.
- Independent review: reviewer found no blocking issues; confirmed the
  classifier is read-only with no deletion, GC, service restart, mount, fsck,
  truncate, or mutation path; confirmed report/docs are honest that filename
  inference is not typed metadata and active metadata-at-creation/cleanup
  enforcement remains open.
- Expected ΔV: 0; this adds the report oracle for snapshot retention, but
  active metadata-at-creation and cleanup enforcement remain open.
- Actual ΔV: 0.
- Receipt: `scripts/node-b-storage-report`; report sample
  `/tmp/node-b-storage-report.md`.
- Open edge: attach typed snapshot metadata at creation time and prove cleanup
  gates before any snapshot deletion.

## 2026-06-14 — structured report oracle

- Claim: the storage classifier needs a machine-readable output so future CI
  or staging verification can assert protected-account and no-delete gates
  without scraping Markdown.
- Move: added `--format json` to `scripts/node-b-storage-report`, reusing the
  same read-only classifier and preserving Markdown as the default output.
- Evidence: `scripts/node-b-storage-report --host node-b --format json --top
  10` completed in 6.755 seconds and wrote `/tmp/node-b-storage-report.json`;
  `jq` parsed the report and verified protected refusal rows for
  `yusefnathanson@me.com`, `a@b.com`, and `b@c.com`; it also verified snapshot
  deletion is unauthorized, Nix root deletion/GC is unauthorized, and
  `baseline_cleanup_plan.active_delete_authorized == 0`.
- Independent review: reviewer found no blocking issues; confirmed JSON mode
  reuses the same read-only classifier, has no deletion/GC/restart/mount/fsck/
  truncate/mutation path, propagates correctly over local and SSH modes, and is
  documented as a structured report oracle rather than deploy proof or cleanup
  authorization.
- Expected ΔV: 0; this creates an automation oracle for staging/CI proof but
  does not itself deploy reporting, enable dry-run staging config, or authorize
  cleanup.
- Actual ΔV: 0.
- Receipt: `scripts/node-b-storage-report`; report sample
  `/tmp/node-b-storage-report.json`.
- Open edge: wire this JSON oracle into staging/CI proof after the report is
  landed and deployed.

## 2026-06-14 — structured report verifier

- Claim: the JSON report should have a reusable fail-closed verifier before it
  is wired into CI or staging proof.
- Move: added `scripts/node-b-storage-verify-report`, a read-only shell/jq
  verifier for `scripts/node-b-storage-report --format json` output.
- Evidence: `scripts/node-b-storage-verify-report
  /tmp/node-b-storage-report.json` passed. Negative smoke reports with
  `baseline_cleanup_plan.active_delete_authorized = 1` and
  `policies.active_delete_authorized = 1` failed as expected.
- Verified gates: report mode is read-only; active deletion is unauthorized;
  `yusefnathanson@me.com`, `a@b.com`, and `b@c.com` are present and
  `refuse_delete`; snapshot deletion is unauthorized; Nix root deletion/GC is
  unauthorized; protected identity refusal bytes are nonzero; manual snapshots
  remain metadata-missing rows; current vmctl active retention projects
  0 bytes.
- Independent review: reviewer found no blocking issues; confirmed the
  verifier only runs `jq -e` against a provided JSON file, has no deletion/GC/
  restart/mount/fsck/truncate/mutation path, fails closed on missing or unsafe
  gate evidence, and is documented as a reusable verifier oracle rather than
  deployment proof or cleanup authorization.
- Expected ΔV: 0; this creates a reusable verifier for staging/CI proof but
  does not itself deploy reporting, enable dry-run staging config, or authorize
  cleanup.
- Actual ΔV: 0.
- Receipt: `scripts/node-b-storage-verify-report`.
- Open edge: run this verifier in the deployed/reporting environment as part of
  staging or CI evidence.

## 2026-06-14 — single-command report proof runner

- Claim: operators and CI/manual probes need one report-only command that emits
  durable Markdown/JSON artifacts and verifies the no-delete/protected-identity
  contract without mutating Node B.
- Move: added `scripts/node-b-storage-proof`, which runs the Markdown and JSON
  reports in parallel, then runs `scripts/node-b-storage-verify-report` against
  the JSON output.
- Evidence: `scripts/node-b-storage-proof --host node-b --top 10 --out-dir
  /tmp/node-b-storage-proof-20260614T154633Z` completed in 7.739 seconds.
  It wrote `/tmp/node-b-storage-proof-20260614T154633Z/node-b-storage-report.md`
  and `/tmp/node-b-storage-proof-20260614T154633Z/node-b-storage-report.json`;
  the verifier passed on the JSON report.
- Finding: the first sequential wrapper pass took 13.736 seconds because it ran
  Markdown and JSON reports serially; running the two read-only report formats
  in parallel restored the proof command under the 10-second operating target.
- Expected ΔV: 0; this makes the report/verifier usable as a proof command but
  does not deploy reporting, enable dry-run staging config, authorize cleanup,
  enforce snapshot metadata, or change Nix retention.
- Actual ΔV: 0.
- Receipt: `scripts/node-b-storage-proof`.
- Open edge: run this proof runner in the deployed/reporting environment or CI
  once the mission moves from read-only local proof to staged reporting.

## 2026-06-14 — shadow dry-run retention plan

- Claim: the widened fake-domain/synthetic retention policy can be exposed for
  staging observation without expanding the active deletion policy.
- Move: added a separate vmctl shadow retention config and
  `GET /internal/vmctl/retention-shadow-plan`; wired Node B config to set the
  shadow plan to dry-run for `example.com`, `example.test`,
  `diagnostic-*`, and `sourcemaxx-proof-*`; extended the storage report and
  verifier to surface the shadow plan when deployed.
- Safety property: `PruneRetention`, `reclaim`, and idle sweeps still consume
  only the active retention policy. The shadow setter force-normalizes any
  non-off mode to `dry-run`.
- Evidence: focused vmctl retention/endpoint tests passed; full
  `nix develop -c go test ./internal/vmctl` passed; script syntax and embedded
  Perl compile passed; `scripts/node-b-storage-proof --host node-b --top 10
  --out-dir /tmp/node-b-storage-proof-20260614T155420Z` completed in
  7.072 seconds and the verifier passed.
- Live Node B observation: before deployment the proof report shows
  `retention_mode: active`, `retention_projected_delete_bytes: 0`,
  `retention_shadow_mode: unavailable`, and `retention_shadow_plan: null`.
- Expected ΔV: 0; this constructs the dry-run/staging observation surface but
  does not land, deploy, or prove it on staging.
- Actual ΔV: 0.
- Receipt: `internal/vmctl/retention_prune.go`,
  `internal/vmctl/handlers.go`, `cmd/vmctl/main.go`, `nix/node-b.nix`,
  `scripts/node-b-storage-report`, `scripts/node-b-storage-verify-report`.
- Open edge: land the change, monitor CI/deploy identity, then rerun the
  storage proof against Node B and require `retention_shadow_mode: dry-run`
  while active retention and protected-account refusals remain bounded.

## 2026-06-14 — deployed shadow dry-run proof

- Claim: the landed shadow retention plan should prove the broadened
  fake-domain/synthetic policy on Node B without expanding active deletion.
- Move: pushed commit `32e754208e2a332165f3bce13ecbdf2ab17c5d97`, monitored
  GitHub Actions run `27504321847`, confirmed the Node B staging deploy job
  succeeded, and ran the report-only storage proof against Node B after deploy.
- CI/deploy evidence: run `27504321847` completed successfully; deploy job
  `81292841840` fetched and deployed
  `32e754208e2a332165f3bce13ecbdf2ab17c5d97`; deploy health output reported
  proxy/platformd `deployed_commit` as the same SHA; staging deploy completed
  at `2026-06-14T16:07:29Z`.
- Runtime evidence: `scripts/node-b-storage-proof --host node-b --top 10
  --out-dir /tmp/node-b-storage-proof-20260614T160853Z` completed in 7.160
  seconds and the JSON verifier passed.
- Safety evidence: deployed report shows active `retention_mode: active`,
  active projected delete count/bytes `0`, `retention_shadow_mode: dry-run`,
  shadow projected delete count `46`, and shadow projected delete bytes
  `30.89 GiB`. The shadow policy includes `example.com`, `example.test`,
  `diagnostic-*`, and `sourcemaxx-proof-*`. Protected identities
  `yusefnathanson@me.com`, `a@b.com`, and `b@c.com` remain refusal rows from
  read-only auth DB mapping.
- CI duration finding: the deploy step spent 257 seconds in selected Nix builds,
  including ordinary and Playwright guest image builds, explaining the unusually
  slow deploy. This will recur when guest-image or NixOS closure inputs change
  or when current deploy roots do not preserve the desired cache; it is not
  caused by docs-only commits.
- Expected ΔV: 1, for staging/deploy proof of the orange report-only behavior
  change.
- Actual ΔV: 1.
- Open edge: active fake-user cleanup remains unauthorized; snapshot deletion
  remains blocked on typed snapshot metadata, recovery settlement, rollback
  proof, and owner approval; Nix GC remains blocked on an explicit
  current/rollback root policy and budget.

## 2026-06-14 — deploy-impact classifier problem checkpoint

- Claim: the unusually slow deploy can repeat for storage-reporting work if the
  deploy-impact classifier keeps treating Node B storage scripts as unknown
  deployed paths.
- Move: probed `.github/scripts/deploy-impact-classify` with
  `scripts/node-b-storage-report`, `scripts/node-b-storage-proof`, and
  `scripts/node-b-storage-verify-report`.
- Evidence: the classifier currently returns `deploy_needed=true`,
  `deploy_host_os=true`, `deploy_ordinary_guest=true`, and
  `deploy_playwright_guest=true` for those storage scripts, with the explanation
  `unknown deployed path: conservative host + both guest images`.
- Finding: this is separate from docs-only path filters; docs-only commit
  `25c4242bbbad89fe150a782f50b3e27a7501fe0c` triggered Docs Truth Check only.
  Storage-tooling script edits are not docs-only and currently request image
  builds unless the classifier explicitly marks them as operator/report tooling.
- Expected ΔV: 0; this is the required problem documentation before changing CI
  deploy-impact behavior.
- Actual ΔV: 0.
- Open edge: update the deploy-impact classifier and its test so storage
  report-tooling changes run CI without requesting Node B host/guest deploys.

## 2026-06-14 — operator tooling classifier and typed snapshot sidecars

- Claim: storage-reporting iterations should not request guest image builds, and
  manual snapshots need typed metadata before any future cleanup can be
  reviewed.
- Move: updated `.github/scripts/deploy-impact-classify` and its test to treat
  `scripts/node-b-storage-*` and `scripts/node-b-data-img-snapshot` as Node B
  operator/report tooling rather than deployed host or guest closures. Added
  `scripts/node-b-data-img-snapshot`, a dry-run-by-default helper that creates
  or annotates manual `data.img.*` snapshots with a
  `choir.manual-data-img-snapshot.v1` sidecar. Extended
  `scripts/node-b-storage-report` and `scripts/node-b-storage-verify-report` to
  consume typed sidecars, count present/missing/invalid metadata, and fail
  closed on invalid sidecars while keeping deletion unauthorized.
- Safety evidence: no live VM state, manual snapshot, Nix root, guest image, or
  service was deleted or pruned. The helper refuses live `data.img` copies while
  `firecracker.pid` is active unless `--allow-running` is explicitly supplied.
- Verification: `bash -n` passed for the touched scripts; deploy-impact
  classifier regression passed; changed-path classifier output reports
  `deploy_needed=false`, `deploy_ordinary_guest=false`, and
  `deploy_playwright_guest=false`; doccheck passed; live
  `scripts/node-b-storage-proof --host node-b --top 10 --out-dir
  /tmp/node-b-storage-proof-20260614T161820Z` completed in 7.217 seconds and
  the verifier passed.
- Metadata fixture evidence: a local temp fixture proved
  `scripts/node-b-data-img-snapshot --apply` writes a valid sidecar; a Node B
  `/tmp` fixture proved `scripts/node-b-storage-report --format json` reports
  one `typed_sidecar_valid` snapshot with `metadata_present_count: 1`,
  `metadata_missing_count: 0`, and `metadata_invalid_count: 0`.
- Live Node B evidence: current report still shows 4 manual snapshots, 4 missing
  metadata rows, 0 typed sidecars, 0 invalid sidecars, and no snapshot deletion
  authorization.
- Expected ΔV: 0; this creates the typed metadata path and stops report-tooling
  edits from requesting image deploys, but cleanup enforcement and Nix GC
  budgeting remain open.
- Actual ΔV: 0.
- Open edge: push/monitor CI to prove the classifier suppresses staging deploy
  for this script-only change; then implement snapshot cleanup gates or an
  explicit Nix current/rollback root policy.

## 2026-06-14 — operator tooling CI no-deploy proof

- Claim: after classifying Node B storage scripts as operator/report tooling,
  storage-tooling changes should run CI without requesting host/guest image
  builds or Node B staging deploy.
- Move: pushed `ce52c115cd03bc07bcf40a3a95a2f31ccd8a7cc8` and monitored GitHub
  Actions.
- Evidence: CI run `27504868005` completed successfully. `Detect Staging Deploy
  Impact` passed in 4 seconds, `Build Frontend` was skipped, and
  `Deploy to Staging (Node B)` was skipped. Docs Truth Check run `27504868013`
  passed. FlakeHub publish run `27504868006` passed.
- Interpretation: this commit proves the specific recurrence mode that caused
  report-tooling edits to request ordinary and Playwright guest image builds is
  repaired for `scripts/node-b-storage-*` and
  `scripts/node-b-data-img-snapshot`.
- Expected ΔV: 0; this prevents tooling iteration from causing image builds but
  does not implement active cleanup, Nix GC budgeting, or snapshot cleanup.
- Actual ΔV: 0.
- Open edge: implement either snapshot cleanup gates over typed sidecars or an
  explicit Nix current/rollback root policy with budgeted GC.

## 2026-06-14 — report-only Nix GC current/rollback plan

- Claim: before making Nix GC more frequent or active, the storage report needs
  a structured current/rollback budget that distinguishes protected roots from
  stale generation/root candidates and keeps GC unauthorized until reviewed.
- Move: extended `scripts/node-b-storage-report` with `nix_roots.gc_plan` and
  added verifier assertions that the plan remains report-only. The plan records
  root free space, the active 40 GiB emergency floor, a proposed 100 GiB target
  headroom, current generation, rollback generation, stale generation review
  candidates, root actions, and delete/GC gates.
- Live evidence: `scripts/node-b-storage-proof --host node-b --top 10 --out-dir
  /tmp/node-b-storage-proof-20260614T163005Z` completed in 7.533 seconds and
  the verifier passed. The JSON report shows `pressure:
  below_target_headroom`, `current_generation: 494`, `rollback_generation:
  493`, `stale_generation_count: 8`, `broken_root_count: 1`,
  `current_available_kib: 78564284`, `active_sweep_min_free_kib: 41943040`,
  and `proposed_target_free_kib: 104857600`.
- Safety evidence: no Nix generation deletion, root deletion, `nix store gc`,
  service restart, VM mutation, or snapshot deletion was run. The changed paths
  classify as operator/report tooling with `deploy_needed=false`.
- Expected ΔV: 0; this creates the Nix GC oracle but does not change the active
  timer or enforce a new GC policy.
- Actual ΔV: 0.
- Open edge: convert this report-only plan into an owner-approved active timer
  policy, or implement snapshot cleanup gates over typed sidecars first.

## 2026-06-14 — active Nix timer target implementation

- Claim: Node B can prune `/nix/store` more frequently without deleting Nix
  roots or weakening rollback evidence by making the daily timer run
  `nix store gc` below a 100 GiB target headroom while keeping the existing
  40 GiB emergency floor and `+8` system-generation retention policy.
- Move: changed `nix/node-b.nix` so `go-choir-disk-retention-sweep` has
  `GO_CHOIR_DISK_GC_TARGET_FREE_KIB=104857600` and runs routine
  `nix store gc` when free space is below that target after vmctl reclaim,
  journal vacuum, and existing generation pruning. Updated the read-only report
  and verifier to distinguish timer-authorized GC from ad hoc GC that the
  report still refuses to authorize.
- Evidence: `bash -n` passed for touched scripts; deploy-impact classifier
  regression passed; `nix-instantiate --parse nix/node-b.nix` passed; classifier
  output for `nix/node-b.nix` is host OS only with `deploy_ordinary_guest=false`
  and `deploy_playwright_guest=false`. Live read-only proof
  `scripts/node-b-storage-proof --host node-b --top 10 --out-dir
  /tmp/node-b-storage-proof-20260614T163631Z` completed in 7.193 seconds and
  verifier passed. The JSON report shows pressure `below_target_headroom`,
  timer action `run_nix_store_gc_from_timer`, current generation 494, rollback
  generation 493, 8 stale generation review candidates, 1 broken root, and
  active vmctl retention projected delete bytes `0`.
- Safety evidence: no live `nix store gc`, generation deletion, root deletion,
  service restart, VM mutation, snapshot deletion, or active prune expansion was
  run during this pass.
- Expected ΔV: 1 after CI/deploy proves the host-OS-only timer behavior on
  Node B.
- Actual ΔV: 0 until the commit is pushed, CI passes, deploy identity is
  verified, and deployed Node B proof observes the target.
- Open edge: push/monitor CI and deploy, then verify deployed Node B reports
  the 100 GiB target and did not request ordinary/Playwright guest image builds.

## 2026-06-14 — deployed Nix timer target proof

- Claim: the active Nix timer target implementation is proven on Node B when CI
  passes, deploy identity matches the commit, guest image builds stay skipped,
  systemd exposes the 100 GiB target, and the read-only storage proof observes
  the target and protected-account gates.
- Move: pushed `c04e9649d28d2e163d7c0eb9d0d3e9e506af649e`, monitored GitHub
  Actions, checked staging health/build identity, inspected the Node B timer
  service environment, and ran a post-deploy storage proof.
- Evidence: CI run `27505328627` completed successfully; `Deploy to Staging
  (Node B)` completed in 29 seconds. Deploy-impact output was host OS only:
  `deploy_ordinary_guest=false`, `deploy_playwright_guest=false`,
  `deploy_active_vm_refresh=false`. Deploy logs show the host NixOS closure
  build took 9 seconds, ordinary and Playwright guest image builds were skipped,
  and guest image installs were skipped. Staging `/health` reports deployed
  commit `c04e9649d28d2e163d7c0eb9d0d3e9e506af649e`. Node B systemd shows
  `GO_CHOIR_DISK_GC_TARGET_FREE_KIB=104857600` on
  `go-choir-disk-gc.service`. Post-deploy
  `scripts/node-b-storage-proof --host node-b --top 10 --out-dir
  /tmp/node-b-storage-proof-20260614T164148Z` completed in 6.873 seconds and
  verifier passed.
- Storage proof details: Nix pressure `below_target_headroom`, timer action
  `run_nix_store_gc_from_timer`, current generation 495, rollback generation
  494, 9 stale generation review candidates, 1 broken root, active vmctl
  retention projected delete bytes `0`, and protected accounts
  `yusefnathanson@me.com`, `a@b.com`, and `b@c.com` remain `refuse_delete`.
- Safety evidence: no manual live `nix store gc`, generation deletion, root
  deletion, service restart, VM mutation, snapshot deletion, or active prune
  expansion was run by this agent. The behavior change is the deployed timer
  target; actual GC will occur on the scheduled timer when still below target.
- Expected ΔV: 1.
- Actual ΔV: 1; active Nix GC/rollback enforcement is deployed and proven.
- Open edge: active VM fake-user cleanup and snapshot cleanup enforcement remain
  unsettled.

## 2026-06-14 — snapshot cleanup gates

- Claim: manual `data.img.*` snapshots stop being ambiguous storage risk when a
  reusable gate can classify each snapshot as preserve/refusal or review-delete
  candidate from typed metadata, age, owner approval, recovery settlement, and
  rollback/replacement proof, while keeping deletion unauthorized by default.
- Move: added `scripts/node-b-storage-snapshot-gates`, a read-only planner over
  `scripts/node-b-storage-report --format json`, and wired it into
  `scripts/node-b-storage-proof` so every proof emits
  `node-b-snapshot-cleanup-gates.{md,json}`. Updated the report next-proof text
  so future passes no longer route back to the already deployed Nix timer work.
- Fixture evidence: a temp JSON report with one typed manual snapshot and one
  filename-inferred pre-prune snapshot proves default mode refuses both rows;
  with modeled owner approval, recovery settlement, and rollback proof, only
  the typed manual snapshot becomes a `review_delete_candidate`; in both cases
  `active_delete_authorized` remains `false`.
- Live evidence: `scripts/node-b-storage-proof --host node-b --top 10 --out-dir
  /tmp/node-b-storage-proof-20260614T165403Z` completed in 7.200 seconds and
  verifier passed. The snapshot gate plan reports mode `report-only; no
  snapshot deletion or VM mutation`, active deletion `false`, 4 manual
  snapshots / 23.82 GiB, 0 typed metadata sidecars, 4 missing sidecars, 0
  invalid sidecars, 0 review-delete candidates, and 4 preserve/refusal rows.
- Safety evidence: no live metadata write, snapshot deletion, VM mutation,
  service restart, Nix GC, or active prune expansion was run. Existing live
  manual snapshots remain preserve/refusal rows because typed sidecars and
  approval/recovery/rollback evidence are absent.
- Expected ΔV: 1.
- Actual ΔV: 1; snapshot cleanup enforcement is now a reusable report-only gate
  and live snapshots are explicitly refused.
- Open edge: active VM fake-user cleanup remains unsettled; missing-auth-user
  retention policy remains undefined.

## 2026-06-14 — VM cleanup gates

- Claim: fake-domain and synthetic-owner VM cleanup can become reviewable
  without authorizing live deletion when the report exports full candidate and
  refusal rows and a separate gate proves protected accounts remain refusals.
- Move: added `scripts/node-b-storage-vm-gates`, extended
  `scripts/node-b-storage-report --format json` with full cleanup
  candidate/refusal arrays, and wired `scripts/node-b-storage-proof` to emit
  `node-b-vm-cleanup-gates.{md,json}` plus jq assertions for report-only mode
  and protected-account refusal.
- Live evidence: `scripts/node-b-storage-proof --host node-b --top 10 --out-dir
  /tmp/node-b-storage-proof-20260614T170339Z` completed in 7.657 seconds and
  verifier checks passed. The VM gate reports mode `report-only; no VM state
  deletion or ownership mutation`, active deletion `false`, protected account
  gate passed, 67 cleanup candidates / 48.44 GiB, 0 default review-delete
  candidates, 67 preserve/review-pending rows / 48.44 GiB, 3 protected identity
  refusals / 32.69 GiB, and 134 missing-auth-user refusals / 64.96 GiB.
- Protected account evidence: `yusefnathanson@me.com`,
  `5bd6de97-3b58-408c-bf89-c42c81b083de`, 31.61 GiB; `a@b.com`,
  `0e5c45ab-44de-49cd-b07d-e58973b21ad5`, 479.55 MiB; `b@c.com`,
  `5885aafc-eb85-4255-9818-d521020bdce2`, 619.13 MiB; all have action
  `refuse_delete`.
- Modeled-approval evidence: running the gate with fake-domain approval,
  synthetic-owner approval, a modeled rollback/refusal record, and modeled
  staging proof produced 50 review candidates / 31.99 GiB while keeping active
  deletion `false`; 17 rows remained preserve/review-pending because they did
  not satisfy the current vmctl ephemeral-primary age/lifecycle gate.
- Safety evidence: no live VM state deletion, `ownerships.json` mutation,
  service restart, snapshot deletion, ad hoc Nix GC, or active prune expansion
  was run. This is an operator-visible reviewed baseline plan, not cleanup.
- Expected ΔV: 0 against active cleanup authorization/enforcement; it closes the
  reviewed-baseline evidence gap but leaves live cleanup unauthorized.
- Actual ΔV: 0.
- Open edge: obtain explicit approval and convert reviewed fake/synthetic VM
  candidates into active cleanup, or define the missing-auth-user UUID VM
  retention policy and keep current refusals.

## 2026-06-14 — missing-auth UUID policy gate

- Claim: missing-auth UUID VM rows should not remain an undefined 64.96 GiB
  refusal bucket; they should have an explicit policy that preserves by default
  and names the proof required before review.
- Move: extended `scripts/node-b-storage-report` so
  `missing_auth_user_record_refusal` rows carry the current vmctl
  ephemeral-primary lifecycle gate. Extended `scripts/node-b-storage-vm-gates`
  with `--missing-auth-approved` and `--missing-auth-lineage-proof`, and added
  proof-runner assertions that `missing_auth_policy.active_delete_authorized`
  remains `false`.
- Live evidence: `scripts/node-b-storage-proof --host node-b --top 10 --out-dir
  /tmp/node-b-storage-proof-20260614T171254Z` completed in 7.674 seconds and
  verifier checks passed. Default mode reports protected account gate passed,
  active deletion `false`, 134 missing-auth UUID VM refusals / 64.96 GiB, and 0
  missing-auth review-delete candidates.
- Modeled-approval evidence: running
  `scripts/node-b-storage-vm-gates
  /tmp/node-b-storage-proof-20260614T171254Z/node-b-storage-report.json
  --format json --missing-auth-approved --missing-auth-lineage-proof
  modeled-lineage --rollback-or-refusal-record modeled-review-record
  --staging-proof modeled-staging-proof` produced 134 missing-auth review
  candidates / 64.96 GiB while keeping active deletion `false`.
- Safety evidence: no live VM state deletion, `ownerships.json` mutation,
  service restart, snapshot deletion, Nix root deletion, ad hoc Nix GC, or
  active prune expansion was run. No authorization was requested or assumed.
- Expected ΔV: 1 for closing the missing-auth undefined-policy edge while
  leaving active cleanup unauthorized.
- Actual ΔV: 1.
- Open edge: active cleanup still requires explicit owner authorization and a
  behavior-changing vmctl path with CI/deploy evidence, or the current
  report-only refusal gates remain the correct state.

## 2026-06-14 — owner authorizes Codex-domain active cleanup

- Claim: the reviewed fake/synthetic VM cleanup class can move from shadow
  observation to active retention when the owner identifies `example.com` and
  `example.test` as Codex-created account domains and authorizes real cleanup.
- Owner clarification: after asking for recurring stale-state cleanup rather
  than a one-time purge, the owner clarified that the intended deletion target
  is accounts made by Codex for agentic testing, and then explicitly stated
  that `example.com` and `example.test` are Codex domains.
- Move: promoted the deployed shadow vmctl policy into the active Node B
  retention policy by adding `example.test` plus synthetic owner prefixes
  `diagnostic-` and `sourcemaxx-proof-` to active retention. The policy remains
  bounded by vmctl's existing reclaimability guard: interactive primary
  desktop, published ownership, terminal VM state, older than 24 hours, and
  per-sweep max delete/byte caps.
- Safety boundary: protected identities remain out of the cleanup class:
  `yusefnathanson@me.com`, `a@b.com`, and `b@c.com`. Missing-auth UUID owners
  remain refusals until lineage/tombstone proof exists. Manual recovery
  snapshots and Nix roots remain outside this VM-retention authorization.
- Mutation class: orange for durable Node B retention policy; red for the
  follow-on live reclaim call after deploy.
- Protected surfaces: vmctl retention policy, `/var/lib/go-choir/vm-state`,
  `ownerships.json`, and Node B disk retention sweep.
- Evidence class: CI/deploy identity for the config change, deployed
  `/internal/vmctl/retention-plan` before cleanup, product-path
  `/internal/vmctl/reclaim` result, post-cleanup proof, and protected-account
  refusal evidence.
- Rollback path: revert the Node B retention config to active `example.com`
  only or disable active retention with `VMCTL_RETENTION_PRUNE_MODE=off`, deploy
  that host config, and preserve any remaining ambiguous rows as report-only
  refusals. Deleted stale Codex VM state is not a protected rollback primitive.
- Heresy delta: discovered none; introduced risk that broad Codex-domain
  cleanup could delete a still-useful Codex proof computer if it is terminal and
  older than the TTL; repaired the prior recurrence risk where shadow-only
  disposable VM state could keep growing indefinitely.

## 2026-06-14 — active cleanup deployed and exercised

- Claim: promoting the shadow Codex-domain policy to active retention should
  reclaim stale agentic test VM state without rebuilding guest images or
  touching protected owner/test computers.
- Commit/deploy evidence: commit
  `6c1448035afce1006d593b2469c9c7990d4f9650` pushed to `origin/main`.
  GitHub Actions run `27506420444` passed. Deploy impact passed, Build
  Frontend was skipped, Docs Truth Check passed, and the Node B deploy
  completed successfully.
- Deploy timing evidence: the deploy built only the host NixOS closure in 8s,
  skipped ordinary and Playwright guest image builds and installs, completed
  `nixos-rebuild switch` in 12s, then spent 90s in service activation. Total
  deploy time was 122s; the prior image-cache issue did not repeat.
- Live cleanup evidence: vmctl restart loaded 208 persisted ownerships, applied
  active retention with `example.com,example.test` and
  `diagnostic-,sourcemaxx-proof-`, then logged `retention prune deleted 46 VM
  state directorie(s), reclaimed 31632.2 MiB`. A second vmctl restart loaded
  162 persisted ownerships.
- Product-path reclaim evidence: explicit
  `POST /internal/vmctl/reclaim` saved at
  `/tmp/node-b-retention-reclaim-20260614T172726Z.json` returned status `ok`,
  `retention.deleted: 0`, and both before/after active retention plans showed
  zero candidates because restart cleanup had already settled the authorized
  class.
- Habitual cleanup evidence: `systemctl start go-choir-disk-gc.service`
  completed successfully. The service ran vmctl reclaim, journal vacuum,
  deleted stale system profile versions 488, 487, 486, and 485, and preserved
  the warm Nix cache because free space was above the 100 GiB target. The timer
  remains scheduled daily with the next run on 2026-06-15 at 00:45:36 UTC.
- Post-cleanup proof: `scripts/node-b-storage-proof --host node-b --top 20
  --out-dir /tmp/node-b-storage-proof-post-cleanup-20260614T172740Z` completed
  in 7.596s and verifier passed. Active and shadow retention both report
  `projected_delete_count: 0`, `projected_delete_bytes: 0`, 162 ownerships, and
  162 state dirs. Disk is 363G used / 111G available / 77% used.
- Protected account evidence: post-cleanup report still marks
  `yusefnathanson@me.com`, `a@b.com`, and `b@c.com` present with action
  `refuse_delete`. Allocated protected VM-state bytes are 33.94 GiB,
  502.85 MiB, and 649.20 MiB respectively.
- Remaining refusals: missing-auth UUID owners remain refused by policy;
  manual recovery snapshots remain report-only; Nix roots are managed only by
  the daily generation/journal/conditional-GC policy, not ad hoc root deletion.
- Actual ΔV: 3; the recurring leak path for Codex-domain agentic test VM state
  is now active, bounded, and exercised, and disk pressure fell from 85% to 77%.
- Residual risk: startup retention produced transient auth DB warning logs
  before deletion, although post-cleanup protected-account evidence is intact.
  A future hardening move should make auth-DB unavailability fail closed for
  domain-derived candidates while still allowing explicit synthetic owner-id
  prefixes.
