# Parallax: M4 — Base API + Blob Store

**Conjecture (C21):** A content-addressed blob store and REST API for
Base items can be built on top of the journal/tree packages (M2+M3) and
API key auth (M1), providing a complete sync substrate for desktop
clients.

**Class:** orange — new HTTP endpoints, new storage
**Worktree:** /Users/wiz/.windsurf/worktrees/go-choir/m4-base-api
**Branch:** orchestrator/m4-base-api
**Depends on:** M1 (API auth — on main), M3 (journal/tree — on main)

## Spec

Build `internal/base/blob` and `internal/base/api`:

### Blob Store (`internal/base/blob/`)
- Immutable, content-addressed (SHA-256)
- `Put(data []byte) (BlobRef, error)` — hash and store
- `Get(ref BlobRef) ([]byte, error)` — retrieve and verify hash
- `Has(ref BlobRef) (bool, error)` — check existence
- Storage backend: filesystem (directory of hashed files)
- Hash verification on Get (detect corruption)

### Base API (`internal/base/api/`)
- `POST /api/base/blobs` — upload blob (returns BlobRef)
- `POST /api/base/items` — create/update item (creates journal Event)
- `GET /api/base/items/{id}` — get item at current state
- `GET /api/base/delta?cursor=...` — get events since cursor
- `GET /api/base/items/{id}/status` — get sync status for item
- `POST /api/base/repair/preview` — preview repair actions

### Auth Integration
- All endpoints require API key Bearer token (from M1)
- Each mutation creates a journal Event with SubjectID = authenticated
  identity (from API key's user ID)
- Scopes: `read:base` for GET endpoints, `write:base` for POST endpoints

## Invariants
- Blob store is content-addressed (SHA-256, hash-verified on read)
- Journal events are append-only (from M3)
- API key auth required (from M1)
- Scope enforcement: `read:base` for reads, `write:base` for writes
- No secrets in responses
- Use `nix develop -c` for all go commands

## Acceptance Criteria
- `nix develop -c go test ./internal/base/blob/...` passes
- `nix develop -c go test ./internal/base/api/...` passes
- `nix develop -c go build ./...` passes
- Tests cover: blob put/get/has, hash verification, API endpoints with
  auth, scope enforcement, journal event creation on mutation, delta
  query with cursor

Return: conjecture verdict, test output, files created.
