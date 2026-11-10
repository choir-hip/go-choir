# Autoradio app API: what the phone can call today (2026-11-10)

Status: map of what exists at origin/main 7f1b2487f; mutation class green;
rollback: revert. This is step 0.2 of the
[goal](../definitions/choir-autoradio-pass1-2026-11-10.md) (lane A, App API).
It reads the code and two unauthenticated GETs of the public front page on
choir.news. Nothing was run against an authenticated route.

## 1. Device sign-in (exists, deployed on main)

Routes are registered in `cmd/auth/main.go:97-102`.

| Step | Method and path | Auth | Request and response |
|---|---|---|---|
| start | `POST /auth/device/begin` (`internal/auth/device_login.go:194`) | none | in `{label}` (64 characters at most). Out `{device_code "choir_dc_...", user_code, verification_uri, verification_uri_complete, expires_in 600, interval 5}` |
| show request | `GET /auth/device/lookup?user_code=` | passkey cookie | `{label, requested_from, requested_at, expires_at, status, grants}` |
| approve | `POST /auth/device/approve` (`:349`) | passkey cookie, JSON | in `{user_code, decision: "approve" or "deny"}`. Out `{status, label}` |
| poll | `POST /auth/device/token` (`:400`) | none; the code is the secret | in `{device_code}`. Out `{api_key "choir_sk_...", key_id, label, scopes[]}`, once |

- Poll errors, each HTTP 400 with `{error, error_description}`:
  `authorization_pending`, `slow_down`, `access_denied`, `expired_token`,
  `invalid_grant` (`:411-436`).
- The token is an opaque `choir_sk_` API key, not a JWT. It is minted with
  `expires_at NULL` (`device_login_store.go:236`): it never expires. There
  is no refresh route.
- Its label is stored as "choir login: <label>".
- Its scopes are `AllScopes()` (`internal/auth/handlers.go:1752`), which
  includes `owner:acts` and `admin`, because the approving passkey session
  grants them.
- Use: `Authorization: Bearer choir_sk_...`. The proxy tries the cookie,
  then the bearer (`internal/proxy/handlers.go:565-575`; `validateAPIKey` at
  `:630`). `requiredAPIKeyScope` (`:593`) maps the path to a scope:
  `/api/precommit/` needs `precommit:read` or `precommit:write`,
  `/api/texture/` needs `read:texture` or `write:texture`, everything else
  needs `read:runtime` or `write:runtime`.
- Revoke and list: `DELETE /auth/api-keys/{id}` and `GET /auth/api-keys`
  take a cookie, or a bearer with `manage:keys` or `admin`. The list is
  `{keys:[{id, label, scopes, computer_id, created_at, expires_at,
  last_used_at, revoked_at}]}`.
- `GET /auth/sessions` lists browser refresh sessions only
  (`internal/auth/recovery.go:515`), never API keys. The goal's check that
  the phone "appears by name in that account's sessions" is therefore not
  met by the existing route; the phone appears in `/auth/api-keys`.
- Deployed spec with a virtual authenticator:
  `frontend/tests/cli-login-approve-deployed.spec.js` (Playwright CDP
  authenticator; env `CLI_LOGIN_LINK_FILE`, `CLI_LOGIN_LABEL`,
  `CLI_LOGIN_RESULT_FILE`), wrapped by `scripts/e2e-cli-login-staging.sh`.
- Missing: refresh, a named revocable device record, expiry, a narrower
  scope set.

## 2. Reading the edition

Public, no auth:

- `GET /api/platform/publications/front?limit=&cursor=&section=` returns
  `{stories[], next_cursor, sections[]}` (`internal/proxy/platform_public.go:94`;
  type `platform.FrontPage`, `internal/platform/publication_front.go:79`).
  A story is `{publication_id, title, dek, updated_at, version_count,
  latest_revision, route{bare, permalink}, importance, section, sections[],
  source_count, words, kind, author}`.
- `GET /api/platform/publications/resolve?route=<bare>` returns a
  `PublicationBundle` (`internal/platform/types.go:230`). It carries
  `artifact.body_doc` (the structured Texture body),
  `artifact.source_entities`, `citations[]`, `payload{revision, text,
  manifest, manifest_hash, resolved_blocks}` and `nav`.
- `GET /api/platform/publications/export?route=&format=txt|md|html|docx|pdf`
  returns a rendering; `raw=1` returns a text body bare.

There is no account-specific edition API. The front list is the paper's,
global, newest first. The only per-account list is on the listener's own
computer: `GET /api/texture/documents?limit=&cursor=` (scope `read:texture`)
returns `{documents:[{doc_id, title, trajectory_id, current_revision_id,
current_version_number, revision_count, agent_revision_pending, ...}],
next_cursor}` (`internal/textureowner/texture.go:150,195,820`).

