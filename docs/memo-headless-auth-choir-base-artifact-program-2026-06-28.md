# Headless Auth, Choir Base, and the Artifact Program

**Date:** 2026-06-28
**Status:** design doc — implementation plan for delegation
**Related:**
- `docs/memo-artifact-program-doctrine-2026-06-28.md` (the tape, the self-authoring program)
- `docs/choir-base-product-spec-2026-06-06.md` (Base product spec)
- `docs/mission-choir-base-reconciliation-kernel-v0.md` (Base mission doc)
- `docs/vision-choir-category-texture-transclusion-v0.md` (the audited computer)
- `internal/auth/store.go` (current auth schema)
- `internal/proxy/handlers.go` (current JWT validation)

## Problem

The audited computer vision requires that every mutation transaction has an
author. Today, the only auth path is WebAuthn/passkeys — which require a
browser, a user gesture, and a hardware authenticator. This blocks:

- **CLI tools** — `choir-cli sync`, `choir-cli upload` from scripts
- **Desktop background sync** — Choir Base needs to sync files without a
  browser open
- **Agent/programmatic access** — the actor runtime needs to author
  transactions
- **CI/staging integration tests** — can't test the API without a browser
- **Mobile background sync** — iOS/Android can't do background sync with
  WebAuthn
- **Inter-node replication** — nodes need to authenticate to each other

Passkeys are the right auth for humans in browsers. They are the wrong auth
for machines, scripts, agents, and background processes.

## Design

### 1. API Keys for Headless Access

Add an API key system to the existing auth service. API keys are:

- Created by authenticated users (via WebAuthn, one time, in the web UI)
- Long-lived opaque tokens: `choir_sk_<32 bytes base64url>`
- Stored as SHA-256 hashes in the auth database (like refresh tokens)
- Validated via `Authorization: Bearer choir_sk_...` header in the proxy
- Scoped to limit what each key can do
- Revocable by the user at any time

#### Database Schema

Add to `internal/auth/store.go`:

```sql
CREATE TABLE IF NOT EXISTS api_keys (
    id          TEXT PRIMARY KEY,
    user_id     TEXT    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_hash    TEXT    NOT NULL,
    label       TEXT    NOT NULL,
    scopes      TEXT    NOT NULL DEFAULT '[]',
    created_at  DATETIME NOT NULL,
    expires_at  DATETIME,
    last_used_at DATETIME,
    revoked_at  DATETIME
);
CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_expires_at ON api_keys(expires_at);
```

- `id` — UUID, the public identifier for the key (used in list/revoke)
- `key_hash` — SHA-256 of the full `choir_sk_...` token
- `label` — human-readable name ("Desktop sync", "CI staging", etc.)
- `scopes` — JSON array of scope strings
- `expires_at` — NULL means no expiry (user-revocable only)
- `last_used_at` — updated on each successful validation
- `revoked_at` — soft delete timestamp; non-NULL means revoked

#### Scope Model

Scopes are strings that limit what an API key can do. The proxy checks
scopes after authentication. Initial scopes:

```text
read:texture       — read texture documents
write:texture      — write texture documents
read:base          — read base items/blobs/journal
write:base         — write base items/blobs/journal
read:runtime       — read runtime state
write:runtime      — write runtime state (agent transactions)
admin              — full access (all scopes)
```

An API key with `["read:base", "write:base"]` can sync files but cannot
write textures or invoke runtime. An API key with `["admin"]` has full
access. Scopes are validated in the proxy, not in upstream services —
the proxy strips scope information and injects `X-Authenticated-Scopes`
header for downstream services that need it.

#### Endpoints

All endpoints require a valid `choir_access` cookie (WebAuthn-authenticated
session):

```text
POST   /auth/api-keys          — create a new API key
GET    /auth/api-keys          — list user's API keys (without secret)
DELETE /auth/api-keys/{id}     — revoke an API key
```

**POST /auth/api-keys:**

Request:
```json
{
  "label": "Desktop sync",
  "scopes": ["read:base", "write:base"],
  "expires_at": null
}
```

