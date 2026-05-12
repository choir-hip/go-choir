# MissionGradient: Choir Self-Development Factory

Date: 2026-05-11

This mission is for a long-running Codex `/goal` run. It should use the
MissionGradient method: optimize one real artifact under invariants, not a
sequence of fake stages.

## Short Goal Prompt

```text
Use MissionGradient. Complete docs/mission-gradient-choir-in-choir-2026-05-11.md in order by optimizing the deployed Choir self-development factory under its invariants and verification criteria. Preserve topology, avoid forbidden shortcuts, and stop/escalate on invariant-level surprises.
```

## Real Artifact

The artifact is the deployed Choir self-development factory:

```text
prompt bar on https://draft.choir-ip.com
  -> conductor routes user intent
  -> VText creates and revises the working document
  -> VText requests researchers and super/cosupers
  -> background microVMs do mutable development work
  -> verified candidate commits cross a shipper boundary
  -> GitHub branches/PRs/CI prove the work
  -> staging deploy shows the improved system
```

The target is not a local coding patch. The target is a product path where
Choir can safely use Choir to improve Choir.

## Mission Priorities

1. Make the active VM and background VM lifecycle durable enough that work is
   not lost across deploys, restarts, commits, or browser reloads.
2. Make the background VM -> shipper -> GitHub path explicit, narrow, auditable,
   and usable from the product workflow.
3. Prove that VText, researchers, super, and cosupers can coordinate through the
   deployed system without manual orchestration or public internal APIs.
4. Use the working self-development loop to build user-visible content apps:
   EPUB reader, PDF reader, image viewer, video/YouTube player, audio player,
   and podcast/RSS player.
5. Improve the desktop/VText UX enough that the proof is usable on staging:
   rendered editable Markdown, sane mobile viewport, compact VText chrome,
   growing prompt bar, working Trace, working Settings, and quiet account UI.
6. Lay the forward-compatible path for publishing VTexts from user desktops to a
   platform publishing desktop backed by a platform Dolt appliance.

## Invariants

These define identity. Do not silently change them to make a check pass.

### Product Entry Invariant

All user-facing acceptance starts from the deployed product path:

```text
browser prompt bar or app UI on https://draft.choir-ip.com
```

Do not prove the main workflow by manually calling internal endpoints, manually
spawning runs, manually editing database rows, or seeding successful artifacts.

### Public API Boundary Invariant

The browser-public API must stay radically small and product-semantic. Browser
clients may submit prompt intent, use app APIs, read authorized Trace
projections, and operate safe product settings. Browser clients must not create
agent runs, choose agent roles, mutate runtime prompts, or call internal
orchestration surfaces such as `/api/agent/*`.

### Actor Authority Invariant

Conductor routes user intent and delegates to appagents. It does not orchestrate
workers.

VText owns canonical document revisions. VText may request researchers and
super/cosupers as needed. Researchers produce findings, evidence, questions, and
proposals. Workers do not patch VText. VText edits the document.

Mutable code/file/system work happens through super/cosupers in background VMs,
not in the active desktop VM. The active desktop VM remains the user's stable
workspace and preview surface.

### VM Persistence Invariant

An active user's desktop VM must not become a disposable fresh VM after deploy,
service restart, commit, or browser reload. VM identity, ownership, disk state,
file root, and runtime state must be durable or reattached from durable metadata.

Background VMs are forks of the active VM or a known base snapshot. Their work
must be exportable, reviewable, and either merged/promoted or discarded with a
rollback path.

### GitHub Shipping Invariant

Background worker VMs must not hold broad GitHub credentials.

Code produced by Choir-in-Choir counts as verified only after crossing this
boundary:

```text
background VM local branch/commit
  -> git bundle or patchset
  -> verification manifest
  -> platform shipper import in clean checkout
  -> shipper reruns required checks
  -> narrow GitHub App or shipper credential pushes branch
  -> GitHub Actions CI
  -> staging deploy/QA
```

Manual patch copying, hidden VM-only state, direct pushes from arbitrary worker
VMs, and direct pushes to `main` are forbidden shortcuts.

Each shipped branch/PR must name:

- Choir run id
- Trace trajectory id
- VM id and snapshot/base id
- base SHA and head SHA
- verification manifest path
- tests run and results
- screenshots/videos/traces
- known residual risks

Branch naming:

```text
agent/<run-id>/<slug>
```

### Trace/Verifier Invariant

Trace is the audit surface for product behavior. It should be auth-gated and
substantially unredacted for the owner/operator. The verifier should evaluate
durable event log causality, not caller-supplied claims.

The event log must show:

- prompt-bar submission
- server-created conductor run
- conductor -> VText delegation with initial document abstract
- VText revisions and user edits
- VText requests to researcher/super/cosuper
- researcher search/extraction calls and provider endpoint outcomes
- worker updates delivered to VText
- background VM fork, command execution, file edits, tests, artifact creation
- shipper import/push/PR/CI/deploy events
- failures, retries, and rate limits

### Search and Extraction Invariant

Search/research must be real when the task depends on current knowledge. The
system should use multiple connectors when available, record provider-level and
endpoint-level success/rate-limit/error/latency stats, and expose those stats in
Trace. Stubs are engineering gradients only. They are not human proof.

Extraction should prefer local, composable rungs where possible:

```text
direct HTTP with browser-ish headers
-> Defuddle/readability extraction
-> SearXNG alternate/canonical discovery
-> lightweight browser acquisition when needed
-> paid/heavy fallback only when justified
```

Obscura should be evaluated as the preferred lightweight browser backend if it
can satisfy the needed capture/DOM/screenshot/session behavior with lower memory
than Playwright/Chromium.

### UX Invariant

The desktop is a user-facing operating surface, not an internal dashboard. VText
must remain a clean writing/research/publishing surface. Trace is for audit and
debugging. Settings is for safe product settings, not prompt/runtime mutation by
default.

