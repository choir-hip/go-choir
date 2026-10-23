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
(`cmd/taperepack`) cannot read the database. On the repack lane's round 4
(`origin/lane/fp10-tape-repack`, `49ba705eb`, not yet on `main`)
`taperepack delete` has `--keep-digests <file>`: enforced first in
`unlinkCovered`, refused at `--apply` without it, and an empty file refused
unless `--keep-digests-empty-ok`. That lane makes the keep file checklist item
6 of [platform-artifact-small-file-growth-2026-10-19.md](platform-artifact-small-file-growth-2026-10-19.md)
("The keep file (M2)"), blocked until a read-only instrument lists the set.
(An earlier round of that lane, `8722c4932`, had no such flag; a reader of
that round will not find it.) Neither commit is on `main` yet: this route and
the repack's keep-file enforcement land in later deploys, and `delete --apply`
on Node B waits for both. The route makes the set readable; the repack
enforces it.

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

## Operator call (Node B, host shell)

Port 8086 is in `choir_internal_token_allowed`'s set (8083, 8084, 8086, 8087),
so `scripts/lib/choir_internal.sh` sends the credential to it and to no other
port; no policy change. From the host shell (never a guest, never a proxy):

```
W=/var/lib/go-choir/taperepack-work   # the repack's work directory (checklist item 5)
. scripts/lib/choir_internal.sh
choir_internal_curl http://127.0.0.1:8086/internal/platform/artifact-gc/tape/extra-refs \
  -fsS > $W/extra-refs.json
jq -e '.queries == 5 and (.digests | length) == .count' $W/extra-refs.json \
  && jq -r '.digests[]' $W/extra-refs.json > $W/keep-digests.txt
```

`-f` makes a 403/500/507 a failed call (no file content to mistake for a keep
set); `jq -e` fails if the shape is wrong. An empty keep set is `"digests":[]`
and a zero-line file, which `taperepack delete --apply` refuses unless
`--keep-digests-empty-ok` (repack round 4). Take the file after the census and before the
delete dry run, and use the same file for the dry run and the apply (checklist
item 6 in the small-file-growth doc, as written on repack `49ba705eb`). If `jq -r` prints a line that is not 64
lowercase hex, stop.

Not run here: no call was made against Node B, staging or any database; the
route is proved by `internal/platform/tape_extra_refs_test.go`
(`go test ./internal/platform -run TapeExtraRefs`): the answer equals the
digests `tapeExtraRefs` adds, a dropped table gives 500 and no digests, an
over-cap set gives 507 and no digests, and a guest, a call without the marker or
with a wrong credential, and POST are refused.

## Heresy delta

- discovered: the sweep's database-side keep set is write-only (built, used,
  discarded), so a second deleter on the host cannot honor it.
- introduced: none intended (a read-only route behind the existing internal
  authority).
- repaired: the keep set is readable (route landed). Item 6 becomes
  satisfiable only when this route and the repack's `--keep-digests`
  enforcement (`49ba705eb`) are both deployed; neither is deployed yet.

## Review round 1 (bunch 143)

Verdict: pass the route, hold the docs. Findings and dispositions:

- F1 (doc, medium): the record said `delete --apply` refuses without
  `--keep-digests` and cited item 6, and the reviewer, reading the repack
  lane's round 3 (`8722c4932`), found no such flag. Fixed in this record: it now
  cites the repack lane's round 4 (`49ba705eb`, which has `--keep-digests`,
  `unlinkCovered` enforcement, the `--apply` refusal and
  `--keep-digests-empty-ok`, and checklist item 6 "The keep file (M2)"), says
  both land in later deploys, defines `W=` in the operator snippet, and says the
  route makes the set readable while the repack enforces it.
- F2 (low, memory): `tapeHexRefs` returns substrings of the row text, so a
  digest inserted into the set keeps its whole row value alive; the 2M cap bounds
  digest count, not retained bytes. The "136 MB" is the JSON answer only. Fix:
  clone each new digest before inserting. Corrected estimate at the cap: the
  set is about 100 bytes per digest in a Go map (64 bytes of digest, a 16 byte
  string header, bucket overhead), so about 200 MB, plus a 32 MB sorted slice
  and the answer streamed from it, so roughly 250 MB peak heap. This is an
  estimate, not measured. `extraRefsMax` is a test-only knob (written before
  the call in tests, never in production); it is documented as such.
- F3 (low, test wiring): the tests called the handler directly, so neither
  `RegisterRoutes` nor the exact path was exercised. Fix: one test through
  `server.NewServer` + `RegisterRoutes` that hits the exact path with GET (200)
  and POST (405).
- Not changed (consumer side, per the reviewer): the five SELECTs have no
  common snapshot, so a keep file can miss a row committed after it was read;
  the repack's apply-time enforcement must use a fresh file (the checklist takes
  the file after the census and before the delete). `queries: 5` is asserted by
  the test and the operator's `jq`; a schema change updates both. No
  concurrency limit: the host operator makes one call at a time.
