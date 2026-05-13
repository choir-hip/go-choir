# Obscura Browser-In-VM Frontier - 2026-05-13

## Current State

The Choir Browser app now has a backend Obscura text-snapshot path when `CHOIR_OBSCURA_BIN` or `OBSCURA_BIN` is configured. Legacy iframe mode still exists as fallback, but it is no longer the only product path.

The remaining frontier is background VM browser view/control. A text snapshot is a useful proof of server-owned navigation, but it is not yet a controllable browser session or a visual remote VM window.

The original iframe-only topology was wrong for Choir-in-Choir because:

- arbitrary sites can block iframe embedding;
- frontend browsing cannot represent a background VM browser session;
- agentic browsing cannot be verified through server-owned navigation, extraction, screenshots, or Trace events;
- current Browser Playwright tests still encode iframe behavior as the product contract.

## Obscura Evidence

Local Obscura material exists outside this repo:

- repo: `/Users/wiz/obscura`
- branch: `choir/playwright-parity-audit-2026-05-10`
- binary smoke path: `/Users/wiz/obscura/target/release/obscura`
- audit docs: `/Users/wiz/obscura/docs/choir/`

Local smoke verified the binary can run a backend browser acquisition:

```sh
/Users/wiz/obscura/target/release/obscura fetch https://example.com --dump text --timeout 10 --quiet
```

Result included the expected `Example Domain` text.

The migrated audit docs report a patched Obscura build that passed the Choir Playwright-parity audit, but that is not clean upstream parity. The mission should treat this as a viable substrate with packaging and fidelity risks, not as a ready dependency.

## Correct Integration Shape

The implementation should keep a backend browser controller behind an internal interface, not wire the frontend directly to Obscura or add browser-public orchestration routes.

Minimum product contract:

- `BrowserProvider` interface with capability discovery: `available`, `provider`, `version`, `supports_screenshot`, `supports_markdown`, `supports_input`.
- Owner-scoped browser sessions persisted in runtime state.
- Product endpoints under `/api/browser/*` for session create, navigate, snapshot/markdown, and close.
- Frontend Browser app sends navigation intents and renders server-owned snapshots.
- Browser navigation/extraction events are recorded into Trace.
- If Obscura is unavailable, the app must fail closed with a product-visible capability state.

Continuous deformation toward the full target:

1. local Obscura CLI fetch smoke; done.
2. runtime provider interface and capability endpoint; done.
3. owner-scoped browser session records; done.
4. navigate plus text snapshot; done.
5. browser navigation/extraction Trace events; done.
6. links snapshot persistence using current Obscura CLI support; done.
7. html snapshot persistence using current Obscura CLI support; done.
8. close semantics and session lifecycle; done.
9. snapshot-vs-control substrate contract; done.
10. screenshot substrate probe through Obscura CDP; done.
11. product-owned CDP screenshot persistence; done.
12. persistent host-process CDP lifecycle; done.
13. bounded input/control command contract; done.
14. background VM browser session identity; done at metadata-binding resolution.
15. Browser app view/control of a background VM session;
16. Choir product path can open its own UI inside a backend browser session.

## Forbidden Shortcut

Do not claim this frontier by embedding an iframe, adding a browser-public internal route, or using Playwright screenshots as if they were backend browser state. Playwright remains the verifier. Obscura should become the product substrate.

## Next Safe Patch

The next safe code patch is not a full iframe replacement. It is the backend browser contract:

- add runtime API tests proving `/api/browser/capabilities` is owner-scoped and no vmctl/provider internals are browser-public;
- add a provider resolver that detects configured Obscura binary paths and reports unavailable cleanly;
- add Browser app capability UI that prefers backend mode and labels legacy iframe mode as legacy only;
- update Browser tests so HTTP navigation no longer requires `data-browser-iframe` once backend mode is available.

That patch preserves topology and gives the following implementation somewhere safe to attach real Obscura sessions.

## Contract Patch Landed

`docs/backend-browser-capability-contract-proof-2026-05-13.md` records the first contract slice:

- authenticated `GET /api/browser/capabilities`;
- `CHOIR_OBSCURA_BIN` / `OBSCURA_BIN` configuration;
- executable detection without running browser work in the request path;
- Browser app backend/legacy mode status.

## Session Patch Landed

`docs/backend-obscura-browser-session-proof-2026-05-13.md` records the second slice:

- owner-scoped browser session persistence;
- server-side Obscura navigation;
- persisted text snapshot rendering in the Browser app;
- gated Playwright proof that backend mode renders `Example Domain` without an iframe.

## Trace Patch Landed

`docs/backend-browser-trace-events-proof-2026-05-13.md` records the third slice:

- browser session create/navigate/failure events;
- event-only browser session trajectories in Trace;
- readable Trace summaries and tones for browser moments.

## Link Snapshot Patch Landed

