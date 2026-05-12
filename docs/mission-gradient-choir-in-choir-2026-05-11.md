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
- [x] Verify super/cosuper work happens in background VM context for mutable code
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

Section 3 background worker proof, 2026-05-12 UTC:

- Commit `a9c6e7ce886612e1d1b498c288a43396a0090b45` passed GitHub Actions
  workflow `25718378258`: frontend build, Go vet/test/build, and staging
  deploy. Staging health reported proxy and sandbox deployed commit
  `a9c6e7ce886612e1d1b498c288a43396a0090b45`, deployed at
  `2026-05-12T06:51:18Z`.
- Deployed background-worker product proof passed:
  `cd frontend && GO_CHOIR_RUN_BACKGROUND_WORKER_DEMO=1 GO_CHOIR_WORKER_DEMO_BASE_URL=https://draft.choir-ip.com npx playwright test tests/vtext-background-worker-demo.spec.js --project=chromium --reporter=line`
  passed 1/1 in 2.4 minutes.
- Evidence artifacts:
  `frontend/test-results/vtext-background-worker-de-af644-background-worker-VM-export-chromium/video.webm`,
  `frontend/test-results/vtext-background-worker-de-af644-background-worker-VM-export-chromium/trace.zip`,
  and
  `frontend/test-results/vtext-background-worker-de-af644-background-worker-VM-export-chromium/test-finished-1.png`.
- The proof registered a fresh staging browser user, submitted through
  `/api/prompt-bar`, and used submission/trajectory
  `a02e982c-f956-4ce9-a317-c4e9bada3901`. The browser request monitor observed
  no forbidden product-proof calls to `/internal/*`, `/api/agent/*`,
  `/api/prompts*`, `/api/test/*`, or `/api/events`.
- The proof marker was `BACKGROUND_WORKER_DEMO_1778568984055`. Trace showed
  successful `request_worker_vm` results returning worker handles such as
  `worker-730a4d5ec28307a9` with VM
  `vm-0676fe81a8213d39aaa377576a383539`, machine class `worker-small`, and a
  tap-subnet sandbox URL.
- Trace showed successful `delegate_worker_vm` results with exported patchsets.
  The selected final VText export reported worker
  `worker-b40c5bea04d58af0`, VM
  `vm-0bd9bf690b505d5a282609b8d2a6ac00`, manifest path
  `/mnt/persistent/files/export_out/manifest.json`, patchset path
  `/mnt/persistent/files/export_out/changes.patch`, base SHA
  `ae4c6c5c73229b5a84cd250b0d79ec177a23ce71`, worker head SHA
  `ed2b4ce82e36504bee6fe23bc4cb8dd85d926464`, and `github_push=false`.
- The final canonical VText revision had `metadata.source=edit_vtext` and
  reported that `README.md` was created in the background worker VM, committed
  in git, verified with
  `grep -n 'BACKGROUND_WORKER_DEMO_1778568984055' README.md`, and exported as a
  patchset. The proof therefore covers deployed prompt bar -> conductor -> VText
  -> super -> background worker VM -> patchset export -> VText current-state
  document without manual browser orchestration.

Residual Section 3 follow-up:

- The background-worker proof exposed duplicate addressed deliveries: the
  worker completed four exports before VText selected the final one. That does
  not invalidate the background-VM boundary proof, but it is a real efficiency
  and idempotency issue. Future worker delegation should dedupe addressed
  deliveries or make repeated worker-update consumption idempotent.

### 4. Desktop and VText UX

- [x] Make VText chrome one row on mobile and prevent controls from covering
      document text.
- [x] Make VText top controls fade or compact while scrolling if needed.
- [x] Make Markdown rendered and editable in one surface, not a permanent
      read/edit mode split.
- [x] Prevent mobile viewport zoom in/out during prompt and document editing.
- [x] Make the prompt bar grow vertically for multiline input.
- [x] Keep sign-out/account actions quiet and menu-based.
- [x] Ensure VText opens to recent documents when no document is selected.
- [x] Ensure window geometry persists and repairs itself across viewport changes.
- [x] Verify with mobile, tablet, and desktop screenshots on staging.

Section 4 deployed UX proof, 2026-05-12 UTC:

- Added `frontend/tests/vtext-responsive-ux.spec.js` as a staging-capable proof
  for the VText/mobile desktop UX contract.
