# MissionGradient: Computer Recovery And System Monitor v0

**Status:** ready for execution
**Date:** 2026-05-19
**Operator:** Codex supervising staging, product-path Playwright, git, CI, deploy, Trace, VText, vmctl evidence, and owner review
**State ledger:** [platform-os-app-state.md](../platform-os-app-state.md)
**Priority policy:** [vm-priority-policy.md](vm-priority-policy.md)
**Starting deployed baseline:** `e61434e88708fdbc6df4c8fbe27e2f64f869d7ca`

## One-Line Goal String

```text
/goal Run docs/mission-computer-recovery-system-monitor-v0.md as a Codex-operated MissionGradient mission: turn the desktop restore recovery patch at e61434e into a durable computer recovery and observability substrate. Build a first-class System Monitor app with excellent UI showing current computer identity, VM warmness/hibernate state, app/window restore weight, resource pressure, runtime health, and safe recovery actions through product APIs. Harden recovery with lazy/suspended app hydration, contextual desktop restore controls, primary-vs-candidate priority policy visibility, and bounded VM/app recovery controls that preserve active computer state. Do not build a fake dashboard, broad kill switch, internal-only debug panel, mobile phone-mode rewrite, or local-only proof. Land platform changes through git/CI/deploy, verify staging identity, and prove on desktop and 390x844 mobile with Playwright screenshots/DOM metrics, vmctl/health evidence, Trace/VText/run-acceptance evidence where relevant, rollback refs, residual risks, and the next realism axis.
```

## Mission Frame

The emergency mobile restore recovery patch fixed one sharp failure mode: a
signed-in user with many restored heavy windows can now avoid hydrating the full
desktop on compact Safari. That patch is necessary but not sufficient. The
deeper product need is a recoverable automatic computer: users should be able
to understand whether their computer is warm, waking, overloaded, hibernated,
or recovering, and they should have safe controls to recover without losing
canonical active state.

This mission turns recovery from a query-param escape hatch into an ordinary
platform faculty. The user-facing artifact is a real System Monitor app. The
substrate artifact is an observability and recovery path that can explain and
repair common desktop/VM/app overload states without requiring terminal access,
host log archaeology, or fastest-finger logout.

## Real Artifact

The artifact is the deployed recovery and observability system:

```text
desktop bootstrap / app restore / vmctl health / runtime health
-> redacted product system status APIs
-> System Monitor app and Settings recovery entry points
-> safe recovery actions for desktop state, app hydration, candidates, and current computer
-> staging proof with screenshots, DOM metrics, health/vmctl evidence, rollback refs
```

The artifact is not:

- a static dashboard with hard-coded or decorative metrics;
- a raw vmctl/internal debug panel exposed to the browser;
- a broad "kill all processes" button;
- a workaround that only works by manually editing storage, cookies, or host
  state;
- a phone-mode simplification that abandons the floating desktop on mobile;
- a local-only claim about VM lifecycle or recovery behavior.

## Starting Belief State

Known from the deployed baseline:

- Staging deployed `e61434e88708fdbc6df4c8fbe27e2f64f869d7ca`.
- The mobile Safari crash loop for `ymnath@choir-ip.com` was account-specific
  desktop restore pressure, not obvious host-wide memory pressure.
- `Desktop.svelte` now detects compact heavy restores and offers clear, keep
  top window, and restore all actions. Query params `?desktop_safe=1` and
  `?desktop_recovery=1` force the same recovery mode.
- vmctl already has typed warmness classes, active pressure reclaim, always-on
  user override support, aggregate health, and candidate/worker hibernation
  semantics.
- The platform state ledger and VM priority policy document the product
  distinction between primary user computers, candidates, workers, future
  platform computers, and always-on tiers.

Main uncertainty:

- Which overloads are still browser-memory/DOM hydration problems, which are
  VM lifecycle cold-start problems, which are app-owned process/resource
  problems, and which are merely missing product status. Probe before mutating
  broad lifecycle policy.

Highest-impact observation:

- Product-path evidence from staging showing the same user can recover from a
  heavy restore, open System Monitor, see honest status, choose a bounded action,
  and return to a usable desktop without canonical data loss.

