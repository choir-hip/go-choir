# MissionGradient: Deploy Impact Isolation And Cache v1

**Status:** ready_for_execution  
**Date:** 2026-05-24  
**Purpose:** make staging deploys proportional to the artifact that changed. Cache is useful only after the deploy graph stops selecting broad roots unnecessarily.

## One-Line Goal String

```text
/goal Run docs/mission-deploy-impact-isolation-cache-v1.md as a Codex-operated MissionGradient mission: optimize Choir's staging deploy path by isolating deploy impact classes before judging cache. Starting from the partial v0 proof at 2568564, preserve the deployed class detector and selected guest-root behavior, then remove the remaining broad-host tax: make frontend-only changes deploy by building/copying only the frontend package, updating a durable frontend pointer, reloading only the edge path required, and proving no NixOS switch, no vmctl restart, and no guest EROFS build. Then split host-service changes so ordinary service edits do not rebuild unrelated services or guest images unless shared dependencies require it, while still preserving staging /health commit identity through deploy metadata rather than binary-wide commit poisoning where feasible. Measure Magic Nix Cache/FlakeHub behavior only after the selected roots are correct. Do not edit Node B directly as source/config, skip health identity, hide fallback remote builds, underbuild ambiguous shared dependencies, or call a checkpoint complete. Finish with frontend-only, host-service-only, ordinary-guest, and cache-hit timing evidence, rollback refs, residual risks, and the next deploy-speed realism axis.
```

## Cognitive Transform Review

### Current Uncertainty Or Obstacle

The v0 deploy-impact classifier proved one important thing and exposed the next bottleneck.

What worked:

- docs and workflow-only changes can skip deploy;
- frontend-only changes skipped ordinary and Playwright guest images;
- frontend-only changes skipped the explicit `go-choir-vmctl` restart in the deploy script;
- GitHub-built closures copied to Node B through the trusted SSH path.

What did not work yet:

- frontend-only changes still selected the full host NixOS closure because Caddy embeds `${goChoirPackages.frontend}` directly in generated config;
- a host NixOS switch still restarted `go-choir-vmctl` and other host services even though the explicit vmctl restart gate was false;
- the host closure still rebuilt broad Go service artifacts because shared repo source and binary-embedded commit metadata poison cache boundaries;
- cache is hard to evaluate while the wrong roots are still selected.

### Selected Transforms

1. **Activation Boundary Transform**  
   The expensive boundary is not just "build" vs "cache." It is "does this change require NixOS activation?" A frontend bundle should be a runtime-served artifact, not a host OS configuration change.

2. **Pointer-Indirection Transform**  
   If Caddy points at an immutable Nix store frontend path, every frontend build changes the Caddy config closure. If Caddy points at a stable product-owned path, deploy can atomically move that pointer without switching the host.

3. **Identity Decoupling Transform**  
   Staging identity is necessary, but binary-wide commit injection turns every commit into every binary changing. Keep `/health` identity through deploy metadata and only embed binary identity where it is genuinely useful.

4. **Negative-Proof Transform**  
   The proof is not "deploy succeeded." The proof is the absence of the wrong work: no guest image build, no `microvm-store-disk.erofs`, no NixOS switch, no vmctl restart for frontend-only changes.

5. **Cache-After-Graph Transform**  
   Paid or free cache can only optimize selected work. First make selected work correct. Then measure cache hits/misses for the remaining selected roots.

### Route-Changing Insights

- The next mission should not continue polishing the class detector first. It should change the deploy topology so frontend can be deployed outside the NixOS system closure.
- The first deploy after this topology change may legitimately be a host deploy because Caddy configuration must change once. The acceptance proof must be a subsequent frontend-only deploy.
- Host-service-only optimization is a different layer: per-service source filters, per-service build metadata, and service-specific restart policy. It should follow frontend pointer isolation, not block it.
- Guest-image impact proof remains essential because a fast path that cannot safely update guest images is worse than a slow path.
- Cache should be measured with impact-class labels in the log. "Magic Nix Cache made it faster" is too vague unless we know which selected root hit or missed.

