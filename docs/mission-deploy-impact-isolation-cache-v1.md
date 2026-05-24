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

## Execution Checkpoint: 2026-05-24

**Status:** checkpoint_complete_for_impact_isolation, cache_not_worth_paying_yet.

The deploy graph is now substantially proportional for the three measured
classes. The remaining long pole is ordinary guest image construction, not broad
host activation.

### Landed Commits

| Commit | Purpose | Result |
| --- | --- | --- |
| `23d8278` | initial v1 mission doc and frontend selected-root path | set up frontend proof |
| `d109645` | serve frontend from stable directory | one-time host cutover |
| `c2e66fe` | frontend-only proof commit | proved frontend package-only deploy |
| `88e213b` | health identity from live deploy metadata | fixed stale health identity |
| `a9c792f` | split Go service build inputs and remove commit poisoning | reduced unrelated Go rebuilds |
| `bbd3d6f` | gateway-only proof before service pointers | proved no guests, but still host closure |
| `8f4dd54` | service pointer deploy path | one-time host cutover |
| `ebdc51c` | gateway-only proof after service pointers | proved service package-only deploy |
| `9b67d53` | compose service pointer deploys with guest images | enabled `sandbox + guest-image` without host OS |
| `4aa75ce` | ordinary guest proof commit | proved ordinary guest-only plus sandbox pointer deploy |

### Frontend-Only Evidence

Proof commit: `c2e66fe`.

- selected root: `.#frontend`
- GitHub prebuild: `28s`
- copy to Node B: `2s`
- no host NixOS closure build
- no NixOS switch
- no ordinary guest image build
- no Playwright guest image build
- no vmctl restart
- frontend asset graph served from `/var/www/go-choir/frontend-current`

The first attempt used `/srv/go-choir/frontend-current` and produced 404s; the
durable path is now `/var/www/go-choir/frontend-current`.

### Host-Service-Only Evidence

Proof commit before service pointers: `bbd3d6f`.

- selected root: host NixOS closure only
- no ordinary guest image
- no Playwright guest image
- no vmctl restart
- still built a broad host NixOS closure and all host service closure references
- deploy job about `8m28s`

Proof commit after service pointers: `ebdc51c`.

- selected root: `.#gateway`
- `DEPLOY_HOST_OS=false`
- `HOST_SERVICES=gateway`
- GitHub prebuild: `188s`
- copy to Node B: `5s`
- remote deploy: host service build `1s`, no NixOS switch, restart only `go-choir-gateway.service`
- no ordinary guest image
- no Playwright guest image
- no vmctl restart
- `/health` reported deployed commit `ebdc51ceada0ce49179b6b5be0f0472fe55efa97`

This proves the service pointer path works. The remaining host-service cost is
the GitHub-side package build, not host activation.

### Ordinary Guest Evidence

Proof commit: `4aa75ce`.

Classifier output for `cmd/sandbox/main.go`:

```text
deploy_host=true
deploy_frontend=false
deploy_host_service=true
deploy_ordinary_guest=true
deploy_playwright_guest=false
deploy_host_os=false
deploy_vmctl_restart=true
host_services=sandbox
```

First run:

- selected roots: `.#sandbox`, `.#guest-image`
- GitHub prebuild: `750s`
- `microvm-store-disk.erofs`: `2m16s`
- copy to Node B: `45s`
- remote host service sandbox build: `1s`
- remote guest image build from copied closure: `5s`
- skipped frontend bundle
- skipped host NixOS closure and NixOS switch
- installed sandbox service pointer at `/var/lib/go-choir/services/sandbox`
- restarted `go-choir-sandbox.service`
- installed ordinary guest image
- skipped Playwright guest image build/install
- restarted `go-choir-vmctl` because ordinary guest image changed
- `/health` reported deployed commit `4aa75cee112d0843a9047fcccc5a5be4d79fe937`

This proves ordinary guest deploys no longer require host NixOS activation or
the Playwright guest image when the change is sandbox-specific.

### Cache Repeat Evidence

The same `4aa75ce` ordinary guest run was rerun without a code change.

- selected roots remained `.#sandbox`, `.#guest-image`
- GitHub prebuild: `836s`, slower than the first run
- `microvm-store-disk.erofs`: `2m59s`, slower than the first run
- copy to Node B: `1s`
- remote deploy remained cheap: sandbox build `0s`, guest image build `1s`, no NixOS switch, Playwright skipped
- Magic Nix Cache emitted FlakeHub 401 errors:
  `Failed to auth to FlakeHub ... User is not authorized for this resource`
- post-job log: `FlakeHub cache is not enabled, not uploading anything to it`

