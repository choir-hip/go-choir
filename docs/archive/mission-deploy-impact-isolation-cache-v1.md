# MissionGradient: Deploy Impact Isolation And Cache v1

**Status:** checkpoint_complete_for_service_pointer_deploys  
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
- Node.js 20 action deprecation warnings were removed by moving workflow
  checkout actions to Node 24-capable versions and setting
  `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24=true`.

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
build directly on Node B, whose Nix store persists across runs.

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

### Sandbox Model Policy Deploy Evidence

Commit `0c8743e7bc9f3a29de7ccbe82949e6b474dfc54a` moved generated and
fallback foreground agent defaults off ChatGPT and onto the available
Fireworks models:

- `conductor`: `accounts/fireworks/models/deepseek-v4-flash`
- `super`, `vsuper`, `co-super`: `accounts/fireworks/models/deepseek-v4-pro`
- `researcher` and `vtext`: `accounts/fireworks/models/deepseek-v4-flash`
- `verifier_multimodal`: `accounts/fireworks/models/kimi-k2p6`

It also taught the deploy-impact classifier that
`internal/runtime/model_policy.go` is a sandbox service plus ordinary guest
change, not a broad host OS change.

Deployed CI run: `26360311083`.

- selected deploy classes: sandbox host service, ordinary guest, vmctl restart
- GitHub runner selected root: `.#sandbox`
- GitHub runner sandbox prebuild: `181s`
- GitHub runner closure copy: `8s`
- Node B host service sandbox build from copied closure: `1s`
- Node B ordinary guest build: `33s`
- Node B deploy through health and asset graph: `43s`
- skipped frontend bundle, host NixOS closure, NixOS switch, and Playwright
  guest image
- staging health after deploy: `ok`
- deployed commit identity: `0c8743e7bc9f3a29de7ccbe82949e6b474dfc54a`

Conclusion: service-pointer plus guest-image deploys now have the opposite
problem from the original pipeline. Remote activation is fast, but the
ephemeral runner still spends minutes rebuilding service roots that Node B can
build from its persistent Nix store. The next patch skips GitHub runner
prebuild/copy for host service roots as well as guest-image roots; frontend and
full host OS roots remain eligible for runner prebuild/copy until measured
otherwise.

### Node B Service Root Build Evidence

Commit `ac7cc3eee1614656f660a069cf11ca126f04cf6f` exercised the same
sandbox-service plus ordinary-guest class after CI stopped prebuilding service
roots on the GitHub runner.

Deployed CI run: `26360488587`.

- selected deploy classes: sandbox host service, ordinary guest, vmctl restart
- GitHub runner prebuild/copy: skipped with `No deploy roots selected`
- Node B host service sandbox build: `65s`
- Node B ordinary guest build: `36s`
- Node B nix build phase total: `101s`
- Node B deploy through health and asset graph: `109s`
- full deploy job: about `2m11s`
- skipped frontend bundle, host NixOS closure, NixOS switch, and Playwright
  guest image
- staging health after deploy: `ok`
- deployed commit identity: `ac7cc3eee1614656f660a069cf11ca126f04cf6f`

Conclusion: service-root prebuild/copy on the GitHub runner was counterproductive
for this class. The previous comparable run spent `181s` building `.#sandbox`
and `8s` copying before Node B deploy began. The new run moved that work onto
Node B and reduced the end-to-end deploy job to about two minutes. The remaining
cost is the real sandbox package rebuild plus EROFS guest image construction.
The next deploy-speed axis is not paid cache first; it is reducing or layering
the sandbox/guest build itself while keeping the impact classifier conservative.

### Trace Live Update Deploy Evidence

Commit `d76cf8481bc46398f7476b57b58b3dc9c2529eb3` shipped the first queued
regression fixes on top of the deploy-speed work:

- Trace subscribes to `/api/ws` events, refreshes the trajectory list when a
  new trajectory appears, and refreshes the selected trajectory when matching
  events arrive.
- Trace exposes a product route,
  `GET /api/trace/trajectories/{id}/logs`, and a `Copy logs` button for
  owner-debuggable trajectory text.
