# Choir CLI and MCP: One Registry, Two Surfaces, and the Hosted MCP

Date: 2026-10-14. Branch `cli-mcp-parity` (from `calibration-level2`,
rebased on `origin/main`). Not pushed, not deployed.

Owner (2026-10-14 ~21:50 UTC): "the Choir MCP should be pretty much
feature identical with the CLI. MCP and the CLI should basically be the
same thing, just for a different type of service, because some agents
prefer MCP to CLI. You can connect MCPs to a regular ChatGPT or Claude
chat, even though they can't do CLI without code execution."

Agreed direction: (1) parity by construction: one command registry
generates both surfaces, and a test fails on drift; (2) chat apps need a
hosted remote MCP (Streamable HTTP plus OAuth 2.1), not the local stdio
server.

Mutation class: **orange** for the CLI and stdio MCP change (client
runtime behaviour; no server route, store or auth change; rollback is a
`git revert` of the code commit). The hosted MCP in §5 is **red** (auth,
sessions, a new public endpoint) and is a design only.

## 1. Result

`choir` has 59 commands. All 59 are declared once in a registry
(`cmd/choir/commands.go`):

| Surface | Count | Commands |
|---|---|---|
| CLI and MCP | 51 | every read and write command below not listed as CLI-only |
| CLI only, with a reason | 8 | `texture watch`, `self-dev wait`, `trace export`, `precommit import-ledger`, `api-key create`, and the meta commands `mcp`, `version`, `help` |
| MCP only | 0 | — |

The CLI's flag parsing, positional arguments, validation, top-level
usage, group usage and per-command `--help` are generated from the
registry. The MCP server's `tools/list` (input schemas, annotations,
scopes) and `tools/call` (argument decoding and validation, the handler,
the result) are generated from the same entries. Each command has one
handler that calls the HTTP API and returns structured output; the CLI
prints it as JSON, and MCP returns it as a text block plus
`structuredContent`.

Before: the CLI had 59 hand-written commands and the stdio MCP server had
4 hand-written tools (the precommit ones) with their own schemas and
argument structs. After: 51 tools, and drift between the surfaces fails
`go test ./cmd/choir`.

## 2. Inventory

Effects: **R** read-only, **W** writes, **D** destructive (replaces,
closes, stops or applies something), **I** idempotent (a retry with the
same key or arguments has no further effect), **L** long-running or
streaming, **F** reads or writes a local file. Scopes are the API key
scopes the proxy checks (`internal/proxy/handlers.go`
`requiredAPIKeyScope`, `internal/proxy/self_development.go`,
`internal/proxy/computer_lifecycle.go`); `public` means the route needs
no scope, `authenticated` means any valid key.

### Trajectories and lifecycle

| CLI | MCP tool | Route(s) | Inputs | Output | Effects | Scopes |
|---|---|---|---|---|---|---|
| `trajectories` | `trajectories_list` | GET `/api/trajectories` | — | `{trajectories}`, first 50 | R | read:runtime |
| `trajectory <id>` | `trajectory_get` | GET `/api/trajectories/{id}` | trajectory_id | lifecycle snapshot or obligations view | R | read:runtime |
| `trajectory cancel <id>` | `trajectory_cancel` | POST `/api/trajectories/{id}/cancel` | trajectory_id, idempotency_key, expected_lifecycle_version, expected_head_revision_id, reason | cancel result | W D I | write:runtime |
| `lifecycle snapshot <id>` | `lifecycle_snapshot` | GET `/api/trajectories/{id}` (must be `choir.durable_work.v1`) | trajectory_id | snapshot | R | read:runtime |
| `lifecycle events <id>` | `lifecycle_events` | GET `/api/trajectories/{id}/events?after=&limit=` | trajectory_id, after, limit | event page | R | read:runtime |
| `lifecycle capsule-evidence <t> <a>` | `lifecycle_capsule_evidence` | GET `/api/trajectories/{t}/capsule-evidence/{a}?attempt=` | trajectory_id, assignment_id, attempt | capsule evidence (schema checked) | R | read:runtime |
| `identity` | `identity_verify` | GET `/api/acceptance/execution-identity?nonce=` | — (client nonce) | envelope, verified client-side: guest Ed25519 receipt, platform join receipt against the repo trust manifest, one commit across host, guest, route and deploy | R | acceptance:read |

### Texture

