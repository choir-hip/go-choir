# Why new Autopaper stories have no "More on this" (2026-10-26 ~11:30 UTC)

Read-only. Code and docs read from `origin/main` (99524f638). Public GETs of choir.news only: `/api/platform/publications/front?limit=100`, `/related` and `/resolve` for each of the 100 stories. No paper-computer access, no auth. Raw data: `front.json`, `links.json`, `caps.json` in the scratchpad directory.

## 1. Direct cause for today's stories

Proven:

- **The only signal that is live is "two stories cite the same capture", and among the 100 newest stories no capture is cited twice.** I pulled each story's cited captures from `/resolve` (transclusions[].ref.doc): 100 stories, 368 distinct captures, 0 captures shared by two stories, 0 pairs. The walk (`internal/platform/publication_related.go:573` relatedWalk, `:700-760` relatedCaptureCandidates) links two stories only through a shared capture or two captures of one page, so there is nothing to link.
- **Zero of the 32 links on the 13 stories that have any point at a story in today's 100.** All 32 targets are stories from 10-22 to 10-25 (one is 10-14, one 10-26). Linking appears only when a new story re-cites a capture from an earlier day.
- **The read switch for the stronger signal is off.** Every one of the 100 answers has `basis.mode == "captures"` and `basis.events == 0`. `relatedMode()` (`publication_related.go:70`) returns `"captures+relations"` only when `CORPUSD_RELATED_STORY=on` (`publication_related_story.go:48-52`). So the corpusd serving choir.news does not read `related_story` edges.
- **Coverage today: 13 of 100 stories have any link. In the first 20 by recency, 0.** The first 60 give 4 of 60 (7 percent), against 14 of 60 (23 percent) at 00:50 UTC today. By number of cited captures: 2 captures 4/63, 3-4 captures 7/27, 5+ captures 2/10. 63 of 100 stories cite exactly two captures. In 4-hour buckets of `updated_at`: 00-04 UTC 7/29, 04-08 UTC 5/39, 08-12 UTC 1/32.
- **It is not a fault.** `partial` is false on 100 of 100, and no answer is an error. The 5-minute cache, the budget (400 ms) and the kill switch (`CORPUSD_RELATED=off`, `:56`) are not involved. The frontend (`TextureEditor.svelte:1042` fetch, `:1894` render; `texture-related-stories.ts` `renderRelatedHTML` returns `''` for an empty list, by design) shows nothing for an empty list, which is the correct output here.

Hypotheses, not proven:

- *Why captures are never shared.* The management overlay tells the desk "r.Captures.Event.Covered names the stories that already hold the event: feed its captures there rather than opening another story ... Records that share EventID are one event: one story, never two" (`internal/runtimeprompts/overlays/rlm_management_runtime.yaml:32`). That is consistent with 0 of 368 captures shared, but I did not read the production path, and the same-day Serbia pairs (four stories, 3 to 11 captures each, no shared capture) show the event layer does not always join. I read neither the tape nor capture-event memberships.
- Whether the two-capture, two-outlet stories are the cause of the thin overlap or only a correlate. The 10-26 00:50 measurement found the same tendency (1 of 34 two-capture stories linked).

## 2. History: builds and what stopped each

| When (UTC) | Build | Mechanism | Status after it | What stopped or limited it |
|---|---|---|---|---|
| 10-19 16:36 | design `8127e0847` (`docs/reports/related-stories-graph-design-2026-10-19.md`) | graph | The design's own baseline: shared URLs link ~8% of 120 stories, "coverage needs a signal wider than one URL, which is the event" (§3.1) | Phase 1 was shipped on the signal the design called too thin |
| 10-19 21:41-21:48 | `6d07099fb`, `e7f0cddd8`, `5f229a91b`, `d3322646a`, `e7ea4afda` | `cites_capture` edges on publish, backfill (1,032 stories, 4,056 edges), walk over shared captures or recaptures, endpoint, "More on this" UI, `CORPUSD_RELATED` kill switch (default on) | 12 of 60 linked (20%); ~4 in 10 links wrong | Live-blog hubs link unrelated stories |
| 10-19 22:38 | `0a541483c` live page gives no vote | removes live URLs from the signal | 9 of 60 (15%); 33 of 38 links right | Precision bought with coverage (docs/problems/related-stories-dead-end, "After the live-page fix") |
| 10-15 to 10-20 | event layer V1 to V3b, `31a5c2bc4` (one event is one incident) | "phase 2": event-based links | Never reached Store B: `choir.world_event` is owner-VM-store only; the blind rerun failed the bar (3 to 6 false merges of 20, `6fb4f4a23`) | The wrong joins were real related pairs but not same-incident, so phase 2 was never built |
| 10-20 | phase 3 design `b09328bfc`, `6ea7e646e`, `d437a1801` (owner ruling: System 1 reads every story and pair); `631a1c5a0` `choir.System1`; `614fd445f` `research.system1` setting | research desk plus a closed-question judge | Verb built and on for the paper (switched-off inventory A7, from the ledger). Nothing writes a relation | Only the judge verb was built. The relation write was not. Checklist item L5-01 was "in flight" (`docs/launch-checklist-2026-10-21.md:403`); the residual register PR-22 deferred it "after the self-development focus" |
| 10-26 00:50 | re-measure `355c603ad` (owner: "a lot of the top stories are missing their related stories section") | read-only | 14 of 60 (23%); coverage tracks cited captures | Doc records the cause. Phase 3 was still unbuilt |
| 10-25 22:02 to 23:06 | `fbaeb4ff1`, `d58d194cf`, `cb72538b5` (lane related-p3) | `choir.RelateStories` verb, `story.relation` tape event, `related_story` edge, read in `relatedCandidates` | Deployed in train 1 (ledger line 87: CI failed on the semantic-kind census for `story.relation`, fixed in `b4b4e551b`). **Both switches default off** | Write: setting `research.related_story`, default false (`internal/store/computer_setting.go:58`, `:217`). Read: `CORPUSD_RELATED_STORY=on` in the host `corpusd-retrieval.env` plus a corpusd restart (not set in `nix/`; no doc records it on). Also needs a research cast, an audit of at least 40 pairs at no more than 10% wrong, and platform-tier computer |

