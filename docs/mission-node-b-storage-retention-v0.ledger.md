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