- The wide Shelf lets minimized app buttons consume the left side while the
  prompt bar stays right-aligned.
- Podcast progress reporting no longer references an undefined playback-rate
  variable.

Deployed CI run: `26360793813`.

- selected deploy classes: frontend bundle, sandbox host service, ordinary
  guest, vmctl restart
- GitHub runner prebuild/copy: skipped with `No deploy roots selected`
- Node B frontend bundle build: `1s`
- Node B host service sandbox build: `66s`
- Node B ordinary guest build: `35s`
- Node B nix build phase total: `102s`
- Node B deploy through health and asset graph: `111s`
- full deploy job: `2m46s`
- skipped host NixOS closure, NixOS switch, and Playwright guest image
- staging health after deploy: `ok`
- deployed commit identity: `d76cf8481bc46398f7476b57b58b3dc9c2529eb3`

Conclusion: a combined frontend plus sandbox plus ordinary-guest deploy now
lands in under three minutes once CI gates pass. The deploy job still spends
almost all of its time building the sandbox package and rebuilding the ordinary
guest image, so the next structural speedup should target guest layering or
decoupling guest images from ordinary sandbox-service changes.

### Active Computer Refresh Checkpoint

After the deploy-speed work, staging exposed a correctness issue that also
affects deploy confidence: active interactive VMs survived vmctl restarts and
kept running stale sandbox builds after a new ordinary guest image deployed.
That made user computers show mixed behavior: the host sandbox/proxy was on the
latest commit, but warm user VMs still routed conductor through older ChatGPT
defaults while VText/researcher used the newer Fireworks path.

The next deploy patch therefore adds a deploy-time active-computer refresh
phase for ordinary guest image changes:

- deploy installs the ordinary guest image;
- deploy restarts vmctl;
- deploy lists active interactive computers through internal vmctl;
- each active interactive computer is force-rebooted onto the deployed guest
  image with persistent data preserved and a new epoch;
- stopped/hibernated computers are left alone because their next resume boots
  from the current image.

This preserves the fast no-host-OS deploy class while ensuring warm user
computers do not continue serving stale runtime code after a guest image deploy.
The remaining structural speed axis is still guest layering/decoupling, but
correct code identity for active computers is now an invariant of the deploy
loop.

### Active Computer Refresh Deploy Evidence

Commit `336ba6b9a29e764fa4395c93e81f03403b78d761` landed the active
interactive computer refresh path and the semantic generated-model-policy
migration away from stale ChatGPT foreground defaults.

Deployed CI run: `26361582906`.

- selected deploy classes: sandbox host service, ordinary guest, vmctl restart
- GitHub runner prebuild/copy: skipped with `No deploy roots selected`
- Node B sandbox host service build: `70s`
- Node B ordinary guest build: `37s`
- Node B nix build phase total: `107s`
- active interactive computer refresh phase: `24s`
- full deploy job: about `3m20s`
- skipped frontend bundle, host NixOS closure, NixOS switch, and Playwright
  guest image
- staging health after deploy: `ok`
- deployed commit identity: `336ba6b9a29e764fa4395c93e81f03403b78d761`

This fixed a real product correctness problem: warm primary computers must not
continue serving old guest runtime code after a guest image deploy.

Commit `9e51b7400fc88b345c6b14d920e4d76ca145370c` then stamped
`CHOIR_DEPLOYED_COMMIT` into the guest sandbox environment so VM health can
report the guest image identity directly.

Deployed CI run: `26361827566`.

- selected deploy classes: ordinary guest, vmctl restart
- Node B ordinary guest build: `54s`
- Node B nix build phase total: `54s`
- active interactive computer refresh phase: `13s`
- full deploy job: about `2m03s`
- active VM health showed `build.commit` and `deployed_commit` both at
  `9e51b7400fc88b345c6b14d920e4d76ca145370c`

### Concurrent Guest Build Evidence

Commit `41a226332e43f78dd843cc272007267b648313c5` changed the remote deploy
script to build ordinary and Playwright guest image roots concurrently when both
are selected.

