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
bar uses. The conductor decides which agent app to route to.

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
choir texture read <doc_id>
choir texture history <doc_id>
```

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

## Architecture Notes

- The CLI avoids importing `internal/runtime` (which needs cgo/ICU) by
  defining local response types that mirror the runtime's JSON shapes.
- The `do` method supports GET, POST, and DELETE with optional JSON body.
- `/api/prompt-bar` is the public run-start endpoint; `/api/agent/*` routes are
  intentionally blocked from public access.
- `/auth/api-keys` routes are served by the auth service and accept both cookie
  and Bearer token auth via `requireAuthUserAny`.
