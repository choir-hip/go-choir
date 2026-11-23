# The voice ask's vector half never answered on staging (2026-11-23)

**Problem.** After the ask landed (`2d44744e0`, `95ce70334`, `42fa0d9f4`) the
lead ran `scripts/proofs/autoradio-ask-paper.sh` against staging twice (14:03Z,
14:04Z). Both failed on one condition: `paper.vector` was "the caller's
deadline passed before the vector half answered". The turn found 15 stories
and queued three, all by keyword. The owner asked for keyword and vector.

**Evidence** (corpusd journal 14:03:58 to 14:04:07Z, and read-only queries on
the `graph` database, 2026-11-23):

| Step | Measured |
| --- | --- |
| index refresh (front window read) | 10 ms |
| query embedding, the two proof turns | 66 ms, 93 ms (journal, 2 h: p50 62 ms, p90 459 ms, max 2,178 ms, n=339) |
| story vector query, the two proof turns | 894 ms, 891 ms, both `context_deadline_exceeded`, 0 rows |
| story vector query, all other callers, 2 h | p50 117 ms, p90 183 ms, max 519 ms (n=337); they ask for at most 40 passages |
| same query by hand, exact read (the production shape: 72,565 passages, under the 100,000 exact limit), LIMIT 40, 45, 50, 60, 80 | 74, 79, 97, 70, 67 ms |
| same, LIMIT 100, 120, 135 | 2.5, 2.3, 2.7 s (4.3 s with the index scan on) |
| story coverage | 11,443 of 11,600 published publications have vectors (98.6%) |

**Cause.** The ask passed `StoryLimit: n*3` with n = 15 x 3 = 45, so the query
asked for 135 passages. Postgres changes plan between LIMIT 80 and 100 and the
query then takes 2.3 to 4.3 s; the vector half's 1 s budget ended it at 0.89 s
with no rows. Not the embedder (66 and 93 ms), not missing vectors, not a
slower path than the research tools'. The two halves also ran one after the
other (the vector query started only after the index walk and BM25).

**Fix.** `askMaxPassages = 80` (the last size measured fast; `askVectorPassageLimit`
asks for twice the stories, never more), and the vector half starts in a
goroutine beside the keyword half. Budgets unchanged: the platform's 1 s for the
walk and the vector half, plus 250 ms for the row reads, inside the desk's 1.5 s
wait. At the measured percentiles the vector half is embed p90 459 ms + query
about 100 ms = about 560 ms, inside the 1 s; an embed at the 2.2 s maximum is
cut and the answer says the vector half did not answer.

**Durable fix, not built** (review, 2026-11-23). Make the statement's plan
independent of LIMIT: pin it with a statement-level `SET LOCAL` for this query,
or force the ordered distance scan, so the exact read under 100,000 rows cannot
turn into a full sort at a higher limit. Or read in two stages (page the index
order and stop once enough distinct `story_id`s are found), so the passage count
stops being a constant tuned to today's planner.

**Watch.** Each ask now logs `corpusd: published ask vector: ms= asked= passages=
stories= wanted=` (and ` short` when the passages gave fewer distinct stories
than the limit asked for), so the cliff moving, or a thin yield, is seen.

**Remaining error.** The cliff is a planner choice measured on one day's data;
if the passage count grows it may move, and 80 must be measured again. Not
proved on staging until the lead reruns the proof after a deploy.