| CLI | MCP tool | Route(s) | Inputs | Output | Effects | Scopes |
|---|---|---|---|---|---|---|
| `texture create` | `texture_create` | POST `/api/texture/lifecycle-documents` | title, content, request_id, desk | `choir.texture_create.v1` | W I (starts desk work) | write:texture |
| `texture revise <doc>` | `texture_revise` | POST `/api/texture/documents/{doc}/revise` | doc_id, prompt, expected_head, request_id | revise receipt | W | write:texture |
| `texture read <doc>` | `texture_read` | GET `/api/texture/documents/{doc}` | doc_id | metadata | R | read:texture |
| `texture history <doc>` | `texture_history` | GET `.../{doc}/history` | doc_id | revision metadata | R | read:texture |
| `texture revisions <doc>` | `texture_revisions` | GET `.../{doc}/revisions` | doc_id | revisions with bodies | R | read:texture |
| `texture show <doc>` | `texture_show` | GET `.../{doc}`, GET `/api/texture/revisions/{rev}` | doc_id, revision | `choir.texture_show.v1` | R | read:texture |
| `texture watch <doc>` | CLI only | GET `.../{doc}/events?after=&limit=` in a loop | after, limit, once, poll-interval, reconnect-attempts | JSONL until a terminal event | R L | read:texture |
| `texture open-source <doc>` | `texture_open_source` | GET `.../{doc}/source-open?revision_id=&source_ref_id=&source_ref_version_id=` | doc_id, revision, source_ref, source_ref_version | `choir.texture_source_open.v1` | R | read:texture |

### Search and runs

| CLI | MCP tool | Route(s) | Inputs | Output | Effects | Scopes |
|---|---|---|---|---|---|---|
| `search <query...>` | `search` | GET `/api/platform/retrieval/search?q=` | query | results | R | public |
| `run start <text...>` | `run_start` | POST `/api/prompt-bar` | text, idempotency_key | durable-work start receipt (checked complete) | W I (starts a run: model and search calls) | write:runtime |
| `run status <id>` | `run_status` | GET `/api/runs/{id}` | run_id | run | R | read:runtime |
| `run list` | `run_list` | GET `/api/runs?limit=` | limit 1..500 | runs | R | read:runtime |
| `run cancel <id>` | `run_cancel` | POST `/api/runs/{id}/cancel` | run_id | result | W D I | write:runtime |

### Files (guest store)

| CLI | MCP tool | Route(s) | Inputs | Output | Effects | Scopes |
|---|---|---|---|---|---|---|
| `files mkdir <path>` | `files_mkdir` | POST `/api/files/{path}` | path | result | W I | write:runtime |
| `files put <path>` | `files_put` | PUT `/api/files/{path}` (octet-stream) | path; CLI: stdin or `--local` (F); MCP: `content` | result | W D I (creates or replaces) | write:runtime |
| `files get <path>` | `files_get` | GET `/api/files/{path}` | path | raw bytes: CLI stdout; MCP text, or a base64 `resource` blob when not UTF-8 | R | read:runtime |

### Computer lifecycle

Every command takes a required `computer` (the stable ComputerID), which
is also sent as `X-Choir-Computer`.

| CLI | MCP tool | Route(s) | Other inputs | Effects | Scopes |
|---|---|---|---|---|---|
| `computer status` | `computer_status` | GET `.../lifecycle/status` | — | R | computer:lifecycle |
| `computer replay-completeness` | `computer_replay_completeness` | GET `.../self-development/replay-completeness` | — | R | computer:self_development:read |
| `computer replace-workspace` | `computer_replace_workspace` | POST `.../lifecycle/replace-workspace` | — | W D | computer:lifecycle |
| `computer rematerialize-from-tape` | `computer_rematerialize_from_tape` | POST `.../lifecycle/rematerialize-from-tape` | CLI `--checkpoint-file` (F); MCP `checkpoint` object | W D | computer:lifecycle |
| `computer checkpoints` (was `computer checkpoint`, a manual bind, deleted 2026-10-26) | `computer_checkpoints` | GET `.../lifecycle/checkpoints` | `--digest` | R | computer:lifecycle |
| `computer import-residue-snapshot` | `computer_import_residue_snapshot` | POST `.../lifecycle/import-residue-snapshot` | — | W (appends one tape snapshot) | computer:lifecycle |
| `computer restore` | `computer_restore` | POST `.../lifecycle/restore` (operand scopes vm_local, computer_surface_frontend) | CLI `--checkpoint-file` (F); MCP `checkpoint` object | W D | computer:lifecycle |
| `computer bootstrap-chain` | `computer_bootstrap_chain` | POST `.../lifecycle/bootstrap-chain` | — | W D (one-way genesis) | computer:lifecycle |
| `computer producer-reports` | `computer_producer_reports` | GET `.../lifecycle/producer-reports?agent_id=&stale_delivered=` | agent_id, stale_delivered | R | computer:lifecycle |
| `computer settle-producer-reports` | `computer_settle_producer_reports` | POST `.../lifecycle/settle-producer-reports` | command_id, trajectory, report_ids, reason | W D I | computer:lifecycle |
| `computer stop` / `restart` / `refresh` | `computer_stop` / `_restart` / `_refresh` | POST `.../lifecycle/{action}` | idempotency_key | W D I | computer:lifecycle |
| `computer start` / `recover` | `computer_start` / `_recover` | POST `.../lifecycle/{action}` | idempotency_key | W I | computer:lifecycle |

(`computer recover` was dispatched but missing from the old usage text;
the generated usage lists it.)

### API keys

