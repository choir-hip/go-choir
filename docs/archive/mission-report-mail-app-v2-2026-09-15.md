# Mission Report: Mail app v2 (redesigned email UI)

Date: 2026-09-15 UTC
Status: complete — deployed and verified on staging. **Historical A/B report:**
the "existing app remains untouched" claim is superseded — EmailApp was later
deleted and the `email` app id cut to MailApp
(`docs/mission-mail-attachments-and-app-cutover-2026-09-17.md`).
Mutation class: orange (new runtime surface; no protected surfaces, no backend changes)
Problem/approach doc: docs/mission-mail-app-v2-2026-09-15.md

## Mission goal and artifact

The existing Email app UI (built months ago) had accumulated design problems:
cramped columns, clipped metadata, no search/filter, weak mobile behavior.
Rather than patch it, build a second mail app from scratch on the same backend
so the owner can A/B them and keep the better one. The existing app remains
untouched.

Artifact: `frontend/src/lib/MailApp.svelte` (~3,000 lines) plus one registry
entry (`id: 'mail'`, icon 📬, order 35, `data-mail-window` shell attr).

## Value criterion

A mail UI that reads like a document surface, not a database grid — while
reusing the exact `/api/email/*` contract, `appContext` keys, and dispatched
events so it is a drop-in alternative to `email`.

## What shipped

- `bdf4d891` green(docs): problem inventory + approach (documentation-first).
- `940f46c1` feat(mail): MailApp.svelte + registry entry.
- `4c0b8b20` green(docs): mission doc completion checkpoint.

## Design decisions

- **Reading pane as document**: HTML bodies render in a sandboxed iframe
  (`sandbox` attr + CSP in srcdoc + sanitized) on a paper surface; plain text
  gets a measure-limited column. HTML/Text toggle.
- **Trust as visual language**: chips appear only for non-trusted states
  (Public inbound, Quarantined, Preview draft); trusted mail carries no chrome.
- **Dense compact list**: avatars (initials; pencil icon for drafts, sent icon
  for outbound), unread dots, relative timestamps, snippets, attachment clips.
- **Folder counts** via `limit=1` probes against the existing list endpoint —
  no backend changes needed.
- **Responsive by container width** (`bind:clientWidth`), not viewport —
  correct inside floating windows at any size: wide = rail + list + reader,
  medium = icon rail, compact = folder chips + slide-over reader.
- **Draft approval framed as review step** ("Approve & send"), keyboard
  navigation (arrows), client-side filter, compose with alias picker.
- **Namespaced classes** (`mail-*`) because global `app.css` `!important`
  rules claim generic class names.

## Evidence

- `vite build` clean; MailApp emitted as lazy chunk `MailApp-CVANtKrD.js`.
- Local stub-API harness (temporary, deleted after use): inbox/drafts/sent/
  quarantine, filter, keyboard nav, reply→draft→send, approve-and-send toast,
  HTML/text toggle, attachments, signed-out preview mode; screenshots at
  1280px / 860px / 390px (`/tmp/mailv2-*.png`).
- Two real bugs found and fixed during verification:
  - Preview list stayed empty: `loadPreviewMailbox` ran synchronously inside
    the reactive auth watcher; mid-`$$.update()` invalidations update DOM but
    do not re-run dependent reactive statements (`filteredMessages` stayed
    stale). Fixed by deferring via `queueMicrotask`.
  - Success notice wiped: `loadDetail` cleared `notice` after `sendReply` set
    it; removed the clear.
- CI run `34947454790`: green after one deploy-job retry (Node B disk-headroom
  preflight flake — infra, not code; reclaim freed 334.7 MiB, still under
  threshold on first attempt).
- Staging deploy: activation receipt `service=frontend
  commit=940f46c1f6b0fd3d7502c1229345826bd3ed5e93`, asset
  `index-CPoOaEgx.js`; `MailApp-CVANtKrD.js` (60,950 bytes) served.
- Deployed acceptance: signed-out choir.news desktop → Mail icon → window
  opens with preview data (`data-mail-app`, 4 rows) on desktop and 390px
  viewports; compact layout engages on mobile (`/tmp/staging-mail-*.png`).
- Note: `x-choir-build-commit: 8c6858ba` header is the proxy binary's build —
  unchanged by a frontend-only deploy; the activation receipt is the frontend
  identity.

## Invariants held

- `EmailApp.svelte` and the `email` registry entry: unchanged.
- No backend changes, no new endpoints.
- Signed-out preview renders local preview data only; mutations dispatch
  `authrequired`.
- HTML bodies sandboxed.

## Residual risks

- Authenticated path verified against a stub API only; real-mailbox proof
  requires a signed-in session (owner A/B step).
- `?app=email&draft=` approval deep-links still open the old app — deliberate
  until the keeper is chosen; switching is a one-line registry/URL change.
- Node B root disk headroom (100 GiB free vs 100 GiB required) remains marginal;
  the deploy preflight will keep failing intermittently until headroom grows.

## Heresy delta

None introduced, none repaired. New code on an existing contract.

## Rollback refs

- `git revert 940f46c1` removes the app and registry entry; `email` unaffected.

## Human-learning digest

- Svelte 3/4 legacy reactivity: synchronous invalidations made inside a
  reactive statement during `$$.update()` do not re-run dependent reactive
  statements — defer store-like loads out of watchers (`queueMicrotask`).
- In this repo's frontend, global `app.css` uses `!important` on generic
  class names; new surfaces must namespace their classes.
