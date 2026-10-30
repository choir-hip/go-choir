# Autopaper Intake Defects Found While Designing L2a

Date: 2026-10-14, about 01:45 UTC. Found by a code survey for station L2a
(synthesis intake), with a heap profile of the live paper computer.
Status: documented; no fix yet. Each defect below names its evidence in
the code, and each one makes Autopaper copy instead of synthesize, or
makes the scaling-cost report wrong.

## Defects

**1. Feed items beyond the per-poll limit are dropped silently.**
- What the code does:
  - The RSS poller keeps the first N items in feed order and stops
    (`internal/sources/rss.go:91-96`), with N = min(max_items, 20, 100)
    (`internal/researchtools/poll_source.go:104-112`).
  - The leftover count is not recorded. `Fetched` is counted after
    truncation (`:143`), and `Skipped` counts only items without a body
    or URL (`internal/sourcegraph/captures.go:122-128`).
  - The validators are stored after a successful write (`:170`), so an
    unchanged feed answers 304 and its leftovers are never fetched.
  - GDELT and Telegram truncate the same way.
- Effect: the paper reads at most 5 items per source per poll (the
  schedules' max_items). On a busy feed, most items are never captured,
  and nothing says so.

**2. A failed arrival append loses the announcement for good.**
- What the code does:
  - The captures are written before the arrival record is minted
    (`internal/agentcore/source_schedule.go:161-176`).
  - If the append fails, the next poll counts those captures as
    `Repeated` and never names them again.
- Effect: "every capture gets an answer" is not guaranteed.

**3. Management's library list is capped at 50.**
- What the code does:
  - `GET /api/texture/documents` returns at most 50 documents
    (`internal/textureowner/texture.go:804`, `handler.go:153-155`).
  - This is management's only view of existing stories
    (`management.yaml`, "Living stories").
  - The paper computer now holds exactly 50.
- Effect: from now on, management cannot see older stories when deciding
  whether a capture belongs to one. That alone produces duplicates.

**4. Research's corpus search finds no current story.**
- What the code does:
  - `choir.SearchWireCorpus` searches this computer's Texture documents
    and keeps only "published" ones (`internal/store/texture.go:550-660`).
  - "Published" means revision metadata holds `corpusd_route_path` or
    `platformd_route_path` (`:608`). The only writer of those keys was
    deleted in `dca5154e` (the autonomous Universal Wire publish path).
  - It never searches captures, which live in host Store B and are
    reached only through `ReadCapture` and `ListCaptures`.
- Effect: research cannot build on the paper's own library or its
  captures. It re-searches the open web for every story (baseline A:
  median about 6 outlets per version, one with 36).

**5. A capture's language is its source's first language, not the
item's.**
- What the code does:
  - Language is stored on web_capture metadata only, as
    `firstString(source.Languages)` (`internal/sources/rss.go:115`).
  - It is absent from the arrival record (`types.CaptureArrival`).
- Effect: cross-language grouping cannot see language. A mixed-language
  feed is mislabelled.

**6. Management cannot address research, and research exists only per
document.**
- What the code does:
  - `recordsRoute` (`internal/agentcore/inbox_records.go:44-86`) lets
    management address only `texture:<doc>` and `source:<id>`.
  - Research desks are minted only inside a Texture turn
    (`internal/store/texture_turn.go:625-656`), keyed to a document.
- Effect: this is not a bug. It is the missing mechanism for the owner's
  topic-researcher architecture (L2a), recorded here because the design
  depends on it.

## Memory findings for the scaling-cost report (station LR)

Read at 00:45 UTC from Node B.

- **Host side.**
  - The paper VM's Firecracker process holds 5.6 GiB RSS.
  - The guest's main process (autoputer) reports Sys 1.46 GB, with
    HeapInuse 1.05 GB (pprof `heap?debug=1`).
  - So about 4 GiB of the VM's host RSS is outside the main Go heap:
    yaegi desk-session workers, which are separate OS processes kept up
    to 30 minutes idle, at most 12 per computer, each with an 8 GiB
    virtual cap (`internal/agentcore/tools_desk.go:40-52`); other guest
    processes; and the guest kernel's page cache from reading the 6.9 GB
    store. With no balloon, none of it is returned to the host.
- **Main heap.** Of the main process's sampled in-use heap (556 MB), 426
  MB (77%) is allocated under
  `computerevent.(*ComputerEventAppender).AppendNewPayload`, mostly Dolt
  buffer pools (`store/pool.buffPool.Get`, 211 MB) and index flattening.
  Whether that is a cache that should be bounded or a leak is the next
  question.
- **Wildcard.** Measuring yaegi's share needs a per-process read inside
  the guest, which no current endpoint exposes. That is a gap for the
  oversubscription probe.

## What follows

- Defects 1-4 are bugs to fix before the scaling-cost report can be
  trusted. They join L2a's S1 (2, 3, 4, 5) and the poll path (1).
- The memory findings go to LR's provisional scaling read.
- Fixes come in later commits that cite this record.

## Correction (panel round 2, 2026-10-14)

The first design of the fix for defect 1 (stop storing validators while
items are deferred) does not work. Truncation happens before the repeat
check, so the next poll keeps the same first N items. The corrected fix
applies the limit to new captures after the repeat check
(`docs/archive/reports/l2a-synthesis-intake-design-2026-10-14.md`, R2-1). Defect 4's
dead filter moves into S1a, and the search over captures stays in S1b.