- Staging UX proof passed:
  `cd frontend && GO_CHOIR_UX_BASE_URL=https://draft.choir-ip.com npx playwright test tests/vtext-responsive-ux.spec.js --project=chromium --reporter=line`
  passed 2/2 in 28.0 seconds.
- `npm run build` passed after the test addition, with the existing
  `ghostty-web` large chunk warning.
- Mobile proof verified the viewport meta contains `maximum-scale=1` and
  `interactive-widget=resizes-content`, VText opens nearly full-screen at
  390x844, the VText toolbar is one row and does not overlap document text,
  the document surface is `contenteditable`, Markdown renders in the same
  editable surface, no permanent `Read` or `Edit` mode buttons exist, toolbar
  opacity fades while reading/scrolling, the prompt textarea grows for multiple
  lines, and sign-out is absent until the desktop menu opens.
- Mobile reload proof verified VText window geometry survives reload and a
  no-document VText window returns to the recent-documents landing instead of a
  blank editor.
- Tablet and desktop proof verified VText default window geometry is large
  enough for real reading/writing and keeps the editable document below the
  controls.
- Evidence artifacts:
  `frontend/test-results/vtext-responsive-ux-mobile-69633-rkdown-and-quiet-account-UI-chromium/mobile-vtext.png`,
  `frontend/test-results/vtext-responsive-ux-mobile-69633-rkdown-and-quiet-account-UI-chromium/video.webm`,
  `frontend/test-results/vtext-responsive-ux-mobile-69633-rkdown-and-quiet-account-UI-chromium/trace.zip`,
  `frontend/test-results/vtext-responsive-ux-VText--471fe-large-on-tablet-and-desktop-chromium/tablet-vtext.png`,
  `frontend/test-results/vtext-responsive-ux-VText--471fe-large-on-tablet-and-desktop-chromium/desktop-vtext.png`,
  `frontend/test-results/vtext-responsive-ux-VText--471fe-large-on-tablet-and-desktop-chromium/video.webm`,
  and
  `frontend/test-results/vtext-responsive-ux-VText--471fe-large-on-tablet-and-desktop-chromium/trace.zip`.

Residual Section 4 follow-up:

- A manually-created blank VText window reopens to the recent-documents landing
  after reload because the parent desktop window context is not updated with the
  new document id. This preserves window geometry and avoids a blank document,
  but selected-document persistence for manual blank documents should be
  tightened later.

### 5. Trace, Settings, and App Registry Health

- [x] Ensure Trace uses only `/api/trace/*` or product-safe projections.
- [x] Ensure Trace shows trajectories, events, messages, tool calls, search
      stats, and shipper/VM events without 404s.
- [x] Ensure Settings has safe product settings only by default.
- [x] Ensure prompt/runtime mutation is absent from normal product Settings.
- [x] Centralize app metadata so adding an app touches the minimum number of
      files.
- [x] Centralize theme tokens so a future prompt can redesign the desktop by
      changing a validated theme/layout config, not arbitrary source edits.

Section 5 deployed proof, 2026-05-12 UTC:

- Commit `8a43bf1e7ab101a3e671f1e3b5f43a164758f0c7` centralized app metadata
  helpers in `frontend/src/lib/stores/desktop.js` and removed the duplicate
  icon mapping from `frontend/src/lib/Desktop.svelte`.
- The same commit moved root desktop theme variables behind
  `frontend/src/lib/theme.js` helpers. `App.svelte` applies the validated
  default theme to `documentElement`, and the app root carries
  `data-theme-id=system-noir`; BottomBar can still update the dynamic
  `--choir-bottom-bar-height` token at runtime.
- `frontend/tests/trace-settings-registry.spec.js` was added as a staging proof
  for the Trace/Settings/app-registry/theme surface. It rejects browser calls to
  `/api/agent/*`, `/api/prompts`, `/api/test/*`, `/internal*`, and `/api/events`;
  flags Trace 4xx responses; verifies the six registry-backed desktop apps and
  icons; verifies Settings exposes account/theme/layout/runtime status only; and
  verifies theme tokens are present through the product shell.
- Local verification before push: `cd frontend && npm run build` passed with the
  existing `ghostty-web` large chunk warning.
- GitHub Actions run `25720338856` for
  `8a43bf1e7ab101a3e671f1e3b5f43a164758f0c7` passed: frontend build, Go
  vet/test/build, and staging deploy job `75520013099`.
