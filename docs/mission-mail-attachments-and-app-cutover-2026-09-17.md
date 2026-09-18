# Mission: Mail attachments + Email app cutover

Date: 2026-09-17
Mutation class: **red** — protected surfaces touched: provider send payload
(Resend attachments), draft approval/version-hash binding (security
boundary), a new authenticated upload endpoint, and guest frontend
restaging. Rollback path: git revert of the mission commits + redeploy;
staged attachment files and new schema columns are additive, so revert is
clean. Heresy delta expected: `repaired` (removes the dead `email` app and
the "attachments are display-only" gap).

## Problem statement (documented before fix)

Owner direction 2026-09-17: the new Mail app (`mail`, MailApp.svelte) is the
keeper; the old Email app (`email`, EmailApp.svelte) goes away. Composed
mail must carry attachments from two sources: files uploaded from the
user's client computer, and files already in the user's autoputer
filesystem. The autoputer must pick up the new frontend version.

Observed gaps (all verified in source):

1. **Outbound attachments do not exist.** `sendEmailRequest`
   (`internal/maild/send.go:14-24`), `createDraftRequest`
   (`internal/maild/drafts.go:29-40`), and `resendSendRequest`
   (`internal/maild/resend.go:56-66`) have no attachment fields. Resend's
   send API accepts `attachments: [{filename, content(base64)}]` — we
   never populate it.
2. **No attachment upload/download route.** `webhook.go:51-61` registers
   no attachment endpoint; inbound attachment *bytes* are never fetched
   (`storage_ref` stays NULL, status `quarantined`). The
   `attachments/quarantine` dir is created but unused.
3. **Maild cannot read the guest filesystem.** Host maild has no channel
   to guest `/api/files`; the browser client is the only existing bridge
   (it can `GET /api/files/{path}` and `POST` to maild).
4. **Two mail apps registered.** `registry.ts:79-108`: `email` →
   EmailApp.svelte (order 30) and `mail` → MailApp.svelte (order 35).
   Draft-approval deep links (`App.svelte:245-260`, `?app=email&draft=`)
   and auth intent kinds (`email_*`) target the `email` id.
5. **Guest frontend is pinned.** The authenticated per-computer surface
   serves `CHOIR_UPDATER_ROOT/current/frontend`
   (`internal/autoputer/computer_surface.go`); a host-shell deploy does
   NOT update it. New frontend reaches the guest only via the autoputer
   package rebuild + active-VM refresh path (CI deploy lines ~1325-1375)
   or `RestagePinnedRelease`/`ImportBaseline`. Constructed-version
   computers are explicitly skipped.

## Approach

**Backend (maild):**

- `POST /api/email/attachments` — authenticated owner upload. Raw body
  bytes + `X-Choir-Filename` + Content-Type (mirror the `/api/files` PUT
  convention, not multipart). Stores bytes under
  `MAILD_STORAGE_ROOT/attachments/outbound/<ownerID>/<id>` and a metadata
  row in the per-owner mailbox DB (`email_attachments` gains
  `direction='outbound'`, `status='staged'`, `storage_ref`, `sha256`).
  Returns `{id, filename, content_type, size_bytes, sha256}`.
- `GET /api/email/attachments/{id}` — owner-scoped download (serves both
  staged outbound and, later, promoted inbound bytes).
- `DELETE /api/email/attachments/{id}` — unstage (staged only).
- `createDraftRequest.attachments: [{attachment_id}]` — binds staged
  attachments at draft create; binding flips status `staged → bound`.
  `email_drafts` gains `attachments_json`.
- **Version-hash binding:** `version_hash` covers the attachment id+sha256
  list, so the approval token binds to the exact bytes the owner
  reviewed. The approval email lists attachment names+sizes.
- `sendApprovedDraft` reads bound bytes, base64s into
  `resendSendRequest.attachments`, sends; `StoreOutboundMessage` records
  attachment rows (`status='sent'`) so Sent shows them.
- Caps: ≤10 files, ≤15 MB/file, ≤25 MB total per draft (under Resend's
  ~40 MB). `APIMaxBytes` stays for JSON routes; the upload route gets its
  own larger cap (`MAILD_ATTACHMENT_MAX_BYTES`).
- Lifecycle: bound attachments deleted with their draft; orphan staged
  rows older than 24h are GC'd by a small sweeper (or named residue if
  deferred — decide in-mission, record either way).