Deployed CI run: `26361945573`.

- selected deploy classes: ordinary guest, Playwright guest, vmctl restart
- ordinary guest image build: `43s`
- Playwright guest image build: `40s`
- Node B nix build phase total: `43s`
- active interactive computer refresh phase: `13s`
- full deploy job: about `1m27s`
- skipped frontend bundle, host service pointer work, host NixOS closure, and
  NixOS switch
- staging health after deploy: `ok`
- deployed commit identity: `41a226332e43f78dd843cc272007267b648313c5`

This is the current best measured path for a guest-image-only deploy that
touches both image classes.

### Podcast Persistence And Runtime Shared-Dependency Evidence

Commit `255bd1b4ee1ceaa944ac655d1e088211c58b5c86` fixed Podcast subscription
persistence and RSS refresh behavior by adding durable owner-scoped
`podcast_subscriptions` records plus product API routes.

Deployed CI run: `26362395479`.

- selected deploy classes before the classifier fix: frontend, ordinary guest,
  vmctl restart, and broad host deployment
- GitHub runner selected roots: `.#frontend` plus the full
  `.#nixosConfigurations.go-choir-b.config.system.build.toplevel`
- GitHub prebuild: `463s`
- full deploy job: about `9m20s`
- staging health after deploy: `ok`
- deployed commit identity: `255bd1b4ee1ceaa944ac655d1e088211c58b5c86`

This was a regression in deploy proportionality, not in product behavior. The
runtime/store/types changes were conservatively treated as broad host closure
changes even though the service-pointer path could carry them.

Commit `9f0d7c4d1c71749884163d287f1b6c26ff209e0d` corrected the classifier so
shared runtime/store/type/event/server/search changes select service pointers
and ordinary guest images instead of the full host OS closure when possible.

Commit `4d8a89a8727f2fc0fc6a0ada5d5689e948e1c7d` then made selected remote
builds concurrent across frontend, service roots, and guest images.

Commit `f096c000eed77bcf46c5f34e16ca6a570dfbd0c2` exercised that path with an
`internal/runtime/podcast.go` comment-only change.

Deployed CI run: `26362845823`.

- selected deploy classes: `HOST_SERVICES=gateway,sandbox`, ordinary guest,
  vmctl restart
- skipped frontend bundle
- skipped host NixOS closure and NixOS switch
- skipped Playwright guest image
- Node B gateway service build: `110s`
- Node B sandbox service build: `111s`
- Node B ordinary guest build: `144s`
- Node B nix build phase total: `144s` because service and guest roots built
  concurrently
- host service install/restart phase: `3s`
- active interactive computer refresh phase: `18s`
- remote deploy total through health and asset graph: `174s`
- full deploy job: `3m19s`
- staging health after deploy: `ok`
- proxy and upstream sandbox health reported deployed commit
  `f096c000eed77bcf46c5f34e16ca6a570dfbd0c2`

Conclusion: the full-host tax is gone for this class. The remaining cost is
real service package rebuild plus ordinary guest image construction.

### Current Bottleneck Analysis

The flake still gives every Go service a source tree containing all
`internal/**/*.go`. More importantly, `cmd/gateway` imports `internal/provider`,
and `internal/provider` imports `internal/runtime` for bridge interfaces and
tool-loop types. That means a runtime package change legitimately changes the
gateway derivation today, even when the touched runtime file is only meaningful
inside sandbox behavior.

There are two separable next axes:

1. Narrow each Go service source to the internal package directories it imports
   so unrelated internal packages do not poison service derivations.
2. Split provider/runtime bridge interfaces into a small shared package so the
   gateway does not depend on the full runtime package.

Do not solve this by pretending runtime changes cannot affect gateway. The
right fix is dependency graph surgery plus deployed timing proof.

### Classifier And Per-Service Source Checkpoint

Commit `80cfba46b9866e3551490447fb3fe41207ca06ba` corrected several impact
classification boundaries before deeper Nix work:

- `internal/provider`, `internal/gateway`, `internal/modelcatalog`,
  `internal/sandbox`, and shared runtime/store/type/event/server/search changes
  select the gateway and sandbox service pointers plus the ordinary guest image.