On the computer (routes `internal/textureowner/router.go:61`):
`GET /api/texture/documents/{id}`, `.../{id}/revisions`,
`GET /api/texture/revisions/{id}`. A revision is `{revision_id, doc_id,
author_kind, author_label, version_number, content, body_doc,
source_entities, source_refs, citations, metadata, provenance,
revision_hash, parent_revision_id, created_at}` (`texture.go:224`).

The block model (`internal/texturedoc/schema.go:257-349`):

- Blocks: `paragraph`, `heading`, lists, `blockquote`, `code_block`,
  `horizontal_rule`, `embed`. Inline: `text`, `hard_break`, `source_ref`.
- `paragraph` and `heading` require `attrs.id`; that id is how edits
  address a block (`internal/texturedoc/outline.go:11`).
- A citation is a `source_ref` node with `attrs.source_entity_id` and
  `display_mode` `numbered_ref` or `expanded_ref`; the entity is in
  `source_entities`.
- `embed` kinds: `capsule_evidence`, `image`, `file`, `video`, `audio`
  (`internal/texturedoc/embed.go:30-60`). Audio holds only `target.url` and
  a title.
- There is no time-span, clip, speech or run-sheet block. The
  `choir.autoradio_run_sheet` kind is reserved and unused
  (`internal/kernelservice/kernelservice.go:45`).

## 3. Prompting a document

`POST /api/prompt-bar` (`internal/textureowner/prompt_bar.go:99`).

- Request `{text, command_id?, focus?{app_agent_id, object_ref,
  selection?}, base_revision_id?}`; unknown fields are refused.
- With `focus.app_agent_id` set to the Texture desk and `object_ref` the
  `doc_id`, the call is an owner revise of that document. It returns 202
  `{schema, continued: true, command_id, doc_id, trajectory_id,
  revision_id, request_id, replay}` (`:59`). `command_id` is the
  idempotency key; `base_revision_id` pins the revision responded to.
- A Texture focus has no position inside the document: `selection` is
  dropped on that path (`prompt_bar.go:245`; only a package desk receives
  it), and neither this route nor `/revise` takes a `block_id`.
- Without a focus the text is routed; the 202 is `{submission_id, state,
  status_url "/api/prompt-bar/submissions/{id}", doc_id, revision_id, ...}`
  and the status URL returns `{state, decision, error}`.
- Equivalent: `POST /api/texture/documents/{id}/revise` takes `{text or
  prompt or intent, client_request_id, base_revision_id,
  expected_head_revision_id, sources[], body}`
  (`internal/textureowner/revise.go:114`).
- `POST /api/autopaper/ask` takes `{text, command_id}`.

How a client learns the result:

- `GET /api/texture/documents/{id}/stream` is SSE; add `?format=json&after=<cursor>&limit=`
  for a polled page of events `{cursor, kind, event_type, revision_id,
  work_state, trajectory_status, command_id, request_id, created_at, ...}`
  (`internal/textureowner/texture_observation.go:39,63`). `Last-Event-ID`
  is honoured.
- `GET .../{id}/events` returns the same page (`texture_observation.go:270`).
- `GET .../{id}` reports `agent_revision_pending`.
- `/api/ws` is a live WebSocket (`internal/agentcore/live_ws.go:52`).