**Frontend (MailApp):**

- Compose gains an attachment strip: chips with filename/size/remove,
  plus two pickers — **Upload** (client file input → POST
  `/api/email/attachments`) and **From Files** (browse `/api/files`,
  GET bytes client-side, upload to maild — client-mediated, no new trust
  boundary). Draft create passes `attachment_ids`.
- Sent/draft detail renders outbound attachment chips with download via
  `GET /api/email/attachments/{id}`.

**Cutover (delete the old app):**

- `registry.ts`: the `email` entry's component becomes MailApp.svelte;
  delete the `mail` entry and `EmailApp.svelte`. MailApp's internal
  `appId`/auth-intent strings move `mail → email` so deep links and
  `email_*` intents keep working unchanged. Keep icon 📬 or ✉️ — pick
  one, delete the other.
- Clean `app.css` `.email-app` residue, `data-email-*` attributes,
  `EmailApp` imports, and the tests that pin the old app
  (`email-app-state.spec.js`, `desktop-state-persistence.spec.js`
  email sections, `prompt-surface-registry.spec.js`).

**Deploy:**

- Standard CI deploy updates the host shell. The mission is not done
  until the **guest** surface serves the new bundle: trigger the
  autoputer package rebuild + active-VM refresh for the retained
  computer (`computer-03335285269bdba4f94377e56879f9e6`) and verify the
  served frontend inside the authenticated desktop, not just
  choir.news's public shell.

## Invariants

- Draft approval semantics: every send remains owner-approved; the
  approval binds to the exact attachment set (ids + hashes) reviewed.
- Per-owner mailbox isolation: attachment bytes and rows live under the
  owner's storage/DB; no cross-owner reads.
- Inbound attachment posture unchanged: received attachments stay
  metadata-only, quarantined; this mission adds no inbound byte path.
- `X-Authenticated-User`/`X-Internal-Caller` injection stays proxy-owned;
  maild never trusts client-supplied identity headers.
- Upload endpoint is owner-scoped, size-capped, and stores outside the
  DB (bytes on disk, metadata in DB) — no blob-in-SQLite.
- Preview mode (signed-out) still renders local data only; mutations
  dispatch `authrequired`.
- No new guest→maild channel: the client mediates attach-from-FS.

## Value criterion

Better = a composed email with real attachments lands in the recipient's
mailbox with bytes intact, through the approval gate, on the deployed
product path — while the old app is fully deleted (no registry entry, no
file, no test pins). Penalties: any path that lets an attachment bypass
version-hash binding; any proof claimed from the host shell while the
guest still serves the old bundle; any mock standing in for Resend.

## Quality gradient

`solid`. Substandard looks like: attachments that upload but aren't in
the sent message; a draft whose approval doesn't cover attachment bytes;
the old app hidden-but-present; proof from local harness only.

## Homotopy axes

- File size: 1 KB → 15 MB boundary.
- File count: 1 → 10 boundary.
- Type coverage: pdf, png, ics, zip, txt.
- Source: client upload → autoputer FS → mixed.
- Environment: local harness → staging → real Resend round-trip.
- Surface: host shell → authenticated guest desktop.

## Conjecture ledger

- **C1 — Resend attachments work as documented.** Test: send a draft
  with a real attachment to `000@choir.news` on staging; the inbound
  webhook delivers attachment metadata. Falsifier: provider rejects or
  strips the field. Scope if supported: outbound attachments viable
  end-to-end.
- **C2 — Client-mediated attach-from-FS suffices for MVP.** Test: attach
  a 5 MB file from `/api/files` through the browser path; measure
  latency/failure. Falsifier: unusable for realistic files → escalate to
  a guest→maild channel design. Bound: files ≤15 MB.
- **C3 — Version-hash binding holds.** Test: stage attachment, create
  draft, tamper staged bytes, attempt send → must fail version check.
  Falsifier: send succeeds → approval boundary broken.
- **C4 — Guest refresh path delivers the new bundle.** Test: after
  deploy + refresh, authenticated desktop on the retained computer loads
  `MailApp` chunk and shows attachment UI. Falsifier: guest still serves
  pinned old bundle → the refresh path is the blocker, not the code.
- **C5 — `email` id cutover is clean.** Test: `?app=email&draft=` deep
  link and `email_*` auth intents open MailApp; zero `EmailApp`
  references remain (`grep` clean). Falsifier: any dead link/intent.