| CLI | MCP tool | Route(s) | Inputs | Output | Effects | Scopes |
|---|---|---|---|---|---|---|
| `api-key list` | `api_key_list` | GET `/auth/api-keys` | — | key metadata (no secrets) | R | authenticated |
| `api-key create` | CLI only | POST `/auth/api-keys` | label, scopes, computer (attenuation), expires_at | the new key **with its secret** | W | manage:keys |
| `api-key revoke <id>` | `api_key_revoke` | DELETE `/auth/api-keys/{id}` | key_id | `{"revoked": id}` | W D I | manage:keys (or the key itself) |

### Self-development

Every command takes a required `computer`.

| CLI | MCP tool | Route(s) | Other inputs | Effects | Scopes |
|---|---|---|---|---|---|
| `self-dev list` | `self_dev_list` | GET `.../self-development/operations?state=` | state[] | R | :read |
| `self-dev show` | `self_dev_show` | GET `.../operations/{op}` | operation | R | :read |
| `self-dev head` | `self_dev_head` | GET `.../head` | — | R | :read |
| `self-dev patch` | `self_dev_patch` | GET `.../operations/{op}/source-patch` | operation; `--out` CLI only (MCP inlines up to 256 KiB); sha256 verified client-side | R | :read |
| ~~`self-dev start`~~ | ~~`self_dev_start`~~ | removed 2026-10-14: the route was deleted in C1e B1 (a06191f9e); a change starts with `run start` on an armed computer | | | |
| `self-dev wait` | CLI only | polls GET `.../operations/{op}` | operation, state[], interval, deadline (2h) | R L | :read |
| `self-dev approve` | `self_dev_approve` | GET op, GET head, GET mode, PUT mode (arm `accept_once`), POST `.../operations/{op}/decision` | operation, idempotency_key, window | W D I (applies the candidate) | :read, :mode, :approve |
| `self-dev reject` | `self_dev_reject` | GET op, GET head, POST decision | operation, idempotency_key, reason | W D I | :read, :approve |
| `self-dev mode get` | `self_dev_mode_get` | GET `.../mode` | — | R | :read |
| `self-dev mode set` | `self_dev_mode_set` | PUT `.../mode` (generation CAS) | mode, expected_generation, idempotency_key, and the accept_once / qualified_consensus binding fields | W D I | :mode |
| `release list` | `release_list` | GET `/api/releases?limit=&publisher_computer_id=` | limit, publisher | R | :read |
| `release show` | `release_show` | GET `/api/releases/{id}` | release; record verified against its id client-side | R | :read |
| `release adoptions` | `release_adoptions` | GET `.../self-development/adoptions[/{aid}]` | adoption | R | :read |
| `release publish` | CLI only (owner act) | POST `/api/computers/{id}/releases` | operation, test_command | W I | :approve, owner:acts |
| `release adopt` | CLI only (owner act) | GET `/api/releases/{id}` (verified), POST `.../self-development/adoptions` | release | W I | :read, :approve, owner:acts |

(`:read` is `computer:self_development:read`, and so on.)

### Trace, precommitments, meta

| CLI | MCP tool | Route(s) | Inputs | Output | Effects | Scopes |
|---|---|---|---|---|---|---|
| `trace export <traj>` | CLI only | about a dozen product GET routes (trajectory, events, runs and run events, Texture revisions, runtime obligations, census and observability, captures, inboxes, publication resolve) | trajectory_id, out | a 0600 bundle file plus a summary | R F (paged) | read:runtime, read:texture |
| `precommit add` | `precommit_add` | POST `/api/precommit/claims` | claim, p, type, criterion, deadline, project, tags, agent, model, session_id, idempotency_key | the claim | W | precommit:write |
| `precommit settle <id>` | `precommit_settle` | POST `/api/precommit/claims/{id}/settlements` | claim_id, outcome, evidence[], method, scorer, scorer_model, note | the claim | W (append-only) | precommit:write |
| `precommit list` | `precommit_list` | GET `/api/precommit/claims` | agent, all, model, type, project, status, limit | claims | R | precommit:read |
| `precommit calibration` | `calibration_summary` | GET `/api/precommit/calibration` | filters, scorer, group_by, include_reconstructed; CLI `--text` | summary; the pasteable track record is MCP's first text block | R | precommit:read |
| `precommit import-ledger` | CLI only | reads a local Markdown ledger; with `--apply`, POST claims and settlements | file, model, agent_prefix, project, apply | the import plan | W F | precommit:write |
| `mcp`, `version`, `help` | CLI only | — | — | — | — | — |

### Why the eight are CLI-only

Each reason is in the registry and printed by `--help`.

- **`texture watch`**: streams JSONL until a terminal event. An MCP
  `tools/call` returns once, and Claude caps a call at 240 seconds. Plan:
  a resource subscription on the hosted server.
- **`self-dev wait`**: polls for up to two hours. An agent polls
  `self_dev_show` instead.
- **`trace export`**: writes an unredacted owner bundle to a local 0600
  file. Returned inline it would put megabytes into model context (Claude
  caps a result at about 150k characters), and a hosted server has no
  local file. Plan: a resource `choir://trajectories/{id}/trace-bundle`.
