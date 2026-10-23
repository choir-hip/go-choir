# The tape sweep's database-side keep set has no read route

Date: 2026-10-22. Lane: `lane/fp10-tape-extra-refs`. Mutation class: orange
(a new read-only internal route; no writes). Found by the repack lane
(`lane/fp10-tape-repack`, [platform-artifact-small-file-growth-2026-10-19.md](platform-artifact-small-file-growth-2026-10-19.md),
"Before delete --apply on Node B: preconditions", item 6, BLOCKED).

## The gap

The tape sweep keeps every 64-hex substring of five database columns
(`tapeExtraRefQueries`, `internal/platform/tape_sweep.go`): `closure_json`
(`computer_version_code_closures`), `program_json`
(`computer_version_artifact_programs`), `checkpoint_json` and `receipt_json`
(`computer_checkpoints`), `certificate_json`
(`computer_route_projection_certificates`). The sweep builds that set in
memory (`live.addAllHex`) and the dry run returns counts only. No route returns
it, and nobody runs SQL against the production Dolt by hand. The host-side repack tool
(`cmd/taperepack`) cannot read the database, and `delete --apply` refuses to
start without `--keep-digests <file>`. So the repack's delete is blocked on a
read-only instrument that lists the set.

## The route

`GET /internal/platform/artifact-gc/tape/extra-refs`, on corpusd (the service
that hosts `/internal/platform/artifact-gc` and `.../tape`; port 8086 on Node
B, `CORPUSD_PORT=8086` in `nix/node-b.nix`).

- Authority: `trustedInternalCaller` (loopback transport, the
  `X-Internal-Caller` marker, and the host-internal credential via hostauth),
  the same check as the tape sweep route. A guest, which reaches corpusd at its
  platform URL without the credential, is refused with 403. GET only.
- It runs exactly the five `tapeExtraRefQueries` through `tapeExtraRefs` (the
  sweep's own function) and extracts digests with the sweep's own
  extraction (`tapeHexRefs`, the one `hex64RE.FindAllString` call that
  `live.addAllHex` and the apply pass's delta now share). It writes nothing,
  takes no sweep run lock, and has no mode gating.
- Answer: `{"digests": [sorted unique 64-hex], "count": N, "queries": 5,
  "rows_scanned": M, "generated_at": "..."}`. `rows_scanned` counts non-NULL
  column values read. If any query, scan or row iteration fails, the answer is
  an error status and no digests: a partial list must not look like a keep
  set.
- Bounds. The answer is built after all five queries succeed, then streamed
  from the sorted slice, so a failure never leaves a half-written list. The
  unique digest set is held in memory and capped at 2,000,000 digests (about
  136 MB of JSON, 64 + 4 bytes each); over the cap the route answers 507 and
  no list, rather than a truncated one. Expected size: the set is the artifact
  digests of every code closure and artifact program plus those named in
  checkpoints, receipts and route certificates, so it grows with versions and
  checkpoints, not with events: thousands to low hundreds of thousands, a few
  MB of JSON. This is an estimate, not measured on Node B (nothing was run
  there); `count` in the answer is the measurement.

## Heresy delta

- discovered: the sweep's database-side keep set is write-only (built, used,
  discarded), so a second deleter on the host cannot honor it.
- introduced: none intended (a read-only route behind the existing internal
  authority).
- repaired: none yet.
