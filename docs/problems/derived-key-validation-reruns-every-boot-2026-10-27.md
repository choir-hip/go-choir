# Every boot re-validates the derived keys from zero, for about 1.5 hours, on the store the desks use (2026-10-27)

Status: open; documented before any fix. Mutation class of any fix: red
(store read path and derived indexes). Owner priority applies: a change
that durably cuts disk, CPU or write volume comes ahead of short-term
uptime (CLAUDE.md, 2026-10-22).

## Evidence (the paper, computer-1355b83b…, console, UTC)

The derived-key families are unvalidated at every process start by design
(`internal/objectgraph/derived_keys.go:27`): an older binary could leave a
"valid" mark lying, so validity is never persisted. Until an online pass
compares a family with og_objects page by page, reads of that family take
the JSON path. The pass validates the families one after another:

| Family | Rows | Pass time | Re-derived | Orphans |
|---|---|---|---|---|
| actor_wake_outbox.projected | 122,095 | 37m42s | 0 | 0 |
| run.agent_id | 56,854 to 57,396 | 17m53s to 30m38s | 0 | 0 |
| commitment_record.addressee | 92,964 | 23m04s | 0 | 0 |
| source_ref.doc_id | 160,096 | 9m17s | 0 | 0 |

So one boot costs about 1.5 hours of paged scans at 40 to 100 rows a
second, and every pass so far found nothing to repair. A restart throws
the progress away: the outbox pass started over at 04:01:50, 04:04:29 and
04:20:12 (two deploys of tonight's trains), and finished only at 04:57:55.

While a pass runs the paper also shows:

- `store_stalled holders … unanswered=live_capsules`: 89 in the 87
  minutes from 02:34 to 04:01, and 136 in the 81 minutes from 04:01 to
  05:22;
- `objectgraph dolt: list objects page: context deadline exceeded` (intake
  labels, 05:21:36);
- runtime health checks failing every hour since at least 17:00 on 10-26
  (55 to 151 an hour), and vmctl marking the paper degraded (22h, 02h,
  05:22) and re-promoting it.

## What this does and does not show

- It shows a per-boot cost of about 1.5 hours of read IO that has
  repaired nothing, and that every deploy pays it again on the paper.
- It does not show that the pass causes the health failures. They predate
  tonight, and the paper's shared slots went from 8 to 12 at 04:06
  (`dispatch.wide_slots`), which also raises store load. The stall rate
  rose about 70% across that change, but the window also holds two boots.
- **Possible link to the b@c capsule build stall**
  ([capsule-frontend-build-hangs-b-c-runtime-2026-10-27.md](capsule-frontend-build-hangs-b-c-runtime-2026-10-27.md)):
  the only fast build ran on a guest resumed from a snapshot, whose process
  had already validated. Both slow builds ran after cold boots, during
  which the pass and the JSON-path reads would run. Call this H8. It is
  not confirmed: b@c's console has not been read for the pass.

## Questions for the design

1. Can validity be persisted safely: a marker bound to the binary's
   derived-key schema version and the store's head, so the same binary
   on an unchanged store skips the pass? An older binary would not know
   the marker and would not trust it either.
2. Can the pass be incremental: check only rows written since the last
   validated head, not the whole family?
3. Should the pass yield to desks, either below a store-load threshold or
   at a lower priority?
4. What does the JSON path cost per read while a family is unvalidated?
   That is the hidden half of the cost.

## Next observations

- b@c's console: did the pass run during the 01:09 and 02:21 to 02:43
  builds? (H8)
- The paper's stall and health-failure rates once the pass has finished,
  with 12 slots still on. That separates the pass from the slot raise.
