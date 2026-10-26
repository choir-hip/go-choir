# Release Reads Are Not Scoped to the Owner

Date: 2026-10-26. Found by the SR9 investigation (read-only). Problem
first; no fix in this commit. Status: open; fix folded into SR9 S1
(docs/definitions/choir-sr9-commons-2026-10-26.md, to be written).

`GET /api/releases` and `GET /api/releases/<id>` (internal/proxy/releases.go
around lines 180-215) check only that the caller is authenticated and, for
an API key, holds `release:read` or `admin`. The list relays
`/internal/platform/releases` with an optional publisher filter and no
owner filter; a single read is made as the host (`computer_id=host`). So any
caller with the read scope sees every owner's published releases: their
publisher computers, base commits, patches and receipts.

Severity: medium. A release is published by an owner act, so it was meant
to be adoptable, but the owner's 2026-10-26 ruling makes changes private
until the user publishes them, with publication scoped (private, shared,
public). Today there is no scope at all, and a patch can carry content its
owner did not mean to share with every other owner.

Fix shape: owner-scoped list and reads by default; a visibility field on
the release (private, shared with named owners or computers, public); the
host read path checks the caller's owner against the visibility.
