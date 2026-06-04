# VText User Draft Versioning Problem - 2026-06-03

## Problem

Direct user edits in VText are currently autosaved by calling the canonical
revision creation endpoint. That means normal typing can create multiple
document versions before the user explicitly asks VText to revise or save a
version.

## Evidence

- `frontend/src/lib/VTextEditor.svelte` schedules `autosaveUserDraft` after
  editor input.
- `autosaveUserDraft` calls `createRevision` with `metadata.autosaved = true`.
- `createRevision` posts to `/api/vtext/documents/{id}/revisions`, whose
  backend handler writes an immutable revision and advances
  `current_revision_id`.

## Desired Behavior

Autosave should preserve the user's in-progress document text durably without
advancing canonical version history. When the user explicitly hits the revision
button, the accumulated user edits should be grouped into one user revision,
and only then should the appagent revision flow begin.

## Belief State

The bug is a boundary error between draft persistence and canonical revision
history. The smallest repair is to stop routing autosave through the immutable
revision endpoint and keep the existing revision endpoint as the only path that
advances the version chain. A server-side draft surface may still be warranted
later for cross-browser or cross-device autosave, but it is not required for
the immediate grouping bug.

## Remaining Error

The exact UI label of the "revision button" is represented in the current
implementation by the VText prompt/revise action. Publishing and proposal flows
also call the same save helper and should continue to promote any pending user
draft into one canonical revision before using that revision.
