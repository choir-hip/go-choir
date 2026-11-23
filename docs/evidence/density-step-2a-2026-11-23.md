# Density, step 2a: part 1 is blocked at the outcome, part 2 measured (2026-11-23)

Design step 2a ([first-design](../design/first-design-2026-11-22.md) section 10),
ledger claim C203 (open; not edited here). Script `scripts/status-density`
(`--fixture` path, case D in `scripts/test-status-scripts.sh`). Read only on
Node B; no model call of any kind, no spend, nothing written there.

## Verdict on C203

**Not measured. The claim cannot be settled from what is reachable read only.**
Its outcome is management's choice per capture, and no reachable source names
a capture with its choice. The vectors were reachable; the outcomes were not.
The script's part 1 is written, tested on a fixture, and runs the day an
outcome file exists. No AUC for part 1 is reported because none was computed.

## What blocked part 1, exactly

- The console line `management story choice` carries run, choice, story,
  offered story ids, and `captures=<n>` (a count). **No capture id.** The same
  run's other console lines carry none either. Corpusd's journal line for the
  guest's neighbour request (`op=vector_neighbours ... ids=5`) carries a count,
  not ids.
- The durable record is `choir.capture_disposition` (and `management_task`),
  registered `OwnerVMStore`, Dolt: it lives inside the paper VM's own store
  (`data.img`, a 128 GiB raw disk image), not in corpusd's `graph` database.
  `og_objects` there holds no such kind (kinds present: web_capture,
  publication_*, support_judgement, entity, podcast_* and similar). Reading a
  live guest's Dolt from the host is not a read route; I did not try.
- The only host-readable trace of a feed is `cites_capture` edges (239,705) from
  published story versions to captures. That is the proxy the offline rung 0
  used ("cited stands in for the paper's feed decision"). It is not the choice:
  it lags by hours, it cannot see declines at all (a declined capture is cited
  by nothing, like one not yet judged), and it cannot say which capture of a
  multi-capture decision it was. I did not substitute it, because the claim is
  worded on the log lines and the lead asked to stop rather than work around.

What the console does give (window 2026-11-23T19:21:50Z to 21:15:26Z, about
1 h 54 min, all five files read): 408 decisions: feed 275, open 60, decline 73,
disposing of 305, 63 and 108 captures (476 judged captures, not the 200
asked; far fewer than a day).

## To unblock (cheapest first)

1. Add `capture_ids=` (ids only) to the `management story choice` line: an
   orange change to `internal/agentcore/management_story_choice.go`. It reaches
   the same two hours of console, about 476 captures, and grows as the console
   is read each hour (`status-hourly --append` style) into a day.
2. Or an owner-authorised read of the paper's `capture_disposition` objects
   (with time) through the paper's own API: a credential question, so not tried.
3. Then: `scripts/status-density --outcomes FILE` (TSV `capture_id<TAB>feed|open|decline`).
   It reads each capture's 20 nearest earlier vectors from `vec_captures`, one
   read-only statement each, and prints n per class, quartiles, and the AUCs
   with bootstrap intervals for d1, d5, d10, d20 and mean10.

## Part 2 (not in the claim): what research's questions show

Window as above. 271 corpus searches (141 by the run's brief, 130 by a cell's
own call); 44 run summary lines. Density of a question is the cosine to its
k-th nearest capture of the paper's computer, **query embedded by the host's own
loopback query embedder (local model, no paid call) from the console's query
text**, neighbours read by one read-only statement each (5 s timeout, LIMIT 20,
HNSW `ef_search` 200, strict-order iterative scan), 272 statements in all, no
failures. The search line logs no similarity score, so recomputation was the
only way. Neighbours are captures with `day` up to the search's day, over every
capture of the computer (private and tombstoned not excluded).

Density of the question, over all 271 searches (quartiles 25/50/75):

| column | 25% | median | 75% |
|---|---|---|---|
| d1 | 0.811 | 0.836 | 0.865 |
| d5 | 0.766 | 0.796 | 0.825 |
| d10 | 0.747 | 0.768 | 0.793 |
| d20 | 0.733 | 0.749 | 0.770 |
| mean10 | 0.773 | 0.796 | 0.823 |

Joined by `run=`: 39 research runs have a cell search, a summary line and a
density (run-mean density of its cell searches); 4 cell runs have no summary
line (still running or no terminal record) and are out.

- **`from_held>0` cannot be tested: it is 0 in all 39 runs.** The summary's
  `from_held` counts web searches answered from held pages; none was in the
  window, so the outcome has no positives.
- Second definition, from the run's `corpus read ... held=` lines: relied on
  held pages = at least half its reads were held (`true` or `own`). 25 runs yes,
  14 no. AUC of run-mean density for "relied", bootstrap 95%:

| column | AUC | interval |
|---|---|---|
| d1 | 0.557 | 0.377 to 0.749 |
| d5 | 0.669 | 0.503 to 0.831 |
| d10 | 0.720 | 0.549 to 0.863 |
| d20 | 0.711 | 0.526 to 0.863 |
| mean10 | 0.689 | 0.523 to 0.846 |

- Spearman of run-mean density against the new share of pages the run's
  searches returned (`pages_new / pages_returned`), n=37: d10 -0.18
  [-0.51, 0.17], d20 -0.22 [-0.52, 0.11], d1 -0.15; all intervals include 0.
- Spearman of a cell search's density against the held captures that same
  search returned, n=130: d1 0.16 [-0.03, 0.34], d10 -0.02 [-0.20, 0.16].
  Density does not predict how many held pages a question returns.

Reading: with 39 runs the direction is that questions asked in denser regions
lead to runs reading more held pages (AUC 0.72 at d10, interval clear of 0.5
at its low end 0.55), and the continuous measure points the same way but is not
distinguishable from zero. One window, one definition chosen after the
fact among two; **not evidence for the design, a lead for a larger window**.

## What this measurement cannot say

- **Part 1 has no result.** Everything above is research, not management.
- **Independence.** Management was shown vector neighbours as candidates
  (`sources=near:` on the line; one `CaptureNeighbours` request per task) and
  prefers a feed when unclear. A capture with close earlier neighbours has
  candidates to be fed to; a feed is partly caused by the same geometry density
  measures. A high AUC would therefore be partly built in, and a low one would
  be the more informative result. An outcome independent of the offered
  candidates: what a reader or the root later does (a story fed this capture
  keeps it cited after a Texture rewrite; the story stays alive, is read, or is
  merged away), or a hand-read of whether the capture reports a happening an
  existing story covers (the offline rung 0b read).
- "Earlier" is by the vector row's `day`, so same-day later captures count;
  the neighbour list is HNSW approximate, not exact, and includes research's own
  page reads (65% of captures) as neighbours.
- Part 2: the corpus is read as of now, the search ran up to two hours ago;
  `held=own` counts as held; run-mean density mixes brief-less cell searches of
  different questions; the 39 runs share a window, a mix of stories and desks.
