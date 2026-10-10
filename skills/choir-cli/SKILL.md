# Choir CLI Skill

## Purpose

The `choir` binary (`cmd/choir`) is the headless control surface for Choir. It
wraps the public `/api/` and `/auth/` HTTP routes with API key (Bearer
`choir_sk_...`) auth so agents and scripts can read Texture documents, observe
trajectories, search, start runs, and manage API keys without a browser.

**Status:** deployed self-development commands are `list`, `show`, `head`,
`start`, `wait`, `approve`, `reject`, and `mode get|set` (2026-10-10). Genesis,
rollback and kernel-capability inspection are API-only (no CLI subcommand yet).
`/goal` remains an external agent-harness invocation, not a CLI command.

## Self-Development Control

Every command targets an explicit stable `ComputerID` (`--computer`). A key
bound to that computer needs the matching scope:
`computer:self_development:read` (list, show, head, mode get, wait),
`:propose` (start), `:approve` plus `:mode` (approve), `:approve` (reject),
`:mode` (mode set).

| Step | CLI surface | Durable evidence |
| --- | --- | --- |
| Arm proposals | `choir self-dev mode set --mode=propose_only --expected-generation=N --idempotency-key=K` | generation-CAS `ModeReceipt` |
| Start a proposal | `choir self-dev start --prompt=... --idempotency-key=K` | operation id |
| Find candidates | `choir self-dev list [--state=awaiting_approval]` | operations, newest first |
| Observe | `choir self-dev wait --operation=ID --state=awaiting_approval` | operation once it reaches the state; exits 1 if it settles elsewhere |
| Read the binding | `choir self-dev head` | canonical/desired/effective heads and state commitments |
| Approve | `choir self-dev approve --operation=ID` | consumed `accept_once` ModeReceipt + decision event |
| Reject | `choir self-dev reject --operation=ID --reason=...` | decision event |

Self-development a Texture request opens (Texture -> management -> engineering
freeze) creates its own operation; `list` is how to find it — do not derive the
id by hand. `approve` is the owner's single approval: it reads the frozen
candidate and the computer's event head, arms `accept_once` bound to exactly
that operation, bundle, heads and commitments (15-minute window, `--window`),
and posts the approve decision, which consumes it. It refuses before any write
unless the operation is `awaiting_approval`. Rollback of an applied operation
is `POST /api/computers/<id>/self-development/rollbacks` (no CLI yet).

Effects default to `off`. `accept_once` authorizes only its exact canonical
approval request and returns to `propose_only` before that approval reaches the
guest. A package, adoption, mutable branch, VM, local test, verifier statement,
checkpoint, or route transition alone is not accepted self-development.

Shared GitHub source still lands through commit, push, CI, deploy identity, and
deployed product-path proof. The reviewed G1 source candidate and the later
deployed release are separate identities and both bind genesis.

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
bar uses. `conductor` is deferred to system-one and does not decide current
routing. The current route is the document-channel/desk path: a bound desk
receives the document event and may use delegated `choir.Cast` for a
per-assignment sub-RLM run. The submission often completes synchronously: the
response `state` may already be `completed`, and `run status` then returns the
`decision` (routed app, `doc_id`, revision/loop ids). For a Texture-routed run,
follow up with `choir texture revisions <doc_id>` to read what the appagent
wrote — the appagent revision typically lands within ~10 seconds of submission.
Each run also creates a trajectory visible in `choir trajectories` under the
same id as the submission's channel.

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

A computer-bound key needs `--computer <ComputerID>` on `api-key create`, and
can only delegate scopes it already holds (an agent cannot widen its own
authority). For a separate disposable test computer with its own owner,
`node scripts/qa_test_computer.mjs --label <name>` registers a fresh account
with a virtual passkey on staging, mints a 7-day key bound to that computer
(self-development, lifecycle and texture scopes), and writes it to
`~/.config/choir-qa/test-computers/<name>.json` (mode 0600) plus a browser
storage state for GUI proofs. It prints only non-secret fields; load the key
with `CHOIR_API_KEY=$(jq -r .api_key <file>)` and never echo it.

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
