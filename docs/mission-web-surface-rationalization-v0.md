# MissionGradient: Web Surface Rationalization v0

Status: proposed next 8-24h mission
Date: 2026-05-13

Doctrine note (2026-06-13): the quoted `Browser app` framing below is
historical problem language. The target doctrine is an explicit split between
Source Viewer/reader artifacts and Web Lens live/original inspection.

## Real Artifact

Choir web surface rationalization: one coherent product and architecture boundary for two different things that have been collapsing into the word "browser":

1. Candidate Desktop Viewer: a user-facing window that renders the same Choir Svelte desktop for a candidate VM by routing normal `/api/*` and WebSocket calls through `desktop_id`.
2. Web Lens / Web Import: an external-web reader/controller that uses iframe when it works, Obscura semantic snapshots when iframe is blocked, optional visual proof screenshots, bounded actions, and import into Choir artifacts.

The artifact is not a full Chrome replacement inside Choir. The goal is to let Choir understand, inspect, cite, import, and act on the web without taking on remote-browser streaming as the product promise.

The mission must turn the current confusing browser-shaped surface into an
honest, verifiable product path:

```text
external URL/search result -> iframe attempt or Obscura snapshot -> text/html/links/forms/visual proof -> bounded action or import -> vtext/content/radio artifact
candidate VM -> same Svelte shell -> same auth/proxy/vmctl routing -> candidate sandbox APIs -> verified preview/promotion decision
```

## Invariants

- Candidate VM viewing uses the normal Choir Svelte app and same-origin `/api/*` routes with a candidate `desktop_id`; it does not use VNC, WebRTC, screenshot streaming, or a browser running inside the candidate VM.
- The Svelte bundle is served by the normal frontend/static edge. Candidate VMs provide sandbox/runtime APIs, not frontend assets.
- The user's existing authenticated session is the authority for viewing their own candidate desktops. The proxy/vmctl layer decides whether the requested `desktop_id` is accessible to that user.
- Browser-public requests never reach internal vmctl/gateway/provider control routes.
- External web browsing through Obscura is a backend web reader/controller, not a promise of full interactive browsing fidelity.
- Obscura returns semantic evidence first: text, links, HTML or DOM-like structure, forms/inputs where available, and traceable source URL. Screenshots are visual proof, not the main substrate.
- Any screenshot use is snapshot/proof oriented, not a continuous remote display protocol.
- Arbitrary remote page JavaScript is not executed inside the Choir origin. Static HTML/DOM previews must be sanitized or isolated.
- Search should prefer gateway search APIs for search-result discovery, then Obscura for fetching/inspecting selected pages. Do not make Google-in-Choir the critical path.
- Existing auth renewal, desktop routing, trace, files, content, vtext, and promotion invariants remain intact.
- Provider/search credentials remain host-side behind gateway. Candidate/background VMs receive only scoped gateway credentials.
- Tests must prove behavior through product routes rather than direct service ports or test-only APIs.

## Value Criterion

Maximize useful web understanding, import, and candidate-world inspection while minimizing:

- browser-engine complexity;
- false promises in the UI;
- iframe-blocking dead ends;
- remote-display/pixel dependence;
- auth and routing bypasses;
- candidate/canonical desktop confusion;
- untrusted remote-JS exposure;
- hidden state and unverifiable browser sessions;
- regression risk to current desktop apps.

Better means a user and an agent can tell what a web surface is capable of, recover when iframe fails, import evidence into Choir artifacts, and inspect a candidate desktop through the same UI stack without confusing that with external web browsing.

## Homotopy Parameters

Increase realism continuously along these axes:

- Candidate viewer: direct `/?desktop_id=x` load -> embedded candidate desktop window -> candidate preview controls -> promotion/reject path from preview evidence.
- External web: iframe-only -> iframe plus Obscura fallback -> semantic snapshot first -> DOM/AX/form snapshot -> bounded actions -> artifact import -> agent research integration.
- Visual proof: no screenshot -> screenshot after navigation -> screenshot after bounded action -> trace-linked visual evidence. Do not evolve this into continuous streaming unless a later mission explicitly changes the invariant.
- Interaction: navigate only -> links -> form discovery -> bounded fill/click -> role/text selectors -> recoverable action traces.
- Search: pasted URL -> gateway search result -> selected page fetch -> source bundle -> vtext/content/radio import.
- Verification: unit contract -> local Playwright proof -> live Obscura proof when configured -> candidate desktop route proof.
- Product honesty: current Browser labels -> explicit Web Lens/Web Import capabilities -> user-facing fallback states -> documented frontier for full interactive browsing.

