# The keep-set accounting missed published content blobs (2026-10-30)

Class: green (problem record). Surfaces: published documents (keep set),
object-storage migration. Found by the object-storage revision 13 pass.

## Evidence [V]

- Every publish and proposal writes its content to
  `platform-artifacts/sha256/<content hash>.txt`
  (`internal/platform/service.go:410-413`) and records that path as
  `storage_ref` on a `choir.artifact_blob` object
  (`internal/platform/publication_graph.go:217,830`; `service.go:432,490,802,870`).
- The public publication readers follow that field
  (`internal/platform/service_publication_read.go:194-199,700-747`).
- `docs/evidence/keep-set-accounting-2026-10-29.md` counted only og
  `body_ref` and capture blob fields, so it reported the 12,534 `sha256/*.txt`
  files as unreferenced (~25 MB). FP0 relayed that to the owner, and
  object-storage revisions 1-12 treated them as an open deletion question
  (Q6). Devin's round-1 finding B6 is what kept them from being deleted.

## Belief state

- These files are the bodies of published documents, which the owner named
  as keep-set. Deleting them would have broken published-version reads.
- [I] Most or all of the 12,534 are named by `artifact_blob` rows; the count
  of files no row names is not yet measured.

## Fix (in design, not code)

- Object storage revision 13 imports them as og-class keep-set objects held
  by their `artifact_blob` rows, and narrows Q6 to files no row names.
- Lesson for the cutover's dry run: the census of references must be built
  from every reader of a blob directory, not from the fields one reviewer
  knew about.