VText Markdown should be rendered and editable in one surface. Avoid a permanent
read/edit mode split. Mobile should feel like a desktop scaled to the viewport:
no page zoom, compact one-row chrome, no controls covering text, prompt bar
expands with multiline input, keyboard slides the desktop instead of zooming it.

### Publishing Forward-Compatibility Invariant

Publishing is not required to be fully complete tonight, but implementation must
not block it. Published VTexts should eventually move from an individual user's
desktop state into a platform publishing desktop backed by platform Dolt. User
desktop VText state and platform publication state are distinct trust domains.

## Value Criterion

Optimize:

```text
J(s, lambda) =
  Q(self-development proof, lambda)
  - alpha * B(bypasses/proxy wins)
  - beta  * R(regressions)
  - gamma * U(unobserved state)
  - delta * G(Goodharted verifier)
```

Better means:

- more of the deployed product path works end to end
- fewer internal/public API leaks exist
- less state is hidden in disposable VMs or manual local edits
- more behavior is visible in Trace/event logs
- more checks are replayable from durable artifacts
- more code crosses the shipper/GitHub/CI boundary safely
- more UX proof is observable on staging

Worse means:

- a test passes by bypassing the product path
- a worker edits live desktop state directly
- a fake provider or fixture is treated as proof
- code exists locally but cannot be produced through Choir
- VM state disappears across restarts
- Trace cannot explain what happened
- GitHub credentials leak into worker VMs

## Homotopy Parameters

Increase these dimensions continuously while preserving the invariants:

- One prompt -> many prompts
- One VText -> many VTexts
- One researcher -> parallel researchers
- One background VM -> multiple background VMs
- One small code change -> medium feature implementation
- One content app -> full content app family
- Fixture content -> live URL/RSS/YouTube/upload content
- Local VM proof -> staging proof -> CI/deploy proof
- Manual operator review -> machine-verifiable event-log proof

Do not create fake islands. A lower-resolution proof must use the same product
entrypoint, authority boundaries, event semantics, VM lifecycle, and shipper path
as the higher-resolution proof.

## Dense Feedback Channels

Use feedback that localizes error:

- `git status`, `git diff --check`, focused unit tests, full Go tests
- frontend build and targeted Playwright tests
- deployed Playwright against `https://draft.choir-ip.com`
- Trace trajectory/event-log assertions
- search provider stats and endpoint-level errors
- vmctl ownership/snapshot/state inspection
- shipper import logs and verification manifests
- GitHub branch/PR/CI status
- staging screenshots/video for UX and content apps
- explicit residual-risk notes in the mission doc

Do not accept "it works" without a trace, command output, artifact, screenshot,
CI result, or durable event.

## Forbidden Shortcuts

- Do not manually call `/api/agent/*` or equivalent internal orchestration APIs
  to prove product behavior.
- Do not make browser-public endpoints for worker spawning, prompt mutation, raw
  runtime internals, or privileged VM/GitHub actions.
- Do not edit the active desktop VM for code changes that should happen in a
  background VM.
- Do not treat local Codex edits as Choir-in-Choir proof.
- Do not treat stubs, canned artifacts, or fake search as human proof.
- Do not hide failures with UI copy such as "completed" when the artifact is not
  actually verified.
- Do not put broad GitHub credentials in worker/background VMs.
- Do not push directly to `main` from a worker.
- Do not create content apps as isolated one-off UI islands. They must share the
  upload/link/content substrate and app registry/routing surface.
- Do not solve mobile UX by disabling the desktop model or allowing viewport zoom
  side effects.

## Rollback Policy

Before risky changes:

- capture current `main` SHA
- capture staging health/build identity
- capture active VM ownership/state metadata
- avoid destructive vmctl cleanup unless explicitly justified
- use branches for generated work
- keep shipper imports replayable
- make background VM promotion reversible

Any deploy or VM lifecycle change must have a rollback note:

- what gets reverted
- what state persists
- what state is disposable
- how to recover the active user desktop

## Unknown Learning Without Drift

Classify surprises:

- Tactical learning: apply directly and record in this doc if useful.
- Target-level learning: update this mission document or create a proposed
  follow-up branch.
- Invariant-level learning: stop and escalate before changing the invariant.

Examples:

- Tactical: a SearXNG endpoint is rate-limited, so use another connector and
  record provider stats.
- Target-level: Obscura cannot record video, but can capture DOM/screenshots, so
  shift its role to backend browser acquisition and keep Playwright for QA.
- Invariant-level: background VMs cannot currently export commits with durable
  provenance. Stop treating content-app work as proven until the shipper path is
  implemented.

## Checklist

Each item is tied to an invariant and must be checked only when verified.

### 0. Baseline Orientation

- [x] Record current local SHA, branch, dirty status, and `origin/main`.
- [x] Record current staging `/health` build identity.
- [x] Confirm deployed app loads without stale JS or removed route calls.
- [x] Record current active desktop VM identity, owner mapping, disk/state paths,
      and whether the VM survives reload/restart.
- [x] Open Trace and Settings on staging and record current failures, if any.

Baseline evidence, 2026-05-12 UTC:

- Local git state: branch `main`, local HEAD
  `270531d8a52ee767736b4a6e11df8a23cb3cbdd7`, `origin/main`
  `270531d8a52ee767736b4a6e11df8a23cb3cbdd7`. Dirty state is this new
  mission document only.
- Staging health: `https://draft.choir-ip.com/health` reports proxy and
  sandbox commit `270531d8a52ee767736b4a6e11df8a23cb3cbdd7`, deployed at
  `2026-05-10T14:38:03Z`, with vmctl routing enabled through
  `http://127.0.0.1:8083`.
- Cache/build identity: root shell returns `Cache-Control: no-store`; current
  JS `/assets/index-QmQg_Bf3.js` and CSS `/assets/index-DPORnfyY.css` return
  `Cache-Control: public, max-age=31536000, immutable`.