## Invariants

- The product object is a persistent user computer. Use "computer" in product
  UI; reserve "VM", "vmctl", "sandbox", and "Firecracker" for technical detail.
- Active primary computer state must remain stable. Do not use recovery as a
  silent reset, data wipe, or candidate promotion bypass.
- Candidate and worker computers are more hibernation-friendly than primary
  user computers. Under pressure, candidates/workers recover or reclaim before
  real primary desktops.
- Always-on or premium primary computers are protected policy objects, not just
  UI flags. If surfaced, show policy reason and limitations honestly.
- Browser-public status remains redacted: no emails, user ids, VM ids,
  credentials, private paths, prompt text, or raw filesystem data.
- Recovery actions are scoped and reversible where possible. Dangerous actions
  must be explicit, owner-authenticated, and evidence-producing.
- Avoid arbitrary process kill in a primary computer. Prefer app suspension,
  app window unload, candidate discard, worker hibernate, or whole-computer
  restart with clear state semantics. If process-level controls are added, they
  must be allowlisted, app-owned, and protected from killing persistence,
  gateway, auth, proxy, or vmctl services.
- Mobile remains the same floating-window desktop. Add responsive density and
  touch affordances; do not create a reduced phone dashboard.
- Platform behavior changes land through git/CI/deploy and require deployed
  staging proof.

## Value Criterion

Minimize:

```text
unexplained boot/reload waits
+ browser crash loops from eager app hydration
+ stale heavy windows mounted by default
+ hidden resource pressure
+ unsafe broad recovery controls
+ primary-computer reclaim before lower-priority resources
+ raw internal status leakage
+ fake or unmeasured metrics
+ inability to recover without terminal/operator access
+ local-only VM lifecycle claims
+ user confusion about warm/waking/hibernated/degraded states
```

subject to the invariants above.

The mission moves uphill when a normal user can open or recover their automatic
computer, see what is consuming resources, understand which computer tier they
are on, and safely recover from overload without knowing implementation names.

## Quality Gradient

Target quality: **solid**, with excellent user-facing System Monitor design.

Solid means:

- status data comes from real runtime/vmctl/desktop sources;
- APIs are typed, redacted, tested, and product-owned;
- recovery actions preserve state and produce inspectable results;
- desktop restore cannot repeatedly crash the same account in the common heavy
  restore case;
- the app is discoverable from launcher and recovery contexts;
- mobile and desktop screenshots show a coherent, readable UI;
- docs and platform state ledger are updated.

Excellent System Monitor UI means:

- information hierarchy starts with user questions: "Is my computer healthy?",
  "What is using resources?", "What can I safely do?";
- dense but readable layout: status header, resource pressure strip,
  computer/warmness card, active apps/windows list, recovery actions, and recent
  events;
- no giant decorative cards, no fake charts, no monochrome status soup;
- charts/sparklines/progress bars only where backed by real samples;
- single intentional scroll surface inside the app window on mobile;
- touch targets are usable in a 390x844 floating desktop window;
- dangerous actions are visually distinct and require confirmation.

## Homotopy Axes

Increase realism continuously without changing islands:

- **Observability source:** `/health` aggregate -> product status API -> per
  current-computer redacted status -> event-backed trend history.
- **Recovery surface:** query param -> desktop recovery panel -> Settings and
  System Monitor actions -> contextual prompt/conductor actions.
- **Hydration policy:** eager restore all windows -> lazy heavy-app mount ->
  suspended/minimized app bodies -> user-configurable restore policy.
- **Recovery strength:** clear windows -> keep top window -> unload/suspend
  heavy app windows -> hibernate/discard candidates -> restart current computer
  with explicit state semantics.
- **Priority model:** documented policy -> visible warmness class -> entitlement
  records -> capacity admission and multi-node migration.
- **Proof realism:** local tests -> staging screenshots/status -> account-level
  recovery proof -> run-acceptance evidence and long-tail regression coverage.

## Investigation And Cognitive Reframing

If the mission stalls, do not stop at "vmctl is broken" or "Safari crashed."
Apply route-changing probes and transforms:

