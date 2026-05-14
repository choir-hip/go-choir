# MissionGradient: Adaptive Computer Lifecycle Control

**Status:** active mission
**Created:** 2026-05-14

## Real Artifact

Optimize the deployed adaptive lifecycle control loop for user computers:

```text
browser page load
-> public desktop ready
-> auth/register ceremony when needed
-> authenticated computer resolve
-> warmness entitlement and keepalive policy
-> warm/resume/recover/boot decision
-> gateway credential reconciliation
-> guest health and bootstrap
-> desktop state restored
-> websocket connected
-> mutation or LLM wait state
-> pressure-aware keepalive/reclaim decision
-> completed product-path outcome
```

The artifact is not a dashboard, a synthetic benchmark, or a cosmetic loading
animation. It is a controller that measures, keeps warm, prefetches, protects,
reclaims, explains, and verifies user-computer lifecycle behavior without losing
the computer ontology or authority boundaries.

The low-resolution artifact is still the real staging product on
`https://draft.choir-ip.com`: public desktop, new registration, returning login,
post-auth private bootstrap, primary-computer keepalive while under capacity,
logout during loading, and a product-path mutation. Higher resolution adds
stochastic arrivals, progressive load, websocket churn, restart/recovery bursts,
pressure/reclaim interactions, LLM/prompt wait states, tiered always-on
computers, and eventually snapshot/restore paths.

`v0` does not mean the mission is a narrow observability exercise. It means the
first deployed resolution of the full control-loop gradient. Observability and
load dynamics are instruments. The product outcome is that Choir should feel
instant under normal use, remain honest under stress, and make always-on
personal computers a policy shape rather than an afterthought.

## Prior Art And Local Learnings

The mission should begin with a focused research pass and fold durable findings
into this document or a follow-up architecture note. Prefer primary sources:

- OpenTelemetry Go supports traces and metrics as stable signals and gives
  Choir a standard shape for stage spans, attributes, and counters without
  inventing a private telemetry vocabulary.
  <https://opentelemetry.io/docs/languages/go/>
- k6 `ramping-arrival-rate` is useful for open-model progressive load because
  arrival rate is independent of system response time, which better exposes
  queueing collapse than only ramping virtual users.
  <https://grafana.com/docs/k6/latest/using-k6/scenarios/executors/ramping-arrival-rate/>
- k6 thresholds provide explicit pass/fail conditions over latency and error
  rates, so performance work can fail fast instead of relying on anecdotal
  timing.
  <https://grafana.com/docs/k6/latest/using-k6/thresholds/>
- Playwright traces preserve action, DOM, console, and network evidence for
  failed or slow product-path flows, which is more useful than a final
  screenshot for lifecycle regressions.
  <https://playwright.dev/docs/trace-viewer-intro>
- Core Web Vitals use field-measurable loading, interaction, and visual
  stability metrics. Choir should use them as browser UX pressure, while also
  adding product-specific computer lifecycle metrics.
  <https://web.dev/articles/vitals>

Local learnings to preserve:

- New accounts can boot while returning accounts can fail differently. Load
  testing must include both account age and lifecycle state.
- A black screen is a telemetry failure and a UX failure. The user should see a
  causal boot/loading console, a reachable logout path, and a useful error state.
- Returning-user correctness cannot depend on deleting cookies, local storage,
  or browser profiles.
- The current pressure lifecycle mission shipped dry-run reclaim telemetry, not
  active reclaim. This mission should use that telemetry as an input, not treat
  reclaim as solved.
- The frontend build currently warns that a `ghostty-web` chunk is large. Heavy
  app chunks are a likely UX/performance target after baseline measurement.
- The most basic optimization is policy, not code cleverness: unless the host is
  near resource pressure, keep primary user computers online. A user should not
  pay cold-start latency merely because an arbitrary short idle timer fired.
- A future paid 24/7 uptime tier should be represented now in lifecycle
  vocabulary, policy tests, telemetry, and reclaim ordering even if billing and
  product UI are not part of this mission.

## Warmness And Uptime Policy

The mission should introduce a typed policy model for computer warmness:

```text
public platform computer
primary user computer
premium always-on primary computer
candidate/background computer
worker computer
critical verifier/promotion/publication computer
```

Default policy:

- Keep primary user computers online while the host is below configured memory,
  CPU, IO, disk, and PID pressure thresholds.