Response (201):
```json
{
  "id": "ak_0123456789",
  "label": "Desktop sync",
  "scopes": ["read:base", "write:base"],
  "created_at": "2026-06-28T12:00:00Z",
  "expires_at": null,
  "secret": "choir_sk_ABCDEF..."
}
```

The `secret` field is only returned once, at creation time. It is never
stored in plaintext — only the SHA-256 hash is persisted.

**GET /auth/api-keys:**

Response (200):
```json
{
  "keys": [
    {
      "id": "ak_0123456789",
      "label": "Desktop sync",
      "scopes": ["read:base", "write:base"],
      "created_at": "2026-06-28T12:00:00Z",
      "expires_at": null,
      "last_used_at": "2026-06-28T14:30:00Z",
      "revoked_at": null
    }
  ]
}
```

**DELETE /auth/api-keys/{id}:**

Sets `revoked_at` to now. Response (204, no body).

#### Proxy Validation

In `internal/proxy/handlers.go`, modify `validateAccessJWT` to also check
for Bearer token auth. The new flow:

```go
func (h *Handler) authenticate(r *http.Request) (*AuthResult, error) {
    // 1. Try cookie-based JWT (browser sessions)
    if result, err := h.validateAccessJWT(r); err == nil {
        return result, nil
    }
    // 2. Try Bearer token (API keys for headless access)
    if result, err := h.validateAPIKey(r); err == nil {
        return result, nil
    }
    return nil, errors.New("no valid authentication")
}
```

`validateAPIKey`:
1. Extract `Authorization: Bearer choir_sk_...` header
2. Hash the token with SHA-256
3. Look up `api_keys` by `key_hash` where `revoked_at IS NULL` and
   (`expires_at IS NULL` or `expires_at > now`)
4. Update `last_used_at`
5. Return `AuthResult` with user ID and scopes

The proxy injects `X-Authenticated-User`, `X-Authenticated-Email`, and
`X-Authenticated-Scopes` headers to upstream. Client-supplied identity
headers are stripped (existing behavior).

#### Auth Result Extension

```go
type AuthResult struct {
    UserID   string
    Email    string
    Valid    bool
    Scopes   []string  // empty for cookie auth = full access
    AuthMethod string  // "cookie" or "api_key"
}
```

Cookie auth (WebAuthn session) has no scope restrictions — the user is
fully authenticated. API key auth has scopes that the proxy enforces.

#### What Does NOT Change

- WebAuthn registration and login flows are unchanged
- Cookie-based session management is unchanged
- Refresh token rotation is unchanged
- Desktop exchange flow is unchanged
- The frontend auth flow is unchanged

API keys are an additional auth path, not a replacement for WebAuthn.

### 2. Choir Base Reconciliation Kernel

Build the first implementation of Choir Base as specified in
`docs/mission-choir-base-reconciliation-kernel-v0.md`. Local-only, pure Go,
no deployment dependencies.

#### Package Structure

```text
internal/base/
  model/      — value types: Item, Version, Blob, Event, Status
  planner/    — pure three-tree reconciliation
  testkit/    — deterministic scenario fixtures
```

Start with these three. Blob store, journal, tree derivation, status, API,
and local adapters come later.

#### Model Types (`internal/base/model`)