- Deployed shell contract:
  `cd frontend && npx playwright test tests/deployed-origin-auth-shell.spec.js --project=chromium`
  passed 12/12. This covers deployed build identity, no stale `/src/` scripts,
  no protected requests for signed-out visitors, no direct service-port calls,
  and no stale route calls such as `/api/agent/topology`, `/api/prompts`, or
  `/api/events` for the signed-out shell.
- Deployed prompt-bar/VText/Trace contract:
  `cd frontend && npx playwright test tests/gateway-e2e-deployed.spec.js --project=chromium`
  passed 1/1. Evidence file:
  `frontend/test-results/gateway-e2e-deployed/test-results.json`. It registered
  `gateway-e2e-1778553536336-a0qgz8@example.com`, submitted through
  `/api/prompt-bar`, received submission
  `f7f32a19-dc06-4371-93fc-df4808a881aa`, opened VText with two revisions, and
  verified a Trace projection with two agents.
- Deployed live-search VText contract:
  `cd frontend && GO_CHOIR_RUN_DEPLOYED_LIVE_SEARCH=1 CHOIR_DEPLOYED_BASE_URL=https://draft.choir-ip.com npx playwright test tests/vtext-deployed-live-search.spec.js --project=chromium`
  passed 1/1 in about 2.2 minutes. This proves current 2026 work can use a
  real `web_search` event and produce a grounded VText revision without
  forbidden browser calls to `/api/agent/spawn`, `/api/agent/topology`,
  `/api/prompts`, or `/api/events`.
- Trace/Settings probe: a deployed-only Playwright probe registered
  `mission-baseline-1778553909298-9m15ek@example.com` with user id
  `f54662a4-bf6a-4151-a765-a2d94b186d7c`, opened Trace and Settings, and
  recorded no forbidden requests, no bad API responses, and no console errors.
  Evidence:
  `frontend/test-results/mission-baseline-2026-05-11/trace-settings-baseline.json`,
  `frontend/test-results/mission-baseline-2026-05-11/trace.png`, and
  `frontend/test-results/mission-baseline-2026-05-11/settings.png`.
- Live vmctl state for that baseline user: vmctl lookup on Node B maps
  user `f54662a4-bf6a-4151-a765-a2d94b186d7c`, desktop `primary`, to
  VM `vm-65bc61e59f53b832d22307b4a85ac881`, kind `interactive`, published
  `true`, sandbox URL `http://172.9.0.2:8085`, state `active`, epoch `1`,
  created `2026-05-12T02:45:10.396Z`, last active
  `2026-05-12T02:45:16.793Z`.
- Live VM disk paths for `vm-65bc61e59f53b832d22307b4a85ac881`:
  `/var/lib/go-choir/vm-state/vm-65bc61e59f53b832d22307b4a85ac881/data.img`
  is 64 MiB; the same directory contains `epoch`, `fc-config.json`, and
  `persist/gateway-token`.
- Important baseline caveat: browser reload survival is covered by the deployed
  tests, but vmctl service-restart survival is not verified because restarting
  vmctl is destructive on staging. Source/deploy inspection indicates a real blocker:
  `cmd/vmctl/main.go` constructs an in-memory `OwnershipRegistry`,
  `.github/workflows/ci.yml` restarts `go-choir-vmctl` on deploy, and
  `internal/vmmanager/manager.go` stops all managed VMs in `Manager.Stop()`.
  At baseline, the guest set `RUNTIME_STORE_PATH=/mnt/persistent/state`, but
  `nix/sandbox-vm.nix` did not set `SANDBOX_FILES_ROOT`, so Files app data may
  have used the sandbox fallback instead of `/mnt/persistent`. The local
  Section 1 patch addresses the Files root config; it has not yet been deployed.

### 1. VM Persistence and Background VM Motion

- [x] Make vmctl ownership durable or reattachable across service restart.
- [x] Ensure active desktop VM file root and runtime state live on persistent
      disk, not `/tmp`.
- [x] Ensure deploy/restart does not kill or replace active VMs unless explicitly
      requested.
- [x] Make background VM fork/snapshot state explicit and inspectable.
- [x] Prove a file created in the active desktop persists across browser reload
      and service restart.
- [x] Prove a background VM can be created from a known active/base snapshot.
- [x] Prove background VM work can be discarded without corrupting active state.

Section 1 local progress, 2026-05-12 UTC:

- Implemented local vmctl ownership persistence. `cmd/vmctl/main.go` now enables
  `OwnershipRegistry` persistence from `VMCTL_OWNERSHIP_PATH` or
  `VM_STATE_DIR/ownerships.json`.
- Persisted ownership metadata is reloaded on vmctl restart. Loaded running,
  booting, degraded, or stopping VMs are marked `stopped` with
  `stopped_by=vmctl-restart`; the next resolve reboots the same VM id and
  `data.img` instead of allocating a fresh desktop.
- Added focused coverage:
  `TestOwnershipRegistry_PersistsOwnershipAndRebootsSameVMIDAfterRestart`.
- Increased new per-VM `data.img` creation from 64 MiB to 2 GiB.
- Added a pre-boot expansion path for existing small `data.img` files. If a VM
  is booted from an old data image smaller than 2 GiB, vmmanager truncates it to
  the new target size and runs `resize2fs` before launching the guest.
- Set `SANDBOX_FILES_ROOT=/mnt/persistent/files` in `nix/sandbox-vm.nix`, so the
  Files app should use the same persistent data volume as `RUNTIME_STORE_PATH`.
- Added explicit fork lineage fields to vmctl ownership and responses:
  `parent_vm_id` and `snapshot_kind`.
- Added a truthful data-image fork path. `vmmanager.BootVM` can create a target
  VM's `data.img` by copying a source VM's `data.img`, preserving sparse holes.
  It rejects unsafe live copies when the source VM is currently running.
