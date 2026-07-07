# Choir CLI Skill

## Purpose

The `choir` binary (`cmd/choir`) is the headless control surface for Choir. It
wraps the public `/api/` and `/auth/` HTTP routes with API key (Bearer
`choir_sk_...`) auth so agents and scripts can read Texture documents, observe
trajectories, search, start runs, and manage API keys without a browser.

## Building

```sh
go build -o /tmp/choir ./cmd/choir
```

The binary has no cgo dependencies — it is pure Go and builds quickly.

## Auth

All commands require an API key:

- `--api-key` flag, or
- `$CHOIR_API_KEY` environment variable

The key must start with `choir_sk_`. Keys are created via the SettingsApp
"API keys" section in the browser UI, or via `choir api-key create` if you
already have a key.

## Host

- `--host` flag, or
- `$CHOIR_HOST` environment variable (defaults to `https://choir.news`)

## Commands

### Run control

```sh
choir run start "your prompt text here"
# Returns: { submission_id, state, created_at, status_url }

choir run status <submission_id>
# Returns: submission state, decision, error
```

`run start` posts to `/api/prompt-bar` — the same endpoint the browser prompt
bar uses. The conductor decides which agent app to route to. The submission
often completes synchronously: the response `state` may already be
`completed`, and `run status` then returns the `decision` (routed app,
`doc_id`, revision/loop ids). For a Texture-routed run, follow up with
`choir texture revisions <doc_id>` to read what the appagent wrote — the
appagent revision typically lands within ~10 seconds of submission. Each run
also creates a trajectory visible in `choir trajectories` under the same id
as the submission's channel.

### API key management

```sh
choir api-key list
# Returns: { keys: [...] }

choir api-key create --label "Devin CLI" --scopes "read:texture,read:base,read:runtime"
# Returns: { id, label, scopes, secret } — secret is shown once

choir api-key revoke <key_id>
# Returns: { revoked: "<key_id>" }
```

API key management routes accept both cookie auth (browser) and Bearer API key
auth (CLI). The first key must be created via the browser UI; subsequent keys
can be created via CLI.

### Texture

```sh
choir texture read <doc_id>       # metadata: title, current revision id, revision count
choir texture history <doc_id>    # revision list, metadata only (no content)
choir texture revisions <doc_id>  # revisions WITH full content + body_doc JSON
```

`texture read` and `texture history` do not return document content. Use
`texture revisions` when you need the actual text — each entry carries a
plain-text `content` field plus the structured `body_doc`
(`choir.texture_doc.v1`).

### Trajectories

```sh
choir trajectories
choir trajectory <id>
```

### Search

```sh
choir search "query terms"
```

### Universal Wire

```sh
choir wire stories
choir wire diagnostics
```

## Output

All output is JSON to stdout. Diagnostics and errors go to stderr. Exit codes:
- 0: success
- 1: API error
- 2: usage error

## Testing

```sh
go test ./cmd/choir/ -count=1
```

## Known Limits (observed 2026-07-07 against choir.news)

- **Fixed 30s HTTP timeout, no `--timeout` flag.** Fine for every route
  except the wire feed.
- **`/api/universal-wire/stories` can hang server-side** — observed taking
  longer than 120s on production, so `choir wire stories` and
  `choir wire diagnostics` time out. This is a server issue, not a CLI one;
  when it happens the CLI reports `context deadline exceeded`.
- **`trajectories` truncates to 50 entries client-side**; there is no paging
  flag yet.
- **`search` takes no limit/filter flags** — it passes the raw query as `q`.
- **No streaming**: `/api/texture/documents/<id>/stream` exists in the proxy
  but the CLI does not expose it; poll `run status` / `texture revisions`
  instead.

## Architecture Notes

- The CLI avoids importing `internal/runtime` (which needs cgo/ICU) by
  defining local response types that mirror the runtime's JSON shapes.
- The `do` method supports GET, POST, and DELETE with optional JSON body.
- `/api/prompt-bar` is the public run-start endpoint; `/api/agent/*` routes are
  intentionally blocked from public access.
- `/auth/api-keys` routes are served by the auth service and accept both cookie
  and Bearer token auth via `requireAuthUserAny`.
