# Mission: Mail app v2 (new email UI alongside existing Email app)

Date: 2026-09-15
Mutation class: **orange** — new runtime product surface (a second registered
desktop app). No protected surfaces touched: no Texture canonical writes, no
auth/session changes, no maild backend changes, no deploy routing changes.
Rollback path: git revert of the mission commits; the existing `email` app is
untouched and remains registered.

## Problem statement (documented before fix)

Owner report 2026-09-15: "There are many issues with the existing email app UI…
rather than fix it, just make another one from scratch. The backend should be
consistent, and the existing app should remain, so I can then decide which one
to keep." Reference screenshot: desktop Email window, 2026-09-15.

Observed defects in `frontend/src/lib/EmailApp.svelte` (desktop screenshot +
code read):

1. **Flat monochrome surface.** Every pane uses `var(--choir-state-selected)`
   (the selection-highlight color) as its background, so rail, list, and reader
   are one undifferentiated dark field. Text hierarchy collapses: nearly all
   text renders in `--choir-text-accent`, so sender/subject/snippet/meta share
   one weight of voice.
2. **Cramped, low-density message list.** Rows are tall cards with a full-width
   trust label line ("Public inbound") on every row; ~4.5 messages visible per
   pane height. Sender names wrap to two lines ("Startup Boston Team").
3. **Trust state is text, not signal.** Every inbound row repeats the same
   "Public inbound" string; quarantined vs public vs trusted are not visually
   distinguishable at scan speed.
4. **Reader is a form, not a document.** Subject/from/to crammed into a header
   row; body in a bordered white box; actions pinned to a footer bar far from
   the content they act on; "Details" disclosure below the fold.
5. **No filtering, no unread badges on folders, no avatars, weak timestamps**
   (always "Sep 14, 7:02 AM" even for today's mail).
6. **Mobile is an afterthought.** A `<select>` mailbox switcher plus a "Back"
   button; no folder chips, no unread counts, same tall rows.
7. **Draft flow reads as broken.** "Create draft" / "Send approved draft" /
   "Email approval link" buttons with no explanation that outbound mail is
   approval-gated by design.

## Approach

New app `mail` ("Mail", icon 📬) registered alongside `email` in
`frontend/src/lib/apps/registry.ts`; new component
`frontend/src/lib/MailApp.svelte`. Same backend contract — identical
`/api/email/*` endpoints, same request shapes, same draft-approval semantics,
same `appContext` keys (`activeFolder`, `selectedId`, `detailPaneOpen`,
`draftId`, `view`, `windowTitle`) and same dispatched events (`contextchange`,
`authrequired`, `authexpired`, `launchapp`) so it is a drop-in alternative.

Design decisions (cognitive-transform output):

- **Object transform:** this mailbox is a supervision surface over untrusted
  inbound mail and approval-gated outbound drafts. Trust state becomes a
  first-class visual signal (chips only when non-trusted; calm default for
  trusted), and the draft view is framed as a review step, not an error.
- **Material transform:** the reading pane is a document, not a form. HTML mail
  renders in a sandboxed iframe on a paper surface; plain text renders on a
  theme surface with measure-limited line length.
- **Density:** list rows are compact (avatar + 2 lines), ~2x the visible
  message count of the old app; unread = accent dot + weight, not a card.
- **Responsive by container, not viewport:** `bind:clientWidth` drives
  wide/medium/compact layouts, so narrow desktop windows and mobile get the
  same correct treatment. Compact = single column with folder chips and a
  slide-over reader; medium = icon rail + list + reader.
- **Theme-native:** all colors via `--choir-*` tokens; namespaced classes to
  avoid the global `app.css` `!important` overrides (`.message-list`,
  `.snippet`, `.badge`, `.active`, etc. are claimed by the theme layer).

## Invariants

- `EmailApp.svelte` and the `email` registry entry unchanged.
- No backend changes; no new endpoints.
- Preview mode (signed-out) renders local preview data only; all mutations
  dispatch `authrequired`.
- HTML bodies stay sandboxed (`sandbox` attr, CSP in srcdoc, sanitized).

## Evidence plan

- `npm run build` (vite + source-contract check) compiles.
- Local visual proof: standalone harness page mounting `MailApp` with stubbed
  `fetch` (authenticated) — desktop 1280px, medium 860px, mobile 390px
  screenshots; exercise list → reader → reply → compose → drafts → quarantine.
- Staging proof after deploy: signed-out preview of the Mail app on
  choir.news (public-preview policy), desktop + mobile viewport.
- Mission report: `docs/mission-report-mail-app-v2-2026-09-15.md` + PDF in
  iCloud mission reports.

## Run Checkpoint & Resumption State

status: in_progress
last checkpoint: problem documented, pre-implementation
current artifact state: EmailApp unchanged; MailApp not yet created
what shipped: nothing yet
what was proven: problem inventory against screenshot + source
unproven or partial claims: none yet
belief-state changes: none
remaining error field: entire implementation + verification
highest-impact remaining uncertainty: whether container-width-driven layout
  behaves inside the floating-window shell on real mobile Safari
next executable probe: implement MailApp.svelte, build, harness screenshots
suggested resume goal string: "finish Mail app v2: implement, verify, land"
evidence artifact refs: owner screenshot 2026-09-15 (local screenshots dir)
rollback refs: git revert of mission commits