- Updated `fork_desktop` so VM-backed forks use `snapshot_kind=data_img_copy`
  and pass `SourceVMID` to vmmanager. Host-process fallback remains
  `snapshot_kind=metadata_only` because there is no separate disk to copy.
- Added focused fork/snapshot coverage:
  `TestOwnershipRegistry_ForkDesktopWithVMManagerUsesSourceDataImage`,
  `TestOwnershipRegistry_ForkDesktopRejectsActiveSourceWithVMManager`,
  `TestCopySparseFileClonesDataImageContent`,
  `TestBootVMRejectsRunningSourceDataImageFork`, and
  `TestBootVMClonesSourceDataImageBeforeLaunch`.
- Added old-disk expansion coverage:
  `TestBootVMExpandsExistingSmallDataImageBeforeLaunch`.
- Added live-process reattach groundwork:
  `vmmanager` now writes `firecracker.pid`, can `ReattachVM` from a persisted
  PID + host URL + epoch, and refuses reattach unless the PID exists and the
  guest `/health` endpoint responds.
- Added a vmctl process-survival mode. Node B config sets
  `KillMode=process` for `go-choir-vmctl` and
  `VMCTL_STOP_MANAGED_ON_EXIT=false`, so vmctl replacement should stop only
  the manager health checker and leave Firecracker child processes available
  for reattach.
- Added reattach coverage:
  `TestReattachVMRequiresPIDAndHealthyGuest` and
  `TestOwnershipRegistry_ReattachesPersistedVMWhenManagerCanAdopt`.
- Fixed a reattach network-slot risk found during review. A replacement vmctl
  manager now reserves the host-port/subnet implied by a reattached VM's
  persisted sandbox URL, so later boots cannot reuse the same guest subnet.
  Coverage: `TestReserveHostURLLockedPreservesReattachedNetworkSlots`.
- Verification passed locally:
  `git diff --check`,
  `go test ./internal/vmmanager ./internal/vmctl ./cmd/vmctl`,
  full `go test ./...` with local ICU flags, and `frontend` `npm run build`
  with the existing Ghostty chunk-size warning.
- Nix verification passed:
  `nix eval .#nixosConfigurations.go-choir-b.config.systemd.services.go-choir-vmctl.serviceConfig.KillMode --raw`
  returned `process`, and the vmctl service environment includes
  `VMCTL_STOP_MANAGED_ON_EXIT=false`.

Unresolved Section 1 gaps:

- Deployed service-restart proof passed on Node B after commit `3b9c356`.
  Proof user `mission-restart-1778558340` resolved to
  VM `vm-bb1b05195c9186fa06f22455522e81ff` at
  `http://172.1.0.2:8085`; a file written through `/api/files` survived
  `systemctl restart go-choir-vmctl` with the same VM id, same sandbox URL,
  active state, and identical file content. Node B evidence: 2.0 GiB
  `data.img`, persisted `ownerships.json`, `firecracker.pid`, `KillMode=process`,
  `VMCTL_STOP_MANAGED_ON_EXIT=false`, and journal line
  `vmmanager: reattached VM vm-bb1b05195c9186fa06f22455522e81ff`.
- Deploy-with-active-VM survival passed on Node B after rerunning GitHub Actions
  workflow `25712214504`, deploy job `75495916606`. Staging deployed commit
  `3ec211fc4e2f7cfe58cb848b5ed242c2e2d5f720`; the same proof user still mapped
  to VM `vm-bb1b05195c9186fa06f22455522e81ff`, state `active`, epoch `1`, and
  sandbox URL `http://172.1.0.2:8085`; the proof file still returned
  `mission restart proof 2026-05-12T03:59:00Z`. Node B `vmctl` health reported
  `active_vms=1` and `total_ownerships=1`. Journal evidence showed vmctl
  stopping only its health checker, loading one persisted ownership, and
  reattaching the same Firecracker PID-backed VM after deployment.
- Active VM runtime state persistence passed on Node B using the same proof VM.
  Created VText document `bb2b1cbe-9793-424d-adc8-3caaf47ff90a` and revision
  `b70d6c59-2cec-4da4-b7fe-fdcb46d6a9da` with content
  `runtime persistence proof 2026-05-12T04:13:02Z`, restarted
  `go-choir-vmctl`, then read back the same document and revision from the same
  VM `vm-bb1b05195c9186fa06f22455522e81ff`, same sandbox URL
  `http://172.1.0.2:8085`, state `active`, epoch `1`.
- Active VM file persistence passed on Node B using the same proof VM. The file
  `/api/files/mission-restart-proof.txt` returned
  `mission restart proof 2026-05-12T03:59:00Z` after browser-path baseline
  reload coverage, direct vmctl restart, and GitHub Actions deploy/restart.
- Background fork/snapshot proof passed on Node B using a hibernated source
  snapshot. The proof VM `vm-bb1b05195c9186fa06f22455522e81ff` was hibernated
  at epoch `1`, then forked into target desktop
  `mission-fork-20260512T041608Z` on
  VM `vm-03af9886dbffe4635887b8fbc45d53bd` with
  `parent_desktop_id=primary`, `parent_vm_id=vm-bb1b05195c9186fa06f22455522e81ff`,
  `snapshot_kind=data_img_copy`, `published=false`, and sandbox URL
  `http://172.2.0.2:8085`. The fork read the source proof file
  `mission restart proof 2026-05-12T03:59:00Z`, proving data-image copy
  inheritance from the source snapshot.
- Background discard proof passed on Node B. A fork-only file
  `/api/files/fork-only-proof.txt` was written inside the fork. The primary VM
  was resumed as the same VM id and epoch at `http://172.3.0.2:8085`; its
  source proof file remained present, and the fork-only file returned `404`.
  Removing target desktop `mission-fork-20260512T041608Z` returned `removed`;
  a lookup for the fork returned `404`; vmctl health then reported
  `active_vms=1` and `total_ownerships=1` with only the primary ownership.