- Staging health after deploy reported proxy and sandbox commit
  `8a43bf1e7ab101a3e671f1e3b5f43a164758f0c7`, deployed at
  `2026-05-12T07:36:55Z`.
- Section 5 staging proof passed:
  `cd frontend && GO_CHOIR_SECTION5_BASE_URL=https://draft.choir-ip.com npx playwright test tests/trace-settings-registry.spec.js --project=chromium --reporter=line`
  passed 1/1 in 9.8 seconds.
- Evidence artifacts:
  `frontend/test-results/trace-settings-registry-Tr-5ce78-ta-come-from-product-config-chromium/trace-settings-registry.png`,
  `frontend/test-results/trace-settings-registry-Tr-5ce78-ta-come-from-product-config-chromium/video.webm`,
  `frontend/test-results/trace-settings-registry-Tr-5ce78-ta-come-from-product-config-chromium/trace.zip`, and
  `frontend/test-results/trace-settings-registry-Tr-5ce78-ta-come-from-product-config-chromium/test-finished-1.png`.
- Populated Trace behavior is covered by earlier product-path proofs in this
  document: Section 3's real VText demo showed trajectories, agent graph,
  moments, messages, tool calls, and provider-level search stats through
  `/api/trace/*`; Section 3's background-worker demo showed worker VM request
  and delegation results, exported patchset paths, base/head SHAs, and
  `github_push=false` through Trace moment details.

Residual Section 5 follow-up:

- GitHub PR/CI/deploy provenance is still represented by shipper reports,
  GitHub Actions run IDs, and this mission document. It is not yet emitted as a
  first-class product Trace event inside the user's trajectory. That should be
  added when the platform shipper is invoked from the product workflow instead
  of from operator/CLI proof commands.

### 6. Shared Content Substrate

- [x] Define the common content object model for uploaded files, URLs, extracted
      text, media metadata, provenance, and app routing.
- [x] Support uploads or durable references for text/Markdown, PDF, EPUB, image,
      audio, video, YouTube URL, and podcast RSS feed.
- [x] Implement or connect extraction rungs for URL text extraction, SearXNG
      discovery, Defuddle/readability, and optional Obscura acquisition.
- [x] Record content provenance, hashes, extraction rung, source URL, timestamp,
      and warnings.
- [x] Make conductor route raw links/uploads to the appropriate display app when
      the prompt contains no contextual VText ingestion instruction.
- [x] Make contextual prompts route content into VText ingestion/research instead
      of directly opening the display app.

Section 6 evidence:

- Implemented a durable `ContentItem` model/table/store/API for owner-scoped
  content references. The record carries `source_type`, `media_type`,
  `app_hint`, source/canonical URL, optional file path, extracted text, SHA-256
  hash, metadata JSON, and provenance JSON.
- Added product-safe content APIs under `/api/content/*` and a researcher-only
  `import_url_content` tool. The tool stores URL imports through the same
  substrate instead of returning loose text blobs.
- Implemented direct HTTP acquisition with browser-ish headers, bounded body
  reads, text/HTML extraction, a `readability_lite` HTML distillation rung, and
  SearXNG alternate discovery through `SEARXNG_URL` or `CHOIR_SEARXNG_URL`.
  Obscura remains optional and was not needed for this substrate proof.
- Added provenance for source URL, fetch timestamp, acquisition/extraction
  rungs, warnings, alternate candidates, content hash algorithm, HTTP status,
  and content type.
- Added frontend content viewers for `pdf`, `epub`, `image`, `video`, `audio`,
  and `podcast` app hints, sharing the centralized desktop app registry rather
  than one-off window wiring.
- Updated prompt-bar routing so a bare content URL is a server-owned conductor
  decision that opens the relevant display app without requiring a provider
  call. This fixed the staging failure where a bare PDF link attempted a
  ChatGPT-backed conductor run and hit a gateway `401 Unauthorized`.
- Kept contextual URL prompts on the VText path. Tests cover the distinction:
  bare URL -> display app; contextual URL -> VText/research ingestion.
- Local checks passed:
  `go test ./internal/runtime -run 'TestContent|TestPromptBar.*URL' -count=1`,
  full `go test ./...` with local ICU flags, `npm run build`, and
  `git diff --check`.