## Dense Feedback Channels

- Go tests for proxy `desktop_id` routing, candidate publication/access rules, and auth-before-vm-side-effects.
- Runtime tests for browser capabilities, Obscura text/html/link snapshots, screenshot capability flags, bounded action traces, and closed-session lifecycle.
- Frontend tests proving external Web Lens uses `fetchWithRenewal` and `withDesktopSelector` for all `/api/browser/*` calls.
- Playwright proof that an iframe-blocked page falls back to backend snapshot/import affordances instead of pretending to be a working browser.
- Playwright proof that a candidate desktop window loads the same Svelte shell with `desktop_id`, routes bootstrap and WebSocket to the candidate, and leaves the outer primary desktop stable.
- Trace assertions for navigation, snapshot, bounded action, import, candidate preview, and promotion/reject events.
- Security assertions for no public access to `/internal/vmctl/*`, `/provider/*`, raw gateway routes, or direct sandbox ports.
- UI assertions that labels and empty/error states do not promise full browser fidelity when Obscura is only providing snapshots.
- Documentation updates that explain Web Lens, Candidate Desktop Viewer, and the later full-browser frontier separately.

## Forbidden Shortcuts

- Do not implement candidate VM viewing as VNC, WebRTC, MJPEG, repeated screenshots, or a remote framebuffer.
- Do not run a browser inside the candidate VM merely to view the candidate desktop unless a future mission proves a separate need for that recursion.
- Do not claim Obscura solved iframe blocking by showing only a screenshot and calling it an interactive browser.
- Do not execute unsanitized remote HTML/JS inside the Choir origin.
- Do not route browser callers to vmctl/gateway/provider internal endpoints.
- Do not use direct service ports in product-path tests.
- Do not make Google-in-Choir the acceptance test for external browsing. Use search APIs plus page fetch/import as the controllable product path.
- Do not hide iframe failure behind vague error copy. Failure mode should become Web Lens snapshot/import behavior.
- Do not add a second unrelated UI stack for candidate desktops.
- Do not break current files, vtext, terminal, trace, settings, or prompt-bar routing while rationalizing the browser surface.

## Rollback Policy

Git:

- Keep changes in focused commits or a clearly reviewable worktree diff.
- Record before/after behavior for any rename or route migration.
- Any candidate viewer or Web Lens UI change must be revertible without database surgery.

Runtime/database:

- Browser session records are append/update state machines; avoid destructive migrations.
- Candidate preview records must preserve owner, desktop ID, VM ID, source run/candidate ID when available, and trace links.
- Failed Obscura/browser sessions remain inspectable as error records rather than disappearing.

Security:

- New viewer/auth paths fail closed.
- New preview capabilities must be scoped to the authenticated user's own candidate desktops.
- Any expansion of remote-page rendering or JS execution is invariant-level and requires escalation before implementation.

## Learning Side-Channel

Write durable learning to one or more of:

- a proof report for Web Lens rationalization;
- a candidate desktop viewer proof report;
- docs/current-architecture.md if architecture language changes;
- README or user-facing docs if app behavior is renamed;
- tests that encode the distinction between candidate desktop viewing and external web browsing.

Classify surprises:

- Tactical learning: adjust app names, route names, snapshot fields, tests, and fallback behavior.
- Target-level learning: update this mission if the best product path is Web Import, Research Browser, or another narrower frame.
- Invariant-level learning: stop and ask before adding remote display protocols, executing remote JS in Choir origin, exposing candidate VMs publicly, or changing auth/gateway/provider trust boundaries.

## Stopping Condition

Stop when the next run has produced one of these:

- A verified product-path rationalization where the external Browser surface is honestly framed as Web Lens/Web Import, iframe blocking falls back to Obscura semantic snapshot/import behavior, candidate VM viewing uses the same Svelte app with candidate `desktop_id` routing, and tests/proof reports cover both paths; or
- A documented invariant-level blocker with the next smallest safe probe and rollback point.

Completion requires evidence, not just code presence: focused Go tests, focused Playwright tests where feasible, trace or session records for the browser path, and a short final report naming residual risks.

## Completion Evidence

- Consolidated browser proof learnings:
  `docs/backend-browser-substrate-learnings.md`

## One-Line Goal

`/goal Use MissionGradient. Complete docs/mission-web-surface-rationalization-v0.md by rationalizing Choir's web surface into Candidate Desktop Viewer via routed Svelte and Web Lens/Web Import via Obscura semantic snapshots, preserving all trust boundaries and proving behavior through product-path verification.`