## Real Artifact

A deployed staging pipeline with four separable artifact classes:

```text
frontend bundle
host service binaries/config
ordinary guest image
playwright guest image
```

Each class should select only its required build roots, copy only those roots, activate only the required runtime surface, and prove staging identity afterward.

## Invariants

- `origin/main` remains the source of tracked deployed files.
- Node B is not edited directly as a source/config shortcut.
- `/health` must report the deployed commit after behavior-changing deploys.
- Frontend asset graph proof must show the served index points at the deployed frontend artifact.
- Guest image updates remain atomic and must restart or reload vmctl when the active VM image changed.
- Ambiguous shared dependencies may conservatively overbuild, but must log why.
- Cache miss or remote fallback is evidence, not success theater.

## Baseline Evidence

Use these measurements as the current state:

- `8335d83`: pre-v0 deploy baseline built host plus both guest roots; build roots took about 1463s, copy 47s, deploy phase about 22s.
- `2568564`: frontend-only proof after v0 selected only the host NixOS root and skipped both guest roots.
  - CI/deploy run completed successfully.
  - Deploy job took about 8m21s.
  - Prebuild built one deploy root in about 445s and copied in about 7s.
  - Logs showed ordinary guest and Playwright guest builds/installs skipped.
  - Logs showed explicit vmctl restart skipped.
  - But NixOS switch still stopped/started host services including `go-choir-vmctl`.

This is `checkpoint_incomplete`, not success.

## Optimization Axes

### P0: Preserve v0 Class Detection

Keep the existing detector behavior unless evidence shows it underbuilds.

Required outputs:

```text
deploy_needed
deploy_frontend
deploy_host_service
deploy_ordinary_guest
deploy_playwright_guest
deploy_host_os
deploy_vmctl_restart
class_explanation
```

### P1: Frontend Pointer Isolation

Change the deployed frontend path from a Caddy-config-embedded Nix store path to a stable product-owned pointer, for example:

```text
/var/lib/go-choir/frontend-current -> /nix/store/...-go-choir-frontend
```

Expected route:

- expose or reuse `.#frontend` as a buildable package root;
- configure Caddy through NixOS to serve `/var/lib/go-choir/frontend-current`;
- install an initial pointer safely during the one-time host deploy;
- for future frontend-only deploys, build/copy only the frontend package, atomically replace the pointer, reload Caddy only if required, update deploy metadata, and run frontend asset graph proof;
- do not run `switch-to-configuration` for frontend-only changes after the one-time cutover.

Acceptance proof must include a second frontend-only commit after the cutover.

### P2: Host-Service Source And Identity Isolation

Reduce broad host-service rebuilds after frontend isolation is proven.

Investigate and patch where safe:

- broad `commonGoArgs.src` causing every Go service to observe unrelated Go/source changes;
- `buildCommit` and `buildDate` in every Go binary forcing rebuilds on every commit;
- service-specific restart behavior after host-service-only deploys.

Preferred shape:

- `/health` and staging deploy identity read canonical deploy metadata from `/var/lib/go-choir/deploy.env`;
- binaries report build identity for their own artifact when useful, but do not need every commit embedded into every service;
- service derivations depend on their command package plus the internal packages and assets they actually import, with conservative shared-dependency fallbacks.

Do not risk underbuilding dynamic skill/prompt/runtime assets. If dependency analysis is ambiguous, log the conservative boundary and continue.

### P3: Guest Image Class Proof

Re-run ordinary guest impact proof after P1/P2 so frontend/host optimizations do not regress guest deploy safety.

Expected:

- ordinary guest change selects `.#guest-image`;
- Playwright guest omitted unless a shared image input changed;
- guest artifacts install atomically;
- vmctl restart/reload is explicit and justified;
- staging health remains correct.

### P4: Cache Measurement After Graph Separation

Once selected roots are correct, compare:

- cold selected-root build;
- repeated no-op or comment-only selected-root build;
- Magic Nix Cache behavior;
- FlakeHub cache behavior if available and authorized.

Record cache as a timing ledger, not as the proof itself.

## Dense Evidence Required