- `internal/platform` selects `corpusd` and `proxy`.
- `internal/vmctl` selects `gateway`, `proxy`, `sandbox`, and `vmctl`, plus the
  ordinary guest image and a vmctl restart.
- `internal/buildinfo` selects host service pointers plus ordinary guest, not a
  full host OS switch.

Commit `91bcc78f137db601d21871d4492d790f9be4777d` narrowed the Nix source
filter for each Go service to the internal package directories it actually
needs, with conservative shared dependencies preserved. This made per-service
pointer builds hashable independently instead of every service observing every
`internal/**/*.go` change.

The first full-flake deployment after narrowing exposed fixed-output
`vendorHash` drift for the per-service Go modules. These were pinned in small
commits:

| Commit | Service | `vendorHash` |
| --- | --- | --- |
| `e7b98b9` | `auth` | `sha256-5lI1eHUCgp1pIEAQxrMXGlZTdGy9l/fIyElT1FilUWA=` |
| `62266ad` | `gateway`, `sandbox` | `sha256-dcaVDKz/yHrr173nTDgVffcuD2rtjEx418J5VcZ7br0=` |
| `17d3ba4` | `corpusd` | `sha256-LHIXwcHctefXm9MrSfqWB/4O+p8HXQi0VDT4NXt9xlg=` |
| `1a86be0` | `proxy` | `sha256-+qN6OZMZuzyZeCmwdnQyzH3teNOY/ChJP1yRsEEiULQ=` |
| `52d64ee` | `vmctl` | `sha256-Zi7CIbMdCmTj2ZhP0J+kNARQAG24v/88KlN5l3S7urE=` |

The important result is not the hashes themselves. It is that the flake can now
evaluate and build narrowed per-service modules without falling back to an
untrusted or mismatched derivation path.

### Clean Full-Flake Deploy Evidence

Commit `52d64ee01b4f7e7f43cf035b7e603191af3968ba` was the first clean deploy
after all per-service hashes were pinned.

- CI run: `26364009817`
- deploy job: `77604584364`
- selected roots on the GitHub runner: `.#frontend` and
  `.#nixosConfigurations.go-choir-b.config.system.build.toplevel`
- no per-service hash mismatch annotation
- copied `8` paths to Node B in about `1s`
- Node B nix build phase: `48s`
- Node B active computer refresh: `19s`
- remote deploy total: `77s`
- full deploy job: `8m36s`
- staging health reported deployed commit
  `52d64ee01b4f7e7f43cf035b7e603191af3968ba`

Conclusion: the full flake can deploy cleanly, but full host-root prebuilds
still cost minutes on the GitHub runner. That path is necessary for flake or
host OS changes, but it should not be the ordinary service-edit path.

### Service-Only Deploy Evidence

Commit `63b8771410ebc3fd436f62bc4eadc4ed3d66fa47` added a harmless comment to
`internal/platform/config.go` to exercise the narrowed service-pointer path.

- CI run: `26364258529`
- deploy job: `77605248081`
- deploy classes:
  - `DEPLOY_HOST=true`
  - `DEPLOY_FRONTEND=false`
  - `DEPLOY_VMCTL_RESTART=false`
  - `DEPLOY_ORDINARY_GUEST=false`
  - `DEPLOY_PLAYWRIGHT_GUEST=false`
  - `DEPLOY_HOST_OS=false`
  - `HOST_SERVICES=corpusd,proxy`
- GitHub runner selected no deploy roots and skipped prebuild/copy.
- Node B skipped frontend build, host NixOS closure build, ordinary guest image,
  Playwright guest image, NixOS switch, vmctl restart, and active computer
  refresh.
- Node B built `corpusd` and `proxy` service roots in parallel; both completed
  in about `11s`.
- Node B nix build phase: `11s`
- remote deploy script total: `18s`
- full deploy job: `39s`
- staging health and the service health endpoints reported deployed commit
  `63b8771410ebc3fd436f62bc4eadc4ed3d66fa47`