The reader's own fork: `POST /api/texture/fork` with `{source{
publication_version_id, manifest_hash}, text?, client_request_id}`
(`internal/textureowner/revise_fork.go:25-69`). Both source values come from
the resolve bundle. It returns the same 202 receipt with a new `doc_id`.
Without `text` it is a verbatim copy; with `text` it is a merge the desk
acts on.

## 4. Jev for a client

- `POST /provider/v1/judgments` (`internal/gateway/jev.go:371`) is
  guest-internal: a VM bearer plus an IP-pin check (`jevPeerAllowed`).
- Wire body `{state: object, questions: {name: {type: "choice" or "score" or
  "noul", ...}}, scope?}` (`jev.go:21`, `validateJudgmentRequest` at `:235`).
  `scope: "interactive"` selects the reserved class, about 2 requests a
  second with a burst of 4 and 2 in flight (`jev_limiter.go:44-46`). A
  `choice` question carries `instructions`, `criteria` and `choices`
  (`internal/agentcore/system1_verb.go:560`).
- `POST /provider/v1/host/judgments` (`internal/gateway/host_judgment.go`)
  is for allowlisted host jobs on a host credential; it refuses `scope`.
- No route is reachable by a signed-in end user. The proxy has no gateway
  URL in its config.
- `internal/jevrecord` is a guest-local JSONL log, not a client API.

## 5. Precommit records from a client (exists)

`/api/precommit/` is served on the computer by `calibration.Handler`
(`internal/agentcore/api_precommit.go`).

| Method and path | Request |
|---|---|
| `POST /api/precommit/claims` | `{agent{name, model, session_id}, claim, type?, p, criterion?, deadline?, project?, tags[], idempotency_key?, source_ref?}` |
| `POST /api/precommit/claims/{id}/settlements` | `{outcome, evidence[], scorer{method, name, model}, note?}` |
| `GET /api/precommit/claims[/{id}]`, `GET /api/precommit/calibration` | none |

Scopes `precommit:read` and `precommit:write`; a device key has both.
Claims are never edited; settlements are appended. A "playback event" has no
first-class kind and nothing ties a claim to a document or a block.

## 6. Push and changelog

- `GET /api/changelog?minor=1&before=&limit=` returns `{events:[{inbox_seq,
  level, changelog{event_id, kind, level, summary, subject, tape_ref, span,
  ...}}], next_before, head, seen_seq, unseen_major, unseen_more}`;
  `PUT /api/changelog/seen {"seq": n}` marks them seen
  (`internal/agentcore/api_changelog.go`). `revision_committed` is one kind
  (`internal/types/commitment_changelog.go`).
- No APNs code and no device registry exist. For one document's "revised",
  use the stream in section 3.

## 7. Content on staging now (2026-11-10)

- The front page is public and answered 200.
- A paging loop capped at 30 pages read 3,000 stories without reaching the
  end, so there are at least 3,000.
- Sections: world, us, politics, business, tech_ai, science, health,
  climate, culture, sports.
- The newest story had 290 words, 11 sources and 14 revisions; the first
  100 stories average about 296 words.
- Across 12 sampled stories there were no embeds, and the one bundle
  searched in full had no audio or podcast reference. Publication refuses a
  document that holds an embed (`internal/texturedoc/embed.go` header), so a
  published story cannot carry audio today.

## 8. Smallest server additions, in order

| # | Route or change | Service | Auth | Class | Protected surface | Size |
|---|---|---|---|---|---|---|
| 1 | Jev for the phone (see Decisions: through the listener's computer) | runtime on the computer | bearer, `write:runtime` | orange or red | provider calls | about 150 lines |
| 2 | Device refresh and narrow scope: `POST /auth/device/refresh`, or an expiring key with a refresh token; `begin` takes `scopes` | auth | refresh secret | red | auth and session renewal | about 250 lines and a migration |
| 3 | Block-addressed prompt: accept `focus.selection` and an optional `block_id` for the Texture desk, pass them to revise and to the desk prompt | textureowner | unchanged | orange | Texture canonical writes | about 60 lines |
| 4 | Audio and time span in the page format (extend `embed` or add a block; speech block) | texturedoc, asset store | unchanged | orange or red | Texture format, publish gate | not estimated; follows step 0.1 |
| 5 | A named device record for the sessions list and revoke (lane S1) | auth | cookie | red | auth | with 2 |
| 6 | A turn-record upload route | computer | bearer | orange | tape | not estimated |
| 7 | APNs relay (lane X) | new | device | orange | none yet | not needed for the loop |

## 9. What the phone can do today with zero server changes

1. `POST /auth/device/begin`, open `verification_uri_complete`, poll
   `POST /auth/device/token`, store the key.
2. `GET /api/platform/publications/front` and `.../resolve?route=` for
   stories as `body_doc` and citations.
3. `POST /api/texture/fork` with `{source{publication_version_id,
   manifest_hash}, text, client_request_id}` for the listener's own
   document.
4. `POST /api/prompt-bar` with a Texture `focus` and `object_ref` the
   `doc_id`; then `GET /api/texture/documents/{id}/stream?format=json&after=`
   and `GET /api/texture/revisions/{id}`.
5. `POST /api/precommit/claims` and `/settlements`; poll `/api/changelog`.

It cannot call Jev, address a block, carry clip metadata in a document, or
hold a refreshable device record.

## 10. Open questions

- Does "their edition" mean the paper's global stories or Textures on the
  listener's own computer? The code has no per-account edition API.
- Does the proxy forward `/api/ws` and SSE for bearer-only clients? Only
  the HTTP bearer path was read.
- Is the browser page at `/device` served on staging? Not fetched.
- Does the desk's revision keep `block_id`s? No writer that preserves them
  was found.

## Decisions (orchestrator, 2026-11-10)

1. **Jev for the phone goes through the listener's own computer**, not a
   new proxy-to-gateway edge. It is a runtime route on the computer that
   calls the existing in-guest System 1 path with the interactive scope and
   records the judgment in the Jev record there. Addition 1 above is read
   this way.
2. **The first end-to-end loop uses only calls that exist today**: device
   sign-in, front page and resolve, fork, the prompt bar with a Texture
   focus, the document stream and precommit claims. The place in the
   document is carried in the prompt text until block-addressed prompts
   (addition 3) exist.
3. **The device key's lack of expiry, refresh and narrow scope is a known
   gap for lane S1.** It is acceptable only while the app is installed from
   Xcode on the owner's phone.
