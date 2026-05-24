# MissionGradient: Deploy Impact Classes And Nix Cache v0

**Status:** ready_for_execution
**Date:** 2026-05-24
**Purpose:** make staging deploy time proportional to the code that changed instead of rebuilding host and guest VM images for every deployed source change. Cache is a supporting mechanism; the real artifact is a deploy graph whose expensive roots are not entered unnecessarily.

## One-Line Goal String

```text
/goal Run docs/mission-deploy-impact-classes-cache-v0.md as a Codex-operated MissionGradient mission: make Choir's staging deploy graph proportional to deploy impact, with cache as support rather than the proof. Starting from the trusted-copy baseline at 8335d83 and the current partial impact-class workflow edits, finish a conservative deploy-impact classifier for frontend, host-service, vmctl-restart, ordinary-guest, playwright-guest, and host-OS changes; build/copy only selected Nix roots; install/restart only affected services or guest images; and preserve staging identity, health checks, and rollback discipline. Prove with timing evidence from frontend-only, host-service-only, and guest-image-impacting pushes that non-guest changes avoid MicroVM EROFS image builds and guest changes still build/install the required image. Separately measure whether Magic Nix Cache/FlakeHub improves the remaining selected roots after graph separation; do not treat paid cache as a substitute for dependency isolation. Do not weaken CI evidence, skip required health checks, hide copy failures behind remote rebuilds, rebuild all images by default, restart vmctl for unrelated frontend changes, or claim success without staging `/health` commit identity, CI/deploy timing deltas, rollback refs, residual cache risks, and the next deploy-speed realism axis.
```

## Cognitive Transform Review

### Current Uncertainty Or Obstacle

The staging deploy path now has two different behaviors:

- the copy/signature blocker is fixed: GitHub-built closures can be copied to Node B through the trusted root SSH deploy channel;
- deploy time is still dominated by building MicroVM guest images, especially `microvm-store-disk.erofs`, even for changes that should not need a new guest image.

The last measured run for commit `8335d83` proved:

```text
CI tests: ~1 minute scale
Prebuild and copy Nix deploy closures: 25m10s
  - build roots: 1463s
  - copy closures: 47s
Node B deploy phase: 22s
Staging health: deployed_commit=8335d83f560ba9af4a8fed0d8a15b8c014f01bd3
```

The `bwrap: setting up uid map: Permission denied` message is a slow-path warning from `microvm.nix` EROFS creation on the GitHub runner, not the immediate deploy failure.

### Selected Transforms

1. **Depth Extraction:** "Microservices" is not enough if the build graph still treats the repo as one closure.
2. **Boundary Transform:** separate deploy control decisions from Nix derivation dependencies.
3. **Load-Bearing Variable Transform:** optimize the expensive artifact class, not the visible deploy shell.
4. **Critical-Path Transform:** optimize the path that blocks iteration, not the total theoretical work. If ordinary product/UI fixes still pay the guest-image tax, the deploy system remains wrong even if a full deploy gets somewhat faster.
5. **False-Economy Transform:** paid cache can reduce repeated selected builds, but it cannot fix a graph that selects the wrong roots. Prove class selection before judging paid cache value.
6. **Anti-Goodhart Transform:** a skipped deploy or skipped guest image install is only success if staging identity and health prove the right deployed state.

### Route-Changing Insights

- Runtime microservice independence does not automatically imply deploy independence. The Nix derivations and CI roots must also be independent.
- The most expensive artifact is the guest EROFS store disk, not `switch-to-configuration`. Avoiding unnecessary guest builds matters more than shaving seconds from Node B activation.
- `buildCommit` in service `ldflags` can poison cache reuse. If every commit changes every binary, impact classes cannot pay off fully.
- Restart policy is part of the graph. A frontend-only change should not restart `vmctl`; a vmctl binary/config or guest image change should.
- A host NixOS closure may still reference many services. Impact classes should first reduce which roots are built/copied/installed, then refine derivation source filters and build metadata.
- The deploy script must retain a reliable fallback path, but a fallback remote build should be reported as a degraded cache result, not as a normal fast deploy success.
- The useful cache question comes after graph separation: for a selected root, did Magic Nix Cache or FlakeHub reduce rebuild/copy time, and which derivations still miss?

### Changed Plan From The Transform Pass

Implementation:

- finish the conservative class selector first, including a separate `deploy_vmctl_restart` decision;
- keep the first cut path-based and explain every class decision in workflow logs;
- make selected-root prebuild/copy and Node B fallback consume the same class booleans;
- only after P0-P2 proof, investigate source-filter and `ldflags` cache poisoning.

Verifier/evidence:

- use timing deltas by deploy class, not a single "deploy faster" number;
- require negative proof for non-guest changes: logs must show no `guest-image`, no `guest-image-playwright`, and no `microvm-store-disk.erofs`;
- require positive proof for guest changes: logs must show the expected image built/installed and `vmctl` restarted or reloaded for a named reason;
- record cache behavior as `selected root cache hit/miss`, not as mission success by itself.