Conclusion: do not pay for FlakeHub Cache yet on this evidence. The current
free Magic Nix Cache setup did not materially improve ordinary guest rebuilds
and added noisy FlakeHub authentication warnings. The workflow now removes Magic
Nix Cache until a paid cache or a narrower guest-image build graph gives it a
clear target.

### Rollback Refs

- Frontend pointer path rollback: revert `d109645` and `c2e66fe` shape back to
  embedded Caddy `${goChoirPackages.frontend}`.
- Service pointer path rollback: revert `8f4dd54` and use host NixOS closure
  activation for host services.
- Sandbox plus guest composition rollback: revert `9b67d53` to send sandbox
  changes through host NixOS closure while keeping ordinary guest image updates.
- Go identity split rollback: revert `a9c792f` and restore commit/date ldflags
  in service binaries.

### Residual Risks

- Guest image construction remains expensive even when correctly selected.
- `internal/runtime`, `internal/store`, `internal/types`, `internal/events`,
  `internal/search`, and `internal/server` still conservatively select host
  closure plus ordinary guest because they are shared enough that underbuilding
  would be worse than overbuilding.
- Only `gateway` and `sandbox` have been exercised through service pointers in
  deployed proof. Other host services should get one focused proof each when
  next touched.
- Service pointer rollback keeps `*-previous` directories but does not yet expose
  a first-class rollback command in CI.
- GitHub Actions still runs broad Go/frontend test jobs for all non-doc changes;
  this mission optimized deploy activation, not CI test selection.
- Node.js 20 action deprecation warnings remain.

### Next Deploy-Speed Realism Axis

The next useful mission should target guest-image rebuild cost directly:

```text
Make ordinary and Playwright guest images compositional so sandbox binary,
skills, Obscura, and base OS/store layers do not force the whole microvm
store-disk EROFS path on every sandbox change. Measure whether a paid binary
cache helps only after the guest image layers are split enough to generate
stable reusable artifacts.
```

### Follow-On Guest Image Builder Experiment

After the cache repeat showed no useful Magic Nix Cache improvement, the next
low-risk lever is the microvm store-disk builder itself. Upstream microvm.nix
defaults to EROFS flags that include fragments and dedupe on newer kernels; that
chooses the single-threaded `mkfs.erofs` path. The follow-on change keeps EROFS
but sets the guest images to fast LZ4-only flags so the builder can use the
multithread-capable tool.

Expected tradeoff: somewhat larger guest store disks, potentially much faster
`microvm-store-disk.erofs` builds. Acceptance is comparative timing from the
next full guest-image deploy, not local proof.

### Guest EROFS Evidence

Commit `278a9ba020f4e6c2fad1672363df64936781bfa1` set ordinary and
Playwright guest images to explicit fast LZ4 EROFS flags.

Deployed CI run: `26359636428`.

- selected deploy classes: ordinary guest, Playwright guest, vmctl restart
- GitHub runner prebuild: `521s`
- GitHub runner closure copy: `163s`
- ordinary `microvm-store-disk.erofs`: `50s`
- Playwright `microvm-store-disk.erofs`: `53s`
- Node B remote build/install/restart/health: `18s`
- Node B deployed commit identity: `278a9ba020f4e6c2fad1672363df64936781bfa1`

The EROFS flag change helped the store-disk builder, but the deploy loop is now
dominated by ephemeral-runner custom derivation rebuilds and copying the guest
image outputs to Node B. The next deploy-loop change should let guest images
build directly on Node B, whose Nix store persists across runs, while keeping
runner prebuild/copy for frontend, host services, and host OS closures.

### Node B Guest Build Evidence

Commit `8b3815916757969f64639e8b0f6d3cec3443c024` changed CI so guest-image
deploy classes skip GitHub runner prebuild/copy and build directly on Node B.
It also moved the remaining frontend actions to Node 24-capable majors and
updated the deployed checkout remote to `choir-hip/go-choir`.

Deployed CI run: `26360101028`.

- selected deploy classes: ordinary guest, Playwright guest, vmctl restart
- GitHub runner prebuild/copy: skipped with `No deploy roots selected`
- Node B ordinary guest build: `49s`
- Node B Playwright guest build: `46s`
- Node B nix build phase total: `95s`
- Node B deploy through health and asset graph: `112s`
- full deploy job: about `2m13s`
- staging health after deploy: `ok`
- deployed commit identity: `8b3815916757969f64639e8b0f6d3cec3443c024`
- FlakeHub rolling publish workflow completed successfully for this commit,
  but the deploy speedup here does not depend on paid FlakeHub cache.

Conclusion: for guest-image deploys, building on Node B from its persistent
Nix store is currently much faster than building and copying full image outputs
from ephemeral GitHub runners. Paid cache may still help for custom derivations,
but it is no longer the first lever for this deploy path.