Every deploy proof should record:

```text
commit SHA
changed paths
impact class outputs
selected roots
whether GitHub prebuild succeeded
whether copy succeeded
whether Node B fallback build ran
NixOS switch: yes/no and why
Caddy reload/restart: yes/no and why
service restarts: names and why
guest image builds/installs: yes/no by class
vmctl restart/reload: yes/no and why
staging /health deployed_commit
frontend asset root/path when applicable
duration by phase
```

## Verification Matrix

| Proof | Expected behavior | Completion signal |
| --- | --- | --- |
| one-time frontend pointer cutover | host deploy allowed | Caddy serves stable frontend pointer and health identity is correct |
| frontend-only after cutover | frontend package only | no NixOS switch, no vmctl restart, no guest EROFS, frontend asset graph current |
| host-service-only | host service roots only | no guest EROFS; restarts limited or conservative reason logged |
| ordinary guest | ordinary guest image only where possible | ordinary image installed atomically; vmctl reload/restart justified |
| cache repeat | same selected class | timing shows hit/miss source and remaining misses |

## Forbidden Shortcuts

- Do not claim frontend isolation from the one-time Caddy cutover deploy; prove the subsequent frontend-only deploy.
- Do not point Caddy at a mutable working tree.
- Do not manually patch Node B tracked config or service files.
- Do not remove staging identity to make binaries more cacheable.
- Do not skip public frontend asset graph proof.
- Do not treat a remote fallback build as equivalent to a cache hit.
- Do not remove guest-image safety checks.
- Do not weaken docs-only CI/deploy skips.

## Rollback

Rollback refs:

- revert frontend pointer/Caddy changes to embedded `${goChoirPackages.frontend}`;
- revert deploy workflow changes to the v0 selected-root behavior;
- restore unconditional host switch for frontend if pointer deploy underbuilds;
- restore broad Go service source or binary commit metadata if source isolation underbuilds;
- deploy a prior known-good `origin/main` through GitHub Actions.

If pointer deployment serves mismatched assets or stale index content, immediately roll back to embedded frontend root before continuing optimization.

## Completion Criteria

The mission is `complete` only when:

- frontend pointer isolation is deployed;
- a subsequent frontend-only commit deploys without NixOS switch, vmctl restart, or guest image build;
- staging `/health` reports the expected commit after that frontend-only deploy;
- frontend asset graph proof shows the served app belongs to the deployed frontend artifact;
- host-service-only proof avoids guest image work and reports any remaining host overbuild precisely;
- ordinary guest proof still builds/installs the needed guest image and restarts/reloads vmctl for a named reason;
- cache timing is measured after graph separation;
- mission doc is updated with evidence, rollback refs, residual risks, and next deploy-speed axis.

Use `checkpoint_incomplete` if only some artifact classes are isolated.

Use `blocked_incomplete` if Nix/Caddy/systemd constraints prevent frontend pointer isolation after root-cause probes, and name the next executable probe.

## Suggested Resume Goal String

```text
/goal Run docs/mission-deploy-impact-isolation-cache-v1.md as a Codex-operated MissionGradient mission: optimize Choir's staging deploy path by isolating deploy impact classes before judging cache. Starting from the partial v0 proof at 2568564, preserve the deployed class detector and selected guest-root behavior, then remove the remaining broad-host tax: make frontend-only changes deploy by building/copying only the frontend package, updating a durable frontend pointer, reloading only the edge path required, and proving no NixOS switch, no vmctl restart, and no guest EROFS build. Then split host-service changes so ordinary service edits do not rebuild unrelated services or guest images unless shared dependencies require it, while still preserving staging /health commit identity through deploy metadata rather than binary-wide commit poisoning where feasible. Measure Magic Nix Cache/FlakeHub behavior only after the selected roots are correct. Do not edit Node B directly as source/config, skip health identity, hide fallback remote builds, underbuild ambiguous shared dependencies, or call a checkpoint complete. Finish with frontend-only, host-service-only, ordinary-guest, and cache-hit timing evidence, rollback refs, residual risks, and the next deploy-speed realism axis.
```