`docs/backend-browser-link-snapshot-proof-2026-05-13.md` records the fourth slice:

- durable extracted links on browser sessions;
- link counts in browser Trace completion events;
- Browser app link panel in backend mode;
- live Obscura Playwright proof against `https://example.com`.

## HTML Snapshot Patch Landed

`docs/backend-browser-html-snapshot-proof-2026-05-13.md` records the fifth slice:

- durable HTML source on browser sessions;
- HTML byte counts in browser Trace completion events;
- Browser app source panel rendered as escaped source text;
- live Obscura Playwright proof against `https://example.com`.

## Lifecycle Patch Landed

`docs/backend-browser-session-lifecycle-proof-2026-05-13.md` records the sixth slice:

- explicit `closed` browser session state;
- owner-scoped idempotent close endpoint;
- navigation rejection after close;
- Browser app close control and local snapshot clearing;
- browser closed-session Trace moment;
- live Obscura Playwright proof against `https://example.com`.

## Substrate Contract Patch Landed

`docs/backend-browser-substrate-contract-proof-2026-05-13.md` records the seventh slice:

- capability `substrate`;
- explicit `obscura_cli_fetch` backend snapshot mode;
- fail-closed support matrix when unconfigured;
- product-visible false support for screenshot, input, and CDP control;
- live Obscura Playwright proof of those attributes through the Browser app.

## CDP Screenshot Substrate Probe Landed

`docs/obscura-cdp-screenshot-substrate-proof-2026-05-13.md` records the eighth slice:

- gated Playwright verifier for `obscura serve`;
- `/json/version` discovery;
- Playwright CDP connection;
- `https://example.com` navigation;
- nontrivial PNG screenshot assertion;
- process teardown.

This proves the installed Obscura binary can supply a screenshot-capable CDP substrate. It does not prove the Choir Browser product owns CDP sessions yet.

## CDP Screenshot Product Patch Landed

`docs/backend-browser-cdp-screenshot-product-proof-2026-05-13.md` records the ninth slice:

- opt-in `CHOIR_OBSCURA_CDP_SCREENSHOTS=1` provider mode;
- runtime-owned Obscura CDP session for screenshot capture;
- durable `browser_sessions.screenshot_png_base64`;
- Browser screenshot rendering;
- Trace screenshot summaries;
- gated Go and live Playwright product proofs.

## Persistent CDP Lifecycle Patch Landed

`docs/backend-browser-persistent-cdp-lifecycle-proof-2026-05-13.md` records the tenth slice:

- runtime CDP session map keyed by Browser session ID;
- reused attached Obscura CDP `sessionId` across navigations;
- persisted `execution_scope` and `backend_session_id`;
- Browser app data attributes for execution/session identity;
- close teardown for active CDP sessions;
- gated Go and live Playwright proofs of session reuse.

## Bounded Control Patch Landed

`docs/backend-browser-bounded-control-proof-2026-05-13.md` records the eleventh slice:

- bounded `fill` and `click` control actions by selector;
- no generic CDP, JavaScript, keyboard, mouse, or frontend iframe control claim;
- product-visible capability flags for bounded input while generic `input` and `cdp` stay false;
- Browser app selector/value/fill/click controls;
- browser control Trace events;
- per-session backend browser operation serialization;
- removal of hidden default navigation on Browser app mount;
- recovery from a live stale-navigation race found by Playwright;
- gated Go and live Playwright proofs of bounded control through the desktop.

The next safe patch is now background VM browser session identity. That keeps Playwright as verifier and Obscura as product substrate while avoiding the false claim that persistent host-process CDP is already a VM browser window.

## Candidate-World Identity Patch Landed

`docs/backend-browser-candidate-world-identity-proof-2026-05-13.md` records the twelfth slice:

- Browser sessions can bind to owner-scoped promotion candidates by `promotion_candidate_id`;
- VM/snapshot/source identity is derived from the promotion queue rather than accepted as arbitrary browser input;
- raw `vm_id` request fields are rejected;
- other owners cannot bind to someone else's candidate;
- Browser session records and Trace payloads carry candidate-world identity;
- Browser app exposes world/candidate/VM/snapshot/source identity as stable data attributes.

The next safe patch is now VM-local browser execution. The missing transition is not identity anymore; it is making the browser substrate run in, or route through, the candidate VM lease while keeping vmctl control endpoints internal-only.

## VM-Local Execution Blocker Recorded

`docs/backend-browser-vm-local-execution-blocker-2026-05-13.md` records the current invariant-level stop:

- host-process CDP control must not be overclaimed as VM-local execution;
- browser-public callers must not pass `vm_id`, `worker_sandbox_url`, or raw vmctl handles;
- the safe next probe is an internal VM browser lease resolver plus server-to-server controller path;
- verification should prove owner scope, inactive lease failure, and no browser-public vmctl exposure before a worker Browser controller is added.