This is the strongest evidence in the mission so far. Ordinary host-service
changes can now deploy without rebuilding guest images, frontend assets, the
host OS closure, or unrelated service pointers.

### Cache Decision Checkpoint

Magic Nix Cache did not produce useful evidence for this repo during the
optimization run. Logs repeatedly showed FlakeHub cache authentication failures
(`401 Unauthorized`) followed by native GitHub Action cache fallback. Even when
the cache path did not hard-fail, the original problem was over-selection of
large deploy roots, not merely cache misses.

Do not pay for FlakeHub Cache solely to solve the current service-deploy loop.
After impact isolation, the measured service-only path is already `39s` end to
end. Paid cache may still be worth evaluating for full host-root or guest-image
changes, but only with a controlled before/after benchmark and a known cache
key. The next higher-leverage work is to reduce guest-image rebuild size and
state-disk pressure, not to add another cache layer blindly.

### Runner Prebuild Removal Checkpoint

Commit `0c30a38e49725c465ea53f64901736cfe50bdfe4` proved the service-pointer
hot path, but the deploy job still spent runner time installing Nix and entering
a prebuild/copy step whose selected root list was intentionally empty. Earlier
measurements showed runner-side Nix builds and closure copies were slower and
less reliable than warm builds on Node B's persistent Nix store, so the workflow
now treats Node B as the only deploy builder.

The resulting workflow simplification removes:

- `DeterminateSystems/determinate-nix-action` from `deploy-staging`;
- the unused runner-side "Prebuild and copy Nix deploy closures" step;
- the now-unneeded `id-token: write` permission on the deploy job.

The remote deploy script still performs checkout, selected frontend/service/
host/guest builds, install, restart, health checks, and frontend asset graph
verification on Node B. Acceptance for this checkpoint is the next behavior
change deploy showing the same staging identity/health proof with a shorter
deploy job setup interval before the SSH deploy begins.

#### Acceptance: first deploy after runner prebuild removal

Commit `e67967c849891da044bc28a495bef172e5bbc16a` exercised a real
frontend-plus-sandbox service-pointer deploy after the runner-side Nix setup and
prebuild/copy path were removed.

- CI run: `26370325173`
- deploy job: `77621335838`
- deploy job duration: `14s`
- selected classes:
  - `deploy_frontend=true`
  - `host_services=sandbox`
  - `deploy_host_os=false`
  - `deploy_ordinary_guest=false`
  - `deploy_playwright_guest=false`
  - `deploy_vmctl_restart=false`
- Node B build phase: `8s`
  - fast sandbox service build: `4s`
  - frontend bundle build: `8s`
- remote deploy total: `10s`
- skipped host NixOS switch, ordinary guest image, Playwright guest image, and
  vmctl restart.
- staging `/health` reported deployed commit
  `e67967c849891da044bc28a495bef172e5bbc16a`.

The removed runner setup saved the previously observed ~20s GitHub-runner Nix
setup/prebuild overhead and made the behavior-changing deploy path shorter than
the CI test fan-out.

#### Acceptance: workflow/docs publishing noise

Commit `c615cfe8a12d259c98d2ce289cf7c2f3ae40feb9` was a docs-only checkpoint.
The main CI workflow correctly skipped it, but the rolling FlakeHub workflow
still published it. Commit `f76407be1e95c78f4f7c24398d3a3cf3f35a8180` added
docs and top-level Markdown path ignores to the FlakeHub workflow. The workflow
change itself still triggered one final FlakeHub publish, as expected.

The follow-up workflow filter also ignores `.github/**`: CI/deploy workflow
edits should validate through the main CI workflow, but they do not change flake
outputs and should not publish a new rolling flake version. This keeps future
docs/workflow-only commits from spending extra GitHub Actions time on cache
publication that cannot improve the staging deploy path.

#### Acceptance: frontend-only Trace patch after runner simplification

Commit `73b9c76b56db4202dc3dd58a5c48c83cf70be86c` hardened Trace live-follow
handling so live events can identify a trajectory through top-level
`trajectory_id`, payload `trajectory_id`, or `loop_id`.