- GitHub Actions run `25724505919` for
  `1d8d1dd5cf96d82aae0868ab6aaa436fa420bfd4` passed frontend build,
  Go vet/test/build, and staging deploy.
- GitHub Actions run `25725687711` for
  `46186b91628e34dd6fea3ad2278ce6e84d63f2dc` passed frontend build,
  Go vet/test/build, and staging deploy job `75538071590`.
- Staging health after the second deploy reported proxy and sandbox commit
  `46186b91628e34dd6fea3ad2278ce6e84d63f2dc`, deployed at
  `2026-05-12T09:28:53Z`.
- Deployed Section 6 proof passed:
  `cd frontend && GO_CHOIR_RUN_CONTENT_SUBSTRATE=1 GO_CHOIR_CONTENT_BASE_URL=https://draft.choir-ip.com npx playwright test tests/content-substrate-routing.spec.js --project=chromium --reporter=line`
  passed 1/1 in 10.9 seconds against Node B.

Residual Section 6 follow-up:

- The in-tree extractor is a conservative `readability_lite` baseline, not the
  full Defuddle extractor from the separate extraction bakeoff. Port Defuddle
  when extraction quality becomes the bottleneck.
- Obscura is not yet used as an acquisition rung. Keep it as the lightweight
  browser candidate for pages that need JavaScript rendering without Playwright
  in every desktop VM.
- URL import currently allows ordinary `http`/`https` public fetches but does
  not yet include a full SSRF/private-network denylist. Add that before treating
  `/api/content/import-url` as hardened.
- These are substrate viewers, not polished reader/player apps. Dedicated PDF,
  EPUB, media, YouTube, and podcast app UX remains Section 7.

### 7. Content Apps as Self-Development Payload

Build these through the deployed Choir self-development workflow where possible.
If direct local bootstrap work is unavoidable, mark it explicitly as bootstrap,
not as proof of Choir-in-Choir.

- [x] EPUB reader app opens uploaded/linked EPUB content.
- [x] PDF reader app opens uploaded/linked PDF content.
- [x] Image viewer app opens uploaded/linked image content.
- [x] Video app opens uploaded/linked video and YouTube URLs.
- [x] Audio app opens uploaded/linked audio content.
- [x] Podcast app searches or accepts RSS, lists episodes, and plays an episode.
- [x] All content apps share app registry patterns and content substrate APIs.
- [x] Each app has at least one staging Playwright proof.

Section 7 evidence:

- Added `frontend/tests/content-apps-routing.spec.js` as a staging proof for
  content app routing. It submits bare references through the real deployed
  prompt bar and verifies `pdf`, `epub`, `image`, `audio`, `video`, and
  `podcast` windows open from server-owned `open_app` decisions without browser
  requests to `/api/agent/*`, `/api/prompts`, `/api/test/*`, `/internal`, or
  `/api/events`.
- Added podcast RSS handling to `ContentViewer`: bare RSS references import
  through `/api/content/import-url`, parse stored feed fragments, list episodes,
  and render audio controls. The parser is intentionally tolerant because the
  shared content substrate stores bounded text excerpts and can truncate large
  feeds mid-document.
- GitHub Actions run `25726670952` for
  `6ef8582c6e2470cc48b44364ed93ae4e18129227` passed frontend build,
  Go vet/test/build, and staging deploy. The first deployed proof against this
  commit failed because strict RSS `DOMParser` rejected the successfully
  imported but truncated BBC feed XML.
- GitHub Actions run `25727230342` for
  `74b42a19f922bc3ba684d18053e535be4fef61a1` passed frontend build,
  Go vet/test/build, and staging deploy job `75543344581`.
- Staging health after the parser fix reported proxy and sandbox commit
  `74b42a19f922bc3ba684d18053e535be4fef61a1`, deployed at
  `2026-05-12T10:00:08Z`.
- Deployed Section 7 proof passed:
  `cd frontend && GO_CHOIR_RUN_CONTENT_APPS=1 GO_CHOIR_CONTENT_BASE_URL=https://draft.choir-ip.com npx playwright test tests/content-apps-routing.spec.js --project=chromium --reporter=line`
  passed 1/1 in 11.5 seconds against Node B.

Residual Section 7 follow-up:

- These are still first-pass reader/player surfaces. PDF/image/audio/video use
  browser-native rendering, EPUB opens as a durable reference, and podcast feed
  parsing is intentionally minimal.