- Start private primary-computer warmup immediately after identity is proven,
  including register/login finish, session refresh, or a valid existing session
  on page load.
- Do not start a private primary computer before identity is proven.
- Treat candidate/background and ordinary worker computers as lower keepalive
  priority than the user's visible primary computer.
- Treat verifier, promotion, rollback, publication, and live foreground prompt
  work as protected, even when pressure exists.
- Reserve a first-class policy lane for premium always-on primary computers.
  The v0 implementation may only model, test, and report this lane, but it
  should not require a later ontology rewrite to sell 24/7 uptime.

The intended gradient is:

```text
fixed idle timeout
-> keep primary computers warm while under capacity
-> pressure-aware dry-run reclaim
-> active reclaim of lower-priority idle resources
-> paid/reserved always-on policy
-> snapshot/restore and migration for larger fleets
```

## Current V0 Implementation Target

The first behavior-changing slice should make the policy real without pretending
the full fleet scheduler exists yet:

- vmctl exposes a typed warmness class for each ownership and a redacted
  aggregate health summary.
- The deployed vmctl default keeps primary user computers warm while host
  pressure is below configured reclaim thresholds.
- Ordinary candidate and worker computers remain eligible before primary
  computers when pressure exists.
- A premium `always-on` class is modeled, tested, and protected from ordinary
  reclaim so a later 24/7 uptime tier does not require ontology changes.
- The browser starts private bootstrap prewarm only after register/login has
  proven identity, using the same authenticated product route as normal
  bootstrap.
- The proxy records aggregate lifecycle stage timings for bootstrap, protected
  API, prompt-bar, and websocket flows without exposing user ids, VM ids,
  prompt text, or credentials through public health.
- Product-path Playwright proof covers signed-out public desktop, post-auth
  prewarm, prompt mutation, returning login, warm primary reuse, deployed
  warmness policy, and lifecycle stage visibility.
- Load dynamics scripts cover public progressive, public stochastic, and
  authenticated bootstrap progressive scenarios using only product/public
  surfaces.

## Invariants

- The product object is a persistent user computer. `sandbox` is still an
  implementation/service name only.
- Signed-out public desktop viewing must not allocate, hydrate, or mutate a
  private active computer.
- Private computer warmup starts only after identity is proven. Do not preboot a
  private computer during unauthenticated `login/begin` or `register/begin`.
- Post-auth prefetch may start as soon as auth completes, provided it uses the
  same product route and authority that normal bootstrap uses.
- A primary user computer should stay online while the host is not under
  configured pressure. Short fixed idle timers are a fallback safety valve, not
  the product policy.
- A premium always-on primary computer must not be reclaimed by ordinary
  pressure policy. It may require reserved capacity, migration, or explicit
  operator intervention, but it should not silently degrade into best-effort
  idle keepalive.
- Resource pressure policy must choose lower-priority idle resources before
  visible primary computers, and visible primary computers before protected
  critical verifier/promotion/publication work only when the policy explicitly
  allows it.
- Mutable product-path work stays behind identity and normal auth/proxy/vmctl
  boundaries.
- Public telemetry may expose aggregate health only. It must not expose raw
  emails, user ids, VM ids, desktop ids, session ids, gateway tokens, provider
  credentials, prompt text, file names, or trace contents.
- Internal/operator telemetry may use stable correlation ids, but those ids must
  be scoped and redacted before browser-public surfaces.
- Load tests must not use browser-public internal routes such as `/internal/*`,
  `/api/test/*`, `/api/agent/*`, raw event mutation endpoints, or manually
  seeded success records.
- Stochastic and progressive load must preserve staging host stability. Tests
  need caps, abort thresholds, and a rollback/stop procedure.
- UX progress must be causal when lifecycle events exist. Do not fake success,
  hide failed bootstrap behind infinite animation, or let loading block logout.
- Platform behavior-changing work remains staging-first:
  commit -> push main -> monitor CI/deploy -> verify deployed identity -> run
  deployed product-path proof.

## Value Criterion

Maximize warm, secure, explainable computer availability per unit of host
capacity while preserving durable state and honest product-path proof.

Optimize:

- p50/p95/p99 `time_to_public_desktop_ready`;
- p50/p95/p99 `time_to_auth_finished`;
- p50/p95/p99 `time_to_authenticated_desktop_ready`;
- percentage of returning primary-computer visits served warm while below
  pressure thresholds;