- Existing deployed VMs with 64 MiB `data.img` should expand on next boot after
  this patch deploys, but this has not been verified on Node B.
- Invariant-level gap narrowed but not closed: VM-backed `fork_desktop` now
  refuses unsafe live active-disk copies and can clone a stopped/known base
  `data.img`. That is truthful for base snapshots, but it is not yet the desired
  live active-desktop fork flow for background coding.
- Worker VMs requested by `request_worker` still boot fresh data disks. They are
  isolated workers, but not clones of the active desktop filesystem.

Section 1 decision point:

- Keeping active VMs live through vmctl deploy/restart is now proven on Node B
  for one active VM: systemd process-only kill mode, no managed-VM stop on vmctl
  exit, PID files, and health-gated reattach preserved the same VM id and file
  content across both direct vmctl restart and a GitHub Actions staging deploy.
  The same path also preserved runtime VText state across restart.
- Forking from a quiesced source snapshot is proven. Forking a currently active
  VM disk still requires a no-disruption snapshot strategy. The current truthful
  implementation refuses live disk copies. Candidate paths: stop/clone/resume
  with visible user disruption, guest-assisted sync/freeze then clone,
  Firecracker snapshot/block-device snapshot support, or waiting for a known
  stopped/base snapshot. Do not reintroduce metadata-only forks as proof of
  background coding isolation.

### 2. GitHub Shipper Boundary

- [x] Design or implement the narrow shipper boundary for background VM commits.
- [x] Ensure worker/background VMs do not need broad GitHub credentials.
- [x] Export a background VM branch as a bundle or patchset plus manifest.
- [x] Import the bundle/patchset into a clean checkout.
- [x] Rerun required checks in the shipper context.
- [x] Push an `agent/<run-id>/<slug>` branch through the approved GitHub
      credential boundary.
- [x] Create or update PR metadata with run id, trace id, VM id, base/head SHA,
      verification manifest, and residual risks.
- [x] Confirm GitHub Actions runs on the pushed branch.

Section 2 local progress, 2026-05-12 UTC:

- Added a platform-side shipper boundary in `internal/shipper` and
  `cmd/shipper`. It imports a worker-produced patch file or sorted
  `.patch`/`.diff` directory plus a verification manifest into a clean checkout
  at an expected `base_sha`, creates a safe `agent/<run-id>/<slug>` branch,
  applies the patchset with `git apply --index`, commits with Choir provenance,
  runs configured checks, writes a JSON import report, and optionally pushes
  through the platform checkout's configured remote.
- The manifest requires `run_id`, `trace_id`, `vm_id`, and `base_sha`; optional
  fields include `snapshot_id`, `expected_head_sha`, verification results,
  residual risks, summary, and generation metadata. Commit messages include
  `Choir-Run-ID`, `Choir-Trace-ID`, `Choir-VM-ID`, and `Choir-Base-SHA`.
- Focused tests passed:
  `go test ./internal/shipper ./cmd/shipper`. Coverage includes clean-checkout
  import, branch safety, provenance commit body, report writing, configured
  checks, and dirty-repo rejection.
- CLI proof passed against a temporary clean git repository:
  `go run ./cmd/shipper import --repo <tmp-repo> --manifest <manifest.json> --patchset <change.patch> --branch agent/run-proof/shipper-proof --check "grep -q 'shipper proof' README.md" --report <report.json>`.
  The resulting report had status `imported`, branch
  `agent/run-proof/shipper-proof`, a new head SHA, and a passing check.
- Added and verified the matching low-privilege export path:
  `go run ./cmd/shipper export --repo <worker-repo> --out <export-dir> --base <base-sha> --run-id run-proof --trace-id trace-proof --vm-id vm-proof --snapshot-id snapshot-proof --summary "Export worker proof" --check "grep -q 'worker export proof' README.md"`.
  It wrote `manifest.json`, `changes.patch`, and `export-report.json` with a
  worker head SHA and passing check result, without any push capability.
- Export-to-import CLI proof passed: a temporary worker repo exported
  `changes.patch` and `manifest.json`; a separate clean checkout detached at the
  same base imported them to branch `agent/run-proof/export-import`, reran
  `grep -q 'worker export proof' README.md`, and wrote an import report. The
  shipper commit intentionally differs from the worker commit because the
  boundary imports a diff into a platform-owned commit while preserving the
  worker head SHA in provenance.
- Full local verification after adding export passed with local ICU flags:
  `go test ./...`.
- Shipper commit `6bc889d974f4068bc5075937c7ee68ef9f886be1` passed GitHub
  Actions workflow `25713210719`: frontend build, Go vet/test/build, and
  staging deploy job `75497805767`. Staging health reported proxy and sandbox
  deployed commit `6bc889d974f4068bc5075937c7ee68ef9f886be1`, deployed at
  `2026-05-12T04:25:18Z`.
- The same deploy also revalidated Section 1 VM reattach after the fork/resume
  proof: user `mission-restart-1778558340` still mapped to
  VM `vm-bb1b05195c9186fa06f22455522e81ff`, sandbox URL
  `http://172.3.0.2:8085`, state `active`, epoch `1`; the proof file still
  returned `mission restart proof 2026-05-12T03:59:00Z`; Node B journal showed
  vmctl loading one persisted ownership and reattaching that VM.
- Shipper export commit `deb377b9c926cba32714b82f0839675f94e2835f` passed
  GitHub Actions workflow `25713576280`: frontend build, Go vet/test/build, and
  staging deploy job `75498879196`. Staging health reported proxy and sandbox
  deployed commit `deb377b9c926cba32714b82f0839675f94e2835f`, deployed at
  `2026-05-12T04:36:26Z`.