- CI run: `26370668893`
- FlakeHub rolling publish: `26370668894`
- deploy classes:
  - `deploy_frontend=true`
  - `deploy_host_os=false`
  - `deploy_ordinary_guest=false`
  - `deploy_playwright_guest=false`
  - `deploy_vmctl_restart=false`
- Node B build phase: `8s`
- remote deploy total through health and asset graph: `13s`
- full deploy job: about `22s`
- skipped host NixOS closure, NixOS switch, ordinary guest image, Playwright
  guest image, vmctl restart, and active computer refresh
- public frontend asset: `index-ZZ7GUVdp.js`
- staging `/health` reported deployed commit
  `73b9c76b56db4202dc3dd58a5c48c83cf70be86c`

This proves frontend-only fixes remain on the fast deploy path after the
runner-side Nix setup was removed.

#### Podcast subscription backfill follow-up

The Podcast app now stores subscriptions in `podcast_subscriptions`, but old
libraries can contain generic XML RSS content items created before that table
existed. The backfill path scans old content items when a user has no explicit
subscription rows. It previously scanned too shallowly and only recognized
podcasts by app/media hints or URL/path text, so old RSS XML artifacts could
look like an empty subscription library.

The follow-up patch makes that backfill more tolerant:

- scan up to `1000` recent content items during the one-time seed;
- recognize RSS channel XML in `text_content`;
- keep the normal subscription table as the durable library after seeding.

Expected deploy class: `gateway,sandbox` service pointers only, no host OS
switch, no guest image rebuild, and no vmctl restart.

Acceptance commit: `ffa7fbf753f047e2014ffa7b09f1bd5c80477847`.

- CI run: `26370833003`
- FlakeHub rolling publish: `26370833002`
- deploy job duration: `14s`
- selected classes:
  - `deploy_frontend=false`
  - `host_services=gateway,sandbox`
  - `deploy_host_os=false`
  - `deploy_ordinary_guest=false`
  - `deploy_playwright_guest=false`
  - `deploy_vmctl_restart=false`
- Node B fast-built `gateway` and `sandbox` in parallel.
- Node B nix/service build phase: `5s`
- host service install/restart phase: `2s`
- hot-refresh phase for active interactive computers: `1s`
- remote deploy through health and asset graph: `9s`
- skipped frontend bundle, host NixOS closure, NixOS switch, ordinary guest
  image, Playwright guest image, guest image install, and vmctl restart.
- hot-refreshed two active primary computers onto the new sandbox runtime
  package without rebooting vmctl.
- staging `/health` reported deployed commit
  `ffa7fbf753f047e2014ffa7b09f1bd5c80477847`.

This is the current best evidence for the service-pointer path: a real runtime
behavior fix, deployed and hot-refreshed into active computers in seconds.

### Updated Residual Risks

- Full host-root deploys still cost minutes on the GitHub runner and should be
  reserved for flake, NixOS, and host configuration changes.
- Ordinary guest image construction remains the dominant cost for runtime and
  sandbox changes that genuinely affect user computers.
- Runtime/provider package coupling still causes some changes to select both
  gateway and sandbox plus ordinary guest. Splitting bridge interfaces remains
  the right follow-up if that class is too expensive.
- Node B state disk pressure is still a product and deploy risk. Earlier VM
  health showed very low state-dir free percentage and hundreds of hibernated
  ownerships. A cleanup/retention policy for old computer images is now part of
  the deploy-reliability backlog.
- FlakeHub cache authentication is still not configured for the organization
  cache. That is noisy but no longer on the critical path for service-only
  deploy speed.

### Updated Next Deploy-Speed Realism Axis

The next deploy-speed mission should target durable guest/runtime cost, not
another cache experiment:

```text
Split the ordinary and Playwright guest-image build graph into stable base
layers and small runtime/sandbox overlays where microvm.nix permits it; add a
safe state-disk retention policy for old candidate and hibernated computer
images; then benchmark full-host, service-only, runtime-plus-ordinary-guest,
and both-guest deploy classes again before deciding whether a paid binary cache
is worth it.
```