```go
// ItemID is a stable, path-independent identifier for a file or folder.
// Format: base_item_<uuid>
type ItemID string

// BlobRef is a content-addressed reference to immutable bytes.
// Format: sha256:<hex>
type BlobRef string

// VersionID is a unique identifier for one version of an item.
// Format: base_ver_<uuid>
type VersionID string

// EventID is a unique identifier for a journal event.
// Format: base_evt_<uuid>
type EventID string

// Item represents a file or folder in the Base namespace.
type Item struct {
    ItemID         ItemID
    OwnerID        string
    ParentItemID   ItemID    // empty for root
    Name           string    // basename within parent
    Kind           ItemKind  // file or folder
    CurrentVersion VersionID // empty if deleted
    DeletedAt      *time.Time
    CreatedAt      time.Time
    UpdatedAt      time.Time
}

type ItemKind string
const (
    KindFile   ItemKind = "file"
    KindFolder ItemKind = "folder"
)

// Version represents one immutable snapshot of an item's content.
type Version struct {
    VersionID       VersionID
    ItemID          ItemID
    BlobRef         BlobRef    // empty for folder
    MediaType       string
    ContentHash     string     // hex SHA-256 of content
    ManifestJSON    string     // filesystem metadata (mode, mtime, size)
    ProvenanceJSON  string     // author, device, subject
    CreatedByDevice string
    CreatedBySubject string    // user ID or API key ID
    CreatedAt       time.Time
}

// Blob represents immutable content-addressed bytes.
type Blob struct {
    BlobRef    BlobRef
    SizeBytes  int64
    SHA256     string
    CreatedAt  time.Time
}

// EventType classifies a journal event.
type EventType string
const (
    EventCreate   EventType = "create"
    EventUpdate   EventType = "update"
    EventDelete   EventType = "delete"
    EventMove     EventType = "move"
)

// Event is an append-only journal entry recording a mutation.
// This IS a tape entry — see docs/memo-artifact-program-doctrine-2026-06-28.md
type Event struct {
    EventID       EventID
    OwnerID       string
    ItemID        ItemID
    DeviceID      string
    SubjectID     string    // user ID or API key ID (the author)
    EventType     EventType
    ParentEventID EventID   // previous event for this item (Merkle chain)
    CursorSeq     int64     // monotonic sequence number
    PayloadJSON   string    // version ref, new name, new parent, etc.
    CreatedAt     time.Time
}

// SyncState represents the sync status of an item on a device.
type SyncState string
const (
    StateSynced    SyncState = "synced"
    StateLocalOnly SyncState = "local_only"
    StateRemoteOnly SyncState = "remote_only"
    StateConflict  SyncState = "conflict"
    StateStuck     SyncState = "stuck"
)

// SyncStatus tracks per-item, per-device sync state.
type SyncStatus struct {
    OwnerID           string
    DeviceID          string
    ItemID            ItemID
    LocalVersionID    VersionID
    RemoteVersionID   VersionID
    SyncedVersionID   VersionID
    State             SyncState
    LastError         string
    RepairHandle      string
    UpdatedAt         time.Time
}
```

#### Tree Representation (`internal/base/planner`)

A Tree is a snapshot of items at a point in time, keyed by ItemID:

```go
type Tree struct {
    Items map[ItemID]model.Item
    Versions map[ItemID]model.Version
}
```

Three trees feed the planner:
- `remote` — the server's current state
- `local` — the device's current state
- `synced` — the last state both sides agreed on (the common ancestor)

#### The Pure Planner

```go
// Plan produces a list of actions that reconcile local and remote trees
// relative to the synced (common ancestor) tree. The planner is pure:
// no filesystem, network, database, wall clock, or random source.
func Plan(remote, local, synced Tree) ([]Action, []Conflict)
```

```go
type ActionType string
const (
    ActionDownload    ActionType = "download"    // remote has it, local doesn't
    ActionUpload      ActionType = "upload"      // local has it, remote doesn't
    ActionDeleteLocal ActionType = "delete_local" // remote deleted, local has
    ActionDeleteRemote ActionType = "delete_remote" // local deleted, remote has
    ActionUpdateLocal ActionType = "update_local"  // remote has newer version
    ActionUpdateRemote ActionType = "update_remote" // local has newer version
    ActionMoveLocal   ActionType = "move_local"   // remote moved it
    ActionMoveRemote  ActionType = "move_remote"  // local moved it
)

type Action struct {
    Type     ActionType
    ItemID   model.ItemID
    Version  model.Version  // the version to apply
    LocalPath string         // for local actions
}

type Conflict struct {
    ItemID    model.ItemID
    LocalVer  model.Version
    RemoteVer model.Version
    SyncedVer model.Version
    Reason    string  // "both modified", "modify/delete", etc.
}
```

The planner compares each item across the three trees:
- If `synced == remote` and `local != synced`: local changed → upload
- If `synced == local` and `remote != synced`: remote changed → download
- If `synced != local` and `synced != remote` and `local != remote`: both
  changed → conflict (preserve both sides)