- That deploy again revalidated Section 1 VM reattach. The same proof user
  `mission-restart-1778558340` still mapped to
  VM `vm-bb1b05195c9186fa06f22455522e81ff`, sandbox URL
  `http://172.3.0.2:8085`, state `active`, epoch `1`; the proof file still
  returned `mission restart proof 2026-05-12T03:59:00Z`. Node B journal showed
  vmctl stopping only its health checker, loading one persisted ownership, and
  reattaching the same VM after the deploy.
- Low-resolution GitHub boundary proof passed without granting worker GitHub
  authority. A worker checkout with no GitHub remote committed proof head
  `bd96e1109649045fe9b86cf4c313e27462a08091`, exported
  `/tmp/go-choir-shipper-proof.yIV4qk/export/manifest.json` and
  `changes.patch` with `cmd/shipper export`, and passed
  `test -f internal/shipper/testdata/github-boundary-proof.txt`.
- A separate platform checkout imported that patchset with `cmd/shipper import
  --push` to branch `agent/run-shipper-proof-20260512/github-boundary`, creating
  platform head `c72a763413a75398897c535f4e7e9c6f03df8bfe` and rerunning the
  same check. The import report was written to
  `/tmp/go-choir-shipper-proof.yIV4qk/import-report.json`.
- Draft PR #2, `[agent] Shipper GitHub boundary proof`, carried the run id
  `run-shipper-proof-20260512`, trace id `trace-shipper-proof-20260512`, VM id
  `vm-shipper-proof`, snapshot id `snapshot-shipper-proof`, base SHA
  `261896f58e69239cb54968e505aad2421f5821ab`, worker head SHA
  `bd96e1109649045fe9b86cf4c313e27462a08091`, platform head SHA
  `c72a763413a75398897c535f4e7e9c6f03df8bfe`, verification commands, and
  residual risks. It was closed after proof rather than merged because the file
  was only a boundary artifact.
- Pull request CI workflow `25713919314` passed on the imported branch:
  frontend build job `75499698777`, Go vet/test/build job `75499698779`, and
  skipped deploy job `75499896511` as expected for `pull_request`.
- Added worker-side `export_patchset` as a super/co-super tool. It wraps
  `internal/shipper.ExportPatchset`, writes only a manifest, patchset, and
  export report under the sandbox tool root, returns `github_push=false`, and
  has no remote push or PR creation capability. It is intentionally absent from
  conductor, VText, and researcher registries.
- `TestExportPatchsetToolExportsWithoutGitHubPush` proves a co-super can export
  a committed repo under the sandbox files root with run id, trace id, VM id,
  snapshot id, base SHA, worker head SHA, and verification checks, while keeping
  GitHub push outside the worker context.
- Full local verification after adding the worker export tool passed with local
  ICU flags: `go test ./...`.
- Worker export tool commit `724b3951b8387ff21ba95803cfb6fe31686dbb61`
  passed GitHub Actions workflow `25714329588`: frontend build job
  `75500868130`, Go vet/test/build job `75500868107`, and staging deploy job
  `75501044792`.
- Staging health after that deploy reported proxy and sandbox deployed commit
  `724b3951b8387ff21ba95803cfb6fe31686dbb61`, deployed at
  `2026-05-12T04:59:41Z`.
- The same deploy again revalidated Section 1 VM reattach. The proof user
  `mission-restart-1778558340` still mapped to
  VM `vm-bb1b05195c9186fa06f22455522e81ff`, sandbox URL
  `http://172.3.0.2:8085`, state `active`, epoch `1`; the proof file still
  returned `mission restart proof 2026-05-12T03:59:00Z`, and the persisted
  VText revision `b70d6c59-2cec-4da4-b7fe-fdcb46d6a9da` still returned
  `runtime persistence proof 2026-05-12T04:13:02Z`. Node B journal showed
  vmctl stopping only its health checker, loading one persisted ownership, and
  reattaching the same VM after the deploy.
- Added the next bridge toward real background VM execution: internal-only
  `/internal/runtime/runs` service-to-service endpoints for starting,
  polling, and inspecting constrained worker-runtime runs. These routes are not
  under `/api/*`, require `X-Internal-Caller: true`, and only allow worker
  profiles (`co-super` or `researcher`).
- Added super-only `delegate_worker_vm`, which takes a typed worker sandbox URL,
  starts a co-super/researcher run inside that worker VM runtime, polls it to
  completion, reads its event log, and returns any successful `export_patchset`
  tool results. This is intentionally unavailable to conductor, VText,
  researcher, and co-super.
- `TestDelegateWorkerVMToolRunsWorkerRuntimeAndCollectsExport` proves an active
  super can delegate to a separate worker-runtime HTTP server, where a co-super
  exports a committed repo patchset with no GitHub push capability.
- Full local verification after adding the worker runtime bridge passed with
  local ICU flags: `go test ./...`.
- Worker runtime bridge commit `22043676fbd4fb9df68cef567e1762f63fbbadf4`
  passed GitHub Actions workflow `25715221014`: frontend build, Go
  vet/test/build, and staging deploy. Staging health reported deployed commit
  `22043676fbd4fb9df68cef567e1762f63fbbadf4`, deployed at
  `2026-05-12T05:28:17Z`.
- The first deployed real-worker export attempt exposed a real environment gap:
  a fresh worker VM `vm-6298a8a17f22063a0ca1e7ec4895ef34` could run
  `git --version` through the `bash` tool, but `export_patchset` failed because
  direct Go `exec.Command("git", ...)` did not inherit a service `PATH`
  containing `git`. That proved adding packages to the guest image was
  necessary but not sufficient for direct tool execs.
- Guest image commit `d6206f1c5574e0756c1528266baed1e79761c32a` added `git`
  and `gnugrep` to the sandbox VM package set and passed GitHub Actions
  workflow `25715750400`, but the deployed proof still failed because the
  sandbox service environment was missing the direct exec `PATH`.
