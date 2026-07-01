# Code / Docs Reconciliation - 2026-06-06

**Status:** review note  
**Scope:** full codebase shape against core docs after syncing to
`origin/main` commit `fab6b25b`.

This note records what was checked while reconciling the docs cleanup branch
with the current codebase. It is not a replacement for
[current-architecture.md](current-architecture.md) or
[platform-os-app-state.md](platform-os-app-state.md).

## Code Shape Checked

Host/service binaries present in `cmd/`:

- `auth`
- `proxy`
- `gateway`
- `vmctl`
- `corpusd`
- `maild`
- `maildctl`
- `sourcecycled`
- `sandbox`
- `zot`

Major internal package families present:

- `internal/auth`, `internal/proxy`, `internal/gateway`, `internal/vmctl`,
  `internal/platform`, `internal/maild`
- `internal/runtime`, `internal/store`, `internal/types`, `internal/sandbox`,
  `internal/vmmanager`, `internal/zot`
- `internal/cycle`, `internal/sourcefetch`, `internal/sourcecontract`,
  `internal/sourceapi`, `internal/sources`, `internal/search`
- `internal/modelcatalog`, `internal/provider`, `internal/events`,
  `internal/markdownstructure`, `internal/server`

Frontend app surfaces present in `frontend/src/lib/` include Desktop, VText,
ContentViewer, Browser, Files, Email, Podcast, Image, Audio, Video, PDF, EPUB,
Settings, Compute Monitor, Desktop Overview, Super Console, and related VText
source rendering/action modules.

## Product Routes Checked

Route registration confirms product/public API surfaces for:

- prompt bar and prompt submission status;
- Trace trajectories;
- VText documents, revisions, history, stream, diagnosis, import, export,
  manifest, diff, blame, source repairs, source attachments, restore, and
  revise;
- content items, podcast routes, browser sessions, desktop state, media
  progress/recents, preferences, computers, AppChangePackages, adoptions,
  continuations, run acceptances, and run-acceptance synthesis;
- proxy bootstrap, websocket, authenticated `/api/*` forwarding, provider deny,
  and browser-public `vmctl` deny;
- platform-public publication publish/resolve/export/retrieval/proposal
  internals behind `corpusd` and proxy routes;
- `sourcecycled` `/internal/source-service/health`, search, and item resolution.

## Reconciliation Changes Made

- `current-architecture.md` now says "codebase/deployable topology" rather
  than overclaiming that every sidecar is always deployed and proven.
- `current-architecture.md` now separates code-present/current foundations,
  active hardening, and target-only direction. This corrects the earlier
  near-term section that made existing public desktop, auth-on-mutation, VText,
  publication, source, and media foundations read as purely future work.
- `docs/README.md` now indexes this reconciliation note and avoids duplicating
  the source/publication consolidation ledger.

## Source / News Addendum

After the main reconciliation pass, the local dirty-main document
`docs/news-system-current-state-and-improvements-2026-06-06.md` was imported
into this worktree and reconciled against the current source/news code.

Important findings:

- `sourcecycled` is a real v0 daemon with internal health/search/item APIs,
  persistent SQLite tables, source registry loading, fetch/item/cycle/event
  storage, and immediate plus 15-minute polling.
- `source_search` is already available to researcher tooling when
  `SOURCE_SERVICE_BASE_URL`, `SOURCE_SERVICE_URL`, or `SOURCECYCLED_API_URL`
  is configured. Node B config sets the local source-service URL.
- VText recognizes `source_service_item:<id>` refs and can preserve them as
  source entities, so the missing layer is product workflow and News app
  exposure, not total absence of source-service representation.
- Per-source cadence metadata exists in `configs/sources.json`, but the daemon
  still runs one global 15-minute loop.
- Deduplication is process-local in `cycle.Engine` before storage. Stable item
  IDs and SQLite primary keys help collapse duplicate rows, but restart-aware
  "new item" accounting and synthesis gating are not yet durable proof.
- No user-facing News/Newspaper app, newsletter pipeline, subscription/event
  stream, or public source API contract is present today.

## Current Confidence

The core docs now match the repository's actual service/package/route/app
shape at the architecture level. Staging remains the authority for claims about
live vmctl behavior, auth/session renewal, gateway credentials, provider/search
calls, background/candidate computers, platform promotion, rollback, and
Choir-in-Choir behavior.

Checks run:

- `git diff --check`
- markdown link scan for `docs/*.md`
- `nix develop -c go test ./internal/sourcecontract ./internal/sourcefetch ./internal/sources ./internal/platform ./cmd/sourcecycled`
- `nix develop -c go test ./internal/runtime -run 'Test(VText|RunAcceptance|AppChange|AppAdoption|PromptBar).*(Source|Revision|Worker|Export|History|Diagnosis|Import|Stale|Evidence|Publication|Package|Adoption|Acceptance|Prompt|Manifest)'`

The markdown link scan still finds five old missing mission links in historical
mission docs. They were pre-existing cleanup debt and are not introduced by
this pass.

No code behavior was changed in this reconciliation pass.