- **Layer separation:** classify each failure as browser hydration, frontend
  state restore, runtime app process, sandbox computer boot, vmctl lifecycle,
  proxy routing, or host pressure.
- **Recovery semantics:** ask what state is being preserved, discarded, or
  restarted. A safe recovery action must name its state boundary.
- **Priority inversion:** check whether a lower-value candidate/worker is still
  consuming capacity while a primary user waits.
- **Observability first:** if a fix cannot be verified, add a redacted status or
  event path before changing policy.
- **User escape hatch:** if the user cannot reach Settings/System Monitor
  because the desktop crashes, improve the recovery interstitial rather than
  assuming the app is reachable.

A precise blocker is acceptable only after named probes identify the blocking
layer and after at least one alternative route has been tried or ruled unsafe.

## Implementation Direction

### P0: Product Status API

Build or extend authenticated product APIs for the current user's computer and
desktop recovery state. The API should combine existing sources rather than
inventing a parallel control plane:

- deployed build identity and runtime health;
- current computer bootstrap state, route state, and warm/waking/recovering
  status;
- redacted warmness class and protection reason for the current computer;
- aggregate host pressure and vmctl reclaim state already safe for browser
  exposure;
- desktop restore inventory: window count, heavy app count, top window, saved
  state age, and whether recovery mode is recommended;
- recent recovery events where available.

Use product API paths. Do not expose raw `/internal/vmctl/*` directly to the
browser.

### P0: System Monitor App

Add a first-class System Monitor app:

- launcher/start menu entry;
- window title and icon;
- mobile floating-window layout;
- status overview for current computer and platform health;
- resource pressure section backed by real metrics or explicit "not available"
  caveats;
- apps/windows section showing current windows, heavy restore weight,
  minimized/suspended/visible state, and safe actions;
- VM/computer section showing warmness class, hibernate/reclaim eligibility,
  bootstrap state, and candidate/worker summary if safely visible;
- recovery section with clear saved windows, keep top window, unload/suspend
  heavy restored apps, restart/wake current computer where supported, and
  discard/hibernate candidate contexts where safe;
- recent events section for recovery actions and lifecycle observations.

This is an app, not a Settings subsection. Settings may link to it or expose a
small "Open System Monitor" recovery affordance.

### P0: Desktop Recovery Hardening

Turn the existing emergency recovery into durable behavior:

- make restore recovery reachable without memorizing query params;
- show why recovery was triggered: number of windows, heavy app count, compact
  viewport, or manual safe mode;
- persist the user's chosen recovery action safely;
- keep the top-window-only path from remounting hidden heavy windows;
- add lazy hydration so minimized or suspended heavy apps do not mount their
  expensive bodies until raised;
- ensure bottom/prompt bar state remains usable during recovery.

### P1: VM And App Recovery Controls

Add only controls with clear state semantics:

- wake/resume current hibernated computer;
- restart current computer only if persistence semantics are explicit and
  product state is preserved;
- hibernate/discard candidate computers and workers;
- unload/suspend heavy app windows inside the desktop shell;
- expose "why unavailable" for any unsupported control.

Do not add arbitrary process-kill UI unless the implementation can prove
app-owned process boundaries and protect critical services.

### P1: Priority Policy UX

Surface the current and future priority model in product language:

- current computer class: primary, candidate, worker, public platform, or
  always-on where known;
- whether it is protected from idle reclaim;
- whether it can be reclaimed under pressure;
- why candidates/workers may hibernate before primary desktops;
- limitations of current always-on configuration and future paid/reserved
  uptime path.

Update docs if implementation behavior changes. Do not make product promises
that vmctl cannot yet enforce.

### P1: Trace, VText, And Run Acceptance

Use Trace/VText/run acceptance when the hardening work is performed through
Choir-in-Choir or when recovery events become durable product evidence:

- Trace should link recovery action/result evidence where available;
- VText should not flicker or lose focus because System Monitor refreshes;
- run acceptance should include recovery proof if a long-running candidate or
  worker mission is involved.