Two points about the switches:

- Public evidence proves the read switch is off. Nothing public shows the write setting. The code default is false, the build doc says "Switches (both off by default)", and no ledger, evidence or checklist line says anyone turned it on. Whether it is on for the paper computer is unproven; I cannot read it.
- Nothing shows a single `related_story` edge has been written. No cast of the research desk on pairs is recorded.

## 3. Is there one substrate cause?

Mostly one, with a process cause on top.

1. **The signal is incidental and moves against the production design (the substrate).** "Same capture" is a by-product of duplicate coverage. The event layer and the management overlay were built to make one story per event, so as that works, the by-product disappears (hypothesis; 0 of 368 shared captures is the consistent evidence). The durable subject facts (events, entities, topic threads, judged relations) have never been stored where the walk can read them: events stayed on the computer, there are no subject nodes, and the relation edge sits behind an off switch. The three attempts to widen the signal (events, System 1 verb, `related_story` edge) each stopped one step before the reader.
2. **No standing measure, so decay and non-arrival are silent (the process cause).** The deployed spec asserts only "at least one story has a non-empty list" (`frontend/tests/autopaper-related-deployed.spec.js:14`, test at `:116`). Coverage of 7 percent or 23 percent both pass. Each build was verified on the day it landed and then not watched.
3. **Shipped-off features are not tracked as such.** The switched-off inventory (`docs/archive/evidence/switched-off-inventory-2026-10-24.md`) predates the two new switches. Residual register PR-22 still says "phase 3 never built", which is now stale (built, off).

There is no regression: no code change, deleted path, or renderer change turned a working feature off. Coverage was 20 percent, 15 percent, 23 percent and now 7 to 13 percent, with the change tracking which stories are in the window. What "stopped working" is the 10-26 00:50 snapshot being higher than the stories published since.

## 4. Smallest fix and the owner act

No code change is needed for the first step. The smallest honest path is the one the build doc already prescribes:

1. Owner act: set `research.related_story` on for the paper computer through `/api/computer-settings` (the route refuses non-platform-tier computers, so a private computer cannot write the paper's graph).
2. Cast research on the known pairs (the Serbia four, the two "20,000 California ballots" stories, the two El-Sayed stories), then on a sample. Edges are recorded with the read still off.
3. Audit at least 40 recorded pairs by reading both stories; at most 10 percent wrong.
4. Owner/ops act: set `CORPUSD_RELATED_STORY=on` in `/var/lib/go-choir/unit-env/corpusd-retrieval.env` and restart corpusd. This is the same file and procedure as `CORPUSD_CAPTURE_SEARCH`. Rollback: unset it, restart, no deploy.
5. Re-run the same public measurement. `basis.mode` becomes `captures+relations`.

Do not lower the 1.0 threshold or the hub cutoff of 6: the threshold is not the binding limit, because the walk finds no shared capture to score.

One gap this still leaves: casting is manual until the batch review (design slice 3) exists, so the next day's stories get no relations unless a cast repeats. The structural fix is the scheduled research review, plus a coverage measure (share of the first 60 front stories with a link, and `basis.mode`) added to the deployed acceptance spec with a floor, so the next regression is visible the same day. Both are code and need their own record (red for the schedule, yellow for the spec).