Scope:

- v0 may remain conservative and overbuild when dependency ownership is ambiguous;
- v0 must not underbuild or silently skip deploy;
- paid FlakeHub cache evaluation is downstream of class-selection proof.

Stopping condition:

- complete only when the deploy graph is demonstrably proportional for at least one frontend-only, one host-service-only, and one ordinary guest-impacting push;
- otherwise report `checkpoint_incomplete` with the exact class or cache boundary still too coarse.

## Mission Frame

The real artifact is not "faster CI" in general. It is a deploy graph where unrelated changes do not enter expensive build classes:

```text
changed paths
  -> deploy impact classes
  -> selected Nix roots
  -> copied closures
  -> selected Node B build/switch/install/restart actions
  -> staging health identity and product proof
  -> timing ledger
```

## Core Invariants

```text
GitHub origin/main remains the source of tracked deployed files.
Node B is not edited directly as a source/config shortcut.
Staging health must report the deployed commit after behavior-changing deploys.
Guest images are installed atomically and never truncated in place.
VM image changes restart or notify vmctl; non-image changes do not churn vmctl.
Cache failures are visible evidence, not silently laundered into success.
```

## Baseline Evidence

Use `8335d83` as the baseline deploy timing:

- GitHub Actions CI run: `26353105621`
- `Prebuild and copy Nix deploy closures`: `2026-05-24T05:39:29Z` to `2026-05-24T06:04:39Z`
- `Built 3 deploy roots in 1463s`
- `Copied deploy closures in 47s`
- Node B deploy phase: `2026-05-24T06:04:39Z` to `2026-05-24T06:05:02Z`
- staging `/health` deployed commit: `8335d83f560ba9af4a8fed0d8a15b8c014f01bd3`

## Deploy Impact Classes

The mission should implement explicit classes. Names can change if code clarity improves, but the semantics should remain.

| Class | Typical paths | Build roots | Node B actions |
| --- | --- | --- | --- |
| `none` | `docs/**`, root `*.md`, `.github/**`, deployed-proof artifacts | no deploy | no deploy |
| `frontend` | `frontend/src/**`, frontend package/config files | host NixOS closure, or frontend package if deploy can consume it safely | switch/reload only as required for Caddy frontend root |
| `host_service` | `cmd/auth`, `cmd/proxy`, `cmd/gateway`, `cmd/platformd`, `cmd/vmctl`, host-relevant `internal/**` | host NixOS closure | switch/restart affected services |
| `ordinary_guest` | `cmd/sandbox`, sandbox runtime internals, `nix/sandbox-vm.nix` ordinary image inputs, prompt defaults/skills included in guest | normal guest image plus host closure if vmctl/env contract changes | install `/var/lib/go-choir/guest`, restart or reload vmctl |
| `playwright_guest` | Playwright worker image inputs, browser-worker tooling | Playwright guest image plus host closure if vmctl/env contract changes | install `/var/lib/go-choir/guest-playwright`, restart or reload vmctl |
| `host_os` | `flake.nix`, `flake.lock`, `nix/node-b.nix`, `nix/hardware.nix`, `nix/disks.nix`, service env/hardening/Caddy/systemd changes | host NixOS closure; guest images only if their inputs changed | switch host, health checks, optional guest install by class |

Path heuristics are allowed for v0, but the evidence must show when the heuristic is conservative. Conservative overbuilding is acceptable as a checkpoint; false underbuilding is not.

## Implementation Gradient

### P0: Measure And Expose Classes

Extend `Detect Staging Deploy Impact` so it emits:

```text
deploy_needed
deploy_host
deploy_frontend
deploy_host_service
deploy_ordinary_guest
deploy_playwright_guest
deploy_host_os
deploy_vmctl_restart
changed_paths
class_explanation
```

Print the class explanation in the workflow log. A reviewer should be able to see why a push did or did not build a guest image.

### P1: Build Only Required Roots

Replace the unconditional root list:

```text
.#nixosConfigurations.go-choir-b.config.system.build.toplevel
.#guest-image
.#guest-image-playwright
```

with a selected root set derived from impact classes.

Required behavior:

- if no roots are selected, skip deploy;
- if only host/frontend/service roots are selected, do not build guest images;
- if only ordinary guest changed, do not build Playwright guest image;
- if only Playwright guest changed, do not build ordinary guest image unless shared guest inputs changed;
- copy selected roots through the trusted SSH store path;
- if copy fails, continue only with a visible degraded-cache warning and timing evidence.

### P2: Node B Actions Match Classes

Pass class booleans into the remote deploy script.

Required behavior:

- build only the selected roots on Node B fallback;
- switch host only when the host closure was selected or changed;
- install only selected guest image artifacts;
- restart `go-choir-vmctl` only when a guest image changed or vmctl itself changed;
- restart/reload only necessary services when feasible, but prefer a safe host switch over fragile manual service surgery for v0.