Do not block the mission on perfect evidence-app integration if the recovery
substrate itself is the active blocker. Record the gap precisely.

## Dense Feedback Channels

Use fast local checks for frontend shape, but deployed staging is the acceptance
environment for lifecycle claims.

Recommended feedback loop:

```text
inspect current API/status surfaces
-> add typed/redacted product status API
-> build System Monitor UI locally
-> local Playwright screenshots/DOM metrics
-> tests for status redaction and recovery actions
-> commit/push main
-> monitor CI/deploy
-> verify staging commit identity
-> staging Playwright recovery + System Monitor proof
-> update docs/state ledger
```

Required product proof:

- desktop viewport screenshot and DOM metrics for System Monitor;
- 390x844 mobile screenshot and DOM metrics for System Monitor;
- recovery interstitial screenshot on a synthetic heavy saved desktop state;
- proof that a safe recovery action changes saved desktop state as intended;
- proof that normal users can return to a usable desktop after recovery;
- health/vmctl evidence showing warmness/reclaim data source;
- evidence that browser-public status is redacted.

## Evidence Ledger

Final report must name:

- pushed commit SHA;
- CI run and status;
- deploy status and staging `/health` identity;
- product API paths added or changed;
- System Monitor app route/window/launcher proof;
- screenshots and DOM metrics for desktop and mobile;
- recovery action tested, before/after state, and rollback path;
- vmctl/health evidence for warmness, pressure, and priority class;
- Trace/VText/run-acceptance refs if used;
- residual risks and the next realism axis.

## Forbidden Shortcuts

- Do not expose raw internal vmctl endpoints to browser clients.
- Do not show fake resource stats, fake charts, or static "healthy" labels.
- Do not use host SSH, database edits, local storage surgery, or query params
  as the only recovery path.
- Do not add a broad "kill processes" or "reset computer" button without state
  semantics, authorization, confirmation, and rollback evidence.
- Do not reclaim or restart active primary computers while lower-priority idle
  candidates/workers remain eligible, unless an invariant-level blocker is
  recorded.
- Do not claim VM lifecycle proof from local dev.
- Do not weaken authentication or leak private identifiers for observability.
- Do not make System Monitor a phone-mode page. It must work as a floating
  desktop app on mobile.
- Do not let UI polish launder missing recovery behavior.

## Rollback Policy

Before landing:

- identify the prior deploy SHA and route rollback path;
- keep recovery API changes backwards-compatible where possible;
- ensure failed System Monitor UI does not block desktop boot;
- keep `?desktop_safe=1` and `?desktop_recovery=1` working until the product
  recovery path is proven better.

Rollback refs must include:

- previous platform commit;
- deploy rollback command or GitHub Actions rollback path;
- desktop state recovery action that lets an affected user regain access;
- any vmctl config changes and how to revert them.

## Learning Side-Channel

Update the platform state ledger when the mission changes:

- app catalog: System Monitor capabilities and proof status;
- desktop shell: restore recovery, lazy hydration, app switching implications;
- VM lifecycle: priority/warmness/recovery behavior;
- known gaps: unsupported resource metrics, process-level controls, multi-node
  migration, paid always-on limitations.

If the mission discovers new architecture constraints, update or reference:

- [vm-priority-policy.md](vm-priority-policy.md);
- [computer-ontology.md](../computer-ontology.md);
- [runtime-invariants.md](../runtime-invariants.md);
- [current-architecture.md](../current-architecture.md).

## Stopping Condition

Stop only when one of these is true:

1. **Full success:** staging shows a first-class System Monitor app and product
   recovery path. A heavy saved desktop can be recovered; status is real and
   redacted; priority/warmness is visible; desktop and mobile proof pass; docs
   are updated; rollback refs are named.
2. **Precise invariant-level blocker:** a named layer prevents safe recovery or
   truthful telemetry after root-cause probes and cognitive reframing. The
   final report includes evidence, why continuing would violate an invariant,
   rollback/current safety state, residual risk, and the next executable probe.

Do not stop merely because the first status metric is missing, a UI route is
awkward, or a recovery control is unsupported. Either implement the next safe
probe or record why the invariant forbids it.