- If `synced` has it but `local` doesn't and `remote` does: local deleted →
  delete remote (or download if remote also changed → conflict)
- If `synced` has it but `remote` doesn't and `local` does: remote deleted →
  delete local (or upload if local also changed → conflict)

Conflicts are never silently resolved. Both sides are preserved. The user
(or an agent) must explicitly resolve them.

#### Testkit (`internal/base/testkit`)

Deterministic scenario fixtures that test the planner:

```go
type Scenario struct {
    Name     string
    Remote   Tree
    Local    Tree
    Synced   Tree
    ExpectedActions  []Action
    ExpectedConflicts []Conflict
}
```

Required scenarios (from the mission stopping condition):
1. Local add vs remote add same path
2. Local edit vs remote edit same file
3. Local delete vs remote edit
4. Local move vs remote edit
5. Duplicate remote event idempotence
6. Corrupt/locked local item → stuck status or explicit conflict

### 3. The Connection: Auth → Base → Tape

#### Author Identity

Every Base Event (tape entry) has a `SubjectID` field. This is the
authenticated identity that authored the transaction:

- WebAuthn session → `SubjectID` = user ID (from JWT `sub` claim)
- API key → `SubjectID` = API key ID (e.g., `ak_0123456789`)

The proxy injects `X-Authenticated-User` and `X-Authenticated-Scopes`
headers. The Base service reads these to populate the Event's `SubjectID`
and `DeviceID` fields.

#### The Tape

The Base journal IS the tape for file mutations. Each Event is a tape entry:

```
Event{
    EventID:       base_evt_<uuid>,
    SubjectID:     ak_0123456789,     // API key that authored this
    DeviceID:      mac-mosiah-desk,   // which device
    EventType:     EventUpdate,
    ParentEventID: base_evt_<prev>,   // Merkle chain
    CursorSeq:     42,                // monotonic
    PayloadJSON:   {"version_id": "base_ver_...", "blob_ref": "sha256:..."},
    CreatedAt:     2026-06-28T12:00:00Z,
}
```

This is a tape entry in the sense of the artifact program doctrine:
- It has an author (SubjectID)
- It has a code version (choir_code git hash, recorded elsewhere)
- It has inputs (previous state via ParentEventID)
- It has outputs (new version via PayloadJSON)
- It is content-addressed (EventID derived from content)
- It is tamper-evident (ParentEventID forms a Merkle chain)

#### The Full Loop

```
user creates API key (WebAuthn, one time, in browser)
  → API key stored as hash in auth DB
  → user gives API key to CLI/desktop/agent

CLI/desktop/agent authenticates with API key (Bearer header)
  → proxy validates API key, injects X-Authenticated-User
  → CLI/desktop/agent writes file via Base API
  → Base service creates Event (tape entry) with SubjectID = API key ID
  → Event appended to journal (the tape)
  → Tree derived from journal
  → Planner reconciles local/remote/synced trees
  → Actions produce new versions
  → New versions are content-addressed blobs
  → Blobs are immutable, hash-verified

The tape is the program. The program computes the computer.
The API key is how machines author the program.
WebAuthn is how humans authorize machines to author the program.
```

## Implementation Plan

### Agent 1: API Auth (orange mutation)

**Scope:** Add API key system to existing auth service.

**Files to modify:**
- `internal/auth/store.go` — add `api_keys` table DDL, `CreateAPIKey`,
  `GetAPIKeyByHash`, `ListAPIKeys`, `RevokeAPIKey`, `TouchAPIKeyLastUsed`
  methods
- `internal/auth/handlers.go` — add `HandleCreateAPIKey`,
  `HandleListAPIKeys`, `HandleRevokeAPIKey` handlers
- `internal/auth/handlers_test.go` — tests for all three handlers
- `internal/proxy/handlers.go` — add `validateAPIKey` method, modify
  `validateAccessJWT` call sites to try Bearer token as fallback
- `internal/proxy/handlers_test.go` — tests for Bearer token auth

**Files to create:**
- `internal/auth/apikeys_test.go` — focused API key tests