- uptime and unexpected-reclaim rate by warmness class;
- p50/p95/p99 computer resolve, warm/resume/recover/boot, guest health,
  bootstrap, websocket connect, and first mutation/LLM wait-state timings;
- warm-hit, resume, recover, and fresh-boot ratios for returning users;
- bootstrap retry counts, 502/503 duration, websocket reconnect counts, and
  causal error classification by stage;
- mobile browser readiness and visual stability during boot/loading;
- test reproducibility across baseline, ramp, stochastic, burst, soak, and fault
  scenarios.

Penalize:

- telemetry gaps where user-visible waiting cannot be assigned to a lifecycle
  stage;
- high-cardinality or sensitive metric labels;
- benchmarks that improve by bypassing auth/proxy/vmctl/gateway boundaries;
- resource warmup before identity proof;
- reclaiming a primary user computer while lower-priority idle resources are
  reclaimable;
- treating a future always-on tier as a boolean config hack rather than a typed
  policy class with verification;
- killing host stability or live user work to satisfy synthetic throughput;
- UI progress that launders failure into apparent forward motion;
- local-only proof for deployed lifecycle claims.

## Homotopy Parameters

Increase realism continuously along these axes:

- Identity: signed-out public visitor -> new account -> returning warm account
  -> returning stopped account -> returning recovered account -> expired-session
  account.
- Load shape: single canary -> progressive arrival-rate ramp -> stochastic
  arrivals -> burst after deploy/restart -> multi-hour soak -> mixed browser and
  websocket churn.
- Lifecycle state: public shell only -> warm computer -> resume/recover path ->
  keepalive while under capacity -> boot under pressure -> boot while dry-run
  reclaim reports candidates -> reserved always-on primary computer.
- Warmness policy: fixed timeout -> under-capacity primary keepalive ->
  priority-ranked dry-run reclaim -> active lower-priority reclaim ->
  reserved always-on tier -> fleet migration/snapshot policy.
- Mutation pressure: read-only desktop -> prompt typing auth boundary ->
  post-auth bootstrap -> LLM-backed prompt wait -> file or artifact write ->
  verifier/promotion-sensitive work.
- Device and network: desktop Chromium -> mobile viewport -> slow network
  profile -> reconnect and tab-background behavior.
- Observability: ad hoc timings -> stage events -> correlated spans/metrics ->
  internal trace view -> aggregate public health -> regression budget.
- Fault realism: delayed bootstrap -> gateway/vmctl transient failure ->
  websocket drop -> service restart -> pressure sample anomaly.

## Dense Feedback Channels

Use feedback that reveals local error, not just pass/fail:

- A canonical lifecycle event schema with stage name, monotonic timestamp,
  correlation id, auth state, lifecycle decision, retry count, status, and
  redaction policy.
- A typed warmness policy schema with class, entitlement, pressure decision,
  protected-work reason, and redacted aggregate reporting.
- Go tests for stage emission, aggregation, redaction, and cardinality limits.
- vmctl policy tests proving primary keepalive under no pressure, lower-priority
  reclaim under pressure, and future always-on class protection.
- Proxy/vmctl tests proving public health exposes only aggregate lifecycle and
  pressure summaries.
- Frontend tests for public desktop readiness, boot console states, logout
  reachability, mobile layout, websocket connection status, and first mutation
  wait state.
- Playwright product canaries with trace/video/network evidence on first retry
  or failure.
- k6 public-read scripts for root desktop/static shell pressure.
- k6 authenticated-bootstrap scripts using product-created test sessions rather
  than internal session seeding.
- Progressive, stochastic, burst, soak, and fault-injection reports with
  thresholds, abort conditions, and residual risks.
- Staging `/health` identity checks before accepting any measurement.
- Baseline/after docs recording p50/p95/p99, warm-hit ratio, failure rates,
  primary keepalive ratio, uptime class behavior, host pressure, deployed SHA,
  and exact command/config used.

## Forbidden Shortcuts

- Do not preboot private computers before identity is proven.
- Do not allocate private mutable computers for signed-out public viewing.
- Do not use a short idle timeout as the primary product policy when capacity is
  available.
- Do not reclaim a visible primary computer before lower-priority idle
  candidates without explicit policy evidence.
- Do not model 24/7 uptime as a cosmetic account flag with no capacity,
  keepalive, reclaim, and verification semantics.
