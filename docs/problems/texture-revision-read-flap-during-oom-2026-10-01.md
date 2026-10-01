# Texture list/revision reads flap during OOM — doc list loads, revision body fails

**Status:** OBSERVED 2026-10-01 on staging (build `4a718af4`, guest
`computer-03335285269bdba4f94377e56879f9e6`). Owner-reported symptom on the
iOS web client; confirmed on journald. This is a *symptom* of the
platform-dolt OOM/realization substrate, not an independent texture bug —
recorded per Problem-Documentation-First and cross-linked to the cluster
assessment.

## Symptom (owner-reported, iOS)

Opening Texture: "Loading recent Textures…" hangs ~1 minute → after a manual
reload the document list appears, with a red **"Load failed"** badge on
"What's new in ai today" → opening it shows an empty `v0` editor ("Start
typing the document…") even though the list shows a `v9 · appagent` revision.

## Root mechanism (confirmed on journald)

Two distinct request layers flap under the OOM/realization cycle:

- **The list** (`GET /api/texture/documents`) eventually returns — slow, not
  failed.
- **The revision fetch** fails server-side:
  `texture api: get current revision for recent metadata: objectgraph dolt:
  scan object: context canceled`,
  `count texture revisions: ... list by metadata page: context canceled`, and
  `json encode error: write tcp ...8085: i/o timeout`.

So the guest's objectgraph-dolt read for the revision body is being canceled
mid-query (context deadline / connection churn during the OOM window) and the
encode write times out to the client. The frontend then falls back to a blank
`v0` — surfacing as "no content" rather than an error to the user.

## Why this is the same substrate

The `context canceled`/`i/o timeout` bursts coincide with the platform-dolt
memory pressure and the `resolve autoputer` realization flaps already
documented in `platform-dolt-oom-realization-cluster-2026-10-01.md`. The guest
is alive but its store reads stall inside the same window that produces the
`502` on `run start`. The texture list read being slower than the revision
read failing is just two queries racing the same degraded store.

## What this is NOT

- Not an embedded-dolt schema/corruption bug — the doc *list* returns; only
  the per-revision body read times out under load.
- Not a frontend render bug — the editor is correctly reflecting a failed
  fetch (it has no content to show). A blank-v0 fallback that *looks* like an
  empty doc is the UX wart, but the store read is the fault.

## Fix boundary

Belongs to the OOM/realization substrate mission
(`docs/definitions/choir-platform-dolt-capacity-stabilization-2026-10-01.md`),
not a texture-code patch. If revision reads still flap on a *stable* guest,
that becomes a real texture-store or query-budget defect to investigate then.
The blank-v0-with-no-error UX could optionally surface the load failure
explicitly, but that is a green-class UX nicety, not the substrate fix.