**Constraints:**
- API key creation requires WebAuthn-authenticated session (cookie)
- API key validation uses Bearer header (no cookie needed)
- Scopes are JSON array in DB, parsed to `[]string` in Go
- `last_used_at` updated on each successful validation
- Revoked keys are soft-deleted (`revoked_at` set)
- Existing cookie auth must continue to work unchanged
- All existing proxy tests must pass

**Verification:**
- `nix develop -c go test ./internal/auth/...`
- `nix develop -c go test ./internal/proxy/...`

### Agent 2: Choir Base Kernel (yellow mutation)

**Scope:** Build `internal/base/model`, `internal/base/planner`,
`internal/base/testkit` as pure Go with deterministic tests.

**Files to create:**
- `internal/base/model/types.go` — all value types
- `internal/base/model/types_test.go` — type validation tests
- `internal/base/planner/planner.go` — `Plan(remote, local, synced Tree) ([]Action, []Conflict)`
- `internal/base/planner/planner_test.go` — planner tests
- `internal/base/testkit/scenarios.go` — scenario fixtures
- `internal/base/testkit/scenarios_test.go` — scenario runner tests

**Constraints:**
- The planner is pure: no I/O, no wall clock, no random
- Conflicts preserve both sides (never silent resolution)
- Stable item IDs, not paths, define identity
- Content-addressed blob refs (SHA-256)
- No external dependencies
- No filesystem, network, or database access
- Must build and test on macOS (local development)

**Required test scenarios:**
1. Local add vs remote add same path
2. Local edit vs remote edit same file
3. Local delete vs remote edit
4. Local move vs remote edit
5. Duplicate remote event idempotence
6. Corrupt/locked local item → stuck or conflict

**Verification:**
- `nix develop -c go test ./internal/base/...`
- `nix develop -c go build ./...`

### Agent 3: (later, not tonight)

Wire Base service to use API auth for transaction authorship. This requires
both Agent 1 and Agent 2 to be complete. Not started tonight.

## Mutation Class Assessment

- **Agent 1 (API auth):** orange — new DB tables, new endpoints, proxy
  behavior change. Full landing loop required (commit → push → CI → staging
  → verify). Rollback: remove `api_keys` table and revert proxy changes.
- **Agent 2 (Choir Base kernel):** yellow — new packages, tests only, no
  production behavior change. No deployment needed. Rollback: remove
  `internal/base/` directory.

## Open Questions

1. **Scope enforcement granularity:** Should the proxy check scopes per-route
   (e.g., `/api/base/*` requires `read:base` or `write:base`) or should
   downstream services check scopes? Proxy-level is simpler but coarser;
   service-level is more precise but requires every service to understand
   scopes. Recommendation: proxy-level for v0, service-level later.

2. **API key rotation:** Should API keys support rotation (generate new key,
   old key works during grace period)? Not for v0. Users revoke and create
   a new key.

3. **Rate limiting:** Should API keys have rate limits? Not for v0. The
   existing rate limiting (if any) applies.

4. **Base journal storage:** The planner is pure, but the journal needs
   storage. SQLite? Dolt? In-memory for tests? The mission doc says
   "in-memory or temp local stores" for v0. The planner doesn't need
   storage at all — it operates on Trees, which are in-memory snapshots.

5. **Event ID derivation:** Should EventID be a UUID or content-derived
   (hash of payload + parent)? Content-derived gives tamper-evidence for
   free. UUID is simpler. Recommendation: content-derived for the Merkle
   chain property, matching the doctrine.

## Lineage

- This design emerged from recognizing that WebAuthn-only auth blocks
  headless access, which blocks Choir Base, which blocks the artifact
  program for files.
- The API key system is the minimal addition that unblocks CLI, desktop,
  agent, CI, and mobile access without replacing WebAuthn for human auth.
- Choir Base is the first concrete implementation of the artifact program
  doctrine — the three-tree reconciliation IS tape consensus for file
  mutations.
- Related: `docs/memo-artifact-program-doctrine-2026-06-28.md` (the tape),
  `docs/choir-base-product-spec-2026-06-06.md` (Base spec),
  `docs/mission-choir-base-reconciliation-kernel-v0.md` (Base mission).