## Cognitive transforms applied

- **Inversion:** instead of "how does maild read guest files" → "the
  client already bridges both surfaces; keep the boundary." Removes a
  new trust channel from the design.
- **Object transform:** an outbound attachment is not a file — it is a
  *staged, hash-bound draft component*. The approval token binds to it;
  that decides the schema (sha256, status lifecycle, version-hash
  coverage), not the file picker.
- **Deletion test:** the mission deletes more than it adds — one whole
  app, its registry entry, its tests, its CSS. The upload endpoint is
  the only genuinely new surface.
- **Boundary transform:** the upload endpoint is a new authenticated
  write surface on a protected path — it gets the same scrutiny as the
  send path (owner scope, caps, no client-supplied identity).

## Evidence plan

- Unit: store attachment CRUD; draft create binds ids; version_hash
  covers attachments; tamper→refusal; resend payload shape (httptest);
  caps enforced.
- Local harness: MailApp compose with stubbed API — upload chip →
  attach-from-files → draft → send; desktop + mobile widths.
- Deployed staging proof (the acceptance): authenticated session on
  choir.news → compose in Mail app → attach one client-uploaded file and
  one autoputer-FS file → approve → send to `000@choir.news` → inbound
  webhook delivers the message with attachment metadata visible in the
  same app. This exercises upload→draft→approval→provider→inbound→display
  with zero mocks.
- Guest proof: authenticated desktop on the retained computer serves the
  new MailApp chunk (serving_join / frontend version evidence), and the
  old `email` app is absent from the registry everywhere.
- Mission report: `docs/mission-report-mail-attachments-2026-09-17.md`
  + PDF in iCloud mission reports.

## Forbidden shortcuts

- No attaching by guest path reference — maild cannot read the guest FS;
  bytes must flow through the client or a designed channel.
- No fake upload endpoint that skips maild storage.
- No send path that bypasses draft approval or version-hash binding.
- No proof claimed from the host shell while the guest serves the pinned
  old bundle.
- No keeping EmailApp "just in case" — the owner already decided; the
  registry keeps exactly one mail app.
- No inbound attachment byte download in this mission — out of scope.

## Rollback policy

Git revert restores the old app + backend. Staged attachment files and
new columns are additive; revert leaves them harmlessly in place.
Deploy rollback = previous frontend bundle + previous maild build; the
guest surface restages to the prior pinned release through the same
refresh path.

## Run Checkpoint & Resumption State

status: implemented — pending deploy + staging proof
current artifact state: backend attachment surface implemented
(`POST/GET/DELETE /api/email/attachments`, draft `attachment_ids` binding,
version-hash coverage, Resend `attachments` payload, staged→bound→sent
lifecycle, lazy 24h GC); frontend cutover done (`email`→MailApp, EmailApp
deleted, tests updated); deploy-impact classifier fixed so `frontend/*`
triggers the canonical guest image rebuild + active-VM refresh (the guest
surface embeds the frontend in the autoputer package inside the guest image —
previously frontend changes never reached guests).

Corrections to the problem statement's item 5: the retained computer
(`computer-03335285269bdba4f94377e56879f9e6`) currently has **no `current`
symlink** in its updater root, so its surface already falls back to the boot
image's baseline frontend and already serves the MailApp bundle — it is not
pinned to a pre-MailApp bundle. On the next boot after the image rebuild,
`ensureServingBaseline` imports the new image's frontend as `current`.
`choir computer refresh` (owner-scoped product path) reboots it onto the new
image; CI's active-VM refresh deliberately skips `constructed-computer-version`
computers, so the retained computer needs the manual refresh.

next executable probe: commit → push → CI deploy (host OS + guest image +
maild + frontend) → `choir computer refresh` the retained computer → staging
round-trip proof (compose in Mail app, attach client-uploaded + autoputer-FS
files, approve, send to 000@choir.news, verify inbound attachment metadata).
suggested resume goal string: "Execute
docs/mission-mail-attachments-and-app-cutover-2026-09-17.md as a
MissionGradient mission: maild attachment staging + draft binding +
Resend send, MailApp compose attachment UI (client upload + attach from
autoputer Files), delete EmailApp and cut the `email` app id to MailApp,
deploy and prove the attachment round-trip on staging including the
guest frontend refresh."