- Service PATH commit `08ad40a098670ea746debd848d79810d590418d7` set an
  explicit `go-choir-sandbox` guest service `PATH` including `git`, `grep`,
  `sed`, shell, coreutils, and related utilities. Nix evaluation proved the
  service environment included `git-2.53.0`; `git diff --check` passed.
- Commit `08ad40a098670ea746debd848d79810d590418d7` passed GitHub Actions
  workflow `25716186696`: frontend build job `75506564185`, Go vet/test/build
  job `75506564186`, and staging deploy job `75506766414`. Staging health
  reported proxy and sandbox deployed commit
  `08ad40a098670ea746debd848d79810d590418d7`, deployed at
  `2026-05-12T05:54:11Z`.
- The same deploy revalidated Section 1 VM reattach. Proof user
  `mission-restart-1778558340` still mapped to active VM
  `vm-bb1b05195c9186fa06f22455522e81ff`, sandbox URL
  `http://172.3.0.2:8085`, epoch `1`, and the proof file still returned
  `mission restart proof 2026-05-12T03:59:00Z`.
- Deployed real-worker export proof passed after the service `PATH` fix. vmctl
  created worker VM `vm-998ed4064f822af12e2f1a6faf01a89d`, worker id
  `worker-5bbc3f0d35502e8a`, sandbox URL `http://172.6.0.2:8085`, trajectory
  `trace-worker-export-path-20260512060024`.
- The worker internal co-super run
  `13c4d1fc-5a02-4265-87bc-f8cbe7b93aab` created a repo under the worker files
  root, committed base SHA `00fb020ddc3b70ff605f6fa310ed117a1dd8a840`,
  committed worker head `470966f07acad294bb07a1580fc606fb003a7f6d`, verified
  `grep -q mission-worker-export-service-path-proof-20260512 README.md`, and
  called `export_patchset`.
- The event log contains successful `export_patchset` tool results with
  `status="exported"` and `github_push=false`. The manifest is available inside
  the worker VM at `/mnt/persistent/files/exports/service-path-proof/manifest.json`
  and the patchset at `/mnt/persistent/files/exports/service-path-proof/changes.patch`.
  The manifest records run id
  `13c4d1fc-5a02-4265-87bc-f8cbe7b93aab`, trace id
  `trace-worker-export-path-20260512060024`, VM id
  `vm-998ed4064f822af12e2f1a6faf01a89d`, snapshot id
  `snapshot-service-path-proof-20260512`, base SHA
  `00fb020ddc3b70ff605f6fa310ed117a1dd8a840`, expected head SHA
  `470966f07acad294bb07a1580fc606fb003a7f6d`, and passed verification source
  `shipper export`.
- The exported patch contains exactly the proof delta:
  `+mission-worker-export-service-path-proof-20260512` in `README.md`.

Residual Section 2 gaps:

- The optional `--push` path is verified against GitHub through a platform
  checkout, PR CI is verified, and the worker VM export is now verified on a
  real deployed Firecracker worker. This still needs composition into a single
  product-path Choir-in-Choir workflow once Section 3 orchestration is stable.
- The worker-side export tool keeps GitHub credentials out of worker contexts
  by design. The remaining gap is product orchestration: VText/super should
  request the worker, delegate the coding task, collect the export, and hand it
  to the platform shipper without manual internal API calls.

### 3. Product Orchestration Proof

- [x] Submit a real request from the deployed prompt bar.
- [x] Verify the browser did not call internal orchestration APIs.
- [x] Verify conductor created a VText with an initial document abstract, not
      template control text.
- [x] Verify VText requested research for current/factual claims instead of
      writing from model priors.
- [x] Verify researchers performed sequential and/or parallel real searches when
      the task required current information.
- [x] Verify search provider/endpoint success, rate limit, error, and latency
      stats appear in Trace.
- [x] Verify VText consumed worker updates and produced full current-state
      document revisions.
- [ ] Verify super/cosuper work happens in background VM context for mutable code
      or filesystem changes.

Section 3 deployed proof, 2026-05-12 UTC:

- Commit `f65a0a4487713ee34144424ac303cd415cfe80fd` passed GitHub Actions
  workflow `25716964367`: frontend build, Go vet/test/build, and staging deploy.
  Staging health reported proxy and sandbox deployed commit
  `f65a0a4487713ee34144424ac303cd415cfe80fd`, deployed at
  `2026-05-12T06:15:30Z`.
- Deployed product-path proof passed:
  `cd frontend && GO_CHOIR_RUN_REAL_VTEXT_DEMO=1 GO_CHOIR_REAL_DEMO_BASE_URL=https://draft.choir-ip.com npx playwright test tests/vtext-real-workflow-demo.spec.js --project=chromium --reporter=line`
  passed 1/1 in 3.7 minutes.
- Evidence artifacts:
  `frontend/test-results/vtext-real-workflow-demo-r-23628-d-artifact-and-verification-chromium/video.webm`,
  `frontend/test-results/vtext-real-workflow-demo-r-23628-d-artifact-and-verification-chromium/trace.zip`,
  and
  `frontend/test-results/vtext-real-workflow-demo-r-23628-d-artifact-and-verification-chromium/test-finished-1.png`.
- The proof registered a fresh staging browser user, submitted through
  `/api/prompt-bar`, and used submission/trajectory
  `8778d93a-e21c-4f32-ba72-6227e53bfaea`. The browser request monitor observed
  no forbidden product-proof calls to `/internal/*`, `/api/agent/*`,
  `/api/prompts*`, `/api/test/*`, or `/api/events`.
- The proof marker was `REAL_VTEXT_DEMO_1778566828334`. The conductor-created
  first VText revision contained the marker and did not contain template/control
  strings such as `Conductor framing`, `Use this vtext`, `User request:`,
  `Current requirements:`, or `Grounding status:`.