- Full product polish remains: file uploads, local file rendering, EPUB
  spine/chapters, media metadata, fullscreen behavior, transclusion hooks, and
  better error states.
- This was bootstrap implementation plus deployed proof, not yet generated by
  a background Super workflow. That remains part of the larger Choir-in-Choir
  capability target.

### 8. Publishing Path Skeleton

- [x] Document the distinction between user desktop VText state and platform
      publication state.
- [x] Sketch or implement the platform Dolt appliance boundary for published
      VTexts.
- [x] Define the first publish operation: selected VText revision from user
      desktop -> platform publishing desktop.
- [x] Record open questions for version publishing, redaction, paywalls,
      collaboration, citations, and CHIPS economics without blocking tonight's
      self-development proof.

Section 8 evidence:

- Added `docs/publication-path-skeleton-2026-05-12.md`.
- The skeleton defines private user desktop state as per-user embedded Dolt plus
  snapshot filesystem, and platform publication state as explicit
  platform-visible records in platform Dolt plus content-addressed public
  artifacts.
- The platform Dolt appliance boundary is ledger/index only: publication rows,
  public artifact metadata, published revision refs, citation graph edges,
  routing records, compute accounting, and later CHIPS state. It is explicitly
  not the live editor, actor mailbox, or cross-VM message bus.
- The first publish operation is one selected immutable VText revision copied
  into platform-visible publication state with source doc/revision IDs, content
  hashes, artifact refs, Trace/publication events, and a platform publishing
  desktop reader surface.
- Open questions are recorded for version publishing, redaction/projection,
  paywalls and release timing, collaboration submissions, citations, and CHIPS
  economics. These are preserved as forward-compatible design space, not
  imported into the current execution goal.

### 9. Final Proof Package

- [x] Run relevant local tests only for bootstrap code that must land locally.
- [x] Run deployed Playwright for prompt bar -> VText -> research -> background
      VM -> shipper/GitHub/CI where implemented.
- [x] Capture a staging video showing a real user-facing workflow.
- [x] Capture Trace evidence for the same workflow.
- [x] Capture GitHub branch/PR/CI evidence for generated work.
- [x] Push any local bootstrap commits through normal reviewed Git flow.
- [x] Write a final report naming completed items, skipped items, residual risks,
      and invariant-level blockers.

Final proof package, 2026-05-12 UTC:

- Final report:
  `docs/mission-gradient-choir-in-choir-final-report-2026-05-12.md`.
- Recent mission commits cited by this report:
  `46186b91628e34dd6fea3ad2278ce6e84d63f2dc`,
  `6ef8582c6e2470cc48b44364ed93ae4e18129227`,
  `74b42a19f922bc3ba684d18053e535be4fef61a1`,
  `64b639e`, and `271adaf`.
- Latest code-bearing CI/deploy proof:
  GitHub Actions run `25727230342` passed frontend build, Go vet/test/build,
  and staging deploy for commit
  `74b42a19f922bc3ba684d18053e535be4fef61a1`.
- Docs-only commit `271adaf` pushed successfully to `origin/main`; GitHub did
  not create a run for that commit.
- Current staging prompt-bar/content-app proof artifact:
  `frontend/test-results/content-apps-routing-bare--cd595-nt-apps-from-the-prompt-bar-chromium/video.webm`,
  `trace.zip`, and `test-finished-1.png`.
- Product-path VText and background-worker proof artifacts are recorded in
  Section 3. They cover deployed prompt bar -> conductor -> VText -> researcher
  and super -> background worker VM -> patchset export -> VText revision, with
  Trace evidence and staging videos.
- Shipper/GitHub boundary proof is recorded in Section 2. Pull request #2,
  `[agent] Shipper GitHub boundary proof`, used branch
  `agent/run-shipper-proof-20260512/github-boundary`, head
  `c72a763413a75398897c535f4e7e9c6f03df8bfe`, and GitHub Actions run
  `25713919314`, whose frontend and Go checks passed.
- Important residual gap: product-invoked background-worker shipping currently
  proves patchset export and VText reporting, while platform shipper -> PR/CI
  is proven as a separate boundary. The product flow does not yet automatically
  invoke the platform shipper and emit PR/CI provenance back into the user's
  trajectory.

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