- **`precommit import-ledger`**: a one-time migration of a local
  Markdown file that runs without a key in a dry run. Agents record
  claims with `precommit_add`.
- **`api-key create`**: returns a new bearer secret. A tool result would
  put a live credential into model context and the chat transcript, where
  a prompt-injected call could mint one and send it somewhere. Hosted
  access comes from OAuth consent, never from minting keys.
- **`mcp`, `version`, `help`**: the server itself; `initialize`'s
  `serverInfo`; `tools/list`.

Parameter-level exceptions, also declared with reasons:

- **On the CLI only:** `files put --local` (MCP takes `content`),
  `--checkpoint-file` on `restore` and `rematerialize-from-tape` (MCP
  takes a `checkpoint` object), and `precommit calibration --text` (MCP's
  first text block is the same text).
- **MCP server configuration, never tool arguments:** `--host`,
  `--api-key-file` and `--timeout`. A model-chosen host could send the key
  anywhere. The shared `--computer` flag is the one client setting that
  is also a tool argument: an optional `computer` on every tool that does
  not already require one.

## 3. The registry

`cmd/choir/registry.go` holds the types and both generators;
`cmd/choir/commands.go` holds the declarations.

A **command** declares:

- path (`["self-dev","mode","set"]`) and MCP tool name (defaults to the
  path joined with `_`; the four precommit tools keep their earlier names);
- summary and description;
- typed **parameters**;
- the API key **scopes** it needs;
- the hints `readOnly`, `destructive`, `idempotent` and `openWorld`;
- an optional CLI usage message (which keeps the old wording);
- `cliOnly` with a reason;
- one **handler** `func(*call) (any, error)`;
- optional CLI `render` (raw bytes, `--text`, the compact revoke line) and
  MCP `lead` (a text block before the JSON) overrides.

The CLI-only commands keep their hand-written runners (`legacy`), which
the test allows only together with a `cliOnly` reason.

A **parameter** declares:

- name: the MCP property, and the CLI flag in kebab case unless `flag`
  overrides it (`--tag` for `tags`, `--key` for `idempotency_key`,
  `--session` for `session_id`, keeping the existing flags);
- kind: string, int, number, bool, duration, string list, or JSON object;
- required, enum, min and max, default, and an environment default
  (non-secret strings only: `$CHOIR_AGENT`, `$CHOIR_MODEL`,
  `$CHOIR_SESSION`, `$CHOIR_PROJECT`, `$CHOIR_COMPUTER_ID`);
- positional mode: one argument, all remaining joined (search text, run
  text, a claim), or all remaining appended to a list (report ids);
- comma splitting for repeatable lists;
- `onlyCLI` or `onlyMCP` with a reason.

**CLI generation.** One `flag.FlagSet` per command: the parameters'
flags, then the shared client flags (`--api-key-file`, `--host`,
`--computer` unless the command owns it, `--timeout`). Commands whose last
positional argument is free text stop flag parsing at the first
positional, as before. Every other command accepts flags before and after
its positional arguments, as `precommit settle` and `trace export`
already did. Validation runs on positional arguments first, then flags,
with the old messages (`trajectory id required`, `--computer is
required`). A handler error is exit 1 and names the object (`choir run
status run-1: http 404: ...`); a usage error is exit 2. `--help` prints
the description, the scopes, the MCP tool name and whether it reads or
writes, then the flags. A group (`choir computer`, `choir self-dev mode`)
lists its subcommands from the registry.

**MCP generation.**

- **`inputSchema`**: a closed JSON Schema object (`additionalProperties:
  false`) with types, descriptions, enums, ranges and defaults; a
  duration is a string with a pattern.
