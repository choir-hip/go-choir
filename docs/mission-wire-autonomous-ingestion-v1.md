# Mission Archive: Wire Autonomous Ingestion (v1)

Date: 2026-06-09

**Status:** archived — not an active mission.

All work, scope, acceptance, and operator decisions live in
[mission-wire-community-news-v1.md](mission-wire-community-news-v1.md).
Use that document and
[choir-wire-source-to-vtext-spec-2026-06-09.md](choir-wire-source-to-vtext-spec-2026-06-09.md)
as the only canonical Wire v1 contracts.

## Why this file exists

This was an early mission draft written when the activation problem (prompt bar
≠ ingestion) was clear but platform-computer topology, data-store boundaries,
adapter credential paths, and Phase A/B/C source scope were still fuzzy.

Running it in parallel with community-news v1 would have duplicated acceptance
matrices and contradicted operator scope (e.g. podcast/web-watch in Phase A,
HN as a separate class, operator approval gates).

## Lessons retained (now encoded in community-news v1)

1. Wire stories are triggered only by **ingestion events**, never the prompt bar.
2. **Community Wire** is platform-computer authority (`global-wire-platform`),
   always-on, with hard cutover off host `sandbox-m1` stubs.
3. **Fetch ledger** may stay host SQLite for v1; **semantic truth** lives in
   platform-computer embedded Dolt; **public read surface** is platformd
   publication projections.
4. MTProto, ATProto, Qdrant, and Postgres migration are **post-core** — after
   the ingestion → processor → VText → auto-publish chain is proven.
5. Prompt-initiated articles and SourceMaxx/StoryGraph remnants are **hard
   deletes**, not audit-and-keep.

Do not revive this file as a second mission. Extend community-news v1 instead.
