# Memo: Choir CLI trajectories decode mismatch + Universal Wire 502 on staging

**Date:** 2026-07-06
**Author:** Devin session (choir CLI acquaintance)
**Status:** discovered — problems documented, fixes pending

## Context

Session goal was to get acquainted with the `choir` CLI using a newly added
`CHOIR_API_KEY` in `.env`. The CLI was built (`go build -o /tmp/choir
./cmd/choir`), auth was verified against `https://choir.news`, and each command
was exercised against staging.

## Problem 1: `choir trajectories` JSON decode mismatch (CLI bug)

**Class:** yellow (CLI client decode fix, no product runtime behavior change).

**Evidence:**

```
$ /tmp/choir trajectories
choir trajectories: decode response: json: cannot unmarshal object into Go
struct field trajectoryRecord.trajectories.settlement_rule of type string
```

**Root cause:** `cmd/choir/main.go:347` declares `SettlementRule string`, but
the runtime returns `settlement_rule` as an object
(`{"require_no_open_work_items": true, ...}`). The real type is
`internal/types.Setrajectory.go:35` `SettlementRule struct`. The CLI's local
`trajectoryRecord` also drops `SubjectRefs map[string]string` and `Status
TrajectoryStatus` fields that the runtime sends.

**Impact:** `choir trajectories` always fails. `choir trajectory <id>` was not
tested (likely affected if it shares the type).

**Belief state:** The CLI was written against an earlier or assumed shape of
`TrajectoryRecord` that predates the `SettlementRule` struct introduction. The
fix is to mirror the runtime type correctly in the CLI's local struct.

**Remaining error field:** Fix the `trajectoryRecord` struct to use a local
`settlementRule` struct mirroring `types.SettlementRule`, add `SubjectRefs` and
`Status` fields. No `omitempty` on `settlement_rule` (it's a struct, not a
pointer).

## Problem 2: `/api/universal-wire/stories` 502 timeout on staging (server-side)

**Class:** orange (staging runtime behavior, not a CLI bug).

**Evidence:**

```
$ curl -s -o /dev/null -w "%{http_code} %{time_total}s" \
    -H "Authorization: Bearer $CHOIR_API_KEY" \
    --max-time 30 https://choir.news/api/universal-wire/stories
http_code=000 time_total=30.005531

(time_starttransfer=0.000000 — server accepts TLS but never sends a response
header before timeout)
```

Earlier test with 180s timeout returned `502` after `180.112591s`.

**Staging health evidence** (`/health` at 2026-07-07T04:0x):

- Deployed commit: `24886d24` (confirmed — my earlier push is live).
- `api.resolve` stage: count=93, errors=23, `avg_duration_ms=31426`,
  `max_duration_ms=180029`. The proxy's upstream resolution is timing out on
  some requests.
- Other endpoints work fine: `/api/trajectories` (106ms), 
  `/api/texture/documents` (120ms), `/api/search` (works), 
  `/api/prompt-bar` (works — `run start` succeeded).

**Root cause (confirmed):** The proxy routes `/api/universal-wire/stories` to
the **platform computer** VM (`universal-wire-platform`/`platform`), a separate
always-on VM from the user's personal VM. Other API endpoints route to the
user's VM (working fine). The platform computer VM is not responding — the
proxy's `resolveSandboxURL` for the platform computer times out at 180s.

Evidence:
- `internal/proxy/handlers.go:970`: `protectedAPIResolveTarget` hard-codes
  `/api/universal-wire/stories` → `vmctl.UniversalWirePlatformOwnerID`/
  `UniversalWirePlatformDesktopID`.
- `internal/proxy/handlers.go:577-578`: "Universal Wire stories read the
  platform computer's embedded store, not the caller's personal computer."
- Health endpoint `api.resolve` stage: count=104, errors=23,
  avg=28102ms, max=180029ms. The 23 errors are the wire stories requests
  that timed out. The 81 successful resolves (other endpoints) have fast
  upstream calls (13ms avg).
- `platform_publication.resolve` and `platform_retrieval.search` stages
  resolve fine (23ms and 455ms avg respectively) — those routes go through
  a different path (corpusd, not the platform computer VM).

The handler (`internal/runtime/universal_wire.go:59`) always writes a JSON
response (line 125), so the hang is NOT in the handler's response writing —
the proxy never reaches the upstream VM at all. The `resolveSandboxURL` call
for the platform computer is what hangs.

**Impact:** `choir wire stories` and `choir wire diagnostics` both fail (they
hit the same endpoint). The Universal Wire feed is unavailable on staging.

**Class:** red (VM lifecycle, deployment routing). The platform computer VM
is down, unresponsive, or missing from vmctl's registry.

**Belief state:** This is a staging VM lifecycle issue. The platform computer
VM (`universal-wire-platform`/`platform`) needs to be checked and possibly
resumed. Cannot be diagnosed or fixed locally per doctrine (staging is the
acceptance environment; local dev is not proof for VM lifecycle).

**Remaining error field:** Needs staging VM access to:
1. Check platform computer VM status via vmctl (is it running/stopped/crashed?)
2. Resume it if stopped
3. Check the runtime inside it if running but unresponsive
4. Check vmctl's registry for the platform computer ownership entry

## Commands that work correctly

- `choir api-key list` — auth verified, returns 2 keys
- `choir run start` — conductor routed prompt to texture app, created doc
- `choir run status` — returns submission state + decision
- `choir texture read <uuid>` — returns doc metadata (must use UUID, not path)
- `choir texture history <uuid>` — returns revision chain
- `choir search` — returns publication results with snippets

## Setup changes

- `.envrc.local` created with `dotenv .env` to auto-load `CHOIR_API_KEY` per
  `AGENTS.md` convention. Both `.env` and `.envrc.local` are gitignored.