### P3: Reduce Cache Poisoning

Investigate whether commit metadata in `ldflags` and frontend build environment forces unnecessary rebuilds.

Preferred direction:

- keep deployed git identity in `/var/lib/go-choir/deploy.env` and `/health`;
- avoid injecting the current commit into every service binary when the service code did not change;
- if binary-embedded commit remains necessary, explain precisely why and what it costs.

Do not remove staging identity. Move identity to deploy metadata if needed.

### P4: Narrow Nix Source Boundaries

The current Go service derivations share a broad filtered repo source. Improve this only after P0-P2 work.

Possible directions:

- per-service source filters;
- split shared internal packages by service dependency;
- keep `cmd/sandbox` and guest-only assets from invalidating host-only services when possible;
- keep frontend and Go build caches independent.

Do not overfit a brittle path list that risks missing real shared dependencies. Prefer a conservative v0 with explicit TODOs over a false-fast deploy.

## Verification Plan

Run at least these deployed pushes or workflow-dispatch equivalents:

1. **Frontend-only proof**
   - touch a harmless frontend UI comment or text;
   - expected: no guest image build, no guest image install, no vmctl restart unless host closure requires it;
   - evidence: timing and staging `/health`.

2. **Host-service-only proof**
   - touch a harmless gateway/provider/proxy comment;
   - expected: host closure only, no guest image build;
   - evidence: timing and staging `/health`.

3. **Ordinary guest-impact proof**
   - touch a harmless sandbox-only comment or prompt default;
   - expected: ordinary guest image path included; Playwright guest omitted unless shared image input forces it;
   - evidence: timing, guest install log, vmctl restart/reload log.

4. **Playwright guest-impact proof**
   - if feasible, touch a Playwright-worker-only input;
   - expected: Playwright guest image included without ordinary guest image unless shared input forces it.

5. **Host OS proof**
   - if feasible and safe, a workflow-only simulation or small Nix host config comment may prove class detection;
   - do not mutate host OS just to satisfy a checkbox if the earlier proofs establish the deploy graph.

## Dense Feedback

Every deploy run should print:

```text
changed paths
impact classes
selected Nix roots
build time per root when available
copy time
Node B build fallback used: yes/no
Node B switch time
guest image install time
vmctl restart/reload: yes/no and why
staging deployed_commit
```

## Forbidden Shortcuts

- Do not skip staging identity proof.
- Do not remove guest image install when the guest image really changed.
- Do not hide failed prebuild/copy behind a successful slow remote rebuild.
- Do not mark a run "fast" if it simply skipped a required deploy.
- Do not use direct Node B tracked-file edits.
- Do not weaken CI tests or docs-only path filters to create the appearance of speed.
- Do not replace image-specific logic with a vague "deploy all" fallback as the normal path.

## Rollback

Rollback refs:

- revert the deploy-impact workflow changes;
- revert any Nix source-filter changes;
- restore unconditional root build/install if class detection underbuilds;
- Node B can deploy a prior `origin/main` commit through the same workflow after revert.

Any suspected underbuild must favor rollback or conservative overbuild before further optimization.

## Completion Criteria

The mission is `complete` only when:

- deploy-impact classes are implemented and logged;
- selected roots are built/copied instead of unconditional host+two guest roots;
- Node B installs/restarts only selected image/service classes;
- staging health reports the expected deployed commit after proof pushes;
- timing evidence shows at least one non-guest deploy avoiding MicroVM EROFS build;
- a guest-impacting proof still correctly builds/installs the needed guest image;
- residual risks and next optimization axis are documented.

If only class detection lands but selected roots remain unconditional, report `checkpoint_incomplete`.

If selected roots work but proof coverage is missing, report `checkpoint_incomplete`.

If class detection risks underbuilding and cannot be made conservative inside the mission, report `blocked_incomplete` with the exact ambiguous path/dependency and the next safe probe.

## Suggested Resume Goal String

```text
/goal Run docs/mission-deploy-impact-classes-cache-v0.md as a Codex-operated MissionGradient mission: make Choir's staging deploy graph proportional to deploy impact. Starting from the proven trusted-copy baseline at 8335d83, split deploy-impact detection into host/frontend/service/ordinary-guest/playwright-guest/host-OS classes, build and copy only the required Nix roots, install/restart only the affected services or guest images, and preserve staging identity and rollback discipline. Optimize cache boundaries so frontend or host-service-only changes do not rebuild MicroVM EROFS guest images, and guest-image changes do not force unrelated service work beyond what the NixOS closure requires. Do not weaken CI evidence, skip required health checks, hide copy failures behind remote rebuilds, rebuild all images by default, or claim success without timing evidence from at least frontend-only, host-service-only, and guest-image-impacting pushes or precise blockers. Finish with CI/deploy timing deltas, staging health identity, rollback refs, residual cache risks, and the next deploy-speed realism axis.
```