- Do not use browser-public internal or test-only routes for acceptance.
- Do not make load tests pass by manually seeding sessions, run records,
  success artifacts, or VM state.
- Do not expose user, VM, session, prompt, credential, or file identifiers in
  public telemetry.
- Do not add unbounded stochastic tests that can saturate staging without an
  abort threshold.
- Do not call a cosmetic BIOS animation instrumentation.
- Do not optimize only the new-account path while leaving returning accounts
  unmeasured.
- Do not hide LLM wait behind generic desktop boot state once a prompt/run stage
  exists.
- Do not claim performance improvement without before/after deployed
  measurements on the same acceptance surface.

## Rollback Policy

Every behavior-changing implementation commit must be revertable by git SHA.
For staging proof, record:

- pushed commit SHA;
- GitHub Actions run and deploy job;
- `/health` deployed proxy and sandbox commit identity;
- instrumentation mode and any sampling/export configuration;
- warmness policy configuration, pressure thresholds, and any uptime-class
  assignments used by the proof;
- load scenario, arrival rates, duration, thresholds, abort conditions, and
  generated test-account scope;
- product-path Playwright command and result;
- k6 command and result, if a load harness is part of the slice;
- any created accounts, handles, computers, or durable artifacts;
- rollback knobs for telemetry export, sampling, post-auth prefetch, primary
  keepalive, pressure reclaim, uptime-class policy, and load harness scheduling.

Telemetry and prefetch should ship behind narrow configuration when they can
change behavior or resource use. A failing or noisy metric path must be
disableable without disabling the desktop.

## Learning Side-Channel

Classify discoveries during the mission:

- Tactical learning: update instrumentation, keepalive policy, tests,
  thresholds, UI state, or scripts directly.
- Target-level learning: update this mission doc if the best first artifact is
  an adaptive keepalive policy, an OpenTelemetry exporter, a product-specific
  event table, a k6 harness, or a frontend readiness contract different from
  the first guess.
- Invariant-level learning: stop and escalate before changing public/private
  state boundaries, credential placement, active-computer ownership,
  auth/session semantics, paid uptime semantics, or promotion/verifier proof
  semantics.

Durable learnings should land in:

- this mission document;
- [runtime-invariants.md](runtime-invariants.md) if new telemetry, load, or
  prefetch rules become operating constraints;
- [current-architecture.md](current-architecture.md) if the lifecycle event
  model becomes part of the architecture;
- focused tests, load scripts, and staging evidence reports.

## Stopping Condition

The mission is complete when staging proves:

- public, new-account, and returning-account desktop readiness are instrumented
  with correlated lifecycle stages;
- primary user computers remain online while staging is below configured
  pressure thresholds, and this is proven by policy tests plus deployed
  aggregate evidence;
- a future premium always-on primary-computer class exists in policy,
  telemetry, and tests, even if billing/UI is deferred;
- public health remains aggregate-only and redacted;
- internal/operator evidence can explain where waiting occurred for a slow
  session without inspecting private content;
- Playwright canaries cover public desktop, new registration, returning login,
  simulated slow bootstrap, logout during loading, and at least one post-auth
  mutation/LLM wait state;
- progressive and stochastic load scripts exist with explicit thresholds and
  abort conditions;
- a baseline report records p50/p95/p99 readiness, warm/resume/recover/boot
  ratios, primary keepalive ratio, uptime-class behavior, bootstrap retry
  rates, websocket behavior, and host pressure;
- at least one UX-driven performance optimization is selected from measured
  evidence, such as post-auth bootstrap prefetch, heavy chunk splitting, desktop
  shell retention during private boot, or causal boot-status events;
- deployed health reports the expected commit;
- residual risks and the next realism axis are named plainly.

## Short Goal Prompt

Use MissionGradient. Complete
`docs/mission-lifecycle-observability-load-dynamics-v0.md` by building the first
deployed resolution of Choir's adaptive user-computer lifecycle control loop:
keep primary computers warm while under capacity, start private warmup
immediately after identity is proven, model a future premium 24/7 uptime class,
instrument correlated lifecycle stages, create progressive and stochastic
product-path load dynamics, and use the evidence to optimize security,
performance, and UX. Preserve public/private authority boundaries, avoid
forbidden shortcuts, prove behavior on staging for platform changes, and
stop/escalate on invariant-level surprises.
