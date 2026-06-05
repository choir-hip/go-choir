# VText Durable Version Number Problem

Date: 2026-06-05

## Problem

VText version labels are not durable document facts. The UI currently derives
`vN` from the index of the revisions returned to the browser. The runtime
revision list endpoint returns only the latest 50 revisions, so documents with
more than 50 revisions appear capped at `v49` even when new revisions continue
to be created.

This is not an account-specific issue. It is a product architecture issue:
version identity is implicit in a truncated transport list instead of stored on
each revision.

## Evidence

- `internal/runtime/vtext.go` lists revisions with a fixed `50` limit.
- `internal/store/vtext.go` defaults revision list limits to `50`.
- `frontend/src/lib/VTextEditor.svelte` displays `v${activeRevisionIndex}`.
- On staging, `choir_private_legal_cloud_proposal.md` had more than 50
  revisions, but the API returned 50 entries and the UI could only display
  `v0` through `v49`.

## Required Fix

Store a durable per-revision version number in the VText revision record.

The version number should:

- be assigned transactionally when a new revision is created;
- start at `0` for the first revision of a document;
- increase monotonically for every new head revision;
- be backfilled for existing documents from the existing revision chain or
  created order;
- be returned by revision APIs;
- be used by the frontend for labels, navigation text, compare labels, publish
  labels, and next-version preview labels.

Increasing the list cap alone is a temporary workaround and does not solve the
identity problem.