- Trace roles for the same trajectory included `conductor`, `vtext`,
  `researcher`, and `super`. The final canonical VText revision had
  `metadata.source=edit_vtext` and `metadata.vtext_edit_kind=vtext_edit`, and
  consumed both researcher and super worker updates.
- Trace search stats for the same trajectory recorded 32 queries, 98 attempts,
  64 successes, and 11 rate limits. Provider-level endpoint stats were visible:
  Brave at `https://api.search.brave.com/res/v1/web/search` had 16 attempts,
  5 successes, and 11 rate limits; Exa at `https://api.exa.ai/search` had
  23 attempts and 23 credit-limit errors; Serper at
  `https://google.serper.dev/search` had 31 attempts and 31 successes; Tavily at
  `https://api.tavily.com/search` had 28 attempts and 28 successes.
- The product path generated real Files app artifacts:
  `artifacts/real_vtext_demo_1778566828334-evolution-ca.html` and
  `artifacts/real_vtext_demo_1778566828334-evolution-ca.verify.js`. Trace showed
  a successful `bash` tool result running Node against the verification script.
  The verification output included `PASS html contains marker`, canvas/control
  checks, `PASS implements automaton step logic`, and
  `Verification passed for REAL_VTEXT_DEMO_1778566828334`.

Residual Section 3 gap:

- This proves deployed prompt bar -> conductor -> VText -> researcher -> super
  -> VText document revision with real search, generated files, Trace evidence,
  and Node verification. It does not yet prove the stronger invariant that
  mutable code/filesystem work is performed in a background VM and exported
  through the shipper path. The generated artifact was product-visible in the
  user's Files app, but the proof did not show background-VM mutable execution.

### 4. Desktop and VText UX

- [ ] Make VText chrome one row on mobile and prevent controls from covering
      document text.
- [ ] Make VText top controls fade or compact while scrolling if needed.
- [ ] Make Markdown rendered and editable in one surface, not a permanent
      read/edit mode split.
- [ ] Prevent mobile viewport zoom in/out during prompt and document editing.
- [ ] Make the prompt bar grow vertically for multiline input.
- [ ] Keep sign-out/account actions quiet and menu-based.
- [ ] Ensure VText opens to recent documents when no document is selected.
- [ ] Ensure window geometry persists and repairs itself across viewport changes.
- [ ] Verify with mobile, tablet, and desktop screenshots on staging.

### 5. Trace, Settings, and App Registry Health

- [ ] Ensure Trace uses only `/api/trace/*` or product-safe projections.
- [ ] Ensure Trace shows trajectories, events, messages, tool calls, search
      stats, and shipper/VM events without 404s.
- [ ] Ensure Settings has safe product settings only by default.
- [ ] Ensure prompt/runtime mutation is absent from normal product Settings.
- [ ] Centralize app metadata so adding an app touches the minimum number of
      files.
- [ ] Centralize theme tokens so a future prompt can redesign the desktop by
      changing a validated theme/layout config, not arbitrary source edits.

### 6. Shared Content Substrate

- [ ] Define the common content object model for uploaded files, URLs, extracted
      text, media metadata, provenance, and app routing.
- [ ] Support uploads or durable references for text/Markdown, PDF, EPUB, image,
      audio, video, YouTube URL, and podcast RSS feed.
- [ ] Implement or connect extraction rungs for URL text extraction, SearXNG
      discovery, Defuddle/readability, and optional Obscura acquisition.
- [ ] Record content provenance, hashes, extraction rung, source URL, timestamp,
      and warnings.
- [ ] Make conductor route raw links/uploads to the appropriate display app when
      the prompt contains no contextual VText ingestion instruction.
- [ ] Make contextual prompts route content into VText ingestion/research instead
      of directly opening the display app.

### 7. Content Apps as Self-Development Payload

Build these through the deployed Choir self-development workflow where possible.
If direct local bootstrap work is unavoidable, mark it explicitly as bootstrap,
not as proof of Choir-in-Choir.

- [ ] EPUB reader app opens uploaded/linked EPUB content.
- [ ] PDF reader app opens uploaded/linked PDF content.
- [ ] Image viewer app opens uploaded/linked image content.
- [ ] Video app opens uploaded/linked video and YouTube URLs.
- [ ] Audio app opens uploaded/linked audio content.
- [ ] Podcast app searches or accepts RSS, lists episodes, and plays an episode.
- [ ] All content apps share app registry patterns and content substrate APIs.
- [ ] Each app has at least one staging Playwright proof.

### 8. Publishing Path Skeleton

- [ ] Document the distinction between user desktop VText state and platform
      publication state.
- [ ] Sketch or implement the platform Dolt appliance boundary for published
      VTexts.
- [ ] Define the first publish operation: selected VText revision from user
      desktop -> platform publishing desktop.
- [ ] Record open questions for version publishing, redaction, paywalls,
      collaboration, citations, and CHIPS economics without blocking tonight's
      self-development proof.

### 9. Final Proof Package

- [ ] Run relevant local tests only for bootstrap code that must land locally.
- [ ] Run deployed Playwright for prompt bar -> VText -> research -> background
      VM -> shipper/GitHub/CI where implemented.
- [ ] Capture a staging video showing a real user-facing workflow.
- [ ] Capture Trace evidence for the same workflow.
- [ ] Capture GitHub branch/PR/CI evidence for generated work.
- [ ] Push any local bootstrap commits through normal reviewed Git flow.
- [ ] Write a final report naming completed items, skipped items, residual risks,
      and invariant-level blockers.

## Completion Standard

The mission is complete only if the final report can convince a skeptical
reviewer that:

- the deployed product path did real work
- no forbidden shortcut was used as proof
- durable VM state exists where claimed
- code crossed an auditable shipper/GitHub boundary where claimed
- Trace can explain the workflow
- staging demonstrates the user-facing result
- unresolved gaps are documented as gaps, not hidden as success