- **`annotations`**:
  - `title` and all four hints, always explicit booleans. ChatGPT
    requires explicit booleans, and the spec's implicit defaults are
    `destructiveHint: true` and `openWorldHint: true`.
  - A read-only tool is `destructiveHint: false` and
    `idempotentHint: true`.
  - Claude's directory states the effect: "Read-only tools can run
    without per-call confirmation, and destructive tools always prompt"
    ([review criteria](https://claude.com/docs/connectors/building/review-criteria)).
- **`_meta`**: `choir.news/scopes` and `choir.news/cli`, the CLI command
  the tool is.
- **`tools/call`**:
  - Decodes the arguments strictly. Unknown names, wrong types (a string
    where an integer belongs), missing required values, enum and range
    violations are each an `isError` result, before any request is made.
  - Applies the same defaults as the CLI.
  - Copies the server's client, swapping in a `computer` argument as the
    target header, and runs the handler.
  - Returns the pretty JSON as text, the same JSON as
    `structuredContent` when it is an object, and raw files as text or a
    base64 blob.

**Safety rule in the handlers.** Every model-written id is escaped as one
path segment (`url.PathEscape`). Guest-store paths are escaped segment by
segment, and `.` and `..` are refused. `trajectory get` and `api-key
revoke` concatenated raw ids before; they now escape.

## 4. Behaviour changes in the existing CLI

Flags, defaults, output and exit codes of every existing command are
kept. All 66 existing tests pass unchanged. Intentional differences:

1. **Flags after a positional id now apply.** Example:
   `choir texture read doc --computer X`. Before, they were silently
   ignored, so the request went to the default computer.
2. **Unexpected extra positional arguments are refused** (exit 2).
   Before, some commands ignored them.
3. **Values are checked locally against enums**: claim `type`, settle
   `outcome` and `method`, list `status`, calibration `scorer`, and
   `self-dev mode set --mode`. A bad value is now exit 2 before any
   request; before, it was the server's 400 and exit 1.
4. **The `lifecycle` subcommands accept only their own flags.** Before,
   all three accepted `--after`, `--limit` and `--attempt`.
5. **`settle-producer-reports --report-ids` is repeatable** as well as
   comma-separated.
6. **Help is generated.** Top-level usage lists every command with its
   positional synopsis; `choir <group> --help` prints the group.
7. **The stdio MCP precommit tools changed in three ways:**
   - `precommit_list` defaults to `status: all, limit: 50`, as the CLI
     does; before, it omitted both.
   - `precommit_add` requires `agent` and `model` (or the server's
     `$CHOIR_AGENT` and `$CHOIR_MODEL`) locally, as the CLI does.
   - `initialize` also answers `2025-11-25`.

## 5. The hosted MCP (design; not built here)

### 5.1 What the clients need today (verified 2026-10-14)

- **Spec.** The latest revision is **2026-07-28**, and it is stateless:
  - no `initialize` and no `Mcp-Session-Id`;
  - each request carries its protocol version and capabilities in
    `_meta`, and servers must implement `server/discover`;
  - new `Mcp-Method` and `Mcp-Name` headers;
  - no GET stream;
  - Dynamic Client Registration (DCR) deprecated in favour of Client ID
    Metadata Documents (CIMD).

  Sources: [changelog](https://modelcontextprotocol.io/specification/2026-07-28/changelog),
  [transport](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http).
  Claude and ChatGPT document their clients against **2025-11-25**: the
  `initialize` handshake and the 2025-11-25 authorization spec. Claude's
  2026-07-28 support "is being rolled out"
  ([Anthropic](https://claude.com/blog/bringing-mcp-2026-07-28-to-claude)).
  The server must therefore answer both, on one endpoint. Choir's tools
  are already stateless request/response, so neither revision needs
  server-side state.
- **Claude** (claude.ai, Desktop, mobile, Cowork; [build](https://claude.com/docs/connectors/building),
  [auth](https://claude.com/docs/connectors/building/authentication),
  [help](https://support.claude.com/en/articles/11175166-getting-started-with-custom-connectors-using-remote-mcp)):
  - **Transport:** Streamable HTTP; the old SSE transport is being
    deprecated.
  - **OAuth:** DCR or CIMD. CIMD is used only if the authorization server
    metadata has `client_id_metadata_document_supported: true` **and**
    `none` in `token_endpoint_auth_methods_supported`.
  - **Flow:** PKCE S256 always; a real **401** to start; only the first
    entry of `authorization_servers` is used; the protected resource
    metadata's `resource` must equal the URL as the user typed it.
  - **Token endpoint:** form-urlencoded, rotating refresh tokens, 10s
    endpoint and 30s refresh budgets.
  - **Callback:** `https://claude.ai/api/mcp/auth_callback`; Claude Code
    uses a loopback redirect, with metadata at
    `https://claude.ai/oauth/claude-code-client-metadata`.
  - **Network and limits:** egress from `160.79.104.0/21`, about 150k
    characters per result, 240 seconds per call.
  - **Not supported:** resource subscriptions, sampling; elicitation is
    not listed as supported.
  - **Plans:** custom connectors on Free (one), Pro, Max, Team and
    Enterprise.
  - **Directory:** a `title` on every tool and an explicit
    `readOnlyHint` / `destructiveHint`; no catch-all tools that mix read
    and write.
- **ChatGPT** (the Apps SDK is now "Plugins"; [MCP server](https://developers.openai.com/plugins/build/mcp-server),
  [auth](https://developers.openai.com/plugins/build/auth),
  [guidelines](https://developers.openai.com/plugins/plugin-guidelines)):
  - **Transport:** Streamable HTTP at a public HTTPS URL.
  - **OAuth:** 2.1 against the 2025-11-25 auth spec. **CIMD preferred**
    (client id `https://chatgpt.com/oauth/client.json` or a per-callback
    variant), DCR as a fallback.
  - **Flow:** PKCE S256; `resource` on both the authorization and token
    requests, carried in the token's `aud`.
  - **Redirects:** `https://chatgpt.com/connector_platform_oauth_redirect`
    or `https://chatgpt.com/connector/oauth/{callback_id}`.
  - **Calls:** made with an mTLS client certificate from OpenAI's
    connector platform.
  - **Annotations:** `readOnlyHint`, `destructiveHint` and
    `openWorldHint` are required as explicit booleans and drive
    confirmation.
  - **Plans:** not fully verified (help.openai.com refused the fetch).
    Search snippets of
    [the developer-mode article](https://help.openai.com/en/articles/12584461-developer-mode-and-mcp-apps-in-chatgpt)
    say full MCP with writes is beta on Business, Enterprise and Edu.
    Treat this as unconfirmed.

### 5.2 Endpoint

`https://choir.news/mcp`, served by the proxy (which already
authenticates and scope-gates `/api/*`):

- **Methods and responses:**
  - POST only, one JSON-RPC message per request.
  - JSON responses (no tool streams, so SSE is unnecessary).
  - A notification gets `202`.
  - GET and DELETE get `405`.
- **Both protocol revisions:**
  - `initialize`, `tools/list` and `tools/call` for 2025-11-25 clients.
  - `server/discover`, plus `ttlMs` and `cacheScope` on list results,
    for 2026-07-28 clients.
  - `MCP-Protocol-Version` checked against the body (`400` on mismatch).
- **Origin:** validated. Server-to-server calls carry none; a browser
  `Origin` other than an allowlisted Choir origin gets 403.
- **No session state.** No `Mcp-Session-Id`, and no handle is treated as
  authentication.
- **Same registry.** The server runs the same registry handlers. Their
  `client` uses an **in-process transport**: a `RoundTripper` that hands
  each request to the proxy's own handler with the authenticated
  principal (user, scopes, computer grant, OAuth client id) attached as
  trusted context. The OAuth token never crosses a hop, which satisfies
  the spec's ban on token passthrough. Every proxy scope check,
  computer-target check and route stays in force, so the hosted tools
  are the CLI's commands by construction.
- **Prerequisite:** move the registry and client into an importable
  package (`internal/choirsurface`). That is a mechanical move; this
  branch keeps them in `cmd/choir`.

### 5.3 Authorization

The auth service is the authorization server; `/mcp` is the protected
resource.

- **Protected resource metadata** (RFC 9728) at
  `/.well-known/oauth-protected-resource/mcp`, and at the root:
  `resource: https://choir.news/mcp`,
  `authorization_servers: [https://choir.news]`, `scopes_supported`, and
  `bearer_methods_supported: [header]`. A missing or invalid token gets
  `401` with `WWW-Authenticate: Bearer
  resource_metadata=".../oauth-protected-resource/mcp", scope="choir:read"`.
- **Authorization server metadata** (RFC 8414) at
  `/.well-known/oauth-authorization-server`:
  - endpoints: `issuer`, `authorization_endpoint` `/oauth/authorize`,
    `token_endpoint` `/oauth/token`, `registration_endpoint`
    `/oauth/register` (the DCR fallback Claude and ChatGPT still use),
    `revocation_endpoint`;
  - `code_challenge_methods_supported: [S256]`;
  - `grant_types_supported: [authorization_code, refresh_token]`;
  - `token_endpoint_auth_methods_supported: [none]`;
  - `client_id_metadata_document_supported: true`;
  - `authorization_response_iss_parameter_supported: true`, with `iss`
    on every authorization response, including errors.
- **Clients.**
  - **CIMD first.** Fetch `https://...` client ids with an SSRF guard:
    https only, public addresses only, size and time limits, cached.
  - **DCR as the fallback.** Rate-limited per IP, with `application_type`
    required.
  - **Redirect URIs** match exactly; loopback redirects are allowed only
    for native clients.
- **Tokens.** The access token is an opaque `choir_oat_...` stored hashed
  in the existing API key store, as a new kind with:
  - owner, scopes and computer grant;
  - `aud = https://choir.news/mcp`, `client_id`, a one-hour expiry, and
    its parent refresh token.

  Refresh tokens rotate on every use, and reuse revokes the family.
  `/api/*` refuses `choir_oat_` tokens (audience binding), and `/mcp`
  refuses `choir_sk_` keys. API keys stay the CLI and stdio credential,
  and OAuth tokens are only for the hosted endpoint.
- **Scopes: consent bundles over the existing API key scopes**, kept
  small in `scopes_supported`:

  | OAuth scope | Grants | Tools |
  |---|---|---|
  | `choir:read` | read:texture, read:runtime, precommit:read, computer:self_development:read, acceptance:read | every read-only tool |
  | `choir:write` | write:texture, write:runtime, precommit:write | create and revise documents, start and cancel runs, files, precommitments |
  | `choir:computer` | computer:lifecycle | status, checkpoint, restore, stop, start, refresh, ... |
  | `choir:self-dev` | computer:self_development:propose (and :mode, :approve only with step-up, below) | self-dev start, and decide |

  `admin` and `manage:keys` are never grantable through OAuth;
  `api_key_revoke` is the one key tool and needs `manage:keys`, so it is
  not offered on the hosted server. `tools/list` is filtered to the
  token's scopes; the spec allows the list to vary by token scope but not
  per connection. A call needing more gets `403
  insufficient_scope` naming every scope it needs in one challenge, so
  the client can step up.
- **Passkey login inside `/oauth/authorize`.**
  1. The authorize request (client id, redirect URI, PKCE challenge,
     `resource`, scope, state) is validated, then stored server-side
     under a short-lived id.
  2. With no Choir session, the browser goes to the existing WebAuthn
     login (`/auth/login/begin|finish`) with a return path; the
     passkey ceremony is unchanged.
  3. With a session, the consent page shows:
     - the client's name and redirect host (from CIMD or DCR);
     - each requested bundle in plain words;
     - a **computer picker**: one computer (the default) or every
       computer the owner has.
  4. The page is non-frameable (`frame-ancestors 'none'`), uses a
     `__Host-` CSRF cookie and a single-use `state`.
  5. Granting `choir:computer` or `choir:self-dev` needs a **fresh
     passkey assertion** (user verification) on the consent page, not
     just a live session.
  6. The code is single-use, valid for 60 seconds, and bound to the PKCE
     challenge, client id, redirect URI and `resource`.

  The existing desktop exchange (`/auth/desktop/exchange-redirect` →
  `/auth/desktop/redeem`) is the in-repo precedent for a one-time,
  user-bound code. The OAuth code follows the same pattern, with PKCE.
- **Per-computer targeting.** A token granted one computer behaves like
  a computer-attenuated API key: a tool's `computer` argument must name
  that computer (the proxy's `requireAPIKeyComputerTarget` check). An
  owner-wide grant accepts any computer the owner has. The consent screen
  says which.
- **Connected apps.** A settings page lists each grant (client, scopes,
  computer, last use) and revokes it, which revokes the token family.

### 5.4 Rate limits and audit

- **Rate limits.**
  - Token buckets per token and per owner: reads about 120 a minute,
    writes about 20 a minute, destructive calls about 5 a minute and 30
    an hour.
  - `/oauth/register` and `/oauth/token` per IP, reusing
    `auth.IPRateLimiter`.
  - CIMD fetches per host.
  - A refused call is an `isError` result saying when to retry, not a
    transport failure.
- **Audit.**
  - Every `tools/call` writes one record: owner, client id and name,
    token id, tool, computer, argument digest, outcome, HTTP status and
    latency.
  - For writes, the arguments themselves are kept as well (minus file
    content).
  - The target computer already records the act on its own tape through
    the product route. The audit record is the platform-side join, and
    it holds the client id that tape does not.
  - Owners see the log on the connected-apps page.

### 5.5 Security review points

1. **Prompt-injected tool calls.** A chat's context holds untrusted
   text (web pages, mail, other connectors' results), and the model may
   be steered into calls the owner did not intend. Mitigations:
   - **Hints that make clients ask.** Accurate annotations mean Claude
     always prompts for a destructive tool and ChatGPT confirms
     consequential writes. The registry test proves the hints are
     explicit and consistent with each tool's scopes.
   - **Scope minimisation.** Consent defaults to `choir:read`. Writes,
     computer lifecycle and self-development are separate bundles, the
     last two behind a fresh passkey.
   - **Some acts stay owner acts.** In the first hosted release,
     `self_dev_approve`, `self_dev_mode_set`, `computer_restore`,
     `computer_bootstrap_chain` and `computer_rematerialize_from_tape`
     are not offered over OAuth at all; they stay owner acts in the
     Choir UI or the CLI. A later release can offer them through a
     confirmation the owner gives on choir.news: the tool returns a
     one-time approval URL in the spirit of URL-mode elicitation, and
     the act runs only after a passkey assertion there. Elicitation is
     not relied on, because Claude does not list it as supported. This
     is the one place where hosted and CLI differ on purpose. The
     registry will mark it per tool, with a `hosted` exclusion and a
     reason enforced by the parity test (H2).
2. **Write confirmation.** Hints are advisory, and the spec says
   clients must treat them as untrusted, so the server does not rely on
   them alone:
   - idempotency keys make retries safe;
   - destructive rate limits bound the damage;
   - the audit log makes every write attributable to a client.
3. **Exfiltration through reads.** Read results flow into a chat that
   may also hold a mail or web connector: the "lethal trifecta".
   Mitigations:
   - one-computer grants by default;
   - no secrets in any tool output (`api-key create` is CLI-only, and
     `api_key_list` returns metadata only);
   - result size caps under Claude's 150k characters;
   - read volume in the audit log.
4. **Model-written arguments.** The key, host and timeout are never
   arguments, ids are escaped per segment, and `..` is refused (both are
   tested). The hosted server takes no URL arguments, so a tool cannot
   be turned into an SSRF probe; the CIMD fetcher is the one outbound
   fetch, and it is guarded.
5. **Tokens.**
   - Audience-bound, short-lived and rotating.
   - Never passed through; the hop is in-process.
   - Never in a query string, and never in logs (only token ids are
     logged).
6. **Confused deputy.** Choir is its own authorization server and does
   not proxy a third party's, so the static-client-id deputy case does
   not arise. Per-client consent, exact redirect matching and single-use
   `state` still apply.
7. **Abuse of DCR and CIMD.**
   - Rate-limited registration.
   - Client names shown with their redirect host on consent, so a
     look-alike name cannot hide its destination.
   - Unconsented registrations expire after a day.

### 5.6 Hosted build plan

Each step goes through the Landing Loop; the whole is red.

| Step | Work |
|---|---|
| H0 | Move the registry and client into `internal/choirsurface`, with no behaviour change |
| H1 | Protected resource and authorization server metadata, `/oauth/authorize` with consent and passkey step-up, `/oauth/token`, `/oauth/register`, CIMD with an SSRF guard, the `choir_oat_` token kind, revocation and the connected-apps page |
| H2 | `/mcp` on the proxy: both protocol revisions, the in-process transport, per-scope `tools/list`, `insufficient_scope`, rate limits, audit |
| H3 | Staging acceptance: add `https://choir.news/mcp` as a custom connector in Claude and in ChatGPT developer mode. Pass when: a read runs without a prompt; a write prompts; a token without `choir:write` gets `insufficient_scope` and steps up; a revoked grant fails; the audit log shows the client |

## 6. Tests

`go test ./cmd/choir` (all 66 existing tests unchanged, plus six new
ones in `cmd/choir/registry_test.go`, failure modes written first):

- **`TestRegistryParityCLIAndMCP`** checks the shape of the registry and
  the two surfaces. It checks that:
  - each command resolves to itself;
  - each has exactly one of a handler or a hand-written runner, and a
    hand-written runner requires a CLI-only reason;
  - tool names are unique and valid;
  - every flag in the **generated** CLI flag set maps to an MCP
    argument, an MCP-server configuration flag, or a CLI-only parameter
    with a reason;
  - every MCP argument maps to a flag or positional, or an MCP-only
    parameter with a reason;
  - no schema exposes `host`, `api_key` or `timeout`;
  - the CLI-only set equals a pinned list, so a new exception needs a
    reason and a test edit;
  - the served `tools/list` equals the registry;
  - the usage names every command.
- **`TestRegistrySchemasAnnotationsAndScopes`** checks, for every
  command and tool:
  - every scope is one the auth service issues (read from
    `internal/auth/handlers.go` `validScopes` by parsing the source);
  - read-only commands need no write scope;
  - schemas are closed objects with valid types, descriptions, required
    properties, enums, defaults and patterns;
  - every hint is an explicit boolean that agrees with the registry;
  - every tool has a title.
- **`TestEveryToolMatchesItsCLICommand`** runs all 51 tools twice
  against a capture server, once through the CLI and once through
  `choir mcp` over stdio, with equivalent inputs. For each tool it
  requires identical HTTP requests (method, escaped path, query, target
  header, canonical body) and identical output: the CLI's JSON equals
  the tool's text and `structuredContent`. A new tool without a case
  fails.
- **`TestMCPProtocolOverStdio`** covers:
  - version negotiation, including 2025-11-25 and an unknown version;
  - no reply to a notification;
  - a CLI-only command refused (`-32602`);
  - `isError` results, with no request made, for: a range violation, an
    unknown `host` argument, a missing required argument, a string where
    an integer belongs, a `..` file path, and an enum violation;
  - an escaped `../runs` id staying in its segment;
  - the `computer` argument becoming `X-Choir-Computer`;
  - `-32601` for an unknown method.
- **`TestMCPFilesGetReturnsBinaryAsABlob`** and
  **`TestCLIGroupsAndHelpComeFromTheRegistry`** cover the remaining
  paths: binary files, group listings, per-command help, and flags after
  a positional id.

Mutation checks run by hand:

- Making the MCP side skip a default fails the equivalence test
  (`precommit_list` request differs).
- Adding an undeclared shared flag fails the parity test (`CLI flag
  --profile is not declared`).

`go build ./...`, `go vet ./cmd/choir` and `go test ./cmd/choir` pass.

## 7. Residuals

- **R1.** The registry lives in `cmd/choir`. The hosted server needs it
  importable (H0).
- **R2.** The stdio server speaks the `initialize` revisions only
  (through 2025-11-25). 2026-07-28 stdio clients (`server/discover`, MRTR)
  are not served yet. The hosted endpoint must serve both (§5.2).
- **R3.** No `outputSchema` is declared. Results are API JSON whose
  schemas live server-side; declaring them would make
  `structuredContent` contractual. The precise way is to generate them
  from the server's response types.
- **R4.** `texture watch` and `trace export` are planned as MCP
  resources; resources are not implemented.
- **R5.** Agent identity is still declared (R1 of the precommitments
  report). On the hosted server the OAuth client id can be bound to the
  claim's agent, and hosted `precommit_add` should take the agent from
  the grant, not from an argument.
- **R6.** Hosted design point to ratify: whether `self_dev_approve` and
  the other owner acts are ever offered over OAuth (§5.5 item 1), and
  through what confirmation.
- **R7.** No staging proof: nothing here is deployed. The CLI change
  ships with the next deploy that includes `cmd/choir`, which has no
  server half.

Heresy delta:

- **Discovered:** `trajectory get` and `api-key revoke` built paths from
  raw ids; flags after a positional id were silently dropped;
  `computer recover` was missing from the usage text.
- **Repaired:** all three.
- **Introduced:** none known.
